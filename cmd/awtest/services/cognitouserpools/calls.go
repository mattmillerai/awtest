package cognitouserpools

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	cognitotypes "github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

var CognitoUserPoolsCalls = []types.AWSService{
	{
		Name: "cognito-idp:ListUserPools",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			var allUserPools []cognitotypes.UserPoolDescriptionType
			for _, region := range types.RegionsFor(ctx) {
				svc := awsv2.NewRegionalClient(cfg, region, cognitoidentityprovider.NewFromConfig)
				output, err := svc.ListUserPools(ctx, &cognitoidentityprovider.ListUserPoolsInput{
					MaxResults: aws.Int32(60),
				})
				if err != nil {
					return nil, err
				}
				allUserPools = append(allUserPools, output.UserPools...)
			}
			return allUserPools, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "cognito-idp:ListUserPools", err)
				return []types.ScanResult{
					{
						ServiceName: "CognitoUserPools",
						MethodName:  "cognito-idp:ListUserPools",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			if pools, ok := output.([]cognitotypes.UserPoolDescriptionType); ok {
				for _, pool := range pools {
					poolName := aws.ToString(pool.Name)
					poolId := aws.ToString(pool.Id)
					poolStatus := string(pool.Status)

					creationDate := ""
					if pool.CreationDate != nil {
						creationDate = pool.CreationDate.Format("2006-01-02 15:04:05")
					}

					results = append(results, types.ScanResult{
						ServiceName:  "CognitoUserPools",
						MethodName:   "cognito-idp:ListUserPools",
						ResourceType: "user-pool",
						ResourceName: poolName,
						Details: map[string]interface{}{
							"Id":           poolId,
							"Status":       poolStatus,
							"CreationDate": creationDate,
						},
						Timestamp: time.Now(),
					})

					utils.PrintResult(debug, "", "cognito-idp:ListUserPools",
						fmt.Sprintf("Found User Pool: %s (ID: %s, Status: %s, Created: %s)", utils.ColorizeItem(poolName), poolId, poolStatus, creationDate), nil)
				}
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
