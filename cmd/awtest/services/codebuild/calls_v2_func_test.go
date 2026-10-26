package codebuild

import (
	"fmt"
	"net/http"
	"reflect"
	"testing"
)

func TestServicesAreV2Only(t *testing.T) {
	for i, svc := range CodeBuildCalls {
		if svc.CallV2 == nil {
			t.Errorf("service %d %s: want CallV2 set and Call nil", i, svc.Name)
		}
	}
}

func names(n int, prefix string) []string {
	var out []string
	for i := 0; i < n; i++ {
		out = append(out, fmt.Sprintf("%s%03d", prefix, i))
	}
	return out
}

func TestListProjectsInvoke(t *testing.T) {
	f := &fakeSvc{handle: func(region, op string, body map[string]interface{}) (int, interface{}) {
		switch op {
		case "ListProjects":
			return http.StatusOK, map[string]interface{}{"projects": []string{"full", "bare"}}
		case "BatchGetProjects":
			return http.StatusOK, map[string]interface{}{"projects": []map[string]interface{}{
				{
					"name": "full", "arn": "arn:aws:codebuild:us-west-2:123456789012:project/full",
					"description": "builds things",
					"source":      map[string]string{"type": "GITHUB"},
					"environment": map[string]string{"type": "LINUX_CONTAINER"},
				},
				{"name": "bare"},
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
			"Name": "full", "Arn": "arn:aws:codebuild:us-west-2:123456789012:project/full", "Description": "builds things",
			"SourceType": "GITHUB", "EnvironmentType": "LINUX_CONTAINER", "Region": "us-west-2",
		},
		{
			"Name": "bare", "Arn": "", "Description": "",
			"SourceType": "", "EnvironmentType": "", "Region": "us-west-2",
		},
	}
	if got := detailsOf(results); !reflect.DeepEqual(got, want) {
		t.Errorf("details = %#v\nwant %#v", got, want)
	}
	for i, name := range []string{"full", "bare"} {
		r := results[i]
		if r.ServiceName != "CodeBuild" || r.MethodName != "codebuild:ListProjects" || r.ResourceType != "project" || r.ResourceName != name {
			t.Errorf("result %d = %+v", i, r)
		}
	}
	batch := f.ops("BatchGetProjects")
	if len(batch) != 1 || !reflect.DeepEqual(batch[0].body["names"], []interface{}{"full", "bare"}) {
		t.Errorf("BatchGetProjects requests = %+v", batch)
	}
}

// ListProjects pages are followed and the names are fetched in batches of 100.
func TestListProjectsPaginatesAndBatches(t *testing.T) {
	all := names(150, "p")
	f := &fakeSvc{handle: func(region, op string, body map[string]interface{}) (int, interface{}) {
		switch op {
		case "ListProjects":
			if body["nextToken"] == "t2" {
				return http.StatusOK, map[string]interface{}{"projects": all[100:]}
			}
			return http.StatusOK, map[string]interface{}{"projects": all[:100], "nextToken": "t2"}
		case "BatchGetProjects":
			var out []map[string]string
			for _, n := range body["names"].([]interface{}) {
				out = append(out, map[string]string{"name": n.(string)})
			}
			return http.StatusOK, map[string]interface{}{"projects": out}
		}
		return apiError("UnknownOperationException", op)
	}}
	ctx := newEnv(t, f, "us-west-2")
	results, err := run(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 150 || results[0].ResourceName != "p000" || results[149].ResourceName != "p149" {
		t.Errorf("got %d results", len(results))
	}
	if got := len(f.ops("ListProjects")); got != 2 {
		t.Errorf("ListProjects calls = %d, want 2", got)
	}
	batch := f.ops("BatchGetProjects")
	if len(batch) != 2 || len(batch[0].body["names"].([]interface{})) != 100 || len(batch[1].body["names"].([]interface{})) != 50 {
		t.Errorf("BatchGetProjects requests = %d", len(batch))
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
	want := "AccessDeniedException: not authorized for ListProjects"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
	if len(results) != 1 || results[0].Error == nil || results[0].Error.Error() != want {
		t.Errorf("Process error result = %+v", results)
	}
	if results[0].MethodName != "codebuild:ListProjects" {
		t.Errorf("MethodName = %q", results[0].MethodName)
	}
}

func TestRegionScopeVisitsEachRegion(t *testing.T) {
	f := &fakeSvc{handle: func(region, op string, body map[string]interface{}) (int, interface{}) {
		switch op {
		case "ListProjects":
			return http.StatusOK, map[string]interface{}{"projects": []string{"proj-" + region}}
		case "BatchGetProjects":
			return http.StatusOK, map[string]interface{}{"projects": []map[string]string{{"name": "proj-" + region}}}
		}
		return apiError("UnknownOperationException", op)
	}}
	scope := []string{"us-east-1", "eu-west-1", "ap-south-1"}
	ctx := newEnv(t, f, scope...)
	results, err := run(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, op := range []string{"ListProjects", "BatchGetProjects"} {
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
