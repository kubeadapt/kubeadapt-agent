package helpers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/kubeadapt/kubeadapt-agent/pkg/model"
)

// SnapshotClient interacts with the agent's debug endpoints.
type SnapshotClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewSnapshotClient creates a client for the agent's debug endpoints.
func NewSnapshotClient(t *testing.T, baseURL string) *SnapshotClient {
	t.Helper()
	return &SnapshotClient{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// GetSnapshot fetches the latest snapshot from /debug/snapshot.
func (c *SnapshotClient) GetSnapshot() (*model.ClusterSnapshot, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/debug/snapshot")
	if err != nil {
		return nil, fmt.Errorf("GET /debug/snapshot: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return nil, fmt.Errorf("no snapshot available yet (204)")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET /debug/snapshot returned %d", resp.StatusCode)
	}

	var snapshot model.ClusterSnapshot
	if err := json.NewDecoder(resp.Body).Decode(&snapshot); err != nil {
		return nil, fmt.Errorf("decode snapshot: %w", err)
	}

	return &snapshot, nil
}

// GetStoreCounts fetches item counts per resource type from /debug/store.
func (c *SnapshotClient) GetStoreCounts() (map[string]int, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/debug/store")
	if err != nil {
		return nil, fmt.Errorf("GET /debug/store: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET /debug/store returned %d", resp.StatusCode)
	}

	var counts map[string]int
	if err := json.NewDecoder(resp.Body).Decode(&counts); err != nil {
		return nil, fmt.Errorf("decode store counts: %w", err)
	}

	return counts, nil
}

// WaitForSnapshot polls /debug/snapshot until a snapshot with data is available.
func (c *SnapshotClient) WaitForSnapshot(t *testing.T, timeout time.Duration) (*model.ClusterSnapshot, error) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	var lastErr error
	for {
		select {
		case <-ticker.C:
			if time.Now().After(deadline) {
				return nil, fmt.Errorf("timeout waiting for snapshot: last error: %v", lastErr)
			}

			snap, err := c.GetSnapshot()
			if err != nil {
				lastErr = err
				t.Logf("  snapshot not ready: %v", err)
				continue
			}

			if snap.Summary.NodeCount > 0 {
				t.Logf("✓ Snapshot available: %d nodes, %d pods", snap.Summary.NodeCount, snap.Summary.PodCount)
				return snap, nil
			}

			t.Log("  snapshot exists but has 0 nodes, waiting...")
		}
	}
}

// CheckReadyz checks if the agent is ready via /readyz.
func (c *SnapshotClient) CheckReadyz() (bool, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/readyz")
	if err != nil {
		return false, fmt.Errorf("GET /readyz: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Ready bool `json:"ready"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, fmt.Errorf("decode readyz: %w", err)
	}

	return result.Ready, nil
}

// CheckHealthz checks if the agent is healthy via /healthz.
func (c *SnapshotClient) CheckHealthz() error {
	resp, err := c.httpClient.Get(c.baseURL + "/healthz")
	if err != nil {
		return fmt.Errorf("GET /healthz: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GET /healthz returned %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
