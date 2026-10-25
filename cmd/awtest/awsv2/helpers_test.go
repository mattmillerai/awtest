package awsv2

import (
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
)

// isolateEnv points HOME and every AWS_* variable the SDK consults at empty or
// temporary values so tests never touch real configuration or the network.
func isolateEnv(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	for _, k := range []string{
		"AWS_PROFILE", "AWS_DEFAULT_PROFILE", "AWS_SHARED_CREDENTIALS_FILE", "AWS_CONFIG_FILE",
		"AWS_ENDPOINT_URL", "AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_SESSION_TOKEN",
		"AWS_REGION", "AWS_DEFAULT_REGION", "AWS_ENDPOINT_URL_STS", "AWS_ENDPOINT_URL_S3",
		"AWS_CONTAINER_CREDENTIALS_FULL_URI", "AWS_CONTAINER_CREDENTIALS_RELATIVE_URI",
		"AWS_WEB_IDENTITY_TOKEN_FILE", "AWS_ROLE_ARN", "AWS_USE_FIPS_ENDPOINT", "AWS_USE_DUALSTACK_ENDPOINT",
	} {
		t.Setenv(k, "")
	}
}

const callerIdentityXML = `<GetCallerIdentityResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/">
<GetCallerIdentityResult><Arn>arn:aws:iam::123456789012:user/test</Arn><UserId>AIDATEST</UserId><Account>123456789012</Account></GetCallerIdentityResult>
<ResponseMetadata><RequestId>r-1</RequestId></ResponseMetadata>
</GetCallerIdentityResponse>`

// fakeTransport is an http.RoundTripper that records requests and returns a
// canned GetCallerIdentity response without touching the network.
type fakeTransport struct {
	mu   sync.Mutex
	reqs []*http.Request
}

func (f *fakeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	f.mu.Lock()
	f.reqs = append(f.reqs, req)
	f.mu.Unlock()
	return &http.Response{
		StatusCode: 200,
		Status:     "200 OK",
		Header:     http.Header{"Content-Type": []string{"text/xml"}},
		Body:       io.NopCloser(strings.NewReader(callerIdentityXML)),
		Request:    req,
	}, nil
}

func (f *fakeTransport) client() *http.Client { return &http.Client{Transport: f} }
