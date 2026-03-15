package backup

import (
	"context"
	"fmt"
	"time"

	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/MillerMedia/awtest/cmd/awtest/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/backup"
)

type bkVault struct {
	Name               string
	Arn                string
	RecoveryPointCount int64
	EncryptionKeyArn   string
	CreationDate       string
	Locked             bool
	Region             string
}

type bkPlan struct {
	Name              string
	PlanId            string
	Arn               string
	CreationDate      string
	LastExecutionDate string
	VersionId         string
	Region            string
}

type bkRecoveryPoint struct {
	RecoveryPointArn string
	VaultName        string
	ResourceArn      string
	ResourceType     string
	Status           string
	CreationDate     string
	BackupSizeInBytes int64
	Region           string
}

type bkVaultPolicy struct {
	VaultName string
	VaultArn  string
	Policy    string
	Region    string
}

func isResourceNotFound(err error) bool {
	if aerr, ok := err.(awserr.Error); ok {
		return aerr.Code() == "ResourceNotFoundException"
	}
	return false
}

var BackupCalls = []types.AWSService{
	{
		Name: "backup:ListBackupVaults",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			var allVaults []bkVault
			var lastErr error

			for _, region := range types.Regions {
				svc := backup.New(sess, &aws.Config{Region: aws.String(region)})
				var nextToken *string
				for {
					input := &backup.ListBackupVaultsInput{
						MaxResults: aws.Int64(1000),
					}
					if nextToken != nil {
						input.NextToken = nextToken
					}
					output, err := svc.ListBackupVaultsWithContext(ctx, input)
					if err != nil {
						lastErr = err
						utils.HandleAWSError(false, "backup:ListBackupVaults", err)
						break
					}

					for _, v := range output.BackupVaultList {
						name := ""
						if v.BackupVaultName != nil {
							name = *v.BackupVaultName
						}

						arn := ""
						if v.BackupVaultArn != nil {
							arn = *v.BackupVaultArn
						}

						var recoveryPointCount int64
						if v.NumberOfRecoveryPoints != nil {
							recoveryPointCount = *v.NumberOfRecoveryPoints
						}

						encryptionKeyArn := ""
						if v.EncryptionKeyArn != nil {
							encryptionKeyArn = *v.EncryptionKeyArn
						}

						creationDate := ""
						if v.CreationDate != nil {
							creationDate = v.CreationDate.Format(time.RFC3339)
						}

						locked := false
						if v.Locked != nil {
							locked = *v.Locked
						}

						allVaults = append(allVaults, bkVault{
							Name:               name,
							Arn:                arn,
							RecoveryPointCount: recoveryPointCount,
							EncryptionKeyArn:   encryptionKeyArn,
							CreationDate:       creationDate,
							Locked:             locked,
							Region:             region,
						})
					}

					if output.NextToken == nil {
						break
					}
					nextToken = output.NextToken
				}
			}

			if len(allVaults) == 0 && lastErr != nil {
				return nil, lastErr
			}
			return allVaults, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "backup:ListBackupVaults", err)
				return []types.ScanResult{
					{
						ServiceName: "Backup",
						MethodName:  "backup:ListBackupVaults",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			vaults, ok := output.([]bkVault)
			if !ok {
				utils.HandleAWSError(debug, "backup:ListBackupVaults", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, v := range vaults {
				results = append(results, types.ScanResult{
					ServiceName:  "Backup",
					MethodName:   "backup:ListBackupVaults",
					ResourceType: "backup-vault",
					ResourceName: v.Name,
					Details: map[string]interface{}{
						"Name":               v.Name,
						"Arn":                v.Arn,
						"RecoveryPointCount": v.RecoveryPointCount,
						"EncryptionKeyArn":   v.EncryptionKeyArn,
						"CreationDate":       v.CreationDate,
						"Locked":             v.Locked,
						"Region":             v.Region,
					},
					Timestamp: time.Now(),
				})

				utils.PrintResult(debug, "", "backup:ListBackupVaults",
					fmt.Sprintf("Backup Vault: %s (Recovery Points: %d, Locked: %v, Region: %s)", utils.ColorizeItem(v.Name), v.RecoveryPointCount, v.Locked, v.Region), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
	{
		Name: "backup:ListBackupPlans",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			var allPlans []bkPlan
			var lastErr error

			for _, region := range types.Regions {
				svc := backup.New(sess, &aws.Config{Region: aws.String(region)})
				var nextToken *string
				for {
					input := &backup.ListBackupPlansInput{
						MaxResults: aws.Int64(1000),
					}
					if nextToken != nil {
						input.NextToken = nextToken
					}
					output, err := svc.ListBackupPlansWithContext(ctx, input)
					if err != nil {
						lastErr = err
						utils.HandleAWSError(false, "backup:ListBackupPlans", err)
						break
					}

					for _, p := range output.BackupPlansList {
						name := ""
						if p.BackupPlanName != nil {
							name = *p.BackupPlanName
						}

						planId := ""
						if p.BackupPlanId != nil {
							planId = *p.BackupPlanId
						}

						arn := ""
						if p.BackupPlanArn != nil {
							arn = *p.BackupPlanArn
						}

						creationDate := ""
						if p.CreationDate != nil {
							creationDate = p.CreationDate.Format(time.RFC3339)
						}

						lastExecutionDate := ""
						if p.LastExecutionDate != nil {
							lastExecutionDate = p.LastExecutionDate.Format(time.RFC3339)
						}

						versionId := ""
						if p.VersionId != nil {
							versionId = *p.VersionId
						}

						allPlans = append(allPlans, bkPlan{
							Name:              name,
							PlanId:            planId,
							Arn:               arn,
							CreationDate:      creationDate,
							LastExecutionDate: lastExecutionDate,
							VersionId:         versionId,
							Region:            region,
						})
					}

					if output.NextToken == nil {
						break
					}
					nextToken = output.NextToken
				}
			}

			if len(allPlans) == 0 && lastErr != nil {
				return nil, lastErr
			}
			return allPlans, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "backup:ListBackupPlans", err)
				return []types.ScanResult{
					{
						ServiceName: "Backup",
						MethodName:  "backup:ListBackupPlans",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			plans, ok := output.([]bkPlan)
			if !ok {
				utils.HandleAWSError(debug, "backup:ListBackupPlans", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, p := range plans {
				results = append(results, types.ScanResult{
					ServiceName:  "Backup",
					MethodName:   "backup:ListBackupPlans",
					ResourceType: "backup-plan",
					ResourceName: p.Name,
					Details: map[string]interface{}{
						"Name":              p.Name,
						"PlanId":            p.PlanId,
						"Arn":               p.Arn,
						"CreationDate":      p.CreationDate,
						"LastExecutionDate": p.LastExecutionDate,
						"VersionId":         p.VersionId,
						"Region":            p.Region,
					},
					Timestamp: time.Now(),
				})

				utils.PrintResult(debug, "", "backup:ListBackupPlans",
					fmt.Sprintf("Backup Plan: %s (Last Execution: %s, Region: %s)", utils.ColorizeItem(p.Name), p.LastExecutionDate, p.Region), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
	{
		Name: "backup:ListRecoveryPointsByBackupVault",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			var allRecoveryPoints []bkRecoveryPoint
			var lastErr error

			for _, region := range types.Regions {
				svc := backup.New(sess, &aws.Config{Region: aws.String(region)})

				// Step 1: List all vaults in this region
				var vaultNames []string
				var vaultNextToken *string
				for {
					vaultInput := &backup.ListBackupVaultsInput{
						MaxResults: aws.Int64(1000),
					}
					if vaultNextToken != nil {
						vaultInput.NextToken = vaultNextToken
					}
					vaultOutput, err := svc.ListBackupVaultsWithContext(ctx, vaultInput)
					if err != nil {
						lastErr = err
						utils.HandleAWSError(false, "backup:ListRecoveryPointsByBackupVault", err)
						break
					}
					for _, v := range vaultOutput.BackupVaultList {
						if v.BackupVaultName != nil {
							vaultNames = append(vaultNames, *v.BackupVaultName)
						}
					}
					if vaultOutput.NextToken == nil {
						break
					}
					vaultNextToken = vaultOutput.NextToken
				}

				// Step 2: For each vault, list recovery points
				for _, vaultName := range vaultNames {
					var nextToken *string
					for {
						input := &backup.ListRecoveryPointsByBackupVaultInput{
							BackupVaultName: aws.String(vaultName),
							MaxResults:      aws.Int64(1000),
						}
						if nextToken != nil {
							input.NextToken = nextToken
						}
						output, err := svc.ListRecoveryPointsByBackupVaultWithContext(ctx, input)
						if err != nil {
							utils.HandleAWSError(false, "backup:ListRecoveryPointsByBackupVault", err)
							break
						}

						for _, rp := range output.RecoveryPoints {
							rpArn := ""
							if rp.RecoveryPointArn != nil {
								rpArn = *rp.RecoveryPointArn
							}

							rpVaultName := ""
							if rp.BackupVaultName != nil {
								rpVaultName = *rp.BackupVaultName
							}

							resourceArn := ""
							if rp.ResourceArn != nil {
								resourceArn = *rp.ResourceArn
							}

							resourceType := ""
							if rp.ResourceType != nil {
								resourceType = *rp.ResourceType
							}

							status := ""
							if rp.Status != nil {
								status = *rp.Status
							}

							creationDate := ""
							if rp.CreationDate != nil {
								creationDate = rp.CreationDate.Format(time.RFC3339)
							}

							var backupSize int64
							if rp.BackupSizeInBytes != nil {
								backupSize = *rp.BackupSizeInBytes
							}

							allRecoveryPoints = append(allRecoveryPoints, bkRecoveryPoint{
								RecoveryPointArn:  rpArn,
								VaultName:         rpVaultName,
								ResourceArn:       resourceArn,
								ResourceType:      resourceType,
								Status:            status,
								CreationDate:      creationDate,
								BackupSizeInBytes: backupSize,
								Region:            region,
							})
						}

						if output.NextToken == nil {
							break
						}
						nextToken = output.NextToken
					}
				}
			}

			if len(allRecoveryPoints) == 0 && lastErr != nil {
				return nil, lastErr
			}
			return allRecoveryPoints, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "backup:ListRecoveryPointsByBackupVault", err)
				return []types.ScanResult{
					{
						ServiceName: "Backup",
						MethodName:  "backup:ListRecoveryPointsByBackupVault",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			recoveryPoints, ok := output.([]bkRecoveryPoint)
			if !ok {
				utils.HandleAWSError(debug, "backup:ListRecoveryPointsByBackupVault", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, rp := range recoveryPoints {
				results = append(results, types.ScanResult{
					ServiceName:  "Backup",
					MethodName:   "backup:ListRecoveryPointsByBackupVault",
					ResourceType: "recovery-point",
					ResourceName: rp.RecoveryPointArn,
					Details: map[string]interface{}{
						"RecoveryPointArn":  rp.RecoveryPointArn,
						"VaultName":         rp.VaultName,
						"ResourceArn":       rp.ResourceArn,
						"ResourceType":      rp.ResourceType,
						"Status":            rp.Status,
						"CreationDate":      rp.CreationDate,
						"BackupSizeInBytes": rp.BackupSizeInBytes,
						"Region":            rp.Region,
					},
					Timestamp: time.Now(),
				})

				utils.PrintResult(debug, "", "backup:ListRecoveryPointsByBackupVault",
					fmt.Sprintf("Backup Recovery Point: %s (Vault: %s, Resource: %s, Status: %s, Region: %s)", utils.ColorizeItem(rp.RecoveryPointArn), rp.VaultName, rp.ResourceArn, rp.Status, rp.Region), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
	{
		Name: "backup:GetBackupVaultAccessPolicy",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			var allPolicies []bkVaultPolicy
			var lastErr error

			for _, region := range types.Regions {
				svc := backup.New(sess, &aws.Config{Region: aws.String(region)})

				// Step 1: List all vaults in this region
				var vaultNames []string
				var vaultNextToken *string
				for {
					vaultInput := &backup.ListBackupVaultsInput{
						MaxResults: aws.Int64(1000),
					}
					if vaultNextToken != nil {
						vaultInput.NextToken = vaultNextToken
					}
					vaultOutput, err := svc.ListBackupVaultsWithContext(ctx, vaultInput)
					if err != nil {
						lastErr = err
						utils.HandleAWSError(false, "backup:GetBackupVaultAccessPolicy", err)
						break
					}
					for _, v := range vaultOutput.BackupVaultList {
						if v.BackupVaultName != nil {
							vaultNames = append(vaultNames, *v.BackupVaultName)
						}
					}
					if vaultOutput.NextToken == nil {
						break
					}
					vaultNextToken = vaultOutput.NextToken
				}

				// Step 2: For each vault, get access policy
				for _, vaultName := range vaultNames {
					output, err := svc.GetBackupVaultAccessPolicyWithContext(ctx, &backup.GetBackupVaultAccessPolicyInput{
						BackupVaultName: aws.String(vaultName),
					})
					if err != nil {
						if isResourceNotFound(err) {
							continue
						}
						utils.HandleAWSError(false, "backup:GetBackupVaultAccessPolicy", err)
						continue
					}

					vaultArn := ""
					if output.BackupVaultArn != nil {
						vaultArn = *output.BackupVaultArn
					}

					policy := ""
					if output.Policy != nil {
						policy = *output.Policy
					}

					allPolicies = append(allPolicies, bkVaultPolicy{
						VaultName: vaultName,
						VaultArn:  vaultArn,
						Policy:    policy,
						Region:    region,
					})
				}
			}

			if len(allPolicies) == 0 && lastErr != nil {
				return nil, lastErr
			}
			return allPolicies, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "backup:GetBackupVaultAccessPolicy", err)
				return []types.ScanResult{
					{
						ServiceName: "Backup",
						MethodName:  "backup:GetBackupVaultAccessPolicy",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			policies, ok := output.([]bkVaultPolicy)
			if !ok {
				utils.HandleAWSError(debug, "backup:GetBackupVaultAccessPolicy", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, p := range policies {
				results = append(results, types.ScanResult{
					ServiceName:  "Backup",
					MethodName:   "backup:GetBackupVaultAccessPolicy",
					ResourceType: "vault-access-policy",
					ResourceName: p.VaultName,
					Details: map[string]interface{}{
						"VaultName": p.VaultName,
						"VaultArn":  p.VaultArn,
						"Policy":    p.Policy,
						"Region":    p.Region,
					},
					Timestamp: time.Now(),
				})

				utils.PrintResult(debug, "", "backup:GetBackupVaultAccessPolicy",
					fmt.Sprintf("Backup Vault Access Policy: %s (Region: %s)", utils.ColorizeItem(p.VaultName), p.Region), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
