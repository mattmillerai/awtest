package awsv2

import (
	"reflect"

	"github.com/aws/aws-sdk-go-v2/aws/retry"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/smithy-go"
)

// ErrorCode returns the API error code carried by err (for example
// "AccessDenied"), or "" if err is nil or is not a smithy.APIError.
func ErrorCode(err error) string {
	if ae := apiError(err); ae != nil {
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
	if ae := apiError(err); ae != nil {
		return apiMessage(ae)
	}
	return innermost(err).Error()
}

// apiMessage returns ae.ErrorMessage(), falling back to an ExceptionMessage
// *string field when that is empty. Some services (IVS, IVS Realtime) put the
// text of AccessDeniedException in "exceptionMessage", which the v2 types
// store in ExceptionMessage while ErrorMessage() only returns Message.
func apiMessage(ae smithy.APIError) string {
	v := reflect.ValueOf(ae)
	if v.Kind() == reflect.Ptr && v.IsNil() {
		return "" // typed nil: calling its methods may panic
	}
	if m := ae.ErrorMessage(); m != "" {
		return m
	}
	for v.IsValid() && (v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface) {
		if v.IsNil() {
			return ""
		}
		v = v.Elem()
	}
	if !v.IsValid() || v.Kind() != reflect.Struct {
		return ""
	}
	f := v.FieldByName("ExceptionMessage")
	if !f.IsValid() || f.Kind() != reflect.Ptr || f.IsNil() || f.Type().Elem().Kind() != reflect.String || !f.CanInterface() {
		return ""
	}
	return f.Elem().String()
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
	if ae := apiError(err); ae != nil {
		return ae.ErrorCode() + ": " + apiMessage(ae)
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
		case *Error:
			next = e.Err
		}
		if next == nil {
			return err
		}
		err = next
	}
}

// Error is a normalised error: Code and Message are the API error code and
// message (or "" and the plain innermost message for non-API errors) and Err
// is the original cause, kept for errors.Is / errors.As.
//
// Error() equals NormalizeError(Err). When Code is non-empty *Error satisfies
// smithy.APIError.
type Error struct {
	Code    string
	Message string
	Err     error
}

// Wrap returns err as an *Error. nil stays nil and an *Error is returned
// unchanged, so Wrap is idempotent.
func Wrap(err error) error {
	if err == nil {
		return nil
	}
	if e, ok := err.(*Error); ok {
		return e
	}
	return &Error{Code: ErrorCode(err), Message: ErrorMessage(err), Err: err}
}

// apiError returns the first smithy.APIError in err's chain, treating an
// *Error with an empty Code as transparent (it implements the interface by
// method set but represents a non-API error). It handles both Unwrap() error
// and Unwrap() []error.
func apiError(err error) smithy.APIError {
	for err != nil {
		if e, ok := err.(*Error); ok {
			if e.Code != "" {
				return e
			}
			err = e.Err
			continue
		}
		if ae, ok := err.(smithy.APIError); ok {
			return ae
		}
		switch u := err.(type) {
		case interface{ Unwrap() error }:
			err = u.Unwrap()
		case interface{ Unwrap() []error }:
			for _, c := range u.Unwrap() {
				if ae := apiError(c); ae != nil {
					return ae
				}
			}
			return nil
		default:
			return nil
		}
	}
	return nil
}

// Error implements error; it is NormalizeError(e.Err).
func (e *Error) Error() string { return NormalizeError(e.Err) }

// Unwrap returns the original cause.
func (e *Error) Unwrap() error { return e.Err }

// ErrorCode implements smithy.APIError.
func (e *Error) ErrorCode() string { return e.Code }

// ErrorMessage implements smithy.APIError.
func (e *Error) ErrorMessage() string { return e.Message }

// ErrorFault implements smithy.APIError, delegating to the cause when it is an
// API error.
func (e *Error) ErrorFault() smithy.ErrorFault {
	if ae := apiError(e.Err); ae != nil {
		return ae.ErrorFault()
	}
	return smithy.FaultUnknown
}
