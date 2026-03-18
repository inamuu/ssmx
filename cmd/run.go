package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	internalaws "github.com/inamuu/ssmx/internal/aws"
	"github.com/inamuu/ssmx/internal/session"
	"github.com/inamuu/ssmx/internal/ui"
	"github.com/spf13/cobra"
)

func run(cmd *cobra.Command, args []string) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	resolvedProfile := resolveProfile()

	cfg, err := internalaws.LoadConfig(ctx, resolvedProfile, region)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	var instanceID string
	if target != "" {
		instanceID = target
	} else {
		instances, err := internalaws.ListRunningInstances(ctx, cfg)
		if err != nil {
			return fmt.Errorf("failed to list instances: %w", err)
		}
		if len(instances) == 0 {
			return fmt.Errorf("no running instances found")
		}

		selected, err := ui.SelectInstance(instances)
		if err != nil {
			return fmt.Errorf("instance selection cancelled: %w", err)
		}
		instanceID = selected.InstanceID
	}

	fmt.Printf("Starting session to %s (keepalive: %ds)...\n", instanceID, keepalive)

	return session.Start(ctx, cfg, instanceID, time.Duration(keepalive)*time.Second)
}

func resolveProfile() string {
	if profile != "" {
		return profile
	}
	if p := os.Getenv("AWS_PROFILE"); p != "" {
		return p
	}
	return "default"
}
