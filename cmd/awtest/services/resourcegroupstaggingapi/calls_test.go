package resourcegroupstaggingapi

import (
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	tagtypes "github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi/types"
)

func TestTagging_Process(t *testing.T) {
	tests := []struct {
		name             string
		call             int
		output           interface{}
		err              error
		wantLen          int
		wantError        bool
		wantResourceName string
		wantDetail       string
		wantDetailValue  interface{}
	}{
		{
			name: "resources",
			call: 0,
			output: []tagtypes.ResourceTagMapping{{
				ResourceARN: aws.String("arn:aws:s3:::example-bucket"),
				Tags:        []tagtypes.Tag{{Key: aws.String("env"), Value: aws.String("demo")}, {Key: aws.String("team"), Value: aws.String("x")}},
			}},
			wantLen:          1,
			wantResourceName: "arn:aws:s3:::example-bucket",
			wantDetail:       "TagCount",
			wantDetailValue:  2,
		},
		{name: "resources error", call: 0, err: fmt.Errorf("AccessDeniedException: denied"), wantLen: 1, wantError: true},
		{name: "resources empty", call: 0, output: []tagtypes.ResourceTagMapping{}, wantLen: 0},
		{name: "resources wrong type", call: 0, output: "oops", wantLen: 0},
		{
			name:             "tag keys",
			call:             1,
			output:           []tagKey{{Key: "env", Region: "us-west-2"}},
			wantLen:          1,
			wantResourceName: "env",
			wantDetail:       "Region",
			wantDetailValue:  "us-west-2",
		},
		{name: "tag keys error", call: 1, err: fmt.Errorf("AccessDeniedException: denied"), wantLen: 1, wantError: true},
		{name: "tag keys empty", call: 1, output: []tagKey{}, wantLen: 0},
		{name: "tag keys wrong type", call: 1, output: 7, wantLen: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := ResourceGroupsTaggingAPICalls[tt.call]
			results := svc.Process(tt.output, tt.err, false)
			if len(results) != tt.wantLen {
				t.Fatalf("len = %d, want %d", len(results), tt.wantLen)
			}
			for _, r := range results {
				if r.ServiceName != "Resource Groups Tagging API" || r.MethodName != svc.Name || r.Timestamp.IsZero() {
					t.Errorf("bad result %+v", r)
				}
				if tt.wantError != (r.Error != nil) {
					t.Errorf("Error = %v, wantError %v", r.Error, tt.wantError)
				}
				if tt.wantResourceName != "" && r.ResourceName != tt.wantResourceName {
					t.Errorf("ResourceName = %q, want %q", r.ResourceName, tt.wantResourceName)
				}
				if tt.wantDetail != "" && r.Details[tt.wantDetail] != tt.wantDetailValue {
					t.Errorf("Details[%s] = %v, want %v", tt.wantDetail, r.Details[tt.wantDetail], tt.wantDetailValue)
				}
			}
		})
	}
}

func TestTagging_Names(t *testing.T) {
	want := []string{"tag:GetResources", "tag:GetTagKeys"}
	if len(ResourceGroupsTaggingAPICalls) != len(want) {
		t.Fatalf("got %d calls, want %d", len(ResourceGroupsTaggingAPICalls), len(want))
	}
	for i, w := range want {
		if ResourceGroupsTaggingAPICalls[i].Name != w || ResourceGroupsTaggingAPICalls[i].CallV2 == nil {
			t.Errorf("call %d = %q, want %q with CallV2 set", i, ResourceGroupsTaggingAPICalls[i].Name, w)
		}
	}
}
