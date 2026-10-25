package services

import (
	"encoding/json"
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/mattmillerai/awtest/cmd/awtest/diff"
	"github.com/mattmillerai/awtest/cmd/awtest/formatters"
	"github.com/mattmillerai/awtest/cmd/awtest/services/iam"
	s3svc "github.com/mattmillerai/awtest/cmd/awtest/services/s3"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
)

// The detail shapes produced by the IAM role/policy and S3 bucket policy
// entries (slices of strings, slices of maps, bools, ints) must survive the
// JSON formatter and be readable by `awtest diff`.
func TestIAMS3DetailsSerializeAndDiffParse(t *testing.T) {
	trust, err := iam.SummarizeTrustPolicy(
		url.QueryEscape(`{"Statement":[{"Effect":"Allow","Principal":{"AWS":"arn:aws:iam::222222222222:root"}}]}`),
		"arn:aws:iam::111111111111:role/r")
	if err != nil {
		t.Fatal(err)
	}
	count, findings, _ := s3svc.PolicyFacts(`{"Statement":[{"Effect":"Allow","Principal":"*"}]}`)
	grants, aclFindings := s3svc.ACLFacts([]s3types.Grant{{
		Grantee:    &s3types.Grantee{Type: s3types.TypeGroup, URI: aws.String("http://acs.amazonaws.com/groups/global/AllUsers")},
		Permission: s3types.PermissionRead,
	}})
	now := time.Now()
	results := []types.ScanResult{
		{ServiceName: "IAM", MethodName: "iam:ListRoles", ResourceType: "role", ResourceName: "r", Timestamp: now,
			Details: map[string]interface{}{"arn": "arn:aws:iam::111111111111:role/r", "trusted_principals": trust.Principals, "trust_findings": trust.Findings, "max_session_duration": int64(3600)}},
		{ServiceName: "IAM", MethodName: "iam:ListGroups", ResourceType: "group", ResourceName: "g", Timestamp: now, Details: map[string]interface{}{}},
		{ServiceName: "IAM", MethodName: "iam:ListPolicies", ResourceType: "policy", ResourceName: "p", Timestamp: now, Details: map[string]interface{}{"attachment_count": int64(2)}},
		{ServiceName: "S3", MethodName: "s3:GetBucketPolicy", ResourceType: "bucket", ResourceName: "b", Timestamp: now,
			Details: map[string]interface{}{"region": "us-east-1", "status": "present", "statement_count": count, "findings": findings}},
		{ServiceName: "S3", MethodName: "s3:GetBucketPolicyStatus", ResourceType: "bucket", ResourceName: "b", Timestamp: now, Details: map[string]interface{}{"status": "public"}},
		{ServiceName: "S3", MethodName: "s3:GetBucketAcl", ResourceType: "bucket", ResourceName: "b", Timestamp: now,
			Details: map[string]interface{}{"status": "ok", "grants": grants, "findings": aclFindings}},
		{ServiceName: "S3", MethodName: "s3:GetPublicAccessBlock", ResourceType: "bucket", ResourceName: "b", Timestamp: now, Details: map[string]interface{}{"status": "none"}},
		{ServiceName: "S3", MethodName: "s3:GetBucketAcl", ResourceType: "bucket", ResourceName: "c", Timestamp: now, Error: errors.New("PermanentRedirect")},
	}

	out, err := formatters.NewJSONFormatter().Format(results)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid([]byte(out)) {
		t.Fatalf("invalid JSON: %s", out)
	}
	var parsed []map[string]interface{}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatal(err)
	}
	tp, _ := parsed[0]["details"].(map[string]interface{})["trust_findings"].([]interface{})
	if len(tp) != 1 {
		t.Errorf("trust_findings not preserved: %v", parsed[0]["details"])
	}

	entries, err := diff.Parse([]byte(out), "fixture")
	if err != nil {
		t.Fatalf("diff.Parse: %v", err)
	}
	if len(entries) != len(results) {
		t.Fatalf("entries = %d, want %d", len(entries), len(results))
	}
	seen := map[string]bool{}
	for _, e := range entries {
		seen[e.MethodName] = true
	}
	for _, m := range []string{"iam:ListRoles", "iam:ListGroups", "iam:ListPolicies", "s3:GetBucketPolicy", "s3:GetBucketPolicyStatus", "s3:GetBucketAcl", "s3:GetPublicAccessBlock"} {
		if !seen[m] {
			t.Errorf("method %s missing from diff entries", m)
		}
	}

	// Envelope form too.
	env, err := formatters.NewJSONFormatter().FormatWithSummary(results, types.ScanSummary{Timestamp: now})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := diff.Parse([]byte(env), "envelope"); err != nil {
		t.Fatalf("diff.Parse envelope: %v", err)
	}
}

func TestIAMS3EntriesRegisteredOnce(t *testing.T) {
	counts := map[string]int{}
	for _, s := range AllServices() {
		counts[s.Name]++
	}
	for _, n := range []string{"iam:ListRoles", "iam:ListGroups", "iam:ListPolicies", "s3:GetBucketPolicy"} {
		if counts[n] != 1 {
			t.Errorf("%s registered %d times, want 1", n, counts[n])
		}
	}
}
