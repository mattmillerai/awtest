package ecr

import (
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecr"
)

func TestDescribeRepositoriesProcess(t *testing.T) {
	process := ECRCalls[0].Process

	tests := []struct {
		name             string
		output           interface{}
		err              error
		wantLen          int
		wantError        bool
		wantResourceName string
		wantArn          string
	}{
		{
			name: "valid repositories",
			output: []*ecr.Repository{
				{
					RepositoryName: aws.String("my-app"),
					RepositoryUri:  aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/my-app"),
					RepositoryArn:  aws.String("arn:aws:ecr:us-east-1:123456789012:repository/my-app"),
				},
				{
					RepositoryName: aws.String("web-frontend"),
					RepositoryUri:  aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/web-frontend"),
					RepositoryArn:  aws.String("arn:aws:ecr:us-east-1:123456789012:repository/web-frontend"),
				},
			},
			wantLen:          2,
			wantResourceName: "my-app",
			wantArn:          "arn:aws:ecr:us-east-1:123456789012:repository/my-app",
		},
		{
			name:    "empty results",
			output:  []*ecr.Repository{},
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
			output: []*ecr.Repository{
				{
					RepositoryName: nil,
					RepositoryUri:  nil,
					RepositoryArn:  nil,
				},
			},
			wantLen:          1,
			wantResourceName: "",
			wantArn:          "",
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
				if results[0].ServiceName != "ECR" {
					t.Errorf("expected ServiceName 'ECR', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "ecr:DescribeRepositories" {
					t.Errorf("expected MethodName 'ecr:DescribeRepositories', got '%s'", results[0].MethodName)
				}
				return
			}

			if tt.wantLen > 0 {
				if results[0].ServiceName != "ECR" {
					t.Errorf("expected ServiceName 'ECR', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "ecr:DescribeRepositories" {
					t.Errorf("expected MethodName 'ecr:DescribeRepositories', got '%s'", results[0].MethodName)
				}
				if results[0].ResourceType != "repository" {
					t.Errorf("expected ResourceType 'repository', got '%s'", results[0].ResourceType)
				}
				if results[0].ResourceName != tt.wantResourceName {
					t.Errorf("expected ResourceName '%s', got '%s'", tt.wantResourceName, results[0].ResourceName)
				}
				if arn, ok := results[0].Details["RepositoryArn"].(string); ok {
					if arn != tt.wantArn {
						t.Errorf("expected RepositoryArn '%s', got '%s'", tt.wantArn, arn)
					}
				} else if tt.wantArn != "" {
					t.Errorf("expected RepositoryArn in Details, got none")
				}
			}
		})
	}
}

func TestListImagesProcess(t *testing.T) {
	process := ECRCalls[1].Process

	tests := []struct {
		name             string
		output           interface{}
		err              error
		wantLen          int
		wantError        bool
		wantResourceName string
	}{
		{
			name: "valid images with tags",
			output: []ecrImage{
				{RepoName: "my-app", ImageTag: "latest", ImageDigest: "sha256:abc123"},
				{RepoName: "my-app", ImageTag: "v1.0.0", ImageDigest: "sha256:def456"},
			},
			wantLen:          2,
			wantResourceName: "latest",
		},
		{
			name: "digest only (no tag)",
			output: []ecrImage{
				{RepoName: "my-app", ImageTag: "", ImageDigest: "sha256:abc123"},
			},
			wantLen:          1,
			wantResourceName: "sha256:abc123",
		},
		{
			name:    "empty results",
			output:  []ecrImage{},
			wantLen: 0,
		},
		{
			name:      "access denied error",
			output:    nil,
			err:       fmt.Errorf("AccessDeniedException: User is not authorized"),
			wantLen:   1,
			wantError: true,
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
				if results[0].ServiceName != "ECR" {
					t.Errorf("expected ServiceName 'ECR', got '%s'", results[0].ServiceName)
				}
				return
			}

			if tt.wantLen > 0 {
				if results[0].ServiceName != "ECR" {
					t.Errorf("expected ServiceName 'ECR', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "ecr:ListImages" {
					t.Errorf("expected MethodName 'ecr:ListImages', got '%s'", results[0].MethodName)
				}
				if results[0].ResourceType != "image" {
					t.Errorf("expected ResourceType 'image', got '%s'", results[0].ResourceType)
				}
				if results[0].ResourceName != tt.wantResourceName {
					t.Errorf("expected ResourceName '%s', got '%s'", tt.wantResourceName, results[0].ResourceName)
				}
			}
		})
	}
}

func TestGetRepositoryPolicyProcess(t *testing.T) {
	process := ECRCalls[2].Process

	tests := []struct {
		name             string
		output           interface{}
		err              error
		wantLen          int
		wantError        bool
		wantResourceName string
	}{
		{
			name: "valid policy",
			output: []ecrPolicy{
				{RepoName: "my-app", PolicyText: `{"Version":"2012-10-17","Statement":[]}`},
			},
			wantLen:          1,
			wantResourceName: "my-app",
		},
		{
			name: "empty policy text",
			output: []ecrPolicy{
				{RepoName: "my-repo", PolicyText: ""},
			},
			wantLen:          1,
			wantResourceName: "my-repo",
		},
		{
			name:    "empty results (no repos with policies)",
			output:  []ecrPolicy{},
			wantLen: 0,
		},
		{
			name:      "access denied error",
			output:    nil,
			err:       fmt.Errorf("AccessDeniedException: User is not authorized"),
			wantLen:   1,
			wantError: true,
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
				if results[0].ServiceName != "ECR" {
					t.Errorf("expected ServiceName 'ECR', got '%s'", results[0].ServiceName)
				}
				return
			}

			if tt.wantLen > 0 {
				if results[0].ServiceName != "ECR" {
					t.Errorf("expected ServiceName 'ECR', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "ecr:GetRepositoryPolicy" {
					t.Errorf("expected MethodName 'ecr:GetRepositoryPolicy', got '%s'", results[0].MethodName)
				}
				if results[0].ResourceType != "repository-policy" {
					t.Errorf("expected ResourceType 'repository-policy', got '%s'", results[0].ResourceType)
				}
				if results[0].ResourceName != tt.wantResourceName {
					t.Errorf("expected ResourceName '%s', got '%s'", tt.wantResourceName, results[0].ResourceName)
				}
			}
		})
	}
}
