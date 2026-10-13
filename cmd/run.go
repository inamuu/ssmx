package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
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

	selectedTarget, err := resolveTarget(ctx, cfg, target, targetType, internalaws.ListSessionTargets)
	if err != nil {
		return err
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

type targetLister func(context.Context, aws.Config, internalaws.SessionTargetKind) ([]internalaws.SessionTarget, error)

// resolveTarget parses --target when given, otherwise lists targets and lets
// the user pick one interactively.
func resolveTarget(ctx context.Context, cfg aws.Config, spec, kindFlag string, list targetLister) (internalaws.SessionTarget, error) {
	kind, err := internalaws.ParseTargetKind(kindFlag)
	if err != nil {
		return internalaws.SessionTarget{}, err
	}

	if spec != "" {
		return internalaws.ParseTargetSpec(spec)
	}

	targets, err := list(ctx, cfg, kind)
	if err != nil {
		return internalaws.SessionTarget{}, fmt.Errorf("failed to list targets: %w", err)
	}
	if len(targets) == 0 {
		return internalaws.SessionTarget{}, fmt.Errorf("no EC2 instances or ECS exec targets found")
	}

	selected, err := ui.SelectSessionTarget(targets)
	if err != nil {
		return internalaws.SessionTarget{}, fmt.Errorf("target selection cancelled: %w", err)
	}
	return *selected, nil
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
