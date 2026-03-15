package efs

import (
	"context"
	"fmt"
	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/MillerMedia/awtest/cmd/awtest/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/efs"
	"time"
)

var EfsCalls = []types.AWSService{
	{
		Name: "efs:DescribeFileSystems",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			var allFileSystems []*efs.FileSystemDescription
			for _, region := range types.Regions {
				regionSess := sess.Copy(&aws.Config{Region: aws.String(region)})
				svc := efs.New(regionSess)
				output, err := svc.DescribeFileSystemsWithContext(ctx, &efs.DescribeFileSystemsInput{})
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

			fileSystems, ok := output.([]*efs.FileSystemDescription)
			if !ok {
				utils.HandleAWSError(debug, "efs:DescribeFileSystems", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			if len(fileSystems) == 0 {
				utils.PrintAccessGranted(debug, "efs:DescribeFileSystems", "file systems")
				return results
			}

			for _, fs := range fileSystems {
				fsId := ""
				if fs.FileSystemId != nil {
					fsId = *fs.FileSystemId
				}

				name := ""
				if fs.Name != nil {
					name = *fs.Name
				}

				lifecycleState := ""
				if fs.LifeCycleState != nil {
					lifecycleState = *fs.LifeCycleState
				}

				var sizeValue int64
				if fs.SizeInBytes != nil && fs.SizeInBytes.Value != nil {
					sizeValue = *fs.SizeInBytes.Value
				}

				var mountTargets int64
				if fs.NumberOfMountTargets != nil {
					mountTargets = *fs.NumberOfMountTargets
				}

				encrypted := false
				if fs.Encrypted != nil {
					encrypted = *fs.Encrypted
				}

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
