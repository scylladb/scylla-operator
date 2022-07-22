package managerv2

import (
	"testing"

	"github.com/aws/smithy-go/ptr"
	"github.com/google/go-cmp/cmp"
	"github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func DefaultScyllaManager() *v1alpha1.ScyllaManager {
	return &v1alpha1.ScyllaManager{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "manager",
			Namespace: "manager",
			UID:       "123",
		},
		Spec: v1alpha1.ScyllaManagerSpec{
			Image:    "docker.io/scylladb/scylla-manager:3.0.0",
			Replicas: 1,
		},
	}
}

func TestMakeDeployment(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name               string
		scyllaManager      *v1alpha1.ScyllaManager
		expectedDeployment *v1.Deployment
	}{
		{
			name:          "simple deployment",
			scyllaManager: DefaultScyllaManager(),
			expectedDeployment: &v1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "manager",
					Namespace: "manager",
					Labels: map[string]string{
						"app.kubernetes.io/name":       "scylla",
						"app.kubernetes.io/managed-by": "scylla-manager-operator",
						"scylla-manager":               "manager-manager",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         controllerGVK.GroupVersion().String(),
							Kind:               controllerGVK.Kind,
							Name:               "manager",
							UID:                "123",
							BlockOwnerDeletion: ptr.Bool(true),
							Controller:         ptr.Bool(true),
						},
					},
				},
				Spec: v1.DeploymentSpec{
					Replicas: ptr.Int32(1),
					Selector: metav1.SetAsLabelSelector(map[string]string{
						"app.kubernetes.io/name":       "scylla",
						"app.kubernetes.io/managed-by": "scylla-manager-operator",
						"scylla-manager":               "manager-manager",
					}),
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app.kubernetes.io/name":       "scylla",
								"app.kubernetes.io/managed-by": "scylla-manager-operator",
								"scylla-manager":               "manager-manager",
							},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:            "scylla-manager-container",
									Image:           "docker.io/scylladb/scylla-manager:3.0.0",
									ImagePullPolicy: corev1.PullIfNotPresent,
									Args: []string{
										"--config-file=/mnt/scylla-manager-config/scylla-manager-config.json",
										"--config-file=/mnt/scylla-manager-config/scylla-manager-secret.json",
									},
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      "scylla-manager-config",
											MountPath: "/mnt/scylla-manager-config/scylla-manager-config.json",
											SubPath:   "scylla-manager-config.json",
										},
										{
											Name:      "scylla-manager-secrets",
											MountPath: "/mnt/scylla-manager-config/scylla-manager-secret.json",
											SubPath:   "scylla-manager-secret.json",
										},
									},
									ReadinessProbe: &corev1.Probe{
										ProbeHandler: corev1.ProbeHandler{
											HTTPGet: &corev1.HTTPGetAction{
												Path: "/api/v1/clusters",
												Port: intstr.FromInt(5080),
											},
										},
										InitialDelaySeconds: 60,
										TimeoutSeconds:      5,
										PeriodSeconds:       10,
										SuccessThreshold:    1,
										FailureThreshold:    1,
									},
									LivenessProbe: &corev1.Probe{
										ProbeHandler: corev1.ProbeHandler{
											HTTPGet: &corev1.HTTPGetAction{
												Path: "/api/v1/version",
												Port: intstr.FromInt(5080),
											},
										},
										InitialDelaySeconds: 60,
										TimeoutSeconds:      5,
										PeriodSeconds:       10,
										SuccessThreshold:    1,
										FailureThreshold:    3,
									},
								},
							},
							Volumes: []corev1.Volume{
								{
									Name: "scylla-manager-config",
									VolumeSource: corev1.VolumeSource{
										ConfigMap: &corev1.ConfigMapVolumeSource{
											LocalObjectReference: corev1.LocalObjectReference{Name: "manager"},
										},
									},
								},
								{
									Name: "scylla-manager-secrets",
									VolumeSource: corev1.VolumeSource{
										Secret: &corev1.SecretVolumeSource{
											SecretName: "manager",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if diff := cmp.Diff(MakeDeployment(tc.scyllaManager), tc.expectedDeployment); diff != "" {
				t.Errorf("expected and actual deployments differ: %s", diff)
			}
		})
	}
}

var minAvailable = intstr.FromInt(1)

func TestMakePodDisruptionBudget(t *testing.T) {
	t.Parallel()
	tt := []struct {
		name          string
		scyllaManager *v1alpha1.ScyllaManager
		expectedPDB   *policyv1.PodDisruptionBudget
	}{
		{
			name:          "simple pdb",
			scyllaManager: DefaultScyllaManager(),
			expectedPDB: &policyv1.PodDisruptionBudget{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "manager",
					Namespace: "manager",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         controllerGVK.GroupVersion().String(),
							Kind:               controllerGVK.Kind,
							Name:               "manager",
							UID:                "123",
							BlockOwnerDeletion: ptr.Bool(true),
							Controller:         ptr.Bool(true),
						},
					},
					Labels: map[string]string{
						"app.kubernetes.io/name":       "scylla",
						"app.kubernetes.io/managed-by": "scylla-manager-operator",
						"scylla-manager":               "manager-manager",
					},
				},
				Spec: policyv1.PodDisruptionBudgetSpec{
					MinAvailable: &minAvailable,
					Selector: metav1.SetAsLabelSelector(map[string]string{
						"app.kubernetes.io/name":       "scylla",
						"app.kubernetes.io/managed-by": "scylla-manager-operator",
						"scylla-manager":               "manager-manager",
					}),
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if diff := cmp.Diff(MakePodDisruptionBudget(tc.scyllaManager), tc.expectedPDB); diff != "" {
				t.Errorf("expected and actual deployments differ: %s", diff)
			}
		})
	}
}

func TestMakeService(t *testing.T) {
	t.Parallel()
	tt := []struct {
		name            string
		scyllaManager   *v1alpha1.ScyllaManager
		expectedService *corev1.Service
	}{
		{
			name:          "simple service",
			scyllaManager: DefaultScyllaManager(),
			expectedService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "manager",
					Namespace: "manager",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         controllerGVK.GroupVersion().String(),
							Kind:               controllerGVK.Kind,
							Name:               "manager",
							UID:                "123",
							BlockOwnerDeletion: ptr.Bool(true),
							Controller:         ptr.Bool(true),
						},
					},
					Labels: map[string]string{
						"app.kubernetes.io/name":       "scylla",
						"app.kubernetes.io/managed-by": "scylla-manager-operator",
						"scylla-manager":               "manager-manager",
					},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{Name: "api-http", Port: 80, Protocol: corev1.ProtocolTCP, TargetPort: intstr.FromInt(5080)},
						{Name: "api-https", Port: 443, Protocol: corev1.ProtocolTCP, TargetPort: intstr.FromInt(5443)},
						{Name: "metrics", Port: 5090, Protocol: corev1.ProtocolTCP, TargetPort: intstr.FromInt(5090)},
					},
					Selector: map[string]string{
						"app.kubernetes.io/name":       "scylla",
						"app.kubernetes.io/managed-by": "scylla-manager-operator",
						"scylla-manager":               "manager-manager",
					},
					Type: corev1.ServiceTypeClusterIP,
				},
			},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if diff := cmp.Diff(MakeService(tc.scyllaManager), tc.expectedService); diff != "" {
				t.Errorf("expected and actual deployments differ: %s", diff)
			}
		})
	}
}
