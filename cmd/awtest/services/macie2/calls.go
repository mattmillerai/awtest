package macie2

import (
	"context"
	"fmt"
	"time"

	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/MillerMedia/awtest/cmd/awtest/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/macie2"
)

type mcClassificationJob struct {
	JobId     string
	Name      string
	JobType   string
	JobStatus string
	CreatedAt string
	Region    string
}

type mcFinding struct {
	Id        string
	Type      string
	Title     string
	Severity  string
	Category  string
	Count     string
	CreatedAt string
	Region    string
}

type mcMonitoredBucket struct {
	BucketName              string
	AccountId               string
	BucketArn               string
	ObjectCount             string
	SizeInBytes             string
	ClassifiableObjectCount string
	SensitivityScore        string
	Region                  string
}

// batchGetFindings fetches full finding details for a batch of IDs with a single retry on failure.
func batchGetFindings(ctx context.Context, svc *macie2.Macie2, ids []*string, region string) ([]mcFinding, error) {
	var results []mcFinding

	batchOutput, err := svc.GetFindingsWithContext(ctx, &macie2.GetFindingsInput{
		FindingIds: ids,
	})
	if err != nil {
		utils.HandleAWSError(false, "macie2:GetFindings", err)
		// Single retry for transient errors
		retryOutput, retryErr := svc.GetFindingsWithContext(ctx, &macie2.GetFindingsInput{
			FindingIds: ids,
		})
		if retryErr != nil {
			utils.HandleAWSError(false, "macie2:GetFindings", retryErr)
			return results, retryErr
		}
		for _, f := range retryOutput.Findings {
			results = append(results, extractFinding(f, region))
		}
		return results, nil
	}

	for _, f := range batchOutput.Findings {
		results = append(results, extractFinding(f, region))
	}

	return results, nil
}

func extractFinding(f *macie2.Finding, region string) mcFinding {
	id := ""
	if f.Id != nil {
		id = *f.Id
	}
	findingType := ""
	if f.Type != nil {
		findingType = *f.Type
	}
	title := ""
	if f.Title != nil {
		title = *f.Title
	}
	severity := ""
	if f.Severity != nil && f.Severity.Description != nil {
		severity = *f.Severity.Description
	}
	category := ""
	if f.Category != nil {
		category = *f.Category
	}
	count := ""
	if f.Count != nil {
		count = fmt.Sprintf("%d", *f.Count)
	}
	createdAt := ""
	if f.CreatedAt != nil {
		createdAt = f.CreatedAt.Format(time.RFC3339)
	}
	return mcFinding{
		Id:        id,
		Type:      findingType,
		Title:     title,
		Severity:  severity,
		Category:  category,
		Count:     count,
		CreatedAt: createdAt,
		Region:    region,
	}
}

var Macie2Calls = []types.AWSService{
	{
		Name: "macie2:ListClassificationJobs",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			var allJobs []mcClassificationJob
			var lastErr error

			for _, region := range types.Regions {
				svc := macie2.New(sess, &aws.Config{Region: aws.String(region)})
				var nextToken *string
				for {
					input := &macie2.ListClassificationJobsInput{
						MaxResults: aws.Int64(25),
					}
					if nextToken != nil {
						input.NextToken = nextToken
					}
					output, err := svc.ListClassificationJobsWithContext(ctx, input)
					if err != nil {
						lastErr = err
						utils.HandleAWSError(false, "macie2:ListClassificationJobs", err)
						break
					}

					for _, job := range output.Items {
						jobId := ""
						if job.JobId != nil {
							jobId = *job.JobId
						}
						name := ""
						if job.Name != nil {
							name = *job.Name
						}
						jobType := ""
						if job.JobType != nil {
							jobType = *job.JobType
						}
						jobStatus := ""
						if job.JobStatus != nil {
							jobStatus = *job.JobStatus
						}
						createdAt := ""
						if job.CreatedAt != nil {
							createdAt = job.CreatedAt.Format(time.RFC3339)
						}

						allJobs = append(allJobs, mcClassificationJob{
							JobId:     jobId,
							Name:      name,
							JobType:   jobType,
							JobStatus: jobStatus,
							CreatedAt: createdAt,
							Region:    region,
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
				utils.HandleAWSError(debug, "macie2:ListClassificationJobs", err)
				return []types.ScanResult{
					{
						ServiceName: "Macie",
						MethodName:  "macie2:ListClassificationJobs",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			jobs, ok := output.([]mcClassificationJob)
			if !ok {
				utils.HandleAWSError(debug, "macie2:ListClassificationJobs", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, job := range jobs {
				jobName := job.Name
				if jobName == "" {
					jobName = job.JobId
				}

				results = append(results, types.ScanResult{
					ServiceName:  "Macie",
					MethodName:   "macie2:ListClassificationJobs",
					ResourceType: "classification-job",
					ResourceName: jobName,
					Details: map[string]interface{}{
						"JobId":     job.JobId,
						"Name":      job.Name,
						"JobType":   job.JobType,
						"JobStatus": job.JobStatus,
						"CreatedAt": job.CreatedAt,
						"Region":    job.Region,
					},
					Timestamp: time.Now(),
				})

				utils.PrintResult(debug, "", "macie2:ListClassificationJobs",
					fmt.Sprintf("Macie Classification Job: %s (Type: %s, Status: %s, Region: %s)", utils.ColorizeItem(jobName), job.JobType, job.JobStatus, job.Region), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
	{
		Name: "macie2:ListFindings",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			var allFindings []mcFinding
			var lastErr error

			for _, region := range types.Regions {
				svc := macie2.New(sess, &aws.Config{Region: aws.String(region)})
				var nextToken *string
				for {
					input := &macie2.ListFindingsInput{
						MaxResults: aws.Int64(50),
					}
					if nextToken != nil {
						input.NextToken = nextToken
					}
					output, err := svc.ListFindingsWithContext(ctx, input)
					if err != nil {
						lastErr = err
						utils.HandleAWSError(false, "macie2:ListFindings", err)
						break
					}

					// Batch-get this page's IDs immediately (max 50 per page = max 50 per batch)
					if len(output.FindingIds) > 0 {
						findings, batchErr := batchGetFindings(ctx, svc, output.FindingIds, region)
						if batchErr != nil {
							lastErr = batchErr
							break
						}
						allFindings = append(allFindings, findings...)
					}

					if output.NextToken == nil {
						break
					}
					nextToken = output.NextToken
				}
			}

			if len(allFindings) == 0 && lastErr != nil {
				return nil, lastErr
			}
			return allFindings, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "macie2:ListFindings", err)
				return []types.ScanResult{
					{
						ServiceName: "Macie",
						MethodName:  "macie2:ListFindings",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			findings, ok := output.([]mcFinding)
			if !ok {
				utils.HandleAWSError(debug, "macie2:ListFindings", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, f := range findings {
				results = append(results, types.ScanResult{
					ServiceName:  "Macie",
					MethodName:   "macie2:ListFindings",
					ResourceType: "finding",
					ResourceName: f.Id,
					Details: map[string]interface{}{
						"Id":        f.Id,
						"Type":      f.Type,
						"Title":     f.Title,
						"Severity":  f.Severity,
						"Category":  f.Category,
						"Count":     f.Count,
						"CreatedAt": f.CreatedAt,
						"Region":    f.Region,
					},
					Timestamp: time.Now(),
				})

				utils.PrintResult(debug, "", "macie2:ListFindings",
					fmt.Sprintf("Macie Finding: %s (Type: %s, Severity: %s, Category: %s, Region: %s)", utils.ColorizeItem(f.Id), f.Type, f.Severity, f.Category, f.Region), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
	{
		Name: "macie2:DescribeBuckets",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			var allBuckets []mcMonitoredBucket
			var lastErr error

			for _, region := range types.Regions {
				svc := macie2.New(sess, &aws.Config{Region: aws.String(region)})
				var nextToken *string
				for {
					input := &macie2.DescribeBucketsInput{
						MaxResults: aws.Int64(50),
					}
					if nextToken != nil {
						input.NextToken = nextToken
					}
					output, err := svc.DescribeBucketsWithContext(ctx, input)
					if err != nil {
						lastErr = err
						utils.HandleAWSError(false, "macie2:DescribeBuckets", err)
						break
					}

					for _, b := range output.Buckets {
						bucketName := ""
						if b.BucketName != nil {
							bucketName = *b.BucketName
						}
						accountId := ""
						if b.AccountId != nil {
							accountId = *b.AccountId
						}
						bucketArn := ""
						if b.BucketArn != nil {
							bucketArn = *b.BucketArn
						}
						objectCount := ""
						if b.ObjectCount != nil {
							objectCount = fmt.Sprintf("%d", *b.ObjectCount)
						}
						sizeInBytes := ""
						if b.SizeInBytes != nil {
							sizeInBytes = fmt.Sprintf("%d", *b.SizeInBytes)
						}
						classifiableObjectCount := ""
						if b.ClassifiableObjectCount != nil {
							classifiableObjectCount = fmt.Sprintf("%d", *b.ClassifiableObjectCount)
						}
						sensitivityScore := ""
						if b.SensitivityScore != nil {
							sensitivityScore = fmt.Sprintf("%d", *b.SensitivityScore)
						}

						allBuckets = append(allBuckets, mcMonitoredBucket{
							BucketName:              bucketName,
							AccountId:               accountId,
							BucketArn:               bucketArn,
							ObjectCount:             objectCount,
							SizeInBytes:             sizeInBytes,
							ClassifiableObjectCount: classifiableObjectCount,
							SensitivityScore:        sensitivityScore,
							Region:                  region,
						})
					}

					if output.NextToken == nil {
						break
					}
					nextToken = output.NextToken
				}
			}

			if len(allBuckets) == 0 && lastErr != nil {
				return nil, lastErr
			}
			return allBuckets, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "macie2:DescribeBuckets", err)
				return []types.ScanResult{
					{
						ServiceName: "Macie",
						MethodName:  "macie2:DescribeBuckets",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			buckets, ok := output.([]mcMonitoredBucket)
			if !ok {
				utils.HandleAWSError(debug, "macie2:DescribeBuckets", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, b := range buckets {
				results = append(results, types.ScanResult{
					ServiceName:  "Macie",
					MethodName:   "macie2:DescribeBuckets",
					ResourceType: "monitored-bucket",
					ResourceName: b.BucketName,
					Details: map[string]interface{}{
						"BucketName":              b.BucketName,
						"AccountId":               b.AccountId,
						"BucketArn":               b.BucketArn,
						"ObjectCount":             b.ObjectCount,
						"SizeInBytes":             b.SizeInBytes,
						"ClassifiableObjectCount": b.ClassifiableObjectCount,
						"SensitivityScore":        b.SensitivityScore,
						"Region":                  b.Region,
					},
					Timestamp: time.Now(),
				})

				utils.PrintResult(debug, "", "macie2:DescribeBuckets",
					fmt.Sprintf("Macie Monitored Bucket: %s (Objects: %s, Classifiable: %s, Sensitivity: %s, Region: %s)", utils.ColorizeItem(b.BucketName), b.ObjectCount, b.ClassifiableObjectCount, b.SensitivityScore, b.Region), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
