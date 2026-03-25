package aws

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/credentials/logincreds"
)

func TestResolveLoginFallbackPlan_PrefersSourceProfileForAssumeRole(t *testing.T) {
	t.Setenv("AWS_PROFILE", "")
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_DEFAULT_REGION", "")
	t.Setenv("AWS_CONFIG_FILE", filepath.Join(t.TempDir(), "config"))
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(t.TempDir(), "credentials"))

	configPath := os.Getenv("AWS_CONFIG_FILE")
	configBody := `[default]
region = ap-northeast-1
login_session = arn:aws:iam::489667813116:user/inamuu
duration_seconds = 3600

[profile estack]
region = ap-northeast-1
source_profile = default
role_arn = arn:aws:iam::194015964148:role/kazumainamura_assumerole
login_session = arn:aws:sts::194015964148:assumed-role/kazumainamura_assumerole/inamuu
duration_seconds = 3600
`
	if err := os.WriteFile(configPath, []byte(configBody), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.WriteFile(os.Getenv("AWS_SHARED_CREDENTIALS_FILE"), []byte(""), 0o600); err != nil {
		t.Fatalf("write credentials: %v", err)
	}

	plan, err := resolveLoginFallbackPlan("estack")
	if err != nil {
		t.Fatalf("resolveLoginFallbackPlan returned error: %v", err)
	}
	if got, want := plan.Region, "ap-northeast-1"; got != want {
		t.Fatalf("plan.Region = %q, want %q", got, want)
	}
	if got, want := plan.LoginSession, "arn:aws:iam::489667813116:user/inamuu"; got != want {
		t.Fatalf("plan.LoginSession = %q, want %q", got, want)
	}
	if got, want := plan.RoleARN, "arn:aws:iam::194015964148:role/kazumainamura_assumerole"; got != want {
		t.Fatalf("plan.RoleARN = %q, want %q", got, want)
	}
}

func TestLoadConfig_UsesDirectLoginSessionFallback(t *testing.T) {
	t.Setenv("AWS_PROFILE", "")
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_DEFAULT_REGION", "")
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")
	t.Setenv("AWS_SESSION_TOKEN", "")
	t.Setenv("AWS_CONFIG_FILE", filepath.Join(t.TempDir(), "config"))
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(t.TempDir(), "credentials"))

	cacheDir := t.TempDir()
	t.Setenv("AWS_LOGIN_CACHE_DIRECTORY", cacheDir)

	configPath := os.Getenv("AWS_CONFIG_FILE")
	configBody := `[profile estack]
region = ap-northeast-1
login_session = arn:aws:sts::194015964148:assumed-role/kazumainamura_assumerole/inamuu
duration_seconds = 3600
`
	if err := os.WriteFile(configPath, []byte(configBody), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.WriteFile(os.Getenv("AWS_SHARED_CREDENTIALS_FILE"), []byte(""), 0o600); err != nil {
		t.Fatalf("write credentials: %v", err)
	}

	tokenPath, err := logincreds.StandardCachedTokenFilepath("arn:aws:sts::194015964148:assumed-role/kazumainamura_assumerole/inamuu", cacheDir)
	if err != nil {
		t.Fatalf("resolve token path: %v", err)
	}
	tokenBody := `{
  "accessToken": {
    "accessKeyId": "ACCESS_KEY",
    "secretAccessKey": "SECRET_KEY",
    "sessionToken": "SESSION_TOKEN",
    "accountId": "194015964148",
    "expiresAt": "` + time.Now().Add(30*time.Minute).UTC().Format(time.RFC3339) + `"
  },
  "tokenType": "Bearer",
  "refreshToken": "REFRESH_TOKEN",
  "identityToken": "IDENTITY_TOKEN",
  "clientId": "CLIENT_ID",
  "dpopKey": "DPOP_KEY"
}`
	if err := os.WriteFile(tokenPath, []byte(tokenBody), 0o600); err != nil {
		t.Fatalf("write token cache: %v", err)
	}

	cfg, err := LoadConfig(context.Background(), "estack", "")
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}
	if got, want := cfg.Region, "ap-northeast-1"; got != want {
		t.Fatalf("cfg.Region = %q, want %q", got, want)
	}

	creds, err := cfg.Credentials.Retrieve(context.Background())
	if err != nil {
		t.Fatalf("Retrieve returned error: %v", err)
	}
	if got, want := creds.AccessKeyID, "ACCESS_KEY"; got != want {
		t.Fatalf("AccessKeyID = %q, want %q", got, want)
	}
	if got, want := creds.SecretAccessKey, "SECRET_KEY"; got != want {
		t.Fatalf("SecretAccessKey = %q, want %q", got, want)
	}
	if got, want := creds.SessionToken, "SESSION_TOKEN"; got != want {
		t.Fatalf("SessionToken = %q, want %q", got, want)
	}
}
