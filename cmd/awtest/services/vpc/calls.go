package vpc

import (
	"context"
	"fmt"
	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/MillerMedia/awtest/cmd/awtest/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"time"
)

// VPCInfrastructure holds aggregated VPC, Subnet, and Security Group results.
type VPCInfrastructure struct {
	VPCs           []*ec2.Vpc
	Subnets        []*ec2.Subnet
	SecurityGroups []*ec2.SecurityGroup
	PartialErrors  []error
}

var VpcCalls = []types.AWSService{
	{
		Name: "ec2:DescribeVpcs",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			infra := &VPCInfrastructure{}
			var lastErr error
			anyRegionSucceeded := false

			for _, region := range types.Regions {
				regionSess := sess.Copy(&aws.Config{Region: aws.String(region)})
				svc := ec2.New(regionSess)
				regionFailed := false

				// DescribeVpcs
				var regionVpcs []*ec2.Vpc
				vpcInput := &ec2.DescribeVpcsInput{}
				for {
					output, err := svc.DescribeVpcsWithContext(ctx, vpcInput)
					if err != nil {
						lastErr = err
						regionFailed = true
						break
					}
					regionVpcs = append(regionVpcs, output.Vpcs...)
					if output.NextToken == nil {
						break
					}
					vpcInput.NextToken = output.NextToken
				}

				if regionFailed {
					continue
				}

				// DescribeSubnets
				var regionSubnets []*ec2.Subnet
				subnetInput := &ec2.DescribeSubnetsInput{}
				for {
					output, err := svc.DescribeSubnetsWithContext(ctx, subnetInput)
					if err != nil {
						infra.PartialErrors = append(infra.PartialErrors, fmt.Errorf("ec2:DescribeSubnets in %s: %v", region, err))
						break
					}
					regionSubnets = append(regionSubnets, output.Subnets...)
					if output.NextToken == nil {
						break
					}
					subnetInput.NextToken = output.NextToken
				}

				// DescribeSecurityGroups
				var regionSGs []*ec2.SecurityGroup
				sgInput := &ec2.DescribeSecurityGroupsInput{}
				for {
					output, err := svc.DescribeSecurityGroupsWithContext(ctx, sgInput)
					if err != nil {
						infra.PartialErrors = append(infra.PartialErrors, fmt.Errorf("ec2:DescribeSecurityGroups in %s: %v", region, err))
						break
					}
					regionSGs = append(regionSGs, output.SecurityGroups...)
					if output.NextToken == nil {
						break
					}
					sgInput.NextToken = output.NextToken
				}

				infra.VPCs = append(infra.VPCs, regionVpcs...)
				infra.Subnets = append(infra.Subnets, regionSubnets...)
				infra.SecurityGroups = append(infra.SecurityGroups, regionSGs...)
				anyRegionSucceeded = true
			}

			if !anyRegionSucceeded && lastErr != nil {
				return nil, lastErr
			}
			return infra, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "ec2:DescribeVpcs", err)
				return []types.ScanResult{
					{
						ServiceName: "VPC",
						MethodName:  "ec2:DescribeVpcs",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			infra, ok := output.(*VPCInfrastructure)
			if !ok {
				utils.HandleAWSError(debug, "ec2:DescribeVpcs", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			if len(infra.VPCs) == 0 && len(infra.Subnets) == 0 && len(infra.SecurityGroups) == 0 {
				utils.PrintAccessGranted(debug, "ec2:DescribeVpcs", "VPC infrastructure")
				return []types.ScanResult{}
			}

			// Report partial errors as ScanResult entries
			for _, partialErr := range infra.PartialErrors {
				results = append(results, types.ScanResult{
					ServiceName: "VPC",
					MethodName:  "ec2:DescribeVpcs",
					Error:       partialErr,
					Timestamp:   time.Now(),
				})
			}

			results = append(results, processVPCs(infra, debug)...)
			results = append(results, processSubnets(infra, debug)...)
			results = append(results, processSecurityGroups(infra, debug)...)

			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}

func processVPCs(infra *VPCInfrastructure, debug bool) []types.ScanResult {
	// Build correlation maps
	subnetsByVPC := make(map[string]int)
	for _, subnet := range infra.Subnets {
		if subnet.VpcId != nil {
			subnetsByVPC[*subnet.VpcId]++
		}
	}

	sgsByVPC := make(map[string]int)
	for _, sg := range infra.SecurityGroups {
		if sg.VpcId != nil {
			sgsByVPC[*sg.VpcId]++
		}
	}

	var results []types.ScanResult
	for _, v := range infra.VPCs {
		vpcId := ""
		if v.VpcId != nil {
			vpcId = *v.VpcId
		}

		cidrBlock := ""
		if v.CidrBlock != nil {
			cidrBlock = *v.CidrBlock
		}

		state := ""
		if v.State != nil {
			state = *v.State
		}

		isDefault := false
		if v.IsDefault != nil {
			isDefault = *v.IsDefault
		}

		subnetCount := subnetsByVPC[vpcId]
		sgCount := sgsByVPC[vpcId]

		results = append(results, types.ScanResult{
			ServiceName:  "VPC",
			MethodName:   "ec2:DescribeVpcs",
			ResourceType: "vpc",
			ResourceName: vpcId,
			Details: map[string]interface{}{
				"VpcId":              vpcId,
				"CidrBlock":          cidrBlock,
				"State":              state,
				"IsDefault":          isDefault,
				"SubnetCount":        subnetCount,
				"SecurityGroupCount": sgCount,
			},
			Timestamp: time.Now(),
		})

		utils.PrintResult(debug, "", "ec2:DescribeVpcs",
			fmt.Sprintf("Found VPC: %s (CIDR: %s, State: %s, Default: %v, Subnets: %d, SecurityGroups: %d)",
				utils.ColorizeItem(vpcId), cidrBlock, state, isDefault, subnetCount, sgCount), nil)
	}
	return results
}

func processSubnets(infra *VPCInfrastructure, debug bool) []types.ScanResult {
	var results []types.ScanResult
	for _, s := range infra.Subnets {
		subnetId := ""
		if s.SubnetId != nil {
			subnetId = *s.SubnetId
		}

		vpcId := ""
		if s.VpcId != nil {
			vpcId = *s.VpcId
		}

		cidrBlock := ""
		if s.CidrBlock != nil {
			cidrBlock = *s.CidrBlock
		}

		az := ""
		if s.AvailabilityZone != nil {
			az = *s.AvailabilityZone
		}

		results = append(results, types.ScanResult{
			ServiceName:  "VPC",
			MethodName:   "ec2:DescribeSubnets",
			ResourceType: "subnet",
			ResourceName: subnetId,
			Details: map[string]interface{}{
				"VpcId":            vpcId,
				"CidrBlock":        cidrBlock,
				"AvailabilityZone": az,
			},
			Timestamp: time.Now(),
		})

		utils.PrintResult(debug, "", "ec2:DescribeSubnets",
			fmt.Sprintf("Found Subnet: %s (VPC: %s, CIDR: %s, AZ: %s)",
				utils.ColorizeItem(subnetId), vpcId, cidrBlock, az), nil)
	}
	return results
}

func processSecurityGroups(infra *VPCInfrastructure, debug bool) []types.ScanResult {
	var results []types.ScanResult
	for _, sg := range infra.SecurityGroups {
		groupId := ""
		if sg.GroupId != nil {
			groupId = *sg.GroupId
		}

		groupName := ""
		if sg.GroupName != nil {
			groupName = *sg.GroupName
		}

		vpcId := ""
		if sg.VpcId != nil {
			vpcId = *sg.VpcId
		}

		description := ""
		if sg.Description != nil {
			description = *sg.Description
		}

		inboundCount := 0
		if sg.IpPermissions != nil {
			inboundCount = len(sg.IpPermissions)
		}

		outboundCount := 0
		if sg.IpPermissionsEgress != nil {
			outboundCount = len(sg.IpPermissionsEgress)
		}

		results = append(results, types.ScanResult{
			ServiceName:  "VPC",
			MethodName:   "ec2:DescribeSecurityGroups",
			ResourceType: "security-group",
			ResourceName: groupId,
			Details: map[string]interface{}{
				"GroupName":     groupName,
				"VpcId":         vpcId,
				"Description":   description,
				"InboundRules":  inboundCount,
				"OutboundRules": outboundCount,
			},
			Timestamp: time.Now(),
		})

		utils.PrintResult(debug, "", "ec2:DescribeSecurityGroups",
			fmt.Sprintf("Found Security Group: %s (%s, VPC: %s, InboundRules: %d, OutboundRules: %d)",
				utils.ColorizeItem(groupId), groupName, vpcId, inboundCount, outboundCount), nil)
	}
	return results
}
