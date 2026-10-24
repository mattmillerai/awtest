package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
)

// runBinaryEnv runs bin with args and a clean AWS environment plus extra vars.
func runBinaryEnv(t *testing.T, bin string, extraEnv []string, args ...string) (stderr string, code int) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	env := []string{}
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "AWS_") {
			continue
		}
		env = append(env, kv)
	}
	cmd.Env = append(env, extraEnv...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("running %v: %v", args, err)
		}
		code = ee.ExitCode()
	}
	return errb.String(), code
}

func TestEndpointFlagBinary(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	bin := buildAwtestBinary(t)

	t.Run("invalid scheme exits 2", func(t *testing.T) {
		errOut, code := runBinaryEnv(t, bin, nil, "-endpoint-url", "ftp://x", "-aki", "AKIAFAKE", "-sak", "fake")
		if code != 2 || !strings.Contains(errOut, "endpoint-url") {
			t.Errorf("exit=%d stderr=%q", code, errOut)
		}
	})

	t.Run("invalid env exits 2", func(t *testing.T) {
		errOut, code := runBinaryEnv(t, bin, []string{"AWS_ENDPOINT_URL=notaurl"}, "-aki", "AKIAFAKE", "-sak", "fake")
		if code != 2 || !strings.Contains(errOut, "AWS_ENDPOINT_URL") {
			t.Errorf("exit=%d stderr=%q", code, errOut)
		}
	})

	var hits int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&hits, 1)
		w.Header().Set("Content-Type", "application/x-amz-json-1.1")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"__type":"AccessDeniedException","message":"denied"}`))
	}))
	defer srv.Close()
	env := []string{"AWS_EC2_METADATA_DISABLED=true"}
	args := []string{"-endpoint-url", srv.URL, "-aki", "AKIAFAKE", "-sak", "fake", "-services", "sts,s3,sqs", "-timeout", "30s"}

	t.Run("end to end", func(t *testing.T) {
		errOut, _ := runBinaryEnv(t, bin, env, args...)
		if !strings.Contains(errOut, "Using custom endpoint") {
			t.Errorf("notice missing from stderr:\n%s", errOut)
		}
		if atomic.LoadInt64(&hits) == 0 {
			t.Error("test server received no requests")
		}
	})

	t.Run("quiet suppresses notice", func(t *testing.T) {
		before := atomic.LoadInt64(&hits)
		errOut, _ := runBinaryEnv(t, bin, env, append([]string{"-quiet"}, args...)...)
		if strings.Contains(errOut, "Using custom endpoint") {
			t.Errorf("notice printed with -quiet:\n%s", errOut)
		}
		if atomic.LoadInt64(&hits) == before {
			t.Error("test server received no requests with -quiet")
		}
	})

	t.Run("help mentions flag", func(t *testing.T) {
		errOut, _ := runBinaryEnv(t, bin, nil, "-h")
		if !strings.Contains(errOut, "endpoint-url") {
			t.Errorf("-h output missing endpoint-url:\n%s", errOut)
		}
	})
}
