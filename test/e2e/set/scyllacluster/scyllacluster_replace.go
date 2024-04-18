// Copyright (C) 2021 ScyllaDB

package scyllacluster

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/scyllaclient"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = g.Describe("ScyllaCluster", func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("scyllacluster")

	const (
		scyllaOSImageRepository         = "docker.io/scylladb/scylla"
		scyllaEnterpriseImageRepository = "docker.io/scylladb/scylla-enterprise"
		clusterIPProcedure              = "ClusterIP"
		hostIDProcedure                 = "HostID"
	)

	validateReplaceViaClusterIPAddress := func(ctx context.Context, configClient *scyllaclient.ConfigClient, preReplaceService *corev1.Service) error {
		replaceAddressFirstBoot, err := configClient.ReplaceAddressFirstBoot(ctx)
		if err != nil {
			return fmt.Errorf("can't get replace_address_first_boot config parameter: %w", err)
		}

		if replaceAddressFirstBoot != preReplaceService.Spec.ClusterIP {
			return fmt.Errorf("unexpected value of replace_address_first_boot scylla config, expected %q, got %q", preReplaceService.Spec.ClusterIP, replaceAddressFirstBoot)
		}

		return nil
	}

	validateReplaceViaHostID := func(ctx context.Context, configClient *scyllaclient.ConfigClient, preReplaceService *corev1.Service) error {
		replaceNodeFirstBoot, err := configClient.ReplaceNodeFirstBoot(ctx)
		if err != nil {
			return fmt.Errorf("can't get replace_node_first_boot config parameter: %w", err)
		}

		if replaceNodeFirstBoot != preReplaceService.Annotations[naming.HostIDAnnotation] {
			return fmt.Errorf("unexpected value of replace_node_first_boot scylla config, expected %q, got %q", preReplaceService.Annotations[naming.HostIDAnnotation], replaceNodeFirstBoot)
		}

		return nil
	}

	type entry struct {
		procedure             string
		scyllaImageRepository string
		scyllaVersion         string
		validateScyllaConfig  func(context.Context, *scyllaclient.ConfigClient, *corev1.Service) error
	}

	describeEntry := func(e *entry) string {
		return fmt.Sprintf(`using %s based procedure when version of ScyllaDB is "%s:%s"`, e.procedure, e.scyllaImageRepository, e.scyllaVersion)
	}

	g.DescribeTable("should replace a node", func(e *entry) {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sc := f.GetDefaultScyllaCluster()
		sc.Spec.Repository = e.scyllaImageRepository
		sc.Spec.Version = e.scyllaVersion
		sc.Spec.Datacenter.Racks[0].Members = 3

		isHeadlessNodeService := sc.Spec.ExposeOptions == nil || sc.Spec.ExposeOptions.NodeService == nil || sc.Spec.ExposeOptions.NodeService.Type == scyllav1.NodeServiceTypeHeadless
		if e.procedure == clusterIPProcedure && isHeadlessNodeService {
			g.Skip("Skipping because ClusterIP-based replace procedure can only be tested on ScyllaClusters exposed on ClusterIPs")
		}

		framework.By("Creating a ScyllaCluster")
		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to roll out (RV=%s)", sc.ResourceVersion)
		waitCtx1, waitCtx1Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx1Cancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx1, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), sc)
		waitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)

		hosts, err := utils.GetBroadcastRPCAddresses(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(hosts).To(o.HaveLen(int(utils.GetMemberCount(sc))))
		di := insertAndVerifyCQLData(ctx, hosts)
		defer di.Close()

		replacedNodeService, err := f.KubeClient().CoreV1().Services(sc.Namespace).Get(ctx, utils.GetNodeName(sc, 0), metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		preReplaceService := replacedNodeService.DeepCopy()
		framework.Infof("Initial service %q UID is %q", preReplaceService.Name, preReplaceService.UID)

		framework.By("Replacing a node #0")
		pod, err := f.KubeClient().CoreV1().Pods(f.Namespace()).Get(
			ctx,
			utils.GetNodeName(sc, 0),
			metav1.GetOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Infof("Initial pod %q UID is %q", pod.Name, pod.UID)

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

		waitCtx2, waitCtx2Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx2Cancel()

		if e.procedure == clusterIPProcedure {
			framework.By("Waiting for the service to be replaced")
			_, err := controllerhelpers.WaitForServiceState(waitCtx2, f.KubeClient().CoreV1().Services(preReplaceService.Namespace), preReplaceService.Name, controllerhelpers.WaitForStateOptions{TolerateDelete: true}, func(svc *corev1.Service) (bool, error) {
				return svc.UID != preReplaceService.UID, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		framework.By("Waiting for the pod to be replaced")
		_, err = controllerhelpers.WaitForPodState(waitCtx2, f.KubeClient().CoreV1().Pods(pod.Namespace), pod.Name, controllerhelpers.WaitForStateOptions{TolerateDelete: true}, func(p *corev1.Pod) (bool, error) {
			return p.UID != pod.UID, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		// Give the controller some time to observe that the pod is down.
		time.Sleep(10 * time.Second)

		framework.By("Waiting for the ScyllaCluster to re-deploy")
		waitCtx3, waitCtx3Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx3Cancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx3, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), sc)

		framework.By("Waiting for the other nodes to acknowledge the replace")

		client, _, err := utils.GetScyllaClient(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(err).NotTo(o.HaveOccurred())

		replacedNodeService, err = f.KubeClient().CoreV1().Services(sc.Namespace).Get(ctx, utils.GetNodeName(sc, 0), metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		replacedNodePod, err := f.KubeClient().CoreV1().Pods(f.Namespace()).Get(ctx, naming.PodNameFromService(replacedNodeService), metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		replacedNodeBroadcastAddress, err := utils.GetBroadcastAddress(ctx, f.KubeClient().CoreV1(), sc, replacedNodeService, replacedNodePod)
		o.Expect(err).NotTo(o.HaveOccurred())

		otherNodeService, err := f.KubeClient().CoreV1().Services(sc.Namespace).Get(ctx, utils.GetNodeName(sc, 1), metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		o.Eventually(func(g o.Gomega) {
			otherNodeBroadcastRPCAddress, err := utils.GetBroadcastRPCAddress(ctx, f.KubeClient().CoreV1(), sc, otherNodeService)
			o.Expect(err).NotTo(o.HaveOccurred())

			status, err := client.Status(ctx, otherNodeBroadcastRPCAddress)
			g.Expect(err).NotTo(o.HaveOccurred())
			g.Expect(status.LiveHosts()).To(o.ContainElement(replacedNodeBroadcastAddress))
		}).WithPolling(time.Second).WithTimeout(5 * time.Minute).Should(o.Succeed())

		hosts, err = utils.GetBroadcastRPCAddresses(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(hosts).To(o.HaveLen(int(utils.GetMemberCount(sc))))

		verifyCQLData(ctx, di)

		framework.By("Verifying ScyllaDB config")

		replacedNodeBroadcastRPCAddress, err := utils.GetBroadcastRPCAddress(ctx, f.KubeClient().CoreV1(), sc, replacedNodeService)
		o.Expect(err).NotTo(o.HaveOccurred())

		configClient, err := utils.GetScyllaConfigClient(ctx, f.KubeClient().CoreV1(), sc, replacedNodeBroadcastRPCAddress)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = e.validateScyllaConfig(ctx, configClient, preReplaceService)
		o.Expect(err).NotTo(o.HaveOccurred())
	},
		g.Entry(describeEntry, framework.RequiresClusterIP, &entry{
			procedure:             clusterIPProcedure,
			scyllaImageRepository: scyllaOSImageRepository,
			scyllaVersion:         "5.1.15",
			validateScyllaConfig:  validateReplaceViaClusterIPAddress,
		}),
		g.Entry(describeEntry, framework.RequiresClusterIP, &entry{
			procedure:             clusterIPProcedure,
			scyllaImageRepository: scyllaEnterpriseImageRepository,
			scyllaVersion:         "2022.2.12",
			validateScyllaConfig:  validateReplaceViaClusterIPAddress,
		}),
		g.Entry(describeEntry, &entry{
			procedure:             hostIDProcedure,
			scyllaImageRepository: scyllaOSImageRepository,
			scyllaVersion:         "5.2.6",
			validateScyllaConfig:  validateReplaceViaHostID,
		}),
		g.Entry(describeEntry, &entry{
			procedure:             hostIDProcedure,
			scyllaImageRepository: scyllaEnterpriseImageRepository,
			scyllaVersion:         "2023.1.0",
			validateScyllaConfig:  validateReplaceViaHostID,
		}),
	)
})
