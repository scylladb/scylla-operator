// Copyright (C) 2021 ScyllaDB

package framework

import (
	"context"
	"fmt"
	"os"
	"path"
	"strconv"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configassets "github.com/scylladb/scylla-operator/assets/config"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	scyllafixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scylla"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryutilwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
)

const (
	ServiceAccountName                   = "e2e-user"
	ServiceAccountTokenSecretName        = "e2e-user-token"
	serviceAccountWaitTimeout            = 1 * time.Minute
	serviceAccountTokenSecretWaitTimeout = 1 * time.Minute
)

type Framework struct {
	FullClient

	name string

	cluster        *Cluster
	workerClusters map[string]*Cluster
}

var _ FullClientInterface = &Framework{}
var _ ClusterInterface = &Framework{}

func NewFramework(namePrefix string) *Framework {
	var err error

	e2eArtifactsDir := ""
	if len(TestContext.ArtifactsDir) != 0 {
		e2eArtifactsDir = path.Join(TestContext.ArtifactsDir, "e2e")
		err = os.Mkdir(e2eArtifactsDir, 0777)
		if !os.IsExist(err) {
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	}

	f := &Framework{
		FullClient: FullClient{},

		name: names.SimpleNameGenerator.GenerateName(fmt.Sprintf("%s-", namePrefix)),

		workerClusters: map[string]*Cluster{},
	}

	clusterName := f.name
	clusterE2EArtifactsDir := ""
	if len(e2eArtifactsDir) != 0 {
		clusterE2EArtifactsDir = path.Join(e2eArtifactsDir, "cluster")
		err = os.Mkdir(clusterE2EArtifactsDir, 0777)
		if !os.IsExist(err) {
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	}
	f.cluster = NewCluster(
		clusterName,
		clusterE2EArtifactsDir,
		TestContext.RestConfig,
		func(ctx context.Context, adminClient kubernetes.Interface, adminClientConfig *restclient.Config) (*corev1.Namespace, Client) {
			return createUserNamespace(ctx, clusterName, f.CommonLabels(), adminClient, adminClientConfig)
		},
	)
	f.FullClient.AdminClient.Config = f.cluster.AdminClientConfig()

	for workerClusterKey, workerClusterRestConfig := range TestContext.WorkerRestConfigs {
		workerClusterE2EArtifactsDir := ""
		if len(e2eArtifactsDir) != 0 {
			workerClusterE2EArtifactsDir = path.Join(e2eArtifactsDir, "workers", workerClusterKey)
			err = os.MkdirAll(workerClusterE2EArtifactsDir, 0777)
			if !os.IsExist(err) {
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		}
		workerClusterName := fmt.Sprintf("%s-%s", f.name, workerClusterKey)
		f.workerClusters[workerClusterKey] = NewCluster(
			workerClusterName,
			workerClusterE2EArtifactsDir,
			workerClusterRestConfig,
			func(ctx context.Context, adminClient kubernetes.Interface, adminClientConfig *restclient.Config) (*corev1.Namespace, Client) {
				return createUserNamespace(ctx, workerClusterName, f.CommonLabels(), adminClient, adminClientConfig)
			},
		)
	}

	g.BeforeEach(f.beforeEach)
	g.JustAfterEach(f.justAfterEach)
	g.AfterEach(f.afterEach)

	return f
}

func (f *Framework) Cluster() ClusterInterface {
	return f.cluster
}

func (f *Framework) WorkerClusters() map[string]ClusterInterface {
	m := make(map[string]ClusterInterface, len(f.workerClusters))
	for k, v := range f.workerClusters {
		m[k] = v
	}
	return m
}

func (f *Framework) Name() string {
	return f.cluster.Name()
}

func (f *Framework) Namespace() string {
	return f.cluster.defaultNamespace.Name
}

func (f *Framework) DefaultNamespaceIfAny() (*corev1.Namespace, Client, bool) {
	return f.cluster.DefaultNamespaceIfAny()
}

func (f *Framework) GetDefaultArtifactsDir() string {
	return f.cluster.GetArtifactsDir()
}

func (f *Framework) GetIngressAddress(hostname string) string {
	if TestContext.IngressController == nil || len(TestContext.IngressController.Address) == 0 {
		return hostname
	}

	return TestContext.IngressController.Address
}

func (f *Framework) FieldManager() string {
	return FieldManager(f.ClientConfig().UserAgent, f.Namespace())
}

func (f *Framework) CommonLabels() map[string]string {
	return map[string]string{
		"e2e":       "scylla-operator",
		"framework": f.name,
	}
}

func (f *Framework) GetDefaultScyllaCluster() *scyllav1.ScyllaCluster {
	renderArgs := map[string]any{
		"scyllaDBVersion":             TestContext.ScyllaDBVersion,
		"scyllaDBManagerVersion":      TestContext.ScyllaDBManagerAgentVersion,
		"nodeServiceType":             TestContext.ScyllaClusterOptions.ExposeOptions.NodeServiceType,
		"nodesBroadcastAddressType":   TestContext.ScyllaClusterOptions.ExposeOptions.NodesBroadcastAddressType,
		"clientsBroadcastAddressType": TestContext.ScyllaClusterOptions.ExposeOptions.ClientsBroadcastAddressType,
		"storageClassName":            TestContext.ScyllaClusterOptions.StorageClassName,
	}

	sc, _, err := scyllafixture.ScyllaClusterTemplate.RenderObject(renderArgs)
	o.Expect(err).NotTo(o.HaveOccurred())

	return sc
}

func (f *Framework) GetDefaultZonalScyllaClusterWithThreeRacks() *scyllav1.ScyllaCluster {
	renderArgs := map[string]any{
		"scyllaDBVersion":             TestContext.ScyllaDBVersion,
		"scyllaDBManagerVersion":      TestContext.ScyllaDBManagerAgentVersion,
		"nodeServiceType":             TestContext.ScyllaClusterOptions.ExposeOptions.NodeServiceType,
		"nodesBroadcastAddressType":   TestContext.ScyllaClusterOptions.ExposeOptions.NodesBroadcastAddressType,
		"clientsBroadcastAddressType": TestContext.ScyllaClusterOptions.ExposeOptions.ClientsBroadcastAddressType,
		"storageClassName":            TestContext.ScyllaClusterOptions.StorageClassName,
		"rackNames":                   []string{"a", "b", "c"},
	}

	sc, _, err := scyllafixture.ZonalScyllaClusterTemplate.RenderObject(renderArgs)
	o.Expect(err).NotTo(o.HaveOccurred())

	return sc
}

func (f *Framework) GetDefaultScyllaDBDatacenter() *scyllav1alpha1.ScyllaDBDatacenter {
	renderArgs := map[string]any{
		"scyllaDBVersion":                TestContext.ScyllaDBVersion,
		"scyllaDBManagerVersion":         TestContext.ScyllaDBManagerAgentVersion,
		"nodeServiceType":                TestContext.ScyllaClusterOptions.ExposeOptions.NodeServiceType,
		"nodesBroadcastAddressType":      TestContext.ScyllaClusterOptions.ExposeOptions.NodesBroadcastAddressType,
		"clientsBroadcastAddressType":    TestContext.ScyllaClusterOptions.ExposeOptions.ClientsBroadcastAddressType,
		"storageClassName":               TestContext.ScyllaClusterOptions.StorageClassName,
		"scyllaDBRepository":             configassets.ScyllaDBImageRepository,
		"scyllaDBManagerAgentRepository": configassets.ScyllaDBManagerAgentImageRepository,
	}

	sdc, _, err := scyllafixture.ScyllaDBDatacenterTemplate.RenderObject(renderArgs)
	o.Expect(err).NotTo(o.HaveOccurred())

	return sdc
}

func (f *Framework) GetDefaultScyllaDBCluster(rkcMap map[string]*scyllav1alpha1.RemoteKubernetesCluster) *scyllav1alpha1.ScyllaDBCluster {
	renderArgs := map[string]any{
		"scyllaDBVersion":                TestContext.ScyllaDBVersion,
		"scyllaDBManagerVersion":         TestContext.ScyllaDBManagerAgentVersion,
		"nodeServiceType":                TestContext.ScyllaClusterOptions.ExposeOptions.NodeServiceType,
		"nodesBroadcastAddressType":      TestContext.ScyllaClusterOptions.ExposeOptions.NodesBroadcastAddressType,
		"clientsBroadcastAddressType":    TestContext.ScyllaClusterOptions.ExposeOptions.ClientsBroadcastAddressType,
		"storageClassName":               TestContext.ScyllaClusterOptions.StorageClassName,
		"remoteKubernetesClusterMap":     rkcMap,
		"scyllaDBRepository":             configassets.ScyllaDBImageRepository,
		"scyllaDBManagerAgentRepository": configassets.ScyllaDBManagerAgentImageRepository,
	}

	sc, _, err := scyllafixture.ScyllaDBClusterTemplate.RenderObject(renderArgs)
	o.Expect(err).NotTo(o.HaveOccurred())

	return sc
}

func (f *Framework) AddCleaners(cleaners ...Cleaner) {
	f.cluster.AddCleaners(cleaners...)
}

func (f *Framework) AddCollectors(collectors ...Collector) {
	f.cluster.AddCollectors(collectors...)
}

func (f *Framework) CreateUserNamespace(ctx context.Context) (*corev1.Namespace, Client) {
	return f.cluster.CreateUserNamespace(ctx)
}

func (f *Framework) GetClusterObjectStorageSettings() (ClusterObjectStorageSettings, bool) {
	if TestContext.ClusterObjectStorageSettings == nil {
		return ClusterObjectStorageSettings{}, false
	}
	return *TestContext.ClusterObjectStorageSettings, true
}

func (f *Framework) GetObjectStorageSettingsForWorkerCluster(workerName string) ClusterObjectStorageSettings {
	settings, ok := TestContext.WorkerClusterObjectStorageSettings[workerName]
	o.Expect(ok).To(o.BeTrue(), fmt.Sprintf("no object storage configured for worker %q", workerName))
	return settings
}

func (f *Framework) beforeEach(ctx context.Context) {
	ns, nsClient := f.CreateUserNamespace(ctx)
	f.cluster.defaultNamespace = ns
	f.cluster.defaultClient = nsClient
	f.FullClient.Client = nsClient
}

func (f *Framework) justAfterEach(ctx context.Context) {
	ginkgoNamespace := f.cluster.defaultNamespace.Name
	f.cluster.Collect(ctx, ginkgoNamespace)
	for _, c := range f.workerClusters {
		c.Collect(ctx, ginkgoNamespace)
	}
}

func (f *Framework) afterEach(ctx context.Context) {
	nilClient := Client{
		Config: nil,
	}

	f.cluster.defaultNamespace = nil
	f.cluster.defaultClient = nilClient
	f.FullClient.Client = nilClient

	shouldCleanup := true
	switch TestContext.CleanupPolicy {
	case CleanupPolicyNever:
		shouldCleanup = false
	case CleanupPolicyOnSuccess:
		if g.CurrentSpecReport().Failed() {
			shouldCleanup = false
		}
	case CleanupPolicyAlways:
	default:
		g.Fail(fmt.Sprintf("unexpected cleanup policy %q", TestContext.CleanupPolicy))
	}

	if shouldCleanup {
		f.cluster.Cleanup(ctx)
		for _, c := range f.workerClusters {
			c.Cleanup(ctx)
		}
	}
}

func createUserNamespace(ctx context.Context, clusterName string, labels map[string]string, adminClient kubernetes.Interface, adminClientConfig *restclient.Config) (*corev1.Namespace, Client) {
	g.By("Creating a new namespace")
	var ns *corev1.Namespace
	generateName := func() string {
		return names.SimpleNameGenerator.GenerateName(fmt.Sprintf("e2e-test-%s-", clusterName))
	}
	name := generateName()
	sr := g.CurrentSpecReport()
	err := apimachineryutilwait.PollImmediate(2*time.Second, 30*time.Second, func() (bool, error) {
		var err error
		// We want to know the name ahead, even if the api call fails.
		ns, err = adminClient.CoreV1().Namespaces().Create(
			ctx,
			&corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:   name,
					Labels: labels,
					Annotations: map[string]string{
						"ginkgo-parallel-process": strconv.Itoa(sr.ParallelProcess),
						"ginkgo-full-text":        sr.FullText(),
					},
				},
			},
			metav1.CreateOptions{},
		)
		if err != nil {
			if apierrors.IsAlreadyExists(err) {
				// regenerate on conflict
				Infof("Namespace name %q was already taken, generating a new name and retrying", name)
				name = generateName()
				return false, nil
			}
			return true, err
		}
		return true, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	Infof("Created namespace %q.", ns.Name)

	// Create user service account.
	userSA, err := adminClient.CoreV1().ServiceAccounts(ns.Name).Create(ctx, &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name: ServiceAccountName,
		},
	}, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	// Grant it edit permission in this namespace.
	_, err = adminClient.RbacV1().RoleBindings(ns.Name).Create(ctx, &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: userSA.Name,
		},
		Subjects: []rbacv1.Subject{
			{
				APIGroup:  corev1.GroupName,
				Kind:      rbacv1.ServiceAccountKind,
				Namespace: userSA.Namespace,
				Name:      userSA.Name,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     "admin",
		},
	}, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	// Grant it permission needed for ScyllaClusters
	_, err = adminClient.RbacV1().RoleBindings(ns.Name).Create(ctx, &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: userSA.Name + "-scyllacluster-member",
		},
		Subjects: []rbacv1.Subject{
			{
				APIGroup:  corev1.GroupName,
				Kind:      rbacv1.ServiceAccountKind,
				Namespace: userSA.Namespace,
				Name:      userSA.Name,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     "scyllacluster-member",
		},
	}, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	// Create a Role with ephemeral containers permissions
	_, err = adminClient.RbacV1().Roles(ns.Name).Create(ctx, &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ephemeral-containers",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"pods/ephemeralcontainers"},
				Verbs:     []string{"get", "list", "patch", "update"},
			},
		},
	}, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	// Grant the user ServiceAccount ephemeral containers permissions
	_, err = adminClient.RbacV1().RoleBindings(ns.Name).Create(ctx, &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: userSA.Name + "-ephemeral-containers",
		},
		Subjects: []rbacv1.Subject{
			{
				APIGroup:  corev1.GroupName,
				Kind:      rbacv1.ServiceAccountKind,
				Namespace: userSA.Namespace,
				Name:      userSA.Name,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     "ephemeral-containers",
		},
	}, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	// Create a service account token Secret for the user ServiceAccount.
	userSATokenSecret, err := adminClient.CoreV1().Secrets(ns.Name).Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: ServiceAccountTokenSecretName,
			Annotations: map[string]string{
				corev1.ServiceAccountNameKey: userSA.Name,
			},
		},
		Type: corev1.SecretTypeServiceAccountToken,
	}, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	By("Waiting for service account token Secret %q in namespace %q.", userSATokenSecret.Name, userSATokenSecret.Namespace)
	ctxUserSATokenSecret, ctxUserSATokenSecretCancel := context.WithTimeout(ctx, serviceAccountTokenSecretWaitTimeout)
	defer ctxUserSATokenSecretCancel()
	userSATokenSecret, err = WaitForServiceAccountTokenSecret(ctxUserSATokenSecret, adminClient.CoreV1(), userSATokenSecret.Namespace, userSATokenSecret.Name)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(userSATokenSecret.Data).To(o.HaveKey(corev1.ServiceAccountTokenKey))

	token := userSATokenSecret.Data[corev1.ServiceAccountTokenKey]
	o.Expect(token).NotTo(o.BeEmpty())

	// Create a restricted client using the user SA.
	userClientConfig := restclient.AnonymousClientConfig(adminClientConfig)
	userClientConfig.BearerToken = string(token)

	// Wait for default ServiceAccount.
	By("Waiting for default ServiceAccount in namespace %q.", ns.Name)
	ctxSa, ctxSaCancel := context.WithTimeout(ctx, serviceAccountWaitTimeout)
	defer ctxSaCancel()
	_, err = WaitForServiceAccount(ctxSa, adminClient.CoreV1(), ns.Name, "default")
	o.Expect(err).NotTo(o.HaveOccurred())

	// Waits for the configmap kube-root-ca.crt containing CA trust bundle so that pods do not have to retry mounting
	// the config map because it creates noise and slows the startup.
	By("Waiting for kube-root-ca.crt in namespace %q.", ns.Name)
	_, err = controllerhelpers.WaitForConfigMapState(
		ctx,
		adminClient.CoreV1().ConfigMaps(ns.Name),
		"kube-root-ca.crt",
		controllerhelpers.WaitForStateOptions{},
		func(configMap *corev1.ConfigMap) (bool, error) {
			return true, nil
		},
	)
	o.Expect(err).NotTo(o.HaveOccurred())

	return ns, Client{Config: userClientConfig}
}
