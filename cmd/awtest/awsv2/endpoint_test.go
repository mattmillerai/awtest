package awsv2

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

func TestApplyEndpoint(t *testing.T) {
	cfg := aws.Config{BaseEndpoint: aws.String("http://ambient.example")}
	if err := ApplyEndpoint(&cfg, ""); err != nil || cfg.BaseEndpoint != nil {
		t.Fatalf("empty endpoint should clear: err=%v be=%v", err, cfg.BaseEndpoint)
	}
	if err := ApplyEndpoint(&cfg, "http://localhost:4566"); err != nil || aws.ToString(cfg.BaseEndpoint) != "http://localhost:4566" {
		t.Fatalf("valid endpoint: err=%v be=%v", err, cfg.BaseEndpoint)
	}
	for _, bad := range []string{"localhost:4566", "ftp://host", "http://", "://x", "not a url", "/path"} {
		c := aws.Config{BaseEndpoint: aws.String("http://keep")}
		if err := ApplyEndpoint(&c, bad); err == nil {
			t.Errorf("%q: expected error", bad)
		}
		if aws.ToString(c.BaseEndpoint) != "http://keep" {
			t.Errorf("%q: cfg modified on error", bad)
		}
	}
}

func stsCtor(c aws.Config, fns ...func(*sts.Options)) *sts.Client {
	return sts.NewFromConfig(c, append(fns, func(o *sts.Options) { o.RetryMaxAttempts = 1 })...)
}

func newCfg(t *testing.T, ft *fakeTransport, region string) aws.Config {
	t.Helper()
	cfg, err := LoadConfig(context.Background(), Options{
		Region: region, AccessKeyID: "AKIAFAKE", SecretAccessKey: "fake", HTTPClient: ft.client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestNewClientExplicitEndpoint(t *testing.T) {
	isolateEnv(t)
	ft := &fakeTransport{}
	c := NewClient(newCfg(t, ft, "us-west-2"), ClientParams{Endpoint: "http://localhost:4566"}, stsCtor)
	if _, err := c.GetCallerIdentity(context.Background(), &sts.GetCallerIdentityInput{}); err != nil {
		t.Fatal(err)
	}
	if got := ft.reqs[0].URL.Host; got != "localhost:4566" {
		t.Fatalf("host = %q", got)
	}
}

func TestNewClientDefaultEndpointAndRegion(t *testing.T) {
	isolateEnv(t)
	ft := &fakeTransport{}
	c := NewClient(newCfg(t, ft, "us-west-2"), ClientParams{}, stsCtor)
	if _, err := c.GetCallerIdentity(context.Background(), &sts.GetCallerIdentityInput{}); err != nil {
		t.Fatal(err)
	}
	if got := ft.reqs[0].URL.Host; got != "sts.us-west-2.amazonaws.com" {
		t.Fatalf("host = %q", got)
	}

	// Region param overrides the config's region.
	ft2 := &fakeTransport{}
	c = NewClient(newCfg(t, ft2, "us-west-2"), ClientParams{Region: "eu-central-1"}, stsCtor)
	if _, err := c.GetCallerIdentity(context.Background(), &sts.GetCallerIdentityInput{}); err != nil {
		t.Fatal(err)
	}
	if got := ft2.reqs[0].URL.Host; got != "sts.eu-central-1.amazonaws.com" {
		t.Fatalf("host = %q", got)
	}
}

// Service-specific env endpoints are resolved inside NewFromConfig and must be
// defeated by NewClient.
func TestNewClientDefeatsAmbientServiceEndpoint(t *testing.T) {
	isolateEnv(t)
	t.Setenv("AWS_ENDPOINT_URL_STS", "http://ambient-sts.invalid")
	t.Setenv("AWS_ENDPOINT_URL", "http://ambient-global.invalid")

	ft := &fakeTransport{}
	cfg := newCfg(t, ft, "us-east-1")
	if cfg.BaseEndpoint != nil {
		t.Fatalf("cfg.BaseEndpoint = %v", *cfg.BaseEndpoint)
	}
	c := NewClient(cfg, ClientParams{}, stsCtor)
	if _, err := c.GetCallerIdentity(context.Background(), &sts.GetCallerIdentityInput{}); err != nil {
		t.Fatal(err)
	}
	if got := ft.reqs[0].URL.Host; got != "sts.us-east-1.amazonaws.com" {
		t.Fatalf("ambient endpoint leaked: host = %q", got)
	}

	ft = &fakeTransport{}
	c = NewClient(newCfg(t, ft, "us-east-1"), ClientParams{Endpoint: "http://explicit.example:9"}, stsCtor)
	if _, err := c.GetCallerIdentity(context.Background(), &sts.GetCallerIdentityInput{}); err != nil {
		t.Fatal(err)
	}
	if got := ft.reqs[0].URL.Host; got != "explicit.example:9" {
		t.Fatalf("host = %q", got)
	}
}

type fakeOpts struct {
	BaseEndpoint *string
	Region       string
	UsePathStyle bool
}

func TestApplyParams(t *testing.T) {
	o := fakeOpts{BaseEndpoint: aws.String("http://ambient"), Region: "keep"}
	applyParams(&o, ClientParams{})
	if o.BaseEndpoint != nil || o.Region != "keep" || o.UsePathStyle {
		t.Fatalf("no endpoint: %+v", o)
	}
	applyParams(&o, ClientParams{Region: "eu-west-1", Endpoint: "http://x:1"})
	if aws.ToString(o.BaseEndpoint) != "http://x:1" || o.Region != "eu-west-1" || !o.UsePathStyle {
		t.Fatalf("endpoint: %+v", o)
	}
}

func TestNewClientGenericCtor(t *testing.T) {
	var got fakeOpts
	ctor := func(_ aws.Config, fns ...func(*fakeOpts)) *fakeOpts {
		for _, f := range fns {
			f(&got)
		}
		return &got
	}
	NewClient(aws.Config{}, ClientParams{Endpoint: "http://s3.local"}, ctor)
	if !got.UsePathStyle || aws.ToString(got.BaseEndpoint) != "http://s3.local" {
		t.Fatalf("%+v", got)
	}
}

func TestApplyParamsPanicsWithoutBaseEndpoint(t *testing.T) {
	type noEndpoint struct{ Region string }
	type wrongType struct{ BaseEndpoint string }
	for name, opts := range map[string]any{"missing": &noEndpoint{}, "wrong type": &wrongType{}, "non-pointer": noEndpoint{}} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				r := recover()
				if r == nil {
					t.Fatal("expected panic")
				}
				if msg, _ := r.(string); !strings.Contains(msg, "awsv2") {
					t.Fatalf("unclear panic: %v", r)
				}
			}()
			applyParams(opts, ClientParams{Endpoint: "http://x"})
		})
	}
}
