package awsv2

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// allowedSchemas maps an SDK service module name to the reason its schemas
// package is acceptable. Empty on purpose: see CONTRIBUTING.md, "Pinned
// aws-sdk-go-v2 service modules".
var allowedSchemas = map[string]string{}

// findRepoRoot walks up from the working directory until it finds go.mod.
func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found in any parent directory")
		}
		dir = parent
	}
}

// schemaViolations returns the service modules under vendorRoot that ship a
// schemas package and are not listed in allowedSchemas.
func schemaViolations(vendorRoot string) ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(vendorRoot, "github.com/aws/aws-sdk-go-v2/service/*/schemas"))
	if err != nil {
		return nil, err
	}
	var bad []string
	for _, m := range matches {
		if fi, err := os.Stat(m); err != nil || !fi.IsDir() {
			continue
		}
		svc := filepath.Base(filepath.Dir(m))
		if _, ok := allowedSchemas[svc]; ok {
			continue
		}
		bad = append(bad, svc)
	}
	sort.Strings(bad)
	return bad, nil
}

func TestNoSDKSchemasPackages(t *testing.T) {
	vendorRoot := filepath.Join(findRepoRoot(t), "vendor")
	if _, err := os.Stat(vendorRoot); err != nil {
		t.Skip("no vendor directory")
	}
	bad, err := schemaViolations(vendorRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(bad) > 0 {
		t.Fatalf("these aws-sdk-go-v2 service modules ship a schemas package: %s\n"+
			"They were bumped past the last pre-schema release; the schemas package adds ~14 MB "+
			"and ~30 ms startup (aws/aws-sdk-go-v2#3601). Pin the module back to the version listed in "+
			"CONTRIBUTING.md (\"Pinned aws-sdk-go-v2 service modules\"), or, if upstream fixed the issue, "+
			"add an allowedSchemas entry / remove the pins as documented there.",
			strings.Join(bad, ", "))
	}
}

func TestSchemaViolationsDetectsSchemasDir(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "github.com/aws/aws-sdk-go-v2/service")
	for _, d := range []string{"foo/schemas", "ok/api"} {
		if err := os.MkdirAll(filepath.Join(base, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	got, err := schemaViolations(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "foo" {
		t.Fatalf("violations = %v, want [foo]", got)
	}
}
