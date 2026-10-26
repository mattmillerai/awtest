package main

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/smithy-go"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
)

func TestClassifyAWSError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCat  ErrorCategory
	}{
		// Nil error
		{
			name:    "nil error returns ErrorNone",
			err:     nil,
			wantCat: ErrorNone,
		},
		// Throttle errors
		{
			name:    "RequestLimitExceeded is throttle",
			err:     &smithy.GenericAPIError{Code: "RequestLimitExceeded", Message: "rate exceeded"},
			wantCat: ErrorThrottle,
		},
		{
			name:    "Throttling is throttle",
			err:     &smithy.GenericAPIError{Code: "Throttling", Message: "rate exceeded"},
			wantCat: ErrorThrottle,
		},
		{
			name:    "TooManyRequestsException is throttle",
			err:     &smithy.GenericAPIError{Code: "TooManyRequestsException", Message: "too many requests"},
			wantCat: ErrorThrottle,
		},
		// Access denied errors
		{
			name:    "AccessDeniedException is denied",
			err:     &smithy.GenericAPIError{Code: "AccessDeniedException", Message: "access denied"},
			wantCat: ErrorDenied,
		},
		{
			name:    "AccessDenied is denied",
			err:     &smithy.GenericAPIError{Code: "AccessDenied", Message: "access denied"},
			wantCat: ErrorDenied,
		},
		{
			name:    "UnauthorizedOperation is denied",
			err:     &smithy.GenericAPIError{Code: "UnauthorizedOperation", Message: "unauthorized"},
			wantCat: ErrorDenied,
		},
		{
			name:    "AuthorizationError is denied",
			err:     &smithy.GenericAPIError{Code: "AuthorizationError", Message: "auth error"},
			wantCat: ErrorDenied,
		},
		{
			name:    "UnauthorizedAccess is denied",
			err:     &smithy.GenericAPIError{Code: "UnauthorizedAccess", Message: "unauthorized access"},
			wantCat: ErrorDenied,
		},
		// Service errors
		{
			name:    "unknown AWS error is service error",
			err:     &smithy.GenericAPIError{Code: "InternalServiceError", Message: "internal error"},
			wantCat: ErrorService,
		},
		{
			name:    "plain Go error is service error",
			err:     fmt.Errorf("connection refused"),
			wantCat: ErrorService,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyAWSError(tt.err)
			if got != tt.wantCat {
				t.Errorf("classifyAWSError() = %d, want %d", got, tt.wantCat)
			}
		})
	}
}

func TestSafeScanPanicRecovery(t *testing.T) {
	panicService := types.AWSService{
		Name: "TestPanic",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			panic("something went wrong")
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			return nil // Never reached
		},
	}

	results, category := safeScan(testCtx(), panicService, false)

	if category != ErrorService {
		t.Errorf("panic category = %d, want ErrorService (%d)", category, ErrorService)
	}
	if len(results) != 1 {
		t.Fatalf("panic results count = %d, want 1", len(results))
	}
	if results[0].ServiceName != "TestPanic" {
		t.Errorf("panic result ServiceName = %q, want %q", results[0].ServiceName, "TestPanic")
	}
	if results[0].Error == nil {
		t.Fatalf("panic result Error is nil, want non-nil")
	}
	if !strings.Contains(results[0].Error.Error(), "panic recovered") {
		t.Errorf("panic result Error = %q, want containing %q", results[0].Error.Error(), "panic recovered")
	}
}

func TestSafeScanPanicWithStringValue(t *testing.T) {
	panicService := types.AWSService{
		Name: "TestPanicString",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			panic("unexpected string panic value")
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			return nil
		},
	}

	results, category := safeScan(testCtx(), panicService, false)

	if category != ErrorService {
		t.Errorf("category = %d, want ErrorService (%d)", category, ErrorService)
	}
	if len(results) != 1 {
		t.Fatalf("results count = %d, want 1", len(results))
	}
	if results[0].Error == nil {
		t.Fatalf("Error is nil, want non-nil")
	}
}

func TestSafeScanPanicNoCredentialLeakage(t *testing.T) {
	// Simulate a panic that includes credential-like data
	panicService := types.AWSService{
		Name: "TestCredLeak",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			panic("error with AKIAIOSFODNN7EXAMPLE and wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY")
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			return nil
		},
	}

	results, _ := safeScan(testCtx(), panicService, false)

	if len(results) != 1 {
		t.Fatalf("results count = %d, want 1", len(results))
	}

	errMsg := results[0].Error.Error()
	if strings.Contains(errMsg, "AKIAIOSFODNN7EXAMPLE") {
		t.Errorf("panic error message contains access key: %q", errMsg)
	}
	if strings.Contains(errMsg, "wJalrXUtnFEMI") {
		t.Errorf("panic error message contains secret key: %q", errMsg)
	}
}

func TestSafeScanThrottleClassification(t *testing.T) {
	throttleCodes := []string{"RequestLimitExceeded", "Throttling", "TooManyRequestsException"}

	for _, code := range throttleCodes {
		t.Run(code, func(t *testing.T) {
			service := types.AWSService{
				Name: "TestThrottle",
				CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
					return nil, &smithy.GenericAPIError{Code: code, Message: "rate exceeded"}
				},
				Process: func(output interface{}, err error, debug bool) []types.ScanResult {
					return []types.ScanResult{{
						ServiceName: "TestThrottle",
						Error:       err,
					}}
				},
			}

			_, category := safeScan(testCtx(), service, false)
			if category != ErrorThrottle {
				t.Errorf("category for %s = %d, want ErrorThrottle (%d)", code, category, ErrorThrottle)
			}
		})
	}
}

func TestSafeScanAccessDeniedClassification(t *testing.T) {
	deniedCodes := []string{"AccessDeniedException", "UnauthorizedOperation", "AccessDenied", "AuthorizationError", "UnauthorizedAccess"}

	for _, code := range deniedCodes {
		t.Run(code, func(t *testing.T) {
			service := types.AWSService{
				Name: "TestDenied",
				CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
					return nil, &smithy.GenericAPIError{Code: code, Message: "access denied"}
				},
				Process: func(output interface{}, err error, debug bool) []types.ScanResult {
					return []types.ScanResult{{
						ServiceName: "TestDenied",
						Error:       err,
					}}
				},
			}

			_, category := safeScan(testCtx(), service, false)
			if category != ErrorDenied {
				t.Errorf("category for %s = %d, want ErrorDenied (%d)", code, category, ErrorDenied)
			}
		})
	}
}

func TestSafeScanServiceErrorClassification(t *testing.T) {
	service := types.AWSService{
		Name: "TestServiceErr",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			return nil, &smithy.GenericAPIError{Code: "InternalServiceError", Message: "something broke"}
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			return []types.ScanResult{{
				ServiceName: "TestServiceErr",
				Error:       err,
			}}
		},
	}

	_, category := safeScan(testCtx(), service, false)
	if category != ErrorService {
		t.Errorf("category = %d, want ErrorService (%d)", category, ErrorService)
	}
}

func TestSafeScanNormalExecution(t *testing.T) {
	service := types.AWSService{
		Name: "TestNormal",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			return "mock-output", nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			return []types.ScanResult{
				{ServiceName: "TestNormal", ResourceName: "resource-1"},
				{ServiceName: "TestNormal", ResourceName: "resource-2"},
			}
		},
	}

	results, category := safeScan(testCtx(), service, false)

	if category != ErrorNone {
		t.Errorf("category = %d, want ErrorNone (%d)", category, ErrorNone)
	}
	if len(results) != 2 {
		t.Fatalf("results count = %d, want 2", len(results))
	}
	if results[0].ResourceName != "resource-1" {
		t.Errorf("results[0].ResourceName = %q, want %q", results[0].ResourceName, "resource-1")
	}
	if results[1].ResourceName != "resource-2" {
		t.Errorf("results[1].ResourceName = %q, want %q", results[1].ResourceName, "resource-2")
	}
}

func TestSafeScanNilError(t *testing.T) {
	service := types.AWSService{
		Name: "TestNilErr",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			return "output", nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			return []types.ScanResult{{ServiceName: "TestNilErr"}}
		},
	}

	_, category := safeScan(testCtx(), service, false)
	if category != ErrorNone {
		t.Errorf("category = %d, want ErrorNone (%d)", category, ErrorNone)
	}
}

func TestScanServicesPanicIsolation(t *testing.T) {
	panicService := types.AWSService{
		Name: "PanicSvc",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			panic("boom")
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			return nil
		},
	}
	normalService := types.AWSService{
		Name: "NormalSvc",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			return "ok", nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			return []types.ScanResult{{ServiceName: "NormalSvc", ResourceName: "res-1"}}
		},
	}

	svcs := []types.AWSService{panicService, normalService}
	results, skipped := scanServices(testCtx(), svcs, 1, true, false)

	if len(skipped) != 0 {
		t.Errorf("skipped = %v, want none", skipped)
	}
	if len(results) != 2 {
		t.Fatalf("results count = %d, want 2 (1 error + 1 normal)", len(results))
	}

	// First result should be the panic error
	if results[0].ServiceName != "PanicSvc" {
		t.Errorf("results[0].ServiceName = %q, want %q", results[0].ServiceName, "PanicSvc")
	}
	if results[0].Error == nil {
		t.Fatalf("results[0].Error is nil, want panic error")
	}
	if !strings.Contains(results[0].Error.Error(), "panic recovered") {
		t.Errorf("results[0].Error = %q, want containing %q", results[0].Error.Error(), "panic recovered")
	}

	// Second result should be the normal service
	if results[1].ServiceName != "NormalSvc" {
		t.Errorf("results[1].ServiceName = %q, want %q", results[1].ServiceName, "NormalSvc")
	}
	if results[1].ResourceName != "res-1" {
		t.Errorf("results[1].ResourceName = %q, want %q", results[1].ResourceName, "res-1")
	}
}

func TestSafeScanNonAWSError(t *testing.T) {
	service := types.AWSService{
		Name: "TestPlainErr",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			return nil, fmt.Errorf("connection timeout")
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			return []types.ScanResult{{
				ServiceName: "TestPlainErr",
				Error:       err,
			}}
		},
	}

	_, category := safeScan(testCtx(), service, false)
	if category != ErrorService {
		t.Errorf("category = %d, want ErrorService (%d)", category, ErrorService)
	}
}
