package types

import (
	"fmt"
	"sort"
	"time"
)

// MaxAccessibleMethodsInSummary caps the number of accessible method names
// displayed in text/table summaries to avoid overwhelming output.
const MaxAccessibleMethodsInSummary = 20

// ScanSummary holds aggregate statistics about a completed scan.
type ScanSummary struct {
	TotalServices        int
	AccessibleServices   int
	AccessDeniedServices int
	TotalResources       int
	AccessibleMethodNames []string
	ScanDuration         time.Duration
	Timestamp            time.Time
}

// GenerateSummary computes aggregate scan statistics from results.
// A service can appear in both accessible and denied counts if it has
// mixed success/error results (e.g., S3 ListBuckets succeeds but ListObjects fails).
func GenerateSummary(results []ScanResult, startTime time.Time) ScanSummary {
	serviceMap := make(map[string]bool)
	accessibleMap := make(map[string]bool)
	deniedMap := make(map[string]bool)
	methodSet := make(map[string]bool)
	resourceCount := 0

	for _, r := range results {
		serviceMap[r.ServiceName] = true
		if r.Error != nil {
			deniedMap[r.ServiceName] = true
		} else {
			accessibleMap[r.ServiceName] = true
			resourceCount++
			if r.MethodName != "" {
				methodSet[r.MethodName] = true
			}
		}
	}

	var methodNames []string
	for name := range methodSet {
		methodNames = append(methodNames, name)
	}
	sort.Strings(methodNames)

	return ScanSummary{
		TotalServices:         len(serviceMap),
		AccessibleServices:    len(accessibleMap),
		AccessDeniedServices:  len(deniedMap),
		TotalResources:        resourceCount,
		AccessibleMethodNames: methodNames,
		ScanDuration:          time.Since(startTime).Truncate(time.Millisecond),
		Timestamp:             startTime,
	}
}

// FormatAccessibleMethods returns the accessible methods section lines for summary display.
// The formatName parameter allows callers to wrap method names (e.g., with ANSI coloring).
// Returns nil if there are no accessible methods.
func FormatAccessibleMethods(methods []string, formatName func(string) string) []string {
	if len(methods) == 0 {
		return nil
	}
	var lines []string
	lines = append(lines, "Accessible Methods:")
	limit := len(methods)
	if limit > MaxAccessibleMethodsInSummary {
		limit = MaxAccessibleMethodsInSummary
	}
	for _, name := range methods[:limit] {
		lines = append(lines, fmt.Sprintf("  - %s", formatName(name)))
	}
	if remaining := len(methods) - limit; remaining > 0 {
		lines = append(lines, fmt.Sprintf("  ... (%d more - use --format=json for full list)", remaining))
	}
	return lines
}
