package main

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
)

const (
	discoverRegionsTimeout = 10 * time.Second
	regionSourceDescribe   = "ec2:DescribeRegions"
	regionSourceFallback   = "built-in list (ec2:DescribeRegions unavailable; opt-in regions not included)"
)

// discoverRegions returns the regions to scan in -all-regions mode.
//
// Decision = hybrid: one read-only ec2:DescribeRegions call (comparable to the
// ec2 calls awtest already makes). With the default AllRegions=false it returns
// only regions enabled for the account, so opt-in regions the account has not
// enabled are skipped instead of erroring, and newly launched regions are picked
// up automatically. The static types.DefaultEnabledRegions list is only the
// fallback when the call fails or returns nothing. cfg is used as-is (client in
// cfg.Region, via awsv2.NewRegionalClient), so -endpoint-url and credentials
// are honored.
func discoverRegions(ctx context.Context, cfg aws.Config) (regions []string, source string) {
	ctx, cancel := context.WithTimeout(ctx, discoverRegionsTimeout)
	defer cancel()

	svc := awsv2.NewRegionalClient(cfg, cfg.Region, ec2.NewFromConfig)
	out, err := svc.DescribeRegions(ctx, &ec2.DescribeRegionsInput{})
	if err == nil && out != nil {
		seen := make(map[string]bool)
		for _, r := range out.Regions {
			name := aws.ToString(r.RegionName)
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			regions = append(regions, name)
		}
		sort.Strings(regions)
	}
	if len(regions) == 0 {
		return append([]string(nil), types.DefaultEnabledRegions...), regionSourceFallback
	}
	return regions, regionSourceDescribe
}

// orderRegions dedupes regions and moves primary (the --region value) first if present.
func orderRegions(regions []string, primary string) []string {
	out := make([]string, 0, len(regions))
	seen := make(map[string]bool, len(regions))
	for _, r := range regions {
		if r == primary && !seen[r] {
			seen[r] = true
			out = append(out, r)
		}
	}
	for _, r := range regions {
		if !seen[r] {
			seen[r] = true
			out = append(out, r)
		}
	}
	return out
}

type regionFailure struct {
	region string
	call   string
	detail string
}

// regionFailures collects per-region call failures; safe for concurrent use.
type regionFailures struct {
	mu       sync.Mutex
	failures []regionFailure
}

func (f *regionFailures) record(region, call, detail string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failures = append(f.failures, regionFailure{region: region, call: call, detail: detail})
}

// summary returns the number of failures and the distinct failing regions in
// order of first occurrence.
func (f *regionFailures) summary() (total int, regions []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	seen := make(map[string]bool)
	for _, fl := range f.failures {
		if !seen[fl.region] {
			seen[fl.region] = true
			regions = append(regions, fl.region)
		}
	}
	return len(f.failures), regions
}

// all returns a copy of the recorded failures.
func (f *regionFailures) all() []regionFailure {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]regionFailure(nil), f.failures...)
}

type allRegionsConfig struct {
	regions  []string
	failures *regionFailures
}

type allRegionsKey struct{}

func withAllRegions(ctx context.Context, cfg *allRegionsConfig) context.Context {
	return context.WithValue(ctx, allRegionsKey{}, cfg)
}

func allRegionsFrom(ctx context.Context) *allRegionsConfig {
	cfg, _ := ctx.Value(allRegionsKey{}).(*allRegionsConfig)
	return cfg
}

// scanService scans one service. Without an allRegionsConfig in ctx it is
// exactly scanWithBackoff. In all-regions mode the first region goes through
// the normal path; if the service never consulted the region scope it is
// global and is done. Otherwise the remaining regions are scanned one by one
// with failures isolated per region.
func scanService(ctx context.Context, service types.AWSService, debug bool) ([]types.ScanResult, ErrorCategory) {
	cfg := allRegionsFrom(ctx)
	if cfg == nil || len(cfg.regions) == 0 {
		return scanWithBackoff(ctx, service, debug)
	}

	firstCtx, scope := types.WithRegionScope(ctx, cfg.regions[:1])
	results, worst := scanWithBackoff(firstCtx, service, debug)
	if !scope.Used() {
		return results, worst
	}

	for _, region := range cfg.regions[1:] {
		if ctx.Err() != nil {
			break
		}
		regionCtx, _ := types.WithRegionScope(ctx, []string{region})
		regionResults, err := scanRegion(regionCtx, service, debug)
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			if cfg.failures != nil {
				cfg.failures.record(region, service.Name, failureDetail(err))
			}
			if c := classifyAWSError(err); c > worst {
				worst = c
			}
			continue
		}
		results = append(results, regionResults...)
	}
	return results, worst
}

var errRegionPanic = errors.New("panic recovered")

// scanRegion runs a single-region CallV2 with panic recovery and the same
// throttle retry policy as scanWithBackoff. Errors are returned, not printed.
func scanRegion(ctx context.Context, service types.AWSService, debug bool) ([]types.ScanResult, error) {
	var err error
	for attempt := 0; attempt <= backoffMaxRetries; attempt++ {
		var results []types.ScanResult
		results, err = callRegionOnce(ctx, service, debug)
		if err == nil || classifyAWSError(err) != ErrorThrottle || attempt == backoffMaxRetries {
			return results, err
		}
		timer := time.NewTimer(calculateBackoff(attempt))
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, err
		case <-timer.C:
		}
	}
	return nil, err
}

func callRegionOnce(ctx context.Context, service types.AWSService, debug bool) (results []types.ScanResult, err error) {
	defer func() {
		if r := recover(); r != nil {
			// Deliberately drop the panic value: it might contain credentials.
			results, err = nil, errRegionPanic
		}
	}()
	output, err := service.Invoke(ctx)
	if err != nil {
		return nil, err
	}
	return service.Process(output, nil, debug), nil
}

func failureDetail(err error) string {
	if code := awsv2.ErrorCode(err); code != "" {
		return code
	}
	return fmt.Sprint(err)
}
