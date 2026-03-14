package main

import "fmt"

// Speed preset constants
const (
	SpeedSafe   = "safe"
	SpeedFast   = "fast"
	SpeedInsane = "insane"
)

// speedPresets maps speed preset names to their concurrency levels.
var speedPresets = map[string]int{
	SpeedSafe:   1,
	SpeedFast:   5,
	SpeedInsane: 20,
}

// SpeedResult holds the resolved speed preset name and effective concurrency.
type SpeedResult struct {
	Preset      string
	Concurrency int
}

// resolveSpeedAndConcurrency resolves the effective concurrency level from the
// --speed and --concurrency flags. If concurrencyExplicit is true, the concurrency
// value overrides the speed preset's mapping.
func resolveSpeedAndConcurrency(speed string, concurrency int, concurrencyExplicit bool) (SpeedResult, error) {
	// Validate speed preset
	presetConcurrency, valid := speedPresets[speed]
	if !valid {
		return SpeedResult{}, fmt.Errorf("invalid speed preset: %q (valid presets: safe, fast, insane)", speed)
	}

	// If --concurrency was explicitly set, it overrides the preset
	if concurrencyExplicit {
		if err := validateConcurrency(concurrency); err != nil {
			return SpeedResult{}, err
		}
		return SpeedResult{
			Preset:      speed,
			Concurrency: concurrency,
		}, nil
	}

	return SpeedResult{
		Preset:      speed,
		Concurrency: presetConcurrency,
	}, nil
}
