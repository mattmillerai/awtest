package utils

import (
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
)

// AWSErrorInfo extracts the API error code and message from an
// aws-sdk-go-v2 API error (smithy.APIError), including one wrapped by
// awsv2.Wrap or the SDK's operation/retry wrappers. ok is false for plain Go
// errors and for nil.
func AWSErrorInfo(err error) (code, message string, ok bool) {
	if code := awsv2.ErrorCode(err); code != "" {
		return code, awsv2.ErrorMessage(err), true
	}
	return "", "", false
}
