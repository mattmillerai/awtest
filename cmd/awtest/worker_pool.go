package main

import (
	"context"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/aws/aws-sdk-go/aws/session"
)

// runWorkerPool executes services concurrently using a fixed worker pool.
// Returns collected results (sorted by ServiceName) and skipped service names.
//
// If concurrency < 1, it defaults to 1 to avoid deadlock from zero workers.
func runWorkerPool(ctx context.Context, svcs []types.AWSService, sess *session.Session, concurrency int, quiet, debug bool, progress *progressReporter) ([]types.ScanResult, []string) {
	if concurrency < 1 {
		concurrency = 1
	}

	var (
		results []types.ScanResult
		skipped []string
		mu      sync.Mutex
		wg      sync.WaitGroup
		drained int32 // atomic flag: set to 1 after drain timeout, workers stop appending
	)

	// Buffered channel sized to total services — non-blocking submit
	jobs := make(chan types.AWSService, len(svcs))

	// Spawn workers
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for service := range jobs {
				// If drain timeout fired, skip remaining work
				if atomic.LoadInt32(&drained) == 1 {
					continue
				}

				// Check context before starting each service
				select {
				case <-ctx.Done():
					if atomic.LoadInt32(&drained) == 1 {
						continue
					}
					mu.Lock()
					skipped = append(skipped, service.Name)
					mu.Unlock()
					continue
				default:
				}

				serviceResults, _ := scanWithBackoff(ctx, service, sess, debug)

				// Only append if drain hasn't fired
				if atomic.LoadInt32(&drained) == 0 {
					mu.Lock()
					results = append(results, serviceResults...)
					mu.Unlock()
				}

				progress.Increment()
			}
		}()
	}

	// Submit all services
	for _, svc := range svcs {
		jobs <- svc
	}
	close(jobs)

	// Wait for workers with drain deadline on context cancellation
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All workers completed normally
	case <-ctx.Done():
		// Context cancelled — give workers 1s to finish current work
		select {
		case <-done:
			// Workers finished within drain window
		case <-time.After(1 * time.Second):
			// Drain timeout expired. Set flag so workers stop appending,
			// then collect remaining unprocessed jobs as skipped.
			// After this point we intentionally do not wait for workers —
			// they will finish on their own but their results are discarded.
			atomic.StoreInt32(&drained, 1)

			// Drain remaining jobs from the (already closed) channel into
			// a local slice first, then append under the lock once.
			var drainedNames []string
			for svc := range jobs {
				drainedNames = append(drainedNames, svc.Name)
			}
			if len(drainedNames) > 0 {
				mu.Lock()
				skipped = append(skipped, drainedNames...)
				mu.Unlock()
			}
		}
	}

	// At this point either all workers finished or the drain flag is set,
	// so no more appends to results/skipped can happen. Safe to sort.
	sort.Slice(results, func(i, j int) bool {
		return results[i].ServiceName < results[j].ServiceName
	})

	return results, skipped
}
