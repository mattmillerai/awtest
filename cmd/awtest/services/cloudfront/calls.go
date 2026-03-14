package cloudfront

import (
	"context"
	"fmt"
	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/MillerMedia/awtest/cmd/awtest/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudfront"
	"time"
)

type DistributionWithOrigins struct {
	Distribution *cloudfront.DistributionSummary
	Origins      []*cloudfront.Origin
	Region       string
}

var CloudFrontCalls = []types.AWSService{
	{
		Name: "cloudfront:ListDistributions",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			var allDistributionsWithOrigins []DistributionWithOrigins

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
				svc := cloudfront.New(regionSess)
				distributionsOutput, err := svc.ListDistributionsWithContext(ctx, &cloudfront.ListDistributionsInput{})
				if err != nil {
					return nil, err
				}

				for _, distribution := range distributionsOutput.DistributionList.Items {
					distributionWithOrigins := DistributionWithOrigins{
						Distribution: distribution,
						Origins:      distribution.Origins.Items,
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
					distributionID := *distributionWithOrigins.Distribution.Id
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
							utils.PrintResult(debug, "", "cloudfront:ListOrigins", fmt.Sprintf("Found Origin: %s (%s)", *origin.Id, distributionID), nil)

							results = append(results, types.ScanResult{
								ServiceName:  "CloudFront",
								MethodName:   "cloudfront:ListOrigins",
								ResourceType: "origin",
								ResourceName: *origin.Id,
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
