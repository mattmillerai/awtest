package main

import (
	"testing"
)

func TestValidateConcurrency(t *testing.T) {
	tests := []struct {
		name    string
		value   int
		wantErr bool
		errMsg  string
	}{
		{"default value 1", 1, false, ""},
		{"valid value 10", 10, false, ""},
		{"valid max 20", 20, false, ""},
		{"valid value 5", 5, false, ""},
		{"invalid zero", 0, true, "Concurrency must be >= 1"},
		{"invalid negative", -1, true, "Concurrency must be >= 1"},
		{"invalid too high 21", 21, true, "Concurrency must be <= 20"},
		{"invalid too high 50", 50, true, "Concurrency must be <= 20"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConcurrency(tt.value)
			if tt.wantErr {
				if err == nil {
					t.Errorf("validateConcurrency(%d) expected error, got nil", tt.value)
				} else if err.Error() != tt.errMsg {
					t.Errorf("validateConcurrency(%d) error = %q, want %q", tt.value, err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("validateConcurrency(%d) unexpected error: %v", tt.value, err)
				}
			}
		})
	}
}
