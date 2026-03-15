package main

import (
	"context"
	"testing"
	"time"

	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/aws/aws-sdk-go/aws/session"
)

// mockService creates a test AWSService with a controllable Call duration.
func mockService(name string, delay time.Duration) types.AWSService {
	return types.AWSService{
		Name: name,
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			select {
			case <-time.After(delay):
				return name + "-result", nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			if err != nil {
				return []types.ScanResult{
					{
						ServiceName: name,
						MethodName:  name,
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}
			return []types.ScanResult{
				{
					ServiceName:  name,
					MethodName:   name,
					ResourceType: "mock",
					ResourceName: "mock-resource",
					Timestamp:    time.Now(),
				},
			}
		},
	}
}

func TestScanServices_NoTimeout(t *testing.T) {
	svcs := []types.AWSService{
		mockService("svc-a", 0),
		mockService("svc-b", 0),
		mockService("svc-c", 0),
	}

	ctx := context.Background()
	results, skipped := scanServices(ctx, svcs, nil, 1, true, false)

	if len(skipped) != 0 {
		t.Errorf("expected 0 skipped services, got %d", len(skipped))
	}
	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}
}

func TestScanServices_AlreadyExpiredContext(t *testing.T) {
	svcs := []types.AWSService{
		mockService("svc-a", 0),
		mockService("svc-b", 0),
		mockService("svc-c", 0),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	results, skipped := scanServices(ctx, svcs, nil, 1, true, false)

	if len(skipped) != 3 {
		t.Errorf("expected 3 skipped services, got %d", len(skipped))
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestScanServices_TimeoutMidScan(t *testing.T) {
	svcs := []types.AWSService{
		mockService("fast-svc", 0),
		mockService("slow-svc", 500*time.Millisecond),
		mockService("never-svc", 0),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// First service completes instantly.
	// Second service takes 500ms but timeout is 100ms, so it gets cancelled mid-call.
	// Third service should be skipped.
	results, skipped := scanServices(ctx, svcs, nil, 1, true, false)

	// fast-svc should complete successfully
	hasSuccessful := false
	for _, r := range results {
		if r.ServiceName == "fast-svc" && r.Error == nil {
			hasSuccessful = true
		}
	}
	if !hasSuccessful {
		t.Error("expected fast-svc to complete successfully")
	}

	// slow-svc should return with a context error (deadline exceeded)
	hasTimedOut := false
	for _, r := range results {
		if r.ServiceName == "slow-svc" && r.Error != nil {
			hasTimedOut = true
		}
	}
	if !hasTimedOut {
		t.Error("expected slow-svc to return with timeout error")
	}

	// never-svc should be in skipped list
	if len(skipped) != 1 || skipped[0] != "never-svc" {
		t.Errorf("expected [never-svc] skipped, got %v", skipped)
	}
}

func TestScanServices_TimeoutBeforeFirstService(t *testing.T) {
	svcs := []types.AWSService{
		mockService("svc-a", 0),
		mockService("svc-b", 0),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before any service runs

	results, skipped := scanServices(ctx, svcs, nil, 1, true, false)

	if len(results) != 0 {
		t.Errorf("expected 0 results when cancelled before first service, got %d", len(results))
	}
	if len(skipped) != 2 {
		t.Errorf("expected 2 skipped services, got %d", len(skipped))
	}
}

func TestScanServices_PartialResultsPreserved(t *testing.T) {
	svcs := []types.AWSService{
		mockService("completed-1", 0),
		mockService("completed-2", 0),
		mockService("slow-blocker", 500*time.Millisecond),
		mockService("skipped-1", 0),
		mockService("skipped-2", 0),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	results, skipped := scanServices(ctx, svcs, nil, 1, true, false)

	// First two should complete, slow-blocker gets cancelled, last two skipped
	completedCount := 0
	for _, r := range results {
		if r.Error == nil {
			completedCount++
		}
	}
	if completedCount != 2 {
		t.Errorf("expected 2 successfully completed results, got %d", completedCount)
	}

	if len(skipped) != 2 {
		t.Errorf("expected 2 skipped services, got %d: %v", len(skipped), skipped)
	}
}
