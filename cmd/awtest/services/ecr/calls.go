package ecr

import (
	"context"
	"fmt"
	"time"

	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/MillerMedia/awtest/cmd/awtest/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecr"
)

type ecrImage struct {
	RepoName    string
	ImageTag    string
	ImageDigest string
}

type ecrPolicy struct {
	RepoName   string
	PolicyText string
}

var ECRCalls = []types.AWSService{
	{
		Name: "ecr:DescribeRepositories",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			var allRepos []*ecr.Repository
			for _, region := range types.Regions {
				sess.Config.Region = aws.String(region)
				svc := ecr.New(sess)
				input := &ecr.DescribeRepositoriesInput{}
				for {
					output, err := svc.DescribeRepositoriesWithContext(ctx, input)
					if err != nil {
						return nil, err
					}
					allRepos = append(allRepos, output.Repositories...)
					if output.NextToken == nil {
						break
					}
					input.NextToken = output.NextToken
				}
			}
			return allRepos, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "ecr:DescribeRepositories", err)
				return []types.ScanResult{
					{
						ServiceName: "ECR",
						MethodName:  "ecr:DescribeRepositories",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			repos, ok := output.([]*ecr.Repository)
			if !ok {
				utils.HandleAWSError(debug, "ecr:DescribeRepositories", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, repo := range repos {
				repoName := ""
				if repo.RepositoryName != nil {
					repoName = *repo.RepositoryName
				}

				repoUri := ""
				if repo.RepositoryUri != nil {
					repoUri = *repo.RepositoryUri
				}

				repoArn := ""
				if repo.RepositoryArn != nil {
					repoArn = *repo.RepositoryArn
				}

				results = append(results, types.ScanResult{
					ServiceName:  "ECR",
					MethodName:   "ecr:DescribeRepositories",
					ResourceType: "repository",
					ResourceName: repoName,
					Details: map[string]interface{}{
						"RepositoryArn": repoArn,
					},
					Timestamp: time.Now(),
				})

				utils.PrintResult(debug, "", "ecr:DescribeRepositories",
					fmt.Sprintf("ECR Repository: %s (URI: %s)", utils.ColorizeItem(repoName), repoUri), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
	{
		Name: "ecr:ListImages",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			var allImages []ecrImage

			for _, region := range types.Regions {
				sess.Config.Region = aws.String(region)
				svc := ecr.New(sess)

				repoInput := &ecr.DescribeRepositoriesInput{}
				for {
					repoOutput, err := svc.DescribeRepositoriesWithContext(ctx, repoInput)
					if err != nil {
						return nil, err
					}

					for _, repo := range repoOutput.Repositories {
						if repo.RepositoryName == nil {
							continue
						}

						imgInput := &ecr.ListImagesInput{
							RepositoryName: repo.RepositoryName,
						}
						for {
							imagesOutput, err := svc.ListImagesWithContext(ctx, imgInput)
							if err != nil {
								break
							}

							for _, img := range imagesOutput.ImageIds {
								image := ecrImage{
									RepoName: *repo.RepositoryName,
								}
								if img.ImageTag != nil {
									image.ImageTag = *img.ImageTag
								}
								if img.ImageDigest != nil {
									image.ImageDigest = *img.ImageDigest
								}
								allImages = append(allImages, image)
							}

							if imagesOutput.NextToken == nil {
								break
							}
							imgInput.NextToken = imagesOutput.NextToken
						}
					}

					if repoOutput.NextToken == nil {
						break
					}
					repoInput.NextToken = repoOutput.NextToken
				}
			}
			return allImages, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "ecr:ListImages", err)
				return []types.ScanResult{
					{
						ServiceName: "ECR",
						MethodName:  "ecr:ListImages",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			images, ok := output.([]ecrImage)
			if !ok {
				utils.HandleAWSError(debug, "ecr:ListImages", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, img := range images {
				resourceName := img.ImageTag
				if resourceName == "" {
					resourceName = img.ImageDigest
				}

				results = append(results, types.ScanResult{
					ServiceName:  "ECR",
					MethodName:   "ecr:ListImages",
					ResourceType: "image",
					ResourceName: resourceName,
					Details:      map[string]interface{}{},
					Timestamp:    time.Now(),
				})

				if img.ImageTag != "" {
					utils.PrintResult(debug, "", "ecr:ListImages",
						fmt.Sprintf("ECR Image: %s:%s (Digest: %s)", img.RepoName, img.ImageTag, img.ImageDigest), nil)
				} else {
					utils.PrintResult(debug, "", "ecr:ListImages",
						fmt.Sprintf("ECR Image: %s (Digest: %s)", img.RepoName, img.ImageDigest), nil)
				}
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
	{
		Name: "ecr:GetRepositoryPolicy",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			var allPolicies []ecrPolicy

			for _, region := range types.Regions {
				sess.Config.Region = aws.String(region)
				svc := ecr.New(sess)

				repoInput := &ecr.DescribeRepositoriesInput{}
				for {
					repoOutput, err := svc.DescribeRepositoriesWithContext(ctx, repoInput)
					if err != nil {
						return nil, err
					}

					for _, repo := range repoOutput.Repositories {
						if repo.RepositoryName == nil {
							continue
						}

						policyOutput, err := svc.GetRepositoryPolicyWithContext(ctx, &ecr.GetRepositoryPolicyInput{
							RepositoryName: repo.RepositoryName,
						})
						if err != nil {
							if awsErr, ok := err.(awserr.Error); ok {
								if awsErr.Code() == "RepositoryPolicyNotFoundException" {
									continue
								}
							}
							continue
						}

						policy := ecrPolicy{
							RepoName: *repo.RepositoryName,
						}
						if policyOutput.PolicyText != nil {
							policy.PolicyText = *policyOutput.PolicyText
						}
						allPolicies = append(allPolicies, policy)
					}

					if repoOutput.NextToken == nil {
						break
					}
					repoInput.NextToken = repoOutput.NextToken
				}
			}
			return allPolicies, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "ecr:GetRepositoryPolicy", err)
				return []types.ScanResult{
					{
						ServiceName: "ECR",
						MethodName:  "ecr:GetRepositoryPolicy",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			policies, ok := output.([]ecrPolicy)
			if !ok {
				utils.HandleAWSError(debug, "ecr:GetRepositoryPolicy", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, policy := range policies {
				results = append(results, types.ScanResult{
					ServiceName:  "ECR",
					MethodName:   "ecr:GetRepositoryPolicy",
					ResourceType: "repository-policy",
					ResourceName: policy.RepoName,
					Details:      map[string]interface{}{},
					Timestamp:    time.Now(),
				})

				utils.PrintResult(debug, "", "ecr:GetRepositoryPolicy",
					fmt.Sprintf("ECR Repository Policy: %s", utils.ColorizeItem(policy.RepoName)), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
