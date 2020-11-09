// +build integration
// Copyright (C) 2017 ScyllaDB

package cluster_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Cluster controller", func() {
	Context("Cluster is scaled sequentially", func() {
		var (
			ns *corev1.Namespace
		)

		BeforeEach(func() {
			var err error
			ns, err = testEnv.CreateNamespace(ctx, "ns")
			Expect(err).To(BeNil())
		})

		AfterEach(func() {
			Expect(testEnv.Delete(ctx, ns)).To(Succeed())
		})

		It("single rack", func() {
			scylla := singleRackCluster(ns)

			Expect(testEnv.Create(ctx, scylla)).To(Succeed())
			defer testEnv.Delete(ctx, scylla)

			Expect(waitForCluster(ctx, scylla)).To(Succeed())

			// Cluster should be scaled sequentially up to 3
			for _, rack := range scylla.Spec.Datacenter.Racks {
				for _, replicas := range clusterScaleSteps(rack.Members) {
					Expect(assertRackScaled(ctx, rack, scylla, replicas)).To(Succeed())
					Expect(createFakePods(ctx, testEnv, rack, scylla)).To(Succeed())
				}
			}

			Expect(assertClusterStatusReflectsSpec(ctx, scylla)).To(Succeed())
		})
	})
})

func singleRackCluster(ns *corev1.Namespace) *scyllav1alpha1.ScyllaCluster {
	return &scyllav1alpha1.ScyllaCluster{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-",
			Namespace:    ns.Name,
		},
		Spec: scyllav1alpha1.ClusterSpec{
			Version:       "4.2.0",
			AgentVersion:  pointer.StringPtr("2.2.0"),
			DeveloperMode: true,
			Datacenter: scyllav1alpha1.DatacenterSpec{
				Name: "dc1",
				Racks: []scyllav1alpha1.RackSpec{
					{
						Name:    "rack1",
						Members: 3,
						Storage: scyllav1alpha1.StorageSpec{
							Capacity: "10M",
						},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("1"),
								corev1.ResourceMemory: resource.MustParse("200M"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("1"),
								corev1.ResourceMemory: resource.MustParse("200M"),
							},
						},
					},
				},
			},
		},
	}
}

func clusterScaleSteps(desiredNodeCount int32) []int32 {
	steps := make([]int32, desiredNodeCount+1)
	for i := range steps {
		steps[i] = int32(i)
	}
	return steps
}

func waitForCluster(ctx context.Context, cluster *scyllav1alpha1.ScyllaCluster) error {
	return wait.Poll(retryInterval, timeout, func() (bool, error) {
		err := testEnv.Get(ctx, naming.NamespacedName(cluster.Name, cluster.Namespace), cluster)
		if apierrors.IsNotFound(err) {
			return false, nil
		} else if err != nil {
			return false, err
		}

		return true, nil
	})
}

func assertClusterStatusReflectsSpec(ctx context.Context, spec *scyllav1alpha1.ScyllaCluster) error {
	return wait.Poll(retryInterval, timeout, func() (bool, error) {
		cluster := &scyllav1alpha1.ScyllaCluster{}
		if err := testEnv.Get(ctx, naming.NamespacedName(spec.Name, spec.Namespace), cluster); err != nil {
			return false, err
		}

		for _, r := range spec.Spec.Datacenter.Racks {
			status := cluster.Status.Racks[r.Name]
			if status.ReadyMembers != r.Members {
				return false, nil
			}
		}
		return true, nil
	})
}

func statefulSetOfRack(ctx context.Context, rack scyllav1alpha1.RackSpec, cluster *scyllav1alpha1.ScyllaCluster) (*appsv1.StatefulSet, error) {
	sts := &appsv1.StatefulSet{}
	return sts, wait.Poll(retryInterval, timeout, func() (bool, error) {
		err := testEnv.Get(ctx, naming.NamespacedName(naming.StatefulSetNameForRack(rack, cluster), cluster.Namespace), sts)
		if apierrors.IsNotFound(err) {
			return false, nil
		} else if err != nil {
			return false, err
		}
		return true, nil
	})
}

func assertRackScaled(ctx context.Context, rack scyllav1alpha1.RackSpec, cluster *scyllav1alpha1.ScyllaCluster, replicas int32) error {
	return wait.Poll(retryInterval, timeout, func() (bool, error) {
		sts, err := statefulSetOfRack(ctx, rack, cluster)
		if err != nil {
			return false, err
		}

		return *sts.Spec.Replicas == replicas, nil
	})
}

func createFakePods(ctx context.Context, client client.Client, rack scyllav1alpha1.RackSpec, cluster *scyllav1alpha1.ScyllaCluster) error {
	sts, err := statefulSetOfRack(ctx, rack, cluster)
	if err != nil {
		return err
	}

	podTemplate := corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "template",
			Namespace: sts.Namespace,
			Labels:    naming.RackLabels(rack, cluster),
		},
		Spec: sts.Spec.Template.Spec,
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}

	for i, c := range podTemplate.Spec.Containers {
		podTemplate.Spec.Containers[i].VolumeMounts = []corev1.VolumeMount{}
		podTemplate.Status.ContainerStatuses = append(podTemplate.Status.ContainerStatuses, corev1.ContainerStatus{
			Name:  c.Name,
			Ready: true,
		})
	}

	for i := sts.Status.Replicas; i < *sts.Spec.Replicas; i++ {
		pod := podTemplate.DeepCopy()
		pod.Name = fmt.Sprintf("%s-%d", sts.Name, i)
		pod.Spec.Hostname = pod.Name
		pod.Spec.Subdomain = cluster.Name
		if err := client.Create(ctx, pod); err != nil {
			return err
		}
	}

	sts.Status.Replicas = *sts.Spec.Replicas
	sts.Status.ReadyReplicas = *sts.Spec.Replicas
	sts.Status.ObservedGeneration = sts.Generation
	if err := client.Status().Update(ctx, sts); err != nil {
		return err
	}

	return nil
}
