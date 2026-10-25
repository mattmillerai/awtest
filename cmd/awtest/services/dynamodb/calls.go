package dynamodb

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamodbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

var DynamoDBCalls = []types.AWSService{
	{
		Name: "dynamodb:ListTables",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			var allTableNames []string
			for _, region := range types.RegionsFor(ctx) {
				svc := awsv2.NewRegionalClient(cfg, region, dynamodb.NewFromConfig)
				output, err := svc.ListTables(ctx, &dynamodb.ListTablesInput{})
				if err != nil {
					return nil, err
				}
				allTableNames = append(allTableNames, output.TableNames...)
			}
			return allTableNames, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "dynamodb:ListTables", err)
				return []types.ScanResult{
					{
						ServiceName: "DynamoDB",
						MethodName:  "dynamodb:ListTables",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			if tableNames, ok := output.([]string); ok {
				for _, tblName := range tableNames {
					results = append(results, types.ScanResult{
						ServiceName:  "DynamoDB",
						MethodName:   "dynamodb:ListTables",
						ResourceType: "table",
						ResourceName: tblName,
						Details:      map[string]interface{}{},
						Timestamp:    time.Now(),
					})

					utils.PrintResult(debug, "", "dynamodb:ListTables", fmt.Sprintf("DynamoDB table: %s", utils.ColorizeItem(tblName)), nil)
				}
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
	{
		Name: "dynamodb:ListExports",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			var allExports []dynamodbtypes.ExportSummary
			for _, region := range types.RegionsFor(ctx) {
				svc := awsv2.NewRegionalClient(cfg, region, dynamodb.NewFromConfig)
				output, err := svc.ListExports(ctx, &dynamodb.ListExportsInput{})
				if err != nil {
					return nil, err
				}
				allExports = append(allExports, output.ExportSummaries...)
			}
			return allExports, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "dynamodb:ListExports", err)
				return []types.ScanResult{
					{
						ServiceName: "DynamoDB",
						MethodName:  "dynamodb:ListExports",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			if exports, ok := output.([]dynamodbtypes.ExportSummary); ok {
				if len(exports) == 0 {
					utils.PrintAccessGranted(debug, "dynamodb:ListExports", "DynamoDB exports")
				} else {
					for _, export := range exports {
						exportArn := aws.ToString(export.ExportArn)

						results = append(results, types.ScanResult{
							ServiceName:  "DynamoDB",
							MethodName:   "dynamodb:ListExports",
							ResourceType: "export",
							ResourceName: exportArn,
							Details:      map[string]interface{}{},
							Timestamp:    time.Now(),
						})

						utils.PrintResult(debug, "", "dynamodb:ListExports", fmt.Sprintf("DynamoDB export: %s", utils.ColorizeItem(exportArn)), nil)
					}
				}
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
	{
		Name: "dynamodb:ListBackups",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			var allBackups []dynamodbtypes.BackupSummary
			for _, region := range types.RegionsFor(ctx) {
				svc := awsv2.NewRegionalClient(cfg, region, dynamodb.NewFromConfig)
				output, err := svc.ListBackups(ctx, &dynamodb.ListBackupsInput{})
				if err != nil {
					return nil, err
				}
				allBackups = append(allBackups, output.BackupSummaries...)
			}
			return allBackups, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "dynamodb:ListBackups", err)
				return []types.ScanResult{
					{
						ServiceName: "DynamoDB",
						MethodName:  "dynamodb:ListBackups",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			if backups, ok := output.([]dynamodbtypes.BackupSummary); ok {
				if len(backups) == 0 {
					utils.PrintAccessGranted(debug, "dynamodb:ListBackups", "DynamoDB backups")
				} else {
					for _, backup := range backups {
						backupArn := aws.ToString(backup.BackupArn)

						results = append(results, types.ScanResult{
							ServiceName:  "DynamoDB",
							MethodName:   "dynamodb:ListBackups",
							ResourceType: "backup",
							ResourceName: backupArn,
							Details:      map[string]interface{}{},
							Timestamp:    time.Now(),
						})

						utils.PrintResult(debug, "", "dynamodb:ListBackups", fmt.Sprintf("DynamoDB backup: %s", utils.ColorizeItem(backupArn)), nil)
					}
				}
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
