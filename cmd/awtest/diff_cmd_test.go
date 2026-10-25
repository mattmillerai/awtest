package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mattmillerai/awtest/cmd/awtest/diff"
)

func TestIsDiffInvocation(t *testing.T) {
	cases := []struct {
		args []string
		want bool
	}{
		{nil, false},
		{[]string{"awtest"}, false},
		{[]string{"awtest", "diff"}, true},
		{[]string{"awtest", "diff", "a", "b"}, true},
		{[]string{"awtest", "-format", "diff"}, false},
		{[]string{"awtest", "-o", "diff"}, false},
		{[]string{"awtest", "-quiet", "diff"}, false},
		{[]string{"awtest", "Diff"}, false},
		{[]string{"diff"}, false},
	}
	for _, tc := range cases {
		if got := isDiffInvocation(tc.args); got != tc.want {
			t.Errorf("isDiffInvocation(%v) = %v, want %v", tc.args, got, tc.want)
		}
	}
}

func TestExistingFlagParsingUnchanged(t *testing.T) {
	cases := []struct {
		name       string
		args       []string
		wantFormat string
		wantOutput string
		wantQuiet  bool
		wantVer    bool
		wantAKI    string
		wantSAK    string
		wantRest   []string
	}{
		{name: "version", args: []string{"-version"}, wantFormat: "text", wantVer: true},
		{name: "short version", args: []string{"-v"}, wantFormat: "text", wantVer: true},
		{name: "short output", args: []string{"-o", "x.json"}, wantFormat: "text", wantOutput: "x.json"},
		{name: "double dash format", args: []string{"--format=json"}, wantFormat: "json"},
		{name: "quiet", args: []string{"-quiet"}, wantFormat: "text", wantQuiet: true},
		{name: "aki sak", args: []string{"-aki", "AKIA1", "-sak", "s3cr3t"}, wantFormat: "text", wantAKI: "AKIA1", wantSAK: "s3cr3t"},
		{name: "positional leftovers", args: []string{"-quiet", "extra", "-v"}, wantFormat: "text", wantQuiet: true, wantRest: []string{"extra", "-v"}},
		{name: "diff after flags is just a positional", args: []string{"-quiet", "diff"}, wantFormat: "text", wantQuiet: true, wantRest: []string{"diff"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fs := flag.NewFlagSet("test", flag.ContinueOnError)
			fs.SetOutput(&bytes.Buffer{})
			format := fs.String("format", "text", "")
			outputFile := fs.String("output-file", "", "")
			quiet := fs.Bool("quiet", false, "")
			aki := fs.String("aki", "", "")
			sak := fs.String("sak", "", "")
			version := fs.Bool("version", false, "")
			registerShortFlags(fs, outputFile, version)

			if err := fs.Parse(tc.args); err != nil {
				t.Fatalf("Parse(%v): %v", tc.args, err)
			}
			if *format != tc.wantFormat || *outputFile != tc.wantOutput || *quiet != tc.wantQuiet ||
				*version != tc.wantVer || *aki != tc.wantAKI || *sak != tc.wantSAK {
				t.Errorf("parsed format=%q out=%q quiet=%v version=%v aki=%q sak=%q", *format, *outputFile, *quiet, *version, *aki, *sak)
			}
			if strings.Join(fs.Args(), "|") != strings.Join(tc.wantRest, "|") {
				t.Errorf("rest = %v, want %v", fs.Args(), tc.wantRest)
			}
			if isDiffInvocation(append([]string{"awtest"}, tc.args...)) {
				t.Errorf("isDiffInvocation true for %v", tc.args)
			}
		})
	}
}

func TestRunDiff(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	before := write("before.json", `[{"serviceName":"S3","methodName":"s3:L","resourceType":"bucket","resourceName":"a","timestamp":"t1"}]`)
	same := write("same.json", `{"metadata":{},"results":[{"serviceName":"S3","methodName":"s3:L","resourceType":"bucket","resourceName":"a","timestamp":"t2"}]}`)
	after := write("after.json", `[{"serviceName":"S3","methodName":"s3:L","resourceType":"bucket","resourceName":"b"}]`)
	bad := write("bad.json", "- yaml: true\n")
	missing := filepath.Join(dir, "nope.json")

	cases := []struct {
		name       string
		args       []string
		wantCode   int
		wantOut    []string
		wantErr    []string
		notWantOut []string
	}{
		{"no differences", []string{before, same}, 0, []string{"No differences."}, nil, nil},
		{"differences", []string{before, after}, 1, []string{"  + s3:L  b (bucket)", "  - s3:L  a (bucket)", "Summary: 1 added, 1 removed, 0 changed"}, nil, nil},
		{"json", []string{"-format", "json", before, after}, 1, []string{`"identical":false`}, nil, nil},
		{"json upper-case", []string{"-format=JSON", before, same}, 0, []string{`"identical":true`}, nil, nil},
		{"flags after positionals", []string{before, after, "-format", "json"}, 1, []string{`"added":[{`}, nil, []string{"Comparing"}},
		{"flag between positionals", []string{before, "-format=json", after}, 1, []string{`"summary":{"added":1,"removed":1,"changed":0}`}, nil, nil},
		{"help", []string{"-h"}, 0, []string{"awtest diff [-format text|json]"}, nil, nil},
		{"help long", []string{"-help"}, 0, []string{"Exit codes:"}, nil, nil},
		{"no args", nil, 2, nil, []string{"Error: ", "exactly two files"}, nil},
		{"one arg", []string{before}, 2, nil, []string{"Error: ", "got 1"}, nil},
		{"three args", []string{before, after, same}, 2, nil, []string{"Error: ", "got 3"}, nil},
		{"bad format", []string{"-format", "yaml", before, after}, 2, nil, []string{"Error: ", "unsupported diff format: yaml"}, nil},
		{"unknown flag", []string{"-bogus", before, after}, 2, nil, []string{"bogus"}, nil},
		{"missing file", []string{before, missing}, 2, nil, []string{"Error: ", missing}, nil},
		{"invalid file", []string{bad, before}, 2, nil, []string{"Error: ", "bad.json", "-format json"}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := runDiff(tc.args, &stdout, &stderr)
			if code != tc.wantCode {
				t.Fatalf("exit = %d, want %d\nstdout=%s\nstderr=%s", code, tc.wantCode, stdout.String(), stderr.String())
			}
			for _, w := range tc.wantOut {
				if !strings.Contains(stdout.String(), w) {
					t.Errorf("stdout missing %q:\n%s", w, stdout.String())
				}
			}
			for _, w := range tc.notWantOut {
				if strings.Contains(stdout.String(), w) {
					t.Errorf("stdout unexpectedly contains %q:\n%s", w, stdout.String())
				}
			}
			for _, w := range tc.wantErr {
				if !strings.Contains(stderr.String(), w) {
					t.Errorf("stderr missing %q:\n%s", w, stderr.String())
				}
			}
			if tc.wantCode == 2 && stdout.Len() != 0 {
				t.Errorf("stdout should be empty on error, got %q", stdout.String())
			}
		})
	}
}

func TestFixturesParse(t *testing.T) {
	for _, name := range []string{"before.json", "after.json"} {
		entries, err := diff.Load(filepath.Join("testdata", name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(entries) == 0 {
			t.Errorf("%s: no entries", name)
		}
	}
}

func runBinary(t *testing.T, bin string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	if err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("running %v: %v", args, err)
		}
		code = ee.ExitCode()
	}
	return out.String(), errb.String(), code
}

func TestIntegration_Diff(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	bin := buildAwtestBinary(t)
	before, after := filepath.Join("testdata", "before.json"), filepath.Join("testdata", "after.json")

	t.Run("text differences", func(t *testing.T) {
		out, errOut, code := runBinary(t, bin, "diff", before, after)
		if code != 1 {
			t.Fatalf("exit = %d, stderr=%s", code, errOut)
		}
		for _, w := range []string{
			"Added resources (2):",
			"  + s3:ListBuckets  new-bucket (bucket)",
			"  + iam:ListUsers  alice (user)",
			"Removed resources (2):",
			"  - s3:ListBuckets  old-bucket (bucket)",
			"Changed calls (2):",
			"  ~ ec2:DescribeInstances  ok -> error (newly denied)",
			"  ~ iam:ListUsers  error -> ok (newly allowed)",
			"Summary: 2 added, 2 removed, 2 changed",
		} {
			if !strings.Contains(out, w) {
				t.Errorf("stdout missing %q:\n%s", w, out)
			}
		}
		if strings.Contains(out, "logs-bucket") {
			t.Errorf("unchanged resource reported:\n%s", out)
		}
	})

	t.Run("json differences", func(t *testing.T) {
		out, errOut, code := runBinary(t, bin, "diff", "-format", "json", before, after)
		if code != 1 {
			t.Fatalf("exit = %d, stderr=%s", code, errOut)
		}
		var got struct {
			Identical bool           `json:"identical"`
			Summary   map[string]int `json:"summary"`
		}
		if err := json.Unmarshal([]byte(out), &got); err != nil {
			t.Fatalf("stdout is not JSON: %v\n%s", err, out)
		}
		if got.Identical || got.Summary["added"] != 2 || got.Summary["removed"] != 2 || got.Summary["changed"] != 2 {
			t.Errorf("unexpected report: %+v", got)
		}
	})

	t.Run("identical", func(t *testing.T) {
		out, _, code := runBinary(t, bin, "diff", before, before)
		if code != 0 || !strings.Contains(out, "No differences.") {
			t.Errorf("exit = %d, stdout=%s", code, out)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		_, errOut, code := runBinary(t, bin, "diff", before, filepath.Join("testdata", "nope.json"))
		if code != 2 || !strings.HasPrefix(errOut, "Error: ") {
			t.Errorf("exit = %d, stderr=%s", code, errOut)
		}
	})

	t.Run("bad arg count", func(t *testing.T) {
		_, errOut, code := runBinary(t, bin, "diff", before)
		if code != 2 || !strings.Contains(errOut, "Error: ") {
			t.Errorf("exit = %d, stderr=%s", code, errOut)
		}
	})
}

func TestIntegration_ExistingInvocationsUnchanged(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	bin := buildAwtestBinary(t)
	want := "awtest 9.9.9-test (built testdate)\n"
	for _, arg := range []string{"-version", "-v"} {
		out, _, code := runBinary(t, bin, arg)
		if code != 0 || out != want {
			t.Errorf("%s: exit=%d stdout=%q", arg, code, out)
		}
	}

	// -format is validated (main.go getFormatter) after session creation with
	// static credentials, which makes no network calls, and before any scan.
	// -quiet avoids the background update check that release builds start.
	_, errOut, code := runBinary(t, bin, "-quiet", "-format", "bogus", "-aki", "AKIAFAKE", "-sak", "fake")
	if code != 1 || !strings.Contains(errOut, "unsupported format") {
		t.Errorf("bogus format: exit=%d stderr=%q", code, errOut)
	}
}
