// Copyright (C) 2017 ScyllaDB

package integration

import (
	"context"
	"fmt"
	"reflect"

	"github.com/scylladb/go-log"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
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

func (s *StatefulSetOperatorStub) CreatePods(ctx context.Context, cluster *scyllav1.ScyllaCluster, podRunning bool, options ...PodOption) error {
	return s.CreatePodsPartition(ctx, cluster, 0, podRunning, options...)
}

func (s *StatefulSetOperatorStub) CreatePodsPartition(ctx context.Context, cluster *scyllav1.ScyllaCluster, partition int, podRunning bool, options ...PodOption) error {
	for _, rack := range cluster.Spec.Datacenter.Racks {
		sts := &appsv1.StatefulSet{}

		err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			err := s.env.Get(ctx, client.ObjectKey{
				Name:      naming.StatefulSetNameForRack(rack, cluster),
				Namespace: cluster.Namespace,
			}, sts)
			if err != nil {
				if apierrors.IsNotFound(err) {
					// StatefulSet for rack might not be created yet.
					return nil
				}
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

			for i := partition; i < int(*sts.Spec.Replicas); i++ {
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

				if podRunning {
					pod.Status.Phase = corev1.PodRunning
					if err := s.env.Status().Update(ctx, pod); err != nil {
						return err
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
			return nil
		})

		if err != nil {
			return err
		}

	}

	return nil
}

func WithPVNodeAffinity(matchExpressions []corev1.NodeSelectorRequirement) func(pv *corev1.PersistentVolume) {
	return func(pv *corev1.PersistentVolume) {
		pv.Spec.NodeAffinity = &corev1.VolumeNodeAffinity{
			Required: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{
						MatchExpressions: matchExpressions,
					},
				},
			},
		}
	}
}

type PVOption func(pv *corev1.PersistentVolume)

func (s *StatefulSetOperatorStub) CreatePVCs(ctx context.Context, cluster *scyllav1.ScyllaCluster, pvOptions ...PVOption) error {
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

		for _, pod := range pods.Items {
			pv := &corev1.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "pv-1337",
				},
				Spec: corev1.PersistentVolumeSpec{
					PersistentVolumeSource: corev1.PersistentVolumeSource{
						Local: &corev1.LocalVolumeSource{
							Path: "/random-path",
						},
					},
					NodeAffinity: &corev1.VolumeNodeAffinity{
						Required: &corev1.NodeSelector{
							NodeSelectorTerms: []corev1.NodeSelectorTerm{{}},
						},
					},
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany},
					Capacity:    corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")},
				},
			}

			for _, opt := range pvOptions {
				opt(pv)
			}

			if err := s.env.Create(ctx, pv); err != nil {
				return err
			}

			pvc := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      naming.PVCNameForPod(pod.Name),
					Namespace: pod.Namespace,
					Labels:    naming.RackLabels(rack, cluster),
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					VolumeName:  pv.Name,
					AccessModes: pv.Spec.AccessModes,
					Resources: corev1.ResourceRequirements{
						Limits:   pv.Spec.Capacity,
						Requests: pv.Spec.Capacity,
					},
				},
			}
			s.logger.Info(ctx, "Creating/updating PVC", "pvc", pvc.Name, "pod", pod.Name)
			if _, err := controllerutil.CreateOrUpdate(ctx, s.env, pvc, func() error { return nil }); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *StatefulSetOperatorStub) SyncStatus(ctx context.Context, cluster *scyllav1.ScyllaCluster) error {
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

func (s *StatefulSetOperatorStub) SyncPods(ctx context.Context, rack scyllav1.RackSpec, cluster *scyllav1.ScyllaCluster) error {
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

	for _, p := range pods.Items {
		if !reflect.DeepEqual(p.Spec, sts.Spec.Template.Spec) {
			p.Spec = sts.Spec.Template.Spec
			for i := range p.Spec.Containers {
				p.Spec.Containers[i].VolumeMounts = []corev1.VolumeMount{}
			}

			s.logger.Info(ctx, "Syncing Pod spec with StatefulSet template", "pod", p.Name, "rack", rack.Name)

			if err := s.env.Delete(ctx, &p); err != nil {
				return err
			}
			p.ResourceVersion = ""
			if err := s.env.Create(ctx, &p); err != nil {
				return err
			}
		}
	}

	return nil
}
