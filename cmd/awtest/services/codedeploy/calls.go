package codedeploy

import (
	"context"
	"fmt"
	"time"

	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/MillerMedia/awtest/cmd/awtest/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/codedeploy"
)

const maxBatchSize = 25

type cdApplication struct {
	Name            string
	ApplicationId   string
	ComputePlatform string
	CreateTime      string
	LinkedToGitHub  string
	Region          string
}

type cdDeploymentGroup struct {
	ApplicationName      string
	GroupName            string
	DeploymentGroupId    string
	DeploymentConfigName string
	ComputePlatform      string
	ServiceRoleArn       string
	Region               string
}

type cdDeploymentConfig struct {
	Name               string
	DeploymentConfigId string
	ComputePlatform    string
	CreateTime         string
	Region             string
}

func batchGetApplications(ctx context.Context, svc *codedeploy.CodeDeploy, names []*string, region string) []cdApplication {
	var results []cdApplication
	for i := 0; i < len(names); i += maxBatchSize {
		end := i + maxBatchSize
		if end > len(names) {
			end = len(names)
		}
		batch := names[i:end]
		output, err := svc.BatchGetApplicationsWithContext(ctx, &codedeploy.BatchGetApplicationsInput{
			ApplicationNames: batch,
		})
		if err != nil {
			utils.HandleAWSError(false, "codedeploy:BatchGetApplications", err)
			continue
		}
		for _, app := range output.ApplicationsInfo {
			if app != nil {
				results = append(results, extractApplication(app, region))
			}
		}
	}
	return results
}

func extractApplication(app *codedeploy.ApplicationInfo, region string) cdApplication {
	name := ""
	if app.ApplicationName != nil {
		name = *app.ApplicationName
	}
	appId := ""
	if app.ApplicationId != nil {
		appId = *app.ApplicationId
	}
	platform := ""
	if app.ComputePlatform != nil {
		platform = *app.ComputePlatform
	}
	createTime := ""
	if app.CreateTime != nil {
		createTime = app.CreateTime.Format(time.RFC3339)
	}
	linkedToGitHub := ""
	if app.LinkedToGitHub != nil {
		linkedToGitHub = fmt.Sprintf("%t", *app.LinkedToGitHub)
	}
	return cdApplication{
		Name:            name,
		ApplicationId:   appId,
		ComputePlatform: platform,
		CreateTime:      createTime,
		LinkedToGitHub:  linkedToGitHub,
		Region:          region,
	}
}

func batchGetDeploymentGroups(ctx context.Context, svc *codedeploy.CodeDeploy, appName string, names []*string, region string) []cdDeploymentGroup {
	var results []cdDeploymentGroup
	for i := 0; i < len(names); i += maxBatchSize {
		end := i + maxBatchSize
		if end > len(names) {
			end = len(names)
		}
		batch := names[i:end]
		output, err := svc.BatchGetDeploymentGroupsWithContext(ctx, &codedeploy.BatchGetDeploymentGroupsInput{
			ApplicationName:      aws.String(appName),
			DeploymentGroupNames: batch,
		})
		if err != nil {
			utils.HandleAWSError(false, "codedeploy:BatchGetDeploymentGroups", err)
			continue
		}
		if output.ErrorMessage != nil && *output.ErrorMessage != "" {
			utils.HandleAWSError(false, "codedeploy:BatchGetDeploymentGroups",
				fmt.Errorf("partial error: %s", *output.ErrorMessage))
		}
		for _, dg := range output.DeploymentGroupsInfo {
			if dg != nil {
				results = append(results, extractDeploymentGroup(dg, region))
			}
		}
	}
	return results
}

func extractDeploymentGroup(dg *codedeploy.DeploymentGroupInfo, region string) cdDeploymentGroup {
	appName := ""
	if dg.ApplicationName != nil {
		appName = *dg.ApplicationName
	}
	groupName := ""
	if dg.DeploymentGroupName != nil {
		groupName = *dg.DeploymentGroupName
	}
	groupId := ""
	if dg.DeploymentGroupId != nil {
		groupId = *dg.DeploymentGroupId
	}
	configName := ""
	if dg.DeploymentConfigName != nil {
		configName = *dg.DeploymentConfigName
	}
	platform := ""
	if dg.ComputePlatform != nil {
		platform = *dg.ComputePlatform
	}
	roleArn := ""
	if dg.ServiceRoleArn != nil {
		roleArn = *dg.ServiceRoleArn
	}
	return cdDeploymentGroup{
		ApplicationName:      appName,
		GroupName:            groupName,
		DeploymentGroupId:    groupId,
		DeploymentConfigName: configName,
		ComputePlatform:      platform,
		ServiceRoleArn:       roleArn,
		Region:               region,
	}
}

func extractDeploymentConfig(cfg *codedeploy.DeploymentConfigInfo, region string) cdDeploymentConfig {
	name := ""
	if cfg.DeploymentConfigName != nil {
		name = *cfg.DeploymentConfigName
	}
	configId := ""
	if cfg.DeploymentConfigId != nil {
		configId = *cfg.DeploymentConfigId
	}
	platform := ""
	if cfg.ComputePlatform != nil {
		platform = *cfg.ComputePlatform
	}
	createTime := ""
	if cfg.CreateTime != nil {
		createTime = cfg.CreateTime.Format(time.RFC3339)
	}
	return cdDeploymentConfig{
		Name:               name,
		DeploymentConfigId: configId,
		ComputePlatform:    platform,
		CreateTime:         createTime,
		Region:             region,
	}
}

var CodeDeployCalls = []types.AWSService{
	{
		Name: "codedeploy:ListApplications",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			var allApps []cdApplication
			var lastErr error

			for _, region := range types.Regions {
				svc := codedeploy.New(sess, &aws.Config{Region: aws.String(region)})
				var allAppNames []*string
				var nextToken *string
				for {
					input := &codedeploy.ListApplicationsInput{}
					if nextToken != nil {
						input.NextToken = nextToken
					}
					output, err := svc.ListApplicationsWithContext(ctx, input)
					if err != nil {
						lastErr = err
						utils.HandleAWSError(false, "codedeploy:ListApplications", err)
						break
					}
					allAppNames = append(allAppNames, output.Applications...)
					if output.NextToken == nil {
						break
					}
					nextToken = output.NextToken
				}
				if len(allAppNames) > 0 {
					allApps = append(allApps, batchGetApplications(ctx, svc, allAppNames, region)...)
				}
			}

			if len(allApps) == 0 && lastErr != nil {
				return nil, lastErr
			}
			return allApps, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "codedeploy:ListApplications", err)
				return []types.ScanResult{
					{
						ServiceName: "CodeDeploy",
						MethodName:  "codedeploy:ListApplications",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			apps, ok := output.([]cdApplication)
			if !ok {
				utils.HandleAWSError(debug, "codedeploy:ListApplications", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, app := range apps {
				results = append(results, types.ScanResult{
					ServiceName:  "CodeDeploy",
					MethodName:   "codedeploy:ListApplications",
					ResourceType: "application",
					ResourceName: app.Name,
					Details: map[string]interface{}{
						"Name":            app.Name,
						"ApplicationId":   app.ApplicationId,
						"ComputePlatform": app.ComputePlatform,
						"CreateTime":      app.CreateTime,
						"LinkedToGitHub":  app.LinkedToGitHub,
						"Region":          app.Region,
					},
					Timestamp: time.Now(),
				})

				utils.PrintResult(debug, "", "codedeploy:ListApplications",
					fmt.Sprintf("CodeDeploy Application: %s (Platform: %s, Created: %s, Region: %s)", utils.ColorizeItem(app.Name), app.ComputePlatform, app.CreateTime, app.Region), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
	{
		Name: "codedeploy:ListDeploymentGroups",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			var allGroups []cdDeploymentGroup
			var lastErr error

			for _, region := range types.Regions {
				svc := codedeploy.New(sess, &aws.Config{Region: aws.String(region)})

				// Step 1: List all application names
				var appNames []*string
				var nextToken *string
				for {
					input := &codedeploy.ListApplicationsInput{}
					if nextToken != nil {
						input.NextToken = nextToken
					}
					output, err := svc.ListApplicationsWithContext(ctx, input)
					if err != nil {
						lastErr = err
						utils.HandleAWSError(false, "codedeploy:ListDeploymentGroups", err)
						break
					}
					appNames = append(appNames, output.Applications...)
					if output.NextToken == nil {
						break
					}
					nextToken = output.NextToken
				}

				// Step 2: For each application, list and batch-get deployment groups
				for _, appNamePtr := range appNames {
					if appNamePtr == nil {
						continue
					}
					appName := *appNamePtr
					var groupNames []*string
					var dgNextToken *string
					for {
						dgInput := &codedeploy.ListDeploymentGroupsInput{
							ApplicationName: aws.String(appName),
						}
						if dgNextToken != nil {
							dgInput.NextToken = dgNextToken
						}
						dgOutput, err := svc.ListDeploymentGroupsWithContext(ctx, dgInput)
						if err != nil {
							utils.HandleAWSError(false, "codedeploy:ListDeploymentGroups", err)
							break
						}
						groupNames = append(groupNames, dgOutput.DeploymentGroups...)
						if dgOutput.NextToken == nil {
							break
						}
						dgNextToken = dgOutput.NextToken
					}
					if len(groupNames) > 0 {
						allGroups = append(allGroups, batchGetDeploymentGroups(ctx, svc, appName, groupNames, region)...)
					}
				}
			}

			if len(allGroups) == 0 && lastErr != nil {
				return nil, lastErr
			}
			return allGroups, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "codedeploy:ListDeploymentGroups", err)
				return []types.ScanResult{
					{
						ServiceName: "CodeDeploy",
						MethodName:  "codedeploy:ListDeploymentGroups",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			groups, ok := output.([]cdDeploymentGroup)
			if !ok {
				utils.HandleAWSError(debug, "codedeploy:ListDeploymentGroups", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, g := range groups {
				resourceName := g.GroupName
				if g.ApplicationName != "" {
					resourceName = g.ApplicationName + "/" + g.GroupName
				}

				results = append(results, types.ScanResult{
					ServiceName:  "CodeDeploy",
					MethodName:   "codedeploy:ListDeploymentGroups",
					ResourceType: "deployment-group",
					ResourceName: resourceName,
					Details: map[string]interface{}{
						"ApplicationName":      g.ApplicationName,
						"GroupName":            g.GroupName,
						"DeploymentGroupId":    g.DeploymentGroupId,
						"DeploymentConfigName": g.DeploymentConfigName,
						"ComputePlatform":      g.ComputePlatform,
						"ServiceRoleArn":       g.ServiceRoleArn,
						"Region":               g.Region,
					},
					Timestamp: time.Now(),
				})

				utils.PrintResult(debug, "", "codedeploy:ListDeploymentGroups",
					fmt.Sprintf("CodeDeploy Deployment Group: %s (App: %s, Config: %s, Role: %s, Region: %s)", utils.ColorizeItem(g.GroupName), g.ApplicationName, g.DeploymentConfigName, g.ServiceRoleArn, g.Region), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
	{
		Name: "codedeploy:ListDeploymentConfigs",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			var allConfigs []cdDeploymentConfig
			var lastErr error

			for _, region := range types.Regions {
				svc := codedeploy.New(sess, &aws.Config{Region: aws.String(region)})
				var configNames []*string
				var nextToken *string
				for {
					input := &codedeploy.ListDeploymentConfigsInput{}
					if nextToken != nil {
						input.NextToken = nextToken
					}
					output, err := svc.ListDeploymentConfigsWithContext(ctx, input)
					if err != nil {
						lastErr = err
						utils.HandleAWSError(false, "codedeploy:ListDeploymentConfigs", err)
						break
					}
					configNames = append(configNames, output.DeploymentConfigsList...)
					if output.NextToken == nil {
						break
					}
					nextToken = output.NextToken
				}

				for _, namePtr := range configNames {
					if namePtr == nil {
						continue
					}
					configName := *namePtr
					output, err := svc.GetDeploymentConfigWithContext(ctx, &codedeploy.GetDeploymentConfigInput{
						DeploymentConfigName: aws.String(configName),
					})
					if err != nil {
						utils.HandleAWSError(false, "codedeploy:ListDeploymentConfigs", err)
						continue
					}
					if output.DeploymentConfigInfo != nil {
						allConfigs = append(allConfigs, extractDeploymentConfig(output.DeploymentConfigInfo, region))
					}
				}
			}

			if len(allConfigs) == 0 && lastErr != nil {
				return nil, lastErr
			}
			return allConfigs, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "codedeploy:ListDeploymentConfigs", err)
				return []types.ScanResult{
					{
						ServiceName: "CodeDeploy",
						MethodName:  "codedeploy:ListDeploymentConfigs",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			configs, ok := output.([]cdDeploymentConfig)
			if !ok {
				utils.HandleAWSError(debug, "codedeploy:ListDeploymentConfigs", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, c := range configs {
				results = append(results, types.ScanResult{
					ServiceName:  "CodeDeploy",
					MethodName:   "codedeploy:ListDeploymentConfigs",
					ResourceType: "deployment-config",
					ResourceName: c.Name,
					Details: map[string]interface{}{
						"Name":               c.Name,
						"DeploymentConfigId": c.DeploymentConfigId,
						"ComputePlatform":    c.ComputePlatform,
						"CreateTime":         c.CreateTime,
						"Region":             c.Region,
					},
					Timestamp: time.Now(),
				})

				utils.PrintResult(debug, "", "codedeploy:ListDeploymentConfigs",
					fmt.Sprintf("CodeDeploy Deployment Config: %s (Platform: %s, Created: %s, Region: %s)", utils.ColorizeItem(c.Name), c.ComputePlatform, c.CreateTime, c.Region), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
