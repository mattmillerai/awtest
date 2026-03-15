package backup

import (
	"fmt"
	"testing"
)

func TestListBackupVaultsProcess(t *testing.T) {
	process := BackupCalls[0].Process

	tests := []struct {
		name               string
		output             interface{}
		err                error
		wantLen            int
		wantError          bool
		wantResourceName   string
		wantName           string
		wantArn            string
		wantRecoveryCount  int64
		wantEncryptionKey  string
		wantCreationDate   string
		wantLocked         bool
		wantRegion         string
	}{
		{
			name: "valid vaults with full details",
			output: []bkVault{
				{
					Name:               "my-backup-vault",
					Arn:                "arn:aws:backup:us-east-1:111111111111:backup-vault:my-backup-vault",
					RecoveryPointCount: 42,
					EncryptionKeyArn:   "arn:aws:kms:us-east-1:111111111111:key/12345",
					CreationDate:       "2026-01-15T10:00:00Z",
					Locked:             true,
					Region:             "us-east-1",
				},
				{
					Name:               "dev-vault",
					Arn:                "arn:aws:backup:us-west-2:111111111111:backup-vault:dev-vault",
					RecoveryPointCount: 5,
					EncryptionKeyArn:   "arn:aws:kms:us-west-2:111111111111:key/67890",
					CreationDate:       "2026-02-20T14:00:00Z",
					Locked:             false,
					Region:             "us-west-2",
				},
			},
			wantLen:            2,
			wantResourceName:   "my-backup-vault",
			wantName:           "my-backup-vault",
			wantArn:            "arn:aws:backup:us-east-1:111111111111:backup-vault:my-backup-vault",
			wantRecoveryCount:  42,
			wantEncryptionKey:  "arn:aws:kms:us-east-1:111111111111:key/12345",
			wantCreationDate:   "2026-01-15T10:00:00Z",
			wantLocked:         true,
			wantRegion:         "us-east-1",
		},
		{
			name:    "empty results",
			output:  []bkVault{},
			wantLen: 0,
		},
		{
			name:      "access denied error",
			output:    nil,
			err:       fmt.Errorf("AccessDeniedException: User is not authorized"),
			wantLen:   1,
			wantError: true,
		},
		{
			name: "nil-safe fields (empty strings)",
			output: []bkVault{
				{
					Name:               "",
					Arn:                "",
					RecoveryPointCount: 0,
					EncryptionKeyArn:   "",
					CreationDate:       "",
					Locked:             false,
					Region:             "",
				},
			},
			wantLen:            1,
			wantResourceName:   "",
			wantName:           "",
			wantArn:            "",
			wantRecoveryCount:  0,
			wantEncryptionKey:  "",
			wantCreationDate:   "",
			wantLocked:         false,
			wantRegion:         "",
		},
		{
			name:    "type assertion failure",
			output:  "invalid type",
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := process(tt.output, tt.err, false)

			if len(results) != tt.wantLen {
				t.Fatalf("expected %d results, got %d", tt.wantLen, len(results))
			}

			if tt.wantError {
				if results[0].Error == nil {
					t.Error("expected error in result, got nil")
				}
				if results[0].ServiceName != "Backup" {
					t.Errorf("expected ServiceName 'Backup', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "backup:ListBackupVaults" {
					t.Errorf("expected MethodName 'backup:ListBackupVaults', got '%s'", results[0].MethodName)
				}
				return
			}

			if tt.wantLen > 0 {
				if results[0].ServiceName != "Backup" {
					t.Errorf("expected ServiceName 'Backup', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "backup:ListBackupVaults" {
					t.Errorf("expected MethodName 'backup:ListBackupVaults', got '%s'", results[0].MethodName)
				}
				if results[0].ResourceType != "backup-vault" {
					t.Errorf("expected ResourceType 'backup-vault', got '%s'", results[0].ResourceType)
				}
				if results[0].ResourceName != tt.wantResourceName {
					t.Errorf("expected ResourceName '%s', got '%s'", tt.wantResourceName, results[0].ResourceName)
				}
				if name, ok := results[0].Details["Name"].(string); ok {
					if name != tt.wantName {
						t.Errorf("expected Name '%s', got '%s'", tt.wantName, name)
					}
				} else if tt.wantName != "" {
					t.Errorf("expected Name in Details, got none")
				}
				if arn, ok := results[0].Details["Arn"].(string); ok {
					if arn != tt.wantArn {
						t.Errorf("expected Arn '%s', got '%s'", tt.wantArn, arn)
					}
				} else if tt.wantArn != "" {
					t.Errorf("expected Arn in Details, got none")
				}
				if count, ok := results[0].Details["RecoveryPointCount"].(int64); ok {
					if count != tt.wantRecoveryCount {
						t.Errorf("expected RecoveryPointCount %d, got %d", tt.wantRecoveryCount, count)
					}
				}
				if key, ok := results[0].Details["EncryptionKeyArn"].(string); ok {
					if key != tt.wantEncryptionKey {
						t.Errorf("expected EncryptionKeyArn '%s', got '%s'", tt.wantEncryptionKey, key)
					}
				} else if tt.wantEncryptionKey != "" {
					t.Errorf("expected EncryptionKeyArn in Details, got none")
				}
				if date, ok := results[0].Details["CreationDate"].(string); ok {
					if date != tt.wantCreationDate {
						t.Errorf("expected CreationDate '%s', got '%s'", tt.wantCreationDate, date)
					}
				} else if tt.wantCreationDate != "" {
					t.Errorf("expected CreationDate in Details, got none")
				}
				if locked, ok := results[0].Details["Locked"].(bool); ok {
					if locked != tt.wantLocked {
						t.Errorf("expected Locked %v, got %v", tt.wantLocked, locked)
					}
				}
				if region, ok := results[0].Details["Region"].(string); ok {
					if region != tt.wantRegion {
						t.Errorf("expected Region '%s', got '%s'", tt.wantRegion, region)
					}
				} else if tt.wantRegion != "" {
					t.Errorf("expected Region in Details, got none")
				}
			}
		})
	}
}

func TestListBackupPlansProcess(t *testing.T) {
	process := BackupCalls[1].Process

	tests := []struct {
		name              string
		output            interface{}
		err               error
		wantLen           int
		wantError         bool
		wantResourceName  string
		wantName          string
		wantPlanId        string
		wantArn           string
		wantCreationDate  string
		wantLastExecution string
		wantVersionId     string
		wantRegion        string
	}{
		{
			name: "valid plans with full details",
			output: []bkPlan{
				{
					Name:              "daily-backup-plan",
					PlanId:            "plan-12345",
					Arn:               "arn:aws:backup:us-east-1:111111111111:backup-plan:plan-12345",
					CreationDate:      "2026-01-10T08:00:00Z",
					LastExecutionDate: "2026-03-12T02:00:00Z",
					VersionId:         "version-abc",
					Region:            "us-east-1",
				},
				{
					Name:              "weekly-backup-plan",
					PlanId:            "plan-67890",
					Arn:               "arn:aws:backup:us-west-2:111111111111:backup-plan:plan-67890",
					CreationDate:      "2026-02-01T12:00:00Z",
					LastExecutionDate: "2026-03-10T06:00:00Z",
					VersionId:         "version-def",
					Region:            "us-west-2",
				},
			},
			wantLen:           2,
			wantResourceName:  "daily-backup-plan",
			wantName:          "daily-backup-plan",
			wantPlanId:        "plan-12345",
			wantArn:           "arn:aws:backup:us-east-1:111111111111:backup-plan:plan-12345",
			wantCreationDate:  "2026-01-10T08:00:00Z",
			wantLastExecution: "2026-03-12T02:00:00Z",
			wantVersionId:     "version-abc",
			wantRegion:        "us-east-1",
		},
		{
			name:    "empty results",
			output:  []bkPlan{},
			wantLen: 0,
		},
		{
			name:      "access denied error",
			output:    nil,
			err:       fmt.Errorf("AccessDeniedException: User is not authorized"),
			wantLen:   1,
			wantError: true,
		},
		{
			name: "nil-safe fields (empty strings)",
			output: []bkPlan{
				{
					Name:              "",
					PlanId:            "",
					Arn:               "",
					CreationDate:      "",
					LastExecutionDate: "",
					VersionId:         "",
					Region:            "",
				},
			},
			wantLen:           1,
			wantResourceName:  "",
			wantName:          "",
			wantPlanId:        "",
			wantArn:           "",
			wantCreationDate:  "",
			wantLastExecution: "",
			wantVersionId:     "",
			wantRegion:        "",
		},
		{
			name:    "type assertion failure",
			output:  "invalid type",
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := process(tt.output, tt.err, false)

			if len(results) != tt.wantLen {
				t.Fatalf("expected %d results, got %d", tt.wantLen, len(results))
			}

			if tt.wantError {
				if results[0].Error == nil {
					t.Error("expected error in result, got nil")
				}
				if results[0].ServiceName != "Backup" {
					t.Errorf("expected ServiceName 'Backup', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "backup:ListBackupPlans" {
					t.Errorf("expected MethodName 'backup:ListBackupPlans', got '%s'", results[0].MethodName)
				}
				return
			}

			if tt.wantLen > 0 {
				if results[0].ServiceName != "Backup" {
					t.Errorf("expected ServiceName 'Backup', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "backup:ListBackupPlans" {
					t.Errorf("expected MethodName 'backup:ListBackupPlans', got '%s'", results[0].MethodName)
				}
				if results[0].ResourceType != "backup-plan" {
					t.Errorf("expected ResourceType 'backup-plan', got '%s'", results[0].ResourceType)
				}
				if results[0].ResourceName != tt.wantResourceName {
					t.Errorf("expected ResourceName '%s', got '%s'", tt.wantResourceName, results[0].ResourceName)
				}
				if name, ok := results[0].Details["Name"].(string); ok {
					if name != tt.wantName {
						t.Errorf("expected Name '%s', got '%s'", tt.wantName, name)
					}
				} else if tt.wantName != "" {
					t.Errorf("expected Name in Details, got none")
				}
				if planId, ok := results[0].Details["PlanId"].(string); ok {
					if planId != tt.wantPlanId {
						t.Errorf("expected PlanId '%s', got '%s'", tt.wantPlanId, planId)
					}
				} else if tt.wantPlanId != "" {
					t.Errorf("expected PlanId in Details, got none")
				}
				if arn, ok := results[0].Details["Arn"].(string); ok {
					if arn != tt.wantArn {
						t.Errorf("expected Arn '%s', got '%s'", tt.wantArn, arn)
					}
				} else if tt.wantArn != "" {
					t.Errorf("expected Arn in Details, got none")
				}
				if date, ok := results[0].Details["CreationDate"].(string); ok {
					if date != tt.wantCreationDate {
						t.Errorf("expected CreationDate '%s', got '%s'", tt.wantCreationDate, date)
					}
				} else if tt.wantCreationDate != "" {
					t.Errorf("expected CreationDate in Details, got none")
				}
				if lastExec, ok := results[0].Details["LastExecutionDate"].(string); ok {
					if lastExec != tt.wantLastExecution {
						t.Errorf("expected LastExecutionDate '%s', got '%s'", tt.wantLastExecution, lastExec)
					}
				} else if tt.wantLastExecution != "" {
					t.Errorf("expected LastExecutionDate in Details, got none")
				}
				if versionId, ok := results[0].Details["VersionId"].(string); ok {
					if versionId != tt.wantVersionId {
						t.Errorf("expected VersionId '%s', got '%s'", tt.wantVersionId, versionId)
					}
				} else if tt.wantVersionId != "" {
					t.Errorf("expected VersionId in Details, got none")
				}
				if region, ok := results[0].Details["Region"].(string); ok {
					if region != tt.wantRegion {
						t.Errorf("expected Region '%s', got '%s'", tt.wantRegion, region)
					}
				} else if tt.wantRegion != "" {
					t.Errorf("expected Region in Details, got none")
				}
			}
		})
	}
}

func TestListRecoveryPointsByBackupVaultProcess(t *testing.T) {
	process := BackupCalls[2].Process

	tests := []struct {
		name             string
		output           interface{}
		err              error
		wantLen          int
		wantError        bool
		wantResourceName string
		wantRPArn        string
		wantVaultName    string
		wantResourceArn  string
		wantResourceType string
		wantStatus       string
		wantCreationDate string
		wantBackupSize   int64
		wantRegion       string
	}{
		{
			name: "valid recovery points with full details",
			output: []bkRecoveryPoint{
				{
					RecoveryPointArn:  "arn:aws:backup:us-east-1:111111111111:recovery-point:rp-12345",
					VaultName:         "my-backup-vault",
					ResourceArn:       "arn:aws:ec2:us-east-1:111111111111:volume/vol-abc",
					ResourceType:      "EBS",
					Status:            "COMPLETED",
					CreationDate:      "2026-03-01T02:00:00Z",
					BackupSizeInBytes: 1073741824,
					Region:            "us-east-1",
				},
				{
					RecoveryPointArn:  "arn:aws:backup:us-west-2:111111111111:recovery-point:rp-67890",
					VaultName:         "dev-vault",
					ResourceArn:       "arn:aws:rds:us-west-2:111111111111:db:mydb",
					ResourceType:      "RDS",
					Status:            "COMPLETED",
					CreationDate:      "2026-03-10T06:00:00Z",
					BackupSizeInBytes: 5368709120,
					Region:            "us-west-2",
				},
			},
			wantLen:          2,
			wantResourceName: "arn:aws:backup:us-east-1:111111111111:recovery-point:rp-12345",
			wantRPArn:        "arn:aws:backup:us-east-1:111111111111:recovery-point:rp-12345",
			wantVaultName:    "my-backup-vault",
			wantResourceArn:  "arn:aws:ec2:us-east-1:111111111111:volume/vol-abc",
			wantResourceType: "EBS",
			wantStatus:       "COMPLETED",
			wantCreationDate: "2026-03-01T02:00:00Z",
			wantBackupSize:   1073741824,
			wantRegion:       "us-east-1",
		},
		{
			name:    "empty results",
			output:  []bkRecoveryPoint{},
			wantLen: 0,
		},
		{
			name:      "access denied error",
			output:    nil,
			err:       fmt.Errorf("AccessDeniedException: User is not authorized"),
			wantLen:   1,
			wantError: true,
		},
		{
			name: "nil-safe fields (empty strings)",
			output: []bkRecoveryPoint{
				{
					RecoveryPointArn:  "",
					VaultName:         "",
					ResourceArn:       "",
					ResourceType:      "",
					Status:            "",
					CreationDate:      "",
					BackupSizeInBytes: 0,
					Region:            "",
				},
			},
			wantLen:          1,
			wantResourceName: "",
			wantRPArn:        "",
			wantVaultName:    "",
			wantResourceArn:  "",
			wantResourceType: "",
			wantStatus:       "",
			wantCreationDate: "",
			wantBackupSize:   0,
			wantRegion:       "",
		},
		{
			name:    "type assertion failure",
			output:  "invalid type",
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := process(tt.output, tt.err, false)

			if len(results) != tt.wantLen {
				t.Fatalf("expected %d results, got %d", tt.wantLen, len(results))
			}

			if tt.wantError {
				if results[0].Error == nil {
					t.Error("expected error in result, got nil")
				}
				if results[0].ServiceName != "Backup" {
					t.Errorf("expected ServiceName 'Backup', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "backup:ListRecoveryPointsByBackupVault" {
					t.Errorf("expected MethodName 'backup:ListRecoveryPointsByBackupVault', got '%s'", results[0].MethodName)
				}
				return
			}

			if tt.wantLen > 0 {
				if results[0].ServiceName != "Backup" {
					t.Errorf("expected ServiceName 'Backup', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "backup:ListRecoveryPointsByBackupVault" {
					t.Errorf("expected MethodName 'backup:ListRecoveryPointsByBackupVault', got '%s'", results[0].MethodName)
				}
				if results[0].ResourceType != "recovery-point" {
					t.Errorf("expected ResourceType 'recovery-point', got '%s'", results[0].ResourceType)
				}
				if results[0].ResourceName != tt.wantResourceName {
					t.Errorf("expected ResourceName '%s', got '%s'", tt.wantResourceName, results[0].ResourceName)
				}
				if rpArn, ok := results[0].Details["RecoveryPointArn"].(string); ok {
					if rpArn != tt.wantRPArn {
						t.Errorf("expected RecoveryPointArn '%s', got '%s'", tt.wantRPArn, rpArn)
					}
				} else if tt.wantRPArn != "" {
					t.Errorf("expected RecoveryPointArn in Details, got none")
				}
				if vaultName, ok := results[0].Details["VaultName"].(string); ok {
					if vaultName != tt.wantVaultName {
						t.Errorf("expected VaultName '%s', got '%s'", tt.wantVaultName, vaultName)
					}
				} else if tt.wantVaultName != "" {
					t.Errorf("expected VaultName in Details, got none")
				}
				if resourceArn, ok := results[0].Details["ResourceArn"].(string); ok {
					if resourceArn != tt.wantResourceArn {
						t.Errorf("expected ResourceArn '%s', got '%s'", tt.wantResourceArn, resourceArn)
					}
				} else if tt.wantResourceArn != "" {
					t.Errorf("expected ResourceArn in Details, got none")
				}
				if resourceType, ok := results[0].Details["ResourceType"].(string); ok {
					if resourceType != tt.wantResourceType {
						t.Errorf("expected ResourceType '%s', got '%s'", tt.wantResourceType, resourceType)
					}
				} else if tt.wantResourceType != "" {
					t.Errorf("expected ResourceType in Details, got none")
				}
				if status, ok := results[0].Details["Status"].(string); ok {
					if status != tt.wantStatus {
						t.Errorf("expected Status '%s', got '%s'", tt.wantStatus, status)
					}
				} else if tt.wantStatus != "" {
					t.Errorf("expected Status in Details, got none")
				}
				if date, ok := results[0].Details["CreationDate"].(string); ok {
					if date != tt.wantCreationDate {
						t.Errorf("expected CreationDate '%s', got '%s'", tt.wantCreationDate, date)
					}
				} else if tt.wantCreationDate != "" {
					t.Errorf("expected CreationDate in Details, got none")
				}
				if size, ok := results[0].Details["BackupSizeInBytes"].(int64); ok {
					if size != tt.wantBackupSize {
						t.Errorf("expected BackupSizeInBytes %d, got %d", tt.wantBackupSize, size)
					}
				}
				if region, ok := results[0].Details["Region"].(string); ok {
					if region != tt.wantRegion {
						t.Errorf("expected Region '%s', got '%s'", tt.wantRegion, region)
					}
				} else if tt.wantRegion != "" {
					t.Errorf("expected Region in Details, got none")
				}
			}
		})
	}
}

func TestGetBackupVaultAccessPolicyProcess(t *testing.T) {
	process := BackupCalls[3].Process

	tests := []struct {
		name             string
		output           interface{}
		err              error
		wantLen          int
		wantError        bool
		wantResourceName string
		wantVaultName    string
		wantVaultArn     string
		wantPolicy       string
		wantRegion       string
	}{
		{
			name: "valid policies with full details",
			output: []bkVaultPolicy{
				{
					VaultName: "my-backup-vault",
					VaultArn:  "arn:aws:backup:us-east-1:111111111111:backup-vault:my-backup-vault",
					Policy:    `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":"*","Action":"backup:CopyIntoBackupVault","Resource":"*"}]}`,
					Region:    "us-east-1",
				},
				{
					VaultName: "shared-vault",
					VaultArn:  "arn:aws:backup:us-west-2:111111111111:backup-vault:shared-vault",
					Policy:    `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":"arn:aws:iam::222222222222:root"},"Action":"backup:CopyIntoBackupVault","Resource":"*"}]}`,
					Region:    "us-west-2",
				},
			},
			wantLen:          2,
			wantResourceName: "my-backup-vault",
			wantVaultName:    "my-backup-vault",
			wantVaultArn:     "arn:aws:backup:us-east-1:111111111111:backup-vault:my-backup-vault",
			wantPolicy:       `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":"*","Action":"backup:CopyIntoBackupVault","Resource":"*"}]}`,
			wantRegion:       "us-east-1",
		},
		{
			name:    "empty results",
			output:  []bkVaultPolicy{},
			wantLen: 0,
		},
		{
			name:      "access denied error",
			output:    nil,
			err:       fmt.Errorf("AccessDeniedException: User is not authorized"),
			wantLen:   1,
			wantError: true,
		},
		{
			name: "nil-safe fields (empty strings)",
			output: []bkVaultPolicy{
				{
					VaultName: "",
					VaultArn:  "",
					Policy:    "",
					Region:    "",
				},
			},
			wantLen:          1,
			wantResourceName: "",
			wantVaultName:    "",
			wantVaultArn:     "",
			wantPolicy:       "",
			wantRegion:       "",
		},
		{
			name:    "type assertion failure",
			output:  "invalid type",
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := process(tt.output, tt.err, false)

			if len(results) != tt.wantLen {
				t.Fatalf("expected %d results, got %d", tt.wantLen, len(results))
			}

			if tt.wantError {
				if results[0].Error == nil {
					t.Error("expected error in result, got nil")
				}
				if results[0].ServiceName != "Backup" {
					t.Errorf("expected ServiceName 'Backup', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "backup:GetBackupVaultAccessPolicy" {
					t.Errorf("expected MethodName 'backup:GetBackupVaultAccessPolicy', got '%s'", results[0].MethodName)
				}
				return
			}

			if tt.wantLen > 0 {
				if results[0].ServiceName != "Backup" {
					t.Errorf("expected ServiceName 'Backup', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "backup:GetBackupVaultAccessPolicy" {
					t.Errorf("expected MethodName 'backup:GetBackupVaultAccessPolicy', got '%s'", results[0].MethodName)
				}
				if results[0].ResourceType != "vault-access-policy" {
					t.Errorf("expected ResourceType 'vault-access-policy', got '%s'", results[0].ResourceType)
				}
				if results[0].ResourceName != tt.wantResourceName {
					t.Errorf("expected ResourceName '%s', got '%s'", tt.wantResourceName, results[0].ResourceName)
				}
				if vaultName, ok := results[0].Details["VaultName"].(string); ok {
					if vaultName != tt.wantVaultName {
						t.Errorf("expected VaultName '%s', got '%s'", tt.wantVaultName, vaultName)
					}
				} else if tt.wantVaultName != "" {
					t.Errorf("expected VaultName in Details, got none")
				}
				if vaultArn, ok := results[0].Details["VaultArn"].(string); ok {
					if vaultArn != tt.wantVaultArn {
						t.Errorf("expected VaultArn '%s', got '%s'", tt.wantVaultArn, vaultArn)
					}
				} else if tt.wantVaultArn != "" {
					t.Errorf("expected VaultArn in Details, got none")
				}
				if policy, ok := results[0].Details["Policy"].(string); ok {
					if policy != tt.wantPolicy {
						t.Errorf("expected Policy '%s', got '%s'", tt.wantPolicy, policy)
					}
				} else if tt.wantPolicy != "" {
					t.Errorf("expected Policy in Details, got none")
				}
				if region, ok := results[0].Details["Region"].(string); ok {
					if region != tt.wantRegion {
						t.Errorf("expected Region '%s', got '%s'", tt.wantRegion, region)
					}
				} else if tt.wantRegion != "" {
					t.Errorf("expected Region in Details, got none")
				}
			}
		})
	}
}
