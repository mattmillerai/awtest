package utils

import (
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/sqs"
)

func TestValidateEndpointURL(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr string
	}{
		{"http with port", "http://localhost:4566", "http://localhost:4566", ""},
		{"https with path", "https://example.com:8443/aws/", "https://example.com:8443/aws/", ""},
		{"uppercase scheme", "HTTP://localhost:4566", "HTTP://localhost:4566", ""},
		{"whitespace trimmed", "  http://localhost:4566\n", "http://localhost:4566", ""},
		{"missing scheme", "localhost:4566", "", "scheme must be http or https"},
		{"no scheme at all", "example.com", "", "scheme must be http or https"},
		{"ftp", "ftp://x", "", "scheme must be http or https"},
		{"no host", "http://", "", "missing host"},
		{"no host with port", "http://:4566", "", "missing host"},
		{"garbage", "http://[::1", "", "invalid -endpoint-url"},
		{"empty", "", "", "empty"},
		{"blank", "   ", "", "empty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateEndpointURL(tt.in)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveEndpointURL(t *testing.T) {
	env := func(m map[string]string) func(string) string {
		return func(k string) string { return m[k] }
	}
	tests := []struct {
		name     string
		flag     string
		env      map[string]string
		want     string
		wantErr  string
		wantErrW string
	}{
		{"flag wins over env", "http://flag:1", map[string]string{"AWS_ENDPOINT_URL": "http://env:2"}, "http://flag:1", "", ""},
		{"env only", "", map[string]string{"AWS_ENDPOINT_URL": "http://env:2"}, "http://env:2", "", ""},
		{"neither", "", nil, "", "", ""},
		{"bad env", "", map[string]string{"AWS_ENDPOINT_URL": "notaurl"}, "", "AWS_ENDPOINT_URL", "scheme"},
		{"bad flag", "ftp://x", nil, "", "flag -endpoint-url", "scheme"},
		{"bad flag not rescued by good env", "ftp://x", map[string]string{"AWS_ENDPOINT_URL": "http://env:2"}, "", "flag -endpoint-url", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveEndpointURL(tt.flag, env(tt.env))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) || !strings.Contains(err.Error(), tt.wantErrW) {
					t.Fatalf("err = %v, want containing %q and %q", err, tt.wantErr, tt.wantErrW)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func newTestSession(t *testing.T) *session.Session {
	t.Helper()
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String("us-west-2"),
		Credentials: credentials.NewStaticCredentials("AKIAFAKE", "fake", ""),
	})
	if err != nil {
		t.Fatal(err)
	}
	return sess
}

func TestApplyEndpointEmptyIsNoOp(t *testing.T) {
	sess := newTestSession(t)
	ApplyEndpoint(sess, "")
	if sess.Config.Endpoint != nil || sess.Config.S3ForcePathStyle != nil {
		t.Errorf("expected no-op, got Endpoint=%v S3ForcePathStyle=%v", sess.Config.Endpoint, sess.Config.S3ForcePathStyle)
	}
}

func TestApplyEndpointSetsConfig(t *testing.T) {
	sess := newTestSession(t)
	ApplyEndpoint(sess, "http://localhost:4566")
	if aws.StringValue(sess.Config.Endpoint) != "http://localhost:4566" {
		t.Errorf("Endpoint = %q", aws.StringValue(sess.Config.Endpoint))
	}
	if !aws.BoolValue(sess.Config.S3ForcePathStyle) {
		t.Error("S3ForcePathStyle not set")
	}
}

func TestApplyEndpointInheritedByClients(t *testing.T) {
	const ep = "http://localhost:4566"
	sess := newTestSession(t)
	ApplyEndpoint(sess, ep)

	if got := s3.New(sess).Client.Endpoint; got != ep {
		t.Errorf("s3 endpoint = %q", got)
	}
	if got := sqs.New(sess.Copy(&aws.Config{Region: aws.String("eu-west-1")})).Client.Endpoint; got != ep {
		t.Errorf("sqs copy endpoint = %q", got)
	}
	if got := ec2.New(sess, &aws.Config{Region: aws.String("ap-south-1")}).Client.Endpoint; got != ep {
		t.Errorf("ec2 endpoint = %q", got)
	}
}

func TestApplyEndpointS3PathStyle(t *testing.T) {
	sess := newTestSession(t)
	ApplyEndpoint(sess, "http://localhost:4566")
	req, _ := s3.New(sess).GetObjectRequest(&s3.GetObjectInput{
		Bucket: aws.String("my-bucket"),
		Key:    aws.String("key.txt"),
	})
	if err := req.Build(); err != nil {
		t.Fatal(err)
	}
	if req.HTTPRequest.URL.Host != "localhost:4566" {
		t.Errorf("host = %q, want localhost:4566", req.HTTPRequest.URL.Host)
	}
	if !strings.HasPrefix(req.HTTPRequest.URL.Path, "/my-bucket/") {
		t.Errorf("path = %q, want path-style with bucket", req.HTTPRequest.URL.Path)
	}
}
