package formatters

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/MillerMedia/awtest/cmd/awtest/types"
)

// FormatWithSummary formats CSV with summary info as header comment rows.
func (f *CSVFormatter) FormatWithSummary(results []types.ScanResult, summary types.ScanSummary) (string, error) {
	var buf bytes.Buffer

	// Write summary as comment header
	fmt.Fprintf(&buf, "# Scan Summary\n")
	fmt.Fprintf(&buf, "# Timestamp: %s\n", summary.Timestamp.Format(time.RFC3339))
	fmt.Fprintf(&buf, "# Duration: %s\n", summary.ScanDuration)
	fmt.Fprintf(&buf, "# Total Services: %d\n", summary.TotalServices)
	fmt.Fprintf(&buf, "# Accessible: %d\n", summary.AccessibleServices)
	fmt.Fprintf(&buf, "# Access Denied: %d\n", summary.AccessDeniedServices)
	fmt.Fprintf(&buf, "# Resources Found: %d\n", summary.TotalResources)

	// Write CSV data
	writer := csv.NewWriter(&buf)
	header := []string{"Service", "Method", "ResourceType", "ResourceName", "Details", "Timestamp", "Error"}
	if err := writer.Write(header); err != nil {
		return "", fmt.Errorf("csv formatting failed: %w", err)
	}

	for _, r := range results {
		details, detailsErr := flattenDetails(r.Details)
		errStr := formatCSVError(r, detailsErr)
		record := []string{
			r.ServiceName,
			r.MethodName,
			r.ResourceType,
			r.ResourceName,
			details,
			r.Timestamp.Format(time.RFC3339),
			errStr,
		}
		if err := writer.Write(record); err != nil {
			return "", fmt.Errorf("csv formatting failed: %w", err)
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", fmt.Errorf("csv formatting failed: %w", err)
	}
	return buf.String(), nil
}

// CSVFormatter formats scan results as CSV output.
type CSVFormatter struct{}

// NewCSVFormatter creates a new CSVFormatter instance.
func NewCSVFormatter() *CSVFormatter {
	return &CSVFormatter{}
}

// Format converts scan results to CSV for spreadsheet import and analysis.
func (f *CSVFormatter) Format(results []types.ScanResult) (string, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// Write header
	header := []string{"Service", "Method", "ResourceType", "ResourceName", "Details", "Timestamp", "Error"}
	if err := writer.Write(header); err != nil {
		return "", fmt.Errorf("csv formatting failed: %w", err)
	}

	// Write data rows
	for _, r := range results {
		details, detailsErr := flattenDetails(r.Details)
		errStr := formatCSVError(r, detailsErr)

		record := []string{
			r.ServiceName,
			r.MethodName,
			r.ResourceType,
			r.ResourceName,
			details,
			r.Timestamp.Format(time.RFC3339),
			errStr,
		}
		if err := writer.Write(record); err != nil {
			return "", fmt.Errorf("csv formatting failed: %w", err)
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", fmt.Errorf("csv formatting failed: %w", err)
	}
	return buf.String(), nil
}

// FileExtension returns "csv" for CSV formatted output.
func (f *CSVFormatter) FileExtension() string {
	return "csv"
}

// flattenDetails converts a map[string]interface{} to a semicolon-separated key=value string.
// Keys are sorted alphabetically for deterministic output.
// Returns the flattened string and an error string if serialization validation fails.
func flattenDetails(details map[string]interface{}) (string, string) {
	if details == nil || len(details) == 0 {
		return "", ""
	}

	// Validate serializability using json.Marshal (same approach as JSON formatter)
	if _, err := json.Marshal(details); err != nil {
		return "", fmt.Sprintf("details serialization error: %v", err)
	}

	keys := make([]string, 0, len(details))
	for k := range details {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, fmt.Sprintf("%s=%v", k, details[k]))
	}
	return strings.Join(pairs, ";"), ""
}

// formatCSVError builds the error column string, combining scan errors and serialization errors.
func formatCSVError(r types.ScanResult, detailsErr string) string {
	if r.Error == nil && detailsErr == "" {
		return ""
	}
	if r.Error != nil && detailsErr != "" {
		return r.Error.Error() + "; " + detailsErr
	}
	if r.Error != nil {
		return r.Error.Error()
	}
	return detailsErr
}
