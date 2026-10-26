package codecommit

import (
	"fmt"
	"net/http"
	"reflect"
	"testing"
)

func names(n int, prefix string) []string {
	var out []string
	for i := 0; i < n; i++ {
		out = append(out, fmt.Sprintf("%s%03d", prefix, i))
	}
	return out
}

func repoList(ns []string) []map[string]string {
	var out []map[string]string
	for _, n := range ns {
		out = append(out, map[string]string{"repositoryName": n})
	}
	return out
}

func TestListRepositoriesInvoke(t *testing.T) {
	f := &fakeSvc{handle: func(region, op string, body map[string]interface{}) (int, interface{}) {
		switch op {
		case "ListRepositories":
			return http.StatusOK, map[string]interface{}{"repositories": repoList([]string{"full", "bare"})}
		case "BatchGetRepositories":
			return http.StatusOK, map[string]interface{}{"repositories": []map[string]interface{}{
				{
					"repositoryName": "full", "Arn": "arn:aws:codecommit:us-west-2:123456789012:full",
					"cloneUrlHttp":          "https://git-codecommit.us-west-2.amazonaws.com/v1/repos/full",
					"cloneUrlSsh":           "ssh://git-codecommit.us-west-2.amazonaws.com/v1/repos/full",
					"defaultBranch":         "main",
					"repositoryDescription": "the repo",
				},
				{"repositoryName": "bare"},
			}}
		}
		return apiError("UnknownOperationException", op)
	}}
	ctx := newEnv(t, f, "us-west-2")
	results, err := run(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := []map[string]interface{}{
		{
			"Name": "full", "Arn": "arn:aws:codecommit:us-west-2:123456789012:full",
			"CloneUrlHttp":  "https://git-codecommit.us-west-2.amazonaws.com/v1/repos/full",
			"CloneUrlSsh":   "ssh://git-codecommit.us-west-2.amazonaws.com/v1/repos/full",
			"DefaultBranch": "main", "Description": "the repo", "Region": "us-west-2",
		},
		{
			"Name": "bare", "Arn": "", "CloneUrlHttp": "", "CloneUrlSsh": "",
			"DefaultBranch": "", "Description": "", "Region": "us-west-2",
		},
	}
	if got := detailsOf(results); !reflect.DeepEqual(got, want) {
		t.Errorf("details = %#v\nwant %#v", got, want)
	}
	for i, name := range []string{"full", "bare"} {
		r := results[i]
		if r.ServiceName != "CodeCommit" || r.MethodName != "codecommit:ListRepositories" || r.ResourceType != "repository" || r.ResourceName != name {
			t.Errorf("result %d = %+v", i, r)
		}
	}
	batch := f.ops("BatchGetRepositories")
	if len(batch) != 1 || !reflect.DeepEqual(batch[0].body["repositoryNames"], []interface{}{"full", "bare"}) {
		t.Errorf("BatchGetRepositories requests = %+v", batch)
	}
}

// ListRepositories pages are followed and the names are fetched in batches of 25.
func TestListRepositoriesPaginatesAndBatches(t *testing.T) {
	all := names(30, "r")
	f := &fakeSvc{handle: func(region, op string, body map[string]interface{}) (int, interface{}) {
		switch op {
		case "ListRepositories":
			if body["nextToken"] == "t2" {
				return http.StatusOK, map[string]interface{}{"repositories": repoList(all[20:])}
			}
			return http.StatusOK, map[string]interface{}{"repositories": repoList(all[:20]), "nextToken": "t2"}
		case "BatchGetRepositories":
			var ns []string
			for _, n := range body["repositoryNames"].([]interface{}) {
				ns = append(ns, n.(string))
			}
			return http.StatusOK, map[string]interface{}{"repositories": repoList(ns)}
		}
		return apiError("UnknownOperationException", op)
	}}
	ctx := newEnv(t, f, "us-west-2")
	results, err := run(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 30 || results[0].ResourceName != "r000" || results[29].ResourceName != "r029" {
		t.Errorf("got %d results", len(results))
	}
	if got := len(f.ops("ListRepositories")); got != 2 {
		t.Errorf("ListRepositories calls = %d, want 2", got)
	}
	batch := f.ops("BatchGetRepositories")
	if len(batch) != 2 || len(batch[0].body["repositoryNames"].([]interface{})) != 25 || len(batch[1].body["repositoryNames"].([]interface{})) != 5 {
		t.Errorf("BatchGetRepositories requests = %d", len(batch))
	}
}

func TestInvokeErrorText(t *testing.T) {
	f := &fakeSvc{handle: func(region, op string, body map[string]interface{}) (int, interface{}) {
		return apiError("AccessDeniedException", "not authorized for "+op)
	}}
	ctx := newEnv(t, f, "us-west-2")
	results, err := run(ctx, 0)
	if err == nil {
		t.Fatal("expected an error")
	}
	want := "AccessDeniedException: not authorized for ListRepositories"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
	if len(results) != 1 || results[0].Error == nil || results[0].Error.Error() != want {
		t.Errorf("Process error result = %+v", results)
	}
	if results[0].MethodName != "codecommit:ListRepositories" {
		t.Errorf("MethodName = %q", results[0].MethodName)
	}
}

func TestRegionScopeVisitsEachRegion(t *testing.T) {
	f := &fakeSvc{handle: func(region, op string, body map[string]interface{}) (int, interface{}) {
		switch op {
		case "ListRepositories", "BatchGetRepositories":
			return http.StatusOK, map[string]interface{}{"repositories": repoList([]string{"repo-" + region})}
		}
		return apiError("UnknownOperationException", op)
	}}
	scope := []string{"us-east-1", "eu-west-1", "ap-south-1"}
	ctx := newEnv(t, f, scope...)
	results, err := run(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, op := range []string{"ListRepositories", "BatchGetRepositories"} {
		if got := f.regionsOf(op); !reflect.DeepEqual(got, scope) {
			t.Errorf("%s SigV4 regions = %v, want %v", op, got, scope)
		}
	}
	var regions []string
	for _, d := range detailsOf(results) {
		regions = append(regions, d["Region"].(string))
	}
	if !reflect.DeepEqual(regions, scope) {
		t.Errorf("result regions = %v, want %v", regions, scope)
	}
}
