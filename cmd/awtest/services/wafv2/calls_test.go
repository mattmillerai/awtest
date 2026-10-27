package wafv2

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	wafv2types "github.com/aws/aws-sdk-go-v2/service/wafv2/types"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
)

func TestWAFv2_Process(t *testing.T) {
	items := []wafItem{
		{Name: "example-acl", Id: "11111111-2222-3333-4444-555555555555", Arn: "arn:aws:wafv2:us-west-2:123456789012:regional/webacl/example-acl/1", Scope: "REGIONAL"},
		{Name: "example-cf-acl", Id: "66666666-7777-8888-9999-000000000000", Arn: "arn:aws:wafv2:us-east-1:123456789012:global/webacl/example-cf-acl/2", Scope: "CLOUDFRONT"},
	}
	tests := []struct {
		name      string
		output    interface{}
		err       error
		wantLen   int
		wantError bool
	}{
		{name: "populated", output: items, wantLen: 2},
		{name: "error", err: fmt.Errorf("AccessDeniedException: denied"), wantLen: 1, wantError: true},
		{name: "empty", output: []wafItem{}, wantLen: 0},
		{name: "wrong type", output: "oops", wantLen: 0},
	}
	for _, svc := range WAFv2Calls {
		for _, tt := range tests {
			t.Run(svc.Name+"/"+tt.name, func(t *testing.T) {
				results := svc.Process(tt.output, tt.err, false)
				if len(results) != tt.wantLen {
					t.Fatalf("len = %d, want %d", len(results), tt.wantLen)
				}
				for i, r := range results {
					if r.ServiceName != "WAFv2" || r.MethodName != svc.Name || r.Timestamp.IsZero() {
						t.Errorf("bad result %+v", r)
					}
					if tt.wantError != (r.Error != nil) {
						t.Errorf("Error = %v, wantError %v", r.Error, tt.wantError)
					}
					if tt.output != nil && !tt.wantError && tt.wantLen > 0 {
						if r.ResourceName != items[i].Name || r.Details["Scope"] != items[i].Scope {
							t.Errorf("result %d = %+v", i, r)
						}
					}
				}
			})
		}
	}
}

func TestScopes(t *testing.T) {
	ctx, _ := types.WithRegionScope(context.Background(), []string{"us-west-2", "eu-west-1"})
	want := []scopedClient{
		{wafv2types.ScopeRegional, "us-west-2"},
		{wafv2types.ScopeRegional, "eu-west-1"},
		{wafv2types.ScopeCloudfront, "us-east-1"},
	}
	if got := scopes(ctx); !reflect.DeepEqual(got, want) {
		t.Errorf("scopes = %v, want %v", got, want)
	}
}

func TestWAFv2_Names(t *testing.T) {
	want := []string{"wafv2:ListWebACLs", "wafv2:ListIPSets"}
	if len(WAFv2Calls) != len(want) {
		t.Fatalf("got %d calls, want %d", len(WAFv2Calls), len(want))
	}
	for i, w := range want {
		if WAFv2Calls[i].Name != w || WAFv2Calls[i].CallV2 == nil {
			t.Errorf("call %d = %q (CallV2 nil: %v), want %q", i, WAFv2Calls[i].Name, WAFv2Calls[i].CallV2 == nil, w)
		}
	}
}
