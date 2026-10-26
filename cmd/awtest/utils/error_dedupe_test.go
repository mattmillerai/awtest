package utils

import (
	"errors"
	"sync"
	"testing"

	"github.com/aws/smithy-go"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
)

// captureLines runs fn with output buffered (ConcurrentMode) and returns the
// number of lines that were printed.
func captureLines(t *testing.T, fn func()) int {
	t.Helper()
	outputMu.Lock()
	outputBuffer = nil
	outputMu.Unlock()
	prevMode, prevQuiet := ConcurrentMode, Quiet
	ConcurrentMode, Quiet = true, false
	t.Cleanup(func() {
		ConcurrentMode, Quiet = prevMode, prevQuiet
		outputMu.Lock()
		outputBuffer = nil
		outputMu.Unlock()
		SetErrorDedupe(false)
	})
	fn()
	outputMu.Lock()
	defer outputMu.Unlock()
	return len(outputBuffer)
}

type dedupeCall struct {
	debug bool
	name  string
	err   error
}

func TestHandleAWSErrorDedupe(t *testing.T) {
	denied := &smithy.GenericAPIError{Code: "AccessDenied", Message: "nope"}
	throttled := &smithy.GenericAPIError{Code: "Throttling", Message: "slow"}
	plain := errors.New("boom")

	tests := []struct {
		name           string
		enabled        bool
		calls          []dedupeCall
		wantPrinted    int
		wantSuppressed int
	}{
		{"disabled prints every time", false,
			[]dedupeCall{{false, "a", denied}, {false, "a", denied}, {false, "a", denied}}, 3, 0},
		{"enabled prints once per call and code", true,
			[]dedupeCall{{false, "a", denied}, {false, "a", denied}, {false, "a", denied}}, 1, 2},
		{"different call prints again", true,
			[]dedupeCall{{false, "a", denied}, {false, "b", denied}, {false, "b", denied}}, 2, 1},
		{"different code prints again", true,
			[]dedupeCall{{false, "a", denied}, {false, "a", throttled}, {false, "a", throttled}}, 2, 1},
		{"non-awserr keyed as error", true,
			[]dedupeCall{{false, "a", plain}, {false, "a", errors.New("other")}}, 1, 1},
		{"debug never suppresses", true,
			[]dedupeCall{{true, "a", denied}, {true, "a", denied}}, 2, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			SetErrorDedupe(tc.enabled)
			printed := captureLines(t, func() {
				for _, c := range tc.calls {
					if got := HandleAWSError(c.debug, c.name, c.err); got != nil {
						t.Errorf("unexpected return %v", got)
					}
				}
			})
			if printed != tc.wantPrinted {
				t.Errorf("printed = %d, want %d", printed, tc.wantPrinted)
			}
			if got := SuppressedErrorLines(); got != tc.wantSuppressed {
				t.Errorf("suppressed = %d, want %d", got, tc.wantSuppressed)
			}
		})
	}
}

func TestHandleAWSErrorDedupeStillReturnsInvalidKey(t *testing.T) {
	SetErrorDedupe(true)
	err := &smithy.GenericAPIError{Code: types.InvalidClientTokenId, Message: "bad token"}
	printed := captureLines(t, func() {
		for i := 0; i < 3; i++ {
			got := HandleAWSError(false, "a", err)
			if _, ok := got.(*types.InvalidKeyError); !ok {
				t.Fatalf("call %d: got %T, want *types.InvalidKeyError", i, got)
			}
		}
	})
	if printed != 1 {
		t.Errorf("printed = %d, want 1", printed)
	}
	if got := SuppressedErrorLines(); got != 2 {
		t.Errorf("suppressed = %d, want 2", got)
	}
}

func TestHandleAWSErrorDedupeConcurrent(t *testing.T) {
	SetErrorDedupe(true)
	err := &smithy.GenericAPIError{Code: "AccessDenied", Message: "nope"}
	const goroutines, perG = 8, 25
	printed := captureLines(t, func() {
		var wg sync.WaitGroup
		for i := 0; i < goroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < perG; j++ {
					HandleAWSError(false, "a", err)
				}
			}()
		}
		wg.Wait()
	})
	if printed != 1 {
		t.Errorf("printed = %d, want 1", printed)
	}
	if got := SuppressedErrorLines(); got != goroutines*perG-1 {
		t.Errorf("suppressed = %d, want %d", got, goroutines*perG-1)
	}
}

func TestSetErrorDedupeResets(t *testing.T) {
	SetErrorDedupe(true)
	err := &smithy.GenericAPIError{Code: "AccessDenied", Message: "nope"}
	captureLines(t, func() {
		HandleAWSError(false, "a", err)
		HandleAWSError(false, "a", err)
	})
	SetErrorDedupe(true)
	if got := SuppressedErrorLines(); got != 0 {
		t.Errorf("suppressed after reset = %d, want 0", got)
	}
}
