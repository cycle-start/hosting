package agent

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/rs/zerolog"

	"github.com/edvin/hosting/internal/model"
)

// DriftEvent represents a detected drift.
type DriftEvent struct {
	Timestamp time.Time `json:"timestamp"`
	NodeID    string    `json:"node_id"`
	Kind      string    `json:"kind"`
	Resource  string    `json:"resource"`
	Action    string    `json:"action"`
	Detail    string    `json:"detail"`
}

// Reconciler manages the periodic reconciliation loop.
type Reconciler struct {
	logger   zerolog.Logger
	nodeID   string
	nodeRole string
	client   *APIClient
	server   *Server

	interval         time.Duration
	maxFixes         int
	circuitThreshold int

	// Circuit breaker state
	consecutiveHighDrift int
	circuitOpen          bool

	// Per-resource mutex to prevent conflicts with Temporal activities
	locks sync.Map

	// Metrics
	reconcileDuration  prometheus.Histogram
	reconcileTotal     *prometheus.CounterVec
	driftDetected      *prometheus.CounterVec
	driftFixed         *prometheus.CounterVec
	circuitBreakerOpen prometheus.Gauge

	// Last desired state (for when API returns 304)
	lastDesiredState *model.DesiredState
}

// NewReconciler creates a new reconciler.
func NewReconciler(
	logger zerolog.Logger,
	nodeID, nodeRole string,
	client *APIClient,
	server *Server,
	interval time.Duration,
	maxFixes, circuitThreshold int,
) *Reconciler {
	r := &Reconciler{
		logger:           logger.With().Str("component", "reconciler").Logger(),
		nodeID:           nodeID,
		nodeRole:         nodeRole,
		client:           client,
		server:           server,
		interval:         interval,
		maxFixes:         maxFixes,
		circuitThreshold: circuitThreshold,
	}

	r.reconcileDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "node_agent_reconcile_duration_seconds",
		Help:    "Duration of each reconciliation cycle",
		Buckets: prometheus.DefBuckets,
	})
	r.reconcileTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "node_agent_reconcile_total",
		Help: "Total reconciliation cycles",
	}, []string{"result"})
	r.driftDetected = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "node_agent_drift_detected_total",
		Help: "Total drift events detected",
	}, []string{"kind", "action"})
	r.driftFixed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "node_agent_drift_fixed_total",
		Help: "Total drift events auto-fixed",
	}, []string{"kind"})
	r.circuitBreakerOpen = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "node_agent_circuit_breaker_open",
		Help: "1 if circuit breaker is open",
	})

	return r
}

// LockResource acquires a per-resource mutex. Returns an unlock function.
func (r *Reconciler) LockResource(kind, tenant, resource string) func() {
	key := kind + ":" + tenant + ":" + resource
	mu, _ := r.locks.LoadOrStore(key, &sync.Mutex{})
	mu.(*sync.Mutex).Lock()
	return mu.(*sync.Mutex).Unlock
}

// FullReconcile performs a complete reconciliation cycle.
func (r *Reconciler) FullReconcile(ctx context.Context) error {
	start := time.Now()
	r.logger.Info().Msg("starting reconciliation cycle")

	// Fetch desired state.
	ds, err := r.client.GetDesiredState(ctx, r.nodeID)
	if err != nil {
		r.reconcileTotal.WithLabelValues("failure").Inc()
		return fmt.Errorf("fetch desired state: %w", err)
	}
	if ds == nil {
		// 304 Not Modified -- use last known state.
		ds = r.lastDesiredState
	}
	if ds == nil {
		r.logger.Warn().Msg("no desired state available, skipping reconciliation")
		return nil
	}
	r.lastDesiredState = ds

	var events []DriftEvent

	// Check circuit breaker.
	if r.circuitOpen {
		r.logger.Warn().Msg("circuit breaker is open, running in report-only mode")
	}

	// Dispatch to role-specific reconciler.
	switch r.nodeRole {
	case "web":
		events, err = r.reconcileWeb(ctx, ds)
	case "database":
		events, err = r.reconcileDatabase(ctx, ds)
	case "valkey":
		events, err = r.reconcileValkey(ctx, ds)
	case "lb":
		events, err = r.reconcileLB(ctx, ds)
	case "s3":
		events, err = r.reconcileStorage(ctx, ds)
	default:
		r.logger.Info().Str("role", r.nodeRole).Msg("no reconciliation needed for this role")
	}

	duration := time.Since(start).Seconds()
	r.reconcileDuration.Observe(duration)

	if err != nil {
		r.reconcileTotal.WithLabelValues("failure").Inc()
		r.logger.Error().Err(err).Float64("duration_s", duration).Msg("reconciliation failed")
	} else {
		r.reconcileTotal.WithLabelValues("success").Inc()
		r.logger.Info().
			Int("drift_events", len(events)).
			Float64("duration_s", duration).
			Msg("reconciliation completed")
	}

	// Update metrics.
	for _, e := range events {
		r.driftDetected.WithLabelValues(e.Kind, e.Action).Inc()
		if e.Action == "auto_fixed" {
			r.driftFixed.WithLabelValues(e.Kind).Inc()
		}
	}

	// Update circuit breaker.
	r.updateCircuitBreaker(len(events))

	// Report drift events to core-api (best effort).
	if len(events) > 0 {
		if reportErr := r.client.ReportDriftEvents(ctx, r.nodeID, events); reportErr != nil {
			r.logger.Warn().Err(reportErr).Msg("failed to report drift events")
		}
	}

	return err
}

func (r *Reconciler) updateCircuitBreaker(driftCount int) {
	if driftCount > r.circuitThreshold {
		r.consecutiveHighDrift++
		if r.consecutiveHighDrift >= 3 {
			r.circuitOpen = true
			r.circuitBreakerOpen.Set(1)
			r.logger.Error().
				Int("consecutive_cycles", r.consecutiveHighDrift).
				Int("threshold", r.circuitThreshold).
				Msg("circuit breaker OPEN -- switching to report-only mode")
		}
	} else {
		r.consecutiveHighDrift = 0
		if r.circuitOpen {
			r.circuitOpen = false
			r.circuitBreakerOpen.Set(0)
			r.logger.Info().Msg("circuit breaker CLOSED -- resuming auto-fix")
		}
	}
}

// RunLoop runs the periodic reconciliation loop.
func (r *Reconciler) RunLoop(ctx context.Context) {
	// Add jitter (0-30s) to spread load across nodes.
	jitter := time.Duration(rand.Int63n(int64(30 * time.Second)))
	r.logger.Info().
		Dur("interval", r.interval).
		Dur("jitter", jitter).
		Msg("starting reconciliation loop")

	time.Sleep(jitter)

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			r.logger.Info().Msg("reconciliation loop stopped")
			return
		case <-ticker.C:
			if err := r.FullReconcile(ctx); err != nil {
				r.logger.Error().Err(err).Msg("periodic reconciliation failed")
			}
		}
	}
}
