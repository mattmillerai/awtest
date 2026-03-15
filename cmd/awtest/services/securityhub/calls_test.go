package securityhub

import (
	"fmt"
	"testing"
)

func TestGetEnabledStandardsProcess(t *testing.T) {
	process := SecurityHubCalls[0].Process

	tests := []struct {
		name                     string
		output                   interface{}
		err                      error
		wantLen                  int
		wantError                bool
		wantResourceName         string
		wantStandardsSubscription string
		wantStandardsArn         string
		wantStandardsStatus      string
		wantRegion               string
	}{
		{
			name: "valid standards",
			output: []shStandard{
				{
					StandardsSubscriptionArn: "arn:aws:securityhub:us-east-1:111111111111:subscription/aws-foundational-security-best-practices/v/1.0.0",
					StandardsArn:             "arn:aws:securityhub:::standards/aws-foundational-security-best-practices/v/1.0.0",
					StandardsStatus:          "READY",
					Region:                   "us-east-1",
				},
				{
					StandardsSubscriptionArn: "arn:aws:securityhub:us-east-1:111111111111:subscription/cis-aws-foundations-benchmark/v/1.2.0",
					StandardsArn:             "arn:aws:securityhub:::standards/cis-aws-foundations-benchmark/v/1.2.0",
					StandardsStatus:          "INCOMPLETE",
					Region:                   "us-east-1",
				},
			},
			wantLen:                  2,
			wantResourceName:         "aws-foundational-security-best-practices",
			wantStandardsSubscription: "arn:aws:securityhub:us-east-1:111111111111:subscription/aws-foundational-security-best-practices/v/1.0.0",
			wantStandardsArn:         "arn:aws:securityhub:::standards/aws-foundational-security-best-practices/v/1.0.0",
			wantStandardsStatus:      "READY",
			wantRegion:               "us-east-1",
		},
		{
			name:    "empty results",
			output:  []shStandard{},
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
			output: []shStandard{
				{
					StandardsSubscriptionArn: "",
					StandardsArn:             "",
					StandardsStatus:          "",
					Region:                   "",
				},
			},
			wantLen:                  1,
			wantResourceName:         "",
			wantStandardsSubscription: "",
			wantStandardsArn:         "",
			wantStandardsStatus:      "",
			wantRegion:               "",
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
				if results[0].ServiceName != "SecurityHub" {
					t.Errorf("expected ServiceName 'SecurityHub', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "securityhub:GetEnabledStandards" {
					t.Errorf("expected MethodName 'securityhub:GetEnabledStandards', got '%s'", results[0].MethodName)
				}
				return
			}

			if tt.wantLen > 0 {
				if results[0].ServiceName != "SecurityHub" {
					t.Errorf("expected ServiceName 'SecurityHub', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "securityhub:GetEnabledStandards" {
					t.Errorf("expected MethodName 'securityhub:GetEnabledStandards', got '%s'", results[0].MethodName)
				}
				if results[0].ResourceType != "standard" {
					t.Errorf("expected ResourceType 'standard', got '%s'", results[0].ResourceType)
				}
				if results[0].ResourceName != tt.wantResourceName {
					t.Errorf("expected ResourceName '%s', got '%s'", tt.wantResourceName, results[0].ResourceName)
				}
				if sub, ok := results[0].Details["StandardsSubscriptionArn"].(string); ok {
					if sub != tt.wantStandardsSubscription {
						t.Errorf("expected StandardsSubscriptionArn '%s', got '%s'", tt.wantStandardsSubscription, sub)
					}
				} else if tt.wantStandardsSubscription != "" {
					t.Errorf("expected StandardsSubscriptionArn in Details, got none")
				}
				if arn, ok := results[0].Details["StandardsArn"].(string); ok {
					if arn != tt.wantStandardsArn {
						t.Errorf("expected StandardsArn '%s', got '%s'", tt.wantStandardsArn, arn)
					}
				} else if tt.wantStandardsArn != "" {
					t.Errorf("expected StandardsArn in Details, got none")
				}
				if status, ok := results[0].Details["StandardsStatus"].(string); ok {
					if status != tt.wantStandardsStatus {
						t.Errorf("expected StandardsStatus '%s', got '%s'", tt.wantStandardsStatus, status)
					}
				} else if tt.wantStandardsStatus != "" {
					t.Errorf("expected StandardsStatus in Details, got none")
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

func TestGetFindingsProcess(t *testing.T) {
	process := SecurityHubCalls[1].Process

	tests := []struct {
		name               string
		output             interface{}
		err                error
		wantLen            int
		wantError          bool
		wantResourceName   string
		wantId             string
		wantTitle          string
		wantSeverityLabel  string
		wantCompliance     string
		wantProductName    string
		wantResourceType   string
		wantRegion         string
		wantGeneratorId    string
	}{
		{
			name: "valid findings with severity and compliance",
			output: []shFinding{
				{
					Id:               "arn:aws:securityhub:us-east-1:111111111111:finding/abc-123",
					Title:            "S3 bucket does not have server-side encryption enabled",
					SeverityLabel:    "HIGH",
					ComplianceStatus: "FAILED",
					ProductName:      "Security Hub",
					ResourceType:     "AwsS3Bucket",
					Region:           "us-east-1",
					GeneratorId:      "aws-foundational-security-best-practices/v/1.0.0/S3.4",
				},
				{
					Id:               "arn:aws:securityhub:us-west-2:111111111111:finding/def-456",
					Title:            "IAM root user access key should not exist",
					SeverityLabel:    "CRITICAL",
					ComplianceStatus: "FAILED",
					ProductName:      "Security Hub",
					ResourceType:     "AwsAccount",
					Region:           "us-west-2",
					GeneratorId:      "aws-foundational-security-best-practices/v/1.0.0/IAM.4",
				},
			},
			wantLen:            2,
			wantResourceName:   "S3 bucket does not have server-side encryption enabled",
			wantId:             "arn:aws:securityhub:us-east-1:111111111111:finding/abc-123",
			wantTitle:          "S3 bucket does not have server-side encryption enabled",
			wantSeverityLabel:  "HIGH",
			wantCompliance:     "FAILED",
			wantProductName:    "Security Hub",
			wantResourceType:   "AwsS3Bucket",
			wantRegion:         "us-east-1",
			wantGeneratorId:    "aws-foundational-security-best-practices/v/1.0.0/S3.4",
		},
		{
			name: "finding with empty title uses generatorId",
			output: []shFinding{
				{
					Id:               "arn:aws:securityhub:us-east-1:111111111111:finding/xyz-789",
					Title:            "",
					SeverityLabel:    "MEDIUM",
					ComplianceStatus: "WARNING",
					ProductName:      "GuardDuty",
					ResourceType:     "AwsEc2Instance",
					Region:           "us-east-1",
					GeneratorId:      "arn:aws:guardduty:us-east-1:111111111111:detector/abc123",
				},
			},
			wantLen:            1,
			wantResourceName:   "arn:aws:guardduty:us-east-1:111111111111:detector/abc123",
			wantId:             "arn:aws:securityhub:us-east-1:111111111111:finding/xyz-789",
			wantTitle:          "",
			wantSeverityLabel:  "MEDIUM",
			wantCompliance:     "WARNING",
			wantProductName:    "GuardDuty",
			wantResourceType:   "AwsEc2Instance",
			wantRegion:         "us-east-1",
			wantGeneratorId:    "arn:aws:guardduty:us-east-1:111111111111:detector/abc123",
		},
		{
			name:    "empty findings",
			output:  []shFinding{},
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
			output: []shFinding{
				{
					Id:               "",
					Title:            "",
					SeverityLabel:    "",
					ComplianceStatus: "",
					ProductName:      "",
					ResourceType:     "",
					Region:           "",
					GeneratorId:      "",
				},
			},
			wantLen:            1,
			wantResourceName:   "",
			wantId:             "",
			wantTitle:          "",
			wantSeverityLabel:  "",
			wantCompliance:     "",
			wantProductName:    "",
			wantResourceType:   "",
			wantRegion:         "",
			wantGeneratorId:    "",
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
				if results[0].ServiceName != "SecurityHub" {
					t.Errorf("expected ServiceName 'SecurityHub', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "securityhub:GetFindings" {
					t.Errorf("expected MethodName 'securityhub:GetFindings', got '%s'", results[0].MethodName)
				}
				return
			}

			if tt.wantLen > 0 {
				if results[0].ServiceName != "SecurityHub" {
					t.Errorf("expected ServiceName 'SecurityHub', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "securityhub:GetFindings" {
					t.Errorf("expected MethodName 'securityhub:GetFindings', got '%s'", results[0].MethodName)
				}
				if results[0].ResourceType != "finding" {
					t.Errorf("expected ResourceType 'finding', got '%s'", results[0].ResourceType)
				}
				if results[0].ResourceName != tt.wantResourceName {
					t.Errorf("expected ResourceName '%s', got '%s'", tt.wantResourceName, results[0].ResourceName)
				}
				if id, ok := results[0].Details["Id"].(string); ok {
					if id != tt.wantId {
						t.Errorf("expected Id '%s', got '%s'", tt.wantId, id)
					}
				} else if tt.wantId != "" {
					t.Errorf("expected Id in Details, got none")
				}
				if title, ok := results[0].Details["Title"].(string); ok {
					if title != tt.wantTitle {
						t.Errorf("expected Title '%s', got '%s'", tt.wantTitle, title)
					}
				} else if tt.wantTitle != "" {
					t.Errorf("expected Title in Details, got none")
				}
				if severity, ok := results[0].Details["SeverityLabel"].(string); ok {
					if severity != tt.wantSeverityLabel {
						t.Errorf("expected SeverityLabel '%s', got '%s'", tt.wantSeverityLabel, severity)
					}
				} else if tt.wantSeverityLabel != "" {
					t.Errorf("expected SeverityLabel in Details, got none")
				}
				if compliance, ok := results[0].Details["ComplianceStatus"].(string); ok {
					if compliance != tt.wantCompliance {
						t.Errorf("expected ComplianceStatus '%s', got '%s'", tt.wantCompliance, compliance)
					}
				} else if tt.wantCompliance != "" {
					t.Errorf("expected ComplianceStatus in Details, got none")
				}
				if productName, ok := results[0].Details["ProductName"].(string); ok {
					if productName != tt.wantProductName {
						t.Errorf("expected ProductName '%s', got '%s'", tt.wantProductName, productName)
					}
				} else if tt.wantProductName != "" {
					t.Errorf("expected ProductName in Details, got none")
				}
				if resourceType, ok := results[0].Details["ResourceType"].(string); ok {
					if resourceType != tt.wantResourceType {
						t.Errorf("expected ResourceType '%s', got '%s'", tt.wantResourceType, resourceType)
					}
				} else if tt.wantResourceType != "" {
					t.Errorf("expected ResourceType in Details, got none")
				}
				if region, ok := results[0].Details["Region"].(string); ok {
					if region != tt.wantRegion {
						t.Errorf("expected Region '%s', got '%s'", tt.wantRegion, region)
					}
				} else if tt.wantRegion != "" {
					t.Errorf("expected Region in Details, got none")
				}
				if generatorId, ok := results[0].Details["GeneratorId"].(string); ok {
					if generatorId != tt.wantGeneratorId {
						t.Errorf("expected GeneratorId '%s', got '%s'", tt.wantGeneratorId, generatorId)
					}
				} else if tt.wantGeneratorId != "" {
					t.Errorf("expected GeneratorId in Details, got none")
				}
			}
		})
	}
}

func TestListEnabledProductsProcess(t *testing.T) {
	process := SecurityHubCalls[2].Process

	tests := []struct {
		name             string
		output           interface{}
		err              error
		wantLen          int
		wantError        bool
		wantResourceName string
		wantProductArn   string
		wantRegion       string
	}{
		{
			name: "valid products",
			output: []shProduct{
				{
					ProductSubscriptionArn: "arn:aws:securityhub:us-east-1:111111111111:product-subscription/aws/securityhub",
					Region:                 "us-east-1",
				},
				{
					ProductSubscriptionArn: "arn:aws:securityhub:us-east-1:111111111111:product-subscription/aws/guardduty",
					Region:                 "us-east-1",
				},
			},
			wantLen:          2,
			wantResourceName: "arn:aws:securityhub:us-east-1:111111111111:product-subscription/aws/securityhub",
			wantProductArn:   "arn:aws:securityhub:us-east-1:111111111111:product-subscription/aws/securityhub",
			wantRegion:       "us-east-1",
		},
		{
			name:    "empty products",
			output:  []shProduct{},
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
			output: []shProduct{
				{
					ProductSubscriptionArn: "",
					Region:                 "",
				},
			},
			wantLen:          1,
			wantResourceName: "",
			wantProductArn:   "",
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
				if results[0].ServiceName != "SecurityHub" {
					t.Errorf("expected ServiceName 'SecurityHub', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "securityhub:ListEnabledProductsForImport" {
					t.Errorf("expected MethodName 'securityhub:ListEnabledProductsForImport', got '%s'", results[0].MethodName)
				}
				return
			}

			if tt.wantLen > 0 {
				if results[0].ServiceName != "SecurityHub" {
					t.Errorf("expected ServiceName 'SecurityHub', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "securityhub:ListEnabledProductsForImport" {
					t.Errorf("expected MethodName 'securityhub:ListEnabledProductsForImport', got '%s'", results[0].MethodName)
				}
				if results[0].ResourceType != "product" {
					t.Errorf("expected ResourceType 'product', got '%s'", results[0].ResourceType)
				}
				if results[0].ResourceName != tt.wantResourceName {
					t.Errorf("expected ResourceName '%s', got '%s'", tt.wantResourceName, results[0].ResourceName)
				}
				if productArn, ok := results[0].Details["ProductSubscriptionArn"].(string); ok {
					if productArn != tt.wantProductArn {
						t.Errorf("expected ProductSubscriptionArn '%s', got '%s'", tt.wantProductArn, productArn)
					}
				} else if tt.wantProductArn != "" {
					t.Errorf("expected ProductSubscriptionArn in Details, got none")
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
