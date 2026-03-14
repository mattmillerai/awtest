package main

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/aws/aws-sdk-go/aws/session"
)

// mockService is defined in timeout_test.go and reused here.

func TestWorkerPoolConcurrentExecution(t *testing.T) {
	// 10 services each taking 100ms. At concurrency=5, should complete in ~200ms, not 1000ms.
	var svcs []types.AWSService
	for i := 0; i < 10; i++ {
		svcs = append(svcs, mockService(fmt.Sprintf("svc-%02d", i), 100*time.Millisecond))
	}

	ctx := context.Background()
	start := time.Now()
	results, skipped := runWorkerPool(ctx, svcs, nil, 5, true, false, nil)
	elapsed := time.Since(start)

	if elapsed > 500*time.Millisecond {
		t.Errorf("concurrent execution too slow: %v (expected <500ms)", elapsed)
	}
	if len(results) != 10 {
		t.Errorf("results count = %d, want 10", len(results))
	}
	if len(skipped) != 0 {
		t.Errorf("skipped = %v, want none", skipped)
	}
}

func TestWorkerPoolDeterministicOrdering(t *testing.T) {
	// Services with varying delays complete in different order,
	// but results should be sorted by ServiceName.
	svcs := []types.AWSService{
		mockService("Zebra", 50*time.Millisecond),
		mockService("Apple", 10*time.Millisecond),
		mockService("Mango", 30*time.Millisecond),
		mockService("Banana", 20*time.Millisecond),
	}

	ctx := context.Background()
	results, _ := runWorkerPool(ctx, svcs, nil, 4, true, false, nil)

	if len(results) != 4 {
		t.Fatalf("results count = %d, want 4", len(results))
	}

	expected := []string{"Apple", "Banana", "Mango", "Zebra"}
	for i, name := range expected {
		if results[i].ServiceName != name {
			t.Errorf("results[%d].ServiceName = %q, want %q", i, results[i].ServiceName, name)
		}
	}
}

func TestWorkerPoolThreadSafety(t *testing.T) {
	// This test is meaningful when run with -race flag.
	// Multiple workers appending to shared results simultaneously.
	var svcs []types.AWSService
	for i := 0; i < 20; i++ {
		svcs = append(svcs, mockService(fmt.Sprintf("svc-%02d", i), 10*time.Millisecond))
	}

	ctx := context.Background()
	results, skipped := runWorkerPool(ctx, svcs, nil, 10, true, false, nil)

	if len(results) != 20 {
		t.Errorf("results count = %d, want 20", len(results))
	}
	if len(skipped) != 0 {
		t.Errorf("skipped = %v, want none", skipped)
	}
}

func TestWorkerPoolContextCancellation(t *testing.T) {
	// Cancel mid-scan, verify partial results preserved and unstarted services skipped.
	var svcs []types.AWSService
	for i := 0; i < 10; i++ {
		svcs = append(svcs, mockService(fmt.Sprintf("svc-%02d", i), 200*time.Millisecond))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	results, skipped := runWorkerPool(ctx, svcs, nil, 2, true, false, nil)

	// With 2 workers and 200ms services, cancel at 100ms means workers are mid-scan.
	// Some results may be partial (context error), some services may be skipped.
	totalProcessed := len(results) + len(skipped)
	if totalProcessed == 0 {
		t.Error("expected some results or skipped services after cancellation")
	}
	// All services should be accounted for (either result or skipped)
	if totalProcessed > 10 {
		t.Errorf("total processed (%d) exceeds service count (10)", totalProcessed)
	}
}

func TestWorkerPoolGracefulDrainDeadline(t *testing.T) {
	// SlowService takes 5s and ignores context cancellation.
	// FastService is queued but should never start because drain fires.
	slowSvc := types.AWSService{
		Name: "SlowService",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			// Ignores context cancellation to test drain deadline
			time.Sleep(5 * time.Second)
			return "slow-output", nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			return []types.ScanResult{{
				ServiceName: "SlowService",
				MethodName:  "SlowService",
				Timestamp:   time.Now(),
			}}
		},
	}
	fastSvc := mockService("FastService", 1*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	results, skipped := runWorkerPool(ctx, []types.AWSService{slowSvc, fastSvc}, nil, 1, true, false, nil)
	elapsed := time.Since(start)

	// Should complete within ~1.1s (50ms context + 1s drain), not 5s
	if elapsed > 2*time.Second {
		t.Errorf("drain deadline not enforced: took %v (expected <2s)", elapsed)
	}

	// SlowService result should be discarded (still running when drain fires)
	for _, r := range results {
		if r.ServiceName == "SlowService" {
			t.Error("SlowService should not appear in results after drain timeout")
		}
	}

	// At least one service should be skipped or have no result
	total := len(results) + len(skipped)
	if total > 2 {
		t.Errorf("total accounted = %d, want <= 2", total)
	}
}

func TestWorkerPoolConcurrencyOneFallback(t *testing.T) {
	// concurrency=1 through scanServices should produce identical results to sequential.
	svcs := []types.AWSService{
		mockService("B-Service", 10*time.Millisecond),
		mockService("A-Service", 10*time.Millisecond),
	}

	ctx := context.Background()
	// Sequential via scanServices
	seqResults, seqSkipped := scanServices(ctx, svcs, nil, 1, true, false)

	// Sequential should preserve insertion order (not sorted)
	if len(seqResults) != 2 {
		t.Fatalf("sequential results count = %d, want 2", len(seqResults))
	}
	if seqResults[0].ServiceName != "B-Service" {
		t.Errorf("sequential results[0] = %q, want %q", seqResults[0].ServiceName, "B-Service")
	}
	if len(seqSkipped) != 0 {
		t.Errorf("sequential skipped = %v, want none", seqSkipped)
	}
}

func TestWorkerPoolPanickingService(t *testing.T) {
	// A panicking service should not crash the pool — safeScan handles recovery.
	panicSvc := types.AWSService{
		Name: "PanicService",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			panic("boom")
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			return nil
		},
	}
	normalSvc := mockService("NormalService", 10*time.Millisecond)

	ctx := context.Background()
	results, skipped := runWorkerPool(ctx, []types.AWSService{panicSvc, normalSvc}, nil, 2, true, false, nil)

	if len(skipped) != 0 {
		t.Errorf("skipped = %v, want none", skipped)
	}
	if len(results) != 2 {
		t.Fatalf("results count = %d, want 2", len(results))
	}

	// Find the panic result
	var foundPanic, foundNormal bool
	for _, r := range results {
		if r.ServiceName == "PanicService" {
			foundPanic = true
			if r.Error == nil {
				t.Error("PanicService result Error is nil, want non-nil")
			} else if !strings.Contains(r.Error.Error(), "panic recovered") {
				t.Errorf("PanicService error = %q, want containing %q", r.Error.Error(), "panic recovered")
			}
		}
		if r.ServiceName == "NormalService" {
			foundNormal = true
			if r.Error != nil {
				t.Errorf("NormalService error = %v, want nil", r.Error)
			}
		}
	}
	if !foundPanic {
		t.Error("PanicService result not found in results")
	}
	if !foundNormal {
		t.Error("NormalService result not found in results")
	}
}

func TestWorkerPoolEmptyServiceList(t *testing.T) {
	ctx := context.Background()
	results, skipped := runWorkerPool(ctx, []types.AWSService{}, nil, 5, true, false, nil)

	if len(results) != 0 {
		t.Errorf("results count = %d, want 0", len(results))
	}
	if len(skipped) != 0 {
		t.Errorf("skipped count = %d, want 0", len(skipped))
	}
}

func TestWorkerPoolSingleService(t *testing.T) {
	svc := mockService("OnlyService", 10*time.Millisecond)

	ctx := context.Background()
	// Test with various concurrency levels
	for _, conc := range []int{1, 5, 20} {
		t.Run(fmt.Sprintf("concurrency=%d", conc), func(t *testing.T) {
			results, skipped := runWorkerPool(ctx, []types.AWSService{svc}, nil, conc, true, false, nil)

			if len(results) != 1 {
				t.Errorf("results count = %d, want 1", len(results))
			}
			if len(skipped) != 0 {
				t.Errorf("skipped = %v, want none", skipped)
			}
			if len(results) > 0 && results[0].ServiceName != "OnlyService" {
				t.Errorf("ServiceName = %q, want %q", results[0].ServiceName, "OnlyService")
			}
		})
	}
}

func TestWorkerPoolConcurrencyOneMatchesSequential(t *testing.T) {
	// runWorkerPool with concurrency=1 should produce the same results
	// as scanServices with concurrency=1 (just sorted).
	svcs := []types.AWSService{
		mockService("Charlie", 10*time.Millisecond),
		mockService("Alpha", 10*time.Millisecond),
		mockService("Bravo", 10*time.Millisecond),
	}

	ctx := context.Background()
	poolResults, poolSkipped := runWorkerPool(ctx, svcs, nil, 1, true, false, nil)
	seqResults, seqSkipped := scanServices(ctx, svcs, nil, 1, true, false)

	if len(poolResults) != len(seqResults) {
		t.Fatalf("pool results=%d, seq results=%d", len(poolResults), len(seqResults))
	}
	if len(poolSkipped) != len(seqSkipped) {
		t.Errorf("pool skipped=%d, seq skipped=%d", len(poolSkipped), len(seqSkipped))
	}

	// Pool results are sorted; sequential preserves insertion order.
	// Verify pool has same service names (sorted).
	expected := []string{"Alpha", "Bravo", "Charlie"}
	for i, name := range expected {
		if poolResults[i].ServiceName != name {
			t.Errorf("poolResults[%d].ServiceName = %q, want %q", i, poolResults[i].ServiceName, name)
		}
	}
}

func TestScanServicesConcurrentDelegation(t *testing.T) {
	// Verify scanServices delegates to worker pool when concurrency > 1
	svcs := []types.AWSService{
		mockService("Zebra", 10*time.Millisecond),
		mockService("Apple", 10*time.Millisecond),
	}

	ctx := context.Background()
	results, _ := scanServices(ctx, svcs, nil, 2, true, false)

	// Worker pool sorts results — Apple should come before Zebra
	if len(results) != 2 {
		t.Fatalf("results count = %d, want 2", len(results))
	}
	if results[0].ServiceName != "Apple" {
		t.Errorf("results[0].ServiceName = %q, want %q (sorted by worker pool)", results[0].ServiceName, "Apple")
	}
	if results[1].ServiceName != "Zebra" {
		t.Errorf("results[1].ServiceName = %q, want %q", results[1].ServiceName, "Zebra")
	}
}
