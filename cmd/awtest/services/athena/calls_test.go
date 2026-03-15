package athena

import (
	"fmt"
	"testing"
)

func TestListWorkGroupsProcess(t *testing.T) {
	process := AthenaCalls[0].Process

	tests := []struct {
		name              string
		output            interface{}
		err               error
		wantLen           int
		wantError         bool
		wantResourceName  string
		wantName          string
		wantState         string
		wantDescription   string
		wantCreationTime  string
		wantEngineVersion string
		wantRegion        string
	}{
		{
			name: "valid workgroups with full details",
			output: []atWorkGroup{
				{
					Name:          "primary",
					State:         "ENABLED",
					Description:   "Default workgroup",
					CreationTime:  "2026-01-15T10:00:00Z",
					EngineVersion: "Athena engine version 3",
					Region:        "us-east-1",
				},
				{
					Name:          "analytics-team",
					State:         "ENABLED",
					Description:   "Analytics team workgroup",
					CreationTime:  "2026-02-20T14:00:00Z",
					EngineVersion: "Athena engine version 3",
					Region:        "us-west-2",
				},
			},
			wantLen:           2,
			wantResourceName:  "primary",
			wantName:          "primary",
			wantState:         "ENABLED",
			wantDescription:   "Default workgroup",
			wantCreationTime:  "2026-01-15T10:00:00Z",
			wantEngineVersion: "Athena engine version 3",
			wantRegion:        "us-east-1",
		},
		{
			name:    "empty results",
			output:  []atWorkGroup{},
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
			output: []atWorkGroup{
				{
					Name:          "",
					State:         "",
					Description:   "",
					CreationTime:  "",
					EngineVersion: "",
					Region:        "",
				},
			},
			wantLen:           1,
			wantResourceName:  "",
			wantName:          "",
			wantState:         "",
			wantDescription:   "",
			wantCreationTime:  "",
			wantEngineVersion: "",
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
				if results[0].ServiceName != "Athena" {
					t.Errorf("expected ServiceName 'Athena', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "athena:ListWorkGroups" {
					t.Errorf("expected MethodName 'athena:ListWorkGroups', got '%s'", results[0].MethodName)
				}
				return
			}

			if tt.wantLen > 0 {
				if results[0].ServiceName != "Athena" {
					t.Errorf("expected ServiceName 'Athena', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "athena:ListWorkGroups" {
					t.Errorf("expected MethodName 'athena:ListWorkGroups', got '%s'", results[0].MethodName)
				}
				if results[0].ResourceType != "workgroup" {
					t.Errorf("expected ResourceType 'workgroup', got '%s'", results[0].ResourceType)
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
				if state, ok := results[0].Details["State"].(string); ok {
					if state != tt.wantState {
						t.Errorf("expected State '%s', got '%s'", tt.wantState, state)
					}
				} else if tt.wantState != "" {
					t.Errorf("expected State in Details, got none")
				}
				if desc, ok := results[0].Details["Description"].(string); ok {
					if desc != tt.wantDescription {
						t.Errorf("expected Description '%s', got '%s'", tt.wantDescription, desc)
					}
				} else if tt.wantDescription != "" {
					t.Errorf("expected Description in Details, got none")
				}
				if ct, ok := results[0].Details["CreationTime"].(string); ok {
					if ct != tt.wantCreationTime {
						t.Errorf("expected CreationTime '%s', got '%s'", tt.wantCreationTime, ct)
					}
				} else if tt.wantCreationTime != "" {
					t.Errorf("expected CreationTime in Details, got none")
				}
				if ev, ok := results[0].Details["EngineVersion"].(string); ok {
					if ev != tt.wantEngineVersion {
						t.Errorf("expected EngineVersion '%s', got '%s'", tt.wantEngineVersion, ev)
					}
				} else if tt.wantEngineVersion != "" {
					t.Errorf("expected EngineVersion in Details, got none")
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

func TestListNamedQueriesProcess(t *testing.T) {
	process := AthenaCalls[1].Process

	tests := []struct {
		name             string
		output           interface{}
		err              error
		wantLen          int
		wantError        bool
		wantResourceName string
		wantName         string
		wantNamedQueryId string
		wantDatabase     string
		wantQueryString  string
		wantWorkGroup    string
		wantDescription  string
		wantRegion       string
	}{
		{
			name: "valid named queries with full details",
			output: []atNamedQuery{
				{
					Name:         "daily-report",
					NamedQueryId: "nq-12345-abcde",
					Database:     "analytics_db",
					QueryString:  "SELECT * FROM events WHERE date = current_date",
					WorkGroup:    "primary",
					Description:  "Daily report query",
					Region:       "us-east-1",
				},
				{
					Name:         "user-count",
					NamedQueryId: "nq-67890-fghij",
					Database:     "users_db",
					QueryString:  "SELECT COUNT(*) FROM users",
					WorkGroup:    "analytics-team",
					Description:  "User count query",
					Region:       "us-west-2",
				},
			},
			wantLen:          2,
			wantResourceName: "daily-report",
			wantName:         "daily-report",
			wantNamedQueryId: "nq-12345-abcde",
			wantDatabase:     "analytics_db",
			wantQueryString:  "SELECT * FROM events WHERE date = current_date",
			wantWorkGroup:    "primary",
			wantDescription:  "Daily report query",
			wantRegion:       "us-east-1",
		},
		{
			name:    "empty results",
			output:  []atNamedQuery{},
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
			output: []atNamedQuery{
				{
					Name:         "",
					NamedQueryId: "",
					Database:     "",
					QueryString:  "",
					WorkGroup:    "",
					Description:  "",
					Region:       "",
				},
			},
			wantLen:          1,
			wantResourceName: "",
			wantName:         "",
			wantNamedQueryId: "",
			wantDatabase:     "",
			wantQueryString:  "",
			wantWorkGroup:    "",
			wantDescription:  "",
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
				if results[0].ServiceName != "Athena" {
					t.Errorf("expected ServiceName 'Athena', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "athena:ListNamedQueries" {
					t.Errorf("expected MethodName 'athena:ListNamedQueries', got '%s'", results[0].MethodName)
				}
				return
			}

			if tt.wantLen > 0 {
				if results[0].ServiceName != "Athena" {
					t.Errorf("expected ServiceName 'Athena', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "athena:ListNamedQueries" {
					t.Errorf("expected MethodName 'athena:ListNamedQueries', got '%s'", results[0].MethodName)
				}
				if results[0].ResourceType != "named-query" {
					t.Errorf("expected ResourceType 'named-query', got '%s'", results[0].ResourceType)
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
				if nqId, ok := results[0].Details["NamedQueryId"].(string); ok {
					if nqId != tt.wantNamedQueryId {
						t.Errorf("expected NamedQueryId '%s', got '%s'", tt.wantNamedQueryId, nqId)
					}
				} else if tt.wantNamedQueryId != "" {
					t.Errorf("expected NamedQueryId in Details, got none")
				}
				if db, ok := results[0].Details["Database"].(string); ok {
					if db != tt.wantDatabase {
						t.Errorf("expected Database '%s', got '%s'", tt.wantDatabase, db)
					}
				} else if tt.wantDatabase != "" {
					t.Errorf("expected Database in Details, got none")
				}
				if qs, ok := results[0].Details["QueryString"].(string); ok {
					if qs != tt.wantQueryString {
						t.Errorf("expected QueryString '%s', got '%s'", tt.wantQueryString, qs)
					}
				} else if tt.wantQueryString != "" {
					t.Errorf("expected QueryString in Details, got none")
				}
				if wg, ok := results[0].Details["WorkGroup"].(string); ok {
					if wg != tt.wantWorkGroup {
						t.Errorf("expected WorkGroup '%s', got '%s'", tt.wantWorkGroup, wg)
					}
				} else if tt.wantWorkGroup != "" {
					t.Errorf("expected WorkGroup in Details, got none")
				}
				if desc, ok := results[0].Details["Description"].(string); ok {
					if desc != tt.wantDescription {
						t.Errorf("expected Description '%s', got '%s'", tt.wantDescription, desc)
					}
				} else if tt.wantDescription != "" {
					t.Errorf("expected Description in Details, got none")
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

func TestListQueryExecutionsProcess(t *testing.T) {
	process := AthenaCalls[2].Process

	tests := []struct {
		name                   string
		output                 interface{}
		err                    error
		wantLen                int
		wantError              bool
		wantResourceName       string
		wantQueryExecutionId   string
		wantQuery              string
		wantStatementType      string
		wantStatus             string
		wantStateChangeReason  string
		wantDatabase           string
		wantOutputLocation     string
		wantWorkGroup          string
		wantSubmissionDateTime string
		wantRegion             string
	}{
		{
			name: "valid executions with full details",
			output: []atQueryExecution{
				{
					QueryExecutionId:   "qe-12345-abcde",
					Query:              "SELECT * FROM events LIMIT 100",
					StatementType:      "DML",
					Status:             "SUCCEEDED",
					StateChangeReason:  "",
					Database:           "analytics_db",
					OutputLocation:     "s3://aws-athena-query-results-111111111111-us-east-1/",
					WorkGroup:          "primary",
					SubmissionDateTime: "2026-03-12T10:00:00Z",
					Region:             "us-east-1",
				},
				{
					QueryExecutionId:   "qe-67890-fghij",
					Query:              "CREATE TABLE new_table AS SELECT ...",
					StatementType:      "DDL",
					Status:             "FAILED",
					StateChangeReason:  "TABLE_NOT_FOUND: Table 'new_table' not found",
					Database:           "users_db",
					OutputLocation:     "s3://aws-athena-query-results-111111111111-us-west-2/",
					WorkGroup:          "analytics-team",
					SubmissionDateTime: "2026-03-11T08:30:00Z",
					Region:             "us-west-2",
				},
			},
			wantLen:                2,
			wantResourceName:       "qe-12345-abcde",
			wantQueryExecutionId:   "qe-12345-abcde",
			wantQuery:              "SELECT * FROM events LIMIT 100",
			wantStatementType:      "DML",
			wantStatus:             "SUCCEEDED",
			wantStateChangeReason:  "",
			wantDatabase:           "analytics_db",
			wantOutputLocation:     "s3://aws-athena-query-results-111111111111-us-east-1/",
			wantWorkGroup:          "primary",
			wantSubmissionDateTime: "2026-03-12T10:00:00Z",
			wantRegion:             "us-east-1",
		},
		{
			name: "failed execution with state change reason",
			output: []atQueryExecution{
				{
					QueryExecutionId:   "qe-failed-12345",
					Query:              "SELECT * FROM nonexistent_table",
					StatementType:      "DML",
					Status:             "FAILED",
					StateChangeReason:  "SYNTAX_ERROR: line 1:14: Table 'nonexistent_table' does not exist",
					Database:           "analytics_db",
					OutputLocation:     "s3://aws-athena-query-results-111111111111-us-east-1/",
					WorkGroup:          "primary",
					SubmissionDateTime: "2026-03-12T11:00:00Z",
					Region:             "us-east-1",
				},
			},
			wantLen:                1,
			wantResourceName:       "qe-failed-12345",
			wantQueryExecutionId:   "qe-failed-12345",
			wantQuery:              "SELECT * FROM nonexistent_table",
			wantStatementType:      "DML",
			wantStatus:             "FAILED",
			wantStateChangeReason:  "SYNTAX_ERROR: line 1:14: Table 'nonexistent_table' does not exist",
			wantDatabase:           "analytics_db",
			wantOutputLocation:     "s3://aws-athena-query-results-111111111111-us-east-1/",
			wantWorkGroup:          "primary",
			wantSubmissionDateTime: "2026-03-12T11:00:00Z",
			wantRegion:             "us-east-1",
		},
		{
			name:    "empty results",
			output:  []atQueryExecution{},
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
			output: []atQueryExecution{
				{
					QueryExecutionId:   "",
					Query:              "",
					StatementType:      "",
					Status:             "",
					StateChangeReason:  "",
					Database:           "",
					OutputLocation:     "",
					WorkGroup:          "",
					SubmissionDateTime: "",
					Region:             "",
				},
			},
			wantLen:                1,
			wantResourceName:       "",
			wantQueryExecutionId:   "",
			wantQuery:              "",
			wantStatementType:      "",
			wantStatus:             "",
			wantStateChangeReason:  "",
			wantDatabase:           "",
			wantOutputLocation:     "",
			wantWorkGroup:          "",
			wantSubmissionDateTime: "",
			wantRegion:             "",
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
				if results[0].ServiceName != "Athena" {
					t.Errorf("expected ServiceName 'Athena', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "athena:ListQueryExecutions" {
					t.Errorf("expected MethodName 'athena:ListQueryExecutions', got '%s'", results[0].MethodName)
				}
				return
			}

			if tt.wantLen > 0 {
				if results[0].ServiceName != "Athena" {
					t.Errorf("expected ServiceName 'Athena', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "athena:ListQueryExecutions" {
					t.Errorf("expected MethodName 'athena:ListQueryExecutions', got '%s'", results[0].MethodName)
				}
				if results[0].ResourceType != "query-execution" {
					t.Errorf("expected ResourceType 'query-execution', got '%s'", results[0].ResourceType)
				}
				if results[0].ResourceName != tt.wantResourceName {
					t.Errorf("expected ResourceName '%s', got '%s'", tt.wantResourceName, results[0].ResourceName)
				}
				if qeId, ok := results[0].Details["QueryExecutionId"].(string); ok {
					if qeId != tt.wantQueryExecutionId {
						t.Errorf("expected QueryExecutionId '%s', got '%s'", tt.wantQueryExecutionId, qeId)
					}
				} else if tt.wantQueryExecutionId != "" {
					t.Errorf("expected QueryExecutionId in Details, got none")
				}
				if query, ok := results[0].Details["Query"].(string); ok {
					if query != tt.wantQuery {
						t.Errorf("expected Query '%s', got '%s'", tt.wantQuery, query)
					}
				} else if tt.wantQuery != "" {
					t.Errorf("expected Query in Details, got none")
				}
				if st, ok := results[0].Details["StatementType"].(string); ok {
					if st != tt.wantStatementType {
						t.Errorf("expected StatementType '%s', got '%s'", tt.wantStatementType, st)
					}
				} else if tt.wantStatementType != "" {
					t.Errorf("expected StatementType in Details, got none")
				}
				if status, ok := results[0].Details["Status"].(string); ok {
					if status != tt.wantStatus {
						t.Errorf("expected Status '%s', got '%s'", tt.wantStatus, status)
					}
				} else if tt.wantStatus != "" {
					t.Errorf("expected Status in Details, got none")
				}
				if scr, ok := results[0].Details["StateChangeReason"].(string); ok {
					if scr != tt.wantStateChangeReason {
						t.Errorf("expected StateChangeReason '%s', got '%s'", tt.wantStateChangeReason, scr)
					}
				} else if tt.wantStateChangeReason != "" {
					t.Errorf("expected StateChangeReason in Details, got none")
				}
				if db, ok := results[0].Details["Database"].(string); ok {
					if db != tt.wantDatabase {
						t.Errorf("expected Database '%s', got '%s'", tt.wantDatabase, db)
					}
				} else if tt.wantDatabase != "" {
					t.Errorf("expected Database in Details, got none")
				}
				if ol, ok := results[0].Details["OutputLocation"].(string); ok {
					if ol != tt.wantOutputLocation {
						t.Errorf("expected OutputLocation '%s', got '%s'", tt.wantOutputLocation, ol)
					}
				} else if tt.wantOutputLocation != "" {
					t.Errorf("expected OutputLocation in Details, got none")
				}
				if wg, ok := results[0].Details["WorkGroup"].(string); ok {
					if wg != tt.wantWorkGroup {
						t.Errorf("expected WorkGroup '%s', got '%s'", tt.wantWorkGroup, wg)
					}
				} else if tt.wantWorkGroup != "" {
					t.Errorf("expected WorkGroup in Details, got none")
				}
				if sd, ok := results[0].Details["SubmissionDateTime"].(string); ok {
					if sd != tt.wantSubmissionDateTime {
						t.Errorf("expected SubmissionDateTime '%s', got '%s'", tt.wantSubmissionDateTime, sd)
					}
				} else if tt.wantSubmissionDateTime != "" {
					t.Errorf("expected SubmissionDateTime in Details, got none")
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

func TestTruncateRuneSafe(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxRunes int
		want     string
	}{
		{
			name:     "short string unchanged",
			input:    "SELECT * FROM t",
			maxRunes: 200,
			want:     "SELECT * FROM t",
		},
		{
			name:     "exact length unchanged",
			input:    "abcde",
			maxRunes: 5,
			want:     "abcde",
		},
		{
			name:     "truncated with ellipsis",
			input:    "abcdefghij",
			maxRunes: 5,
			want:     "abcde...",
		},
		{
			name:     "multi-byte characters safe",
			input:    "\u00e9\u00e9\u00e9\u00e9\u00e9\u00e9",
			maxRunes: 3,
			want:     "\u00e9\u00e9\u00e9...",
		},
		{
			name:     "empty string",
			input:    "",
			maxRunes: 200,
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateRuneSafe(tt.input, tt.maxRunes)
			if got != tt.want {
				t.Errorf("truncateRuneSafe(%q, %d) = %q, want %q", tt.input, tt.maxRunes, got, tt.want)
			}
		})
	}
}
