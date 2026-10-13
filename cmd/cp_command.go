package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	internalaws "github.com/inamuu/ssmx/internal/aws"
	"github.com/inamuu/ssmx/internal/session"
	"github.com/spf13/cobra"
)

type remoteCopyFlags struct {
	target       string
	targetType   string
	documentName string
	timeout      time.Duration
	workDir      string
}

type remoteCopySpec struct {
	commands    []string
	content     []byte
	displayName string
	remotePath  string
}

func newCopyCommand() *cobra.Command {
	flags := &remoteCopyFlags{}

	cmd := &cobra.Command{
		Use:   "cp <local-path> [remote-path]",
		Short: "Upload a local file to an SSM-managed instance or ECS container",
		Long: "Upload a local file to an SSM-managed EC2 instance (via AWS Run Command) or an ECS container\n" +
			"(via ECS Exec) without executing it. ECS containers need /bin/sh and base64.",
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return copyRemoteFile(cmd, args, flags)
		},
	}

	cmd.Flags().StringVarP(&flags.target, "target", "t", "", "Instance ID or ecs:<cluster>/<task-id>/<container> (skip interactive selection)")
	cmd.Flags().StringVar(&flags.target, "instance", "", "Alias of --target")
	cmd.Flags().StringVar(&flags.targetType, "type", "", "Show only one target type in the selector: ec2 or ecs")
	cmd.Flags().StringVarP(&flags.documentName, "document", "d", "AWS-RunShellScript", "SSM document name (EC2 only)")
	cmd.Flags().DurationVar(&flags.timeout, "timeout", 10*time.Minute, "Overall timeout")
	cmd.Flags().StringVarP(&flags.workDir, "workdir", "w", "", "Remote working directory")
	_ = cmd.Flags().MarkHidden("instance")

	return cmd
}

func copyRemoteFile(cmd *cobra.Command, args []string, flags *remoteCopyFlags) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if flags.timeout > 0 {
		var timeoutCancel context.CancelFunc
		ctx, timeoutCancel = context.WithTimeout(ctx, flags.timeout)
		defer timeoutCancel()
	}

	spec, err := buildRemoteCopySpec(args[0], args[1:])
	if err != nil {
		return err
	}

	cfg, err := internalaws.LoadConfig(ctx, resolveProfile(), region)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	selected, err := resolveTarget(ctx, cfg, flags.target, flags.targetType, internalaws.ListCopyTargets)
	if err != nil {
		return err
	}

	if selected.Kind == internalaws.SessionTargetKindECS {
		if err := session.UploadToECS(ctx, cfg, selected, spec.remotePath, flags.workDir, spec.content); err != nil {
			return fmt.Errorf("remote copy to %s failed: %w", selected.ErrorLabel(), err)
		}
		fmt.Fprintf(os.Stderr, "container: %s\n", selected.PrimaryLabel())
		fmt.Fprintf(os.Stderr, "source: %s\n", spec.displayName)
		fmt.Fprintf(os.Stderr, "destination: %s\n", spec.remotePath)
		fmt.Fprintf(os.Stderr, "status: Success\n")
		return nil
	}

	instanceID := selected.TargetID
	commandID, err := internalaws.SendCommand(ctx, cfg, instanceID, flags.documentName, flags.workDir, spec.commands)
	if err != nil {
		return fmt.Errorf("failed to send command: %w", err)
	}

	invocation, err := internalaws.WaitForCommandInvocation(ctx, cfg, commandID, instanceID)
	if err != nil {
		return fmt.Errorf("failed to get command result: %w", err)
	}

	fmt.Fprintf(os.Stderr, "instance: %s\n", instanceID)
	fmt.Fprintf(os.Stderr, "source: %s\n", spec.displayName)
	fmt.Fprintf(os.Stderr, "destination: %s\n", spec.remotePath)
	fmt.Fprintf(os.Stderr, "status: %s\n", aws.ToString(invocation.StatusDetails))

	if out := strings.TrimRight(aws.ToString(invocation.StandardOutputContent), "\n"); out != "" {
		fmt.Println(out)
	}
	if errOut := strings.TrimRight(aws.ToString(invocation.StandardErrorContent), "\n"); errOut != "" {
		fmt.Fprintln(os.Stderr, errOut)
	}

	if invocation.Status != ssmtypes.CommandInvocationStatusSuccess {
		return fmt.Errorf("remote copy finished with status %s", aws.ToString(invocation.StatusDetails))
	}

	return nil
}

func buildRemoteCopySpec(localPath string, args []string) (remoteCopySpec, error) {
	content, err := os.ReadFile(localPath)
	if err != nil {
		return remoteCopySpec{}, fmt.Errorf("read local file: %w", err)
	}

	remotePath := defaultRemoteScriptPath(localPath)
	if len(args) > 0 && args[0] != "" {
		remotePath = args[0]
	}

	var b strings.Builder
	writeRemoteUploadCommand(&b, remotePath, content)

	return remoteCopySpec{
		commands:    []string{b.String()},
		content:     content,
		displayName: localPath,
		remotePath:  remotePath,
	}, nil
}
