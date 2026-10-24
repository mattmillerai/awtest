package main

import (
	"bytes"
	"flag"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func newShortFlagTestSet() (*flag.FlagSet, *string, *bool) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	outputFile := fs.String("output-file", "", "Write output to file instead of stdout (short: -o)")
	version := fs.Bool("version", false, "Print version and build date (short: -v)")
	registerShortFlags(fs, outputFile, version)
	return fs, outputFile, version
}

func TestRegisterShortFlags_OutputFileAlias(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"short", []string{"-o", "x.json"}},
		{"long", []string{"-output-file", "x.json"}},
		{"shortEquals", []string{"--o=x.json"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fs, outputFile, _ := newShortFlagTestSet()
			if err := fs.Parse(tc.args); err != nil {
				t.Fatalf("Parse(%v) returned error: %v", tc.args, err)
			}
			if *outputFile != "x.json" {
				t.Errorf("Parse(%v): outputFile = %q, want %q", tc.args, *outputFile, "x.json")
			}
		})
	}
}

func TestRegisterShortFlags_VersionAlias(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"shortDash", []string{"-v"}},
		{"doubleDash", []string{"--v"}},
		{"longName", []string{"-version"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fs, _, version := newShortFlagTestSet()
			if err := fs.Parse(tc.args); err != nil {
				t.Fatalf("Parse(%v) returned error: %v", tc.args, err)
			}
			if !*version {
				t.Errorf("Parse(%v): version = false, want true", tc.args)
			}
		})
	}
}

func TestRegisterShortFlags_LookupUsage(t *testing.T) {
	fs, _, _ := newShortFlagTestSet()

	o := fs.Lookup("o")
	if o == nil {
		t.Fatal("fs.Lookup(\"o\") returned nil, want registered flag")
	}
	if o.Usage != "Shorthand for -output-file" {
		t.Errorf("Lookup(\"o\").Usage = %q, want %q", o.Usage, "Shorthand for -output-file")
	}

	v := fs.Lookup("v")
	if v == nil {
		t.Fatal("fs.Lookup(\"v\") returned nil, want registered flag")
	}
	if v.Usage != "Shorthand for -version" {
		t.Errorf("Lookup(\"v\").Usage = %q, want %q", v.Usage, "Shorthand for -version")
	}
}

// buildAwtestBinary compiles the awtest binary once for integration tests.
func buildAwtestBinary(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	binName := "awtest"
	if runtime.GOOS == "windows" {
		binName = "awtest.exe"
	}
	binPath := filepath.Join(dir, binName)

	cmd := exec.Command("go", "build",
		"-ldflags", "-X main.Version=9.9.9-test -X main.BuildDate=testdate",
		"-o", binPath, ".")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build awtest binary: %v\n%s", err, out)
	}
	return binPath
}

func TestIntegration_VersionFlagAliases(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	binPath := buildAwtestBinary(t)
	want := "awtest 9.9.9-test (built testdate)\n"

	for _, arg := range []string{"-v", "--v", "-version"} {
		t.Run(arg, func(t *testing.T) {
			cmd := exec.Command(binPath, arg)
			var stdout bytes.Buffer
			cmd.Stdout = &stdout
			if err := cmd.Run(); err != nil {
				t.Fatalf("%s: command failed: %v", arg, err)
			}
			if stdout.String() != want {
				t.Errorf("%s: stdout = %q, want %q", arg, stdout.String(), want)
			}
		})
	}
}

func TestIntegration_HelpListsShortFlags(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	binPath := buildAwtestBinary(t)

	cmd := exec.Command(binPath, "-h")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code := exitErr.ExitCode()
			if code != 0 && code != 2 {
				t.Fatalf("-h: unexpected exit code %d, stderr=%s", code, stderr.String())
			}
		} else {
			t.Fatalf("-h: command failed: %v", err)
		}
	}

	output := stderr.String()
	if !strings.Contains(output, "-o") || !strings.Contains(output, "Shorthand for -output-file") {
		t.Errorf("-h output missing -o/-output-file shorthand info:\n%s", output)
	}
	if !strings.Contains(output, "-v") || !strings.Contains(output, "Shorthand for -version") {
		t.Errorf("-h output missing -v/-version shorthand info:\n%s", output)
	}
}
