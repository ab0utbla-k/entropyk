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
	"time"

	entropykiov1alpha1 "github.com/ab0utbla-k/entropyk/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("ChaosSchedule Controller", func() {
	It("should create experiment on schedule", func() {
		dep := createDeployment(ctx, "dep-sched", "default", 3)
		createRunningPods(ctx, dep)
		exp := createExperiment(ctx, "exp-template-sched", "default", dep.Name, 30*time.Second)
		sched := &entropykiov1alpha1.ChaosSchedule{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "sched-create",
				Namespace: "default",
			},
			Spec: entropykiov1alpha1.ChaosScheduleSpec{
				ExperimentRef: exp.Name,
				Schedule:      "* * * * *",
			},
		}
		Expect(k8sClient.Create(ctx, sched)).To(Succeed())
		sched.Status.LastScheduleTime = &metav1.Time{Time: time.Now().Add(-2 * time.Minute)}
		Expect(k8sClient.Status().Update(ctx, sched)).To(Succeed())

		Eventually(func(g Gomega) {
			var got entropykiov1alpha1.ChaosSchedule
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(sched), &got)).To(Succeed())
			g.Expect(got.Status.Phase).To(Equal(entropykiov1alpha1.SchedulePhaseRunning))
			g.Expect(got.Status.ActiveExperimentName).NotTo(BeNil())
		}, timeout, interval).Should(Succeed())
	})

	It("should block when safeguards fail", func() {
		dep := createDeployment(ctx, "dep-safeguard", "default", 3)
		exp := createExperiment(ctx, "exp-template-safeguard", "default", dep.Name, 30*time.Second)
		sched := &entropykiov1alpha1.ChaosSchedule{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "sched-safeguard",
				Namespace: "default",
			},
			Spec: entropykiov1alpha1.ChaosScheduleSpec{
				ExperimentRef: exp.Name,
				Schedule:      "* * * * *",
				Safeguards: &entropykiov1alpha1.Safeguards{
					MinReplicasAvailable: new(int32(2)),
				},
			},
		}
		Expect(k8sClient.Create(ctx, sched)).To(Succeed())
		sched.Status.LastScheduleTime = &metav1.Time{Time: time.Now().Add(-2 * time.Minute)}
		Expect(k8sClient.Status().Update(ctx, sched)).To(Succeed())

		Consistently(func(g Gomega) {
			var got entropykiov1alpha1.ChaosSchedule
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(sched), &got)).To(Succeed())
			g.Expect(got.Status.ActiveExperimentName).To(BeNil())
		}, timeout, interval).Should(Succeed())
	})
})
