package amplify

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/amplify"
	amplifytypes "github.com/aws/aws-sdk-go-v2/service/amplify/types"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

type AppWithBranches struct {
	App      amplifytypes.App
	Branches []amplifytypes.Branch
	Region   string
}

var AmplifyCalls = []types.AWSService{
	{
		Name: "amplify:ListApps",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			var allAppsWithBranches []AppWithBranches

			for _, region := range types.RegionsFor(ctx) {
				svc := awsv2.NewRegionalClient(cfg, region, amplify.NewFromConfig)
				appsOutput, err := svc.ListApps(ctx, &amplify.ListAppsInput{})
				if err != nil {
					return nil, err
				}

				for _, app := range appsOutput.Apps {
					branchesOutput, err := svc.ListBranches(ctx, &amplify.ListBranchesInput{AppId: app.AppId})
					if err != nil {
						return nil, err
					}

					appWithBranches := AppWithBranches{
						App:      app,
						Branches: branchesOutput.Branches,
						Region:   region,
					}
					allAppsWithBranches = append(allAppsWithBranches, appWithBranches)
				}
			}
			return allAppsWithBranches, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "amplify:ListApps", err)
				return []types.ScanResult{
					{
						ServiceName: "Amplify",
						MethodName:  "amplify:ListApps",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			if appsWithBranches, ok := output.([]AppWithBranches); ok {
				for _, appWithBranches := range appsWithBranches {
					fmt.Println()
					appName := aws.ToString(appWithBranches.App.Name)
					utils.PrintResult(debug, "", "amplify:ListApps", fmt.Sprintf("Found Amplify App: %s", appName), nil)

					results = append(results, types.ScanResult{
						ServiceName:  "Amplify",
						MethodName:   "amplify:ListApps",
						ResourceType: "app",
						ResourceName: appName,
						Details:      map[string]interface{}{},
						Timestamp:    time.Now(),
					})

					if len(appWithBranches.Branches) > 0 {
						for _, branch := range appWithBranches.Branches {
							branchName := aws.ToString(branch.BranchName)
							utils.PrintResult(debug, "", "amplify:ListBranches", fmt.Sprintf("Found Branch: %s (%s)", branchName, appName), nil)

							results = append(results, types.ScanResult{
								ServiceName:  "Amplify",
								MethodName:   "amplify:ListApps",
								ResourceType: "branch",
								ResourceName: branchName,
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
