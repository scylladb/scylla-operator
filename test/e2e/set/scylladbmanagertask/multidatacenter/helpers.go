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
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	scyllaclusterverification "github.com/scylladb/scylla-operator/test/e2e/utils/verification/scyllacluster"
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
// v1.ScyllaCluster. The supported multi-datacenter setup has no single object spanning the datacenters, so each one is
// created and rolled out on its own, and they are joined into a single ring through spec.externalSeeds.
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

	// namespace is the namespace this datacenter's ScyllaCluster lives in, in its own Kubernetes cluster.
	namespace string

	// client is a client for the Kubernetes cluster this datacenter runs in, scoped to namespace.
	client framework.Client

	// sc is this datacenter's ScyllaCluster. It is populated by setUpMultiDatacenterScyllaCluster.
	sc *scyllav1.ScyllaCluster
}

// orderWorkerClusterKeysByControlPlaneAffinity returns the keys of all worker clusters, ordered so that the ones
// pointing at the control plane cluster come first.
//
// ScyllaDB Manager is only ever deployed in the control plane cluster, so the datacenter registering with it has to
// run there. The worker entries are otherwise interchangeable to the test, which only needs their object storage
// settings, so this ordering is a preference and not a requirement: it spreads datacenters across distinct Kubernetes
// clusters when the setup provides them, and degrades to a single cluster when it doesn't. That keeps the specs
// working both on a multi-cluster job and on a local single-cluster setup, where every worker entry points at the
// same cluster.
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

// setUpMultiDatacenterScyllaCluster creates one ScyllaCluster per datacenter and waits for them to form a single ring.
//
// Datacenters are created in order. The first one is the seed of the cluster and the only one registering with the
// global ScyllaDB Manager instance; every other one is joined to it through spec.externalSeeds and opts out of the
// registration, so that the ring is registered exactly once.
//
// mutateScyllaCluster, when not nil, is called for every datacenter's ScyllaCluster before it is created.
func setUpMultiDatacenterScyllaCluster(ctx context.Context, f *framework.Framework, clusterName string, datacenters []*datacenter, mutateScyllaCluster func(dc *datacenter, sc *scyllav1.ScyllaCluster)) {
	g.GinkgoHelper()

	o.Expect(datacenters).NotTo(o.BeEmpty())

	var externalSeeds []string
	var sharedAgentAuthToken string

	for i, dc := range datacenters {
		sc := f.GetDefaultScyllaCluster()
		sc.GenerateName = ""
		sc.Name = clusterName
		sc.Spec.Datacenter.Name = dc.name
		sc.Spec.ExternalSeeds = externalSeeds

		if i != 0 {
			// Only the first datacenter registers the ring with the global ScyllaDB Manager instance. Registration
			// is opt-out, so without this every datacenter would register the same ring again.
			metav1.SetMetaDataAnnotation(&sc.ObjectMeta, naming.DisableGlobalScyllaDBManagerIntegrationAnnotation, naming.LabelValueTrue)

			// ScyllaDB Manager can only reach every node of the ring if all datacenters share a single ScyllaDB
			// Manager Agent auth token. Each datacenter is otherwise provisioned with a new, random one.
			framework.By("Creating a Secret with the shared ScyllaDB Manager Agent auth token in datacenter %q", dc.name)
			sharedAgentAuthTokenSecret := createSharedAgentAuthTokenSecret(ctx, dc.client.KubeClient().CoreV1(), dc.namespace, sharedAgentAuthToken)
			metav1.SetMetaDataAnnotation(&sc.ObjectMeta, naming.ScyllaDBManagerAgentAuthTokenOverrideSecretRefAnnotation, sharedAgentAuthTokenSecret.Name)
		}

		if mutateScyllaCluster != nil {
			mutateScyllaCluster(dc, sc)
		}

		framework.By("Creating ScyllaCluster %q of datacenter %q", naming.ManualRef(dc.namespace, sc.Name), dc.name)
		sc, err := dc.client.ScyllaClient().ScyllaV1().ScyllaClusters(dc.namespace).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for ScyllaCluster of datacenter %q to roll out (RV=%s)", dc.name, sc.ResourceVersion)
		rolloutCtx, rolloutCtxCancel := utils.ContextForMultiDatacenterRollout(ctx, sc)
		defer rolloutCtxCancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(rolloutCtx, dc.client.ScyllaClient().ScyllaV1().ScyllaClusters(dc.namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scyllaclusterverification.Verify(ctx, dc.client.KubeClient(), dc.client.ScyllaClient(), sc)

		dc.sc = sc

		if i == 0 {
			scyllaclusterverification.WaitForFullQuorum(ctx, dc.client.KubeClient().CoreV1(), sc)

			// The remaining datacenters have to be provisioned with the auth token of the first one, which is the
			// only one letting its token be generated.
			sharedAgentAuthToken = getAgentAuthToken(ctx, dc.client.KubeClient().CoreV1(), sc)
		}

		broadcastAddresses, err := utils.GetBroadcastAddresses(ctx, dc.client.KubeClient().CoreV1(), sc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(broadcastAddresses).To(o.HaveLen(int(utils.GetMemberCount(sc))))
		externalSeeds = slices.Concat(externalSeeds, broadcastAddresses)
	}

	framework.By("Waiting for the multi-datacenter cluster to reach full quorum")
	scyllaclusterverification.WaitForFullMultiDCQuorum(ctx, dcCoreClientMap(datacenters), datacenterScyllaClusters(datacenters))
}

// createSharedAgentAuthTokenSecret creates the Secret holding the ScyllaDB Manager Agent auth token shared by all
// datacenters of the cluster. It is referenced from the ScyllaCluster through
// naming.ScyllaDBManagerAgentAuthTokenOverrideSecretRefAnnotation, which propagates to the ScyllaDBDatacenter the
// ScyllaCluster is migrated into, so it has to live in the same namespace as the ScyllaCluster.
// It has to exist before the ScyllaCluster is created, or the datacenter's rollout blocks waiting for it.
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

// getAgentAuthToken returns the ScyllaDB Manager Agent auth token provisioned for the given ScyllaCluster.
func getAgentAuthToken(ctx context.Context, coreClient corev1client.CoreV1Interface, sc *scyllav1.ScyllaCluster) string {
	g.GinkgoHelper()

	secretName := naming.AgentAuthTokenSecretNameForScyllaCluster(sc)
	secret, err := coreClient.Secrets(sc.Namespace).Get(ctx, secretName, metav1.GetOptions{})
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
		expectedNodeCount += int(utils.GetMemberCount(dc.sc))
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

func datacenterScyllaClusters(datacenters []*datacenter) []*scyllav1.ScyllaCluster {
	scs := make([]*scyllav1.ScyllaCluster, 0, len(datacenters))
	for _, dc := range datacenters {
		scs = append(scs, dc.sc)
	}

	return scs
}
