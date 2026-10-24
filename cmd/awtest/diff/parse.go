// Package diff compares two awtest JSON scan outputs offline.
package diff

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// Entry is one result object from an awtest JSON scan. Timestamp and details
// are deliberately not retained: they never take part in a comparison.
type Entry struct {
	ServiceName  string
	MethodName   string
	ResourceType string
	ResourceName string
	Error        string
}

// Load reads and parses the awtest JSON output file at path.
func Load(path string) ([]Entry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w", path, err)
	}
	return Parse(data, path)
}

// ParseReader reads all of r and parses it. name is used in error messages.
func ParseReader(r io.Reader, name string) ([]Entry, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w", name, err)
	}
	return Parse(data, name)
}

// Parse accepts either a bare array of results or an envelope object with a
// "results" array, and returns the entries. name is used in error messages.
func Parse(data []byte, name string) ([]Entry, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	var top json.RawMessage
	if err := dec.Decode(&top); err != nil {
		return nil, fmt.Errorf("%s is not valid JSON (%v); the file must be awtest output produced with -format json, not yaml, csv or text", name, err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("%s has unexpected data after the JSON value; the file must contain a single awtest -format json document", name)
	}

	var items []json.RawMessage
	switch firstByte(top) {
	case '[':
		if err := json.Unmarshal(top, &items); err != nil {
			return nil, fmt.Errorf("%s: cannot read results array: %v", name, err)
		}
	case '{':
		var env map[string]json.RawMessage
		if err := json.Unmarshal(top, &env); err != nil {
			return nil, fmt.Errorf("%s: cannot read JSON object: %v", name, err)
		}
		raw, ok := env["results"]
		if !ok {
			return nil, fmt.Errorf("%s is not awtest JSON output: top-level object has no \"results\" array", name)
		}
		if firstByte(raw) != '[' {
			return nil, fmt.Errorf("%s is not awtest JSON output: \"results\" must be an array", name)
		}
		if err := json.Unmarshal(raw, &items); err != nil {
			return nil, fmt.Errorf("%s: cannot read \"results\" array: %v", name, err)
		}
	default:
		return nil, fmt.Errorf("%s is not awtest JSON output: top-level value must be an array or an object with a \"results\" array", name)
	}

	entries := make([]Entry, 0, len(items))
	for i, raw := range items {
		e, err := parseEntry(raw)
		if err != nil {
			return nil, fmt.Errorf("%s: result #%d: %v", name, i+1, err)
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func firstByte(b []byte) byte {
	b = bytes.TrimSpace(b)
	if len(b) == 0 {
		return 0
	}
	return b[0]
}

func parseEntry(raw json.RawMessage) (Entry, error) {
	if firstByte(raw) != '{' {
		return Entry{}, fmt.Errorf("must be a JSON object")
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return Entry{}, err
	}
	var e Entry
	fields := []struct {
		key string
		dst *string
	}{
		{"serviceName", &e.ServiceName},
		{"methodName", &e.MethodName},
		{"resourceType", &e.ResourceType},
		{"resourceName", &e.ResourceName},
		{"error", &e.Error},
	}
	for _, f := range fields {
		v, ok := m[f.key]
		if !ok || bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
			continue
		}
		if err := json.Unmarshal(v, f.dst); err != nil {
			return Entry{}, fmt.Errorf("%q must be a string", f.key)
		}
	}
	if e.MethodName == "" {
		return Entry{}, fmt.Errorf("missing non-empty string \"methodName\"")
	}
	return e, nil
}
