package session

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	plugindatachannel "github.com/aws/session-manager-plugin/src/datachannel"
	"github.com/aws/session-manager-plugin/src/log"
	"github.com/aws/session-manager-plugin/src/message"
	pluginsession "github.com/aws/session-manager-plugin/src/sessionmanagerplugin/session"
	"github.com/google/uuid"
	internalaws "github.com/inamuu/ssmx/internal/aws"
)

const (
	uploadReadyMarker = "__SSMX_READY__"
	uploadDoneMarker  = "__SSMX_DONE__"
	uploadErrMarker   = "__SSMX_ERR__"

	uploadReadyTimeout   = 60 * time.Second
	uploadLineLength     = 76
	uploadLinesPerPacket = 16
)

// uploadScript runs inside the container. It turns off tty echo, tells the
// client it is ready, then decodes base64 from stdin into $1 until EOF.
const uploadScript = `p="$1"; ` +
	`if [ -n "$2" ]; then cd "$2" || { echo ` + uploadErrMarker + ` cannot cd to "$2"; exit 1; }; fi; ` +
	`command -v base64 >/dev/null 2>&1 || { echo ` + uploadErrMarker + ` base64 command not found in container; exit 1; }; ` +
	`stty -echo 2>/dev/null; ` +
	`echo ` + uploadReadyMarker + `; ` +
	`base64 -d > "$p" || { echo ` + uploadErrMarker + ` failed to write "$p"; exit 1; }; ` +
	`echo ` + uploadDoneMarker + ` $(wc -c < "$p")`

// activeUpload switches the registered ECS shell session into upload mode.
// The session plugin registry is global, so this is set only while
// UploadToECS runs.
var activeUpload *ecsUpload

// UploadToECS writes content to remotePath inside an ECS container through
// ECS Exec. The file is streamed as base64 over the session's stdin, so the
// container needs /bin/sh and base64.
func UploadToECS(ctx context.Context, cfg aws.Config, target internalaws.SessionTarget, remotePath, workDir string, content []byte) error {
	if strings.Contains(remotePath, "'") || strings.Contains(workDir, "'") {
		return errors.New("remote path and workdir must not contain single quotes")
	}

	command := fmt.Sprintf("/bin/sh -c '%s' ssmx '%s' '%s'", uploadScript, remotePath, workDir)
	output, err := ecs.NewFromConfig(cfg).ExecuteCommand(ctx, &ecs.ExecuteCommandInput{
		Cluster:     aws.String(target.ClusterName),
		Task:        aws.String(target.TaskArn),
		Container:   aws.String(target.ContainerName),
		Command:     aws.String(command),
		Interactive: true,
	})
	if err != nil {
		return err
	}
	if output.Session == nil || output.Session.SessionId == nil || output.Session.StreamUrl == nil || output.Session.TokenValue == nil {
		return errors.New("execute-command response missing session details")
	}

	u := &ecsUpload{ctx: ctx, content: content, ready: make(chan struct{}), done: make(chan struct{})}
	activeUpload = u
	defer func() { activeUpload = nil }()

	s := &pluginsession.Session{
		SessionId:   aws.ToString(output.Session.SessionId),
		StreamUrl:   aws.ToString(output.Session.StreamUrl),
		TokenValue:  aws.ToString(output.Session.TokenValue),
		ClientId:    uuid.NewString(),
		TargetId:    target.PrimaryLabel(),
		Region:      cfg.Region,
		DataChannel: &plugindatachannel.DataChannel{},
	}

	// The plugin prints session start/exit banners to stdout; keep them out
	// of the cp output.
	if devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0); err == nil {
		stdout := os.Stdout
		os.Stdout = devNull
		defer func() {
			os.Stdout = stdout
			devNull.Close()
		}()
	}

	logger := log.Logger(false, s.ClientId)
	execErr := s.Execute(logger)
	if s.DataChannel != nil {
		_ = s.DataChannel.Close(logger)
	}
	if execErr != nil {
		return execErr
	}

	return checkUploadOutput(u.outputString(), len(content))
}

func checkUploadOutput(out string, wantSize int) error {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if msg, ok := strings.CutPrefix(line, uploadErrMarker); ok {
			return errors.New(strings.TrimSpace(msg))
		}
		if size, ok := strings.CutPrefix(line, uploadDoneMarker); ok {
			got, err := strconv.Atoi(strings.TrimSpace(size))
			if err != nil {
				return fmt.Errorf("unexpected size output %q", line)
			}
			if got != wantSize {
				return fmt.Errorf("size mismatch: wrote %d bytes, want %d", got, wantSize)
			}
			return nil
		}
	}

	out = strings.TrimSpace(out)
	if out == "" {
		return errors.New("upload did not complete (no output from container)")
	}
	return fmt.Errorf("upload did not complete: %s", out)
}

type ecsUpload struct {
	ctx     context.Context
	content []byte

	mu        sync.Mutex
	output    bytes.Buffer
	ready     chan struct{}
	readyOnce sync.Once
	done      chan struct{}
	doneOnce  sync.Once
}

func (u *ecsUpload) appendOutput(p []byte) {
	u.mu.Lock()
	u.output.Write(p)
	out := u.output.String()
	u.mu.Unlock()

	if strings.Contains(out, uploadReadyMarker) {
		u.readyOnce.Do(func() { close(u.ready) })
	}
}

func (u *ecsUpload) outputString() string {
	u.mu.Lock()
	defer u.mu.Unlock()
	return strings.ReplaceAll(u.output.String(), "\r", "")
}

func (u *ecsUpload) finish() {
	u.doneOnce.Do(func() { close(u.done) })
}

// run is called as the session handler once the data channel is established.
// It returns when the remote command exits (channel closed) or ctx is done.
func (u *ecsUpload) run(logger log.T, dc plugindatachannel.IDataChannel) error {
	sizeData, _ := json.Marshal(message.SizeData{Cols: 200, Rows: 50})
	if err := dc.SendInputDataMessage(logger, message.Size, sizeData); err != nil {
		return fmt.Errorf("send terminal size: %w", err)
	}

	select {
	case <-u.ready:
	case <-u.done:
		return nil
	case <-u.ctx.Done():
		return u.ctx.Err()
	case <-time.After(uploadReadyTimeout):
		return errors.New("timed out waiting for the container to become ready")
	}

	encoded := base64.StdEncoding.EncodeToString(u.content)
	var packet strings.Builder
	flush := func() error {
		if packet.Len() == 0 {
			return nil
		}
		if err := dc.SendInputDataMessage(logger, message.Output, []byte(packet.String())); err != nil {
			return fmt.Errorf("send file data: %w", err)
		}
		packet.Reset()
		time.Sleep(time.Millisecond)
		return nil
	}

	lines := 0
	for start := 0; start < len(encoded); start += uploadLineLength {
		end := min(start+uploadLineLength, len(encoded))
		packet.WriteString(encoded[start:end])
		packet.WriteByte('\n')
		lines++
		if lines%uploadLinesPerPacket == 0 {
			if err := flush(); err != nil {
				return err
			}
		}
		select {
		case <-u.done:
			return nil
		case <-u.ctx.Done():
			return u.ctx.Err()
		default:
		}
	}
	if err := flush(); err != nil {
		return err
	}

	// Ctrl-D at the beginning of a line ends stdin for base64 -d.
	if err := dc.SendInputDataMessage(logger, message.Output, []byte{0x04}); err != nil {
		return fmt.Errorf("send EOF: %w", err)
	}

	select {
	case <-u.done:
		return nil
	case <-u.ctx.Done():
		return u.ctx.Err()
	}
}
