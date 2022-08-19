package managerv2

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/aws/smithy-go/ptr"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/managerclient"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/json"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

func MakeDeployment(sm *v1alpha1.ScyllaManager) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sm.Name,
			Namespace: sm.Namespace,
			Labels:    naming.ManagerLabels(sm),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(sm, controllerGVK),
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.Int32(sm.Spec.Replicas),
			Selector: metav1.SetAsLabelSelector(naming.ManagerLabels(sm)),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: naming.ManagerLabels(sm),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "scylla-manager-container",
							Image:           sm.Spec.Image,
							Resources:       sm.Spec.Resources,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Args: []string{
								"--config-file=" + naming.ScyllaManagerConfigDirName + naming.ScyllaManagerConfigName,
								"--config-file=" + naming.ScyllaManagerConfigDirName + naming.ScyllaManagerSecretName,
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "scylla-manager-config",
									MountPath: naming.ScyllaManagerConfigDirName + naming.ScyllaManagerConfigName,
									SubPath:   naming.ScyllaManagerConfigName,
								},
								{
									Name:      "scylla-manager-secrets",
									MountPath: naming.ScyllaManagerConfigDirName + naming.ScyllaManagerSecretName,
									SubPath:   naming.ScyllaManagerSecretName,
								},
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: naming.ScyllaManagerReadinessProbePath,
										Port: intstr.FromInt(naming.ScyllaManagerProbePort),
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
										Path: naming.ScyllaManagerLivenessProbePath,
										Port: intstr.FromInt(naming.ScyllaManagerProbePort),
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
					ImagePullSecrets: sm.Spec.ImagePullSecrets,
					Volumes: []corev1.Volume{
						{
							Name: "scylla-manager-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{Name: sm.Name},
								},
							},
						},
						{
							Name: "scylla-manager-secrets",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: sm.Name,
								},
							},
						},
					},
				},
			},
		},
	}
}

func MakePodDisruptionBudget(sm *v1alpha1.ScyllaManager) *policyv1.PodDisruptionBudget {
	minAvailable := intstr.FromInt(1)

	return &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sm.Name,
			Namespace: sm.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(sm, controllerGVK),
			},
			Labels: naming.ManagerLabels(sm),
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			MinAvailable: &minAvailable,
			Selector:     metav1.SetAsLabelSelector(naming.ManagerLabels(sm)),
		},
	}
}

func MakeService(sm *v1alpha1.ScyllaManager) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sm.Name,
			Namespace: sm.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(sm, controllerGVK),
			},
			Labels: naming.ManagerLabels(sm),
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{Name: "api-http", Port: 80, Protocol: corev1.ProtocolTCP, TargetPort: intstr.FromInt(naming.ScyllaManagerHttpPort)},
				{Name: "api-https", Port: 443, Protocol: corev1.ProtocolTCP, TargetPort: intstr.FromInt(naming.ScyllaManagerHttpsPort)},
				{Name: "metrics", Port: 5090, Protocol: corev1.ProtocolTCP, TargetPort: intstr.FromInt(naming.ScyllaManagerMetricsPort)},
			},
			Selector: naming.ManagerLabels(sm),
			Type:     corev1.ServiceTypeClusterIP,
		},
	}
}

func MakeConfigMap(sm *v1alpha1.ScyllaManager) (*corev1.ConfigMap, error) {
	c, err := makeManagerConfig(sm)
	if err != nil {
		return nil, err
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sm.Name,
			Namespace: sm.Namespace,
			Labels:    naming.ManagerLabels(sm),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(sm, controllerGVK),
			},
		},
		BinaryData: map[string][]byte{naming.ScyllaManagerConfigName: c},
	}, nil
}

func MakeSecret(sm *v1alpha1.ScyllaManager) (*corev1.Secret, error) {
	s, err := makeManagerSecret(sm)
	if err != nil {
		return nil, err
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sm.Name,
			Namespace: sm.Namespace,
			Labels:    naming.ManagerLabels(sm),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(sm, controllerGVK),
			},
		},
		Data: map[string][]byte{naming.ScyllaManagerSecretName: s},
	}, nil
}

type ScyllaManagerDatabaseConfig struct {
	Hosts    []string `json:"hosts,omitempty"`
	User     string   `json:"user,omitempty"`
	Password string   `json:"password,omitempty"`
}

type ScyllaManagerConfig struct {
	Database *ScyllaManagerDatabaseConfig `json:"database,omitempty"`
	Http     string                       `json:"http,omitempty"`
	Https    string                       `json:"https,omitempty"`
}

func makeManagerConfig(sm *v1alpha1.ScyllaManager) ([]byte, error) {
	return json.Marshal(
		&ScyllaManagerConfig{
			Database: &ScyllaManagerDatabaseConfig{
				Hosts: []string{sm.Spec.Database.Connection.Server},
			},
			Http:  ":5080",
			Https: ":5443",
		})
}

func makeManagerSecret(sm *v1alpha1.ScyllaManager) ([]byte, error) {
	return json.Marshal(&ScyllaManagerConfig{
		Database: &ScyllaManagerDatabaseConfig{
			User:     sm.Spec.Database.Connection.Username,
			Password: sm.Spec.Database.Connection.Password,
		}})
}

func MakeClusters(scyllaClusters []*scyllav1.ScyllaCluster, secretLister corev1listers.SecretLister) ([]*managerclient.Cluster, error) {
	var errs []error
	clusters := make([]*managerclient.Cluster, 0, len(scyllaClusters))
	for _, sc := range scyllaClusters {
		secret, err := secretLister.Secrets(sc.Namespace).Get(naming.AgentAuthTokenSecretName(sc.Name))
		if err != nil {
			errs = append(errs, fmt.Errorf("get secret of cluster %s/%s: %v", sc.Namespace, sc.Name, err))
			continue
		}

		token, err := helpers.GetAgentAuthTokenFromSecret(secret)
		if err != nil {
			errs = append(errs, fmt.Errorf("read token from secret of cluster %s/%s: %v", sc.Namespace, sc.Name, err))
			continue
		}

		name := naming.ManagerClusterName(sc)
		clusters = append(clusters, &managerclient.Cluster{
			AuthToken: token,
			Name:      name,
			Host:      naming.CrossNamespaceServiceNameForCluster(sc),
		})
	}

	return clusters, utilerrors.NewAggregate(errs)
}

func MakeRepairTasks(sm *v1alpha1.ScyllaManager, clusters []*managerclient.Cluster) ([]*managerclient.Task, error) {
	makeTask := func(r v1alpha1.RepairTaskSpec, c *managerclient.Cluster) (*managerclient.Task, error) {
		p := make(map[string]interface{})
		p["failfast"] = r.FailFast
		p["dc"] = r.DC
		intensity, err := strconv.ParseFloat(r.Intensity, 64)
		if err != nil {
			return nil, err
		}
		p["intensity"] = intensity
		p["parallel"] = r.Parallel
		p["keyspace"] = r.Keyspace
		p["smalltablethreshold"] = r.SmallTableThreshold
		p["host"] = r.Host
		var window []string
		if r.TimeWindow != "" {
			window = strings.Split(r.TimeWindow, ",")
		}
		return &managerclient.Task{
			ClusterID:  c.ID,
			Enabled:    !r.Disable,
			Name:       r.Name,
			Properties: p,
			Schedule: &managerclient.Schedule{
				Cron:       r.Cron,
				NumRetries: *r.NumRetries,
				Timezone:   r.Timezone,
				Window:     window,
			},
			Type: naming.RepairTask,
		}, nil
	}

	var errs []error
	tasks := make([]*managerclient.Task, 0, len(clusters)*len(sm.Spec.Repairs))
	for _, c := range clusters {
		for _, t := range sm.Spec.Repairs {
			task, err := makeTask(t, c)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			tasks = append(tasks, task)
		}
	}

	return tasks, utilerrors.NewAggregate(errs)
}

func MakeBackupTasks(sm *v1alpha1.ScyllaManager, clusters []*managerclient.Cluster) []*managerclient.Task {
	makeTask := func(r v1alpha1.BackupTaskSpec, c *managerclient.Cluster) *managerclient.Task {
		p := make(map[string]interface{})
		p["dc"] = r.DC
		p["keyspace"] = r.Keyspace
		p["location"] = r.Location
		p["ratelimit"] = r.RateLimit
		p["retention"] = r.Retention
		p["snapshotparallel"] = r.SnapshotParallel
		p["uploadparallel"] = r.UploadParallel
		var window []string
		if r.TimeWindow != "" {
			window = strings.Split(r.TimeWindow, ",")
		}
		return &managerclient.Task{
			ClusterID:  c.ID,
			Enabled:    !r.Disable,
			Name:       r.Name,
			Properties: p,
			Schedule: &managerclient.Schedule{
				Cron:       r.Cron,
				NumRetries: *r.NumRetries,
				Timezone:   r.Timezone,
				Window:     window,
			},
			Type: naming.BackupTask,
		}
	}

	tasks := make([]*managerclient.Task, 0, len(clusters)*len(sm.Spec.Backups))
	for _, c := range clusters {
		for _, t := range sm.Spec.Backups {
			tasks = append(tasks, makeTask(t, c))
		}
	}

	return tasks
}
