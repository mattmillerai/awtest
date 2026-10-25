package transcribe

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/transcribe"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

var TranscribeCalls = []types.AWSService{
	{
		Name: "transcribe:ListTranscriptionJobs",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			svc := awsv2.NewRegionalClient(cfg, "", transcribe.NewFromConfig)
			input := &transcribe.ListTranscriptionJobsInput{}
			return svc.ListTranscriptionJobs(ctx, input)
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "transcribe:ListTranscriptionJobs", err)
				return []types.ScanResult{
					{
						ServiceName: "Transcribe",
						MethodName:  "transcribe:ListTranscriptionJobs",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			if jobsOutput, ok := output.(*transcribe.ListTranscriptionJobsOutput); ok {
				for _, job := range jobsOutput.TranscriptionJobSummaries {
					jobName := aws.ToString(job.TranscriptionJobName)
					utils.PrintResult(debug, "", "transcribe:ListTranscriptionJobs", jobName, nil)

					results = append(results, types.ScanResult{
						ServiceName:  "Transcribe",
						MethodName:   "transcribe:ListTranscriptionJobs",
						ResourceType: "transcription-job",
						ResourceName: jobName,
						Details:      map[string]interface{}{},
						Timestamp:    time.Now(),
					})
				}
			}

			return results
		},
		ModuleName: types.DefaultModuleName,
	},
	{
		Name: "transcribe:ListVocabularies",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			svc := awsv2.NewRegionalClient(cfg, "", transcribe.NewFromConfig)
			input := &transcribe.ListVocabulariesInput{}
			return svc.ListVocabularies(ctx, input)
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "transcribe:ListVocabularies", err)
				return []types.ScanResult{
					{
						ServiceName: "Transcribe",
						MethodName:  "transcribe:ListVocabularies",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			if vocabOutput, ok := output.(*transcribe.ListVocabulariesOutput); ok {
				for _, vocab := range vocabOutput.Vocabularies {
					vocabName := aws.ToString(vocab.VocabularyName)
					utils.PrintResult(debug, "", "transcribe:ListVocabularies", vocabName, nil)

					results = append(results, types.ScanResult{
						ServiceName:  "Transcribe",
						MethodName:   "transcribe:ListVocabularies",
						ResourceType: "vocabulary",
						ResourceName: vocabName,
						Details:      map[string]interface{}{},
						Timestamp:    time.Now(),
					})
				}
			}

			return results
		},
		ModuleName: types.DefaultModuleName,
	},
	{
		Name: "transcribe:ListLanguageModels",
		CallV2: func(ctx context.Context, cfg aws.Config) (interface{}, error) {
			svc := awsv2.NewRegionalClient(cfg, "", transcribe.NewFromConfig)
			input := &transcribe.ListLanguageModelsInput{}
			return svc.ListLanguageModels(ctx, input)
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "transcribe:ListLanguageModels", err)
				return []types.ScanResult{
					{
						ServiceName: "Transcribe",
						MethodName:  "transcribe:ListLanguageModels",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			if langModelOutput, ok := output.(*transcribe.ListLanguageModelsOutput); ok {
				for _, langModel := range langModelOutput.Models {
					modelName := aws.ToString(langModel.ModelName)
					utils.PrintResult(debug, "", "transcribe:ListLanguageModels", modelName, nil)

					results = append(results, types.ScanResult{
						ServiceName:  "Transcribe",
						MethodName:   "transcribe:ListLanguageModels",
						ResourceType: "language-model",
						ResourceName: modelName,
						Details:      map[string]interface{}{},
						Timestamp:    time.Now(),
					})
				}
			}

			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
