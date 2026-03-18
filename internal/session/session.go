package session

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/mmmorris1975/ssm-session-client/datachannel"
)

func Start(ctx context.Context, cfg aws.Config, instanceID string, keepalive time.Duration) error {
	c := new(datachannel.SsmDataChannel)
	if err := c.Open(cfg, &ssm.StartSessionInput{Target: aws.String(instanceID)}); err != nil {
		return fmt.Errorf("failed to start session: %w", err)
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
