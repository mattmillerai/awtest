package macie2

import (
	"fmt"
	"testing"
)

func TestListClassificationJobsProcess(t *testing.T) {
	process := Macie2Calls[0].Process

	tests := []struct {
		name             string
		output           interface{}
		err              error
		wantLen          int
		wantError        bool
		wantResourceName string
		wantJobId        string
		wantName         string
		wantJobType      string
		wantJobStatus    string
		wantCreatedAt    string
		wantRegion       string
	}{
		{
			name: "valid jobs with full details",
			output: []mcClassificationJob{
				{
					JobId:     "job-12345-abcde",
					Name:      "sensitive-data-scan",
					JobType:   "SCHEDULED",
					JobStatus: "RUNNING",
					CreatedAt: "2026-01-15T10:00:00Z",
					Region:    "us-east-1",
				},
				{
					JobId:     "job-67890-fghij",
					Name:      "one-time-classification",
					JobType:   "ONE_TIME",
					JobStatus: "COMPLETE",
					CreatedAt: "2026-02-20T14:00:00Z",
					Region:    "us-west-2",
				},
			},
			wantLen:          2,
			wantResourceName: "sensitive-data-scan",
			wantJobId:        "job-12345-abcde",
			wantName:         "sensitive-data-scan",
			wantJobType:      "SCHEDULED",
			wantJobStatus:    "RUNNING",
			wantCreatedAt:    "2026-01-15T10:00:00Z",
			wantRegion:       "us-east-1",
		},
		{
			name: "job with empty name uses JobId as ResourceName",
			output: []mcClassificationJob{
				{
					JobId:     "job-no-name-12345",
					Name:      "",
					JobType:   "ONE_TIME",
					JobStatus: "IDLE",
					CreatedAt: "2026-03-01T08:00:00Z",
					Region:    "eu-west-1",
				},
			},
			wantLen:          1,
			wantResourceName: "job-no-name-12345",
			wantJobId:        "job-no-name-12345",
			wantName:         "",
			wantJobType:      "ONE_TIME",
			wantJobStatus:    "IDLE",
			wantCreatedAt:    "2026-03-01T08:00:00Z",
			wantRegion:       "eu-west-1",
		},
		{
			name:    "empty results",
			output:  []mcClassificationJob{},
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
			output: []mcClassificationJob{
				{
					JobId:     "",
					Name:      "",
					JobType:   "",
					JobStatus: "",
					CreatedAt: "",
					Region:    "",
				},
			},
			wantLen:          1,
			wantResourceName: "",
			wantJobId:        "",
			wantName:         "",
			wantJobType:      "",
			wantJobStatus:    "",
			wantCreatedAt:    "",
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
				if results[0].ServiceName != "Macie" {
					t.Errorf("expected ServiceName 'Macie', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "macie2:ListClassificationJobs" {
					t.Errorf("expected MethodName 'macie2:ListClassificationJobs', got '%s'", results[0].MethodName)
				}
				return
			}

			if tt.wantLen > 0 {
				if results[0].ServiceName != "Macie" {
					t.Errorf("expected ServiceName 'Macie', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "macie2:ListClassificationJobs" {
					t.Errorf("expected MethodName 'macie2:ListClassificationJobs', got '%s'", results[0].MethodName)
				}
				if results[0].ResourceType != "classification-job" {
					t.Errorf("expected ResourceType 'classification-job', got '%s'", results[0].ResourceType)
				}
				if results[0].ResourceName != tt.wantResourceName {
					t.Errorf("expected ResourceName '%s', got '%s'", tt.wantResourceName, results[0].ResourceName)
				}
				if jobId, ok := results[0].Details["JobId"].(string); ok {
					if jobId != tt.wantJobId {
						t.Errorf("expected JobId '%s', got '%s'", tt.wantJobId, jobId)
					}
				} else if tt.wantJobId != "" {
					t.Errorf("expected JobId in Details, got none")
				}
				if name, ok := results[0].Details["Name"].(string); ok {
					if name != tt.wantName {
						t.Errorf("expected Name '%s', got '%s'", tt.wantName, name)
					}
				} else if tt.wantName != "" {
					t.Errorf("expected Name in Details, got none")
				}
				if jobType, ok := results[0].Details["JobType"].(string); ok {
					if jobType != tt.wantJobType {
						t.Errorf("expected JobType '%s', got '%s'", tt.wantJobType, jobType)
					}
				} else if tt.wantJobType != "" {
					t.Errorf("expected JobType in Details, got none")
				}
				if jobStatus, ok := results[0].Details["JobStatus"].(string); ok {
					if jobStatus != tt.wantJobStatus {
						t.Errorf("expected JobStatus '%s', got '%s'", tt.wantJobStatus, jobStatus)
					}
				} else if tt.wantJobStatus != "" {
					t.Errorf("expected JobStatus in Details, got none")
				}
				if createdAt, ok := results[0].Details["CreatedAt"].(string); ok {
					if createdAt != tt.wantCreatedAt {
						t.Errorf("expected CreatedAt '%s', got '%s'", tt.wantCreatedAt, createdAt)
					}
				} else if tt.wantCreatedAt != "" {
					t.Errorf("expected CreatedAt in Details, got none")
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

func TestListFindingsProcess(t *testing.T) {
	process := Macie2Calls[1].Process

	tests := []struct {
		name             string
		output           interface{}
		err              error
		wantLen          int
		wantError        bool
		wantResourceName string
		wantId           string
		wantType         string
		wantTitle        string
		wantSeverity     string
		wantCategory     string
		wantCount        string
		wantCreatedAt    string
		wantRegion       string
	}{
		{
			name: "valid findings with full details",
			output: []mcFinding{
				{
					Id:        "finding-12345-abcde",
					Type:      "SensitiveData:S3Object/Multiple",
					Title:     "Sensitive data found in S3 object",
					Severity:  "High",
					Category:  "CLASSIFICATION",
					Count:     "5",
					CreatedAt: "2026-01-15T10:00:00Z",
					Region:    "us-east-1",
				},
				{
					Id:        "finding-67890-fghij",
					Type:      "Policy:IAMUser/S3BucketPublic",
					Title:     "S3 bucket is publicly accessible",
					Severity:  "Medium",
					Category:  "POLICY",
					Count:     "1",
					CreatedAt: "2026-02-20T14:00:00Z",
					Region:    "us-west-2",
				},
			},
			wantLen:          2,
			wantResourceName: "finding-12345-abcde",
			wantId:           "finding-12345-abcde",
			wantType:         "SensitiveData:S3Object/Multiple",
			wantTitle:        "Sensitive data found in S3 object",
			wantSeverity:     "High",
			wantCategory:     "CLASSIFICATION",
			wantCount:        "5",
			wantCreatedAt:    "2026-01-15T10:00:00Z",
			wantRegion:       "us-east-1",
		},
		{
			name:    "empty results",
			output:  []mcFinding{},
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
			output: []mcFinding{
				{
					Id:        "",
					Type:      "",
					Title:     "",
					Severity:  "",
					Category:  "",
					Count:     "",
					CreatedAt: "",
					Region:    "",
				},
			},
			wantLen:          1,
			wantResourceName: "",
			wantId:           "",
			wantType:         "",
			wantTitle:        "",
			wantSeverity:     "",
			wantCategory:     "",
			wantCount:        "",
			wantCreatedAt:    "",
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
				if results[0].ServiceName != "Macie" {
					t.Errorf("expected ServiceName 'Macie', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "macie2:ListFindings" {
					t.Errorf("expected MethodName 'macie2:ListFindings', got '%s'", results[0].MethodName)
				}
				return
			}

			if tt.wantLen > 0 {
				if results[0].ServiceName != "Macie" {
					t.Errorf("expected ServiceName 'Macie', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "macie2:ListFindings" {
					t.Errorf("expected MethodName 'macie2:ListFindings', got '%s'", results[0].MethodName)
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
				if findingType, ok := results[0].Details["Type"].(string); ok {
					if findingType != tt.wantType {
						t.Errorf("expected Type '%s', got '%s'", tt.wantType, findingType)
					}
				} else if tt.wantType != "" {
					t.Errorf("expected Type in Details, got none")
				}
				if title, ok := results[0].Details["Title"].(string); ok {
					if title != tt.wantTitle {
						t.Errorf("expected Title '%s', got '%s'", tt.wantTitle, title)
					}
				} else if tt.wantTitle != "" {
					t.Errorf("expected Title in Details, got none")
				}
				if severity, ok := results[0].Details["Severity"].(string); ok {
					if severity != tt.wantSeverity {
						t.Errorf("expected Severity '%s', got '%s'", tt.wantSeverity, severity)
					}
				} else if tt.wantSeverity != "" {
					t.Errorf("expected Severity in Details, got none")
				}
				if category, ok := results[0].Details["Category"].(string); ok {
					if category != tt.wantCategory {
						t.Errorf("expected Category '%s', got '%s'", tt.wantCategory, category)
					}
				} else if tt.wantCategory != "" {
					t.Errorf("expected Category in Details, got none")
				}
				if count, ok := results[0].Details["Count"].(string); ok {
					if count != tt.wantCount {
						t.Errorf("expected Count '%s', got '%s'", tt.wantCount, count)
					}
				} else if tt.wantCount != "" {
					t.Errorf("expected Count in Details, got none")
				}
				if createdAt, ok := results[0].Details["CreatedAt"].(string); ok {
					if createdAt != tt.wantCreatedAt {
						t.Errorf("expected CreatedAt '%s', got '%s'", tt.wantCreatedAt, createdAt)
					}
				} else if tt.wantCreatedAt != "" {
					t.Errorf("expected CreatedAt in Details, got none")
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

func TestDescribeBucketsProcess(t *testing.T) {
	process := Macie2Calls[2].Process

	tests := []struct {
		name                        string
		output                      interface{}
		err                         error
		wantLen                     int
		wantError                   bool
		wantResourceName            string
		wantBucketName              string
		wantAccountId               string
		wantBucketArn               string
		wantObjectCount             string
		wantSizeInBytes             string
		wantClassifiableObjectCount string
		wantSensitivityScore        string
		wantRegion                  string
	}{
		{
			name: "valid buckets with full details",
			output: []mcMonitoredBucket{
				{
					BucketName:              "my-data-bucket",
					AccountId:               "111111111111",
					BucketArn:               "arn:aws:s3:::my-data-bucket",
					ObjectCount:             "1500",
					SizeInBytes:             "52428800",
					ClassifiableObjectCount: "1200",
					SensitivityScore:        "85",
					Region:                  "us-east-1",
				},
				{
					BucketName:              "logs-bucket",
					AccountId:               "111111111111",
					BucketArn:               "arn:aws:s3:::logs-bucket",
					ObjectCount:             "50000",
					SizeInBytes:             "1073741824",
					ClassifiableObjectCount: "0",
					SensitivityScore:        "10",
					Region:                  "us-west-2",
				},
			},
			wantLen:                     2,
			wantResourceName:            "my-data-bucket",
			wantBucketName:              "my-data-bucket",
			wantAccountId:               "111111111111",
			wantBucketArn:               "arn:aws:s3:::my-data-bucket",
			wantObjectCount:             "1500",
			wantSizeInBytes:             "52428800",
			wantClassifiableObjectCount: "1200",
			wantSensitivityScore:        "85",
			wantRegion:                  "us-east-1",
		},
		{
			name:    "empty results",
			output:  []mcMonitoredBucket{},
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
			output: []mcMonitoredBucket{
				{
					BucketName:              "",
					AccountId:               "",
					BucketArn:               "",
					ObjectCount:             "",
					SizeInBytes:             "",
					ClassifiableObjectCount: "",
					SensitivityScore:        "",
					Region:                  "",
				},
			},
			wantLen:                     1,
			wantResourceName:            "",
			wantBucketName:              "",
			wantAccountId:               "",
			wantBucketArn:               "",
			wantObjectCount:             "",
			wantSizeInBytes:             "",
			wantClassifiableObjectCount: "",
			wantSensitivityScore:        "",
			wantRegion:                  "",
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
				if results[0].ServiceName != "Macie" {
					t.Errorf("expected ServiceName 'Macie', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "macie2:DescribeBuckets" {
					t.Errorf("expected MethodName 'macie2:DescribeBuckets', got '%s'", results[0].MethodName)
				}
				return
			}

			if tt.wantLen > 0 {
				if results[0].ServiceName != "Macie" {
					t.Errorf("expected ServiceName 'Macie', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "macie2:DescribeBuckets" {
					t.Errorf("expected MethodName 'macie2:DescribeBuckets', got '%s'", results[0].MethodName)
				}
				if results[0].ResourceType != "monitored-bucket" {
					t.Errorf("expected ResourceType 'monitored-bucket', got '%s'", results[0].ResourceType)
				}
				if results[0].ResourceName != tt.wantResourceName {
					t.Errorf("expected ResourceName '%s', got '%s'", tt.wantResourceName, results[0].ResourceName)
				}
				if bucketName, ok := results[0].Details["BucketName"].(string); ok {
					if bucketName != tt.wantBucketName {
						t.Errorf("expected BucketName '%s', got '%s'", tt.wantBucketName, bucketName)
					}
				} else if tt.wantBucketName != "" {
					t.Errorf("expected BucketName in Details, got none")
				}
				if accountId, ok := results[0].Details["AccountId"].(string); ok {
					if accountId != tt.wantAccountId {
						t.Errorf("expected AccountId '%s', got '%s'", tt.wantAccountId, accountId)
					}
				} else if tt.wantAccountId != "" {
					t.Errorf("expected AccountId in Details, got none")
				}
				if bucketArn, ok := results[0].Details["BucketArn"].(string); ok {
					if bucketArn != tt.wantBucketArn {
						t.Errorf("expected BucketArn '%s', got '%s'", tt.wantBucketArn, bucketArn)
					}
				} else if tt.wantBucketArn != "" {
					t.Errorf("expected BucketArn in Details, got none")
				}
				if objectCount, ok := results[0].Details["ObjectCount"].(string); ok {
					if objectCount != tt.wantObjectCount {
						t.Errorf("expected ObjectCount '%s', got '%s'", tt.wantObjectCount, objectCount)
					}
				} else if tt.wantObjectCount != "" {
					t.Errorf("expected ObjectCount in Details, got none")
				}
				if sizeInBytes, ok := results[0].Details["SizeInBytes"].(string); ok {
					if sizeInBytes != tt.wantSizeInBytes {
						t.Errorf("expected SizeInBytes '%s', got '%s'", tt.wantSizeInBytes, sizeInBytes)
					}
				} else if tt.wantSizeInBytes != "" {
					t.Errorf("expected SizeInBytes in Details, got none")
				}
				if coc, ok := results[0].Details["ClassifiableObjectCount"].(string); ok {
					if coc != tt.wantClassifiableObjectCount {
						t.Errorf("expected ClassifiableObjectCount '%s', got '%s'", tt.wantClassifiableObjectCount, coc)
					}
				} else if tt.wantClassifiableObjectCount != "" {
					t.Errorf("expected ClassifiableObjectCount in Details, got none")
				}
				if ss, ok := results[0].Details["SensitivityScore"].(string); ok {
					if ss != tt.wantSensitivityScore {
						t.Errorf("expected SensitivityScore '%s', got '%s'", tt.wantSensitivityScore, ss)
					}
				} else if tt.wantSensitivityScore != "" {
					t.Errorf("expected SensitivityScore in Details, got none")
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
