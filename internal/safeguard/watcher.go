package safeguard

import (
	"context"
	"fmt"
	"time"

	temperv1alpha1 "github.com/ab0utbla-k/temper/api/v1alpha1"
	"github.com/ab0utbla-k/temper/internal/metrics"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type Watcher struct {
	client client.Client
	// recorder is held for future use; the watcher will emit events when
	// transient safeguard checks fail.
	recorder            events.EventRecorder
	consecutiveFailures map[string]int
	failureThreshold    int
	newAlertChecker     func(string) (AlertChecker, error)
	newMetricsQuerier   func(string) (MetricsQuerier, error)
}

func NewWatcher(c client.Client, rec events.EventRecorder) *Watcher {
	return &Watcher{
		client:              c,
		recorder:            rec,
		consecutiveFailures: make(map[string]int),
		failureThreshold:    3,
		newAlertChecker:     func(url string) (AlertChecker, error) { return NewAlertmanagerChecker(url) },
		newMetricsQuerier:   func(url string) (MetricsQuerier, error) { return NewPrometheusQuerier(url) },
	}
}

func (w *Watcher) Start(ctx context.Context) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			w.checkAll(ctx)
		}
	}
}

func (w *Watcher) NeedLeaderElection() bool {
	return true
}

func (w *Watcher) checkAll(ctx context.Context) {
	log := logf.FromContext(ctx)
	var schedList temperv1alpha1.ChaosScheduleList
	if err := w.client.List(ctx, &schedList); err != nil {
		log.Error(err, "Failed to list ChaosSchedules")
		return
	}

	for i := range schedList.Items {
		sched := &schedList.Items[i]
		if sched.Status.Phase != temperv1alpha1.SchedulePhaseRunning || sched.Status.ActiveExperimentName == nil {
			continue
		}

		w.checkSchedule(ctx, sched)
	}
}

func (w *Watcher) checkSchedule(ctx context.Context, sched *temperv1alpha1.ChaosSchedule) {
	log := logf.FromContext(ctx)
	sg := sched.Spec.Safeguards

	if sg == nil {
		return
	}

	haltReason, checkErr := w.checkAlerts(ctx, sched.Namespace, sg)
	if haltReason == "" && checkErr == nil {
		haltReason, checkErr = w.checkSLO(ctx, sched.Namespace, sg)
	}

	key := fmt.Sprintf("%s/%s", sched.Namespace, sched.Name)
	needsReplicaCheck := sg.MinReplicasAvailable != nil || sg.MaxUnavailable != nil

	if haltReason == "" && checkErr == nil && !needsReplicaCheck {
		delete(w.consecutiveFailures, key)
		return
	}

	var exp temperv1alpha1.ChaosExperiment
	if err := w.client.Get(ctx, client.ObjectKey{
		Namespace: sched.Namespace,
		Name:      *sched.Status.ActiveExperimentName,
	}, &exp); err != nil {
		log.Error(err, "Failed to get active experiment", "experiment", *sched.Status.ActiveExperimentName)
		return
	}

	if haltReason == "" && needsReplicaCheck {
		if exp.Spec.Target.Name == nil {
			log.Info("Skipping replica check: no target name", "schedule", key)
		} else {
			metrics.SafeguardChecksTotal.WithLabelValues(sched.Namespace, metrics.SafeguardTypeReplicas).Inc()

			var dep appsv1.Deployment
			if err := w.client.Get(ctx, client.ObjectKey{
				Namespace: sched.Namespace,
				Name:      *exp.Spec.Target.Name,
			}, &dep); err != nil {
				checkErr = err
			} else {
				if sg.MinReplicasAvailable != nil && dep.Status.AvailableReplicas < *sg.MinReplicasAvailable {
					haltReason = fmt.Sprintf("Available replicas %d < minimum %d", dep.Status.AvailableReplicas, *sg.MinReplicasAvailable)
				}
				if sg.MaxUnavailable != nil && dep.Status.UnavailableReplicas > *sg.MaxUnavailable {
					haltReason = fmt.Sprintf("Unavailable replicas %d > maximum %d", dep.Status.UnavailableReplicas, *sg.MaxUnavailable)
				}
			}
		}
	}

	switch {
	case haltReason != "":
		if err := w.haltExperiment(ctx, &exp, haltReason); err != nil {
			log.Error(err, "Failed to annotate experiment for halt", "experiment", exp.Name, "reason", haltReason)
			return
		}
		log.Info("Halting experiment", "experiment", exp.Name, "schedule", key, "reason", haltReason)
		delete(w.consecutiveFailures, key)
	case checkErr != nil:
		w.consecutiveFailures[key]++
		log.Info("Safeguard check failed", "schedule", key, "consecutive", w.consecutiveFailures[key], "threshold", w.failureThreshold, "error", checkErr)

		if w.consecutiveFailures[key] >= w.failureThreshold {
			reason := fmt.Sprintf("Safeguard checks unreachable for %ds: %v", w.failureThreshold*5, checkErr)

			if err := w.haltExperiment(ctx, &exp, reason); err != nil {
				log.Error(err, "Failed to annotate experiment for halt", "experiment", exp.Name, "reason", reason)
				return
			}
			log.Info("Halting experiment", "experiment", exp.Name, "schedule", key, "reason", reason)
			delete(w.consecutiveFailures, key)
		}
	default:
		delete(w.consecutiveFailures, key)
	}
}

func (w *Watcher) checkAlerts(ctx context.Context, namespace string, sg *temperv1alpha1.Safeguards) (string, error) {
	if sg.AlertSource == nil {
		return "", nil
	}

	checker, err := w.newAlertChecker(sg.AlertSource.URL)
	if err != nil {
		return fmt.Sprintf("Invalid alert source config: %v", err), nil
	}

	metrics.SafeguardChecksTotal.WithLabelValues(namespace, metrics.SafeguardTypeAlerts).Inc()

	return CheckAlertsFiring(ctx, sg.HaltOnAlertLabels, checker)
}

func (w *Watcher) checkSLO(ctx context.Context, namespace string, sg *temperv1alpha1.Safeguards) (string, error) {
	if sg.MetricsSource == nil || sg.SLOProtection == nil {
		return "", nil
	}

	querier, err := w.newMetricsQuerier(sg.MetricsSource.URL)
	if err != nil {
		return fmt.Sprintf("Invalid metrics source config: %v", err), nil
	}

	metrics.SafeguardChecksTotal.WithLabelValues(namespace, metrics.SafeguardTypeSLO).Inc()

	return CheckSLOBreach(ctx, sg.SLOProtection, querier)
}

func (w *Watcher) haltExperiment(ctx context.Context, exp *temperv1alpha1.ChaosExperiment, reason string) error {
	if exp.Annotations == nil {
		exp.Annotations = make(map[string]string)
	}
	exp.Annotations[temperv1alpha1.AnnotationHaltReason] = reason

	if err := w.client.Update(ctx, exp); err != nil {
		return err
	}

	metrics.SafeguardHaltsTotal.WithLabelValues(exp.Namespace, reason).Inc()
	return nil
}
