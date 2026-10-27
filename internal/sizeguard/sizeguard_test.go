package sizeguard

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

const (
	maxBinaryBytes  = 36 * 1000 * 1000 // 36 MB decimal
	maxStartupRSSKB = 40 * 1000        // 40 MB; Maxrss is in KB on Linux
	rssRuns         = 5
)

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found in any parent directory")
		}
		dir = parent
	}
}

func TestReleaseBinarySizeAndStartupRSS(t *testing.T) {
	if os.Getenv("AWTEST_SIZE_GUARD") != "1" {
		t.Skip("set AWTEST_SIZE_GUARD=1 to run")
	}
	root := findRepoRoot(t)
	bin := filepath.Join(t.TempDir(), "awtest")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute) // cold CI cache: the vendored SDK takes several minutes to compile
	defer cancel()
	build := exec.CommandContext(ctx, "go", "build", "-mod=vendor", "-trimpath", "-ldflags", "-s -w", "-o", bin, "./cmd/awtest")
	build.Dir = root
	build.Env = append(os.Environ(), "GOOS=linux", "GOARCH=amd64", "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}

	fi, err := os.Stat(bin)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("binary size: %.2f MB (limit %.2f MB)", float64(fi.Size())/1e6, float64(maxBinaryBytes)/1e6)
	if fi.Size() > maxBinaryBytes {
		t.Fatalf("binary is %.2f MB, over the %.2f MB limit. An aws-sdk-go-v2 service module was probably bumped "+
			"to a release with a schemas package (aws/aws-sdk-go-v2#3601). See CONTRIBUTING.md, "+
			"\"Pinned aws-sdk-go-v2 service modules\".",
			float64(fi.Size())/1e6, float64(maxBinaryBytes)/1e6)
	}

	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Logf("skipping RSS check on %s/%s", runtime.GOOS, runtime.GOARCH)
		return
	}

	// Take the minimum of several runs to avoid scheduler noise. Maxrss may
	// include the small test process's pre-exec high-water mark.
	home := t.TempDir()
	var env []string
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "AWS_") && !strings.HasPrefix(e, "HOME=") {
			env = append(env, e)
		}
	}
	env = append(env, "HOME="+home)

	minRSS := int64(-1)
	for i := 0; i < rssRuns; i++ {
		rctx, rcancel := context.WithTimeout(context.Background(), 30*time.Second)
		cmd := exec.CommandContext(rctx, bin, "-version")
		cmd.Env = env
		cmd.Dir = home
		err := cmd.Run()
		rcancel()
		if err != nil {
			t.Fatalf("run %d of %s -version: %v", i+1, bin, err)
		}
		rss := cmd.ProcessState.SysUsage().(*syscall.Rusage).Maxrss
		if minRSS < 0 || rss < minRSS {
			minRSS = rss
		}
	}
	t.Logf("startup max RSS (min of %d runs): %d KB (limit %d KB)", rssRuns, minRSS, maxStartupRSSKB)
	if minRSS > maxStartupRSSKB {
		t.Fatalf("startup RSS %d KB exceeds %d KB (aws/aws-sdk-go-v2#3601; see CONTRIBUTING.md, "+
			"\"Pinned aws-sdk-go-v2 service modules\")", minRSS, maxStartupRSSKB)
	}
}
