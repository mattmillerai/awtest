package formatters

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/MillerMedia/awtest/cmd/awtest/types"
)

func TestTextFormatter_Format(t *testing.T) {
	formatter := NewTextFormatter()
	fixedTime := time.Date(2026, 3, 2, 14, 30, 0, 0, time.UTC)

	tests := []struct {
		name      string
		results   []types.ScanResult
		wantErr   bool
		checkFunc func(t *testing.T, output string)
	}{
		{
			name: "single valid result with resource",
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
				if !strings.Contains(output, "S3") {
					t.Error("expected output to contain service name S3")
				}
				if !strings.Contains(output, "s3:ListBuckets") {
					t.Error("expected output to contain method name")
				}
				if !strings.Contains(output, "bucket") {
					t.Error("expected output to contain resource type")
				}
				if !strings.Contains(output, "test-bucket") {
					t.Error("expected output to contain resource name")
				}
			},
		},
		{
			name:    "empty results returns no results message",
			results: []types.ScanResult{},
			wantErr: false,
			checkFunc: func(t *testing.T, output string) {
				if output != "No results found" {
					t.Errorf("expected 'No results found', got %q", output)
				}
			},
		},
		{
			name: "result with error includes error info",
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
				if !strings.Contains(output, "Error") {
					t.Error("expected output to contain 'Error'")
				}
				if !strings.Contains(output, "AccessDeniedException") {
					t.Error("expected output to contain error message")
				}
			},
		},
		{
			name: "multiple results produce multiple lines",
			results: []types.ScanResult{
				{ServiceName: "S3", MethodName: "s3:ListBuckets", ResourceType: "bucket", ResourceName: "bucket-1", Timestamp: fixedTime},
				{ServiceName: "EC2", MethodName: "ec2:DescribeInstances", ResourceType: "instance", ResourceName: "i-123", Timestamp: fixedTime},
			},
			wantErr: false,
			checkFunc: func(t *testing.T, output string) {
				lines := strings.Split(output, "\n")
				if len(lines) != 2 {
					t.Errorf("expected 2 lines, got %d", len(lines))
				}
			},
		},
		{
			name: "empty service name uses default module name",
			results: []types.ScanResult{
				{ServiceName: "", MethodName: "test:Method", ResourceName: "resource", Timestamp: fixedTime},
			},
			wantErr: false,
			checkFunc: func(t *testing.T, output string) {
				if !strings.Contains(output, types.DefaultModuleName) {
					t.Errorf("expected default module name %s in output", types.DefaultModuleName)
				}
			},
		},
		{
			name: "result with only resource name",
			results: []types.ScanResult{
				{ServiceName: "Lambda", MethodName: "lambda:ListFunctions", ResourceName: "my-function", Timestamp: fixedTime},
			},
			wantErr: false,
			checkFunc: func(t *testing.T, output string) {
				if !strings.Contains(output, "my-function") {
					t.Error("expected output to contain resource name")
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

func TestTextFormatter_FormatWithSummary(t *testing.T) {
	formatter := NewTextFormatter()
	fixedTime := time.Date(2026, 3, 3, 10, 30, 0, 0, time.UTC)
	results := []types.ScanResult{
		{ServiceName: "S3", MethodName: "s3:ListBuckets", ResourceType: "bucket", ResourceName: "bucket-1", Timestamp: fixedTime},
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

	// Verify text content present
	if !strings.Contains(output, "S3") {
		t.Error("text should contain service name")
	}
	// Verify summary section present
	if !strings.Contains(output, "Scan Summary") {
		t.Error("text output should contain Scan Summary section")
	}
	if !strings.Contains(output, "Total Services:     2") {
		t.Error("text output should contain total services")
	}
	if !strings.Contains(output, "Accessible:         1") {
		t.Error("text output should contain accessible count")
	}
	if !strings.Contains(output, "Resources Found:    1") {
		t.Error("text output should contain resources count")
	}
}

func TestTextFormatter_FormatWithSummary_AccessibleMethods(t *testing.T) {
	formatter := NewTextFormatter()
	fixedTime := time.Date(2026, 3, 3, 10, 30, 0, 0, time.UTC)
	results := []types.ScanResult{
		{ServiceName: "S3", MethodName: "s3:ListBuckets", ResourceType: "bucket", ResourceName: "bucket-1", Timestamp: fixedTime},
	}
	summary := types.ScanSummary{
		TotalServices:         2,
		AccessibleServices:    1,
		AccessDeniedServices:  1,
		TotalResources:        1,
		AccessibleMethodNames: []string{"s3:ListBuckets"},
		ScanDuration:          5 * time.Second,
		Timestamp:             fixedTime,
	}

	output, err := formatter.FormatWithSummary(results, summary)
	if err != nil {
		t.Fatalf("FormatWithSummary() error: %v", err)
	}

	if !strings.Contains(output, "Accessible Methods:") {
		t.Error("expected 'Accessible Methods:' section in output")
	}
	if !strings.Contains(output, "s3:ListBuckets") {
		t.Error("expected method name in accessible methods list")
	}
}

func TestTextFormatter_FormatWithSummary_NoHits(t *testing.T) {
	formatter := NewTextFormatter()
	fixedTime := time.Date(2026, 3, 3, 10, 30, 0, 0, time.UTC)
	results := []types.ScanResult{
		{ServiceName: "EC2", MethodName: "ec2:DescribeInstances", Error: errors.New("denied"), Timestamp: fixedTime},
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

	if strings.Contains(output, "Accessible Methods:") {
		t.Error("should not show 'Accessible Methods:' section when no hits")
	}
}

func TestTextFormatter_FormatWithSummary_HitListCap(t *testing.T) {
	formatter := NewTextFormatter()
	fixedTime := time.Date(2026, 3, 3, 10, 30, 0, 0, time.UTC)

	// Create 25 method names
	var methods []string
	for i := 0; i < 25; i++ {
		methods = append(methods, fmt.Sprintf("svc:Method%02d", i))
	}

	results := []types.ScanResult{
		{ServiceName: "S3", MethodName: "s3:ListBuckets", ResourceName: "b", Timestamp: fixedTime},
	}
	summary := types.ScanSummary{
		TotalServices:         1,
		AccessibleServices:    1,
		TotalResources:        1,
		AccessibleMethodNames: methods,
		ScanDuration:          5 * time.Second,
		Timestamp:             fixedTime,
	}

	output, err := formatter.FormatWithSummary(results, summary)
	if err != nil {
		t.Fatalf("FormatWithSummary() error: %v", err)
	}

	if !strings.Contains(output, "5 more - use --format=json for full list") {
		t.Error("expected overflow message for >20 methods")
	}
	// Should show first 20 but not 21st
	if !strings.Contains(output, "svc:Method19") {
		t.Error("expected 20th method in output")
	}
	if strings.Contains(output, "  - svc:Method20") {
		t.Error("should not show 21st method in capped output")
	}
}

func TestTextFormatter_FileExtension(t *testing.T) {
	formatter := NewTextFormatter()
	if ext := formatter.FileExtension(); ext != "txt" {
		t.Errorf("FileExtension() = %s, want txt", ext)
	}
}

func TestNewTextFormatter(t *testing.T) {
	formatter := NewTextFormatter()
	if formatter == nil {
		t.Error("NewTextFormatter() returned nil")
	}
}

// Test that TextFormatter satisfies OutputFormatter interface
func TestTextFormatter_ImplementsInterface(t *testing.T) {
	var _ OutputFormatter = (*TextFormatter)(nil)
}
