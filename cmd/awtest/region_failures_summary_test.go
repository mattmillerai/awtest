package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrintRegionFailuresSummaryLine(t *testing.T) {
	withFailures := &regionFailures{}
	withFailures.record("af-south-1", "ec2:Describe", "not enabled")

	tests := []struct {
		name       string
		f          *regionFailures
		suppressed int
		want       []string
		wantNot    []string
		wantLines  int
	}{
		{"nothing to report", &regionFailures{}, 0, nil, []string{"All-regions"}, 0},
		{"only suppressed", &regionFailures{}, 7, []string{"All-regions:", "7 repeated per-region error line(s) were suppressed."}, nil, 1},
		{"failures without suppressed", withFailures, 0, []string{"1 region-level call(s) failed", "af-south-1"}, []string{"suppressed"}, 1},
		{"failures and suppressed on one line", withFailures, 3, []string{"af-south-1", "3 repeated per-region error line(s) were suppressed."}, nil, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			printRegionFailures(&buf, tc.f, false, tc.suppressed)
			out := buf.String()
			for _, w := range tc.want {
				if !strings.Contains(out, w) {
					t.Errorf("output %q missing %q", out, w)
				}
			}
			for _, w := range tc.wantNot {
				if strings.Contains(out, w) {
					t.Errorf("output %q should not contain %q", out, w)
				}
			}
			if got := strings.Count(out, "\n"); got != tc.wantLines {
				t.Errorf("lines = %d, want %d (%q)", got, tc.wantLines, out)
			}
		})
	}
}
