# JIT Transcode — Design Spec

**Date:** 2026-04-28

## Overview

CLI-приложение на Go, которое транскодирует произвольный отрезок локального видеофайла в H264/MPEG-TS с фиксированными параметрами. Использует `github.com/asticode/go-astiav` (враппер над libav) — без внешних процессов.

## CLI Interface

```
./jit-transcode -input <path> -offset <seconds> -duration <seconds> -output <path.ts>
```

| Флаг | Тип | Описание |
|------|-----|----------|
| `-input` | string | Путь к входному видеофайлу |
| `-offset` | float64 | Смещение от начала файла в секундах |
| `-duration` | float64 | Длительность кодируемого отрезка в секундах |
| `-output` | string | Путь к выходному `.ts` файлу |

## Pipeline

```
Input file
    │
    ▼
[Demuxer] — AVFormatContext, seek к offset
    │  raw packets (только видео-стрим)
    ▼
[Decoder] — AVCodecContext (авто-детект кодека источника)
    │  decoded AVFrame (YUV)
    ▼
[Scaler] — libswscale → YUV420P 640×360
    │  scaled AVFrame
    ▼
[Encoder] — libx264, 1Mbps, GOP = round(fps×2)
    │  encoded AVPacket
    ▼
[Muxer] — mpegts, запись в .ts файл
```

Аудио не обрабатывается — аудио-пакеты пропускаются.

## Компоненты

### Demuxer / Seek
- `avformat.OpenInput` открывает входной файл
- `avformat.FindStreamInfo` определяет стримы
- Находим первый видео-стрим по `CodecType == MediaTypeVideo`
- `avformat.SeekFile` с флагом `AVSEEK_FLAG_BACKWARD` — ищем ближайший keyframe до `offset`
- После seek декодируем и отбрасываем фреймы пока `framePTS < offset` (точный trim на уровне фреймов)

### Decoder
- Создаём `AVCodecContext` по `CodecID` из параметров видео-стрима
- `avcodec.Open` с параметрами стрима (`CodecParameters.ToContext`)
- Декодируем пакет → получаем `AVFrame`

### Scaler
- `swscale.NewContext` с входными размерами из декодера и выходными 640×360, `SWS_BILINEAR`
- Аллоцируем выходной `AVFrame` под `AV_PIX_FMT_YUV420P`

### Encoder
- Codec: `libx264`
- Width: 640, Height: 360
- PixFmt: `AV_PIX_FMT_YUV420P`
- BitRate: 1 000 000 bps
- TimeBase: `1/fps` (берём `FrameRate` из входного стрима; если не определён — fallback 25 fps)
- GOPSize: `int(math.Round(fps * 2))` — ключевой кадр каждые 2 секунды
- MaxBFrames: 0 — упрощает временны́е метки в TS
- AVOption `preset=fast`

### Временны́е метки
- Первый выходной фрейм: `pts = 0`
- Каждый следующий: `pts += 1` (в единицах TimeBase `1/fps`)
- При записи пакета в muxer: `av_rescale_q` из TimeBase энкодера в TimeBase стрима выходного контейнера

### Muxer
- `avformat.AllocOutputContext2` с форматом `mpegts`
- Создаём выходной видео-стрим, копируем параметры с энкодера
- `avformat.WriteHeader` перед первым пакетом
- `avformat.WriteTrailer` после флаша энкодера

### Остановка
- В основном цикле: если `framePTS >= offset + duration` — прерываем цикл
- Если `offset + duration` превышает длину файла — читаем до EOF, это не ошибка
- Затем флашим энкодер (`nil`-фрейм) и записываем оставшиеся пакеты

## Обработка ошибок

Любая ошибка — `log.Fatal(err)`. Приложение завершается с ненулевым кодом. Все ресурсы освобождаются через `defer` (контексты, фреймы, пакеты).

## Структура файлов

```
jit-transcode/
├── main.go        # весь код приложения
├── go.mod
└── go.sum
```

## Зависимости

- `github.com/asticode/go-astiav` — уже в go.mod
