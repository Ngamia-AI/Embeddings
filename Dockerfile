# syntax=docker/dockerfile:1

# ---- build ----
FROM golang:1.26 AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /out/embeddings-api ./cmd/api

# ---- runtime ----
FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates ffmpeg \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --system --uid 10001 --no-create-home app

WORKDIR /app
COPY --from=build /out/embeddings-api /app/embeddings-api
COPY migrations /app/migrations

ENV PORT=8090 \
    MIGRATIONS_DIR=/app/migrations \
    FFMPEG_PATH=/usr/bin/ffmpeg

USER app
EXPOSE 8090
ENTRYPOINT ["/app/embeddings-api"]
