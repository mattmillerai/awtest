package services

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"unicode"
)

var writeVerbs = []string{
	"Create", "Delete", "Put", "Start", "Stop", "Update", "Modify", "Terminate",
	"Invoke", "Send", "Publish", "Run", "Attach", "Detach", "Add", "Remove",
	"Set", "Tag", "Untag", "Enable", "Disable", "Reboot", "Reset", "Execute",
	"Upload", "Import", "Register", "Deregister", "Associate", "Disassociate",
	"Authorize", "Revoke", "Cancel", "Write", "Restore", "Rotate", "Subscribe",
	"Unsubscribe", "Purge", "Initiate", "Complete", "Abort",
}

// readOnlyAllowlist holds legitimately read-only calls whose names start with a write verb.
var readOnlyAllowlist = map[string]bool{}

func TestAllCallsAreReadOnly(t *testing.T) {
	for _, svc := range AllServices() {
		if readOnlyAllowlist[svc.Name] {
			continue
		}
		action := svc.Name
		if i := strings.Index(action, ":"); i >= 0 {
			action = action[i+1:]
		}
		for _, verb := range writeVerbs {
			if strings.HasPrefix(action, verb) {
				t.Errorf("%s looks like a write operation (prefix %q)", svc.Name, verb)
			}
		}
	}
}

func TestServiceSourcesHaveNoMutatingSDKCalls(t *testing.T) {
	fset := token.NewFileSet()
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			name := sel.Sel.Name
			for _, verb := range writeVerbs {
				rest := strings.TrimPrefix(name, verb)
				if rest != name && rest != "" && unicode.IsUpper(rune(rest[0])) {
					t.Errorf("%s: mutating SDK call %s", path, name)
					break
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
