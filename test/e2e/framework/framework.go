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
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	scyllafixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scylla"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
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

	name            string
	e2eArtifactsDir string

	clusters []*Cluster
}

var _ FullClientInterface = &Framework{}
var _ ClusterInterface = &Framework{}

func NewFramework(namePrefix string) *Framework {
	var err error

	f := &Framework{
		name:            names.SimpleNameGenerator.GenerateName(fmt.Sprintf("%s-", namePrefix)),
		e2eArtifactsDir: "",
		FullClient:      FullClient{},
		clusters:        make([]*Cluster, 0, len(TestContext.RestConfigs)),
	}

	if len(TestContext.ArtifactsDir) != 0 {
		f.e2eArtifactsDir = path.Join(TestContext.ArtifactsDir, "e2e")
		err = os.Mkdir(f.e2eArtifactsDir, 0777)
		if !os.IsExist(err) {
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	}

	o.Expect(TestContext.RestConfigs).NotTo(o.BeEmpty())
	for i, restConfig := range TestContext.RestConfigs {
		clusterName := fmt.Sprintf("%s-%d", f.name, i)

		clusterE2EArtifactsDir := ""
		if len(f.e2eArtifactsDir) != 0 {
			clusterE2EArtifactsDir = path.Join(f.e2eArtifactsDir, fmt.Sprintf("cluster-%d", i))
			err = os.Mkdir(clusterE2EArtifactsDir, 0777)
			if !os.IsExist(err) {
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		}

		c := NewCluster(
			clusterName,
			clusterE2EArtifactsDir,
			restConfig,
			func(ctx context.Context, adminClient kubernetes.Interface, adminClientConfig *restclient.Config) (*corev1.Namespace, Client) {
				return CreateUserNamespace(ctx, clusterName, f.CommonLabels(), adminClient, adminClientConfig)
			},
		)
		f.clusters = append(f.clusters, c)
	}

	f.FullClient.AdminClient.Config = f.defaultCluster().AdminClientConfig()

	g.BeforeEach(f.beforeEach)
	g.JustAfterEach(f.justAfterEach)
	g.AfterEach(f.afterEach)

	return f
}

func (f *Framework) Cluster(idx int) ClusterInterface {
	o.Expect(idx).NotTo(o.BeNumerically(">=", len(f.clusters)))
	return f.clusters[idx]
}

func (f *Framework) Namespace() string {
	return f.defaultCluster().defaultNamespace.Name
}

func (f *Framework) DefaultNamespaceIfAny() (*corev1.Namespace, Client, bool) {
	return f.defaultCluster().DefaultNamespaceIfAny()
}

func (f *Framework) GetDefaultArtifactsDir() string {
	return f.defaultCluster().GetArtifactsDir()
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
		"scyllaDBVersion":             configassets.Project.Operator.ScyllaDBVersion,
		"scyllaDBManagerVersion":      configassets.Project.Operator.ScyllaDBManagerAgentVersion,
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
		"scyllaDBVersion":             configassets.Project.Operator.ScyllaDBVersion,
		"scyllaDBManagerVersion":      configassets.Project.Operator.ScyllaDBManagerAgentVersion,
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

func (f *Framework) AddCleaners(cleaners ...CleanupInterface) {
	f.defaultCluster().AddCleaners(cleaners...)
}

func (f *Framework) CreateUserNamespace(ctx context.Context) (*corev1.Namespace, Client) {
	return f.defaultCluster().CreateUserNamespace(ctx)
}

func (f *Framework) GetObjectStorageType() ObjectStorageType {
	return TestContext.ObjectStorageType
}

func (f *Framework) GetObjectStorageProvider() string {
	switch TestContext.ObjectStorageType {
	case ObjectStorageTypeGCS:
		return "gcs"
	case ObjectStorageTypeS3:
		return "s3"
	default:
		return ""
	}
}

func (f *Framework) GetObjectStorageBucket() string {
	return TestContext.ObjectStorageBucket
}

func (f *Framework) GetGCSServiceAccountKey() []byte {
	return TestContext.GCSServiceAccountKey
}

func (f *Framework) GetS3CredentialsFile() []byte {
	return TestContext.S3CredentialsFile
}

func (f *Framework) defaultCluster() *Cluster {
	o.Expect(f.clusters).NotTo(o.BeEmpty())
	return f.clusters[0]
}

func (f *Framework) beforeEach(ctx context.Context) {
	ns, nsClient := f.defaultCluster().CreateUserNamespace(ctx)
	f.defaultCluster().defaultNamespace = ns
	f.defaultCluster().defaultClient = nsClient
	f.FullClient.Client = nsClient
}

func (f *Framework) justAfterEach(ctx context.Context) {
	ginkgoNamespace := f.defaultCluster().defaultNamespace.Name
	for _, c := range f.clusters {
		c.Collect(ctx, ginkgoNamespace)
	}
}

func (f *Framework) afterEach(ctx context.Context) {
	nilClient := Client{
		Config: nil,
	}

	f.defaultCluster().defaultNamespace = nil
	f.defaultCluster().defaultClient = nilClient
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
		for _, c := range f.clusters {
			c.Cleanup(ctx)
		}
	}
}

func CreateUserNamespace(ctx context.Context, clusterName string, labels map[string]string, adminClient kubernetes.Interface, adminClientConfig *restclient.Config) (*corev1.Namespace, Client) {
	g.By("Creating a new namespace")
	var ns *corev1.Namespace
	generateName := func() string {
		return names.SimpleNameGenerator.GenerateName(fmt.Sprintf("e2e-test-%s-", clusterName))
	}
	name := generateName()
	sr := g.CurrentSpecReport()
	err := wait.PollImmediate(2*time.Second, 30*time.Second, func() (bool, error) {
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

	// Grant it permission needed for ScyllaCLusters
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
