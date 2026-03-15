package amplify

import (
	"context"
	"fmt"
	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/MillerMedia/awtest/cmd/awtest/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/amplify"
	"time"
)

type AppWithBranches struct {
	App      *amplify.App
	Branches []*amplify.Branch
	Region   string
}

var AmplifyCalls = []types.AWSService{
	{
		Name: "amplify:ListApps",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			var allAppsWithBranches []AppWithBranches

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
				svc := amplify.New(regionSess)
				appsOutput, err := svc.ListAppsWithContext(ctx, &amplify.ListAppsInput{})
				if err != nil {
					return nil, err
				}

				for _, app := range appsOutput.Apps {
					branchesOutput, err := svc.ListBranchesWithContext(ctx, &amplify.ListBranchesInput{AppId: app.AppId})
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
					appName := *appWithBranches.App.Name
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
							utils.PrintResult(debug, "", "amplify:ListBranches", fmt.Sprintf("Found Branch: %s (%s)", *branch.BranchName, appName), nil)

							results = append(results, types.ScanResult{
								ServiceName:  "Amplify",
								MethodName:   "amplify:ListApps",
								ResourceType: "branch",
								ResourceName: *branch.BranchName,
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
