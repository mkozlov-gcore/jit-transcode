package main

import (
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
)

func TestHandlerMissingFile(t *testing.T) {
	h := newHandler("/tmp")
	req := httptest.NewRequest(http.MethodGet, "/transcode?duration=10", nil)
	w := httptest.NewRecorder()
	h(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlerMissingDuration(t *testing.T) {
	h := newHandler("/tmp")
	req := httptest.NewRequest(http.MethodGet, "/transcode?file=video.mp4", nil)
	w := httptest.NewRecorder()
	h(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlerZeroDuration(t *testing.T) {
	h := newHandler("/tmp")
	req := httptest.NewRequest(http.MethodGet, "/transcode?file=video.mp4&duration=0", nil)
	w := httptest.NewRecorder()
	h(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlerNegativeDuration(t *testing.T) {
	h := newHandler("/tmp")
	req := httptest.NewRequest(http.MethodGet, "/transcode?file=video.mp4&duration=-5", nil)
	w := httptest.NewRecorder()
	h(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlerInvalidOffset(t *testing.T) {
	h := newHandler("/tmp")
	req := httptest.NewRequest(http.MethodGet, "/transcode?file=video.mp4&duration=10&offset=abc", nil)
	w := httptest.NewRecorder()
	h(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlerFileNotFound(t *testing.T) {
	h := newHandler("/tmp")
	req := httptest.NewRequest(http.MethodGet, "/transcode?file=definitely_does_not_exist_xyz.mp4&duration=10", nil)
	w := httptest.NewRecorder()
	h(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlerPathTraversal(t *testing.T) {
	dir := t.TempDir()
	h := newHandler(dir)
	req := httptest.NewRequest(http.MethodGet, "/transcode?file=../etc/passwd&duration=10", nil)
	w := httptest.NewRecorder()
	h(w, req)
	if w.Code == http.StatusOK {
		t.Fatal("path traversal should not succeed")
	}
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for traversal attempt, got %d", w.Code)
	}
}

func TestHandlerNegativeOffset(t *testing.T) {
	h := newHandler("/tmp")
	req := httptest.NewRequest(http.MethodGet, "/transcode?file=video.mp4&duration=10&offset=-1", nil)
	w := httptest.NewRecorder()
	h(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlerInvalidDurationString(t *testing.T) {
	h := newHandler("/tmp")
	req := httptest.NewRequest(http.MethodGet, "/transcode?file=video.mp4&duration=abc", nil)
	w := httptest.NewRecorder()
	h(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlerIntegration(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not found in PATH, skipping integration test")
	}

	dir := t.TempDir()
	inputFile := dir + "/input.mp4"

	cmd := exec.Command("ffmpeg", "-y",
		"-f", "lavfi", "-i", "testsrc=duration=30:size=1280x720:rate=25",
		"-c:v", "libx264", "-t", "30",
		inputFile,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg generate failed: %v\n%s", err, out)
	}

	h := newHandler(dir)
	req := httptest.NewRequest(http.MethodGet, "/transcode?file=input.mp4&offset=5&duration=10", nil)
	w := httptest.NewRecorder()
	h(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	ct := w.Header().Get("Content-Type")
	if ct != "video/mp2t" {
		t.Errorf("expected Content-Type video/mp2t, got %q", ct)
	}

	dur := w.Header().Get("X-Transcode-Duration")
	if dur == "" {
		t.Error("X-Transcode-Duration header is missing")
	}
	if !strings.HasSuffix(dur, "ms") {
		t.Errorf("X-Transcode-Duration should end with 'ms', got %q", dur)
	}

	if w.Body.Len() == 0 {
		t.Error("response body is empty")
	}
}
