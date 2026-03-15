package codebuild

import (
	"context"
	"fmt"
	"time"

	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/MillerMedia/awtest/cmd/awtest/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/codebuild"
)

type cbProject struct {
	Name            string
	Arn             string
	Description     string
	SourceType      string
	EnvironmentType string
	Region          string
}

type cbEnvVar struct {
	ProjectName  string
	VariableName string
	VariableType string
	Region       string
}

type cbBuild struct {
	BuildId     string
	ProjectName string
	BuildStatus string
	StartTime   string
	Region      string
}

var CodeBuildCalls = []types.AWSService{
	{
		Name: "codebuild:ListProjects",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			var allProjects []cbProject
			var lastErr error

			for _, region := range types.Regions {
				svc := codebuild.New(sess, &aws.Config{Region: aws.String(region)})

				var projectNames []*string
				input := &codebuild.ListProjectsInput{}
				for {
					output, err := svc.ListProjectsWithContext(ctx, input)
					if err != nil {
						lastErr = err
						utils.HandleAWSError(false, "codebuild:ListProjects", err)
						break
					}

					projectNames = append(projectNames, output.Projects...)

					if output.NextToken == nil {
						break
					}
					input.NextToken = output.NextToken
				}

				// Batch get project details (max 100 per call)
				for i := 0; i < len(projectNames); i += 100 {
					end := i + 100
					if end > len(projectNames) {
						end = len(projectNames)
					}
					batch := projectNames[i:end]

					batchInput := &codebuild.BatchGetProjectsInput{
						Names: batch,
					}
					batchOutput, err := svc.BatchGetProjectsWithContext(ctx, batchInput)
					if err != nil {
						lastErr = err
						utils.HandleAWSError(false, "codebuild:ListProjects", err)
						break
					}

					for _, project := range batchOutput.Projects {
						name := ""
						if project.Name != nil {
							name = *project.Name
						}

						arn := ""
						if project.Arn != nil {
							arn = *project.Arn
						}

						description := ""
						if project.Description != nil {
							description = *project.Description
						}

						sourceType := ""
						if project.Source != nil && project.Source.Type != nil {
							sourceType = *project.Source.Type
						}

						environmentType := ""
						if project.Environment != nil && project.Environment.Type != nil {
							environmentType = *project.Environment.Type
						}

						allProjects = append(allProjects, cbProject{
							Name:            name,
							Arn:             arn,
							Description:     description,
							SourceType:      sourceType,
							EnvironmentType: environmentType,
							Region:          region,
						})
					}
				}
			}

			if len(allProjects) == 0 && lastErr != nil {
				return nil, lastErr
			}
			return allProjects, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "codebuild:ListProjects", err)
				return []types.ScanResult{
					{
						ServiceName: "CodeBuild",
						MethodName:  "codebuild:ListProjects",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			projects, ok := output.([]cbProject)
			if !ok {
				utils.HandleAWSError(debug, "codebuild:ListProjects", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, project := range projects {
				results = append(results, types.ScanResult{
					ServiceName:  "CodeBuild",
					MethodName:   "codebuild:ListProjects",
					ResourceType: "project",
					ResourceName: project.Name,
					Details: map[string]interface{}{
						"Name":            project.Name,
						"Arn":             project.Arn,
						"Description":     project.Description,
						"SourceType":      project.SourceType,
						"EnvironmentType": project.EnvironmentType,
						"Region":          project.Region,
					},
					Timestamp: time.Now(),
				})

				utils.PrintResult(debug, "", "codebuild:ListProjects",
					fmt.Sprintf("CodeBuild Project: %s (Source: %s, Env: %s, Region: %s)", utils.ColorizeItem(project.Name), project.SourceType, project.EnvironmentType, project.Region), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
	{
		Name: "codebuild:ListProjectEnvironmentVariables",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			var allEnvVars []cbEnvVar
			var lastErr error

			for _, region := range types.Regions {
				svc := codebuild.New(sess, &aws.Config{Region: aws.String(region)})

				var projectNames []*string
				input := &codebuild.ListProjectsInput{}
				for {
					output, err := svc.ListProjectsWithContext(ctx, input)
					if err != nil {
						lastErr = err
						utils.HandleAWSError(false, "codebuild:ListProjectEnvironmentVariables", err)
						break
					}

					projectNames = append(projectNames, output.Projects...)

					if output.NextToken == nil {
						break
					}
					input.NextToken = output.NextToken
				}

				// Batch get project details (max 100 per call)
				for i := 0; i < len(projectNames); i += 100 {
					end := i + 100
					if end > len(projectNames) {
						end = len(projectNames)
					}
					batch := projectNames[i:end]

					batchInput := &codebuild.BatchGetProjectsInput{
						Names: batch,
					}
					batchOutput, err := svc.BatchGetProjectsWithContext(ctx, batchInput)
					if err != nil {
						lastErr = err
						utils.HandleAWSError(false, "codebuild:ListProjectEnvironmentVariables", err)
						break
					}

					for _, project := range batchOutput.Projects {
						projectName := ""
						if project.Name != nil {
							projectName = *project.Name
						}

						if project.Environment != nil {
							for _, envVar := range project.Environment.EnvironmentVariables {
								varName := ""
								if envVar.Name != nil {
									varName = *envVar.Name
								}

								varType := ""
								if envVar.Type != nil {
									varType = *envVar.Type
								}

								allEnvVars = append(allEnvVars, cbEnvVar{
									ProjectName:  projectName,
									VariableName: varName,
									VariableType: varType,
									Region:       region,
								})
							}
						}
					}
				}
			}

			if len(allEnvVars) == 0 && lastErr != nil {
				return nil, lastErr
			}
			return allEnvVars, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "codebuild:ListProjectEnvironmentVariables", err)
				return []types.ScanResult{
					{
						ServiceName: "CodeBuild",
						MethodName:  "codebuild:ListProjectEnvironmentVariables",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			envVars, ok := output.([]cbEnvVar)
			if !ok {
				utils.HandleAWSError(debug, "codebuild:ListProjectEnvironmentVariables", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, envVar := range envVars {
				results = append(results, types.ScanResult{
					ServiceName:  "CodeBuild",
					MethodName:   "codebuild:ListProjectEnvironmentVariables",
					ResourceType: "environment-variable",
					ResourceName: envVar.ProjectName + "/" + envVar.VariableName,
					Details: map[string]interface{}{
						"ProjectName":  envVar.ProjectName,
						"VariableName": envVar.VariableName,
						"VariableType": envVar.VariableType,
						"Region":       envVar.Region,
					},
					Timestamp: time.Now(),
				})

				utils.PrintResult(debug, "", "codebuild:ListProjectEnvironmentVariables",
					fmt.Sprintf("CodeBuild Env Var: %s/%s (Type: %s)", utils.ColorizeItem(envVar.ProjectName), envVar.VariableName, envVar.VariableType), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
	{
		Name: "codebuild:ListBuilds",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			var allBuilds []cbBuild
			var lastErr error

			for _, region := range types.Regions {
				svc := codebuild.New(sess, &aws.Config{Region: aws.String(region)})

				// First page only — no pagination, cap at 100 build IDs per region
				listBuildsOutput, err := svc.ListBuildsWithContext(ctx, &codebuild.ListBuildsInput{})
				if err != nil {
					lastErr = err
					utils.HandleAWSError(false, "codebuild:ListBuilds", err)
					continue
				}

				if len(listBuildsOutput.Ids) == 0 {
					continue
				}

				// Batch get build details (max 100 IDs per call)
				for i := 0; i < len(listBuildsOutput.Ids); i += 100 {
					end := i + 100
					if end > len(listBuildsOutput.Ids) {
						end = len(listBuildsOutput.Ids)
					}
					batch := listBuildsOutput.Ids[i:end]

					batchInput := &codebuild.BatchGetBuildsInput{
						Ids: batch,
					}
					batchOutput, err := svc.BatchGetBuildsWithContext(ctx, batchInput)
					if err != nil {
						lastErr = err
						utils.HandleAWSError(false, "codebuild:ListBuilds", err)
						break
					}

					for _, build := range batchOutput.Builds {
						buildId := ""
						if build.Id != nil {
							buildId = *build.Id
						}

						projectName := ""
						if build.ProjectName != nil {
							projectName = *build.ProjectName
						}

						buildStatus := ""
						if build.BuildStatus != nil {
							buildStatus = *build.BuildStatus
						}

						startTime := ""
						if build.StartTime != nil {
							startTime = build.StartTime.Format(time.RFC3339)
						}

						allBuilds = append(allBuilds, cbBuild{
							BuildId:     buildId,
							ProjectName: projectName,
							BuildStatus: buildStatus,
							StartTime:   startTime,
							Region:      region,
						})
					}
				}
			}

			if len(allBuilds) == 0 && lastErr != nil {
				return nil, lastErr
			}
			return allBuilds, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "codebuild:ListBuilds", err)
				return []types.ScanResult{
					{
						ServiceName: "CodeBuild",
						MethodName:  "codebuild:ListBuilds",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			builds, ok := output.([]cbBuild)
			if !ok {
				utils.HandleAWSError(debug, "codebuild:ListBuilds", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, build := range builds {
				results = append(results, types.ScanResult{
					ServiceName:  "CodeBuild",
					MethodName:   "codebuild:ListBuilds",
					ResourceType: "build",
					ResourceName: build.BuildId,
					Details: map[string]interface{}{
						"BuildId":     build.BuildId,
						"ProjectName": build.ProjectName,
						"BuildStatus": build.BuildStatus,
						"StartTime":   build.StartTime,
						"Region":      build.Region,
					},
					Timestamp: time.Now(),
				})

				utils.PrintResult(debug, "", "codebuild:ListBuilds",
					fmt.Sprintf("CodeBuild Build: %s (Project: %s, Status: %s, Region: %s)", utils.ColorizeItem(build.BuildId), build.ProjectName, build.BuildStatus, build.Region), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
