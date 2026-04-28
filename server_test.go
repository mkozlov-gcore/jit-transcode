package main

import (
	"net/http"
	"net/http/httptest"
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
