package fargate

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

var FargateCalls = []types.AWSService{
	{
		Name: "ecs:ListFargateTasks",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			var allTasks []*ecs.Task
			var lastErr error
			anyRegionSucceeded := false
			for _, region := range types.Regions {
				regionSess := sess.Copy(&aws.Config{Region: aws.String(region)})
				svc := ecs.New(regionSess)

				// Step 1: List all clusters in this region
				var clusterArns []*string
				listClustersInput := &ecs.ListClustersInput{}
				regionFailed := false
				for {
					clustersOutput, err := svc.ListClustersWithContext(ctx, listClustersInput)
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
					var taskArns []*string
					listTasksInput := &ecs.ListTasksInput{
						Cluster:    clusterArn,
						LaunchType: aws.String("FARGATE"),
					}
					for {
						tasksOutput, err := svc.ListTasksWithContext(ctx, listTasksInput)
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
						describeOutput, err := svc.DescribeTasksWithContext(ctx, &ecs.DescribeTasksInput{
							Cluster: clusterArn,
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

			tasks, ok := output.([]*ecs.Task)
			if !ok {
				utils.HandleAWSError(debug, "ecs:ListFargateTasks", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			if len(tasks) == 0 {
				utils.PrintAccessGranted(debug, "ecs:ListFargateTasks", "Fargate tasks")
				return results
			}

			for _, task := range tasks {
				taskArn := ""
				if task.TaskArn != nil {
					taskArn = *task.TaskArn
				}

				clusterArn := ""
				if task.ClusterArn != nil {
					clusterArn = *task.ClusterArn
				}

				lastStatus := ""
				if task.LastStatus != nil {
					lastStatus = *task.LastStatus
				}

				desiredStatus := ""
				if task.DesiredStatus != nil {
					desiredStatus = *task.DesiredStatus
				}

				taskDefArn := ""
				if task.TaskDefinitionArn != nil {
					taskDefArn = *task.TaskDefinitionArn
				}

				cpu := ""
				if task.Cpu != nil {
					cpu = *task.Cpu
				}

				memory := ""
				if task.Memory != nil {
					memory = *task.Memory
				}

				launchType := ""
				if task.LaunchType != nil {
					launchType = *task.LaunchType
				}

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
