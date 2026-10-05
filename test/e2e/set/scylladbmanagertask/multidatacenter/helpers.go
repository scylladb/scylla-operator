// Copyright (C) 2026 ScyllaDB

package multidatacenter

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/scylladb/scylla-manager/v3/pkg/managerclient"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	utilsv1alpha1 "github.com/scylladb/scylla-operator/test/e2e/utils/v1alpha1"
	scylladbdatacenterverification "github.com/scylladb/scylla-operator/test/e2e/utils/verification/scylladbdatacenter"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
)

const (
	// managerClusterStatusHealthyTimeout is the maximum amount of time it should take for ScyllaDB Manager to report
	// a healthy status for every node of a registered cluster.
	//
	// ScyllaDB Manager registers a cluster through the nodes of a single datacenter and discovers the rest of the ring
	// from there. Until it refreshes its per-host config cache, which it does on an interval of its own, it reports the
	// nodes it has discovered since as having no host config available. This has to outlast that refresh.
	managerClusterStatusHealthyTimeout = 6 * time.Minute
)

// datacenter is a single datacenter of a manually wired multi-datacenter ScyllaDB cluster, represented by exactly one
// ScyllaDBDatacenter. The supported multi-datacenter setup has no single object spanning the datacenters, so each one
// is created and rolled out on its own, and they are joined into a single ring through spec.scyllaDB.externalSeeds.
type datacenter struct {
	// name is the ScyllaDB datacenter name. It ends up in cassandra-rackdc.properties and, for backup tasks, in the
	// datacenter-scoped ScyllaDB Manager locations, so it is distinct from workerClusterKey.
	name string

	// workerClusterKey identifies the worker cluster entry supplying this datacenter's object storage settings.
	// It is unrelated to the Kubernetes cluster the datacenter runs in: a bucket is reachable from any cluster
	// holding its credentials, and the credentials are mounted by the test itself. Keeping the two decoupled lets
	// the first datacenter run in the control plane cluster, where ScyllaDB Manager lives, while still drawing its
	// bucket from a worker entry - the only place object storage settings are configured in multi-datacenter runs.
	workerClusterKey string

	// namespace is the namespace this datacenter's ScyllaDBDatacenter lives in, in its own Kubernetes cluster.
	namespace string

	// client is a client for the Kubernetes cluster this datacenter runs in, scoped to namespace.
	client framework.Client

	// sdc is this datacenter's ScyllaDBDatacenter. It is populated by setUpMultiDatacenterScyllaDBDatacenters.
	sdc *scyllav1alpha1.ScyllaDBDatacenter
}

// orderWorkerClusterKeysByControlPlaneAffinity returns the keys of all worker clusters, ordered so that the ones
// pointing at the control plane cluster come first.
//
// ScyllaDB Manager is only ever deployed in the control plane cluster, so the datacenter registering with it has to
// run there. The control plane cluster is expected to be listed among the workers, see the --worker-kubeconfigs
// flag, which is what lets the first datacenter draw its object storage settings from a worker entry.
// The remaining entries are interchangeable to the test, so the ordering only spreads the datacenters across distinct
// Kubernetes clusters when the setup provides them, and degrades to a single cluster when it doesn't.
func orderWorkerClusterKeysByControlPlaneAffinity(f *framework.Framework) []string {
	g.GinkgoHelper()

	workerClusters := f.WorkerClusters()
	controlPlaneHost := f.AdminClientConfig().Host

	var controlPlaneKeys, otherKeys []string
	for _, key := range slices.Sorted(maps.Keys(workerClusters)) {
		if workerClusters[key].AdminClientConfig().Host == controlPlaneHost {
			controlPlaneKeys = append(controlPlaneKeys, key)
			continue
		}

		otherKeys = append(otherKeys, key)
	}

	return slices.Concat(controlPlaneKeys, otherKeys)
}

// newDatacenters returns the three datacenters of a multi-datacenter cluster. The first one runs in the control plane
// cluster, in the given namespace, and the remaining two in namespaces of their own in the clusters of the last two
// worker entries, which are the ones the furthest from the control plane in the given order.
func newDatacenters(ctx context.Context, f *framework.Framework, workerClusterKeys []string, controlPlaneNS string, controlPlaneNSClient framework.Client) []*datacenter {
	g.GinkgoHelper()

	o.Expect(len(workerClusterKeys)).To(o.BeNumerically(">=", 3), "at least 3 worker clusters are required")

	datacenters := []*datacenter{
		{
			name:             "dc1",
			workerClusterKey: workerClusterKeys[0],
			namespace:        controlPlaneNS,
			client:           controlPlaneNSClient,
		},
	}

	for i, workerClusterKey := range workerClusterKeys[len(workerClusterKeys)-2:] {
		ns, nsClient := f.WorkerClusters()[workerClusterKey].CreateUserNamespace(ctx)
		datacenters = append(datacenters, &datacenter{
			name:             fmt.Sprintf("dc%d", i+2),
			workerClusterKey: workerClusterKey,
			namespace:        ns.Name,
			client:           nsClient,
		})
	}

	return datacenters
}

// datacentersInSameNamespaces returns new datacenters living in the namespaces of the given ones, for another
// multi-datacenter cluster to be set up in their place.
func datacentersInSameNamespaces(datacenters []*datacenter) []*datacenter {
	newDatacenters := make([]*datacenter, 0, len(datacenters))
	for _, dc := range datacenters {
		newDatacenters = append(newDatacenters, &datacenter{
			name:             dc.name,
			workerClusterKey: dc.workerClusterKey,
			namespace:        dc.namespace,
			client:           dc.client,
		})
	}

	return newDatacenters
}

// setUpMultiDatacenterScyllaDBDatacenters creates one ScyllaDBDatacenter per datacenter and waits for them to form a
// single ring.
//
// Datacenters are created in order. The first one is the seed of the cluster and the only one registering with the
// global ScyllaDB Manager instance; every other one is joined to it through spec.scyllaDB.externalSeeds, so that the ring is
// registered exactly once.
//
// mutateScyllaDBDatacenter, when not nil, is called for every datacenter's ScyllaDBDatacenter before it is created.
func setUpMultiDatacenterScyllaDBDatacenters(ctx context.Context, f *framework.Framework, clusterName string, datacenters []*datacenter, mutateScyllaDBDatacenter func(dc *datacenter, sdc *scyllav1alpha1.ScyllaDBDatacenter)) {
	g.GinkgoHelper()

	o.Expect(datacenters).NotTo(o.BeEmpty())

	var externalSeeds []string
	var sharedAgentAuthToken string

	for i, dc := range datacenters {
		sdc := f.GetDefaultScyllaDBDatacenter()
		sdc.GenerateName = ""
		sdc.Name = clusterName
		sdc.Spec.ClusterName = clusterName
		sdc.Spec.DatacenterName = pointer.Ptr(dc.name)
		sdc.Spec.ScyllaDB.ExternalSeeds = externalSeeds

		if i == 0 {
			// Only the first datacenter registers the ring with the global ScyllaDB Manager instance.
			metav1.SetMetaDataLabel(&sdc.ObjectMeta, naming.GlobalScyllaDBManagerRegistrationLabel, naming.LabelValueTrue)
		} else {
			// ScyllaDB Manager can only reach every node of the ring if all datacenters share a single ScyllaDB
			// Manager Agent auth token. Each datacenter is otherwise provisioned with a new, random one.
			framework.By("Creating a Secret with the shared ScyllaDB Manager Agent auth token in datacenter %q", dc.name)
			sharedAgentAuthTokenSecret := createSharedAgentAuthTokenSecret(ctx, dc.client.KubeClient().CoreV1(), dc.namespace, sharedAgentAuthToken)
			metav1.SetMetaDataAnnotation(&sdc.ObjectMeta, naming.ScyllaDBManagerAgentAuthTokenOverrideSecretRefAnnotation, sharedAgentAuthTokenSecret.Name)
		}

		if mutateScyllaDBDatacenter != nil {
			mutateScyllaDBDatacenter(dc, sdc)
		}

		framework.By("Creating ScyllaDBDatacenter %q of datacenter %q", naming.ManualRef(dc.namespace, sdc.Name), dc.name)
		sdc, err := dc.client.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(dc.namespace).Create(ctx, sdc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for ScyllaDBDatacenter of datacenter %q to roll out (RV=%s)", dc.name, sdc.ResourceVersion)
		rolloutCtx, rolloutCtxCancel := utilsv1alpha1.ContextForMultiDatacenterRollout(ctx, sdc)
		defer rolloutCtxCancel()
		sdc, err = controllerhelpers.WaitForScyllaDBDatacenterState(rolloutCtx, dc.client.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(dc.namespace), sdc.Name, controllerhelpers.WaitForStateOptions{}, utilsv1alpha1.IsScyllaDBDatacenterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scylladbdatacenterverification.Verify(ctx, dc.client.KubeClient(), dc.client.ScyllaClient(), sdc)

		dc.sdc = sdc

		// Every node has to see the entire ring formed so far before the next datacenter joins it through its seeds.
		ring := datacenters[:i+1]
		framework.By("Waiting for the ring of datacenters %v to reach full quorum", datacenterNames(ring))
		scylladbdatacenterverification.WaitForFullMultiDCQuorum(ctx, dcCoreClientMap(ring), datacenterScyllaDBDatacenters(ring))

		if i == 0 {
			// The remaining datacenters have to be provisioned with the auth token of the first one, which is the
			// only one letting its token be generated.
			sharedAgentAuthToken = getAgentAuthToken(ctx, dc.client.KubeClient().CoreV1(), sdc)
		}

		broadcastAddresses, err := utilsv1alpha1.GetBroadcastAddresses(ctx, dc.client.KubeClient().CoreV1(), sdc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(broadcastAddresses).To(o.HaveLen(int(utilsv1alpha1.GetNodeCount(sdc))))
		externalSeeds = slices.Concat(externalSeeds, broadcastAddresses)
	}
}

// createSharedAgentAuthTokenSecret creates the Secret holding the ScyllaDB Manager Agent auth token shared by all
// datacenters of the cluster. It is referenced from the ScyllaDBDatacenter through
// naming.ScyllaDBManagerAgentAuthTokenOverrideSecretRefAnnotation, so it has to live in the same namespace.
// It has to exist before the ScyllaDBDatacenter is created, or the datacenter's rollout blocks waiting for it.
func createSharedAgentAuthTokenSecret(ctx context.Context, coreClient corev1client.CoreV1Interface, namespace string, authToken string) *corev1.Secret {
	g.GinkgoHelper()

	o.Expect(authToken).NotTo(o.BeEmpty())

	authTokenConfig, err := helpers.GetAgentAuthTokenConfig(authToken)
	o.Expect(err).NotTo(o.HaveOccurred())

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "shared-agent-auth-token-",
		},
		Data: map[string][]byte{
			naming.ScyllaAgentAuthTokenFileName: authTokenConfig,
		},
	}

	secret, err = coreClient.Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	return secret
}

// getAgentAuthToken returns the ScyllaDB Manager Agent auth token provisioned for the given ScyllaDBDatacenter.
func getAgentAuthToken(ctx context.Context, coreClient corev1client.CoreV1Interface, sdc *scyllav1alpha1.ScyllaDBDatacenter) string {
	g.GinkgoHelper()

	secretName := naming.AgentAuthTokenSecretName(sdc)
	secret, err := coreClient.Secrets(sdc.Namespace).Get(ctx, secretName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	authToken, err := helpers.GetAgentAuthTokenFromSecret(secret)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(authToken).NotTo(o.BeEmpty())

	return authToken
}

// verifyManagerClusterStatusHealthy asserts that ScyllaDB Manager sees every node of every given datacenter and can
// reach all of them over both CQL and the ScyllaDB Manager Agent's REST API.
//
// This is what makes the shared agent auth token observable: an agent provisioned with a different token rejects
// ScyllaDB Manager's requests, which surfaces here as an unhealthy REST status for that datacenter's hosts.
//
// Call it once ScyllaDB Manager has run a task against the cluster. A task can only run when ScyllaDB Manager holds a
// config for every host, so by then the status has settled and this returns immediately, instead of waiting out the
// config cache refresh.
func verifyManagerClusterStatusHealthy(ctx context.Context, managerClient *managerclient.Client, managerClusterID string, datacenters []*datacenter) {
	g.GinkgoHelper()

	expectedNodeCount := 0
	expectedDatacenterNames := make([]string, 0, len(datacenters))
	for _, dc := range datacenters {
		expectedDatacenterNames = append(expectedDatacenterNames, dc.name)
		expectedNodeCount += int(utilsv1alpha1.GetNodeCount(dc.sdc))
	}

	statusCtx, statusCtxCancel := context.WithTimeoutCause(
		ctx,
		managerClusterStatusHealthyTimeout,
		fmt.Errorf("ScyllaDB Manager has not reported a healthy status for all nodes of cluster %q in time", managerClusterID),
	)
	defer statusCtxCancel()

	o.Eventually(func(eo o.Gomega) {
		clusterStatus, err := managerClient.ClusterStatus(statusCtx, managerClusterID)
		eo.Expect(err).NotTo(o.HaveOccurred())
		eo.Expect(clusterStatus).To(o.HaveLen(expectedNodeCount))

		var gotDatacenterNames []string
		for _, hostStatus := range clusterStatus {
			if !slices.Contains(gotDatacenterNames, hostStatus.Dc) {
				gotDatacenterNames = append(gotDatacenterNames, hostStatus.Dc)
			}

			// A failed health check is reported through the cause, which is also what ScyllaDB Manager renders as the
			// error for a host. Asserting on it, rather than on the spelling of the status, keeps this independent of
			// the set of status values ScyllaDB Manager happens to use.
			eo.Expect(hostStatus.CqlStatus).NotTo(o.BeEmpty(), "ScyllaDB Manager has no CQL status for host %q of datacenter %q", hostStatus.Host, hostStatus.Dc)
			eo.Expect(hostStatus.CqlCause).To(o.BeEmpty(), "host %q of datacenter %q is not reachable over CQL (status %q)", hostStatus.Host, hostStatus.Dc, hostStatus.CqlStatus)

			eo.Expect(hostStatus.RestStatus).NotTo(o.BeEmpty(), "ScyllaDB Manager has no ScyllaDB Manager Agent status for host %q of datacenter %q", hostStatus.Host, hostStatus.Dc)
			eo.Expect(hostStatus.RestCause).To(o.BeEmpty(), "ScyllaDB Manager Agent of host %q of datacenter %q is not reachable (status %q)", hostStatus.Host, hostStatus.Dc, hostStatus.RestStatus)
		}

		eo.Expect(gotDatacenterNames).To(o.ConsistOf(expectedDatacenterNames))
	}).WithContext(statusCtx).WithPolling(5 * time.Second).Should(o.Succeed())
}

func dcCoreClientMap(datacenters []*datacenter) map[string]corev1client.CoreV1Interface {
	m := make(map[string]corev1client.CoreV1Interface, len(datacenters))
	for _, dc := range datacenters {
		m[dc.name] = dc.client.KubeClient().CoreV1()
	}

	return m
}

func datacenterScyllaDBDatacenters(datacenters []*datacenter) []*scyllav1alpha1.ScyllaDBDatacenter {
	sdcs := make([]*scyllav1alpha1.ScyllaDBDatacenter, 0, len(datacenters))
	for _, dc := range datacenters {
		sdcs = append(sdcs, dc.sdc)
	}

	return sdcs
}

func datacenterNames(datacenters []*datacenter) []string {
	names := make([]string, 0, len(datacenters))
	for _, dc := range datacenters {
		names = append(names, dc.name)
	}

	return names
}

// datacenterBroadcastRPCAddresses returns the broadcast RPC addresses of every node of every given datacenter.
func datacenterBroadcastRPCAddresses(ctx context.Context, datacenters []*datacenter) []string {
	g.GinkgoHelper()

	var hosts []string
	for _, dc := range datacenters {
		dcHosts, err := utilsv1alpha1.GetBroadcastRPCAddresses(ctx, dc.client.KubeClient().CoreV1(), dc.sdc)
		o.Expect(err).NotTo(o.HaveOccurred())
		hosts = append(hosts, dcHosts...)
	}

	return hosts
}
