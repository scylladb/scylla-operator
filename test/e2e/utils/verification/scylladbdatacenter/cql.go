// Copyright (C) 2025 ScyllaDB

package scylladbdatacenter

import (
	"context"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
)

func WaitForFullQuorum(ctx context.Context, client corev1client.CoreV1Interface, sdc *scyllav1alpha1.ScyllaDBDatacenter) {
	return
}
