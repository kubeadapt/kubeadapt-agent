package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/klauspost/compress/zstd"

	"github.com/kubeadapt/kubeadapt-agent/internal/config"
	agenterrors "github.com/kubeadapt/kubeadapt-agent/internal/errors"
	"github.com/kubeadapt/kubeadapt-agent/internal/observability"
	"github.com/kubeadapt/kubeadapt-agent/pkg/model"
)

func testSnapshot() *model.ClusterSnapshot {
	return &model.ClusterSnapshot{
		SnapshotID:        "snap-001",
		ClusterID:         "cluster-test",
		ClusterName:       "test-cluster",
		Timestamp:         time.Now().Unix(),
		AgentVersion:      "v2.0.0-test",
		Provider:          "aws",
		Region:            "us-east-1",
		KubernetesVersion: "1.30.0",
		Nodes: []model.NodeInfo{
			{Name: "node-1"},
		},
		Summary: model.ClusterSummary{
			NodeCount: 1,
			PodCount:  5,
		},
	}
}

func testConfig(serverURL string) *config.Config {
	return &config.Config{
		APIKey:         "test-api-key-abc",
		ClusterID:      "cluster-test",
		BackendURL:     serverURL,
		AgentVersion:   "v2.0.0-test",
		MaxRetries:     0,
		RequestTimeout: 10 * time.Second,
	}
}

// TestClient_Send_StreamingCompression verifies the body is valid zstd-compressed JSON.
func TestClient_Send_StreamingCompression(t *testing.T) {
	var receivedBody []byte
	var receivedEncoding string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedEncoding = r.Header.Get("Content-Encoding")
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("failed to read body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		receivedBody = body

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(model.SnapshotResponse{
			Success:   true,
			Message:   "ingested",
			ClusterID: "cluster-test",
		})
	}))
	defer srv.Close()

	cfg := testConfig(srv.URL)
	metrics := observability.NewMetrics()
	errCollector := agenterrors.NewErrorCollector(agenterrors.RealClock{})
	client := NewClient(cfg, metrics, errCollector)

	snapshot := testSnapshot()
	result, err := client.Send(context.Background(), snapshot)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !result.Success {
		t.Fatal("expected Success=true")
	}

	// Verify Content-Encoding was zstd.
	if receivedEncoding != "zstd" {
		t.Fatalf("expected Content-Encoding 'zstd', got %q", receivedEncoding)
	}

	// Verify body is valid zstd — decompress it.
	decoder, err := zstd.NewReader(bytes.NewReader(receivedBody))
	if err != nil {
		t.Fatalf("failed to create zstd decoder: %v", err)
	}
	defer decoder.Close()

	decompressed, err := io.ReadAll(decoder)
	if err != nil {
		t.Fatalf("failed to decompress body: %v", err)
	}

	// Verify decompressed JSON is a valid ClusterSnapshot.
	var got model.ClusterSnapshot
	if err := json.Unmarshal(decompressed, &got); err != nil {
		t.Fatalf("failed to unmarshal decompressed body: %v", err)
	}
	if got.SnapshotID != snapshot.SnapshotID {
		t.Fatalf("expected SnapshotID %q, got %q", snapshot.SnapshotID, got.SnapshotID)
	}
	if got.ClusterName != snapshot.ClusterName {
		t.Fatalf("expected ClusterName %q, got %q", snapshot.ClusterName, got.ClusterName)
	}
}

// TestClient_Send_Headers verifies all required headers are set.
func TestClient_Send_Headers(t *testing.T) {
	var headers http.Header

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(model.SnapshotResponse{Success: true})
	}))
	defer srv.Close()

	cfg := testConfig(srv.URL)
	client := NewClient(cfg, nil, nil)
	snapshot := testSnapshot()

	_, err := client.Send(context.Background(), snapshot)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	checks := map[string]string{
		"Authorization":    "Bearer test-api-key-abc",
		"Content-Type":     "application/json",
		"Content-Encoding": "zstd",
		"X-Cluster-Id":     "cluster-test",
		"X-Agent-Version":  "v2.0.0-test",
		"X-Snapshot-Id":    "snap-001",
	}
	for hdr, want := range checks {
		got := headers.Get(hdr)
		if got != want {
			t.Errorf("header %s: expected %q, got %q", hdr, want, got)
		}
	}
}

// TestClient_Send_200_ParsesResponse verifies response is parsed correctly.
func TestClient_Send_200_ParsesResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Drain the request body to prevent broken pipe.
		io.Copy(io.Discard, r.Body)
		r.Body.Close()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(model.SnapshotResponse{
			Success:     true,
			Message:     "processed",
			ClusterID:   "cluster-test",
			ReceivedAt:  1700000000,
			ProcessedAt: 1700000001,
			Quota: model.QuotaStatus{
				PlanType:      "pro",
				IsWithinQuota: true,
			},
			Directives: model.Directives{
				NextSnapshotInSeconds: 60,
				CollectVPAs:           true,
			},
		})
	}))
	defer srv.Close()

	cfg := testConfig(srv.URL)
	client := NewClient(cfg, nil, nil)

	result, err := client.Send(context.Background(), testSnapshot())
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !result.Success {
		t.Fatal("expected Success=true")
	}
	if result.Quota.PlanType != "pro" {
		t.Fatalf("expected quota plan 'pro', got %q", result.Quota.PlanType)
	}
	if result.Directives.NextSnapshotInSeconds != 60 {
		t.Fatalf("expected NextSnapshotInSeconds=60, got %d", result.Directives.NextSnapshotInSeconds)
	}
}

// TestClient_Send_401_AuthError verifies auth failure is returned as error.
func TestClient_Send_401_AuthError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		r.Body.Close()
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	cfg := testConfig(srv.URL)
	client := NewClient(cfg, nil, nil)

	_, err := client.Send(context.Background(), testSnapshot())
	if err == nil {
		t.Fatal("expected error for 401")
	}
	if !strings.Contains(err.Error(), "authentication failed") {
		t.Fatalf("expected auth failure error, got: %v", err)
	}
}

// TestClient_Send_5xx_Error verifies server errors are returned.
func TestClient_Send_5xx_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		r.Body.Close()
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cfg := testConfig(srv.URL)
	cfg.MaxRetries = 0 // No retries for this test.
	client := NewClient(cfg, nil, nil)

	_, err := client.Send(context.Background(), testSnapshot())
	if err == nil {
		t.Fatal("expected error for 500")
	}
	if !strings.Contains(err.Error(), "server error") {
		t.Fatalf("expected server error, got: %v", err)
	}
}

// TestClient_Send_ContractTest is the round-trip contract test:
// snapshot → compress → decompress → unmarshal → equals original.
func TestClient_Send_ContractTest(t *testing.T) {
	var receivedBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("failed to read body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		receivedBody = body
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(model.SnapshotResponse{Success: true})
	}))
	defer srv.Close()

	cfg := testConfig(srv.URL)
	client := NewClient(cfg, nil, nil)

	original := &model.ClusterSnapshot{
		SnapshotID:        "snap-contract",
		ClusterID:         "cluster-contract",
		ClusterName:       "contract-test",
		Timestamp:         1700000000,
		AgentVersion:      "v2.0.0",
		Provider:          "gcp",
		Region:            "europe-west1",
		KubernetesVersion: "1.31.0",
		Nodes: []model.NodeInfo{
			{Name: "node-a"},
			{Name: "node-b"},
		},
		Pods: []model.PodInfo{
			{Name: "pod-1", Namespace: "default"},
		},
		Summary: model.ClusterSummary{
			NodeCount:        2,
			PodCount:         1,
			TotalCPUCapacity: 8.0,
		},
		Health: model.AgentHealth{
			State:         "running",
			UptimeSeconds: 3600,
		},
	}

	_, err := client.Send(context.Background(), original)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Decompress.
	decoder, err := zstd.NewReader(bytes.NewReader(receivedBody))
	if err != nil {
		t.Fatalf("failed to create zstd decoder: %v", err)
	}
	defer decoder.Close()

	decompressed, err := io.ReadAll(decoder)
	if err != nil {
		t.Fatalf("failed to decompress: %v", err)
	}

	// Unmarshal and compare.
	var roundTripped model.ClusterSnapshot
	if err := json.Unmarshal(decompressed, &roundTripped); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// Compare key fields.
	if roundTripped.SnapshotID != original.SnapshotID {
		t.Errorf("SnapshotID: want %q, got %q", original.SnapshotID, roundTripped.SnapshotID)
	}
	if roundTripped.ClusterID != original.ClusterID {
		t.Errorf("ClusterID: want %q, got %q", original.ClusterID, roundTripped.ClusterID)
	}
	if roundTripped.Provider != original.Provider {
		t.Errorf("Provider: want %q, got %q", original.Provider, roundTripped.Provider)
	}
	if roundTripped.KubernetesVersion != original.KubernetesVersion {
		t.Errorf("KubernetesVersion: want %q, got %q", original.KubernetesVersion, roundTripped.KubernetesVersion)
	}
	if len(roundTripped.Nodes) != len(original.Nodes) {
		t.Errorf("Nodes: want %d, got %d", len(original.Nodes), len(roundTripped.Nodes))
	}
	if len(roundTripped.Pods) != len(original.Pods) {
		t.Errorf("Pods: want %d, got %d", len(original.Pods), len(roundTripped.Pods))
	}
	if roundTripped.Summary.NodeCount != original.Summary.NodeCount {
		t.Errorf("Summary.NodeCount: want %d, got %d", original.Summary.NodeCount, roundTripped.Summary.NodeCount)
	}
	if roundTripped.Summary.TotalCPUCapacity != original.Summary.TotalCPUCapacity {
		t.Errorf("Summary.TotalCPUCapacity: want %f, got %f", original.Summary.TotalCPUCapacity, roundTripped.Summary.TotalCPUCapacity)
	}
	if roundTripped.Health.State != original.Health.State {
		t.Errorf("Health.State: want %q, got %q", original.Health.State, roundTripped.Health.State)
	}
	if roundTripped.Health.UptimeSeconds != original.Health.UptimeSeconds {
		t.Errorf("Health.UptimeSeconds: want %d, got %d", original.Health.UptimeSeconds, roundTripped.Health.UptimeSeconds)
	}
}

// TestClient_Send_RetryCreatesFreshPipe verifies that each retry attempt creates
// a new io.Pipe and sends a valid compressed body.
func TestClient_Send_RetryCreatesFreshPipe(t *testing.T) {
	var attempts int32
	var bodySizes []int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		n := atomic.AddInt32(&attempts, 1)
		bodySizes = append(bodySizes, len(body))

		if n <= 2 {
			// First two attempts: return 503.
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		// Third attempt: verify we got a valid body.
		if len(body) == 0 {
			t.Error("retry received empty body — pipe was not re-created")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Verify body is valid zstd.
		decoder, err := zstd.NewReader(bytes.NewReader(body))
		if err != nil {
			t.Errorf("retry body is not valid zstd: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer decoder.Close()
		decompressed, err := io.ReadAll(decoder)
		if err != nil {
			t.Errorf("failed to decompress retry body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		var snap model.ClusterSnapshot
		if err := json.Unmarshal(decompressed, &snap); err != nil {
			t.Errorf("failed to unmarshal retry body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(model.SnapshotResponse{Success: true})
	}))
	defer srv.Close()

	cfg := testConfig(srv.URL)
	cfg.MaxRetries = 3
	client := NewClient(cfg, nil, nil)

	result, err := client.Send(context.Background(), testSnapshot())
	if err != nil {
		t.Fatalf("Send failed after retries: %v", err)
	}
	if !result.Success {
		t.Fatal("expected Success=true after retries")
	}

	got := atomic.LoadInt32(&attempts)
	if got != 3 {
		t.Fatalf("expected 3 attempts, got %d", got)
	}

	// Verify all attempts received non-empty bodies (fresh pipes).
	for i, size := range bodySizes {
		if size == 0 {
			t.Errorf("attempt %d received empty body", i+1)
		}
	}
}

// TestClient_Send_5xx_RetriedThenFails verifies that retries exhaust and return error.
func TestClient_Send_5xx_RetriedThenFails(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		r.Body.Close()
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	cfg := testConfig(srv.URL)
	cfg.MaxRetries = 2
	client := NewClient(cfg, nil, nil)

	_, err := client.Send(context.Background(), testSnapshot())
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}

	got := atomic.LoadInt32(&attempts)
	if got != 3 { // 1 initial + 2 retries
		t.Fatalf("expected 3 attempts (1 + 2 retries), got %d", got)
	}
}

// TestClient_Send_ContextCancellation verifies cancellation is respected.
func TestClient_Send_ContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Slow server — should be canceled.
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := testConfig(srv.URL)
	cfg.MaxRetries = 0
	client := NewClient(cfg, nil, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := client.Send(ctx, testSnapshot())
	if err == nil {
		t.Fatal("expected error from canceled context")
	}
}
