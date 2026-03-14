package formatters

import "github.com/MillerMedia/awtest/cmd/awtest/types"

// OutputFormatter defines the interface for all output formatters.
// Implementations of this interface can format scan results into various output formats
// such as JSON, YAML, CSV, or table formats.
type OutputFormatter interface {
	// Format takes scan results and returns formatted output string.
	// Returns an error if formatting fails.
	Format(results []types.ScanResult) (string, error)

	// FormatWithSummary formats scan results with an accompanying scan summary.
	// For JSON/YAML, wraps results in a metadata envelope.
	// For CSV/Table/Text, appends summary after the data.
	FormatWithSummary(results []types.ScanResult, summary types.ScanSummary) (string, error)

	// FileExtension returns the file extension for this format (e.g., "json", "yaml", "txt").
	FileExtension() string
}
