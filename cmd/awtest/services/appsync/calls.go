package appsync

import (
	"context"
	"fmt"
	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/MillerMedia/awtest/cmd/awtest/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/appsync"
	"time"
)

var AppSyncCalls = []types.AWSService{
	{
		Name: "appsync:ListGraphqlApis",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			var allApis []*appsync.GraphqlApi
			originalConfig := sess.Config
			for _, region := range types.Regions {
				regionConfig := &aws.Config{
					Region:      aws.String(region),
					Credentials: originalConfig.Credentials,
				}
				regionSess, err := session.NewSession(regionConfig)
				if err != nil {
					return nil, err
				}
				svc := appsync.New(regionSess)
				output, err := svc.ListGraphqlApisWithContext(ctx, &appsync.ListGraphqlApisInput{})
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

			if apis, ok := output.([]*appsync.GraphqlApi); ok {
				for _, api := range apis {
					results = append(results, types.ScanResult{
						ServiceName:  "AppSync",
						MethodName:   "appsync:ListGraphqlApis",
						ResourceType: "graphql-api",
						ResourceName: *api.Name,
						Details:      map[string]interface{}{},
						Timestamp:    time.Now(),
					})

					utils.PrintResult(debug, "", "appsync:ListGraphqlApis", fmt.Sprintf("Found AppSync API: %s", *api.Name), nil)
				}
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
