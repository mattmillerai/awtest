package formatters

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"gopkg.in/yaml.v3"
)

func TestYAMLFormatter_Format(t *testing.T) {
	formatter := NewYAMLFormatter()
	fixedTime := time.Date(2026, 3, 2, 14, 30, 0, 0, time.UTC)

	tests := []struct {
		name      string
		results   []types.ScanResult
		wantErr   bool
		checkFunc func(t *testing.T, output string)
	}{
		{
			name: "single valid result with all fields",
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
				var parsed []yamlScanResult
				if err := yaml.Unmarshal([]byte(output), &parsed); err != nil {
					t.Fatalf("invalid YAML: %v", err)
				}
				if len(parsed) != 1 {
					t.Fatalf("expected 1 result, got %d", len(parsed))
				}
				if parsed[0].ServiceName != "S3" {
					t.Errorf("expected S3, got %s", parsed[0].ServiceName)
				}
				if parsed[0].MethodName != "s3:ListBuckets" {
					t.Errorf("expected s3:ListBuckets, got %s", parsed[0].MethodName)
				}
				if parsed[0].ResourceType != "bucket" {
					t.Errorf("expected bucket, got %s", parsed[0].ResourceType)
				}
				if parsed[0].ResourceName != "test-bucket" {
					t.Errorf("expected test-bucket, got %s", parsed[0].ResourceName)
				}
			},
		},
		{
			name:    "empty results returns empty YAML sequence",
			results: []types.ScanResult{},
			wantErr: false,
			checkFunc: func(t *testing.T, output string) {
				if strings.TrimSpace(output) != "[]" {
					t.Errorf("expected [], got %q", strings.TrimSpace(output))
				}
			},
		},
		{
			name: "result with error field",
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
				var parsed []yamlScanResult
				if err := yaml.Unmarshal([]byte(output), &parsed); err != nil {
					t.Fatalf("invalid YAML: %v", err)
				}
				if parsed[0].Error != "AccessDeniedException: not authorized" {
					t.Errorf("error not serialized correctly: %s", parsed[0].Error)
				}
			},
		},
		{
			name: "special characters in resource names",
			results: []types.ScanResult{
				{
					ServiceName:  "S3",
					ResourceName: "arn:aws:s3:::my-bucket",
					Timestamp:    fixedTime,
				},
				{
					ServiceName:  "SSM",
					ResourceName: "key with: colon",
					Timestamp:    fixedTime,
				},
				{
					ServiceName:  "Config",
					ResourceName: "hash#value",
					Timestamp:    fixedTime,
				},
			},
			wantErr: false,
			checkFunc: func(t *testing.T, output string) {
				var parsed []yamlScanResult
				if err := yaml.Unmarshal([]byte(output), &parsed); err != nil {
					t.Fatalf("invalid YAML: %v", err)
				}
				if len(parsed) != 3 {
					t.Fatalf("expected 3 results, got %d", len(parsed))
				}
				if parsed[0].ResourceName != "arn:aws:s3:::my-bucket" {
					t.Errorf("ARN not preserved: %s", parsed[0].ResourceName)
				}
				if parsed[1].ResourceName != "key with: colon" {
					t.Errorf("colon string not preserved: %s", parsed[1].ResourceName)
				}
				if parsed[2].ResourceName != "hash#value" {
					t.Errorf("hash string not preserved: %s", parsed[2].ResourceName)
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
				var parsed []yamlScanResult
				if err := yaml.Unmarshal([]byte(output), &parsed); err != nil {
					t.Fatalf("invalid YAML: %v", err)
				}
				if len(parsed) != 3 {
					t.Fatalf("expected 3 results, got %d", len(parsed))
				}
			},
		},
		{
			name: "nil details serialized as empty map",
			results: []types.ScanResult{
				{ServiceName: "Lambda", Timestamp: fixedTime},
			},
			wantErr: false,
			checkFunc: func(t *testing.T, output string) {
				var parsed []yamlScanResult
				if err := yaml.Unmarshal([]byte(output), &parsed); err != nil {
					t.Fatalf("invalid YAML: %v", err)
				}
				if parsed[0].Details == nil {
					t.Error("nil details should serialize as empty map, not null")
				}
				if len(parsed[0].Details) != 0 {
					t.Errorf("expected empty details map, got %v", parsed[0].Details)
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
			name: "successful result has no error field",
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
				if strings.Contains(output, "error:") {
					t.Error("successful result should not contain error field (omitempty)")
				}
			},
		},
		{
			name: "YAML round-trip marshal then unmarshal",
			results: []types.ScanResult{
				{
					ServiceName:  "S3",
					MethodName:   "s3:ListBuckets",
					ResourceType: "bucket",
					ResourceName: "test-bucket",
					Details:      map[string]interface{}{"region": "us-east-1", "count": 5},
					Timestamp:    fixedTime,
				},
			},
			wantErr: false,
			checkFunc: func(t *testing.T, output string) {
				var parsed []yamlScanResult
				if err := yaml.Unmarshal([]byte(output), &parsed); err != nil {
					t.Fatalf("round-trip unmarshal failed: %v", err)
				}
				if parsed[0].ServiceName != "S3" {
					t.Errorf("round-trip: serviceName = %s, want S3", parsed[0].ServiceName)
				}
				if parsed[0].Details["region"] != "us-east-1" {
					t.Errorf("round-trip: details region = %v, want us-east-1", parsed[0].Details["region"])
				}
				// Re-marshal to verify consistency
				data2, err := yaml.Marshal(parsed)
				if err != nil {
					t.Fatalf("re-marshal failed: %v", err)
				}
				var parsed2 []yamlScanResult
				if err := yaml.Unmarshal(data2, &parsed2); err != nil {
					t.Fatalf("second round-trip unmarshal failed: %v", err)
				}
				if parsed2[0].ServiceName != parsed[0].ServiceName {
					t.Error("round-trip data inconsistent")
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

func TestYAMLFormatter_FileExtension(t *testing.T) {
	formatter := NewYAMLFormatter()
	if ext := formatter.FileExtension(); ext != "yaml" {
		t.Errorf("FileExtension() = %s, want yaml", ext)
	}
}

func TestNewYAMLFormatter(t *testing.T) {
	formatter := NewYAMLFormatter()
	if formatter == nil {
		t.Error("NewYAMLFormatter() returned nil")
	}
}

// Test that YAMLFormatter satisfies OutputFormatter interface
func TestYAMLFormatter_ImplementsInterface(t *testing.T) {
	var _ OutputFormatter = (*YAMLFormatter)(nil)
}

func TestYAMLFormatter_FormatWithSummary(t *testing.T) {
	formatter := NewYAMLFormatter()
	fixedTime := time.Date(2026, 3, 3, 10, 30, 0, 0, time.UTC)
	results := []types.ScanResult{
		{ServiceName: "S3", MethodName: "s3:ListBuckets", ResourceName: "bucket-1", Timestamp: fixedTime},
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

	// Verify metadata section present
	if !strings.Contains(output, "metadata:") {
		t.Error("YAML should contain metadata section")
	}
	if !strings.Contains(output, "totalServices: 2") {
		t.Error("YAML metadata should contain totalServices")
	}
	if !strings.Contains(output, "results:") {
		t.Error("YAML should contain results section")
	}

	// Parse and verify structure
	var parsed yamlEnvelope
	if err := yaml.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("invalid YAML: %v", err)
	}
	if parsed.Metadata.TotalServices != 2 {
		t.Errorf("metadata.totalServices = %d, want 2", parsed.Metadata.TotalServices)
	}
	if len(parsed.Results) != 1 {
		t.Errorf("expected 1 result, got %d", len(parsed.Results))
	}
}

func TestYAMLFormatter_FormatWithSummary_AccessibleMethodNames(t *testing.T) {
	formatter := NewYAMLFormatter()
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

	var parsed yamlEnvelope
	if err := yaml.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("invalid YAML: %v", err)
	}
	if len(parsed.Metadata.AccessibleMethodNames) != 2 {
		t.Errorf("accessibleMethodNames length = %d, want 2", len(parsed.Metadata.AccessibleMethodNames))
	}
	if parsed.Metadata.AccessibleMethodNames[0] != "iam:ListUsers" {
		t.Errorf("accessibleMethodNames[0] = %q, want %q", parsed.Metadata.AccessibleMethodNames[0], "iam:ListUsers")
	}
}

func TestYAMLFormatter_FormatWithSummary_NoHits_OmitsField(t *testing.T) {
	formatter := NewYAMLFormatter()
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

func TestYAMLFormatter_ResilientSerialization(t *testing.T) {
	formatter := NewYAMLFormatter()
	fixedTime := time.Date(2026, 3, 2, 14, 30, 0, 0, time.UTC)

	// Details with an unserializable value (function)
	badDetails := map[string]interface{}{
		"fn": func() {},
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

	var parsed []yamlScanResult
	if err := yaml.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("output should be valid YAML: %v", err)
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
	// Verify specific error detail is preserved (not generic message)
	if !strings.Contains(parsed[1].Error, "func") {
		t.Errorf("error should contain specific type info, got: %s", parsed[1].Error)
	}
	// Third result should be unaffected
	if parsed[2].Details["state"] != "running" {
		t.Error("third result details should be preserved")
	}
}
