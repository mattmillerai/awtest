package diff

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

var changeLabels = map[string]string{
	ChangeNewlyAllowed: "newly allowed",
	ChangeNewlyDenied:  "newly denied",
	ChangeChanged:      "changed",
}

// RenderText writes a human-readable report to w.
func RenderText(w io.Writer, r Report, beforeName, afterName string) error {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Comparing %s -> %s\n", beforeName, afterName)
	if r.Identical() {
		sb.WriteString("\nNo differences.\n")
		_, err := io.WriteString(w, sb.String())
		return err
	}
	writeResources := func(title, marker string, rs []Resource) {
		if len(rs) == 0 {
			return
		}
		fmt.Fprintf(&sb, "\n%s (%d):\n", title, len(rs))
		for _, x := range rs {
			fmt.Fprintf(&sb, "  %s %s  %s (%s)\n", marker, x.MethodName, x.ResourceName, x.ResourceType)
		}
	}
	writeResources("Added resources", "+", r.Added)
	writeResources("Removed resources", "-", r.Removed)
	if len(r.Changed) > 0 {
		fmt.Fprintf(&sb, "\nChanged calls (%d):\n", len(r.Changed))
		for _, c := range r.Changed {
			fmt.Fprintf(&sb, "  ~ %s  %s -> %s (%s)\n", c.MethodName, c.Before, c.After, changeLabels[c.Change])
		}
	}
	fmt.Fprintf(&sb, "\nSummary: %d added, %d removed, %d changed\n", len(r.Added), len(r.Removed), len(r.Changed))
	_, err := io.WriteString(w, sb.String())
	return err
}

type jsonSummary struct {
	Added   int `json:"added"`
	Removed int `json:"removed"`
	Changed int `json:"changed"`
}

type jsonReport struct {
	Identical    bool         `json:"identical"`
	Summary      jsonSummary  `json:"summary"`
	Added        []Resource   `json:"added"`
	Removed      []Resource   `json:"removed"`
	ChangedCalls []CallChange `json:"changedCalls"`
}

// RenderJSON writes the report as compact JSON followed by a newline.
// Arrays are always emitted as arrays, never null.
func RenderJSON(w io.Writer, r Report) error {
	jr := jsonReport{
		Identical:    r.Identical(),
		Summary:      jsonSummary{len(r.Added), len(r.Removed), len(r.Changed)},
		Added:        r.Added,
		Removed:      r.Removed,
		ChangedCalls: r.Changed,
	}
	if jr.Added == nil {
		jr.Added = []Resource{}
	}
	if jr.Removed == nil {
		jr.Removed = []Resource{}
	}
	if jr.ChangedCalls == nil {
		jr.ChangedCalls = []CallChange{}
	}
	data, err := json.Marshal(jr)
	if err != nil {
		return fmt.Errorf("json formatting failed: %w", err)
	}
	_, err = w.Write(append(data, '\n'))
	return err
}
