package health

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/kubeadapt/kubeadapt-agent/internal/observability"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// ReadinessChecker reports whether the agent is ready to serve traffic.
type ReadinessChecker interface {
	IsReady() bool
}

// SnapshotProvider returns the latest cluster snapshot for debugging.
type SnapshotProvider interface {
	LatestSnapshot() interface{}
}

// StoreStats returns item counts per resource type for debugging.
type StoreStats interface {
	ItemCounts() map[string]int
}

// Server exposes health, readiness, metrics, and debug endpoints.
type Server struct {
	httpServer *http.Server
	metrics    *observability.Metrics
	readiness  ReadinessChecker
	snapshot   SnapshotProvider
	store      StoreStats
	listener   net.Listener
}

// NewServer creates a new health server on the given port.
// Pass port=0 to let the OS pick a free port (useful for tests).
// When enableDebug is true, pprof and debug endpoints are registered.
func NewServer(port int, metrics *observability.Metrics, readiness ReadinessChecker, snapshot SnapshotProvider, store StoreStats, enableDebug bool) *Server {
	s := &Server{
		metrics:   metrics,
		readiness: readiness,
		snapshot:  snapshot,
		store:     store,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealthz)
	mux.HandleFunc("/readyz", s.handleReadyz)
	mux.Handle("/metrics", promhttp.HandlerFor(metrics.Registry, promhttp.HandlerOpts{}))

	if enableDebug {
		// pprof handlers — only enabled when KUBEADAPT_DEBUG_ENDPOINTS=true
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

		// debug endpoints
		mux.HandleFunc("/debug/snapshot", s.handleDebugSnapshot)
		mux.HandleFunc("/debug/store", s.handleDebugStore)
	}

	s.httpServer = &http.Server{
		Addr:           fmt.Sprintf(":%d", port),
		Handler:        mux,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   30 * time.Second,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	return s
}

// Start begins listening and serving HTTP in a background goroutine.
func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.httpServer.Addr)
	if err != nil {
		return fmt.Errorf("health server listen: %w", err)
	}
	s.listener = ln
	// Update Addr to the actual address (important when port=0).
	s.httpServer.Addr = ln.Addr().String()

	go func() {
		if err := s.httpServer.Serve(ln); err != nil && err != http.ErrServerClosed {
			_ = err // server exited with unexpected error; ignore during shutdown
		}
	}()
	return nil
}

// Stop gracefully shuts down the HTTP server.
func (s *Server) Stop(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleReadyz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ready := s.readiness.IsReady()
	if ready {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	_ = json.NewEncoder(w).Encode(map[string]bool{"ready": ready})
}

func (s *Server) handleDebugSnapshot(w http.ResponseWriter, _ *http.Request) {
	snap := s.snapshot.LatestSnapshot()
	if snap == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(snap)
}

func (s *Server) handleDebugStore(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(s.store.ItemCounts())
}
