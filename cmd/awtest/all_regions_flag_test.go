package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/MillerMedia/awtest/cmd/awtest/types"
)

// signingRegions collects the region of each SigV4 Credential scope seen for
// one service (Credential=<akid>/<date>/<region>/<service>/aws4_request).
type signingRegions struct {
	mu      sync.Mutex
	service string
	regions map[string]int
}

func (s *signingRegions) observe(r *http.Request) {
	parts := strings.Split(r.Header.Get("Authorization"), "Credential=")
	if len(parts) < 2 {
		return
	}
	scope := strings.Split(strings.SplitN(parts[1], ",", 2)[0], "/")
	if len(scope) != 5 || scope[3] != s.service {
		return
	}
	s.mu.Lock()
	s.regions[scope[2]]++
	s.mu.Unlock()
}

func (s *signingRegions) reset() {
	s.mu.Lock()
	s.regions = map[string]int{}
	s.mu.Unlock()
}

func (s *signingRegions) snapshot() map[string]int {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]int, len(s.regions))
	for k, v := range s.regions {
		out[k] = v
	}
	return out
}

func TestAllRegionsFlagBinary(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	bin := buildAwtestBinary(t)

	seen := &signingRegions{service: "sns", regions: map[string]int{}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen.observe(r)
		w.Header().Set("Content-Type", "application/x-amz-json-1.1")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"__type":"AccessDeniedException","message":"denied"}`))
	}))
	defer srv.Close()
	env := []string{"AWS_EC2_METADATA_DISABLED=true"}
	base := []string{"-endpoint-url", srv.URL, "-aki", "AKIAFAKE", "-sak", "fake", "-services", "sns", "-timeout", "60s"}

	t.Run("all regions", func(t *testing.T) {
		seen.reset()
		errOut, _ := runBinaryEnv(t, bin, env, append([]string{"-all-regions"}, base...)...)
		for _, want := range []string{"All-regions mode: scanning 17 regions", "built-in list"} {
			if !strings.Contains(errOut, want) {
				t.Errorf("stderr missing %q:\n%s", want, errOut)
			}
		}
		got := seen.snapshot()
		if len(got) < 2 {
			t.Errorf("expected requests signed for multiple regions, got %v", got)
		}
		if got["us-west-2"] == 0 {
			t.Errorf("primary region us-west-2 not scanned: %v", got)
		}
		if !strings.Contains(errOut, "region-level call(s) failed") {
			t.Errorf("failure summary missing:\n%s", errOut)
		}
	})

	t.Run("quiet suppresses notice", func(t *testing.T) {
		seen.reset()
		errOut, _ := runBinaryEnv(t, bin, env, append([]string{"-quiet", "-all-regions"}, base...)...)
		if strings.Contains(errOut, "All-regions") {
			t.Errorf("all-regions output with -quiet:\n%s", errOut)
		}
		if len(seen.snapshot()) < 2 {
			t.Errorf("-quiet should still scan all regions, got %v", seen.snapshot())
		}
	})

	t.Run("default unchanged without flag", func(t *testing.T) {
		seen.reset()
		errOut, _ := runBinaryEnv(t, bin, env, base...)
		if strings.Contains(errOut, "All-regions") {
			t.Errorf("all-regions output without flag:\n%s", errOut)
		}
		allowed := map[string]bool{}
		for _, r := range types.Regions {
			allowed[r] = true
		}
		got := seen.snapshot()
		if len(got) == 0 {
			t.Fatal("no sns requests observed")
		}
		for r := range got {
			if !allowed[r] {
				t.Errorf("request signed for non-default region %q", r)
			}
		}
	})

	t.Run("help mentions flag", func(t *testing.T) {
		errOut, _ := runBinaryEnv(t, bin, nil, "-h")
		if !strings.Contains(errOut, "all-regions") {
			t.Errorf("-h output missing all-regions:\n%s", errOut)
		}
	})
}
