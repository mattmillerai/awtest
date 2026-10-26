package awsv2

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
)

type writerToBody struct {
	*bytes.Reader
	closed bool
}

func (b *writerToBody) Close() error { b.closed = true; return nil }

type captureClient struct{ body io.ReadCloser }

func (c *captureClient) Do(r *http.Request) (*http.Response, error) {
	c.body = r.Body
	return &http.Response{StatusCode: 200, Body: http.NoBody}, nil
}

// A request body that implements io.WriterTo must not reach net/http as one:
// see bodyShieldClient for the write-loop race this prevents.
func TestBodyShieldHidesWriterTo(t *testing.T) {
	cap := &captureClient{}
	c := shieldHTTPClient(cap)
	orig := &writerToBody{Reader: bytes.NewReader([]byte("payload"))}
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "http://example.invalid", orig)
	if _, err := c.Do(req); err != nil {
		t.Fatal(err)
	}
	if _, ok := cap.body.(io.WriterTo); ok {
		t.Fatal("body seen by the underlying client still implements io.WriterTo")
	}
	got, err := io.ReadAll(cap.body)
	if err != nil || string(got) != "payload" {
		t.Fatalf("read = %q, %v", got, err)
	}
	if err := cap.body.Close(); err != nil || !orig.closed {
		t.Fatalf("Close not delegated: err=%v closed=%v", err, orig.closed)
	}
}

func TestBodyShieldPassesThroughNilAndNoBody(t *testing.T) {
	cap := &captureClient{}
	c := shieldHTTPClient(cap)
	for _, b := range []io.ReadCloser{nil, http.NoBody} {
		req, _ := http.NewRequest("GET", "http://example.invalid", nil)
		req.Body = b
		if _, err := c.Do(req); err != nil {
			t.Fatal(err)
		}
		if cap.body != b {
			t.Fatalf("body %v was wrapped as %v", b, cap.body)
		}
	}
	if shieldHTTPClient(nil) != nil {
		t.Fatal("nil client must stay nil")
	}
	if again := shieldHTTPClient(c); again != c {
		t.Fatal("shielding twice must not double-wrap")
	}
}

func TestLoadConfigShieldsHTTPClient(t *testing.T) {
	cfg, err := LoadConfig(context.Background(), Options{
		Region: "us-east-1", AccessKeyID: "AKIAEXAMPLE", SecretAccessKey: "x",
		HTTPClient: &captureClient{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg.HTTPClient.(*bodyShieldClient); !ok {
		t.Fatalf("cfg.HTTPClient is %T, want *bodyShieldClient", cfg.HTTPClient)
	}
	var _ aws.HTTPClient = cfg.HTTPClient
}
