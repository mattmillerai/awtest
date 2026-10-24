package types

import (
	"context"
	"sync/atomic"
)

// DefaultEnabledRegions are the commercial AWS regions that are enabled by
// default for every account. It is the fallback list for -all-regions when
// regions cannot be discovered with ec2:DescribeRegions.
//
// GovCloud and China partitions are intentionally excluded: they need separate
// credentials and endpoints.
var DefaultEnabledRegions = []string{
	"us-east-1",
	"us-east-2",
	"us-west-1",
	"us-west-2",
	"ap-south-1",
	"ap-northeast-1",
	"ap-northeast-2",
	"ap-northeast-3",
	"ap-southeast-1",
	"ap-southeast-2",
	"ca-central-1",
	"eu-central-1",
	"eu-west-1",
	"eu-west-2",
	"eu-west-3",
	"eu-north-1",
	"sa-east-1",
}

// OptInRegions are commercial regions that must be explicitly enabled per
// account before they can be used; calls to a region the account has not
// enabled fail with authentication errors. They are only scanned when
// ec2:DescribeRegions reports them as enabled. GovCloud and China partitions
// are intentionally excluded.
var OptInRegions = []string{
	"af-south-1",
	"ap-east-1",
	"ap-east-2",
	"ap-south-2",
	"ap-southeast-3",
	"ap-southeast-4",
	"ap-southeast-5",
	"ap-southeast-7",
	"ca-west-1",
	"eu-central-2",
	"eu-south-1",
	"eu-south-2",
	"il-central-1",
	"me-central-1",
	"me-south-1",
	"mx-central-1",
}

type regionScopeKey struct{}

// RegionScope carries the regions a scan should cover and records whether any
// service consulted it (i.e. whether the service is regional).
type RegionScope struct {
	regions []string
	used    atomic.Bool
}

// WithRegionScope returns a context carrying the given regions and the scope
// handle, which can be queried afterwards with Used.
func WithRegionScope(ctx context.Context, regions []string) (context.Context, *RegionScope) {
	scope := &RegionScope{regions: append([]string(nil), regions...)}
	return context.WithValue(ctx, regionScopeKey{}, scope), scope
}

// RegionsFor returns the regions a service should loop over. Without a scope
// in ctx it returns a copy of the default Regions.
func RegionsFor(ctx context.Context) []string {
	if scope, ok := ctx.Value(regionScopeKey{}).(*RegionScope); ok && scope != nil {
		scope.used.Store(true)
		return append([]string(nil), scope.regions...)
	}
	return append([]string(nil), Regions...)
}

// Used reports whether RegionsFor was called with this scope.
func (s *RegionScope) Used() bool {
	return s.used.Load()
}
