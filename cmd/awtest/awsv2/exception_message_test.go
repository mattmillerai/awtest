package awsv2

import (
	"testing"

	ivstypes "github.com/aws/aws-sdk-go-v2/service/ivs/types"
	"github.com/aws/smithy-go"
)

func TestErrorMessageExceptionMessageFallback(t *testing.T) {
	em := "denied by ivs"
	err := &ivstypes.AccessDeniedException{ExceptionMessage: &em}
	if got := ErrorMessage(err); got != em {
		t.Errorf("ErrorMessage = %q, want %q", got, em)
	}
	wrapped := &smithy.OperationError{ServiceID: "ivs", OperationName: "ListChannels", Err: err}
	if got := NormalizeError(wrapped); got != "AccessDeniedException: "+em {
		t.Errorf("NormalizeError = %q", got)
	}
	if got := Wrap(wrapped).(*Error).Message; got != em {
		t.Errorf("Wrap message = %q", got)
	}
}

func TestErrorMessagePrefersMessage(t *testing.T) {
	m, em := "primary", "secondary"
	err := &ivstypes.AccessDeniedException{Message: &m, ExceptionMessage: &em}
	if got := ErrorMessage(err); got != m {
		t.Errorf("ErrorMessage = %q, want %q", got, m)
	}
}

func TestErrorMessageNilSafe(t *testing.T) {
	var nilPtr *ivstypes.AccessDeniedException
	_ = ErrorMessage(nilPtr)
	if got := ErrorMessage(&ivstypes.AccessDeniedException{}); got != "" {
		t.Errorf("empty = %q", got)
	}
	if got := ErrorMessage(&smithy.GenericAPIError{Code: "X"}); got != "" {
		t.Errorf("generic = %q", got)
	}
}
