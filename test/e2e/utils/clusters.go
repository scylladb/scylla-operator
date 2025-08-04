package utils

import (
	"context"
	"fmt"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/gather/collect"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

var (
	remoteKubernetesClusterResourceInfo = collect.ResourceInfo{
		Resource: scyllav1alpha1.GroupVersion.WithResource("remotekubernetesclusters"),
		Scope:    meta.RESTScopeRoot,
	}
)

// SetUpRemoteKubernetesClusters creates RemoteKubernetesCluster objects for each worker cluster.
// It returns a map of RemoteKubernetesCluster objects keyed by cluster name and a map of clusters keyed by RemoteKubernetesCluster name.
func SetUpRemoteKubernetesClusters(ctx context.Context, metaCluster framework.ClusterInterface, workerClusters map[string]framework.ClusterInterface) (map[string]*scyllav1alpha1.RemoteKubernetesCluster, map[string]framework.ClusterInterface, error) {
	framework.By("Creating RemoteKubernetesClusters")
	rkcMap := make(map[string]*scyllav1alpha1.RemoteKubernetesCluster, len(workerClusters))
	rkcClusterMap := make(map[string]framework.ClusterInterface, len(workerClusters))

	rkcSecretsNs, _ := metaCluster.CreateUserNamespace(ctx)
	for key, cluster := range workerClusters {
		tokenNs, _ := cluster.CreateUserNamespace(ctx)

		clusterName := cluster.Name()

		framework.By("Creating SA having Operator ClusterRole in cluster %q", clusterName)
		adminKubeconfig, err := GetKubeConfigHavingOperatorRemoteClusterRole(ctx, cluster.KubeAdminClient(), cluster.AdminClientConfig(), clusterName, tokenNs.Name)
		if err != nil {
			return nil, nil, fmt.Errorf("can't get kubeconfig for cluster %q: %w", clusterName, err)
		}

		kubeconfig, err := clientcmd.Write(adminKubeconfig)
		if err != nil {
			return nil, nil, fmt.Errorf("can't write kubeconfig for cluster %q: %w", clusterName, err)
		}

		rkc, err := GetRemoteKubernetesClusterWithKubeconfig(ctx, metaCluster.KubeAdminClient(), kubeconfig, clusterName, rkcSecretsNs.Name)
		if err != nil {
			return nil, nil, fmt.Errorf("can't make remotekubernetescluster for cluster %q: %w", clusterName, err)
		}

		rc := framework.NewRestoringCleaner(
			ctx,
			metaCluster.AdminClientConfig(),
			metaCluster.KubeAdminClient(),
			metaCluster.DynamicAdminClient(),
			remoteKubernetesClusterResourceInfo,
			rkc.Namespace,
			rkc.Name,
			framework.RestoreStrategyRecreate,
		)
		metaCluster.AddCleanerCollectors(rc)
		rc.DeleteObject(ctx, true)

		framework.By("Creating RemoteKubernetesCluster %q with credentials to cluster %q", rkc.Name, clusterName)
		rkc, err = metaCluster.ScyllaAdminClient().ScyllaV1alpha1().RemoteKubernetesClusters().Create(ctx, rkc, metav1.CreateOptions{})
		if err != nil {
			return nil, nil, fmt.Errorf("can't create remotekubernetescluster for cluster %q: %w", clusterName, err)
		}

		rkcMap[key] = rkc
		rkcClusterMap[rkc.Name] = cluster
	}

	for _, rkc := range rkcMap {
		err := func() error {
			framework.By("Waiting for the RemoteKubernetesCluster %q to roll out (RV=%s)", rkc.Name, rkc.ResourceVersion)
			waitCtx1, waitCtx1Cancel := ContextForRemoteKubernetesClusterRollout(ctx, rkc)
			defer waitCtx1Cancel()

			_, err := controllerhelpers.WaitForRemoteKubernetesClusterState(waitCtx1, metaCluster.ScyllaAdminClient().ScyllaV1alpha1().RemoteKubernetesClusters(), rkc.Name, controllerhelpers.WaitForStateOptions{}, IsRemoteKubernetesClusterRolledOut)
			if err != nil {
				return fmt.Errorf("can't wait for remotekubernetescluster %q to roll out: %w", rkc.Name, err)
			}

			return nil
		}()
		if err != nil {
			return nil, nil, fmt.Errorf("can't wait for remotekubernetescluster %q to roll out: %w", rkc.Name, err)
		}
	}

	return rkcMap, rkcClusterMap, nil
}

func GetRemoteKubernetesClusterWithOperatorClusterRole(ctx context.Context, kubeAdminClient kubernetes.Interface, adminConfig *rest.Config, name, namespace string) (*scyllav1alpha1.RemoteKubernetesCluster, error) {
	adminKubeconfig, err := GetKubeConfigHavingOperatorRemoteClusterRole(ctx, kubeAdminClient, adminConfig, name, namespace)
	if err != nil {
		return nil, fmt.Errorf("can't create kubeconfig: %w", err)
	}

	kubeconfig, err := clientcmd.Write(adminKubeconfig)
	if err != nil {
		return nil, fmt.Errorf("can't write kubeconfig: %w", err)
	}

	return GetRemoteKubernetesClusterWithKubeconfig(ctx, kubeAdminClient, kubeconfig, name, namespace)
}

func GetRemoteKubernetesClusterWithKubeconfig(ctx context.Context, kubeAdminClient kubernetes.Interface, kubeconfig []byte, name, namespace string) (*scyllav1alpha1.RemoteKubernetesCluster, error) {
	kubeConfigSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Data: map[string][]byte{
			"kubeconfig": kubeconfig,
		},
		Type: corev1.SecretTypeOpaque,
	}

	kubeConfigSecret, err := kubeAdminClient.CoreV1().Secrets(namespace).Create(ctx, kubeConfigSecret, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("can't create secret: %w", err)
	}

	rkc := &scyllav1alpha1.RemoteKubernetesCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: scyllav1alpha1.RemoteKubernetesClusterSpec{
			KubeconfigSecretRef: corev1.SecretReference{
				Namespace: kubeConfigSecret.Namespace,
				Name:      kubeConfigSecret.Name,
			},
		},
	}
	return rkc, nil
}

func GetKubeConfigHavingOperatorRemoteClusterRole(ctx context.Context, kubeAdminClient kubernetes.Interface, adminConfig *rest.Config, name, namespace string) (clientcmdapi.Config, error) {
	testSA, err := kubeAdminClient.CoreV1().ServiceAccounts(namespace).Create(ctx, &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: name,
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return clientcmdapi.Config{}, fmt.Errorf("can't create service account: %w", err)
	}

	_, err = kubeAdminClient.RbacV1().ClusterRoleBindings().Create(ctx, &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: testSA.Name,
		},
		Subjects: []rbacv1.Subject{
			{
				APIGroup:  corev1.GroupName,
				Kind:      rbacv1.ServiceAccountKind,
				Namespace: testSA.Namespace,
				Name:      testSA.Name,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     "scylladb:controller:operator-remote",
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return clientcmdapi.Config{}, fmt.Errorf("can't create role binding: %w", err)
	}

	userSATokenSecret, err := kubeAdminClient.CoreV1().Secrets(namespace).Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: name,
			Annotations: map[string]string{
				corev1.ServiceAccountNameKey: testSA.Name,
			},
		},
		Type: corev1.SecretTypeServiceAccountToken,
	}, metav1.CreateOptions{})
	if err != nil {
		return clientcmdapi.Config{}, fmt.Errorf("can't token secret: %w", err)
	}

	ctxUserSATokenSecret, ctxUserSATokenSecretCancel := context.WithTimeout(ctx, time.Minute)
	defer ctxUserSATokenSecretCancel()
	userSATokenSecret, err = framework.WaitForServiceAccountTokenSecret(ctxUserSATokenSecret, kubeAdminClient.CoreV1(), userSATokenSecret.Namespace, userSATokenSecret.Name)
	if err != nil {
		return clientcmdapi.Config{}, fmt.Errorf("can't create role binding: %w", err)
	}

	if token, ok := userSATokenSecret.Data[corev1.ServiceAccountTokenKey]; !ok || len(token) == 0 {
		return clientcmdapi.Config{}, fmt.Errorf("token secret %q has none or empty token", naming.ObjRef(userSATokenSecret))
	}

	token := userSATokenSecret.Data[corev1.ServiceAccountTokenKey]

	clusters := make(map[string]*clientcmdapi.Cluster)
	clusters[name] = &clientcmdapi.Cluster{
		Server:                   adminConfig.Host,
		CertificateAuthorityData: adminConfig.CAData,
	}

	contexts := make(map[string]*clientcmdapi.Context)
	contexts["default"] = &clientcmdapi.Context{
		Cluster:   name,
		AuthInfo:  "default",
		Namespace: namespace,
	}

	authInfos := make(map[string]*clientcmdapi.AuthInfo)
	authInfos["default"] = &clientcmdapi.AuthInfo{
		Token: string(token),
	}

	return clientcmdapi.Config{
		Kind:           "Config",
		APIVersion:     "v1",
		Clusters:       clusters,
		Contexts:       contexts,
		AuthInfos:      authInfos,
		CurrentContext: "default",
	}, nil
}

func RegisterCollectionOfRemoteScyllaDBClusterNamespaces(ctx context.Context, sc *scyllav1alpha1.ScyllaDBCluster, rkcClusterMap map[string]framework.ClusterInterface) error {
	for _, dc := range sc.Spec.Datacenters {
		cluster := rkcClusterMap[dc.RemoteKubernetesClusterName]

		remoteNamespaceName, err := naming.RemoteNamespaceName(sc, &dc)
		if err != nil {
			return fmt.Errorf("can't get remote namespace name: %w", err)
		}

		cluster.AddCollectors(framework.NewNotFoundTolerantNamespaceCollector(cluster.AdminClientConfig(), cluster.KubeAdminClient(), cluster.DynamicAdminClient(), remoteNamespaceName))
	}

	return nil
}
