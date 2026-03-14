package organizations

import (
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/organizations"
)

func TestListAccountsProcess(t *testing.T) {
	process := OrganizationsCalls[0].Process

	joinedTime := time.Date(2023, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name                string
		output              interface{}
		err                 error
		wantLen             int
		wantError           bool
		wantResourceName    string
		wantAccountId       string
		wantStatus          string
		wantArn             string
		wantJoinedTimestamp string
	}{
		{
			name: "valid accounts",
			output: []*organizations.Account{
				{
					Id:              aws.String("123456789012"),
					Name:            aws.String("Production"),
					Email:           aws.String("prod@example.com"),
					Status:          aws.String("ACTIVE"),
					Arn:             aws.String("arn:aws:organizations::111111111111:account/o-abc123/123456789012"),
					JoinedTimestamp: &joinedTime,
				},
				{
					Id:     aws.String("987654321098"),
					Name:   aws.String("Development"),
					Email:  aws.String("dev@example.com"),
					Status: aws.String("ACTIVE"),
					Arn:    aws.String("arn:aws:organizations::111111111111:account/o-abc123/987654321098"),
				},
			},
			wantLen:             2,
			wantResourceName:    "Production",
			wantAccountId:       "123456789012",
			wantStatus:          "ACTIVE",
			wantArn:             "arn:aws:organizations::111111111111:account/o-abc123/123456789012",
			wantJoinedTimestamp: "2023-01-15T10:30:00Z",
		},
		{
			name:    "empty results",
			output:  []*organizations.Account{},
			wantLen: 0,
		},
		{
			name:      "access denied error",
			output:    nil,
			err:       fmt.Errorf("AccessDeniedException: User is not authorized"),
			wantLen:   1,
			wantError: true,
		},
		{
			name: "nil fields",
			output: []*organizations.Account{
				{
					Id:              nil,
					Name:            nil,
					Email:           nil,
					Status:          nil,
					Arn:             nil,
					JoinedTimestamp: nil,
				},
			},
			wantLen:             1,
			wantResourceName:    "",
			wantAccountId:       "",
			wantStatus:          "",
			wantArn:             "",
			wantJoinedTimestamp: "",
		},
		{
			name:    "type assertion failure",
			output:  "invalid type",
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := process(tt.output, tt.err, false)

			if len(results) != tt.wantLen {
				t.Fatalf("expected %d results, got %d", tt.wantLen, len(results))
			}

			if tt.wantError {
				if results[0].Error == nil {
					t.Error("expected error in result, got nil")
				}
				if results[0].ServiceName != "Organizations" {
					t.Errorf("expected ServiceName 'Organizations', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "organizations:ListAccounts" {
					t.Errorf("expected MethodName 'organizations:ListAccounts', got '%s'", results[0].MethodName)
				}
				return
			}

			if tt.wantLen > 0 {
				if results[0].ServiceName != "Organizations" {
					t.Errorf("expected ServiceName 'Organizations', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "organizations:ListAccounts" {
					t.Errorf("expected MethodName 'organizations:ListAccounts', got '%s'", results[0].MethodName)
				}
				if results[0].ResourceType != "account" {
					t.Errorf("expected ResourceType 'account', got '%s'", results[0].ResourceType)
				}
				if results[0].ResourceName != tt.wantResourceName {
					t.Errorf("expected ResourceName '%s', got '%s'", tt.wantResourceName, results[0].ResourceName)
				}
				if accountId, ok := results[0].Details["AccountId"].(string); ok {
					if accountId != tt.wantAccountId {
						t.Errorf("expected AccountId '%s', got '%s'", tt.wantAccountId, accountId)
					}
				} else if tt.wantAccountId != "" {
					t.Errorf("expected AccountId in Details, got none")
				}
				if status, ok := results[0].Details["Status"].(string); ok {
					if status != tt.wantStatus {
						t.Errorf("expected Status '%s', got '%s'", tt.wantStatus, status)
					}
				} else if tt.wantStatus != "" {
					t.Errorf("expected Status in Details, got none")
				}
				if arn, ok := results[0].Details["Arn"].(string); ok {
					if arn != tt.wantArn {
						t.Errorf("expected Arn '%s', got '%s'", tt.wantArn, arn)
					}
				} else if tt.wantArn != "" {
					t.Errorf("expected Arn in Details, got none")
				}
				if joined, ok := results[0].Details["JoinedTimestamp"].(string); ok {
					if joined != tt.wantJoinedTimestamp {
						t.Errorf("expected JoinedTimestamp '%s', got '%s'", tt.wantJoinedTimestamp, joined)
					}
				} else if tt.wantJoinedTimestamp != "" {
					t.Errorf("expected JoinedTimestamp in Details, got none")
				}
			}
		})
	}
}

func TestListOrganizationalUnitsProcess(t *testing.T) {
	process := OrganizationsCalls[1].Process

	tests := []struct {
		name             string
		output           interface{}
		err              error
		wantLen          int
		wantError        bool
		wantResourceName string
		wantOUId         string
		wantParentId     string
	}{
		{
			name: "valid OUs with hierarchy",
			output: []orgOU{
				{
					OUId:     "ou-abc1-12345678",
					OUName:   "Engineering",
					OUArn:    "arn:aws:organizations::111111111111:ou/o-abc123/ou-abc1-12345678",
					ParentId: "r-abc1",
				},
				{
					OUId:     "ou-abc1-87654321",
					OUName:   "Security",
					OUArn:    "arn:aws:organizations::111111111111:ou/o-abc123/ou-abc1-87654321",
					ParentId: "r-abc1",
				},
			},
			wantLen:          2,
			wantResourceName: "Engineering",
			wantOUId:         "ou-abc1-12345678",
			wantParentId:     "r-abc1",
		},
		{
			name:    "empty OUs",
			output:  []orgOU{},
			wantLen: 0,
		},
		{
			name:      "access denied error",
			output:    nil,
			err:       fmt.Errorf("AccessDeniedException: User is not authorized"),
			wantLen:   1,
			wantError: true,
		},
		{
			name:    "type assertion failure",
			output:  "invalid type",
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := process(tt.output, tt.err, false)

			if len(results) != tt.wantLen {
				t.Fatalf("expected %d results, got %d", tt.wantLen, len(results))
			}

			if tt.wantError {
				if results[0].Error == nil {
					t.Error("expected error in result, got nil")
				}
				if results[0].ServiceName != "Organizations" {
					t.Errorf("expected ServiceName 'Organizations', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "organizations:ListOrganizationalUnits" {
					t.Errorf("expected MethodName 'organizations:ListOrganizationalUnits', got '%s'", results[0].MethodName)
				}
				return
			}

			if tt.wantLen > 0 {
				if results[0].ServiceName != "Organizations" {
					t.Errorf("expected ServiceName 'Organizations', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "organizations:ListOrganizationalUnits" {
					t.Errorf("expected MethodName 'organizations:ListOrganizationalUnits', got '%s'", results[0].MethodName)
				}
				if results[0].ResourceType != "organizational-unit" {
					t.Errorf("expected ResourceType 'organizational-unit', got '%s'", results[0].ResourceType)
				}
				if results[0].ResourceName != tt.wantResourceName {
					t.Errorf("expected ResourceName '%s', got '%s'", tt.wantResourceName, results[0].ResourceName)
				}
				if ouId, ok := results[0].Details["OUId"].(string); ok {
					if ouId != tt.wantOUId {
						t.Errorf("expected OUId '%s', got '%s'", tt.wantOUId, ouId)
					}
				} else if tt.wantOUId != "" {
					t.Errorf("expected OUId in Details, got none")
				}
				if parentId, ok := results[0].Details["ParentId"].(string); ok {
					if parentId != tt.wantParentId {
						t.Errorf("expected ParentId '%s', got '%s'", tt.wantParentId, parentId)
					}
				} else if tt.wantParentId != "" {
					t.Errorf("expected ParentId in Details, got none")
				}
			}
		})
	}
}

func TestListPoliciesProcess(t *testing.T) {
	process := OrganizationsCalls[2].Process

	tests := []struct {
		name             string
		output           interface{}
		err              error
		wantLen          int
		wantError        bool
		wantResourceName string
		wantPolicyId     string
		wantArn          string
	}{
		{
			name: "valid SCPs",
			output: []*organizations.PolicySummary{
				{
					Id:   aws.String("p-abc12345"),
					Name: aws.String("FullAWSAccess"),
					Arn:  aws.String("arn:aws:organizations::111111111111:policy/o-abc123/service_control_policy/p-abc12345"),
				},
				{
					Id:   aws.String("p-def67890"),
					Name: aws.String("DenyS3Delete"),
					Arn:  aws.String("arn:aws:organizations::111111111111:policy/o-abc123/service_control_policy/p-def67890"),
				},
			},
			wantLen:          2,
			wantResourceName: "FullAWSAccess",
			wantPolicyId:     "p-abc12345",
			wantArn:          "arn:aws:organizations::111111111111:policy/o-abc123/service_control_policy/p-abc12345",
		},
		{
			name:    "empty policies",
			output:  []*organizations.PolicySummary{},
			wantLen: 0,
		},
		{
			name:      "access denied error",
			output:    nil,
			err:       fmt.Errorf("AccessDeniedException: User is not authorized"),
			wantLen:   1,
			wantError: true,
		},
		{
			name: "nil fields",
			output: []*organizations.PolicySummary{
				{
					Id:   nil,
					Name: nil,
					Arn:  nil,
				},
			},
			wantLen:          1,
			wantResourceName: "",
			wantPolicyId:     "",
			wantArn:          "",
		},
		{
			name:    "type assertion failure",
			output:  "invalid type",
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := process(tt.output, tt.err, false)

			if len(results) != tt.wantLen {
				t.Fatalf("expected %d results, got %d", tt.wantLen, len(results))
			}

			if tt.wantError {
				if results[0].Error == nil {
					t.Error("expected error in result, got nil")
				}
				if results[0].ServiceName != "Organizations" {
					t.Errorf("expected ServiceName 'Organizations', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "organizations:ListPolicies" {
					t.Errorf("expected MethodName 'organizations:ListPolicies', got '%s'", results[0].MethodName)
				}
				return
			}

			if tt.wantLen > 0 {
				if results[0].ServiceName != "Organizations" {
					t.Errorf("expected ServiceName 'Organizations', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "organizations:ListPolicies" {
					t.Errorf("expected MethodName 'organizations:ListPolicies', got '%s'", results[0].MethodName)
				}
				if results[0].ResourceType != "service-control-policy" {
					t.Errorf("expected ResourceType 'service-control-policy', got '%s'", results[0].ResourceType)
				}
				if results[0].ResourceName != tt.wantResourceName {
					t.Errorf("expected ResourceName '%s', got '%s'", tt.wantResourceName, results[0].ResourceName)
				}
				if policyId, ok := results[0].Details["PolicyId"].(string); ok {
					if policyId != tt.wantPolicyId {
						t.Errorf("expected PolicyId '%s', got '%s'", tt.wantPolicyId, policyId)
					}
				} else if tt.wantPolicyId != "" {
					t.Errorf("expected PolicyId in Details, got none")
				}
				if arn, ok := results[0].Details["Arn"].(string); ok {
					if arn != tt.wantArn {
						t.Errorf("expected Arn '%s', got '%s'", tt.wantArn, arn)
					}
				} else if tt.wantArn != "" {
					t.Errorf("expected Arn in Details, got none")
				}
			}
		})
	}
}
