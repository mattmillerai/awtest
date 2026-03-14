package main

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
)

// --- calculateBackoff tests ---

func TestCalculateBackoffRanges(t *testing.T) {
	// Attempt 0: base 1s * 2^0 = 1s, jitter [0.5, 1.5) → [0.5s, 1.5s)
	// Attempt 1: base 1s * 2^1 = 2s, jitter [0.5, 1.5) → [1.0s, 3.0s)
	// Attempt 2: base 1s * 2^2 = 4s, jitter [0.5, 1.5) → [2.0s, 6.0s)
	tests := []struct {
		attempt int
		minMs   int64
		maxMs   int64
	}{
		{0, 500, 1500},
		{1, 1000, 3000},
		{2, 2000, 6000},
	}

	for _, tt := range tests {
		for i := 0; i < 50; i++ {
			d := calculateBackoff(tt.attempt)
			ms := d.Milliseconds()
			if ms < tt.minMs || ms >= tt.maxMs {
				t.Errorf("calculateBackoff(%d) = %dms, want [%d, %d)", tt.attempt, ms, tt.minMs, tt.maxMs)
			}
		}
	}
}

func TestCalculateBackoffJitterVariation(t *testing.T) {
	// Call 100 times and verify not all results are identical
	seen := make(map[int64]bool)
	for i := 0; i < 100; i++ {
		d := calculateBackoff(0)
		seen[d.Milliseconds()] = true
	}
	if len(seen) < 2 {
		t.Errorf("calculateBackoff(0) produced only %d distinct values over 100 calls, expected variation", len(seen))
	}
}

func TestCalculateBackoffNeverExceedsMax(t *testing.T) {
	// Even at very high attempt numbers, delay should not exceed backoffMaxDelay
	for attempt := 0; attempt < 20; attempt++ {
		for i := 0; i < 20; i++ {
			d := calculateBackoff(attempt)
			if d > backoffMaxDelay {
				t.Errorf("calculateBackoff(%d) = %v, exceeds max %v", attempt, d, backoffMaxDelay)
			}
		}
	}
}

// --- scanWithBackoff tests ---

// makeThrottleService creates a service that returns throttle errors for the first
// throttleFor calls, then succeeds with successResults.
func makeThrottleService(name string, throttleFor int, successResults []types.ScanResult) (types.AWSService, *int32) {
	var callCount int32
	svc := types.AWSService{
		Name: name,
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			n := atomic.AddInt32(&callCount, 1)
			if int(n) <= throttleFor {
				return nil, awserr.New("Throttling", "rate exceeded", nil)
			}
			return "ok", nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			if err != nil {
				return []types.ScanResult{{ServiceName: name, Error: err}}
			}
			return successResults
		},
	}
	return svc, &callCount
}

func TestScanWithBackoffNonThrottleError(t *testing.T) {
	var callCount int32
	svc := types.AWSService{
		Name: "TestServiceErr",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			atomic.AddInt32(&callCount, 1)
			return nil, awserr.New("InternalServiceError", "something broke", nil)
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			return []types.ScanResult{{ServiceName: "TestServiceErr", Error: err}}
		},
	}

	_, category := scanWithBackoff(context.Background(), svc, nil, false)

	if category != ErrorService {
		t.Errorf("category = %d, want ErrorService (%d)", category, ErrorService)
	}
	if atomic.LoadInt32(&callCount) != 1 {
		t.Errorf("callCount = %d, want 1 (no retry for non-throttle)", atomic.LoadInt32(&callCount))
	}
}

func TestScanWithBackoffAccessDenied(t *testing.T) {
	var callCount int32
	svc := types.AWSService{
		Name: "TestDenied",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			atomic.AddInt32(&callCount, 1)
			return nil, awserr.New("AccessDeniedException", "access denied", nil)
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			return []types.ScanResult{{ServiceName: "TestDenied", Error: err}}
		},
	}

	_, category := scanWithBackoff(context.Background(), svc, nil, false)

	if category != ErrorDenied {
		t.Errorf("category = %d, want ErrorDenied (%d)", category, ErrorDenied)
	}
	if atomic.LoadInt32(&callCount) != 1 {
		t.Errorf("callCount = %d, want 1 (no retry for access denied)", atomic.LoadInt32(&callCount))
	}
}

func TestScanWithBackoffSuccess(t *testing.T) {
	var callCount int32
	svc := types.AWSService{
		Name: "TestSuccess",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			atomic.AddInt32(&callCount, 1)
			return "ok", nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			return []types.ScanResult{{ServiceName: "TestSuccess", ResourceName: "res-1"}}
		},
	}

	results, category := scanWithBackoff(context.Background(), svc, nil, false)

	if category != ErrorNone {
		t.Errorf("category = %d, want ErrorNone (%d)", category, ErrorNone)
	}
	if atomic.LoadInt32(&callCount) != 1 {
		t.Errorf("callCount = %d, want 1 (no retry for success)", atomic.LoadInt32(&callCount))
	}
	if len(results) != 1 || results[0].ResourceName != "res-1" {
		t.Errorf("unexpected results: %+v", results)
	}
}

func TestScanWithBackoffThrottleThenSuccess(t *testing.T) {
	successResults := []types.ScanResult{
		{ServiceName: "TestRetry", ResourceName: "res-1"},
		{ServiceName: "TestRetry", ResourceName: "res-2"},
	}
	svc, callCount := makeThrottleService("TestRetry", 1, successResults)

	results, category := scanWithBackoff(context.Background(), svc, nil, false)

	if category != ErrorNone {
		t.Errorf("category = %d, want ErrorNone (%d)", category, ErrorNone)
	}
	if atomic.LoadInt32(callCount) != 2 {
		t.Errorf("callCount = %d, want 2 (1 throttle + 1 success)", atomic.LoadInt32(callCount))
	}
	if len(results) != 2 {
		t.Fatalf("results count = %d, want 2", len(results))
	}
	if results[0].ResourceName != "res-1" || results[1].ResourceName != "res-2" {
		t.Errorf("unexpected results: %+v", results)
	}
}

func TestScanWithBackoffExhaustedRetries(t *testing.T) {
	// Throttle forever — should exhaust all retries
	svc, callCount := makeThrottleService("TestExhausted", 100, nil)

	results, category := scanWithBackoff(context.Background(), svc, nil, false)

	if category != ErrorThrottle {
		t.Errorf("category = %d, want ErrorThrottle (%d)", category, ErrorThrottle)
	}
	// Should be called backoffMaxRetries + 1 times (initial + retries)
	expectedCalls := int32(backoffMaxRetries + 1)
	if atomic.LoadInt32(callCount) != expectedCalls {
		t.Errorf("callCount = %d, want %d", atomic.LoadInt32(callCount), expectedCalls)
	}
	if len(results) != 1 {
		t.Fatalf("results count = %d, want 1", len(results))
	}
	if results[0].Error == nil {
		t.Fatalf("expected error result, got nil")
	}
}

func TestScanWithBackoffRateLimitedErrorMessage(t *testing.T) {
	svc, _ := makeThrottleService("TestRateMsg", 100, nil)

	results, _ := scanWithBackoff(context.Background(), svc, nil, false)

	if len(results) != 1 {
		t.Fatalf("results count = %d, want 1", len(results))
	}
	if results[0].Error == nil {
		t.Fatalf("expected error, got nil")
	}
	errMsg := results[0].Error.Error()
	if !strings.Contains(errMsg, "rate limited") {
		t.Errorf("error message = %q, want containing %q", errMsg, "rate limited")
	}
}

func TestScanWithBackoffContextCancellation(t *testing.T) {
	// Create a service that always throttles
	svc, _ := makeThrottleService("TestCancel", 100, nil)

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel immediately so the backoff select picks up ctx.Done()
	cancel()

	start := time.Now()
	_, category := scanWithBackoff(ctx, svc, nil, false)
	elapsed := time.Since(start)

	if category != ErrorThrottle {
		t.Errorf("category = %d, want ErrorThrottle (%d)", category, ErrorThrottle)
	}
	// Should return promptly (well under any backoff delay)
	if elapsed > 500*time.Millisecond {
		t.Errorf("scanWithBackoff took %v with cancelled context, expected prompt return", elapsed)
	}
}

func TestScanWithBackoffTransparentSuccess(t *testing.T) {
	// AC #4: After retry, result appears normal — no throttling indication
	successResults := []types.ScanResult{
		{ServiceName: "TestTransparent", ResourceName: "resource-a"},
	}
	svc, _ := makeThrottleService("TestTransparent", 2, successResults)

	results, category := scanWithBackoff(context.Background(), svc, nil, false)

	if category != ErrorNone {
		t.Errorf("category = %d, want ErrorNone (transparent success)", category)
	}
	if len(results) != 1 {
		t.Fatalf("results count = %d, want 1", len(results))
	}
	if results[0].ResourceName != "resource-a" {
		t.Errorf("ResourceName = %q, want %q", results[0].ResourceName, "resource-a")
	}
	// No error should be present — transparent success
	if results[0].Error != nil {
		t.Errorf("expected no error for transparent success, got: %v", results[0].Error)
	}
}

// --- NFR51 total delay test ---

func TestCalculateBackoffTotalDelayWithinNFR51(t *testing.T) {
	// NFR51: maximum total delay per service is 15 seconds.
	// Sum delays for attempts 0, 1, 2 (the 3 sleeps before retries).
	// Run multiple iterations to account for jitter randomness.
	for i := 0; i < 100; i++ {
		var total time.Duration
		for attempt := 0; attempt < backoffMaxRetries; attempt++ {
			total += calculateBackoff(attempt)
		}
		if total > backoffMaxDelay {
			t.Errorf("iteration %d: total backoff delay = %v, exceeds NFR51 cap of %v", i, total, backoffMaxDelay)
		}
	}
}

// --- AC2 integration test: throttled service does not block others ---

func TestWorkerPoolThrottledServiceDoesNotBlockOthers(t *testing.T) {
	// One service throttles once then succeeds; two others succeed immediately.
	// All results should be present.
	var throttleCount int32
	throttleSvc := types.AWSService{
		Name: "ThrottleSvc",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			n := atomic.AddInt32(&throttleCount, 1)
			if n <= 1 {
				return nil, awserr.New("Throttling", "rate exceeded", nil)
			}
			return "ok", nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			if err != nil {
				return []types.ScanResult{{ServiceName: "ThrottleSvc", Error: err}}
			}
			return []types.ScanResult{{ServiceName: "ThrottleSvc", ResourceName: "throttle-res"}}
		},
	}

	fastSvc1 := types.AWSService{
		Name: "FastSvc1",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			return "ok", nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			return []types.ScanResult{{ServiceName: "FastSvc1", ResourceName: "fast-res-1"}}
		},
	}

	fastSvc2 := types.AWSService{
		Name: "FastSvc2",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			return "ok", nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			return []types.ScanResult{{ServiceName: "FastSvc2", ResourceName: "fast-res-2"}}
		},
	}

	svcs := []types.AWSService{throttleSvc, fastSvc1, fastSvc2}
	results, skipped := runWorkerPool(context.Background(), svcs, nil, 3, true, false, nil)

	if len(skipped) != 0 {
		t.Errorf("skipped = %v, want none", skipped)
	}

	// All 3 services should produce results
	found := make(map[string]bool)
	for _, r := range results {
		found[r.ServiceName] = true
	}
	for _, name := range []string{"ThrottleSvc", "FastSvc1", "FastSvc2"} {
		if !found[name] {
			t.Errorf("missing results for %s — throttled service may have blocked others", name)
		}
	}
}
