package main

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/smithy-go"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
)

// v2APIError builds the error a CallV2 service yields through
// types.AWSService.Invoke: a smithy API error inside the SDK's operation
// wrapper, normalised with awsv2.Wrap.
func v2APIError(code string) error {
	return awsv2.Wrap(&smithy.OperationError{ServiceID: "EC2", OperationName: "DescribeInstances",
		Err: &smithy.GenericAPIError{Code: code, Message: "msg for " + code}})
}

func TestClassifyAWSErrorV2(t *testing.T) {
	cases := []struct {
		code string
		want ErrorCategory
	}{
		{"RequestLimitExceeded", ErrorThrottle},
		{"Throttling", ErrorThrottle},
		{"TooManyRequestsException", ErrorThrottle},
		{"AccessDeniedException", ErrorDenied},
		{"AccessDenied", ErrorDenied},
		{"UnauthorizedOperation", ErrorDenied},
		{"AuthorizationError", ErrorDenied},
		{"UnauthorizedAccess", ErrorDenied},
		{"InternalError", ErrorService},
		{"ThrottlingException", ErrorService}, // retried by the SDK, not by awtest
	}
	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			if got := classifyAWSError(v2APIError(tc.code)); got != tc.want {
				t.Errorf("v2 %s: got %v, want %v", tc.code, got, tc.want)
			}
			if got := classifyAWSError(awserr.New(tc.code, "m", nil)); got != tc.want {
				t.Errorf("v1 %s: got %v, want %v", tc.code, got, tc.want)
			}
		})
	}
	if got := classifyAWSError(awsv2.Wrap(errors.New("dial tcp: connection refused"))); got != ErrorService {
		t.Errorf("v2 non-API error: got %v, want ErrorService", got)
	}
	if got := classifyAWSError(nil); got != ErrorNone {
		t.Errorf("nil: got %v", got)
	}
}

func TestFailureDetailV2(t *testing.T) {
	if got := failureDetail(v2APIError("AuthFailure")); got != "AuthFailure" {
		t.Errorf("v2 detail = %q, want the code", got)
	}
	if got := failureDetail(awserr.New("AuthFailure", "m", nil)); got != "AuthFailure" {
		t.Errorf("v1 detail = %q, want the code", got)
	}
	plain := errors.New("plain failure")
	if got := failureDetail(plain); got != "plain failure" {
		t.Errorf("plain detail = %q", got)
	}
	if got := failureDetail(awsv2.Wrap(plain)); got != "plain failure" {
		t.Errorf("v2 non-API detail = %q", got)
	}
}

// probeThrottleCodes are the codes probed for the reverse cross-check: the
// awtest throttle codes plus throttle-like codes that awtest must NOT retry
// (the SDK handles them).
var probeThrottleCodes = []string{
	"RequestLimitExceeded", "Throttling", "TooManyRequestsException",
	"ThrottlingException", "SlowDown", "RequestThrottled", "ProvisionedThroughputExceededException",
	"ThrottledException", "EC2ThrottledException", "TooManyRequests", "BandwidthLimitExceeded",
}

// The SDK-vs-awtest retry split must not drift: the codes awsv2's retryer
// refuses to retry are exactly the codes classifyAWSError treats as throttle
// (and so retries itself with backoff).
func TestThrottleCodesMatchAwsv2(t *testing.T) {
	for _, code := range awsv2.AwtestThrottleCodes {
		if got := classifyAWSError(awserr.New(code, "m", nil)); got != ErrorThrottle {
			t.Errorf("v1 %s: classified %v, want ErrorThrottle", code, got)
		}
		if got := classifyAWSError(v2APIError(code)); got != ErrorThrottle {
			t.Errorf("v2 %s: classified %v, want ErrorThrottle", code, got)
		}
	}

	inList := map[string]bool{}
	for _, c := range awsv2.AwtestThrottleCodes {
		inList[c] = true
	}
	var missing []string
	for _, code := range probeThrottleCodes {
		for name, err := range map[string]error{"v1": awserr.New(code, "m", nil), "v2": v2APIError(code)} {
			if classifyAWSError(err) == ErrorThrottle && !inList[code] {
				missing = append(missing, fmt.Sprintf("%s(%s)", code, name))
			}
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Errorf("classifyAWSError treats these as throttle but awsv2.AwtestThrottleCodes lacks them (SDK would retry too): %v", missing)
	}
	for _, code := range []string{"ThrottlingException", "SlowDown", "RequestThrottled", "ProvisionedThroughputExceededException"} {
		if inList[code] {
			t.Errorf("%s is in awsv2.AwtestThrottleCodes but awtest does not retry it", code)
		}
	}
}

func TestSafeScanV2ServiceUsesContextConfig(t *testing.T) {
	var gotRegion string
	svc := types.AWSService{
		Name: "v2-throttled",
		CallV2: func(_ context.Context, cfg aws.Config) (interface{}, error) {
			gotRegion = cfg.Region
			return nil, &smithy.GenericAPIError{Code: "Throttling", Message: "slow down"}
		},
		Process: func(_ interface{}, err error, _ bool) []types.ScanResult {
			return []types.ScanResult{{ServiceName: "v2-throttled", Error: err}}
		},
	}
	ctx := types.WithAWSConfig(context.Background(), aws.Config{Region: "eu-north-1"})
	results, cat := safeScan(ctx, svc, nil, false)
	if cat != ErrorThrottle {
		t.Errorf("category = %v, want ErrorThrottle", cat)
	}
	if gotRegion != "eu-north-1" {
		t.Errorf("CallV2 saw region %q, want the context config's", gotRegion)
	}
	if len(results) != 1 || results[0].Error == nil || results[0].Error.Error() != "Throttling: slow down" {
		t.Errorf("results = %+v, want one normalised error", results)
	}

	// No config in the context: a clear error, not a panic.
	results, cat = safeScan(context.Background(), svc, nil, false)
	if cat != ErrorService || len(results) != 1 || results[0].Error == nil {
		t.Errorf("missing config: cat=%v results=%+v", cat, results)
	}
}
