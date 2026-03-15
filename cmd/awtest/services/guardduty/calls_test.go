package guardduty

import (
	"fmt"
	"reflect"
	"testing"
)

func TestListDetectorsProcess(t *testing.T) {
	process := GuardDutyCalls[0].Process

	tests := []struct {
		name             string
		output           interface{}
		err              error
		wantLen          int
		wantError        bool
		wantResourceName string
		wantDetectorId   string
		wantStatus       string
		wantServiceRole  string
		wantRegion       string
		wantFeatures     []string
	}{
		{
			name: "valid detectors",
			output: []guarddutyDetector{
				{
					DetectorId:  "abc123def456",
					Status:      "ENABLED",
					ServiceRole: "arn:aws:iam::111111111111:role/aws-service-role/guardduty.amazonaws.com/AWSServiceRoleForAmazonGuardDuty",
					CreatedAt:   "2023-01-15T10:30:00Z",
					Region:      "us-east-1",
					Features:    []string{"FLOW_LOGS", "CLOUD_TRAIL", "S3_DATA_EVENTS"},
				},
				{
					DetectorId:  "xyz789ghi012",
					Status:      "DISABLED",
					ServiceRole: "arn:aws:iam::111111111111:role/aws-service-role/guardduty.amazonaws.com/AWSServiceRoleForAmazonGuardDuty",
					CreatedAt:   "2023-06-01T08:00:00Z",
					Region:      "us-west-2",
					Features:    nil,
				},
			},
			wantLen:          2,
			wantResourceName: "abc123def456",
			wantDetectorId:   "abc123def456",
			wantStatus:       "ENABLED",
			wantServiceRole:  "arn:aws:iam::111111111111:role/aws-service-role/guardduty.amazonaws.com/AWSServiceRoleForAmazonGuardDuty",
			wantRegion:       "us-east-1",
			wantFeatures:     []string{"FLOW_LOGS", "CLOUD_TRAIL", "S3_DATA_EVENTS"},
		},
		{
			name:    "empty results",
			output:  []guarddutyDetector{},
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
			name: "nil-safe fields (empty strings)",
			output: []guarddutyDetector{
				{
					DetectorId:  "",
					Status:      "",
					ServiceRole: "",
					CreatedAt:   "",
					Region:      "",
					Features:    nil,
				},
			},
			wantLen:          1,
			wantResourceName: "",
			wantDetectorId:   "",
			wantStatus:       "",
			wantServiceRole:  "",
			wantRegion:       "",
			wantFeatures:     nil,
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
				if results[0].ServiceName != "GuardDuty" {
					t.Errorf("expected ServiceName 'GuardDuty', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "guardduty:ListDetectors" {
					t.Errorf("expected MethodName 'guardduty:ListDetectors', got '%s'", results[0].MethodName)
				}
				return
			}

			if tt.wantLen > 0 {
				if results[0].ServiceName != "GuardDuty" {
					t.Errorf("expected ServiceName 'GuardDuty', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "guardduty:ListDetectors" {
					t.Errorf("expected MethodName 'guardduty:ListDetectors', got '%s'", results[0].MethodName)
				}
				if results[0].ResourceType != "detector" {
					t.Errorf("expected ResourceType 'detector', got '%s'", results[0].ResourceType)
				}
				if results[0].ResourceName != tt.wantResourceName {
					t.Errorf("expected ResourceName '%s', got '%s'", tt.wantResourceName, results[0].ResourceName)
				}
				if detectorId, ok := results[0].Details["DetectorId"].(string); ok {
					if detectorId != tt.wantDetectorId {
						t.Errorf("expected DetectorId '%s', got '%s'", tt.wantDetectorId, detectorId)
					}
				} else if tt.wantDetectorId != "" {
					t.Errorf("expected DetectorId in Details, got none")
				}
				if status, ok := results[0].Details["Status"].(string); ok {
					if status != tt.wantStatus {
						t.Errorf("expected Status '%s', got '%s'", tt.wantStatus, status)
					}
				} else if tt.wantStatus != "" {
					t.Errorf("expected Status in Details, got none")
				}
				if serviceRole, ok := results[0].Details["ServiceRole"].(string); ok {
					if serviceRole != tt.wantServiceRole {
						t.Errorf("expected ServiceRole '%s', got '%s'", tt.wantServiceRole, serviceRole)
					}
				} else if tt.wantServiceRole != "" {
					t.Errorf("expected ServiceRole in Details, got none")
				}
				if region, ok := results[0].Details["Region"].(string); ok {
					if region != tt.wantRegion {
						t.Errorf("expected Region '%s', got '%s'", tt.wantRegion, region)
					}
				} else if tt.wantRegion != "" {
					t.Errorf("expected Region in Details, got none")
				}
				if features, ok := results[0].Details["Features"].([]string); ok {
					if !reflect.DeepEqual(features, tt.wantFeatures) {
						t.Errorf("expected Features %v, got %v", tt.wantFeatures, features)
					}
				} else if tt.wantFeatures != nil {
					t.Errorf("expected Features in Details, got none")
				}
			}
		})
	}
}

func TestGetFindingsProcess(t *testing.T) {
	process := GuardDutyCalls[1].Process

	tests := []struct {
		name             string
		output           interface{}
		err              error
		wantLen          int
		wantError        bool
		wantResourceName string
		wantFindingId    string
		wantType         string
		wantSeverity     float64
		wantRegion       string
		wantDetectorId   string
	}{
		{
			name: "valid findings with title",
			output: []guarddutyFinding{
				{
					FindingId:   "finding-abc-123",
					Type:        "Recon:EC2/PortProbeUnprotectedPort",
					Title:       "Unprotected port on EC2 instance is being probed",
					Severity:    8.0,
					Description: "EC2 instance has an unprotected port",
					Region:      "us-east-1",
					DetectorId:  "detector-abc-123",
				},
				{
					FindingId:   "finding-def-456",
					Type:        "UnauthorizedAccess:IAMUser/MaliciousIPCaller",
					Title:       "API was invoked from a malicious IP address",
					Severity:    5.0,
					Description: "An API was invoked from a known malicious IP",
					Region:      "us-west-2",
					DetectorId:  "detector-abc-123",
				},
			},
			wantLen:          2,
			wantResourceName: "Unprotected port on EC2 instance is being probed",
			wantFindingId:    "finding-abc-123",
			wantType:         "Recon:EC2/PortProbeUnprotectedPort",
			wantSeverity:     8.0,
			wantRegion:       "us-east-1",
			wantDetectorId:   "detector-abc-123",
		},
		{
			name: "finding with empty title uses type as resource name",
			output: []guarddutyFinding{
				{
					FindingId:   "finding-xyz-789",
					Type:        "Recon:EC2/Portscan",
					Title:       "",
					Severity:    2.0,
					Description: "Port scan detected",
					Region:      "us-east-1",
					DetectorId:  "detector-abc-123",
				},
			},
			wantLen:          1,
			wantResourceName: "Recon:EC2/Portscan",
			wantFindingId:    "finding-xyz-789",
			wantType:         "Recon:EC2/Portscan",
			wantSeverity:     2.0,
			wantRegion:       "us-east-1",
			wantDetectorId:   "detector-abc-123",
		},
		{
			name:    "empty findings",
			output:  []guarddutyFinding{},
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
			name: "nil-safe fields (empty/zero values)",
			output: []guarddutyFinding{
				{
					FindingId:   "",
					Type:        "",
					Title:       "",
					Severity:    0,
					Description: "",
					Region:      "",
					DetectorId:  "",
				},
			},
			wantLen:          1,
			wantResourceName: "",
			wantFindingId:    "",
			wantType:         "",
			wantSeverity:     0,
			wantRegion:       "",
			wantDetectorId:   "",
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
				if results[0].ServiceName != "GuardDuty" {
					t.Errorf("expected ServiceName 'GuardDuty', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "guardduty:GetFindings" {
					t.Errorf("expected MethodName 'guardduty:GetFindings', got '%s'", results[0].MethodName)
				}
				return
			}

			if tt.wantLen > 0 {
				if results[0].ServiceName != "GuardDuty" {
					t.Errorf("expected ServiceName 'GuardDuty', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "guardduty:GetFindings" {
					t.Errorf("expected MethodName 'guardduty:GetFindings', got '%s'", results[0].MethodName)
				}
				if results[0].ResourceType != "finding" {
					t.Errorf("expected ResourceType 'finding', got '%s'", results[0].ResourceType)
				}
				if results[0].ResourceName != tt.wantResourceName {
					t.Errorf("expected ResourceName '%s', got '%s'", tt.wantResourceName, results[0].ResourceName)
				}
				if findingId, ok := results[0].Details["FindingId"].(string); ok {
					if findingId != tt.wantFindingId {
						t.Errorf("expected FindingId '%s', got '%s'", tt.wantFindingId, findingId)
					}
				} else if tt.wantFindingId != "" {
					t.Errorf("expected FindingId in Details, got none")
				}
				if findingType, ok := results[0].Details["Type"].(string); ok {
					if findingType != tt.wantType {
						t.Errorf("expected Type '%s', got '%s'", tt.wantType, findingType)
					}
				} else if tt.wantType != "" {
					t.Errorf("expected Type in Details, got none")
				}
				if severity, ok := results[0].Details["Severity"].(float64); ok {
					if severity != tt.wantSeverity {
						t.Errorf("expected Severity '%f', got '%f'", tt.wantSeverity, severity)
					}
				}
				if region, ok := results[0].Details["Region"].(string); ok {
					if region != tt.wantRegion {
						t.Errorf("expected Region '%s', got '%s'", tt.wantRegion, region)
					}
				} else if tt.wantRegion != "" {
					t.Errorf("expected Region in Details, got none")
				}
				if detectorId, ok := results[0].Details["DetectorId"].(string); ok {
					if detectorId != tt.wantDetectorId {
						t.Errorf("expected DetectorId '%s', got '%s'", tt.wantDetectorId, detectorId)
					}
				} else if tt.wantDetectorId != "" {
					t.Errorf("expected DetectorId in Details, got none")
				}
			}
		})
	}
}

func TestListFiltersProcess(t *testing.T) {
	process := GuardDutyCalls[2].Process

	tests := []struct {
		name             string
		output           interface{}
		err              error
		wantLen          int
		wantError        bool
		wantResourceName string
		wantFilterName   string
		wantAction       string
		wantDescription  string
		wantDetectorId   string
		wantRegion       string
	}{
		{
			name: "valid filters",
			output: []guarddutyFilter{
				{
					FilterName:  "SuppressLowSeverity",
					Action:      "ARCHIVE",
					Description: "Suppress low severity findings",
					DetectorId:  "detector-abc-123",
					Region:      "us-east-1",
				},
				{
					FilterName:  "NotifyHighSeverity",
					Action:      "NOOP",
					Description: "Notify on high severity findings",
					DetectorId:  "detector-abc-123",
					Region:      "us-east-1",
				},
			},
			wantLen:          2,
			wantResourceName: "SuppressLowSeverity",
			wantFilterName:   "SuppressLowSeverity",
			wantAction:       "ARCHIVE",
			wantDescription:  "Suppress low severity findings",
			wantDetectorId:   "detector-abc-123",
			wantRegion:       "us-east-1",
		},
		{
			name:    "empty filters",
			output:  []guarddutyFilter{},
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
			name: "nil-safe fields (empty strings)",
			output: []guarddutyFilter{
				{
					FilterName:  "",
					Action:      "",
					Description: "",
					DetectorId:  "",
					Region:      "",
				},
			},
			wantLen:          1,
			wantResourceName: "",
			wantFilterName:   "",
			wantAction:       "",
			wantDescription:  "",
			wantDetectorId:   "",
			wantRegion:       "",
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
				if results[0].ServiceName != "GuardDuty" {
					t.Errorf("expected ServiceName 'GuardDuty', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "guardduty:ListFilters" {
					t.Errorf("expected MethodName 'guardduty:ListFilters', got '%s'", results[0].MethodName)
				}
				return
			}

			if tt.wantLen > 0 {
				if results[0].ServiceName != "GuardDuty" {
					t.Errorf("expected ServiceName 'GuardDuty', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "guardduty:ListFilters" {
					t.Errorf("expected MethodName 'guardduty:ListFilters', got '%s'", results[0].MethodName)
				}
				if results[0].ResourceType != "filter" {
					t.Errorf("expected ResourceType 'filter', got '%s'", results[0].ResourceType)
				}
				if results[0].ResourceName != tt.wantResourceName {
					t.Errorf("expected ResourceName '%s', got '%s'", tt.wantResourceName, results[0].ResourceName)
				}
				if filterName, ok := results[0].Details["FilterName"].(string); ok {
					if filterName != tt.wantFilterName {
						t.Errorf("expected FilterName '%s', got '%s'", tt.wantFilterName, filterName)
					}
				} else if tt.wantFilterName != "" {
					t.Errorf("expected FilterName in Details, got none")
				}
				if action, ok := results[0].Details["Action"].(string); ok {
					if action != tt.wantAction {
						t.Errorf("expected Action '%s', got '%s'", tt.wantAction, action)
					}
				} else if tt.wantAction != "" {
					t.Errorf("expected Action in Details, got none")
				}
				if description, ok := results[0].Details["Description"].(string); ok {
					if description != tt.wantDescription {
						t.Errorf("expected Description '%s', got '%s'", tt.wantDescription, description)
					}
				} else if tt.wantDescription != "" {
					t.Errorf("expected Description in Details, got none")
				}
				if detectorId, ok := results[0].Details["DetectorId"].(string); ok {
					if detectorId != tt.wantDetectorId {
						t.Errorf("expected DetectorId '%s', got '%s'", tt.wantDetectorId, detectorId)
					}
				} else if tt.wantDetectorId != "" {
					t.Errorf("expected DetectorId in Details, got none")
				}
				if region, ok := results[0].Details["Region"].(string); ok {
					if region != tt.wantRegion {
						t.Errorf("expected Region '%s', got '%s'", tt.wantRegion, region)
					}
				} else if tt.wantRegion != "" {
					t.Errorf("expected Region in Details, got none")
				}
			}
		})
	}
}
