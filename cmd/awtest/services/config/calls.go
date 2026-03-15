package config

import (
	"context"
	"fmt"
	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/MillerMedia/awtest/cmd/awtest/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/configservice"
	"time"
)

type configResults struct {
	Recorders        []*configservice.ConfigurationRecorder
	RecorderStatuses []*configservice.ConfigurationRecorderStatus
	Rules            []*configservice.ConfigRule
}

var ConfigCalls = []types.AWSService{
	{
		Name: "config:DescribeConfigurationRecorders",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			var allRecorders []*configservice.ConfigurationRecorder
			var allRecorderStatuses []*configservice.ConfigurationRecorderStatus
			var allRules []*configservice.ConfigRule
			for _, region := range types.Regions {
				regionSess := sess.Copy(&aws.Config{Region: aws.String(region)})
				svc := configservice.New(regionSess)

				recOutput, err := svc.DescribeConfigurationRecordersWithContext(ctx, &configservice.DescribeConfigurationRecordersInput{})
				if err != nil {
					return nil, err
				}
				allRecorders = append(allRecorders, recOutput.ConfigurationRecorders...)

				statusOutput, err := svc.DescribeConfigurationRecorderStatusWithContext(ctx, &configservice.DescribeConfigurationRecorderStatusInput{})
				if err != nil {
					return nil, err
				}
				allRecorderStatuses = append(allRecorderStatuses, statusOutput.ConfigurationRecordersStatus...)

				rulesOutput, err := svc.DescribeConfigRulesWithContext(ctx, &configservice.DescribeConfigRulesInput{})
				if err != nil {
					return nil, err
				}
				allRules = append(allRules, rulesOutput.ConfigRules...)
			}
			return &configResults{Recorders: allRecorders, RecorderStatuses: allRecorderStatuses, Rules: allRules}, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "config:DescribeConfigurationRecorders", err)
				return []types.ScanResult{
					{
						ServiceName: "Config",
						MethodName:  "config:DescribeConfigurationRecorders",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			if data, ok := output.(*configResults); ok {
				// Build a map of recorder name -> recording status
				recorderStatusMap := make(map[string]bool)
				for _, status := range data.RecorderStatuses {
					if status.Name != nil && status.Recording != nil {
						recorderStatusMap[*status.Name] = *status.Recording
					}
				}

				for _, recorder := range data.Recorders {
					recorderName := ""
					if recorder.Name != nil {
						recorderName = *recorder.Name
					}

					roleArn := ""
					if recorder.RoleARN != nil {
						roleArn = *recorder.RoleARN
					}

					recording := false
					if isRecording, found := recorderStatusMap[recorderName]; found {
						recording = isRecording
					}

					recordingStatus := "Stopped"
					if recording {
						recordingStatus = "Recording"
					}

					results = append(results, types.ScanResult{
						ServiceName:  "Config",
						MethodName:   "config:DescribeConfigurationRecorders",
						ResourceType: "configuration-recorder",
						ResourceName: recorderName,
						Details: map[string]interface{}{
							"RoleARN":         roleArn,
							"RecordingStatus": recordingStatus,
						},
						Timestamp: time.Now(),
					})

					utils.PrintResult(debug, "", "config:DescribeConfigurationRecorders",
						fmt.Sprintf("Found Config Recorder: %s (Role: %s, Status: %s)", utils.ColorizeItem(recorderName), roleArn, recordingStatus), nil)
				}

				for _, rule := range data.Rules {
					ruleName := ""
					if rule.ConfigRuleName != nil {
						ruleName = *rule.ConfigRuleName
					}

					ruleState := ""
					if rule.ConfigRuleState != nil {
						ruleState = *rule.ConfigRuleState
					}

					sourceOwner := ""
					if rule.Source != nil && rule.Source.Owner != nil {
						sourceOwner = *rule.Source.Owner
					}

					results = append(results, types.ScanResult{
						ServiceName:  "Config",
						MethodName:   "config:DescribeConfigRules",
						ResourceType: "config-rule",
						ResourceName: ruleName,
						Details: map[string]interface{}{
							"State": ruleState,
							"Owner": sourceOwner,
						},
						Timestamp: time.Now(),
					})

					utils.PrintResult(debug, "", "config:DescribeConfigRules",
						fmt.Sprintf("Found Config Rule: %s (State: %s, Owner: %s)", utils.ColorizeItem(ruleName), ruleState, sourceOwner), nil)
				}
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
