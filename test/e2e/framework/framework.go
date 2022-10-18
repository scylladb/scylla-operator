// Copyright (C) 2021 ScyllaDB

package framework

import (
	"context"
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"os"
	"path"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllaclientset "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	watchtools "k8s.io/client-go/tools/watch"
	"k8s.io/klog/v2"
)

const (
	ServiceAccountName        = "e2e-user"
	serviceAccountWaitTimeout = 1 * time.Minute
)

type Framework struct {
	name      string
	namespace *corev1.Namespace

	adminClientConfig *restclient.Config
	clientConfig      *restclient.Config
	username          string
}

func NewFramework(name string) *Framework {
	uniqueName := names.SimpleNameGenerator.GenerateName(fmt.Sprintf("%s-", name))

	adminClientConfig := restclient.CopyConfig(TestContext.RestConfig)
	adminClientConfig.UserAgent = "scylla-operator-e2e"
	adminClientConfig.QPS = 20
	adminClientConfig.Burst = 50

	f := &Framework{
		name:              uniqueName,
		username:          "admin",
		adminClientConfig: adminClientConfig,
	}

	g.BeforeEach(f.beforeEach)
	g.AfterEach(f.afterEach)

	return f
}

func (f *Framework) Namespace() string {
	return f.namespace.Name
}

func (f *Framework) Username() string {
	return f.username
}

func (f *Framework) FieldManager() string {
	h := sha512.Sum512([]byte(fmt.Sprintf("scylla-operator-e2e-%s", f.Namespace())))
	return base64.StdEncoding.EncodeToString(h[:])
}

func (f *Framework) ClientConfig() *restclient.Config {
	return f.clientConfig
}

func (f *Framework) AdminClientConfig() *restclient.Config {
	return f.adminClientConfig
}

func (f *Framework) DiscoveryClient() *discovery.DiscoveryClient {
	client, err := discovery.NewDiscoveryClientForConfig(f.ClientConfig())
	o.Expect(err).NotTo(o.HaveOccurred())
	return client
}

func (f *Framework) DynamicClient() dynamic.Interface {
	client, err := dynamic.NewForConfig(f.ClientConfig())
	o.Expect(err).NotTo(o.HaveOccurred())
	return client
}

func (f *Framework) DynamicAdminClient() dynamic.Interface {
	client, err := dynamic.NewForConfig(f.AdminClientConfig())
	o.Expect(err).NotTo(o.HaveOccurred())
	return client
}

func (f *Framework) KubeClient() *kubernetes.Clientset {
	client, err := kubernetes.NewForConfig(f.ClientConfig())
	o.Expect(err).NotTo(o.HaveOccurred())
	return client
}

func (f *Framework) KubeAdminClient() *kubernetes.Clientset {
	client, err := kubernetes.NewForConfig(f.AdminClientConfig())
	o.Expect(err).NotTo(o.HaveOccurred())
	return client
}

func (f *Framework) ScyllaClient() *scyllaclientset.Clientset {
	client, err := scyllaclientset.NewForConfig(f.ClientConfig())
	o.Expect(err).NotTo(o.HaveOccurred())
	return client
}
func (f *Framework) ScyllaAdminClient() *scyllaclientset.Clientset {
	client, err := scyllaclientset.NewForConfig(f.AdminClientConfig())
	o.Expect(err).NotTo(o.HaveOccurred())
	return client
}

func (f *Framework) setupNamespace(ctx context.Context) {
	By("Creating a new namespace")
	var ns *corev1.Namespace
	generateName := func() string {
		return names.SimpleNameGenerator.GenerateName(fmt.Sprintf("e2e-test-%s-", f.name))
	}
	name := generateName()
	err := wait.PollImmediate(2*time.Second, 30*time.Second, func() (bool, error) {
		var err error
		// We want to know the name ahead, even if the api call fails.
		ns, err = f.KubeAdminClient().CoreV1().Namespaces().Create(
			ctx,
			&corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
					Labels: map[string]string{
						"e2e":       "scylla-operator",
						"framework": f.name,
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

	f.namespace = ns

	// Create user service account.
	userSA, err := f.KubeAdminClient().CoreV1().ServiceAccounts(ns.Name).Create(ctx, &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name: ServiceAccountName,
		},
	}, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	// Grant it edit permission in this namespace.
	_, err = f.KubeAdminClient().RbacV1().RoleBindings(ns.Name).Create(ctx, &rbacv1.RoleBinding{
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

	// Wait for user ServiceAccount.
	By(fmt.Sprintf("Waiting for ServiceAccount %q in namespace %q.", userSA.Name, userSA.Namespace))
	ctxUserSa, ctxUserSaCancel := watchtools.ContextWithOptionalTimeout(ctx, serviceAccountWaitTimeout)
	defer ctxUserSaCancel()
	userSA, err = WaitForServiceAccount(ctxUserSa, f.KubeAdminClient().CoreV1(), userSA.Namespace, userSA.Name)
	o.Expect(err).NotTo(o.HaveOccurred())

	// Create a restricted client using the default SA.
	var token []byte
	for _, secretName := range userSA.Secrets {
		secret, err := f.KubeAdminClient().CoreV1().Secrets(userSA.Namespace).Get(ctx, secretName.Name, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		if secret.Type == corev1.SecretTypeServiceAccountToken {
			name := secret.Annotations[corev1.ServiceAccountNameKey]
			uid := secret.Annotations[corev1.ServiceAccountUIDKey]
			if name == userSA.Name && uid == string(userSA.UID) {
				t, found := secret.Data[corev1.ServiceAccountTokenKey]
				if found {
					token = t
					break
				}
			}
		}
	}
	o.Expect(token).NotTo(o.HaveLen(0))

	f.clientConfig = restclient.AnonymousClientConfig(f.AdminClientConfig())
	f.clientConfig.BearerToken = string(token)

	// Wait for default ServiceAccount.
	By(fmt.Sprintf("Waiting for default ServiceAccount in namespace %q.", ns.Name))
	ctxSa, ctxSaCancel := watchtools.ContextWithOptionalTimeout(ctx, serviceAccountWaitTimeout)
	defer ctxSaCancel()
	_, err = WaitForServiceAccount(ctxSa, f.KubeAdminClient().CoreV1(), ns.Namespace, "default")
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (f *Framework) deleteNamespace(ctx context.Context, ns *corev1.Namespace) {
	By("Destroying namespace %q.", ns.Name)
	var gracePeriod int64 = 0
	var propagation = metav1.DeletePropagationForeground
	err := f.KubeAdminClient().CoreV1().Namespaces().Delete(
		ctx,
		ns.Name,
		metav1.DeleteOptions{
			GracePeriodSeconds: &gracePeriod,
			PropagationPolicy:  &propagation,
			Preconditions: &metav1.Preconditions{
				UID: &ns.UID,
			},
		},
	)
	o.Expect(err).NotTo(o.HaveOccurred())

	// We have deleted only the namespace object but it is still there with deletionTimestamp set.

	By("Waiting for namespace %q to be removed.", ns.Name)
	err = WaitForObjectDeletion(ctx, f.DynamicAdminClient(), corev1.SchemeGroupVersion.WithResource("namespaces"), "", ns.Name, &ns.UID)
	o.Expect(err).NotTo(o.HaveOccurred())
	klog.InfoS("Namespace removed.", "Namespace", ns.Name)
}

func (f *Framework) beforeEach() {
	f.setupNamespace(context.Background())
}

func (f *Framework) afterEach() {
	if f.namespace == nil {
		return
	}

	ctx, ctxCancel := context.WithCancel(context.Background())
	defer ctxCancel()

	defer func() {
		keepNamespace := false
		switch TestContext.DeleteTestingNSPolicy {
		case DeleteTestingNSPolicyNever:
			keepNamespace = true
		case DeleteTestingNSPolicyOnSuccess:
			if g.CurrentSpecReport().Failed() {
				keepNamespace = true
			}
		case DeleteTestingNSPolicyAlways:
		default:
		}

		if keepNamespace {
			By("Keeping namespace %q for debugging", f.Namespace())
			return
		}

		f.deleteNamespace(ctx, f.namespace)
		f.namespace = nil
		f.clientConfig = nil
	}()

	// Print events if the test failed.
	if g.CurrentSpecReport().Failed() {
		By(fmt.Sprintf("Collecting events from namespace %q.", f.namespace.Name))
		DumpEventsInNamespace(ctx, f.KubeAdminClient(), f.namespace.Name)
	}

	// CI can't keep namespaces alive because it could get out of resources for the other tests
	// so we need to collect the namespaced dump before destroying the namespace.
	// Collecting artifacts even for successful runs helps to verify if it went
	// as expected and the amount of data is bearable.
	if len(TestContext.ArtifactsDir) != 0 {
		By(fmt.Sprintf("Collecting dumps from namespace %q.", f.namespace.Name))

		d := path.Join(TestContext.ArtifactsDir, "e2e-namespaces")
		err := os.Mkdir(d, 0777)
		if err != nil && !os.IsExist(err) {
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		err = DumpNamespace(ctx, f.KubeAdminClient().Discovery(), f.DynamicAdminClient(), f.KubeAdminClient().CoreV1(), d, f.Namespace())
		o.Expect(err).NotTo(o.HaveOccurred())
	}
}
