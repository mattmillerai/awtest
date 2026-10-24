package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/MillerMedia/awtest/cmd/awtest/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
)

func TestOrderRegions(t *testing.T) {
	tests := []struct {
		name    string
		in      []string
		primary string
		want    []string
	}{
		{"primary moved first", []string{"a-1", "b-1", "c-1"}, "c-1", []string{"c-1", "a-1", "b-1"}},
		{"primary absent", []string{"a-1", "b-1"}, "z-1", []string{"a-1", "b-1"}},
		{"dedupe", []string{"a-1", "b-1", "a-1", "b-1"}, "b-1", []string{"b-1", "a-1"}},
		{"already first", []string{"a-1", "b-1"}, "a-1", []string{"a-1", "b-1"}},
		{"empty", nil, "a-1", []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := orderRegions(tt.in, tt.primary); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func testSession(t *testing.T, endpoint string) *session.Session {
	t.Helper()
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String("us-west-2"),
		Credentials: credentials.NewStaticCredentials("AKIAFAKEFAKEFAKE", "fake", ""),
		MaxRetries:  aws.Int(0),
	})
	if err != nil {
		t.Fatal(err)
	}
	if endpoint != "" {
		utils.ApplyEndpoint(sess, endpoint)
	}
	return sess
}

const describeRegionsXML = `<?xml version="1.0" encoding="UTF-8"?>
<DescribeRegionsResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <requestId>abc</requestId>
  <regionInfo>%s</regionInfo>
</DescribeRegionsResponse>`

func regionItem(name string) string {
	return fmt.Sprintf("<item><regionName>%s</regionName><regionEndpoint>ec2.%s.amazonaws.com</regionEndpoint><optInStatus>opt-in-not-required</optInStatus></item>", name, name)
}

func TestDiscoverRegions(t *testing.T) {
	fallback := append([]string(nil), types.DefaultEnabledRegions...)
	tests := []struct {
		name       string
		status     int
		body       string
		wantRegion []string
		wantSource string
	}{
		{"success sorted and deduped", 200,
			fmt.Sprintf(describeRegionsXML, regionItem("us-west-2")+regionItem("af-south-1")+regionItem("eu-west-1")+regionItem("us-west-2")),
			[]string{"af-south-1", "eu-west-1", "us-west-2"}, regionSourceDescribe},
		{"403 falls back", 403,
			`<Response><Errors><Error><Code>UnauthorizedOperation</Code><Message>no</Message></Error></Errors></Response>`,
			fallback, regionSourceFallback},
		{"empty list falls back", 200, fmt.Sprintf(describeRegionsXML, ""), fallback, regionSourceFallback},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()
			got, source := discoverRegions(context.Background(), testSession(t, srv.URL))
			if !reflect.DeepEqual(got, tt.wantRegion) {
				t.Errorf("regions = %v, want %v", got, tt.wantRegion)
			}
			if source != tt.wantSource {
				t.Errorf("source = %q, want %q", source, tt.wantSource)
			}
		})
	}

	t.Run("fallback is a copy", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(403) }))
		defer srv.Close()
		got, _ := discoverRegions(context.Background(), testSession(t, srv.URL))
		orig := types.DefaultEnabledRegions[0]
		got[0] = "mutated"
		if types.DefaultEnabledRegions[0] != orig {
			t.Error("fallback aliases types.DefaultEnabledRegions")
		}
	})
}

// regionalFake loops over types.RegionsFor(ctx) like real services do. fail
// maps a region to the action taken when it is reached.
type regionalFake struct {
	mu   sync.Mutex
	seen []string
	fail map[string]func() error
}

func (f *regionalFake) service(name string) types.AWSService {
	return types.AWSService{
		Name: name,
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			var out []string
			for _, region := range types.RegionsFor(ctx) {
				f.mu.Lock()
				f.seen = append(f.seen, region)
				f.mu.Unlock()
				if fn := f.fail[region]; fn != nil {
					if err := fn(); err != nil {
						return nil, err
					}
				}
				out = append(out, region)
			}
			return out, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			if err != nil {
				return []types.ScanResult{{ServiceName: name, MethodName: name, Error: err}}
			}
			var rs []types.ScanResult
			for _, r := range output.([]string) {
				rs = append(rs, types.ScanResult{ServiceName: name, MethodName: name, ResourceName: r})
			}
			return rs
		},
	}
}

func resourceNames(rs []types.ScanResult) []string {
	var names []string
	for _, r := range rs {
		names = append(names, r.ResourceName)
	}
	return names
}

func newCfg(regions ...string) (*allRegionsConfig, *regionFailures) {
	f := &regionFailures{}
	return &allRegionsConfig{regions: regions, failures: f}, f
}

func TestScanServiceNoConfigMatchesScanWithBackoff(t *testing.T) {
	fake := &regionalFake{}
	svc := fake.service("Fake")
	ctx := context.Background()

	want, wantCat := scanWithBackoff(ctx, svc, nil, false)
	got, gotCat := scanService(ctx, svc, nil, false)
	if !reflect.DeepEqual(resourceNames(got), resourceNames(want)) || gotCat != wantCat {
		t.Errorf("got %v/%v, want %v/%v", resourceNames(got), gotCat, resourceNames(want), wantCat)
	}
	if !reflect.DeepEqual(resourceNames(got), types.Regions) {
		t.Errorf("default scan should use the 4 default regions, got %v", resourceNames(got))
	}
}

func TestScanServiceGlobalRunsOnce(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	svc := types.AWSService{
		Name: "Global",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			mu.Lock()
			calls++
			mu.Unlock()
			return "x", nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			return []types.ScanResult{{ServiceName: "Global", ResourceName: "only"}}
		},
	}
	cfg, f := newCfg("eu-west-1", "us-east-1", "ap-south-1")
	got, cat := scanService(withAllRegions(context.Background(), cfg), svc, nil, false)
	if calls != 1 || len(got) != 1 || cat != ErrorNone {
		t.Errorf("calls=%d results=%d cat=%v, want 1/1/none", calls, len(got), cat)
	}
	if n, _ := f.summary(); n != 0 {
		t.Errorf("unexpected failures: %d", n)
	}
}

func TestScanServiceRegionalRunsPerRegion(t *testing.T) {
	fake := &regionalFake{}
	regions := []string{"eu-west-1", "us-east-1", "ap-south-1"}
	cfg, f := newCfg(regions...)
	got, cat := scanService(withAllRegions(context.Background(), cfg), fake.service("Fake"), nil, false)
	if !reflect.DeepEqual(resourceNames(got), regions) {
		t.Errorf("results %v, want %v (primary first)", resourceNames(got), regions)
	}
	if !reflect.DeepEqual(fake.seen, regions) {
		t.Errorf("seen %v, want %v", fake.seen, regions)
	}
	if cat != ErrorNone {
		t.Errorf("cat = %v", cat)
	}
	if n, _ := f.summary(); n != 0 {
		t.Errorf("unexpected failures: %d", n)
	}
}

func TestScanServiceRegionFailureIsolated(t *testing.T) {
	fake := &regionalFake{fail: map[string]func() error{
		"ap-south-1": func() error { return awserr.New("AuthFailure", "not enabled", nil) },
	}}
	cfg, f := newCfg("us-west-2", "ap-south-1", "eu-west-1")
	got, cat := scanService(withAllRegions(context.Background(), cfg), fake.service("Fake"), nil, false)
	if want := []string{"us-west-2", "eu-west-1"}; !reflect.DeepEqual(resourceNames(got), want) {
		t.Errorf("results %v, want %v", resourceNames(got), want)
	}
	for _, r := range got {
		if r.HasError() {
			t.Errorf("error result added for isolated region failure: %+v", r)
		}
	}
	total, regions := f.summary()
	if total != 1 || !reflect.DeepEqual(regions, []string{"ap-south-1"}) {
		t.Errorf("summary = %d %v", total, regions)
	}
	if fl := f.all()[0]; fl.call != "Fake" || fl.detail != "AuthFailure" {
		t.Errorf("failure = %+v", fl)
	}
	if cat != ErrorService {
		t.Errorf("cat = %v, want ErrorService", cat)
	}
}

func TestScanServicePanicInLaterRegionRecovered(t *testing.T) {
	fake := &regionalFake{fail: map[string]func() error{
		"eu-west-1": func() error { panic("secret-credential-value") },
	}}
	cfg, f := newCfg("us-west-2", "eu-west-1", "us-east-1")
	got, _ := scanService(withAllRegions(context.Background(), cfg), fake.service("Fake"), nil, false)
	if want := []string{"us-west-2", "us-east-1"}; !reflect.DeepEqual(resourceNames(got), want) {
		t.Errorf("results %v, want %v", resourceNames(got), want)
	}
	all := f.all()
	if len(all) != 1 || all[0].region != "eu-west-1" || all[0].detail != "panic recovered" {
		t.Errorf("failures = %+v", all)
	}
}

func TestScanServiceContextCancelStopsEarly(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	fake := &regionalFake{fail: map[string]func() error{
		"us-east-1": func() error { cancel(); return nil },
	}}
	cfg, f := newCfg("us-west-2", "us-east-1", "eu-west-1", "ap-south-1")
	got, _ := scanService(withAllRegions(ctx, cfg), fake.service("Fake"), nil, false)
	sort.Strings(fake.seen)
	if want := []string{"us-east-1", "us-west-2"}; !reflect.DeepEqual(fake.seen, want) {
		t.Errorf("seen %v, want %v", fake.seen, want)
	}
	if len(got) != 2 {
		t.Errorf("results %v", resourceNames(got))
	}
	if n, _ := f.summary(); n != 0 {
		t.Errorf("cancellation must not be recorded as failure, got %d", n)
	}
}

func TestScanServiceThrottleRetriedInLaterRegion(t *testing.T) {
	// One retry costs ~0.5-1.5s of backoff; acceptable for covering the path.
	if testing.Short() {
		t.Skip("skipping backoff test in short mode")
	}
	attempts := 0
	fake := &regionalFake{fail: map[string]func() error{
		"eu-west-1": func() error {
			attempts++
			if attempts == 1 {
				return awserr.New("Throttling", "slow down", nil)
			}
			return nil
		},
	}}
	cfg, f := newCfg("us-west-2", "eu-west-1")
	start := time.Now()
	got, cat := scanService(withAllRegions(context.Background(), cfg), fake.service("Fake"), nil, false)
	if attempts != 2 || len(got) != 2 || cat != ErrorNone {
		t.Errorf("attempts=%d results=%d cat=%v", attempts, len(got), cat)
	}
	if n, _ := f.summary(); n != 0 {
		t.Errorf("failures = %d", n)
	}
	if time.Since(start) > 5*time.Second {
		t.Error("retry took unexpectedly long")
	}
}

func TestThrottleClassificationUsedForRegionErrors(t *testing.T) {
	if classifyAWSError(awserr.New("Throttling", "x", nil)) != ErrorThrottle {
		t.Error("Throttling must classify as throttle")
	}
	if classifyAWSError(errors.New("plain")) != ErrorService {
		t.Error("plain errors classify as service")
	}
}

func TestRunWorkerPoolAllRegionsRace(t *testing.T) {
	regions := []string{"us-west-2", "eu-west-1", "ap-south-1", "af-south-1"}
	cfg, f := newCfg(regions...)
	var svcs []types.AWSService
	for i := 0; i < 12; i++ {
		fake := &regionalFake{fail: map[string]func() error{
			"af-south-1": func() error { return awserr.New("AuthFailure", "x", nil) },
		}}
		svcs = append(svcs, fake.service(fmt.Sprintf("svc-%02d", i)))
	}
	svcs = append(svcs, mockService("global", 0))

	results, skipped := runWorkerPool(withAllRegions(context.Background(), cfg), svcs, nil, 6, true, false, nil)
	if len(skipped) != 0 {
		t.Errorf("skipped = %v", skipped)
	}
	// 12 regional services x 3 successful regions + 1 global result.
	if want := 12*3 + 1; len(results) != want {
		t.Errorf("results = %d, want %d", len(results), want)
	}
	if total, failed := f.summary(); total != 12 || !reflect.DeepEqual(failed, []string{"af-south-1"}) {
		t.Errorf("summary = %d %v", total, failed)
	}
}
