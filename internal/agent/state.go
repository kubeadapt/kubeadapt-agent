package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/kubeadapt/kubeadapt-agent/internal/errors"
)

// AgentState represents the current lifecycle state of the agent.
type AgentState string

// Agent lifecycle states.
const (
	StateStarting AgentState = "starting"
	StateRunning  AgentState = "running"
	StateBackoff  AgentState = "backoff"
	StateStopped  AgentState = "stopped"
	StateExiting  AgentState = "exiting"
)

// StateMachine tracks the agent's lifecycle state and handles
// transitions driven by HTTP response codes from the backend.
type StateMachine struct {
	mu           sync.RWMutex
	state        AgentState
	stateReason  string
	backoffUntil time.Time
	clock        errors.Clock
	cancelFunc   context.CancelFunc // set externally, called on StateExiting
}

// NewStateMachine creates a StateMachine starting in StateStarting.
func NewStateMachine(clock errors.Clock) *StateMachine {
	return &StateMachine{
		state: StateStarting,
		clock: clock,
	}
}

// State returns the current agent state.
func (sm *StateMachine) State() AgentState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.state
}

// StateReason returns the human-readable reason for the current state.
func (sm *StateMachine) StateReason() string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.stateReason
}

// SetCancelFunc registers the context cancel function called on StateExiting.
func (sm *StateMachine) SetCancelFunc(cancel context.CancelFunc) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.cancelFunc = cancel
}

// TransitionTo directly sets the agent state with a reason.
func (sm *StateMachine) TransitionTo(state AgentState, reason string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.state = state
	sm.stateReason = reason
}

// HandleHTTPStatus transitions state based on the HTTP status code
// returned by the backend API.
func (sm *StateMachine) HandleHTTPStatus(statusCode int, retryAfterSeconds int) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	switch {
	case statusCode == 200:
		sm.state = StateRunning
		sm.stateReason = ""
	case statusCode == 401 || statusCode == 403:
		sm.state = StateStopped
		sm.stateReason = "authentication failed"
	case statusCode == 402:
		sm.state = StateBackoff
		sm.stateReason = "quota exceeded"
		backoff := time.Duration(retryAfterSeconds) * time.Second
		if backoff == 0 {
			backoff = 5 * time.Minute
		}
		sm.backoffUntil = sm.clock.Now().Add(backoff)
	case statusCode == 410:
		sm.state = StateExiting
		sm.stateReason = "agent deprecated"
		if sm.cancelFunc != nil {
			sm.cancelFunc()
		}
	case statusCode == 429:
		sm.state = StateBackoff
		sm.stateReason = "rate limited"
		backoff := time.Duration(retryAfterSeconds) * time.Second
		if backoff == 0 {
			backoff = 30 * time.Second
		}
		sm.backoffUntil = sm.clock.Now().Add(backoff)
	case statusCode >= 500:
		// 5xx errors are handled by transport retry, state stays unchanged.
		// Only record the reason for observability.
		sm.stateReason = fmt.Sprintf("server error: %d", statusCode)
	}
}

// IsBackoffExpired returns true if the backoff period has elapsed.
func (sm *StateMachine) IsBackoffExpired() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.clock.Now().After(sm.backoffUntil)
}

// BackoffRemaining returns the duration until backoff expires, or 0 if expired.
func (sm *StateMachine) BackoffRemaining() time.Duration {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	remaining := sm.backoffUntil.Sub(sm.clock.Now())
	if remaining < 0 {
		return 0
	}
	return remaining
}
