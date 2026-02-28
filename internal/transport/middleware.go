package transport

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/kubeadapt/kubeadapt-agent/pkg/model"
)

// authTransport adds an Authorization: Bearer header to every request.
type authTransport struct {
	token string
	next  http.RoundTripper
}

// WithAuth wraps a RoundTripper with bearer-token authorization.
func WithAuth(token string, next http.RoundTripper) http.RoundTripper {
	return &authTransport{token: token, next: next}
}

func (a *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("Authorization", "Bearer "+a.token)
	return a.next.RoundTrip(req)
}

// loggingTransport logs request method/URL and response status.
type loggingTransport struct {
	logger *slog.Logger
	next   http.RoundTripper
}

// WithLogging wraps a RoundTripper with request/response logging.
func WithLogging(logger *slog.Logger, next http.RoundTripper) http.RoundTripper {
	return &loggingTransport{logger: logger, next: next}
}

func (l *loggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()
	resp, err := l.next.RoundTrip(req)
	elapsed := time.Since(start)

	if err != nil {
		l.logger.Error("HTTP request failed",
			"method", req.Method,
			"url", req.URL.String(),
			"duration_ms", elapsed.Milliseconds(),
			"error", err,
		)
		return resp, err
	}

	l.logger.Info("HTTP request completed",
		"method", req.Method,
		"url", req.URL.String(),
		"status", resp.StatusCode,
		"duration_ms", elapsed.Milliseconds(),
	)
	return resp, nil
}

// retryTransport retries on 5xx and 429 errors with exponential backoff.
// It does NOT retry on 401/403 (auth failures).
type retryTransport struct {
	maxRetries int
	next       http.RoundTripper
}

// WithRetry wraps a RoundTripper with retry logic for transient errors.
func WithRetry(maxRetries int, next http.RoundTripper) http.RoundTripper {
	return &retryTransport{maxRetries: maxRetries, next: next}
}

func (r *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error

	for attempt := 0; attempt <= r.maxRetries; attempt++ {
		resp, err = r.next.RoundTrip(req)
		if err != nil {
			// Network error — retry.
			if attempt < r.maxRetries {
				sleepWithBackoff(attempt)
				continue
			}
			return nil, err
		}

		// Success or client error that shouldn't be retried.
		if resp.StatusCode < 500 && resp.StatusCode != 429 {
			return resp, nil
		}

		// 429 Too Many Requests — respect Retry-After or body retry_after_seconds.
		if resp.StatusCode == 429 {
			delay := retryAfterDelay(resp)
			if attempt < r.maxRetries {
				drainAndClose(resp.Body)
				time.Sleep(delay)
				continue
			}
			return resp, nil
		}

		// 5xx — retry with backoff.
		if attempt < r.maxRetries {
			drainAndClose(resp.Body)
			sleepWithBackoff(attempt)
			continue
		}
	}

	return resp, err
}

// sleepWithBackoff sleeps for exponential backoff: 1s * 2^attempt.
func sleepWithBackoff(attempt int) {
	d := time.Duration(math.Pow(2, float64(attempt))) * time.Second
	time.Sleep(d)
}

// retryAfterDelay extracts the delay from a 429 response.
// It checks the Retry-After header first, then falls back to
// parsing the response body for retry_after_seconds.
func retryAfterDelay(resp *http.Response) time.Duration {
	const defaultDelay = 5 * time.Second

	// Check Retry-After header (seconds).
	if ra := resp.Header.Get("Retry-After"); ra != "" {
		if secs, err := strconv.Atoi(ra); err == nil && secs > 0 {
			return time.Duration(secs) * time.Second
		}
	}

	// Try parsing body for retry_after_seconds.
	if resp.Body != nil {
		var errResp model.SnapshotErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err == nil {
			if errResp.RetryAfterSeconds != nil && *errResp.RetryAfterSeconds > 0 {
				return time.Duration(*errResp.RetryAfterSeconds) * time.Second
			}
		}
	}

	return defaultDelay
}

// drainAndClose reads remaining body bytes and closes, preventing connection leaks.
func drainAndClose(body io.ReadCloser) {
	if body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, body)
	body.Close()
}

// ParseResponse reads an HTTP response and returns the appropriate result or error.
func ParseResponse(resp *http.Response) (*model.SnapshotResponse, error) {
	defer drainAndClose(resp.Body)

	switch {
	case resp.StatusCode == http.StatusOK:
		var result model.SnapshotResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("transport: failed to decode 200 response: %w", err)
		}
		return &result, nil

	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return nil, fmt.Errorf("transport: authentication failed (HTTP %d)", resp.StatusCode)

	case resp.StatusCode == http.StatusPaymentRequired:
		var errResp model.SnapshotErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err == nil {
			msg := errResp.Message
			if errResp.RetryAfterSeconds != nil {
				msg = fmt.Sprintf("%s (retry after %ds)", msg, *errResp.RetryAfterSeconds)
			}
			return nil, fmt.Errorf("transport: quota exceeded: %s", msg)
		}
		return nil, fmt.Errorf("transport: quota exceeded (HTTP 402)")

	case resp.StatusCode == http.StatusGone:
		return nil, fmt.Errorf("transport: agent deprecated (HTTP 410)")

	case resp.StatusCode == http.StatusTooManyRequests:
		return nil, fmt.Errorf("transport: rate limited (HTTP 429)")

	case resp.StatusCode >= 500:
		return nil, fmt.Errorf("transport: server error (HTTP %d)", resp.StatusCode)

	default:
		return nil, fmt.Errorf("transport: unexpected status (HTTP %d)", resp.StatusCode)
	}
}
