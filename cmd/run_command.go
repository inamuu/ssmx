package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	internalaws "github.com/inamuu/ssmx/internal/aws"
	"github.com/inamuu/ssmx/internal/ui"
	"github.com/spf13/cobra"
)

type remoteRunFlags struct {
	target       string
	command      string
	script       string
	documentName string
	timeout      time.Duration
	workDir      string
}

type remoteRunSpec struct {
	commands    []string
	displayName string
}

func newRunCommand() *cobra.Command {
	flags := &remoteRunFlags{}

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run an SSM Run Command or local script on an instance",
		Long:  "Run a shell command or upload and execute a local script on an SSM-managed EC2 instance.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRemoteCommand(cmd, args, flags)
		},
	}

	cmd.Flags().StringVarP(&flags.target, "target", "t", "", "Instance ID (skip interactive selection)")
	cmd.Flags().StringVar(&flags.target, "instance", "", "Alias of --target")
	cmd.Flags().StringVarP(&flags.command, "command", "c", "", "Shell command to run on the instance")
	cmd.Flags().StringVarP(&flags.script, "script", "s", "", "Local script file to upload and run on the instance")
	cmd.Flags().StringVarP(&flags.documentName, "document", "d", "AWS-RunShellScript", "SSM document name")
	cmd.Flags().DurationVar(&flags.timeout, "timeout", 10*time.Minute, "Overall timeout")
	cmd.Flags().StringVarP(&flags.workDir, "workdir", "w", "", "Remote working directory")
	_ = cmd.Flags().MarkHidden("instance")

	return cmd
}

func runRemoteCommand(cmd *cobra.Command, args []string, flags *remoteRunFlags) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if flags.timeout > 0 {
		var timeoutCancel context.CancelFunc
		ctx, timeoutCancel = context.WithTimeout(ctx, flags.timeout)
		defer timeoutCancel()
	}

	spec, err := buildRemoteRunSpec(flags.command, flags.script, args)
	if err != nil {
		return err
	}

	cfg, err := internalaws.LoadConfig(ctx, resolveProfile(), region)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	instanceID := flags.target
	if instanceID == "" {
		instances, err := internalaws.ListManagedInstances(ctx, cfg)
		if err != nil {
			return fmt.Errorf("failed to list managed instances: %w", err)
		}
		if len(instances) == 0 {
			return fmt.Errorf("no online SSM-managed EC2 instances found")
		}

		selected, err := ui.SelectInstance(instances)
		if err != nil {
			return fmt.Errorf("instance selection cancelled: %w", err)
		}
		instanceID = selected.InstanceID
	}

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
	fmt.Fprintf(os.Stderr, "status: %s\n", aws.ToString(invocation.StatusDetails))

	if out := strings.TrimRight(aws.ToString(invocation.StandardOutputContent), "\n"); out != "" {
		fmt.Println(out)
	}
	if errOut := strings.TrimRight(aws.ToString(invocation.StandardErrorContent), "\n"); errOut != "" {
		fmt.Fprintln(os.Stderr, errOut)
	}

	if invocation.Status != ssmtypes.CommandInvocationStatusSuccess {
		return fmt.Errorf("remote command finished with status %s", aws.ToString(invocation.StatusDetails))
	}

	return nil
}

func buildRemoteRunSpec(command, script string, args []string) (remoteRunSpec, error) {
	if (command == "") == (script == "") {
		return remoteRunSpec{}, fmt.Errorf("specify exactly one of --command or --script")
	}

	if command != "" {
		if info, err := os.Stat(command); err == nil && !info.IsDir() {
			script = command
			command = ""
		}
	}

	if command != "" {
		fullCommand := command
		for _, arg := range args {
			fullCommand += " " + shellQuote(arg)
		}
		return remoteRunSpec{
			commands:    []string{fullCommand},
			displayName: "inline command",
		}, nil
	}

	content, err := os.ReadFile(script)
	if err != nil {
		return remoteRunSpec{}, fmt.Errorf("read script: %w", err)
	}

	remotePath := "/tmp/" + filepath.Base(script)
	var b strings.Builder
	fmt.Fprintf(&b, "cat <<'__SSMX_REMOTE_SCRIPT__' > %s\n", shellQuote(remotePath))
	b.Write(content)
	if len(content) == 0 || content[len(content)-1] != '\n' {
		b.WriteByte('\n')
	}
	fmt.Fprintf(&b, "__SSMX_REMOTE_SCRIPT__\nchmod +x %s\n", shellQuote(remotePath))
	fmt.Fprintf(&b, "%s", shellQuote(remotePath))
	for _, arg := range args {
		fmt.Fprintf(&b, " %s", shellQuote(arg))
	}
	b.WriteByte('\n')

	return remoteRunSpec{
		commands:    []string{b.String()},
		displayName: script,
	}, nil
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}
