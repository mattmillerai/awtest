package iam

import (
	"context"
	"fmt"
	"time"

	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
)

// Bounds that keep the IAM scans' API call count predictable.
const (
	// maxRoles caps the roles returned by iam:ListRoles.
	maxRoles = 1000
	// maxGroups caps the groups returned by iam:ListGroups.
	maxGroups = 1000
	// maxPolicies caps the customer managed policies returned by iam:ListPolicies.
	maxPolicies = 1000
	// maxRoleFollowups is the number of roles that get per-role policy follow-up calls.
	maxRoleFollowups = 100
	// maxGroupFollowups is the number of groups that get per-group policy follow-up calls.
	maxGroupFollowups = 100
)

// listResult carries a capped list from Call to Process.
type listResult struct {
	Items     interface{}
	Truncated bool
}

// followupItem is one entry returned by a follow-up call.
type followupItem struct {
	ResourceType string
	Name         string
	Details      map[string]interface{}
}

// followupStep is one per-entity follow-up API call.
type followupStep struct {
	Method string // e.g. "iam:ListAttachedRolePolicies"
	Label  string // used in the printed line, e.g. "attached policy"
	List   func(ctx context.Context, name string) ([]followupItem, error)
}

func isAccessDenied(err error) bool {
	return awsv2.HasErrorCode(err, "AccessDenied", "AccessDeniedException", "UnauthorizedOperation", "Forbidden")
}

// runFollowups runs steps for the first limit names. kind is the parent
// resource type ("role" or "group"); listMethod names the listing call used
// for the truncation note. A step that is denied for one entity is skipped
// for the remaining ones (and noted once) to avoid a flood of denied calls.
func runFollowups(ctx context.Context, debug bool, kind, listMethod string, names []string, limit int, steps []followupStep) []types.ScanResult {
	var results []types.ScanResult
	followed := len(names)
	if followed > limit {
		followed = limit
	}
	denied := make([]bool, len(steps))

	for idx, name := range names[:followed] {
		for si, step := range steps {
			if denied[si] {
				continue
			}
			items, err := step.List(ctx, name)
			if err != nil {
				err = awsv2.Wrap(err)
				utils.HandleAWSError(debug, step.Method, err)
				results = append(results, types.ScanResult{
					ServiceName:  "IAM",
					MethodName:   step.Method,
					ResourceType: kind,
					ResourceName: name,
					Error:        err,
					Timestamp:    time.Now(),
				})
				if isAccessDenied(err) {
					denied[si] = true
					if remaining := followed - idx - 1; remaining > 0 {
						results = append(results, types.ScanResult{
							ServiceName:  "IAM",
							MethodName:   step.Method,
							ResourceType: "note",
							ResourceName: "followups-skipped",
							Details: map[string]interface{}{
								"reason":  "AccessDenied",
								"skipped": remaining,
							},
							Timestamp: time.Now(),
						})
					}
				}
				continue
			}
			for _, it := range items {
				results = append(results, types.ScanResult{
					ServiceName:  "IAM",
					MethodName:   step.Method,
					ResourceType: it.ResourceType,
					// "<role|group>/<policy>" keeps identities unique per owner so
					// `awtest diff` does not merge same-named policies of different roles.
					ResourceName: name + "/" + it.Name,
					Details:      it.Details,
					Timestamp:    time.Now(),
				})
				utils.PrintResult(debug, "", step.Method, fmt.Sprintf("IAM %s: %s | %s: %s", kind, utils.ColorizeItem(name), step.Label, it.Name), nil)
			}
		}
	}

	if len(names) > limit {
		results = append(results, types.ScanResult{
			ServiceName:  "IAM",
			MethodName:   listMethod,
			ResourceType: "note",
			ResourceName: "followups-truncated",
			Details: map[string]interface{}{
				"truncated":            true,
				kind + "s_total":       len(names),
				kind + "s_followed_up": followed,
			},
			Timestamp: time.Now(),
		})
		utils.PrintResult(debug, "", listMethod, fmt.Sprintf("IAM %ss: %d found, policy follow-ups limited to the first %d", kind, len(names), followed), nil)
	}
	return results
}

// listTruncatedNote records that a listing hit its cap.
func listTruncatedNote(debug bool, method, kind string, limit int) types.ScanResult {
	utils.PrintResult(debug, "", method, fmt.Sprintf("IAM %s list truncated at %d", kind, limit), nil)
	return types.ScanResult{
		ServiceName:  "IAM",
		MethodName:   method,
		ResourceType: "note",
		ResourceName: "list-truncated",
		Details:      map[string]interface{}{"truncated": true, "limit": limit},
		Timestamp:    time.Now(),
	}
}

func errResult(method string, err error) []types.ScanResult {
	return []types.ScanResult{{ServiceName: "IAM", MethodName: method, Error: awsv2.Wrap(err), Timestamp: time.Now()}}
}

func str(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
