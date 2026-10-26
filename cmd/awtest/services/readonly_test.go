package services

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
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

// writeVerbPrefix returns the write verb action starts with, or "".
func writeVerbPrefix(action string) string {
	for _, verb := range writeVerbs {
		if strings.HasPrefix(action, verb) {
			return verb
		}
	}
	return ""
}

// mutatingCalls returns the name of every method call in src whose name is a
// write verb followed by an upper-case letter (DeleteBucket, PutObject, ...).
func mutatingCalls(filename string, src []byte) ([]string, error) {
	f, err := parser.ParseFile(token.NewFileSet(), filename, src, 0)
	if err != nil {
		return nil, err
	}
	var found []string
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
				found = append(found, name)
				break
			}
		}
		return true
	})
	return found, nil
}

func TestAllCallsAreReadOnly(t *testing.T) {
	for _, svc := range AllServices() {
		if readOnlyAllowlist[svc.Name] {
			continue
		}
		action := svc.Name
		if i := strings.Index(action, ":"); i >= 0 {
			action = action[i+1:]
		}
		if verb := writeVerbPrefix(action); verb != "" {
			t.Errorf("%s looks like a write operation (prefix %q)", svc.Name, verb)
		}
	}
}

func TestServiceSourcesHaveNoMutatingSDKCalls(t *testing.T) {
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		found, err := mutatingCalls(path, src)
		if err != nil {
			return err
		}
		for _, name := range found {
			t.Errorf("%s: mutating SDK call %s", path, name)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// Mutation-style self-test: the checks above must catch a write verb, both as
// a service action name and as a v2 client method call, and must not flag
// read-only calls.
func TestReadOnlyChecksCatchWriteVerbs(t *testing.T) {
	for _, action := range []string{"DeleteBucket", "PutBucketPolicy", "TerminateInstances", "CreateQueue", "SendMessage"} {
		if writeVerbPrefix(action) == "" {
			t.Errorf("write action %s not caught", action)
		}
	}
	for _, action := range []string{"ListBuckets", "DescribeInstances", "GetBucketPolicy", "ListObjectsV2"} {
		if v := writeVerbPrefix(action); v != "" {
			t.Errorf("read action %s flagged as write (%s)", action, v)
		}
	}

	bad := []byte(`package x

import "github.com/aws/aws-sdk-go-v2/service/s3"

func f(ctx context.Context, svc *s3.Client) {
	_, _ = svc.DeleteBucket(ctx, &s3.DeleteBucketInput{})
	_, _ = svc.PutObject(ctx, &s3.PutObjectInput{})
}
`)
	found, err := mutatingCalls("bad.go", bad)
	if err != nil || len(found) != 2 {
		t.Errorf("expected 2 mutating calls, got %v (%v)", found, err)
	}

	good := []byte(`package x

import "github.com/aws/aws-sdk-go-v2/service/s3"

func f(ctx context.Context, svc *s3.Client) {
	_, _ = svc.ListBuckets(ctx, &s3.ListBucketsInput{})
	_, _ = svc.GetBucketPolicy(ctx, &s3.GetBucketPolicyInput{})
	p := s3.NewListObjectsV2Paginator(svc, &s3.ListObjectsV2Input{})
	_ = p
}
`)
	found, err = mutatingCalls("good.go", good)
	if err != nil || len(found) != 0 {
		t.Errorf("read-only source flagged: %v (%v)", found, err)
	}
}
