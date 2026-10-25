package utils

import (
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
)

// AWSErrorInfo extracts the API error code and message from an error of either
// SDK: an aws-sdk-go v1 awserr.Error (matched exactly as before, by direct
// type assertion) or an aws-sdk-go-v2 API error, including one wrapped by
// awsv2.Wrap or the SDK's operation/retry wrappers. ok is false for plain Go
// errors and for nil.
func AWSErrorInfo(err error) (code, message string, ok bool) {
	if awsErr, isV1 := err.(awserr.Error); isV1 {
		return awsErr.Code(), awsErr.Message(), true
	}
	if code := awsv2.ErrorCode(err); code != "" {
		return code, awsv2.ErrorMessage(err), true
	}
	return "", "", false
}
