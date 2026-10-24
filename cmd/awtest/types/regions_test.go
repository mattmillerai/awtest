package types

import (
	"context"
	"reflect"
	"regexp"
	"testing"
)

func TestRegionsForDefault(t *testing.T) {
	got := RegionsFor(context.Background())
	if !reflect.DeepEqual(got, Regions) {
		t.Fatalf("RegionsFor(no scope) = %v, want %v", got, Regions)
	}
	if len(got) != 4 {
		t.Errorf("default should be 4 regions, got %d", len(got))
	}
	got[0] = "mutated"
	if Regions[0] == "mutated" {
		t.Error("RegionsFor returned an alias of Regions, not a copy")
	}
}

func TestRegionsForScoped(t *testing.T) {
	in := []string{"eu-west-1", "ap-south-1"}
	ctx, scope := WithRegionScope(context.Background(), in)
	if scope.Used() {
		t.Fatal("scope must not be Used before RegionsFor is called")
	}
	// Scope is untouched by unrelated context derivation.
	child, cancel := context.WithCancel(ctx)
	defer cancel()
	if scope.Used() {
		t.Fatal("deriving a context must not mark the scope used")
	}
	got := RegionsFor(child)
	if !reflect.DeepEqual(got, in) {
		t.Fatalf("RegionsFor = %v, want %v", got, in)
	}
	if !scope.Used() {
		t.Error("scope should be Used after RegionsFor")
	}
	got[0] = "mutated"
	in[1] = "changed"
	again := RegionsFor(ctx)
	if !reflect.DeepEqual(again, []string{"eu-west-1", "ap-south-1"}) {
		t.Errorf("scope regions were aliased: %v", again)
	}
}

func TestRegionsForNestedScopeInnerWins(t *testing.T) {
	outerCtx, outer := WithRegionScope(context.Background(), []string{"us-east-1"})
	innerCtx, inner := WithRegionScope(outerCtx, []string{"eu-west-1"})
	if got := RegionsFor(innerCtx); !reflect.DeepEqual(got, []string{"eu-west-1"}) {
		t.Errorf("got %v", got)
	}
	if !inner.Used() || outer.Used() {
		t.Errorf("inner.Used=%v outer.Used=%v, want true/false", inner.Used(), outer.Used())
	}
}

func TestStaticRegionLists(t *testing.T) {
	pattern := regexp.MustCompile(`^[a-z]{2}(-[a-z]+)+-\d$`)
	seen := map[string]string{}
	for name, list := range map[string][]string{"DefaultEnabledRegions": DefaultEnabledRegions, "OptInRegions": OptInRegions} {
		for _, r := range list {
			if !pattern.MatchString(r) {
				t.Errorf("%s: %q is not a valid region name", name, r)
			}
			if prev, dup := seen[r]; dup {
				t.Errorf("%s: %q already present in %s", name, r, prev)
			}
			seen[r] = name
		}
	}
	for _, r := range Regions {
		found := false
		for _, d := range DefaultEnabledRegions {
			found = found || d == r
		}
		if !found {
			t.Errorf("default region %q missing from DefaultEnabledRegions", r)
		}
	}
	if len(DefaultEnabledRegions) != 17 {
		t.Errorf("DefaultEnabledRegions has %d entries, want 17", len(DefaultEnabledRegions))
	}
}
