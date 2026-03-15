package main

import (
	"strings"
	"testing"
)

func TestResolveSpeedAndConcurrency(t *testing.T) {
	tests := []struct {
		name                string
		speed               string
		concurrency         int
		concurrencyExplicit bool
		wantPreset          string
		wantConcurrency     int
		wantErr             bool
		errContains         string
	}{
		// Default: no flags → safe, concurrency=1
		{
			name:            "default no flags",
			speed:           SpeedSafe,
			concurrency:     1,
			wantPreset:      SpeedSafe,
			wantConcurrency: 1,
		},
		// Each speed preset → correct concurrency mapping
		{
			name:            "speed safe",
			speed:           SpeedSafe,
			concurrency:     1,
			wantPreset:      SpeedSafe,
			wantConcurrency: 1,
		},
		{
			name:            "speed fast",
			speed:           SpeedFast,
			concurrency:     1,
			wantPreset:      SpeedFast,
			wantConcurrency: 5,
		},
		{
			name:            "speed insane",
			speed:           SpeedInsane,
			concurrency:     1,
			wantPreset:      SpeedInsane,
			wantConcurrency: 20,
		},
		// Concurrency override scenarios
		{
			name:                "concurrency overrides fast",
			speed:               SpeedFast,
			concurrency:         10,
			concurrencyExplicit: true,
			wantPreset:          SpeedFast,
			wantConcurrency:     10,
		},
		{
			name:                "concurrency overrides insane",
			speed:               SpeedInsane,
			concurrency:         3,
			concurrencyExplicit: true,
			wantPreset:          SpeedInsane,
			wantConcurrency:     3,
		},
		{
			name:                "concurrency=1 with insane",
			speed:               SpeedInsane,
			concurrency:         1,
			concurrencyExplicit: true,
			wantPreset:          SpeedInsane,
			wantConcurrency:     1,
		},
		{
			name:                "concurrency=20 with safe",
			speed:               SpeedSafe,
			concurrency:         20,
			concurrencyExplicit: true,
			wantPreset:          SpeedSafe,
			wantConcurrency:     20,
		},
		{
			name:                "concurrency without speed flag",
			speed:               SpeedSafe,
			concurrency:         10,
			concurrencyExplicit: true,
			wantPreset:          SpeedSafe,
			wantConcurrency:     10,
		},
		// Invalid speed preset → error with valid options listed
		{
			name:        "invalid speed preset",
			speed:       "invalid",
			concurrency: 1,
			wantErr:     true,
			errContains: "valid presets: safe, fast, insane",
		},
		{
			name:        "empty speed preset",
			speed:       "",
			concurrency: 1,
			wantErr:     true,
			errContains: "valid presets: safe, fast, insane",
		},
		{
			name:        "uppercase speed preset",
			speed:       "FAST",
			concurrency: 1,
			wantErr:     true,
			errContains: "valid presets: safe, fast, insane",
		},
		// Concurrency out-of-range with speed preset → error
		{
			name:                "concurrency too high with speed",
			speed:               SpeedFast,
			concurrency:         21,
			concurrencyExplicit: true,
			wantErr:             true,
			errContains:         "Concurrency must be <= 20",
		},
		{
			name:                "concurrency zero with speed",
			speed:               SpeedFast,
			concurrency:         0,
			concurrencyExplicit: true,
			wantErr:             true,
			errContains:         "Concurrency must be >= 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := resolveSpeedAndConcurrency(tt.speed, tt.concurrency, tt.concurrencyExplicit)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errContains)
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error = %q, want containing %q", err.Error(), tt.errContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Preset != tt.wantPreset {
				t.Errorf("preset = %q, want %q", result.Preset, tt.wantPreset)
			}
			if result.Concurrency != tt.wantConcurrency {
				t.Errorf("concurrency = %d, want %d", result.Concurrency, tt.wantConcurrency)
			}
		})
	}
}
