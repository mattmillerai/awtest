package athena

import (
	"context"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/MillerMedia/awtest/cmd/awtest/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/athena"
)

type atWorkGroup struct {
	Name          string
	State         string
	Description   string
	CreationTime  string
	EngineVersion string
	Region        string
}

type atNamedQuery struct {
	Name         string
	NamedQueryId string
	Database     string
	QueryString  string
	WorkGroup    string
	Description  string
	Region       string
}

type atQueryExecution struct {
	QueryExecutionId   string
	Query              string
	StatementType      string
	Status             string
	StateChangeReason  string
	Database           string
	OutputLocation     string
	WorkGroup          string
	SubmissionDateTime string
	Region             string
}

// truncateRuneSafe truncates a string to maxRunes runes, appending "..." if truncated.
// This avoids corrupting multi-byte UTF-8 characters at the boundary.
func truncateRuneSafe(s string, maxRunes int) string {
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	runes := []rune(s)
	return string(runes[:maxRunes]) + "..."
}

// batchGetNamedQueries fetches details for a batch of IDs with a single retry for unprocessed IDs.
func batchGetNamedQueries(ctx context.Context, svc *athena.Athena, ids []*string, region string) []atNamedQuery {
	var results []atNamedQuery

	batchOutput, err := svc.BatchGetNamedQueryWithContext(ctx, &athena.BatchGetNamedQueryInput{
		NamedQueryIds: ids,
	})
	if err != nil {
		utils.HandleAWSError(false, "athena:BatchGetNamedQuery", err)
		return results
	}

	for _, nq := range batchOutput.NamedQueries {
		results = append(results, extractNamedQuery(nq, region))
	}

	// Single retry for unprocessed IDs
	if len(batchOutput.UnprocessedNamedQueryIds) > 0 {
		var retryIds []*string
		for _, u := range batchOutput.UnprocessedNamedQueryIds {
			if u.NamedQueryId != nil {
				retryIds = append(retryIds, u.NamedQueryId)
			}
		}
		if len(retryIds) > 0 {
			retryOutput, retryErr := svc.BatchGetNamedQueryWithContext(ctx, &athena.BatchGetNamedQueryInput{
				NamedQueryIds: retryIds,
			})
			if retryErr != nil {
				utils.HandleAWSError(false, "athena:BatchGetNamedQuery", retryErr)
			} else {
				for _, nq := range retryOutput.NamedQueries {
					results = append(results, extractNamedQuery(nq, region))
				}
				if len(retryOutput.UnprocessedNamedQueryIds) > 0 {
					utils.HandleAWSError(false, "athena:BatchGetNamedQuery",
						fmt.Errorf("%d unprocessed query IDs after retry", len(retryOutput.UnprocessedNamedQueryIds)))
				}
			}
		}
	}

	return results
}

func extractNamedQuery(nq *athena.NamedQuery, region string) atNamedQuery {
	name := ""
	if nq.Name != nil {
		name = *nq.Name
	}
	namedQueryId := ""
	if nq.NamedQueryId != nil {
		namedQueryId = *nq.NamedQueryId
	}
	database := ""
	if nq.Database != nil {
		database = *nq.Database
	}
	queryString := ""
	if nq.QueryString != nil {
		queryString = *nq.QueryString
	}
	workGroup := ""
	if nq.WorkGroup != nil {
		workGroup = *nq.WorkGroup
	}
	description := ""
	if nq.Description != nil {
		description = *nq.Description
	}
	return atNamedQuery{
		Name:         name,
		NamedQueryId: namedQueryId,
		Database:     database,
		QueryString:  queryString,
		WorkGroup:    workGroup,
		Description:  description,
		Region:       region,
	}
}

// batchGetQueryExecutions fetches details for a batch of IDs with a single retry for unprocessed IDs.
func batchGetQueryExecutions(ctx context.Context, svc *athena.Athena, ids []*string, region string) []atQueryExecution {
	var results []atQueryExecution

	batchOutput, err := svc.BatchGetQueryExecutionWithContext(ctx, &athena.BatchGetQueryExecutionInput{
		QueryExecutionIds: ids,
	})
	if err != nil {
		utils.HandleAWSError(false, "athena:BatchGetQueryExecution", err)
		return results
	}

	for _, qe := range batchOutput.QueryExecutions {
		results = append(results, extractQueryExecution(qe, region))
	}

	// Single retry for unprocessed IDs
	if len(batchOutput.UnprocessedQueryExecutionIds) > 0 {
		var retryIds []*string
		for _, u := range batchOutput.UnprocessedQueryExecutionIds {
			if u.QueryExecutionId != nil {
				retryIds = append(retryIds, u.QueryExecutionId)
			}
		}
		if len(retryIds) > 0 {
			retryOutput, retryErr := svc.BatchGetQueryExecutionWithContext(ctx, &athena.BatchGetQueryExecutionInput{
				QueryExecutionIds: retryIds,
			})
			if retryErr != nil {
				utils.HandleAWSError(false, "athena:BatchGetQueryExecution", retryErr)
			} else {
				for _, qe := range retryOutput.QueryExecutions {
					results = append(results, extractQueryExecution(qe, region))
				}
				if len(retryOutput.UnprocessedQueryExecutionIds) > 0 {
					utils.HandleAWSError(false, "athena:BatchGetQueryExecution",
						fmt.Errorf("%d unprocessed execution IDs after retry", len(retryOutput.UnprocessedQueryExecutionIds)))
				}
			}
		}
	}

	return results
}

func extractQueryExecution(qe *athena.QueryExecution, region string) atQueryExecution {
	queryExecutionId := ""
	if qe.QueryExecutionId != nil {
		queryExecutionId = *qe.QueryExecutionId
	}
	query := ""
	if qe.Query != nil {
		query = truncateRuneSafe(*qe.Query, 200)
	}
	statementType := ""
	if qe.StatementType != nil {
		statementType = *qe.StatementType
	}
	status := ""
	submissionDateTime := ""
	stateChangeReason := ""
	if qe.Status != nil {
		if qe.Status.State != nil {
			status = *qe.Status.State
		}
		if qe.Status.SubmissionDateTime != nil {
			submissionDateTime = qe.Status.SubmissionDateTime.Format(time.RFC3339)
		}
		if qe.Status.StateChangeReason != nil {
			stateChangeReason = *qe.Status.StateChangeReason
		}
	}
	database := ""
	if qe.QueryExecutionContext != nil && qe.QueryExecutionContext.Database != nil {
		database = *qe.QueryExecutionContext.Database
	}
	outputLocation := ""
	if qe.ResultConfiguration != nil && qe.ResultConfiguration.OutputLocation != nil {
		outputLocation = *qe.ResultConfiguration.OutputLocation
	}
	workGroup := ""
	if qe.WorkGroup != nil {
		workGroup = *qe.WorkGroup
	}
	return atQueryExecution{
		QueryExecutionId:   queryExecutionId,
		Query:              query,
		StatementType:      statementType,
		Status:             status,
		StateChangeReason:  stateChangeReason,
		Database:           database,
		OutputLocation:     outputLocation,
		WorkGroup:          workGroup,
		SubmissionDateTime: submissionDateTime,
		Region:             region,
	}
}

var AthenaCalls = []types.AWSService{
	{
		Name: "athena:ListWorkGroups",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			var allWorkGroups []atWorkGroup
			var lastErr error

			for _, region := range types.Regions {
				svc := athena.New(sess, &aws.Config{Region: aws.String(region)})
				var nextToken *string
				for {
					input := &athena.ListWorkGroupsInput{
						MaxResults: aws.Int64(50),
					}
					if nextToken != nil {
						input.NextToken = nextToken
					}
					output, err := svc.ListWorkGroupsWithContext(ctx, input)
					if err != nil {
						lastErr = err
						utils.HandleAWSError(false, "athena:ListWorkGroups", err)
						break
					}

					for _, wg := range output.WorkGroups {
						name := ""
						if wg.Name != nil {
							name = *wg.Name
						}

						state := ""
						if wg.State != nil {
							state = *wg.State
						}

						description := ""
						if wg.Description != nil {
							description = *wg.Description
						}

						creationTime := ""
						if wg.CreationTime != nil {
							creationTime = wg.CreationTime.Format(time.RFC3339)
						}

						engineVersion := ""
						if wg.EngineVersion != nil && wg.EngineVersion.EffectiveEngineVersion != nil {
							engineVersion = *wg.EngineVersion.EffectiveEngineVersion
						}

						allWorkGroups = append(allWorkGroups, atWorkGroup{
							Name:          name,
							State:         state,
							Description:   description,
							CreationTime:  creationTime,
							EngineVersion: engineVersion,
							Region:        region,
						})
					}

					if output.NextToken == nil {
						break
					}
					nextToken = output.NextToken
				}
			}

			if len(allWorkGroups) == 0 && lastErr != nil {
				return nil, lastErr
			}
			return allWorkGroups, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "athena:ListWorkGroups", err)
				return []types.ScanResult{
					{
						ServiceName: "Athena",
						MethodName:  "athena:ListWorkGroups",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			workGroups, ok := output.([]atWorkGroup)
			if !ok {
				utils.HandleAWSError(debug, "athena:ListWorkGroups", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, wg := range workGroups {
				results = append(results, types.ScanResult{
					ServiceName:  "Athena",
					MethodName:   "athena:ListWorkGroups",
					ResourceType: "workgroup",
					ResourceName: wg.Name,
					Details: map[string]interface{}{
						"Name":          wg.Name,
						"State":         wg.State,
						"Description":   wg.Description,
						"CreationTime":  wg.CreationTime,
						"EngineVersion": wg.EngineVersion,
						"Region":        wg.Region,
					},
					Timestamp: time.Now(),
				})

				utils.PrintResult(debug, "", "athena:ListWorkGroups",
					fmt.Sprintf("Athena Workgroup: %s (State: %s, Engine: %s, Region: %s)", utils.ColorizeItem(wg.Name), wg.State, wg.EngineVersion, wg.Region), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
	{
		Name: "athena:ListNamedQueries",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			var allNamedQueries []atNamedQuery
			var lastErr error

			for _, region := range types.Regions {
				svc := athena.New(sess, &aws.Config{Region: aws.String(region)})

				// Step 1: List all workgroups in this region
				var workGroupNames []string
				var wgNextToken *string
				for {
					wgInput := &athena.ListWorkGroupsInput{
						MaxResults: aws.Int64(50),
					}
					if wgNextToken != nil {
						wgInput.NextToken = wgNextToken
					}
					wgOutput, err := svc.ListWorkGroupsWithContext(ctx, wgInput)
					if err != nil {
						lastErr = err
						utils.HandleAWSError(false, "athena:ListNamedQueries", err)
						break
					}
					for _, wg := range wgOutput.WorkGroups {
						if wg.Name != nil {
							workGroupNames = append(workGroupNames, *wg.Name)
						}
					}
					if wgOutput.NextToken == nil {
						break
					}
					wgNextToken = wgOutput.NextToken
				}

				// Step 2: For each workgroup, list and batch-get named queries per page
				for _, wgName := range workGroupNames {
					var nextToken *string
					for {
						input := &athena.ListNamedQueriesInput{
							MaxResults: aws.Int64(50),
							WorkGroup:  aws.String(wgName),
						}
						if nextToken != nil {
							input.NextToken = nextToken
						}
						output, err := svc.ListNamedQueriesWithContext(ctx, input)
						if err != nil {
							utils.HandleAWSError(false, "athena:ListNamedQueries", err)
							break
						}

						// Batch-get this page's IDs immediately (max 50 per page = max 50 per batch)
						if len(output.NamedQueryIds) > 0 {
							allNamedQueries = append(allNamedQueries,
								batchGetNamedQueries(ctx, svc, output.NamedQueryIds, region)...)
						}

						if output.NextToken == nil {
							break
						}
						nextToken = output.NextToken
					}
				}
			}

			if len(allNamedQueries) == 0 && lastErr != nil {
				return nil, lastErr
			}
			return allNamedQueries, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "athena:ListNamedQueries", err)
				return []types.ScanResult{
					{
						ServiceName: "Athena",
						MethodName:  "athena:ListNamedQueries",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			namedQueries, ok := output.([]atNamedQuery)
			if !ok {
				utils.HandleAWSError(debug, "athena:ListNamedQueries", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, nq := range namedQueries {
				results = append(results, types.ScanResult{
					ServiceName:  "Athena",
					MethodName:   "athena:ListNamedQueries",
					ResourceType: "named-query",
					ResourceName: nq.Name,
					Details: map[string]interface{}{
						"Name":         nq.Name,
						"NamedQueryId": nq.NamedQueryId,
						"Database":     nq.Database,
						"QueryString":  nq.QueryString,
						"WorkGroup":    nq.WorkGroup,
						"Description":  nq.Description,
						"Region":       nq.Region,
					},
					Timestamp: time.Now(),
				})

				utils.PrintResult(debug, "", "athena:ListNamedQueries",
					fmt.Sprintf("Athena Named Query: %s (Database: %s, WorkGroup: %s, Region: %s)\n        Query: %s", utils.ColorizeItem(nq.Name), nq.Database, nq.WorkGroup, nq.Region, truncateRuneSafe(nq.QueryString, 120)), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
	{
		Name: "athena:ListQueryExecutions",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			var allQueryExecutions []atQueryExecution
			var lastErr error

			for _, region := range types.Regions {
				svc := athena.New(sess, &aws.Config{Region: aws.String(region)})

				// Step 1: List all workgroups in this region
				var workGroupNames []string
				var wgNextToken *string
				for {
					wgInput := &athena.ListWorkGroupsInput{
						MaxResults: aws.Int64(50),
					}
					if wgNextToken != nil {
						wgInput.NextToken = wgNextToken
					}
					wgOutput, err := svc.ListWorkGroupsWithContext(ctx, wgInput)
					if err != nil {
						lastErr = err
						utils.HandleAWSError(false, "athena:ListQueryExecutions", err)
						break
					}
					for _, wg := range wgOutput.WorkGroups {
						if wg.Name != nil {
							workGroupNames = append(workGroupNames, *wg.Name)
						}
					}
					if wgOutput.NextToken == nil {
						break
					}
					wgNextToken = wgOutput.NextToken
				}

				// Step 2: For each workgroup, list and batch-get query executions per page
				for _, wgName := range workGroupNames {
					var nextToken *string
					for {
						input := &athena.ListQueryExecutionsInput{
							MaxResults: aws.Int64(50),
							WorkGroup:  aws.String(wgName),
						}
						if nextToken != nil {
							input.NextToken = nextToken
						}
						output, err := svc.ListQueryExecutionsWithContext(ctx, input)
						if err != nil {
							utils.HandleAWSError(false, "athena:ListQueryExecutions", err)
							break
						}

						// Batch-get this page's IDs immediately (max 50 per page = max 50 per batch)
						if len(output.QueryExecutionIds) > 0 {
							allQueryExecutions = append(allQueryExecutions,
								batchGetQueryExecutions(ctx, svc, output.QueryExecutionIds, region)...)
						}

						if output.NextToken == nil {
							break
						}
						nextToken = output.NextToken
					}
				}
			}

			if len(allQueryExecutions) == 0 && lastErr != nil {
				return nil, lastErr
			}
			return allQueryExecutions, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "athena:ListQueryExecutions", err)
				return []types.ScanResult{
					{
						ServiceName: "Athena",
						MethodName:  "athena:ListQueryExecutions",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			queryExecutions, ok := output.([]atQueryExecution)
			if !ok {
				utils.HandleAWSError(debug, "athena:ListQueryExecutions", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, qe := range queryExecutions {
				results = append(results, types.ScanResult{
					ServiceName:  "Athena",
					MethodName:   "athena:ListQueryExecutions",
					ResourceType: "query-execution",
					ResourceName: qe.QueryExecutionId,
					Details: map[string]interface{}{
						"QueryExecutionId":   qe.QueryExecutionId,
						"Query":              qe.Query,
						"StatementType":      qe.StatementType,
						"Status":             qe.Status,
						"StateChangeReason":  qe.StateChangeReason,
						"Database":           qe.Database,
						"OutputLocation":     qe.OutputLocation,
						"WorkGroup":          qe.WorkGroup,
						"SubmissionDateTime": qe.SubmissionDateTime,
						"Region":             qe.Region,
					},
					Timestamp: time.Now(),
				})

				printMsg := fmt.Sprintf("Athena Query Execution: %s (Status: %s, Database: %s, WorkGroup: %s, Region: %s)\n        Query: %s\n        Output: %s",
					utils.ColorizeItem(qe.QueryExecutionId), qe.Status, qe.Database, qe.WorkGroup, qe.Region,
					truncateRuneSafe(qe.Query, 120), qe.OutputLocation)
				if qe.StateChangeReason != "" {
					printMsg += fmt.Sprintf("\n        Reason: %s", qe.StateChangeReason)
				}
				utils.PrintResult(debug, "", "athena:ListQueryExecutions", printMsg, nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
