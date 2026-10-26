package backup

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"sync"
	"testing"

	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
)

var credScopeRegion = regexp.MustCompile(`Credential=[^/]+/\d{8}/([^/]+)/backup/aws4_request`)

type recorded struct {
	region string
	method string
	path   string
	query  map[string][]string
}

// fakeSvc speaks the REST-JSON protocol for backup. handle receives the
// SigV4 credential-scope region and the request and returns the HTTP status
// and JSON response.
type fakeSvc struct {
	handle func(region string, r *http.Request) (int, interface{})

	mu   sync.Mutex
	reqs []recorded
}

func (f *fakeSvc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	region := ""
	if m := credScopeRegion.FindStringSubmatch(r.Header.Get("Authorization")); m != nil {
		region = m[1]
	}
	f.mu.Lock()
	f.reqs = append(f.reqs, recorded{region, r.Method, r.URL.Path, r.URL.Query()})
	f.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	status, out := f.handle(region, r)
	if status >= 400 {
		if m, ok := out.(map[string]string); ok {
			w.Header().Set("X-Amzn-Errortype", m["code"])
			out = map[string]string{"message": m["message"]}
		}
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(out)
}

func apiError(code, msg string) (int, interface{}) {
	return http.StatusForbidden, map[string]string{"code": code, "message": msg}
}

func (f *fakeSvc) requests() []recorded {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]recorded(nil), f.reqs...)
}

func newEnv(t *testing.T, f *fakeSvc, regions ...string) context.Context {
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
	t.Setenv("AWS_ENDPOINT_URL_BACKUP", unreachable)

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
	ctx := types.WithAWSConfig(context.Background(), cfg)
	if len(regions) > 0 {
		ctx, _ = types.WithRegionScope(ctx, regions)
	}
	return ctx
}

func run(ctx context.Context, i int) ([]types.ScanResult, error) {
	svc := BackupCalls[i]
	out, err := svc.Invoke(ctx, nil)
	return svc.Process(out, err, false), err
}

func detailsOf(results []types.ScanResult) []map[string]interface{} {
	var out []map[string]interface{}
	for _, r := range results {
		out = append(out, r.Details)
	}
	return out
}

func TestServicesAreV2Only(t *testing.T) {
	for i, svc := range BackupCalls {
		if svc.CallV2 == nil || svc.Call != nil {
			t.Errorf("service %d %s: want CallV2 set and Call nil", i, svc.Name)
		}
	}
}

func TestListBackupVaultsInvoke(t *testing.T) {
	f := &fakeSvc{handle: func(region string, r *http.Request) (int, interface{}) {
		if r.Method != http.MethodGet || r.URL.Path != "/backup-vaults" {
			return apiError("UnknownOperationException", r.Method+" "+r.URL.Path)
		}
		return http.StatusOK, map[string]interface{}{"BackupVaultList": []map[string]interface{}{
			{
				"BackupVaultName": "full", "BackupVaultArn": "arn:aws:backup:us-west-2:123456789012:backup-vault:full",
				"NumberOfRecoveryPoints": 7, "EncryptionKeyArn": "arn:aws:kms:us-west-2:123456789012:key/abc",
				"CreationDate": 1700000000, "Locked": true,
			},
			{"BackupVaultName": "bare"},
		}}
	}}
	ctx := newEnv(t, f, "us-west-2")
	results, err := run(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := []map[string]interface{}{
		{
			"Name": "full", "Arn": "arn:aws:backup:us-west-2:123456789012:backup-vault:full",
			"RecoveryPointCount": int64(7), "EncryptionKeyArn": "arn:aws:kms:us-west-2:123456789012:key/abc",
			"CreationDate": "2023-11-14T22:13:20Z", "Locked": true, "Region": "us-west-2",
		},
		{
			"Name": "bare", "Arn": "", "RecoveryPointCount": int64(0), "EncryptionKeyArn": "",
			"CreationDate": "", "Locked": false, "Region": "us-west-2",
		},
	}
	if got := detailsOf(results); !reflect.DeepEqual(got, want) {
		t.Errorf("details = %#v\nwant %#v", got, want)
	}
	for i, name := range []string{"full", "bare"} {
		r := results[i]
		if r.ServiceName != "Backup" || r.MethodName != "backup:ListBackupVaults" || r.ResourceType != "backup-vault" || r.ResourceName != name {
			t.Errorf("result %d = %+v", i, r)
		}
	}
	reqs := f.requests()
	if len(reqs) != 1 || reqs[0].region != "us-west-2" || reqs[0].query["maxResults"][0] != "1000" {
		t.Errorf("requests = %+v", reqs)
	}
}

func TestListBackupVaultsPaginates(t *testing.T) {
	f := &fakeSvc{handle: func(region string, r *http.Request) (int, interface{}) {
		if r.URL.Query().Get("nextToken") == "t2" {
			return http.StatusOK, map[string]interface{}{"BackupVaultList": []map[string]string{{"BackupVaultName": "second"}}}
		}
		return http.StatusOK, map[string]interface{}{"BackupVaultList": []map[string]string{{"BackupVaultName": "first"}}, "NextToken": "t2"}
	}}
	ctx := newEnv(t, f, "us-west-2")
	results, err := run(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 || results[0].ResourceName != "first" || results[1].ResourceName != "second" {
		t.Errorf("results = %+v", results)
	}
	if got := len(f.requests()); got != 2 {
		t.Errorf("requests = %d, want 2", got)
	}
}

func TestInvokeErrorText(t *testing.T) {
	f := &fakeSvc{handle: func(region string, r *http.Request) (int, interface{}) {
		return apiError("AccessDeniedException", "not authorized")
	}}
	ctx := newEnv(t, f, "us-west-2")
	results, err := run(ctx, 0)
	if err == nil {
		t.Fatal("expected an error")
	}
	want := "AccessDeniedException: not authorized"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
	if len(results) != 1 || results[0].Error == nil || results[0].Error.Error() != want {
		t.Errorf("Process error result = %+v", results)
	}
	if results[0].MethodName != "backup:ListBackupVaults" {
		t.Errorf("MethodName = %q", results[0].MethodName)
	}
}

func TestRegionScopeVisitsEachRegion(t *testing.T) {
	f := &fakeSvc{handle: func(region string, r *http.Request) (int, interface{}) {
		return http.StatusOK, map[string]interface{}{"BackupVaultList": []map[string]string{{"BackupVaultName": "vault-" + region}}}
	}}
	scope := []string{"us-east-1", "eu-west-1", "ap-south-1"}
	ctx := newEnv(t, f, scope...)
	results, err := run(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	var sigv4 []string
	for _, r := range f.requests() {
		sigv4 = append(sigv4, r.region)
	}
	if !reflect.DeepEqual(sigv4, scope) {
		t.Errorf("SigV4 regions = %v, want %v", sigv4, scope)
	}
	var regions []string
	for _, d := range detailsOf(results) {
		regions = append(regions, d["Region"].(string))
	}
	if !reflect.DeepEqual(regions, scope) {
		t.Errorf("result regions = %v, want %v", regions, scope)
	}
}
