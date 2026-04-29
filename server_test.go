package main

import (
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
)

const validPath = "/video_0.ts"

func TestHandlerInvalidPath(t *testing.T) {
	h := newHandler()
	for _, path := range []string{"/transcode", "/video_.ts", "/video_abc.ts", "/video_0.mp4", "/"} {
		req := httptest.NewRequest(http.MethodGet, path+"?file=http://example.com/video.mp4&duration=10", nil)
		w := httptest.NewRecorder()
		h(w, req)
		if w.Code != http.StatusNotFound {
			t.Errorf("path %q: expected 404, got %d", path, w.Code)
		}
	}
}

func TestHandlerMissingFile(t *testing.T) {
	h := newHandler()
	req := httptest.NewRequest(http.MethodGet, validPath+"?duration=10", nil)
	w := httptest.NewRecorder()
	h(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlerMissingDuration(t *testing.T) {
	h := newHandler()
	req := httptest.NewRequest(http.MethodGet, validPath+"?file=http://example.com/video.mp4", nil)
	w := httptest.NewRecorder()
	h(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlerZeroDuration(t *testing.T) {
	h := newHandler()
	req := httptest.NewRequest(http.MethodGet, validPath+"?file=http://example.com/video.mp4&duration=0", nil)
	w := httptest.NewRecorder()
	h(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlerNegativeDuration(t *testing.T) {
	h := newHandler()
	req := httptest.NewRequest(http.MethodGet, validPath+"?file=http://example.com/video.mp4&duration=-5", nil)
	w := httptest.NewRecorder()
	h(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlerInvalidOffset(t *testing.T) {
	h := newHandler()
	req := httptest.NewRequest(http.MethodGet, validPath+"?file=http://example.com/video.mp4&duration=10&offset=abc", nil)
	w := httptest.NewRecorder()
	h(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlerNegativeOffset(t *testing.T) {
	h := newHandler()
	req := httptest.NewRequest(http.MethodGet, validPath+"?file=http://example.com/video.mp4&duration=10&offset=-1", nil)
	w := httptest.NewRecorder()
	h(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlerInvalidDurationString(t *testing.T) {
	h := newHandler()
	req := httptest.NewRequest(http.MethodGet, validPath+"?file=http://example.com/video.mp4&duration=abc", nil)
	w := httptest.NewRecorder()
	h(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlerInvalidFileScheme(t *testing.T) {
	h := newHandler()
	for _, file := range []string{"ftp://example.com/video.mp4", "/local/path/video.mp4", "../etc/passwd", "video.mp4"} {
		req := httptest.NewRequest(http.MethodGet, validPath+"?file="+file+"&duration=10", nil)
		w := httptest.NewRecorder()
		h(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("file %q: expected 400, got %d", file, w.Code)
		}
	}
}

func TestManifestHandlerMissingFile(t *testing.T) {
	h := manifestHandler()
	req := httptest.NewRequest(http.MethodGet, "/video_jit.m3u8", nil)
	w := httptest.NewRecorder()
	h(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestManifestHandlerInvalidFileScheme(t *testing.T) {
	h := manifestHandler()
	req := httptest.NewRequest(http.MethodGet, "/video_jit.m3u8?file=/local/path.mp4", nil)
	w := httptest.NewRecorder()
	h(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestManifestHandlerIntegration(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not found in PATH, skipping integration test")
	}

	dir := t.TempDir()
	inputFile := dir + "/input.mp4"

	cmd := exec.Command("ffmpeg", "-y",
		"-f", "lavfi", "-i", "testsrc=duration=10:size=640x360:rate=25",
		"-c:v", "libx264", "-t", "10",
		inputFile,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg generate failed: %v\n%s", err, out)
	}

	fileServer := httptest.NewServer(http.FileServer(http.Dir(dir)))
	defer fileServer.Close()

	h := manifestHandler()
	req := httptest.NewRequest(http.MethodGet, "/video_jit.m3u8?file="+fileServer.URL+"/input.mp4", nil)
	w := httptest.NewRecorder()
	h(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	body := w.Body.String()
	if !strings.HasPrefix(body, "#EXTM3U") {
		t.Errorf("expected m3u8 body to start with #EXTM3U, got: %q", body[:min(len(body), 50)])
	}
	if !strings.Contains(body, "#EXT-X-ENDLIST") {
		t.Error("manifest missing #EXT-X-ENDLIST")
	}
	if !strings.Contains(body, "#EXT-X-PLAYLIST-TYPE:VOD") {
		t.Error("manifest missing #EXT-X-PLAYLIST-TYPE:VOD")
	}
	if !strings.Contains(body, "video_0.ts") {
		t.Error("manifest missing first segment video_0.ts")
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/vnd.apple.mpegurl" {
		t.Errorf("expected Content-Type application/vnd.apple.mpegurl, got %q", ct)
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

	fileServer := httptest.NewServer(http.FileServer(http.Dir(dir)))
	defer fileServer.Close()

	h := newHandler()
	req := httptest.NewRequest(http.MethodGet, "/video_0.ts?file="+fileServer.URL+"/input.mp4&offset=5&duration=10", nil)
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
