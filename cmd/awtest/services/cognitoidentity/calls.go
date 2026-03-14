package cognitoidentity

import (
	"context"
	"fmt"
	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/MillerMedia/awtest/cmd/awtest/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cognitoidentity"
	"time"
)

var CognitoIdentityCalls = []types.AWSService{
	{
		Name: "cognitoidentity:ListIdentityPools",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			svc := cognitoidentity.New(sess)
			input := &cognitoidentity.ListIdentityPoolsInput{
				MaxResults: aws.Int64(60), // You can adjust this as needed
			}
			output, err := svc.ListIdentityPoolsWithContext(ctx, input)
			if err != nil {
				return nil, err
			}
			return output.IdentityPools, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "cognitoidentity:ListIdentityPools", err)
				return []types.ScanResult{
					{
						ServiceName: "CognitoIdentity",
						MethodName:  "cognitoidentity:ListIdentityPools",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			if identityPools, ok := output.([]*cognitoidentity.IdentityPoolShortDescription); ok {
				for _, pool := range identityPools {
					results = append(results, types.ScanResult{
						ServiceName:  "CognitoIdentity",
						MethodName:   "cognitoidentity:ListIdentityPools",
						ResourceType: "identity-pool",
						ResourceName: *pool.IdentityPoolName,
						Details:      map[string]interface{}{},
						Timestamp:    time.Now(),
					})

					utils.PrintResult(debug, "", "cognitoidentity:ListIdentityPools", fmt.Sprintf("Found Identity Pool: %s (%s)", *pool.IdentityPoolName, *pool.IdentityPoolId), nil)
				}
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
