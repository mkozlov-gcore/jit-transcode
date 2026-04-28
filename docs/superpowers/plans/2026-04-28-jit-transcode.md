# JIT Transcode Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** CLI-инструмент на Go, который берёт отрезок локального видеофайла и транскодирует его в H264/MPEG-TS через go-astiav (libav) без внешних процессов.

**Architecture:** `main.go` парсит флаги и вызывает `Transcode()`. `transcode.go` содержит всю логику пайплайна: открытие входного файла, seek, decode → scale → encode → mux в MPEG-TS. Аудио игнорируется.

**Tech Stack:** Go 1.25, `github.com/asticode/go-astiav v0.40.0` (враппер libav), `libx264`, `libswscale`.

---

## File Map

| Файл | Роль |
|------|------|
| `main.go` | CLI-флаги, валидация, вызов Transcode |
| `transcode.go` | `Options`, `Transcode()` — весь пайплайн |
| `transcode_test.go` | Интеграционный тест |

---

## Task 1: CLI-парсинг и валидация

**Files:**
- Modify: `main.go`

- [ ] **Step 1.1: Написать тест CLI-валидации**

Создать `transcode_test.go`:

```go
package main

import (
	"os"
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
```

- [ ] **Step 1.2: Запустить тест, убедиться что падает**

```bash
cd /Users/maximk/dev/gcore/jit-transcode && go test ./... -run TestTranscodeInvalid -v
```

Ожидаемо: `FAIL — undefined: Options`

- [ ] **Step 1.3: Реализовать `Options` и `Validate` в `transcode.go`**

```go
package main

import (
	"errors"
	"math"

	"github.com/asticode/go-astiav"
)

const (
	outWidth   = 640
	outHeight  = 360
	outBitRate = 1_000_000
	defaultFPS = 25
)

type Options struct {
	Input    string
	Output   string
	Offset   float64
	Duration float64
}

func (o Options) Validate() error {
	if o.Input == "" {
		return errors.New("input path is required")
	}
	if o.Output == "" {
		return errors.New("output path is required")
	}
	if o.Duration <= 0 {
		return errors.New("duration must be positive")
	}
	return nil
}
```

- [ ] **Step 1.4: Запустить тесты**

```bash
go test ./... -run TestTranscode -v
```

Ожидаемо: `PASS`

- [ ] **Step 1.5: Написать `main.go`**

```go
package main

import (
	"flag"
	"log"
)

func main() {
	input := flag.String("input", "", "path to input video file")
	output := flag.String("output", "", "path to output .ts file")
	offset := flag.Float64("offset", 0, "start offset in seconds")
	duration := flag.Float64("duration", 0, "duration in seconds")
	flag.Parse()

	opts := Options{
		Input:    *input,
		Output:   *output,
		Offset:   *offset,
		Duration: *duration,
	}
	if err := opts.Validate(); err != nil {
		flag.Usage()
		log.Fatal(err)
	}

	if err := Transcode(opts); err != nil {
		log.Fatal(err)
	}
}
```

- [ ] **Step 1.6: Убедиться что компилируется (Transcode пока стаб)**

Добавить в `transcode.go` стаб:

```go
func Transcode(opts Options) error {
	return nil
}
```

```bash
go build ./...
```

Ожидаемо: успешная компиляция без ошибок.

- [ ] **Step 1.7: Коммит**

```bash
git init && git add main.go transcode.go transcode_test.go && git commit -m "feat: CLI parsing and Options validation"
```

---

## Task 2: Открытие входного файла и поиск видеопотока

**Files:**
- Modify: `transcode.go` — заменить стаб `Transcode()`

- [ ] **Step 2.1: Реализовать открытие входного файла и нахождение видеопотока**

Заменить стаб `Transcode` в `transcode.go`:

```go
func Transcode(opts Options) error {
	astiav.SetLogLevel(astiav.LogLevelError)

	// --- Входной контекст ---
	inputFC := astiav.AllocFormatContext()
	if inputFC == nil {
		return errors.New("failed to alloc input format context")
	}
	defer inputFC.Free()

	if err := inputFC.OpenInput(opts.Input, nil, nil); err != nil {
		return fmt.Errorf("opening input %q: %w", opts.Input, err)
	}
	defer inputFC.CloseInput()

	if err := inputFC.FindStreamInfo(nil); err != nil {
		return fmt.Errorf("finding stream info: %w", err)
	}

	// --- Видеопоток ---
	var inputStream *astiav.Stream
	for _, s := range inputFC.Streams() {
		if s.CodecParameters().MediaType() == astiav.MediaTypeVideo {
			inputStream = s
			break
		}
	}
	if inputStream == nil {
		return errors.New("no video stream found in input")
	}

	return nil // стаб — остальное добавим в следующих задачах
}
```

Добавить импорты в `transcode.go`:

```go
import (
	"errors"
	"fmt"
	"math"

	"github.com/asticode/go-astiav"
)
```

- [ ] **Step 2.2: Убедиться что компилируется**

```bash
go build ./...
```

Ожидаемо: успешно.

- [ ] **Step 2.3: Коммит**

```bash
git add transcode.go && git commit -m "feat: open input file and find video stream"
```

---

## Task 3: Декодер и seek к offset

**Files:**
- Modify: `transcode.go` — добавить decoder и seek перед `return nil`

- [ ] **Step 3.1: Добавить decoder и seek**

В функции `Transcode`, перед `return nil`, добавить:

```go
	// --- Декодер ---
	decoder := astiav.FindDecoder(inputStream.CodecParameters().CodecID())
	if decoder == nil {
		return fmt.Errorf("decoder not found for codec %s", inputStream.CodecParameters().CodecID())
	}

	decoderCtx := astiav.AllocCodecContext(decoder)
	if decoderCtx == nil {
		return errors.New("failed to alloc decoder context")
	}
	defer decoderCtx.Free()

	if err := inputStream.CodecParameters().ToCodecContext(decoderCtx); err != nil {
		return fmt.Errorf("copying codec parameters to decoder: %w", err)
	}

	if err := decoderCtx.Open(decoder, nil); err != nil {
		return fmt.Errorf("opening decoder: %w", err)
	}

	// --- FPS ---
	fpsRat := inputStream.RealFrameRate()
	fpsFloat := float64(fpsRat.Num()) / float64(fpsRat.Den())
	if fpsFloat <= 0 {
		fpsFloat = defaultFPS
	}

	// --- Seek ---
	// Используем AV_TIME_BASE (seekTS в микросекундах), stream_index=-1
	seekTS := int64(opts.Offset * float64(astiav.TimeBase))
	if err := inputFC.SeekFrame(-1, seekTS, astiav.SeekFlagBackward); err != nil {
		return fmt.Errorf("seeking to offset %.2fs: %w", opts.Offset, err)
	}
	// Сбрасываем буферы декодера после seek
	decoderCtx.FlushBuffers()
```

- [ ] **Step 3.2: Убедиться что компилируется**

```bash
go build ./...
```

Ожидаемо: успешно.

- [ ] **Step 3.3: Коммит**

```bash
git add transcode.go && git commit -m "feat: setup decoder and seek to offset"
```

---

## Task 4: Масштабировщик (libswscale)

**Files:**
- Modify: `transcode.go` — добавить scaler после decoder

- [ ] **Step 4.1: Добавить инициализацию scaler**

После строки `decoderCtx.FlushBuffers()`, перед `return nil`:

```go
	// --- Scaler ---
	swsCtx, err := astiav.NewSwsContext(
		decoderCtx.Width(), decoderCtx.Height(), decoderCtx.PixelFormat(),
		outWidth, outHeight, astiav.PixelFormatYuv420P,
		astiav.NewSwsFlags(astiav.SwsFlagBilinear),
	)
	if err != nil {
		return fmt.Errorf("creating swscale context: %w", err)
	}
	defer swsCtx.Free()

	scaledFrame := astiav.AllocFrame()
	if scaledFrame == nil {
		return errors.New("failed to alloc scaled frame")
	}
	defer scaledFrame.Free()

	scaledFrame.SetWidth(outWidth)
	scaledFrame.SetHeight(outHeight)
	scaledFrame.SetPixelFormat(astiav.PixelFormatYuv420P)
	if err := scaledFrame.AllocBuffer(0); err != nil {
		return fmt.Errorf("allocating scaled frame buffer: %w", err)
	}
```

- [ ] **Step 4.2: Убедиться что компилируется**

```bash
go build ./...
```

Ожидаемо: успешно.

- [ ] **Step 4.3: Коммит**

```bash
git add transcode.go && git commit -m "feat: setup libswscale scaler 640x360 YUV420P"
```

---

## Task 5: Энкодер (libx264)

**Files:**
- Modify: `transcode.go` — добавить encoder после scaler

- [ ] **Step 5.1: Добавить инициализацию encoder**

После `scaledFrame.AllocBuffer(0)`, перед `return nil`:

```go
	// --- Encoder ---
	encoder := astiav.FindEncoderByName("libx264")
	if encoder == nil {
		return errors.New("libx264 encoder not found (install libx264 and rebuild ffmpeg with --enable-libx264)")
	}

	encoderCtx := astiav.AllocCodecContext(encoder)
	if encoderCtx == nil {
		return errors.New("failed to alloc encoder context")
	}
	defer encoderCtx.Free()

	encoderCtx.SetWidth(outWidth)
	encoderCtx.SetHeight(outHeight)
	encoderCtx.SetPixelFormat(astiav.PixelFormatYuv420P)
	encoderCtx.SetBitRate(outBitRate)

	fpsInt := int(math.Round(fpsFloat))
	if fpsInt <= 0 {
		fpsInt = defaultFPS
	}
	encoderCtx.SetTimeBase(astiav.NewRational(1, fpsInt))
	encoderCtx.SetGopSize(int(math.Round(fpsFloat * 2))) // ключевой кадр каждые 2с
	encoderCtx.SetMaxBFrames(0)                          // упрощает DTS в MPEG-TS

	// Опции x264 через словарь
	encDict := astiav.NewDictionary()
	defer encDict.Free()
	encDict.Set("preset", "fast", 0)

	if err := encoderCtx.Open(encoder, encDict); err != nil {
		return fmt.Errorf("opening encoder: %w", err)
	}
```

- [ ] **Step 5.2: Убедиться что компилируется**

```bash
go build ./...
```

Ожидаемо: успешно.

- [ ] **Step 5.3: Коммит**

```bash
git add transcode.go && git commit -m "feat: setup libx264 encoder 1Mbps GOP=2s"
```

---

## Task 6: Выходной мультиплексор (MPEG-TS)

**Files:**
- Modify: `transcode.go` — добавить output muxer после encoder

- [ ] **Step 6.1: Добавить output muxer**

После `encoderCtx.Open(encoder, encDict)`, перед `return nil`:

```go
	// --- Output muxer ---
	var outputFC *astiav.FormatContext
	if err := astiav.AllocOutputContext2(&outputFC, nil, "mpegts", opts.Output); err != nil {
		return fmt.Errorf("allocating output context: %w", err)
	}
	defer outputFC.Free()

	outputStream := outputFC.NewStream(nil)
	if outputStream == nil {
		return errors.New("failed to create output stream")
	}

	if err := encoderCtx.ToCodecParameters(outputStream.CodecParameters()); err != nil {
		return fmt.Errorf("copying encoder parameters to output stream: %w", err)
	}
	outputStream.SetTimeBase(encoderCtx.TimeBase())

	// Открываем выходной файл (avio_open)
	if !outputFC.OutputFormat().Flags().Has(astiav.IOFormatFlagNoFile) {
		var pb *astiav.IOContext
		if err := astiav.OpenIOContext(&pb, opts.Output, astiav.IOContextFlagWrite, nil, nil); err != nil {
			return fmt.Errorf("opening output file %q: %w", opts.Output, err)
		}
		defer pb.Close()
		outputFC.SetPb(pb)
	}

	if err := outputFC.WriteHeader(nil); err != nil {
		return fmt.Errorf("writing MPEG-TS header: %w", err)
	}
```

- [ ] **Step 6.2: Убедиться что компилируется**

```bash
go build ./...
```

Ожидаемо: успешно.

- [ ] **Step 6.3: Коммит**

```bash
git add transcode.go && git commit -m "feat: setup MPEG-TS output muxer"
```

---

## Task 7: Основной цикл транскодирования

**Files:**
- Modify: `transcode.go` — заменить `return nil` на реальный цикл

- [ ] **Step 7.1: Реализовать основной transcode-цикл**

Заменить `return nil` в конце `Transcode` на:

```go
	// --- Цикл ---
	pkt := astiav.AllocPacket()
	if pkt == nil {
		return errors.New("failed to alloc packet")
	}
	defer pkt.Free()

	decodedFrame := astiav.AllocFrame()
	if decodedFrame == nil {
		return errors.New("failed to alloc decoded frame")
	}
	defer decodedFrame.Free()

	encodedPkt := astiav.AllocPacket()
	if encodedPkt == nil {
		return errors.New("failed to alloc encoded packet")
	}
	defer encodedPkt.Free()

	endSecs := opts.Offset + opts.Duration
	var outPTS int64

	// writeEncodedPackets сливает все готовые пакеты из энкодера в muxer
	writeEncodedPackets := func() error {
		for {
			if err := encoderCtx.ReceivePacket(encodedPkt); err != nil {
				if errors.Is(err, astiav.ErrEagain) || errors.Is(err, astiav.ErrEof) {
					break
				}
				return fmt.Errorf("receiving encoded packet: %w", err)
			}
			encodedPkt.SetStreamIndex(outputStream.Index())
			encodedPkt.RescaleTs(encoderCtx.TimeBase(), outputStream.TimeBase())
			if err := outputFC.WriteInterleavedFrame(encodedPkt); err != nil {
				return fmt.Errorf("writing packet to muxer: %w", err)
			}
			encodedPkt.Unref()
		}
		return nil
	}

	done := false
	for !done {
		if err := inputFC.ReadFrame(pkt); err != nil {
			if errors.Is(err, astiav.ErrEof) {
				break
			}
			return fmt.Errorf("reading frame: %w", err)
		}

		if pkt.StreamIndex() != inputStream.Index() {
			pkt.Unref()
			continue
		}

		if err := decoderCtx.SendPacket(pkt); err != nil {
			pkt.Unref()
			return fmt.Errorf("sending packet to decoder: %w", err)
		}
		pkt.Unref()

		for {
			if err := decoderCtx.ReceiveFrame(decodedFrame); err != nil {
				if errors.Is(err, astiav.ErrEagain) || errors.Is(err, astiav.ErrEof) {
					break
				}
				return fmt.Errorf("receiving decoded frame: %w", err)
			}

			// Время кадра в секундах относительно начала файла
			tb := inputStream.TimeBase()
			frameSecs := float64(decodedFrame.Pts()) * float64(tb.Num()) / float64(tb.Den())

			// Пропускаем кадры до offset (seek не гарантирует точность до кадра)
			if frameSecs < opts.Offset {
				decodedFrame.Unref()
				continue
			}

			// Останавливаемся после offset+duration
			if frameSecs >= endSecs {
				done = true
				decodedFrame.Unref()
				break
			}

			// Масштабируем
			if err := swsCtx.ScaleFrame(decodedFrame, scaledFrame); err != nil {
				decodedFrame.Unref()
				return fmt.Errorf("scaling frame: %w", err)
			}
			decodedFrame.Unref()

			// Проставляем выходной PTS (0, 1, 2, ... в единицах encoderCtx.TimeBase)
			scaledFrame.SetPts(outPTS)
			outPTS++

			// Кодируем
			if err := encoderCtx.SendFrame(scaledFrame); err != nil {
				return fmt.Errorf("sending frame to encoder: %w", err)
			}
			scaledFrame.Unref()

			if err := writeEncodedPackets(); err != nil {
				return err
			}
		}
	}

	// --- Flush encoder ---
	if err := encoderCtx.SendFrame(nil); err != nil {
		return fmt.Errorf("flushing encoder: %w", err)
	}
	if err := writeEncodedPackets(); err != nil {
		return err
	}

	// --- Trailer ---
	if err := outputFC.WriteTrailer(); err != nil {
		return fmt.Errorf("writing trailer: %w", err)
	}

	return nil
```

- [ ] **Step 7.2: Финальная компиляция**

```bash
go build ./...
```

Ожидаемо: успешно, без ошибок.

- [ ] **Step 7.3: Коммит**

```bash
git add transcode.go && git commit -m "feat: implement main transcode loop with seek, decode, scale, encode, mux"
```

---

## Task 8: Интеграционный тест

**Files:**
- Modify: `transcode_test.go` — добавить интеграционный тест

> **Требования:** `ffmpeg` и `ffprobe` должны быть установлены для генерации тестового видео и проверки результата.

- [ ] **Step 8.1: Добавить интеграционный тест в `transcode_test.go`**

```go
package main

import (
	"os"
	"os/exec"
	"testing"
)

// TestTranscodeIntegration генерирует тестовое видео через ffmpeg,
// транскодирует 5-секундный отрезок и проверяет выходной файл.
func TestTranscodeIntegration(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not found in PATH, skipping integration test")
	}

	// Генерируем 30-секундное тестовое видео 1280x720 25fps
	inputFile := t.TempDir() + "/input.mp4"
	outputFile := t.TempDir() + "/output.ts"

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

	// Проверяем что выходной файл существует и не пуст
	info, err := os.Stat(outputFile)
	if err != nil {
		t.Fatalf("output file not found: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("output file is empty")
	}

	// Проверяем параметры через ffprobe
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
```

- [ ] **Step 8.2: Запустить интеграционный тест**

```bash
go test ./... -run TestTranscodeIntegration -v -timeout 120s
```

Ожидаемо: `PASS`, в логе видны строки `codec_name=h264 width=640 height=360`.

- [ ] **Step 8.3: Коммит**

```bash
git add transcode_test.go && git commit -m "test: add integration test for Transcode"
```

---

## Task 9: Ручная проверка

- [ ] **Step 9.1: Собрать бинарник**

```bash
go build -o jit-transcode .
```

- [ ] **Step 9.2: Запустить на реальном видеофайле**

```bash
./jit-transcode -input /path/to/your/video.mp4 -offset 30 -duration 60 -output out.ts
```

Ожидаемо: файл `out.ts` создан, нет ошибок в stdout/stderr.

- [ ] **Step 9.3: Проверить параметры выходного файла**

```bash
ffprobe -v error -select_streams v:0 \
  -show_entries stream=codec_name,width,height,bit_rate,r_frame_rate \
  -of default=noprint_wrappers=1 out.ts
```

Ожидаемо:
```
codec_name=h264
width=640
height=360
bit_rate=~1000000
r_frame_rate=25/1  (или FPS исходника)
```

- [ ] **Step 9.4: Финальный коммит**

```bash
git add . && git commit -m "feat: jit-transcode complete — H264/MPEG-TS with seek and duration"
```
