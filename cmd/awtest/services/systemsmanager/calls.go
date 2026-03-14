package systemsmanager

import (
	"context"
	"fmt"
	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/MillerMedia/awtest/cmd/awtest/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"time"
)

var SystemsManagerCalls = []types.AWSService{
	{
		Name: "ssm:DescribeParameters",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			var allParameters []*ssm.ParameterMetadata
			var lastErr error
			anyRegionSucceeded := false
			for _, region := range types.Regions {
				regionSess := sess.Copy(&aws.Config{Region: aws.String(region)})
				svc := ssm.New(regionSess)
				input := &ssm.DescribeParametersInput{}
				regionFailed := false
				for {
					output, err := svc.DescribeParametersWithContext(ctx, input)
					if err != nil {
						lastErr = err
						regionFailed = true
						break
					}
					allParameters = append(allParameters, output.Parameters...)
					if output.NextToken == nil {
						break
					}
					input.NextToken = output.NextToken
				}
				if !regionFailed {
					anyRegionSucceeded = true
				}
			}
			if !anyRegionSucceeded && lastErr != nil {
				return nil, lastErr
			}
			return allParameters, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "ssm:DescribeParameters", err)
				return []types.ScanResult{
					{
						ServiceName: "Systems Manager",
						MethodName:  "ssm:DescribeParameters",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			parameters, ok := output.([]*ssm.ParameterMetadata)
			if !ok {
				utils.HandleAWSError(debug, "ssm:DescribeParameters", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			if len(parameters) == 0 {
				utils.PrintAccessGranted(debug, "ssm:DescribeParameters", "SSM parameters")
				return []types.ScanResult{}
			}

			for _, param := range parameters {
				name := ""
				if param.Name != nil {
					name = *param.Name
				}

				paramType := ""
				if param.Type != nil {
					paramType = *param.Type
				}

				description := ""
				if param.Description != nil {
					description = *param.Description
				}

				lastModified := ""
				if param.LastModifiedDate != nil {
					lastModified = param.LastModifiedDate.Format("2006-01-02 15:04:05")
				}

				var version int64
				if param.Version != nil {
					version = *param.Version
				}

				results = append(results, types.ScanResult{
					ServiceName:  "Systems Manager",
					MethodName:   "ssm:DescribeParameters",
					ResourceType: "parameter",
					ResourceName: name,
					Details: map[string]interface{}{
						"Type":             paramType,
						"Description":      description,
						"LastModifiedDate": lastModified,
						"Version":          version,
					},
					Timestamp: time.Now(),
				})

				utils.PrintResult(debug, "", "ssm:DescribeParameters",
					fmt.Sprintf("Found SSM Parameter: %s (Type: %s, Description: %s, LastModified: %s, Version: %d)",
						utils.ColorizeItem(name), paramType, description, lastModified, version), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
