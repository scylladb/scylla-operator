// Copyright (C) 2017 ScyllaDB

package integration

import (
	"context"
	"fmt"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/v1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/pointer"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (t *TestEnvironment) SingleRackCluster(ns *corev1.Namespace) *scyllav1.ScyllaCluster {
	return &scyllav1.ScyllaCluster{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-",
			Namespace:    ns.Name,
		},
		Spec: scyllav1.ClusterSpec{
			Version:       "4.2.0",
			AgentVersion:  pointer.StringPtr("2.2.0"),
			DeveloperMode: true,
			Datacenter: scyllav1.DatacenterSpec{
				Name: "dc1",
				Racks: []scyllav1.RackSpec{
					{
						Name:    "rack1",
						Members: 3,
						Storage: scyllav1.StorageSpec{
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
						Placement: &scyllav1.PlacementSpec{
							Tolerations: []corev1.Toleration{
								{
									Key:      "node.kubernetes.io/not-ready",
									Operator: corev1.TolerationOpExists,
								},
							},
						},
					},
				},
			},
		},
	}
}

func (t *TestEnvironment) MultiRackCluster(ns *corev1.Namespace, members ...int32) *scyllav1.ScyllaCluster {
	storage := scyllav1.StorageSpec{
		Capacity: "10M",
	}
	resources := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("1"),
			corev1.ResourceMemory: resource.MustParse("200M"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("1"),
			corev1.ResourceMemory: resource.MustParse("200M"),
		},
	}
	placement := &scyllav1.PlacementSpec{
		Tolerations: []corev1.Toleration{
			{
				Key:      "node.kubernetes.io/not-ready",
				Operator: corev1.TolerationOpExists,
			},
		},
	}

		c := &scyllav1.ScyllaCluster{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-",
			Namespace:    ns.Name,
		},
		Spec: scyllav1.ClusterSpec{
			Version:       "4.2.0",
			AgentVersion:  pointer.StringPtr("2.2.0"),
			DeveloperMode: true,
			Datacenter: scyllav1.DatacenterSpec{
				Name:  "dc1",
				Racks: []scyllav1.RackSpec{},
			},
		},
	}

	for i, m := range members {
		c.Spec.Datacenter.Racks = append(c.Spec.Datacenter.Racks, scyllav1.RackSpec{
			Name:      fmt.Sprintf("rack%d", i),
			Members:   m,
			Storage:   storage,
			Resources: resources,
			Placement: placement,
		})
	}

	return c
}

func (t *TestEnvironment) WaitForCluster(ctx context.Context, cluster *scyllav1.ScyllaCluster) error {
	return wait.Poll(t.RetryInterval, t.Timeout, func() (bool, error) {
		err := t.Get(ctx, naming.NamespacedName(cluster.Name, cluster.Namespace), cluster)
		if apierrors.IsNotFound(err) {
			return false, nil
		} else if err != nil {
			return false, err
		}

		return true, nil
	})
}

func (t *TestEnvironment) ClusterScaleSteps(desiredNodeCount int32) []int32 {
	steps := make([]int32, desiredNodeCount+1)
	for i := range steps {
		steps[i] = int32(i)
	}
	return steps
}

func (t *TestEnvironment) StatefulSetOfRack(ctx context.Context, rack scyllav1.RackSpec, cluster *scyllav1.ScyllaCluster) (*appsv1.StatefulSet, error) {
	sts := &appsv1.StatefulSet{}
	return sts, wait.Poll(t.RetryInterval, t.Timeout, func() (bool, error) {
		err := t.Get(ctx, naming.NamespacedName(naming.StatefulSetNameForRack(rack, cluster), cluster.Namespace), sts)
		if apierrors.IsNotFound(err) {
			return false, nil
		} else if err != nil {
			return false, err
		}
		return true, nil
	})
}

func (t *TestEnvironment) AssertRackScaled(ctx context.Context, rack scyllav1.RackSpec, cluster *scyllav1.ScyllaCluster, replicas int32) error {
	return wait.Poll(t.RetryInterval, t.Timeout, func() (bool, error) {
		sts, err := t.StatefulSetOfRack(ctx, rack, cluster)
		if err != nil {
			return false, err
		}
		return *sts.Spec.Replicas >= replicas, nil
	})
}

func (t *TestEnvironment) RackMemberServices(ctx context.Context, namespace string, rack scyllav1.RackSpec, cluster *scyllav1.ScyllaCluster) ([]corev1.Service, error) {
	services := &corev1.ServiceList{}
	err := wait.PollImmediate(t.RetryInterval, t.Timeout, func() (bool, error) {
		err := t.List(ctx, services, &client.ListOptions{
			Namespace:     namespace,
			LabelSelector: naming.RackSelector(rack, cluster),
		})
		if err != nil {
			return false, err
		}
		return len(services.Items) == int(rack.Members), nil
	})
	if err != nil {
		return nil, err
	}

	return services.Items, nil
}

func (t *TestEnvironment) MembersNotScheduled(spec scyllav1.RackSpec, status scyllav1.RackStatus) error {
	return wait.Poll(t.RetryInterval, t.Timeout, func() (bool, error) {
		return spec.Members > status.Members, nil
	})
}

func (t *TestEnvironment) AssertRackResourcesScaled(ctx context.Context, rack scyllav1.RackSpec, cluster *scyllav1.ScyllaCluster, resources corev1.ResourceRequirements) error {
	return wait.Poll(t.RetryInterval, t.Timeout, func() (bool, error) {
		c := &scyllav1.ScyllaCluster{}
		if err := t.Get(ctx, naming.NamespacedName(cluster.Name, cluster.Namespace), c); err != nil {
			return false, err
		}
		return reflect.DeepEqual(c.Spec.Datacenter.Racks[0].Resources, resources), nil
	})
}

func (t *TestEnvironment) SingleSecret(name string, ns *corev1.Namespace, token string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns.Name,
		},
		Data: map[string][]byte{
			"token": []byte(token),
		},
	}
}

func (t *TestEnvironment) AssertRackScaledDown(ctx context.Context, rack scyllav1.RackSpec, cluster *scyllav1.ScyllaCluster, replicas int32) error {
	return wait.Poll(t.RetryInterval, t.Timeout, func() (bool, error) {
		sts, err := t.StatefulSetOfRack(ctx, rack, cluster)
		if err != nil {
			return false, err
		}
		return *sts.Spec.Replicas <= replicas, nil
	})
}
