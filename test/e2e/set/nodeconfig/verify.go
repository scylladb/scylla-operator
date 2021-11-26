package nodeconfig

import (
	"context"

	o "github.com/onsi/gomega"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

func verifyDaemonSet(ds *appsv1.DaemonSet) {
	o.Expect(ds.DeletionTimestamp).To(o.BeNil())
	o.Expect(ds.Status.ObservedGeneration).To(o.Equal(ds.Generation))
	o.Expect(ds.Status.UpdatedNumberScheduled).To(o.Equal(ds.Status.DesiredNumberScheduled))
	o.Expect(ds.Status.NumberAvailable).To(o.Equal(ds.Status.DesiredNumberScheduled))
}

func verifyNodeConfig(ctx context.Context, kubeClient kubernetes.Interface, nc *scyllav1alpha1.NodeConfig) {
	framework.By("Verifying NodeConfig %q", naming.ObjRef(nc))

	o.Expect(nc.Status.ObservedGeneration).NotTo(o.BeNil())

	daemonSets, err := utils.GetDaemonSetsForNodeConfig(ctx, kubeClient.AppsV1(), nc)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(daemonSets).To(o.HaveLen(1))

	ds := daemonSets[0]
	verifyDaemonSet(ds)

	o.Expect(nc.Status.Conditions).To(o.HaveLen(1))

	reconciledCond := controllerhelpers.FindNodeConfigCondition(nc.Status.Conditions, scyllav1alpha1.NodeConfigReconciledConditionType)
	o.Expect(reconciledCond).NotTo(o.BeNil())
	o.Expect(reconciledCond.Status).To(o.Equal(corev1.ConditionTrue))
	o.Expect(reconciledCond.Reason).To(o.Equal("FullyReconciledAndUp"))
	o.Expect(reconciledCond.Message).To(o.Equal("All operands are reconciled and available."))
	o.Expect(reconciledCond.LastTransitionTime).NotTo(o.BeNil())
}
