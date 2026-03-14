package formatters

import (
	"encoding/csv"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/MillerMedia/awtest/cmd/awtest/types"
)

func TestCSVFormatter_Format(t *testing.T) {
	formatter := NewCSVFormatter()
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
				records := parseCSV(t, output)
				if len(records) != 2 { // header + 1 data row
					t.Fatalf("expected 2 rows (header + 1 data), got %d", len(records))
				}
				row := records[1]
				if row[0] != "S3" {
					t.Errorf("Service = %s, want S3", row[0])
				}
				if row[1] != "s3:ListBuckets" {
					t.Errorf("Method = %s, want s3:ListBuckets", row[1])
				}
				if row[2] != "bucket" {
					t.Errorf("ResourceType = %s, want bucket", row[2])
				}
				if row[3] != "test-bucket" {
					t.Errorf("ResourceName = %s, want test-bucket", row[3])
				}
				if row[4] != "region=us-east-1" {
					t.Errorf("Details = %s, want region=us-east-1", row[4])
				}
			},
		},
		{
			name:    "empty results returns header row only",
			results: []types.ScanResult{},
			wantErr: false,
			checkFunc: func(t *testing.T, output string) {
				records := parseCSV(t, output)
				if len(records) != 1 {
					t.Fatalf("expected 1 row (header only), got %d", len(records))
				}
				verifyHeader(t, records[0])
			},
		},
		{
			name: "result with error field populates Error column",
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
				records := parseCSV(t, output)
				row := records[1]
				if row[6] != "AccessDeniedException: not authorized" {
					t.Errorf("Error column = %s, want AccessDeniedException: not authorized", row[6])
				}
			},
		},
		{
			name: "special characters properly escaped per RFC 4180",
			results: []types.ScanResult{
				{
					ServiceName:  "S3",
					ResourceName: "value,with,commas",
					Timestamp:    fixedTime,
				},
				{
					ServiceName:  "EC2",
					ResourceName: `value "with" quotes`,
					Timestamp:    fixedTime,
				},
				{
					ServiceName:  "IAM",
					ResourceName: "value\nwith\nnewlines",
					Timestamp:    fixedTime,
				},
			},
			wantErr: false,
			checkFunc: func(t *testing.T, output string) {
				records := parseCSV(t, output)
				if len(records) != 4 { // header + 3 data rows
					t.Fatalf("expected 4 rows, got %d", len(records))
				}
				if records[1][3] != "value,with,commas" {
					t.Errorf("commas not preserved: %s", records[1][3])
				}
				if records[2][3] != `value "with" quotes` {
					t.Errorf("quotes not preserved: %s", records[2][3])
				}
				if records[3][3] != "value\nwith\nnewlines" {
					t.Errorf("newlines not preserved: %s", records[3][3])
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
				records := parseCSV(t, output)
				if len(records) != 4 { // header + 3 data rows
					t.Fatalf("expected 4 rows, got %d", len(records))
				}
			},
		},
		{
			name: "nil details produces empty Details column",
			results: []types.ScanResult{
				{ServiceName: "Lambda", Timestamp: fixedTime},
			},
			wantErr: false,
			checkFunc: func(t *testing.T, output string) {
				records := parseCSV(t, output)
				if records[1][4] != "" {
					t.Errorf("nil details should produce empty string, got %q", records[1][4])
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
				records := parseCSV(t, output)
				if records[1][5] != "2026-03-02T14:30:00Z" {
					t.Errorf("Timestamp = %s, want 2026-03-02T14:30:00Z", records[1][5])
				}
			},
		},
		{
			name: "successful result has empty Error column",
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
				records := parseCSV(t, output)
				if records[1][6] != "" {
					t.Errorf("successful result should have empty Error column, got %q", records[1][6])
				}
			},
		},
		{
			name: "complex Details map with multiple key-value pairs",
			results: []types.ScanResult{
				{
					ServiceName: "S3",
					Details: map[string]interface{}{
						"region": "us-east-1",
						"count":  5,
						"active": true,
					},
					Timestamp: fixedTime,
				},
			},
			wantErr: false,
			checkFunc: func(t *testing.T, output string) {
				records := parseCSV(t, output)
				details := records[1][4]
				// Keys should be sorted alphabetically
				if details != "active=true;count=5;region=us-east-1" {
					t.Errorf("Details = %s, want active=true;count=5;region=us-east-1", details)
				}
			},
		},
		{
			name: "Details value ordering is deterministic (sorted keys)",
			results: []types.ScanResult{
				{
					ServiceName: "S3",
					Details: map[string]interface{}{
						"zebra":    "z",
						"alpha":    "a",
						"middle":   "m",
						"bravo":    "b",
						"november": "n",
					},
					Timestamp: fixedTime,
				},
			},
			wantErr: false,
			checkFunc: func(t *testing.T, output string) {
				records := parseCSV(t, output)
				details := records[1][4]
				expected := "alpha=a;bravo=b;middle=m;november=n;zebra=z"
				if details != expected {
					t.Errorf("Details not sorted:\ngot:  %s\nwant: %s", details, expected)
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

func TestCSVFormatter_RoundTrip(t *testing.T) {
	formatter := NewCSVFormatter()
	fixedTime := time.Date(2026, 3, 2, 14, 30, 0, 0, time.UTC)

	results := []types.ScanResult{
		{
			ServiceName:  "S3",
			MethodName:   "s3:ListBuckets",
			ResourceType: "bucket",
			ResourceName: "test-bucket",
			Details:      map[string]interface{}{"region": "us-east-1", "count": 5},
			Timestamp:    fixedTime,
		},
		{
			ServiceName:  "EC2",
			MethodName:   "ec2:DescribeInstances",
			ResourceType: "instance",
			ResourceName: "i-abc123",
			Error:        errors.New("access denied"),
			Timestamp:    fixedTime,
		},
	}

	output, err := formatter.Format(results)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	// Parse with csv.Reader
	reader := csv.NewReader(strings.NewReader(output))
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("csv.Reader failed to parse output: %v", err)
	}

	// Verify structure
	if len(records) != 3 { // header + 2 data rows
		t.Fatalf("expected 3 rows, got %d", len(records))
	}

	// Verify header
	verifyHeader(t, records[0])

	// Verify field count per row
	for i, row := range records {
		if len(row) != 7 {
			t.Errorf("row %d has %d fields, want 7", i, len(row))
		}
	}

	// Verify data values round-trip correctly
	if records[1][0] != "S3" {
		t.Errorf("row 1 Service = %s, want S3", records[1][0])
	}
	if records[1][4] != "count=5;region=us-east-1" {
		t.Errorf("row 1 Details = %s, want count=5;region=us-east-1", records[1][4])
	}
	if records[2][6] != "access denied" {
		t.Errorf("row 2 Error = %s, want access denied", records[2][6])
	}
}

func TestCSVFormatter_FileExtension(t *testing.T) {
	formatter := NewCSVFormatter()
	if ext := formatter.FileExtension(); ext != "csv" {
		t.Errorf("FileExtension() = %s, want csv", ext)
	}
}

func TestNewCSVFormatter(t *testing.T) {
	formatter := NewCSVFormatter()
	if formatter == nil {
		t.Error("NewCSVFormatter() returned nil")
	}
}

// Test that CSVFormatter satisfies OutputFormatter interface
func TestCSVFormatter_ImplementsInterface(t *testing.T) {
	var _ OutputFormatter = (*CSVFormatter)(nil)
}

func TestCSVFormatter_ResilientSerialization(t *testing.T) {
	formatter := NewCSVFormatter()
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

	records := parseCSV(t, output)
	if len(records) != 4 { // header + 3 data rows
		t.Fatalf("expected 4 rows (header + 3 data), got %d", len(records))
	}

	// Good result should have details intact
	if records[1][4] != "region=us-east-1" {
		t.Errorf("first result details should be preserved, got %s", records[1][4])
	}

	// Bad result should have empty details and error noting serialization failure
	if records[2][4] != "" {
		t.Errorf("bad result should have empty details, got %s", records[2][4])
	}
	if !strings.Contains(records[2][6], "serialization error") {
		t.Errorf("bad result should note serialization error, got: %s", records[2][6])
	}

	// Third result should be unaffected
	if records[3][4] != "state=running" {
		t.Errorf("third result details should be preserved, got %s", records[3][4])
	}
}

func TestCSVFormatter_FormatWithSummary(t *testing.T) {
	formatter := NewCSVFormatter()
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

	// Verify header comments contain summary info
	if !strings.Contains(output, "# Scan Summary") {
		t.Error("CSV should contain summary header comment")
	}
	if !strings.Contains(output, "# Total Services: 2") {
		t.Error("CSV should contain total services in header")
	}
	if !strings.Contains(output, "# Accessible: 1") {
		t.Error("CSV should contain accessible count in header")
	}
	if !strings.Contains(output, "# Access Denied: 1") {
		t.Error("CSV should contain denied count in header")
	}
	if !strings.Contains(output, "# Resources Found: 1") {
		t.Error("CSV should contain resources count in header")
	}

	// Verify CSV data is still parseable (skip comment lines)
	lines := strings.Split(output, "\n")
	var csvLines []string
	for _, line := range lines {
		if !strings.HasPrefix(line, "#") {
			csvLines = append(csvLines, line)
		}
	}
	csvData := strings.Join(csvLines, "\n")
	reader := csv.NewReader(strings.NewReader(csvData))
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("CSV data should be parseable: %v", err)
	}
	if len(records) != 2 { // header + 1 data row
		t.Errorf("expected 2 CSV rows (header + 1 data), got %d", len(records))
	}
}

// parseCSV is a test helper that parses CSV output using csv.Reader.
func parseCSV(t *testing.T, output string) [][]string {
	t.Helper()
	reader := csv.NewReader(strings.NewReader(output))
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("failed to parse CSV output: %v", err)
	}
	return records
}

// verifyHeader checks that the CSV header row matches the expected columns.
func verifyHeader(t *testing.T, header []string) {
	t.Helper()
	expected := []string{"Service", "Method", "ResourceType", "ResourceName", "Details", "Timestamp", "Error"}
	if len(header) != len(expected) {
		t.Fatalf("header has %d columns, want %d", len(header), len(expected))
	}
	for i, col := range expected {
		if header[i] != col {
			t.Errorf("header[%d] = %s, want %s", i, header[i], col)
		}
	}
}
