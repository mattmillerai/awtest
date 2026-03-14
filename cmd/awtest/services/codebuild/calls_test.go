package codebuild

import (
	"fmt"
	"testing"
)

func TestListProjectsProcess(t *testing.T) {
	process := CodeBuildCalls[0].Process

	tests := []struct {
		name                string
		output              interface{}
		err                 error
		wantLen             int
		wantError           bool
		wantResourceName    string
		wantName            string
		wantArn             string
		wantDescription     string
		wantSourceType      string
		wantEnvironmentType string
		wantRegion          string
	}{
		{
			name: "valid projects",
			output: []cbProject{
				{
					Name:            "my-build-project",
					Arn:             "arn:aws:codebuild:us-east-1:111111111111:project/my-build-project",
					Description:     "My build project",
					SourceType:      "GITHUB",
					EnvironmentType: "LINUX_CONTAINER",
					Region:          "us-east-1",
				},
				{
					Name:            "another-project",
					Arn:             "arn:aws:codebuild:us-west-2:111111111111:project/another-project",
					Description:     "Another project",
					SourceType:      "CODECOMMIT",
					EnvironmentType: "ARM_CONTAINER",
					Region:          "us-west-2",
				},
			},
			wantLen:             2,
			wantResourceName:    "my-build-project",
			wantName:            "my-build-project",
			wantArn:             "arn:aws:codebuild:us-east-1:111111111111:project/my-build-project",
			wantDescription:     "My build project",
			wantSourceType:      "GITHUB",
			wantEnvironmentType: "LINUX_CONTAINER",
			wantRegion:          "us-east-1",
		},
		{
			name:    "empty results",
			output:  []cbProject{},
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
			output: []cbProject{
				{
					Name:            "",
					Arn:             "",
					Description:     "",
					SourceType:      "",
					EnvironmentType: "",
					Region:          "",
				},
			},
			wantLen:             1,
			wantResourceName:    "",
			wantName:            "",
			wantArn:             "",
			wantDescription:     "",
			wantSourceType:      "",
			wantEnvironmentType: "",
			wantRegion:          "",
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
				if results[0].ServiceName != "CodeBuild" {
					t.Errorf("expected ServiceName 'CodeBuild', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "codebuild:ListProjects" {
					t.Errorf("expected MethodName 'codebuild:ListProjects', got '%s'", results[0].MethodName)
				}
				return
			}

			if tt.wantLen > 0 {
				if results[0].ServiceName != "CodeBuild" {
					t.Errorf("expected ServiceName 'CodeBuild', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "codebuild:ListProjects" {
					t.Errorf("expected MethodName 'codebuild:ListProjects', got '%s'", results[0].MethodName)
				}
				if results[0].ResourceType != "project" {
					t.Errorf("expected ResourceType 'project', got '%s'", results[0].ResourceType)
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
				if arn, ok := results[0].Details["Arn"].(string); ok {
					if arn != tt.wantArn {
						t.Errorf("expected Arn '%s', got '%s'", tt.wantArn, arn)
					}
				} else if tt.wantArn != "" {
					t.Errorf("expected Arn in Details, got none")
				}
				if desc, ok := results[0].Details["Description"].(string); ok {
					if desc != tt.wantDescription {
						t.Errorf("expected Description '%s', got '%s'", tt.wantDescription, desc)
					}
				} else if tt.wantDescription != "" {
					t.Errorf("expected Description in Details, got none")
				}
				if sourceType, ok := results[0].Details["SourceType"].(string); ok {
					if sourceType != tt.wantSourceType {
						t.Errorf("expected SourceType '%s', got '%s'", tt.wantSourceType, sourceType)
					}
				} else if tt.wantSourceType != "" {
					t.Errorf("expected SourceType in Details, got none")
				}
				if envType, ok := results[0].Details["EnvironmentType"].(string); ok {
					if envType != tt.wantEnvironmentType {
						t.Errorf("expected EnvironmentType '%s', got '%s'", tt.wantEnvironmentType, envType)
					}
				} else if tt.wantEnvironmentType != "" {
					t.Errorf("expected EnvironmentType in Details, got none")
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

func TestListProjectEnvironmentVariablesProcess(t *testing.T) {
	process := CodeBuildCalls[1].Process

	tests := []struct {
		name             string
		output           interface{}
		err              error
		wantLen          int
		wantError        bool
		wantResourceName string
		wantProjectName  string
		wantVariableName string
		wantVariableType string
		wantRegion       string
	}{
		{
			name: "valid env vars with PLAINTEXT type",
			output: []cbEnvVar{
				{
					ProjectName:  "my-project",
					VariableName: "AWS_DEFAULT_REGION",
					VariableType: "PLAINTEXT",
					Region:       "us-east-1",
				},
				{
					ProjectName:  "my-project",
					VariableName: "DB_PASSWORD",
					VariableType: "SECRETS_MANAGER",
					Region:       "us-east-1",
				},
			},
			wantLen:          2,
			wantResourceName: "my-project/AWS_DEFAULT_REGION",
			wantProjectName:  "my-project",
			wantVariableName: "AWS_DEFAULT_REGION",
			wantVariableType: "PLAINTEXT",
			wantRegion:       "us-east-1",
		},
		{
			name: "PARAMETER_STORE type",
			output: []cbEnvVar{
				{
					ProjectName:  "build-project",
					VariableName: "API_KEY",
					VariableType: "PARAMETER_STORE",
					Region:       "us-west-2",
				},
			},
			wantLen:          1,
			wantResourceName: "build-project/API_KEY",
			wantProjectName:  "build-project",
			wantVariableName: "API_KEY",
			wantVariableType: "PARAMETER_STORE",
			wantRegion:       "us-west-2",
		},
		{
			name: "SECRETS_MANAGER type",
			output: []cbEnvVar{
				{
					ProjectName:  "secure-project",
					VariableName: "SECRET_TOKEN",
					VariableType: "SECRETS_MANAGER",
					Region:       "eu-west-1",
				},
			},
			wantLen:          1,
			wantResourceName: "secure-project/SECRET_TOKEN",
			wantProjectName:  "secure-project",
			wantVariableName: "SECRET_TOKEN",
			wantVariableType: "SECRETS_MANAGER",
			wantRegion:       "eu-west-1",
		},
		{
			name:    "empty results",
			output:  []cbEnvVar{},
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
			output: []cbEnvVar{
				{
					ProjectName:  "",
					VariableName: "",
					VariableType: "",
					Region:       "",
				},
			},
			wantLen:          1,
			wantResourceName: "/",
			wantProjectName:  "",
			wantVariableName: "",
			wantVariableType: "",
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
				if results[0].ServiceName != "CodeBuild" {
					t.Errorf("expected ServiceName 'CodeBuild', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "codebuild:ListProjectEnvironmentVariables" {
					t.Errorf("expected MethodName 'codebuild:ListProjectEnvironmentVariables', got '%s'", results[0].MethodName)
				}
				return
			}

			if tt.wantLen > 0 {
				if results[0].ServiceName != "CodeBuild" {
					t.Errorf("expected ServiceName 'CodeBuild', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "codebuild:ListProjectEnvironmentVariables" {
					t.Errorf("expected MethodName 'codebuild:ListProjectEnvironmentVariables', got '%s'", results[0].MethodName)
				}
				if results[0].ResourceType != "environment-variable" {
					t.Errorf("expected ResourceType 'environment-variable', got '%s'", results[0].ResourceType)
				}
				if results[0].ResourceName != tt.wantResourceName {
					t.Errorf("expected ResourceName '%s', got '%s'", tt.wantResourceName, results[0].ResourceName)
				}
				if projectName, ok := results[0].Details["ProjectName"].(string); ok {
					if projectName != tt.wantProjectName {
						t.Errorf("expected ProjectName '%s', got '%s'", tt.wantProjectName, projectName)
					}
				} else if tt.wantProjectName != "" {
					t.Errorf("expected ProjectName in Details, got none")
				}
				if varName, ok := results[0].Details["VariableName"].(string); ok {
					if varName != tt.wantVariableName {
						t.Errorf("expected VariableName '%s', got '%s'", tt.wantVariableName, varName)
					}
				} else if tt.wantVariableName != "" {
					t.Errorf("expected VariableName in Details, got none")
				}
				if varType, ok := results[0].Details["VariableType"].(string); ok {
					if varType != tt.wantVariableType {
						t.Errorf("expected VariableType '%s', got '%s'", tt.wantVariableType, varType)
					}
				} else if tt.wantVariableType != "" {
					t.Errorf("expected VariableType in Details, got none")
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

func TestListBuildsProcess(t *testing.T) {
	process := CodeBuildCalls[2].Process

	tests := []struct {
		name             string
		output           interface{}
		err              error
		wantLen          int
		wantError        bool
		wantResourceName string
		wantBuildId      string
		wantProjectName  string
		wantBuildStatus  string
		wantStartTime    string
		wantRegion       string
	}{
		{
			name: "valid builds",
			output: []cbBuild{
				{
					BuildId:     "my-project:build-1",
					ProjectName: "my-project",
					BuildStatus: "SUCCEEDED",
					StartTime:   "2026-03-10T12:00:00Z",
					Region:      "us-east-1",
				},
				{
					BuildId:     "my-project:build-2",
					ProjectName: "my-project",
					BuildStatus: "FAILED",
					StartTime:   "2026-03-09T10:00:00Z",
					Region:      "us-east-1",
				},
			},
			wantLen:          2,
			wantResourceName: "my-project:build-1",
			wantBuildId:      "my-project:build-1",
			wantProjectName:  "my-project",
			wantBuildStatus:  "SUCCEEDED",
			wantStartTime:    "2026-03-10T12:00:00Z",
			wantRegion:       "us-east-1",
		},
		{
			name:    "empty results",
			output:  []cbBuild{},
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
			output: []cbBuild{
				{
					BuildId:     "",
					ProjectName: "",
					BuildStatus: "",
					StartTime:   "",
					Region:      "",
				},
			},
			wantLen:          1,
			wantResourceName: "",
			wantBuildId:      "",
			wantProjectName:  "",
			wantBuildStatus:  "",
			wantStartTime:    "",
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
				if results[0].ServiceName != "CodeBuild" {
					t.Errorf("expected ServiceName 'CodeBuild', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "codebuild:ListBuilds" {
					t.Errorf("expected MethodName 'codebuild:ListBuilds', got '%s'", results[0].MethodName)
				}
				return
			}

			if tt.wantLen > 0 {
				if results[0].ServiceName != "CodeBuild" {
					t.Errorf("expected ServiceName 'CodeBuild', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "codebuild:ListBuilds" {
					t.Errorf("expected MethodName 'codebuild:ListBuilds', got '%s'", results[0].MethodName)
				}
				if results[0].ResourceType != "build" {
					t.Errorf("expected ResourceType 'build', got '%s'", results[0].ResourceType)
				}
				if results[0].ResourceName != tt.wantResourceName {
					t.Errorf("expected ResourceName '%s', got '%s'", tt.wantResourceName, results[0].ResourceName)
				}
				if buildId, ok := results[0].Details["BuildId"].(string); ok {
					if buildId != tt.wantBuildId {
						t.Errorf("expected BuildId '%s', got '%s'", tt.wantBuildId, buildId)
					}
				} else if tt.wantBuildId != "" {
					t.Errorf("expected BuildId in Details, got none")
				}
				if projectName, ok := results[0].Details["ProjectName"].(string); ok {
					if projectName != tt.wantProjectName {
						t.Errorf("expected ProjectName '%s', got '%s'", tt.wantProjectName, projectName)
					}
				} else if tt.wantProjectName != "" {
					t.Errorf("expected ProjectName in Details, got none")
				}
				if buildStatus, ok := results[0].Details["BuildStatus"].(string); ok {
					if buildStatus != tt.wantBuildStatus {
						t.Errorf("expected BuildStatus '%s', got '%s'", tt.wantBuildStatus, buildStatus)
					}
				} else if tt.wantBuildStatus != "" {
					t.Errorf("expected BuildStatus in Details, got none")
				}
				if startTime, ok := results[0].Details["StartTime"].(string); ok {
					if startTime != tt.wantStartTime {
						t.Errorf("expected StartTime '%s', got '%s'", tt.wantStartTime, startTime)
					}
				} else if tt.wantStartTime != "" {
					t.Errorf("expected StartTime in Details, got none")
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
