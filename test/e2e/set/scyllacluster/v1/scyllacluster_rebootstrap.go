// Copyright (c) 2021-2022 ScyllaDB.

package v1

import (
	"context"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	scyllafixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scylla/v1"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	v1utils "github.com/scylladb/scylla-operator/test/e2e/utils/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = g.Describe("ScyllaCluster", func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("scyllacluster")

	g.It("should re-bootstrap from old PVCs", func() {
		const membersCount = 3

		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sc := scyllafixture.BasicScyllaCluster.ReadOrFail()
		sc.Spec.Datacenter.Racks[0].Members = membersCount

		framework.By("Creating a ScyllaCluster")
		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		originalSC := sc.DeepCopy()
		originalSC.ResourceVersion = ""

		framework.By("Waiting for the ScyllaCluster to deploy (RV=%s)", sc.ResourceVersion)
		waitCtx1, waitCtx1Cancel := v1utils.ContextForRollout(ctx, sc)
		defer waitCtx1Cancel()
		sc, err = v1utils.WaitForScyllaClusterState(waitCtx1, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, v1utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), f.ScyllaClient(), sc)
		hosts := getScyllaHostsAndWaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(hosts).To(o.HaveLen(membersCount))
		di := insertAndVerifyCQLData(ctx, hosts)
		defer di.Close()

		framework.By("Deleting the ScyllaCluster")
		var propagationPolicy = metav1.DeletePropagationForeground
		err = f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace).Delete(ctx, sc.Name, metav1.DeleteOptions{
			Preconditions: &metav1.Preconditions{
				UID: &sc.UID,
			},
			PropagationPolicy: &propagationPolicy,
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		waitCtx2, waitCtx2Cancel := context.WithCancel(ctx)
		defer waitCtx2Cancel()
		err = framework.WaitForObjectDeletion(
			waitCtx2,
			f.DynamicClient(),
			scyllav1.GroupVersion.WithResource("scyllaclusters"),
			sc.Namespace,
			sc.Name,
			&sc.UID,
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Verifying PVCs' presence")
		pvcs, err := f.KubeClient().CoreV1().PersistentVolumeClaims(sc.Namespace).List(ctx, metav1.ListOptions{})
		o.Expect(pvcs.Items).To(o.HaveLen(membersCount))
		o.Expect(err).NotTo(o.HaveOccurred())

		pvcMap := map[string]*corev1.PersistentVolumeClaim{}
		for i := range pvcs.Items {
			pvc := &pvcs.Items[i]
			pvcMap[pvc.Name] = pvc
		}

		stsName := fmt.Sprintf("%s-%s-%s", sc.Name, sc.Spec.Datacenter.Name, sc.Spec.Datacenter.Racks[0].Name)
		for i := int32(0); i < sc.Spec.Datacenter.Racks[0].Members; i++ {
			podName := fmt.Sprintf("%s-%d", stsName, i)
			pvcName := naming.PVCNameForPod(podName)
			o.Expect(pvcMap).To(o.HaveKey(pvcName))
			o.Expect(pvcMap[pvcName].ObjectMeta.DeletionTimestamp).To(o.BeNil())
		}

		framework.By("Redeploying the ScyllaCluster")
		sc = originalSC.DeepCopy()
		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to redeploy (RV=%s)", sc.ResourceVersion)
		waitCtx3, waitCtx3Cancel := v1utils.ContextForRollout(ctx, sc)
		defer waitCtx3Cancel()
		sc, err = v1utils.WaitForScyllaClusterState(waitCtx3, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, v1utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), f.ScyllaClient(), sc)

		oldHosts := hosts
		hosts = getScyllaHostsAndWaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(hosts).To(o.HaveLen(len(oldHosts)))
		o.Expect(hosts).NotTo(o.ConsistOf(oldHosts))
		err = di.SetClientEndpoints(hosts)
		o.Expect(err).NotTo(o.HaveOccurred())
		verifyCQLData(ctx, di)
	})
})
