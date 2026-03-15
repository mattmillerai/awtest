package formatters

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/MillerMedia/awtest/cmd/awtest/types"
)

func TestTableFormatter_Format(t *testing.T) {
	formatter := NewTableFormatter()
	fixedTime := time.Date(2026, 3, 2, 14, 30, 0, 0, time.UTC)

	tests := []struct {
		name      string
		results   []types.ScanResult
		wantErr   bool
		checkFunc func(t *testing.T, output string)
	}{
		{
			name: "single valid result produces table with correct columns",
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
				// Verify header columns present
				if !strings.Contains(output, "SERVICE") {
					t.Error("table should contain SERVICE header")
				}
				if !strings.Contains(output, "METHOD") {
					t.Error("table should contain METHOD header")
				}
				if !strings.Contains(output, "RESOURCE TYPE") {
					t.Error("table should contain RESOURCE TYPE header")
				}
				if !strings.Contains(output, "RESOURCE NAME") {
					t.Error("table should contain RESOURCE NAME header")
				}
				if !strings.Contains(output, "TIMESTAMP") {
					t.Error("table should contain TIMESTAMP header")
				}
				// Verify data present
				if !strings.Contains(output, "S3") {
					t.Error("table should contain service name S3")
				}
				if !strings.Contains(output, "s3:ListBuckets") {
					t.Error("table should contain method name")
				}
				if !strings.Contains(output, "bucket") {
					t.Error("table should contain resource type")
				}
				if !strings.Contains(output, "test-bucket") {
					t.Error("table should contain resource name")
				}
			},
		},
		{
			name:    "empty results returns no results found message",
			results: []types.ScanResult{},
			wantErr: false,
			checkFunc: func(t *testing.T, output string) {
				if output != "No results found" {
					t.Errorf("empty results should return 'No results found', got %q", output)
				}
			},
		},
		{
			name: "multiple results produce correct row count",
			results: []types.ScanResult{
				{ServiceName: "S3", MethodName: "s3:ListBuckets", ResourceName: "bucket-1", Timestamp: fixedTime},
				{ServiceName: "EC2", MethodName: "ec2:DescribeInstances", ResourceName: "i-123", Timestamp: fixedTime},
				{ServiceName: "IAM", MethodName: "iam:ListUsers", ResourceName: "admin", Timestamp: fixedTime},
			},
			wantErr: false,
			checkFunc: func(t *testing.T, output string) {
				if !strings.Contains(output, "S3") {
					t.Error("missing S3 row")
				}
				if !strings.Contains(output, "EC2") {
					t.Error("missing EC2 row")
				}
				if !strings.Contains(output, "IAM") {
					t.Error("missing IAM row")
				}
			},
		},
		{
			name: "long resource names are truncated with ellipsis",
			results: []types.ScanResult{
				{
					ServiceName:  "S3",
					MethodName:   "s3:ListBuckets",
					ResourceType: "bucket",
					ResourceName: "this-is-a-very-long-resource-name-that-should-trigger-truncation",
					Timestamp:    fixedTime,
				},
			},
			wantErr: false,
			checkFunc: func(t *testing.T, output string) {
				if !strings.Contains(output, "...") {
					t.Error("long resource name should be truncated with ellipsis")
				}
			},
		},
		{
			name: "result with error includes error indicator",
			results: []types.ScanResult{
				{
					ServiceName:  "RDS",
					MethodName:   "rds:DescribeDBInstances",
					ResourceType: "instance",
					ResourceName: "my-db",
					Error:        errors.New("denied"),
					Timestamp:    fixedTime,
				},
			},
			wantErr: false,
			checkFunc: func(t *testing.T, output string) {
				if !strings.Contains(output, "[ERROR:") {
					t.Error("table should contain error indicator [ERROR:]")
				}
				if !strings.Contains(output, "denied") {
					t.Error("table should contain error message")
				}
			},
		},
		{
			name: "result with nil error has no error indicator",
			results: []types.ScanResult{
				{
					ServiceName:  "S3",
					MethodName:   "s3:ListBuckets",
					ResourceType: "bucket",
					ResourceName: "clean-bucket",
					Timestamp:    fixedTime,
				},
			},
			wantErr: false,
			checkFunc: func(t *testing.T, output string) {
				if strings.Contains(output, "[ERROR:") {
					t.Error("table should NOT contain error indicator for nil error")
				}
			},
		},
		{
			name: "table output contains borders",
			results: []types.ScanResult{
				{ServiceName: "S3", MethodName: "s3:ListBuckets", Timestamp: fixedTime},
			},
			wantErr: false,
			checkFunc: func(t *testing.T, output string) {
				if !strings.Contains(output, "+") {
					t.Error("table should contain border characters (+)")
				}
				if !strings.Contains(output, "|") {
					t.Error("table should contain border characters (|)")
				}
				if !strings.Contains(output, "-") {
					t.Error("table should contain border characters (-)")
				}
			},
		},
		{
			name: "table output line width does not exceed 120 characters with max-width data",
			results: []types.ScanResult{
				{
					ServiceName:  "CloudFormation",
					MethodName:   "cloudformation:DescribeStackResources",
					ResourceType: "stack-resource-type",
					ResourceName: "my-very-long-resource-name-that-exceeds-column-width-limit",
					Timestamp:    fixedTime,
				},
			},
			wantErr: false,
			checkFunc: func(t *testing.T, output string) {
				lines := strings.Split(output, "\n")
				for i, line := range lines {
					if len(line) > 120 {
						t.Errorf("line %d exceeds 120 characters: length=%d, content=%q", i, len(line), line)
					}
				}
			},
		},
		{
			name: "timestamp formatted correctly in table",
			results: []types.ScanResult{
				{ServiceName: "EC2", Timestamp: fixedTime},
			},
			wantErr: false,
			checkFunc: func(t *testing.T, output string) {
				if !strings.Contains(output, "2026-03-02T14:30:00Z") {
					t.Error("table should contain RFC3339 formatted timestamp")
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

func TestTableFormatter_FormatWithSummary(t *testing.T) {
	formatter := NewTableFormatter()
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

	// Verify table content present
	if !strings.Contains(output, "S3") {
		t.Error("table should contain service name")
	}
	// Verify summary section present
	if !strings.Contains(output, "Scan Summary") {
		t.Error("table output should contain Scan Summary section")
	}
	if !strings.Contains(output, "Total Services:     2") {
		t.Error("table output should contain total services")
	}
	if !strings.Contains(output, "Accessible:         1") {
		t.Error("table output should contain accessible count")
	}
	if !strings.Contains(output, "Resources Found:    1") {
		t.Error("table output should contain resources count")
	}
}

func TestTableFormatter_FormatWithSummary_AccessibleMethods(t *testing.T) {
	formatter := NewTableFormatter()
	fixedTime := time.Date(2026, 3, 3, 10, 30, 0, 0, time.UTC)
	results := []types.ScanResult{
		{ServiceName: "S3", MethodName: "s3:ListBuckets", ResourceName: "bucket-1", Timestamp: fixedTime},
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
		t.Error("expected 'Accessible Methods:' section in table output")
	}
	if !strings.Contains(output, "s3:ListBuckets") {
		t.Error("expected method name in accessible methods list")
	}
}

func TestTableFormatter_FormatWithSummary_NoHits(t *testing.T) {
	formatter := NewTableFormatter()
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

func TestTableFormatter_FormatWithSummary_HitListCap(t *testing.T) {
	formatter := NewTableFormatter()
	fixedTime := time.Date(2026, 3, 3, 10, 30, 0, 0, time.UTC)

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
}

func TestTableFormatter_FileExtension(t *testing.T) {
	formatter := NewTableFormatter()
	if ext := formatter.FileExtension(); ext != "txt" {
		t.Errorf("FileExtension() = %s, want txt", ext)
	}
}

func TestNewTableFormatter(t *testing.T) {
	formatter := NewTableFormatter()
	if formatter == nil {
		t.Error("NewTableFormatter() returned nil")
	}
}

