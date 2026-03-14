package services

import (
	"fmt"
	"os"
	"strings"

	"github.com/MillerMedia/awtest/cmd/awtest/types"
)

// FilterServices filters the given service list based on include/exclude patterns.
// Include filter is applied first, then exclude filter removes from the included set.
// Service names are matched case-insensitively with partial matching support.
func FilterServices(allServices []types.AWSService, include, exclude string) []types.AWSService {
	includeMap := parseServiceList(include)
	excludeMap := parseServiceList(exclude)

	// Warn about unrecognized service names
	if len(includeMap) > 0 || len(excludeMap) > 0 {
		knownPrefixes := make(map[string]bool)
		for _, svc := range allServices {
			knownPrefixes[extractServiceName(svc.Name)] = true
		}
		warnUnrecognized(includeMap, knownPrefixes)
		warnUnrecognized(excludeMap, knownPrefixes)
	}

	// If no filters, return all services
	if len(includeMap) == 0 && len(excludeMap) == 0 {
		return allServices
	}

	var result []types.AWSService

	// Apply include filter first (or start with all if no include filter)
	if len(includeMap) > 0 {
		for _, svc := range allServices {
			prefix := extractServiceName(svc.Name)
			if matchesFilter(prefix, includeMap) {
				result = append(result, svc)
			}
		}
	} else {
		result = make([]types.AWSService, len(allServices))
		copy(result, allServices)
	}

	// Apply exclude filter
	if len(excludeMap) > 0 {
		var filtered []types.AWSService
		for _, svc := range result {
			prefix := extractServiceName(svc.Name)
			if !matchesFilter(prefix, excludeMap) {
				filtered = append(filtered, svc)
			}
		}
		result = filtered
	}

	return result
}

// parseServiceList splits a comma-separated string into a map of lowercased, trimmed service names.
func parseServiceList(csv string) map[string]bool {
	result := make(map[string]bool)
	if strings.TrimSpace(csv) == "" {
		return result
	}
	parts := strings.Split(csv, ",")
	for _, part := range parts {
		name := strings.ToLower(strings.TrimSpace(part))
		if name != "" {
			result[name] = true
		}
	}
	return result
}

// extractServiceName extracts the service prefix from an AWSService.Name field.
// For example, "s3:ListBuckets" returns "s3", "ec2:DescribeInstances" returns "ec2".
func extractServiceName(callName string) string {
	idx := strings.Index(callName, ":")
	if idx == -1 {
		return strings.ToLower(callName)
	}
	return strings.ToLower(callName[:idx])
}

// matchesFilter checks if a service prefix matches any filter entry.
// Supports both exact and partial (substring) matching.
func matchesFilter(prefix string, filterMap map[string]bool) bool {
	for filterName := range filterMap {
		if prefix == filterName || strings.Contains(prefix, filterName) {
			return true
		}
	}
	return false
}

// warnUnrecognized prints warnings for filter names that don't match any known service prefix.
func warnUnrecognized(filterMap map[string]bool, knownPrefixes map[string]bool) {
	for filterName := range filterMap {
		found := false
		for known := range knownPrefixes {
			if known == filterName || strings.Contains(known, filterName) {
				found = true
				break
			}
		}
		if !found {
			fmt.Fprintf(os.Stderr, "Warning: unrecognized service name '%s'\n", filterName)
		}
	}
}
