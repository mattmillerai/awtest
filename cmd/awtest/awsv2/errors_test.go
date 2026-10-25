package awsv2

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go"
)

func TestNormalizeErrorNil(t *testing.T) {
	if NormalizeError(nil) != "" || ErrorCode(nil) != "" || ErrorMessage(nil) != "" || HasErrorCode(nil, "X") {
		t.Fatal("nil must yield zero values")
	}
}

func TestNormalizeErrorSynthetic(t *testing.T) {
	api := &smithy.GenericAPIError{Code: "AccessDenied", Message: "nope"}
	if got := NormalizeError(api); got != "AccessDenied: nope" {
		t.Fatal(got)
	}
	wrapped := &smithy.OperationError{ServiceID: "STS", OperationName: "GetCallerIdentity", Err: fmt.Errorf("wrap: %w", api)}
	if got := NormalizeError(wrapped); got != "AccessDenied: nope" {
		t.Fatal(got)
	}
	if ErrorCode(wrapped) != "AccessDenied" || ErrorMessage(wrapped) != "nope" {
		t.Fatal("code/message")
	}
	if !HasErrorCode(wrapped, "Other", "AccessDenied") || HasErrorCode(wrapped, "Other") || HasErrorCode(wrapped) {
		t.Fatal("HasErrorCode")
	}
	plain := &smithy.OperationError{ServiceID: "STS", OperationName: "GetCallerIdentity", Err: errors.New("boom")}
	if got := NormalizeError(plain); got != "boom" {
		t.Fatal(got)
	}
	if ErrorCode(plain) != "" || ErrorMessage(plain) != "boom" || HasErrorCode(plain, "") {
		t.Fatal("plain")
	}
	if got := NormalizeError(errors.New("bare")); got != "bare" {
		t.Fatal(got)
	}
}

func stsAgainst(t *testing.T, endpoint string) *sts.Client {
	t.Helper()
	isolateEnv(t)
	cfg, err := LoadConfig(context.Background(), Options{Region: "us-east-1", AccessKeyID: "AKIAFAKE", SecretAccessKey: "fake", Endpoint: endpoint})
	if err != nil {
		t.Fatal(err)
	}
	return NewClient(cfg, ClientParams{Endpoint: endpoint}, func(c aws.Config, fns ...func(*sts.Options)) *sts.Client {
		return sts.NewFromConfig(c, append(fns, func(o *sts.Options) { o.RetryMaxAttempts = 1 })...)
	})
}

func errServer(t *testing.T, status int, code string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml")
		w.WriteHeader(status)
		fmt.Fprintf(w, `<ErrorResponse><Error><Type>Sender</Type><Code>%s</Code><Message>nope</Message></Error><RequestId>r-1</RequestId></ErrorResponse>`, code)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestNormalizeErrorEndToEnd(t *testing.T) {
	for _, tt := range []struct {
		status int
		code   string
	}{{403, "AccessDenied"}, {500, "InternalFailure"}} {
		t.Run(fmt.Sprint(tt.status), func(t *testing.T) {
			srv := errServer(t, tt.status, tt.code)
			_, err := stsAgainst(t, srv.URL).GetCallerIdentity(context.Background(), &sts.GetCallerIdentityInput{})
			if err == nil {
				t.Fatal("expected error")
			}
			want := tt.code + ": nope"
			if got := NormalizeError(err); got != want {
				t.Fatalf("got %q want %q (raw %v)", got, want, err)
			}
			if !HasErrorCode(err, tt.code) || ErrorCode(err) != tt.code {
				t.Fatal("code")
			}
			if !strings.Contains(err.Error(), "r-1") {
				t.Log("raw error has no request id")
			}
		})
	}
}

func TestNormalizeErrorTransport(t *testing.T) {
	// Connection refused.
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()
	_, err := stsAgainst(t, url).GetCallerIdentity(context.Background(), &sts.GetCallerIdentityInput{})
	if err == nil {
		t.Fatal("expected error")
	}
	got := NormalizeError(err)
	if !strings.Contains(got, "connection refused") {
		t.Fatalf("got %q", got)
	}
	for _, noise := range []string{"operation error", "exceeded maximum number of attempts", "RequestID", "https response error"} {
		if strings.Contains(got, noise) {
			t.Fatalf("%q contains noise %q", got, noise)
		}
	}
	if ErrorCode(err) != "" {
		t.Fatalf("code = %q", ErrorCode(err))
	}

	// Context cancellation.
	srv2 := errServer(t, 403, "AccessDenied")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = stsAgainst(t, srv2.URL).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err == nil {
		t.Fatal("expected error")
	}
	got = NormalizeError(err)
	if !strings.Contains(got, "context canceled") || strings.Contains(got, "operation error") || strings.Contains(got, ": AccessDenied") {
		t.Fatalf("got %q", got)
	}
}
