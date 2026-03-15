package main

import (
	"strings"
	"testing"
)

func TestGetFormatter_ValidFormats(t *testing.T) {
	formats := []string{"text", "json", "yaml", "csv", "table"}
	for _, format := range formats {
		t.Run(format, func(t *testing.T) {
			f, err := getFormatter(format)
			if err != nil {
				t.Errorf("getFormatter(%q) returned error: %v", format, err)
			}
			if f == nil {
				t.Errorf("getFormatter(%q) returned nil formatter", format)
			}
		})
	}
}

func TestGetFormatter_CaseInsensitive(t *testing.T) {
	cases := []string{"JSON", "Json", "YAML", "Yaml", "CSV", "Csv", "TABLE", "Table", "TEXT", "Text"}
	for _, format := range cases {
		t.Run(format, func(t *testing.T) {
			f, err := getFormatter(format)
			if err != nil {
				t.Errorf("getFormatter(%q) should be case-insensitive, got error: %v", format, err)
			}
			if f == nil {
				t.Errorf("getFormatter(%q) returned nil formatter", format)
			}
		})
	}
}

func TestGetFormatter_InvalidFormat(t *testing.T) {
	f, err := getFormatter("invalid")
	if err == nil {
		t.Error("getFormatter(\"invalid\") should return error")
	}
	if f != nil {
		t.Error("getFormatter(\"invalid\") should return nil formatter")
	}
	if !strings.Contains(err.Error(), "unsupported format") {
		t.Errorf("error should mention 'unsupported format', got: %v", err)
	}
	if !strings.Contains(err.Error(), "text, json, yaml, csv, table") {
		t.Errorf("error should list supported formats, got: %v", err)
	}
}

func TestGetFormatter_EmptyString(t *testing.T) {
	f, err := getFormatter("")
	if err == nil {
		t.Error("getFormatter(\"\") should return error")
	}
	if f != nil {
		t.Error("getFormatter(\"\") should return nil formatter")
	}
}
