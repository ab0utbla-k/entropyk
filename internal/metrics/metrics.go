package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	ExperimentsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "chaos_experiments_total",
		Help: "Total number of experiments by status.",
	}, []string{"namespace", "experiment", "status"})

	ScenariosExecutedTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "chaos_scenarios_executed_total",
		Help: "Total number of scenarios injected.",
	}, []string{"namespace", "experiment", "type"})

	PodsKilledTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "chaos_pods_killed_total",
		Help: "Total number of pods killed.",
	}, []string{"namespace", "experiment"})

	RecoveryTimeSeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "chaos_recovery_time_seconds",
		Help:    "Time for target to recover after fault injection.",
		Buckets: []float64{1, 5, 10, 15, 30, 60},
	}, []string{"namespace", "experiment", "type"})

	ExperimentDurationSeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "chaos_experiment_duration_seconds",
		Help:    "Total wall-clock duration of experiments.",
		Buckets: []float64{10, 30, 60, 120, 300, 600},
	}, []string{"namespace", "experiment"})
)

func init() {
	metrics.Registry.MustRegister(
		ExperimentsTotal,
		ScenariosExecutedTotal,
		PodsKilledTotal,
		RecoveryTimeSeconds,
		ExperimentDurationSeconds,
	)
}
