package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"

	"github.com/kubeadapt/kubeadapt-agent/internal/config"
	agenterrors "github.com/kubeadapt/kubeadapt-agent/internal/errors"
	"github.com/kubeadapt/kubeadapt-agent/internal/observability"
	"github.com/kubeadapt/kubeadapt-agent/pkg/model"
)

// Client sends ClusterSnapshots to the backend over HTTP with streaming
// zstd compression. It never buffers the full JSON payload in memory.
type Client struct {
	httpClient     *http.Client
	config         *config.Config
	metrics        *observability.Metrics
	errorCollector *agenterrors.ErrorCollector
}

// NewClient creates a transport Client with middleware applied.
// Retry is handled at the Send level (not the RoundTripper) because
// the streaming io.Pipe body must be re-created on each attempt.
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

	return &Client{
		httpClient: &http.Client{
			Timeout:   cfg.RequestTimeout,
			Transport: transport,
		},
		config:         cfg,
		metrics:        metrics,
		errorCollector: errCollector,
	}
}

// Send streams a ClusterSnapshot to the backend using io.Pipe + zstd compression.
// It re-creates the io.Pipe on each retry attempt since a pipe can only be consumed once.
func (c *Client) Send(ctx context.Context, snapshot *model.ClusterSnapshot) (*model.SnapshotResponse, error) {
	start := time.Now()

	var result *model.SnapshotResponse
	var compressedBytes int64
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

		resp, bytes, err := c.doSend(ctx, snapshot)
		compressedBytes = bytes
		if err != nil {
			lastErr = err
			// Don't retry auth failures or non-retryable errors.
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

	return result, nil
}

// doSend performs a single HTTP POST with streaming compression.
// Each call creates a fresh io.Pipe so it can be called multiple times for retries.
func (c *Client) doSend(ctx context.Context, snapshot *model.ClusterSnapshot) (*model.SnapshotResponse, int64, error) {
	pr, pw := io.Pipe()

	// CountingWriter wraps the pipe writer to track compressed bytes.
	cw := NewCountingWriter(pw)

	// Create zstd encoder writing to the counting writer.
	zw, err := zstd.NewWriter(cw, zstd.WithEncoderLevel(zstd.SpeedDefault))
	if err != nil {
		_ = pw.Close()
		return nil, 0, fmt.Errorf("transport: failed to create zstd encoder: %w", err)
	}

	// Goroutine: encode JSON → zstd → pipe.
	go func() {
		encodeErr := json.NewEncoder(zw).Encode(snapshot)
		// Close zstd first to flush, then close the pipe.
		closeErr := zw.Close()
		if encodeErr != nil {
			pw.CloseWithError(fmt.Errorf("transport: JSON encode failed: %w", encodeErr))
		} else if closeErr != nil {
			pw.CloseWithError(fmt.Errorf("transport: zstd close failed: %w", closeErr))
		} else {
			_ = pw.Close()
		}
	}()

	// Build the request reading from the pipe.
	url := c.config.BackendURL + "/api/v1/metrics/ingest"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, pr)
	if err != nil {
		_ = pr.Close()
		return nil, 0, fmt.Errorf("transport: failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "zstd")
	req.Header.Set("X-Cluster-ID", c.config.ClusterID)
	req.Header.Set("X-Agent-Version", c.config.AgentVersion)
	req.Header.Set("X-Snapshot-ID", snapshot.SnapshotID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, cw.Count(), fmt.Errorf("transport: HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	result, err := ParseResponse(resp)
	if err != nil {
		return nil, cw.Count(), err
	}

	return result, cw.Count(), nil
}

// isNonRetryableError checks if an error should not be retried.
func isNonRetryableError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "authentication failed") ||
		strings.Contains(msg, "quota exceeded") ||
		strings.Contains(msg, "agent deprecated")
}
