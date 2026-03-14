package mediaconvert

import (
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/mediaconvert"
)

func TestListQueuesProcess(t *testing.T) {
	process := MediaConvertCalls[0].Process

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
		wantType             string
		wantPricingPlan      string
		wantDescription      string
		wantSubmittedJobs    string
		wantProgressingJobs  string
		wantCreatedAt        string
		wantRegion           string
	}{
		{
			name: "valid queues with full details",
			output: []mcQueue{
				{
					Name:                 "Default",
					Arn:                  "arn:aws:mediaconvert:us-east-1:123456789012:queues/Default",
					Status:               "ACTIVE",
					Type:                 "SYSTEM",
					PricingPlan:          "ON_DEMAND",
					Description:          "Default queue",
					SubmittedJobsCount:   "5",
					ProgressingJobsCount: "2",
					CreatedAt:            "2026-01-15T10:00:00Z",
					Region:               "us-east-1",
				},
				{
					Name:                 "custom-queue",
					Arn:                  "arn:aws:mediaconvert:us-west-2:123456789012:queues/custom-queue",
					Status:               "PAUSED",
					Type:                 "CUSTOM",
					PricingPlan:          "RESERVED",
					Description:          "Custom processing queue",
					SubmittedJobsCount:   "0",
					ProgressingJobsCount: "0",
					CreatedAt:            "2026-02-20T14:00:00Z",
					Region:               "us-west-2",
				},
			},
			wantLen:              2,
			wantResourceName:     "Default",
			wantName:             "Default",
			wantArn:              "arn:aws:mediaconvert:us-east-1:123456789012:queues/Default",
			wantStatus:           "ACTIVE",
			wantType:             "SYSTEM",
			wantPricingPlan:      "ON_DEMAND",
			wantDescription:      "Default queue",
			wantSubmittedJobs:    "5",
			wantProgressingJobs:  "2",
			wantCreatedAt:        "2026-01-15T10:00:00Z",
			wantRegion:           "us-east-1",
		},
		{
			name:    "empty results",
			output:  []mcQueue{},
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
			output: []mcQueue{
				{
					Name:                 "",
					Arn:                  "",
					Status:               "",
					Type:                 "",
					PricingPlan:          "",
					Description:          "",
					SubmittedJobsCount:   "",
					ProgressingJobsCount: "",
					CreatedAt:            "",
					Region:               "",
				},
			},
			wantLen:              1,
			wantResourceName:     "",
			wantName:             "",
			wantArn:              "",
			wantStatus:           "",
			wantType:             "",
			wantPricingPlan:      "",
			wantDescription:      "",
			wantSubmittedJobs:    "",
			wantProgressingJobs:  "",
			wantCreatedAt:        "",
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
				if results[0].ServiceName != "MediaConvert" {
					t.Errorf("expected ServiceName 'MediaConvert', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "mediaconvert:ListQueues" {
					t.Errorf("expected MethodName 'mediaconvert:ListQueues', got '%s'", results[0].MethodName)
				}
				return
			}

			if tt.wantLen > 0 {
				if results[0].ServiceName != "MediaConvert" {
					t.Errorf("expected ServiceName 'MediaConvert', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "mediaconvert:ListQueues" {
					t.Errorf("expected MethodName 'mediaconvert:ListQueues', got '%s'", results[0].MethodName)
				}
				if results[0].ResourceType != "queue" {
					t.Errorf("expected ResourceType 'queue', got '%s'", results[0].ResourceType)
				}
				if results[0].ResourceName != tt.wantResourceName {
					t.Errorf("expected ResourceName '%s', got '%s'", tt.wantResourceName, results[0].ResourceName)
				}
				if v, ok := results[0].Details["Name"].(string); ok {
					if v != tt.wantName {
						t.Errorf("expected Name '%s', got '%s'", tt.wantName, v)
					}
				} else if tt.wantName != "" {
					t.Errorf("expected Name in Details, got none")
				}
				if v, ok := results[0].Details["Arn"].(string); ok {
					if v != tt.wantArn {
						t.Errorf("expected Arn '%s', got '%s'", tt.wantArn, v)
					}
				} else if tt.wantArn != "" {
					t.Errorf("expected Arn in Details, got none")
				}
				if v, ok := results[0].Details["Status"].(string); ok {
					if v != tt.wantStatus {
						t.Errorf("expected Status '%s', got '%s'", tt.wantStatus, v)
					}
				} else if tt.wantStatus != "" {
					t.Errorf("expected Status in Details, got none")
				}
				if v, ok := results[0].Details["Type"].(string); ok {
					if v != tt.wantType {
						t.Errorf("expected Type '%s', got '%s'", tt.wantType, v)
					}
				} else if tt.wantType != "" {
					t.Errorf("expected Type in Details, got none")
				}
				if v, ok := results[0].Details["PricingPlan"].(string); ok {
					if v != tt.wantPricingPlan {
						t.Errorf("expected PricingPlan '%s', got '%s'", tt.wantPricingPlan, v)
					}
				} else if tt.wantPricingPlan != "" {
					t.Errorf("expected PricingPlan in Details, got none")
				}
				if v, ok := results[0].Details["Description"].(string); ok {
					if v != tt.wantDescription {
						t.Errorf("expected Description '%s', got '%s'", tt.wantDescription, v)
					}
				} else if tt.wantDescription != "" {
					t.Errorf("expected Description in Details, got none")
				}
				if v, ok := results[0].Details["SubmittedJobsCount"].(string); ok {
					if v != tt.wantSubmittedJobs {
						t.Errorf("expected SubmittedJobsCount '%s', got '%s'", tt.wantSubmittedJobs, v)
					}
				} else if tt.wantSubmittedJobs != "" {
					t.Errorf("expected SubmittedJobsCount in Details, got none")
				}
				if v, ok := results[0].Details["ProgressingJobsCount"].(string); ok {
					if v != tt.wantProgressingJobs {
						t.Errorf("expected ProgressingJobsCount '%s', got '%s'", tt.wantProgressingJobs, v)
					}
				} else if tt.wantProgressingJobs != "" {
					t.Errorf("expected ProgressingJobsCount in Details, got none")
				}
				if v, ok := results[0].Details["CreatedAt"].(string); ok {
					if v != tt.wantCreatedAt {
						t.Errorf("expected CreatedAt '%s', got '%s'", tt.wantCreatedAt, v)
					}
				} else if tt.wantCreatedAt != "" {
					t.Errorf("expected CreatedAt in Details, got none")
				}
				if v, ok := results[0].Details["Region"].(string); ok {
					if v != tt.wantRegion {
						t.Errorf("expected Region '%s', got '%s'", tt.wantRegion, v)
					}
				} else if tt.wantRegion != "" {
					t.Errorf("expected Region in Details, got none")
				}
			}
		})
	}
}

func TestListJobsProcess(t *testing.T) {
	process := MediaConvertCalls[1].Process

	tests := []struct {
		name             string
		output           interface{}
		err              error
		wantLen          int
		wantError        bool
		wantResourceName string
		wantId           string
		wantArn          string
		wantStatus       string
		wantQueue        string
		wantRole         string
		wantJobTemplate  string
		wantCreatedAt    string
		wantCurrentPhase string
		wantErrorMessage string
		wantRegion       string
	}{
		{
			name: "valid jobs with full details",
			output: []mcJob{
				{
					Id:           "1234567890123-abc123",
					Arn:          "arn:aws:mediaconvert:us-east-1:123456789012:jobs/1234567890123-abc123",
					Status:       "COMPLETE",
					Queue:        "arn:aws:mediaconvert:us-east-1:123456789012:queues/Default",
					Role:         "arn:aws:iam::123456789012:role/MediaConvertRole",
					JobTemplate:  "System-Generic_Hd_Mp4_Avc_Aac_16x9_1280x720p_24Hz_4.5Mbps",
					CreatedAt:    "2026-01-15T10:00:00Z",
					CurrentPhase: "UPLOADING",
					ErrorMessage: "",
					Region:       "us-east-1",
				},
				{
					Id:           "9876543210987-def456",
					Arn:          "arn:aws:mediaconvert:us-west-2:123456789012:jobs/9876543210987-def456",
					Status:       "ERROR",
					Queue:        "arn:aws:mediaconvert:us-west-2:123456789012:queues/custom-queue",
					Role:         "arn:aws:iam::123456789012:role/VideoProcessingRole",
					JobTemplate:  "",
					CreatedAt:    "2026-02-20T14:00:00Z",
					CurrentPhase: "",
					ErrorMessage: "Input file not found",
					Region:       "us-west-2",
				},
			},
			wantLen:          2,
			wantResourceName: "1234567890123-abc123",
			wantId:           "1234567890123-abc123",
			wantArn:          "arn:aws:mediaconvert:us-east-1:123456789012:jobs/1234567890123-abc123",
			wantStatus:       "COMPLETE",
			wantQueue:        "arn:aws:mediaconvert:us-east-1:123456789012:queues/Default",
			wantRole:         "arn:aws:iam::123456789012:role/MediaConvertRole",
			wantJobTemplate:  "System-Generic_Hd_Mp4_Avc_Aac_16x9_1280x720p_24Hz_4.5Mbps",
			wantCreatedAt:    "2026-01-15T10:00:00Z",
			wantCurrentPhase: "UPLOADING",
			wantErrorMessage: "",
			wantRegion:       "us-east-1",
		},
		{
			name:    "empty results",
			output:  []mcJob{},
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
			output: []mcJob{
				{
					Id:           "",
					Arn:          "",
					Status:       "",
					Queue:        "",
					Role:         "",
					JobTemplate:  "",
					CreatedAt:    "",
					CurrentPhase: "",
					ErrorMessage: "",
					Region:       "",
				},
			},
			wantLen:          1,
			wantResourceName: "",
			wantId:           "",
			wantArn:          "",
			wantStatus:       "",
			wantQueue:        "",
			wantRole:         "",
			wantJobTemplate:  "",
			wantCreatedAt:    "",
			wantCurrentPhase: "",
			wantErrorMessage: "",
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
				if results[0].ServiceName != "MediaConvert" {
					t.Errorf("expected ServiceName 'MediaConvert', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "mediaconvert:ListJobs" {
					t.Errorf("expected MethodName 'mediaconvert:ListJobs', got '%s'", results[0].MethodName)
				}
				return
			}

			if tt.wantLen > 0 {
				if results[0].ServiceName != "MediaConvert" {
					t.Errorf("expected ServiceName 'MediaConvert', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "mediaconvert:ListJobs" {
					t.Errorf("expected MethodName 'mediaconvert:ListJobs', got '%s'", results[0].MethodName)
				}
				if results[0].ResourceType != "job" {
					t.Errorf("expected ResourceType 'job', got '%s'", results[0].ResourceType)
				}
				if results[0].ResourceName != tt.wantResourceName {
					t.Errorf("expected ResourceName '%s', got '%s'", tt.wantResourceName, results[0].ResourceName)
				}
				if v, ok := results[0].Details["Id"].(string); ok {
					if v != tt.wantId {
						t.Errorf("expected Id '%s', got '%s'", tt.wantId, v)
					}
				} else if tt.wantId != "" {
					t.Errorf("expected Id in Details, got none")
				}
				if v, ok := results[0].Details["Arn"].(string); ok {
					if v != tt.wantArn {
						t.Errorf("expected Arn '%s', got '%s'", tt.wantArn, v)
					}
				} else if tt.wantArn != "" {
					t.Errorf("expected Arn in Details, got none")
				}
				if v, ok := results[0].Details["Status"].(string); ok {
					if v != tt.wantStatus {
						t.Errorf("expected Status '%s', got '%s'", tt.wantStatus, v)
					}
				} else if tt.wantStatus != "" {
					t.Errorf("expected Status in Details, got none")
				}
				if v, ok := results[0].Details["Queue"].(string); ok {
					if v != tt.wantQueue {
						t.Errorf("expected Queue '%s', got '%s'", tt.wantQueue, v)
					}
				} else if tt.wantQueue != "" {
					t.Errorf("expected Queue in Details, got none")
				}
				if v, ok := results[0].Details["Role"].(string); ok {
					if v != tt.wantRole {
						t.Errorf("expected Role '%s', got '%s'", tt.wantRole, v)
					}
				} else if tt.wantRole != "" {
					t.Errorf("expected Role in Details, got none")
				}
				if v, ok := results[0].Details["JobTemplate"].(string); ok {
					if v != tt.wantJobTemplate {
						t.Errorf("expected JobTemplate '%s', got '%s'", tt.wantJobTemplate, v)
					}
				} else if tt.wantJobTemplate != "" {
					t.Errorf("expected JobTemplate in Details, got none")
				}
				if v, ok := results[0].Details["CreatedAt"].(string); ok {
					if v != tt.wantCreatedAt {
						t.Errorf("expected CreatedAt '%s', got '%s'", tt.wantCreatedAt, v)
					}
				} else if tt.wantCreatedAt != "" {
					t.Errorf("expected CreatedAt in Details, got none")
				}
				if v, ok := results[0].Details["CurrentPhase"].(string); ok {
					if v != tt.wantCurrentPhase {
						t.Errorf("expected CurrentPhase '%s', got '%s'", tt.wantCurrentPhase, v)
					}
				} else if tt.wantCurrentPhase != "" {
					t.Errorf("expected CurrentPhase in Details, got none")
				}
				if v, ok := results[0].Details["ErrorMessage"].(string); ok {
					if v != tt.wantErrorMessage {
						t.Errorf("expected ErrorMessage '%s', got '%s'", tt.wantErrorMessage, v)
					}
				} else if tt.wantErrorMessage != "" {
					t.Errorf("expected ErrorMessage in Details, got none")
				}
				if v, ok := results[0].Details["Region"].(string); ok {
					if v != tt.wantRegion {
						t.Errorf("expected Region '%s', got '%s'", tt.wantRegion, v)
					}
				} else if tt.wantRegion != "" {
					t.Errorf("expected Region in Details, got none")
				}
			}
		})
	}
}

func TestListPresetsProcess(t *testing.T) {
	process := MediaConvertCalls[2].Process

	tests := []struct {
		name             string
		output           interface{}
		err              error
		wantLen          int
		wantError        bool
		wantResourceName string
		wantName         string
		wantArn          string
		wantDescription  string
		wantType         string
		wantCategory     string
		wantCreatedAt    string
		wantRegion       string
	}{
		{
			name: "valid presets with full details",
			output: []mcPreset{
				{
					Name:        "System-Generic_Hd_Mp4",
					Arn:         "arn:aws:mediaconvert:us-east-1:123456789012:presets/System-Generic_Hd_Mp4",
					Description: "Generic HD MP4 preset",
					Type:        "SYSTEM",
					Category:    "VIDEO",
					CreatedAt:   "2026-01-15T10:00:00Z",
					Region:      "us-east-1",
				},
				{
					Name:        "custom-audio-preset",
					Arn:         "arn:aws:mediaconvert:us-west-2:123456789012:presets/custom-audio-preset",
					Description: "Custom audio encoding",
					Type:        "CUSTOM",
					Category:    "AUDIO",
					CreatedAt:   "2026-02-20T14:00:00Z",
					Region:      "us-west-2",
				},
			},
			wantLen:          2,
			wantResourceName: "System-Generic_Hd_Mp4",
			wantName:         "System-Generic_Hd_Mp4",
			wantArn:          "arn:aws:mediaconvert:us-east-1:123456789012:presets/System-Generic_Hd_Mp4",
			wantDescription:  "Generic HD MP4 preset",
			wantType:         "SYSTEM",
			wantCategory:     "VIDEO",
			wantCreatedAt:    "2026-01-15T10:00:00Z",
			wantRegion:       "us-east-1",
		},
		{
			name:    "empty results",
			output:  []mcPreset{},
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
			output: []mcPreset{
				{
					Name:        "",
					Arn:         "",
					Description: "",
					Type:        "",
					Category:    "",
					CreatedAt:   "",
					Region:      "",
				},
			},
			wantLen:          1,
			wantResourceName: "",
			wantName:         "",
			wantArn:          "",
			wantDescription:  "",
			wantType:         "",
			wantCategory:     "",
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
				if results[0].ServiceName != "MediaConvert" {
					t.Errorf("expected ServiceName 'MediaConvert', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "mediaconvert:ListPresets" {
					t.Errorf("expected MethodName 'mediaconvert:ListPresets', got '%s'", results[0].MethodName)
				}
				return
			}

			if tt.wantLen > 0 {
				if results[0].ServiceName != "MediaConvert" {
					t.Errorf("expected ServiceName 'MediaConvert', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "mediaconvert:ListPresets" {
					t.Errorf("expected MethodName 'mediaconvert:ListPresets', got '%s'", results[0].MethodName)
				}
				if results[0].ResourceType != "preset" {
					t.Errorf("expected ResourceType 'preset', got '%s'", results[0].ResourceType)
				}
				if results[0].ResourceName != tt.wantResourceName {
					t.Errorf("expected ResourceName '%s', got '%s'", tt.wantResourceName, results[0].ResourceName)
				}
				if v, ok := results[0].Details["Name"].(string); ok {
					if v != tt.wantName {
						t.Errorf("expected Name '%s', got '%s'", tt.wantName, v)
					}
				} else if tt.wantName != "" {
					t.Errorf("expected Name in Details, got none")
				}
				if v, ok := results[0].Details["Arn"].(string); ok {
					if v != tt.wantArn {
						t.Errorf("expected Arn '%s', got '%s'", tt.wantArn, v)
					}
				} else if tt.wantArn != "" {
					t.Errorf("expected Arn in Details, got none")
				}
				if v, ok := results[0].Details["Description"].(string); ok {
					if v != tt.wantDescription {
						t.Errorf("expected Description '%s', got '%s'", tt.wantDescription, v)
					}
				} else if tt.wantDescription != "" {
					t.Errorf("expected Description in Details, got none")
				}
				if v, ok := results[0].Details["Type"].(string); ok {
					if v != tt.wantType {
						t.Errorf("expected Type '%s', got '%s'", tt.wantType, v)
					}
				} else if tt.wantType != "" {
					t.Errorf("expected Type in Details, got none")
				}
				if v, ok := results[0].Details["Category"].(string); ok {
					if v != tt.wantCategory {
						t.Errorf("expected Category '%s', got '%s'", tt.wantCategory, v)
					}
				} else if tt.wantCategory != "" {
					t.Errorf("expected Category in Details, got none")
				}
				if v, ok := results[0].Details["CreatedAt"].(string); ok {
					if v != tt.wantCreatedAt {
						t.Errorf("expected CreatedAt '%s', got '%s'", tt.wantCreatedAt, v)
					}
				} else if tt.wantCreatedAt != "" {
					t.Errorf("expected CreatedAt in Details, got none")
				}
				if v, ok := results[0].Details["Region"].(string); ok {
					if v != tt.wantRegion {
						t.Errorf("expected Region '%s', got '%s'", tt.wantRegion, v)
					}
				} else if tt.wantRegion != "" {
					t.Errorf("expected Region in Details, got none")
				}
			}
		})
	}
}

func TestExtractQueue(t *testing.T) {
	ts := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name              string
		input             *mediaconvert.Queue
		region            string
		wantName          string
		wantArn           string
		wantStatus        string
		wantType          string
		wantPricingPlan   string
		wantDescription   string
		wantSubmitted     string
		wantProgressing   string
		wantCreatedAt     string
		wantRegion        string
	}{
		{
			name: "all fields populated",
			input: &mediaconvert.Queue{
				Name:                 aws.String("Default"),
				Arn:                  aws.String("arn:aws:mediaconvert:us-east-1:123456789012:queues/Default"),
				Status:               aws.String("ACTIVE"),
				Type:                 aws.String("SYSTEM"),
				PricingPlan:          aws.String("ON_DEMAND"),
				Description:          aws.String("Default queue"),
				SubmittedJobsCount:   aws.Int64(5),
				ProgressingJobsCount: aws.Int64(2),
				CreatedAt:            &ts,
			},
			region:          "us-east-1",
			wantName:        "Default",
			wantArn:         "arn:aws:mediaconvert:us-east-1:123456789012:queues/Default",
			wantStatus:      "ACTIVE",
			wantType:        "SYSTEM",
			wantPricingPlan: "ON_DEMAND",
			wantDescription: "Default queue",
			wantSubmitted:   "5",
			wantProgressing: "2",
			wantCreatedAt:   "2026-01-15T10:00:00Z",
			wantRegion:      "us-east-1",
		},
		{
			name:            "all fields nil",
			input:           &mediaconvert.Queue{},
			region:          "eu-west-1",
			wantName:        "",
			wantArn:         "",
			wantStatus:      "",
			wantType:        "",
			wantPricingPlan: "",
			wantDescription: "",
			wantSubmitted:   "",
			wantProgressing: "",
			wantCreatedAt:   "",
			wantRegion:      "eu-west-1",
		},
		{
			name: "partial fields populated",
			input: &mediaconvert.Queue{
				Name:               aws.String("partial-queue"),
				Status:             aws.String("PAUSED"),
				SubmittedJobsCount: aws.Int64(0),
			},
			region:          "us-west-2",
			wantName:        "partial-queue",
			wantArn:         "",
			wantStatus:      "PAUSED",
			wantType:        "",
			wantPricingPlan: "",
			wantDescription: "",
			wantSubmitted:   "0",
			wantProgressing: "",
			wantCreatedAt:   "",
			wantRegion:      "us-west-2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractQueue(tt.input, tt.region)
			if result.Name != tt.wantName {
				t.Errorf("Name: got %q, want %q", result.Name, tt.wantName)
			}
			if result.Arn != tt.wantArn {
				t.Errorf("Arn: got %q, want %q", result.Arn, tt.wantArn)
			}
			if result.Status != tt.wantStatus {
				t.Errorf("Status: got %q, want %q", result.Status, tt.wantStatus)
			}
			if result.Type != tt.wantType {
				t.Errorf("Type: got %q, want %q", result.Type, tt.wantType)
			}
			if result.PricingPlan != tt.wantPricingPlan {
				t.Errorf("PricingPlan: got %q, want %q", result.PricingPlan, tt.wantPricingPlan)
			}
			if result.Description != tt.wantDescription {
				t.Errorf("Description: got %q, want %q", result.Description, tt.wantDescription)
			}
			if result.SubmittedJobsCount != tt.wantSubmitted {
				t.Errorf("SubmittedJobsCount: got %q, want %q", result.SubmittedJobsCount, tt.wantSubmitted)
			}
			if result.ProgressingJobsCount != tt.wantProgressing {
				t.Errorf("ProgressingJobsCount: got %q, want %q", result.ProgressingJobsCount, tt.wantProgressing)
			}
			if result.CreatedAt != tt.wantCreatedAt {
				t.Errorf("CreatedAt: got %q, want %q", result.CreatedAt, tt.wantCreatedAt)
			}
			if result.Region != tt.wantRegion {
				t.Errorf("Region: got %q, want %q", result.Region, tt.wantRegion)
			}
		})
	}
}

func TestExtractJob(t *testing.T) {
	ts := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name             string
		input            *mediaconvert.Job
		region           string
		wantId           string
		wantArn          string
		wantStatus       string
		wantQueue        string
		wantRole         string
		wantJobTemplate  string
		wantCreatedAt    string
		wantCurrentPhase string
		wantErrorMessage string
		wantRegion       string
	}{
		{
			name: "all fields populated",
			input: &mediaconvert.Job{
				Id:           aws.String("1234567890123-abc123"),
				Arn:          aws.String("arn:aws:mediaconvert:us-east-1:123456789012:jobs/1234567890123-abc123"),
				Status:       aws.String("COMPLETE"),
				Queue:        aws.String("arn:aws:mediaconvert:us-east-1:123456789012:queues/Default"),
				Role:         aws.String("arn:aws:iam::123456789012:role/MediaConvertRole"),
				JobTemplate:  aws.String("System-Generic_Hd_Mp4"),
				CreatedAt:    &ts,
				CurrentPhase: aws.String("UPLOADING"),
				ErrorMessage: aws.String(""),
			},
			region:           "us-east-1",
			wantId:           "1234567890123-abc123",
			wantArn:          "arn:aws:mediaconvert:us-east-1:123456789012:jobs/1234567890123-abc123",
			wantStatus:       "COMPLETE",
			wantQueue:        "arn:aws:mediaconvert:us-east-1:123456789012:queues/Default",
			wantRole:         "arn:aws:iam::123456789012:role/MediaConvertRole",
			wantJobTemplate:  "System-Generic_Hd_Mp4",
			wantCreatedAt:    "2026-01-15T10:00:00Z",
			wantCurrentPhase: "UPLOADING",
			wantErrorMessage: "",
			wantRegion:       "us-east-1",
		},
		{
			name:             "all fields nil",
			input:            &mediaconvert.Job{},
			region:           "eu-west-1",
			wantId:           "",
			wantArn:          "",
			wantStatus:       "",
			wantQueue:        "",
			wantRole:         "",
			wantJobTemplate:  "",
			wantCreatedAt:    "",
			wantCurrentPhase: "",
			wantErrorMessage: "",
			wantRegion:       "eu-west-1",
		},
		{
			name: "partial fields - error job",
			input: &mediaconvert.Job{
				Id:           aws.String("failed-job-id"),
				Status:       aws.String("ERROR"),
				ErrorMessage: aws.String("Input file not found"),
			},
			region:           "us-west-2",
			wantId:           "failed-job-id",
			wantArn:          "",
			wantStatus:       "ERROR",
			wantQueue:        "",
			wantRole:         "",
			wantJobTemplate:  "",
			wantCreatedAt:    "",
			wantCurrentPhase: "",
			wantErrorMessage: "Input file not found",
			wantRegion:       "us-west-2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractJob(tt.input, tt.region)
			if result.Id != tt.wantId {
				t.Errorf("Id: got %q, want %q", result.Id, tt.wantId)
			}
			if result.Arn != tt.wantArn {
				t.Errorf("Arn: got %q, want %q", result.Arn, tt.wantArn)
			}
			if result.Status != tt.wantStatus {
				t.Errorf("Status: got %q, want %q", result.Status, tt.wantStatus)
			}
			if result.Queue != tt.wantQueue {
				t.Errorf("Queue: got %q, want %q", result.Queue, tt.wantQueue)
			}
			if result.Role != tt.wantRole {
				t.Errorf("Role: got %q, want %q", result.Role, tt.wantRole)
			}
			if result.JobTemplate != tt.wantJobTemplate {
				t.Errorf("JobTemplate: got %q, want %q", result.JobTemplate, tt.wantJobTemplate)
			}
			if result.CreatedAt != tt.wantCreatedAt {
				t.Errorf("CreatedAt: got %q, want %q", result.CreatedAt, tt.wantCreatedAt)
			}
			if result.CurrentPhase != tt.wantCurrentPhase {
				t.Errorf("CurrentPhase: got %q, want %q", result.CurrentPhase, tt.wantCurrentPhase)
			}
			if result.ErrorMessage != tt.wantErrorMessage {
				t.Errorf("ErrorMessage: got %q, want %q", result.ErrorMessage, tt.wantErrorMessage)
			}
			if result.Region != tt.wantRegion {
				t.Errorf("Region: got %q, want %q", result.Region, tt.wantRegion)
			}
		})
	}
}

func TestExtractPreset(t *testing.T) {
	ts := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name            string
		input           *mediaconvert.Preset
		region          string
		wantName        string
		wantArn         string
		wantDescription string
		wantType        string
		wantCategory    string
		wantCreatedAt   string
		wantRegion      string
	}{
		{
			name: "all fields populated",
			input: &mediaconvert.Preset{
				Name:        aws.String("System-Generic_Hd_Mp4"),
				Arn:         aws.String("arn:aws:mediaconvert:us-east-1:123456789012:presets/System-Generic_Hd_Mp4"),
				Description: aws.String("Generic HD MP4 preset"),
				Type:        aws.String("SYSTEM"),
				Category:    aws.String("VIDEO"),
				CreatedAt:   &ts,
			},
			region:          "us-east-1",
			wantName:        "System-Generic_Hd_Mp4",
			wantArn:         "arn:aws:mediaconvert:us-east-1:123456789012:presets/System-Generic_Hd_Mp4",
			wantDescription: "Generic HD MP4 preset",
			wantType:        "SYSTEM",
			wantCategory:    "VIDEO",
			wantCreatedAt:   "2026-01-15T10:00:00Z",
			wantRegion:      "us-east-1",
		},
		{
			name:            "all fields nil",
			input:           &mediaconvert.Preset{},
			region:          "eu-west-1",
			wantName:        "",
			wantArn:         "",
			wantDescription: "",
			wantType:        "",
			wantCategory:    "",
			wantCreatedAt:   "",
			wantRegion:      "eu-west-1",
		},
		{
			name: "partial fields populated",
			input: &mediaconvert.Preset{
				Name:     aws.String("custom-preset"),
				Type:     aws.String("CUSTOM"),
				Category: aws.String("AUDIO"),
			},
			region:          "us-west-2",
			wantName:        "custom-preset",
			wantArn:         "",
			wantDescription: "",
			wantType:        "CUSTOM",
			wantCategory:    "AUDIO",
			wantCreatedAt:   "",
			wantRegion:      "us-west-2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractPreset(tt.input, tt.region)
			if result.Name != tt.wantName {
				t.Errorf("Name: got %q, want %q", result.Name, tt.wantName)
			}
			if result.Arn != tt.wantArn {
				t.Errorf("Arn: got %q, want %q", result.Arn, tt.wantArn)
			}
			if result.Description != tt.wantDescription {
				t.Errorf("Description: got %q, want %q", result.Description, tt.wantDescription)
			}
			if result.Type != tt.wantType {
				t.Errorf("Type: got %q, want %q", result.Type, tt.wantType)
			}
			if result.Category != tt.wantCategory {
				t.Errorf("Category: got %q, want %q", result.Category, tt.wantCategory)
			}
			if result.CreatedAt != tt.wantCreatedAt {
				t.Errorf("CreatedAt: got %q, want %q", result.CreatedAt, tt.wantCreatedAt)
			}
			if result.Region != tt.wantRegion {
				t.Errorf("Region: got %q, want %q", result.Region, tt.wantRegion)
			}
		})
	}
}
