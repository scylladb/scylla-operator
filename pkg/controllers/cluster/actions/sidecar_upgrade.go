// Copyright (C) 2021 ScyllaDB

package actions

import (
	"github.com/pkg/errors"
	"github.com/scylladb/go-log"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

const (
	SidecarVersionUpgradeAction = "sidecar-upgrade"
)

type SidecarUpgrade struct {
	sidecar corev1.Container
}

func (a *SidecarUpgrade) RackUpdated(sts *appsv1.StatefulSet) (bool, error) {
	sidecarIdx, err := naming.FindSidecarInjectorContainer(sts.Spec.Template.Spec.InitContainers)
	if err != nil {
		return false, errors.Wrap(err, "find sidecar container in pod")
	}

	return sts.Spec.Template.Spec.InitContainers[sidecarIdx].Image == a.sidecar.Image, nil
}

func (a *SidecarUpgrade) Update(sts *appsv1.StatefulSet) error {
	initContainers := sts.Spec.Template.Spec.InitContainers
	sidecarIdx, err := naming.FindSidecarInjectorContainer(initContainers)
	if err != nil {
		return errors.Wrap(err, "find sidecar container in existing sts")
	}

	sts.Spec.Template.Spec.InitContainers[sidecarIdx] = a.sidecar
	return nil
}

func NewSidecarUpgrade(c *scyllav1.ScyllaCluster, sidecar corev1.Container, l log.Logger) *rackSynchronizedAction {
	return &rackSynchronizedAction{
		subAction: &SidecarUpgrade{
			sidecar: sidecar,
		},
		cluster: c,
		logger:  l,
	}
}

func (a *SidecarUpgrade) Name() string {
	return SidecarVersionUpgradeAction
}
