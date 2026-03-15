package cognitouserpools

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cognitoidentityprovider"
	"testing"
	"time"
)

func TestProcess_ValidPools(t *testing.T) {
	process := CognitoUserPoolsCalls[0].Process

	created := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
	pools := []*cognitoidentityprovider.UserPoolDescriptionType{
		{
			Name:         aws.String("my-pool"),
			Id:           aws.String("us-east-1_abc123"),
			Status:       aws.String("Enabled"),
			CreationDate: &created,
		},
		{
			Name:         aws.String("another-pool"),
			Id:           aws.String("us-west-2_def456"),
			Status:       aws.String("Enabled"),
			CreationDate: &created,
		},
	}

	results := process(pools, nil, false)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	if results[0].ServiceName != "CognitoUserPools" {
		t.Errorf("expected ServiceName 'CognitoUserPools', got '%s'", results[0].ServiceName)
	}
	if results[0].MethodName != "cognito-idp:ListUserPools" {
		t.Errorf("expected MethodName 'cognito-idp:ListUserPools', got '%s'", results[0].MethodName)
	}
	if results[0].ResourceType != "user-pool" {
		t.Errorf("expected ResourceType 'user-pool', got '%s'", results[0].ResourceType)
	}
	if results[0].ResourceName != "my-pool" {
		t.Errorf("expected ResourceName 'my-pool', got '%s'", results[0].ResourceName)
	}
	if results[1].ResourceName != "another-pool" {
		t.Errorf("expected ResourceName 'another-pool', got '%s'", results[1].ResourceName)
	}
	if results[0].Details["Id"] != "us-east-1_abc123" {
		t.Errorf("expected Details Id 'us-east-1_abc123', got '%v'", results[0].Details["Id"])
	}
	if results[0].Details["Status"] != "Enabled" {
		t.Errorf("expected Details Status 'Enabled', got '%v'", results[0].Details["Status"])
	}
	if results[0].Details["CreationDate"] != "2025-06-15 10:30:00" {
		t.Errorf("expected Details CreationDate '2025-06-15 10:30:00', got '%v'", results[0].Details["CreationDate"])
	}
}

func TestProcess_EmptyResults(t *testing.T) {
	process := CognitoUserPoolsCalls[0].Process

	pools := []*cognitoidentityprovider.UserPoolDescriptionType{}
	results := process(pools, nil, false)

	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

func TestProcess_AccessDenied(t *testing.T) {
	process := CognitoUserPoolsCalls[0].Process

	testErr := fmt.Errorf("AccessDeniedException: User is not authorized")
	results := process(nil, testErr, false)

	if len(results) != 1 {
		t.Fatalf("expected 1 error result, got %d", len(results))
	}
	if results[0].Error == nil {
		t.Error("expected error in result, got nil")
	}
	if results[0].ServiceName != "CognitoUserPools" {
		t.Errorf("expected ServiceName 'CognitoUserPools', got '%s'", results[0].ServiceName)
	}
}

func TestProcess_NilFields(t *testing.T) {
	process := CognitoUserPoolsCalls[0].Process

	pools := []*cognitoidentityprovider.UserPoolDescriptionType{
		{
			Name: nil,
			Id:   nil,
		},
	}

	results := process(pools, nil, false)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ResourceName != "" {
		t.Errorf("expected empty ResourceName for nil Name, got '%s'", results[0].ResourceName)
	}
}
