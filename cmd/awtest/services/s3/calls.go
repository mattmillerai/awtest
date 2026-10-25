package s3

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

var S3Calls = []types.AWSService{
	{
		Name: "s3:ListBuckets",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			svc := awsv2.NewRegionalClient(cfg, "", s3.NewFromConfig)
			output, err := svc.ListBuckets(ctx, &s3.ListBucketsInput{})
			return map[string]interface{}{
				"output": output,
				"cfg":    cfg,
				"ctx":    ctx,
			}, err
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				err = awsv2.Wrap(err)
				utils.HandleAWSError(debug, "s3:ListBuckets", err)
				return []types.ScanResult{
					{
						ServiceName: "S3",
						MethodName:  "s3:ListBuckets",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			if outputMap, ok := output.(map[string]interface{}); ok {
				s3Output, _ := outputMap["output"].(*s3.ListBucketsOutput)
				cfg, _ := outputMap["cfg"].(aws.Config)
				ctx, _ := outputMap["ctx"].(context.Context)
				if ctx == nil {
					ctx = context.Background()
				}
				if s3Output == nil {
					return results
				}
				defaultSvc := awsv2.NewRegionalClient(cfg, "", s3.NewFromConfig)
				for _, bucket := range s3Output.Buckets {
					bucketName := aws.ToString(bucket.Name)

					// Add bucket result
					results = append(results, types.ScanResult{
						ServiceName:  "S3",
						MethodName:   "s3:ListBuckets",
						ResourceType: "bucket",
						ResourceName: bucketName,
						Details:      map[string]interface{}{},
						Timestamp:    time.Now(),
					})

					// Keep backward compatibility - print result
					utils.PrintResult(debug, "", "s3:ListBuckets", fmt.Sprintf("S3 bucket: %s", utils.ColorizeItem(bucketName)), nil)

					// Get the region of the bucket
					locationOutput, err := defaultSvc.GetBucketLocation(ctx, &s3.GetBucketLocationInput{
						Bucket: aws.String(bucketName),
					})

					if err == nil {
						// This is the correct region. Perform ListObjects here.
						region := string(locationOutput.LocationConstraint)
						if region == "" {
							region = "us-east-1" // default to us-east-1 if the bucket region is not specified
						}

						svc := awsv2.NewRegionalClient(cfg, region, s3.NewFromConfig)

						// Counter for the objects
						objCount := 0
						paginator := s3.NewListObjectsV2Paginator(svc, &s3.ListObjectsV2Input{Bucket: bucket.Name})
						for paginator.HasMorePages() {
							var page *s3.ListObjectsV2Output
							page, err = paginator.NextPage(ctx)
							if err != nil {
								break
							}
							objCount += len(page.Contents)
							// Continue fetching pages only if we have less than 10000 objects (HasMorePages is false on the last page)
							if objCount >= 10000 {
								break
							}
						}

						if err != nil {
							err = awsv2.Wrap(err)
							utils.HandleAWSError(debug, "s3:ListObjects", err)
							results = append(results, types.ScanResult{
								ServiceName:  "S3",
								MethodName:   "s3:ListObjects",
								ResourceType: "bucket",
								ResourceName: bucketName,
								Error:        err,
								Timestamp:    time.Now(),
							})
						} else {
							if objCount >= 10000 {
								utils.PrintResult(debug, "", "s3:ListObjects", fmt.Sprintf("S3 Bucket: %s | 10000+ objects", utils.ColorizeItem(bucketName)), nil)
							} else {
								utils.PrintResult(debug, "", "s3:ListObjects", fmt.Sprintf("S3 Bucket: %s | %d objects", utils.ColorizeItem(bucketName), objCount), nil)
							}

							// Add object count result
							results = append(results, types.ScanResult{
								ServiceName:  "S3",
								MethodName:   "s3:ListObjects",
								ResourceType: "bucket",
								ResourceName: bucketName,
								Details: map[string]interface{}{
									"object_count": objCount,
									"region":       region,
								},
								Timestamp: time.Now(),
							})
						}
					}
				}
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
