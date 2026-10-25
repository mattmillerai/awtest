package awsv2

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/config"
)

const assumeRoleXML = `<AssumeRoleResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/">
<AssumeRoleResult>
<AssumedRoleUser><AssumedRoleId>AROATEST:sess</AssumedRoleId><Arn>arn:aws:sts::123456789012:assumed-role/r/sess</Arn></AssumedRoleUser>
<Credentials><AccessKeyId>ASIAFAKEKEY</AccessKeyId><SecretAccessKey>fakesecret</SecretAccessKey><SessionToken>faketoken</SessionToken><Expiration>2099-01-01T00:00:00Z</Expiration></Credentials>
</AssumeRoleResult>
<ResponseMetadata><RequestId>r-2</RequestId></ResponseMetadata>
</AssumeRoleResponse>`

const assumeRoleWebIdentityXML = `<AssumeRoleWithWebIdentityResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/">
<AssumeRoleWithWebIdentityResult>
<AssumedRoleUser><AssumedRoleId>AROATEST:sess</AssumedRoleId><Arn>arn:aws:sts::123456789012:assumed-role/r/sess</Arn></AssumedRoleUser>
<Credentials><AccessKeyId>ASIAFAKEKEY</AccessKeyId><SecretAccessKey>fakesecret</SecretAccessKey><SessionToken>faketoken</SessionToken><Expiration>2099-01-01T00:00:00Z</Expiration></Credentials>
</AssumeRoleWithWebIdentityResult>
<ResponseMetadata><RequestId>r-3</RequestId></ResponseMetadata>
</AssumeRoleWithWebIdentityResponse>`

// stsFake serves AssumeRole and AssumeRoleWithWebIdentity responses and
// records the host and action of every request.
type stsFake struct {
	mu      sync.Mutex
	hosts   []string
	actions []string
}

func (f *stsFake) RoundTrip(req *http.Request) (*http.Response, error) {
	body := ""
	if req.Body != nil {
		b, _ := io.ReadAll(req.Body)
		body = string(b)
	}
	f.mu.Lock()
	f.hosts = append(f.hosts, req.URL.Host)
	f.mu.Unlock()
	resp := assumeRoleXML
	action := "AssumeRole"
	if strings.Contains(body, "Action=AssumeRoleWithWebIdentity") {
		resp = assumeRoleWebIdentityXML
		action = "AssumeRoleWithWebIdentity"
	}
	f.mu.Lock()
	f.actions = append(f.actions, action)
	f.mu.Unlock()
	return &http.Response{
		StatusCode: 200,
		Status:     "200 OK",
		Header:     http.Header{"Content-Type": []string{"text/xml"}},
		Body:       io.NopCloser(strings.NewReader(resp)),
		Request:    req,
	}, nil
}

func (f *stsFake) client() *http.Client { return &http.Client{Transport: f} }

// roleFiles writes shared files with a static-key source profile and a role
// profile; extra is appended to the default, src and role profile sections
// and svc is an additional top-level section.
func roleFiles(t *testing.T, extra, svc string) {
	t.Helper()
	writeAWSFiles(t,
		"[default]\nregion = "+testRegion+"\n"+extra+"\n"+
			"[profile src]\nregion = "+testRegion+"\n"+extra+"\n"+
			"[profile role]\nrole_arn = arn:aws:iam::123456789012:role/r\nsource_profile = src\nregion = "+testRegion+"\n"+extra+"\n"+svc,
		"[src]\naws_access_key_id = AKIASOURCEKEY\naws_secret_access_key = sourcesecret\n")
}

func webIdentityEnv(t *testing.T) {
	t.Helper()
	tok := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(tok, []byte("web-identity-token"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AWS_ROLE_ARN", "arn:aws:iam::123456789012:role/wi")
	t.Setenv("AWS_WEB_IDENTITY_TOKEN_FILE", tok)
}

func TestCredentialChainEndpointIsolation(t *testing.T) {
	type ambient struct {
		name       string
		env        map[string]string
		fileCfg    string // lines for each profile section
		svcSection string // extra top-level section
	}
	amb := "https://" + ambientHost
	ambients := []ambient{
		{name: "AWS_ENDPOINT_URL", env: map[string]string{"AWS_ENDPOINT_URL": amb}},
		{name: "AWS_ENDPOINT_URL_STS", env: map[string]string{"AWS_ENDPOINT_URL_STS": amb}},
		{name: "both", env: map[string]string{"AWS_ENDPOINT_URL": amb, "AWS_ENDPOINT_URL_STS": amb}},
		{name: "shared-config endpoint_url", fileCfg: "endpoint_url = " + amb + "\n"},
		{name: "shared-config services sts", fileCfg: "services = x\n",
			svcSection: "[services x]\nsts =\n  endpoint_url = " + amb + "\n"},
	}
	setups := []struct {
		name  string
		prep  func(t *testing.T) Options
		wantA string
	}{
		{"role-profile", func(t *testing.T) Options { return Options{Profile: "role"} }, "AssumeRole"},
		{"default-chain-AWS_PROFILE", func(t *testing.T) Options {
			t.Setenv("AWS_PROFILE", "role")
			return Options{}
		}, "AssumeRole"},
		{"web-identity", func(t *testing.T) Options {
			webIdentityEnv(t)
			return Options{}
		}, "AssumeRoleWithWebIdentity"},
	}

	for _, su := range setups {
		for _, am := range ambients {
			for _, explicit := range []string{"", "https://" + explicitHost} {
				mode := "no-explicit"
				if explicit != "" {
					mode = "explicit"
				}
				t.Run(su.name+"/"+am.name+"/"+mode, func(t *testing.T) {
					isolateEnv(t)
					roleFiles(t, am.fileCfg, am.svcSection)
					for k, v := range am.env {
						t.Setenv(k, v)
					}
					opts := su.prep(t)
					opts.Region = testRegion
					opts.Endpoint = explicit
					fake := &stsFake{}
					opts.HTTPClient = fake.client()

					cfg, err := LoadConfig(context.Background(), opts)
					if err != nil {
						t.Fatalf("LoadConfig: %v", err)
					}
					c, err := cfg.Credentials.Retrieve(context.Background())
					if err != nil {
						t.Fatalf("Retrieve: %v", err)
					}
					if c.AccessKeyID != "ASIAFAKEKEY" || c.SecretAccessKey != "fakesecret" || c.SessionToken != "faketoken" {
						t.Fatalf("credentials = %+v, want the fake assumed ones", c)
					}

					want := "sts." + testRegion + ".amazonaws.com"
					if explicit != "" {
						want = explicitHost
					}
					fake.mu.Lock()
					defer fake.mu.Unlock()
					if len(fake.hosts) == 0 {
						t.Fatal("no STS request recorded; test is vacuous")
					}
					for i, h := range fake.hosts {
						if h != want {
							t.Errorf("request %d (%s) host = %q, want %q", i, fake.actions[i], h, want)
						}
						if h == ambientHost {
							t.Errorf("request %d was sent to the ambient host", i)
						}
					}
					if fake.actions[0] != su.wantA {
						t.Errorf("first action = %s, want %s", fake.actions[0], su.wantA)
					}
				})
			}
		}
	}
}

// Control: without the mitigation the SDK's own loader sends AssumeRole to
// the ambient host, proving the matrix above can fail.
func TestCredentialChainControlUnmitigatedLoaderIsRedirected(t *testing.T) {
	isolateEnv(t)
	roleFiles(t, "", "")
	t.Setenv("AWS_ENDPOINT_URL_STS", "https://"+ambientHost)

	fake := &stsFake{}
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(testRegion),
		config.WithSharedConfigProfile("role"),
		config.WithHTTPClient(fake.client()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cfg.Credentials.Retrieve(context.Background()); err != nil {
		t.Fatal(err)
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.hosts) == 0 || fake.hosts[0] != ambientHost {
		t.Fatalf("unmitigated loader did not honour ambient endpoint; hosts: %v", fake.hosts)
	}
}

func TestLoadConfigRestoresIgnoreEndpointsEnv(t *testing.T) {
	const key = "AWS_IGNORE_CONFIGURED_ENDPOINT_URLS"
	cases := []struct {
		name    string
		prepare func(t *testing.T)
		set     bool
		value   string
	}{
		{"unset", func(t *testing.T) {
			t.Setenv(key, "")
			os.Unsetenv(key)
		}, false, ""},
		{"false", func(t *testing.T) { t.Setenv(key, "false") }, true, "false"},
		{"true", func(t *testing.T) { t.Setenv(key, "true") }, true, "true"},
	}
	paths := []struct {
		name string
		opts Options
		fail bool
	}{
		{"success", Options{AccessKeyID: "AKIAX", SecretAccessKey: "s"}, false},
		{"bad profile", Options{Profile: "does-not-exist"}, true},
		{"bad endpoint", Options{Endpoint: "ftp://x"}, true},
	}
	for _, c := range cases {
		for _, p := range paths {
			t.Run(c.name+"/"+p.name, func(t *testing.T) {
				isolateEnv(t)
				c.prepare(t)
				_, err := LoadConfig(context.Background(), p.opts)
				if (err != nil) != p.fail {
					t.Fatalf("err = %v, want failure=%v", err, p.fail)
				}
				v, ok := os.LookupEnv(key)
				if ok != c.set || v != c.value {
					t.Fatalf("after LoadConfig %s = %q (set=%v), want %q (set=%v)", key, v, ok, c.value, c.set)
				}
			})
		}
	}
}

// The variable must be held during the load, otherwise the mitigation is a
// no-op; observe it from a custom HTTP client factory is not possible, so use
// the loader option hook instead.
func TestLoadDefaultConfigIsolatedSetsVariableDuringLoad(t *testing.T) {
	isolateEnv(t)
	os.Unsetenv("AWS_IGNORE_CONFIGURED_ENDPOINT_URLS")
	var seen string
	_, err := loadDefaultConfigIsolated(context.Background(), []func(*config.LoadOptions) error{
		func(*config.LoadOptions) error {
			seen = os.Getenv("AWS_IGNORE_CONFIGURED_ENDPOINT_URLS")
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if seen != "true" {
		t.Fatalf("variable during load = %q, want true", seen)
	}
}
