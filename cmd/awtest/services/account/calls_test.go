package account

import (
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	accounttypes "github.com/aws/aws-sdk-go-v2/service/account/types"
)

func TestAccount_Process(t *testing.T) {
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
			name: "regions",
			call: 0,
			output: []accounttypes.Region{
				{RegionName: aws.String("us-east-1"), RegionOptStatus: accounttypes.RegionOptStatusEnabledByDefault},
				{RegionName: aws.String("af-south-1"), RegionOptStatus: accounttypes.RegionOptStatusDisabled},
			},
			wantLen:          2,
			wantResourceName: "",
		},
		{name: "regions error", call: 0, err: fmt.Errorf("AccessDeniedException: denied"), wantLen: 1, wantError: true},
		{name: "regions empty", call: 0, output: []accounttypes.Region{}, wantLen: 0},
		{name: "regions wrong type", call: 0, output: "oops", wantLen: 0},
		{
			name:             "contact",
			call:             1,
			output:           accounttypes.ContactInformation{FullName: aws.String("Example User"), CompanyName: aws.String("Example Co"), CountryCode: aws.String("US")},
			wantLen:          1,
			wantResourceName: "Example User",
		},
		{name: "contact sparse", call: 1, output: accounttypes.ContactInformation{}, wantLen: 1},
		{name: "contact access denied", call: 1, err: fmt.Errorf("AccessDeniedException: not authorized to perform account:GetContactInformation"), wantLen: 1, wantError: true},
		{name: "contact wrong type", call: 1, output: 3, wantLen: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := AccountCalls[tt.call]
			results := svc.Process(tt.output, tt.err, false)
			if len(results) != tt.wantLen {
				t.Fatalf("len = %d, want %d", len(results), tt.wantLen)
			}
			for _, r := range results {
				if r.ServiceName != "Account" || r.MethodName != svc.Name || r.Timestamp.IsZero() {
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

func TestAccount_RegionsDetails(t *testing.T) {
	results := AccountCalls[0].Process([]accounttypes.Region{
		{RegionName: aws.String("af-south-1"), RegionOptStatus: accounttypes.RegionOptStatusDisabled},
	}, nil, false)
	if len(results) != 1 || results[0].ResourceName != "af-south-1" || results[0].Details["RegionOptStatus"] != "DISABLED" {
		t.Errorf("results = %+v", results)
	}
}

func TestAccount_Names(t *testing.T) {
	want := []string{"account:ListRegions", "account:GetContactInformation"}
	if len(AccountCalls) != len(want) {
		t.Fatalf("got %d calls, want %d", len(AccountCalls), len(want))
	}
	for i, w := range want {
		if AccountCalls[i].Name != w || AccountCalls[i].CallV2 == nil {
			t.Errorf("call %d = %q, want %q with CallV2 set", i, AccountCalls[i].Name, w)
		}
	}
}
