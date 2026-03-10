package helpers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/kubeadapt/kubeadapt-agent/pkg/model"
)

// StubClient interacts with the ingestion stub's test-query endpoints.
type StubClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewStubClient creates a client for the ingestion stub.
func NewStubClient(t *testing.T, baseURL string) *StubClient {
	t.Helper()
	return &StubClient{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// PayloadCount returns the number of payloads received by the stub.
func (c *StubClient) PayloadCount() (int, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/stub/payloads/count")
	if err != nil {
		return 0, fmt.Errorf("GET /stub/payloads/count: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Count int `json:"count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("decode count: %w", err)
	}

	return result.Count, nil
}

// LatestPayload returns the most recent snapshot received by the stub.
func (c *StubClient) LatestPayload() (*model.ClusterSnapshot, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/stub/payloads/latest")
	if err != nil {
		return nil, fmt.Errorf("GET /stub/payloads/latest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return nil, fmt.Errorf("no payloads received yet")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET /stub/payloads/latest returned %d", resp.StatusCode)
	}

	var snapshot model.ClusterSnapshot
	if err := json.NewDecoder(resp.Body).Decode(&snapshot); err != nil {
		return nil, fmt.Errorf("decode latest payload: %w", err)
	}

	return &snapshot, nil
}

// WaitForPayloads polls /stub/payloads/count until at least minCount payloads are received.
func (c *StubClient) WaitForPayloads(t *testing.T, minCount int, timeout time.Duration) error {
	t.Helper()

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if time.Now().After(deadline) {
				count, _ := c.PayloadCount()
				return fmt.Errorf("timeout: only %d/%d payloads received", count, minCount)
			}

			count, err := c.PayloadCount()
			if err != nil {
				t.Logf("  stub count error: %v", err)
				continue
			}

			if count >= minCount {
				t.Logf("✓ Stub received %d payloads (min %d)", count, minCount)
				return nil
			}

			t.Logf("  stub has %d/%d payloads", count, minCount)
		}
	}
}

// Flush clears all stored payloads in the stub.
func (c *StubClient) Flush() error {
	resp, err := c.httpClient.Post(c.baseURL+"/stub/flush", "application/json", nil)
	if err != nil {
		return fmt.Errorf("POST /stub/flush: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("POST /stub/flush returned %d", resp.StatusCode)
	}

	return nil
}

// SetMode switches the stub's response mode (200, 429, 500, 503).
func (c *StubClient) SetMode(statusCode int) error {
	body, _ := json.Marshal(map[string]int{"status": statusCode})
	resp, err := c.httpClient.Post(c.baseURL+"/stub/mode", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("POST /stub/mode: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("POST /stub/mode returned %d", resp.StatusCode)
	}

	return nil
}

// GetLatestHeaders returns the HTTP headers from the most recent request to the stub.
func (c *StubClient) GetLatestHeaders() (http.Header, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/stub/headers/latest")
	if err != nil {
		return nil, fmt.Errorf("GET /stub/headers/latest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return nil, fmt.Errorf("no requests received yet")
	}

	var headers http.Header
	if err := json.NewDecoder(resp.Body).Decode(&headers); err != nil {
		return nil, fmt.Errorf("decode headers: %w", err)
	}

	return headers, nil
}
