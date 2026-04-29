# Stage 1: build FFmpeg 7 from source
FROM debian:bookworm-slim AS ffmpeg-builder

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    build-essential \
    curl \
    nasm \
    libx264-dev \
    libgnutls28-dev \
    pkg-config \
    && rm -rf /var/lib/apt/lists/*

ARG FFMPEG_VERSION=8.1
RUN curl -fsSL "https://ffmpeg.org/releases/ffmpeg-${FFMPEG_VERSION}.tar.xz" \
    | tar -xJ \
    && cd ffmpeg-${FFMPEG_VERSION} \
    && ./configure \
        --prefix=/opt/ffmpeg \
        --enable-shared \
        --disable-static \
        --disable-programs \
        --disable-doc \
        --enable-gpl \
        --enable-libx264 \
        --enable-gnutls \
    && make -j"$(nproc)" \
    && make install

# Stage 2: build Go application
FROM golang:1.25-bookworm AS app-builder

COPY --from=ffmpeg-builder /opt/ffmpeg /opt/ffmpeg

ENV PKG_CONFIG_PATH=/opt/ffmpeg/lib/pkgconfig
ENV CGO_ENABLED=1
ENV LD_LIBRARY_PATH=/opt/ffmpeg/lib

RUN apt-get update && apt-get install -y --no-install-recommends \
    libx264-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o jit-transcode .

# Stage 3: minimal runtime
FROM debian:bookworm-slim AS runtime

RUN apt-get update && apt-get install -y --no-install-recommends \
    libx264-164 \
    libgnutls30 \
    && rm -rf /var/lib/apt/lists/*

COPY --from=ffmpeg-builder /opt/ffmpeg/lib /opt/ffmpeg/lib
COPY --from=app-builder /build/jit-transcode /usr/local/bin/jit-transcode

ENV LD_LIBRARY_PATH=/opt/ffmpeg/lib

EXPOSE 8080
ENTRYPOINT ["jit-transcode"]
CMD ["-addr", ":8080"]
