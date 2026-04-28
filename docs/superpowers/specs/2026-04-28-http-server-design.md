# JIT Transcode HTTP Server — Design Spec

**Date:** 2026-04-28

## Overview

Добавить HTTP-сервер поверх существующего `Transcode()`. Клиент делает GET-запрос с параметрами файла, смещения и длительности — сервер транскодирует сегмент на лету и возвращает готовый `.ts` файл.

## CLI Interface

```
./jit-transcode -dir /videos -addr :8080
```

| Флаг | Тип | Default | Описание |
|------|-----|---------|----------|
| `-dir` | string | — | Директория с входными файлами (обязательный) |
| `-addr` | string | `:8080` | Адрес и порт HTTP-сервера |

Старые CLI-флаги (`-input`, `-output`, `-offset`, `-duration`) удаляются.

## HTTP API

**Маршрут:** `GET /transcode`

**Параметры:**

| Параметр | Тип | Обязательный | Описание |
|----------|-----|--------------|----------|
| `file` | string | да | Имя файла в `-dir` директории |
| `offset` | float64 | нет (default `0`) | Смещение от начала файла, секунды |
| `duration` | float64 | да | Длительность сегмента, секунды |

**Пример запроса:**
```
GET /transcode?file=video.mp4&offset=30&duration=10
```

**Успешный ответ:**
- Status: `200 OK`
- `Content-Type: video/mp2t`
- `X-Transcode-Duration: 1234ms` — время транскодирования
- Body: бинарные данные `.ts` файла

**Коды ошибок:**
| Код | Причина |
|-----|---------|
| `400` | Отсутствующий/невалидный параметр (`file`, `duration <= 0`) |
| `404` | Файл не найден в директории |
| `500` | Ошибка транскодирования |

## Архитектура

```
GET /transcode?file=video.mp4&offset=30&duration=10
          │
          ▼
     [Handler — server.go]
          │  1. Распарсить параметры
          │  2. filepath.Base(file) → безопасный путь
          │  3. Проверить существование файла
          │  4. os.CreateTemp("", "jit-*.ts")
          │  5. time.Now() → Transcode() → elapsed
          │  6. Set X-Transcode-Duration header
          │  7. Set Content-Type: video/mp2t
          │  8. io.Copy(w, tmpFile)
          │  9. defer os.Remove(tmpFile)
          ▼
     TS-данные клиенту
```

## Безопасность

`file`-параметр очищается через `filepath.Base()` — принимается только имя файла без пути. Это предотвращает directory traversal атаки (`../../../etc/passwd`).

Итоговый путь: `filepath.Join(dir, filepath.Base(file))`.

## Структура файлов

| Файл | Изменение |
|------|-----------|
| `main.go` | Заменить CLI-флаги на `-dir` / `-addr`, запустить `http.ListenAndServe` |
| `server.go` | Новый — хендлер `/transcode` |
| `transcode.go` | Без изменений |
| `transcode_test.go` | Без изменений (добавить тесты для server.go отдельно) |

## Параллелизм

Каждый запрос обрабатывается в отдельной goroutine (стандартное поведение `net/http`). Ограничений на количество одновременных транскодирований нет.

## Temp-файл lifecycle

```go
tmp, err := os.CreateTemp("", "jit-*.ts")
defer os.Remove(tmp.Name())
defer tmp.Close()
// Transcode → tmp
// Seek tmp to 0
// io.Copy(w, tmp)
```

Temp-файл всегда удаляется через `defer`, даже при ошибках.
