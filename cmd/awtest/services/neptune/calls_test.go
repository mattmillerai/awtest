package neptune

import (
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/neptune"
)

func TestDescribeDBClustersProcess(t *testing.T) {
	process := NeptuneCalls[0].Process

	tests := []struct {
		name                     string
		output                   interface{}
		err                      error
		wantLen                  int
		wantError                bool
		wantResourceName         string
		wantDBClusterIdentifier  string
		wantDBClusterArn         string
		wantStatus               string
		wantEngine               string
		wantEngineVersion        string
		wantEndpoint             string
		wantReaderEndpoint       string
		wantPort                 string
		wantMultiAZ              string
		wantStorageEncrypted     string
		wantKmsKeyId             string
		wantDeletionProtection   string
		wantIAMAuth              string
		wantParameterGroup       string
		wantClusterCreateTime    string
		wantRegion               string
	}{
		{
			name: "valid clusters with full details",
			output: []npCluster{
				{
					DBClusterIdentifier:              "my-neptune-cluster",
					DBClusterArn:                     "arn:aws:rds:us-east-1:123456789012:cluster:my-neptune-cluster",
					Status:                           "available",
					Engine:                           "neptune",
					EngineVersion:                    "1.2.0.2",
					Endpoint:                         "my-neptune-cluster.cluster-abc123.us-east-1.neptune.amazonaws.com",
					ReaderEndpoint:                   "my-neptune-cluster.cluster-ro-abc123.us-east-1.neptune.amazonaws.com",
					Port:                             "8182",
					MultiAZ:                          "true",
					StorageEncrypted:                 "true",
					KmsKeyId:                         "arn:aws:kms:us-east-1:123456789012:key/12345678-1234-1234-1234-123456789012",
					DeletionProtection:               "true",
					IAMDatabaseAuthenticationEnabled: "true",
					DBClusterParameterGroup:          "default.neptune1",
					ClusterCreateTime:                "2026-01-15T10:00:00Z",
					Region:                           "us-east-1",
				},
				{
					DBClusterIdentifier:              "test-cluster",
					DBClusterArn:                     "arn:aws:rds:us-west-2:123456789012:cluster:test-cluster",
					Status:                           "creating",
					Engine:                           "neptune",
					EngineVersion:                    "1.3.0.0",
					Endpoint:                         "test-cluster.cluster-def456.us-west-2.neptune.amazonaws.com",
					ReaderEndpoint:                   "test-cluster.cluster-ro-def456.us-west-2.neptune.amazonaws.com",
					Port:                             "8182",
					MultiAZ:                          "false",
					StorageEncrypted:                 "false",
					KmsKeyId:                         "",
					DeletionProtection:               "false",
					IAMDatabaseAuthenticationEnabled: "false",
					DBClusterParameterGroup:          "custom-params",
					ClusterCreateTime:                "2026-02-20T14:00:00Z",
					Region:                           "us-west-2",
				},
			},
			wantLen:                  2,
			wantResourceName:         "my-neptune-cluster",
			wantDBClusterIdentifier:  "my-neptune-cluster",
			wantDBClusterArn:         "arn:aws:rds:us-east-1:123456789012:cluster:my-neptune-cluster",
			wantStatus:               "available",
			wantEngine:               "neptune",
			wantEngineVersion:        "1.2.0.2",
			wantEndpoint:             "my-neptune-cluster.cluster-abc123.us-east-1.neptune.amazonaws.com",
			wantReaderEndpoint:       "my-neptune-cluster.cluster-ro-abc123.us-east-1.neptune.amazonaws.com",
			wantPort:                 "8182",
			wantMultiAZ:              "true",
			wantStorageEncrypted:     "true",
			wantKmsKeyId:             "arn:aws:kms:us-east-1:123456789012:key/12345678-1234-1234-1234-123456789012",
			wantDeletionProtection:   "true",
			wantIAMAuth:              "true",
			wantParameterGroup:       "default.neptune1",
			wantClusterCreateTime:    "2026-01-15T10:00:00Z",
			wantRegion:               "us-east-1",
		},
		{
			name:    "empty results",
			output:  []npCluster{},
			wantLen: 0,
		},
		{
			name:      "access denied error",
			output:    nil,
			err:       fmt.Errorf("AccessDeniedException: User is not authorized"),
			wantLen:   1,
			wantError: true,
		},
		{
			name: "nil-safe fields (empty strings)",
			output: []npCluster{
				{
					DBClusterIdentifier:              "",
					DBClusterArn:                     "",
					Status:                           "",
					Engine:                           "",
					EngineVersion:                    "",
					Endpoint:                         "",
					ReaderEndpoint:                   "",
					Port:                             "",
					MultiAZ:                          "",
					StorageEncrypted:                 "",
					KmsKeyId:                         "",
					DeletionProtection:               "",
					IAMDatabaseAuthenticationEnabled: "",
					DBClusterParameterGroup:          "",
					ClusterCreateTime:                "",
					Region:                           "",
				},
			},
			wantLen:                  1,
			wantResourceName:         "",
			wantDBClusterIdentifier:  "",
			wantDBClusterArn:         "",
			wantStatus:               "",
			wantEngine:               "",
			wantEngineVersion:        "",
			wantEndpoint:             "",
			wantReaderEndpoint:       "",
			wantPort:                 "",
			wantMultiAZ:              "",
			wantStorageEncrypted:     "",
			wantKmsKeyId:             "",
			wantDeletionProtection:   "",
			wantIAMAuth:              "",
			wantParameterGroup:       "",
			wantClusterCreateTime:    "",
			wantRegion:               "",
		},
		{
			name:    "type assertion failure",
			output:  "invalid type",
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := process(tt.output, tt.err, false)

			if len(results) != tt.wantLen {
				t.Fatalf("expected %d results, got %d", tt.wantLen, len(results))
			}

			if tt.wantError {
				if results[0].Error == nil {
					t.Error("expected error in result, got nil")
				}
				if results[0].ServiceName != "Neptune" {
					t.Errorf("expected ServiceName 'Neptune', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "neptune:DescribeDBClusters" {
					t.Errorf("expected MethodName 'neptune:DescribeDBClusters', got '%s'", results[0].MethodName)
				}
				return
			}

			if tt.wantLen > 0 {
				if results[0].ServiceName != "Neptune" {
					t.Errorf("expected ServiceName 'Neptune', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "neptune:DescribeDBClusters" {
					t.Errorf("expected MethodName 'neptune:DescribeDBClusters', got '%s'", results[0].MethodName)
				}
				if results[0].ResourceType != "db-cluster" {
					t.Errorf("expected ResourceType 'db-cluster', got '%s'", results[0].ResourceType)
				}
				if results[0].ResourceName != tt.wantResourceName {
					t.Errorf("expected ResourceName '%s', got '%s'", tt.wantResourceName, results[0].ResourceName)
				}
				if v, ok := results[0].Details["DBClusterIdentifier"].(string); ok {
					if v != tt.wantDBClusterIdentifier {
						t.Errorf("expected DBClusterIdentifier '%s', got '%s'", tt.wantDBClusterIdentifier, v)
					}
				} else if tt.wantDBClusterIdentifier != "" {
					t.Errorf("expected DBClusterIdentifier in Details, got none")
				}
				if v, ok := results[0].Details["DBClusterArn"].(string); ok {
					if v != tt.wantDBClusterArn {
						t.Errorf("expected DBClusterArn '%s', got '%s'", tt.wantDBClusterArn, v)
					}
				} else if tt.wantDBClusterArn != "" {
					t.Errorf("expected DBClusterArn in Details, got none")
				}
				if v, ok := results[0].Details["Status"].(string); ok {
					if v != tt.wantStatus {
						t.Errorf("expected Status '%s', got '%s'", tt.wantStatus, v)
					}
				} else if tt.wantStatus != "" {
					t.Errorf("expected Status in Details, got none")
				}
				if v, ok := results[0].Details["Engine"].(string); ok {
					if v != tt.wantEngine {
						t.Errorf("expected Engine '%s', got '%s'", tt.wantEngine, v)
					}
				} else if tt.wantEngine != "" {
					t.Errorf("expected Engine in Details, got none")
				}
				if v, ok := results[0].Details["EngineVersion"].(string); ok {
					if v != tt.wantEngineVersion {
						t.Errorf("expected EngineVersion '%s', got '%s'", tt.wantEngineVersion, v)
					}
				} else if tt.wantEngineVersion != "" {
					t.Errorf("expected EngineVersion in Details, got none")
				}
				if v, ok := results[0].Details["Endpoint"].(string); ok {
					if v != tt.wantEndpoint {
						t.Errorf("expected Endpoint '%s', got '%s'", tt.wantEndpoint, v)
					}
				} else if tt.wantEndpoint != "" {
					t.Errorf("expected Endpoint in Details, got none")
				}
				if v, ok := results[0].Details["ReaderEndpoint"].(string); ok {
					if v != tt.wantReaderEndpoint {
						t.Errorf("expected ReaderEndpoint '%s', got '%s'", tt.wantReaderEndpoint, v)
					}
				} else if tt.wantReaderEndpoint != "" {
					t.Errorf("expected ReaderEndpoint in Details, got none")
				}
				if v, ok := results[0].Details["Port"].(string); ok {
					if v != tt.wantPort {
						t.Errorf("expected Port '%s', got '%s'", tt.wantPort, v)
					}
				} else if tt.wantPort != "" {
					t.Errorf("expected Port in Details, got none")
				}
				if v, ok := results[0].Details["MultiAZ"].(string); ok {
					if v != tt.wantMultiAZ {
						t.Errorf("expected MultiAZ '%s', got '%s'", tt.wantMultiAZ, v)
					}
				} else if tt.wantMultiAZ != "" {
					t.Errorf("expected MultiAZ in Details, got none")
				}
				if v, ok := results[0].Details["StorageEncrypted"].(string); ok {
					if v != tt.wantStorageEncrypted {
						t.Errorf("expected StorageEncrypted '%s', got '%s'", tt.wantStorageEncrypted, v)
					}
				} else if tt.wantStorageEncrypted != "" {
					t.Errorf("expected StorageEncrypted in Details, got none")
				}
				if v, ok := results[0].Details["KmsKeyId"].(string); ok {
					if v != tt.wantKmsKeyId {
						t.Errorf("expected KmsKeyId '%s', got '%s'", tt.wantKmsKeyId, v)
					}
				} else if tt.wantKmsKeyId != "" {
					t.Errorf("expected KmsKeyId in Details, got none")
				}
				if v, ok := results[0].Details["DeletionProtection"].(string); ok {
					if v != tt.wantDeletionProtection {
						t.Errorf("expected DeletionProtection '%s', got '%s'", tt.wantDeletionProtection, v)
					}
				} else if tt.wantDeletionProtection != "" {
					t.Errorf("expected DeletionProtection in Details, got none")
				}
				if v, ok := results[0].Details["IAMDatabaseAuthenticationEnabled"].(string); ok {
					if v != tt.wantIAMAuth {
						t.Errorf("expected IAMDatabaseAuthenticationEnabled '%s', got '%s'", tt.wantIAMAuth, v)
					}
				} else if tt.wantIAMAuth != "" {
					t.Errorf("expected IAMDatabaseAuthenticationEnabled in Details, got none")
				}
				if v, ok := results[0].Details["DBClusterParameterGroup"].(string); ok {
					if v != tt.wantParameterGroup {
						t.Errorf("expected DBClusterParameterGroup '%s', got '%s'", tt.wantParameterGroup, v)
					}
				} else if tt.wantParameterGroup != "" {
					t.Errorf("expected DBClusterParameterGroup in Details, got none")
				}
				if v, ok := results[0].Details["ClusterCreateTime"].(string); ok {
					if v != tt.wantClusterCreateTime {
						t.Errorf("expected ClusterCreateTime '%s', got '%s'", tt.wantClusterCreateTime, v)
					}
				} else if tt.wantClusterCreateTime != "" {
					t.Errorf("expected ClusterCreateTime in Details, got none")
				}
				if v, ok := results[0].Details["Region"].(string); ok {
					if v != tt.wantRegion {
						t.Errorf("expected Region '%s', got '%s'", tt.wantRegion, v)
					}
				} else if tt.wantRegion != "" {
					t.Errorf("expected Region in Details, got none")
				}
			}
		})
	}
}

func TestDescribeDBInstancesProcess(t *testing.T) {
	process := NeptuneCalls[1].Process

	tests := []struct {
		name                    string
		output                  interface{}
		err                     error
		wantLen                 int
		wantError               bool
		wantResourceName        string
		wantDBInstanceIdentifier string
		wantDBInstanceArn       string
		wantDBInstanceClass     string
		wantEngine              string
		wantEngineVersion       string
		wantDBInstanceStatus    string
		wantEndpointAddress     string
		wantEndpointPort        string
		wantDBClusterIdentifier string
		wantAvailabilityZone    string
		wantPubliclyAccessible  string
		wantStorageEncrypted    string
		wantAutoUpgrade         string
		wantRegion              string
	}{
		{
			name: "valid instances with full details",
			output: []npInstance{
				{
					DBInstanceIdentifier:    "my-neptune-instance-1",
					DBInstanceArn:           "arn:aws:rds:us-east-1:123456789012:db:my-neptune-instance-1",
					DBInstanceClass:         "db.r5.large",
					Engine:                  "neptune",
					EngineVersion:           "1.2.0.2",
					DBInstanceStatus:        "available",
					EndpointAddress:         "my-neptune-instance-1.abc123.us-east-1.neptune.amazonaws.com",
					EndpointPort:            "8182",
					DBClusterIdentifier:     "my-neptune-cluster",
					AvailabilityZone:        "us-east-1a",
					PubliclyAccessible:      "false",
					StorageEncrypted:        "true",
					AutoMinorVersionUpgrade: "true",
					Region:                  "us-east-1",
				},
				{
					DBInstanceIdentifier:    "test-instance-1",
					DBInstanceArn:           "arn:aws:rds:us-west-2:123456789012:db:test-instance-1",
					DBInstanceClass:         "db.t3.medium",
					Engine:                  "neptune",
					EngineVersion:           "1.3.0.0",
					DBInstanceStatus:        "creating",
					EndpointAddress:         "test-instance-1.def456.us-west-2.neptune.amazonaws.com",
					EndpointPort:            "8182",
					DBClusterIdentifier:     "test-cluster",
					AvailabilityZone:        "us-west-2b",
					PubliclyAccessible:      "true",
					StorageEncrypted:        "false",
					AutoMinorVersionUpgrade: "false",
					Region:                  "us-west-2",
				},
			},
			wantLen:                  2,
			wantResourceName:         "my-neptune-instance-1",
			wantDBInstanceIdentifier: "my-neptune-instance-1",
			wantDBInstanceArn:        "arn:aws:rds:us-east-1:123456789012:db:my-neptune-instance-1",
			wantDBInstanceClass:      "db.r5.large",
			wantEngine:               "neptune",
			wantEngineVersion:        "1.2.0.2",
			wantDBInstanceStatus:     "available",
			wantEndpointAddress:      "my-neptune-instance-1.abc123.us-east-1.neptune.amazonaws.com",
			wantEndpointPort:         "8182",
			wantDBClusterIdentifier:  "my-neptune-cluster",
			wantAvailabilityZone:     "us-east-1a",
			wantPubliclyAccessible:   "false",
			wantStorageEncrypted:     "true",
			wantAutoUpgrade:          "true",
			wantRegion:               "us-east-1",
		},
		{
			name:    "empty results",
			output:  []npInstance{},
			wantLen: 0,
		},
		{
			name:      "access denied error",
			output:    nil,
			err:       fmt.Errorf("AccessDeniedException: User is not authorized"),
			wantLen:   1,
			wantError: true,
		},
		{
			name: "nil-safe fields (empty strings)",
			output: []npInstance{
				{
					DBInstanceIdentifier:    "",
					DBInstanceArn:           "",
					DBInstanceClass:         "",
					Engine:                  "",
					EngineVersion:           "",
					DBInstanceStatus:        "",
					EndpointAddress:         "",
					EndpointPort:            "",
					DBClusterIdentifier:     "",
					AvailabilityZone:        "",
					PubliclyAccessible:      "",
					StorageEncrypted:        "",
					AutoMinorVersionUpgrade: "",
					Region:                  "",
				},
			},
			wantLen:                  1,
			wantResourceName:         "",
			wantDBInstanceIdentifier: "",
			wantDBInstanceArn:        "",
			wantDBInstanceClass:      "",
			wantEngine:               "",
			wantEngineVersion:        "",
			wantDBInstanceStatus:     "",
			wantEndpointAddress:      "",
			wantEndpointPort:         "",
			wantDBClusterIdentifier:  "",
			wantAvailabilityZone:     "",
			wantPubliclyAccessible:   "",
			wantStorageEncrypted:     "",
			wantAutoUpgrade:          "",
			wantRegion:               "",
		},
		{
			name:    "type assertion failure",
			output:  "invalid type",
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := process(tt.output, tt.err, false)

			if len(results) != tt.wantLen {
				t.Fatalf("expected %d results, got %d", tt.wantLen, len(results))
			}

			if tt.wantError {
				if results[0].Error == nil {
					t.Error("expected error in result, got nil")
				}
				if results[0].ServiceName != "Neptune" {
					t.Errorf("expected ServiceName 'Neptune', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "neptune:DescribeDBInstances" {
					t.Errorf("expected MethodName 'neptune:DescribeDBInstances', got '%s'", results[0].MethodName)
				}
				return
			}

			if tt.wantLen > 0 {
				if results[0].ServiceName != "Neptune" {
					t.Errorf("expected ServiceName 'Neptune', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "neptune:DescribeDBInstances" {
					t.Errorf("expected MethodName 'neptune:DescribeDBInstances', got '%s'", results[0].MethodName)
				}
				if results[0].ResourceType != "db-instance" {
					t.Errorf("expected ResourceType 'db-instance', got '%s'", results[0].ResourceType)
				}
				if results[0].ResourceName != tt.wantResourceName {
					t.Errorf("expected ResourceName '%s', got '%s'", tt.wantResourceName, results[0].ResourceName)
				}
				if v, ok := results[0].Details["DBInstanceIdentifier"].(string); ok {
					if v != tt.wantDBInstanceIdentifier {
						t.Errorf("expected DBInstanceIdentifier '%s', got '%s'", tt.wantDBInstanceIdentifier, v)
					}
				} else if tt.wantDBInstanceIdentifier != "" {
					t.Errorf("expected DBInstanceIdentifier in Details, got none")
				}
				if v, ok := results[0].Details["DBInstanceArn"].(string); ok {
					if v != tt.wantDBInstanceArn {
						t.Errorf("expected DBInstanceArn '%s', got '%s'", tt.wantDBInstanceArn, v)
					}
				} else if tt.wantDBInstanceArn != "" {
					t.Errorf("expected DBInstanceArn in Details, got none")
				}
				if v, ok := results[0].Details["DBInstanceClass"].(string); ok {
					if v != tt.wantDBInstanceClass {
						t.Errorf("expected DBInstanceClass '%s', got '%s'", tt.wantDBInstanceClass, v)
					}
				} else if tt.wantDBInstanceClass != "" {
					t.Errorf("expected DBInstanceClass in Details, got none")
				}
				if v, ok := results[0].Details["Engine"].(string); ok {
					if v != tt.wantEngine {
						t.Errorf("expected Engine '%s', got '%s'", tt.wantEngine, v)
					}
				} else if tt.wantEngine != "" {
					t.Errorf("expected Engine in Details, got none")
				}
				if v, ok := results[0].Details["EngineVersion"].(string); ok {
					if v != tt.wantEngineVersion {
						t.Errorf("expected EngineVersion '%s', got '%s'", tt.wantEngineVersion, v)
					}
				} else if tt.wantEngineVersion != "" {
					t.Errorf("expected EngineVersion in Details, got none")
				}
				if v, ok := results[0].Details["DBInstanceStatus"].(string); ok {
					if v != tt.wantDBInstanceStatus {
						t.Errorf("expected DBInstanceStatus '%s', got '%s'", tt.wantDBInstanceStatus, v)
					}
				} else if tt.wantDBInstanceStatus != "" {
					t.Errorf("expected DBInstanceStatus in Details, got none")
				}
				if v, ok := results[0].Details["EndpointAddress"].(string); ok {
					if v != tt.wantEndpointAddress {
						t.Errorf("expected EndpointAddress '%s', got '%s'", tt.wantEndpointAddress, v)
					}
				} else if tt.wantEndpointAddress != "" {
					t.Errorf("expected EndpointAddress in Details, got none")
				}
				if v, ok := results[0].Details["EndpointPort"].(string); ok {
					if v != tt.wantEndpointPort {
						t.Errorf("expected EndpointPort '%s', got '%s'", tt.wantEndpointPort, v)
					}
				} else if tt.wantEndpointPort != "" {
					t.Errorf("expected EndpointPort in Details, got none")
				}
				if v, ok := results[0].Details["DBClusterIdentifier"].(string); ok {
					if v != tt.wantDBClusterIdentifier {
						t.Errorf("expected DBClusterIdentifier '%s', got '%s'", tt.wantDBClusterIdentifier, v)
					}
				} else if tt.wantDBClusterIdentifier != "" {
					t.Errorf("expected DBClusterIdentifier in Details, got none")
				}
				if v, ok := results[0].Details["AvailabilityZone"].(string); ok {
					if v != tt.wantAvailabilityZone {
						t.Errorf("expected AvailabilityZone '%s', got '%s'", tt.wantAvailabilityZone, v)
					}
				} else if tt.wantAvailabilityZone != "" {
					t.Errorf("expected AvailabilityZone in Details, got none")
				}
				if v, ok := results[0].Details["PubliclyAccessible"].(string); ok {
					if v != tt.wantPubliclyAccessible {
						t.Errorf("expected PubliclyAccessible '%s', got '%s'", tt.wantPubliclyAccessible, v)
					}
				} else if tt.wantPubliclyAccessible != "" {
					t.Errorf("expected PubliclyAccessible in Details, got none")
				}
				if v, ok := results[0].Details["StorageEncrypted"].(string); ok {
					if v != tt.wantStorageEncrypted {
						t.Errorf("expected StorageEncrypted '%s', got '%s'", tt.wantStorageEncrypted, v)
					}
				} else if tt.wantStorageEncrypted != "" {
					t.Errorf("expected StorageEncrypted in Details, got none")
				}
				if v, ok := results[0].Details["AutoMinorVersionUpgrade"].(string); ok {
					if v != tt.wantAutoUpgrade {
						t.Errorf("expected AutoMinorVersionUpgrade '%s', got '%s'", tt.wantAutoUpgrade, v)
					}
				} else if tt.wantAutoUpgrade != "" {
					t.Errorf("expected AutoMinorVersionUpgrade in Details, got none")
				}
				if v, ok := results[0].Details["Region"].(string); ok {
					if v != tt.wantRegion {
						t.Errorf("expected Region '%s', got '%s'", tt.wantRegion, v)
					}
				} else if tt.wantRegion != "" {
					t.Errorf("expected Region in Details, got none")
				}
			}
		})
	}
}

func TestDescribeDBClusterParameterGroupsProcess(t *testing.T) {
	process := NeptuneCalls[2].Process

	tests := []struct {
		name             string
		output           interface{}
		err              error
		wantLen          int
		wantError        bool
		wantResourceName string
		wantName         string
		wantArn          string
		wantDescription  string
		wantFamily       string
		wantRegion       string
	}{
		{
			name: "valid parameter groups with full details",
			output: []npParameterGroup{
				{
					DBClusterParameterGroupName: "default.neptune1",
					DBClusterParameterGroupArn:  "arn:aws:rds:us-east-1:123456789012:cluster-pg:default.neptune1",
					Description:                 "Default parameter group for neptune1",
					DBParameterGroupFamily:      "neptune1",
					Region:                      "us-east-1",
				},
				{
					DBClusterParameterGroupName: "custom-params",
					DBClusterParameterGroupArn:  "arn:aws:rds:us-west-2:123456789012:cluster-pg:custom-params",
					Description:                 "Custom parameter group",
					DBParameterGroupFamily:      "neptune1.2",
					Region:                      "us-west-2",
				},
			},
			wantLen:          2,
			wantResourceName: "default.neptune1",
			wantName:         "default.neptune1",
			wantArn:          "arn:aws:rds:us-east-1:123456789012:cluster-pg:default.neptune1",
			wantDescription:  "Default parameter group for neptune1",
			wantFamily:       "neptune1",
			wantRegion:       "us-east-1",
		},
		{
			name:    "empty results",
			output:  []npParameterGroup{},
			wantLen: 0,
		},
		{
			name:      "access denied error",
			output:    nil,
			err:       fmt.Errorf("AccessDeniedException: User is not authorized"),
			wantLen:   1,
			wantError: true,
		},
		{
			name: "nil-safe fields (empty strings)",
			output: []npParameterGroup{
				{
					DBClusterParameterGroupName: "",
					DBClusterParameterGroupArn:  "",
					Description:                 "",
					DBParameterGroupFamily:      "",
					Region:                      "",
				},
			},
			wantLen:          1,
			wantResourceName: "",
			wantName:         "",
			wantArn:          "",
			wantDescription:  "",
			wantFamily:       "",
			wantRegion:       "",
		},
		{
			name:    "type assertion failure",
			output:  "invalid type",
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := process(tt.output, tt.err, false)

			if len(results) != tt.wantLen {
				t.Fatalf("expected %d results, got %d", tt.wantLen, len(results))
			}

			if tt.wantError {
				if results[0].Error == nil {
					t.Error("expected error in result, got nil")
				}
				if results[0].ServiceName != "Neptune" {
					t.Errorf("expected ServiceName 'Neptune', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "neptune:DescribeDBClusterParameterGroups" {
					t.Errorf("expected MethodName 'neptune:DescribeDBClusterParameterGroups', got '%s'", results[0].MethodName)
				}
				return
			}

			if tt.wantLen > 0 {
				if results[0].ServiceName != "Neptune" {
					t.Errorf("expected ServiceName 'Neptune', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "neptune:DescribeDBClusterParameterGroups" {
					t.Errorf("expected MethodName 'neptune:DescribeDBClusterParameterGroups', got '%s'", results[0].MethodName)
				}
				if results[0].ResourceType != "db-cluster-parameter-group" {
					t.Errorf("expected ResourceType 'db-cluster-parameter-group', got '%s'", results[0].ResourceType)
				}
				if results[0].ResourceName != tt.wantResourceName {
					t.Errorf("expected ResourceName '%s', got '%s'", tt.wantResourceName, results[0].ResourceName)
				}
				if v, ok := results[0].Details["DBClusterParameterGroupName"].(string); ok {
					if v != tt.wantName {
						t.Errorf("expected DBClusterParameterGroupName '%s', got '%s'", tt.wantName, v)
					}
				} else if tt.wantName != "" {
					t.Errorf("expected DBClusterParameterGroupName in Details, got none")
				}
				if v, ok := results[0].Details["DBClusterParameterGroupArn"].(string); ok {
					if v != tt.wantArn {
						t.Errorf("expected DBClusterParameterGroupArn '%s', got '%s'", tt.wantArn, v)
					}
				} else if tt.wantArn != "" {
					t.Errorf("expected DBClusterParameterGroupArn in Details, got none")
				}
				if v, ok := results[0].Details["Description"].(string); ok {
					if v != tt.wantDescription {
						t.Errorf("expected Description '%s', got '%s'", tt.wantDescription, v)
					}
				} else if tt.wantDescription != "" {
					t.Errorf("expected Description in Details, got none")
				}
				if v, ok := results[0].Details["DBParameterGroupFamily"].(string); ok {
					if v != tt.wantFamily {
						t.Errorf("expected DBParameterGroupFamily '%s', got '%s'", tt.wantFamily, v)
					}
				} else if tt.wantFamily != "" {
					t.Errorf("expected DBParameterGroupFamily in Details, got none")
				}
				if v, ok := results[0].Details["Region"].(string); ok {
					if v != tt.wantRegion {
						t.Errorf("expected Region '%s', got '%s'", tt.wantRegion, v)
					}
				} else if tt.wantRegion != "" {
					t.Errorf("expected Region in Details, got none")
				}
			}
		})
	}
}

func TestExtractCluster(t *testing.T) {
	ts := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name                 string
		input                *neptune.DBCluster
		region               string
		wantIdentifier       string
		wantArn              string
		wantStatus           string
		wantEngine           string
		wantEngineVersion    string
		wantEndpoint         string
		wantReaderEndpoint   string
		wantPort             string
		wantMultiAZ          string
		wantStorageEncrypted string
		wantKmsKeyId         string
		wantDeletionProt     string
		wantIAMAuth          string
		wantParamGroup       string
		wantCreateTime       string
		wantRegion           string
	}{
		{
			name: "all fields populated",
			input: &neptune.DBCluster{
				DBClusterIdentifier:              aws.String("my-neptune-cluster"),
				DBClusterArn:                     aws.String("arn:aws:rds:us-east-1:123456789012:cluster:my-neptune-cluster"),
				Status:                           aws.String("available"),
				Engine:                           aws.String("neptune"),
				EngineVersion:                    aws.String("1.2.0.2"),
				Endpoint:                         aws.String("my-neptune-cluster.cluster-abc123.us-east-1.neptune.amazonaws.com"),
				ReaderEndpoint:                   aws.String("my-neptune-cluster.cluster-ro-abc123.us-east-1.neptune.amazonaws.com"),
				Port:                             aws.Int64(8182),
				MultiAZ:                          aws.Bool(true),
				StorageEncrypted:                 aws.Bool(true),
				KmsKeyId:                         aws.String("arn:aws:kms:us-east-1:123456789012:key/12345678"),
				DeletionProtection:               aws.Bool(true),
				IAMDatabaseAuthenticationEnabled: aws.Bool(true),
				DBClusterParameterGroup:          aws.String("default.neptune1"),
				ClusterCreateTime:                &ts,
			},
			region:               "us-east-1",
			wantIdentifier:       "my-neptune-cluster",
			wantArn:              "arn:aws:rds:us-east-1:123456789012:cluster:my-neptune-cluster",
			wantStatus:           "available",
			wantEngine:           "neptune",
			wantEngineVersion:    "1.2.0.2",
			wantEndpoint:         "my-neptune-cluster.cluster-abc123.us-east-1.neptune.amazonaws.com",
			wantReaderEndpoint:   "my-neptune-cluster.cluster-ro-abc123.us-east-1.neptune.amazonaws.com",
			wantPort:             "8182",
			wantMultiAZ:          "true",
			wantStorageEncrypted: "true",
			wantKmsKeyId:         "arn:aws:kms:us-east-1:123456789012:key/12345678",
			wantDeletionProt:     "true",
			wantIAMAuth:          "true",
			wantParamGroup:       "default.neptune1",
			wantCreateTime:       "2026-01-15T10:00:00Z",
			wantRegion:           "us-east-1",
		},
		{
			name:                 "all fields nil",
			input:                &neptune.DBCluster{},
			region:               "eu-west-1",
			wantIdentifier:       "",
			wantArn:              "",
			wantStatus:           "",
			wantEngine:           "",
			wantEngineVersion:    "",
			wantEndpoint:         "",
			wantReaderEndpoint:   "",
			wantPort:             "",
			wantMultiAZ:          "",
			wantStorageEncrypted: "",
			wantKmsKeyId:         "",
			wantDeletionProt:     "",
			wantIAMAuth:          "",
			wantParamGroup:       "",
			wantCreateTime:       "",
			wantRegion:           "eu-west-1",
		},
		{
			name: "partial fields populated",
			input: &neptune.DBCluster{
				DBClusterIdentifier: aws.String("partial-cluster"),
				Status:              aws.String("creating"),
				Port:                aws.Int64(8182),
				MultiAZ:             aws.Bool(false),
			},
			region:               "us-west-2",
			wantIdentifier:       "partial-cluster",
			wantArn:              "",
			wantStatus:           "creating",
			wantEngine:           "",
			wantEngineVersion:    "",
			wantEndpoint:         "",
			wantReaderEndpoint:   "",
			wantPort:             "8182",
			wantMultiAZ:          "false",
			wantStorageEncrypted: "",
			wantKmsKeyId:         "",
			wantDeletionProt:     "",
			wantIAMAuth:          "",
			wantParamGroup:       "",
			wantCreateTime:       "",
			wantRegion:           "us-west-2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractCluster(tt.input, tt.region)
			if result.DBClusterIdentifier != tt.wantIdentifier {
				t.Errorf("DBClusterIdentifier: got %q, want %q", result.DBClusterIdentifier, tt.wantIdentifier)
			}
			if result.DBClusterArn != tt.wantArn {
				t.Errorf("DBClusterArn: got %q, want %q", result.DBClusterArn, tt.wantArn)
			}
			if result.Status != tt.wantStatus {
				t.Errorf("Status: got %q, want %q", result.Status, tt.wantStatus)
			}
			if result.Engine != tt.wantEngine {
				t.Errorf("Engine: got %q, want %q", result.Engine, tt.wantEngine)
			}
			if result.EngineVersion != tt.wantEngineVersion {
				t.Errorf("EngineVersion: got %q, want %q", result.EngineVersion, tt.wantEngineVersion)
			}
			if result.Endpoint != tt.wantEndpoint {
				t.Errorf("Endpoint: got %q, want %q", result.Endpoint, tt.wantEndpoint)
			}
			if result.ReaderEndpoint != tt.wantReaderEndpoint {
				t.Errorf("ReaderEndpoint: got %q, want %q", result.ReaderEndpoint, tt.wantReaderEndpoint)
			}
			if result.Port != tt.wantPort {
				t.Errorf("Port: got %q, want %q", result.Port, tt.wantPort)
			}
			if result.MultiAZ != tt.wantMultiAZ {
				t.Errorf("MultiAZ: got %q, want %q", result.MultiAZ, tt.wantMultiAZ)
			}
			if result.StorageEncrypted != tt.wantStorageEncrypted {
				t.Errorf("StorageEncrypted: got %q, want %q", result.StorageEncrypted, tt.wantStorageEncrypted)
			}
			if result.KmsKeyId != tt.wantKmsKeyId {
				t.Errorf("KmsKeyId: got %q, want %q", result.KmsKeyId, tt.wantKmsKeyId)
			}
			if result.DeletionProtection != tt.wantDeletionProt {
				t.Errorf("DeletionProtection: got %q, want %q", result.DeletionProtection, tt.wantDeletionProt)
			}
			if result.IAMDatabaseAuthenticationEnabled != tt.wantIAMAuth {
				t.Errorf("IAMDatabaseAuthenticationEnabled: got %q, want %q", result.IAMDatabaseAuthenticationEnabled, tt.wantIAMAuth)
			}
			if result.DBClusterParameterGroup != tt.wantParamGroup {
				t.Errorf("DBClusterParameterGroup: got %q, want %q", result.DBClusterParameterGroup, tt.wantParamGroup)
			}
			if result.ClusterCreateTime != tt.wantCreateTime {
				t.Errorf("ClusterCreateTime: got %q, want %q", result.ClusterCreateTime, tt.wantCreateTime)
			}
			if result.Region != tt.wantRegion {
				t.Errorf("Region: got %q, want %q", result.Region, tt.wantRegion)
			}
		})
	}
}

func TestExtractInstance(t *testing.T) {
	tests := []struct {
		name                 string
		input                *neptune.DBInstance
		region               string
		wantIdentifier       string
		wantArn              string
		wantClass            string
		wantEngine           string
		wantEngineVersion    string
		wantStatus           string
		wantEndpointAddress  string
		wantEndpointPort     string
		wantClusterID        string
		wantAZ               string
		wantPublicAccess     string
		wantStorageEncrypted string
		wantAutoUpgrade      string
		wantRegion           string
	}{
		{
			name: "all fields populated with nested endpoint",
			input: &neptune.DBInstance{
				DBInstanceIdentifier: aws.String("my-instance-1"),
				DBInstanceArn:        aws.String("arn:aws:rds:us-east-1:123456789012:db:my-instance-1"),
				DBInstanceClass:      aws.String("db.r5.large"),
				Engine:               aws.String("neptune"),
				EngineVersion:        aws.String("1.2.0.2"),
				DBInstanceStatus:     aws.String("available"),
				Endpoint: &neptune.Endpoint{
					Address: aws.String("my-instance-1.abc123.us-east-1.neptune.amazonaws.com"),
					Port:    aws.Int64(8182),
				},
				DBClusterIdentifier:     aws.String("my-cluster"),
				AvailabilityZone:        aws.String("us-east-1a"),
				PubliclyAccessible:      aws.Bool(false),
				StorageEncrypted:        aws.Bool(true),
				AutoMinorVersionUpgrade: aws.Bool(true),
			},
			region:               "us-east-1",
			wantIdentifier:       "my-instance-1",
			wantArn:              "arn:aws:rds:us-east-1:123456789012:db:my-instance-1",
			wantClass:            "db.r5.large",
			wantEngine:           "neptune",
			wantEngineVersion:    "1.2.0.2",
			wantStatus:           "available",
			wantEndpointAddress:  "my-instance-1.abc123.us-east-1.neptune.amazonaws.com",
			wantEndpointPort:     "8182",
			wantClusterID:        "my-cluster",
			wantAZ:               "us-east-1a",
			wantPublicAccess:     "false",
			wantStorageEncrypted: "true",
			wantAutoUpgrade:      "true",
			wantRegion:           "us-east-1",
		},
		{
			name:                 "all fields nil",
			input:                &neptune.DBInstance{},
			region:               "eu-west-1",
			wantIdentifier:       "",
			wantArn:              "",
			wantClass:            "",
			wantEngine:           "",
			wantEngineVersion:    "",
			wantStatus:           "",
			wantEndpointAddress:  "",
			wantEndpointPort:     "",
			wantClusterID:        "",
			wantAZ:               "",
			wantPublicAccess:     "",
			wantStorageEncrypted: "",
			wantAutoUpgrade:      "",
			wantRegion:           "eu-west-1",
		},
		{
			name: "nil endpoint (endpoint struct is nil)",
			input: &neptune.DBInstance{
				DBInstanceIdentifier: aws.String("no-endpoint-instance"),
				DBInstanceStatus:     aws.String("creating"),
				Endpoint:             nil,
			},
			region:               "us-west-2",
			wantIdentifier:       "no-endpoint-instance",
			wantArn:              "",
			wantClass:            "",
			wantEngine:           "",
			wantEngineVersion:    "",
			wantStatus:           "creating",
			wantEndpointAddress:  "",
			wantEndpointPort:     "",
			wantClusterID:        "",
			wantAZ:               "",
			wantPublicAccess:     "",
			wantStorageEncrypted: "",
			wantAutoUpgrade:      "",
			wantRegion:           "us-west-2",
		},
		{
			name: "endpoint struct populated but address/port nil",
			input: &neptune.DBInstance{
				DBInstanceIdentifier: aws.String("partial-endpoint"),
				Endpoint:             &neptune.Endpoint{},
			},
			region:               "ap-southeast-1",
			wantIdentifier:       "partial-endpoint",
			wantArn:              "",
			wantClass:            "",
			wantEngine:           "",
			wantEngineVersion:    "",
			wantStatus:           "",
			wantEndpointAddress:  "",
			wantEndpointPort:     "",
			wantClusterID:        "",
			wantAZ:               "",
			wantPublicAccess:     "",
			wantStorageEncrypted: "",
			wantAutoUpgrade:      "",
			wantRegion:           "ap-southeast-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractInstance(tt.input, tt.region)
			if result.DBInstanceIdentifier != tt.wantIdentifier {
				t.Errorf("DBInstanceIdentifier: got %q, want %q", result.DBInstanceIdentifier, tt.wantIdentifier)
			}
			if result.DBInstanceArn != tt.wantArn {
				t.Errorf("DBInstanceArn: got %q, want %q", result.DBInstanceArn, tt.wantArn)
			}
			if result.DBInstanceClass != tt.wantClass {
				t.Errorf("DBInstanceClass: got %q, want %q", result.DBInstanceClass, tt.wantClass)
			}
			if result.Engine != tt.wantEngine {
				t.Errorf("Engine: got %q, want %q", result.Engine, tt.wantEngine)
			}
			if result.EngineVersion != tt.wantEngineVersion {
				t.Errorf("EngineVersion: got %q, want %q", result.EngineVersion, tt.wantEngineVersion)
			}
			if result.DBInstanceStatus != tt.wantStatus {
				t.Errorf("DBInstanceStatus: got %q, want %q", result.DBInstanceStatus, tt.wantStatus)
			}
			if result.EndpointAddress != tt.wantEndpointAddress {
				t.Errorf("EndpointAddress: got %q, want %q", result.EndpointAddress, tt.wantEndpointAddress)
			}
			if result.EndpointPort != tt.wantEndpointPort {
				t.Errorf("EndpointPort: got %q, want %q", result.EndpointPort, tt.wantEndpointPort)
			}
			if result.DBClusterIdentifier != tt.wantClusterID {
				t.Errorf("DBClusterIdentifier: got %q, want %q", result.DBClusterIdentifier, tt.wantClusterID)
			}
			if result.AvailabilityZone != tt.wantAZ {
				t.Errorf("AvailabilityZone: got %q, want %q", result.AvailabilityZone, tt.wantAZ)
			}
			if result.PubliclyAccessible != tt.wantPublicAccess {
				t.Errorf("PubliclyAccessible: got %q, want %q", result.PubliclyAccessible, tt.wantPublicAccess)
			}
			if result.StorageEncrypted != tt.wantStorageEncrypted {
				t.Errorf("StorageEncrypted: got %q, want %q", result.StorageEncrypted, tt.wantStorageEncrypted)
			}
			if result.AutoMinorVersionUpgrade != tt.wantAutoUpgrade {
				t.Errorf("AutoMinorVersionUpgrade: got %q, want %q", result.AutoMinorVersionUpgrade, tt.wantAutoUpgrade)
			}
			if result.Region != tt.wantRegion {
				t.Errorf("Region: got %q, want %q", result.Region, tt.wantRegion)
			}
		})
	}
}

func TestExtractParameterGroup(t *testing.T) {
	tests := []struct {
		name            string
		input           *neptune.DBClusterParameterGroup
		region          string
		wantName        string
		wantArn         string
		wantDescription string
		wantFamily      string
		wantRegion      string
	}{
		{
			name: "all fields populated",
			input: &neptune.DBClusterParameterGroup{
				DBClusterParameterGroupName: aws.String("default.neptune1"),
				DBClusterParameterGroupArn:  aws.String("arn:aws:rds:us-east-1:123456789012:cluster-pg:default.neptune1"),
				Description:                 aws.String("Default parameter group for neptune1"),
				DBParameterGroupFamily:      aws.String("neptune1"),
			},
			region:          "us-east-1",
			wantName:        "default.neptune1",
			wantArn:         "arn:aws:rds:us-east-1:123456789012:cluster-pg:default.neptune1",
			wantDescription: "Default parameter group for neptune1",
			wantFamily:      "neptune1",
			wantRegion:      "us-east-1",
		},
		{
			name:            "all fields nil",
			input:           &neptune.DBClusterParameterGroup{},
			region:          "eu-west-1",
			wantName:        "",
			wantArn:         "",
			wantDescription: "",
			wantFamily:      "",
			wantRegion:      "eu-west-1",
		},
		{
			name: "partial fields populated",
			input: &neptune.DBClusterParameterGroup{
				DBClusterParameterGroupName: aws.String("custom-params"),
				DBParameterGroupFamily:      aws.String("neptune1.2"),
			},
			region:          "us-west-2",
			wantName:        "custom-params",
			wantArn:         "",
			wantDescription: "",
			wantFamily:      "neptune1.2",
			wantRegion:      "us-west-2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractParameterGroup(tt.input, tt.region)
			if result.DBClusterParameterGroupName != tt.wantName {
				t.Errorf("DBClusterParameterGroupName: got %q, want %q", result.DBClusterParameterGroupName, tt.wantName)
			}
			if result.DBClusterParameterGroupArn != tt.wantArn {
				t.Errorf("DBClusterParameterGroupArn: got %q, want %q", result.DBClusterParameterGroupArn, tt.wantArn)
			}
			if result.Description != tt.wantDescription {
				t.Errorf("Description: got %q, want %q", result.Description, tt.wantDescription)
			}
			if result.DBParameterGroupFamily != tt.wantFamily {
				t.Errorf("DBParameterGroupFamily: got %q, want %q", result.DBParameterGroupFamily, tt.wantFamily)
			}
			if result.Region != tt.wantRegion {
				t.Errorf("Region: got %q, want %q", result.Region, tt.wantRegion)
			}
		})
	}
}
