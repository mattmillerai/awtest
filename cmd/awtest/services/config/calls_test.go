package config

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/configservice"
	"testing"
)

func TestProcess_ValidConfig(t *testing.T) {
	process := ConfigCalls[0].Process
	results := process(&configResults{
		Recorders: []*configservice.ConfigurationRecorder{
			{
				Name:    aws.String("default"),
				RoleARN: aws.String("arn:aws:iam::123456789012:role/config-role"),
			},
		},
		RecorderStatuses: []*configservice.ConfigurationRecorderStatus{
			{
				Name:      aws.String("default"),
				Recording: aws.Bool(true),
			},
		},
		Rules: []*configservice.ConfigRule{
			{
				ConfigRuleName:  aws.String("s3-bucket-versioning"),
				ConfigRuleState: aws.String("ACTIVE"),
				Source:          &configservice.Source{Owner: aws.String("AWS")},
			},
		},
	}, nil, false)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Verify recorder result
	if results[0].ServiceName != "Config" {
		t.Errorf("expected ServiceName 'Config', got '%s'", results[0].ServiceName)
	}
	if results[0].MethodName != "config:DescribeConfigurationRecorders" {
		t.Errorf("expected MethodName 'config:DescribeConfigurationRecorders', got '%s'", results[0].MethodName)
	}
	if results[0].ResourceType != "configuration-recorder" {
		t.Errorf("expected ResourceType 'configuration-recorder', got '%s'", results[0].ResourceType)
	}
	if results[0].ResourceName != "default" {
		t.Errorf("expected ResourceName 'default', got '%s'", results[0].ResourceName)
	}
	if results[0].Details["RoleARN"] != "arn:aws:iam::123456789012:role/config-role" {
		t.Errorf("expected RoleARN in Details, got '%v'", results[0].Details["RoleARN"])
	}
	if results[0].Details["RecordingStatus"] != "Recording" {
		t.Errorf("expected RecordingStatus 'Recording', got '%v'", results[0].Details["RecordingStatus"])
	}

	// Verify rule result
	if results[1].ServiceName != "Config" {
		t.Errorf("expected ServiceName 'Config', got '%s'", results[1].ServiceName)
	}
	if results[1].MethodName != "config:DescribeConfigRules" {
		t.Errorf("expected MethodName 'config:DescribeConfigRules', got '%s'", results[1].MethodName)
	}
	if results[1].ResourceType != "config-rule" {
		t.Errorf("expected ResourceType 'config-rule', got '%s'", results[1].ResourceType)
	}
	if results[1].ResourceName != "s3-bucket-versioning" {
		t.Errorf("expected ResourceName 's3-bucket-versioning', got '%s'", results[1].ResourceName)
	}
	if results[1].Details["State"] != "ACTIVE" {
		t.Errorf("expected State 'ACTIVE', got '%v'", results[1].Details["State"])
	}
	if results[1].Details["Owner"] != "AWS" {
		t.Errorf("expected Owner 'AWS', got '%v'", results[1].Details["Owner"])
	}
}

func TestProcess_RecorderStopped(t *testing.T) {
	process := ConfigCalls[0].Process
	results := process(&configResults{
		Recorders: []*configservice.ConfigurationRecorder{
			{
				Name:    aws.String("default"),
				RoleARN: aws.String("arn:aws:iam::123456789012:role/config-role"),
			},
		},
		RecorderStatuses: []*configservice.ConfigurationRecorderStatus{
			{
				Name:      aws.String("default"),
				Recording: aws.Bool(false),
			},
		},
	}, nil, false)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Details["RecordingStatus"] != "Stopped" {
		t.Errorf("expected RecordingStatus 'Stopped', got '%v'", results[0].Details["RecordingStatus"])
	}
}

func TestProcess_EmptyRecorders(t *testing.T) {
	process := ConfigCalls[0].Process
	results := process(&configResults{
		Recorders: []*configservice.ConfigurationRecorder{},
		Rules: []*configservice.ConfigRule{
			{
				ConfigRuleName:  aws.String("my-rule"),
				ConfigRuleState: aws.String("ACTIVE"),
				Source:          &configservice.Source{Owner: aws.String("AWS")},
			},
		},
	}, nil, false)

	if len(results) != 1 {
		t.Fatalf("expected 1 result (rule only), got %d", len(results))
	}
	if results[0].ResourceType != "config-rule" {
		t.Errorf("expected ResourceType 'config-rule', got '%s'", results[0].ResourceType)
	}
}

func TestProcess_EmptyRules(t *testing.T) {
	process := ConfigCalls[0].Process
	results := process(&configResults{
		Recorders: []*configservice.ConfigurationRecorder{
			{
				Name:    aws.String("default"),
				RoleARN: aws.String("arn:aws:iam::123456789012:role/config-role"),
			},
		},
		RecorderStatuses: []*configservice.ConfigurationRecorderStatus{
			{
				Name:      aws.String("default"),
				Recording: aws.Bool(true),
			},
		},
		Rules: []*configservice.ConfigRule{},
	}, nil, false)

	if len(results) != 1 {
		t.Fatalf("expected 1 result (recorder only), got %d", len(results))
	}
	if results[0].ResourceType != "configuration-recorder" {
		t.Errorf("expected ResourceType 'configuration-recorder', got '%s'", results[0].ResourceType)
	}
}

func TestProcess_AccessDenied(t *testing.T) {
	process := ConfigCalls[0].Process
	testErr := fmt.Errorf("AccessDeniedException: User is not authorized")
	results := process(nil, testErr, false)

	if len(results) != 1 {
		t.Fatalf("expected 1 error result, got %d", len(results))
	}
	if results[0].Error == nil {
		t.Error("expected error in result, got nil")
	}
	if results[0].ServiceName != "Config" {
		t.Errorf("expected ServiceName 'Config', got '%s'", results[0].ServiceName)
	}
	if results[0].MethodName != "config:DescribeConfigurationRecorders" {
		t.Errorf("expected MethodName 'config:DescribeConfigurationRecorders', got '%s'", results[0].MethodName)
	}
}

func TestProcess_NilFields(t *testing.T) {
	process := ConfigCalls[0].Process
	results := process(&configResults{
		Recorders: []*configservice.ConfigurationRecorder{
			{
				Name:    nil,
				RoleARN: nil,
			},
		},
		Rules: []*configservice.ConfigRule{
			{
				ConfigRuleName:  nil,
				ConfigRuleState: nil,
				Source:          nil,
			},
		},
	}, nil, false)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].ResourceName != "" {
		t.Errorf("expected empty ResourceName for nil Name, got '%s'", results[0].ResourceName)
	}
	if results[0].Details["RoleARN"] != "" {
		t.Errorf("expected empty RoleARN for nil, got '%v'", results[0].Details["RoleARN"])
	}
	if results[0].Details["RecordingStatus"] != "Stopped" {
		t.Errorf("expected RecordingStatus 'Stopped' for missing status, got '%v'", results[0].Details["RecordingStatus"])
	}
	if results[1].ResourceName != "" {
		t.Errorf("expected empty ResourceName for nil ConfigRuleName, got '%s'", results[1].ResourceName)
	}
	if results[1].Details["State"] != "" {
		t.Errorf("expected empty State for nil, got '%v'", results[1].Details["State"])
	}
	if results[1].Details["Owner"] != "" {
		t.Errorf("expected empty Owner for nil Source, got '%v'", results[1].Details["Owner"])
	}
}
