package ecs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

// Bounds that keep the ECS scans' API call count predictable.
const (
	// maxClusters caps the clusters whose services are listed, per region.
	maxClusters = 50
	// maxServices caps the services listed per cluster.
	maxServices = 50
	// maxTaskDefPages caps the ListTaskDefinitions pages read per region.
	maxTaskDefPages = 5
	// taskDefPageSize is the ListTaskDefinitions page size (the API maximum).
	taskDefPageSize = 100
)

func init() {
	ECSCalls = append(ECSCalls, listServicesCall, listTaskDefinitionsCall)
}

// arnName returns the last path segment of an ARN.
func arnName(arn string) string {
	if i := strings.LastIndex(arn, "/"); i >= 0 {
		return arn[i+1:]
	}
	return arn
}

type serviceEntry struct {
	Region, ClusterArn, ServiceArn string
}

// depthResult carries entries and the caps that were hit from Call to Process.
type depthResult struct {
	Entries []serviceEntry
	// Truncated lists "<region>" or "<region>/<cluster>" scopes that hit a cap.
	Truncated []string
}

var listServicesCall = types.AWSService{
	Name: "ecs:ListServices",
	CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
		res := &depthResult{}
		for _, region := range types.RegionsFor(ctx) {
			svc := awsv2.NewRegionalClient(cfg, region, ecs.NewFromConfig)
			clusters, err := svc.ListClusters(ctx, &ecs.ListClustersInput{MaxResults: aws.Int32(maxClusters)})
			if err != nil {
				return nil, err
			}
			if clusters.NextToken != nil {
				res.Truncated = append(res.Truncated, region)
			}
			for _, clusterArn := range clusters.ClusterArns {
				services, err := svc.ListServices(ctx, &ecs.ListServicesInput{
					Cluster:    aws.String(clusterArn),
					MaxResults: aws.Int32(maxServices),
				})
				if err != nil {
					return nil, err
				}
				if services.NextToken != nil {
					res.Truncated = append(res.Truncated, region+"/"+arnName(clusterArn))
				}
				for _, serviceArn := range services.ServiceArns {
					res.Entries = append(res.Entries, serviceEntry{region, clusterArn, serviceArn})
				}
			}
		}
		return res, nil
	},
	Process: func(output interface{}, err error, debug bool) []types.ScanResult {
		if err != nil {
			utils.HandleAWSError(debug, "ecs:ListServices", err)
			return []types.ScanResult{{ServiceName: "ECS", MethodName: "ecs:ListServices", Error: err, Timestamp: time.Now()}}
		}
		res, _ := output.(*depthResult)
		if res == nil {
			return nil
		}
		var results []types.ScanResult
		for _, e := range res.Entries {
			cluster, service := arnName(e.ClusterArn), arnName(e.ServiceArn)
			results = append(results, types.ScanResult{
				ServiceName:  "ECS",
				MethodName:   "ecs:ListServices",
				ResourceType: "service",
				ResourceName: cluster + "/" + service,
				Details: map[string]interface{}{
					"region":      e.Region,
					"cluster":     cluster,
					"service":     service,
					"service_arn": e.ServiceArn,
				},
				Timestamp: time.Now(),
			})
			utils.PrintResult(debug, "", "ecs:ListServices", fmt.Sprintf("ECS cluster: %s | service: %s", utils.ColorizeItem(cluster), service), nil)
		}
		for _, scope := range res.Truncated {
			results = append(results, types.ScanResult{
				ServiceName:  "ECS",
				MethodName:   "ecs:ListServices",
				ResourceType: "note",
				ResourceName: "list-truncated",
				Details:      map[string]interface{}{"truncated": true, "scope": scope, "limit_clusters": maxClusters, "limit_services": maxServices},
				Timestamp:    time.Now(),
			})
			utils.PrintResult(debug, "", "ecs:ListServices", fmt.Sprintf("ECS %s: list truncated (limits: %d clusters, %d services per cluster)", scope, maxClusters, maxServices), nil)
		}
		return results
	},
	ModuleName: types.DefaultModuleName,
}

type taskDefEntry struct {
	Region, Arn string
}

type taskDefResult struct {
	Entries   []taskDefEntry
	Truncated []string // regions that hit the page cap
}

var listTaskDefinitionsCall = types.AWSService{
	Name: "ecs:ListTaskDefinitions",
	CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
		res := &taskDefResult{}
		for _, region := range types.RegionsFor(ctx) {
			svc := awsv2.NewRegionalClient(cfg, region, ecs.NewFromConfig)
			var token *string
			for page := 0; ; page++ {
				if page == maxTaskDefPages {
					res.Truncated = append(res.Truncated, region)
					break
				}
				out, err := svc.ListTaskDefinitions(ctx, &ecs.ListTaskDefinitionsInput{
					Status:     "ACTIVE",
					MaxResults: aws.Int32(taskDefPageSize),
					NextToken:  token,
				})
				if err != nil {
					return nil, err
				}
				for _, arn := range out.TaskDefinitionArns {
					res.Entries = append(res.Entries, taskDefEntry{region, arn})
				}
				if out.NextToken == nil {
					break
				}
				token = out.NextToken
			}
		}
		return res, nil
	},
	Process: func(output interface{}, err error, debug bool) []types.ScanResult {
		if err != nil {
			utils.HandleAWSError(debug, "ecs:ListTaskDefinitions", err)
			return []types.ScanResult{{ServiceName: "ECS", MethodName: "ecs:ListTaskDefinitions", Error: err, Timestamp: time.Now()}}
		}
		res, _ := output.(*taskDefResult)
		if res == nil {
			return nil
		}
		var results []types.ScanResult
		for _, e := range res.Entries {
			name := arnName(e.Arn) // "<family>:<revision>"
			results = append(results, types.ScanResult{
				ServiceName:  "ECS",
				MethodName:   "ecs:ListTaskDefinitions",
				ResourceType: "task-definition",
				ResourceName: name,
				Details:      map[string]interface{}{"region": e.Region, "arn": e.Arn},
				Timestamp:    time.Now(),
			})
			utils.PrintResult(debug, "", "ecs:ListTaskDefinitions", fmt.Sprintf("ECS task definition: %s", utils.ColorizeItem(name)), nil)
		}
		for _, region := range res.Truncated {
			results = append(results, types.ScanResult{
				ServiceName:  "ECS",
				MethodName:   "ecs:ListTaskDefinitions",
				ResourceType: "note",
				ResourceName: "list-truncated",
				Details:      map[string]interface{}{"truncated": true, "region": region, "limit": maxTaskDefPages * taskDefPageSize},
				Timestamp:    time.Now(),
			})
			utils.PrintResult(debug, "", "ecs:ListTaskDefinitions", fmt.Sprintf("ECS %s: task definition list truncated at %d", region, maxTaskDefPages*taskDefPageSize), nil)
		}
		return results
	},
	ModuleName: types.DefaultModuleName,
}
