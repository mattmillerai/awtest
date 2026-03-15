package elasticache

import (
	"context"
	"fmt"
	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/MillerMedia/awtest/cmd/awtest/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/elasticache"
	"time"
)

var ElastiCacheCalls = []types.AWSService{
	{
		Name: "elasticache:DescribeCacheClusters",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			var allClusters []*elasticache.CacheCluster
			var lastErr error
			anyRegionSucceeded := false
			for _, region := range types.Regions {
				regionSess := sess.Copy(&aws.Config{Region: aws.String(region)})
				svc := elasticache.New(regionSess)
				input := &elasticache.DescribeCacheClustersInput{
					ShowCacheNodeInfo: aws.Bool(true),
				}
				regionFailed := false
				for {
					output, err := svc.DescribeCacheClustersWithContext(ctx, input)
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

			clusters, ok := output.([]*elasticache.CacheCluster)
			if !ok {
				utils.HandleAWSError(debug, "elasticache:DescribeCacheClusters", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			if len(clusters) == 0 {
				utils.PrintAccessGranted(debug, "elasticache:DescribeCacheClusters", "clusters")
				return results
			}

			for _, cluster := range clusters {
				clusterId := ""
				if cluster.CacheClusterId != nil {
					clusterId = *cluster.CacheClusterId
				}

				engine := ""
				if cluster.Engine != nil {
					engine = *cluster.Engine
				}

				engineVersion := ""
				if cluster.EngineVersion != nil {
					engineVersion = *cluster.EngineVersion
				}

				nodeType := ""
				if cluster.CacheNodeType != nil {
					nodeType = *cluster.CacheNodeType
				}

				status := ""
				if cluster.CacheClusterStatus != nil {
					status = *cluster.CacheClusterStatus
				}

				var numNodes int64
				if cluster.NumCacheNodes != nil {
					numNodes = *cluster.NumCacheNodes
				}

				az := ""
				if cluster.PreferredAvailabilityZone != nil {
					az = *cluster.PreferredAvailabilityZone
				}

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
