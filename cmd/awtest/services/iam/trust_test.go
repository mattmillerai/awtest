package iam

import (
	"net/url"
	"reflect"
	"strings"
	"testing"
)

const testRoleARN = "arn:aws:iam::111111111111:role/test"

func enc(s string) string { return url.QueryEscape(s) }

func TestSummarizeTrustPolicy(t *testing.T) {
	tests := []struct {
		name       string
		doc        string
		principals []string
		findings   []string
		wantErr    bool
	}{
		{
			name:       "service principal",
			doc:        enc(`{"Statement":[{"Effect":"Allow","Principal":{"Service":"ec2.amazonaws.com"},"Action":"sts:AssumeRole"}]}`),
			principals: []string{"Service:ec2.amazonaws.com"},
		},
		{
			name:       "cross-account ARN",
			doc:        enc(`{"Statement":[{"Effect":"Allow","Principal":{"AWS":"arn:aws:iam::222222222222:role/x"},"Action":"sts:AssumeRole"}]}`),
			principals: []string{"AWS:arn:aws:iam::222222222222:role/x"},
			findings:   []string{"cross-account principal: 222222222222 (role account is 111111111111)"},
		},
		{
			name:       "same-account root",
			doc:        enc(`{"Statement":[{"Effect":"Allow","Principal":{"AWS":"arn:aws:iam::111111111111:root"}}]}`),
			principals: []string{"AWS:arn:aws:iam::111111111111:root"},
		},
		{
			name:       "12-digit account id",
			doc:        enc(`{"Statement":{"Effect":"Allow","Principal":{"AWS":"333333333333"}}}`),
			principals: []string{"AWS:333333333333"},
			findings:   []string{"cross-account principal: 333333333333 (role account is 111111111111)"},
		},
		{
			name:       "wildcard string",
			doc:        enc(`{"Statement":[{"Effect":"Allow","Principal":"*"}]}`),
			principals: []string{"*"},
			findings:   []string{"wildcard principal (*) in Allow statement"},
		},
		{
			name:       "wildcard AWS map",
			doc:        enc(`{"Statement":[{"Effect":"Allow","Principal":{"AWS":"*"}}]}`),
			principals: []string{"AWS:*"},
			findings:   []string{"wildcard principal (*) in Allow statement"},
		},
		{
			name:       "wildcard with condition",
			doc:        enc(`{"Statement":[{"Effect":"Allow","Principal":{"AWS":["*"]},"Condition":{"StringEquals":{"sts:ExternalId":"x"}}}]}`),
			principals: []string{"AWS:*"},
			findings:   []string{"wildcard principal (*) in Allow statement (statement has a Condition)"},
		},
		{
			name: "array statements and list principals",
			doc: enc(`{"Statement":[
				{"Effect":"Allow","Principal":{"Service":["lambda.amazonaws.com","ec2.amazonaws.com"]}},
				{"Effect":"Allow","Principal":{"AWS":["arn:aws:iam::444444444444:root","arn:aws:iam::111111111111:user/u"]}}]}`),
			principals: []string{"Service:lambda.amazonaws.com", "Service:ec2.amazonaws.com", "AWS:arn:aws:iam::444444444444:root", "AWS:arn:aws:iam::111111111111:user/u"},
			findings:   []string{"cross-account principal: 444444444444 (role account is 111111111111)"},
		},
		{
			name:       "deny statement ignored",
			doc:        enc(`{"Statement":[{"Effect":"Deny","Principal":"*"},{"Effect":"Deny","Principal":{"AWS":"arn:aws:iam::999999999999:root"}}]}`),
			principals: []string{},
		},
		{
			name:       "federated",
			doc:        enc(`{"Statement":[{"Effect":"Allow","Principal":{"Federated":"arn:aws:iam::111111111111:oidc-provider/token.actions.githubusercontent.com"},"Action":"sts:AssumeRoleWithWebIdentity"}]}`),
			principals: []string{"Federated:arn:aws:iam::111111111111:oidc-provider/token.actions.githubusercontent.com"},
		},
		{
			name:       "unique id",
			doc:        enc(`{"Statement":[{"Effect":"Allow","Principal":{"AWS":"AIDAEXAMPLEEXAMPLE1"}}]}`),
			principals: []string{"AWS:AIDAEXAMPLEEXAMPLE1"},
			findings:   []string{"unresolved principal id: AIDAEXAMPLEEXAMPLE1"},
		},
		{
			name:       "duplicate findings collapsed",
			doc:        enc(`{"Statement":[{"Effect":"Allow","Principal":"*"},{"Effect":"Allow","Principal":"*"}]}`),
			principals: []string{"*"},
			findings:   []string{"wildcard principal (*) in Allow statement"},
		},
		{name: "malformed url encoding", doc: "%zz", principals: []string{}, wantErr: true},
		{name: "malformed json", doc: enc(`{"Statement": [`), principals: []string{}, wantErr: true},
		{name: "bad principal type", doc: enc(`{"Statement":[{"Effect":"Allow","Principal":{"AWS":5}}]}`), principals: []string{}, wantErr: true},
		{name: "empty", doc: "", principals: []string{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := SummarizeTrustPolicy(tc.doc, testRoleARN)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if !reflect.DeepEqual(got.Principals, tc.principals) {
				t.Errorf("principals = %v, want %v", got.Principals, tc.principals)
			}
			want := tc.findings
			if want == nil {
				want = []string{}
			}
			if !reflect.DeepEqual(got.Findings, want) {
				t.Errorf("findings = %v, want %v", got.Findings, want)
			}
			for _, f := range got.Findings {
				for _, bad := range []string{"risky", "vulnerable", "dangerous"} {
					if strings.Contains(f, bad) {
						t.Errorf("finding %q contains severity word %q", f, bad)
					}
				}
			}
		})
	}
}
