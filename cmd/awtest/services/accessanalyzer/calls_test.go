package accessanalyzer

import (
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	aatypes "github.com/aws/aws-sdk-go-v2/service/accessanalyzer/types"
)

func TestAccessAnalyzer_Process(t *testing.T) {
	tests := []struct {
		name             string
		call             int
		output           interface{}
		err              error
		wantLen          int
		wantError        bool
		wantResourceName string
	}{
		{
			name: "analyzers",
			call: 0,
			output: []aatypes.AnalyzerSummary{{
				Name:   aws.String("example-analyzer"),
				Arn:    aws.String("arn:aws:access-analyzer:us-west-2:123456789012:analyzer/example-analyzer"),
				Type:   aatypes.TypeAccount,
				Status: aatypes.AnalyzerStatusActive,
			}},
			wantLen:          1,
			wantResourceName: "example-analyzer",
		},
		{name: "analyzers error", call: 0, err: fmt.Errorf("AccessDeniedException: denied"), wantLen: 1, wantError: true},
		{name: "analyzers empty", call: 0, output: []aatypes.AnalyzerSummary{}, wantLen: 0},
		{name: "analyzers wrong type", call: 0, output: "oops", wantLen: 0},
		{
			name: "findings",
			call: 1,
			output: []finding{
				{Id: "f-1", Status: "ACTIVE", Resource: "arn:aws:s3:::example-bucket", AnalyzerArn: "arn:a"},
				{Id: "f-2", Status: "ARCHIVED", Resource: "arn:aws:iam::123456789012:role/example-role", AnalyzerArn: "arn:a"},
			},
			wantLen:          2,
			wantResourceName: "",
		},
		{name: "findings error", call: 1, err: fmt.Errorf("AccessDeniedException: denied"), wantLen: 1, wantError: true},
		{name: "findings empty", call: 1, output: []finding{}, wantLen: 0},
		{name: "findings wrong type", call: 1, output: nil, wantLen: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := AccessAnalyzerCalls[tt.call]
			results := svc.Process(tt.output, tt.err, false)
			if len(results) != tt.wantLen {
				t.Fatalf("len = %d, want %d", len(results), tt.wantLen)
			}
			for _, r := range results {
				if r.ServiceName != "Access Analyzer" || r.MethodName != svc.Name || r.Timestamp.IsZero() {
					t.Errorf("bad result %+v", r)
				}
				if tt.wantError != (r.Error != nil) {
					t.Errorf("Error = %v, wantError %v", r.Error, tt.wantError)
				}
				if tt.wantResourceName != "" && r.ResourceName != tt.wantResourceName {
					t.Errorf("ResourceName = %q, want %q", r.ResourceName, tt.wantResourceName)
				}
			}
		})
	}
}

func TestAccessAnalyzer_Names(t *testing.T) {
	want := []string{"accessanalyzer:ListAnalyzers", "accessanalyzer:ListFindingsV2"}
	if len(AccessAnalyzerCalls) != len(want) {
		t.Fatalf("got %d calls, want %d", len(AccessAnalyzerCalls), len(want))
	}
	for i, w := range want {
		if AccessAnalyzerCalls[i].Name != w || AccessAnalyzerCalls[i].CallV2 == nil {
			t.Errorf("call %d = %q, want %q with CallV2 set", i, AccessAnalyzerCalls[i].Name, w)
		}
	}
}
