package ses

import (
	"context"
	"fmt"
	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/MillerMedia/awtest/cmd/awtest/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ses"
	"time"
)

var SESCalls = []types.AWSService{
	{
		Name: "ses:ListIdentities",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			originalConfig := sess.Config
			var allIdentities []string
			for _, region := range types.Regions {
				regionConfig := &aws.Config{
					Region:      aws.String(region),
					Credentials: originalConfig.Credentials,
				}
				regionSess, err := session.NewSession(regionConfig)
				if err != nil {
					return nil, err
				}
				svc := ses.New(regionSess)
				output, err := svc.ListIdentitiesWithContext(ctx, &ses.ListIdentitiesInput{})
				if err != nil {
					return nil, err
				}
				allIdentities = append(allIdentities, aws.StringValueSlice(output.Identities)...)
			}
			return allIdentities, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "ses:ListIdentities", err)
				return []types.ScanResult{
					{
						ServiceName: "SES",
						MethodName:  "ses:ListIdentities",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			if identities, ok := output.([]string); ok {
				for _, identity := range identities {
					results = append(results, types.ScanResult{
						ServiceName:  "SES",
						MethodName:   "ses:ListIdentities",
						ResourceType: "identity",
						ResourceName: identity,
						Details:      map[string]interface{}{},
						Timestamp:    time.Now(),
					})

					utils.PrintResult(debug, "", "ses:ListIdentities", fmt.Sprintf("Found Identity: %s", identity), nil)
				}
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
