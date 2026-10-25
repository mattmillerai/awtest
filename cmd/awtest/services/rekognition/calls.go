package rekognition

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rekognition"
	rekognitiontypes "github.com/aws/aws-sdk-go-v2/service/rekognition/types"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

var RekognitionCalls = []types.AWSService{
	{
		Name: "rekognition:ListCollections",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			svc := awsv2.NewRegionalClient(cfg, "", rekognition.NewFromConfig)
			output, err := svc.ListCollections(ctx, &rekognition.ListCollectionsInput{
				MaxResults: aws.Int32(4096),
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

			if collections, ok := output.([]string); ok {
				if len(collections) == 0 {
					utils.PrintAccessGranted(debug, "rekognition:ListCollections", "Rekognition collections")
					return results
				}
				for _, collectionID := range collections {
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
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			svc := awsv2.NewRegionalClient(cfg, "", rekognition.NewFromConfig)
			output, err := svc.ListStreamProcessors(ctx, &rekognition.ListStreamProcessorsInput{})
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

			if processors, ok := output.([]rekognitiontypes.StreamProcessor); ok {
				if len(processors) == 0 {
					utils.PrintAccessGranted(debug, "rekognition:ListStreamProcessors", "Rekognition stream processors")
					return results
				}
				for _, sp := range processors {
					name := aws.ToString(sp.Name)
					status := string(sp.Status)

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
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			svc := awsv2.NewRegionalClient(cfg, "", rekognition.NewFromConfig)
			output, err := svc.DescribeProjects(ctx, &rekognition.DescribeProjectsInput{})
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

			if projects, ok := output.([]rekognitiontypes.ProjectDescription); ok {
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
					status := string(proj.Status)

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
