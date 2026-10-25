package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mattmillerai/awtest/cmd/awtest/diff"
)

const goldenSamplePath = "testdata/golden/replay-sample.json"

// v1TransportSuffix is the aws-sdk-go v1 error noise the v2 migration drops.
var v1TransportSuffix = regexp.MustCompile(`\n\tstatus code: \d+, request id: \S+$`)

// v1StructDump is the second v1 noise form, printed for JSON-protocol services:
// a Go struct dump (with the request id) appended after "<code>: <message>".
var v1StructDump = regexp.MustCompile(`(?s)\n\{\n  RespMetadata: \{.*\n\}$`)

// normalizeGolden drops volatile fields (timestamp), strips the v1 transport
// suffix from errors and re-encodes deterministically. Result order is kept.
func normalizeGolden(data []byte) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var results []map[string]any
	if err := dec.Decode(&results); err != nil {
		return nil, fmt.Errorf("decode results: %w", err)
	}
	for _, r := range results {
		delete(r, "timestamp")
		if s, ok := r["error"].(string); ok {
			s = v1TransportSuffix.ReplaceAllString(s, "")
			r["error"] = v1StructDump.ReplaceAllString(s, "")
		}
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(results); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// compareGolden is stricter than `awtest diff`: besides the resource-level
// comparison it requires identical details and result order.
func compareGolden(want, got []byte) (string, bool) {
	var report strings.Builder
	ok := true

	wantEntries, err := diff.Parse(want, "want")
	if err != nil {
		return "parse want: " + err.Error(), false
	}
	gotEntries, err := diff.Parse(got, "got")
	if err != nil {
		return "parse got: " + err.Error(), false
	}
	if rep := diff.Compare(wantEntries, gotEntries); !rep.Identical() {
		ok = false
		if err := diff.RenderText(&report, rep, "want", "got"); err != nil {
			fmt.Fprintf(&report, "render diff: %v\n", err)
		}
	}

	wn, err := normalizeGolden(want)
	if err != nil {
		return "normalize want: " + err.Error(), false
	}
	gn, err := normalizeGolden(got)
	if err != nil {
		return "normalize got: " + err.Error(), false
	}
	if !bytes.Equal(wn, gn) {
		ok = false
		var wa, ga []json.RawMessage
		_ = json.Unmarshal(wn, &wa)
		_ = json.Unmarshal(gn, &ga)
		idx := 0
		for idx < len(wa) && idx < len(ga) && jsonCompact(wa[idx]) == jsonCompact(ga[idx]) {
			idx++
		}
		fmt.Fprintf(&report, "normalized output differs (want %d results, got %d results)\n", len(wa), len(ga))
		fmt.Fprintf(&report, "first differing result at index %d", idx)
		if idx < len(wa) {
			fmt.Fprintf(&report, " (want: %s)", describeResult(wa[idx]))
		}
		if idx < len(ga) {
			fmt.Fprintf(&report, " (got: %s)", describeResult(ga[idx]))
		}
		report.WriteString("\n")
		if idx < len(wa) {
			fmt.Fprintf(&report, "want: %s\n", jsonCompact(wa[idx]))
		} else {
			report.WriteString("want: <none>\n")
		}
		if idx < len(ga) {
			fmt.Fprintf(&report, "got:  %s\n", jsonCompact(ga[idx]))
		} else {
			report.WriteString("got:  <none>\n")
		}
	}
	return report.String(), ok
}

func jsonCompact(raw json.RawMessage) string {
	var b bytes.Buffer
	if err := json.Compact(&b, raw); err != nil {
		return string(raw)
	}
	return b.String()
}

func describeResult(raw json.RawMessage) string {
	var r struct {
		ServiceName string `json:"serviceName"`
		MethodName  string `json:"methodName"`
	}
	_ = json.Unmarshal(raw, &r)
	return r.ServiceName + " " + r.MethodName
}

func runGoldenScan(t *testing.T, bin string) []byte {
	t.Helper()
	out := filepath.Join(t.TempDir(), "out.json")
	stderr, code := runBinaryEnv(t, bin, nil, "-replay", "builtin:sample", "-quiet", "-format", "json", "-output-file", out)
	if code != 0 {
		t.Fatalf("awtest exited %d: %s", code, stderr)
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	norm, err := normalizeGolden(raw)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	return norm
}

func TestGoldenReplaySample(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping golden integration test in short mode")
	}
	bin := buildAwtestBinary(t)

	first := runGoldenScan(t, bin)
	second := runGoldenScan(t, bin)
	if !bytes.Equal(first, second) {
		report, _ := compareGolden(first, second)
		t.Fatalf("replay output is not deterministic across runs:\n%s", report)
	}

	if os.Getenv("AWTEST_UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(goldenSamplePath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenSamplePath, first, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("updated golden file %s", goldenSamplePath)
		return
	}

	want, err := os.ReadFile(goldenSamplePath)
	if err != nil {
		t.Fatalf("read golden (regenerate with AWTEST_UPDATE_GOLDEN=1): %v", err)
	}
	if report, ok := compareGolden(want, first); !ok {
		t.Fatalf("output differs from %s:\n%s", goldenSamplePath, report)
	}
}

func TestGoldenHarnessSelfCheck(t *testing.T) {
	mk := func(ts, errStr string) string {
		s := `{"serviceName":"Amplify","methodName":"amplify:ListApps","resourceType":"app","resourceName":"a1","details":{"k":"v"}`
		if errStr != "" {
			s += `,"error":` + errStr
		}
		if ts != "" {
			s += `,"timestamp":"` + ts + `"`
		}
		return s + `}`
	}
	second := `{"serviceName":"S3","methodName":"s3:ListBuckets","resourceType":"bucket","resourceName":"b1","details":{},"timestamp":"t"}`
	arr := func(items ...string) []byte { return []byte("[" + strings.Join(items, ",") + "]") }

	v1Err := `"AccessDeniedException: denied\n\tstatus code: 403, request id: replay-00000004"`
	v2Err := `"AccessDeniedException: denied"`

	n1, err := normalizeGolden(arr(mk("2026-01-01T00:00:00Z", v1Err)))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(n1, []byte("timestamp")) || bytes.Contains(n1, []byte("status code")) || bytes.Contains(n1, []byte("request id")) {
		t.Errorf("normalize left volatile data: %s", n1)
	}
	n2, err := normalizeGolden(arr(mk("", v2Err)))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(n1, n2) {
		t.Errorf("v1 and v2 style errors normalize differently:\n%s\n%s", n1, n2)
	}
	if !bytes.Contains(n2, []byte("AccessDeniedException: denied")) || !bytes.HasSuffix(n2, []byte("\n")) {
		t.Errorf("unexpected normalized output: %q", n2)
	}

	// JSON-protocol struct dump, with and without the Type line.
	dumpTyped := `"AccessDeniedException: denied\n{\n  RespMetadata: {\n    StatusCode: 403,\n    RequestID: \"replay-00000137\"\n  },\n  Message_: \"denied\",\n  Type: \"AccessDeniedException\"\n}"`
	dumpPlain := `"AccessDeniedException: denied\n{\n  RespMetadata: {\n    StatusCode: 403,\n    RequestID: \"replay-00000137\"\n  },\n  Message_: \"denied\"\n}"`
	for name, e := range map[string]string{"struct dump with Type": dumpTyped, "struct dump without Type": dumpPlain} {
		nd, err := normalizeGolden(arr(mk("t", e)))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(nd, n2) {
			t.Errorf("%s normalizes differently from plain form:\n%s\n%s", name, nd, n2)
		}
	}

	base := arr(mk("t1", v1Err), second)
	okCases := map[string][]byte{
		"identical":         base,
		"timestamps":        arr(mk("other", v1Err), second),
		"v1 suffix vs none": arr(mk("t1", v2Err), second),
	}
	for name, got := range okCases {
		if rep, ok := compareGolden(base, got); !ok {
			t.Errorf("%s: expected ok, got report: %s", name, rep)
		}
	}

	badCases := map[string][]byte{
		"changed resourceName": arr(strings.Replace(mk("t1", v1Err), `"a1"`, `"a2"`, 1), second),
		"changed details":      arr(strings.Replace(mk("t1", v1Err), `"v"`, `"w"`, 1), second),
		"removed result":       arr(mk("t1", v1Err)),
		"reordered":            arr(second, mk("t1", v1Err)),
		"changed error code":   arr(strings.Replace(mk("t1", v1Err), "AccessDeniedException", "ThrottlingException", 1), second),
	}
	for name, got := range badCases {
		rep, ok := compareGolden(base, got)
		if ok {
			t.Errorf("%s: expected mismatch", name)
			continue
		}
		if rep == "" || !(strings.Contains(rep, "Amplify") || strings.Contains(rep, "amplify:ListApps") || strings.Contains(rep, "S3") || strings.Contains(rep, "s3:ListBuckets")) {
			t.Errorf("%s: report should mention service/method, got: %q", name, rep)
		}
	}
}
