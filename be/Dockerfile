# syntax=docker/dockerfile:1

FROM golang:1.25-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api
# Batch workers are built into the same image and selected at runtime via
# `command:` in docker-compose — one image, isolated processes/containers.
# See docs/AGENT_STATE.md "배치 잡을 독립 워커 프로세스로 분리".
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/worker-close-domestic ./cmd/worker-close-domestic
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/worker-close-overseas ./cmd/worker-close-overseas
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/worker-scores ./cmd/worker-scores
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/worker-index ./cmd/worker-index
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/worker-fetch ./cmd/worker-fetch
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/worker-research ./cmd/worker-research
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/worker-earnings ./cmd/worker-earnings
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/worker-filings ./cmd/worker-filings

FROM debian:bookworm-slim AS runtime

RUN apt-get update \
  && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    tesseract-ocr \
    tesseract-ocr-kor \
    tesseract-ocr-eng \
  && rm -rf /var/lib/apt/lists/* \
  && useradd -r -u 10001 -g root appuser \
  && mkdir -p /app/migrations /var/lib/quant-port/import /var/lib/quant-port/media \
  && chown -R appuser:root /app /var/lib/quant-port/import /var/lib/quant-port/media \
  && chmod 775 /var/lib/quant-port/import /var/lib/quant-port/media

WORKDIR /app

COPY --from=build /out/api ./api
COPY --from=build /out/worker-close-domestic ./worker-close-domestic
COPY --from=build /out/worker-close-overseas ./worker-close-overseas
COPY --from=build /out/worker-scores ./worker-scores
COPY --from=build /out/worker-index ./worker-index
COPY --from=build /out/worker-fetch ./worker-fetch
COPY --from=build /out/worker-research ./worker-research
COPY --from=build /out/worker-earnings ./worker-earnings
COPY --from=build /out/worker-filings ./worker-filings
COPY migrations ./migrations
COPY docs/BATCH_SCHEDULE.md ./docs/BATCH_SCHEDULE.md
RUN chown -R appuser:root /app/docs \
  && chmod 664 /app/docs/BATCH_SCHEDULE.md

# media 디렉터리는 이미지에도 만들어 둔다 — docker가 빈 named volume을 마운트할 때
# 이미지의 소유권/권한을 복사하므로, 없으면 root 소유 볼륨이 생겨 appuser가 못 쓴다.
ENV API_HOST=0.0.0.0 \
    API_PORT=8080 \
    IMPORT_TEMP_DIR=/var/lib/quant-port/import \
    MEDIA_DIR=/var/lib/quant-port/media

USER appuser

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
  CMD curl -fsS "http://127.0.0.1:${API_PORT:-8080}/health" >/dev/null || exit 1

# Default command runs the API; docker-compose overrides `command:` for each
# worker service to run its own binary from this same image instead.
CMD ["./api"]
