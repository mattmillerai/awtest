package awsv2

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

func creds(t *testing.T, cfg aws.Config) aws.Credentials {
	t.Helper()
	c, err := cfg.Credentials.Retrieve(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestLoadConfigStaticCreds(t *testing.T) {
	isolateEnv(t)
	tests := []struct {
		name, key, token, wantToken string
	}{
		{"plain static", "AKIAFAKE", "", ""},
		{"ASIA with token", "ASIAFAKE", "tok", "tok"},
		{"ASIA without token", "ASIAFAKE", "", ""},
		{"non-ASIA token ignored", "AKIAFAKE", "tok", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := LoadConfig(context.Background(), Options{AccessKeyID: tt.key, SecretAccessKey: "sec", SessionToken: tt.token})
			if err != nil {
				t.Fatal(err)
			}
			c := creds(t, cfg)
			if c.AccessKeyID != tt.key || c.SecretAccessKey != "sec" || c.SessionToken != tt.wantToken {
				t.Fatalf("got %+v", c)
			}
		})
	}
}

func TestLoadConfigIncompleteStaticFallsThrough(t *testing.T) {
	isolateEnv(t)
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIAENV")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "envsecret")
	cfg, err := LoadConfig(context.Background(), Options{AccessKeyID: "AKIAONLY"})
	if err != nil {
		t.Fatal(err)
	}
	if got := creds(t, cfg).AccessKeyID; got != "AKIAENV" {
		t.Fatalf("got %q", got)
	}
}

func TestLoadConfigStaticBeatsEnvAndIgnoresFiles(t *testing.T) {
	isolateEnv(t)
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIAENV")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "envsecret")
	t.Setenv("AWS_SESSION_TOKEN", "envtoken")
	// A profile that does not exist would be a hard error if files were read.
	t.Setenv("AWS_PROFILE", "does-not-exist")
	// A real file that must not be consulted.
	dir := t.TempDir()
	f := filepath.Join(dir, "creds")
	if err := os.WriteFile(f, []byte("[default]\naws_access_key_id=AKIAFILE\naws_secret_access_key=filesecret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cf := filepath.Join(dir, "config")
	if err := os.WriteFile(cf, []byte("[default]\nregion=ap-east-1\nendpoint_url=http://ambient-file.invalid\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", f)
	t.Setenv("AWS_CONFIG_FILE", cf)

	cfg, err := LoadConfig(context.Background(), Options{AccessKeyID: "AKIASTATIC", SecretAccessKey: "s", Profile: "other"})
	if err != nil {
		t.Fatalf("static path must not read shared files: %v", err)
	}
	c := creds(t, cfg)
	if c.AccessKeyID != "AKIASTATIC" || c.SessionToken != "" {
		t.Fatalf("got %+v", c)
	}
	if cfg.Region != "" || cfg.BaseEndpoint != nil {
		t.Fatalf("shared config leaked: region=%q endpoint=%v", cfg.Region, cfg.BaseEndpoint)
	}
}

// Sanity check that a missing profile really is an error on the non-static
// paths, proving the test above is meaningful.
func TestLoadConfigMissingProfileErrors(t *testing.T) {
	isolateEnv(t)
	if _, err := LoadConfig(context.Background(), Options{Profile: "does-not-exist"}); err == nil {
		t.Fatal("expected error for missing profile")
	}
	t.Setenv("AWS_PROFILE", "does-not-exist")
	if _, err := LoadConfig(context.Background(), Options{}); err == nil {
		t.Fatal("expected error for missing AWS_PROFILE")
	}
}

func TestLoadConfigProfile(t *testing.T) {
	isolateEnv(t)
	dir := t.TempDir()
	f := filepath.Join(dir, "credentials")
	if err := os.WriteFile(f, []byte("[myprof]\naws_access_key_id=AKIAPROF\naws_secret_access_key=profsecret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", f)
	t.Setenv("AWS_CONFIG_FILE", filepath.Join(dir, "config-missing"))

	cfg, err := LoadConfig(context.Background(), Options{Profile: "myprof", Region: "ap-south-1"})
	if err != nil {
		t.Fatal(err)
	}
	if got := creds(t, cfg).AccessKeyID; got != "AKIAPROF" {
		t.Fatalf("got %q", got)
	}
	if cfg.Region != "ap-south-1" {
		t.Fatalf("region = %q", cfg.Region)
	}
}

func TestLoadConfigDefaultChainFromEnv(t *testing.T) {
	isolateEnv(t)
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIAENV")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "envsecret")
	t.Setenv("AWS_SESSION_TOKEN", "envtoken")
	t.Setenv("AWS_REGION", "eu-north-1")
	cfg, err := LoadConfig(context.Background(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	c := creds(t, cfg)
	if c.AccessKeyID != "AKIAENV" || c.SessionToken != "envtoken" {
		t.Fatalf("got %+v", c)
	}
	if cfg.Region != "eu-north-1" {
		t.Fatalf("region = %q", cfg.Region)
	}
	// Explicit region wins over env.
	cfg, err = LoadConfig(context.Background(), Options{Region: "us-west-1"})
	if err != nil || cfg.Region != "us-west-1" {
		t.Fatalf("err=%v region=%q", err, cfg.Region)
	}
}

func TestLoadConfigEndpoint(t *testing.T) {
	isolateEnv(t)
	t.Setenv("AWS_ENDPOINT_URL", "http://ambient.invalid")
	cfg, err := LoadConfig(context.Background(), Options{AccessKeyID: "AKIAX", SecretAccessKey: "s"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BaseEndpoint != nil {
		t.Fatalf("ambient endpoint survived: %q", *cfg.BaseEndpoint)
	}

	// Also on the default chain path.
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIAENV")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "s")
	cfg, err = LoadConfig(context.Background(), Options{})
	if err != nil || cfg.BaseEndpoint != nil {
		t.Fatalf("err=%v be=%v", err, cfg.BaseEndpoint)
	}

	cfg, err = LoadConfig(context.Background(), Options{Endpoint: "http://localhost:4566"})
	if err != nil || aws.ToString(cfg.BaseEndpoint) != "http://localhost:4566" {
		t.Fatalf("err=%v be=%v", err, cfg.BaseEndpoint)
	}
}

func TestLoadConfigInvalidEndpoint(t *testing.T) {
	isolateEnv(t)
	for _, bad := range []string{"localhost:4566", "ftp://x", "http://"} {
		if _, err := LoadConfig(context.Background(), Options{AccessKeyID: "AKIAX", SecretAccessKey: "s", Endpoint: bad}); err == nil {
			t.Errorf("%q: expected error", bad)
		}
	}
}

// errTransport fails every request so the SDK retries and logs.
type errTransport struct{}

func (errTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return nil, http.ErrHandlerTimeout
}

func TestLoadConfigDebugLogging(t *testing.T) {
	isolateEnv(t)
	const secret, token = "SUPERSECRETVALUE", "SESSIONTOKENVALUE"
	run := func(debug, quiet bool) (aws.Config, string) {
		var buf bytes.Buffer
		cfg, err := LoadConfig(context.Background(), Options{
			AccessKeyID: "ASIAFAKEKEY", SecretAccessKey: secret, SessionToken: token,
			Debug: debug, Quiet: quiet, Stderr: &buf, HTTPClient: &http.Client{Transport: errTransport{}},
		})
		if err != nil {
			t.Fatal(err)
		}
		c := NewClient(cfg, ClientParams{Region: "us-east-1"}, func(c aws.Config, fns ...func(*sts.Options)) *sts.Client {
			return sts.NewFromConfig(c, append(fns, func(o *sts.Options) { o.RetryMaxAttempts = 2 })...)
		})
		_, _ = c.GetCallerIdentity(context.Background(), &sts.GetCallerIdentityInput{})
		return cfg, buf.String()
	}

	cfg, out := run(true, false)
	if cfg.ClientLogMode != aws.LogRetries {
		t.Fatalf("log mode = %v", cfg.ClientLogMode)
	}
	if out == "" {
		t.Fatal("expected retry logging")
	}
	for _, s := range []string{secret, token, "ASIAFAKEKEY", "Authorization", "AWS4-HMAC"} {
		if strings.Contains(out, s) {
			t.Fatalf("debug log leaked %q:\n%s", s, out)
		}
	}

	cfg, out = run(true, true)
	if cfg.ClientLogMode != 0 || out != "" {
		t.Fatalf("quiet must not log: mode=%v out=%q", cfg.ClientLogMode, out)
	}
	cfg, out = run(false, false)
	if cfg.ClientLogMode != 0 || out != "" {
		t.Fatalf("no debug must not log: mode=%v out=%q", cfg.ClientLogMode, out)
	}
}
