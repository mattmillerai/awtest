package codecommit

import (
	"context"
	"fmt"
	"time"

	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/MillerMedia/awtest/cmd/awtest/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/codecommit"
)

type ccRepository struct {
	Name          string
	Arn           string
	CloneUrlHttp  string
	CloneUrlSsh   string
	DefaultBranch string
	Description   string
	Region        string
}

type ccBranch struct {
	RepositoryName string
	BranchName     string
	Region         string
}

var CodeCommitCalls = []types.AWSService{
	{
		Name: "codecommit:ListRepositories",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			var allRepos []ccRepository
			var lastErr error

			for _, region := range types.Regions {
				svc := codecommit.New(sess, &aws.Config{Region: aws.String(region)})

				var repoNames []*string
				input := &codecommit.ListRepositoriesInput{}
				for {
					output, err := svc.ListRepositoriesWithContext(ctx, input)
					if err != nil {
						lastErr = err
						utils.HandleAWSError(false, "codecommit:ListRepositories", err)
						break
					}

					for _, repo := range output.Repositories {
						if repo.RepositoryName != nil {
							repoNames = append(repoNames, repo.RepositoryName)
						}
					}

					if output.NextToken == nil {
						break
					}
					input.NextToken = output.NextToken
				}

				// Batch get repository details (max 25 per call)
				for i := 0; i < len(repoNames); i += 25 {
					end := i + 25
					if end > len(repoNames) {
						end = len(repoNames)
					}
					batch := repoNames[i:end]

					batchInput := &codecommit.BatchGetRepositoriesInput{
						RepositoryNames: batch,
					}
					batchOutput, err := svc.BatchGetRepositoriesWithContext(ctx, batchInput)
					if err != nil {
						lastErr = err
						utils.HandleAWSError(false, "codecommit:ListRepositories", err)
						break
					}

					for _, repo := range batchOutput.Repositories {
						name := ""
						if repo.RepositoryName != nil {
							name = *repo.RepositoryName
						}

						arn := ""
						if repo.Arn != nil {
							arn = *repo.Arn
						}

						cloneUrlHttp := ""
						if repo.CloneUrlHttp != nil {
							cloneUrlHttp = *repo.CloneUrlHttp
						}

						cloneUrlSsh := ""
						if repo.CloneUrlSsh != nil {
							cloneUrlSsh = *repo.CloneUrlSsh
						}

						defaultBranch := ""
						if repo.DefaultBranch != nil {
							defaultBranch = *repo.DefaultBranch
						}

						description := ""
						if repo.RepositoryDescription != nil {
							description = *repo.RepositoryDescription
						}

						allRepos = append(allRepos, ccRepository{
							Name:          name,
							Arn:           arn,
							CloneUrlHttp:  cloneUrlHttp,
							CloneUrlSsh:   cloneUrlSsh,
							DefaultBranch: defaultBranch,
							Description:   description,
							Region:        region,
						})
					}
				}
			}

			if len(allRepos) == 0 && lastErr != nil {
				return nil, lastErr
			}
			return allRepos, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "codecommit:ListRepositories", err)
				return []types.ScanResult{
					{
						ServiceName: "CodeCommit",
						MethodName:  "codecommit:ListRepositories",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			repos, ok := output.([]ccRepository)
			if !ok {
				utils.HandleAWSError(debug, "codecommit:ListRepositories", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, repo := range repos {
				results = append(results, types.ScanResult{
					ServiceName:  "CodeCommit",
					MethodName:   "codecommit:ListRepositories",
					ResourceType: "repository",
					ResourceName: repo.Name,
					Details: map[string]interface{}{
						"Name":          repo.Name,
						"Arn":           repo.Arn,
						"CloneUrlHttp":  repo.CloneUrlHttp,
						"CloneUrlSsh":   repo.CloneUrlSsh,
						"DefaultBranch": repo.DefaultBranch,
						"Description":   repo.Description,
						"Region":        repo.Region,
					},
					Timestamp: time.Now(),
				})

				utils.PrintResult(debug, "", "codecommit:ListRepositories",
					fmt.Sprintf("CodeCommit Repository: %s (HTTP: %s, Default Branch: %s, Region: %s)", utils.ColorizeItem(repo.Name), repo.CloneUrlHttp, repo.DefaultBranch, repo.Region), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
	{
		Name: "codecommit:ListBranches",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			var allBranches []ccBranch
			var lastErr error

			for _, region := range types.Regions {
				svc := codecommit.New(sess, &aws.Config{Region: aws.String(region)})

				var repoNames []*string
				listInput := &codecommit.ListRepositoriesInput{}
				for {
					listOutput, err := svc.ListRepositoriesWithContext(ctx, listInput)
					if err != nil {
						lastErr = err
						utils.HandleAWSError(false, "codecommit:ListBranches", err)
						break
					}

					for _, repo := range listOutput.Repositories {
						if repo.RepositoryName != nil {
							repoNames = append(repoNames, repo.RepositoryName)
						}
					}

					if listOutput.NextToken == nil {
						break
					}
					listInput.NextToken = listOutput.NextToken
				}

				for _, repoName := range repoNames {
					branchInput := &codecommit.ListBranchesInput{
						RepositoryName: repoName,
					}
					for {
						branchOutput, err := svc.ListBranchesWithContext(ctx, branchInput)
						if err != nil {
							lastErr = err
							utils.HandleAWSError(false, "codecommit:ListBranches", err)
							break
						}

						for _, branch := range branchOutput.Branches {
							branchName := ""
							if branch != nil {
								branchName = *branch
							}

							allBranches = append(allBranches, ccBranch{
								RepositoryName: *repoName,
								BranchName:     branchName,
								Region:         region,
							})
						}

						if branchOutput.NextToken == nil {
							break
						}
						branchInput.NextToken = branchOutput.NextToken
					}
				}
			}

			if len(allBranches) == 0 && lastErr != nil {
				return nil, lastErr
			}
			return allBranches, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "codecommit:ListBranches", err)
				return []types.ScanResult{
					{
						ServiceName: "CodeCommit",
						MethodName:  "codecommit:ListBranches",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			branches, ok := output.([]ccBranch)
			if !ok {
				utils.HandleAWSError(debug, "codecommit:ListBranches", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, branch := range branches {
				results = append(results, types.ScanResult{
					ServiceName:  "CodeCommit",
					MethodName:   "codecommit:ListBranches",
					ResourceType: "branch",
					ResourceName: branch.RepositoryName + "/" + branch.BranchName,
					Details: map[string]interface{}{
						"RepositoryName": branch.RepositoryName,
						"BranchName":     branch.BranchName,
						"Region":         branch.Region,
					},
					Timestamp: time.Now(),
				})

				utils.PrintResult(debug, "", "codecommit:ListBranches",
					fmt.Sprintf("CodeCommit Branch: %s/%s (Region: %s)", utils.ColorizeItem(branch.RepositoryName), branch.BranchName, branch.Region), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
