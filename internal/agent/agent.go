package agent

import (
	"context"
	stderrors "errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/kubeadapt/kubeadapt-agent/internal/collector"
	"github.com/kubeadapt/kubeadapt-agent/internal/config"
	"github.com/kubeadapt/kubeadapt-agent/internal/errors"
	"github.com/kubeadapt/kubeadapt-agent/internal/observability"
	"github.com/kubeadapt/kubeadapt-agent/internal/snapshot"
	"github.com/kubeadapt/kubeadapt-agent/internal/transport"
	"github.com/kubeadapt/kubeadapt-agent/pkg/model"
)

// Agent is the main orchestrator that wires together all subsystems and runs
// the snapshot-send loop.
type Agent struct {
	config         *config.Config
	registry       *collector.Registry
	builder        *snapshot.SnapshotBuilder
	transport      *transport.Client
	stateMachine   *StateMachine
	errorCollector *errors.ErrorCollector
	metrics        *observability.Metrics

	latestSnapshot atomic.Pointer[model.ClusterSnapshot]
	ready          atomic.Bool
	startedAt      time.Time
}

// NewAgent creates an Agent with all required dependencies.
func NewAgent(
	cfg *config.Config,
	registry *collector.Registry,
	builder *snapshot.SnapshotBuilder,
	transport *transport.Client,
	stateMachine *StateMachine,
	errCollector *errors.ErrorCollector,
	metrics *observability.Metrics,
) *Agent {
	return &Agent{
		config:         cfg,
		registry:       registry,
		builder:        builder,
		transport:      transport,
		stateMachine:   stateMachine,
		errorCollector: errCollector,
		metrics:        metrics,
		startedAt:      time.Now(),
	}
}

// IsReady reports whether the agent has completed initial sync and is
// actively collecting data. Implements health.ReadinessChecker.
func (a *Agent) IsReady() bool {
	return a.ready.Load()
}

// LatestSnapshot returns the most recent ClusterSnapshot, or nil if none
// has been built yet. Implements health.SnapshotProvider.
func (a *Agent) LatestSnapshot() interface{} {
	snap := a.latestSnapshot.Load()
	if snap == nil {
		return nil
	}
	return snap
}

// Run executes the agent lifecycle: start collectors, wait for sync,
// then enter the snapshot-send loop until the context is canceled or
// the state machine transitions to a terminal state.
func (a *Agent) Run(ctx context.Context) error {
	// Wire the cancel func into the state machine so 410 can trigger exit.
	a.stateMachine.SetCancelFunc(func() {
		// The parent context cancel is handled by the caller.
		// The state machine sets StateExiting which the loop detects.
	})

	// 1. Start all collectors.
	if err := a.registry.StartAll(ctx); err != nil {
		var partial *collector.PartialStartError
		if stderrors.As(err, &partial) {
			slog.Warn("some collectors failed to start, continuing with partial data",
				"failed", partial.Failed, "total", partial.Total)
		} else {
			return fmt.Errorf("failed to start collectors: %w", err)
		}
	}
	defer a.registry.StopAll()

	// 2. Wait for initial sync (with configurable timeout).
	syncTimeout := a.config.InformerSyncTimeout
	if syncTimeout == 0 {
		syncTimeout = 5 * time.Minute
	}
	slog.Info("waiting for informer sync", "timeout", syncTimeout)

	syncCtx, syncCancel := context.WithTimeout(ctx, syncTimeout)
	defer syncCancel()
	syncStart := time.Now()
	if err := a.registry.WaitForSync(syncCtx); err != nil {
		a.errorCollector.Report(errors.AgentError{
			Code:      errors.ErrInformerSyncTimeout,
			Message:   fmt.Sprintf("informer sync timed out after %s: %v", syncTimeout, err),
			Component: "agent",
			Timestamp: time.Now().UnixMilli(),
			Err:       err,
		})
		slog.Warn("informer sync incomplete, continuing with partial data",
			"error", err,
			"timeout", syncTimeout,
			"elapsed", time.Since(syncStart).Round(time.Millisecond),
		)
		// Continue — partial data is better than no data.
	} else {
		slog.Info("informer sync completed",
			"elapsed", time.Since(syncStart).Round(time.Millisecond),
		)
	}

	// 2b. Log post-sync store diagnostics so operators can verify counts.
	a.logStoreCounts(ctx)

	// 3. Transition to Running.
	a.stateMachine.TransitionTo(StateRunning, "informers synced")
	a.ready.Store(true)
	slog.Info("agent is ready", "state", StateRunning)

	// 4. Main loop.
	ticker := time.NewTicker(a.config.SnapshotInterval)
	defer ticker.Stop()

	// Do first snapshot immediately.
	a.doSnapshot(ctx)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}

		state := a.stateMachine.State()
		switch state {
		case StateRunning:
			a.doSnapshot(ctx)
		case StateBackoff:
			if a.stateMachine.IsBackoffExpired() {
				a.stateMachine.TransitionTo(StateRunning, "backoff expired")
				a.doSnapshot(ctx)
			} else {
				slog.Debug("in backoff, skipping snapshot",
					"remaining", a.stateMachine.BackoffRemaining())
			}
		case StateStopped, StateExiting:
			slog.Info("agent exiting", "state", state,
				"reason", a.stateMachine.StateReason())
			return nil
		}

		if s := a.stateMachine.State(); s == StateStopped || s == StateExiting {
			slog.Info("agent exiting", "state", s,
				"reason", a.stateMachine.StateReason())
			return nil
		}
	}
}

func (a *Agent) logStoreCounts(ctx context.Context) {
	snap := a.builder.Build(ctx)
	slog.Info("post-sync store counts",
		"nodes", len(snap.Nodes),
		"pods", len(snap.Pods),
		"namespaces", len(snap.Namespaces),
		"deployments", len(snap.Deployments),
		"statefulsets", len(snap.StatefulSets),
		"daemonsets", len(snap.DaemonSets),
		"jobs", len(snap.Jobs),
		"cronjobs", len(snap.CronJobs),
		"hpas", len(snap.HPAs),
		"services", len(snap.Services),
		"ingresses", len(snap.Ingresses),
		"pvs", len(snap.PVs),
		"pvcs", len(snap.PVCs),
	)
}

func (a *Agent) doSnapshot(ctx context.Context) {
	snap := a.builder.Build(ctx)
	a.latestSnapshot.Store(snap)

	resp, err := a.transport.Send(ctx, snap)
	if err != nil {
		slog.Error("snapshot send failed", "error", err)
		return
	}

	state := a.stateMachine.State()
	if state == StateStopped || state == StateExiting {
		return
	}

	a.stateMachine.HandleHTTPStatus(200, 0)

	if resp != nil {
		slog.Info("snapshot sent successfully",
			"snapshot_id", snap.SnapshotID,
			"quota_plan", resp.Quota.PlanType,
			"within_quota", resp.Quota.IsWithinQuota,
		)
	}
}
