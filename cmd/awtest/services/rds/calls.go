package rds

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

var RDSCalls = []types.AWSService{
	{
		Name: "rds:DescribeDBInstances",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			var allDBInstances []rdstypes.DBInstance
			for _, region := range types.RegionsFor(ctx) {
				svc := awsv2.NewRegionalClient(cfg, region, rds.NewFromConfig)
				output, err := svc.DescribeDBInstances(ctx, &rds.DescribeDBInstancesInput{})
				if err != nil {
					return nil, err
				}
				allDBInstances = append(allDBInstances, output.DBInstances...)
			}
			return allDBInstances, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "rds:DescribeDBInstances", err)
				return []types.ScanResult{
					{
						ServiceName: "RDS",
						MethodName:  "rds:DescribeDBInstances",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			dbInstances, ok := output.([]rdstypes.DBInstance)
			if !ok {
				utils.HandleAWSError(debug, "rds:DescribeDBInstances", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, db := range dbInstances {
				dbId := aws.ToString(db.DBInstanceIdentifier)
				az := aws.ToString(db.AvailabilityZone)

				results = append(results, types.ScanResult{
					ServiceName:  "RDS",
					MethodName:   "rds:DescribeDBInstances",
					ResourceType: "db-instance",
					ResourceName: dbId,
					Details:      map[string]interface{}{"availability_zone": az},
					Timestamp:    time.Now(),
				})

				utils.PrintResult(debug, "", "rds:DescribeDBInstances", fmt.Sprintf("Found RDS instance: %s (%s)", dbId, az), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
