package sagemaker

import (
	"fmt"
	"testing"
)

func TestListNotebookInstancesProcess(t *testing.T) {
	process := SageMakerCalls[0].Process

	tests := []struct {
		name                      string
		output                    interface{}
		err                       error
		wantLen                   int
		wantError                 bool
		wantResourceName          string
		wantName                  string
		wantArn                   string
		wantStatus                string
		wantInstanceType          string
		wantURL                   string
		wantDefaultCodeRepository string
		wantLastModifiedTime      string
		wantRegion                string
	}{
		{
			name: "valid notebooks with full details",
			output: []smNotebook{
				{
					Name:                  "my-notebook",
					Arn:                   "arn:aws:sagemaker:us-east-1:111111111111:notebook-instance/my-notebook",
					Status:                "InService",
					InstanceType:          "ml.t2.medium",
					URL:                   "my-notebook.notebook.us-east-1.sagemaker.aws",
					DefaultCodeRepository: "https://github.com/org/repo",
					LastModifiedTime:      "2026-03-01T12:00:00Z",
					Region:                "us-east-1",
				},
				{
					Name:                  "dev-notebook",
					Arn:                   "arn:aws:sagemaker:us-west-2:111111111111:notebook-instance/dev-notebook",
					Status:                "Stopped",
					InstanceType:          "ml.m5.xlarge",
					URL:                   "dev-notebook.notebook.us-west-2.sagemaker.aws",
					DefaultCodeRepository: "",
					LastModifiedTime:      "2026-02-15T08:30:00Z",
					Region:                "us-west-2",
				},
			},
			wantLen:                   2,
			wantResourceName:          "my-notebook",
			wantName:                  "my-notebook",
			wantArn:                   "arn:aws:sagemaker:us-east-1:111111111111:notebook-instance/my-notebook",
			wantStatus:                "InService",
			wantInstanceType:          "ml.t2.medium",
			wantURL:                   "my-notebook.notebook.us-east-1.sagemaker.aws",
			wantDefaultCodeRepository: "https://github.com/org/repo",
			wantLastModifiedTime:      "2026-03-01T12:00:00Z",
			wantRegion:                "us-east-1",
		},
		{
			name:    "empty results",
			output:  []smNotebook{},
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
			output: []smNotebook{
				{
					Name:                  "",
					Arn:                   "",
					Status:                "",
					InstanceType:          "",
					URL:                   "",
					DefaultCodeRepository: "",
					LastModifiedTime:      "",
					Region:                "",
				},
			},
			wantLen:                   1,
			wantResourceName:          "",
			wantName:                  "",
			wantArn:                   "",
			wantStatus:                "",
			wantInstanceType:          "",
			wantURL:                   "",
			wantDefaultCodeRepository: "",
			wantLastModifiedTime:      "",
			wantRegion:                "",
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
				if results[0].ServiceName != "SageMaker" {
					t.Errorf("expected ServiceName 'SageMaker', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "sagemaker:ListNotebookInstances" {
					t.Errorf("expected MethodName 'sagemaker:ListNotebookInstances', got '%s'", results[0].MethodName)
				}
				return
			}

			if tt.wantLen > 0 {
				if results[0].ServiceName != "SageMaker" {
					t.Errorf("expected ServiceName 'SageMaker', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "sagemaker:ListNotebookInstances" {
					t.Errorf("expected MethodName 'sagemaker:ListNotebookInstances', got '%s'", results[0].MethodName)
				}
				if results[0].ResourceType != "notebook-instance" {
					t.Errorf("expected ResourceType 'notebook-instance', got '%s'", results[0].ResourceType)
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
				if status, ok := results[0].Details["Status"].(string); ok {
					if status != tt.wantStatus {
						t.Errorf("expected Status '%s', got '%s'", tt.wantStatus, status)
					}
				} else if tt.wantStatus != "" {
					t.Errorf("expected Status in Details, got none")
				}
				if instanceType, ok := results[0].Details["InstanceType"].(string); ok {
					if instanceType != tt.wantInstanceType {
						t.Errorf("expected InstanceType '%s', got '%s'", tt.wantInstanceType, instanceType)
					}
				} else if tt.wantInstanceType != "" {
					t.Errorf("expected InstanceType in Details, got none")
				}
				if url, ok := results[0].Details["URL"].(string); ok {
					if url != tt.wantURL {
						t.Errorf("expected URL '%s', got '%s'", tt.wantURL, url)
					}
				} else if tt.wantURL != "" {
					t.Errorf("expected URL in Details, got none")
				}
				if repo, ok := results[0].Details["DefaultCodeRepository"].(string); ok {
					if repo != tt.wantDefaultCodeRepository {
						t.Errorf("expected DefaultCodeRepository '%s', got '%s'", tt.wantDefaultCodeRepository, repo)
					}
				} else if tt.wantDefaultCodeRepository != "" {
					t.Errorf("expected DefaultCodeRepository in Details, got none")
				}
				if lastMod, ok := results[0].Details["LastModifiedTime"].(string); ok {
					if lastMod != tt.wantLastModifiedTime {
						t.Errorf("expected LastModifiedTime '%s', got '%s'", tt.wantLastModifiedTime, lastMod)
					}
				} else if tt.wantLastModifiedTime != "" {
					t.Errorf("expected LastModifiedTime in Details, got none")
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

func TestListEndpointsProcess(t *testing.T) {
	process := SageMakerCalls[1].Process

	tests := []struct {
		name                 string
		output               interface{}
		err                  error
		wantLen              int
		wantError            bool
		wantResourceName     string
		wantName             string
		wantArn              string
		wantStatus           string
		wantCreationTime     string
		wantLastModifiedTime string
		wantRegion           string
	}{
		{
			name: "valid endpoints with full details",
			output: []smEndpoint{
				{
					Name:             "my-endpoint",
					Arn:              "arn:aws:sagemaker:us-east-1:111111111111:endpoint/my-endpoint",
					Status:           "InService",
					CreationTime:     "2026-01-15T10:30:00Z",
					LastModifiedTime: "2026-02-20T09:00:00Z",
					Region:           "us-east-1",
				},
				{
					Name:             "staging-endpoint",
					Arn:              "arn:aws:sagemaker:us-west-2:111111111111:endpoint/staging-endpoint",
					Status:           "Creating",
					CreationTime:     "2026-02-20T14:00:00Z",
					LastModifiedTime: "2026-02-20T14:00:00Z",
					Region:           "us-west-2",
				},
			},
			wantLen:              2,
			wantResourceName:     "my-endpoint",
			wantName:             "my-endpoint",
			wantArn:              "arn:aws:sagemaker:us-east-1:111111111111:endpoint/my-endpoint",
			wantStatus:           "InService",
			wantCreationTime:     "2026-01-15T10:30:00Z",
			wantLastModifiedTime: "2026-02-20T09:00:00Z",
			wantRegion:           "us-east-1",
		},
		{
			name:    "empty results",
			output:  []smEndpoint{},
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
			output: []smEndpoint{
				{
					Name:             "",
					Arn:              "",
					Status:           "",
					CreationTime:     "",
					LastModifiedTime: "",
					Region:           "",
				},
			},
			wantLen:              1,
			wantResourceName:     "",
			wantName:             "",
			wantArn:              "",
			wantStatus:           "",
			wantCreationTime:     "",
			wantLastModifiedTime: "",
			wantRegion:           "",
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
				if results[0].ServiceName != "SageMaker" {
					t.Errorf("expected ServiceName 'SageMaker', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "sagemaker:ListEndpoints" {
					t.Errorf("expected MethodName 'sagemaker:ListEndpoints', got '%s'", results[0].MethodName)
				}
				return
			}

			if tt.wantLen > 0 {
				if results[0].ServiceName != "SageMaker" {
					t.Errorf("expected ServiceName 'SageMaker', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "sagemaker:ListEndpoints" {
					t.Errorf("expected MethodName 'sagemaker:ListEndpoints', got '%s'", results[0].MethodName)
				}
				if results[0].ResourceType != "endpoint" {
					t.Errorf("expected ResourceType 'endpoint', got '%s'", results[0].ResourceType)
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
				if status, ok := results[0].Details["Status"].(string); ok {
					if status != tt.wantStatus {
						t.Errorf("expected Status '%s', got '%s'", tt.wantStatus, status)
					}
				} else if tt.wantStatus != "" {
					t.Errorf("expected Status in Details, got none")
				}
				if creationTime, ok := results[0].Details["CreationTime"].(string); ok {
					if creationTime != tt.wantCreationTime {
						t.Errorf("expected CreationTime '%s', got '%s'", tt.wantCreationTime, creationTime)
					}
				} else if tt.wantCreationTime != "" {
					t.Errorf("expected CreationTime in Details, got none")
				}
				if lastMod, ok := results[0].Details["LastModifiedTime"].(string); ok {
					if lastMod != tt.wantLastModifiedTime {
						t.Errorf("expected LastModifiedTime '%s', got '%s'", tt.wantLastModifiedTime, lastMod)
					}
				} else if tt.wantLastModifiedTime != "" {
					t.Errorf("expected LastModifiedTime in Details, got none")
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

func TestListModelsProcess(t *testing.T) {
	process := SageMakerCalls[2].Process

	tests := []struct {
		name             string
		output           interface{}
		err              error
		wantLen          int
		wantError        bool
		wantResourceName string
		wantName         string
		wantArn          string
		wantCreationTime string
		wantRegion       string
	}{
		{
			name: "valid models with full details",
			output: []smModel{
				{
					Name:         "my-model",
					Arn:          "arn:aws:sagemaker:us-east-1:111111111111:model/my-model",
					CreationTime: "2026-01-10T08:00:00Z",
					Region:       "us-east-1",
				},
				{
					Name:         "prod-model-v2",
					Arn:          "arn:aws:sagemaker:us-west-2:111111111111:model/prod-model-v2",
					CreationTime: "2026-03-01T12:00:00Z",
					Region:       "us-west-2",
				},
			},
			wantLen:          2,
			wantResourceName: "my-model",
			wantName:         "my-model",
			wantArn:          "arn:aws:sagemaker:us-east-1:111111111111:model/my-model",
			wantCreationTime: "2026-01-10T08:00:00Z",
			wantRegion:       "us-east-1",
		},
		{
			name:    "empty results",
			output:  []smModel{},
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
			output: []smModel{
				{
					Name:         "",
					Arn:          "",
					CreationTime: "",
					Region:       "",
				},
			},
			wantLen:          1,
			wantResourceName: "",
			wantName:         "",
			wantArn:          "",
			wantCreationTime: "",
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
				if results[0].ServiceName != "SageMaker" {
					t.Errorf("expected ServiceName 'SageMaker', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "sagemaker:ListModels" {
					t.Errorf("expected MethodName 'sagemaker:ListModels', got '%s'", results[0].MethodName)
				}
				return
			}

			if tt.wantLen > 0 {
				if results[0].ServiceName != "SageMaker" {
					t.Errorf("expected ServiceName 'SageMaker', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "sagemaker:ListModels" {
					t.Errorf("expected MethodName 'sagemaker:ListModels', got '%s'", results[0].MethodName)
				}
				if results[0].ResourceType != "model" {
					t.Errorf("expected ResourceType 'model', got '%s'", results[0].ResourceType)
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
				if creationTime, ok := results[0].Details["CreationTime"].(string); ok {
					if creationTime != tt.wantCreationTime {
						t.Errorf("expected CreationTime '%s', got '%s'", tt.wantCreationTime, creationTime)
					}
				} else if tt.wantCreationTime != "" {
					t.Errorf("expected CreationTime in Details, got none")
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

func TestListTrainingJobsProcess(t *testing.T) {
	process := SageMakerCalls[3].Process

	tests := []struct {
		name                 string
		output               interface{}
		err                  error
		wantLen              int
		wantError            bool
		wantResourceName     string
		wantName             string
		wantArn              string
		wantStatus           string
		wantCreationTime     string
		wantLastModifiedTime string
		wantRegion           string
	}{
		{
			name: "valid training jobs with full details",
			output: []smTrainingJob{
				{
					Name:             "training-job-001",
					Arn:              "arn:aws:sagemaker:us-east-1:111111111111:training-job/training-job-001",
					Status:           "Completed",
					CreationTime:     "2026-02-01T09:00:00Z",
					LastModifiedTime: "2026-02-01T11:30:00Z",
					Region:           "us-east-1",
				},
				{
					Name:             "training-job-002",
					Arn:              "arn:aws:sagemaker:us-west-2:111111111111:training-job/training-job-002",
					Status:           "InProgress",
					CreationTime:     "2026-03-10T16:00:00Z",
					LastModifiedTime: "2026-03-10T16:05:00Z",
					Region:           "us-west-2",
				},
			},
			wantLen:              2,
			wantResourceName:     "training-job-001",
			wantName:             "training-job-001",
			wantArn:              "arn:aws:sagemaker:us-east-1:111111111111:training-job/training-job-001",
			wantStatus:           "Completed",
			wantCreationTime:     "2026-02-01T09:00:00Z",
			wantLastModifiedTime: "2026-02-01T11:30:00Z",
			wantRegion:           "us-east-1",
		},
		{
			name:    "empty results",
			output:  []smTrainingJob{},
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
			output: []smTrainingJob{
				{
					Name:             "",
					Arn:              "",
					Status:           "",
					CreationTime:     "",
					LastModifiedTime: "",
					Region:           "",
				},
			},
			wantLen:              1,
			wantResourceName:     "",
			wantName:             "",
			wantArn:              "",
			wantStatus:           "",
			wantCreationTime:     "",
			wantLastModifiedTime: "",
			wantRegion:           "",
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
				if results[0].ServiceName != "SageMaker" {
					t.Errorf("expected ServiceName 'SageMaker', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "sagemaker:ListTrainingJobs" {
					t.Errorf("expected MethodName 'sagemaker:ListTrainingJobs', got '%s'", results[0].MethodName)
				}
				return
			}

			if tt.wantLen > 0 {
				if results[0].ServiceName != "SageMaker" {
					t.Errorf("expected ServiceName 'SageMaker', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "sagemaker:ListTrainingJobs" {
					t.Errorf("expected MethodName 'sagemaker:ListTrainingJobs', got '%s'", results[0].MethodName)
				}
				if results[0].ResourceType != "training-job" {
					t.Errorf("expected ResourceType 'training-job', got '%s'", results[0].ResourceType)
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
				if status, ok := results[0].Details["Status"].(string); ok {
					if status != tt.wantStatus {
						t.Errorf("expected Status '%s', got '%s'", tt.wantStatus, status)
					}
				} else if tt.wantStatus != "" {
					t.Errorf("expected Status in Details, got none")
				}
				if creationTime, ok := results[0].Details["CreationTime"].(string); ok {
					if creationTime != tt.wantCreationTime {
						t.Errorf("expected CreationTime '%s', got '%s'", tt.wantCreationTime, creationTime)
					}
				} else if tt.wantCreationTime != "" {
					t.Errorf("expected CreationTime in Details, got none")
				}
				if lastMod, ok := results[0].Details["LastModifiedTime"].(string); ok {
					if lastMod != tt.wantLastModifiedTime {
						t.Errorf("expected LastModifiedTime '%s', got '%s'", tt.wantLastModifiedTime, lastMod)
					}
				} else if tt.wantLastModifiedTime != "" {
					t.Errorf("expected LastModifiedTime in Details, got none")
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
