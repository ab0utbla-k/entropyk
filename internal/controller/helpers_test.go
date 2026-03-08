package controller

import (
	"context"
	"fmt"
	"time"

	entropykiov1alpha1 "github.com/ab0utbla-k/entropyk/api/v1alpha1"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	timeout  = 10 * time.Second
	interval = 250 * time.Millisecond
)

func createDeployment(ctx context.Context, name, namespace string, replicas int) *appsv1.Deployment { //nolint:unparam // may vary
	labels := map[string]string{"app": name}
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: new(int32(replicas)),
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "test", Image: "busybox"},
					},
				},
			},
		},
	}

	Expect(k8sClient.Create(ctx, dep)).To(Succeed())

	return dep
}

func createRunningPods(ctx context.Context, dep *appsv1.Deployment) {
	for i := range *dep.Spec.Replicas {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-%d", dep.Name, i),
				Namespace: dep.Namespace,
				Labels:    dep.Spec.Template.Labels,
			},
			Spec: dep.Spec.Template.Spec,
		}
		Expect(k8sClient.Create(ctx, pod)).To(Succeed())

		pod.Status.Phase = corev1.PodRunning
		Expect(k8sClient.Status().Update(ctx, pod)).To(Succeed())
	}
}

func patchDeploymentAvailable(ctx context.Context, name, namespace string) {
	var dep appsv1.Deployment
	Expect(k8sClient.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, &dep)).To(Succeed())

	dep.Status.Conditions = append(dep.Status.Conditions, appsv1.DeploymentCondition{
		Type:   appsv1.DeploymentAvailable,
		Status: corev1.ConditionTrue,
	})

	Expect(k8sClient.Status().Update(ctx, &dep)).To(Succeed())
}

func createExperiment(
	ctx context.Context,
	name, namespace, targetDeployment string, //nolint:unparam // may vary
	duration time.Duration,
) *entropykiov1alpha1.ChaosExperiment {
	exp := &entropykiov1alpha1.ChaosExperiment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: entropykiov1alpha1.ChaosExperimentSpec{
			Target: entropykiov1alpha1.Target{
				Kind: "Deployment",
				Name: new(targetDeployment),
			},
			Scenarios: []entropykiov1alpha1.Scenario{
				{
					Type:     entropykiov1alpha1.ScenarioTypePodKill,
					Duration: metav1.Duration{Duration: duration},
				},
			},
		},
	}

	Expect(k8sClient.Create(ctx, exp)).To(Succeed())

	return exp
}
