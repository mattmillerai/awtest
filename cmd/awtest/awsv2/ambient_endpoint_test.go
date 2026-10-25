package awsv2

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

const (
	ambientHost  = "ambient.invalid"
	explicitHost = "explicit.invalid"
	testRegion   = "us-west-2"
)

// writeAWSFiles writes a shared config and credentials file under a temp dir
// and points AWS_CONFIG_FILE / AWS_SHARED_CREDENTIALS_FILE at them. configBody
// is the full config file content.
func writeAWSFiles(t *testing.T, configBody, credsBody string) {
	t.Helper()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config")
	credPath := filepath.Join(dir, "credentials")
	if err := os.WriteFile(cfgPath, []byte(configBody), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(credPath, []byte(credsBody), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AWS_CONFIG_FILE", cfgPath)
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", credPath)
}

func TestAmbientEndpointIgnored(t *testing.T) {
	type ambient struct {
		name string
		// env sets environment variables; fileCfg is extra config-file lines
		// for the profile section.
		env     map[string]string
		fileCfg string
		// svcSection is an extra top-level section.
		svcSection string
	}
	ambients := []ambient{
		{name: "AWS_ENDPOINT_URL", env: map[string]string{"AWS_ENDPOINT_URL": "https://" + ambientHost}},
		{name: "AWS_ENDPOINT_URL_STS", env: map[string]string{"AWS_ENDPOINT_URL_STS": "https://" + ambientHost}},
		{name: "both", env: map[string]string{
			"AWS_ENDPOINT_URL":     "https://" + ambientHost,
			"AWS_ENDPOINT_URL_STS": "https://" + ambientHost,
		}},
		{name: "shared-config endpoint_url", fileCfg: "endpoint_url = https://" + ambientHost + "\n"},
		{name: "shared-config services section", fileCfg: "services = x\n",
			svcSection: "[services x]\nsts =\n  endpoint_url = https://" + ambientHost + "\n"},
	}
	creds := []string{"static", "profile", "default-chain"}

	for _, cp := range creds {
		for _, am := range ambients {
			for _, explicit := range []string{"", "https://" + explicitHost} {
				mode := "no-explicit"
				if explicit != "" {
					mode = "explicit"
				}
				t.Run(cp+"/"+am.name+"/"+mode, func(t *testing.T) {
					isolateEnv(t)
					writeAWSFiles(t,
						"[default]\n"+am.fileCfg+"\n[profile p]\n"+am.fileCfg+"\n"+am.svcSection,
						"[p]\naws_access_key_id = AKIAPROFILEKEY\naws_secret_access_key = profilesecret\n")
					for k, v := range am.env {
						t.Setenv(k, v)
					}

					opts := Options{Region: testRegion, Endpoint: explicit}
					switch cp {
					case "static":
						opts.AccessKeyID = "AKIASTATICKEY"
						opts.SecretAccessKey = "staticsecret"
						t.Setenv("AWS_PROFILE", "p")
					case "profile":
						opts.Profile = "p"
					case "default-chain":
						t.Setenv("AWS_ACCESS_KEY_ID", "AKIAENVKEY")
						t.Setenv("AWS_SECRET_ACCESS_KEY", "envsecret")
					}

					ft := &fakeTransport{}
					opts.HTTPClient = ft.client()
					cfg, err := LoadConfig(context.Background(), opts)
					if err != nil {
						t.Fatalf("LoadConfig: %v", err)
					}
					if explicit == "" {
						if cfg.BaseEndpoint != nil {
							t.Fatalf("cfg.BaseEndpoint = %q, want nil", *cfg.BaseEndpoint)
						}
					} else if cfg.BaseEndpoint == nil || *cfg.BaseEndpoint != explicit {
						t.Fatalf("cfg.BaseEndpoint = %v, want %q", cfg.BaseEndpoint, explicit)
					}
					cfg.RetryMaxAttempts = 1

					client := NewClient(cfg, ClientParams{Endpoint: explicit}, sts.NewFromConfig)
					for i := 0; i < 2; i++ {
						if _, err := client.GetCallerIdentity(context.Background(), &sts.GetCallerIdentityInput{}); err != nil {
							t.Fatalf("GetCallerIdentity: %v", err)
						}
					}

					want := "sts." + testRegion + ".amazonaws.com"
					if explicit != "" {
						want = explicitHost
					}
					if len(ft.reqs) != 2 {
						t.Fatalf("recorded %d requests, want 2", len(ft.reqs))
					}
					for _, r := range ft.reqs {
						if r.URL.Host != want {
							t.Errorf("request host = %q, want %q", r.URL.Host, want)
						}
					}
				})
			}
		}
	}
}

// NewClient must not depend on the isolation LoadConfig leaves in
// cfg.ConfigSources: given a config from the SDK's plain loader it still sends
// to the real endpoint, or to the explicit one.
func TestNewClientDefeatsAmbientOnUnisolatedConfig(t *testing.T) {
	for _, explicit := range []string{"", "https://" + explicitHost} {
		t.Run("explicit="+explicit, func(t *testing.T) {
			isolateEnv(t)
			t.Setenv("AWS_ENDPOINT_URL_STS", "https://"+ambientHost)
			t.Setenv("AWS_ACCESS_KEY_ID", "AKIAENVKEY")
			t.Setenv("AWS_SECRET_ACCESS_KEY", "envsecret")
			ft := &fakeTransport{}
			cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(testRegion), config.WithHTTPClient(ft.client()))
			if err != nil {
				t.Fatal(err)
			}
			client := NewClient(cfg, ClientParams{Endpoint: explicit}, sts.NewFromConfig)
			if _, err := client.GetCallerIdentity(context.Background(), &sts.GetCallerIdentityInput{}); err != nil {
				t.Fatal(err)
			}
			want := "sts." + testRegion + ".amazonaws.com"
			if explicit != "" {
				want = explicitHost
			}
			if len(ft.reqs) != 1 || ft.reqs[0].URL.Host != want {
				t.Fatalf("requests = %v, want one to %s", ft.reqs, want)
			}
		})
	}
}

// Control: a client built without the helper from a config loaded by the SDK's
// own loader is redirected by the ambient service-specific variable, which
// proves the matrix above would catch a bypass. (A config from LoadConfig
// additionally records "ignore configured endpoints" in its config sources, so
// plain clients built from it are not redirected either.)
func TestAmbientControlPlainClientIsRedirected(t *testing.T) {
	isolateEnv(t)
	t.Setenv("AWS_ENDPOINT_URL_STS", "https://"+ambientHost)
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIAENVKEY")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "envsecret")

	ft := &fakeTransport{}
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(testRegion), config.WithHTTPClient(ft.client()))
	if err != nil {
		t.Fatal(err)
	}
	cfg.RetryMaxAttempts = 1

	client := sts.NewFromConfig(cfg)
	if _, err := client.GetCallerIdentity(context.Background(), &sts.GetCallerIdentityInput{}); err != nil {
		t.Fatal(err)
	}
	if len(ft.reqs) != 1 || ft.reqs[0].URL.Host != ambientHost {
		t.Fatalf("plain client did not honour ambient endpoint; requests: %v", ft.reqs)
	}
}

// Control: the SDK's own loader picks up AWS_ENDPOINT_URL, so LoadConfig's
// clearing of BaseEndpoint is doing real work.
func TestAmbientControlDefaultLoaderSetsBaseEndpoint(t *testing.T) {
	isolateEnv(t)
	t.Setenv("AWS_ENDPOINT_URL", "https://"+ambientHost)

	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(testRegion))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BaseEndpoint == nil || *cfg.BaseEndpoint != "https://"+ambientHost {
		t.Fatalf("cfg.BaseEndpoint = %v, want https://%s", cfg.BaseEndpoint, ambientHost)
	}
}
