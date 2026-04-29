package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"time"
)

var videoSegmentPath = regexp.MustCompile(`^/video_\d+\.ts$`)

func withCORS(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Range")
		w.Header().Set("Access-Control-Expose-Headers", "X-Transcode-Duration, Content-Length, Content-Range")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func newHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !videoSegmentPath.MatchString(r.URL.Path) {
			http.NotFound(w, r)
			return
		}

		q := r.URL.Query()

		fileURL := q.Get("file")
		if fileURL == "" {
			http.Error(w, "missing parameter: file", http.StatusBadRequest)
			return
		}

		u, err := url.ParseRequestURI(fileURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
			http.Error(w, "invalid parameter: file must be an http or https URL", http.StatusBadRequest)
			return
		}

		durationStr := q.Get("duration")
		if durationStr == "" {
			http.Error(w, "missing parameter: duration", http.StatusBadRequest)
			return
		}
		duration, err := strconv.ParseFloat(durationStr, 64)
		if err != nil || duration <= 0 {
			http.Error(w, "invalid parameter: duration must be a positive number", http.StatusBadRequest)
			return
		}

		var offset float64
		if offsetStr := q.Get("offset"); offsetStr != "" {
			offset, err = strconv.ParseFloat(offsetStr, 64)
			if err != nil || offset < 0 {
				http.Error(w, "invalid parameter: offset must be a non-negative number", http.StatusBadRequest)
				return
			}
		}

		tmp, err := os.CreateTemp("", "jit-*.ts")
		if err != nil {
			http.Error(w, "internal error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer os.Remove(tmp.Name())
		defer tmp.Close()

		start := time.Now()
		if err := Transcode(Options{
			Input:    fileURL,
			Output:   tmp.Name(),
			Offset:   offset,
			Duration: duration,
		}); err != nil {
			http.Error(w, "transcode error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		elapsed := time.Since(start)

		if _, err := tmp.Seek(0, io.SeekStart); err != nil {
			http.Error(w, "internal error: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "video/mp2t")
		w.Header().Set("X-Transcode-Duration", fmt.Sprintf("%dms", elapsed.Milliseconds()))
		if _, err := io.Copy(w, tmp); err != nil {
			log.Printf("error writing response: %v", err)
		}
	}
}
