package codedeploy

import (
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/codedeploy"
)

func TestListApplicationsProcess(t *testing.T) {
	process := CodeDeployCalls[0].Process

	tests := []struct {
		name             string
		output           interface{}
		err              error
		wantLen          int
		wantError        bool
		wantResourceName string
		wantName         string
		wantAppId        string
		wantPlatform     string
		wantCreateTime   string
		wantLinkedGH     string
		wantRegion       string
	}{
		{
			name: "valid applications with full details",
			output: []cdApplication{
				{
					Name:            "my-web-app",
					ApplicationId:   "app-12345-abcde",
					ComputePlatform: "Server",
					CreateTime:      "2026-01-15T10:00:00Z",
					LinkedToGitHub:  "true",
					Region:          "us-east-1",
				},
				{
					Name:            "my-lambda-app",
					ApplicationId:   "app-67890-fghij",
					ComputePlatform: "Lambda",
					CreateTime:      "2026-02-20T14:00:00Z",
					LinkedToGitHub:  "false",
					Region:          "us-west-2",
				},
			},
			wantLen:          2,
			wantResourceName: "my-web-app",
			wantName:         "my-web-app",
			wantAppId:        "app-12345-abcde",
			wantPlatform:     "Server",
			wantCreateTime:   "2026-01-15T10:00:00Z",
			wantLinkedGH:     "true",
			wantRegion:       "us-east-1",
		},
		{
			name:    "empty results",
			output:  []cdApplication{},
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
			output: []cdApplication{
				{
					Name:            "",
					ApplicationId:   "",
					ComputePlatform: "",
					CreateTime:      "",
					LinkedToGitHub:  "",
					Region:          "",
				},
			},
			wantLen:          1,
			wantResourceName: "",
			wantName:         "",
			wantAppId:        "",
			wantPlatform:     "",
			wantCreateTime:   "",
			wantLinkedGH:     "",
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
				if results[0].ServiceName != "CodeDeploy" {
					t.Errorf("expected ServiceName 'CodeDeploy', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "codedeploy:ListApplications" {
					t.Errorf("expected MethodName 'codedeploy:ListApplications', got '%s'", results[0].MethodName)
				}
				return
			}

			if tt.wantLen > 0 {
				if results[0].ServiceName != "CodeDeploy" {
					t.Errorf("expected ServiceName 'CodeDeploy', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "codedeploy:ListApplications" {
					t.Errorf("expected MethodName 'codedeploy:ListApplications', got '%s'", results[0].MethodName)
				}
				if results[0].ResourceType != "application" {
					t.Errorf("expected ResourceType 'application', got '%s'", results[0].ResourceType)
				}
				if results[0].ResourceName != tt.wantResourceName {
					t.Errorf("expected ResourceName '%s', got '%s'", tt.wantResourceName, results[0].ResourceName)
				}
				if name, ok := results[0].Details["Name"].(string); ok {
					if name != tt.wantName {
						t.Errorf("expected Name '%s', got '%s'", tt.wantName, name)
					}
				} else if tt.wantName != "" {
					t.Errorf("expected Name in Details, got none")
				}
				if appId, ok := results[0].Details["ApplicationId"].(string); ok {
					if appId != tt.wantAppId {
						t.Errorf("expected ApplicationId '%s', got '%s'", tt.wantAppId, appId)
					}
				} else if tt.wantAppId != "" {
					t.Errorf("expected ApplicationId in Details, got none")
				}
				if platform, ok := results[0].Details["ComputePlatform"].(string); ok {
					if platform != tt.wantPlatform {
						t.Errorf("expected ComputePlatform '%s', got '%s'", tt.wantPlatform, platform)
					}
				} else if tt.wantPlatform != "" {
					t.Errorf("expected ComputePlatform in Details, got none")
				}
				if createTime, ok := results[0].Details["CreateTime"].(string); ok {
					if createTime != tt.wantCreateTime {
						t.Errorf("expected CreateTime '%s', got '%s'", tt.wantCreateTime, createTime)
					}
				} else if tt.wantCreateTime != "" {
					t.Errorf("expected CreateTime in Details, got none")
				}
				if linkedGH, ok := results[0].Details["LinkedToGitHub"].(string); ok {
					if linkedGH != tt.wantLinkedGH {
						t.Errorf("expected LinkedToGitHub '%s', got '%s'", tt.wantLinkedGH, linkedGH)
					}
				} else if tt.wantLinkedGH != "" {
					t.Errorf("expected LinkedToGitHub in Details, got none")
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

func TestListDeploymentGroupsProcess(t *testing.T) {
	process := CodeDeployCalls[1].Process

	tests := []struct {
		name             string
		output           interface{}
		err              error
		wantLen          int
		wantError        bool
		wantResourceName string
		wantAppName      string
		wantGroupName    string
		wantGroupId      string
		wantConfigName   string
		wantPlatform     string
		wantRoleArn      string
		wantRegion       string
	}{
		{
			name: "valid deployment groups with full details",
			output: []cdDeploymentGroup{
				{
					ApplicationName:      "my-web-app",
					GroupName:            "production",
					DeploymentGroupId:    "dg-12345-abcde",
					DeploymentConfigName: "CodeDeployDefault.OneAtATime",
					ComputePlatform:      "Server",
					ServiceRoleArn:       "arn:aws:iam::111111111111:role/CodeDeployRole",
					Region:               "us-east-1",
				},
				{
					ApplicationName:      "my-web-app",
					GroupName:            "staging",
					DeploymentGroupId:    "dg-67890-fghij",
					DeploymentConfigName: "CodeDeployDefault.AllAtOnce",
					ComputePlatform:      "Server",
					ServiceRoleArn:       "arn:aws:iam::111111111111:role/CodeDeployRole",
					Region:               "us-east-1",
				},
			},
			wantLen:          2,
			wantResourceName: "my-web-app/production",
			wantAppName:      "my-web-app",
			wantGroupName:    "production",
			wantGroupId:      "dg-12345-abcde",
			wantConfigName:   "CodeDeployDefault.OneAtATime",
			wantPlatform:     "Server",
			wantRoleArn:      "arn:aws:iam::111111111111:role/CodeDeployRole",
			wantRegion:       "us-east-1",
		},
		{
			name:    "empty results",
			output:  []cdDeploymentGroup{},
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
			output: []cdDeploymentGroup{
				{
					ApplicationName:      "",
					GroupName:            "",
					DeploymentGroupId:    "",
					DeploymentConfigName: "",
					ComputePlatform:      "",
					ServiceRoleArn:       "",
					Region:               "",
				},
			},
			wantLen:          1,
			wantResourceName: "",
			wantAppName:      "",
			wantGroupName:    "",
			wantGroupId:      "",
			wantConfigName:   "",
			wantPlatform:     "",
			wantRoleArn:      "",
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
				if results[0].ServiceName != "CodeDeploy" {
					t.Errorf("expected ServiceName 'CodeDeploy', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "codedeploy:ListDeploymentGroups" {
					t.Errorf("expected MethodName 'codedeploy:ListDeploymentGroups', got '%s'", results[0].MethodName)
				}
				return
			}

			if tt.wantLen > 0 {
				if results[0].ServiceName != "CodeDeploy" {
					t.Errorf("expected ServiceName 'CodeDeploy', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "codedeploy:ListDeploymentGroups" {
					t.Errorf("expected MethodName 'codedeploy:ListDeploymentGroups', got '%s'", results[0].MethodName)
				}
				if results[0].ResourceType != "deployment-group" {
					t.Errorf("expected ResourceType 'deployment-group', got '%s'", results[0].ResourceType)
				}
				if results[0].ResourceName != tt.wantResourceName {
					t.Errorf("expected ResourceName '%s', got '%s'", tt.wantResourceName, results[0].ResourceName)
				}
				if appName, ok := results[0].Details["ApplicationName"].(string); ok {
					if appName != tt.wantAppName {
						t.Errorf("expected ApplicationName '%s', got '%s'", tt.wantAppName, appName)
					}
				} else if tt.wantAppName != "" {
					t.Errorf("expected ApplicationName in Details, got none")
				}
				if groupName, ok := results[0].Details["GroupName"].(string); ok {
					if groupName != tt.wantGroupName {
						t.Errorf("expected GroupName '%s', got '%s'", tt.wantGroupName, groupName)
					}
				} else if tt.wantGroupName != "" {
					t.Errorf("expected GroupName in Details, got none")
				}
				if groupId, ok := results[0].Details["DeploymentGroupId"].(string); ok {
					if groupId != tt.wantGroupId {
						t.Errorf("expected DeploymentGroupId '%s', got '%s'", tt.wantGroupId, groupId)
					}
				} else if tt.wantGroupId != "" {
					t.Errorf("expected DeploymentGroupId in Details, got none")
				}
				if configName, ok := results[0].Details["DeploymentConfigName"].(string); ok {
					if configName != tt.wantConfigName {
						t.Errorf("expected DeploymentConfigName '%s', got '%s'", tt.wantConfigName, configName)
					}
				} else if tt.wantConfigName != "" {
					t.Errorf("expected DeploymentConfigName in Details, got none")
				}
				if platform, ok := results[0].Details["ComputePlatform"].(string); ok {
					if platform != tt.wantPlatform {
						t.Errorf("expected ComputePlatform '%s', got '%s'", tt.wantPlatform, platform)
					}
				} else if tt.wantPlatform != "" {
					t.Errorf("expected ComputePlatform in Details, got none")
				}
				if roleArn, ok := results[0].Details["ServiceRoleArn"].(string); ok {
					if roleArn != tt.wantRoleArn {
						t.Errorf("expected ServiceRoleArn '%s', got '%s'", tt.wantRoleArn, roleArn)
					}
				} else if tt.wantRoleArn != "" {
					t.Errorf("expected ServiceRoleArn in Details, got none")
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

func TestListDeploymentConfigsProcess(t *testing.T) {
	process := CodeDeployCalls[2].Process

	tests := []struct {
		name             string
		output           interface{}
		err              error
		wantLen          int
		wantError        bool
		wantResourceName string
		wantName         string
		wantConfigId     string
		wantPlatform     string
		wantCreateTime   string
		wantRegion       string
	}{
		{
			name: "valid configs with full details",
			output: []cdDeploymentConfig{
				{
					Name:               "CodeDeployDefault.OneAtATime",
					DeploymentConfigId: "config-12345-abcde",
					ComputePlatform:    "Server",
					CreateTime:         "2026-01-15T10:00:00Z",
					Region:             "us-east-1",
				},
				{
					Name:               "custom-canary-config",
					DeploymentConfigId: "config-67890-fghij",
					ComputePlatform:    "Lambda",
					CreateTime:         "2026-02-20T14:00:00Z",
					Region:             "us-west-2",
				},
			},
			wantLen:          2,
			wantResourceName: "CodeDeployDefault.OneAtATime",
			wantName:         "CodeDeployDefault.OneAtATime",
			wantConfigId:     "config-12345-abcde",
			wantPlatform:     "Server",
			wantCreateTime:   "2026-01-15T10:00:00Z",
			wantRegion:       "us-east-1",
		},
		{
			name:    "empty results",
			output:  []cdDeploymentConfig{},
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
			output: []cdDeploymentConfig{
				{
					Name:               "",
					DeploymentConfigId: "",
					ComputePlatform:    "",
					CreateTime:         "",
					Region:             "",
				},
			},
			wantLen:          1,
			wantResourceName: "",
			wantName:         "",
			wantConfigId:     "",
			wantPlatform:     "",
			wantCreateTime:   "",
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
				if results[0].ServiceName != "CodeDeploy" {
					t.Errorf("expected ServiceName 'CodeDeploy', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "codedeploy:ListDeploymentConfigs" {
					t.Errorf("expected MethodName 'codedeploy:ListDeploymentConfigs', got '%s'", results[0].MethodName)
				}
				return
			}

			if tt.wantLen > 0 {
				if results[0].ServiceName != "CodeDeploy" {
					t.Errorf("expected ServiceName 'CodeDeploy', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "codedeploy:ListDeploymentConfigs" {
					t.Errorf("expected MethodName 'codedeploy:ListDeploymentConfigs', got '%s'", results[0].MethodName)
				}
				if results[0].ResourceType != "deployment-config" {
					t.Errorf("expected ResourceType 'deployment-config', got '%s'", results[0].ResourceType)
				}
				if results[0].ResourceName != tt.wantResourceName {
					t.Errorf("expected ResourceName '%s', got '%s'", tt.wantResourceName, results[0].ResourceName)
				}
				if name, ok := results[0].Details["Name"].(string); ok {
					if name != tt.wantName {
						t.Errorf("expected Name '%s', got '%s'", tt.wantName, name)
					}
				} else if tt.wantName != "" {
					t.Errorf("expected Name in Details, got none")
				}
				if configId, ok := results[0].Details["DeploymentConfigId"].(string); ok {
					if configId != tt.wantConfigId {
						t.Errorf("expected DeploymentConfigId '%s', got '%s'", tt.wantConfigId, configId)
					}
				} else if tt.wantConfigId != "" {
					t.Errorf("expected DeploymentConfigId in Details, got none")
				}
				if platform, ok := results[0].Details["ComputePlatform"].(string); ok {
					if platform != tt.wantPlatform {
						t.Errorf("expected ComputePlatform '%s', got '%s'", tt.wantPlatform, platform)
					}
				} else if tt.wantPlatform != "" {
					t.Errorf("expected ComputePlatform in Details, got none")
				}
				if createTime, ok := results[0].Details["CreateTime"].(string); ok {
					if createTime != tt.wantCreateTime {
						t.Errorf("expected CreateTime '%s', got '%s'", tt.wantCreateTime, createTime)
					}
				} else if tt.wantCreateTime != "" {
					t.Errorf("expected CreateTime in Details, got none")
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

func TestMaxBatchSize(t *testing.T) {
	if maxBatchSize != 25 {
		t.Errorf("expected maxBatchSize to be 25, got %d", maxBatchSize)
	}
}

func TestExtractApplication(t *testing.T) {
	ts := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		input      *codedeploy.ApplicationInfo
		region     string
		wantName   string
		wantAppId  string
		wantPlatf  string
		wantCreate string
		wantGH     string
		wantRegion string
	}{
		{
			name: "all fields populated",
			input: &codedeploy.ApplicationInfo{
				ApplicationName: aws.String("my-app"),
				ApplicationId:   aws.String("app-123"),
				ComputePlatform: aws.String("Server"),
				CreateTime:      &ts,
				LinkedToGitHub:  aws.Bool(true),
			},
			region:     "us-east-1",
			wantName:   "my-app",
			wantAppId:  "app-123",
			wantPlatf:  "Server",
			wantCreate: "2026-01-15T10:00:00Z",
			wantGH:     "true",
			wantRegion: "us-east-1",
		},
		{
			name:       "all fields nil",
			input:      &codedeploy.ApplicationInfo{},
			region:     "eu-west-1",
			wantName:   "",
			wantAppId:  "",
			wantPlatf:  "",
			wantCreate: "",
			wantGH:     "",
			wantRegion: "eu-west-1",
		},
		{
			name: "LinkedToGitHub false",
			input: &codedeploy.ApplicationInfo{
				ApplicationName: aws.String("no-gh"),
				LinkedToGitHub:  aws.Bool(false),
			},
			region:     "us-west-2",
			wantName:   "no-gh",
			wantGH:     "false",
			wantRegion: "us-west-2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractApplication(tt.input, tt.region)
			if result.Name != tt.wantName {
				t.Errorf("Name: got %q, want %q", result.Name, tt.wantName)
			}
			if result.ApplicationId != tt.wantAppId {
				t.Errorf("ApplicationId: got %q, want %q", result.ApplicationId, tt.wantAppId)
			}
			if result.ComputePlatform != tt.wantPlatf {
				t.Errorf("ComputePlatform: got %q, want %q", result.ComputePlatform, tt.wantPlatf)
			}
			if result.CreateTime != tt.wantCreate {
				t.Errorf("CreateTime: got %q, want %q", result.CreateTime, tt.wantCreate)
			}
			if result.LinkedToGitHub != tt.wantGH {
				t.Errorf("LinkedToGitHub: got %q, want %q", result.LinkedToGitHub, tt.wantGH)
			}
			if result.Region != tt.wantRegion {
				t.Errorf("Region: got %q, want %q", result.Region, tt.wantRegion)
			}
		})
	}
}

func TestExtractDeploymentGroup(t *testing.T) {
	tests := []struct {
		name          string
		input         *codedeploy.DeploymentGroupInfo
		region        string
		wantAppName   string
		wantGroupName string
		wantGroupId   string
		wantConfig    string
		wantPlatform  string
		wantRole      string
	}{
		{
			name: "all fields populated",
			input: &codedeploy.DeploymentGroupInfo{
				ApplicationName:      aws.String("my-app"),
				DeploymentGroupName:  aws.String("prod"),
				DeploymentGroupId:    aws.String("dg-123"),
				DeploymentConfigName: aws.String("CodeDeployDefault.OneAtATime"),
				ComputePlatform:      aws.String("Server"),
				ServiceRoleArn:       aws.String("arn:aws:iam::111:role/CDRole"),
			},
			region:        "us-east-1",
			wantAppName:   "my-app",
			wantGroupName: "prod",
			wantGroupId:   "dg-123",
			wantConfig:    "CodeDeployDefault.OneAtATime",
			wantPlatform:  "Server",
			wantRole:      "arn:aws:iam::111:role/CDRole",
		},
		{
			name:          "all fields nil",
			input:         &codedeploy.DeploymentGroupInfo{},
			region:        "eu-west-1",
			wantAppName:   "",
			wantGroupName: "",
			wantGroupId:   "",
			wantConfig:    "",
			wantPlatform:  "",
			wantRole:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractDeploymentGroup(tt.input, tt.region)
			if result.ApplicationName != tt.wantAppName {
				t.Errorf("ApplicationName: got %q, want %q", result.ApplicationName, tt.wantAppName)
			}
			if result.GroupName != tt.wantGroupName {
				t.Errorf("GroupName: got %q, want %q", result.GroupName, tt.wantGroupName)
			}
			if result.DeploymentGroupId != tt.wantGroupId {
				t.Errorf("DeploymentGroupId: got %q, want %q", result.DeploymentGroupId, tt.wantGroupId)
			}
			if result.DeploymentConfigName != tt.wantConfig {
				t.Errorf("DeploymentConfigName: got %q, want %q", result.DeploymentConfigName, tt.wantConfig)
			}
			if result.ComputePlatform != tt.wantPlatform {
				t.Errorf("ComputePlatform: got %q, want %q", result.ComputePlatform, tt.wantPlatform)
			}
			if result.ServiceRoleArn != tt.wantRole {
				t.Errorf("ServiceRoleArn: got %q, want %q", result.ServiceRoleArn, tt.wantRole)
			}
			if result.Region != tt.region {
				t.Errorf("Region: got %q, want %q", result.Region, tt.region)
			}
		})
	}
}

func TestExtractDeploymentConfig(t *testing.T) {
	ts := time.Date(2026, 2, 20, 14, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		input      *codedeploy.DeploymentConfigInfo
		region     string
		wantName   string
		wantId     string
		wantPlatf  string
		wantCreate string
	}{
		{
			name: "all fields populated",
			input: &codedeploy.DeploymentConfigInfo{
				DeploymentConfigName: aws.String("custom-config"),
				DeploymentConfigId:   aws.String("cfg-456"),
				ComputePlatform:      aws.String("Lambda"),
				CreateTime:           &ts,
			},
			region:     "us-west-2",
			wantName:   "custom-config",
			wantId:     "cfg-456",
			wantPlatf:  "Lambda",
			wantCreate: "2026-02-20T14:00:00Z",
		},
		{
			name:       "all fields nil",
			input:      &codedeploy.DeploymentConfigInfo{},
			region:     "ap-southeast-1",
			wantName:   "",
			wantId:     "",
			wantPlatf:  "",
			wantCreate: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractDeploymentConfig(tt.input, tt.region)
			if result.Name != tt.wantName {
				t.Errorf("Name: got %q, want %q", result.Name, tt.wantName)
			}
			if result.DeploymentConfigId != tt.wantId {
				t.Errorf("DeploymentConfigId: got %q, want %q", result.DeploymentConfigId, tt.wantId)
			}
			if result.ComputePlatform != tt.wantPlatf {
				t.Errorf("ComputePlatform: got %q, want %q", result.ComputePlatform, tt.wantPlatf)
			}
			if result.CreateTime != tt.wantCreate {
				t.Errorf("CreateTime: got %q, want %q", result.CreateTime, tt.wantCreate)
			}
			if result.Region != tt.region {
				t.Errorf("Region: got %q, want %q", result.Region, tt.region)
			}
		})
	}
}
