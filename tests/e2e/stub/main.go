// Package main implements a minimal HTTP stub that mimics ingestion-api for E2E tests.
// It accepts agent snapshot payloads (zstd-compressed JSON), stores them in memory,
// and exposes test-query endpoints for assertions.
//
// Inspired by Datadog's fakeintake pattern.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/klauspost/compress/zstd"
)

// Stub holds received payloads and current failure mode.
type Stub struct {
	mu       sync.Mutex
	payloads []json.RawMessage // raw JSON snapshots after decompression
	mode     int               // HTTP status code to return (200 = normal)
	headers  []http.Header     // headers from each request (for auth assertion)
}

func main() {
	s := &Stub{mode: http.StatusOK}

	mux := http.NewServeMux()

	// Agent's target endpoint
	mux.HandleFunc("/api/v1/metrics/ingest", s.handleIngest)

	// Test-query endpoints
	mux.HandleFunc("/stub/payloads", s.handleGetPayloads)
	mux.HandleFunc("/stub/payloads/latest", s.handleGetLatest)
	mux.HandleFunc("/stub/payloads/count", s.handleGetCount)
	mux.HandleFunc("/stub/flush", s.handleFlush)
	mux.HandleFunc("/stub/mode", s.handleMode)
	mux.HandleFunc("/stub/headers/latest", s.handleGetLatestHeaders)

	// Health check
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"status":"ok"}`)
	})

	srv := &http.Server{
		Addr:           ":8080",
		Handler:        mux,
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	log.Println("ingestion-stub listening on :8080")
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

// handleIngest accepts POST from the agent, decompresses zstd body, stores raw JSON.
func (s *Stub) handleIngest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check failure mode
	s.mu.Lock()
	mode := s.mode
	s.mu.Unlock()

	if mode != http.StatusOK {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(mode)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("stub returning %d", mode),
			"message": fmt.Sprintf("stub failure mode %d active", mode),
		})
		return
	}

	// Decompress body
	var reader io.Reader
	if r.Header.Get("Content-Encoding") == "zstd" {
		zr, err := zstd.NewReader(r.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf("zstd init: %v", err), http.StatusBadRequest)
			return
		}
		defer zr.Close()
		reader = zr
	} else {
		reader = r.Body
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		http.Error(w, fmt.Sprintf("read body: %v", err), http.StatusBadRequest)
		return
	}

	// Validate it's valid JSON
	if !json.Valid(data) {
		http.Error(w, "body is not valid JSON after decompression", http.StatusBadRequest)
		return
	}

	// Store payload and headers
	s.mu.Lock()
	s.payloads = append(s.payloads, json.RawMessage(data))
	s.headers = append(s.headers, r.Header.Clone())
	s.mu.Unlock()

	log.Printf("received snapshot #%d (%d bytes, snapshot_id=%s)",
		len(s.payloads), len(data), r.Header.Get("X-Snapshot-Id"))

	// Return valid SnapshotResponse
	now := time.Now().UnixMilli()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":      true,
		"message":      "ingested",
		"cluster_id":   "e2e-test-cluster",
		"received_at":  now,
		"processed_at": now,
		"quota": map[string]interface{}{
			"plan_type":         "unlimited",
			"cpu_limit":         999999,
			"current_cpu_usage": 0,
			"remaining_cpu":     999999,
			"is_within_quota":   true,
			"cluster_cpu":       0,
		},
		"directives": map[string]interface{}{
			"next_snapshot_in_seconds": 10,
			"collect_vpas":             false,
			"collect_karpenter":        false,
		},
	})
}

// handleGetPayloads returns all received snapshots as a JSON array.
func (s *Stub) handleGetPayloads(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	payloads := make([]json.RawMessage, len(s.payloads))
	copy(payloads, s.payloads)
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payloads)
}

// handleGetLatest returns the most recent snapshot.
func (s *Stub) handleGetLatest(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.payloads) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(s.payloads[len(s.payloads)-1])
}

// handleGetCount returns the number of received payloads.
func (s *Stub) handleGetCount(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	count := len(s.payloads)
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]int{"count": count})
}

// handleFlush clears all stored payloads.
func (s *Stub) handleFlush(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.mu.Lock()
	s.payloads = nil
	s.headers = nil
	s.mu.Unlock()

	log.Println("flushed all payloads")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, `{"flushed":true}`)
}

// handleMode switches the failure mode. POST with {"status": 200|429|500|503}.
func (s *Stub) handleMode(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		s.mu.Lock()
		mode := s.mode
		s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]int{"status": mode})
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		Status int `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, fmt.Sprintf("decode: %v", err), http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	s.mode = body.Status
	s.mu.Unlock()

	log.Printf("mode switched to %d", body.Status)
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]int{"status": body.Status})
}

// handleGetLatestHeaders returns headers from the most recent request.
func (s *Stub) handleGetLatestHeaders(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.headers) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	latest := s.headers[len(s.headers)-1]
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(latest)
}
