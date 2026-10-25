package systemsmanager

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

var SystemsManagerCalls = []types.AWSService{
	{
		Name: "ssm:DescribeParameters",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			var allParameters []ssmtypes.ParameterMetadata
			var lastErr error
			anyRegionSucceeded := false
			for _, region := range types.RegionsFor(ctx) {
				svc := awsv2.NewRegionalClient(cfg, region, ssm.NewFromConfig)
				input := &ssm.DescribeParametersInput{}
				regionFailed := false
				for {
					output, err := svc.DescribeParameters(ctx, input)
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

			parameters, ok := output.([]ssmtypes.ParameterMetadata)
			if !ok {
				utils.HandleAWSError(debug, "ssm:DescribeParameters", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			if len(parameters) == 0 {
				utils.PrintAccessGranted(debug, "ssm:DescribeParameters", "SSM parameters")
				return []types.ScanResult{}
			}

			for _, param := range parameters {
				name := aws.ToString(param.Name)
				paramType := string(param.Type)
				description := aws.ToString(param.Description)

				lastModified := ""
				if param.LastModifiedDate != nil {
					lastModified = param.LastModifiedDate.Format("2006-01-02 15:04:05")
				}

				version := param.Version

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
