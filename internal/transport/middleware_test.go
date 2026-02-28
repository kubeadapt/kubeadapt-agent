package transport

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/kubeadapt/kubeadapt-agent/pkg/model"
)

func TestWithAuth_SetsBearer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get("Authorization")
		if got != "Bearer test-token-xyz" {
			t.Errorf("expected Authorization 'Bearer test-token-xyz', got %q", got)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := &http.Client{
		Transport: WithAuth("test-token-xyz", http.DefaultTransport),
	}
	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestWithRetry_5xx_Retries(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&attempts, 1)
		if n <= 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(model.SnapshotResponse{Success: true})
	}))
	defer srv.Close()

	// Use a fast retry transport for testing (we override sleepWithBackoff indirectly
	// by having maxRetries=3 and a fast server).
	client := &http.Client{
		Transport: WithRetry(3, http.DefaultTransport),
	}
	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()

	got := atomic.LoadInt32(&attempts)
	if got < 3 {
		t.Fatalf("expected at least 3 attempts, got %d", got)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 after retries, got %d", resp.StatusCode)
	}
}

func TestWithRetry_401_NoRetry(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	client := &http.Client{
		Transport: WithRetry(3, http.DefaultTransport),
	}
	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()

	got := atomic.LoadInt32(&attempts)
	if got != 1 {
		t.Fatalf("expected exactly 1 attempt for 401, got %d", got)
	}
}

func TestParseResponse_200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(model.SnapshotResponse{
			Success:   true,
			Message:   "ok",
			ClusterID: "cluster-1",
		})
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	result, err := ParseResponse(resp)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if !result.Success {
		t.Fatal("expected Success=true")
	}
	if result.ClusterID != "cluster-1" {
		t.Fatalf("expected ClusterID 'cluster-1', got %q", result.ClusterID)
	}
}

func TestParseResponse_401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	_, err = ParseResponse(resp)
	if err == nil {
		t.Fatal("expected error for 401")
	}
}

func TestParseResponse_402_QuotaExceeded(t *testing.T) {
	retryAfter := 60
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired)
		json.NewEncoder(w).Encode(model.SnapshotErrorResponse{
			Success:           false,
			Error:             "quota_exceeded",
			Message:           "CPU quota exceeded",
			RetryAfterSeconds: &retryAfter,
		})
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	_, err = ParseResponse(resp)
	if err == nil {
		t.Fatal("expected error for 402")
	}
	// Should contain retry info.
	if got := err.Error(); got == "" {
		t.Fatal("expected non-empty error message")
	}
}

func TestParseResponse_410_Deprecated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusGone)
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	_, err = ParseResponse(resp)
	if err == nil {
		t.Fatal("expected error for 410")
	}
}

func TestParseResponse_5xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	_, err = ParseResponse(resp)
	if err == nil {
		t.Fatal("expected error for 500")
	}
}
