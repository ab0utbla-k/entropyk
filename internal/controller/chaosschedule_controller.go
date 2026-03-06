/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	entropykiov1alpha1 "github.com/ab0utbla-k/entropyk/api/v1alpha1"
)

// ChaosScheduleReconciler reconciles a ChaosSchedule object
type ChaosScheduleReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder events.EventRecorder
}

// +kubebuilder:rbac:groups=entropyk.io,resources=chaosschedules,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=entropyk.io,resources=chaosschedules/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=entropyk.io,resources=chaosschedules/finalizers,verbs=update
// +kubebuilder:rbac:groups=entropyk.io,resources=chaosexperiments,verbs=get;list;watch;create;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.23.1/pkg/reconcile
func (r *ChaosScheduleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var sched entropykiov1alpha1.ChaosSchedule
	if err := r.Get(ctx, req.NamespacedName, &sched); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get ChaosSchedule: %w", err)
	}
	// If suspended, just set phase and stop.
	if sched.Spec.Suspend {
		if sched.Status.Phase != entropykiov1alpha1.SchedulePhasePaused {
			sched.Status.Phase = entropykiov1alpha1.SchedulePhasePaused
			if err := r.Status().Update(ctx, &sched); err != nil {
				return ctrl.Result{}, fmt.Errorf("update status to Paused: %w", err)
			}
			r.Recorder.Eventf(&sched, nil, "Normal", "Paused", "Paused", "Schedule suspended")
		}
		return ctrl.Result{}, nil
	}

	// Phase dispatch.
	switch sched.Status.Phase {
	case "", entropykiov1alpha1.SchedulePhaseIdle:
		return r.reconcileIdle(ctx, &sched)
	case entropykiov1alpha1.SchedulePhaseRunning:
		return r.reconcileRunning(ctx, &sched)
	case entropykiov1alpha1.SchedulePhasePaused:
		// Was paused, now unsuspended — go back to Idle.
		sched.Status.Phase = entropykiov1alpha1.SchedulePhaseIdle
		if err := r.Status().Update(ctx, &sched); err != nil {
			return ctrl.Result{}, fmt.Errorf("update status to Idle: %w", err)
		}
		r.Recorder.Eventf(&sched, nil, "Normal", "Resumed", "Resumed", "Schedule resumed")
		return ctrl.Result{RequeueAfter: time.Second}, nil
	case entropykiov1alpha1.SchedulePhaseHalted, entropykiov1alpha1.SchedulePhaseCompleted, entropykiov1alpha1.SchedulePhaseFailed:
		return ctrl.Result{}, nil
	default:
		log.Info("Unknown phase", "phase", sched.Status.Phase)
		return ctrl.Result{}, nil
	}
}

func (r *ChaosScheduleReconciler) reconcileIdle(ctx context.Context, sched *entropykiov1alpha1.ChaosSchedule) (ctrl.Result, error) {
	loc, err := time.LoadLocation(sched.Spec.Timezone)
	if err != nil {
		return r.failSchedule(ctx, sched, fmt.Sprintf("Invalid timezone %q: %v", sched.Spec.Timezone, err))
	}

	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	cronSched, err := parser.Parse(sched.Spec.Schedule)
	if err != nil {
		return r.failSchedule(ctx, sched, fmt.Sprintf("Invalid cron expression %q: %v", sched.Spec.Schedule, err))
	}

	now := time.Now().In(loc)
	var lastRun time.Time
	if sched.Status.LastScheduleTime != nil {
		lastRun = sched.Status.LastScheduleTime.Time
	} else {
		lastRun = sched.CreationTimestamp.Time
	}

	nextRun := cronSched.Next(lastRun)
	if now.Before(nextRun) {
		// Not time yet — requeue at next fire time.
		return ctrl.Result{RequeueAfter: nextRun.Sub(now)}, nil
	}
	// Time to fire — create a ChaosExperiment.
	return r.createExperiment(ctx, sched)
}

func (r *ChaosScheduleReconciler) reconcileRunning(ctx context.Context, sched *entropykiov1alpha1.ChaosSchedule) (ctrl.Result, error) {
	if sched.Status.ActiveExperimentName == nil {
		// Shouldn't happen — recover by going back to Idle.
		log := logf.FromContext(ctx)
		log.Info("Running phase with no active experiment, recovering to Idle")
		sched.Status.Phase = entropykiov1alpha1.SchedulePhaseIdle
		if err := r.Status().Update(ctx, sched); err != nil {
			return ctrl.Result{}, fmt.Errorf("update status to Idle: %w", err)
		}
		return ctrl.Result{RequeueAfter: time.Second}, nil
	}

	var exp entropykiov1alpha1.ChaosExperiment
	if err := r.Get(ctx, client.ObjectKey{
		Namespace: sched.Namespace,
		Name:      *sched.Status.ActiveExperimentName,
	}, &exp); err != nil {
		if apierrors.IsNotFound(err) {
			// Experiment was deleted externally — go back to Idle.
			sched.Status.Phase = entropykiov1alpha1.SchedulePhaseIdle
			sched.Status.ActiveExperimentName = nil
			if err := r.Status().Update(ctx, sched); err != nil {
				return ctrl.Result{}, fmt.Errorf("update status to Idle: %w", err)
			}
			return ctrl.Result{RequeueAfter: time.Second}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get active experiment: %w", err)
	}

	switch exp.Status.Phase {
	case entropykiov1alpha1.ExperimentPhaseCompleted:
		sched.Status.Phase = entropykiov1alpha1.SchedulePhaseIdle
		sched.Status.ActiveExperimentName = nil
		sched.Status.History.SuccessfulRuns++
		if err := r.Status().Update(ctx, sched); err != nil {
			return ctrl.Result{}, fmt.Errorf("update status after experiment completed: %w", err)
		}
		r.Recorder.Eventf(sched, &exp, "Normal", "ExperimentCompleted", "ExperimentCompleted",
			"Experiment %s completed", exp.Name)
		return ctrl.Result{RequeueAfter: time.Second}, nil
	case entropykiov1alpha1.ExperimentPhaseFailed:
		sched.Status.Phase = entropykiov1alpha1.SchedulePhaseIdle
		sched.Status.ActiveExperimentName = nil
		if err := r.Status().Update(ctx, sched); err != nil {
			return ctrl.Result{}, fmt.Errorf("update status after experiment failed: %w", err)
		}
		r.Recorder.Eventf(sched, &exp, "Warning", "ExperimentFailed", "ExperimentFailed",
			"Experiment %s failed", exp.Name)
		return ctrl.Result{RequeueAfter: time.Second}, nil
	default:
		// Still running — wait for Owns() watch to notify us.
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}
}

func (r *ChaosScheduleReconciler) failSchedule(ctx context.Context, sched *entropykiov1alpha1.ChaosSchedule, reason string) (ctrl.Result, error) {
	sched.Status.Phase = entropykiov1alpha1.SchedulePhaseFailed
	if err := r.Status().Update(ctx, sched); err != nil {
		return ctrl.Result{}, fmt.Errorf("update status to Failed: %w", err)
	}
	r.Recorder.Eventf(sched, nil, "Warning", "Failed", "Failed", reason)
	return ctrl.Result{}, nil
}

func (r *ChaosScheduleReconciler) createExperiment(ctx context.Context, sched *entropykiov1alpha1.ChaosSchedule) (ctrl.Result, error) {
	var template entropykiov1alpha1.ChaosExperiment

	if err := r.Get(ctx, client.ObjectKey{
		Namespace: sched.Namespace,
		Name:      sched.Spec.ExperimentRef,
	}, &template); err != nil {
		return r.failSchedule(ctx, sched, fmt.Sprintf("Get experiment template %q: %v", sched.Spec.ExperimentRef, err))
	}

	// Create a new experiment from the template.
	expName := fmt.Sprintf("%s-%d", sched.Name, time.Now().Unix())

	exp := &entropykiov1alpha1.ChaosExperiment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: sched.Namespace,
			Name:      expName,
		},
		Spec: template.Spec,
	}
	// Owner reference — schedule owns the experiment.
	if err := controllerutil.SetControllerReference(sched, exp, r.Scheme); err != nil {
		return ctrl.Result{}, fmt.Errorf("set owner reference: %w", err)
	}

	if err := r.Create(ctx, exp); err != nil {
		return r.failSchedule(ctx, sched, fmt.Sprintf("create experiment: %v", err))
	}

	// Update schedule status.
	sched.Status.Phase = entropykiov1alpha1.SchedulePhaseRunning
	sched.Status.ActiveExperimentName = new(expName)
	sched.Status.LastScheduleTime = new(metav1.Now())
	sched.Status.History.TotalRuns++

	if err := r.Status().Update(ctx, sched); err != nil {
		return ctrl.Result{}, fmt.Errorf("update status after creating experiment: %w", err)
	}
	r.Recorder.Eventf(sched, exp, "Normal", "ExperimentCreated", "ExperimentCreated",
		"Created experiment %s", expName)

	return ctrl.Result{RequeueAfter: time.Second}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ChaosScheduleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&entropykiov1alpha1.ChaosSchedule{}).
		Owns(&entropykiov1alpha1.ChaosExperiment{}).
		Named("chaosschedule").
		Complete(r)
}
