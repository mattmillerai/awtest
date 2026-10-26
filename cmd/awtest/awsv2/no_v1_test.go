package awsv2

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// v1ImportPrefix is the import path prefix of aws-sdk-go v1, which has been
// removed from the module. It is built from parts so that this file does not
// itself contain a v1 import path.
const v1ImportPrefix = "github.com/aws/" + "aws-sdk-go" + "/"

// importsV1 parses src (imports only) and returns every aws-sdk-go v1 import.
func importsV1(filename string, src []byte) ([]string, error) {
	f, err := parser.ParseFile(token.NewFileSet(), filename, src, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}
	var found []string
	for _, imp := range f.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			return nil, err
		}
		if strings.HasPrefix(path, v1ImportPrefix) || path == strings.TrimSuffix(v1ImportPrefix, "/") {
			found = append(found, path)
		}
	}
	return found, nil
}

// TestNoV1SDKImports proves aws-sdk-go v1 is gone: no Go file under cmd/awtest
// (tests included) imports it. Together with TestNoV2ClientBypass this keeps
// every service on the guarded v2 client path.
func TestNoV1SDKImports(t *testing.T) {
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	visited := 0
	var violations []string
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		visited++
		found, err := importsV1(path, src)
		if err != nil {
			return err
		}
		for _, p := range found {
			violations = append(violations, path+": imports "+p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if visited <= 100 {
		t.Fatalf("walk of %s visited only %d files; wrong path would make this guard vacuous", root, visited)
	}
	if len(violations) > 0 {
		sort.Strings(violations)
		t.Fatalf("aws-sdk-go v1 has been removed; use aws-sdk-go-v2:\n  %s", strings.Join(violations, "\n  "))
	}
}

func TestV1ImportScannerDetects(t *testing.T) {
	bad := []byte("package x\n\nimport (\n\t\"" + v1ImportPrefix + "aws/session\"\n\t\"fmt\"\n)\n")
	if got, err := importsV1("x.go", bad); err != nil || len(got) != 1 {
		t.Errorf("v1 import not detected: %v, %v", got, err)
	}
	good := []byte("package x\n\nimport (\n\t\"github.com/aws/aws-sdk-go-v2/aws\"\n\t\"fmt\"\n)\n")
	if got, err := importsV1("y.go", good); err != nil || len(got) != 0 {
		t.Errorf("v2 import flagged: %v, %v", got, err)
	}
}
