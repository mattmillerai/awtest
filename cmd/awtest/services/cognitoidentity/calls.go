package cognitoidentity

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentity"
	cogtypes "github.com/aws/aws-sdk-go-v2/service/cognitoidentity/types"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

var CognitoIdentityCalls = []types.AWSService{
	{
		Name: "cognitoidentity:ListIdentityPools",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			svc := awsv2.NewRegionalClient(cfg, "", cognitoidentity.NewFromConfig)
			input := &cognitoidentity.ListIdentityPoolsInput{
				MaxResults: aws.Int32(60), // You can adjust this as needed
			}
			output, err := svc.ListIdentityPools(ctx, input)
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

			identityPools, ok := output.([]cogtypes.IdentityPoolShortDescription)
			if !ok {
				utils.HandleAWSError(debug, "cognitoidentity:ListIdentityPools", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, pool := range identityPools {
				poolName := aws.ToString(pool.IdentityPoolName)
				results = append(results, types.ScanResult{
					ServiceName:  "CognitoIdentity",
					MethodName:   "cognitoidentity:ListIdentityPools",
					ResourceType: "identity-pool",
					ResourceName: poolName,
					Details:      map[string]interface{}{},
					Timestamp:    time.Now(),
				})

				utils.PrintResult(debug, "", "cognitoidentity:ListIdentityPools", fmt.Sprintf("Found Identity Pool: %s (%s)", poolName, aws.ToString(pool.IdentityPoolId)), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
