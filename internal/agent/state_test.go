package agent

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockClock is a controllable clock for testing.
type mockClock struct {
	mu  sync.Mutex
	now time.Time
}

func newMockClock(t time.Time) *mockClock {
	return &mockClock{now: t}
}

func (c *mockClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *mockClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func TestStateInitial(t *testing.T) {
	clk := newMockClock(time.Now())
	sm := NewStateMachine(clk)

	assert.Equal(t, StateStarting, sm.State())
	assert.Equal(t, "", sm.StateReason())
}

func TestStateTransitionToRunning(t *testing.T) {
	clk := newMockClock(time.Now())
	sm := NewStateMachine(clk)

	sm.TransitionTo(StateRunning, "informers synced")

	assert.Equal(t, StateRunning, sm.State())
	assert.Equal(t, "informers synced", sm.StateReason())
}

func TestStateHTTP200StaysRunning(t *testing.T) {
	clk := newMockClock(time.Now())
	sm := NewStateMachine(clk)
	sm.TransitionTo(StateRunning, "")

	sm.HandleHTTPStatus(200, 0)

	assert.Equal(t, StateRunning, sm.State())
	assert.Equal(t, "", sm.StateReason())
}

func TestStateHTTP401Stopped(t *testing.T) {
	clk := newMockClock(time.Now())
	sm := NewStateMachine(clk)
	sm.TransitionTo(StateRunning, "")

	sm.HandleHTTPStatus(401, 0)

	assert.Equal(t, StateStopped, sm.State())
	assert.Equal(t, "authentication failed", sm.StateReason())
}

func TestStateHTTP403Stopped(t *testing.T) {
	clk := newMockClock(time.Now())
	sm := NewStateMachine(clk)
	sm.TransitionTo(StateRunning, "")

	sm.HandleHTTPStatus(403, 0)

	assert.Equal(t, StateStopped, sm.State())
	assert.Equal(t, "authentication failed", sm.StateReason())
}

func TestStateHTTP402BackoffWithRetryAfter(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	clk := newMockClock(base)
	sm := NewStateMachine(clk)
	sm.TransitionTo(StateRunning, "")

	sm.HandleHTTPStatus(402, 120)

	assert.Equal(t, StateBackoff, sm.State())
	assert.Equal(t, "quota exceeded", sm.StateReason())

	// Backoff should be 120 seconds
	remaining := sm.BackoffRemaining()
	assert.InDelta(t, 120.0, remaining.Seconds(), 1.0)

	// Not expired yet
	assert.False(t, sm.IsBackoffExpired())

	// Advance past backoff
	clk.Advance(121 * time.Second)
	assert.True(t, sm.IsBackoffExpired())
}

func TestStateHTTP402BackoffDefaultDuration(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	clk := newMockClock(base)
	sm := NewStateMachine(clk)
	sm.TransitionTo(StateRunning, "")

	sm.HandleHTTPStatus(402, 0) // no retryAfter → default 5 min

	remaining := sm.BackoffRemaining()
	assert.InDelta(t, 300.0, remaining.Seconds(), 1.0)
}

func TestStateHTTP410ExitingCallsCancel(t *testing.T) {
	clk := newMockClock(time.Now())
	sm := NewStateMachine(clk)
	sm.TransitionTo(StateRunning, "")

	ctx, cancel := context.WithCancel(context.Background())
	sm.SetCancelFunc(cancel)

	sm.HandleHTTPStatus(410, 0)

	assert.Equal(t, StateExiting, sm.State())
	assert.Equal(t, "agent deprecated", sm.StateReason())

	// Context should be canceled
	select {
	case <-ctx.Done():
		// expected
	default:
		t.Fatal("expected context to be canceled on 410")
	}
}

func TestStateHTTP410WithoutCancelFunc(t *testing.T) {
	clk := newMockClock(time.Now())
	sm := NewStateMachine(clk)
	sm.TransitionTo(StateRunning, "")

	// No cancel func set — should not panic
	require.NotPanics(t, func() {
		sm.HandleHTTPStatus(410, 0)
	})

	assert.Equal(t, StateExiting, sm.State())
}

func TestStateHTTP429BackoffRespectsRetryAfter(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	clk := newMockClock(base)
	sm := NewStateMachine(clk)
	sm.TransitionTo(StateRunning, "")

	sm.HandleHTTPStatus(429, 60)

	assert.Equal(t, StateBackoff, sm.State())
	assert.Equal(t, "rate limited", sm.StateReason())

	remaining := sm.BackoffRemaining()
	assert.InDelta(t, 60.0, remaining.Seconds(), 1.0)
}

func TestStateHTTP429BackoffDefaultDuration(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	clk := newMockClock(base)
	sm := NewStateMachine(clk)
	sm.TransitionTo(StateRunning, "")

	sm.HandleHTTPStatus(429, 0) // no retryAfter → default 30s

	remaining := sm.BackoffRemaining()
	assert.InDelta(t, 30.0, remaining.Seconds(), 1.0)
}

func TestStateBackoffExpiry(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	clk := newMockClock(base)
	sm := NewStateMachine(clk)
	sm.TransitionTo(StateRunning, "")

	sm.HandleHTTPStatus(429, 10)

	assert.False(t, sm.IsBackoffExpired())
	assert.True(t, sm.BackoffRemaining() > 0)

	clk.Advance(5 * time.Second)
	assert.False(t, sm.IsBackoffExpired())
	assert.True(t, sm.BackoffRemaining() > 0)

	clk.Advance(6 * time.Second) // total 11s > 10s backoff
	assert.True(t, sm.IsBackoffExpired())
	assert.Equal(t, time.Duration(0), sm.BackoffRemaining())
}

func TestStateBackoffRemainingReturnsCorrectDuration(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	clk := newMockClock(base)
	sm := NewStateMachine(clk)
	sm.TransitionTo(StateRunning, "")

	sm.HandleHTTPStatus(429, 60)

	clk.Advance(20 * time.Second)
	remaining := sm.BackoffRemaining()
	assert.InDelta(t, 40.0, remaining.Seconds(), 1.0)

	clk.Advance(50 * time.Second) // past backoff
	remaining = sm.BackoffRemaining()
	assert.Equal(t, time.Duration(0), remaining)
}

func TestStateBackoffToStoppedOn401(t *testing.T) {
	clk := newMockClock(time.Now())
	sm := NewStateMachine(clk)
	sm.TransitionTo(StateRunning, "")
	sm.HandleHTTPStatus(429, 60) // enter backoff
	require.Equal(t, StateBackoff, sm.State())

	sm.HandleHTTPStatus(401, 0) // auth fail during backoff

	assert.Equal(t, StateStopped, sm.State())
}

func TestState410FromAnyState(t *testing.T) {
	states := []AgentState{StateStarting, StateRunning, StateBackoff, StateStopped}
	for _, s := range states {
		t.Run(string(s), func(t *testing.T) {
			clk := newMockClock(time.Now())
			sm := NewStateMachine(clk)
			sm.TransitionTo(s, "setup")

			sm.HandleHTTPStatus(410, 0)

			assert.Equal(t, StateExiting, sm.State())
		})
	}
}

func TestState5xxKeepsRunning(t *testing.T) {
	clk := newMockClock(time.Now())
	sm := NewStateMachine(clk)
	sm.TransitionTo(StateRunning, "")

	sm.HandleHTTPStatus(500, 0)

	assert.Equal(t, StateRunning, sm.State())
	assert.Equal(t, "server error: 500", sm.StateReason())
}

func TestStateConcurrentHandleHTTPStatus(t *testing.T) {
	clk := newMockClock(time.Now())
	sm := NewStateMachine(clk)
	sm.TransitionTo(StateRunning, "")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sm.SetCancelFunc(cancel)

	var wg sync.WaitGroup
	codes := []int{200, 200, 429, 200, 402, 200, 200, 200, 200, 200}

	for _, code := range codes {
		wg.Add(1)
		go func(c int) {
			defer wg.Done()
			sm.HandleHTTPStatus(c, 30)
		}(code)
	}

	wg.Wait()

	// Just verify no race/panic — state is valid
	state := sm.State()
	assert.Contains(t, []AgentState{StateRunning, StateBackoff}, state)

	_ = ctx // keep linter happy
}
