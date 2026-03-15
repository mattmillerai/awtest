package codecommit

import (
	"fmt"
	"testing"
)

func TestListRepositoriesProcess(t *testing.T) {
	process := CodeCommitCalls[0].Process

	tests := []struct {
		name              string
		output            interface{}
		err               error
		wantLen           int
		wantError         bool
		wantResourceName  string
		wantName          string
		wantArn           string
		wantCloneUrlHttp  string
		wantCloneUrlSsh   string
		wantDefaultBranch string
		wantDescription   string
		wantRegion        string
	}{
		{
			name: "valid repos with full details",
			output: []ccRepository{
				{
					Name:          "my-repo",
					Arn:           "arn:aws:codecommit:us-east-1:111111111111:my-repo",
					CloneUrlHttp:  "https://git-codecommit.us-east-1.amazonaws.com/v1/repos/my-repo",
					CloneUrlSsh:   "ssh://git-codecommit.us-east-1.amazonaws.com/v1/repos/my-repo",
					DefaultBranch: "main",
					Description:   "My test repository",
					Region:        "us-east-1",
				},
				{
					Name:          "another-repo",
					Arn:           "arn:aws:codecommit:us-west-2:111111111111:another-repo",
					CloneUrlHttp:  "https://git-codecommit.us-west-2.amazonaws.com/v1/repos/another-repo",
					CloneUrlSsh:   "ssh://git-codecommit.us-west-2.amazonaws.com/v1/repos/another-repo",
					DefaultBranch: "develop",
					Description:   "Another repository",
					Region:        "us-west-2",
				},
			},
			wantLen:           2,
			wantResourceName:  "my-repo",
			wantName:          "my-repo",
			wantArn:           "arn:aws:codecommit:us-east-1:111111111111:my-repo",
			wantCloneUrlHttp:  "https://git-codecommit.us-east-1.amazonaws.com/v1/repos/my-repo",
			wantCloneUrlSsh:   "ssh://git-codecommit.us-east-1.amazonaws.com/v1/repos/my-repo",
			wantDefaultBranch: "main",
			wantDescription:   "My test repository",
			wantRegion:        "us-east-1",
		},
		{
			name:    "empty results",
			output:  []ccRepository{},
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
			output: []ccRepository{
				{
					Name:          "",
					Arn:           "",
					CloneUrlHttp:  "",
					CloneUrlSsh:   "",
					DefaultBranch: "",
					Description:   "",
					Region:        "",
				},
			},
			wantLen:           1,
			wantResourceName:  "",
			wantName:          "",
			wantArn:           "",
			wantCloneUrlHttp:  "",
			wantCloneUrlSsh:   "",
			wantDefaultBranch: "",
			wantDescription:   "",
			wantRegion:        "",
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
				if results[0].ServiceName != "CodeCommit" {
					t.Errorf("expected ServiceName 'CodeCommit', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "codecommit:ListRepositories" {
					t.Errorf("expected MethodName 'codecommit:ListRepositories', got '%s'", results[0].MethodName)
				}
				return
			}

			if tt.wantLen > 0 {
				if results[0].ServiceName != "CodeCommit" {
					t.Errorf("expected ServiceName 'CodeCommit', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "codecommit:ListRepositories" {
					t.Errorf("expected MethodName 'codecommit:ListRepositories', got '%s'", results[0].MethodName)
				}
				if results[0].ResourceType != "repository" {
					t.Errorf("expected ResourceType 'repository', got '%s'", results[0].ResourceType)
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
				if cloneHttp, ok := results[0].Details["CloneUrlHttp"].(string); ok {
					if cloneHttp != tt.wantCloneUrlHttp {
						t.Errorf("expected CloneUrlHttp '%s', got '%s'", tt.wantCloneUrlHttp, cloneHttp)
					}
				} else if tt.wantCloneUrlHttp != "" {
					t.Errorf("expected CloneUrlHttp in Details, got none")
				}
				if cloneSsh, ok := results[0].Details["CloneUrlSsh"].(string); ok {
					if cloneSsh != tt.wantCloneUrlSsh {
						t.Errorf("expected CloneUrlSsh '%s', got '%s'", tt.wantCloneUrlSsh, cloneSsh)
					}
				} else if tt.wantCloneUrlSsh != "" {
					t.Errorf("expected CloneUrlSsh in Details, got none")
				}
				if defaultBranch, ok := results[0].Details["DefaultBranch"].(string); ok {
					if defaultBranch != tt.wantDefaultBranch {
						t.Errorf("expected DefaultBranch '%s', got '%s'", tt.wantDefaultBranch, defaultBranch)
					}
				} else if tt.wantDefaultBranch != "" {
					t.Errorf("expected DefaultBranch in Details, got none")
				}
				if desc, ok := results[0].Details["Description"].(string); ok {
					if desc != tt.wantDescription {
						t.Errorf("expected Description '%s', got '%s'", tt.wantDescription, desc)
					}
				} else if tt.wantDescription != "" {
					t.Errorf("expected Description in Details, got none")
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

func TestListBranchesProcess(t *testing.T) {
	process := CodeCommitCalls[1].Process

	tests := []struct {
		name               string
		output             interface{}
		err                error
		wantLen            int
		wantError          bool
		wantResourceName   string
		wantRepositoryName string
		wantBranchName     string
		wantRegion         string
	}{
		{
			name: "valid branches",
			output: []ccBranch{
				{
					RepositoryName: "my-repo",
					BranchName:     "main",
					Region:         "us-east-1",
				},
				{
					RepositoryName: "my-repo",
					BranchName:     "develop",
					Region:         "us-east-1",
				},
				{
					RepositoryName: "another-repo",
					BranchName:     "main",
					Region:         "us-west-2",
				},
			},
			wantLen:            3,
			wantResourceName:   "my-repo/main",
			wantRepositoryName: "my-repo",
			wantBranchName:     "main",
			wantRegion:         "us-east-1",
		},
		{
			name:    "empty results",
			output:  []ccBranch{},
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
			output: []ccBranch{
				{
					RepositoryName: "",
					BranchName:     "",
					Region:         "",
				},
			},
			wantLen:            1,
			wantResourceName:   "/",
			wantRepositoryName: "",
			wantBranchName:     "",
			wantRegion:         "",
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
				if results[0].ServiceName != "CodeCommit" {
					t.Errorf("expected ServiceName 'CodeCommit', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "codecommit:ListBranches" {
					t.Errorf("expected MethodName 'codecommit:ListBranches', got '%s'", results[0].MethodName)
				}
				return
			}

			if tt.wantLen > 0 {
				if results[0].ServiceName != "CodeCommit" {
					t.Errorf("expected ServiceName 'CodeCommit', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "codecommit:ListBranches" {
					t.Errorf("expected MethodName 'codecommit:ListBranches', got '%s'", results[0].MethodName)
				}
				if results[0].ResourceType != "branch" {
					t.Errorf("expected ResourceType 'branch', got '%s'", results[0].ResourceType)
				}
				if results[0].ResourceName != tt.wantResourceName {
					t.Errorf("expected ResourceName '%s', got '%s'", tt.wantResourceName, results[0].ResourceName)
				}
				if repoName, ok := results[0].Details["RepositoryName"].(string); ok {
					if repoName != tt.wantRepositoryName {
						t.Errorf("expected RepositoryName '%s', got '%s'", tt.wantRepositoryName, repoName)
					}
				} else if tt.wantRepositoryName != "" {
					t.Errorf("expected RepositoryName in Details, got none")
				}
				if branchName, ok := results[0].Details["BranchName"].(string); ok {
					if branchName != tt.wantBranchName {
						t.Errorf("expected BranchName '%s', got '%s'", tt.wantBranchName, branchName)
					}
				} else if tt.wantBranchName != "" {
					t.Errorf("expected BranchName in Details, got none")
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
