package services

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

var (
	forbiddenSessionFuncs = map[string]bool{
		"New":                   true,
		"NewSession":            true,
		"NewSessionWithOptions": true,
		"Must":                  true,
	}
	forbiddenConfigKeys = map[string]bool{
		"Endpoint":         true,
		"EndpointResolver": true,
		"S3ForcePathStyle": true,
		"S3UseARNRegion":   true,
		"HTTPClient":       true,
	}
)

// scanForSessionBypass parses src and returns a description of every place it
// constructs a session or sets an aws.Config key owned by the central
// endpoint helper (utils.ApplyEndpoint).
func scanForSessionBypass(filename string, src []byte) ([]string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, src, 0)
	if err != nil {
		return nil, err
	}
	var found []string
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.CallExpr:
			if sel, ok := x.Fun.(*ast.SelectorExpr); ok {
				if id, ok := sel.X.(*ast.Ident); ok && id.Name == "session" && forbiddenSessionFuncs[sel.Sel.Name] {
					found = append(found, fmt.Sprintf("%s: call to session.%s", fset.Position(x.Pos()), sel.Sel.Name))
				}
			}
		case *ast.CompositeLit:
			sel, ok := x.Type.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if id, ok := sel.X.(*ast.Ident); !ok || id.Name != "aws" || sel.Sel.Name != "Config" {
				return true
			}
			for _, elt := range x.Elts {
				kv, ok := elt.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				if key, ok := kv.Key.(*ast.Ident); ok && forbiddenConfigKeys[key.Name] {
					found = append(found, fmt.Sprintf("%s: aws.Config sets %s", fset.Position(kv.Pos()), key.Name))
				}
			}
		}
		return true
	})
	return found, nil
}

func TestNoServiceBuildsItsOwnSession(t *testing.T) {
	var violations []string
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		found, err := scanForSessionBypass(path, src)
		if err != nil {
			return err
		}
		violations = append(violations, found...)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) > 0 {
		t.Errorf("services must not construct sessions or set endpoint/HTTP settings themselves; "+
			"derive per-region sessions with sess.Copy(&aws.Config{Region: aws.String(region)}) so that "+
			"-endpoint-url, credentials and the HTTP client are inherited:\n  %s", strings.Join(violations, "\n  "))
	}
}

func TestSessionBypassScannerDetects(t *testing.T) {
	src := []byte(`package x

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
)

func f() {
	_, _ = session.NewSession(&aws.Config{Region: aws.String("us-east-1")})
	_ = session.Must(session.NewSession())
	_ = &aws.Config{Endpoint: aws.String("http://x"), S3ForcePathStyle: aws.Bool(true)}
}
`)
	found, err := scanForSessionBypass("x.go", src)
	if err != nil {
		t.Fatal(err)
	}
	// NewSession x2, Must, Endpoint, S3ForcePathStyle
	if len(found) != 5 {
		t.Errorf("expected 5 detections, got %d: %v", len(found), found)
	}

	clean := []byte(`package x

func f(sess *session.Session) {
	_ = sess.Copy(&aws.Config{Region: aws.String("us-east-1")})
}
`)
	found, err = scanForSessionBypass("y.go", clean)
	if err != nil || len(found) != 0 {
		t.Errorf("clean source flagged: %v, %v", found, err)
	}
}

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

func fakeStaticSession(t *testing.T) *session.Session {
	t.Helper()
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String("us-west-2"),
		Credentials: credentials.NewStaticCredentials("AKIAFAKEFAKEFAKE", "fake", ""),
		MaxRetries:  aws.Int(0),
	})
	if err != nil {
		t.Fatal(err)
	}
	return sess
}

func TestAllServicesHonorCustomEndpoint(t *testing.T) {
	guard, srv, counter := setupGuard(t)

	// Every target gets both a v1 session and a v2 config built the guarded
	// way (explicit test HTTP client, ambient AWS_ENDPOINT_URL* set); Invoke
	// uses whichever SDK the service implements.
	sess := fakeStaticSession(t)
	utils.ApplyEndpoint(sess, srv.URL)
	targets := guardTargets()
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
			_, _ = svc.Invoke(ctx, sess)
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

	t.Run("control: bypassing client is detected", func(t *testing.T) {
		g2 := &guardTransport{allowedHost: guard.allowedHost, delegate: guard.delegate}
		http.DefaultTransport = g2
		bypass := s3.New(session.Must(session.NewSession(&aws.Config{
			Region:      aws.String("us-west-2"),
			Credentials: credentials.NewStaticCredentials("AKIAFAKEFAKEFAKE", "fake", ""),
			MaxRetries:  aws.Int(0),
		})))
		_, _ = bypass.ListBuckets(&s3.ListBucketsInput{})
		if len(g2.snapshot()) == 0 {
			t.Error("bypassing client was not recorded as a violation")
		}
	})

	// Negative control for v2: a service that builds its client with a plain
	// NewFromConfig lets the ambient AWS_ENDPOINT_URL_STS win over the
	// configured endpoint; the same harness must record that.
	t.Run("control: unguarded v2 client is detected", func(t *testing.T) {
		g2 := &guardTransport{allowedHost: guard.allowedHost, delegate: guard.delegate}
		cfg2 := guardedV2Config(t, g2, srv.URL)
		ctx, cancel := context.WithTimeout(types.WithAWSConfig(context.Background(), cfg2), 15*time.Second)
		defer cancel()
		bad := unguardedSyntheticV2Service()
		_, _ = bad.Invoke(ctx, sess)
		v := g2.snapshot()
		if len(v) == 0 {
			t.Fatal("unguarded v2 client was not recorded as an endpoint bypass")
		}
		if !strings.Contains(v[0], "ambient-endpoint.invalid") {
			t.Errorf("violation does not point at the ambient endpoint: %v", v)
		}
	})
}
