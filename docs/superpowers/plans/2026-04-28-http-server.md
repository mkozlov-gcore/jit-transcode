# JIT Transcode HTTP Server Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Добавить HTTP-сервер, который принимает GET-запросы с параметрами файла/offset/duration, транскодирует сегмент на лету и возвращает `.ts` данные с заголовком `X-Transcode-Duration`.

**Architecture:** Хендлер `newHandler(dir)` валидирует параметры, вызывает существующий `Transcode()` во временный файл, затем копирует результат в `http.ResponseWriter`. `main.go` переключается на запуск HTTP-сервера с флагами `-dir` и `-addr`.

**Tech Stack:** Go stdlib (`net/http`, `os`, `io`, `path/filepath`), существующий `Transcode(Options) error` из `transcode.go`.

---

## File Map

| Файл | Изменение |
|------|-----------|
| `main.go` | Заменить CLI-флаги на `-dir`/`-addr`, запустить HTTP-сервер |
| `server.go` | Новый — `newHandler(dir string) http.HandlerFunc` |
| `server_test.go` | Новый — unit и интеграционные тесты хендлера |
| `transcode.go` | Без изменений |
| `transcode_test.go` | Без изменений |

---

## Task 1: Обновить main.go

**Files:**
- Modify: `main.go`

- [ ] **Step 1.1: Написать обновлённый `main.go`**

Полностью заменить содержимое `main.go`:

```go
package main

import (
	"flag"
	"log"
	"net/http"
)

func main() {
	dir := flag.String("dir", "", "directory containing input video files")
	addr := flag.String("addr", ":8080", "HTTP server address")
	flag.Parse()

	if *dir == "" {
		flag.Usage()
		log.Fatal("dir is required")
	}

	http.HandleFunc("/transcode", newHandler(*dir))
	log.Printf("listening on %s, serving files from %s", *addr, *dir)
	if err := http.ListenAndServe(*addr, nil); err != nil {
		log.Fatal(err)
	}
}
```

- [ ] **Step 1.2: Убедиться что не компилируется (newHandler ещё не определён)**

```bash
cd /Users/maximk/dev/gcore/jit-transcode && go build ./...
```

Ожидаемо: ошибка `undefined: newHandler`

- [ ] **Step 1.3: Коммит**

```bash
git add main.go && git commit -m "feat: switch main to HTTP server mode with -dir/-addr flags"
```

---

## Task 2: Создать server.go с хендлером

**Files:**
- Create: `server.go`

- [ ] **Step 2.1: Создать `server_test.go` с unit-тестами валидации**

```go
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
	dir := t.TempDir() // пустая директория
	h := newHandler(dir)
	req := httptest.NewRequest(http.MethodGet, "/transcode?file=../etc/passwd&duration=10", nil)
	w := httptest.NewRecorder()
	h(w, req)
	// filepath.Base("../etc/passwd") == "passwd", которого нет в dir → 404
	if w.Code == http.StatusOK {
		t.Fatal("path traversal should not succeed")
	}
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for traversal attempt, got %d", w.Code)
	}
}
```

- [ ] **Step 2.2: Запустить тесты — убедиться что падают**

```bash
go test ./... -run TestHandler -v
```

Ожидаемо: FAIL — `undefined: newHandler`

- [ ] **Step 2.3: Создать `server.go` с реализацией**

```go
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

		// --- Валидация параметров ---
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

		// --- Проверка файла (защита от directory traversal) ---
		inputPath := filepath.Join(dir, filepath.Base(file))
		if _, err := os.Stat(inputPath); os.IsNotExist(err) {
			http.Error(w, "file not found", http.StatusNotFound)
			return
		}

		// --- Временный файл для output ---
		tmp, err := os.CreateTemp("", "jit-*.ts")
		if err != nil {
			http.Error(w, "internal error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer os.Remove(tmp.Name())
		defer tmp.Close()

		// --- Транскодирование ---
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

		// --- Ответ ---
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
```

- [ ] **Step 2.4: Запустить unit-тесты хендлера**

```bash
go test ./... -run TestHandler -v
```

Ожидаемо: все 7 тестов PASS

- [ ] **Step 2.5: Убедиться что всё компилируется**

```bash
go build ./...
```

Ожидаемо: успешно.

- [ ] **Step 2.6: Коммит**

```bash
git add server.go server_test.go && git commit -m "feat: HTTP handler with param validation, temp-file transcode, X-Transcode-Duration"
```

---

## Task 3: Интеграционный тест хендлера

**Files:**
- Modify: `server_test.go` — добавить интеграционный тест

- [ ] **Step 3.1: Добавить `TestHandlerIntegration` в `server_test.go`**

Дописать в конец `server_test.go`:

```go
import (
	"os/exec"
	"strings"
)

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
```

Обратите внимание: импорты нужно объединить с уже существующими в файле. Итоговый блок импортов `server_test.go`:

```go
import (
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
)
```

- [ ] **Step 3.2: Запустить интеграционный тест**

```bash
go test ./... -run TestHandlerIntegration -v -timeout 120s
```

Ожидаемо: PASS, в выводе видны заголовки `Content-Type: video/mp2t` и `X-Transcode-Duration: NNNms`.

- [ ] **Step 3.3: Запустить все тесты**

```bash
go test ./... -v -timeout 120s
```

Ожидаемо: все тесты PASS.

- [ ] **Step 3.4: Коммит**

```bash
git add server_test.go && git commit -m "test: integration test for HTTP handler"
```

---

## Task 4: Ручная проверка

- [ ] **Step 4.1: Собрать бинарник**

```bash
go build -o jit-transcode .
```

- [ ] **Step 4.2: Запустить сервер**

```bash
./jit-transcode -dir /path/to/videos -addr :8080
```

- [ ] **Step 4.3: Проверить запрос**

```bash
curl -v "http://localhost:8080/transcode?file=video.mp4&offset=30&duration=10" -o segment.ts
```

Ожидаемо в заголовках:
```
Content-Type: video/mp2t
X-Transcode-Duration: NNNms
```

- [ ] **Step 4.4: Проверить 400 при невалидных параметрах**

```bash
curl -v "http://localhost:8080/transcode?file=video.mp4"
```

Ожидаемо: `400 Bad Request`, тело `missing parameter: duration`

- [ ] **Step 4.5: Финальный коммит**

```bash
git add . && git commit -m "feat: HTTP JIT transcode server complete"
```
