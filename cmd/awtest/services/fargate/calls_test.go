package fargate

import (
	"fmt"
	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
	"testing"
)

func TestProcess(t *testing.T) {
	process := FargateCalls[0].Process

	tests := []struct {
		name          string
		input         interface{}
		err           error
		expectedCount int
		expectError   bool
		checkResults  func(t *testing.T, results []types.ScanResult)
	}{
		{
			name: "valid Fargate task with all fields",
			input: []*ecs.Task{
				{
					TaskArn:           aws.String("arn:aws:ecs:us-east-1:123456:task/my-cluster/abc123"),
					ClusterArn:        aws.String("arn:aws:ecs:us-east-1:123456:cluster/my-cluster"),
					LastStatus:        aws.String("RUNNING"),
					DesiredStatus:     aws.String("RUNNING"),
					TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-1:123456:task-definition/my-task:1"),
					Cpu:               aws.String("256"),
					Memory:            aws.String("512"),
					LaunchType:        aws.String("FARGATE"),
				},
			},
			expectedCount: 1,
			checkResults: func(t *testing.T, results []types.ScanResult) {
				r := results[0]
				if r.ServiceName != "Fargate" {
					t.Errorf("expected ServiceName 'Fargate', got '%s'", r.ServiceName)
				}
				if r.MethodName != "ecs:ListFargateTasks" {
					t.Errorf("expected MethodName 'ecs:ListFargateTasks', got '%s'", r.MethodName)
				}
				if r.ResourceType != "task" {
					t.Errorf("expected ResourceType 'task', got '%s'", r.ResourceType)
				}
				if r.ResourceName != "arn:aws:ecs:us-east-1:123456:task/my-cluster/abc123" {
					t.Errorf("expected ResourceName 'arn:aws:ecs:us-east-1:123456:task/my-cluster/abc123', got '%s'", r.ResourceName)
				}
				if r.Details["ClusterArn"] != "arn:aws:ecs:us-east-1:123456:cluster/my-cluster" {
					t.Errorf("expected ClusterArn, got '%v'", r.Details["ClusterArn"])
				}
				if r.Details["LastStatus"] != "RUNNING" {
					t.Errorf("expected LastStatus 'RUNNING', got '%v'", r.Details["LastStatus"])
				}
				if r.Details["DesiredStatus"] != "RUNNING" {
					t.Errorf("expected DesiredStatus 'RUNNING', got '%v'", r.Details["DesiredStatus"])
				}
				if r.Details["TaskDefinitionArn"] != "arn:aws:ecs:us-east-1:123456:task-definition/my-task:1" {
					t.Errorf("expected TaskDefinitionArn, got '%v'", r.Details["TaskDefinitionArn"])
				}
				if r.Details["Cpu"] != "256" {
					t.Errorf("expected Cpu '256', got '%v'", r.Details["Cpu"])
				}
				if r.Details["Memory"] != "512" {
					t.Errorf("expected Memory '512', got '%v'", r.Details["Memory"])
				}
				if r.Details["LaunchType"] != "FARGATE" {
					t.Errorf("expected LaunchType 'FARGATE', got '%v'", r.Details["LaunchType"])
				}
			},
		},
		{
			name: "multiple Fargate tasks",
			input: []*ecs.Task{
				{TaskArn: aws.String("arn:task-1"), LaunchType: aws.String("FARGATE")},
				{TaskArn: aws.String("arn:task-2"), LaunchType: aws.String("FARGATE")},
			},
			expectedCount: 2,
			checkResults: func(t *testing.T, results []types.ScanResult) {
				if results[0].ResourceName != "arn:task-1" {
					t.Errorf("expected first ResourceName 'arn:task-1', got '%s'", results[0].ResourceName)
				}
				if results[1].ResourceName != "arn:task-2" {
					t.Errorf("expected second ResourceName 'arn:task-2', got '%s'", results[1].ResourceName)
				}
			},
		},
		{
			name:          "empty results",
			input:         []*ecs.Task{},
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
				if results[0].ServiceName != "Fargate" {
					t.Errorf("expected ServiceName 'Fargate', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "ecs:ListFargateTasks" {
					t.Errorf("expected MethodName 'ecs:ListFargateTasks', got '%s'", results[0].MethodName)
				}
			},
		},
		{
			name: "nil field handling",
			input: []*ecs.Task{
				{
					TaskArn:           nil,
					ClusterArn:        nil,
					LastStatus:        nil,
					DesiredStatus:     nil,
					TaskDefinitionArn: nil,
					Cpu:               nil,
					Memory:            nil,
					LaunchType:        nil,
				},
			},
			expectedCount: 1,
			checkResults: func(t *testing.T, results []types.ScanResult) {
				r := results[0]
				if r.ResourceName != "" {
					t.Errorf("expected empty ResourceName for nil TaskArn, got '%s'", r.ResourceName)
				}
				if r.Details["ClusterArn"] != "" {
					t.Errorf("expected empty ClusterArn for nil, got '%v'", r.Details["ClusterArn"])
				}
				if r.Details["LastStatus"] != "" {
					t.Errorf("expected empty LastStatus for nil, got '%v'", r.Details["LastStatus"])
				}
				if r.Details["DesiredStatus"] != "" {
					t.Errorf("expected empty DesiredStatus for nil, got '%v'", r.Details["DesiredStatus"])
				}
				if r.Details["TaskDefinitionArn"] != "" {
					t.Errorf("expected empty TaskDefinitionArn for nil, got '%v'", r.Details["TaskDefinitionArn"])
				}
				if r.Details["Cpu"] != "" {
					t.Errorf("expected empty Cpu for nil, got '%v'", r.Details["Cpu"])
				}
				if r.Details["Memory"] != "" {
					t.Errorf("expected empty Memory for nil, got '%v'", r.Details["Memory"])
				}
				if r.Details["LaunchType"] != "" {
					t.Errorf("expected empty LaunchType for nil, got '%v'", r.Details["LaunchType"])
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
