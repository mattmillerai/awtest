package neptune

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/neptune"
	nptypes "github.com/aws/aws-sdk-go-v2/service/neptune/types"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

type npCluster struct {
	DBClusterIdentifier              string
	DBClusterArn                     string
	Status                           string
	Engine                           string
	EngineVersion                    string
	Endpoint                         string
	ReaderEndpoint                   string
	Port                             string
	MultiAZ                          string
	StorageEncrypted                 string
	KmsKeyId                         string
	DeletionProtection               string
	IAMDatabaseAuthenticationEnabled string
	DBClusterParameterGroup          string
	ClusterCreateTime                string
	Region                           string
}

type npInstance struct {
	DBInstanceIdentifier    string
	DBInstanceArn           string
	DBInstanceClass         string
	Engine                  string
	EngineVersion           string
	DBInstanceStatus        string
	EndpointAddress         string
	EndpointPort            string
	DBClusterIdentifier     string
	AvailabilityZone        string
	PubliclyAccessible      string
	StorageEncrypted        string
	AutoMinorVersionUpgrade string
	Region                  string
}

type npParameterGroup struct {
	DBClusterParameterGroupName string
	DBClusterParameterGroupArn  string
	Description                 string
	DBParameterGroupFamily      string
	Region                      string
}

func extractCluster(cluster nptypes.DBCluster, region string) npCluster {
	identifier := aws.ToString(cluster.DBClusterIdentifier)
	arn := aws.ToString(cluster.DBClusterArn)
	status := aws.ToString(cluster.Status)
	engine := aws.ToString(cluster.Engine)
	engineVersion := aws.ToString(cluster.EngineVersion)
	endpoint := aws.ToString(cluster.Endpoint)
	readerEndpoint := aws.ToString(cluster.ReaderEndpoint)
	port := ""
	if cluster.Port != nil {
		port = fmt.Sprintf("%d", *cluster.Port)
	}
	multiAZ := ""
	if cluster.MultiAZ != nil {
		multiAZ = fmt.Sprintf("%t", *cluster.MultiAZ)
	}
	storageEncrypted := ""
	if cluster.StorageEncrypted != nil {
		storageEncrypted = fmt.Sprintf("%t", *cluster.StorageEncrypted)
	}
	kmsKeyId := aws.ToString(cluster.KmsKeyId)
	deletionProtection := ""
	if cluster.DeletionProtection != nil {
		deletionProtection = fmt.Sprintf("%t", *cluster.DeletionProtection)
	}
	iamAuth := ""
	if cluster.IAMDatabaseAuthenticationEnabled != nil {
		iamAuth = fmt.Sprintf("%t", *cluster.IAMDatabaseAuthenticationEnabled)
	}
	parameterGroup := aws.ToString(cluster.DBClusterParameterGroup)
	createTime := ""
	if cluster.ClusterCreateTime != nil {
		createTime = cluster.ClusterCreateTime.Format(time.RFC3339)
	}
	return npCluster{
		DBClusterIdentifier:              identifier,
		DBClusterArn:                     arn,
		Status:                           status,
		Engine:                           engine,
		EngineVersion:                    engineVersion,
		Endpoint:                         endpoint,
		ReaderEndpoint:                   readerEndpoint,
		Port:                             port,
		MultiAZ:                          multiAZ,
		StorageEncrypted:                 storageEncrypted,
		KmsKeyId:                         kmsKeyId,
		DeletionProtection:               deletionProtection,
		IAMDatabaseAuthenticationEnabled: iamAuth,
		DBClusterParameterGroup:          parameterGroup,
		ClusterCreateTime:                createTime,
		Region:                           region,
	}
}

func extractInstance(instance nptypes.DBInstance, region string) npInstance {
	identifier := aws.ToString(instance.DBInstanceIdentifier)
	arn := aws.ToString(instance.DBInstanceArn)
	instanceClass := aws.ToString(instance.DBInstanceClass)
	engine := aws.ToString(instance.Engine)
	engineVersion := aws.ToString(instance.EngineVersion)
	status := aws.ToString(instance.DBInstanceStatus)
	endpointAddress := ""
	endpointPort := ""
	if instance.Endpoint != nil {
		endpointAddress = aws.ToString(instance.Endpoint.Address)
		if instance.Endpoint.Port != nil {
			endpointPort = fmt.Sprintf("%d", *instance.Endpoint.Port)
		}
	}
	clusterIdentifier := aws.ToString(instance.DBClusterIdentifier)
	az := aws.ToString(instance.AvailabilityZone)
	publiclyAccessible := ""
	if instance.PubliclyAccessible != nil {
		publiclyAccessible = fmt.Sprintf("%t", *instance.PubliclyAccessible)
	}
	storageEncrypted := ""
	if instance.StorageEncrypted != nil {
		storageEncrypted = fmt.Sprintf("%t", *instance.StorageEncrypted)
	}
	autoUpgrade := ""
	if instance.AutoMinorVersionUpgrade != nil {
		autoUpgrade = fmt.Sprintf("%t", *instance.AutoMinorVersionUpgrade)
	}
	return npInstance{
		DBInstanceIdentifier:    identifier,
		DBInstanceArn:           arn,
		DBInstanceClass:         instanceClass,
		Engine:                  engine,
		EngineVersion:           engineVersion,
		DBInstanceStatus:        status,
		EndpointAddress:         endpointAddress,
		EndpointPort:            endpointPort,
		DBClusterIdentifier:     clusterIdentifier,
		AvailabilityZone:        az,
		PubliclyAccessible:      publiclyAccessible,
		StorageEncrypted:        storageEncrypted,
		AutoMinorVersionUpgrade: autoUpgrade,
		Region:                  region,
	}
}

func extractParameterGroup(pg nptypes.DBClusterParameterGroup, region string) npParameterGroup {
	return npParameterGroup{
		DBClusterParameterGroupName: aws.ToString(pg.DBClusterParameterGroupName),
		DBClusterParameterGroupArn:  aws.ToString(pg.DBClusterParameterGroupArn),
		Description:                 aws.ToString(pg.Description),
		DBParameterGroupFamily:      aws.ToString(pg.DBParameterGroupFamily),
		Region:                      region,
	}
}

var NeptuneCalls = []types.AWSService{
	{
		Name: "neptune:DescribeDBClusters",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			var allClusters []npCluster
			var lastErr error

			for _, region := range types.RegionsFor(ctx) {
				svc := awsv2.NewRegionalClient(cfg, region, neptune.NewFromConfig)
				var marker *string
				for {
					input := &neptune.DescribeDBClustersInput{}
					if marker != nil {
						input.Marker = marker
					}
					output, err := svc.DescribeDBClusters(ctx, input)
					if err != nil {
						lastErr = err
						utils.HandleAWSError(false, "neptune:DescribeDBClusters", err)
						break
					}
					for _, cluster := range output.DBClusters {
						allClusters = append(allClusters, extractCluster(cluster, region))
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
				utils.HandleAWSError(debug, "neptune:DescribeDBClusters", err)
				return []types.ScanResult{
					{
						ServiceName: "Neptune",
						MethodName:  "neptune:DescribeDBClusters",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			clusters, ok := output.([]npCluster)
			if !ok {
				utils.HandleAWSError(debug, "neptune:DescribeDBClusters", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, c := range clusters {
				results = append(results, types.ScanResult{
					ServiceName:  "Neptune",
					MethodName:   "neptune:DescribeDBClusters",
					ResourceType: "db-cluster",
					ResourceName: c.DBClusterIdentifier,
					Details: map[string]interface{}{
						"DBClusterIdentifier":              c.DBClusterIdentifier,
						"DBClusterArn":                     c.DBClusterArn,
						"Status":                           c.Status,
						"Engine":                           c.Engine,
						"EngineVersion":                    c.EngineVersion,
						"Endpoint":                         c.Endpoint,
						"ReaderEndpoint":                   c.ReaderEndpoint,
						"Port":                             c.Port,
						"MultiAZ":                          c.MultiAZ,
						"StorageEncrypted":                 c.StorageEncrypted,
						"KmsKeyId":                         c.KmsKeyId,
						"DeletionProtection":               c.DeletionProtection,
						"IAMDatabaseAuthenticationEnabled": c.IAMDatabaseAuthenticationEnabled,
						"DBClusterParameterGroup":          c.DBClusterParameterGroup,
						"ClusterCreateTime":                c.ClusterCreateTime,
						"Region":                           c.Region,
					},
					Timestamp: time.Now(),
				})

				utils.PrintResult(debug, "", "neptune:DescribeDBClusters",
					fmt.Sprintf("Neptune DB Cluster: %s (Status: %s, Engine: %s %s, Region: %s)", utils.ColorizeItem(c.DBClusterIdentifier), c.Status, c.Engine, c.EngineVersion, c.Region), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
	{
		Name: "neptune:DescribeDBInstances",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			var allInstances []npInstance
			var lastErr error

			for _, region := range types.RegionsFor(ctx) {
				svc := awsv2.NewRegionalClient(cfg, region, neptune.NewFromConfig)
				var marker *string
				for {
					input := &neptune.DescribeDBInstancesInput{}
					if marker != nil {
						input.Marker = marker
					}
					output, err := svc.DescribeDBInstances(ctx, input)
					if err != nil {
						lastErr = err
						utils.HandleAWSError(false, "neptune:DescribeDBInstances", err)
						break
					}
					for _, instance := range output.DBInstances {
						allInstances = append(allInstances, extractInstance(instance, region))
					}
					if output.Marker == nil {
						break
					}
					marker = output.Marker
				}
			}

			if len(allInstances) == 0 && lastErr != nil {
				return nil, lastErr
			}
			return allInstances, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "neptune:DescribeDBInstances", err)
				return []types.ScanResult{
					{
						ServiceName: "Neptune",
						MethodName:  "neptune:DescribeDBInstances",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			instances, ok := output.([]npInstance)
			if !ok {
				utils.HandleAWSError(debug, "neptune:DescribeDBInstances", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, i := range instances {
				results = append(results, types.ScanResult{
					ServiceName:  "Neptune",
					MethodName:   "neptune:DescribeDBInstances",
					ResourceType: "db-instance",
					ResourceName: i.DBInstanceIdentifier,
					Details: map[string]interface{}{
						"DBInstanceIdentifier":    i.DBInstanceIdentifier,
						"DBInstanceArn":           i.DBInstanceArn,
						"DBInstanceClass":         i.DBInstanceClass,
						"Engine":                  i.Engine,
						"EngineVersion":           i.EngineVersion,
						"DBInstanceStatus":        i.DBInstanceStatus,
						"EndpointAddress":         i.EndpointAddress,
						"EndpointPort":            i.EndpointPort,
						"DBClusterIdentifier":     i.DBClusterIdentifier,
						"AvailabilityZone":        i.AvailabilityZone,
						"PubliclyAccessible":      i.PubliclyAccessible,
						"StorageEncrypted":        i.StorageEncrypted,
						"AutoMinorVersionUpgrade": i.AutoMinorVersionUpgrade,
						"Region":                  i.Region,
					},
					Timestamp: time.Now(),
				})

				utils.PrintResult(debug, "", "neptune:DescribeDBInstances",
					fmt.Sprintf("Neptune DB Instance: %s (Status: %s, Class: %s, Cluster: %s, Region: %s)", utils.ColorizeItem(i.DBInstanceIdentifier), i.DBInstanceStatus, i.DBInstanceClass, i.DBClusterIdentifier, i.Region), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
	{
		Name: "neptune:DescribeDBClusterParameterGroups",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			var allGroups []npParameterGroup
			var lastErr error

			for _, region := range types.RegionsFor(ctx) {
				svc := awsv2.NewRegionalClient(cfg, region, neptune.NewFromConfig)
				var marker *string
				for {
					input := &neptune.DescribeDBClusterParameterGroupsInput{}
					if marker != nil {
						input.Marker = marker
					}
					output, err := svc.DescribeDBClusterParameterGroups(ctx, input)
					if err != nil {
						lastErr = err
						utils.HandleAWSError(false, "neptune:DescribeDBClusterParameterGroups", err)
						break
					}
					for _, pg := range output.DBClusterParameterGroups {
						allGroups = append(allGroups, extractParameterGroup(pg, region))
					}
					if output.Marker == nil {
						break
					}
					marker = output.Marker
				}
			}

			if len(allGroups) == 0 && lastErr != nil {
				return nil, lastErr
			}
			return allGroups, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "neptune:DescribeDBClusterParameterGroups", err)
				return []types.ScanResult{
					{
						ServiceName: "Neptune",
						MethodName:  "neptune:DescribeDBClusterParameterGroups",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			groups, ok := output.([]npParameterGroup)
			if !ok {
				utils.HandleAWSError(debug, "neptune:DescribeDBClusterParameterGroups", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, g := range groups {
				results = append(results, types.ScanResult{
					ServiceName:  "Neptune",
					MethodName:   "neptune:DescribeDBClusterParameterGroups",
					ResourceType: "db-cluster-parameter-group",
					ResourceName: g.DBClusterParameterGroupName,
					Details: map[string]interface{}{
						"DBClusterParameterGroupName": g.DBClusterParameterGroupName,
						"DBClusterParameterGroupArn":  g.DBClusterParameterGroupArn,
						"Description":                 g.Description,
						"DBParameterGroupFamily":      g.DBParameterGroupFamily,
						"Region":                      g.Region,
					},
					Timestamp: time.Now(),
				})

				utils.PrintResult(debug, "", "neptune:DescribeDBClusterParameterGroups",
					fmt.Sprintf("Neptune Cluster Parameter Group: %s (Family: %s, Region: %s)", utils.ColorizeItem(g.DBClusterParameterGroupName), g.DBParameterGroupFamily, g.Region), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
