package systemsmanager

import (
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"testing"
	"time"
)

func TestProcess(t *testing.T) {
	process := SystemsManagerCalls[0].Process

	tests := []struct {
		name          string
		input         interface{}
		err           error
		expectedCount int
		expectError   bool
		checkResults  func(t *testing.T, results []types.ScanResult)
	}{
		{
			name: "string parameter with all fields",
			input: []ssmtypes.ParameterMetadata{
				{
					Name:             aws.String("/app/config/db-host"),
					Type:             ssmtypes.ParameterTypeString,
					Description:      aws.String("Database hostname"),
					LastModifiedDate: aws.Time(time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)),
					Version:          3,
				},
			},
			expectedCount: 1,
			checkResults: func(t *testing.T, results []types.ScanResult) {
				r := results[0]
				if r.ServiceName != "Systems Manager" {
					t.Errorf("expected ServiceName 'Systems Manager', got '%s'", r.ServiceName)
				}
				if r.MethodName != "ssm:DescribeParameters" {
					t.Errorf("expected MethodName 'ssm:DescribeParameters', got '%s'", r.MethodName)
				}
				if r.ResourceType != "parameter" {
					t.Errorf("expected ResourceType 'parameter', got '%s'", r.ResourceType)
				}
				if r.ResourceName != "/app/config/db-host" {
					t.Errorf("expected ResourceName '/app/config/db-host', got '%s'", r.ResourceName)
				}
				if r.Details["Type"] != "String" {
					t.Errorf("expected Type 'String', got '%v'", r.Details["Type"])
				}
				if r.Details["Description"] != "Database hostname" {
					t.Errorf("expected Description 'Database hostname', got '%v'", r.Details["Description"])
				}
				if r.Details["LastModifiedDate"] != "2025-06-15 10:30:00" {
					t.Errorf("expected LastModifiedDate '2025-06-15 10:30:00', got '%v'", r.Details["LastModifiedDate"])
				}
				if r.Details["Version"] != int64(3) {
					t.Errorf("expected Version 3, got '%v'", r.Details["Version"])
				}
			},
		},
		{
			name: "secure string parameter",
			input: []ssmtypes.ParameterMetadata{
				{
					Name:    aws.String("/app/secrets/api-key"),
					Type:    ssmtypes.ParameterTypeSecureString,
					Version: 1,
				},
			},
			expectedCount: 1,
			checkResults: func(t *testing.T, results []types.ScanResult) {
				r := results[0]
				if r.Details["Type"] != "SecureString" {
					t.Errorf("expected Type 'SecureString', got '%v'", r.Details["Type"])
				}
				if r.ResourceName != "/app/secrets/api-key" {
					t.Errorf("expected ResourceName '/app/secrets/api-key', got '%s'", r.ResourceName)
				}
			},
		},
		{
			name: "multiple parameters",
			input: []ssmtypes.ParameterMetadata{
				{Name: aws.String("/param-1"), Type: ssmtypes.ParameterTypeString},
				{Name: aws.String("/param-2"), Type: ssmtypes.ParameterTypeSecureString},
			},
			expectedCount: 2,
			checkResults: func(t *testing.T, results []types.ScanResult) {
				if results[0].ResourceName != "/param-1" {
					t.Errorf("expected first ResourceName '/param-1', got '%s'", results[0].ResourceName)
				}
				if results[1].ResourceName != "/param-2" {
					t.Errorf("expected second ResourceName '/param-2', got '%s'", results[1].ResourceName)
				}
			},
		},
		{
			name:          "empty results",
			input:         []ssmtypes.ParameterMetadata{},
			expectedCount: 0,
		},
		{
			name:          "access denied error",
			input:         nil,
			err:           fmt.Errorf("AccessDeniedException: User is not authorized"),
			expectedCount: 1,
			expectError:   true,
			checkResults: func(t *testing.T, results []types.ScanResult) {
				if results[0].Error == nil {
					t.Error("expected error in result, got nil")
				}
				if results[0].ServiceName != "Systems Manager" {
					t.Errorf("expected ServiceName 'Systems Manager', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "ssm:DescribeParameters" {
					t.Errorf("expected MethodName 'ssm:DescribeParameters', got '%s'", results[0].MethodName)
				}
			},
		},
		{
			name: "nil field handling",
			input: []ssmtypes.ParameterMetadata{
				{
					Name:             nil,
					Type:             "",
					Description:      nil,
					LastModifiedDate: nil,
					Version:          0,
				},
			},
			expectedCount: 1,
			checkResults: func(t *testing.T, results []types.ScanResult) {
				r := results[0]
				if r.ResourceName != "" {
					t.Errorf("expected empty ResourceName for nil Name, got '%s'", r.ResourceName)
				}
				if r.Details["Type"] != "" {
					t.Errorf("expected empty Type for nil, got '%v'", r.Details["Type"])
				}
				if r.Details["Description"] != "" {
					t.Errorf("expected empty Description for nil, got '%v'", r.Details["Description"])
				}
				if r.Details["LastModifiedDate"] != "" {
					t.Errorf("expected empty LastModifiedDate for nil, got '%v'", r.Details["LastModifiedDate"])
				}
				if r.Details["Version"] != int64(0) {
					t.Errorf("expected Version 0 for nil, got '%v'", r.Details["Version"])
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			results := process(tc.input, tc.err, false)
			if len(results) != tc.expectedCount {
				t.Fatalf("expected %d results, got %d", tc.expectedCount, len(results))
			}
			if tc.checkResults != nil {
				tc.checkResults(t, results)
			}
		})
	}
}
