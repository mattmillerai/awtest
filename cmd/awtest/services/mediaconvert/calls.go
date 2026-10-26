package mediaconvert

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/mediaconvert"
	mctypes "github.com/aws/aws-sdk-go-v2/service/mediaconvert/types"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

type mcQueue struct {
	Name                 string
	Arn                  string
	Status               string
	Type                 string
	PricingPlan          string
	Description          string
	SubmittedJobsCount   string
	ProgressingJobsCount string
	CreatedAt            string
	Region               string
}

type mcJob struct {
	Id           string
	Arn          string
	Status       string
	Queue        string
	Role         string
	JobTemplate  string
	CreatedAt    string
	CurrentPhase string
	ErrorMessage string
	Region       string
}

type mcPreset struct {
	Name        string
	Arn         string
	Description string
	Type        string
	Category    string
	CreatedAt   string
	Region      string
}

func extractQueue(queue mctypes.Queue, region string) mcQueue {
	submittedJobs := ""
	if queue.SubmittedJobsCount != nil {
		submittedJobs = fmt.Sprintf("%d", *queue.SubmittedJobsCount)
	}
	progressingJobs := ""
	if queue.ProgressingJobsCount != nil {
		progressingJobs = fmt.Sprintf("%d", *queue.ProgressingJobsCount)
	}
	createdAt := ""
	if queue.CreatedAt != nil {
		createdAt = queue.CreatedAt.Format(time.RFC3339)
	}
	return mcQueue{
		Name:                 aws.ToString(queue.Name),
		Arn:                  aws.ToString(queue.Arn),
		Status:               string(queue.Status),
		Type:                 string(queue.Type),
		PricingPlan:          string(queue.PricingPlan),
		Description:          aws.ToString(queue.Description),
		SubmittedJobsCount:   submittedJobs,
		ProgressingJobsCount: progressingJobs,
		CreatedAt:            createdAt,
		Region:               region,
	}
}

func extractJob(job mctypes.Job, region string) mcJob {
	createdAt := ""
	if job.CreatedAt != nil {
		createdAt = job.CreatedAt.Format(time.RFC3339)
	}
	return mcJob{
		Id:           aws.ToString(job.Id),
		Arn:          aws.ToString(job.Arn),
		Status:       string(job.Status),
		Queue:        aws.ToString(job.Queue),
		Role:         aws.ToString(job.Role),
		JobTemplate:  aws.ToString(job.JobTemplate),
		CreatedAt:    createdAt,
		CurrentPhase: string(job.CurrentPhase),
		ErrorMessage: aws.ToString(job.ErrorMessage),
		Region:       region,
	}
}

func extractPreset(preset mctypes.Preset, region string) mcPreset {
	createdAt := ""
	if preset.CreatedAt != nil {
		createdAt = preset.CreatedAt.Format(time.RFC3339)
	}
	return mcPreset{
		Name:        aws.ToString(preset.Name),
		Arn:         aws.ToString(preset.Arn),
		Description: aws.ToString(preset.Description),
		Type:        string(preset.Type),
		Category:    aws.ToString(preset.Category),
		CreatedAt:   createdAt,
		Region:      region,
	}
}

var MediaConvertCalls = []types.AWSService{
	{
		Name: "mediaconvert:ListQueues",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			var allQueues []mcQueue
			var lastErr error

			for _, region := range types.RegionsFor(ctx) {
				svc := awsv2.NewRegionalClient(cfg, region, mediaconvert.NewFromConfig)
				var nextToken *string
				for {
					input := &mediaconvert.ListQueuesInput{
						MaxResults: aws.Int32(20),
					}
					if nextToken != nil {
						input.NextToken = nextToken
					}
					output, err := svc.ListQueues(ctx, input)
					if err != nil {
						lastErr = err
						utils.HandleAWSError(false, "mediaconvert:ListQueues", err)
						break
					}
					for _, queue := range output.Queues {
						allQueues = append(allQueues, extractQueue(queue, region))
					}
					if output.NextToken == nil {
						break
					}
					nextToken = output.NextToken
				}
			}

			if len(allQueues) == 0 && lastErr != nil {
				return nil, lastErr
			}
			return allQueues, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "mediaconvert:ListQueues", err)
				return []types.ScanResult{
					{
						ServiceName: "MediaConvert",
						MethodName:  "mediaconvert:ListQueues",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			queues, ok := output.([]mcQueue)
			if !ok {
				utils.HandleAWSError(debug, "mediaconvert:ListQueues", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, q := range queues {
				results = append(results, types.ScanResult{
					ServiceName:  "MediaConvert",
					MethodName:   "mediaconvert:ListQueues",
					ResourceType: "queue",
					ResourceName: q.Name,
					Details: map[string]interface{}{
						"Name":                 q.Name,
						"Arn":                  q.Arn,
						"Status":               q.Status,
						"Type":                 q.Type,
						"PricingPlan":          q.PricingPlan,
						"Description":          q.Description,
						"SubmittedJobsCount":   q.SubmittedJobsCount,
						"ProgressingJobsCount": q.ProgressingJobsCount,
						"CreatedAt":            q.CreatedAt,
						"Region":               q.Region,
					},
					Timestamp: time.Now(),
				})

				utils.PrintResult(debug, "", "mediaconvert:ListQueues",
					fmt.Sprintf("MediaConvert Queue: %s (Status: %s, Type: %s, Region: %s)", utils.ColorizeItem(q.Name), q.Status, q.Type, q.Region), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
	{
		Name: "mediaconvert:ListJobs",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			var allJobs []mcJob
			var lastErr error

			for _, region := range types.RegionsFor(ctx) {
				svc := awsv2.NewRegionalClient(cfg, region, mediaconvert.NewFromConfig)
				var nextToken *string
				for {
					input := &mediaconvert.ListJobsInput{
						MaxResults: aws.Int32(20),
					}
					if nextToken != nil {
						input.NextToken = nextToken
					}
					output, err := svc.ListJobs(ctx, input)
					if err != nil {
						lastErr = err
						utils.HandleAWSError(false, "mediaconvert:ListJobs", err)
						break
					}
					for _, job := range output.Jobs {
						allJobs = append(allJobs, extractJob(job, region))
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
				utils.HandleAWSError(debug, "mediaconvert:ListJobs", err)
				return []types.ScanResult{
					{
						ServiceName: "MediaConvert",
						MethodName:  "mediaconvert:ListJobs",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			jobs, ok := output.([]mcJob)
			if !ok {
				utils.HandleAWSError(debug, "mediaconvert:ListJobs", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, j := range jobs {
				results = append(results, types.ScanResult{
					ServiceName:  "MediaConvert",
					MethodName:   "mediaconvert:ListJobs",
					ResourceType: "job",
					ResourceName: j.Id,
					Details: map[string]interface{}{
						"Id":           j.Id,
						"Arn":          j.Arn,
						"Status":       j.Status,
						"Queue":        j.Queue,
						"Role":         j.Role,
						"JobTemplate":  j.JobTemplate,
						"CreatedAt":    j.CreatedAt,
						"CurrentPhase": j.CurrentPhase,
						"ErrorMessage": j.ErrorMessage,
						"Region":       j.Region,
					},
					Timestamp: time.Now(),
				})

				utils.PrintResult(debug, "", "mediaconvert:ListJobs",
					fmt.Sprintf("MediaConvert Job: %s (Status: %s, Queue: %s, Region: %s)", utils.ColorizeItem(j.Id), j.Status, j.Queue, j.Region), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
	{
		Name: "mediaconvert:ListPresets",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			var allPresets []mcPreset
			var lastErr error

			for _, region := range types.RegionsFor(ctx) {
				svc := awsv2.NewRegionalClient(cfg, region, mediaconvert.NewFromConfig)
				var nextToken *string
				for {
					input := &mediaconvert.ListPresetsInput{
						MaxResults: aws.Int32(20),
					}
					if nextToken != nil {
						input.NextToken = nextToken
					}
					output, err := svc.ListPresets(ctx, input)
					if err != nil {
						lastErr = err
						utils.HandleAWSError(false, "mediaconvert:ListPresets", err)
						break
					}
					for _, preset := range output.Presets {
						allPresets = append(allPresets, extractPreset(preset, region))
					}
					if output.NextToken == nil {
						break
					}
					nextToken = output.NextToken
				}
			}

			if len(allPresets) == 0 && lastErr != nil {
				return nil, lastErr
			}
			return allPresets, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "mediaconvert:ListPresets", err)
				return []types.ScanResult{
					{
						ServiceName: "MediaConvert",
						MethodName:  "mediaconvert:ListPresets",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			presets, ok := output.([]mcPreset)
			if !ok {
				utils.HandleAWSError(debug, "mediaconvert:ListPresets", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, p := range presets {
				results = append(results, types.ScanResult{
					ServiceName:  "MediaConvert",
					MethodName:   "mediaconvert:ListPresets",
					ResourceType: "preset",
					ResourceName: p.Name,
					Details: map[string]interface{}{
						"Name":        p.Name,
						"Arn":         p.Arn,
						"Description": p.Description,
						"Type":        p.Type,
						"Category":    p.Category,
						"CreatedAt":   p.CreatedAt,
						"Region":      p.Region,
					},
					Timestamp: time.Now(),
				})

				utils.PrintResult(debug, "", "mediaconvert:ListPresets",
					fmt.Sprintf("MediaConvert Preset: %s (Type: %s, Category: %s, Region: %s)", utils.ColorizeItem(p.Name), p.Type, p.Category, p.Region), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
