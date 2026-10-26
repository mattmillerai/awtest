package cloudfront

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

type DistributionWithOrigins struct {
	Distribution cftypes.DistributionSummary
	Origins      []cftypes.Origin
	Region       string
}

var CloudFrontCalls = []types.AWSService{
	{
		Name: "cloudfront:ListDistributions",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			var allDistributionsWithOrigins []DistributionWithOrigins

			for _, region := range types.RegionsFor(ctx) {
				svc := awsv2.NewRegionalClient(cfg, region, cloudfront.NewFromConfig)
				distributionsOutput, err := svc.ListDistributions(ctx, &cloudfront.ListDistributionsInput{})
				if err != nil {
					return nil, err
				}

				if distributionsOutput.DistributionList == nil {
					continue
				}
				for _, distribution := range distributionsOutput.DistributionList.Items {
					var origins []cftypes.Origin
					if distribution.Origins != nil {
						origins = distribution.Origins.Items
					}
					distributionWithOrigins := DistributionWithOrigins{
						Distribution: distribution,
						Origins:      origins,
						Region:       region,
					}
					allDistributionsWithOrigins = append(allDistributionsWithOrigins, distributionWithOrigins)
				}
			}
			return allDistributionsWithOrigins, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "cloudfront:ListDistributions", err)
				return []types.ScanResult{
					{
						ServiceName: "CloudFront",
						MethodName:  "cloudfront:ListDistributions",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			if distributionsWithOrigins, ok := output.([]DistributionWithOrigins); ok {
				for _, distributionWithOrigins := range distributionsWithOrigins {
					fmt.Println()
					distributionID := aws.ToString(distributionWithOrigins.Distribution.Id)
					utils.PrintResult(debug, "", "cloudfront:ListDistributions", fmt.Sprintf("Found Distribution: %s", distributionID), nil)

					results = append(results, types.ScanResult{
						ServiceName:  "CloudFront",
						MethodName:   "cloudfront:ListDistributions",
						ResourceType: "distribution",
						ResourceName: distributionID,
						Details:      map[string]interface{}{},
						Timestamp:    time.Now(),
					})

					if len(distributionWithOrigins.Origins) > 0 {
						for _, origin := range distributionWithOrigins.Origins {
							originID := aws.ToString(origin.Id)
							utils.PrintResult(debug, "", "cloudfront:ListOrigins", fmt.Sprintf("Found Origin: %s (%s)", originID, distributionID), nil)

							results = append(results, types.ScanResult{
								ServiceName:  "CloudFront",
								MethodName:   "cloudfront:ListOrigins",
								ResourceType: "origin",
								ResourceName: originID,
								Details:      map[string]interface{}{},
								Timestamp:    time.Now(),
							})
						}
					}
				}
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
	// You can add more methods here similar to the one above
}
