package utils

import (
	"sync"
)

var (
	dedupeMu         sync.Mutex
	dedupeEnabled    bool
	dedupeSeen       map[string]struct{}
	dedupeSuppressed int
)

// SetErrorDedupe enables or disables de-duplication of repeated
// HandleAWSError lines (same call and error code) and resets its state.
func SetErrorDedupe(enabled bool) {
	dedupeMu.Lock()
	defer dedupeMu.Unlock()
	dedupeEnabled = enabled
	dedupeSeen = make(map[string]struct{})
	dedupeSuppressed = 0
}

// SuppressedErrorLines returns how many error lines were suppressed as repeats.
func SuppressedErrorLines() int {
	dedupeMu.Lock()
	defer dedupeMu.Unlock()
	return dedupeSuppressed
}

// suppressRepeatedError reports whether the error line for callName/err has
// already been printed and should be skipped. Repeats are counted.
func suppressRepeatedError(debug bool, callName string, err error) bool {
	if debug {
		return false
	}
	dedupeMu.Lock()
	defer dedupeMu.Unlock()
	if !dedupeEnabled {
		return false
	}
	code := "error"
	if c, _, ok := AWSErrorInfo(err); ok {
		code = c
	}
	key := callName + "|" + code
	if _, seen := dedupeSeen[key]; seen {
		dedupeSuppressed++
		return true
	}
	dedupeSeen[key] = struct{}{}
	return false
}
