package apigatewayv2

import (
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	apigwtypes "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
)

func TestAPIGatewayV2_Process(t *testing.T) {
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
			name: "apis",
			call: 0,
			output: []apigwtypes.Api{{
				Name:         aws.String("example-http-api"),
				ApiId:        aws.String("a1b2c3d4"),
				ProtocolType: apigwtypes.ProtocolTypeHttp,
			}},
			wantLen:          1,
			wantResourceName: "example-http-api",
		},
		{name: "apis error", call: 0, err: fmt.Errorf("AccessDeniedException: denied"), wantLen: 1, wantError: true},
		{name: "apis empty", call: 0, output: []apigwtypes.Api{}, wantLen: 0},
		{name: "apis wrong type", call: 0, output: "oops", wantLen: 0},
		{
			name:             "domains",
			call:             1,
			output:           []apigwtypes.DomainName{{DomainName: aws.String("api.example.com")}},
			wantLen:          1,
			wantResourceName: "api.example.com",
		},
		{name: "domains error", call: 1, err: fmt.Errorf("AccessDeniedException: denied"), wantLen: 1, wantError: true},
		{name: "domains empty", call: 1, output: []apigwtypes.DomainName{}, wantLen: 0},
		{name: "domains wrong type", call: 1, output: nil, wantLen: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := APIGatewayV2Calls[tt.call]
			results := svc.Process(tt.output, tt.err, false)
			if len(results) != tt.wantLen {
				t.Fatalf("len = %d, want %d", len(results), tt.wantLen)
			}
			for _, r := range results {
				if r.ServiceName != "API Gateway V2" || r.MethodName != svc.Name || r.Timestamp.IsZero() {
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

func TestAPIGatewayV2_Names(t *testing.T) {
	want := []string{"apigatewayv2:GetApis", "apigatewayv2:GetDomainNames"}
	if len(APIGatewayV2Calls) != len(want) {
		t.Fatalf("got %d calls, want %d", len(APIGatewayV2Calls), len(want))
	}
	for i, w := range want {
		if APIGatewayV2Calls[i].Name != w || APIGatewayV2Calls[i].CallV2 == nil {
			t.Errorf("call %d = %q, want %q with CallV2 set", i, APIGatewayV2Calls[i].Name, w)
		}
	}
}
