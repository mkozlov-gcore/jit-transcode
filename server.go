package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

func newHandler(dir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		file := q.Get("file")
		if file == "" {
			http.Error(w, "missing parameter: file", http.StatusBadRequest)
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

		inputPath := filepath.Join(dir, filepath.Base(file))
		if _, err := os.Stat(inputPath); os.IsNotExist(err) {
			http.Error(w, "file not found", http.StatusNotFound)
			return
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
			Input:    inputPath,
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
