package main

import (
	"testing"

	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
)

func TestV2OptionsMirrorsV1CredentialPaths(t *testing.T) {
	cases := []struct {
		name                          string
		keyID, secret, token, profile string
		want                          awsv2.Options
	}{
		{"static keys beat profile", "AKIAX", "s", "tok", "p",
			awsv2.Options{AccessKeyID: "AKIAX", SecretAccessKey: "s", SessionToken: "tok"}},
		{"profile when keys incomplete", "AKIAX", "", "", "p", awsv2.Options{Profile: "p"}},
		{"profile when no keys", "", "", "", "p", awsv2.Options{Profile: "p"}},
		{"default chain", "", "", "", "", awsv2.Options{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := v2Options("eu-west-1", "http://localhost:4566", tc.keyID, tc.secret, tc.token, tc.profile, true, true)
			want := tc.want
			want.Region, want.Endpoint, want.Quiet, want.Debug = "eu-west-1", "http://localhost:4566", true, true
			if got != want {
				t.Errorf("got %+v, want %+v", got, want)
			}
		})
	}
}
