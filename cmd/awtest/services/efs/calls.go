package efs

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/efs"
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

var EfsCalls = []types.AWSService{
	{
		Name: "efs:DescribeFileSystems",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			var allFileSystems []efstypes.FileSystemDescription
			for _, region := range types.RegionsFor(ctx) {
				svc := awsv2.NewRegionalClient(cfg, region, efs.NewFromConfig)
				output, err := svc.DescribeFileSystems(ctx, &efs.DescribeFileSystemsInput{})
				if err != nil {
					return nil, err
				}
				allFileSystems = append(allFileSystems, output.FileSystems...)
			}
			return allFileSystems, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "efs:DescribeFileSystems", err)
				return []types.ScanResult{
					{
						ServiceName: "EFS",
						MethodName:  "efs:DescribeFileSystems",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			fileSystems, ok := output.([]efstypes.FileSystemDescription)
			if !ok {
				utils.HandleAWSError(debug, "efs:DescribeFileSystems", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			if len(fileSystems) == 0 {
				utils.PrintAccessGranted(debug, "efs:DescribeFileSystems", "file systems")
				return results
			}

			for _, fs := range fileSystems {
				fsId := aws.ToString(fs.FileSystemId)
				name := aws.ToString(fs.Name)
				lifecycleState := string(fs.LifeCycleState)

				var sizeValue int64
				if fs.SizeInBytes != nil {
					sizeValue = fs.SizeInBytes.Value
				}

				mountTargets := int64(fs.NumberOfMountTargets)
				encrypted := aws.ToBool(fs.Encrypted)

				sizeStr := fmt.Sprintf("%d bytes", sizeValue)

				results = append(results, types.ScanResult{
					ServiceName:  "EFS",
					MethodName:   "efs:DescribeFileSystems",
					ResourceType: "file-system",
					ResourceName: fsId,
					Details: map[string]interface{}{
						"Name":                 name,
						"LifeCycleState":       lifecycleState,
						"SizeInBytes":          sizeValue,
						"NumberOfMountTargets": mountTargets,
						"Encrypted":            encrypted,
					},
					Timestamp: time.Now(),
				})

				utils.PrintResult(debug, "", "efs:DescribeFileSystems",
					fmt.Sprintf("Found EFS: %s (Name: %s, State: %s, Size: %s, MountTargets: %d, Encrypted: %v)",
						utils.ColorizeItem(fsId), name, lifecycleState, sizeStr, mountTargets, encrypted), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
