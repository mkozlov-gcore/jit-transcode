package main

import (
	"os"
	"os/exec"
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

func TestTranscodeIntegration(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not found in PATH, skipping integration test")
	}

	tempDir := t.TempDir()
	inputFile := tempDir + "/input.mp4"
	outputFile := tempDir + "/output.ts"

	cmd := exec.Command("ffmpeg", "-y",
		"-f", "lavfi", "-i", "testsrc=duration=30:size=1280x720:rate=25",
		"-c:v", "libx264", "-t", "30",
		inputFile,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg generate failed: %v\n%s", err, out)
	}

	err := Transcode(Options{
		Input:    inputFile,
		Output:   outputFile,
		Offset:   5,
		Duration: 10,
	})
	if err != nil {
		t.Fatalf("Transcode failed: %v", err)
	}

	info, err := os.Stat(outputFile)
	if err != nil {
		t.Fatalf("output file not found: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("output file is empty")
	}

	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Log("ffprobe not found, skipping parameter verification")
		return
	}
	probeOut, err := exec.Command("ffprobe", "-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=codec_name,width,height,bit_rate",
		"-of", "default=noprint_wrappers=1",
		outputFile,
	).CombinedOutput()
	if err != nil {
		t.Fatalf("ffprobe failed: %v\n%s", err, probeOut)
	}
	t.Logf("ffprobe output:\n%s", probeOut)
}
