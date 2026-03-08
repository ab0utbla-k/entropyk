package controller

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	entropykiov1alpha1 "github.com/ab0utbla-k/entropyk/api/v1alpha1"
)

var _ = Describe("ChaosExperiment Controller", func() {
	It("should add finalizer on creation", func() {
		dep := createDeployment(ctx, "dep-finalizer", "default", 1)
		createRunningPods(ctx, dep)
		exp := createExperiment(ctx, "exp-finalizer", "default", dep.Name, 30*time.Second)

		Eventually(func(g Gomega) {
			var got entropykiov1alpha1.ChaosExperiment
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(exp), &got)).To(Succeed())
			g.Expect(controllerutil.ContainsFinalizer(&got, experimentFinalizer)).To(BeTrue())
		}, timeout, interval).Should(Succeed())
	})

	It("should run pod-kill and complete", func() {
		dep := createDeployment(ctx, "dep-happy", "default", 3)
		createRunningPods(ctx, dep)
		exp := createExperiment(ctx, "exp-happy", "default", dep.Name, 5*time.Second)

		Eventually(func(g Gomega) {
			var got entropykiov1alpha1.ChaosExperiment
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(exp), &got)).To(Succeed())
			g.Expect(got.Status.Phase).To(Equal(entropykiov1alpha1.ExperimentPhaseRunning))
		}, timeout, interval).Should(Succeed())

		patchDeploymentAvailable(ctx, dep.Name, dep.Namespace)

		Eventually(func(g Gomega) {
			var got entropykiov1alpha1.ChaosExperiment
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(exp), &got)).To(Succeed())
			g.Expect(got.Status.Phase).To(Equal(entropykiov1alpha1.ExperimentPhaseCompleted))
			g.Expect(got.Status.Metrics).NotTo(BeNil())
			g.Expect(got.Status.Metrics.TotalPodsKilled).To(BeNumerically(">", 0))
		}, 20*time.Second, interval).Should(Succeed())
	})

	It("should revert on deletion while running", func() {
		dep := createDeployment(ctx, "dep-delete", "default", 1)
		createRunningPods(ctx, dep)
		exp := createExperiment(ctx, "exp-delete", "default", dep.Name, 30*time.Second)

		Eventually(func(g Gomega) {
			var got entropykiov1alpha1.ChaosExperiment
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(exp), &got)).To(Succeed())
			g.Expect(got.Status.Phase).To(Equal(entropykiov1alpha1.ExperimentPhaseRunning))
		}, timeout, interval).Should(Succeed())

		Expect(k8sClient.Delete(ctx, exp)).To(Succeed())

		Eventually(func(g Gomega) {
			var got entropykiov1alpha1.ChaosExperiment
			err := k8sClient.Get(ctx, client.ObjectKeyFromObject(exp), &got)
			g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
		}, timeout, interval).Should(Succeed())
	})

	It("should fail when target deployment doesn't exist", func() {
		exp := createExperiment(ctx, "exp-no-target", "default", "nonexistent", 5*time.Second)

		Eventually(func(g Gomega) {
			var got entropykiov1alpha1.ChaosExperiment
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(exp), &got)).To(Succeed())
			g.Expect(got.Status.Phase).To(Equal(entropykiov1alpha1.ExperimentPhaseFailed))
		}, timeout, interval).Should(Succeed())
	})
})
