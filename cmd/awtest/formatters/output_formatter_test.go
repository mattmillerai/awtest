package formatters

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/MillerMedia/awtest/cmd/awtest/types"
)

// mockFormatter is a test implementation of the OutputFormatter interface.
type mockFormatter struct {
	formatFunc func([]types.ScanResult) (string, error)
	extension  string
}

func (m *mockFormatter) Format(results []types.ScanResult) (string, error) {
	return m.formatFunc(results)
}

func (m *mockFormatter) FormatWithSummary(results []types.ScanResult, summary types.ScanSummary) (string, error) {
	return m.formatFunc(results)
}

func (m *mockFormatter) FileExtension() string {
	return m.extension
}

func TestOutputFormatter_InterfaceContract(t *testing.T) {
	// Verify that a mock can satisfy the interface
	var _ OutputFormatter = (*mockFormatter)(nil)
}

func TestMockFormatter_Format(t *testing.T) {
	mock := &mockFormatter{
		formatFunc: func(results []types.ScanResult) (string, error) {
			var lines []string
			for _, r := range results {
				lines = append(lines, fmt.Sprintf("%s:%s", r.ServiceName, r.ResourceName))
			}
			return strings.Join(lines, "\n"), nil
		},
		extension: "txt",
	}

	results := []types.ScanResult{
		{
			ServiceName:  "S3",
			MethodName:   "s3:ListBuckets",
			ResourceType: "bucket",
			ResourceName: "test-bucket",
			Details:      map[string]interface{}{"region": "us-east-1"},
			Timestamp:    time.Now(),
		},
		{
			ServiceName:  "EC2",
			MethodName:   "ec2:DescribeInstances",
			ResourceType: "instance",
			ResourceName: "i-1234567890",
			Timestamp:    time.Now(),
		},
	}

	output, err := mock.Format(results)
	if err != nil {
		t.Fatalf("Format() returned unexpected error: %v", err)
	}
	if !strings.Contains(output, "S3:test-bucket") {
		t.Error("output should contain S3:test-bucket")
	}
	if !strings.Contains(output, "EC2:i-1234567890") {
		t.Error("output should contain EC2:i-1234567890")
	}
}

func TestMockFormatter_FormatEmpty(t *testing.T) {
	mock := &mockFormatter{
		formatFunc: func(results []types.ScanResult) (string, error) {
			if len(results) == 0 {
				return "[]", nil
			}
			return "", nil
		},
		extension: "json",
	}

	output, err := mock.Format([]types.ScanResult{})
	if err != nil {
		t.Fatalf("Format() returned unexpected error: %v", err)
	}
	if output != "[]" {
		t.Errorf("Format() = %s, want []", output)
	}
}

func TestMockFormatter_FormatError(t *testing.T) {
	mock := &mockFormatter{
		formatFunc: func(results []types.ScanResult) (string, error) {
			return "", fmt.Errorf("formatting failed")
		},
		extension: "txt",
	}

	_, err := mock.Format([]types.ScanResult{{ServiceName: "S3"}})
	if err == nil {
		t.Error("Format() should return an error")
	}
	if err.Error() != "formatting failed" {
		t.Errorf("error = %s, want 'formatting failed'", err.Error())
	}
}

func TestMockFormatter_FileExtension(t *testing.T) {
	tests := []struct {
		extension string
	}{
		{"json"},
		{"yaml"},
		{"csv"},
		{"txt"},
	}

	for _, tt := range tests {
		mock := &mockFormatter{
			formatFunc: func(results []types.ScanResult) (string, error) { return "", nil },
			extension:  tt.extension,
		}
		if ext := mock.FileExtension(); ext != tt.extension {
			t.Errorf("FileExtension() = %s, want %s", ext, tt.extension)
		}
	}
}

func TestJSONFormatter_SatisfiesOutputFormatter(t *testing.T) {
	// Verify JSONFormatter satisfies the OutputFormatter interface at compile time
	var _ OutputFormatter = (*JSONFormatter)(nil)

	// Also verify at runtime
	formatter := NewJSONFormatter()
	var iface OutputFormatter = formatter
	if iface == nil {
		t.Error("JSONFormatter should satisfy OutputFormatter interface")
	}
}
