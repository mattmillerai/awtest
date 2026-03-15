package ecs

import (
	"context"
	"fmt"
	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/MillerMedia/awtest/cmd/awtest/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecs"
	"time"
)

var ECSCalls = []types.AWSService{
	{
		Name: "ecs:ListClusters",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			var allClusters []*ecs.Cluster

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
				svc := ecs.New(regionSess)
				output, err := svc.ListClustersWithContext(ctx, &ecs.ListClustersInput{})
				if err != nil {
					return nil, err
				}

				if len(output.ClusterArns) > 0 {
					describeOutput, err := svc.DescribeClustersWithContext(ctx, &ecs.DescribeClustersInput{
						Clusters: output.ClusterArns,
					})
					if err != nil {
						return nil, err
					}
					allClusters = append(allClusters, describeOutput.Clusters...)
				}
			}
			return allClusters, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "ecs:ListClusters", err)
				return []types.ScanResult{
					{
						ServiceName: "ECS",
						MethodName:  "ecs:ListClusters",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}
			if clusters, ok := output.([]*ecs.Cluster); ok {
				if len(clusters) == 0 {
					utils.PrintResult(debug, "", "ecs:ListClusters", "No ECS clusters found.", nil)
				} else {
					for _, cluster := range clusters {
						clusterName := aws.StringValue(cluster.ClusterName)
						status := aws.StringValue(cluster.Status)
						runningTasks := aws.Int64Value(cluster.RunningTasksCount)
						pendingTasks := aws.Int64Value(cluster.PendingTasksCount)
						activeServices := aws.Int64Value(cluster.ActiveServicesCount)

						utils.PrintResult(debug, "", "ecs:ListClusters", fmt.Sprintf("ECS cluster: %s", utils.ColorizeItem(clusterName)), nil)
						utils.PrintResult(debug, "", "ecs:ListClusters", fmt.Sprintf("Status: %s", status), nil)
						utils.PrintResult(debug, "", "ecs:ListClusters", fmt.Sprintf("Running Tasks: %d", runningTasks), nil)
						utils.PrintResult(debug, "", "ecs:ListClusters", fmt.Sprintf("Pending Tasks: %d", pendingTasks), nil)
						utils.PrintResult(debug, "", "ecs:ListClusters", fmt.Sprintf("Active Services: %d", activeServices), nil)

						results = append(results, types.ScanResult{
							ServiceName:  "ECS",
							MethodName:   "ecs:ListClusters",
							ResourceType: "cluster",
							ResourceName: clusterName,
							Details: map[string]interface{}{
								"status":          status,
								"running_tasks":   runningTasks,
								"pending_tasks":   pendingTasks,
								"active_services": activeServices,
							},
							Timestamp: time.Now(),
						})
					}
				}
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
