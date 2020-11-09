// Copyright (C) 2017 ScyllaDB

package integration

import (
	"context"
	"fmt"
	"time"

	"github.com/onsi/ginkgo"
	"github.com/scylladb/go-log"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type statefulSetOperatorStub struct {
	t        ginkgo.GinkgoTInterface
	env      *TestEnvironment
	interval time.Duration
	logger   log.Logger
}

func NewStatefulSetOperatorStub(t ginkgo.GinkgoTInterface, env *TestEnvironment, interval time.Duration) *statefulSetOperatorStub {
	return &statefulSetOperatorStub{
		t:        t,
		env:      env,
		interval: interval,
		logger:   env.logger.Named("sts_stub"),
	}
}

func (s *statefulSetOperatorStub) Start(ctx context.Context, name, namespace string) {
	go func() {
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				cluster := &scyllav1alpha1.ScyllaCluster{}
				if err := s.env.Get(ctx, client.ObjectKey{
					Name:      name,
					Namespace: namespace,
				}, cluster); err != nil {
					s.t.Errorf("refresh scylla cluster obj, err: %s", err)
					continue
				}

				if err := s.syncStatefulSet(ctx, cluster); err != nil {
					s.t.Errorf("sync statefulset, err: %s", err)
				}
			}
		}
	}()
}

func (s *statefulSetOperatorStub) syncStatefulSet(ctx context.Context, cluster *scyllav1alpha1.ScyllaCluster) error {
	stss := &appsv1.StatefulSetList{}
	err := s.env.List(ctx, stss, &client.ListOptions{Namespace: cluster.Namespace})
	if err != nil {
		return err
	}

	for _, sts := range stss.Items {
		var rack scyllav1alpha1.RackSpec
		for _, r := range cluster.Spec.Datacenter.Racks {
			if sts.Name == naming.StatefulSetNameForRack(r, cluster) {
				rack = r
				break
			}
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
			s.logger.Info(ctx, "Spawning fake Pod", "sts", sts.Name, "pod", pod.Name)
			if err := s.env.Create(ctx, pod); err != nil {
				return err
			}
		}

		sts.Status.Replicas = *sts.Spec.Replicas
		sts.Status.ReadyReplicas = *sts.Spec.Replicas
		sts.Status.ObservedGeneration = sts.Generation
		s.logger.Info(ctx, "Updating StatefulSet status", "replicas", sts.Status.Replicas, "observed_generation", sts.Status.ObservedGeneration)
		if err := s.env.Status().Update(ctx, &sts); err != nil {
			return err
		}
	}

	return nil
}
