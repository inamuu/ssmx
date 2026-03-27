package session

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	internalaws "github.com/inamuu/ssmx/internal/aws"
	"github.com/mmmorris1975/ssm-session-client/datachannel"
)

func Start(ctx context.Context, cfg aws.Config, target internalaws.SessionTarget, keepalive time.Duration) error {
	if target.Kind == internalaws.SessionTargetKindECS {
		if err := startECSPluginSession(ctx, cfg, target); err != nil {
			return fmt.Errorf("failed to start session (%s): %w", target.ErrorLabel(), err)
		}
		return nil
	}

	c := new(datachannel.SsmDataChannel)
	if err := openSession(ctx, c, cfg, target); err != nil {
		return fmt.Errorf("failed to start session (%s): %w", target.ErrorLabel(), err)
	}
	defer c.Close()

	if err := initializeTerminal(c); err != nil {
		return fmt.Errorf("failed to initialize terminal: %w", err)
	}
	defer cleanupTerminal()

	// Start keepalive goroutine
	stopKeepalive := make(chan struct{})
	go runKeepalive(c, keepalive, stopKeepalive)
	defer close(stopKeepalive)

	errCh := make(chan error, 5)

	// stdin -> data channel
	go func() {
		if _, err := io.Copy(c, os.Stdin); err != nil {
			errCh <- err
		}
	}()

	// data channel -> stdout
	if _, err := io.Copy(os.Stdout, c); err != nil {
		if !errors.Is(err, io.EOF) {
			errCh <- err
		}
	}
	close(errCh)

	return <-errCh
}

func openSession(ctx context.Context, c *datachannel.SsmDataChannel, cfg aws.Config, target internalaws.SessionTarget) error {
	switch target.Kind {
	case internalaws.SessionTargetKindECS:
		output, err := ecs.NewFromConfig(cfg).ExecuteCommand(ctx, &ecs.ExecuteCommandInput{
			Cluster:     aws.String(target.ClusterName),
			Task:        aws.String(target.TaskArn),
			Container:   aws.String(target.ContainerName),
			Command:     aws.String(target.Command),
			Interactive: true,
		})
		if err != nil {
			return err
		}
		if output.Session == nil || output.Session.StreamUrl == nil || output.Session.TokenValue == nil {
			return errors.New("execute-command response missing session details")
		}
		return c.StartSessionFromDataChannelURL(aws.ToString(output.Session.StreamUrl), aws.ToString(output.Session.TokenValue))
	default:
		return c.Open(cfg, &ssm.StartSessionInput{Target: aws.String(target.TargetID)})
	}
}

// runKeepalive periodically sends a terminal size update to keep the WebSocket alive.
func runKeepalive(c *datachannel.SsmDataChannel, interval time.Duration, stop <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			rows, cols, err := getWinSize()
			if err != nil {
				rows = 45
				cols = 132
			}
			_ = c.SetTerminalSize(rows, cols)
		}
	}
}
