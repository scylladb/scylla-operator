// Copyright (C) 2026 ScyllaDB

package utils

import (
	"context"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
)

// SetUpObjectStorageCredentials creates a Secret with the credentials of the given object storage in the namespace and
// mounts it into the ScyllaDB Manager Agent of every rack of the ScyllaCluster. When the S3 settings carry a custom
// ScyllaDB Manager Agent config, it is stored in a Secret of its own and set as the racks' agent config.
// The ScyllaCluster is modified in place and has to be created afterwards.
func SetUpObjectStorageCredentials(ctx context.Context, ns string, nsClient framework.Client, sc *scyllav1.ScyllaCluster, objectStorageSettings framework.ClusterObjectStorageSettings) {
	g.GinkgoHelper()

	o.Expect(objectStorageSettings.Type()).To(o.BeElementOf(framework.ObjectStorageTypeGCS, framework.ObjectStorageTypeS3))
	switch objectStorageSettings.Type() {
	case framework.ObjectStorageTypeGCS:
		gcServiceAccountKey := objectStorageSettings.GCSServiceAccountKey()
		o.Expect(gcServiceAccountKey).NotTo(o.BeEmpty())

		setUpGCSCredentials(ctx, nsClient.KubeClient().CoreV1(), sc, ns, gcServiceAccountKey)

	case framework.ObjectStorageTypeS3:
		s3CredentialsFile := objectStorageSettings.S3CredentialsFile()
		o.Expect(s3CredentialsFile).NotTo(o.BeEmpty())

		setUpS3Credentials(ctx, nsClient.KubeClient().CoreV1(), sc, ns, s3CredentialsFile, objectStorageSettings.S3AgentConfig())

	}
}

func setUpGCSCredentials(ctx context.Context, coreClient corev1client.CoreV1Interface, sc *scyllav1.ScyllaCluster, namespace string, serviceAccountKey []byte) {
	g.GinkgoHelper()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "gcs-service-account-key-",
		},
		Data: map[string][]byte{
			"gcs-service-account.json": serviceAccountKey,
		},
	}

	secret, err := coreClient.Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	for i := range sc.Spec.Datacenter.Racks {
		sc.Spec.Datacenter.Racks[i].Volumes = append(sc.Spec.Datacenter.Racks[i].Volumes, corev1.Volume{
			Name: "gcs-service-account",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secret.Name,
					Items: []corev1.KeyToPath{
						{
							Key:  "gcs-service-account.json",
							Path: "gcs-service-account.json",
						},
					},
				},
			},
		})
		sc.Spec.Datacenter.Racks[i].AgentVolumeMounts = append(sc.Spec.Datacenter.Racks[i].AgentVolumeMounts, corev1.VolumeMount{
			Name:      "gcs-service-account",
			ReadOnly:  true,
			MountPath: "/etc/scylla-manager-agent/gcs-service-account.json",
			SubPath:   "gcs-service-account.json",
		})
	}
}

func setUpS3Credentials(ctx context.Context, coreClient corev1client.CoreV1Interface, sc *scyllav1.ScyllaCluster, namespace string, s3CredentialsFile []byte, s3AgentConfig []byte) {
	g.GinkgoHelper()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "s3-credentials-file-",
		},
		Data: map[string][]byte{
			"credentials": s3CredentialsFile,
		},
	}

	secret, err := coreClient.Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	var agentConfigSecretName string
	if len(s3AgentConfig) > 0 {
		agentConfigSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "s3-agent-config-",
			},
			Data: map[string][]byte{
				naming.ScyllaAgentConfigFileName: s3AgentConfig,
			},
		}

		agentConfigSecret, err = coreClient.Secrets(namespace).Create(ctx, agentConfigSecret, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		agentConfigSecretName = agentConfigSecret.Name
	}

	for i := range sc.Spec.Datacenter.Racks {
		sc.Spec.Datacenter.Racks[i].Volumes = append(sc.Spec.Datacenter.Racks[i].Volumes, corev1.Volume{
			Name: "aws-credentials",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secret.Name,
					Items: []corev1.KeyToPath{
						{
							Key:  "credentials",
							Path: "credentials",
						},
					},
				},
			},
		})
		sc.Spec.Datacenter.Racks[i].AgentVolumeMounts = append(sc.Spec.Datacenter.Racks[i].AgentVolumeMounts, corev1.VolumeMount{
			Name:      "aws-credentials",
			ReadOnly:  true,
			MountPath: "/var/lib/scylla-manager/.aws/credentials",
			SubPath:   "credentials",
		})

		// The custom agent config must carry no auth token: a token there would take precedence over the one the
		// operator provisions, including a shared one referenced through the agent auth token override annotation.
		if len(agentConfigSecretName) > 0 {
			sc.Spec.Datacenter.Racks[i].ScyllaAgentConfig = agentConfigSecretName
		}
	}
}
