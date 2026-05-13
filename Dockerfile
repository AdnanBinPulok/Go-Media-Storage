# ── Stage 1: build ────────────────────────────────────────────────────────────
FROM golang:1.25-alpine AS builder

# CGo enabled for WebP support (chai2010/webp requires libwebp)
ENV CGO_ENABLED=1 GOOS=linux GOARCH=amd64

RUN apk add --no-cache gcc musl-dev libwebp-dev

WORKDIR /build

# Cache dependency downloads separately from source changes
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source
COPY . .

RUN go build -trimpath -ldflags="-s -w" -o go-media-storage .

# ── Stage 2: runtime ───────────────────────────────────────────────────────────
FROM alpine:3.21

# ca-certificates: needed for outbound TLS (e.g. database over SSL)
# tzdata: predictable time zone handling
# libwebp: runtime shared library required by the WebP encoder
RUN apk add --no-cache ca-certificates tzdata libwebp && \
    addgroup -S app && adduser -S -G app app

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/go-media-storage .

# Copy static assets the server reads at runtime
COPY --chown=app:app static/   ./static/
COPY --chown=app:app assets/   ./assets/
COPY --chown=app:app data/site_example.json     ./data/site_example.json
COPY --chown=app:app data/whitelist_example.json ./data/whitelist_example.json

# Create runtime directories
RUN mkdir -p storage secrets data logs && \
    chown -R app:app /app

USER app

EXPOSE 8432

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:8432/api/ping || exit 1

CMD ["./go-media-storage"]
