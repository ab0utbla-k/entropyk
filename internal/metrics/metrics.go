package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	SafeguardTypeAlerts   = "alerts"
	SafeguardTypeSLO      = "slo"
	SafeguardTypeReplicas = "replicas"
)

var (
	ExperimentsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "chaos_experiments_total",
		Help: "Total number of experiments by status.",
	}, []string{"namespace", "source", "status"})

	ScenariosExecutedTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "chaos_scenarios_executed_total",
		Help: "Total number of scenarios injected.",
	}, []string{"namespace", "source", "type"})

	PodsKilledTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "chaos_pods_killed_total",
		Help: "Total number of pods killed.",
	}, []string{"namespace", "source"})

	RecoveryTimeSeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "chaos_recovery_time_seconds",
		Help:    "Time for target to recover after fault injection.",
		Buckets: []float64{1, 5, 10, 15, 30, 60},
	}, []string{"namespace", "source", "type"})

	ExperimentDurationSeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "chaos_experiment_duration_seconds",
		Help:    "Total wall-clock duration of experiments.",
		Buckets: []float64{10, 30, 60, 120, 300, 600},
	}, []string{"namespace", "source"})

	SafeguardChecksTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "chaos_safeguard_checks_total",
		Help: "Total number of safeguard checks executed, by type.",
	}, []string{"namespace", "type"})

	SafeguardHaltsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "chaos_safeguard_halts_total",
		Help: "Total number of times the safeguard watcher halted an experiment, by code.",
	}, []string{"namespace", "code"})

	ExperimentsHaltedTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "chaos_experiments_halted_total",
		Help: "Total number of experiments transitioned to the Halted phase, by code.",
	}, []string{"namespace", "source", "code"})
)

func init() {
	metrics.Registry.MustRegister(
		ExperimentsTotal,
		ScenariosExecutedTotal,
		PodsKilledTotal,
		RecoveryTimeSeconds,
		ExperimentDurationSeconds,
		SafeguardChecksTotal,
		SafeguardHaltsTotal,
		ExperimentsHaltedTotal,
	)
}
