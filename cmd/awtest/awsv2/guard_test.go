package awsv2

import (
	"fmt"
	"go/ast"
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

const (
	v2Prefix       = "github.com/aws/aws-sdk-go-v2"
	v2AwsPath      = "github.com/aws/aws-sdk-go-v2/aws"
	v2ConfigPath   = "github.com/aws/aws-sdk-go-v2/config"
	v2CredsPath    = "github.com/aws/aws-sdk-go-v2/credentials"
	awsv2PkgPath   = "github.com/mattmillerai/awtest/cmd/awtest/awsv2"
	v2ServicePath  = "github.com/aws/aws-sdk-go-v2/service/"
	v2FeaturePath  = "github.com/aws/aws-sdk-go-v2/feature/"
	bypassGuidance = "build v2 clients only with awsv2.NewClient or awsv2.NewRegionalClient (cmd/awtest/awsv2)"
)

var (
	// sanctionedCallees are the awsv2 functions that may be handed a service
	// constructor. Adding to this set widens the guard; do so deliberately.
	sanctionedCallees = map[string]bool{
		"NewClient":         true,
		"NewRegionalClient": true,
	}
	forbiddenV2Ctors = map[string]bool{
		"New":              true,
		"NewFromConfig":    true,
		"NewPresignClient": true,
		"NewDownloader":    true,
		"NewUploader":      true,
	}
	forbiddenV2Keys = map[string]bool{
		"BaseEndpoint":                true,
		"UsePathStyle":                true,
		"EndpointResolver":            true,
		"EndpointResolverV2":          true,
		"EndpointResolverWithOptions": true,
		"HTTPClient":                  true,
		"UseARNRegion":                true,
	}
)

func isV2ServiceOrFeature(path string) bool {
	return strings.HasPrefix(path, v2ServicePath) || strings.HasPrefix(path, v2FeaturePath)
}

// scanV2Bypass parses src and returns a description of every place a file that
// imports aws-sdk-go-v2 loads config, builds credentials, constructs a client
// without awsv2.NewClient, or sets an endpoint-related setting directly.
func scanV2Bypass(filename string, src []byte) ([]string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, src, 0)
	if err != nil {
		return nil, err
	}

	var found []string
	report := func(pos token.Pos, format string, args ...any) {
		found = append(found, fmt.Sprintf("%s: %s", fset.Position(pos), fmt.Sprintf(format, args...)))
	}

	// local import name -> import path
	imports := map[string]string{}
	usesV2 := false
	for _, imp := range f.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			return nil, err
		}
		isV2 := path == v2Prefix || strings.HasPrefix(path, v2Prefix+"/")
		if isV2 {
			usesV2 = true
		}
		if path == v2ConfigPath || path == v2CredsPath {
			report(imp.Pos(), "imports %s; only package awsv2 may load config or build credentials", path)
		}
		name := ""
		if imp.Name != nil {
			name = imp.Name.Name
		}
		switch name {
		case "_":
			continue
		case ".":
			if isV2ServiceOrFeature(path) || path == v2AwsPath {
				report(imp.Pos(), "dot-imports %s; references cannot be checked", path)
			}
			continue
		case "":
			name = path[strings.LastIndex(path, "/")+1:]
		}
		imports[name] = path
	}
	if !usesV2 {
		return nil, nil
	}

	// pathOf returns the import path of a package-qualified selector's X.
	pathOf := func(sel *ast.SelectorExpr) string {
		id, ok := sel.X.(*ast.Ident)
		if !ok || id.Obj != nil { // Obj != nil: shadowed by a local declaration
			return ""
		}
		return imports[id.Name]
	}

	// sanctioned holds constructor selectors passed directly to awsv2.NewClient.
	sanctioned := map[*ast.SelectorExpr]bool{}
	isNewClient := func(fun ast.Expr) bool {
		switch x := fun.(type) {
		case *ast.IndexExpr:
			fun = x.X
		case *ast.IndexListExpr:
			fun = x.X
		}
		sel, ok := fun.(*ast.SelectorExpr)
		return ok && sanctionedCallees[sel.Sel.Name] && pathOf(sel) == awsv2PkgPath
	}

	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.CallExpr:
			if isNewClient(x.Fun) {
				for _, arg := range x.Args {
					if sel, ok := arg.(*ast.SelectorExpr); ok {
						sanctioned[sel] = true
					}
				}
			}
		case *ast.SelectorExpr:
			if p := pathOf(x); isV2ServiceOrFeature(p) && forbiddenV2Ctors[x.Sel.Name] && !sanctioned[x] {
				report(x.Pos(), "reference to %s.%s outside awsv2.NewClient", p, x.Sel.Name)
			}
			if forbiddenV2Keys[x.Sel.Name] {
				report(x.Sel.Pos(), "selector sets or reads %s", x.Sel.Name)
			}
		case *ast.CompositeLit:
			if sel, ok := x.Type.(*ast.SelectorExpr); ok {
				p := pathOf(sel)
				if isV2ServiceOrFeature(p) && sel.Sel.Name == "Options" {
					// Only client Options are forbidden; request inputs
					// (sts.GetCallerIdentityInput{...}) are how a service
					// builds its calls.
					report(x.Pos(), "composite literal of %s.%s", p, sel.Sel.Name)
				} else if p == v2AwsPath && sel.Sel.Name == "Config" {
					report(x.Pos(), "composite literal of aws.Config")
				}
			}
			for _, elt := range x.Elts {
				if kv, ok := elt.(*ast.KeyValueExpr); ok {
					if key, ok := kv.Key.(*ast.Ident); ok && forbiddenV2Keys[key.Name] {
						report(kv.Pos(), "composite literal sets %s", key.Name)
					}
				}
			}
		}
		return true
	})
	return found, nil
}

func TestNoV2ClientBypass(t *testing.T) {
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	self, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}

	visited := 0
	var violations []string
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path == self {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		visited++
		found, err := scanV2Bypass(path, src)
		if err != nil {
			return err
		}
		violations = append(violations, found...)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if visited <= 50 {
		t.Fatalf("walk of %s visited only %d files; wrong path would make this guard vacuous", root, visited)
	}
	if len(violations) > 0 {
		sort.Strings(violations)
		t.Fatalf("v2 client bypass found; %s:\n  %s", bypassGuidance, strings.Join(violations, "\n  "))
	}
}

func TestV2BypassScannerDetects(t *testing.T) {
	const hdr = "package x\n\n"
	cases := []struct {
		name string
		src  string
		want int // minimum number of violations; 0 means must be clean
	}{
		{"config import", hdr + `import "github.com/aws/aws-sdk-go-v2/config"
var _ = config.LoadDefaultConfig`, 1},
		{"credentials import", hdr + `import "github.com/aws/aws-sdk-go-v2/credentials"
var _ = credentials.NewStaticCredentialsProvider`, 1},
		{"aliased config import", hdr + `import c "github.com/aws/aws-sdk-go-v2/config"
var _ = c.LoadDefaultConfig`, 1},
		{"NewFromConfig call", hdr + `import "github.com/aws/aws-sdk-go-v2/service/sts"
func f() { _ = sts.NewFromConfig(nil) }`, 1},
		{"aliased NewFromConfig call", hdr + `import stsx "github.com/aws/aws-sdk-go-v2/service/sts"
func f() { _ = stsx.NewFromConfig(nil) }`, 1},
		{"function value", hdr + `import "github.com/aws/aws-sdk-go-v2/service/sts"
func f() { g := sts.NewFromConfig; _ = g }`, 1},
		{"New on service", hdr + `import "github.com/aws/aws-sdk-go-v2/service/s3"
func f() { _ = s3.New(s3.Options{}) }`, 2},
		{"presign client", hdr + `import "github.com/aws/aws-sdk-go-v2/service/s3"
func f() { _ = s3.NewPresignClient(nil) }`, 1},
		{"feature downloader", hdr + `import "github.com/aws/aws-sdk-go-v2/feature/s3/manager"
func f() { _ = manager.NewDownloader(nil); _ = manager.NewUploader(nil) }`, 2},
		{"NewFromConfig wrapped, not a direct argument", hdr + `import (
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
)
func f() { awsv2.NewClient(nil, awsv2.ClientParams{}, wrap(sts.NewFromConfig)) }`, 1},
		{"NewFromConfig passed to a different NewClient", hdr + `import (
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"example.com/other"
)
func f() { other.NewClient(nil, sts.NewFromConfig) }`, 1},
		{"service options literal", hdr + `import "github.com/aws/aws-sdk-go-v2/service/sts"
var _ = sts.Options{}`, 1},
		{"service input literal is allowed", hdr + `import "github.com/aws/aws-sdk-go-v2/service/sts"
var _ = &sts.GetCallerIdentityInput{}`, 0},
		{"aws.Config literal", hdr + `import "github.com/aws/aws-sdk-go-v2/aws"
var _ = aws.Config{}`, 1},
		{"&aws.Config literal", hdr + `import "github.com/aws/aws-sdk-go-v2/aws"
var _ = &aws.Config{}`, 1},
		{"BaseEndpoint assign", hdr + `import "github.com/aws/aws-sdk-go-v2/aws"
func f(o *opts) { o.BaseEndpoint = aws.String("x") }`, 1},
		{"UsePathStyle assign", hdr + `import "github.com/aws/aws-sdk-go-v2/aws"
var _ = aws.String
func f(o *opts) { o.UsePathStyle = true }`, 1},
		{"keyed HTTPClient", hdr + `import "github.com/aws/aws-sdk-go-v2/aws"
var _ = aws.String
var _ = thing{HTTPClient: nil}`, 1},
		{"keyed BaseEndpoint", hdr + `import "github.com/aws/aws-sdk-go-v2/aws"
var _ = aws.String
var _ = thing{BaseEndpoint: nil}`, 1},
		{"resolver and ARN region selectors", hdr + `import "github.com/aws/aws-sdk-go-v2/aws"
var _ = aws.String
func f(o *opts) { o.EndpointResolverV2 = nil; o.EndpointResolverWithOptions = nil; o.EndpointResolver = nil; o.UseARNRegion = true }`, 4},
		{"dot import of service", hdr + `import . "github.com/aws/aws-sdk-go-v2/service/sts"
var _ = NewFromConfig`, 1},

		{"sanctioned NewClient", hdr + `import (
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
)
func f(cfg int) { _ = awsv2.NewClient(cfg, awsv2.ClientParams{Region: "r", Endpoint: "e"}, sts.NewFromConfig) }`, 0},
		{"sanctioned generic NewClient", hdr + `import (
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
)
func f(cfg int) {
	_ = awsv2.NewClient[sts.Client, sts.Options](cfg, awsv2.ClientParams{}, sts.NewFromConfig)
	_ = awsv2.NewClient[sts.Client](cfg, awsv2.ClientParams{}, sts.NewFromConfig)
}`, 0},
		{"sanctioned NewRegionalClient", hdr + `import (
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
)
func f(cfg int) {
	_ = awsv2.NewRegionalClient(cfg, "us-east-1", sts.NewFromConfig)
	_ = awsv2.NewRegionalClient[sts.Client, sts.Options](cfg, "us-east-1", sts.NewFromConfig)
}`, 0},
		{"unsanctioned awsv2 function", hdr + `import (
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
)
func f(cfg int) { _ = awsv2.NewOtherClient(cfg, sts.NewFromConfig) }`, 1},
		{"NewRegionalClient from another package", hdr + `import (
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"example.com/other"
)
func f() { other.NewRegionalClient(nil, "r", sts.NewFromConfig) }`, 1},
		{"sanctioned aliased awsv2", hdr + `import (
	"github.com/aws/aws-sdk-go-v2/service/sts"
	av2 "github.com/mattmillerai/awtest/cmd/awtest/awsv2"
)
func f(cfg int) { _ = av2.NewClient(cfg, av2.ClientParams{}, sts.NewFromConfig) }`, 0},
		{"sanctioned aliased service", hdr + `import (
	stsx "github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
)
func f(cfg int) { _ = awsv2.NewClient(cfg, awsv2.ClientParams{}, stsx.NewFromConfig) }`, 0},
		{"aws helpers only", hdr + `import "github.com/aws/aws-sdk-go-v2/aws"
func f() string { return aws.ToString(aws.String("x")) }`, 0},
		{"Region key is allowed", hdr + `import "github.com/aws/aws-sdk-go-v2/aws"
var _ = aws.String
var _ = thing{Region: "r"}`, 0},
		{"Options Transport is the sanctioned way to restrict the HTTP client", hdr + `import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
)
func f(rt int) (aws.Config, error) {
	return awsv2.LoadConfig(nil, awsv2.Options{Region: "r", Transport: rt, HTTPTimeout: 1})
}`, 0},
		{"Options HTTPClient is still forbidden outside tests", hdr + `import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/mattmillerai/awtest/cmd/awtest/awsv2"
)
func f(c int) (aws.Config, error) {
	return awsv2.LoadConfig(nil, awsv2.Options{Region: "r", HTTPClient: c})
}`, 1},
		{"files that import no aws-sdk-go-v2 package are ignored", hdr + `import (
	"example.com/other/aws"
	"example.com/other/service/sts"
)
func f() { _ = sts.New(nil); _ = aws.Config{}; _ = thing{HTTPClient: nil} }`, 0},
		{"no imports", hdr + `func f(o *opts) { o.BaseEndpoint = nil }`, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := scanV2Bypass("x.go", []byte(tc.src))
			if err != nil {
				t.Fatal(err)
			}
			if tc.want == 0 && len(got) != 0 {
				t.Fatalf("expected clean, got %v", got)
			}
			if tc.want > 0 && len(got) < tc.want {
				t.Fatalf("expected at least %d violations, got %v", tc.want, got)
			}
		})
	}
}
