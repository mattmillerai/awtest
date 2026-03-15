package elasticache

import (
	"fmt"
	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/elasticache"
	"testing"
)

func TestProcess(t *testing.T) {
	process := ElastiCacheCalls[0].Process

	tests := []struct {
		name          string
		input         interface{}
		err           error
		expectedCount int
		expectError   bool
		checkResults  func(t *testing.T, results []types.ScanResult)
	}{
		{
			name: "valid Redis cluster with all fields",
			input: []*elasticache.CacheCluster{
				{
					CacheClusterId:          aws.String("my-redis-cluster"),
					Engine:                  aws.String("redis"),
					EngineVersion:           aws.String("7.0.7"),
					CacheNodeType:           aws.String("cache.t3.micro"),
					CacheClusterStatus:      aws.String("available"),
					NumCacheNodes:           aws.Int64(1),
					PreferredAvailabilityZone: aws.String("us-east-1a"),
				},
			},
			expectedCount: 1,
			checkResults: func(t *testing.T, results []types.ScanResult) {
				r := results[0]
				if r.ServiceName != "ElastiCache" {
					t.Errorf("expected ServiceName 'ElastiCache', got '%s'", r.ServiceName)
				}
				if r.MethodName != "elasticache:DescribeCacheClusters" {
					t.Errorf("expected MethodName 'elasticache:DescribeCacheClusters', got '%s'", r.MethodName)
				}
				if r.ResourceType != "cluster" {
					t.Errorf("expected ResourceType 'cluster', got '%s'", r.ResourceType)
				}
				if r.ResourceName != "my-redis-cluster" {
					t.Errorf("expected ResourceName 'my-redis-cluster', got '%s'", r.ResourceName)
				}
				if r.Details["Engine"] != "redis" {
					t.Errorf("expected Engine 'redis', got '%v'", r.Details["Engine"])
				}
				if r.Details["EngineVersion"] != "7.0.7" {
					t.Errorf("expected EngineVersion '7.0.7', got '%v'", r.Details["EngineVersion"])
				}
				if r.Details["CacheNodeType"] != "cache.t3.micro" {
					t.Errorf("expected CacheNodeType 'cache.t3.micro', got '%v'", r.Details["CacheNodeType"])
				}
				if r.Details["CacheClusterStatus"] != "available" {
					t.Errorf("expected CacheClusterStatus 'available', got '%v'", r.Details["CacheClusterStatus"])
				}
				if r.Details["NumCacheNodes"] != int64(1) {
					t.Errorf("expected NumCacheNodes 1, got '%v'", r.Details["NumCacheNodes"])
				}
				if r.Details["PreferredAvailabilityZone"] != "us-east-1a" {
					t.Errorf("expected PreferredAvailabilityZone 'us-east-1a', got '%v'", r.Details["PreferredAvailabilityZone"])
				}
			},
		},
		{
			name: "Memcached cluster",
			input: []*elasticache.CacheCluster{
				{
					CacheClusterId:     aws.String("my-memcached"),
					Engine:             aws.String("memcached"),
					EngineVersion:      aws.String("1.6.22"),
					CacheNodeType:      aws.String("cache.m5.large"),
					CacheClusterStatus: aws.String("available"),
					NumCacheNodes:      aws.Int64(3),
				},
			},
			expectedCount: 1,
			checkResults: func(t *testing.T, results []types.ScanResult) {
				r := results[0]
				if r.Details["Engine"] != "memcached" {
					t.Errorf("expected Engine 'memcached', got '%v'", r.Details["Engine"])
				}
				if r.Details["NumCacheNodes"] != int64(3) {
					t.Errorf("expected NumCacheNodes 3, got '%v'", r.Details["NumCacheNodes"])
				}
			},
		},
		{
			name: "multiple clusters",
			input: []*elasticache.CacheCluster{
				{CacheClusterId: aws.String("redis-1"), Engine: aws.String("redis")},
				{CacheClusterId: aws.String("memcached-1"), Engine: aws.String("memcached")},
			},
			expectedCount: 2,
			checkResults: func(t *testing.T, results []types.ScanResult) {
				if results[0].ResourceName != "redis-1" {
					t.Errorf("expected first ResourceName 'redis-1', got '%s'", results[0].ResourceName)
				}
				if results[1].ResourceName != "memcached-1" {
					t.Errorf("expected second ResourceName 'memcached-1', got '%s'", results[1].ResourceName)
				}
			},
		},
		{
			name:          "empty results",
			input:         []*elasticache.CacheCluster{},
			expectedCount: 0,
		},
		{
			name:          "access denied error",
			input:         nil,
			err:           fmt.Errorf("AccessDeniedException: User is not authorized"),
			expectedCount: 1,
			expectError:   true,
			checkResults: func(t *testing.T, results []types.ScanResult) {
				if results[0].Error == nil {
					t.Error("expected error in result, got nil")
				}
				if results[0].ServiceName != "ElastiCache" {
					t.Errorf("expected ServiceName 'ElastiCache', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "elasticache:DescribeCacheClusters" {
					t.Errorf("expected MethodName 'elasticache:DescribeCacheClusters', got '%s'", results[0].MethodName)
				}
			},
		},
		{
			name: "nil field handling",
			input: []*elasticache.CacheCluster{
				{
					CacheClusterId:          nil,
					Engine:                  nil,
					EngineVersion:           nil,
					CacheNodeType:           nil,
					CacheClusterStatus:      nil,
					NumCacheNodes:           nil,
					PreferredAvailabilityZone: nil,
				},
			},
			expectedCount: 1,
			checkResults: func(t *testing.T, results []types.ScanResult) {
				r := results[0]
				if r.ResourceName != "" {
					t.Errorf("expected empty ResourceName for nil CacheClusterId, got '%s'", r.ResourceName)
				}
				if r.Details["Engine"] != "" {
					t.Errorf("expected empty Engine for nil, got '%v'", r.Details["Engine"])
				}
				if r.Details["EngineVersion"] != "" {
					t.Errorf("expected empty EngineVersion for nil, got '%v'", r.Details["EngineVersion"])
				}
				if r.Details["CacheNodeType"] != "" {
					t.Errorf("expected empty CacheNodeType for nil, got '%v'", r.Details["CacheNodeType"])
				}
				if r.Details["CacheClusterStatus"] != "" {
					t.Errorf("expected empty CacheClusterStatus for nil, got '%v'", r.Details["CacheClusterStatus"])
				}
				if r.Details["NumCacheNodes"] != int64(0) {
					t.Errorf("expected NumCacheNodes 0 for nil, got '%v'", r.Details["NumCacheNodes"])
				}
				if r.Details["PreferredAvailabilityZone"] != "" {
					t.Errorf("expected empty PreferredAvailabilityZone for nil, got '%v'", r.Details["PreferredAvailabilityZone"])
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			results := process(tc.input, tc.err, false)
			if len(results) != tc.expectedCount {
				t.Fatalf("expected %d results, got %d", tc.expectedCount, len(results))
			}
			if tc.checkResults != nil {
				tc.checkResults(t, results)
			}
		})
	}
}
