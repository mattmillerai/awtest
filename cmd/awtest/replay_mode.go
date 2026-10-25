package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/mattmillerai/awtest/cmd/awtest/replay"
)

// maxUnmatchedShown is the number of unmatched call names listed in the
// replay summary line.
const maxUnmatchedShown = 20

// startReplay loads the fixtures named by the -replay value and starts the
// loopback server. It also drops environment settings that would otherwise
// leak into the SDK session (custom CA bundle, client-side monitoring).
func startReplay(spec string) (*replay.Server, error) {
	fsys, err := replay.OpenFS(spec)
	if err != nil {
		return nil, err
	}
	fx, err := replay.Load(fsys)
	if err != nil {
		return nil, err
	}
	for _, k := range []string{"AWS_CA_BUNDLE", "AWS_CSM_ENABLED"} {
		os.Unsetenv(k)
	}
	return replay.Start(fx)
}

// replayCredsSupplied reports whether the user supplied credentials or a
// profile that replay mode will ignore.
func replayCredsSupplied(akid, secret, token string, getenv func(string) string) bool {
	if akid != "" || secret != "" || token != "" {
		return true
	}
	for _, k := range []string{"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_SESSION_TOKEN", "AWS_PROFILE"} {
		if getenv(k) != "" {
			return true
		}
	}
	return false
}

// unmatchedSummary renders the one-line summary of calls that had no fixture,
// or "" when there were none. names must be sorted and de-duplicated.
func unmatchedSummary(names []string) string {
	if len(names) == 0 {
		return ""
	}
	shown := names
	more := ""
	if len(shown) > maxUnmatchedShown {
		more = fmt.Sprintf(", and %d more", len(shown)-maxUnmatchedShown)
		shown = shown[:maxUnmatchedShown]
	}
	return fmt.Sprintf("Replay: %d request(s) had no fixture (answered with an AccessDenied-style error): %s%s",
		len(names), strings.Join(shown, ", "), more)
}
