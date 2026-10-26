package emr

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/emr"
	emrtypes "github.com/aws/aws-sdk-go-v2/service/emr/types"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

type emrCluster struct {
	ClusterId    string
	Name         string
	State        string
	ReleaseLabel string
	Region       string
}

type emrInstanceGroup struct {
	Id                     string
	Name                   string
	ClusterId              string
	ClusterName            string
	InstanceGroupType      string
	InstanceType           string
	RequestedInstanceCount string
	RunningInstanceCount   string
	Market                 string
	State                  string
	Region                 string
}

type emrSecurityConfig struct {
	Name             string
	CreationDateTime string
	Region           string
}

func extractCluster(cluster emrtypes.Cluster, region string) emrCluster {
	id := ""
	if cluster.Id != nil {
		id = *cluster.Id
	}
	name := ""
	if cluster.Name != nil {
		name = *cluster.Name
	}
	state := ""
	if cluster.Status != nil {
		state = string(cluster.Status.State)
	}
	releaseLabel := ""
	if cluster.ReleaseLabel != nil {
		releaseLabel = *cluster.ReleaseLabel
	}
	return emrCluster{
		ClusterId:    id,
		Name:         name,
		State:        state,
		ReleaseLabel: releaseLabel,
		Region:       region,
	}
}

func extractInstanceGroup(ig emrtypes.InstanceGroup, clusterId, clusterName, region string) emrInstanceGroup {
	id := ""
	if ig.Id != nil {
		id = *ig.Id
	}
	name := ""
	if ig.Name != nil {
		name = *ig.Name
	}
	igType := string(ig.InstanceGroupType)
	instanceType := ""
	if ig.InstanceType != nil {
		instanceType = *ig.InstanceType
	}
	requestedCount := ""
	if ig.RequestedInstanceCount != nil {
		requestedCount = fmt.Sprintf("%d", *ig.RequestedInstanceCount)
	}
	runningCount := ""
	if ig.RunningInstanceCount != nil {
		runningCount = fmt.Sprintf("%d", *ig.RunningInstanceCount)
	}
	market := string(ig.Market)
	state := ""
	if ig.Status != nil {
		state = string(ig.Status.State)
	}
	return emrInstanceGroup{
		Id:                     id,
		Name:                   name,
		ClusterId:              clusterId,
		ClusterName:            clusterName,
		InstanceGroupType:      igType,
		InstanceType:           instanceType,
		RequestedInstanceCount: requestedCount,
		RunningInstanceCount:   runningCount,
		Market:                 market,
		State:                  state,
		Region:                 region,
	}
}

func extractSecurityConfig(cfg emrtypes.SecurityConfigurationSummary, region string) emrSecurityConfig {
	name := ""
	if cfg.Name != nil {
		name = *cfg.Name
	}
	creationDateTime := ""
	if cfg.CreationDateTime != nil {
		creationDateTime = cfg.CreationDateTime.Format(time.RFC3339)
	}
	return emrSecurityConfig{
		Name:             name,
		CreationDateTime: creationDateTime,
		Region:           region,
	}
}

var EMRCalls = []types.AWSService{
	{
		Name: "emr:ListClusters",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			var allClusters []emrCluster
			var lastErr error

			for _, region := range types.RegionsFor(ctx) {
				svc := awsv2.NewRegionalClient(cfg, region, emr.NewFromConfig)
				var marker *string
				for {
					input := &emr.ListClustersInput{}
					if marker != nil {
						input.Marker = marker
					}
					output, err := svc.ListClusters(ctx, input)
					if err != nil {
						lastErr = err
						utils.HandleAWSError(false, "emr:ListClusters", err)
						break
					}
					for _, summary := range output.Clusters {
						if summary.Id == nil {
							continue
						}
						descOutput, err := svc.DescribeCluster(ctx, &emr.DescribeClusterInput{
							ClusterId: summary.Id,
						})
						if err != nil {
							utils.HandleAWSError(false, "emr:ListClusters", err)
							continue
						}
						if descOutput.Cluster != nil {
							allClusters = append(allClusters, extractCluster(*descOutput.Cluster, region))
						}
					}
					if output.Marker == nil {
						break
					}
					marker = output.Marker
				}
			}

			if len(allClusters) == 0 && lastErr != nil {
				return nil, lastErr
			}
			return allClusters, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "emr:ListClusters", err)
				return []types.ScanResult{
					{
						ServiceName: "EMR",
						MethodName:  "emr:ListClusters",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			clusters, ok := output.([]emrCluster)
			if !ok {
				utils.HandleAWSError(debug, "emr:ListClusters", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, c := range clusters {
				results = append(results, types.ScanResult{
					ServiceName:  "EMR",
					MethodName:   "emr:ListClusters",
					ResourceType: "cluster",
					ResourceName: c.Name,
					Details: map[string]interface{}{
						"ClusterId":    c.ClusterId,
						"Name":         c.Name,
						"State":        c.State,
						"ReleaseLabel": c.ReleaseLabel,
						"Region":       c.Region,
					},
					Timestamp: time.Now(),
				})

				utils.PrintResult(debug, "", "emr:ListClusters",
					fmt.Sprintf("EMR Cluster: %s (State: %s, Release: %s, Region: %s)", utils.ColorizeItem(c.Name), c.State, c.ReleaseLabel, c.Region), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
	{
		Name: "emr:ListInstanceGroups",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			var allInstanceGroups []emrInstanceGroup
			var lastErr error

			for _, region := range types.RegionsFor(ctx) {
				svc := awsv2.NewRegionalClient(cfg, region, emr.NewFromConfig)

				// Step 1: List all cluster IDs and names
				var clusterIds []struct{ Id, Name string }
				var clusterMarker *string
				for {
					input := &emr.ListClustersInput{}
					if clusterMarker != nil {
						input.Marker = clusterMarker
					}
					output, err := svc.ListClusters(ctx, input)
					if err != nil {
						lastErr = err
						utils.HandleAWSError(false, "emr:ListInstanceGroups", err)
						break
					}
					for _, c := range output.Clusters {
						if c.Id != nil {
							name := ""
							if c.Name != nil {
								name = *c.Name
							}
							clusterIds = append(clusterIds, struct{ Id, Name string }{*c.Id, name})
						}
					}
					if output.Marker == nil {
						break
					}
					clusterMarker = output.Marker
				}

				// Step 2: For each cluster, list instance groups
				for _, cluster := range clusterIds {
					var igMarker *string
					for {
						igInput := &emr.ListInstanceGroupsInput{
							ClusterId: aws.String(cluster.Id),
						}
						if igMarker != nil {
							igInput.Marker = igMarker
						}
						igOutput, err := svc.ListInstanceGroups(ctx, igInput)
						if err != nil {
							utils.HandleAWSError(false, "emr:ListInstanceGroups", err)
							break
						}
						for _, ig := range igOutput.InstanceGroups {
							allInstanceGroups = append(allInstanceGroups, extractInstanceGroup(ig, cluster.Id, cluster.Name, region))
						}
						if igOutput.Marker == nil {
							break
						}
						igMarker = igOutput.Marker
					}
				}
			}

			if len(allInstanceGroups) == 0 && lastErr != nil {
				return nil, lastErr
			}
			return allInstanceGroups, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "emr:ListInstanceGroups", err)
				return []types.ScanResult{
					{
						ServiceName: "EMR",
						MethodName:  "emr:ListInstanceGroups",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			groups, ok := output.([]emrInstanceGroup)
			if !ok {
				utils.HandleAWSError(debug, "emr:ListInstanceGroups", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, ig := range groups {
				results = append(results, types.ScanResult{
					ServiceName:  "EMR",
					MethodName:   "emr:ListInstanceGroups",
					ResourceType: "instance-group",
					ResourceName: ig.Name,
					Details: map[string]interface{}{
						"Id":                     ig.Id,
						"Name":                   ig.Name,
						"ClusterId":              ig.ClusterId,
						"ClusterName":            ig.ClusterName,
						"InstanceGroupType":      ig.InstanceGroupType,
						"InstanceType":           ig.InstanceType,
						"RequestedInstanceCount": ig.RequestedInstanceCount,
						"RunningInstanceCount":   ig.RunningInstanceCount,
						"Market":                 ig.Market,
						"State":                  ig.State,
						"Region":                 ig.Region,
					},
					Timestamp: time.Now(),
				})

				utils.PrintResult(debug, "", "emr:ListInstanceGroups",
					fmt.Sprintf("EMR Instance Group: %s (Cluster: %s, Type: %s, Instance: %s, Count: %s, Region: %s)", utils.ColorizeItem(ig.Name), ig.ClusterName, ig.InstanceGroupType, ig.InstanceType, ig.RequestedInstanceCount, ig.Region), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
	{
		Name: "emr:ListSecurityConfigurations",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			var allConfigs []emrSecurityConfig
			var lastErr error

			for _, region := range types.RegionsFor(ctx) {
				svc := awsv2.NewRegionalClient(cfg, region, emr.NewFromConfig)
				var marker *string
				for {
					input := &emr.ListSecurityConfigurationsInput{}
					if marker != nil {
						input.Marker = marker
					}
					output, err := svc.ListSecurityConfigurations(ctx, input)
					if err != nil {
						lastErr = err
						utils.HandleAWSError(false, "emr:ListSecurityConfigurations", err)
						break
					}
					for _, sc := range output.SecurityConfigurations {
						allConfigs = append(allConfigs, extractSecurityConfig(sc, region))
					}
					if output.Marker == nil {
						break
					}
					marker = output.Marker
				}
			}

			if len(allConfigs) == 0 && lastErr != nil {
				return nil, lastErr
			}
			return allConfigs, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "emr:ListSecurityConfigurations", err)
				return []types.ScanResult{
					{
						ServiceName: "EMR",
						MethodName:  "emr:ListSecurityConfigurations",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			configs, ok := output.([]emrSecurityConfig)
			if !ok {
				utils.HandleAWSError(debug, "emr:ListSecurityConfigurations", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, cfg := range configs {
				results = append(results, types.ScanResult{
					ServiceName:  "EMR",
					MethodName:   "emr:ListSecurityConfigurations",
					ResourceType: "security-configuration",
					ResourceName: cfg.Name,
					Details: map[string]interface{}{
						"Name":             cfg.Name,
						"CreationDateTime": cfg.CreationDateTime,
						"Region":           cfg.Region,
					},
					Timestamp: time.Now(),
				})

				utils.PrintResult(debug, "", "emr:ListSecurityConfigurations",
					fmt.Sprintf("EMR Security Configuration: %s (Created: %s, Region: %s)", utils.ColorizeItem(cfg.Name), cfg.CreationDateTime, cfg.Region), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
