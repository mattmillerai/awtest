package transcribe

import (
	"context"
	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/MillerMedia/awtest/cmd/awtest/utils"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/transcribeservice"
	"time"
)

var TranscribeCalls = []types.AWSService{
	{
		Name: "transcribe:ListTranscriptionJobs",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			svc := transcribeservice.New(sess)
			input := &transcribeservice.ListTranscriptionJobsInput{}
			return svc.ListTranscriptionJobsWithContext(ctx, input)
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

			if jobsOutput, ok := output.(*transcribeservice.ListTranscriptionJobsOutput); ok {
				for _, job := range jobsOutput.TranscriptionJobSummaries {
					utils.PrintResult(debug, "", "transcribe:ListTranscriptionJobs", *job.TranscriptionJobName, nil)

					results = append(results, types.ScanResult{
						ServiceName:  "Transcribe",
						MethodName:   "transcribe:ListTranscriptionJobs",
						ResourceType: "transcription-job",
						ResourceName: *job.TranscriptionJobName,
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
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			svc := transcribeservice.New(sess)
			input := &transcribeservice.ListVocabulariesInput{}
			return svc.ListVocabulariesWithContext(ctx, input)
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

			if vocabOutput, ok := output.(*transcribeservice.ListVocabulariesOutput); ok {
				for _, vocab := range vocabOutput.Vocabularies {
					utils.PrintResult(debug, "", "transcribe:ListVocabularies", *vocab.VocabularyName, nil)

					results = append(results, types.ScanResult{
						ServiceName:  "Transcribe",
						MethodName:   "transcribe:ListVocabularies",
						ResourceType: "vocabulary",
						ResourceName: *vocab.VocabularyName,
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
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			svc := transcribeservice.New(sess)
			input := &transcribeservice.ListLanguageModelsInput{}
			return svc.ListLanguageModelsWithContext(ctx, input)
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

			if langModelOutput, ok := output.(*transcribeservice.ListLanguageModelsOutput); ok {
				for _, langModel := range langModelOutput.Models {
					utils.PrintResult(debug, "", "transcribe:ListLanguageModels", *langModel.ModelName, nil)

					results = append(results, types.ScanResult{
						ServiceName:  "Transcribe",
						MethodName:   "transcribe:ListLanguageModels",
						ResourceType: "language-model",
						ResourceName: *langModel.ModelName,
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
		Name: "transcribe:StartTranscriptionJob",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			svc := transcribeservice.New(sess)
			input := &transcribeservice.StartTranscriptionJobInput{}
			return svc.StartTranscriptionJobWithContext(ctx, input)
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "transcribe:StartTranscriptionJob", err)
				return []types.ScanResult{
					{
						ServiceName: "Transcribe",
						MethodName:  "transcribe:StartTranscriptionJob",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			if jobOutput, ok := output.(*transcribeservice.StartTranscriptionJobOutput); ok {
				utils.PrintResult(debug, "", "transcribe:StartTranscriptionJob", *jobOutput.TranscriptionJob.TranscriptionJobName, nil)

				results = append(results, types.ScanResult{
					ServiceName:  "Transcribe",
					MethodName:   "transcribe:StartTranscriptionJob",
					ResourceType: "transcription-job",
					ResourceName: *jobOutput.TranscriptionJob.TranscriptionJobName,
					Details:      map[string]interface{}{},
					Timestamp:    time.Now(),
				})
			}

			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
