package awsv2

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
)

// DefaultMaxAttempts is the total number of attempts per request. aws-sdk-go
// v1 retried 3 times, i.e. 4 attempts; v2's own default is 3 attempts.
const DefaultMaxAttempts = 4

// AwtestThrottleCodes are the API error codes awtest's own scan backoff
// (cmd/awtest/backoff.go, classified by classifyAWSError in
// cmd/awtest/safe_scan.go) retries with 1s/2s/4s jittered delays. The SDK must
// never retry them as well, or the two layers would multiply.
//
// This list must stay in sync with classifyAWSError in cmd/awtest/safe_scan.go.
var AwtestThrottleCodes = []string{"RequestLimitExceeded", "Throttling", "TooManyRequestsException"}

// IsAwtestThrottle reports whether err carries one of AwtestThrottleCodes.
func IsAwtestThrottle(err error) bool {
	return HasErrorCode(err, AwtestThrottleCodes...)
}

func maxAttempts(opts Options) int {
	if opts.MaxAttempts > 0 {
		return opts.MaxAttempts
	}
	return DefaultMaxAttempts
}

// newRetryer returns the retryer constructor installed by LoadConfig: the
// standard retryer, with awtest's throttle codes excluded from retrying and
// the SDK's default retryable checks otherwise.
func newRetryer(opts Options) func() aws.Retryer {
	attempts := maxAttempts(opts)
	return func() aws.Retryer {
		return retry.NewStandard(func(so *retry.StandardOptions) {
			so.MaxAttempts = attempts
			if opts.RetryBackoff != nil {
				so.Backoff = opts.RetryBackoff
			}
			so.Retryables = append([]retry.IsErrorRetryable{
				retry.IsErrorRetryableFunc(func(err error) aws.Ternary {
					if IsAwtestThrottle(err) {
						return aws.FalseTernary
					}
					return aws.UnknownTernary
				}),
			}, retry.DefaultRetryables...)
		})
	}
}
