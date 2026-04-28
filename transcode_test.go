package main

import (
	"testing"
)

func TestTranscodeInvalidOptions(t *testing.T) {
	tests := []struct {
		name string
		opts Options
	}{
		{"empty input", Options{Input: "", Output: "out.ts", Offset: 0, Duration: 10}},
		{"empty output", Options{Input: "in.mp4", Output: "", Offset: 0, Duration: 10}},
		{"zero duration", Options{Input: "in.mp4", Output: "out.ts", Offset: 0, Duration: 0}},
		{"negative duration", Options{Input: "in.mp4", Output: "out.ts", Offset: 0, Duration: -1}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.opts.Validate()
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestTranscodeValidOptions(t *testing.T) {
	opts := Options{Input: "in.mp4", Output: "out.ts", Offset: 10, Duration: 30}
	if err := opts.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
