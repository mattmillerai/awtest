package ecs

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

var ECSCalls = []types.AWSService{
	{
		Name: "ecs:ListClusters",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			var allClusters []ecstypes.Cluster

			for _, region := range types.RegionsFor(ctx) {
				svc := awsv2.NewRegionalClient(cfg, region, ecs.NewFromConfig)
				output, err := svc.ListClusters(ctx, &ecs.ListClustersInput{})
				if err != nil {
					return nil, err
				}

				if len(output.ClusterArns) > 0 {
					describeOutput, err := svc.DescribeClusters(ctx, &ecs.DescribeClustersInput{
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
			if clusters, ok := output.([]ecstypes.Cluster); ok {
				if len(clusters) == 0 {
					utils.PrintResult(debug, "", "ecs:ListClusters", "No ECS clusters found.", nil)
				} else {
					for _, cluster := range clusters {
						clusterName := aws.ToString(cluster.ClusterName)
						status := aws.ToString(cluster.Status)
						runningTasks := int64(cluster.RunningTasksCount)
						pendingTasks := int64(cluster.PendingTasksCount)
						activeServices := int64(cluster.ActiveServicesCount)

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
