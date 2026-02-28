package health

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kubeadapt/kubeadapt-agent/internal/observability"
)

// --- Mock implementations ---

type mockReadiness struct {
	ready bool
}

func (m *mockReadiness) IsReady() bool { return m.ready }

type mockSnapshot struct {
	data interface{}
}

func (m *mockSnapshot) LatestSnapshot() interface{} { return m.data }

type mockStoreStats struct {
	counts map[string]int
}

func (m *mockStoreStats) ItemCounts() map[string]int { return m.counts }

// --- Helper to build a test server's mux ---

func newTestServer(ready bool, snapshot interface{}, counts map[string]int) *Server {
	metrics := observability.NewMetrics()
	r := &mockReadiness{ready: ready}
	s := &mockSnapshot{data: snapshot}
	st := &mockStoreStats{counts: counts}
	return NewServer(0, metrics, r, s, st, true) // enableDebug=true for tests that check debug endpoints
}

// --- Tests ---

func TestHealthz(t *testing.T) {
	srv := newTestServer(true, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]string
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if result["status"] != "ok" {
		t.Fatalf("expected status=ok, got %s", result["status"])
	}
}

func TestReadyzReady(t *testing.T) {
	srv := newTestServer(true, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]bool
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if !result["ready"] {
		t.Fatal("expected ready=true")
	}
}

func TestReadyzNotReady(t *testing.T) {
	srv := newTestServer(false, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]bool
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if result["ready"] {
		t.Fatal("expected ready=false")
	}
}

func TestMetrics(t *testing.T) {
	srv := newTestServer(true, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "kubeadapt_agent_") {
		t.Fatal("expected Prometheus metrics containing kubeadapt_agent_ prefix")
	}
}

func TestDebugStoreItemCounts(t *testing.T) {
	counts := map[string]int{
		"nodes": 50,
		"pods":  2000,
	}
	srv := newTestServer(true, nil, counts)
	req := httptest.NewRequest(http.MethodGet, "/debug/store", nil)
	w := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]int
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if result["nodes"] != 50 {
		t.Fatalf("expected nodes=50, got %d", result["nodes"])
	}
	if result["pods"] != 2000 {
		t.Fatalf("expected pods=2000, got %d", result["pods"])
	}
}

func TestDebugSnapshotNoData(t *testing.T) {
	srv := newTestServer(true, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/debug/snapshot", nil)
	w := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
}

func TestDebugSnapshotWithData(t *testing.T) {
	snapshot := map[string]interface{}{
		"cluster": "test-cluster",
		"nodes":   3,
	}
	srv := newTestServer(true, snapshot, nil)
	req := httptest.NewRequest(http.MethodGet, "/debug/snapshot", nil)
	w := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if result["cluster"] != "test-cluster" {
		t.Fatalf("expected cluster=test-cluster, got %v", result["cluster"])
	}
}

func TestDebugEndpointsDisabled(t *testing.T) {
	metrics := observability.NewMetrics()
	r := &mockReadiness{ready: true}
	s := &mockSnapshot{data: map[string]string{"key": "val"}}
	st := &mockStoreStats{counts: map[string]int{"nodes": 1}}

	srv := NewServer(0, metrics, r, s, st, false)

	// /debug/store should 404 when debug is disabled
	req := httptest.NewRequest(http.MethodGet, "/debug/store", nil)
	w := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(w, req)
	if w.Result().StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for /debug/store when debug disabled, got %d", w.Result().StatusCode)
	}

	// /debug/snapshot should 404 when debug is disabled
	req = httptest.NewRequest(http.MethodGet, "/debug/snapshot", nil)
	w = httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(w, req)
	if w.Result().StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for /debug/snapshot when debug disabled, got %d", w.Result().StatusCode)
	}

	// /healthz should still work
	req = httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w = httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(w, req)
	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for /healthz, got %d", w.Result().StatusCode)
	}
}

func TestServerStartStop(t *testing.T) {
	metrics := observability.NewMetrics()
	r := &mockReadiness{ready: true}
	s := &mockSnapshot{}
	st := &mockStoreStats{counts: map[string]int{}}

	srv := NewServer(0, metrics, r, s, st, false)

	if err := srv.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}

	// Give server a moment to start
	time.Sleep(50 * time.Millisecond)

	// Verify server is responding
	addr := srv.httpServer.Addr
	resp, err := http.Get("http://" + addr + "/healthz")
	if err != nil {
		t.Fatalf("failed to reach server: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Stop(ctx); err != nil {
		t.Fatalf("failed to stop server: %v", err)
	}
}
