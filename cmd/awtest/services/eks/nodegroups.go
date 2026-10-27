package eks

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

// Bounds that keep eks:ListNodegroups' API call count predictable.
const (
	// maxClusters caps the clusters whose node groups are listed, per region.
	maxClusters = 50
	// maxNodegroups caps the node groups listed per cluster.
	maxNodegroups = 50
)

func init() {
	EKSCalls = append(EKSCalls, listNodegroupsCall)
}

type nodegroupEntry struct {
	Region, Cluster, Nodegroup string
}

type nodegroupResult struct {
	Entries []nodegroupEntry
	// Truncated lists "<region>" or "<region>/<cluster>" scopes that hit a cap.
	Truncated []string
}

var listNodegroupsCall = types.AWSService{
	Name: "eks:ListNodegroups",
	CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
		res := &nodegroupResult{}
		for _, region := range types.RegionsFor(ctx) {
			svc := awsv2.NewRegionalClient(cfg, region, eks.NewFromConfig)
			clusters, err := svc.ListClusters(ctx, &eks.ListClustersInput{MaxResults: aws.Int32(maxClusters)})
			if err != nil {
				return nil, err
			}
			if clusters.NextToken != nil {
				res.Truncated = append(res.Truncated, region)
			}
			for _, cluster := range clusters.Clusters {
				groups, err := svc.ListNodegroups(ctx, &eks.ListNodegroupsInput{
					ClusterName: aws.String(cluster),
					MaxResults:  aws.Int32(maxNodegroups),
				})
				if err != nil {
					return nil, err
				}
				if groups.NextToken != nil {
					res.Truncated = append(res.Truncated, region+"/"+cluster)
				}
				for _, g := range groups.Nodegroups {
					res.Entries = append(res.Entries, nodegroupEntry{region, cluster, g})
				}
			}
		}
		return res, nil
	},
	Process: func(output interface{}, err error, debug bool) []types.ScanResult {
		if err != nil {
			utils.HandleAWSError(debug, "eks:ListNodegroups", err)
			return []types.ScanResult{{ServiceName: "EKS", MethodName: "eks:ListNodegroups", Error: err, Timestamp: time.Now()}}
		}
		res, _ := output.(*nodegroupResult)
		if res == nil {
			return nil
		}
		var results []types.ScanResult
		for _, e := range res.Entries {
			results = append(results, types.ScanResult{
				ServiceName:  "EKS",
				MethodName:   "eks:ListNodegroups",
				ResourceType: "nodegroup",
				ResourceName: e.Cluster + "/" + e.Nodegroup,
				Details:      map[string]interface{}{"region": e.Region, "cluster": e.Cluster, "nodegroup": e.Nodegroup},
				Timestamp:    time.Now(),
			})
			utils.PrintResult(debug, "", "eks:ListNodegroups", fmt.Sprintf("EKS cluster: %s | nodegroup: %s", utils.ColorizeItem(e.Cluster), e.Nodegroup), nil)
		}
		for _, scope := range res.Truncated {
			results = append(results, types.ScanResult{
				ServiceName:  "EKS",
				MethodName:   "eks:ListNodegroups",
				ResourceType: "note",
				ResourceName: "list-truncated",
				Details:      map[string]interface{}{"truncated": true, "scope": scope, "limit_clusters": maxClusters, "limit_nodegroups": maxNodegroups},
				Timestamp:    time.Now(),
			})
			utils.PrintResult(debug, "", "eks:ListNodegroups", fmt.Sprintf("EKS %s: list truncated (limits: %d clusters, %d nodegroups per cluster)", scope, maxClusters, maxNodegroups), nil)
		}
		return results
	},
	ModuleName: types.DefaultModuleName,
}
