package eks

import (
	"context"
	"fmt"
	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/MillerMedia/awtest/cmd/awtest/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/eks"
	"time"
)

var EKSCalls = []types.AWSService{
	{
		Name: "eks:ListClusters",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			var allClusters []*eks.Cluster
			for _, region := range types.Regions {
				regionSess := sess.Copy(&aws.Config{Region: aws.String(region)})
				svc := eks.New(regionSess)
				listOutput, err := svc.ListClustersWithContext(ctx, &eks.ListClustersInput{})
				if err != nil {
					return nil, err
				}
				for _, clusterName := range listOutput.Clusters {
					descOutput, err := svc.DescribeClusterWithContext(ctx, &eks.DescribeClusterInput{
						Name: clusterName,
					})
					if err != nil {
						continue
					}
					allClusters = append(allClusters, descOutput.Cluster)
				}
			}
			return allClusters, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "eks:ListClusters", err)
				return []types.ScanResult{
					{
						ServiceName: "EKS",
						MethodName:  "eks:ListClusters",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			clusters, ok := output.([]*eks.Cluster)
			if !ok {
				utils.HandleAWSError(debug, "eks:ListClusters", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			if len(clusters) == 0 {
				utils.PrintAccessGranted(debug, "eks:ListClusters", "clusters")
				return results
			}

			for _, cluster := range clusters {
				name := ""
				if cluster.Name != nil {
					name = *cluster.Name
				}

				arn := ""
				if cluster.Arn != nil {
					arn = *cluster.Arn
				}

				status := ""
				if cluster.Status != nil {
					status = *cluster.Status
				}

				version := ""
				if cluster.Version != nil {
					version = *cluster.Version
				}

				endpoint := ""
				if cluster.Endpoint != nil {
					endpoint = *cluster.Endpoint
				}

				roleArn := ""
				if cluster.RoleArn != nil {
					roleArn = *cluster.RoleArn
				}

				vpcId := ""
				var subnetCount int
				var sgCount int
				if cluster.ResourcesVpcConfig != nil {
					if cluster.ResourcesVpcConfig.VpcId != nil {
						vpcId = *cluster.ResourcesVpcConfig.VpcId
					}
					subnetCount = len(cluster.ResourcesVpcConfig.SubnetIds)
					sgCount = len(cluster.ResourcesVpcConfig.SecurityGroupIds)
				}

				results = append(results, types.ScanResult{
					ServiceName:  "EKS",
					MethodName:   "eks:ListClusters",
					ResourceType: "cluster",
					ResourceName: name,
					Details: map[string]interface{}{
						"Arn":            arn,
						"Status":         status,
						"Version":        version,
						"Endpoint":       endpoint,
						"RoleArn":        roleArn,
						"VpcId":          vpcId,
						"Subnets":        subnetCount,
						"SecurityGroups": sgCount,
					},
					Timestamp: time.Now(),
				})

				vpcInfo := fmt.Sprintf("VPC: %s, Subnets: %d, SGs: %d", vpcId, subnetCount, sgCount)
				utils.PrintResult(debug, "", "eks:ListClusters",
					fmt.Sprintf("Found EKS Cluster: %s (Status: %s, Version: %s, Endpoint: %s, Role: %s, %s)",
						utils.ColorizeItem(name), status, version, endpoint, roleArn, vpcInfo), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
