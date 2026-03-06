package scenario

import (
	"context"
	"fmt"
	"math/rand/v2"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// PodKill deletes random pods from the target workload.
type PodKill struct {
	Client client.Client
	Count  int32
}

func (p *PodKill) Inject(ctx context.Context, target Target) error {
	if target.Name == "" {
		return fmt.Errorf("pod-kill scenario requires a named target (label selector not yet supported)")
	}

	var dep appsv1.Deployment
	if err := p.Client.Get(ctx, client.ObjectKey{
		Name:      target.Name,
		Namespace: target.Namespace,
	}, &dep); err != nil {
		return fmt.Errorf("get deployment: %w", err)
	}

	selector, err := metav1.LabelSelectorAsSelector(dep.Spec.Selector)
	if err != nil {
		return fmt.Errorf("parse selector: %w", err)
	}

	var podList corev1.PodList
	if err = p.Client.List(ctx, &podList,
		client.InNamespace(target.Namespace),
		client.MatchingLabelsSelector{Selector: selector},
	); err != nil {
		return fmt.Errorf("list pods: %w", err)
	}

	var running []corev1.Pod
	for _, pod := range podList.Items {
		if pod.Status.Phase == corev1.PodRunning {
			running = append(running, pod)
		}
	}

	if len(running) == 0 {
		return fmt.Errorf("no running pods found for deployment %s/%s", target.Namespace, target.Name)
	}

	count := int(p.Count)
	if count <= 0 {
		return fmt.Errorf("count must be at least 1, got %d", p.Count)
	}
	if count > len(running) {
		count = len(running)
	}

	rand.Shuffle(len(running), func(i, j int) {
		running[i], running[j] = running[j], running[i]
	})

	for _, pod := range running[:count] {
		if err := p.Client.Delete(ctx, &pod); err != nil {
			return fmt.Errorf("delete pod %s: %w", pod.Name, err)
		}
	}

	return nil
}

func (p *PodKill) Revert(_ context.Context, _ Target) error {
	// Pod-kill is self-reverting — Kubernetes replaces deleted pods automatically.
	return nil
}

func (p *PodKill) RecoveryProbe() RecoveryProbe {
	return RecoveryProbe{
		Condition: &ConditionProbe{
			Type:   "Available",
			Status: metav1.ConditionTrue,
		},
	}
}
