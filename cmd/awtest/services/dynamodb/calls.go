package dynamodb

import (
	"context"
	"fmt"
	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/MillerMedia/awtest/cmd/awtest/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"time"
)

var DynamoDBCalls = []types.AWSService{
	{
		Name: "dynamodb:ListTables",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			var allTableNames []*string
			for _, region := range types.Regions {
				sess.Config.Region = aws.String(region)
				svc := dynamodb.New(sess)
				output, err := svc.ListTablesWithContext(ctx, &dynamodb.ListTablesInput{})
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

			if tableNames, ok := output.([]*string); ok {
				for _, tableName := range tableNames {
					tblName := ""
					if tableName != nil {
						tblName = *tableName
					}

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
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			var allExports []*dynamodb.ExportSummary
			for _, region := range types.Regions {
				sess.Config.Region = aws.String(region)
				svc := dynamodb.New(sess)
				output, err := svc.ListExportsWithContext(ctx, &dynamodb.ListExportsInput{})
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

			if exports, ok := output.([]*dynamodb.ExportSummary); ok {
				if len(exports) == 0 {
					utils.PrintAccessGranted(debug, "dynamodb:ListExports", "DynamoDB exports")
				} else {
					for _, export := range exports {
						exportArn := ""
						if export.ExportArn != nil {
							exportArn = *export.ExportArn
						}

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
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			var allBackups []*dynamodb.BackupSummary
			for _, region := range types.Regions {
				sess.Config.Region = aws.String(region)
				svc := dynamodb.New(sess)
				output, err := svc.ListBackupsWithContext(ctx, &dynamodb.ListBackupsInput{})
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

			if backups, ok := output.([]*dynamodb.BackupSummary); ok {
				if len(backups) == 0 {
					utils.PrintAccessGranted(debug, "dynamodb:ListBackups", "DynamoDB backups")
				} else {
					for _, backup := range backups {
						backupArn := ""
						if backup.BackupArn != nil {
							backupArn = *backup.BackupArn
						}

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
