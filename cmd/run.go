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

func runSession(cmd *cobra.Command, args []string) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	resolvedProfile := resolveProfile()

	cfg, err := internalaws.LoadConfig(ctx, resolvedProfile, region)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	var selectedTarget internalaws.SessionTarget
	if target != "" {
		selectedTarget = internalaws.SessionTarget{
			Kind:     internalaws.SessionTargetKindEC2,
			TargetID: target,
		}
	} else {
		targets, err := internalaws.ListSessionTargets(ctx, cfg)
		if err != nil {
			return fmt.Errorf("failed to list targets: %w", err)
		}
		if len(targets) == 0 {
			return fmt.Errorf("no runnable EC2 instances or ECS exec targets found")
		}

		selected, err := ui.SelectSessionTarget(targets)
		if err != nil {
			return fmt.Errorf("target selection cancelled: %w", err)
		}
		selectedTarget = *selected
	}

	if command != "" {
		if selectedTarget.Kind != internalaws.SessionTargetKindECS {
			return fmt.Errorf("--command is only supported for ECS targets")
		}
		selectedTarget.Command = command
	}

	fmt.Printf("Starting session to %s (keepalive: %ds)...\n", selectedTarget.PrimaryLabel(), keepalive)

	return session.Start(ctx, cfg, selectedTarget, time.Duration(keepalive)*time.Second)
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
