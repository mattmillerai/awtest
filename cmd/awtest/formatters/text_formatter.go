package formatters

import (
	"fmt"
	"strings"
	"time"

	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/MillerMedia/awtest/cmd/awtest/utils"
)

// TextFormatter formats scan results as colorized text output,
// replicating the existing terminal output produced by utils.PrintResult().
type TextFormatter struct{}

// NewTextFormatter creates a new TextFormatter instance.
func NewTextFormatter() *TextFormatter {
	return &TextFormatter{}
}

// Compile-time interface compliance check.
var _ OutputFormatter = (*TextFormatter)(nil)

// Format converts scan results to colorized text output.
// Each result is formatted as: [ServiceName] [MethodName] [severity] ResourceType: ResourceName
func (f *TextFormatter) Format(results []types.ScanResult) (string, error) {
	if len(results) == 0 {
		return "No results found", nil
	}

	var lines []string
	for _, r := range results {
		moduleName := r.ServiceName
		if moduleName == "" {
			moduleName = types.DefaultModuleName
		}

		severity := utils.DetermineSeverity(r.Error)

		var resultStr string
		if r.Error != nil {
			resultStr = fmt.Sprintf("Error: %s", r.Error.Error())
		} else if r.ResourceType != "" && r.ResourceName != "" {
			resultStr = fmt.Sprintf("%s: %s", r.ResourceType, r.ResourceName)
		} else if r.ResourceName != "" {
			resultStr = r.ResourceName
		} else if r.ResourceType != "" {
			resultStr = r.ResourceType
		} else {
			resultStr = "No details"
		}

		line := utils.ColorizeMessage(moduleName, r.MethodName, severity, resultStr)
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n"), nil
}

// FormatWithSummary formats results as text with a summary section appended.
func (f *TextFormatter) FormatWithSummary(results []types.ScanResult, summary types.ScanSummary) (string, error) {
	textOutput, err := f.Format(results)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString(textOutput)
	sb.WriteString("\n========================================\n")
	fmt.Fprintf(&sb, "Scan Summary\n")
	sb.WriteString("========================================\n")
	fmt.Fprintf(&sb, "Timestamp:          %s\n", summary.Timestamp.Format(time.RFC3339))
	fmt.Fprintf(&sb, "Duration:           %s\n", summary.ScanDuration)
	fmt.Fprintf(&sb, "Total Services:     %d\n", summary.TotalServices)
	fmt.Fprintf(&sb, "Accessible:         %d\n", summary.AccessibleServices)
	fmt.Fprintf(&sb, "Access Denied:      %d\n", summary.AccessDeniedServices)
	fmt.Fprintf(&sb, "Resources Found:    %d\n", summary.TotalResources)
	methodLines := types.FormatAccessibleMethods(summary.AccessibleMethodNames, func(name string) string {
		return name
	})
	for _, line := range methodLines {
		fmt.Fprintf(&sb, "%s\n", line)
	}
	sb.WriteString("========================================\n")

	return sb.String(), nil
}

// FileExtension returns "txt" for text formatted output.
func (f *TextFormatter) FileExtension() string {
	return "txt"
}
