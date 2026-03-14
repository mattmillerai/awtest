package vpc

import (
	"fmt"
	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"testing"
)

func TestProcess(t *testing.T) {
	process := VpcCalls[0].Process

	tests := []struct {
		name          string
		input         interface{}
		err           error
		expectedCount int
		expectError   bool
		checkResults  func(t *testing.T, results []types.ScanResult)
	}{
		{
			name: "VPC with subnets and security groups",
			input: &VPCInfrastructure{
				VPCs: []*ec2.Vpc{
					{
						VpcId:     aws.String("vpc-0123456789abcdef0"),
						CidrBlock: aws.String("10.0.0.0/16"),
						State:     aws.String("available"),
						IsDefault: aws.Bool(false),
					},
				},
				Subnets: []*ec2.Subnet{
					{
						SubnetId:         aws.String("subnet-aaa"),
						VpcId:            aws.String("vpc-0123456789abcdef0"),
						CidrBlock:        aws.String("10.0.1.0/24"),
						AvailabilityZone: aws.String("us-east-1a"),
					},
					{
						SubnetId:         aws.String("subnet-bbb"),
						VpcId:            aws.String("vpc-0123456789abcdef0"),
						CidrBlock:        aws.String("10.0.2.0/24"),
						AvailabilityZone: aws.String("us-east-1b"),
					},
				},
				SecurityGroups: []*ec2.SecurityGroup{
					{
						GroupId:             aws.String("sg-aaa"),
						GroupName:           aws.String("default"),
						VpcId:               aws.String("vpc-0123456789abcdef0"),
						Description:         aws.String("default VPC security group"),
						IpPermissions:       []*ec2.IpPermission{{IpProtocol: aws.String("-1")}},
						IpPermissionsEgress: []*ec2.IpPermission{{IpProtocol: aws.String("-1")}},
					},
					{
						GroupId:             aws.String("sg-bbb"),
						GroupName:           aws.String("web"),
						VpcId:               aws.String("vpc-0123456789abcdef0"),
						Description:         aws.String("web security group"),
						IpPermissions:       []*ec2.IpPermission{{IpProtocol: aws.String("tcp")}, {IpProtocol: aws.String("tcp")}},
						IpPermissionsEgress: []*ec2.IpPermission{},
					},
				},
			},
			expectedCount: 5, // 1 VPC + 2 subnets + 2 SGs
			checkResults: func(t *testing.T, results []types.ScanResult) {
				// Check VPC result
				r := results[0]
				if r.ServiceName != "VPC" {
					t.Errorf("expected ServiceName 'VPC', got '%s'", r.ServiceName)
				}
				if r.MethodName != "ec2:DescribeVpcs" {
					t.Errorf("expected MethodName 'ec2:DescribeVpcs', got '%s'", r.MethodName)
				}
				if r.ResourceType != "vpc" {
					t.Errorf("expected ResourceType 'vpc', got '%s'", r.ResourceType)
				}
				if r.ResourceName != "vpc-0123456789abcdef0" {
					t.Errorf("expected ResourceName 'vpc-0123456789abcdef0', got '%s'", r.ResourceName)
				}
				if r.Details["VpcId"] != "vpc-0123456789abcdef0" {
					t.Errorf("expected VpcId 'vpc-0123456789abcdef0', got '%v'", r.Details["VpcId"])
				}
				if r.Details["CidrBlock"] != "10.0.0.0/16" {
					t.Errorf("expected CidrBlock '10.0.0.0/16', got '%v'", r.Details["CidrBlock"])
				}
				if r.Details["State"] != "available" {
					t.Errorf("expected State 'available', got '%v'", r.Details["State"])
				}
				if r.Details["IsDefault"] != false {
					t.Errorf("expected IsDefault false, got '%v'", r.Details["IsDefault"])
				}
				if r.Details["SubnetCount"] != 2 {
					t.Errorf("expected SubnetCount 2, got '%v'", r.Details["SubnetCount"])
				}
				if r.Details["SecurityGroupCount"] != 2 {
					t.Errorf("expected SecurityGroupCount 2, got '%v'", r.Details["SecurityGroupCount"])
				}

				// Check subnet result
				s := results[1]
				if s.ResourceType != "subnet" {
					t.Errorf("expected ResourceType 'subnet', got '%s'", s.ResourceType)
				}
				if s.MethodName != "ec2:DescribeSubnets" {
					t.Errorf("expected MethodName 'ec2:DescribeSubnets', got '%s'", s.MethodName)
				}
				if s.ResourceName != "subnet-aaa" {
					t.Errorf("expected ResourceName 'subnet-aaa', got '%s'", s.ResourceName)
				}
				if s.Details["VpcId"] != "vpc-0123456789abcdef0" {
					t.Errorf("expected VpcId 'vpc-0123456789abcdef0', got '%v'", s.Details["VpcId"])
				}

				// Check security group result
				sg := results[3]
				if sg.ResourceType != "security-group" {
					t.Errorf("expected ResourceType 'security-group', got '%s'", sg.ResourceType)
				}
				if sg.MethodName != "ec2:DescribeSecurityGroups" {
					t.Errorf("expected MethodName 'ec2:DescribeSecurityGroups', got '%s'", sg.MethodName)
				}
				if sg.Details["InboundRules"] != 1 {
					t.Errorf("expected InboundRules 1, got '%v'", sg.Details["InboundRules"])
				}
				if sg.Details["OutboundRules"] != 1 {
					t.Errorf("expected OutboundRules 1, got '%v'", sg.Details["OutboundRules"])
				}

				// Check second SG has 2 inbound rules
				sg2 := results[4]
				if sg2.Details["InboundRules"] != 2 {
					t.Errorf("expected InboundRules 2, got '%v'", sg2.Details["InboundRules"])
				}
			},
		},
		{
			name: "default VPC",
			input: &VPCInfrastructure{
				VPCs: []*ec2.Vpc{
					{
						VpcId:     aws.String("vpc-default"),
						CidrBlock: aws.String("172.31.0.0/16"),
						State:     aws.String("available"),
						IsDefault: aws.Bool(true),
					},
				},
				Subnets:        []*ec2.Subnet{},
				SecurityGroups: []*ec2.SecurityGroup{},
			},
			expectedCount: 1,
			checkResults: func(t *testing.T, results []types.ScanResult) {
				r := results[0]
				if r.Details["IsDefault"] != true {
					t.Errorf("expected IsDefault true, got '%v'", r.Details["IsDefault"])
				}
				if r.Details["SubnetCount"] != 0 {
					t.Errorf("expected SubnetCount 0, got '%v'", r.Details["SubnetCount"])
				}
				if r.Details["SecurityGroupCount"] != 0 {
					t.Errorf("expected SecurityGroupCount 0, got '%v'", r.Details["SecurityGroupCount"])
				}
			},
		},
		{
			name: "multiple VPCs with mixed subnets and SGs",
			input: &VPCInfrastructure{
				VPCs: []*ec2.Vpc{
					{VpcId: aws.String("vpc-aaa"), CidrBlock: aws.String("10.0.0.0/16"), State: aws.String("available"), IsDefault: aws.Bool(false)},
					{VpcId: aws.String("vpc-bbb"), CidrBlock: aws.String("10.1.0.0/16"), State: aws.String("available"), IsDefault: aws.Bool(true)},
				},
				Subnets: []*ec2.Subnet{
					{SubnetId: aws.String("subnet-1"), VpcId: aws.String("vpc-aaa"), CidrBlock: aws.String("10.0.1.0/24"), AvailabilityZone: aws.String("us-east-1a")},
					{SubnetId: aws.String("subnet-2"), VpcId: aws.String("vpc-aaa"), CidrBlock: aws.String("10.0.2.0/24"), AvailabilityZone: aws.String("us-east-1b")},
					{SubnetId: aws.String("subnet-3"), VpcId: aws.String("vpc-bbb"), CidrBlock: aws.String("10.1.1.0/24"), AvailabilityZone: aws.String("us-east-1a")},
				},
				SecurityGroups: []*ec2.SecurityGroup{
					{GroupId: aws.String("sg-1"), GroupName: aws.String("default"), VpcId: aws.String("vpc-aaa"), Description: aws.String("default"), IpPermissions: []*ec2.IpPermission{}, IpPermissionsEgress: []*ec2.IpPermission{}},
				},
			},
			expectedCount: 6, // 2 VPCs + 3 subnets + 1 SG
			checkResults: func(t *testing.T, results []types.ScanResult) {
				// VPC aaa has 2 subnets and 1 SG
				if results[0].Details["SubnetCount"] != 2 {
					t.Errorf("expected vpc-aaa SubnetCount 2, got '%v'", results[0].Details["SubnetCount"])
				}
				if results[0].Details["SecurityGroupCount"] != 1 {
					t.Errorf("expected vpc-aaa SecurityGroupCount 1, got '%v'", results[0].Details["SecurityGroupCount"])
				}
				// VPC bbb has 1 subnet and 0 SGs
				if results[1].Details["SubnetCount"] != 1 {
					t.Errorf("expected vpc-bbb SubnetCount 1, got '%v'", results[1].Details["SubnetCount"])
				}
				if results[1].Details["SecurityGroupCount"] != 0 {
					t.Errorf("expected vpc-bbb SecurityGroupCount 0, got '%v'", results[1].Details["SecurityGroupCount"])
				}
			},
		},
		{
			name: "empty results",
			input: &VPCInfrastructure{
				VPCs:           []*ec2.Vpc{},
				Subnets:        []*ec2.Subnet{},
				SecurityGroups: []*ec2.SecurityGroup{},
			},
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
				if results[0].ServiceName != "VPC" {
					t.Errorf("expected ServiceName 'VPC', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "ec2:DescribeVpcs" {
					t.Errorf("expected MethodName 'ec2:DescribeVpcs', got '%s'", results[0].MethodName)
				}
			},
		},
		{
			name: "partial errors from subnet and SG enumeration",
			input: &VPCInfrastructure{
				VPCs: []*ec2.Vpc{
					{VpcId: aws.String("vpc-partial"), CidrBlock: aws.String("10.0.0.0/16"), State: aws.String("available"), IsDefault: aws.Bool(false)},
				},
				Subnets:        []*ec2.Subnet{},
				SecurityGroups: []*ec2.SecurityGroup{},
				PartialErrors: []error{
					fmt.Errorf("ec2:DescribeSubnets in us-east-1: AccessDeniedException"),
					fmt.Errorf("ec2:DescribeSecurityGroups in us-east-1: AccessDeniedException"),
				},
			},
			expectedCount: 3, // 2 partial error results + 1 VPC
			checkResults: func(t *testing.T, results []types.ScanResult) {
				// First two results should be partial errors
				if results[0].Error == nil {
					t.Error("expected error in first partial error result, got nil")
				}
				if results[1].Error == nil {
					t.Error("expected error in second partial error result, got nil")
				}
				// Third result should be the VPC
				if results[2].ResourceType != "vpc" {
					t.Errorf("expected ResourceType 'vpc', got '%s'", results[2].ResourceType)
				}
				if results[2].ResourceName != "vpc-partial" {
					t.Errorf("expected ResourceName 'vpc-partial', got '%s'", results[2].ResourceName)
				}
			},
		},
		{
			name: "nil field handling",
			input: &VPCInfrastructure{
				VPCs: []*ec2.Vpc{
					{
						VpcId:     nil,
						CidrBlock: nil,
						State:     nil,
						IsDefault: nil,
					},
				},
				Subnets: []*ec2.Subnet{
					{
						SubnetId:         nil,
						VpcId:            nil,
						CidrBlock:        nil,
						AvailabilityZone: nil,
					},
				},
				SecurityGroups: []*ec2.SecurityGroup{
					{
						GroupId:             nil,
						GroupName:           nil,
						VpcId:               nil,
						Description:         nil,
						IpPermissions:       nil,
						IpPermissionsEgress: nil,
					},
				},
			},
			expectedCount: 3, // 1 VPC + 1 subnet + 1 SG
			checkResults: func(t *testing.T, results []types.ScanResult) {
				// VPC with nil fields
				r := results[0]
				if r.ResourceName != "" {
					t.Errorf("expected empty ResourceName for nil VpcId, got '%s'", r.ResourceName)
				}
				if r.Details["CidrBlock"] != "" {
					t.Errorf("expected empty CidrBlock for nil, got '%v'", r.Details["CidrBlock"])
				}
				if r.Details["State"] != "" {
					t.Errorf("expected empty State for nil, got '%v'", r.Details["State"])
				}
				if r.Details["IsDefault"] != false {
					t.Errorf("expected IsDefault false for nil, got '%v'", r.Details["IsDefault"])
				}

				// Subnet with nil fields
				s := results[1]
				if s.ResourceName != "" {
					t.Errorf("expected empty ResourceName for nil SubnetId, got '%s'", s.ResourceName)
				}
				if s.Details["VpcId"] != "" {
					t.Errorf("expected empty VpcId for nil, got '%v'", s.Details["VpcId"])
				}

				// Security Group with nil fields
				sg := results[2]
				if sg.ResourceName != "" {
					t.Errorf("expected empty ResourceName for nil GroupId, got '%s'", sg.ResourceName)
				}
				if sg.Details["InboundRules"] != 0 {
					t.Errorf("expected InboundRules 0 for nil, got '%v'", sg.Details["InboundRules"])
				}
				if sg.Details["OutboundRules"] != 0 {
					t.Errorf("expected OutboundRules 0 for nil, got '%v'", sg.Details["OutboundRules"])
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
