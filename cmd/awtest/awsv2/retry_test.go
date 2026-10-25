package awsv2

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go"
)

// zeroDelay makes retries instantaneous.
var zeroDelay = retry.BackoffDelayerFunc(func(int, error) (time.Duration, error) { return 0, nil })

func stsErrorXML(code string) string {
	return `<ErrorResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/"><Error><Type>Sender</Type><Code>` + code +
		`</Code><Message>fake ` + code + `</Message></Error><RequestId>req-1</RequestId></ErrorResponse>`
}

// countingTransport answers every request with a canned failure and counts
// the requests.
type countingTransport struct {
	mu     sync.Mutex
	n      int
	status int
	body   string
	err    error
}

func (c *countingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	c.mu.Lock()
	c.n++
	c.mu.Unlock()
	if req.Body != nil {
		io.Copy(io.Discard, req.Body)
	}
	if c.err != nil {
		return nil, c.err
	}
	return &http.Response{
		StatusCode: c.status,
		Status:     http.StatusText(c.status),
		Header:     http.Header{"Content-Type": []string{"text/xml"}},
		Body:       io.NopCloser(strings.NewReader(c.body)),
		Request:    req,
	}, nil
}

func (c *countingTransport) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}

func callSTS(t *testing.T, opts Options, ct *countingTransport) error {
	t.Helper()
	isolateEnv(t)
	opts.Region = testRegion
	opts.AccessKeyID, opts.SecretAccessKey = "AKIAFAKE", "fake"
	opts.HTTPClient = &http.Client{Transport: ct}
	if opts.RetryBackoff == nil {
		opts.RetryBackoff = zeroDelay
	}
	cfg, err := LoadConfig(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	client := NewRegionalClient(cfg, testRegion, sts.NewFromConfig)
	_, err = client.GetCallerIdentity(context.Background(), &sts.GetCallerIdentityInput{})
	if err == nil {
		t.Fatal("expected an error")
	}
	return err
}

func TestRetryPolicyRequestCounts(t *testing.T) {
	connReset := &net.OpError{Op: "read", Net: "tcp", Err: os.NewSyscallError("read", syscall.ECONNRESET)}
	cases := []struct {
		name     string
		opts     Options
		ct       func() *countingTransport
		want     int
		wantCode string
	}{
		{"Throttling is left to awtest", Options{}, func() *countingTransport {
			return &countingTransport{status: 400, body: stsErrorXML("Throttling")}
		}, 1, "Throttling"},
		{"RequestLimitExceeded is left to awtest", Options{}, func() *countingTransport {
			return &countingTransport{status: 503, body: stsErrorXML("RequestLimitExceeded")}
		}, 1, "RequestLimitExceeded"},
		{"TooManyRequestsException is left to awtest", Options{}, func() *countingTransport {
			return &countingTransport{status: 429, body: stsErrorXML("TooManyRequestsException")}
		}, 1, "TooManyRequestsException"},
		{"ThrottlingException is retried by the SDK", Options{}, func() *countingTransport {
			return &countingTransport{status: 400, body: stsErrorXML("ThrottlingException")}
		}, 4, "ThrottlingException"},
		{"HTTP 503 ServiceUnavailable", Options{}, func() *countingTransport {
			return &countingTransport{status: 503, body: stsErrorXML("ServiceUnavailable")}
		}, 4, "ServiceUnavailable"},
		{"connection reset", Options{}, func() *countingTransport {
			return &countingTransport{err: connReset}
		}, 4, ""},
		{"non-retryable client error", Options{}, func() *countingTransport {
			return &countingTransport{status: 403, body: stsErrorXML("AccessDenied")}
		}, 1, "AccessDenied"},
		{"MaxAttempts=1", Options{MaxAttempts: 1}, func() *countingTransport {
			return &countingTransport{status: 503, body: stsErrorXML("ServiceUnavailable")}
		}, 1, "ServiceUnavailable"},
		{"MaxAttempts=2", Options{MaxAttempts: 2}, func() *countingTransport {
			return &countingTransport{status: 503, body: stsErrorXML("ServiceUnavailable")}
		}, 2, "ServiceUnavailable"},
		{"MaxAttempts=6", Options{MaxAttempts: 6}, func() *countingTransport {
			return &countingTransport{status: 503, body: stsErrorXML("ServiceUnavailable")}
		}, 6, "ServiceUnavailable"},
		{"MaxAttempts=2 does not retry throttle", Options{MaxAttempts: 2}, func() *countingTransport {
			return &countingTransport{status: 400, body: stsErrorXML("Throttling")}
		}, 1, "Throttling"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ct := tc.ct()
			err := callSTS(t, tc.opts, ct)
			if got := ct.count(); got != tc.want {
				t.Errorf("requests = %d, want %d (err: %v)", got, tc.want, err)
			}
			if got := ErrorCode(err); got != tc.wantCode {
				t.Errorf("ErrorCode = %q, want %q (err: %v)", got, tc.wantCode, err)
			}
			if wantThrottle := tc.wantCode != "" && IsAwtestThrottle(err); wantThrottle != contains(AwtestThrottleCodes, tc.wantCode) {
				t.Errorf("IsAwtestThrottle mismatch for %q", tc.wantCode)
			}
		})
	}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func TestRetryPolicyConfigReportsMaxAttempts(t *testing.T) {
	isolateEnv(t)
	for _, tc := range []struct{ in, want int }{{0, 4}, {1, 1}, {2, 2}, {7, 7}} {
		cfg, err := LoadConfig(context.Background(), Options{AccessKeyID: "AKIAX", SecretAccessKey: "s", MaxAttempts: tc.in})
		if err != nil {
			t.Fatal(err)
		}
		if cfg.RetryMaxAttempts != tc.want {
			t.Errorf("MaxAttempts=%d: cfg.RetryMaxAttempts = %d, want %d", tc.in, cfg.RetryMaxAttempts, tc.want)
		}
		if cfg.Retryer == nil {
			t.Fatal("cfg.Retryer is nil")
		}
		if got := cfg.Retryer().MaxAttempts(); got != tc.want {
			t.Errorf("MaxAttempts=%d: retryer.MaxAttempts() = %d, want %d", tc.in, got, tc.want)
		}
	}
	if DefaultMaxAttempts != 4 {
		t.Errorf("DefaultMaxAttempts = %d, want 4", DefaultMaxAttempts)
	}
}

func TestRetryableDecisions(t *testing.T) {
	isolateEnv(t)
	cfg, err := LoadConfig(context.Background(), Options{AccessKeyID: "AKIAX", SecretAccessKey: "s"})
	if err != nil {
		t.Fatal(err)
	}
	r := cfg.Retryer()
	for _, code := range AwtestThrottleCodes {
		if r.IsErrorRetryable(&smithy.GenericAPIError{Code: code}) {
			t.Errorf("%s must not be retryable", code)
		}
		// Also when wrapped in the SDK's operation/response layers.
		if r.IsErrorRetryable(Wrap(&smithy.GenericAPIError{Code: code})) {
			t.Errorf("wrapped %s must not be retryable", code)
		}
	}
	if !r.IsErrorRetryable(&smithy.GenericAPIError{Code: "ThrottlingException"}) {
		t.Error("ThrottlingException should stay retryable")
	}
	if r.IsErrorRetryable(context.Canceled) {
		t.Error("context cancellation must not be retryable")
	}
	if r.IsErrorRetryable(errors.New("plain")) {
		t.Error("unknown error must not be retryable")
	}
}

func TestIsAwtestThrottle(t *testing.T) {
	for _, code := range AwtestThrottleCodes {
		if !IsAwtestThrottle(&smithy.GenericAPIError{Code: code}) {
			t.Errorf("%s not recognised", code)
		}
	}
	for _, err := range []error{nil, errors.New("Throttling"), &smithy.GenericAPIError{Code: "ThrottlingException"}, &smithy.GenericAPIError{Code: "AccessDenied"}} {
		if IsAwtestThrottle(err) {
			t.Errorf("%v wrongly recognised", err)
		}
	}
	want := []string{"RequestLimitExceeded", "Throttling", "TooManyRequestsException"}
	if strings.Join(AwtestThrottleCodes, ",") != strings.Join(want, ",") {
		t.Errorf("AwtestThrottleCodes = %v", AwtestThrottleCodes)
	}
}

// The retryer must also govern the STS client built inside the SDK loader for
// the credential chain.
func TestRetryPolicyAppliesToCredentialChainSTS(t *testing.T) {
	cases := []struct {
		name string
		ct   *countingTransport
		want int
	}{
		{"503 retried", &countingTransport{status: 503, body: stsErrorXML("ServiceUnavailable")}, 4},
		{"Throttling left to awtest", &countingTransport{status: 400, body: stsErrorXML("Throttling")}, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			isolateEnv(t)
			roleFiles(t, "", "")
			cfg, err := LoadConfig(context.Background(), Options{
				Region: testRegion, Profile: "role",
				HTTPClient: &http.Client{Transport: tc.ct}, RetryBackoff: zeroDelay,
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := cfg.Credentials.Retrieve(context.Background()); err == nil {
				t.Fatal("expected Retrieve to fail")
			}
			if got := tc.ct.count(); got != tc.want {
				t.Errorf("AssumeRole requests = %d, want %d", got, tc.want)
			}
		})
	}
}
