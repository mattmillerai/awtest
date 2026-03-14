package formatters

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/MillerMedia/awtest/cmd/awtest/types"
)

func TestJSONFormatter_Format(t *testing.T) {
	formatter := NewJSONFormatter()
	fixedTime := time.Date(2026, 3, 2, 14, 30, 0, 0, time.UTC)

	tests := []struct {
		name      string
		results   []types.ScanResult
		wantErr   bool
		checkFunc func(t *testing.T, output string)
	}{
		{
			name: "single valid result",
			results: []types.ScanResult{
				{
					ServiceName:  "S3",
					MethodName:   "s3:ListBuckets",
					ResourceType: "bucket",
					ResourceName: "test-bucket",
					Details:      map[string]interface{}{"region": "us-east-1"},
					Timestamp:    fixedTime,
				},
			},
			wantErr: false,
			checkFunc: func(t *testing.T, output string) {
				var parsed []jsonScanResult
				if err := json.Unmarshal([]byte(output), &parsed); err != nil {
					t.Fatalf("invalid JSON: %v", err)
				}
				if len(parsed) != 1 {
					t.Fatalf("expected 1 result, got %d", len(parsed))
				}
				if parsed[0].ServiceName != "S3" {
					t.Errorf("expected S3, got %s", parsed[0].ServiceName)
				}
				if !strings.Contains(output, "serviceName") {
					t.Error("missing camelCase serviceName")
				}
				if !strings.Contains(output, "methodName") {
					t.Error("missing camelCase methodName")
				}
			},
		},
		{
			name:    "empty results returns empty array",
			results: []types.ScanResult{},
			wantErr: false,
			checkFunc: func(t *testing.T, output string) {
				if strings.TrimSpace(output) != "[]" {
					t.Errorf("expected [], got %s", output)
				}
			},
		},
		{
			name: "result with error",
			results: []types.ScanResult{
				{
					ServiceName: "RDS",
					MethodName:  "rds:DescribeDBInstances",
					Error:       errors.New("AccessDeniedException: not authorized"),
					Timestamp:   fixedTime,
				},
			},
			wantErr: false,
			checkFunc: func(t *testing.T, output string) {
				var parsed []jsonScanResult
				if err := json.Unmarshal([]byte(output), &parsed); err != nil {
					t.Fatalf("invalid JSON: %v", err)
				}
				if parsed[0].Error != "AccessDeniedException: not authorized" {
					t.Errorf("error not serialized correctly: %s", parsed[0].Error)
				}
			},
		},
		{
			name: "timestamp in RFC3339 format",
			results: []types.ScanResult{
				{ServiceName: "EC2", Timestamp: fixedTime},
			},
			wantErr: false,
			checkFunc: func(t *testing.T, output string) {
				if !strings.Contains(output, "2026-03-02T14:30:00Z") {
					t.Error("timestamp not in RFC3339 format")
				}
			},
		},
		{
			name: "multiple results",
			results: []types.ScanResult{
				{ServiceName: "S3", MethodName: "s3:ListBuckets", ResourceName: "bucket-1", Timestamp: fixedTime},
				{ServiceName: "EC2", MethodName: "ec2:DescribeInstances", ResourceName: "i-123", Timestamp: fixedTime},
				{ServiceName: "IAM", MethodName: "iam:ListUsers", ResourceName: "admin", Timestamp: fixedTime},
			},
			wantErr: false,
			checkFunc: func(t *testing.T, output string) {
				var parsed []jsonScanResult
				if err := json.Unmarshal([]byte(output), &parsed); err != nil {
					t.Fatalf("invalid JSON: %v", err)
				}
				if len(parsed) != 3 {
					t.Fatalf("expected 3 results, got %d", len(parsed))
				}
			},
		},
		{
			name: "nil details serialized as empty object",
			results: []types.ScanResult{
				{ServiceName: "Lambda", Timestamp: fixedTime},
			},
			wantErr: false,
			checkFunc: func(t *testing.T, output string) {
				var parsed []jsonScanResult
				if err := json.Unmarshal([]byte(output), &parsed); err != nil {
					t.Fatalf("invalid JSON: %v", err)
				}
				if parsed[0].Details == nil {
					t.Error("nil details should serialize as empty object, not null")
				}
				if len(parsed[0].Details) != 0 {
					t.Errorf("expected empty details map, got %v", parsed[0].Details)
				}
			},
		},
		{
			name: "successful result has no error field in JSON",
			results: []types.ScanResult{
				{
					ServiceName:  "S3",
					MethodName:   "s3:ListBuckets",
					ResourceType: "bucket",
					ResourceName: "test-bucket",
					Timestamp:    fixedTime,
				},
			},
			wantErr: false,
			checkFunc: func(t *testing.T, output string) {
				if strings.Contains(output, `"error"`) {
					t.Error("successful result should not contain error field (omitempty)")
				}
			},
		},
		{
			name: "camelCase field naming verified",
			results: []types.ScanResult{
				{
					ServiceName:  "S3",
					MethodName:   "s3:ListBuckets",
					ResourceType: "bucket",
					ResourceName: "test-bucket",
					Details:      map[string]interface{}{"key": "value"},
					Timestamp:    fixedTime,
				},
			},
			wantErr: false,
			checkFunc: func(t *testing.T, output string) {
				expectedFields := []string{"serviceName", "methodName", "resourceType", "resourceName", "details", "timestamp"}
				for _, field := range expectedFields {
					if !strings.Contains(output, `"`+field+`"`) {
						t.Errorf("missing camelCase field: %s", field)
					}
				}
				// Ensure PascalCase is NOT present
				unexpectedFields := []string{"ServiceName", "MethodName", "ResourceType", "ResourceName", "Timestamp"}
				for _, field := range unexpectedFields {
					if strings.Contains(output, `"`+field+`"`) {
						t.Errorf("found PascalCase field that should be camelCase: %s", field)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := formatter.Format(tt.results)
			if (err != nil) != tt.wantErr {
				t.Errorf("Format() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.checkFunc != nil {
				tt.checkFunc(t, output)
			}
		})
	}
}

func TestJSONFormatter_FileExtension(t *testing.T) {
	formatter := NewJSONFormatter()
	if ext := formatter.FileExtension(); ext != "json" {
		t.Errorf("FileExtension() = %s, want json", ext)
	}
}

func TestNewJSONFormatter(t *testing.T) {
	formatter := NewJSONFormatter()
	if formatter == nil {
		t.Error("NewJSONFormatter() returned nil")
	}
}

// Test that JSONFormatter satisfies OutputFormatter interface
func TestJSONFormatter_ImplementsInterface(t *testing.T) {
	var _ OutputFormatter = (*JSONFormatter)(nil)
}

func TestJSONFormatter_CompactOutput(t *testing.T) {
	formatter := NewJSONFormatter()
	results := []types.ScanResult{
		{
			ServiceName: "S3",
			MethodName:  "s3:ListBuckets",
			Timestamp:   time.Date(2026, 3, 2, 14, 30, 0, 0, time.UTC),
		},
	}
	output, err := formatter.Format(results)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(output, "\n") {
		t.Error("compact JSON should not contain newlines")
	}
}

func TestJSONFormatter_FormatWithSummary(t *testing.T) {
	formatter := NewJSONFormatter()
	fixedTime := time.Date(2026, 3, 3, 10, 30, 0, 0, time.UTC)
	results := []types.ScanResult{
		{ServiceName: "S3", MethodName: "s3:ListBuckets", ResourceName: "bucket-1", Timestamp: fixedTime},
		{ServiceName: "EC2", Error: errors.New("denied"), Timestamp: fixedTime},
	}
	summary := types.ScanSummary{
		TotalServices:        2,
		AccessibleServices:   1,
		AccessDeniedServices: 1,
		TotalResources:       1,
		ScanDuration:         5 * time.Second,
		Timestamp:            fixedTime,
	}

	output, err := formatter.FormatWithSummary(results, summary)
	if err != nil {
		t.Fatalf("FormatWithSummary() error: %v", err)
	}

	// Parse as JSON
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// Verify metadata key exists
	if _, ok := parsed["metadata"]; !ok {
		t.Error("JSON envelope should contain 'metadata' key")
	}
	// Verify results key exists
	if _, ok := parsed["results"]; !ok {
		t.Error("JSON envelope should contain 'results' key")
	}

	// Parse metadata
	var metadata struct {
		Timestamp            string `json:"timestamp"`
		Duration             string `json:"duration"`
		TotalServices        int    `json:"totalServices"`
		AccessibleServices   int    `json:"accessibleServices"`
		AccessDeniedServices int    `json:"accessDeniedServices"`
		TotalResources       int    `json:"totalResources"`
	}
	if err := json.Unmarshal(parsed["metadata"], &metadata); err != nil {
		t.Fatalf("invalid metadata JSON: %v", err)
	}
	if metadata.TotalServices != 2 {
		t.Errorf("metadata.totalServices = %d, want 2", metadata.TotalServices)
	}
	if metadata.AccessibleServices != 1 {
		t.Errorf("metadata.accessibleServices = %d, want 1", metadata.AccessibleServices)
	}
	if metadata.TotalResources != 1 {
		t.Errorf("metadata.totalResources = %d, want 1", metadata.TotalResources)
	}

	// Verify results array
	var resultsParsed []jsonScanResult
	if err := json.Unmarshal(parsed["results"], &resultsParsed); err != nil {
		t.Fatalf("invalid results JSON: %v", err)
	}
	if len(resultsParsed) != 2 {
		t.Errorf("expected 2 results, got %d", len(resultsParsed))
	}
}

func TestJSONFormatter_FormatWithSummary_AccessibleMethodNames(t *testing.T) {
	formatter := NewJSONFormatter()
	fixedTime := time.Date(2026, 3, 3, 10, 30, 0, 0, time.UTC)
	results := []types.ScanResult{
		{ServiceName: "S3", MethodName: "s3:ListBuckets", ResourceName: "bucket-1", Timestamp: fixedTime},
	}
	summary := types.ScanSummary{
		TotalServices:         2,
		AccessibleServices:    1,
		AccessDeniedServices:  1,
		TotalResources:        1,
		AccessibleMethodNames: []string{"iam:ListUsers", "s3:ListBuckets"},
		ScanDuration:          5 * time.Second,
		Timestamp:             fixedTime,
	}

	output, err := formatter.FormatWithSummary(results, summary)
	if err != nil {
		t.Fatalf("FormatWithSummary() error: %v", err)
	}

	var parsed map[string]json.RawMessage
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	var metadata struct {
		AccessibleMethodNames []string `json:"accessibleMethodNames"`
	}
	if err := json.Unmarshal(parsed["metadata"], &metadata); err != nil {
		t.Fatalf("invalid metadata JSON: %v", err)
	}
	if len(metadata.AccessibleMethodNames) != 2 {
		t.Errorf("accessibleMethodNames length = %d, want 2", len(metadata.AccessibleMethodNames))
	}
	if metadata.AccessibleMethodNames[0] != "iam:ListUsers" {
		t.Errorf("accessibleMethodNames[0] = %q, want %q", metadata.AccessibleMethodNames[0], "iam:ListUsers")
	}
}

func TestJSONFormatter_FormatWithSummary_NoHits_OmitsField(t *testing.T) {
	formatter := NewJSONFormatter()
	fixedTime := time.Date(2026, 3, 3, 10, 30, 0, 0, time.UTC)
	results := []types.ScanResult{
		{ServiceName: "EC2", Error: errors.New("denied"), Timestamp: fixedTime},
	}
	summary := types.ScanSummary{
		TotalServices:        1,
		AccessDeniedServices: 1,
		ScanDuration:         5 * time.Second,
		Timestamp:            fixedTime,
	}

	output, err := formatter.FormatWithSummary(results, summary)
	if err != nil {
		t.Fatalf("FormatWithSummary() error: %v", err)
	}

	if strings.Contains(output, "accessibleMethodNames") {
		t.Error("should omit accessibleMethodNames when empty (omitempty)")
	}
}

func TestJSONFormatter_ResilientSerialization(t *testing.T) {
	formatter := NewJSONFormatter()
	fixedTime := time.Date(2026, 3, 2, 14, 30, 0, 0, time.UTC)

	// Details with an unserializable value (channel)
	badDetails := map[string]interface{}{
		"ch": make(chan int),
	}
	results := []types.ScanResult{
		{ServiceName: "S3", Details: map[string]interface{}{"region": "us-east-1"}, Timestamp: fixedTime},
		{ServiceName: "BadService", Details: badDetails, Timestamp: fixedTime},
		{ServiceName: "EC2", Details: map[string]interface{}{"state": "running"}, Timestamp: fixedTime},
	}

	output, err := formatter.Format(results)
	if err != nil {
		t.Fatalf("resilient formatter should not return error, got: %v", err)
	}

	var parsed []jsonScanResult
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("output should be valid JSON: %v", err)
	}
	if len(parsed) != 3 {
		t.Fatalf("expected 3 results (including failed one), got %d", len(parsed))
	}
	// Good results should have their details intact
	if parsed[0].Details["region"] != "us-east-1" {
		t.Error("first result details should be preserved")
	}
	// Bad result should have empty details and an error noting the serialization failure
	if len(parsed[1].Details) != 0 {
		t.Error("bad result should have empty details")
	}
	if !strings.Contains(parsed[1].Error, "serialization error") {
		t.Errorf("bad result should note serialization error, got: %s", parsed[1].Error)
	}
	// Third result should be unaffected
	if parsed[2].Details["state"] != "running" {
		t.Error("third result details should be preserved")
	}
}
