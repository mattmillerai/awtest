package services

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mattmillerai/awtest/cmd/awtest/types"
)

// regionsAllowlist lists non-test files that may reference types.Regions
// directly (keyed by slash-separated path relative to this directory).
var regionsAllowlist = map[string]string{
	// Single call through one regional endpoint for Direct Connect gateways;
	// intentionally not region-scoped.
	"directconnect/calls.go": "single global-ish gateway call uses types.Regions[0]",
}

// scanForDirectRegions returns every reference to the selector types.Regions in src.
func scanForDirectRegions(filename string, src []byte) ([]string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, src, 0)
	if err != nil {
		return nil, err
	}
	var found []string
	ast.Inspect(f, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == "types" && sel.Sel.Name == "Regions" {
				found = append(found, fset.Position(sel.Pos()).String())
			}
		}
		return true
	})
	return found, nil
}

func TestServicesUseRegionsFor(t *testing.T) {
	var violations []string
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		if _, ok := regionsAllowlist[filepath.ToSlash(path)]; ok {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		found, err := scanForDirectRegions(path, src)
		if err != nil {
			return err
		}
		violations = append(violations, found...)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) > 0 {
		t.Errorf("services must not reference types.Regions directly; loop over types.RegionsFor(ctx) "+
			"so that -all-regions is honored:\n  %s", strings.Join(violations, "\n  "))
	}
}

func TestDirectRegionsScannerDetects(t *testing.T) {
	bad := []byte(`package x

func f() {
	for _, r := range types.Regions {
		_ = r
	}
	_ = types.Regions[0]
}
`)
	found, err := scanForDirectRegions("x.go", bad)
	if err != nil || len(found) != 2 {
		t.Errorf("expected 2 detections, got %v (%v)", found, err)
	}
	good := []byte(`package x

func f(ctx context.Context) {
	for _, r := range types.RegionsFor(ctx) {
		_ = r
	}
}
`)
	found, err = scanForDirectRegions("y.go", good)
	if err != nil || len(found) != 0 {
		t.Errorf("clean source flagged: %v, %v", found, err)
	}
}

func TestAllServicesHonorRegionScope(t *testing.T) {
	guard, _, _ := setupGuard(t)

	var (
		mu      sync.Mutex
		current string
		signed  = map[string]map[string]bool{} // service -> signing regions
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if parts := strings.Split(r.Header.Get("Authorization"), "Credential="); len(parts) == 2 {
			scope := strings.Split(strings.SplitN(parts[1], ",", 2)[0], "/")
			if len(scope) == 5 {
				mu.Lock()
				if signed[current] == nil {
					signed[current] = map[string]bool{}
				}
				signed[current][scope[2]] = true
				mu.Unlock()
			}
		}
		w.Header().Set("Content-Type", "application/x-amz-json-1.1")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"__type":"AccessDeniedException","message":"denied"}`))
	}))
	defer srv.Close()
	guard.allowedHost = strings.TrimPrefix(srv.URL, "http://")

	scoped := []string{"eu-west-1", "ap-south-1", "af-south-1"}
	allowed := map[string]bool{"us-west-2": true} // the config's default region
	for _, r := range scoped {
		allowed[r] = true
	}

	targets := guardTargets(t)
	setAmbientEndpoints(t, targets)
	cfg := guardedV2Config(t, guard, srv.URL)

	used := 0
	scopeUsed := map[string]bool{}
	for _, tg := range targets {
		svc := tg.svc
		guard.setCurrent(svc.Name)
		mu.Lock()
		current = svc.Name
		mu.Unlock()

		ctx, scope := types.WithRegionScope(context.Background(), scoped)
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("panic in %s: %v", svc.Name, r)
				}
			}()
			ctx, cancel := context.WithTimeout(types.WithAWSConfig(ctx, cfg), 15*time.Second)
			defer cancel()
			_, _ = svc.Invoke(ctx)
		}()
		if !scope.Used() {
			continue
		}
		scopeUsed[svc.Name] = true
		used++
		mu.Lock()
		var bad []string
		for r := range signed[svc.Name] {
			if !allowed[r] {
				bad = append(bad, r)
			}
		}
		mu.Unlock()
		if len(bad) > 0 {
			sort.Strings(bad)
			t.Errorf("%s sent requests signed for unscoped region(s) %v", svc.Name, bad)
		}
	}
	guard.setCurrent("")

	if v := guard.snapshot(); len(v) > 0 {
		t.Errorf("%d request(s) bypassed the custom endpoint:\n  %s", len(v), strings.Join(v, "\n  "))
	}
	// The v2 stand-in must have consulted the scope and signed for each scoped
	// region (not only the config's default region).
	syn := syntheticV2Service().Name
	if !scopeUsed[syn] {
		t.Errorf("synthetic v2 service did not consult the region scope")
	}
	mu.Lock()
	for _, r := range scoped {
		if !signed[syn][r] {
			t.Errorf("synthetic v2 service sent no request signed for scoped region %s (signed: %v)", r, signed[syn])
		}
	}
	mu.Unlock()
	if used < 30 {
		t.Errorf("only %d services consulted the region scope, want >= 30", used)
	}
	t.Log(fmt.Sprintf("%d regional services honored the scope", used))
}
