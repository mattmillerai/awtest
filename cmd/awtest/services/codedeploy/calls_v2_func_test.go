package codedeploy

import (
	"net/http"
	"reflect"
	"testing"
)

func TestListApplicationsInvoke(t *testing.T) {
	f := &fakeSvc{handle: func(region, op string, body map[string]interface{}) (int, interface{}) {
		switch op {
		case "ListApplications":
			return http.StatusOK, map[string]interface{}{"applications": []string{"demo-app", "bare-app"}}
		case "BatchGetApplications":
			return http.StatusOK, map[string]interface{}{"applicationsInfo": []map[string]interface{}{
				{
					"applicationName": "demo-app", "applicationId": "11111111-aaaa-bbbb-cccc-222222222222",
					"computePlatform": "Server", "createTime": 1700000000, "linkedToGitHub": true,
				},
				{"applicationName": "bare-app"},
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
			"Name": "demo-app", "ApplicationId": "11111111-aaaa-bbbb-cccc-222222222222", "ComputePlatform": "Server",
			"CreateTime": "2023-11-14T22:13:20Z", "LinkedToGitHub": "true", "Region": "us-west-2",
		},
		// v2 models LinkedToGitHub as a plain bool: absent reads "false" (v1 printed "").
		{
			"Name": "bare-app", "ApplicationId": "", "ComputePlatform": "",
			"CreateTime": "", "LinkedToGitHub": "false", "Region": "us-west-2",
		},
	}
	if got := detailsOf(results); !reflect.DeepEqual(got, want) {
		t.Errorf("details = %#v\nwant %#v", got, want)
	}
	for i, name := range []string{"demo-app", "bare-app"} {
		r := results[i]
		if r.ServiceName != "CodeDeploy" || r.MethodName != "codedeploy:ListApplications" || r.ResourceType != "application" || r.ResourceName != name {
			t.Errorf("result %d = %+v", i, r)
		}
	}
	batch := f.ops("BatchGetApplications")
	if len(batch) != 1 || !reflect.DeepEqual(batch[0].body["applicationNames"], []interface{}{"demo-app", "bare-app"}) {
		t.Errorf("BatchGetApplications requests = %+v", batch)
	}
}

func TestListApplicationsPaginates(t *testing.T) {
	f := &fakeSvc{handle: func(region, op string, body map[string]interface{}) (int, interface{}) {
		switch op {
		case "ListApplications":
			if body["nextToken"] == "t2" {
				return http.StatusOK, map[string]interface{}{"applications": []string{"second"}}
			}
			return http.StatusOK, map[string]interface{}{"applications": []string{"first"}, "nextToken": "t2"}
		case "BatchGetApplications":
			var out []map[string]string
			for _, n := range body["applicationNames"].([]interface{}) {
				out = append(out, map[string]string{"applicationName": n.(string)})
			}
			return http.StatusOK, map[string]interface{}{"applicationsInfo": out}
		}
		return apiError("UnknownOperationException", op)
	}}
	ctx := newEnv(t, f, "us-west-2")
	results, err := run(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 || results[0].ResourceName != "first" || results[1].ResourceName != "second" {
		t.Errorf("results = %+v", results)
	}
	if got := len(f.ops("ListApplications")); got != 2 {
		t.Errorf("ListApplications calls = %d, want 2", got)
	}
	// All names are collected across pages before a single batch call.
	if got := len(f.ops("BatchGetApplications")); got != 1 {
		t.Errorf("BatchGetApplications calls = %d, want 1", got)
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
	want := "AccessDeniedException: not authorized for ListApplications"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
	if len(results) != 1 || results[0].Error == nil || results[0].Error.Error() != want {
		t.Errorf("Process error result = %+v", results)
	}
	if results[0].MethodName != "codedeploy:ListApplications" {
		t.Errorf("MethodName = %q", results[0].MethodName)
	}
}

func TestRegionScopeVisitsEachRegion(t *testing.T) {
	f := &fakeSvc{handle: func(region, op string, body map[string]interface{}) (int, interface{}) {
		switch op {
		case "ListApplications":
			return http.StatusOK, map[string]interface{}{"applications": []string{"app-" + region}}
		case "BatchGetApplications":
			return http.StatusOK, map[string]interface{}{"applicationsInfo": []map[string]string{{"applicationName": "app-" + region}}}
		}
		return apiError("UnknownOperationException", op)
	}}
	scope := []string{"us-east-1", "eu-west-1", "ap-south-1"}
	ctx := newEnv(t, f, scope...)
	results, err := run(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, op := range []string{"ListApplications", "BatchGetApplications"} {
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
