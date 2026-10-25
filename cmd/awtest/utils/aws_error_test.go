package utils

import (
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/smithy-go"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
)

// v2Err builds what a CallV2 service surfaces through types.AWSService.Invoke.
func v2Err(code, msg string) error {
	return awsv2.Wrap(&smithy.OperationError{ServiceID: "STS", OperationName: "GetCallerIdentity",
		Err: &smithy.GenericAPIError{Code: code, Message: msg}})
}

func TestAWSErrorInfo(t *testing.T) {
	cases := []struct {
		name      string
		err       error
		code, msg string
		ok        bool
	}{
		{"v1", awserr.New("AccessDenied", "v1 msg", nil), "AccessDenied", "v1 msg", true},
		{"v2 wrapped", v2Err("Throttling", "slow"), "Throttling", "slow", true},
		{"v2 raw smithy", &smithy.GenericAPIError{Code: "X", Message: "y"}, "X", "y", true},
		{"plain", errors.New("boom"), "", "", false},
		{"v2 non-API", awsv2.Wrap(errors.New("dial tcp: refused")), "", "", false},
		{"nil", nil, "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, msg, ok := AWSErrorInfo(tc.err)
			if code != tc.code || msg != tc.msg || ok != tc.ok {
				t.Fatalf("got (%q,%q,%v), want (%q,%q,%v)", code, msg, ok, tc.code, tc.msg, tc.ok)
			}
		})
	}
}

// captureHandle runs HandleAWSError in buffered mode and returns the printed
// lines (colour codes included) and its result.
func captureHandle(t *testing.T, debug bool, call string, err error) (string, error) {
	t.Helper()
	outputMu.Lock()
	outputBuffer = nil
	outputMu.Unlock()
	ConcurrentMode = true
	t.Cleanup(func() {
		ConcurrentMode = false
		outputMu.Lock()
		outputBuffer = nil
		outputMu.Unlock()
	})
	res := HandleAWSError(debug, call, err)
	outputMu.Lock()
	defer outputMu.Unlock()
	return strings.Join(outputBuffer, "\n"), res
}

func TestHandleAWSErrorV2InvalidKey(t *testing.T) {
	for _, code := range []string{types.InvalidClientTokenId, types.InvalidAccessKeyId} {
		out, res := captureHandle(t, false, "sts:GetCallerIdentity", v2Err(code, "raw aws text"))
		var ike *types.InvalidKeyError
		if !errors.As(res, &ike) {
			t.Fatalf("%s: result = %v, want *types.InvalidKeyError", code, res)
		}
		if ike.Message != types.AwsErrorMessages[code] {
			t.Errorf("%s: message %q, want %q", code, ike.Message, types.AwsErrorMessages[code])
		}
		if !strings.Contains(out, types.AwsErrorMessages[code]) {
			t.Errorf("%s: output %q lacks pretty message", code, out)
		}
	}
}

func TestHandleAWSErrorV2InvalidKeyQuiet(t *testing.T) {
	Quiet = true
	t.Cleanup(func() { Quiet = false })
	res := HandleAWSError(false, "sts:GetCallerIdentity", v2Err(types.InvalidClientTokenId, "x"))
	var ike *types.InvalidKeyError
	if !errors.As(res, &ike) || ike.Message != types.AwsErrorMessages[types.InvalidClientTokenId] {
		t.Fatalf("quiet mode result = %v", res)
	}
	if res := HandleAWSError(false, "x", v2Err("AccessDenied", "x")); res != nil {
		t.Errorf("quiet non-key error must return nil, got %v", res)
	}
}

func TestHandleAWSErrorV2MatchesV1Output(t *testing.T) {
	for _, tc := range []struct{ code, msg string }{
		{"AccessDenied", "User is not authorized"},
		{"UnauthorizedOperation", "You are not authorized"},
		{"SomethingElse", "custom message"},
		{types.InvalidClientTokenId, "bad token"},
	} {
		for _, debug := range []bool{false, true} {
			v1Out, v1Res := captureHandle(t, debug, "svc:Call", awserr.New(tc.code, tc.msg, nil))
			v2Out, v2Res := captureHandle(t, debug, "svc:Call", v2Err(tc.code, tc.msg))
			if v1Out != v2Out {
				t.Errorf("%s debug=%v: v1 output %q != v2 output %q", tc.code, debug, v1Out, v2Out)
			}
			if (v1Res == nil) != (v2Res == nil) {
				t.Errorf("%s debug=%v: v1 result %v, v2 result %v", tc.code, debug, v1Res, v2Res)
			}
		}
	}
}

func TestErrorDedupeV2UsesCode(t *testing.T) {
	SetErrorDedupe(true)
	t.Cleanup(func() { SetErrorDedupe(false) })
	if suppressRepeatedError(false, "svc:Call", v2Err("AccessDenied", "a")) {
		t.Fatal("first occurrence must print")
	}
	if !suppressRepeatedError(false, "svc:Call", v2Err("AccessDenied", "b")) {
		t.Error("same call and v2 code must be suppressed")
	}
	if suppressRepeatedError(false, "svc:Call", v2Err("Other", "a")) {
		t.Error("different v2 code must print")
	}
}
