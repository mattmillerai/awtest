package replay_test

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/mattmillerai/awtest/cmd/awtest/replay"
	"github.com/mattmillerai/awtest/cmd/awtest/services"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
)

// failTransport records any request that reaches http.DefaultTransport. The
// replay session must never use it.
type failTransport struct {
	mu   sync.Mutex
	seen []string
}

func (f *failTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	f.mu.Lock()
	f.seen = append(f.seen, req.URL.String())
	f.mu.Unlock()
	return nil, fmt.Errorf("replay no-bypass: unexpected request to %s", req.URL.Host)
}

func startSample(t *testing.T) *replay.Server {
	t.Helper()
	fx, err := replay.Load(replay.SampleFS())
	if err != nil {
		t.Fatal(err)
	}
	srv, err := replay.Start(fx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(srv.Close)
	return srv
}

func TestEveryServiceStaysOnReplayServer(t *testing.T) {
	guard := &failTransport{}
	orig := http.DefaultTransport
	http.DefaultTransport = guard
	t.Cleanup(func() { http.DefaultTransport = orig })

	srv := startSample(t)
	sess, err := replay.NewSession(srv, "us-west-2")
	if err != nil {
		t.Fatal(err)
	}
	cfg := newReplayConfig(t, srv)

	for _, svc := range services.AllServices() {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("panic in %s: %v", svc.Name, r)
				}
			}()
			ctx, cancel := context.WithTimeout(types.WithAWSConfig(context.Background(), cfg), 30*time.Second)
			defer cancel()
			out, err := svc.Invoke(ctx, sess)
			svc.Process(out, err, false)
		}()
	}

	if srv.Requests() == 0 {
		t.Error("replay server saw no requests")
	}
	guard.mu.Lock()
	defer guard.mu.Unlock()
	if len(guard.seen) > 0 {
		t.Errorf("%d request(s) used http.DefaultTransport: %v", len(guard.seen), guard.seen)
	}
}

func TestSampleProducesResults(t *testing.T) {
	srv := startSample(t)
	sess, err := replay.NewSession(srv, "us-west-2")
	if err != nil {
		t.Fatal(err)
	}
	cfg := newReplayConfig(t, srv)
	want := map[string]bool{
		"sts:GetCallerIdentity": true, "s3:ListBuckets": true, "s3:GetBucketPolicy": true,
		"iam:ListUsers": true, "iam:ListRoles": true, "iam:ListGroups": true, "iam:ListPolicies": true,
		"ec2:DescribeInstances": true,
	}
	found := map[string]bool{}
	for _, svc := range services.AllServices() {
		if !want[svc.Name] {
			continue
		}
		found[svc.Name] = true
		out, err := svc.Invoke(types.WithAWSConfig(context.Background(), cfg), sess)
		results := svc.Process(out, err, false)
		if len(results) == 0 {
			t.Errorf("%s: no results", svc.Name)
		}
		assertNoErrors(t, svc.Name, results)
	}
	for name := range want {
		if !found[name] {
			t.Errorf("service %s not registered", name)
		}
	}
}

func assertNoErrors(t *testing.T, name string, results []types.ScanResult) {
	t.Helper()
	for _, r := range results {
		if r.Error != nil {
			t.Errorf("%s: %s %s returned error: %v", name, r.MethodName, r.ResourceName, r.Error)
		}
	}
}
