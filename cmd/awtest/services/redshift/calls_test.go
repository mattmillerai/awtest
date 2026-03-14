package redshift

import (
	"fmt"
	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/redshift"
	"testing"
)

func TestProcess(t *testing.T) {
	process := RedshiftCalls[0].Process

	tests := []struct {
		name          string
		input         interface{}
		err           error
		expectedCount int
		expectError   bool
		checkResults  func(t *testing.T, results []types.ScanResult)
	}{
		{
			name: "valid cluster with all fields",
			input: []*redshift.Cluster{
				{
					ClusterIdentifier: aws.String("my-redshift-cluster"),
					NodeType:          aws.String("dc2.large"),
					ClusterStatus:     aws.String("available"),
					MasterUsername:    aws.String("admin"),
					DBName:            aws.String("mydb"),
					Endpoint: &redshift.Endpoint{
						Address: aws.String("my-redshift-cluster.abc123.us-east-1.redshift.amazonaws.com"),
						Port:    aws.Int64(5439),
					},
					Encrypted:     aws.Bool(false),
					NumberOfNodes: aws.Int64(2),
				},
			},
			expectedCount: 1,
			checkResults: func(t *testing.T, results []types.ScanResult) {
				r := results[0]
				if r.ServiceName != "Redshift" {
					t.Errorf("expected ServiceName 'Redshift', got '%s'", r.ServiceName)
				}
				if r.MethodName != "redshift:DescribeClusters" {
					t.Errorf("expected MethodName 'redshift:DescribeClusters', got '%s'", r.MethodName)
				}
				if r.ResourceType != "cluster" {
					t.Errorf("expected ResourceType 'cluster', got '%s'", r.ResourceType)
				}
				if r.ResourceName != "my-redshift-cluster" {
					t.Errorf("expected ResourceName 'my-redshift-cluster', got '%s'", r.ResourceName)
				}
				if r.Details["NodeType"] != "dc2.large" {
					t.Errorf("expected NodeType 'dc2.large', got '%v'", r.Details["NodeType"])
				}
				if r.Details["ClusterStatus"] != "available" {
					t.Errorf("expected ClusterStatus 'available', got '%v'", r.Details["ClusterStatus"])
				}
				if r.Details["MasterUsername"] != "admin" {
					t.Errorf("expected MasterUsername 'admin', got '%v'", r.Details["MasterUsername"])
				}
				if r.Details["DBName"] != "mydb" {
					t.Errorf("expected DBName 'mydb', got '%v'", r.Details["DBName"])
				}
				if r.Details["Endpoint"] != "my-redshift-cluster.abc123.us-east-1.redshift.amazonaws.com:5439" {
					t.Errorf("expected Endpoint with address:port, got '%v'", r.Details["Endpoint"])
				}
				if r.Details["Encrypted"] != false {
					t.Errorf("expected Encrypted false, got '%v'", r.Details["Encrypted"])
				}
				if r.Details["NumberOfNodes"] != int64(2) {
					t.Errorf("expected NumberOfNodes 2, got '%v'", r.Details["NumberOfNodes"])
				}
			},
		},
		{
			name: "encrypted cluster",
			input: []*redshift.Cluster{
				{
					ClusterIdentifier: aws.String("encrypted-cluster"),
					NodeType:          aws.String("ra3.xlplus"),
					ClusterStatus:     aws.String("available"),
					Encrypted:         aws.Bool(true),
					NumberOfNodes:     aws.Int64(4),
				},
			},
			expectedCount: 1,
			checkResults: func(t *testing.T, results []types.ScanResult) {
				r := results[0]
				if r.Details["Encrypted"] != true {
					t.Errorf("expected Encrypted true, got '%v'", r.Details["Encrypted"])
				}
				if r.Details["NumberOfNodes"] != int64(4) {
					t.Errorf("expected NumberOfNodes 4, got '%v'", r.Details["NumberOfNodes"])
				}
			},
		},
		{
			name: "multiple clusters",
			input: []*redshift.Cluster{
				{ClusterIdentifier: aws.String("cluster-1"), NodeType: aws.String("dc2.large")},
				{ClusterIdentifier: aws.String("cluster-2"), NodeType: aws.String("ra3.xlplus")},
			},
			expectedCount: 2,
			checkResults: func(t *testing.T, results []types.ScanResult) {
				if results[0].ResourceName != "cluster-1" {
					t.Errorf("expected first ResourceName 'cluster-1', got '%s'", results[0].ResourceName)
				}
				if results[1].ResourceName != "cluster-2" {
					t.Errorf("expected second ResourceName 'cluster-2', got '%s'", results[1].ResourceName)
				}
			},
		},
		{
			name:          "empty results",
			input:         []*redshift.Cluster{},
			expectedCount: 0,
		},
		{
			name:          "access denied error",
			input:         nil,
			err:           fmt.Errorf("AccessDeniedException: User is not authorized"),
			expectedCount: 1,
			expectError:   true,
			checkResults: func(t *testing.T, results []types.ScanResult) {
				if results[0].Error == nil {
					t.Error("expected error in result, got nil")
				}
				if results[0].ServiceName != "Redshift" {
					t.Errorf("expected ServiceName 'Redshift', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "redshift:DescribeClusters" {
					t.Errorf("expected MethodName 'redshift:DescribeClusters', got '%s'", results[0].MethodName)
				}
			},
		},
		{
			name: "endpoint with nil address",
			input: []*redshift.Cluster{
				{
					ClusterIdentifier: aws.String("partial-endpoint"),
					Endpoint: &redshift.Endpoint{
						Address: nil,
						Port:    aws.Int64(5439),
					},
				},
			},
			expectedCount: 1,
			checkResults: func(t *testing.T, results []types.ScanResult) {
				if results[0].Details["Endpoint"] != ":5439" {
					t.Errorf("expected Endpoint ':5439', got '%v'", results[0].Details["Endpoint"])
				}
			},
		},
		{
			name: "endpoint with nil port",
			input: []*redshift.Cluster{
				{
					ClusterIdentifier: aws.String("no-port"),
					Endpoint: &redshift.Endpoint{
						Address: aws.String("my-cluster.abc123.us-east-1.redshift.amazonaws.com"),
						Port:    nil,
					},
				},
			},
			expectedCount: 1,
			checkResults: func(t *testing.T, results []types.ScanResult) {
				if results[0].Details["Endpoint"] != "my-cluster.abc123.us-east-1.redshift.amazonaws.com" {
					t.Errorf("expected Endpoint without port, got '%v'", results[0].Details["Endpoint"])
				}
			},
		},
		{
			name: "nil field handling",
			input: []*redshift.Cluster{
				{
					ClusterIdentifier: nil,
					NodeType:          nil,
					ClusterStatus:     nil,
					MasterUsername:    nil,
					DBName:            nil,
					Endpoint:          nil,
					Encrypted:         nil,
					NumberOfNodes:     nil,
				},
			},
			expectedCount: 1,
			checkResults: func(t *testing.T, results []types.ScanResult) {
				r := results[0]
				if r.ResourceName != "" {
					t.Errorf("expected empty ResourceName for nil ClusterIdentifier, got '%s'", r.ResourceName)
				}
				if r.Details["NodeType"] != "" {
					t.Errorf("expected empty NodeType for nil, got '%v'", r.Details["NodeType"])
				}
				if r.Details["ClusterStatus"] != "" {
					t.Errorf("expected empty ClusterStatus for nil, got '%v'", r.Details["ClusterStatus"])
				}
				if r.Details["MasterUsername"] != "" {
					t.Errorf("expected empty MasterUsername for nil, got '%v'", r.Details["MasterUsername"])
				}
				if r.Details["DBName"] != "" {
					t.Errorf("expected empty DBName for nil, got '%v'", r.Details["DBName"])
				}
				if r.Details["Endpoint"] != "" {
					t.Errorf("expected empty Endpoint for nil, got '%v'", r.Details["Endpoint"])
				}
				if r.Details["Encrypted"] != false {
					t.Errorf("expected Encrypted false for nil, got '%v'", r.Details["Encrypted"])
				}
				if r.Details["NumberOfNodes"] != int64(0) {
					t.Errorf("expected NumberOfNodes 0 for nil, got '%v'", r.Details["NumberOfNodes"])
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			results := process(tc.input, tc.err, false)
			if len(results) != tc.expectedCount {
				t.Fatalf("expected %d results, got %d", tc.expectedCount, len(results))
			}
			if tc.checkResults != nil {
				tc.checkResults(t, results)
			}
		})
	}
}
