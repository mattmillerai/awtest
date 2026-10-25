package iam

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// TrustSummary is a factual summary of a role's trust (AssumeRolePolicyDocument) policy.
type TrustSummary struct {
	// Principals lists the principals of Allow statements, e.g.
	// "Service:ec2.amazonaws.com", "AWS:arn:aws:iam::123456789012:root", "*".
	Principals []string
	// Findings are plain statements of fact about the policy (no severity).
	Findings []string
}

type trustDocument struct {
	Statement json.RawMessage `json:"Statement"`
}

type trustStatement struct {
	Effect    string          `json:"Effect"`
	Principal json.RawMessage `json:"Principal"`
	Condition json.RawMessage `json:"Condition"`
}

// accountFromARN returns the account id field of an ARN, or "".
func accountFromARN(arn string) string {
	parts := strings.SplitN(arn, ":", 6)
	if len(parts) < 6 || parts[0] != "arn" {
		return ""
	}
	return parts[4]
}

func isAccountID(s string) bool {
	if len(s) != 12 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// stringOrList decodes a JSON string or array of strings.
func stringOrList(raw json.RawMessage) ([]string, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return []string{s}, nil
	}
	var l []string
	if err := json.Unmarshal(raw, &l); err != nil {
		return nil, fmt.Errorf("expected string or list of strings: %s", string(raw))
	}
	return l, nil
}

func hasCondition(raw json.RawMessage) bool {
	s := strings.TrimSpace(string(raw))
	return s != "" && s != "null" && s != "{}"
}

func appendUnique(list []string, v string) []string {
	for _, e := range list {
		if e == v {
			return list
		}
	}
	return append(list, v)
}

// SummarizeTrustPolicy parses a URL-encoded AssumeRolePolicyDocument as
// returned by ListRoles. roleARN is used to determine the role's own account.
// Only Allow statements contribute principals and findings. An empty
// document yields an empty summary and no error.
func SummarizeTrustPolicy(encoded, roleARN string) (TrustSummary, error) {
	sum := TrustSummary{Principals: []string{}, Findings: []string{}}
	if strings.TrimSpace(encoded) == "" {
		return sum, nil
	}
	decoded, err := url.QueryUnescape(encoded)
	if err != nil {
		return sum, fmt.Errorf("url decode: %w", err)
	}
	var doc trustDocument
	if err := json.Unmarshal([]byte(decoded), &doc); err != nil {
		return sum, fmt.Errorf("json parse: %w", err)
	}
	var stmts []trustStatement
	if len(doc.Statement) > 0 {
		if err := json.Unmarshal(doc.Statement, &stmts); err != nil {
			var one trustStatement
			if err2 := json.Unmarshal(doc.Statement, &one); err2 != nil {
				return sum, fmt.Errorf("statement parse: %w", err2)
			}
			stmts = []trustStatement{one}
		}
	}
	roleAccount := accountFromARN(roleARN)

	for _, st := range stmts {
		if st.Effect != "Allow" || len(st.Principal) == 0 {
			continue
		}
		cond := hasCondition(st.Condition)
		type entry struct{ kind, value string }
		var entries []entry

		var plain string
		if err := json.Unmarshal(st.Principal, &plain); err == nil {
			entries = append(entries, entry{"", plain})
		} else {
			var m map[string]json.RawMessage
			if err := json.Unmarshal(st.Principal, &m); err != nil {
				return sum, fmt.Errorf("principal parse: %w", err)
			}
			for _, kind := range []string{"AWS", "Service", "Federated", "CanonicalUser"} {
				raw, ok := m[kind]
				if !ok {
					continue
				}
				vals, err := stringOrList(raw)
				if err != nil {
					return sum, fmt.Errorf("principal %s: %w", kind, err)
				}
				for _, v := range vals {
					entries = append(entries, entry{kind, v})
				}
			}
		}

		for _, e := range entries {
			if e.kind == "" {
				sum.Principals = appendUnique(sum.Principals, e.value)
			} else {
				sum.Principals = appendUnique(sum.Principals, e.kind+":"+e.value)
			}
			if e.value == "*" && (e.kind == "" || e.kind == "AWS") {
				f := "wildcard principal (*) in Allow statement"
				if cond {
					f += " (statement has a Condition)"
				}
				sum.Findings = appendUnique(sum.Findings, f)
				continue
			}
			if e.kind != "AWS" {
				continue
			}
			acct := ""
			switch {
			case strings.HasPrefix(e.value, "arn:"):
				acct = accountFromARN(e.value)
			case isAccountID(e.value):
				acct = e.value
			default:
				sum.Findings = appendUnique(sum.Findings, "unresolved principal id: "+e.value)
				continue
			}
			if acct != "" && acct != roleAccount {
				sum.Findings = appendUnique(sum.Findings,
					fmt.Sprintf("cross-account principal: %s (role account is %s)", acct, roleAccount))
			}
		}
	}
	return sum, nil
}
