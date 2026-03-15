// Exponential backoff for throttled AWS API calls.
// Policy: base 1s, 2x multiplier, ±50% jitter, max 3 retries, 15s cap per service (NFR51).
package main

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/aws/aws-sdk-go/aws/session"
)

// Backoff constants for exponential retry on throttled requests.
const (
	backoffBaseDelay    = 1 * time.Second
	backoffMultiplier   = 2.0
	backoffMaxRetries   = 3
	backoffMaxDelay     = 15 * time.Second
	backoffJitterFactor = 0.5
)

// calculateBackoff returns the backoff delay for the given retry attempt.
// Applies exponential growth with ±50% jitter, capped at backoffMaxDelay.
func calculateBackoff(attempt int) time.Duration {
	// Base delay * 2^attempt
	delay := float64(backoffBaseDelay)
	for i := 0; i < attempt; i++ {
		delay *= backoffMultiplier
	}

	// Apply jitter: multiply by (0.5 + rand in [0, 1.0)) giving range [0.5, 1.5) of computed delay
	jitter := backoffJitterFactor + rand.Float64()*(2*backoffJitterFactor)
	delay *= jitter

	d := time.Duration(delay)
	if d > backoffMaxDelay {
		d = backoffMaxDelay
	}
	return d
}

// scanWithBackoff wraps safeScan with retry logic for throttled requests.
// On ErrorThrottle, retries up to backoffMaxRetries times with exponential backoff.
// All other error categories return immediately without retry.
// Backoff state is local — no global coordination.
func scanWithBackoff(ctx context.Context, service types.AWSService, sess *session.Session, debug bool) ([]types.ScanResult, ErrorCategory) {
	for attempt := 0; attempt <= backoffMaxRetries; attempt++ {
		results, category := safeScan(ctx, service, sess, debug)

		if category != ErrorThrottle {
			return results, category
		}

		// Last attempt exhausted — don't sleep, return rate-limited error
		if attempt == backoffMaxRetries {
			break
		}

		delay := calculateBackoff(attempt)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			// Context cancelled during backoff — return what we have
			return results, category
		case <-timer.C:
			// Continue to next retry
		}
	}

	// All retries exhausted — return rate-limited error
	return []types.ScanResult{{
		ServiceName: service.Name,
		MethodName:  service.Name,
		Error:       fmt.Errorf("service rate limited after %d retries", backoffMaxRetries),
		Timestamp:   time.Now(),
	}}, ErrorThrottle
}
