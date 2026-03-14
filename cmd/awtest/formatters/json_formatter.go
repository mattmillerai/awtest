package formatters

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/MillerMedia/awtest/cmd/awtest/types"
)

// jsonScanResult is the JSON-serializable representation of a ScanResult.
// Uses camelCase field names per Go JSON conventions and NFR20.
type jsonScanResult struct {
	ServiceName  string                 `json:"serviceName"`
	MethodName   string                 `json:"methodName"`
	ResourceType string                 `json:"resourceType"`
	ResourceName string                 `json:"resourceName"`
	Details      map[string]interface{} `json:"details"`
	Error        string                 `json:"error,omitempty"`
	Timestamp    string                 `json:"timestamp"`
}

// JSONFormatter formats scan results as JSON output.
type JSONFormatter struct{}

// NewJSONFormatter creates a new JSONFormatter instance.
func NewJSONFormatter() *JSONFormatter {
	return &JSONFormatter{}
}

// Format converts scan results to compact JSON for SIEM and machine consumption.
func (f *JSONFormatter) Format(results []types.ScanResult) (string, error) {
	jsonResults := make([]jsonScanResult, 0, len(results))
	for _, r := range results {
		jr := jsonScanResult{
			ServiceName:  r.ServiceName,
			MethodName:   r.MethodName,
			ResourceType: r.ResourceType,
			ResourceName: r.ResourceName,
			Details:      map[string]interface{}{},
			Timestamp:    r.Timestamp.Format(time.RFC3339),
		}
		if r.Details != nil {
			if _, err := json.Marshal(r.Details); err == nil {
				jr.Details = r.Details
			} else {
				jr.Error = fmt.Sprintf("details serialization error: %v", err)
			}
		}
		if r.Error != nil {
			if jr.Error != "" {
				jr.Error = r.Error.Error() + "; " + jr.Error
			} else {
				jr.Error = r.Error.Error()
			}
		}
		jsonResults = append(jsonResults, jr)
	}

	data, err := json.Marshal(jsonResults)
	if err != nil {
		return "", fmt.Errorf("json formatting failed: %w", err)
	}
	return string(data), nil
}

// jsonMetadata is the metadata envelope for JSON output with summary.
type jsonMetadata struct {
	Timestamp            string   `json:"timestamp"`
	Duration             string   `json:"duration"`
	TotalServices        int      `json:"totalServices"`
	AccessibleServices   int      `json:"accessibleServices"`
	AccessDeniedServices int      `json:"accessDeniedServices"`
	TotalResources       int      `json:"totalResources"`
	AccessibleMethodNames []string `json:"accessibleMethodNames,omitempty"`
}

// jsonEnvelope wraps results with metadata for structured output.
type jsonEnvelope struct {
	Metadata jsonMetadata     `json:"metadata"`
	Results  []jsonScanResult `json:"results"`
}

// FormatWithSummary wraps JSON results in a metadata envelope.
func (f *JSONFormatter) FormatWithSummary(results []types.ScanResult, summary types.ScanSummary) (string, error) {
	jsonResults := make([]jsonScanResult, 0, len(results))
	for _, r := range results {
		jr := jsonScanResult{
			ServiceName:  r.ServiceName,
			MethodName:   r.MethodName,
			ResourceType: r.ResourceType,
			ResourceName: r.ResourceName,
			Details:      map[string]interface{}{},
			Timestamp:    r.Timestamp.Format(time.RFC3339),
		}
		if r.Details != nil {
			if _, err := json.Marshal(r.Details); err == nil {
				jr.Details = r.Details
			} else {
				jr.Error = fmt.Sprintf("details serialization error: %v", err)
			}
		}
		if r.Error != nil {
			if jr.Error != "" {
				jr.Error = r.Error.Error() + "; " + jr.Error
			} else {
				jr.Error = r.Error.Error()
			}
		}
		jsonResults = append(jsonResults, jr)
	}

	envelope := jsonEnvelope{
		Metadata: jsonMetadata{
			Timestamp:             summary.Timestamp.Format(time.RFC3339),
			Duration:              summary.ScanDuration.String(),
			TotalServices:         summary.TotalServices,
			AccessibleServices:    summary.AccessibleServices,
			AccessDeniedServices:  summary.AccessDeniedServices,
			TotalResources:        summary.TotalResources,
			AccessibleMethodNames: summary.AccessibleMethodNames,
		},
		Results: jsonResults,
	}

	data, err := json.Marshal(envelope)
	if err != nil {
		return "", fmt.Errorf("json formatting failed: %w", err)
	}
	return string(data), nil
}

// FileExtension returns "json" for JSON formatted output.
func (f *JSONFormatter) FileExtension() string {
	return "json"
}
