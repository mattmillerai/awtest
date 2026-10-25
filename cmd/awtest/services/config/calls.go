package config

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/configservice"
	configtypes "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

type configResults struct {
	Recorders        []configtypes.ConfigurationRecorder
	RecorderStatuses []configtypes.ConfigurationRecorderStatus
	Rules            []configtypes.ConfigRule
}

var ConfigCalls = []types.AWSService{
	{
		Name: "config:DescribeConfigurationRecorders",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			var allRecorders []configtypes.ConfigurationRecorder
			var allRecorderStatuses []configtypes.ConfigurationRecorderStatus
			var allRules []configtypes.ConfigRule
			for _, region := range types.RegionsFor(ctx) {
				svc := awsv2.NewRegionalClient(cfg, region, configservice.NewFromConfig)

				recOutput, err := svc.DescribeConfigurationRecorders(ctx, &configservice.DescribeConfigurationRecordersInput{})
				if err != nil {
					return nil, err
				}
				allRecorders = append(allRecorders, recOutput.ConfigurationRecorders...)

				statusOutput, err := svc.DescribeConfigurationRecorderStatus(ctx, &configservice.DescribeConfigurationRecorderStatusInput{})
				if err != nil {
					return nil, err
				}
				allRecorderStatuses = append(allRecorderStatuses, statusOutput.ConfigurationRecordersStatus...)

				rulesOutput, err := svc.DescribeConfigRules(ctx, &configservice.DescribeConfigRulesInput{})
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
					if status.Name != nil {
						recorderStatusMap[*status.Name] = status.Recording
					}
				}

				for _, recorder := range data.Recorders {
					recorderName := aws.ToString(recorder.Name)
					roleArn := aws.ToString(recorder.RoleARN)

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
					ruleName := aws.ToString(rule.ConfigRuleName)
					ruleState := string(rule.ConfigRuleState)

					sourceOwner := ""
					if rule.Source != nil {
						sourceOwner = string(rule.Source.Owner)
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
