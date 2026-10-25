package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mattmillerai/awtest/cmd/awtest/replay"
)

type replayResult struct {
	ServiceName  string                 `json:"serviceName"`
	MethodName   string                 `json:"methodName"`
	ResourceType string                 `json:"resourceType"`
	ResourceName string                 `json:"resourceName"`
	Details      map[string]interface{} `json:"details"`
	Error        string                 `json:"error"`
}

// runReplayBin runs bin with a clean AWS environment plus extraEnv.
func runReplayBin(t *testing.T, bin string, extraEnv []string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "AWS_") {
			cmd.Env = append(cmd.Env, kv)
		}
	}
	cmd.Env = append(cmd.Env, extraEnv...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("running %v: %v", args, err)
		}
		code = ee.ExitCode()
	}
	return out.String(), errb.String(), code
}

// parseScanJSON accepts a bare results array (-quiet) or the metadata envelope.
func parseScanJSON(t *testing.T, stdout string) []replayResult {
	t.Helper()
	var arr []replayResult
	if err := json.Unmarshal([]byte(stdout), &arr); err == nil {
		return arr
	}
	var env struct {
		Results []replayResult `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("stdout is not valid scan JSON: %v\n%.500s", err, stdout)
	}
	return env.Results
}

func findResult(rs []replayResult, method, name string) *replayResult {
	for i := range rs {
		if rs[i].MethodName == method && rs[i].ResourceName == name {
			return &rs[i]
		}
	}
	return nil
}

func detailStrings(r *replayResult, key string) string {
	return fmt.Sprint(r.Details[key])
}

func TestReplayFlagBinary(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	bin := buildAwtestBinary(t)
	core := []string{"-replay", "builtin:sample", "-format", "json", "-services", "sts,s3,iam,ec2"}

	t.Run("core services", func(t *testing.T) {
		out, errOut, code := runReplayBin(t, bin, nil, append(core, "-quiet")...)
		if code != 0 {
			t.Fatalf("exit = %d, stderr=%s", code, errOut)
		}
		rs := parseScanJSON(t, out)
		for _, c := range []struct{ method, name string }{
			{"sts:GetCallerIdentity", "AIDAEXAMPLEDEMO0001"},
			{"s3:ListBuckets", "example-public-assets"},
			{"s3:ListBuckets", "example-logs"},
			{"s3:GetBucketPolicy", "example-public-assets"},
			{"iam:ListUsers", "alice"},
			{"iam:ListRoles", "example-cross-account-role"},
			{"ec2:DescribeInstances", "i-0example00000001"},
			{"ec2:DescribeInstances", "i-0example00000002"},
		} {
			r := findResult(rs, c.method, c.name)
			if r == nil {
				t.Errorf("missing result %s %s", c.method, c.name)
			} else if r.Error != "" {
				t.Errorf("%s %s has error %q", c.method, c.name, r.Error)
			}
		}
		if r := findResult(rs, "s3:GetBucketPolicy", "example-public-assets"); r != nil && !strings.Contains(detailStrings(r, "findings"), "wildcard principal") {
			t.Errorf("public bucket policy has no wildcard finding: %v", r.Details)
		}
		if r := findResult(rs, "iam:ListRoles", "example-cross-account-role"); r != nil && !strings.Contains(detailStrings(r, "trust_findings"), "cross-account") {
			t.Errorf("cross-account role has no cross-account finding: %v", r.Details)
		}
		if r := findResult(rs, "iam:ListAccessKeys", "AKIAEXAMPLEFAKE0001"); r == nil {
			t.Error("per-user access key fixture not served")
		}
	})

	t.Run("full scan", func(t *testing.T) {
		// Without -quiet, inline results also go to stdout, so write JSON to a file.
		outFile := filepath.Join(t.TempDir(), "full.json")
		_, errOut, code := runReplayBin(t, bin, nil, "-replay", "builtin:sample", "-format", "json", "-o", outFile, "-speed", "fast", "-timeout", "4m")
		if code != 0 {
			t.Fatalf("exit = %d, stderr=%s", code, errOut)
		}
		out, err := os.ReadFile(outFile)
		if err != nil {
			t.Fatal(err)
		}
		if rs := parseScanJSON(t, string(out)); len(rs) == 0 {
			t.Error("no results")
		}
		if !strings.Contains(errOut, "Replay mode: serving canned responses from builtin:sample at http://127.0.0.1:") {
			t.Errorf("notice missing:\n%s", errOut)
		}
		if !strings.Contains(errOut, "Replay: ") || !strings.Contains(errOut, "request(s) had no fixture (answered with an AccessDenied-style error): ") {
			t.Errorf("no-fixture summary missing:\n%s", errOut)
		}
		if strings.Contains(errOut, "Using custom endpoint") {
			t.Errorf("unexpected endpoint notice:\n%s", errOut)
		}
	})

	t.Run("all regions falls back to static list", func(t *testing.T) {
		_, errOut, code := runReplayBin(t, bin, nil, "-replay", "builtin:sample", "-all-regions", "-format", "json", "-services", "ec2", "-speed", "fast")
		if code != 0 {
			t.Fatalf("exit = %d, stderr=%s", code, errOut)
		}
		if !strings.Contains(errOut, "All-regions mode: scanning") || !strings.Contains(errOut, "built-in list") {
			t.Errorf("expected static fallback:\n%s", errOut)
		}
	})

	t.Run("notice and quiet", func(t *testing.T) {
		_, errOut, _ := runReplayBin(t, bin, nil, core...)
		for _, w := range []string{"Replay mode", "results do not come from real AWS", "credentials are fake"} {
			if !strings.Contains(errOut, w) {
				t.Errorf("stderr missing %q:\n%s", w, errOut)
			}
		}
		_, errOut, _ = runReplayBin(t, bin, nil, append(core, "-quiet")...)
		if strings.Contains(errOut, "Replay") {
			t.Errorf("-quiet printed replay output:\n%s", errOut)
		}
	})

	t.Run("debug shows notice and fake key", func(t *testing.T) {
		out, _, code := runReplayBin(t, bin, nil, "-replay", "builtin:sample", "-services", "sts", "-debug", "-quiet")
		if code != 0 {
			t.Fatalf("exit = %d", code)
		}
		if !strings.Contains(out, "Replay mode: serving canned responses") || !strings.Contains(out, replay.FakeAccessKeyID) {
			t.Errorf("debug output:\n%s", out)
		}
	})

	t.Run("supplied credentials are ignored", func(t *testing.T) {
		const realKey, realSecret = "AKIAIOSFODNN7REALKEY", "realSecretKeyThatMustNeverAppear0123456789"
		env := []string{"AWS_ACCESS_KEY_ID=AKIAENVREALKEY0000AA", "AWS_SECRET_ACCESS_KEY=envSecretThatMustNeverAppear", "AWS_PROFILE=does-not-exist", "AWS_CA_BUNDLE=/nonexistent/ca.pem"}
		args := append(append([]string{}, core...), "-aki", realKey, "-sak", realSecret, "-debug")
		out, errOut, code := runReplayBin(t, bin, env, args...)
		if code != 0 {
			t.Fatalf("exit = %d, stderr=%s", code, errOut)
		}
		if !strings.Contains(errOut, "ignored in replay mode") {
			t.Errorf("ignored note missing:\n%s", errOut)
		}
		for _, s := range []string{realKey, realSecret, "AKIAENVREALKEY0000AA", "envSecretThatMustNeverAppear", "does-not-exist"} {
			if strings.Contains(out, s) || strings.Contains(errOut, s) {
				t.Errorf("%q appeared in output", s)
			}
		}
		_, errOut, _ = runReplayBin(t, bin, env, append(args, "-quiet")...)
		if strings.Contains(errOut, "ignored in replay mode") {
			t.Errorf("-quiet printed the ignored note")
		}
	})

	t.Run("AWS_ENDPOINT_URL is ignored", func(t *testing.T) {
		_, errOut, code := runReplayBin(t, bin, []string{"AWS_ENDPOINT_URL=notaurl"}, append(core, "-quiet")...)
		if code != 0 {
			t.Errorf("exit = %d, stderr=%s", code, errOut)
		}
	})

	t.Run("usage errors exit 2", func(t *testing.T) {
		_, errOut, code := runReplayBin(t, bin, nil, "-replay", "builtin:sample", "-endpoint-url", "http://localhost:4566")
		if code != 2 || !strings.Contains(errOut, "-endpoint-url") {
			t.Errorf("endpoint-url: exit=%d stderr=%s", code, errOut)
		}
		_, errOut, code = runReplayBin(t, bin, nil, "-replay", filepath.Join(t.TempDir(), "missing"))
		if code != 2 || !strings.Contains(errOut, "-replay") {
			t.Errorf("missing dir: exit=%d stderr=%s", code, errOut)
		}
		dup := t.TempDir()
		writeFixture(t, dup, "sts/GetCallerIdentity.xml", "<x/>")
		writeFixture(t, dup, "sts/GetCallerIdentity.json", "{}")
		_, errOut, code = runReplayBin(t, bin, nil, "-replay", dup)
		if code != 2 || !strings.Contains(errOut, "duplicate") {
			t.Errorf("duplicate: exit=%d stderr=%s", code, errOut)
		}
	})

	t.Run("help mentions flag", func(t *testing.T) {
		_, errOut, _ := runReplayBin(t, bin, nil, "-h")
		if strings.Count(errOut, "-replay") < 2 {
			t.Errorf("-h should mention -replay in the header and the flag list:\n%s", errOut)
		}
	})

	t.Run("proxy environment is not used", func(t *testing.T) {
		ln, err := net.Listen("tcp4", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer ln.Close()
		var conns int64
		go func() {
			for {
				c, err := ln.Accept()
				if err != nil {
					return
				}
				atomic.AddInt64(&conns, 1)
				c.Close()
			}
		}()
		p := "http://" + ln.Addr().String()
		env := []string{"HTTP_PROXY=" + p, "HTTPS_PROXY=" + p, "ALL_PROXY=" + p, "http_proxy=" + p, "https_proxy=" + p, "all_proxy=" + p, "NO_PROXY=", "no_proxy="}
		out, errOut, code := runReplayBin(t, bin, env, append(core, "-quiet")...)
		if code != 0 {
			t.Fatalf("exit = %d, stderr=%s", code, errOut)
		}
		if findResult(parseScanJSON(t, out), "s3:ListBuckets", "example-logs") == nil {
			t.Error("scan did not reach the replay server")
		}
		if n := atomic.LoadInt64(&conns); n != 0 {
			t.Errorf("proxy received %d connection(s)", n)
		}
	})

	t.Run("diff", func(t *testing.T) {
		dir := t.TempDir()
		run := func(name string, replayArg string) string {
			out, errOut, code := runReplayBin(t, bin, nil, "-replay", replayArg, "-format", "json", "-services", "sts,s3,iam,ec2", "-quiet")
			if code != 0 {
				t.Fatalf("%s: exit = %d, stderr=%s", name, code, errOut)
			}
			p := filepath.Join(dir, name)
			if err := os.WriteFile(p, []byte(out), 0o644); err != nil {
				t.Fatal(err)
			}
			return p
		}
		a, b := run("a.json", "builtin:sample"), run("b.json", "builtin:sample")
		out, errOut, code := runReplayBin(t, bin, nil, "diff", a, b)
		if code != 0 {
			t.Errorf("identical fixtures: exit = %d\n%s%s", code, out, errOut)
		}

		mod := copySample(t)
		writeFixture(t, mod, "s3/ListBuckets.xml", `<ListAllMyBucketsResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><Owner><ID>x</ID></Owner><Buckets><Bucket><Name>example-public-assets</Name><CreationDate>2024-03-10T12:00:00.000Z</CreationDate></Bucket></Buckets></ListAllMyBucketsResult>`)
		c := run("c.json", mod)
		out, errOut, code = runReplayBin(t, bin, nil, "diff", a, c)
		if code != 1 || !strings.Contains(out, "example-logs") {
			t.Errorf("removed bucket: exit = %d\n%s%s", code, out, errOut)
		}
	})

	t.Run("custom directory with status suffix", func(t *testing.T) {
		dir := t.TempDir()
		writeFixture(t, dir, "sts/GetCallerIdentity.xml", `<GetCallerIdentityResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/"><GetCallerIdentityResult><Arn>arn:aws:iam::123456789012:user/lab-user</Arn><UserId>AIDALABUSER000000001</UserId><Account>123456789012</Account></GetCallerIdentityResult></GetCallerIdentityResponse>`)
		writeFixture(t, dir, "iam/ListUsers.403.xml", `<ErrorResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/"><Error><Type>Sender</Type><Code>AccessDenied</Code><Message>lab says no</Message></Error><RequestId>1</RequestId></ErrorResponse>`)
		out, errOut, code := runReplayBin(t, bin, nil, "-replay", dir, "-format", "json", "-services", "sts,iam", "-quiet")
		if code != 0 {
			t.Fatalf("exit = %d, stderr=%s", code, errOut)
		}
		rs := parseScanJSON(t, out)
		if findResult(rs, "sts:GetCallerIdentity", "AIDALABUSER000000001") == nil {
			t.Errorf("custom sts fixture not served: %s", out)
		}
		var denied bool
		for _, r := range rs {
			if r.MethodName == "iam:ListUsers" && strings.Contains(r.Error, "AccessDenied") && strings.Contains(r.Error, "lab says no") {
				denied = true
			}
		}
		if !denied {
			t.Errorf("status-suffixed fixture did not produce AccessDenied: %s", out)
		}
		_, errOut, _ = runReplayBin(t, bin, nil, "-replay", dir, "-services", "sts,iam", "-format", "json")
		if !strings.Contains(errOut, "Replay mode: serving canned responses from "+dir) {
			t.Errorf("notice should name the directory:\n%s", errOut)
		}
	})
}

func writeFixture(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// copySample writes the embedded sample fixtures to a temp directory.
func copySample(t *testing.T) string {
	t.Helper()
	dst := t.TempDir()
	sample := replay.SampleFS()
	err := fs.WalkDir(sample, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := fs.ReadFile(sample, p)
		if err != nil {
			return err
		}
		writeFixture(t, dst, p, string(data))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return dst
}

func TestUnmatchedSummary(t *testing.T) {
	if got := unmatchedSummary(nil); got != "" {
		t.Errorf("empty = %q", got)
	}
	got := unmatchedSummary([]string{"a:One", "b:Two"})
	want := "Replay: 2 request(s) had no fixture (answered with an AccessDenied-style error): a:One, b:Two"
	if got != want {
		t.Errorf("got %q", got)
	}
	var many []string
	for i := 0; i < 25; i++ {
		many = append(many, fmt.Sprintf("s:A%02d", i))
	}
	got = unmatchedSummary(many)
	if !strings.HasPrefix(got, "Replay: 25 request(s)") || !strings.HasSuffix(got, "s:A19, and 5 more") || strings.Contains(got, "A20") {
		t.Errorf("truncation: %q", got)
	}
}
