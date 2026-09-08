//go:build envtest

package controllers

import (
	"context"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	okubecrypto "github.com/scylladb/scylla-operator/pkg/kubecrypto"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/test/envtest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// The rack StatefulSets are made from more than the ScyllaDBDatacenter spec: the managed config feeds their Pod
// template through an inputs hash, and the agent token and TLS objects are managed alongside them. These specs pin
// that plumbing.
var _ = g.Describe("ScyllaDBDatacenter controller config plumbing", func() {
	const rackName = "rack-a"

	var env *envtest.Environment
	g.BeforeEach(func(ctx g.SpecContext) {
		env = envtest.Setup(ctx)
	})

	g.It("should roll the rack StatefulSet out when the managed config changes", func(ctx g.SpecContext) {
		g.By("Running ScyllaDBDatacenter controller")
		runScyllaDBDatacenterController(ctx, env)

		g.By("Creating ScyllaOperatorConfig singleton")
		createScyllaOperatorConfig(ctx, env)

		g.By("Creating a ScyllaDBDatacenter with a single rack and Alternator enabled")
		sdc := makeEnvtestScyllaDBDatacenter(env.Namespace(), []string{rackName}, func(sdc *scyllav1alpha1.ScyllaDBDatacenter) {
			sdc.Spec.ScyllaDB.AlternatorOptions = &scyllav1alpha1.AlternatorOptions{
				WriteIsolation: "always",
			}
		})
		sdc, err := env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).Create(ctx, sdc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for the rack StatefulSet and marking it as rolled out")
		rackStatefulSetName := naming.StatefulSetNameForRack(sdc.Spec.Racks[0], sdc)
		rackStatefulSet := waitForStatefulSet(ctx, env, rackStatefulSetName, scyllaDBDatacenterControllerDefaultEventuallyTimeout)
		markStatefulSetAsRolledOut(ctx, env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()), rackStatefulSetName)

		managedConfigMapName := naming.GetScyllaDBManagedConfigCMName(sdc.Name)
		managedConfigMap, err := env.TypedKubeClient().CoreV1().ConfigMaps(env.Namespace()).Get(ctx, managedConfigMapName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(managedConfigMap.Data[naming.ScyllaDBManagedConfigName]).To(o.ContainSubstring("alternator_write_isolation: always"))

		inputsHash, ok := rackStatefulSet.Spec.Template.Annotations[naming.InputsHashAnnotation]
		o.Expect(ok).To(o.BeTrue())
		o.Expect(inputsHash).NotTo(o.BeEmpty())

		g.By("Verifying the rack StatefulSet is stable")
		o.Consistently(func(co o.Gomega, ctx context.Context) {
			sts, err := env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()).Get(ctx, rackStatefulSetName, metav1.GetOptions{})
			co.Expect(err).NotTo(o.HaveOccurred())
			co.Expect(sts.Generation).To(o.Equal(rackStatefulSet.Generation))
		}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultConsistentlyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())

		// The write isolation is only rendered into the managed config, so nothing else about the StatefulSet
		// changes.
		g.By("Changing the Alternator write isolation")
		updateScyllaDBDatacenter(ctx, env, sdc.Name, func(sdc *scyllav1alpha1.ScyllaDBDatacenter) {
			sdc.Spec.ScyllaDB.AlternatorOptions.WriteIsolation = "forbid_rmw"
		})

		g.By("Waiting for the managed config to change and the rack StatefulSet to roll out with a new inputs hash")
		o.Eventually(func(eo o.Gomega, ctx context.Context) {
			cm, err := env.TypedKubeClient().CoreV1().ConfigMaps(env.Namespace()).Get(ctx, managedConfigMapName, metav1.GetOptions{})
			eo.Expect(err).NotTo(o.HaveOccurred())
			eo.Expect(cm.Data[naming.ScyllaDBManagedConfigName]).To(o.ContainSubstring("alternator_write_isolation: forbid_rmw"))

			sts, err := env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()).Get(ctx, rackStatefulSetName, metav1.GetOptions{})
			eo.Expect(err).NotTo(o.HaveOccurred())
			eo.Expect(sts.UID).To(o.Equal(rackStatefulSet.UID))
			eo.Expect(sts.Generation).To(o.BeNumerically(">", rackStatefulSet.Generation))
			eo.Expect(sts.Spec.Template.Annotations).To(o.HaveKey(naming.InputsHashAnnotation))
			eo.Expect(sts.Spec.Template.Annotations[naming.InputsHashAnnotation]).NotTo(o.Equal(inputsHash))
		}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())
	})

	g.It("should manage the ScyllaDB Manager agent auth token", func(ctx g.SpecContext) {
		g.By("Running ScyllaDBDatacenter controller")
		runScyllaDBDatacenterController(ctx, env)

		g.By("Creating ScyllaOperatorConfig singleton")
		createScyllaOperatorConfig(ctx, env)

		g.By("Creating a ScyllaDBDatacenter with a single rack")
		sdc := makeEnvtestScyllaDBDatacenter(env.Namespace(), []string{rackName})
		sdc, err := env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).Create(ctx, sdc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for the agent auth token Secret to be created with a generated token")
		agentAuthTokenSecretName := naming.AgentAuthTokenSecretName(sdc)
		var generatedToken string
		o.Eventually(func(eo o.Gomega, ctx context.Context) {
			secret, err := env.TypedKubeClient().CoreV1().Secrets(env.Namespace()).Get(ctx, agentAuthTokenSecretName, metav1.GetOptions{})
			eo.Expect(err).NotTo(o.HaveOccurred())
			eo.Expect(metav1.IsControlledBy(secret, sdc)).To(o.BeTrue())

			generatedToken, err = helpers.GetAgentAuthTokenFromSecret(secret)
			eo.Expect(err).NotTo(o.HaveOccurred())
			eo.Expect(generatedToken).NotTo(o.BeEmpty())
		}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())

		g.By("Forcing a sync with a ScyllaDB argument change")
		updateScyllaDBDatacenter(ctx, env, sdc.Name, func(sdc *scyllav1alpha1.ScyllaDBDatacenter) {
			sdc.Spec.ScyllaDB.AdditionalScyllaDBArguments = []string{"--logger-log-level=compaction=debug"}
		})
		waitForObservedGeneration(ctx, env, sdc.Name)

		g.By("Verifying the generated token is kept")
		secret, err := env.TypedKubeClient().CoreV1().Secrets(env.Namespace()).Get(ctx, agentAuthTokenSecretName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		token, err := helpers.GetAgentAuthTokenFromSecret(secret)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(token).To(o.Equal(generatedToken))

		g.By("Creating an override Secret with a token of its own")
		const overrideToken = "envtest-override-token"
		overrideTokenConfig, err := helpers.GetAgentAuthTokenConfig(overrideToken)
		o.Expect(err).NotTo(o.HaveOccurred())
		overrideSecret, err := env.TypedKubeClient().CoreV1().Secrets(env.Namespace()).Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "agent-auth-token-override",
				Namespace: env.Namespace(),
			},
			Data: map[string][]byte{
				naming.ScyllaAgentAuthTokenFileName: overrideTokenConfig,
			},
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Pointing the ScyllaDBDatacenter at the override Secret")
		updateScyllaDBDatacenter(ctx, env, sdc.Name, func(sdc *scyllav1alpha1.ScyllaDBDatacenter) {
			if sdc.Annotations == nil {
				sdc.Annotations = map[string]string{}
			}
			sdc.Annotations[naming.ScyllaDBManagerAgentAuthTokenOverrideSecretRefAnnotation] = overrideSecret.Name
		})

		g.By("Waiting for the agent auth token Secret to carry the override token")
		waitForAgentAuthToken(ctx, env, agentAuthTokenSecretName, overrideToken)

		// A custom agent config takes precedence over the override.
		g.By("Creating a custom agent config Secret with a token of its own")
		const customConfigToken = "envtest-custom-config-token"
		customConfig, err := helpers.GetAgentAuthTokenConfig(customConfigToken)
		o.Expect(err).NotTo(o.HaveOccurred())
		customConfigSecret, err := env.TypedKubeClient().CoreV1().Secrets(env.Namespace()).Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "agent-custom-config",
				Namespace: env.Namespace(),
			},
			Data: map[string][]byte{
				naming.ScyllaAgentConfigFileName: customConfig,
			},
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Referencing the custom agent config Secret from the rack template while the override Secret stays referenced")
		updateScyllaDBDatacenter(ctx, env, sdc.Name, func(sdc *scyllav1alpha1.ScyllaDBDatacenter) {
			sdc.Spec.RackTemplate.ScyllaDBManagerAgent = &scyllav1alpha1.ScyllaDBManagerAgentTemplate{
				CustomConfigSecretRef: pointer.Ptr(customConfigSecret.Name),
			}
		})

		g.By("Waiting for the agent auth token Secret to carry the custom config token")
		waitForAgentAuthToken(ctx, env, agentAuthTokenSecretName, customConfigToken)
	})

	g.It("should manage the CA, serving and client certificates with the CA bundles", func(ctx g.SpecContext) {
		g.By("Running ScyllaDBDatacenter controller")
		runScyllaDBDatacenterController(ctx, env)

		g.By("Creating ScyllaOperatorConfig singleton")
		createScyllaOperatorConfig(ctx, env)

		g.By("Creating a ScyllaDBDatacenter with a single rack")
		sdc := makeEnvtestScyllaDBDatacenter(env.Namespace(), []string{rackName})
		sdc, err := env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).Create(ctx, sdc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for the certificate Secrets and CA bundle ConfigMaps to be created")
		o.Eventually(func(eo o.Gomega, ctx context.Context) {
			for _, secretName := range []string{
				naming.GetScyllaClusterLocalClientCAName(sdc.Name),
				naming.GetScyllaClusterLocalUserAdminCertName(sdc.Name),
				naming.GetScyllaClusterLocalServingCAName(sdc.Name),
				naming.GetScyllaClusterLocalServingCertName(sdc.Name),
			} {
				secret, err := env.TypedKubeClient().CoreV1().Secrets(env.Namespace()).Get(ctx, secretName, metav1.GetOptions{})
				eo.Expect(err).NotTo(o.HaveOccurred())
				eo.Expect(metav1.IsControlledBy(secret, sdc)).To(o.BeTrue(), "Secret %q", secretName)
				eo.Expect(secret.Type).To(o.Equal(corev1.SecretTypeTLS), "Secret %q", secretName)

				certBytes, keyBytes, err := okubecrypto.GetCertKeyDataFromSecret(secret)
				eo.Expect(err).NotTo(o.HaveOccurred(), "Secret %q", secretName)
				eo.Expect(certBytes).NotTo(o.BeEmpty(), "Secret %q", secretName)
				eo.Expect(keyBytes).NotTo(o.BeEmpty(), "Secret %q", secretName)
			}

			for _, configMapName := range []string{
				naming.GetScyllaClusterLocalClientCAName(sdc.Name),
				naming.GetScyllaClusterLocalServingCAName(sdc.Name),
			} {
				cm, err := env.TypedKubeClient().CoreV1().ConfigMaps(env.Namespace()).Get(ctx, configMapName, metav1.GetOptions{})
				eo.Expect(err).NotTo(o.HaveOccurred())
				eo.Expect(metav1.IsControlledBy(cm, sdc)).To(o.BeTrue(), "ConfigMap %q", configMapName)

				caBundle, err := okubecrypto.GetCABundleDataFromConfigMap(cm)
				eo.Expect(err).NotTo(o.HaveOccurred(), "ConfigMap %q", configMapName)
				eo.Expect(caBundle).NotTo(o.BeEmpty(), "ConfigMap %q", configMapName)
			}

			connectionConfigsSecret, err := env.TypedKubeClient().CoreV1().Secrets(env.Namespace()).Get(ctx, naming.GetScyllaClusterLocalAdminCQLConnectionConfigsName(sdc.Name), metav1.GetOptions{})
			eo.Expect(err).NotTo(o.HaveOccurred())
			eo.Expect(metav1.IsControlledBy(connectionConfigsSecret, sdc)).To(o.BeTrue())
			eo.Expect(connectionConfigsSecret.Data).NotTo(o.BeEmpty())
		}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())
	})
})

// waitForAgentAuthToken waits for the named agent auth token Secret to carry the given token.
func waitForAgentAuthToken(ctx context.Context, e *envtest.Environment, secretName, token string) {
	g.GinkgoHelper()

	o.Eventually(func(eo o.Gomega, ctx context.Context) {
		secret, err := e.TypedKubeClient().CoreV1().Secrets(e.Namespace()).Get(ctx, secretName, metav1.GetOptions{})
		eo.Expect(err).NotTo(o.HaveOccurred())

		actualToken, err := helpers.GetAgentAuthTokenFromSecret(secret)
		eo.Expect(err).NotTo(o.HaveOccurred())
		eo.Expect(actualToken).To(o.Equal(token))
	}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())
}
