package appsync

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/appsync"
	appsynctypes "github.com/aws/aws-sdk-go-v2/service/appsync/types"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

var AppSyncCalls = []types.AWSService{
	{
		Name: "appsync:ListGraphqlApis",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			var allApis []appsynctypes.GraphqlApi
			for _, region := range types.RegionsFor(ctx) {
				svc := awsv2.NewRegionalClient(cfg, region, appsync.NewFromConfig)
				output, err := svc.ListGraphqlApis(ctx, &appsync.ListGraphqlApisInput{})
				if err != nil {
					return nil, err
				}
				allApis = append(allApis, output.GraphqlApis...)
			}
			return allApis, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "appsync:ListGraphqlApis", err)
				return []types.ScanResult{
					{
						ServiceName: "AppSync",
						MethodName:  "appsync:ListGraphqlApis",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			apis, ok := output.([]appsynctypes.GraphqlApi)
			if !ok {
				utils.HandleAWSError(debug, "appsync:ListGraphqlApis", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, api := range apis {
				apiName := aws.ToString(api.Name)
				results = append(results, types.ScanResult{
					ServiceName:  "AppSync",
					MethodName:   "appsync:ListGraphqlApis",
					ResourceType: "graphql-api",
					ResourceName: apiName,
					Details:      map[string]interface{}{},
					Timestamp:    time.Now(),
				})

				utils.PrintResult(debug, "", "appsync:ListGraphqlApis", fmt.Sprintf("Found AppSync API: %s", apiName), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
