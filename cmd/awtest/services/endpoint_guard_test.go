package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
)

// The static halves of this guard (no client built without
// awsv2.NewClient/NewRegionalClient, no endpoint settings set directly, no
// aws-sdk-go v1 import anywhere) live in cmd/awtest/awsv2/guard_test.go
// (TestNoV2ClientBypass, TestNoV1SDKImports). This file is the dynamic half:
// it runs every service in AllServices through Invoke and proves that every
// request goes to the custom endpoint.

// guardTransport lets traffic to the local test server through and records
// (without touching the network) anything addressed elsewhere.
type guardTransport struct {
	allowedHost string
	delegate    http.RoundTripper

	mu         sync.Mutex
	current    string
	violations []string
}

func (g *guardTransport) setCurrent(name string) {
	g.mu.Lock()
	g.current = name
	g.mu.Unlock()
}

func (g *guardTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Host == g.allowedHost {
		return g.delegate.RoundTrip(req)
	}
	g.mu.Lock()
	g.violations = append(g.violations, fmt.Sprintf("%s (during %s)", req.URL.Host, g.current))
	g.mu.Unlock()
	return nil, errors.New("endpoint guard: request to non-test host blocked: " + req.URL.Host)
}

func (g *guardTransport) snapshot() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]string(nil), g.violations...)
}

func setupGuard(t *testing.T) (*guardTransport, *httptest.Server, *int64Counter) {
	t.Helper()
	counter := &int64Counter{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		counter.inc()
		w.Header().Set("Content-Type", "application/x-amz-json-1.1")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"__type":"AccessDeniedException","message":"denied"}`))
	}))
	t.Cleanup(srv.Close)

	g := &guardTransport{allowedHost: strings.TrimPrefix(srv.URL, "http://"), delegate: http.DefaultTransport}
	orig := http.DefaultTransport
	http.DefaultTransport = g
	t.Cleanup(func() { http.DefaultTransport = orig })

	t.Setenv("AWS_ACCESS_KEY_ID", "AKIAFAKEFAKEFAKE")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "fake")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	t.Setenv("AWS_REGION", "us-west-2")
	// t.Setenv registers the restore; then unset for the test's duration.
	for _, k := range []string{"AWS_PROFILE", "AWS_ENDPOINT_URL"} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}
	return g, srv, counter
}

type int64Counter struct {
	mu sync.Mutex
	n  int
}

func (c *int64Counter) inc() { c.mu.Lock(); c.n++; c.mu.Unlock() }
func (c *int64Counter) get() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}

func TestAllServicesImplementCallV2(t *testing.T) {
	all := AllServices()
	if len(all) == 0 {
		t.Fatal("AllServices is empty")
	}
	for _, svc := range all {
		if svc.CallV2 == nil {
			t.Errorf("service %q has no CallV2", svc.Name)
		}
		if svc.Process == nil {
			t.Errorf("service %q has no Process", svc.Name)
		}
	}
}

func TestAllServicesHonorCustomEndpoint(t *testing.T) {
	guard, srv, counter := setupGuard(t)

	// Every target gets a v2 config built the guarded way (explicit test HTTP
	// client, ambient AWS_ENDPOINT_URL* set to an unreachable host).
	targets := guardTargets(t)
	setAmbientEndpoints(t, targets)
	cfg := guardedV2Config(t, guard, srv.URL)

	var noRequest []string
	reached := 0

	for _, tg := range targets {
		svc := tg.svc
		guard.setCurrent(svc.Name)
		before := counter.get()
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("panic in %s: %v", svc.Name, r)
				}
			}()
			ctx, cancel := context.WithTimeout(types.WithAWSConfig(context.Background(), cfg), 15*time.Second)
			defer cancel()
			_, _ = svc.Invoke(ctx)
		}()
		if counter.get() > before {
			reached++
		} else {
			noRequest = append(noRequest, svc.Name)
		}
	}
	guard.setCurrent("")

	if v := guard.snapshot(); len(v) > 0 {
		sort.Strings(v)
		t.Errorf("%d request(s) bypassed the custom endpoint and would have hit real AWS:\n  %s", len(v), strings.Join(v, "\n  "))
	}
	if min := len(targets) * 9 / 10; reached < min {
		t.Errorf("only %d of %d calls reached the test server (want >= %d); did not: %v", reached, len(targets), min, noRequest)
	} else if len(noRequest) > 0 {
		t.Logf("calls that made no request: %v", noRequest)
	}
	for _, name := range noRequest {
		if name == syntheticV2Service().Name {
			t.Errorf("synthetic v2 service made no request to the test server")
		}
	}

	// Negative controls: the same harness must record a service that builds
	// its client with a plain NewFromConfig (the ambient
	// AWS_ENDPOINT_URL_STS wins over the configured endpoint) or one that
	// points its client at a foreign endpoint.
	t.Run("control: unguarded v2 client is detected", func(t *testing.T) {
		g2 := &guardTransport{allowedHost: guard.allowedHost, delegate: guard.delegate}
		cfg2 := guardedV2Config(t, g2, srv.URL)
		ctx, cancel := context.WithTimeout(types.WithAWSConfig(context.Background(), cfg2), 15*time.Second)
		defer cancel()
		bad := unguardedSyntheticV2Service()
		_, _ = bad.Invoke(ctx)
		v := g2.snapshot()
		if len(v) == 0 {
			t.Fatal("unguarded v2 client was not recorded as an endpoint bypass")
		}
		if !strings.Contains(v[0], "ambient-endpoint.invalid") {
			t.Errorf("violation does not point at the ambient endpoint: %v", v)
		}
	})

	t.Run("control: foreign endpoint is detected", func(t *testing.T) {
		g2 := &guardTransport{allowedHost: guard.allowedHost, delegate: guard.delegate}
		cfg2 := guardedV2Config(t, g2, srv.URL)
		ctx, cancel := context.WithTimeout(types.WithAWSConfig(context.Background(), cfg2), 15*time.Second)
		defer cancel()
		foreign := types.AWSService{
			Name: "sts:ForeignEndpointSyntheticV2",
			CallV2: syntheticV2Call(func(cfg aws.Config, region string) *sts.Client {
				cfg.Region = region
				return sts.NewFromConfig(cfg, func(o *sts.Options) { o.BaseEndpoint = aws.String("http://real-aws.invalid") })
			}),
			Process: func(interface{}, error, bool) []types.ScanResult { return nil },
		}
		_, _ = foreign.Invoke(ctx)
		v := g2.snapshot()
		if len(v) == 0 || !strings.Contains(v[0], "real-aws.invalid") {
			t.Fatalf("foreign endpoint was not recorded as a violation: %v", v)
		}
	})
}
