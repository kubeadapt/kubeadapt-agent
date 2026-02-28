package errors

import (
	"sync"
	"time"
)

// Code represents a typed error code understood by the backend.
type Code string

// Agent error codes reported to the backend.
const (
	ErrMetricsUnavailable  Code = "METRICS_UNAVAILABLE"
	ErrInformerSyncFailed  Code = "INFORMER_SYNC_FAILED"
	ErrInformerSyncTimeout Code = "INFORMER_SYNC_TIMEOUT"
	ErrBackendUnreachable  Code = "BACKEND_UNREACHABLE"
	ErrSnapshotBuildFailed Code = "SNAPSHOT_BUILD_FAILED"
	ErrCompressionFailed   Code = "COMPRESSION_FAILED"
	ErrAuthFailed          Code = "AUTH_FAILED"
	ErrBufferFull          Code = "BUFFER_FULL"
	ErrCRDNotFound         Code = "CRD_NOT_FOUND"
	ErrDiscoveryFailed     Code = "DISCOVERY_FAILED"
	ErrTimeout             Code = "TIMEOUT"
	ErrPartialData         Code = "PARTIAL_DATA"
)

// defaultTTL is the auto-expiry duration for errors not re-reported.
const defaultTTL = 5 * time.Minute

// Clock abstracts time for testability.
type Clock interface {
	Now() time.Time
}

// RealClock uses the system clock.
type RealClock struct{}

// Now returns the current time.
func (RealClock) Now() time.Time { return time.Now() }

// AgentError represents a typed agent error with code, component, and optional wrapped error.
type AgentError struct {
	Code      Code   `json:"code"`
	Message   string `json:"message"`
	Component string `json:"component"`
	Timestamp int64  `json:"timestamp"`
	Err       error  `json:"-"`
}

// Error implements the error interface.
func (e *AgentError) Error() string {
	return e.Message
}

// Unwrap returns the wrapped error for errors.Is/As compatibility.
func (e *AgentError) Unwrap() error {
	return e.Err
}

// entry wraps an AgentError with its last-reported time for expiry tracking.
type entry struct {
	err        AgentError
	lastReport time.Time
}

// ErrorCollector is a thread-safe store for active agent errors.
// Errors are keyed by Code+Component and auto-expire after 5 minutes
// if not re-reported.
type ErrorCollector struct {
	mu      sync.Mutex
	clock   Clock
	entries map[string]entry // key = string(Code) + "|" + Component
}

// NewErrorCollector creates an ErrorCollector with the given clock.
func NewErrorCollector(clock Clock) *ErrorCollector {
	return &ErrorCollector{
		clock:   clock,
		entries: make(map[string]entry),
	}
}

// key builds the dedup key for an error.
func key(code Code, component string) string {
	return string(code) + "|" + component
}

// Report stores or refreshes an error. The dedup key is Code+Component.
func (ec *ErrorCollector) Report(err AgentError) {
	ec.mu.Lock()
	defer ec.mu.Unlock()

	k := key(err.Code, err.Component)
	ec.entries[k] = entry{
		err:        err,
		lastReport: ec.clock.Now(),
	}
}

// GetActiveErrors returns all errors that have been reported within the TTL window.
func (ec *ErrorCollector) GetActiveErrors() []AgentError {
	ec.mu.Lock()
	defer ec.mu.Unlock()

	now := ec.clock.Now()
	result := make([]AgentError, 0, len(ec.entries))
	for k, e := range ec.entries {
		if now.Sub(e.lastReport) > defaultTTL {
			delete(ec.entries, k)
			continue
		}
		result = append(result, e.err)
	}
	return result
}

// GetActiveErrorCodes returns a deduplicated list of active error codes.
func (ec *ErrorCollector) GetActiveErrorCodes() []string {
	ec.mu.Lock()
	defer ec.mu.Unlock()

	now := ec.clock.Now()
	seen := make(map[Code]struct{})
	codes := make([]string, 0)
	for k, e := range ec.entries {
		if now.Sub(e.lastReport) > defaultTTL {
			delete(ec.entries, k)
			continue
		}
		if _, ok := seen[e.err.Code]; !ok {
			seen[e.err.Code] = struct{}{}
			codes = append(codes, string(e.err.Code))
		}
	}
	return codes
}

// Clear removes all tracked errors.
func (ec *ErrorCollector) Clear() {
	ec.mu.Lock()
	defer ec.mu.Unlock()

	ec.entries = make(map[string]entry)
}
