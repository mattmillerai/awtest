package formatters

import (
	"bytes"
	"fmt"
	"time"

	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/olekukonko/tablewriter"
)

// Column width limits to enforce 120-char total table width.
// Total budget: 120 - 16 (borders/padding for 5 columns) = 104 chars of content.
const (
	colWidthService      = 16
	colWidthMethod       = 24
	colWidthResourceType = 16
	colWidthResourceName = 26
	colWidthTimestamp     = 20
)

// TableFormatter formats scan results as ASCII tables for terminal display.
type TableFormatter struct{}

// NewTableFormatter creates a new TableFormatter instance.
func NewTableFormatter() *TableFormatter {
	return &TableFormatter{}
}

// Compile-time interface compliance check.
var _ OutputFormatter = (*TableFormatter)(nil)

// Format converts scan results to an ASCII table string.
func (f *TableFormatter) Format(results []types.ScanResult) (string, error) {
	if len(results) == 0 {
		return "No results found", nil
	}

	var buf bytes.Buffer
	table := tablewriter.NewWriter(&buf)
	table.SetHeader([]string{"Service", "Method", "Resource Type", "Resource Name", "Timestamp"})
	table.SetBorder(true)
	table.SetAutoWrapText(false)
	table.SetRowLine(false)

	for _, r := range results {
		resourceName := r.ResourceName
		if r.Error != nil {
			resourceName = resourceName + " [ERROR: " + r.Error.Error() + "]"
		}
		table.Append([]string{
			truncateColumn(r.ServiceName, colWidthService),
			truncateColumn(r.MethodName, colWidthMethod),
			truncateColumn(r.ResourceType, colWidthResourceType),
			truncateColumn(resourceName, colWidthResourceName),
			r.Timestamp.Format(time.RFC3339),
		})
	}

	table.Render()
	return buf.String(), nil
}

// FormatWithSummary formats results as a table with a summary section appended.
func (f *TableFormatter) FormatWithSummary(results []types.ScanResult, summary types.ScanSummary) (string, error) {
	tableOutput, err := f.Format(results)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	buf.WriteString(tableOutput)
	fmt.Fprintf(&buf, "\n========================================\n")
	fmt.Fprintf(&buf, "Scan Summary\n")
	fmt.Fprintf(&buf, "========================================\n")
	fmt.Fprintf(&buf, "Timestamp:          %s\n", summary.Timestamp.Format(time.RFC3339))
	fmt.Fprintf(&buf, "Duration:           %s\n", summary.ScanDuration)
	fmt.Fprintf(&buf, "Total Services:     %d\n", summary.TotalServices)
	fmt.Fprintf(&buf, "Accessible:         %d\n", summary.AccessibleServices)
	fmt.Fprintf(&buf, "Access Denied:      %d\n", summary.AccessDeniedServices)
	fmt.Fprintf(&buf, "Resources Found:    %d\n", summary.TotalResources)
	methodLines := types.FormatAccessibleMethods(summary.AccessibleMethodNames, func(name string) string {
		return name
	})
	for _, line := range methodLines {
		fmt.Fprintf(&buf, "%s\n", line)
	}
	fmt.Fprintf(&buf, "========================================\n")

	return buf.String(), nil
}

// FileExtension returns "txt" for table formatted output.
func (f *TableFormatter) FileExtension() string {
	return "txt"
}

// truncateColumn shortens a string to maxLen, appending "..." if truncated.
func truncateColumn(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
