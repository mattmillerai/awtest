package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// releaseURL is the GitHub Releases API endpoint. Package-level var so tests can override.
var releaseURL = "https://api.github.com/repos/mattmillerai/awtest/releases/latest"

// releasesPageURL is the human-facing download page shown in upgrade hints.
const releasesPageURL = "https://github.com/mattmillerai/awtest/releases/latest"

const updateWarning = "Warning: Unable to check for updates"

// checkForUpdate queries the GitHub Releases API and compares the latest release
// against currentVersion. Returns (stdout message, stderr warning). Exactly one
// will be non-empty: a success message for stdout, or a warning for stderr.
func checkForUpdate(currentVersion string) (message string, warning string) {
	if currentVersion == "dev" {
		return "Development build — skipping update check", ""
	}

	// Redirects are followed explicitly (the repo moved owners, so the old API
	// URL may answer with a 301), capped at 5 hops.
	client := &http.Client{
		Timeout: 5 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("stopped after 5 redirects")
			}
			return nil
		},
	}

	req, err := http.NewRequest("GET", releaseURL, nil)
	if err != nil {
		return "", updateWarning
	}
	req.Header.Set("User-Agent", "awtest/"+currentVersion)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return "", updateWarning
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", updateWarning
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", updateWarning
	}

	if release.TagName == "" {
		return "", updateWarning
	}

	latestClean := strings.TrimPrefix(release.TagName, "v")
	currentClean := strings.TrimPrefix(currentVersion, "v")

	if !isNewer(latestClean, currentClean) {
		return fmt.Sprintf("awtest v%s is up to date", currentClean), ""
	}

	// Update available — build upgrade instructions
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("awtest v%s is available (you have v%s)\n", latestClean, currentClean))

	if isHomebrewInstall() {
		sb.WriteString("\nUpgrade with:\n")
		sb.WriteString("  brew upgrade awtest\n")
	} else {
		sb.WriteString("\nDownload the latest release:\n")
		sb.WriteString("  " + releasesPageURL + "\n")
	}

	return sb.String(), ""
}

// isNewer returns true if latest is a newer semver than current.
// Both must be valid X.Y.Z format (with optional "v" prefix). Returns false on parse errors.
func isNewer(latest, current string) bool {
	latest = strings.TrimPrefix(latest, "v")
	current = strings.TrimPrefix(current, "v")

	lParts := strings.Split(latest, ".")
	cParts := strings.Split(current, ".")

	if len(lParts) != 3 || len(cParts) != 3 {
		return false
	}

	for i := 0; i < 3; i++ {
		l, errL := strconv.Atoi(lParts[i])
		c, errC := strconv.Atoi(cParts[i])
		if errL != nil || errC != nil {
			return false
		}
		if l > c {
			return true
		}
		if l < c {
			return false
		}
	}
	return false
}

// isHomebrewInstall detects if the running binary was installed via Homebrew
// by checking if the executable path contains Homebrew-specific directories.
func isHomebrewInstall() bool {
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	return strings.Contains(exe, "/Cellar/") || strings.Contains(exe, "/homebrew/")
}
