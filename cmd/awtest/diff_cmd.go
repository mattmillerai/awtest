package main

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/MillerMedia/awtest/cmd/awtest/diff"
)

const diffUsage = `Usage:
  awtest diff [-format text|json] <before.json> <after.json>

Compare two awtest JSON scan outputs (from -format json) offline.

Flags:
  -format string
        Report format: text or json (default "text")

Exit codes:
  0  no differences
  1  differences found
  2  usage or input error
`

// isDiffInvocation reports whether args (os.Args style) select the diff subcommand.
func isDiffInvocation(args []string) bool {
	return len(args) > 1 && args[1] == "diff"
}

// runDiff implements `awtest diff` and returns the process exit code.
func runDiff(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("diff", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprint(fs.Output(), diffUsage) }
	format := fs.String("format", "text", "Report format: text or json")

	var files []string
	rest := args
	for {
		if err := fs.Parse(rest); err != nil {
			if err == flag.ErrHelp {
				fmt.Fprint(stdout, diffUsage)
				return 0
			}
			return 2 // flag package already reported the error
		}
		if fs.NArg() == 0 {
			break
		}
		files = append(files, fs.Arg(0))
		rest = fs.Args()[1:]
	}

	if len(files) != 2 {
		fmt.Fprintf(stderr, "Error: diff requires exactly two files (<before.json> <after.json>), got %d\n", len(files))
		fmt.Fprint(stderr, diffUsage)
		return 2
	}
	f := strings.ToLower(*format)
	if f != "text" && f != "json" {
		fmt.Fprintf(stderr, "Error: unsupported diff format: %s (supported: text, json)\n", *format)
		return 2
	}

	before, err := diff.Load(files[0])
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 2
	}
	after, err := diff.Load(files[1])
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 2
	}

	report := diff.Compare(before, after)
	if f == "json" {
		err = diff.RenderJSON(stdout, report)
	} else {
		err = diff.RenderText(stdout, report, files[0], files[1])
	}
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 2
	}
	if report.Identical() {
		return 0
	}
	return 1
}
