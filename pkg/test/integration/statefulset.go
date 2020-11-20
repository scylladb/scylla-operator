// Copyright (C) 2017 ScyllaDB

package integration

import (
	"context"
	"fmt"

	"github.com/scylladb/go-log"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type StatefulSetOperatorStub struct {
	env    *TestEnvironment
	logger log.Logger

	stopCh chan struct{}
}

func NewStatefulSetOperatorStub(env *TestEnvironment) *StatefulSetOperatorStub {

	return &StatefulSetOperatorStub{
		env:    env,
		logger: env.logger.Named("sts_stub"),
	}
}

func WithPodCondition(condition corev1.PodCondition) func(pod *corev1.Pod) {
	return func(pod *corev1.Pod) {
		for i, pc := range pod.Status.Conditions {
			if pc.Type == condition.Type {
				pod.Status.Conditions[i].Status = condition.Status
				return
			}
		}
		pod.Status.Conditions = append(pod.Status.Conditions, condition)
	}

}

type PodOption func(pod *corev1.Pod)

func (s *StatefulSetOperatorStub) CreatePods(ctx context.Context, cluster *scyllav1alpha1.ScyllaCluster, options ...PodOption) error {
	for _, rack := range cluster.Spec.Datacenter.Racks {
		sts := &appsv1.StatefulSet{}

		err := s.env.Get(ctx, client.ObjectKey{
			Name:      naming.StatefulSetNameForRack(rack, cluster),
			Namespace: cluster.Namespace,
		}, sts)
		if err != nil {
			return err
		}

		podTemplate := &corev1.Pod{
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

		for _, opt := range options {
			opt(podTemplate)
		}

		mutateFn := func() error {
			return nil
		}

		for i := 0; i < int(*sts.Spec.Replicas); i++ {
			pod := podTemplate.DeepCopy()
			pod.Name = fmt.Sprintf("%s-%d", sts.Name, i)
			pod.Spec.Hostname = pod.Name
			pod.Spec.Subdomain = cluster.Name

			if op, err := controllerutil.CreateOrUpdate(ctx, s.env, pod, mutateFn); err != nil {
				return err
			} else {
				switch op {
				case controllerutil.OperationResultCreated:
					s.logger.Info(ctx, "Spawned fake Pod", "sts", sts.Name, "pod", pod.Name)
				case controllerutil.OperationResultUpdated:
					s.logger.Info(ctx, "Updated fake Pod", "sts", sts.Name, "pod", pod.Name)
				}
			}
		}

		sts.Status.Replicas = *sts.Spec.Replicas
		sts.Status.ReadyReplicas = *sts.Spec.Replicas
		sts.Status.ObservedGeneration = sts.Generation
		s.logger.Info(ctx, "Updating StatefulSet status", "replicas", sts.Status.Replicas, "observed_generation", sts.Status.ObservedGeneration)
		if err := s.env.Status().Update(ctx, sts); err != nil {
			return err
		}
	}

	return nil
}

func (s *StatefulSetOperatorStub) SyncStatus(ctx context.Context, cluster *scyllav1alpha1.ScyllaCluster) error {
	for _, rack := range cluster.Spec.Datacenter.Racks {
		sts := &appsv1.StatefulSet{}

		err := s.env.Get(ctx, client.ObjectKey{
			Name:      naming.StatefulSetNameForRack(rack, cluster),
			Namespace: cluster.Namespace,
		}, sts)
		if err != nil {
			return err
		}

		pods := &corev1.PodList{}
		if err := s.env.List(ctx, pods, &client.ListOptions{
			Namespace:     cluster.Namespace,
			LabelSelector: naming.RackSelector(rack, cluster),
		}); err != nil {
			return err
		}

		sts.Status.Replicas = int32(len(pods.Items))
		sts.Status.ReadyReplicas = int32(len(pods.Items))
		sts.Status.ObservedGeneration = sts.Generation
		s.logger.Info(ctx, "Updating StatefulSet status", "replicas", sts.Status.Replicas, "observed_generation", sts.Status.ObservedGeneration)
		if err := s.env.Status().Update(ctx, sts); err != nil {
			return err
		}
	}

	return nil
}
