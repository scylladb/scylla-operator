package utils

import (
	"context"
	"fmt"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

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
