// Copyright (C) 2025 ScyllaDB

package scylladbdatacenter

import (
	"context"
	"fmt"
	"sort"

	o "github.com/onsi/gomega"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	utilsv1alpha1 "github.com/scylladb/scylla-operator/test/e2e/utils/v1alpha1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
)

func WaitForFullQuorum(ctx context.Context, client corev1client.CoreV1Interface, sdc *scyllav1alpha1.ScyllaDBDatacenter) {
	framework.By("Waiting for the ScyllaDBDatacenter to reach consistency ALL")
	err := waitForFullQuorum(ctx, client, sdc)
	o.Expect(err).NotTo(o.HaveOccurred())
	framework.Infof("ScyllaDB nodes have reached status consistency.")
}

func waitForFullQuorum(ctx context.Context, client corev1client.CoreV1Interface, sdc *scyllav1alpha1.ScyllaDBDatacenter) error {
	broadcastAddresses, err := utilsv1alpha1.GetBroadcastAddresses(ctx, client, sdc)
	if err != nil {
		return fmt.Errorf("can't get broadcast addresses for ScyllaDBDatacenter %q: %w", naming.ObjRef(sdc), err)
	}

	sort.Strings(broadcastAddresses)

	err = utilsv1alpha1.WaitForFullQuorum(ctx, client, sdc, broadcastAddresses)
	if err != nil {
		return fmt.Errorf("can't wait for scylla nodes to reach status consistency: %w", err)
	}

	return nil
}
