package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildRemoteRunSpec_Command(t *testing.T) {
	spec, err := buildRemoteRunSpec("echo", "", []string{"hello world", "it's"})
	if err != nil {
		t.Fatalf("buildRemoteRunSpec returned error: %v", err)
	}

	if got, want := spec.displayName, "inline command"; got != want {
		t.Fatalf("displayName = %q, want %q", got, want)
	}

	want := "echo 'hello world' 'it'\"'\"'s'"
	if got := spec.commands[0]; got != want {
		t.Fatalf("command = %q, want %q", got, want)
	}
}

func TestBuildRemoteRunSpec_Script(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "deploy.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\necho hi"), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	spec, err := buildRemoteRunSpec("", scriptPath, []string{"--dry-run"})
	if err != nil {
		t.Fatalf("buildRemoteRunSpec returned error: %v", err)
	}

	if got, want := spec.displayName, scriptPath; got != want {
		t.Fatalf("displayName = %q, want %q", got, want)
	}
	if len(spec.commands) != 1 {
		t.Fatalf("len(commands) = %d, want 1", len(spec.commands))
	}
	if got := spec.commands[0]; !strings.Contains(got, "cat <<'__SSMX_REMOTE_SCRIPT__' > '/tmp/deploy.sh'") {
		t.Fatalf("script command missing upload preamble: %q", got)
	}
	if got := spec.commands[0]; !strings.Contains(got, "'/tmp/deploy.sh' '--dry-run'") {
		t.Fatalf("script command missing execution line: %q", got)
	}
}

func TestBuildRemoteRunSpec_CommandFileBecomesScript(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "task.sh")
	if err := os.WriteFile(scriptPath, []byte("echo ok\n"), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	spec, err := buildRemoteRunSpec(scriptPath, "", []string{"arg1"})
	if err != nil {
		t.Fatalf("buildRemoteRunSpec returned error: %v", err)
	}
	if got, want := spec.displayName, scriptPath; got != want {
		t.Fatalf("displayName = %q, want %q", got, want)
	}
	if got := spec.commands[0]; !strings.Contains(got, "'/tmp/task.sh' 'arg1'") {
		t.Fatalf("script command missing args: %q", got)
	}
}
