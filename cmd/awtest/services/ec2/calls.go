package ec2

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

type EC2InstanceDetails struct {
	Instance  ec2types.Instance
	UserData  string
	ElasticIP string
	Tags      []ec2types.Tag
}

var EC2Calls = []types.AWSService{
	{
		Name: "ec2:DescribeInstances",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			var allDetails []EC2InstanceDetails
			for _, region := range types.RegionsFor(ctx) {
				svc := awsv2.NewRegionalClient(cfg, region, ec2.NewFromConfig)
				input := &ec2.DescribeInstancesInput{}
				output, err := svc.DescribeInstances(ctx, input)
				if err != nil {
					return nil, err
				}
				for _, reservation := range output.Reservations {
					for _, instance := range reservation.Instances {
						// Get user data
						userData, err := getInstanceUserData(ctx, svc, aws.ToString(instance.InstanceId))
						if err != nil {
							userData = fmt.Sprintf("Error retrieving user data: %v", awsv2.Wrap(err))
						}

						// Get Elastic IP
						elasticIP := ""
						addressesOutput, err := svc.DescribeAddresses(ctx, &ec2.DescribeAddressesInput{
							Filters: []ec2types.Filter{
								{
									Name:   aws.String("instance-id"),
									Values: []string{aws.ToString(instance.InstanceId)},
								},
							},
						})
						if err == nil && len(addressesOutput.Addresses) > 0 {
							elasticIP = aws.ToString(addressesOutput.Addresses[0].PublicIp)
						}

						// Store instance details with user data, Elastic IP, and tags
						details := EC2InstanceDetails{
							Instance:  instance,
							UserData:  userData,
							ElasticIP: elasticIP,
							Tags:      instance.Tags,
						}
						allDetails = append(allDetails, details)
					}
				}
			}
			return allDetails, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "ec2:DescribeInstances", err)
				return []types.ScanResult{
					{
						ServiceName: "EC2",
						MethodName:  "ec2:DescribeInstances",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			if details, ok := output.([]EC2InstanceDetails); ok {
				for _, detail := range details {
					instance := detail.Instance
					instanceID := aws.ToString(instance.InstanceId)
					ipAddress := aws.ToString(instance.PublicIpAddress)

					stateName := ""
					if instance.State != nil {
						stateName = string(instance.State.Name)
					}

					instanceType := string(instance.InstanceType)

					// Build details map
					instanceDetails := map[string]interface{}{
						"state":      stateName,
						"type":       instanceType,
						"ip":         ipAddress,
						"elastic_ip": detail.ElasticIP,
						"user_data":  detail.UserData,
					}

					// Add tags to details
					if len(detail.Tags) > 0 {
						tags := make(map[string]string)
						for _, tag := range detail.Tags {
							if tag.Key != nil && tag.Value != nil {
								tags[*tag.Key] = *tag.Value
							}
						}
						instanceDetails["tags"] = tags
					}

					// Add instance result
					results = append(results, types.ScanResult{
						ServiceName:  "EC2",
						MethodName:   "ec2:DescribeInstances",
						ResourceType: "instance",
						ResourceName: instanceID,
						Details:      instanceDetails,
						Timestamp:    time.Now(),
					})

					// Keep backward compatibility - print results
					utils.PrintResult(debug, "", "ec2:DescribeInstances", fmt.Sprintf("EC2 instance: [ID: %s, State: %s, Type: %s, IP: %s]",
						utils.ColorizeItem(instanceID),
						stateName,
						instanceType,
						ipAddress), nil)

					// Print Elastic IP if it exists
					if detail.ElasticIP != "" {
						utils.PrintResult(debug, "", "ec2:DescribeInstances", fmt.Sprintf("Elastic IP: %s", detail.ElasticIP), nil)
					}

					// Print user data
					if detail.UserData != "" {
						utils.PrintResult(debug, "", "ec2:DescribeInstances", fmt.Sprintf("User Data for instance %s:\n%s", instanceID, detail.UserData), nil)
					}

					// Print tags if they exist
					if len(detail.Tags) > 0 {
						utils.PrintResult(debug, "", "ec2:DescribeInstances", "Tags:", nil)
						for _, tag := range detail.Tags {
							if tag.Key != nil && tag.Value != nil {
								utils.PrintResult(debug, "", "ec2:DescribeInstances", fmt.Sprintf("  %s: %s", *tag.Key, *tag.Value), nil)
							}
						}
					}
				}
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}

// getInstanceUserData retrieves the user data for a specific EC2 instance.
func getInstanceUserData(ctx context.Context, svc *ec2.Client, instanceId string) (string, error) {
	input := &ec2.DescribeInstanceAttributeInput{
		InstanceId: aws.String(instanceId),
		Attribute:  ec2types.InstanceAttributeNameUserData,
	}

	result, err := svc.DescribeInstanceAttribute(ctx, input)
	if err != nil {
		return "", err
	}

	// User data is returned as base64 encoded, so we need to decode it
	if result.UserData != nil && result.UserData.Value != nil {
		userDataBytes, err := base64.StdEncoding.DecodeString(*result.UserData.Value)
		if err != nil {
			return "", err
		}
		return string(userDataBytes), nil
	}

	return "", nil
}
