// Copyright (C) 2021 ScyllaDB

package nodeconfigpod

import (
	"github.com/scylladb/scylla-operator/pkg/naming"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func makeConfigMap(pod *corev1.Pod, data map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: pod.Namespace,
			Name:      naming.GetTuningConfigMapNameForPod(pod),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(pod, podControllerGVK),
			},
			Labels: map[string]string{
				naming.OwnerUIDLabel:      string(pod.UID),
				naming.ConfigMapTypeLabel: string(naming.NodeConfigDataConfigMapType),
			},
		},
		Data: data,
	}
}
