# Build stage — builds the ingestion stub binary from the repo root
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache ca-certificates

WORKDIR /app

# Download dependencies first (layer cache)
COPY go.mod go.sum ./
RUN go mod download

# Copy source needed for the stub binary
COPY pkg/ pkg/
COPY tests/e2e/stub/ tests/e2e/stub/

RUN CGO_ENABLED=0 go build -ldflags="-w -s" -o ingestion-stub ./tests/e2e/stub

# Runtime stage
FROM gcr.io/distroless/static-debian12

COPY --from=builder /app/ingestion-stub .

USER nonroot:nonroot

EXPOSE 8080

ENTRYPOINT ["/ingestion-stub"]
