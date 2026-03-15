package efs

import (
	"fmt"
	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/efs"
	"testing"
)

func TestProcess(t *testing.T) {
	process := EfsCalls[0].Process

	tests := []struct {
		name          string
		input         interface{}
		err           error
		expectedCount int
		expectError   bool
		checkResults  func(t *testing.T, results []types.ScanResult)
	}{
		{
			name: "valid file systems with all fields",
			input: []*efs.FileSystemDescription{
				{
					FileSystemId:         aws.String("fs-12345678"),
					Name:                 aws.String("my-efs"),
					LifeCycleState:       aws.String("available"),
					SizeInBytes:          &efs.FileSystemSize{Value: aws.Int64(1024000)},
					NumberOfMountTargets: aws.Int64(2),
					Encrypted:            aws.Bool(true),
				},
			},
			expectedCount: 1,
			checkResults: func(t *testing.T, results []types.ScanResult) {
				r := results[0]
				if r.ServiceName != "EFS" {
					t.Errorf("expected ServiceName 'EFS', got '%s'", r.ServiceName)
				}
				if r.MethodName != "efs:DescribeFileSystems" {
					t.Errorf("expected MethodName 'efs:DescribeFileSystems', got '%s'", r.MethodName)
				}
				if r.ResourceType != "file-system" {
					t.Errorf("expected ResourceType 'file-system', got '%s'", r.ResourceType)
				}
				if r.ResourceName != "fs-12345678" {
					t.Errorf("expected ResourceName 'fs-12345678', got '%s'", r.ResourceName)
				}
				if r.Details["Name"] != "my-efs" {
					t.Errorf("expected Name 'my-efs', got '%v'", r.Details["Name"])
				}
				if r.Details["LifeCycleState"] != "available" {
					t.Errorf("expected LifeCycleState 'available', got '%v'", r.Details["LifeCycleState"])
				}
				if r.Details["SizeInBytes"] != int64(1024000) {
					t.Errorf("expected SizeInBytes 1024000, got '%v'", r.Details["SizeInBytes"])
				}
				if r.Details["NumberOfMountTargets"] != int64(2) {
					t.Errorf("expected NumberOfMountTargets 2, got '%v'", r.Details["NumberOfMountTargets"])
				}
				if r.Details["Encrypted"] != true {
					t.Errorf("expected Encrypted true, got '%v'", r.Details["Encrypted"])
				}
			},
		},
		{
			name: "encrypted vs unencrypted",
			input: []*efs.FileSystemDescription{
				{
					FileSystemId:   aws.String("fs-encrypted"),
					LifeCycleState: aws.String("available"),
					SizeInBytes:    &efs.FileSystemSize{Value: aws.Int64(500)},
					Encrypted:      aws.Bool(true),
				},
				{
					FileSystemId:   aws.String("fs-unencrypted"),
					LifeCycleState: aws.String("available"),
					SizeInBytes:    &efs.FileSystemSize{Value: aws.Int64(1000)},
					Encrypted:      aws.Bool(false),
				},
			},
			expectedCount: 2,
			checkResults: func(t *testing.T, results []types.ScanResult) {
				if results[0].Details["Encrypted"] != true {
					t.Errorf("expected first fs Encrypted true, got '%v'", results[0].Details["Encrypted"])
				}
				if results[1].Details["Encrypted"] != false {
					t.Errorf("expected second fs Encrypted false, got '%v'", results[1].Details["Encrypted"])
				}
			},
		},
		{
			name:          "empty results",
			input:         []*efs.FileSystemDescription{},
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
				if results[0].ServiceName != "EFS" {
					t.Errorf("expected ServiceName 'EFS', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "efs:DescribeFileSystems" {
					t.Errorf("expected MethodName 'efs:DescribeFileSystems', got '%s'", results[0].MethodName)
				}
			},
		},
		{
			name: "nil field handling",
			input: []*efs.FileSystemDescription{
				{
					FileSystemId:         nil,
					Name:                 nil,
					LifeCycleState:       nil,
					SizeInBytes:          nil,
					NumberOfMountTargets: nil,
					Encrypted:            nil,
				},
			},
			expectedCount: 1,
			checkResults: func(t *testing.T, results []types.ScanResult) {
				r := results[0]
				if r.ResourceName != "" {
					t.Errorf("expected empty ResourceName for nil FileSystemId, got '%s'", r.ResourceName)
				}
				if r.Details["Name"] != "" {
					t.Errorf("expected empty Name for nil, got '%v'", r.Details["Name"])
				}
				if r.Details["LifeCycleState"] != "" {
					t.Errorf("expected empty LifeCycleState for nil, got '%v'", r.Details["LifeCycleState"])
				}
				if r.Details["SizeInBytes"] != int64(0) {
					t.Errorf("expected SizeInBytes 0 for nil, got '%v'", r.Details["SizeInBytes"])
				}
				if r.Details["NumberOfMountTargets"] != int64(0) {
					t.Errorf("expected NumberOfMountTargets 0 for nil, got '%v'", r.Details["NumberOfMountTargets"])
				}
				if r.Details["Encrypted"] != false {
					t.Errorf("expected Encrypted false for nil, got '%v'", r.Details["Encrypted"])
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
