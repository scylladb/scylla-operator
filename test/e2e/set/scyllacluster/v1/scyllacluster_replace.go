// Copyright (c) 2022 ScyllaDB.

package v1

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/scylladb/scylla-operator/pkg/naming"
	scyllafixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scylla/v1"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	v1utils "github.com/scylladb/scylla-operator/test/e2e/utils/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = g.Describe("ScyllaCluster replace", func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("scyllacluster")

	g.It("should replace a node", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sc := scyllafixture.BasicScyllaCluster.ReadOrFail()
		sc.Spec.Datacenter.Racks[0].Members = 2

		framework.By("Creating a ScyllaCluster")
		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to deploy (RV=%s)", sc.ResourceVersion)
		waitCtx1, waitCtx1Cancel := v1utils.ContextForRollout(ctx, sc)
		defer waitCtx1Cancel()
		sc, err = v1utils.WaitForScyllaClusterState(waitCtx1, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, v1utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), f.ScyllaClient(), sc)
		hosts := getScyllaHostsAndWaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(hosts).To(o.HaveLen(2))
		di := insertAndVerifyCQLData(ctx, hosts)
		defer di.Close()

		framework.By("Replacing a node #0")
		pod, err := f.KubeClient().CoreV1().Pods(f.Namespace()).Get(
			ctx,
			v1utils.GetNodeName(sc, 0),
			metav1.GetOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Infof("Initial pod %q has UID is %q", pod.Name, pod.UID)

		_, err = f.KubeClient().CoreV1().Services(f.Namespace()).Patch(
			ctx,
			pod.Name,
			types.MergePatchType,
			[]byte(fmt.Sprintf(
				`{"metadata":{"labels": {"%s": ""}}}`,
				naming.ReplaceLabel,
			)),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the pod to be replaced")
		waitCtx2, waitCtx2Cancel := v1utils.ContextForRollout(ctx, sc)
		defer waitCtx2Cancel()
		_, err = utils.WaitForPodState(waitCtx2, f.KubeClient().CoreV1().Pods(pod.Namespace), pod.Name, utils.WaitForStateOptions{TolerateDelete: true}, func(p *corev1.Pod) (bool, error) {
			return p.UID != pod.UID, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		// Give the controller some time to observe that the pod is down.
		time.Sleep(10 * time.Second)

		framework.By("Waiting for the ScyllaCluster to re-deploy (RV=%s)", sc.ResourceVersion)
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
