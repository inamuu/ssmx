package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildRemoteCopySpec_DefaultDestination(t *testing.T) {
	dir := t.TempDir()
	localPath := filepath.Join(dir, "deploy.sh")
	if err := os.WriteFile(localPath, []byte("#!/bin/sh\necho hi"), 0o644); err != nil {
		t.Fatalf("write local file: %v", err)
	}

	spec, err := buildRemoteCopySpec(localPath, nil)
	if err != nil {
		t.Fatalf("buildRemoteCopySpec returned error: %v", err)
	}

	if got, want := spec.displayName, localPath; got != want {
		t.Fatalf("displayName = %q, want %q", got, want)
	}
	if got, want := spec.remotePath, "/tmp/deploy.sh"; got != want {
		t.Fatalf("remotePath = %q, want %q", got, want)
	}
	if got := spec.commands[0]; !strings.Contains(got, "cat <<'__SSMX_REMOTE_SCRIPT__' > '/tmp/deploy.sh'") {
		t.Fatalf("copy command missing upload preamble: %q", got)
	}
	if got := spec.commands[0]; strings.Contains(got, "chmod +x") {
		t.Fatalf("copy command should not chmod: %q", got)
	}
	if got := spec.commands[0]; strings.Contains(got, "'/tmp/deploy.sh'") && strings.Contains(got, "chmod +x") {
		t.Fatalf("copy command should not execute file: %q", got)
	}
}

func TestBuildRemoteCopySpec_CustomDestination(t *testing.T) {
	dir := t.TempDir()
	localPath := filepath.Join(dir, "task.txt")
	if err := os.WriteFile(localPath, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write local file: %v", err)
	}

	spec, err := buildRemoteCopySpec(localPath, []string{"/var/tmp/copied.txt"})
	if err != nil {
		t.Fatalf("buildRemoteCopySpec returned error: %v", err)
	}

	if got, want := spec.remotePath, "/var/tmp/copied.txt"; got != want {
		t.Fatalf("remotePath = %q, want %q", got, want)
	}
	if got := spec.commands[0]; !strings.Contains(got, "cat <<'__SSMX_REMOTE_SCRIPT__' > '/var/tmp/copied.txt'") {
		t.Fatalf("copy command missing custom destination: %q", got)
	}
}
