package s3

import (
	"context"
	"fmt"
	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/MillerMedia/awtest/cmd/awtest/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"time"
)

var S3Calls = []types.AWSService{
	{
		Name: "s3:ListBuckets",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			svc := s3.New(sess)
			output, err := svc.ListBucketsWithContext(ctx, &s3.ListBucketsInput{})
			return map[string]interface{}{
				"output": output,
				"sess":   sess,
				"ctx":    ctx,
			}, err
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
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
				sess, _ := outputMap["sess"].(*session.Session)
				ctx, _ := outputMap["ctx"].(context.Context)
				if ctx == nil {
					ctx = context.Background()
				}
				for _, bucket := range s3Output.Buckets {
					bucketName := ""
					if bucket.Name != nil {
						bucketName = *bucket.Name
					}

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
					svc := s3.New(sess)
					locationOutput, err := svc.GetBucketLocationWithContext(ctx, &s3.GetBucketLocationInput{
						Bucket: aws.String(bucketName),
					})

					if err == nil {
						// This is the correct region. Perform ListObjects here.
						region := locationOutput.LocationConstraint
						if region == nil {
							region = aws.String("us-east-1") // default to us-east-1 if the bucket region is not specified
						}

						sessWithRegion := sess.Copy(&aws.Config{Region: region})
						svc := s3.New(sessWithRegion)
						listObjInput := &s3.ListObjectsV2Input{Bucket: bucket.Name}

						// Counter for the objects
						objCount := 0
						// Function to handle each page of results
						err = svc.ListObjectsV2PagesWithContext(ctx, listObjInput, func(page *s3.ListObjectsV2Output, lastPage bool) bool {
							objCount += len(page.Contents)
							// Continue fetching pages only if we have less than 10000 objects and this is not the last page
							return objCount < 10000 && !lastPage
						})

						if err != nil {
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
									"region":       *region,
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
