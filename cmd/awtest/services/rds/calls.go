package rds

import (
	"context"
	"fmt"
	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/MillerMedia/awtest/cmd/awtest/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/rds"
	"time"
)

var RDSCalls = []types.AWSService{
	{
		Name: "rds:DescribeDBInstances",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			var allDBInstances []*rds.DBInstance
			for _, region := range types.Regions {
				sess.Config.Region = aws.String(region)
				svc := rds.New(sess)
				output, err := svc.DescribeDBInstancesWithContext(ctx, &rds.DescribeDBInstancesInput{})
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

			if dbInstances, ok := output.([]*rds.DBInstance); ok {
				for _, db := range dbInstances {
					dbId := ""
					az := ""
					if db.DBInstanceIdentifier != nil {
						dbId = *db.DBInstanceIdentifier
					}
					if db.AvailabilityZone != nil {
						az = *db.AvailabilityZone
					}

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
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
