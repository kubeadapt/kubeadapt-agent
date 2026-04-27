package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"

	"github.com/kubeadapt/kubeadapt-agent/internal/config"
	agenterrors "github.com/kubeadapt/kubeadapt-agent/internal/errors"
	"github.com/kubeadapt/kubeadapt-agent/internal/observability"
	"github.com/kubeadapt/kubeadapt-agent/pkg/model"
)

// Protocol constants shared with the ingestion-api server. Values must match
// internal/server/middleware/prefilter.go or pre-filter returns 404.
const (
	// ProtocolHeader is the handshake header the server pre-filter requires.
	ProtocolHeader = "X-Kubeadapt-Protocol"

	// ProtocolVersion is the wire-protocol version this agent speaks.
	ProtocolVersion = "v1"

	// ContentEncoding is the only encoding the ingest endpoint accepts.
	ContentEncoding = "zstd"

	// DefaultMaxCompressedBodyBytes mirrors the server's MAX_COMPRESSED_BODY_SIZE.
	// Oversize snapshots are rejected locally to avoid an HTTP 413 round-trip.
	DefaultMaxCompressedBodyBytes int64 = 52428800 // 50 MiB

	// IngestPath is the REST path we POST snapshots to.
	IngestPath = "/api/v1/metrics/ingest"
)

// ErrPayloadTooLarge is returned when the compressed snapshot exceeds the
// server's declared limit. Non-retryable — the same payload will fail again.
var ErrPayloadTooLarge = errors.New("transport: compressed snapshot exceeds server limit")

// SendStats holds payload size info from the last successful send.
type SendStats struct {
	OriginalBytes    int64
	CompressedBytes  int64
	EncodeDurationMs int64
}

// Client sends ClusterSnapshots to the backend as zstd-compressed HTTP POSTs.
// The compressed body is buffered (not streamed) so net/http can set
// Content-Length, which the server's pre-filter middleware requires.
// Memory cost: up to MaxCompressedBodyBytes (50 MiB) per in-flight request.
type Client struct {
	httpClient             *http.Client
	config                 *config.Config
	metrics                *observability.Metrics
	errorCollector         *agenterrors.ErrorCollector
	maxCompressedBodyBytes int64
	lastSendStats          SendStats // updated after each successful send
}

// NewClient creates a transport Client with middleware applied.
// Retry is handled at the Send level so the buffered body can be re-read.
func NewClient(cfg *config.Config, metrics *observability.Metrics, errCollector *agenterrors.ErrorCollector) *Client {
	// Use an explicit transport instead of http.DefaultTransport to avoid
	// sharing mutable state with other code in the process.
	base := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
	}

	// Auth middleware decorates every request with the bearer token.
	transport := WithAuth(cfg.APIKey, base)

	// Allow the config to override the default cap; fall back to 50 MiB.
	maxCompressed := cfg.MaxCompressedBodyBytes
	if maxCompressed <= 0 {
		maxCompressed = DefaultMaxCompressedBodyBytes
	}

	return &Client{
		httpClient: &http.Client{
			Timeout:   cfg.RequestTimeout,
			Transport: transport,
		},
		config:                 cfg,
		metrics:                metrics,
		errorCollector:         errCollector,
		maxCompressedBodyBytes: maxCompressed,
	}
}

// Send serializes, zstd-compresses, and POSTs a ClusterSnapshot.
// Retries are applied at this layer; protocol/size/auth/quota errors are
// treated as terminal (see isNonRetryableError).
func (c *Client) Send(ctx context.Context, snapshot *model.ClusterSnapshot) (*model.SnapshotResponse, error) {
	start := time.Now()

	// Encode + compress once before the retry loop; output is identical across
	// attempts and lets us enforce the size cap before sending.
	encodeStart := time.Now()
	compressed, originalBytes, encodeErr := c.encodeSnapshot(snapshot)
	encodeDurationMs := time.Since(encodeStart).Milliseconds()
	if encodeErr != nil {
		return nil, encodeErr
	}
	compressedBytes := int64(len(compressed))

	// Reject locally if over the server's declared cap to avoid an HTTP 413 round-trip.
	if c.maxCompressedBodyBytes > 0 && compressedBytes > c.maxCompressedBodyBytes {
		return nil, fmt.Errorf("%w (compressed=%d, limit=%d)",
			ErrPayloadTooLarge, compressedBytes, c.maxCompressedBodyBytes)
	}

	var result *model.SnapshotResponse
	var lastErr error

	maxAttempts := c.config.MaxRetries + 1
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			// Record retry metric.
			if c.metrics != nil {
				c.metrics.TransportRetries.Inc()
			}
			sleepWithBackoff(attempt - 1)
		}

		// Check context before each attempt.
		if err := ctx.Err(); err != nil {
			lastErr = fmt.Errorf("transport: context canceled before attempt %d: %w", attempt+1, err)
			break
		}

		resp, err := c.doSend(ctx, snapshot.SnapshotID, compressed)
		if err != nil {
			lastErr = err
			// Don't retry auth failures, payload-too-large, or protocol errors.
			if isNonRetryableError(err) {
				break
			}
			continue
		}

		result = resp
		lastErr = nil
		break
	}

	elapsed := time.Since(start)

	// Record metrics if available.
	if c.metrics != nil {
		c.metrics.SnapshotSendDuration.Observe(elapsed.Seconds())
		if compressedBytes > 0 {
			c.metrics.SnapshotSizeBytes.WithLabelValues("compressed").Observe(float64(compressedBytes))
		}
		if lastErr != nil {
			c.metrics.SnapshotSendTotal.WithLabelValues("error").Inc()
		} else {
			c.metrics.SnapshotSendTotal.WithLabelValues("success").Inc()
		}
	}

	if lastErr != nil {
		if c.errorCollector != nil {
			c.errorCollector.Report(agenterrors.AgentError{
				Code:      agenterrors.ErrBackendUnreachable,
				Message:   fmt.Sprintf("snapshot send failed: %v", lastErr),
				Component: "transport",
				Timestamp: time.Now().UnixMilli(),
				Err:       lastErr,
			})
		}
		return nil, lastErr
	}

	// Store stats for agent health reporting.
	c.lastSendStats = SendStats{
		OriginalBytes:    originalBytes,
		CompressedBytes:  compressedBytes,
		EncodeDurationMs: encodeDurationMs,
	}

	return result, nil
}

// LastSendStats returns payload size info from the most recent successful send.
func (c *Client) LastSendStats() SendStats {
	return c.lastSendStats
}

// encodeSnapshot marshals the snapshot as JSON and wraps it in zstd.
// Returns compressed bytes plus pre-compression byte count for observability.
func (c *Client) encodeSnapshot(snapshot *model.ClusterSnapshot) ([]byte, int64, error) {
	var compressed bytes.Buffer

	zw, err := zstd.NewWriter(&compressed, zstd.WithEncoderLevel(zstd.SpeedDefault))
	if err != nil {
		return nil, 0, fmt.Errorf("transport: failed to create zstd encoder: %w", err)
	}

	// Tee through CountingWriter to capture pre-compression byte count.
	orig := NewCountingWriter(zw)

	if err := json.NewEncoder(orig).Encode(snapshot); err != nil {
		_ = zw.Close()
		return nil, 0, fmt.Errorf("transport: JSON encode failed: %w", err)
	}
	if err := zw.Close(); err != nil {
		return nil, 0, fmt.Errorf("transport: zstd close failed: %w", err)
	}

	return compressed.Bytes(), orig.Count(), nil
}

// doSend performs a single HTTP POST of the already-compressed body.
// Separated from encodeSnapshot so retries don't re-run JSON+zstd.
func (c *Client) doSend(ctx context.Context, snapshotID string, compressed []byte) (*model.SnapshotResponse, error) {
	// bytes.NewReader is an io.ReadSeeker so net/http auto-sets Content-Length
	// and can replay the body on redirect; required by the server pre-filter.
	body := bytes.NewReader(compressed)

	url := c.config.BackendURL + IngestPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, fmt.Errorf("transport: failed to create request: %w", err)
	}

	// Set explicitly so middleware that wraps the body can't drop it.
	req.ContentLength = int64(len(compressed))

	// Protocol handshake; remaining headers are observability metadata.
	req.Header.Set(ProtocolHeader, ProtocolVersion)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", ContentEncoding)
	req.Header.Set("X-Agent-Version", c.config.AgentVersion)
	req.Header.Set("X-Snapshot-ID", snapshotID)
	// Idempotency-Key lets the server dedupe retries; harmless if unsupported.
	req.Header.Set("Idempotency-Key", snapshotID)
	req.Header.Set("User-Agent", fmt.Sprintf("kubeadapt-agent/%s", c.config.AgentVersion))

	resp, err := c.httpClient.Do(req) //nolint:gosec // URL is from agent config
	if err != nil {
		return nil, fmt.Errorf("transport: HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	return ParseResponse(resp)
}

// isNonRetryableError returns true for errors where retry would fail identically
// (auth, quota, protocol, size). Retry only transient network / 5xx / 429 errors.
func isNonRetryableError(err error) bool {
	if errors.Is(err, ErrPayloadTooLarge) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "authentication failed") ||
		strings.Contains(msg, "quota exceeded") ||
		strings.Contains(msg, "agent deprecated") ||
		strings.Contains(msg, "protocol mismatch") ||
		strings.Contains(msg, "unsupported encoding") ||
		strings.Contains(msg, "length required") ||
		strings.Contains(msg, "payload too large")
}
