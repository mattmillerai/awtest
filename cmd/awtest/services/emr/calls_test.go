package emr

import (
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/emr"
)

func TestListClustersProcess(t *testing.T) {
	process := EMRCalls[0].Process

	tests := []struct {
		name             string
		output           interface{}
		err              error
		wantLen          int
		wantError        bool
		wantResourceName string
		wantClusterId    string
		wantName         string
		wantState        string
		wantRelease      string
		wantRegion       string
	}{
		{
			name: "valid clusters with full details",
			output: []emrCluster{
				{
					ClusterId:    "j-XXXXXXXXXXXXX",
					Name:         "my-spark-cluster",
					State:        "RUNNING",
					ReleaseLabel: "emr-6.10.0",
					Region:       "us-east-1",
				},
				{
					ClusterId:    "j-YYYYYYYYYYYYY",
					Name:         "my-hive-cluster",
					State:        "WAITING",
					ReleaseLabel: "emr-6.8.0",
					Region:       "us-west-2",
				},
			},
			wantLen:          2,
			wantResourceName: "my-spark-cluster",
			wantClusterId:    "j-XXXXXXXXXXXXX",
			wantName:         "my-spark-cluster",
			wantState:        "RUNNING",
			wantRelease:      "emr-6.10.0",
			wantRegion:       "us-east-1",
		},
		{
			name:    "empty results",
			output:  []emrCluster{},
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
			output: []emrCluster{
				{
					ClusterId:    "",
					Name:         "",
					State:        "",
					ReleaseLabel: "",
					Region:       "",
				},
			},
			wantLen:          1,
			wantResourceName: "",
			wantClusterId:    "",
			wantName:         "",
			wantState:        "",
			wantRelease:      "",
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
				if results[0].ServiceName != "EMR" {
					t.Errorf("expected ServiceName 'EMR', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "emr:ListClusters" {
					t.Errorf("expected MethodName 'emr:ListClusters', got '%s'", results[0].MethodName)
				}
				return
			}

			if tt.wantLen > 0 {
				if results[0].ServiceName != "EMR" {
					t.Errorf("expected ServiceName 'EMR', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "emr:ListClusters" {
					t.Errorf("expected MethodName 'emr:ListClusters', got '%s'", results[0].MethodName)
				}
				if results[0].ResourceType != "cluster" {
					t.Errorf("expected ResourceType 'cluster', got '%s'", results[0].ResourceType)
				}
				if results[0].ResourceName != tt.wantResourceName {
					t.Errorf("expected ResourceName '%s', got '%s'", tt.wantResourceName, results[0].ResourceName)
				}
				if clusterId, ok := results[0].Details["ClusterId"].(string); ok {
					if clusterId != tt.wantClusterId {
						t.Errorf("expected ClusterId '%s', got '%s'", tt.wantClusterId, clusterId)
					}
				} else if tt.wantClusterId != "" {
					t.Errorf("expected ClusterId in Details, got none")
				}
				if name, ok := results[0].Details["Name"].(string); ok {
					if name != tt.wantName {
						t.Errorf("expected Name '%s', got '%s'", tt.wantName, name)
					}
				} else if tt.wantName != "" {
					t.Errorf("expected Name in Details, got none")
				}
				if state, ok := results[0].Details["State"].(string); ok {
					if state != tt.wantState {
						t.Errorf("expected State '%s', got '%s'", tt.wantState, state)
					}
				} else if tt.wantState != "" {
					t.Errorf("expected State in Details, got none")
				}
				if release, ok := results[0].Details["ReleaseLabel"].(string); ok {
					if release != tt.wantRelease {
						t.Errorf("expected ReleaseLabel '%s', got '%s'", tt.wantRelease, release)
					}
				} else if tt.wantRelease != "" {
					t.Errorf("expected ReleaseLabel in Details, got none")
				}
				if region, ok := results[0].Details["Region"].(string); ok {
					if region != tt.wantRegion {
						t.Errorf("expected Region '%s', got '%s'", tt.wantRegion, region)
					}
				} else if tt.wantRegion != "" {
					t.Errorf("expected Region in Details, got none")
				}
			}
		})
	}
}

func TestListInstanceGroupsProcess(t *testing.T) {
	process := EMRCalls[1].Process

	tests := []struct {
		name             string
		output           interface{}
		err              error
		wantLen          int
		wantError        bool
		wantResourceName string
		wantId           string
		wantName         string
		wantClusterId    string
		wantClusterName  string
		wantIGType       string
		wantInstType     string
		wantReqCount     string
		wantRunCount     string
		wantMarket       string
		wantState        string
		wantRegion       string
	}{
		{
			name: "valid instance groups with full details",
			output: []emrInstanceGroup{
				{
					Id:                     "ig-XXXXXXXXXXXXX",
					Name:                   "Master",
					ClusterId:              "j-XXXXXXXXXXXXX",
					ClusterName:            "my-spark-cluster",
					InstanceGroupType:      "MASTER",
					InstanceType:           "m5.xlarge",
					RequestedInstanceCount: "1",
					RunningInstanceCount:   "1",
					Market:                 "ON_DEMAND",
					State:                  "RUNNING",
					Region:                 "us-east-1",
				},
				{
					Id:                     "ig-YYYYYYYYYYYYY",
					Name:                   "Core",
					ClusterId:              "j-XXXXXXXXXXXXX",
					ClusterName:            "my-spark-cluster",
					InstanceGroupType:      "CORE",
					InstanceType:           "m5.2xlarge",
					RequestedInstanceCount: "4",
					RunningInstanceCount:   "4",
					Market:                 "SPOT",
					State:                  "RUNNING",
					Region:                 "us-east-1",
				},
			},
			wantLen:          2,
			wantResourceName: "Master",
			wantId:           "ig-XXXXXXXXXXXXX",
			wantName:         "Master",
			wantClusterId:    "j-XXXXXXXXXXXXX",
			wantClusterName:  "my-spark-cluster",
			wantIGType:       "MASTER",
			wantInstType:     "m5.xlarge",
			wantReqCount:     "1",
			wantRunCount:     "1",
			wantMarket:       "ON_DEMAND",
			wantState:        "RUNNING",
			wantRegion:       "us-east-1",
		},
		{
			name:    "empty results",
			output:  []emrInstanceGroup{},
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
			output: []emrInstanceGroup{
				{
					Id:                     "",
					Name:                   "",
					ClusterId:              "",
					ClusterName:            "",
					InstanceGroupType:      "",
					InstanceType:           "",
					RequestedInstanceCount: "",
					RunningInstanceCount:   "",
					Market:                 "",
					State:                  "",
					Region:                 "",
				},
			},
			wantLen:          1,
			wantResourceName: "",
			wantId:           "",
			wantName:         "",
			wantClusterId:    "",
			wantClusterName:  "",
			wantIGType:       "",
			wantInstType:     "",
			wantReqCount:     "",
			wantRunCount:     "",
			wantMarket:       "",
			wantState:        "",
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
				if results[0].ServiceName != "EMR" {
					t.Errorf("expected ServiceName 'EMR', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "emr:ListInstanceGroups" {
					t.Errorf("expected MethodName 'emr:ListInstanceGroups', got '%s'", results[0].MethodName)
				}
				return
			}

			if tt.wantLen > 0 {
				if results[0].ServiceName != "EMR" {
					t.Errorf("expected ServiceName 'EMR', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "emr:ListInstanceGroups" {
					t.Errorf("expected MethodName 'emr:ListInstanceGroups', got '%s'", results[0].MethodName)
				}
				if results[0].ResourceType != "instance-group" {
					t.Errorf("expected ResourceType 'instance-group', got '%s'", results[0].ResourceType)
				}
				if results[0].ResourceName != tt.wantResourceName {
					t.Errorf("expected ResourceName '%s', got '%s'", tt.wantResourceName, results[0].ResourceName)
				}
				if id, ok := results[0].Details["Id"].(string); ok {
					if id != tt.wantId {
						t.Errorf("expected Id '%s', got '%s'", tt.wantId, id)
					}
				} else if tt.wantId != "" {
					t.Errorf("expected Id in Details, got none")
				}
				if name, ok := results[0].Details["Name"].(string); ok {
					if name != tt.wantName {
						t.Errorf("expected Name '%s', got '%s'", tt.wantName, name)
					}
				} else if tt.wantName != "" {
					t.Errorf("expected Name in Details, got none")
				}
				if clusterId, ok := results[0].Details["ClusterId"].(string); ok {
					if clusterId != tt.wantClusterId {
						t.Errorf("expected ClusterId '%s', got '%s'", tt.wantClusterId, clusterId)
					}
				} else if tt.wantClusterId != "" {
					t.Errorf("expected ClusterId in Details, got none")
				}
				if clusterName, ok := results[0].Details["ClusterName"].(string); ok {
					if clusterName != tt.wantClusterName {
						t.Errorf("expected ClusterName '%s', got '%s'", tt.wantClusterName, clusterName)
					}
				} else if tt.wantClusterName != "" {
					t.Errorf("expected ClusterName in Details, got none")
				}
				if igType, ok := results[0].Details["InstanceGroupType"].(string); ok {
					if igType != tt.wantIGType {
						t.Errorf("expected InstanceGroupType '%s', got '%s'", tt.wantIGType, igType)
					}
				} else if tt.wantIGType != "" {
					t.Errorf("expected InstanceGroupType in Details, got none")
				}
				if instType, ok := results[0].Details["InstanceType"].(string); ok {
					if instType != tt.wantInstType {
						t.Errorf("expected InstanceType '%s', got '%s'", tt.wantInstType, instType)
					}
				} else if tt.wantInstType != "" {
					t.Errorf("expected InstanceType in Details, got none")
				}
				if reqCount, ok := results[0].Details["RequestedInstanceCount"].(string); ok {
					if reqCount != tt.wantReqCount {
						t.Errorf("expected RequestedInstanceCount '%s', got '%s'", tt.wantReqCount, reqCount)
					}
				} else if tt.wantReqCount != "" {
					t.Errorf("expected RequestedInstanceCount in Details, got none")
				}
				if runCount, ok := results[0].Details["RunningInstanceCount"].(string); ok {
					if runCount != tt.wantRunCount {
						t.Errorf("expected RunningInstanceCount '%s', got '%s'", tt.wantRunCount, runCount)
					}
				} else if tt.wantRunCount != "" {
					t.Errorf("expected RunningInstanceCount in Details, got none")
				}
				if market, ok := results[0].Details["Market"].(string); ok {
					if market != tt.wantMarket {
						t.Errorf("expected Market '%s', got '%s'", tt.wantMarket, market)
					}
				} else if tt.wantMarket != "" {
					t.Errorf("expected Market in Details, got none")
				}
				if state, ok := results[0].Details["State"].(string); ok {
					if state != tt.wantState {
						t.Errorf("expected State '%s', got '%s'", tt.wantState, state)
					}
				} else if tt.wantState != "" {
					t.Errorf("expected State in Details, got none")
				}
				if region, ok := results[0].Details["Region"].(string); ok {
					if region != tt.wantRegion {
						t.Errorf("expected Region '%s', got '%s'", tt.wantRegion, region)
					}
				} else if tt.wantRegion != "" {
					t.Errorf("expected Region in Details, got none")
				}
			}
		})
	}
}

func TestListSecurityConfigurationsProcess(t *testing.T) {
	process := EMRCalls[2].Process

	tests := []struct {
		name             string
		output           interface{}
		err              error
		wantLen          int
		wantError        bool
		wantResourceName string
		wantName         string
		wantCreated      string
		wantRegion       string
	}{
		{
			name: "valid configs with full details",
			output: []emrSecurityConfig{
				{
					Name:             "my-security-config",
					CreationDateTime: "2026-01-15T10:00:00Z",
					Region:           "us-east-1",
				},
				{
					Name:             "encryption-config",
					CreationDateTime: "2026-02-20T14:00:00Z",
					Region:           "us-west-2",
				},
			},
			wantLen:          2,
			wantResourceName: "my-security-config",
			wantName:         "my-security-config",
			wantCreated:      "2026-01-15T10:00:00Z",
			wantRegion:       "us-east-1",
		},
		{
			name:    "empty results",
			output:  []emrSecurityConfig{},
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
			output: []emrSecurityConfig{
				{
					Name:             "",
					CreationDateTime: "",
					Region:           "",
				},
			},
			wantLen:          1,
			wantResourceName: "",
			wantName:         "",
			wantCreated:      "",
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
				if results[0].ServiceName != "EMR" {
					t.Errorf("expected ServiceName 'EMR', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "emr:ListSecurityConfigurations" {
					t.Errorf("expected MethodName 'emr:ListSecurityConfigurations', got '%s'", results[0].MethodName)
				}
				return
			}

			if tt.wantLen > 0 {
				if results[0].ServiceName != "EMR" {
					t.Errorf("expected ServiceName 'EMR', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "emr:ListSecurityConfigurations" {
					t.Errorf("expected MethodName 'emr:ListSecurityConfigurations', got '%s'", results[0].MethodName)
				}
				if results[0].ResourceType != "security-configuration" {
					t.Errorf("expected ResourceType 'security-configuration', got '%s'", results[0].ResourceType)
				}
				if results[0].ResourceName != tt.wantResourceName {
					t.Errorf("expected ResourceName '%s', got '%s'", tt.wantResourceName, results[0].ResourceName)
				}
				if name, ok := results[0].Details["Name"].(string); ok {
					if name != tt.wantName {
						t.Errorf("expected Name '%s', got '%s'", tt.wantName, name)
					}
				} else if tt.wantName != "" {
					t.Errorf("expected Name in Details, got none")
				}
				if created, ok := results[0].Details["CreationDateTime"].(string); ok {
					if created != tt.wantCreated {
						t.Errorf("expected CreationDateTime '%s', got '%s'", tt.wantCreated, created)
					}
				} else if tt.wantCreated != "" {
					t.Errorf("expected CreationDateTime in Details, got none")
				}
				if region, ok := results[0].Details["Region"].(string); ok {
					if region != tt.wantRegion {
						t.Errorf("expected Region '%s', got '%s'", tt.wantRegion, region)
					}
				} else if tt.wantRegion != "" {
					t.Errorf("expected Region in Details, got none")
				}
			}
		})
	}
}

func TestExtractCluster(t *testing.T) {
	tests := []struct {
		name         string
		input        *emr.Cluster
		region       string
		wantId       string
		wantName     string
		wantState    string
		wantRelease  string
		wantRegion   string
	}{
		{
			name: "all fields populated",
			input: &emr.Cluster{
				Id:           aws.String("j-ABC123"),
				Name:         aws.String("my-cluster"),
				Status:       &emr.ClusterStatus{State: aws.String("RUNNING")},
				ReleaseLabel: aws.String("emr-6.10.0"),
			},
			region:      "us-east-1",
			wantId:      "j-ABC123",
			wantName:    "my-cluster",
			wantState:   "RUNNING",
			wantRelease: "emr-6.10.0",
			wantRegion:  "us-east-1",
		},
		{
			name:        "all fields nil",
			input:       &emr.Cluster{},
			region:      "eu-west-1",
			wantId:      "",
			wantName:    "",
			wantState:   "",
			wantRelease: "",
			wantRegion:  "eu-west-1",
		},
		{
			name: "status present but state nil",
			input: &emr.Cluster{
				Id:     aws.String("j-DEF456"),
				Name:   aws.String("partial-cluster"),
				Status: &emr.ClusterStatus{},
			},
			region:      "us-west-2",
			wantId:      "j-DEF456",
			wantName:    "partial-cluster",
			wantState:   "",
			wantRelease: "",
			wantRegion:  "us-west-2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractCluster(tt.input, tt.region)
			if result.ClusterId != tt.wantId {
				t.Errorf("ClusterId: got %q, want %q", result.ClusterId, tt.wantId)
			}
			if result.Name != tt.wantName {
				t.Errorf("Name: got %q, want %q", result.Name, tt.wantName)
			}
			if result.State != tt.wantState {
				t.Errorf("State: got %q, want %q", result.State, tt.wantState)
			}
			if result.ReleaseLabel != tt.wantRelease {
				t.Errorf("ReleaseLabel: got %q, want %q", result.ReleaseLabel, tt.wantRelease)
			}
			if result.Region != tt.wantRegion {
				t.Errorf("Region: got %q, want %q", result.Region, tt.wantRegion)
			}
		})
	}
}

func TestExtractInstanceGroup(t *testing.T) {
	tests := []struct {
		name        string
		input       *emr.InstanceGroup
		clusterId   string
		clusterName string
		region      string
		wantId      string
		wantName    string
		wantIGType  string
		wantInst    string
		wantReq     string
		wantRun     string
		wantMarket  string
		wantState   string
	}{
		{
			name: "all fields populated",
			input: &emr.InstanceGroup{
				Id:                     aws.String("ig-ABC123"),
				Name:                   aws.String("Master"),
				InstanceGroupType:      aws.String("MASTER"),
				InstanceType:           aws.String("m5.xlarge"),
				RequestedInstanceCount: aws.Int64(1),
				RunningInstanceCount:   aws.Int64(1),
				Market:                 aws.String("ON_DEMAND"),
				Status:                 &emr.InstanceGroupStatus{State: aws.String("RUNNING")},
			},
			clusterId:   "j-ABC123",
			clusterName: "my-cluster",
			region:      "us-east-1",
			wantId:      "ig-ABC123",
			wantName:    "Master",
			wantIGType:  "MASTER",
			wantInst:    "m5.xlarge",
			wantReq:     "1",
			wantRun:     "1",
			wantMarket:  "ON_DEMAND",
			wantState:   "RUNNING",
		},
		{
			name:        "all fields nil",
			input:       &emr.InstanceGroup{},
			clusterId:   "j-DEF456",
			clusterName: "empty-cluster",
			region:      "eu-west-1",
			wantId:      "",
			wantName:    "",
			wantIGType:  "",
			wantInst:    "",
			wantReq:     "",
			wantRun:     "",
			wantMarket:  "",
			wantState:   "",
		},
		{
			name: "status present but state nil",
			input: &emr.InstanceGroup{
				Id:     aws.String("ig-GHI789"),
				Name:   aws.String("Core"),
				Status: &emr.InstanceGroupStatus{},
			},
			clusterId:   "j-GHI789",
			clusterName: "partial",
			region:      "us-west-2",
			wantId:      "ig-GHI789",
			wantName:    "Core",
			wantIGType:  "",
			wantInst:    "",
			wantReq:     "",
			wantRun:     "",
			wantMarket:  "",
			wantState:   "",
		},
		{
			name: "zero counts",
			input: &emr.InstanceGroup{
				Id:                     aws.String("ig-ZERO"),
				RequestedInstanceCount: aws.Int64(0),
				RunningInstanceCount:   aws.Int64(0),
			},
			clusterId:   "j-ZERO",
			clusterName: "zero-cluster",
			region:      "ap-southeast-1",
			wantId:      "ig-ZERO",
			wantReq:     "0",
			wantRun:     "0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractInstanceGroup(tt.input, tt.clusterId, tt.clusterName, tt.region)
			if result.Id != tt.wantId {
				t.Errorf("Id: got %q, want %q", result.Id, tt.wantId)
			}
			if result.Name != tt.wantName {
				t.Errorf("Name: got %q, want %q", result.Name, tt.wantName)
			}
			if result.ClusterId != tt.clusterId {
				t.Errorf("ClusterId: got %q, want %q", result.ClusterId, tt.clusterId)
			}
			if result.ClusterName != tt.clusterName {
				t.Errorf("ClusterName: got %q, want %q", result.ClusterName, tt.clusterName)
			}
			if result.InstanceGroupType != tt.wantIGType {
				t.Errorf("InstanceGroupType: got %q, want %q", result.InstanceGroupType, tt.wantIGType)
			}
			if result.InstanceType != tt.wantInst {
				t.Errorf("InstanceType: got %q, want %q", result.InstanceType, tt.wantInst)
			}
			if result.RequestedInstanceCount != tt.wantReq {
				t.Errorf("RequestedInstanceCount: got %q, want %q", result.RequestedInstanceCount, tt.wantReq)
			}
			if result.RunningInstanceCount != tt.wantRun {
				t.Errorf("RunningInstanceCount: got %q, want %q", result.RunningInstanceCount, tt.wantRun)
			}
			if result.Market != tt.wantMarket {
				t.Errorf("Market: got %q, want %q", result.Market, tt.wantMarket)
			}
			if result.State != tt.wantState {
				t.Errorf("State: got %q, want %q", result.State, tt.wantState)
			}
			if result.Region != tt.region {
				t.Errorf("Region: got %q, want %q", result.Region, tt.region)
			}
		})
	}
}

func TestExtractSecurityConfig(t *testing.T) {
	ts := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		input       *emr.SecurityConfigurationSummary
		region      string
		wantName    string
		wantCreated string
		wantRegion  string
	}{
		{
			name: "all fields populated",
			input: &emr.SecurityConfigurationSummary{
				Name:             aws.String("my-config"),
				CreationDateTime: &ts,
			},
			region:      "us-east-1",
			wantName:    "my-config",
			wantCreated: "2026-01-15T10:00:00Z",
			wantRegion:  "us-east-1",
		},
		{
			name:        "all fields nil",
			input:       &emr.SecurityConfigurationSummary{},
			region:      "eu-west-1",
			wantName:    "",
			wantCreated: "",
			wantRegion:  "eu-west-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractSecurityConfig(tt.input, tt.region)
			if result.Name != tt.wantName {
				t.Errorf("Name: got %q, want %q", result.Name, tt.wantName)
			}
			if result.CreationDateTime != tt.wantCreated {
				t.Errorf("CreationDateTime: got %q, want %q", result.CreationDateTime, tt.wantCreated)
			}
			if result.Region != tt.wantRegion {
				t.Errorf("Region: got %q, want %q", result.Region, tt.wantRegion)
			}
		})
	}
}
