package utils

import (
	"errors"
	"strings"
	"testing"

	"github.com/logrusorgru/aurora"
)

func TestDetermineSeverity_NilError(t *testing.T) {
	severity := DetermineSeverity(nil)
	if severity != "hit" {
		t.Errorf("DetermineSeverity(nil) = %q, want %q", severity, "hit")
	}
}

func TestDetermineSeverity_NonNilError(t *testing.T) {
	severity := DetermineSeverity(errors.New("access denied"))
	if severity != "info" {
		t.Errorf("DetermineSeverity(err) = %q, want %q", severity, "info")
	}
}

func TestColorizeMessage_HitSeverity(t *testing.T) {
	msg := ColorizeMessage("S3", "s3:ListBuckets", "hit", "Access granted")
	expectedGreen := aurora.BrightGreen("hit").String()
	if !strings.Contains(msg, expectedGreen) {
		t.Errorf("expected bright green hit severity %q in message, got %q", expectedGreen, msg)
	}
}

func TestColorizeMessage_HighSeverity(t *testing.T) {
	msg := ColorizeMessage("S3", "s3:ListBuckets", "high", "Alert")
	if !strings.Contains(msg, "high") {
		t.Error("expected 'high' in colorized message")
	}
}

func TestColorizeMessage_InfoSeverity(t *testing.T) {
	msg := ColorizeMessage("S3", "s3:ListBuckets", "info", "Access denied")
	if !strings.Contains(msg, "info") {
		t.Error("expected 'info' in colorized message")
	}
}
