package sagemaker

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

type smNotebook struct {
	Name                  string
	Arn                   string
	Status                string
	InstanceType          string
	URL                   string
	DefaultCodeRepository string
	LastModifiedTime      string
	Region                string
}

type smEndpoint struct {
	Name             string
	Arn              string
	Status           string
	CreationTime     string
	LastModifiedTime string
	Region           string
}

type smModel struct {
	Name         string
	Arn          string
	CreationTime string
	Region       string
}

type smTrainingJob struct {
	Name             string
	Arn              string
	Status           string
	CreationTime     string
	LastModifiedTime string
	Region           string
}

var SageMakerCalls = []types.AWSService{
	{
		Name: "sagemaker:ListNotebookInstances",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			var allNotebooks []smNotebook
			var lastErr error

			for _, region := range types.RegionsFor(ctx) {
				svc := awsv2.NewRegionalClient(cfg, region, sagemaker.NewFromConfig)
				var nextToken *string
				for {
					input := &sagemaker.ListNotebookInstancesInput{
						MaxResults: aws.Int32(100),
					}
					if nextToken != nil {
						input.NextToken = nextToken
					}
					output, err := svc.ListNotebookInstances(ctx, input)
					if err != nil {
						lastErr = err
						utils.HandleAWSError(false, "sagemaker:ListNotebookInstances", err)
						break
					}

					for _, nb := range output.NotebookInstances {
						lastModified := ""
						if nb.LastModifiedTime != nil {
							lastModified = nb.LastModifiedTime.Format(time.RFC3339)
						}

						allNotebooks = append(allNotebooks, smNotebook{
							Name:                  aws.ToString(nb.NotebookInstanceName),
							Arn:                   aws.ToString(nb.NotebookInstanceArn),
							Status:                string(nb.NotebookInstanceStatus),
							InstanceType:          string(nb.InstanceType),
							URL:                   aws.ToString(nb.Url),
							DefaultCodeRepository: aws.ToString(nb.DefaultCodeRepository),
							LastModifiedTime:      lastModified,
							Region:                region,
						})
					}

					if output.NextToken == nil {
						break
					}
					nextToken = output.NextToken
				}
			}

			if len(allNotebooks) == 0 && lastErr != nil {
				return nil, lastErr
			}
			return allNotebooks, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "sagemaker:ListNotebookInstances", err)
				return []types.ScanResult{
					{
						ServiceName: "SageMaker",
						MethodName:  "sagemaker:ListNotebookInstances",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			notebooks, ok := output.([]smNotebook)
			if !ok {
				utils.HandleAWSError(debug, "sagemaker:ListNotebookInstances", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, nb := range notebooks {
				results = append(results, types.ScanResult{
					ServiceName:  "SageMaker",
					MethodName:   "sagemaker:ListNotebookInstances",
					ResourceType: "notebook-instance",
					ResourceName: nb.Name,
					Details: map[string]interface{}{
						"Name":                  nb.Name,
						"Arn":                   nb.Arn,
						"Status":                nb.Status,
						"InstanceType":          nb.InstanceType,
						"URL":                   nb.URL,
						"DefaultCodeRepository": nb.DefaultCodeRepository,
						"LastModifiedTime":      nb.LastModifiedTime,
						"Region":                nb.Region,
					},
					Timestamp: time.Now(),
				})

				utils.PrintResult(debug, "", "sagemaker:ListNotebookInstances",
					fmt.Sprintf("SageMaker Notebook: %s (Status: %s, Type: %s, Region: %s)", utils.ColorizeItem(nb.Name), nb.Status, nb.InstanceType, nb.Region), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
	{
		Name: "sagemaker:ListEndpoints",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			var allEndpoints []smEndpoint
			var lastErr error

			for _, region := range types.RegionsFor(ctx) {
				svc := awsv2.NewRegionalClient(cfg, region, sagemaker.NewFromConfig)
				var nextToken *string
				for {
					input := &sagemaker.ListEndpointsInput{
						MaxResults: aws.Int32(100),
					}
					if nextToken != nil {
						input.NextToken = nextToken
					}
					output, err := svc.ListEndpoints(ctx, input)
					if err != nil {
						lastErr = err
						utils.HandleAWSError(false, "sagemaker:ListEndpoints", err)
						break
					}

					for _, ep := range output.Endpoints {
						creationTime := ""
						if ep.CreationTime != nil {
							creationTime = ep.CreationTime.Format(time.RFC3339)
						}

						lastModified := ""
						if ep.LastModifiedTime != nil {
							lastModified = ep.LastModifiedTime.Format(time.RFC3339)
						}

						allEndpoints = append(allEndpoints, smEndpoint{
							Name:             aws.ToString(ep.EndpointName),
							Arn:              aws.ToString(ep.EndpointArn),
							Status:           string(ep.EndpointStatus),
							CreationTime:     creationTime,
							LastModifiedTime: lastModified,
							Region:           region,
						})
					}

					if output.NextToken == nil {
						break
					}
					nextToken = output.NextToken
				}
			}

			if len(allEndpoints) == 0 && lastErr != nil {
				return nil, lastErr
			}
			return allEndpoints, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "sagemaker:ListEndpoints", err)
				return []types.ScanResult{
					{
						ServiceName: "SageMaker",
						MethodName:  "sagemaker:ListEndpoints",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			endpoints, ok := output.([]smEndpoint)
			if !ok {
				utils.HandleAWSError(debug, "sagemaker:ListEndpoints", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, ep := range endpoints {
				results = append(results, types.ScanResult{
					ServiceName:  "SageMaker",
					MethodName:   "sagemaker:ListEndpoints",
					ResourceType: "endpoint",
					ResourceName: ep.Name,
					Details: map[string]interface{}{
						"Name":             ep.Name,
						"Arn":              ep.Arn,
						"Status":           ep.Status,
						"CreationTime":     ep.CreationTime,
						"LastModifiedTime": ep.LastModifiedTime,
						"Region":           ep.Region,
					},
					Timestamp: time.Now(),
				})

				utils.PrintResult(debug, "", "sagemaker:ListEndpoints",
					fmt.Sprintf("SageMaker Endpoint: %s (Status: %s, Region: %s)", utils.ColorizeItem(ep.Name), ep.Status, ep.Region), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
	{
		Name: "sagemaker:ListModels",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			var allModels []smModel
			var lastErr error

			for _, region := range types.RegionsFor(ctx) {
				svc := awsv2.NewRegionalClient(cfg, region, sagemaker.NewFromConfig)
				var nextToken *string
				for {
					input := &sagemaker.ListModelsInput{
						MaxResults: aws.Int32(100),
					}
					if nextToken != nil {
						input.NextToken = nextToken
					}
					output, err := svc.ListModels(ctx, input)
					if err != nil {
						lastErr = err
						utils.HandleAWSError(false, "sagemaker:ListModels", err)
						break
					}

					for _, m := range output.Models {
						creationTime := ""
						if m.CreationTime != nil {
							creationTime = m.CreationTime.Format(time.RFC3339)
						}

						allModels = append(allModels, smModel{
							Name:         aws.ToString(m.ModelName),
							Arn:          aws.ToString(m.ModelArn),
							CreationTime: creationTime,
							Region:       region,
						})
					}

					if output.NextToken == nil {
						break
					}
					nextToken = output.NextToken
				}
			}

			if len(allModels) == 0 && lastErr != nil {
				return nil, lastErr
			}
			return allModels, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "sagemaker:ListModels", err)
				return []types.ScanResult{
					{
						ServiceName: "SageMaker",
						MethodName:  "sagemaker:ListModels",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			models, ok := output.([]smModel)
			if !ok {
				utils.HandleAWSError(debug, "sagemaker:ListModels", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, m := range models {
				results = append(results, types.ScanResult{
					ServiceName:  "SageMaker",
					MethodName:   "sagemaker:ListModels",
					ResourceType: "model",
					ResourceName: m.Name,
					Details: map[string]interface{}{
						"Name":         m.Name,
						"Arn":          m.Arn,
						"CreationTime": m.CreationTime,
						"Region":       m.Region,
					},
					Timestamp: time.Now(),
				})

				utils.PrintResult(debug, "", "sagemaker:ListModels",
					fmt.Sprintf("SageMaker Model: %s (Created: %s, Region: %s)", utils.ColorizeItem(m.Name), m.CreationTime, m.Region), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
	{
		Name: "sagemaker:ListTrainingJobs",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			var allJobs []smTrainingJob
			var lastErr error

			for _, region := range types.RegionsFor(ctx) {
				svc := awsv2.NewRegionalClient(cfg, region, sagemaker.NewFromConfig)
				var nextToken *string
				for {
					input := &sagemaker.ListTrainingJobsInput{
						MaxResults: aws.Int32(100),
					}
					if nextToken != nil {
						input.NextToken = nextToken
					}
					output, err := svc.ListTrainingJobs(ctx, input)
					if err != nil {
						lastErr = err
						utils.HandleAWSError(false, "sagemaker:ListTrainingJobs", err)
						break
					}

					for _, job := range output.TrainingJobSummaries {
						creationTime := ""
						if job.CreationTime != nil {
							creationTime = job.CreationTime.Format(time.RFC3339)
						}

						lastModified := ""
						if job.LastModifiedTime != nil {
							lastModified = job.LastModifiedTime.Format(time.RFC3339)
						}

						allJobs = append(allJobs, smTrainingJob{
							Name:             aws.ToString(job.TrainingJobName),
							Arn:              aws.ToString(job.TrainingJobArn),
							Status:           string(job.TrainingJobStatus),
							CreationTime:     creationTime,
							LastModifiedTime: lastModified,
							Region:           region,
						})
					}

					if output.NextToken == nil {
						break
					}
					nextToken = output.NextToken
				}
			}

			if len(allJobs) == 0 && lastErr != nil {
				return nil, lastErr
			}
			return allJobs, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "sagemaker:ListTrainingJobs", err)
				return []types.ScanResult{
					{
						ServiceName: "SageMaker",
						MethodName:  "sagemaker:ListTrainingJobs",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			jobs, ok := output.([]smTrainingJob)
			if !ok {
				utils.HandleAWSError(debug, "sagemaker:ListTrainingJobs", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, job := range jobs {
				results = append(results, types.ScanResult{
					ServiceName:  "SageMaker",
					MethodName:   "sagemaker:ListTrainingJobs",
					ResourceType: "training-job",
					ResourceName: job.Name,
					Details: map[string]interface{}{
						"Name":             job.Name,
						"Arn":              job.Arn,
						"Status":           job.Status,
						"CreationTime":     job.CreationTime,
						"LastModifiedTime": job.LastModifiedTime,
						"Region":           job.Region,
					},
					Timestamp: time.Now(),
				})

				utils.PrintResult(debug, "", "sagemaker:ListTrainingJobs",
					fmt.Sprintf("SageMaker Training Job: %s (Status: %s, Region: %s)", utils.ColorizeItem(job.Name), job.Status, job.Region), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
