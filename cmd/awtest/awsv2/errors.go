package awsv2

import (
	"errors"

	"github.com/aws/aws-sdk-go-v2/aws/retry"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/smithy-go"
)

// ErrorCode returns the API error code carried by err (for example
// "AccessDenied"), or "" if err is nil or is not a smithy.APIError.
func ErrorCode(err error) string {
	var ae smithy.APIError
	if errors.As(err, &ae) {
		return ae.ErrorCode()
	}
	return ""
}

// ErrorMessage returns the API error message carried by err or, for errors
// that are not API errors, the plain message of the innermost meaningful cause
// (see NormalizeError). It returns "" for a nil error.
func ErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	var ae smithy.APIError
	if errors.As(err, &ae) {
		return ae.ErrorMessage()
	}
	return innermost(err).Error()
}

// NormalizeError renders err as "<code>: <message>" for API errors, matching
// the shape the tool prints today. It never includes the request ID, HTTP
// status code or operation prefix. Non-API errors (timeouts, DNS failures,
// context cancellation) are rendered as the plain message of their innermost
// meaningful cause with no code prefix. A nil error yields "".
func NormalizeError(err error) string {
	if err == nil {
		return ""
	}
	var ae smithy.APIError
	if errors.As(err, &ae) {
		return ae.ErrorCode() + ": " + ae.ErrorMessage()
	}
	return innermost(err).Error()
}

// HasErrorCode reports whether err carries an API error code equal to any of
// codes.
func HasErrorCode(err error, codes ...string) bool {
	code := ErrorCode(err)
	if code == "" {
		return false
	}
	for _, c := range codes {
		if c == code {
			return true
		}
	}
	return false
}

// innermost strips the SDK's transport wrappers (smithy.OperationError, the
// retry "exceeded maximum number of attempts" error and the HTTP response
// error) and returns the first error that is not one of them.
func innermost(err error) error {
	for {
		var next error
		switch e := err.(type) {
		case *smithy.OperationError:
			next = e.Err
		case *retry.MaxAttemptsError:
			next = e.Err
		case *awshttp.ResponseError:
			next = e.Err
		}
		if next == nil {
			return err
		}
		err = next
	}
}
