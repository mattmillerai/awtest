package dynamodb

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
)

var credScopeRegion = regexp.MustCompile(`Credential=[^/]+/\d{8}/([^/]+)/dynamodb/aws4_request`)

type recorded struct {
	region, op string
	body       string
}

// fakeDynamoDB speaks the DynamoDB JSON-1.0 protocol for ListTables,
// ListExports and ListBackups. Data is keyed by the SigV4 credential-scope
// region; an empty name yields an entry without the name field.
type fakeDynamoDB struct {
	names map[string][]string // region -> table names / export ARNs / backup ARNs
	// deny maps "region/op" -> AccessDeniedException message.
	deny map[string]string

	mu   sync.Mutex
	reqs []recorded
}

func (f *fakeDynamoDB) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	region := ""
	if m := credScopeRegion.FindStringSubmatch(r.Header.Get("Authorization")); m != nil {
		region = m[1]
	}
	target := r.Header.Get("X-Amz-Target")
	op := target[strings.LastIndex(target, ".")+1:]
	raw, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	f.reqs = append(f.reqs, recorded{region, op, strings.TrimSpace(string(raw))})
	f.mu.Unlock()

	w.Header().Set("Content-Type", "application/x-amz-json-1.0")
	if msg, ok := f.deny[region+"/"+op]; ok {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"__type": "AccessDeniedException", "message": msg})
		return
	}
	entries := func(field string) []map[string]string {
		var list []map[string]string
		for _, n := range f.names[region] {
			e := map[string]string{}
			if n != "" {
				e[field] = n
			}
			list = append(list, e)
		}
		return list
	}
	switch op {
	case "ListTables":
		// TableNames is a list of plain strings.
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"TableNames": f.names[region]})
	case "ListExports":
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ExportSummaries": entries("ExportArn")})
	case "ListBackups":
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"BackupSummaries": entries("BackupArn")})
	default:
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"__type": "UnknownOperationException", "message": op})
	}
}

func (f *fakeDynamoDB) ops() []recorded {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]recorded(nil), f.reqs...)
}

func newEnv(t *testing.T, f *fakeDynamoDB) context.Context {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)

	t.Setenv("HOME", t.TempDir())
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	for _, k := range []string{"AWS_PROFILE", "AWS_DEFAULT_PROFILE", "AWS_SHARED_CREDENTIALS_FILE", "AWS_CONFIG_FILE", "AWS_REGION", "AWS_DEFAULT_REGION"} {
		t.Setenv(k, "")
	}
	// A client that honoured ambient endpoints could never reach the server.
	const unreachable = "http://ambient-endpoint.invalid:9"
	t.Setenv("AWS_ENDPOINT_URL", unreachable)
	t.Setenv("AWS_ENDPOINT_URL_DYNAMODB", unreachable)

	cfg, err := awsv2.LoadConfig(context.Background(), awsv2.Options{
		Region:          "us-west-2",
		AccessKeyID:     "AKIAFAKEFAKEFAKE",
		SecretAccessKey: "fake",
		Endpoint:        srv.URL,
		HTTPClient:      srv.Client(),
		MaxAttempts:     1,
		Quiet:           true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return types.WithAWSConfig(context.Background(), cfg)
}

func run(ctx context.Context, i int) ([]types.ScanResult, error) {
	svc := DynamoDBCalls[i]
	out, err := svc.Invoke(ctx)
	return svc.Process(out, err, false), err
}

var ops = []struct{ op, rtype string }{
	{"ListTables", "table"},
	{"ListExports", "export"},
	{"ListBackups", "backup"},
}

func TestServicesAreV2Only(t *testing.T) {
	if len(DynamoDBCalls) != len(ops) {
		t.Fatalf("expected %d services, got %d", len(ops), len(DynamoDBCalls))
	}
	for i, svc := range DynamoDBCalls {
		if svc.Name != "dynamodb:"+ops[i].op {
			t.Errorf("service %d name = %q", i, svc.Name)
		}
		if svc.CallV2 == nil {
			t.Errorf("%s: want CallV2 set and Call nil", svc.Name)
		}
	}
}

// Every region in the scope is visited once, in order, with the matching
// SigV4 region, and the results keep that order.
func TestMultiRegionLoop(t *testing.T) {
	for i, c := range ops {
		t.Run(c.op, func(t *testing.T) {
			f := &fakeDynamoDB{names: map[string][]string{
				"us-east-1": {"east-1", ""}, // the empty name has no field: reported as ""
				"us-east-2": {"east-2"},
				"us-west-2": {"west-2"},
			}}
			ctx := newEnv(t, f)
			regions := types.RegionsFor(ctx)
			if len(regions) < 3 {
				t.Fatalf("default regions = %v", regions)
			}
			results, err := run(ctx, i)
			if err != nil {
				t.Fatal(err)
			}
			var gotRegions []string
			for _, r := range f.ops() {
				if r.op != c.op {
					t.Errorf("op = %q, want %q", r.op, c.op)
				}
				if r.body != "{}" {
					t.Errorf("body = %q, want empty input (no pagination, no limit)", r.body)
				}
				gotRegions = append(gotRegions, r.region)
			}
			if !reflect.DeepEqual(gotRegions, regions) {
				t.Errorf("request regions = %v, want %v (in order, once each)", gotRegions, regions)
			}
			var want []string
			for _, r := range regions {
				want = append(want, f.names[r]...)
			}
			if len(results) != len(want) {
				t.Fatalf("got %d results, want %d: %+v", len(results), len(want), results)
			}
			for j, r := range results {
				if r.ServiceName != "DynamoDB" || r.MethodName != "dynamodb:"+c.op || r.ResourceType != c.rtype ||
					r.ResourceName != want[j] || r.Error != nil || r.Timestamp.IsZero() {
					t.Errorf("unexpected result %+v (want name %q)", r, want[j])
				}
				if !reflect.DeepEqual(r.Details, map[string]interface{}{}) {
					t.Errorf("details = %#v, want empty map", r.Details)
				}
			}
		})
	}
}

func TestRegionScope(t *testing.T) {
	for i, c := range ops {
		t.Run(c.op, func(t *testing.T) {
			f := &fakeDynamoDB{names: map[string][]string{"us-east-1": {"east"}, "eu-west-1": {"eu"}}}
			ctx, scope := types.WithRegionScope(newEnv(t, f), []string{"eu-west-1"})
			results, err := run(ctx, i)
			if err != nil {
				t.Fatal(err)
			}
			if !scope.Used() {
				t.Error("service did not consult the region scope")
			}
			if got := f.ops(); len(got) != 1 || got[0].region != "eu-west-1" {
				t.Errorf("requests = %v, want only eu-west-1", got)
			}
			if len(results) != 1 || results[0].ResourceName != "eu" {
				t.Errorf("results = %+v", results)
			}
		})
	}
}

func TestNothingFoundYieldsNoResults(t *testing.T) {
	for i := range DynamoDBCalls {
		results, err := run(newEnv(t, &fakeDynamoDB{}), i)
		if err != nil || len(results) != 0 {
			t.Errorf("%s: results = %+v, err = %v", DynamoDBCalls[i].Name, results, err)
		}
	}
}

// v1 aborted the whole call on the first failing region and discarded the
// results gathered so far.
func TestErrorAbortsLoopAndIsNormalized(t *testing.T) {
	for i, c := range ops {
		t.Run(c.op, func(t *testing.T) {
			f := &fakeDynamoDB{
				names: map[string][]string{"us-east-1": {"east"}},
				deny:  map[string]string{"us-west-1/" + c.op: "not allowed"},
			}
			ctx := newEnv(t, f)
			results, err := run(ctx, i)
			if err == nil {
				t.Fatal("expected error")
			}
			if got, want := err.Error(), "AccessDeniedException: not allowed"; got != want {
				t.Errorf("error = %q, want %q", got, want)
			}
			regions := types.RegionsFor(ctx)
			idx := -1
			for j, r := range regions {
				if r == "us-west-1" {
					idx = j
				}
			}
			if got := f.ops(); len(got) != idx+1 {
				t.Errorf("%d requests, want loop to stop at us-west-1 (index %d)", len(got), idx)
			}
			if len(results) != 1 || results[0].Error == nil || results[0].MethodName != "dynamodb:"+c.op ||
				results[0].ServiceName != "DynamoDB" || results[0].ResourceName != "" {
				t.Errorf("results = %+v", results)
			}
		})
	}
}
