package session

import (
	"encoding/base64"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestCheckUploadOutput(t *testing.T) {
	if path, err := checkUploadOutput("__SSMX_READY__\n__SSMX_DONE__ 12 /tmp/run.sh\n", 12); err != nil || path != "/tmp/run.sh" {
		t.Fatalf("expected success, got %q, %v", path, err)
	}
	if _, err := checkUploadOutput("__SSMX_DONE__ 10 /tmp/a\n", 12); err == nil {
		t.Fatal("size mismatch should fail")
	}
	if _, err := checkUploadOutput("__SSMX_ERR__ base64 command not found in container\n", 12); err == nil || err.Error() != "base64 command not found in container" {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := checkUploadOutput("", 12); err == nil {
		t.Fatal("empty output should fail")
	}
}

// TestUploadScript runs the container-side script with a local sh.
func TestUploadScript(t *testing.T) {
	dir := t.TempDir()
	content := "hello\nworld\n"
	input := "\n" + base64.StdEncoding.EncodeToString([]byte(content)) + "\n"

	for _, dest := range []string{dir, dir + "/", dir + "/out.txt"} {
		cmd := exec.Command("sh", "-c", uploadScript, "ssmx", dest, "", "run.sh")
		cmd.Stdin = strings.NewReader(input)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("dest %q: %v: %s", dest, err, out)
		}
		path, err := checkUploadOutput(string(out), len(content))
		if err != nil {
			t.Fatalf("dest %q: %v", dest, err)
		}
		if got, _ := os.ReadFile(path); string(got) != content {
			t.Fatalf("dest %q: wrote %q to %s", dest, got, path)
		}
	}

	cmd := exec.Command("sh", "-c", uploadScript, "ssmx", "/nonexistent/dir/x", "", "run.sh")
	cmd.Stdin = strings.NewReader("\n")
	out, _ := cmd.CombinedOutput()
	if _, err := checkUploadOutput(string(out), 0); err == nil || !strings.Contains(err.Error(), "cannot write") {
		t.Fatalf("expected write error, got %v (%s)", err, out)
	}
}
