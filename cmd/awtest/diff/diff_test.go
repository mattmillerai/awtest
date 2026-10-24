package diff

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/MillerMedia/awtest/cmd/awtest/formatters"
	"github.com/MillerMedia/awtest/cmd/awtest/types"
)

func TestParse(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    int
		wantErr string
	}{
		{"bare array", `[{"serviceName":"S3","methodName":"s3:ListBuckets","resourceName":"b"}]`, 1, ""},
		{"envelope", `{"metadata":{},"results":[{"methodName":"a:B"},{"methodName":"a:C"}]}`, 2, ""},
		{"empty array", `[]`, 0, ""},
		{"empty results", `{"metadata":{},"results":[]}`, 0, ""},
		{"missing results key", `{"metadata":{}}`, 0, `no "results" array`},
		{"results not array", `{"results":{}}`, 0, `"results" must be an array`},
		{"scalar top level", `42`, 0, "array or an object"},
		{"non-object entries", `[1,"x"]`, 0, "must be a JSON object"},
		{"missing methodName", `[{"serviceName":"S3"}]`, 0, "methodName"},
		{"empty methodName", `[{"methodName":""}]`, 0, "methodName"},
		{"non-string methodName", `[{"methodName":5}]`, 0, "must be a string"},
		{"invalid JSON", `{"results":[`, 0, "not valid JSON"},
		{"empty input", ``, 0, "not valid JSON"},
		{"yaml-looking", "- serviceName: S3\n  methodName: s3:ListBuckets\n", 0, "not yaml, csv or text"},
		{"trailing data", `[] []`, 0, "unexpected data after"},
		{"trailing garbage", `[]xyz`, 0, "unexpected data after"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Parse([]byte(tc.input), "f.json")
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tc.wantErr)
				}
				if !strings.Contains(err.Error(), "f.json") {
					t.Errorf("err %q does not mention file name", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != tc.want {
				t.Errorf("len = %d, want %d", len(got), tc.want)
			}
		})
	}
}

func TestLoadUnreadableFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "missing.json")
	_, err := Load(p)
	if err == nil || !strings.Contains(err.Error(), p) || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("err = %v, want path and cause", err)
	}
}

func TestLoadAndParseReader(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.json")
	if err := os.WriteFile(p, []byte(`[{"methodName":"a:B","error":"denied"}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Load(p)
	if err != nil || len(got) != 1 || got[0].Error != "denied" {
		t.Fatalf("Load = %v, %v", got, err)
	}
	got, err = ParseReader(strings.NewReader(`[]`), "r")
	if err != nil || len(got) != 0 {
		t.Fatalf("ParseReader = %v, %v", got, err)
	}
}

func res(svc, method, name string) Entry {
	return Entry{ServiceName: svc, MethodName: method, ResourceType: "t", ResourceName: name}
}

func fail(svc, method string) Entry {
	return Entry{ServiceName: svc, MethodName: method, Error: "AccessDenied"}
}

func TestCompare(t *testing.T) {
	cases := []struct {
		name        string
		before      []Entry
		after       []Entry
		wantAdded   []string
		wantRemoved []string
		wantChanged []CallChange
	}{
		{name: "no diff", before: []Entry{res("S3", "s3:L", "a")}, after: []Entry{res("S3", "s3:L", "a")}},
		{name: "both empty"},
		{
			name: "added", before: []Entry{res("S3", "s3:L", "a")},
			after:     []Entry{res("S3", "s3:L", "b"), res("S3", "s3:L", "a")},
			wantAdded: []string{"b"},
		},
		{
			name: "removed", before: []Entry{res("S3", "s3:L", "a"), res("S3", "s3:L", "b")},
			after:       []Entry{res("S3", "s3:L", "a")},
			wantRemoved: []string{"b"},
		},
		{
			name: "error to ok", before: []Entry{fail("IAM", "iam:L")}, after: []Entry{res("IAM", "iam:L", "u")},
			wantAdded:   []string{"u"},
			wantChanged: []CallChange{{"IAM", "iam:L", "error", "ok", ChangeNewlyAllowed}},
		},
		{
			name: "error to partial", before: []Entry{fail("IAM", "iam:L")},
			after:       []Entry{res("IAM", "iam:L", "u"), fail("IAM", "iam:L")},
			wantAdded:   []string{"u"},
			wantChanged: []CallChange{{"IAM", "iam:L", "error", "partial", ChangeNewlyAllowed}},
		},
		{
			name: "ok to error", before: []Entry{res("EC2", "ec2:D", "i")}, after: []Entry{fail("EC2", "ec2:D")},
			wantRemoved: []string{"i"},
			wantChanged: []CallChange{{"EC2", "ec2:D", "ok", "error", ChangeNewlyDenied}},
		},
		{
			name: "none to error", after: []Entry{fail("EC2", "ec2:D")},
			wantChanged: []CallChange{{"EC2", "ec2:D", "none", "error", ChangeNewlyDenied}},
		},
		{
			name: "partial to error", before: []Entry{res("A", "a:b", "x"), fail("A", "a:b")}, after: []Entry{fail("A", "a:b")},
			wantRemoved: []string{"x"},
			wantChanged: []CallChange{{"A", "a:b", "partial", "error", ChangeNewlyDenied}},
		},
		{
			name: "ok to partial", before: []Entry{res("A", "a:b", "x")}, after: []Entry{res("A", "a:b", "x"), fail("A", "a:b")},
			wantChanged: []CallChange{{"A", "a:b", "ok", "partial", ChangeChanged}},
		},
		{
			name: "error to none", before: []Entry{fail("A", "a:b")},
			wantChanged: []CallChange{{"A", "a:b", "error", "none", ChangeChanged}},
		},
		{
			name: "ok to none not a call change", before: []Entry{res("A", "a:b", "x")},
			wantRemoved: []string{"x"},
		},
		{
			name:   "timestamp/details/type-only changes ignored",
			before: []Entry{{ServiceName: "S3", MethodName: "s3:L", ResourceType: "bucket", ResourceName: "a"}},
			after:  []Entry{{ServiceName: "S3", MethodName: "s3:L", ResourceType: "other", ResourceName: "a"}},
		},
		{
			name:      "duplicates deduped",
			before:    []Entry{res("S3", "s3:L", "a"), res("S3", "s3:L", "a")},
			after:     []Entry{res("S3", "s3:L", "a"), res("S3", "s3:L", "z"), res("S3", "s3:L", "z")},
			wantAdded: []string{"z"},
		},
		{
			name:      "same name different method is distinct",
			before:    []Entry{res("S3", "s3:L", "a")},
			after:     []Entry{res("S3", "s3:L", "a"), res("S3", "s3:G", "a")},
			wantAdded: []string{"a"},
		},
	}
	names := func(rs []Resource) []string {
		var out []string
		for _, r := range rs {
			out = append(out, r.ResourceName)
		}
		return out
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := Compare(tc.before, tc.after)
			if got := names(r.Added); !reflect.DeepEqual(got, tc.wantAdded) {
				t.Errorf("added = %v, want %v", got, tc.wantAdded)
			}
			if got := names(r.Removed); !reflect.DeepEqual(got, tc.wantRemoved) {
				t.Errorf("removed = %v, want %v", got, tc.wantRemoved)
			}
			var gotCh []CallChange
			gotCh = append(gotCh, r.Changed...)
			if !reflect.DeepEqual(gotCh, tc.wantChanged) {
				t.Errorf("changed = %v, want %v", gotCh, tc.wantChanged)
			}
			wantIdentical := len(tc.wantAdded)+len(tc.wantRemoved)+len(tc.wantChanged) == 0
			if r.Identical() != wantIdentical {
				t.Errorf("Identical = %v, want %v", r.Identical(), wantIdentical)
			}
		})
	}
}

func TestCompareSorted(t *testing.T) {
	after := []Entry{res("S3", "s3:L", "b"), res("EC2", "ec2:D", "z"), res("S3", "s3:L", "a"), res("EC2", "ec2:A", "q")}
	r := Compare(nil, after)
	var got []string
	for _, x := range r.Added {
		got = append(got, x.MethodName+"/"+x.ResourceName)
	}
	want := []string{"ec2:A/q", "ec2:D/z", "s3:L/a", "s3:L/b"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("order = %v, want %v", got, want)
	}
}

func TestRenderText(t *testing.T) {
	rep := Compare(
		[]Entry{res("S3", "s3:L", "old"), fail("IAM", "iam:L")},
		[]Entry{res("S3", "s3:L", "new"), res("IAM", "iam:L", "u")},
	)
	var buf bytes.Buffer
	if err := RenderText(&buf, rep, "b.json", "a.json"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"Comparing b.json -> a.json\n",
		"Added resources (2):\n",
		"  + s3:L  new (t)\n",
		"Removed resources (1):\n",
		"  - s3:L  old (t)\n",
		"Changed calls (1):\n",
		"  ~ iam:L  error -> ok (newly allowed)\n",
		"Summary: 2 added, 1 removed, 1 changed\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "No differences.") {
		t.Errorf("unexpected 'No differences.':\n%s", out)
	}
}

func TestRenderTextIdentical(t *testing.T) {
	var buf bytes.Buffer
	if err := RenderText(&buf, Compare(nil, nil), "a", "b"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "Comparing a -> b\n") || !strings.Contains(out, "No differences.\n") || strings.Contains(out, "Summary:") {
		t.Errorf("unexpected output:\n%s", out)
	}
}

func TestRenderJSON(t *testing.T) {
	t.Run("identical arrays never null", func(t *testing.T) {
		var buf bytes.Buffer
		if err := RenderJSON(&buf, Report{}); err != nil { // zero-value Report has nil slices
			t.Fatal(err)
		}
		want := `{"identical":true,"summary":{"added":0,"removed":0,"changed":0},"added":[],"removed":[],"changedCalls":[]}` + "\n"
		if buf.String() != want {
			t.Errorf("got %q, want %q", buf.String(), want)
		}
	})
	t.Run("with differences", func(t *testing.T) {
		rep := Compare([]Entry{fail("IAM", "iam:L")}, []Entry{res("IAM", "iam:L", "u")})
		var buf bytes.Buffer
		if err := RenderJSON(&buf, rep); err != nil {
			t.Fatal(err)
		}
		if !strings.HasSuffix(buf.String(), "\n") || strings.Count(buf.String(), "\n") != 1 {
			t.Errorf("want compact output with single trailing newline: %q", buf.String())
		}
		var got struct {
			Identical bool                `json:"identical"`
			Summary   map[string]int      `json:"summary"`
			Added     []map[string]string `json:"added"`
			Removed   []map[string]string `json:"removed"`
			Changed   []map[string]string `json:"changedCalls"`
		}
		if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got.Identical || got.Summary["added"] != 1 || got.Summary["removed"] != 0 || got.Summary["changed"] != 1 {
			t.Errorf("unexpected: %+v", got)
		}
		if got.Removed == nil || len(got.Removed) != 0 {
			t.Errorf("removed should be empty array, got %v", got.Removed)
		}
		if got.Added[0]["resourceName"] != "u" || got.Changed[0]["change"] != "newly_allowed" {
			t.Errorf("unexpected: %+v", got)
		}
	})
}

// TestFormatterOutputAccepted proves real formatter output (both shapes) parses.
func TestFormatterOutputAccepted(t *testing.T) {
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	results := []types.ScanResult{
		{ServiceName: "S3", MethodName: "s3:ListBuckets", ResourceType: "bucket", ResourceName: "b1",
			Details: map[string]interface{}{"region": "us-west-2"}, Timestamp: ts},
		{ServiceName: "IAM", MethodName: "iam:ListUsers", Error: errors.New("AccessDenied"), Timestamp: ts},
		{ServiceName: "EC2", MethodName: "ec2:DescribeInstances", ResourceType: "instance", ResourceName: "i-1", Timestamp: ts},
	}
	f := formatters.NewJSONFormatter()

	bare, err := f.Format(results)
	if err != nil {
		t.Fatal(err)
	}
	env, err := f.FormatWithSummary(results, types.GenerateSummary(results, ts))
	if err != nil {
		t.Fatal(err)
	}
	for name, out := range map[string]string{"Format": bare, "FormatWithSummary": env} {
		got, err := Parse([]byte(out), name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(got) != 3 || got[1].Error != "AccessDenied" || got[0].ResourceName != "b1" {
			t.Errorf("%s: unexpected entries %+v", name, got)
		}
	}

	// Empty scans from the real formatters are valid too.
	empty, _ := f.Format(nil)
	if got, err := Parse([]byte(empty), "empty"); err != nil || len(got) != 0 {
		t.Errorf("empty bare: %v %v", got, err)
	}
	emptyEnv, _ := f.FormatWithSummary(nil, types.GenerateSummary(nil, ts))
	if got, err := Parse([]byte(emptyEnv), "emptyEnv"); err != nil || len(got) != 0 {
		t.Errorf("empty envelope: %v %v", got, err)
	}
}
