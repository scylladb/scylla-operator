// Copyright (C) 2026 ScyllaDB

package v1alpha1

import (
	"context"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
)

// SetUpObjectStorageCredentials creates a Secret with the credentials of the given object storage in the namespace and
// mounts it into the ScyllaDB Manager Agent of the ScyllaDBDatacenter. When the S3 settings carry a custom ScyllaDB
// Manager Agent config, it is stored in a Secret of its own and set as the agent's custom config.
// The ScyllaDBDatacenter is modified in place and has to be created afterwards.
func SetUpObjectStorageCredentials(ctx context.Context, ns string, nsClient framework.Client, sdc *scyllav1alpha1.ScyllaDBDatacenter, objectStorageSettings framework.ClusterObjectStorageSettings) {
	g.GinkgoHelper()

	o.Expect(objectStorageSettings.Type()).To(o.BeElementOf(framework.ObjectStorageTypeGCS, framework.ObjectStorageTypeS3))
	switch objectStorageSettings.Type() {
	case framework.ObjectStorageTypeGCS:
		gcServiceAccountKey := objectStorageSettings.GCSServiceAccountKey()
		o.Expect(gcServiceAccountKey).NotTo(o.BeEmpty())

		setUpGCSCredentials(ctx, nsClient.KubeClient().CoreV1(), sdc, ns, gcServiceAccountKey)

	case framework.ObjectStorageTypeS3:
		s3CredentialsFile := objectStorageSettings.S3CredentialsFile()
		o.Expect(s3CredentialsFile).NotTo(o.BeEmpty())

		setUpS3Credentials(ctx, nsClient.KubeClient().CoreV1(), sdc, ns, s3CredentialsFile, objectStorageSettings.S3AgentConfig())

	}
}

func setUpGCSCredentials(ctx context.Context, coreClient corev1client.CoreV1Interface, sdc *scyllav1alpha1.ScyllaDBDatacenter, namespace string, serviceAccountKey []byte) {
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

	sdc.Spec.RackTemplate.ScyllaDBManagerAgent.Volumes = append(sdc.Spec.RackTemplate.ScyllaDBManagerAgent.Volumes, corev1.Volume{
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

	sdc.Spec.RackTemplate.ScyllaDBManagerAgent.VolumeMounts = append(sdc.Spec.RackTemplate.ScyllaDBManagerAgent.VolumeMounts, corev1.VolumeMount{
		Name:      "gcs-service-account",
		ReadOnly:  true,
		MountPath: "/etc/scylla-manager-agent/gcs-service-account.json",
		SubPath:   "gcs-service-account.json",
	})
}

func setUpS3Credentials(ctx context.Context, coreClient corev1client.CoreV1Interface, sdc *scyllav1alpha1.ScyllaDBDatacenter, namespace string, s3CredentialsFile []byte, s3AgentConfig []byte) {
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

	sdc.Spec.RackTemplate.ScyllaDBManagerAgent.Volumes = append(sdc.Spec.RackTemplate.ScyllaDBManagerAgent.Volumes, corev1.Volume{
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

	sdc.Spec.RackTemplate.ScyllaDBManagerAgent.VolumeMounts = append(sdc.Spec.RackTemplate.ScyllaDBManagerAgent.VolumeMounts, corev1.VolumeMount{
		Name:      "aws-credentials",
		ReadOnly:  true,
		MountPath: "/var/lib/scylla-manager/.aws/credentials",
		SubPath:   "credentials",
	})

	if len(s3AgentConfig) > 0 {
		// The custom agent config must carry no auth token: a token there would take precedence over the one the
		// operator provisions, including a shared one referenced through the agent auth token override annotation.
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

		// The ref goes on every rack, not on the rack template: the rack template's ref is not propagated to the racks.
		for i := range sdc.Spec.Racks {
			if sdc.Spec.Racks[i].ScyllaDBManagerAgent == nil {
				sdc.Spec.Racks[i].ScyllaDBManagerAgent = &scyllav1alpha1.ScyllaDBManagerAgentTemplate{}
			}
			sdc.Spec.Racks[i].ScyllaDBManagerAgent.CustomConfigSecretRef = pointer.Ptr(agentConfigSecret.Name)
		}
	}
}
