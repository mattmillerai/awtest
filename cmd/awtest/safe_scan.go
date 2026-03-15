package main

import (
	"context"
	"fmt"
	"time"

	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
)

// ErrorCategory classifies AWS errors for retry/skip/report decisions.
type ErrorCategory int

const (
	ErrorNone     ErrorCategory = iota // No error
	ErrorThrottle                      // Retry with backoff (Story 6.4)
	ErrorDenied                        // Skip service silently
	ErrorService                       // Report in results
)

// classifyAWSError determines how to handle an AWS API error.
func classifyAWSError(err error) ErrorCategory {
	if err == nil {
		return ErrorNone
	}

	if awsErr, ok := err.(awserr.Error); ok {
		switch awsErr.Code() {
		case "RequestLimitExceeded", "Throttling", "TooManyRequestsException":
			return ErrorThrottle
		case "AccessDeniedException", "AccessDenied", "UnauthorizedOperation", "AuthorizationError", "UnauthorizedAccess":
			return ErrorDenied
		default:
			return ErrorService
		}
	}

	// Non-AWS error (plain Go error)
	return ErrorService
}

// safeScan wraps service execution with panic recovery and error classification.
func safeScan(ctx context.Context, service types.AWSService, sess *session.Session, debug bool) (results []types.ScanResult, category ErrorCategory) {
	defer func() {
		if r := recover(); r != nil {
			// DO NOT include stack trace or panic value details that might contain credentials
			results = []types.ScanResult{{
				ServiceName: service.Name,
				MethodName:  service.Name,
				Error:       fmt.Errorf("service scan failed: panic recovered"),
				Timestamp:   time.Now(),
			}}
			category = ErrorService
		}
	}()

	output, err := service.Call(ctx, sess)
	serviceResults := service.Process(output, err, debug)

	if err != nil {
		return serviceResults, classifyAWSError(err)
	}
	return serviceResults, ErrorNone
}
