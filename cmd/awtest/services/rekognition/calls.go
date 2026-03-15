package rekognition

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/MillerMedia/awtest/cmd/awtest/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/rekognition"
)

var RekognitionCalls = []types.AWSService{
	{
		Name: "rekognition:ListCollections",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			svc := rekognition.New(sess)
			output, err := svc.ListCollectionsWithContext(ctx, &rekognition.ListCollectionsInput{
				MaxResults: aws.Int64(4096),
			})
			if err != nil {
				return nil, err
			}
			return output.CollectionIds, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "rekognition:ListCollections", err)
				return []types.ScanResult{
					{
						ServiceName: "Rekognition",
						MethodName:  "rekognition:ListCollections",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			if collections, ok := output.([]*string); ok {
				if len(collections) == 0 {
					utils.PrintAccessGranted(debug, "rekognition:ListCollections", "Rekognition collections")
					return results
				}
				for _, id := range collections {
					collectionID := ""
					if id != nil {
						collectionID = *id
					}

					results = append(results, types.ScanResult{
						ServiceName:  "Rekognition",
						MethodName:   "rekognition:ListCollections",
						ResourceType: "collection",
						ResourceName: collectionID,
						Details:      map[string]interface{}{},
						Timestamp:    time.Now(),
					})

					utils.PrintResult(debug, "", "rekognition:ListCollections", fmt.Sprintf("Rekognition collection: %s", utils.ColorizeItem(collectionID)), nil)
				}
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
	{
		// Note: ListStreamProcessors is a legacy API; AWS recommends newer Rekognition Streaming APIs.
		Name: "rekognition:ListStreamProcessors",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			svc := rekognition.New(sess)
			output, err := svc.ListStreamProcessorsWithContext(ctx, &rekognition.ListStreamProcessorsInput{})
			if err != nil {
				return nil, err
			}
			return output.StreamProcessors, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "rekognition:ListStreamProcessors", err)
				return []types.ScanResult{
					{
						ServiceName: "Rekognition",
						MethodName:  "rekognition:ListStreamProcessors",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			if processors, ok := output.([]*rekognition.StreamProcessor); ok {
				if len(processors) == 0 {
					utils.PrintAccessGranted(debug, "rekognition:ListStreamProcessors", "Rekognition stream processors")
					return results
				}
				for _, sp := range processors {
					name := ""
					if sp.Name != nil {
						name = *sp.Name
					}
					status := ""
					if sp.Status != nil {
						status = *sp.Status
					}

					results = append(results, types.ScanResult{
						ServiceName:  "Rekognition",
						MethodName:   "rekognition:ListStreamProcessors",
						ResourceType: "stream-processor",
						ResourceName: name,
						Details:      map[string]interface{}{"status": status},
						Timestamp:    time.Now(),
					})

					utils.PrintResult(debug, "", "rekognition:ListStreamProcessors", fmt.Sprintf("Rekognition stream processor: %s", utils.ColorizeItem(name)), nil)
				}
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
	{
		Name: "rekognition:DescribeProjects",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			svc := rekognition.New(sess)
			output, err := svc.DescribeProjectsWithContext(ctx, &rekognition.DescribeProjectsInput{})
			if err != nil {
				return nil, err
			}
			return output.ProjectDescriptions, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "rekognition:DescribeProjects", err)
				return []types.ScanResult{
					{
						ServiceName: "Rekognition",
						MethodName:  "rekognition:DescribeProjects",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			if projects, ok := output.([]*rekognition.ProjectDescription); ok {
				if len(projects) == 0 {
					utils.PrintAccessGranted(debug, "rekognition:DescribeProjects", "Rekognition projects")
					return results
				}
				for _, proj := range projects {
					name := ""
					if proj.ProjectArn != nil {
						// Extract project name from ARN: arn:aws:rekognition:REGION:ACCOUNT:project/NAME/TIMESTAMP
						parts := strings.Split(*proj.ProjectArn, "/")
						if len(parts) >= 2 {
							name = parts[1]
						} else {
							name = *proj.ProjectArn
						}
					}
					status := ""
					if proj.Status != nil {
						status = *proj.Status
					}

					results = append(results, types.ScanResult{
						ServiceName:  "Rekognition",
						MethodName:   "rekognition:DescribeProjects",
						ResourceType: "project",
						ResourceName: name,
						Details:      map[string]interface{}{"status": status},
						Timestamp:    time.Now(),
					})

					utils.PrintResult(debug, "", "rekognition:DescribeProjects", fmt.Sprintf("Rekognition project: %s", utils.ColorizeItem(name)), nil)
				}
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
