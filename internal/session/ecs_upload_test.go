package session

import "testing"

func TestCheckUploadOutput(t *testing.T) {
	if err := checkUploadOutput("__SSMX_READY__\n__SSMX_DONE__ 12\n", 12); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if err := checkUploadOutput("__SSMX_READY__\n__SSMX_DONE__      12\n", 12); err != nil {
		t.Fatalf("padded wc output should be accepted, got %v", err)
	}
	if err := checkUploadOutput("__SSMX_DONE__ 10\n", 12); err == nil {
		t.Fatal("size mismatch should fail")
	}
	if err := checkUploadOutput("__SSMX_ERR__ base64 command not found in container\n", 12); err == nil || err.Error() != "base64 command not found in container" {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := checkUploadOutput("", 12); err == nil {
		t.Fatal("empty output should fail")
	}
}
