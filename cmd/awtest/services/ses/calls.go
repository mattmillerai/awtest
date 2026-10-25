package ses

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

var SESCalls = []types.AWSService{
	{
		Name: "ses:ListIdentities",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			var allIdentities []string
			for _, region := range types.RegionsFor(ctx) {
				svc := awsv2.NewRegionalClient(cfg, region, ses.NewFromConfig)
				output, err := svc.ListIdentities(ctx, &ses.ListIdentitiesInput{})
				if err != nil {
					return nil, err
				}
				allIdentities = append(allIdentities, output.Identities...)
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

			identities, ok := output.([]string)
			if !ok {
				utils.HandleAWSError(debug, "ses:ListIdentities", fmt.Errorf("unexpected output type %T", output))
				return results
			}

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
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
