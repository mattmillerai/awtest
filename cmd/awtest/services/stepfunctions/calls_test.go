package stepfunctions

import (
	"fmt"
	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sfn"
	"testing"
	"time"
)

func TestProcess(t *testing.T) {
	process := StepFunctionsCalls[0].Process

	tests := []struct {
		name          string
		input         interface{}
		err           error
		expectedCount int
		expectError   bool
		checkResults  func(t *testing.T, results []types.ScanResult)
	}{
		{
			name: "STANDARD state machine with all fields",
			input: []*sfn.StateMachineListItem{
				{
					Name:            aws.String("my-workflow"),
					StateMachineArn: aws.String("arn:aws:states:us-east-1:123456789:stateMachine:my-workflow"),
					Type:            aws.String("STANDARD"),
					CreationDate:    func() *time.Time { t := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC); return &t }(),
				},
			},
			expectedCount: 1,
			checkResults: func(t *testing.T, results []types.ScanResult) {
				r := results[0]
				if r.ServiceName != "Step Functions" {
					t.Errorf("expected ServiceName 'Step Functions', got '%s'", r.ServiceName)
				}
				if r.MethodName != "states:ListStateMachines" {
					t.Errorf("expected MethodName 'states:ListStateMachines', got '%s'", r.MethodName)
				}
				if r.ResourceType != "state-machine" {
					t.Errorf("expected ResourceType 'state-machine', got '%s'", r.ResourceType)
				}
				if r.ResourceName != "my-workflow" {
					t.Errorf("expected ResourceName 'my-workflow', got '%s'", r.ResourceName)
				}
				if r.Details["StateMachineArn"] != "arn:aws:states:us-east-1:123456789:stateMachine:my-workflow" {
					t.Errorf("expected StateMachineArn, got '%v'", r.Details["StateMachineArn"])
				}
				if r.Details["Type"] != "STANDARD" {
					t.Errorf("expected Type 'STANDARD', got '%v'", r.Details["Type"])
				}
				if r.Details["CreationDate"] != "2024-01-15 10:30:00" {
					t.Errorf("expected CreationDate '2024-01-15 10:30:00', got '%v'", r.Details["CreationDate"])
				}
			},
		},
		{
			name: "EXPRESS state machine",
			input: []*sfn.StateMachineListItem{
				{
					Name:            aws.String("express-workflow"),
					StateMachineArn: aws.String("arn:aws:states:us-west-2:123456789:stateMachine:express-workflow"),
					Type:            aws.String("EXPRESS"),
					CreationDate:    func() *time.Time { t := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC); return &t }(),
				},
			},
			expectedCount: 1,
			checkResults: func(t *testing.T, results []types.ScanResult) {
				if results[0].Details["Type"] != "EXPRESS" {
					t.Errorf("expected Type 'EXPRESS', got '%v'", results[0].Details["Type"])
				}
			},
		},
		{
			name: "multiple state machines",
			input: []*sfn.StateMachineListItem{
				{Name: aws.String("workflow-1"), Type: aws.String("STANDARD")},
				{Name: aws.String("workflow-2"), Type: aws.String("EXPRESS")},
			},
			expectedCount: 2,
			checkResults: func(t *testing.T, results []types.ScanResult) {
				if results[0].ResourceName != "workflow-1" {
					t.Errorf("expected first ResourceName 'workflow-1', got '%s'", results[0].ResourceName)
				}
				if results[1].ResourceName != "workflow-2" {
					t.Errorf("expected second ResourceName 'workflow-2', got '%s'", results[1].ResourceName)
				}
			},
		},
		{
			name:          "empty results",
			input:         []*sfn.StateMachineListItem{},
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
				if results[0].ServiceName != "Step Functions" {
					t.Errorf("expected ServiceName 'Step Functions', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "states:ListStateMachines" {
					t.Errorf("expected MethodName 'states:ListStateMachines', got '%s'", results[0].MethodName)
				}
			},
		},
		{
			name: "nil field handling",
			input: []*sfn.StateMachineListItem{
				{
					Name:            nil,
					StateMachineArn: nil,
					Type:            nil,
					CreationDate:    nil,
				},
			},
			expectedCount: 1,
			checkResults: func(t *testing.T, results []types.ScanResult) {
				r := results[0]
				if r.ResourceName != "" {
					t.Errorf("expected empty ResourceName for nil Name, got '%s'", r.ResourceName)
				}
				if r.Details["StateMachineArn"] != "" {
					t.Errorf("expected empty StateMachineArn for nil, got '%v'", r.Details["StateMachineArn"])
				}
				if r.Details["Type"] != "" {
					t.Errorf("expected empty Type for nil, got '%v'", r.Details["Type"])
				}
				if r.Details["CreationDate"] != "" {
					t.Errorf("expected empty CreationDate for nil, got '%v'", r.Details["CreationDate"])
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
