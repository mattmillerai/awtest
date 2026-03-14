package cognitouserpools

import (
	"context"
	"fmt"
	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/MillerMedia/awtest/cmd/awtest/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cognitoidentityprovider"
	"time"
)

var CognitoUserPoolsCalls = []types.AWSService{
	{
		Name: "cognito-idp:ListUserPools",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			var allUserPools []*cognitoidentityprovider.UserPoolDescriptionType
			for _, region := range types.Regions {
				sess.Config.Region = aws.String(region)
				svc := cognitoidentityprovider.New(sess)
				output, err := svc.ListUserPoolsWithContext(ctx, &cognitoidentityprovider.ListUserPoolsInput{
					MaxResults: aws.Int64(60),
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

			if pools, ok := output.([]*cognitoidentityprovider.UserPoolDescriptionType); ok {
				for _, pool := range pools {
					poolName := ""
					if pool.Name != nil {
						poolName = *pool.Name
					}

					poolId := ""
					if pool.Id != nil {
						poolId = *pool.Id
					}

					poolStatus := ""
					if pool.Status != nil {
						poolStatus = *pool.Status
					}

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
