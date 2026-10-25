package awsv2

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/smithy-go"
)

func TestWrapNil(t *testing.T) {
	if Wrap(nil) != nil {
		t.Fatal("Wrap(nil) must be nil")
	}
}

func TestWrapAPIError(t *testing.T) {
	api := &smithy.GenericAPIError{Code: "AccessDenied", Message: "nope", Fault: smithy.FaultClient}
	op := &smithy.OperationError{ServiceID: "STS", OperationName: "GetCallerIdentity", Err: api}
	err := Wrap(op)

	var e *Error
	if !errors.As(err, &e) {
		t.Fatalf("Wrap returned %T", err)
	}
	if e.Code != "AccessDenied" || e.Message != "nope" || e.Err != op {
		t.Fatalf("got %+v", e)
	}
	if err.Error() != "AccessDenied: nope" || err.Error() != NormalizeError(op) {
		t.Fatalf("Error() = %q", err.Error())
	}
	if !errors.Is(err, api) || errors.Unwrap(err) != op {
		t.Fatal("Unwrap must expose the cause")
	}

	var ae smithy.APIError
	if !errors.As(err, &ae) || ae.ErrorCode() != "AccessDenied" || ae.ErrorMessage() != "nope" || ae.ErrorFault() != smithy.FaultClient {
		t.Fatalf("errors.As APIError: %v", ae)
	}
	var oe *smithy.OperationError
	if !errors.As(err, &oe) || oe != op {
		t.Fatal("errors.As must still reach *smithy.OperationError")
	}
	if ErrorCode(err) != "AccessDenied" || ErrorMessage(err) != "nope" || !HasErrorCode(err, "AccessDenied") {
		t.Fatal("helpers on wrapped error")
	}
}

func TestWrapIdempotentAndNested(t *testing.T) {
	api := &smithy.GenericAPIError{Code: "Boom", Message: "m"}
	once := Wrap(api)
	if twice := Wrap(once); twice != once {
		t.Fatal("Wrap of an *Error must return it unchanged")
	}

	// An *Error buried inside another wrapper is re-wrapped but renders the same.
	buried := fmt.Errorf("ctx: %w", once)
	w := Wrap(buried)
	if w == buried || w.Error() != "Boom: m" || ErrorCode(w) != "Boom" {
		t.Fatalf("nested: %v / %q", w, ErrorCode(w))
	}
	var ae smithy.APIError
	if !errors.As(w, &ae) || ae.ErrorCode() != "Boom" {
		t.Fatal("nested errors.As APIError")
	}
}

func TestWrapContextErrors(t *testing.T) {
	for _, base := range []error{context.Canceled, context.DeadlineExceeded} {
		err := Wrap(&smithy.OperationError{ServiceID: "STS", OperationName: "X", Err: fmt.Errorf("send: %w", base)})
		if !errors.Is(err, base) {
			t.Errorf("errors.Is(%v) lost", base)
		}
		if err.Error() != "send: "+base.Error() && err.Error() != base.Error() {
			t.Errorf("Error() = %q", err.Error())
		}
		if ErrorCode(err) != "" {
			t.Errorf("context error has code %q", ErrorCode(err))
		}
	}
	// Plain context error: unchanged message.
	if got := Wrap(context.Canceled).Error(); got != "context canceled" {
		t.Fatalf("got %q", got)
	}
}

func TestWrapNonAPIError(t *testing.T) {
	plain := &smithy.OperationError{ServiceID: "STS", OperationName: "X", Err: errors.New("dial tcp: lookup sts: no such host")}
	err := Wrap(plain)
	var e *Error
	if !errors.As(err, &e) {
		t.Fatal("not an *Error")
	}
	if e.Code != "" || e.Message != "dial tcp: lookup sts: no such host" {
		t.Fatalf("got %+v", e)
	}
	// No ": " prefix even though *Error implements smithy.APIError by method set.
	if err.Error() != "dial tcp: lookup sts: no such host" {
		t.Fatalf("Error() = %q", err.Error())
	}
	if ErrorCode(err) != "" || HasErrorCode(err, "") {
		t.Fatal("non-API error must have no code")
	}
	if ErrorMessage(err) != "dial tcp: lookup sts: no such host" || NormalizeError(err) != err.Error() {
		t.Fatal("helpers on wrapped non-API error")
	}
	if f := e.ErrorFault(); f != smithy.FaultUnknown {
		t.Fatalf("fault = %v", f)
	}
	// Wrapping again, and nesting under another wrapper, stays clean.
	if got := Wrap(fmt.Errorf("outer: %w", err)).Error(); got != "outer: dial tcp: lookup sts: no such host" && got != "dial tcp: lookup sts: no such host" {
		t.Fatalf("nested non-API: %q", got)
	}
}

func TestWrapJoinedErrors(t *testing.T) {
	api := &smithy.GenericAPIError{Code: "Denied", Message: "d"}
	err := Wrap(errors.Join(errors.New("first"), api))
	if ErrorCode(err) != "Denied" {
		t.Fatalf("code = %q", ErrorCode(err))
	}
}

func TestErrorFaultDelegates(t *testing.T) {
	for _, f := range []smithy.ErrorFault{smithy.FaultClient, smithy.FaultServer, smithy.FaultUnknown} {
		e := Wrap(&smithy.GenericAPIError{Code: "C", Message: "m", Fault: f}).(*Error)
		if e.ErrorFault() != f {
			t.Errorf("fault %v not delegated", f)
		}
	}
}

func TestErrorSatisfiesAPIErrorOnlyWithCode(t *testing.T) {
	var _ smithy.APIError = (*Error)(nil)
	e := &Error{Code: "X", Message: "y", Err: errors.New("cause")}
	var ae smithy.APIError
	if !errors.As(error(e), &ae) || ae.ErrorCode() != "X" || ae.ErrorMessage() != "y" {
		t.Fatal("Code != \"\" must satisfy smithy.APIError")
	}
}

// A real SDK error: the rendered text carries neither the request ID, HTTP
// status nor operation prefix, while errors.As still reaches the transport
// error types.
func TestWrapRealSDKError(t *testing.T) {
	ct := &countingTransport{status: 403, body: stsErrorXML("AccessDenied")}
	raw := callSTS(t, Options{MaxAttempts: 1}, ct)

	// Sanity: the raw SDK text is noisy.
	if !strings.Contains(raw.Error(), "StatusCode: 403") || !strings.Contains(raw.Error(), "RequestID: req-1") || !strings.Contains(raw.Error(), "operation error") {
		t.Fatalf("raw error is not noisy, test is vacuous: %v", raw)
	}

	err := Wrap(raw)
	if err.Error() != "AccessDenied: fake AccessDenied" {
		t.Fatalf("Error() = %q", err.Error())
	}
	for _, noise := range []string{"req-1", "403", "StatusCode", "RequestID", "operation error", "GetCallerIdentity", "STS"} {
		if strings.Contains(err.Error(), noise) {
			t.Errorf("Error() contains %q: %q", noise, err.Error())
		}
	}
	var re *awshttp.ResponseError
	if !errors.As(err, &re) || re.HTTPStatusCode() != http.StatusForbidden || re.ServiceRequestID() != "req-1" {
		t.Fatal("errors.As must still reach *awshttp.ResponseError with status and request id")
	}
	var oe *smithy.OperationError
	if !errors.As(err, &oe) || oe.OperationName != "GetCallerIdentity" {
		t.Fatal("errors.As must still reach *smithy.OperationError")
	}
	var ae smithy.APIError
	if !errors.As(err, &ae) || ae.ErrorCode() != "AccessDenied" {
		t.Fatal("errors.As APIError")
	}
}
