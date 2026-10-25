package elasticache

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	ectypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

var ElastiCacheCalls = []types.AWSService{
	{
		Name: "elasticache:DescribeCacheClusters",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			var allClusters []ectypes.CacheCluster
			var lastErr error
			anyRegionSucceeded := false
			for _, region := range types.RegionsFor(ctx) {
				svc := awsv2.NewRegionalClient(cfg, region, elasticache.NewFromConfig)
				input := &elasticache.DescribeCacheClustersInput{
					ShowCacheNodeInfo: aws.Bool(true),
				}
				regionFailed := false
				for {
					output, err := svc.DescribeCacheClusters(ctx, input)
					if err != nil {
						lastErr = err
						regionFailed = true
						break
					}
					allClusters = append(allClusters, output.CacheClusters...)
					if output.Marker == nil {
						break
					}
					input.Marker = output.Marker
				}
				if !regionFailed {
					anyRegionSucceeded = true
				}
			}
			if !anyRegionSucceeded && lastErr != nil {
				return nil, lastErr
			}
			return allClusters, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "elasticache:DescribeCacheClusters", err)
				return []types.ScanResult{
					{
						ServiceName: "ElastiCache",
						MethodName:  "elasticache:DescribeCacheClusters",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			clusters, ok := output.([]ectypes.CacheCluster)
			if !ok {
				utils.HandleAWSError(debug, "elasticache:DescribeCacheClusters", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			if len(clusters) == 0 {
				utils.PrintAccessGranted(debug, "elasticache:DescribeCacheClusters", "clusters")
				return results
			}

			for _, cluster := range clusters {
				clusterId := aws.ToString(cluster.CacheClusterId)
				engine := aws.ToString(cluster.Engine)
				engineVersion := aws.ToString(cluster.EngineVersion)
				nodeType := aws.ToString(cluster.CacheNodeType)
				status := aws.ToString(cluster.CacheClusterStatus)
				numNodes := int64(aws.ToInt32(cluster.NumCacheNodes))
				az := aws.ToString(cluster.PreferredAvailabilityZone)

				results = append(results, types.ScanResult{
					ServiceName:  "ElastiCache",
					MethodName:   "elasticache:DescribeCacheClusters",
					ResourceType: "cluster",
					ResourceName: clusterId,
					Details: map[string]interface{}{
						"Engine":                    engine,
						"EngineVersion":             engineVersion,
						"CacheNodeType":             nodeType,
						"CacheClusterStatus":        status,
						"NumCacheNodes":             numNodes,
						"PreferredAvailabilityZone": az,
					},
					Timestamp: time.Now(),
				})

				utils.PrintResult(debug, "", "elasticache:DescribeCacheClusters",
					fmt.Sprintf("Found ElastiCache Cluster: %s (Engine: %s %s, Type: %s, Status: %s, Nodes: %d, AZ: %s)",
						utils.ColorizeItem(clusterId), engine, engineVersion, nodeType, status, numNodes, az), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
