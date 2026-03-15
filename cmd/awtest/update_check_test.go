package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsNewer(t *testing.T) {
	tests := []struct {
		name    string
		latest  string
		current string
		want    bool
	}{
		{"patch bump", "1.0.1", "1.0.0", true},
		{"minor bump", "1.1.0", "1.0.0", true},
		{"major bump", "2.0.0", "1.0.0", true},
		{"equal", "1.0.0", "1.0.0", false},
		{"current newer patch", "1.0.0", "1.0.1", false},
		{"current newer minor", "1.0.0", "1.1.0", false},
		{"current newer major", "1.0.0", "2.0.0", false},
		{"v prefix latest", "v1.1.0", "1.0.0", true},
		{"v prefix current", "1.1.0", "v1.0.0", true},
		{"v prefix both", "v1.1.0", "v1.0.0", true},
		{"multi-digit", "1.10.0", "1.9.0", true},
		{"malformed latest", "abc", "1.0.0", false},
		{"malformed current", "1.0.0", "xyz", false},
		{"empty latest", "", "1.0.0", false},
		{"empty current", "1.0.0", "", false},
		{"two segments", "1.1", "1.0.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isNewer(tt.latest, tt.current)
			if got != tt.want {
				t.Errorf("isNewer(%q, %q) = %v, want %v", tt.latest, tt.current, got, tt.want)
			}
		})
	}
}

func TestIsHomebrewInstall(t *testing.T) {
	// We can only test the function exists and returns a bool.
	// Actual path detection depends on runtime environment.
	_ = isHomebrewInstall()
}

func TestCheckForUpdate_UpdateAvailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := struct {
			TagName string `json:"tag_name"`
		}{TagName: "v2.0.0"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	origURL := releaseURL
	releaseURL = server.URL
	defer func() { releaseURL = origURL }()

	msg, warn := checkForUpdate("1.0.0")
	if warn != "" {
		t.Fatalf("unexpected warning: %s", warn)
	}
	if msg == "" {
		t.Fatal("expected non-empty message for update available")
	}
	if !containsStr(msg, "2.0.0") {
		t.Errorf("expected message to contain '2.0.0', got: %s", msg)
	}
}

func TestCheckForUpdate_UpToDate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := struct {
			TagName string `json:"tag_name"`
		}{TagName: "v1.0.0"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	origURL := releaseURL
	releaseURL = server.URL
	defer func() { releaseURL = origURL }()

	msg, warn := checkForUpdate("1.0.0")
	if warn != "" {
		t.Fatalf("unexpected warning: %s", warn)
	}
	if !containsStr(msg, "up to date") {
		t.Errorf("expected 'up to date' message, got: %s", msg)
	}
}

func TestCheckForUpdate_NetworkError(t *testing.T) {
	origURL := releaseURL
	releaseURL = "http://127.0.0.1:1" // connection refused
	defer func() { releaseURL = origURL }()

	msg, warn := checkForUpdate("1.0.0")
	if msg != "" {
		t.Errorf("expected empty message on error, got: %s", msg)
	}
	if !containsStr(warn, "Unable to check for updates") {
		t.Errorf("expected warning message, got: %s", warn)
	}
}

func TestCheckForUpdate_DevBuild(t *testing.T) {
	msg, warn := checkForUpdate("dev")
	if warn != "" {
		t.Fatalf("unexpected warning: %s", warn)
	}
	if !containsStr(msg, "Development build") {
		t.Errorf("expected dev build message, got: %s", msg)
	}
}

func TestCheckForUpdate_MalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, "not json")
	}))
	defer server.Close()

	origURL := releaseURL
	releaseURL = server.URL
	defer func() { releaseURL = origURL }()

	msg, warn := checkForUpdate("1.0.0")
	if msg != "" {
		t.Errorf("expected empty message on error, got: %s", msg)
	}
	if !containsStr(warn, "Unable to check for updates") {
		t.Errorf("expected warning message, got: %s", warn)
	}
}

func TestCheckForUpdate_Non200Status(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	origURL := releaseURL
	releaseURL = server.URL
	defer func() { releaseURL = origURL }()

	msg, warn := checkForUpdate("1.0.0")
	if msg != "" {
		t.Errorf("expected empty message on error, got: %s", msg)
	}
	if !containsStr(warn, "Unable to check for updates") {
		t.Errorf("expected warning message, got: %s", warn)
	}
}

func TestCheckForUpdate_UserAgent(t *testing.T) {
	var gotUA string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		resp := struct {
			TagName string `json:"tag_name"`
		}{TagName: "v1.0.0"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	origURL := releaseURL
	releaseURL = server.URL
	defer func() { releaseURL = origURL }()

	checkForUpdate("1.0.0")
	if !containsStr(gotUA, "awtest") {
		t.Errorf("expected User-Agent to contain 'awtest', got: %s", gotUA)
	}
}

// containsStr checks if s contains substr (avoids importing strings in test).
func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
