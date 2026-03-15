package eks

import (
	"fmt"
	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/eks"
	"testing"
)

func TestProcess(t *testing.T) {
	process := EKSCalls[0].Process

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
			input: []*eks.Cluster{
				{
					Name:     aws.String("my-cluster"),
					Arn:      aws.String("arn:aws:eks:us-east-1:123456789012:cluster/my-cluster"),
					Status:   aws.String("ACTIVE"),
					Version:  aws.String("1.28"),
					Endpoint: aws.String("https://ABC123.gr7.us-east-1.eks.amazonaws.com"),
					RoleArn:  aws.String("arn:aws:iam::123456789012:role/eks-role"),
					ResourcesVpcConfig: &eks.VpcConfigResponse{
						VpcId:            aws.String("vpc-12345"),
						SubnetIds:        []*string{aws.String("subnet-1"), aws.String("subnet-2")},
						SecurityGroupIds: []*string{aws.String("sg-1")},
					},
				},
			},
			expectedCount: 1,
			checkResults: func(t *testing.T, results []types.ScanResult) {
				r := results[0]
				if r.ServiceName != "EKS" {
					t.Errorf("expected ServiceName 'EKS', got '%s'", r.ServiceName)
				}
				if r.MethodName != "eks:ListClusters" {
					t.Errorf("expected MethodName 'eks:ListClusters', got '%s'", r.MethodName)
				}
				if r.ResourceType != "cluster" {
					t.Errorf("expected ResourceType 'cluster', got '%s'", r.ResourceType)
				}
				if r.ResourceName != "my-cluster" {
					t.Errorf("expected ResourceName 'my-cluster', got '%s'", r.ResourceName)
				}
				if r.Details["Arn"] != "arn:aws:eks:us-east-1:123456789012:cluster/my-cluster" {
					t.Errorf("expected Arn match, got '%v'", r.Details["Arn"])
				}
				if r.Details["Status"] != "ACTIVE" {
					t.Errorf("expected Status 'ACTIVE', got '%v'", r.Details["Status"])
				}
				if r.Details["Version"] != "1.28" {
					t.Errorf("expected Version '1.28', got '%v'", r.Details["Version"])
				}
				if r.Details["Endpoint"] != "https://ABC123.gr7.us-east-1.eks.amazonaws.com" {
					t.Errorf("expected Endpoint match, got '%v'", r.Details["Endpoint"])
				}
				if r.Details["RoleArn"] != "arn:aws:iam::123456789012:role/eks-role" {
					t.Errorf("expected RoleArn match, got '%v'", r.Details["RoleArn"])
				}
				if r.Details["VpcId"] != "vpc-12345" {
					t.Errorf("expected VpcId 'vpc-12345', got '%v'", r.Details["VpcId"])
				}
				if r.Details["Subnets"] != 2 {
					t.Errorf("expected Subnets 2, got '%v'", r.Details["Subnets"])
				}
				if r.Details["SecurityGroups"] != 1 {
					t.Errorf("expected SecurityGroups 1, got '%v'", r.Details["SecurityGroups"])
				}
			},
		},
		{
			name: "multiple clusters",
			input: []*eks.Cluster{
				{
					Name:    aws.String("cluster-1"),
					Arn:     aws.String("arn:aws:eks:us-east-1:123456789012:cluster/cluster-1"),
					Status:  aws.String("ACTIVE"),
					Version: aws.String("1.28"),
				},
				{
					Name:    aws.String("cluster-2"),
					Arn:     aws.String("arn:aws:eks:us-west-2:123456789012:cluster/cluster-2"),
					Status:  aws.String("CREATING"),
					Version: aws.String("1.27"),
				},
			},
			expectedCount: 2,
			checkResults: func(t *testing.T, results []types.ScanResult) {
				if results[0].ResourceName != "cluster-1" {
					t.Errorf("expected first cluster 'cluster-1', got '%s'", results[0].ResourceName)
				}
				if results[1].ResourceName != "cluster-2" {
					t.Errorf("expected second cluster 'cluster-2', got '%s'", results[1].ResourceName)
				}
				if results[1].Details["Status"] != "CREATING" {
					t.Errorf("expected second cluster status 'CREATING', got '%v'", results[1].Details["Status"])
				}
			},
		},
		{
			name:          "empty results",
			input:         []*eks.Cluster{},
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
				if results[0].ServiceName != "EKS" {
					t.Errorf("expected ServiceName 'EKS', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "eks:ListClusters" {
					t.Errorf("expected MethodName 'eks:ListClusters', got '%s'", results[0].MethodName)
				}
			},
		},
		{
			name: "nil field handling",
			input: []*eks.Cluster{
				{
					Name:               nil,
					Arn:                nil,
					Status:             nil,
					Version:            nil,
					Endpoint:           nil,
					RoleArn:            nil,
					ResourcesVpcConfig: nil,
				},
			},
			expectedCount: 1,
			checkResults: func(t *testing.T, results []types.ScanResult) {
				r := results[0]
				if r.ResourceName != "" {
					t.Errorf("expected empty ResourceName for nil Name, got '%s'", r.ResourceName)
				}
				if r.Details["Arn"] != "" {
					t.Errorf("expected empty Arn for nil, got '%v'", r.Details["Arn"])
				}
				if r.Details["Status"] != "" {
					t.Errorf("expected empty Status for nil, got '%v'", r.Details["Status"])
				}
				if r.Details["Version"] != "" {
					t.Errorf("expected empty Version for nil, got '%v'", r.Details["Version"])
				}
				if r.Details["Endpoint"] != "" {
					t.Errorf("expected empty Endpoint for nil, got '%v'", r.Details["Endpoint"])
				}
				if r.Details["RoleArn"] != "" {
					t.Errorf("expected empty RoleArn for nil, got '%v'", r.Details["RoleArn"])
				}
				if r.Details["VpcId"] != "" {
					t.Errorf("expected empty VpcId for nil ResourcesVpcConfig, got '%v'", r.Details["VpcId"])
				}
				if r.Details["Subnets"] != 0 {
					t.Errorf("expected Subnets 0 for nil ResourcesVpcConfig, got '%v'", r.Details["Subnets"])
				}
				if r.Details["SecurityGroups"] != 0 {
					t.Errorf("expected SecurityGroups 0 for nil ResourcesVpcConfig, got '%v'", r.Details["SecurityGroups"])
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
