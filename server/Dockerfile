# ─────────────────────────────────────────────
# Stage 1: Build
# ─────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

WORKDIR /build

# Cache dependencies separately from source.
COPY server/go.mod server/go.sum ./
RUN go mod download

# Copy source and compile a fully-static binary.
COPY server/. .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o anki-server \
    ./cmd

# ─────────────────────────────────────────────
# Stage 2: Runtime (~8 MB final image)
# ─────────────────────────────────────────────
FROM scratch

# Copy CA certificates for HTTPS calls (DeepL, etc.)
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy compiled binary.
COPY --from=builder /build/anki-server /anki-server

# Persistent buffer volume mount point.
VOLUME ["/data"]

# Default environment — override in Coolify / docker-compose.
ENV PORT=3000 \
    ANKI_URL=http://host.docker.internal:8765 \
    BUFFER_FILE=/data/anki_buffer.json \
    LOG_LEVEL=info

EXPOSE 3000

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD ["/anki-server", "healthcheck"]

ENTRYPOINT ["/anki-server"]
