package fargate

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

var FargateCalls = []types.AWSService{
	{
		Name: "ecs:ListFargateTasks",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			var allTasks []ecstypes.Task
			var lastErr error
			anyRegionSucceeded := false
			for _, region := range types.RegionsFor(ctx) {
				svc := awsv2.NewRegionalClient(cfg, region, ecs.NewFromConfig)

				// Step 1: List all clusters in this region
				var clusterArns []string
				listClustersInput := &ecs.ListClustersInput{}
				regionFailed := false
				for {
					clustersOutput, err := svc.ListClusters(ctx, listClustersInput)
					if err != nil {
						lastErr = err
						regionFailed = true
						break
					}
					clusterArns = append(clusterArns, clustersOutput.ClusterArns...)
					if clustersOutput.NextToken == nil {
						break
					}
					listClustersInput.NextToken = clustersOutput.NextToken
				}
				if regionFailed {
					continue
				}
				anyRegionSucceeded = true

				// Step 2: For each cluster, list Fargate tasks
				for _, clusterArn := range clusterArns {
					var taskArns []string
					listTasksInput := &ecs.ListTasksInput{
						Cluster:    aws.String(clusterArn),
						LaunchType: ecstypes.LaunchTypeFargate,
					}
					for {
						tasksOutput, err := svc.ListTasks(ctx, listTasksInput)
						if err != nil {
							break
						}
						taskArns = append(taskArns, tasksOutput.TaskArns...)
						if tasksOutput.NextToken == nil {
							break
						}
						listTasksInput.NextToken = tasksOutput.NextToken
					}

					// Step 3: Describe tasks to get full details (batch max 100)
					for i := 0; i < len(taskArns); i += 100 {
						end := i + 100
						if end > len(taskArns) {
							end = len(taskArns)
						}
						describeOutput, err := svc.DescribeTasks(ctx, &ecs.DescribeTasksInput{
							Cluster: aws.String(clusterArn),
							Tasks:   taskArns[i:end],
						})
						if err != nil {
							continue
						}
						allTasks = append(allTasks, describeOutput.Tasks...)
					}
				}
			}
			if !anyRegionSucceeded && lastErr != nil {
				return nil, lastErr
			}
			return allTasks, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "ecs:ListFargateTasks", err)
				return []types.ScanResult{
					{
						ServiceName: "Fargate",
						MethodName:  "ecs:ListFargateTasks",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			tasks, ok := output.([]ecstypes.Task)
			if !ok {
				utils.HandleAWSError(debug, "ecs:ListFargateTasks", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			if len(tasks) == 0 {
				utils.PrintAccessGranted(debug, "ecs:ListFargateTasks", "Fargate tasks")
				return results
			}

			for _, task := range tasks {
				taskArn := aws.ToString(task.TaskArn)
				clusterArn := aws.ToString(task.ClusterArn)
				lastStatus := aws.ToString(task.LastStatus)
				desiredStatus := aws.ToString(task.DesiredStatus)
				taskDefArn := aws.ToString(task.TaskDefinitionArn)
				cpu := aws.ToString(task.Cpu)
				memory := aws.ToString(task.Memory)
				launchType := string(task.LaunchType)

				results = append(results, types.ScanResult{
					ServiceName:  "Fargate",
					MethodName:   "ecs:ListFargateTasks",
					ResourceType: "task",
					ResourceName: taskArn,
					Details: map[string]interface{}{
						"ClusterArn":        clusterArn,
						"LastStatus":        lastStatus,
						"DesiredStatus":     desiredStatus,
						"TaskDefinitionArn": taskDefArn,
						"Cpu":               cpu,
						"Memory":            memory,
						"LaunchType":        launchType,
					},
					Timestamp: time.Now(),
				})

				utils.PrintResult(debug, "", "ecs:ListFargateTasks",
					fmt.Sprintf("Found Fargate Task: %s (Status: %s, Desired: %s, CPU: %s, Memory: %s, TaskDef: %s)",
						utils.ColorizeItem(taskArn), lastStatus, desiredStatus, cpu, memory, taskDefArn), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
