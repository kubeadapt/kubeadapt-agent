# Build stage - ARM64-only (no cross-compilation needed)
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

WORKDIR /workspace

# Download dependencies first (better caching)
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
# CGO_ENABLED=0 produces a statically-linked binary (required for distroless)
RUN CGO_ENABLED=0 go build -ldflags="-w -s" -o app ./cmd/app

# Runtime stage - distroless for minimal attack surface
FROM gcr.io/distroless/static:nonroot

WORKDIR /

# Copy the binary from builder stage
COPY --from=builder /workspace/app .

# Use non-root user (65532:65532 is 'nonroot' user in distroless)
USER 65532:65532

ENTRYPOINT ["/app"]
