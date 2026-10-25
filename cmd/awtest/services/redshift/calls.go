package redshift

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/redshift"
	redshifttypes "github.com/aws/aws-sdk-go-v2/service/redshift/types"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

var RedshiftCalls = []types.AWSService{
	{
		Name: "redshift:DescribeClusters",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			var allClusters []redshifttypes.Cluster
			var lastErr error
			anyRegionSucceeded := false
			for _, region := range types.RegionsFor(ctx) {
				svc := awsv2.NewRegionalClient(cfg, region, redshift.NewFromConfig)
				input := &redshift.DescribeClustersInput{}
				regionFailed := false
				for {
					output, err := svc.DescribeClusters(ctx, input)
					if err != nil {
						lastErr = err
						regionFailed = true
						break
					}
					allClusters = append(allClusters, output.Clusters...)
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
				utils.HandleAWSError(debug, "redshift:DescribeClusters", err)
				return []types.ScanResult{
					{
						ServiceName: "Redshift",
						MethodName:  "redshift:DescribeClusters",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			clusters, ok := output.([]redshifttypes.Cluster)
			if !ok {
				utils.HandleAWSError(debug, "redshift:DescribeClusters", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			if len(clusters) == 0 {
				utils.PrintAccessGranted(debug, "redshift:DescribeClusters", "Redshift clusters")
				return results
			}

			for _, cluster := range clusters {
				clusterId := ""
				if cluster.ClusterIdentifier != nil {
					clusterId = *cluster.ClusterIdentifier
				}

				nodeType := ""
				if cluster.NodeType != nil {
					nodeType = *cluster.NodeType
				}

				status := ""
				if cluster.ClusterStatus != nil {
					status = *cluster.ClusterStatus
				}

				masterUser := ""
				if cluster.MasterUsername != nil {
					masterUser = *cluster.MasterUsername
				}

				dbName := ""
				if cluster.DBName != nil {
					dbName = *cluster.DBName
				}

				endpoint := ""
				if cluster.Endpoint != nil {
					addr := ""
					if cluster.Endpoint.Address != nil {
						addr = *cluster.Endpoint.Address
					}
					if cluster.Endpoint.Port != nil {
						endpoint = fmt.Sprintf("%s:%d", addr, int64(*cluster.Endpoint.Port))
					} else {
						endpoint = addr
					}
				}

				encrypted := aws.ToBool(cluster.Encrypted)

				numNodes := int64(aws.ToInt32(cluster.NumberOfNodes))

				results = append(results, types.ScanResult{
					ServiceName:  "Redshift",
					MethodName:   "redshift:DescribeClusters",
					ResourceType: "cluster",
					ResourceName: clusterId,
					Details: map[string]interface{}{
						"NodeType":       nodeType,
						"ClusterStatus":  status,
						"MasterUsername": masterUser,
						"DBName":         dbName,
						"Endpoint":       endpoint,
						"Encrypted":      encrypted,
						"NumberOfNodes":  numNodes,
					},
					Timestamp: time.Now(),
				})

				utils.PrintResult(debug, "", "redshift:DescribeClusters",
					fmt.Sprintf("Found Redshift Cluster: %s (Type: %s, Status: %s, User: %s, DB: %s, Endpoint: %s, Encrypted: %v, Nodes: %d)",
						utils.ColorizeItem(clusterId), nodeType, status, masterUser, dbName, endpoint, encrypted, numNodes), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
