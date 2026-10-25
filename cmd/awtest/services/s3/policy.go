package s3

import (
	"encoding/json"
	"fmt"

	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
)

// maxBuckets caps the buckets examined by s3:GetBucketPolicy.
const maxBuckets = 500

const (
	allUsersURI      = "http://acs.amazonaws.com/groups/global/AllUsers"
	authUsersURI     = "http://acs.amazonaws.com/groups/global/AuthenticatedUsers"
	statusDenied     = "denied"
	codeNoPolicy     = "NoSuchBucketPolicy"
	codeNoPublicBlck = "NoSuchPublicAccessBlockConfiguration"
)

// NormalizeBucketRegion maps a GetBucketLocation LocationConstraint to a region.
func NormalizeBucketRegion(loc *string) string {
	if loc == nil || *loc == "" {
		return "us-east-1"
	}
	if *loc == "EU" {
		return "eu-west-1"
	}
	return *loc
}

func errCode(err error) string {
	return awsv2.ErrorCode(err)
}

// isDeniedCode reports whether err is an access-denied style S3 error.
func isDeniedCode(err error) bool {
	switch errCode(err) {
	case "AccessDenied", "AccessDeniedException", "Forbidden", "AllAccessDisabled":
		return true
	}
	return false
}

// statusFromError maps err to a non-error status: "denied" for access-denied
// codes, noneStatus when the error code equals noneCode. ok is false when the
// error should be reported as an error.
func statusFromError(err error, noneCode, noneStatus string) (status string, ok bool) {
	if isDeniedCode(err) {
		return statusDenied, true
	}
	if noneCode != "" && errCode(err) == noneCode {
		return noneStatus, true
	}
	return "", false
}

type policyDoc struct {
	Statement json.RawMessage `json:"Statement"`
}

type policyStatement struct {
	Effect    string          `json:"Effect"`
	Principal json.RawMessage `json:"Principal"`
	Condition json.RawMessage `json:"Condition"`
}

func principalIsWildcard(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s == "*"
	}
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil {
		return false
	}
	aws, ok := m["AWS"]
	if !ok {
		return false
	}
	if json.Unmarshal(aws, &s) == nil {
		return s == "*"
	}
	var l []string
	if json.Unmarshal(aws, &l) == nil {
		for _, v := range l {
			if v == "*" {
				return true
			}
		}
	}
	return false
}

// PolicyFacts returns the statement count and factual findings of a bucket
// policy document.
func PolicyFacts(doc string) (count int, findings []string, err error) {
	findings = []string{}
	var d policyDoc
	if err := json.Unmarshal([]byte(doc), &d); err != nil {
		return 0, findings, err
	}
	var stmts []policyStatement
	if len(d.Statement) > 0 {
		if err := json.Unmarshal(d.Statement, &stmts); err != nil {
			var one policyStatement
			if err2 := json.Unmarshal(d.Statement, &one); err2 != nil {
				return 0, findings, err2
			}
			stmts = []policyStatement{one}
		}
	}
	seen := map[string]bool{}
	for _, st := range stmts {
		if st.Effect != "Allow" || !principalIsWildcard(st.Principal) {
			continue
		}
		f := "Allow statement with wildcard principal (*)"
		if c := string(st.Condition); c != "" && c != "null" && c != "{}" {
			f += " (statement has a Condition)"
		}
		if !seen[f] {
			seen[f] = true
			findings = append(findings, f)
		}
	}
	return len(stmts), findings, nil
}

// ACLFacts converts ACL grants into detail maps and factual findings.
func ACLFacts(grants []s3types.Grant) ([]map[string]interface{}, []string) {
	out := []map[string]interface{}{}
	findings := []string{}
	for _, g := range grants {
		if g.Grantee == nil {
			continue
		}
		gr := g.Grantee
		grantee := ""
		switch {
		case gr.URI != nil:
			grantee = *gr.URI
		case gr.ID != nil:
			grantee = *gr.ID
		case gr.EmailAddress != nil:
			grantee = *gr.EmailAddress
		}
		perm := string(g.Permission)
		gtype := string(gr.Type)
		out = append(out, map[string]interface{}{"grantee_type": gtype, "grantee": grantee, "permission": perm})
		switch grantee {
		case allUsersURI:
			findings = append(findings, fmt.Sprintf("grant to AllUsers (%s): %s", allUsersURI, perm))
		case authUsersURI:
			findings = append(findings, fmt.Sprintf("grant to AuthenticatedUsers (%s): %s", authUsersURI, perm))
		}
	}
	return out, findings
}
