//go:build !windows

package session

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"os/signal"
	"time"

	"github.com/aws/session-manager-plugin/src/config"
	"github.com/aws/session-manager-plugin/src/log"
	"github.com/aws/session-manager-plugin/src/message"
	pluginsession "github.com/aws/session-manager-plugin/src/sessionmanagerplugin/session"
	"github.com/aws/session-manager-plugin/src/sessionmanagerplugin/session/sessionutil"
	"golang.org/x/crypto/ssh/terminal"
)

const (
	ecsResizePollInterval      = 500 * time.Millisecond
	ecsResizeKeepaliveInterval = 5 * time.Minute
	ecsStdinBufferLimit        = 1024
)

type ecsKeepaliveShellSession struct {
	pluginsession.Session
	sizeData          message.SizeData
	lastSizeSentAt    time.Time
	originalSttyState bytes.Buffer
}

func init() {
	pluginsession.Register(&ecsKeepaliveShellSession{})
}

func (ecsKeepaliveShellSession) Name() string {
	return config.ShellPluginName
}

func (s *ecsKeepaliveShellSession) Initialize(logger log.T, sessionVar *pluginsession.Session) {
	s.Session = *sessionVar
	s.DataChannel.RegisterOutputStreamHandler(s.ProcessStreamMessagePayload, true)
	s.DataChannel.GetWsChannel().SetOnMessage(func(input []byte) {
		s.DataChannel.OutputMessageHandler(logger, s.Stop, s.SessionId, input)
	})
}

func (s *ecsKeepaliveShellSession) SetSessionHandlers(logger log.T) error {
	s.handleTerminalResize(logger)
	s.handleControlSignals(logger)
	return s.handleKeyboardInput(logger)
}

func (s *ecsKeepaliveShellSession) ProcessStreamMessagePayload(logger log.T, outputMessage message.ClientMessage) (bool, error) {
	s.DisplayMode.DisplayMessage(logger, outputMessage)
	return true, nil
}

func (s *ecsKeepaliveShellSession) Stop() {
	_ = setSttyState(&s.originalSttyState)
	_ = setSttyState(bytes.NewBufferString("echo"))
	os.Exit(0)
}

func (s *ecsKeepaliveShellSession) handleControlSignals(logger log.T) {
	go func() {
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, sessionutil.ControlSignals...)
		for {
			sig := <-signals
			if b, ok := sessionutil.SignalsByteMap[sig]; ok {
				if err := s.DataChannel.SendInputDataMessage(logger, message.Output, []byte{b}); err != nil {
					logger.Errorf("Failed to send control signals: %v", err)
				}
			}
		}
	}()
}

func (s *ecsKeepaliveShellSession) handleTerminalResize(logger log.T) {
	go func() {
		for {
			width, height, err := terminal.GetSize(int(os.Stdout.Fd()))
			if err != nil {
				width = 300
				height = 100
				logger.Errorf("Could not get size of the terminal: %s, using width %d height %d", err, width, height)
			}

			sizeData := message.SizeData{
				Cols: uint32(width),
				Rows: uint32(height),
			}
			shouldSend := s.sizeData.Rows != sizeData.Rows || s.sizeData.Cols != sizeData.Cols
			if !shouldSend && !s.lastSizeSentAt.IsZero() && time.Since(s.lastSizeSentAt) >= ecsResizeKeepaliveInterval {
				shouldSend = true
			}
			if shouldSend || s.lastSizeSentAt.IsZero() {
				s.sizeData = sizeData
				s.lastSizeSentAt = time.Now()
				inputSizeData, marshalErr := json.Marshal(sizeData)
				if marshalErr != nil {
					logger.Errorf("Cannot marshal size data: %v", marshalErr)
				} else if err := s.DataChannel.SendInputDataMessage(logger, message.Size, inputSizeData); err != nil {
					logger.Errorf("Failed to send size data: %v", err)
				}
			}

			time.Sleep(ecsResizePollInterval)
		}
	}()
}

func (s *ecsKeepaliveShellSession) handleKeyboardInput(logger log.T) error {
	s.disableEchoAndInputBuffering()

	stdinBytes := make([]byte, ecsStdinBufferLimit)
	reader := bufio.NewReader(os.Stdin)
	for {
		stdinBytesLen, err := reader.Read(stdinBytes)
		if err != nil {
			logger.Errorf("Unable read from stdin: %v", err)
			return err
		}

		if err := s.Session.DataChannel.SendInputDataMessage(logger, message.Output, stdinBytes[:stdinBytesLen]); err != nil {
			logger.Errorf("Failed to send UTF8 char: %v", err)
			return err
		}
		time.Sleep(time.Millisecond)
	}
}

func (s *ecsKeepaliveShellSession) disableEchoAndInputBuffering() {
	_ = getSttyState(&s.originalSttyState)
	_ = setSttyState(bytes.NewBufferString("cbreak"))
	_ = setSttyState(bytes.NewBufferString("-echo"))
}

func getSttyState(state *bytes.Buffer) error {
	cmd := exec.Command("stty", "-g")
	cmd.Stdin = os.Stdin
	cmd.Stdout = state
	return cmd.Run()
}

func setSttyState(state *bytes.Buffer) error {
	cmd := exec.Command("stty", state.String())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	return cmd.Run()
}
