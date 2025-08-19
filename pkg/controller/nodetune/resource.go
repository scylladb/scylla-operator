package nodetune

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/cmdutil"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
)

const (
	rootUID int64 = 0
	rootGID int64 = 0
)

// TODO: set anti affinities so config jobs don't run on the same node at the same time

func makeJobsForNode(
	ctx context.Context,
	nc *scyllav1alpha1.NodeConfig,
	controllerRef *metav1.OwnerReference,
	namespace string,
	nodeName string,
	nodeUID types.UID,
	scyllaImage string,
	selfPod *corev1.Pod,
) ([]*batchv1.Job, error) {
	var jobs []*batchv1.Job

	if nc.Spec.DisableOptimizations {
		klog.V(2).InfoS("NodeConfig's optimizations are disabled, skipping perftune Job creation for node")
		return jobs, nil
	}

	jobs = append(jobs, makePerftuneJobForNode(
		controllerRef,
		namespace,
		nc.Name,
		nodeName,
		nodeUID,
		scyllaImage,
		&selfPod.Spec,
	))

	return jobs, nil
}

func makePerftuneJobForNode(controllerRef *metav1.OwnerReference, namespace, nodeConfigName, nodeName string, nodeUID types.UID, image string, podSpec *corev1.PodSpec) *batchv1.Job {
	podSpec = podSpec.DeepCopy()

	args := []string{
		"--tune=system",
		"--tune-clock",
	}

	labels := map[string]string{
		naming.NodeConfigNameLabel:          nodeConfigName,
		naming.NodeConfigJobForNodeUIDLabel: string(nodeUID),
		naming.NodeConfigJobTypeLabel:       string(naming.NodeConfigJobTypeNode),
	}

	annotations := map[string]string{
		naming.NodeConfigJobForNodeKey: nodeName,
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			// TODO: hash the name to avoid overflow.
			Name:            fmt.Sprintf("perftune-node-%s", nodeUID),
			OwnerReferences: []metav1.OwnerReference{*controllerRef},
			Labels:          labels,
			Annotations:     annotations,
		},
		Spec: batchv1.JobSpec{
			// TODO: handle failed jobs and retry.
			BackoffLimit: pointer.Ptr(int32(math.MaxInt32)),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: annotations,
				},
				Spec: corev1.PodSpec{
					Tolerations:        podSpec.Tolerations,
					NodeName:           nodeName,
					RestartPolicy:      corev1.RestartPolicyOnFailure,
					HostPID:            true,
					HostNetwork:        true,
					ServiceAccountName: naming.PerftuneServiceAccountName,
					Containers: []corev1.Container{
						{
							Name:            naming.PerftuneContainerName,
							Image:           image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command:         []string{"/opt/scylladb/scripts/perftune.py"},
							Args:            args,
							Env: []corev1.EnvVar{
								{
									Name:  "SYSTEMD_IGNORE_CHROOT",
									Value: "1",
								},
							},
							SecurityContext: &corev1.SecurityContext{
								Privileged: pointer.Ptr(true),
							},
							VolumeMounts: []corev1.VolumeMount{
								makeVolumeMount("host-sys-class", "/sys/class", false),
								makeVolumeMount("host-sys-devices", "/sys/devices", false),
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("50Mi"),
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						// Network device and clock tuning
						makeHostDirVolume("host-sys-class", "/sys/class"),
						makeHostDirVolume("host-sys-devices", "/sys/devices"),
					},
				},
			},
		},
	}

	return job
}

func makeRlimitsJobForContainer(controllerRef *metav1.OwnerReference, namespace, nodeConfigName, nodeName string, nodeUID types.UID, image string, podSpec *corev1.PodSpec, scyllaPod *corev1.Pod, scyllaHostPID int) (*batchv1.Job, error) {
	scyllaContainerID, err := controllerhelpers.GetScyllaContainerID(scyllaPod)
	if err != nil {
		return nil, fmt.Errorf("can't get scylla container id: %w", err)
	}

	jobData := containerJobData{
		ContainerIDs: []string{scyllaContainerID},
	}
	jobDataBytes, err := json.Marshal(jobData)
	if err != nil {
		return nil, fmt.Errorf("can't marshal job data: %w", err)
	}

	labels := map[string]string{
		naming.NodeConfigNameLabel:          nodeConfigName,
		naming.NodeConfigJobForNodeUIDLabel: string(nodeUID),
		naming.NodeConfigJobTypeLabel:       string(naming.NodeConfigJobTypeContainerResourceLimits),
	}
	annotations := map[string]string{
		naming.NodeConfigJobForNodeKey: nodeName,
		naming.NodeConfigJobData:       string(jobDataBytes),
	}

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       namespace,
			Name:            fmt.Sprintf("containers-rlimits-%s", scyllaPod.UID),
			OwnerReferences: []metav1.OwnerReference{*controllerRef},
			Labels:          labels,
			Annotations:     annotations,
		},
		Spec: batchv1.JobSpec{
			// TODO: handle failed jobs and retry.
			BackoffLimit: pointer.Ptr(int32(math.MaxInt32)),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: annotations,
				},
				Spec: corev1.PodSpec{
					Tolerations:        podSpec.Tolerations,
					NodeName:           nodeName,
					RestartPolicy:      corev1.RestartPolicyOnFailure,
					HostPID:            true,
					ServiceAccountName: naming.RlimitsJobServiceAccountName,
					SecurityContext: &corev1.PodSecurityContext{
						RunAsUser:  pointer.Ptr(rootUID),
						RunAsGroup: pointer.Ptr(rootGID),
					},
					Containers: []corev1.Container{
						{
							Name:            naming.RLimitsContainerName,
							Image:           image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command: []string{
								"/usr/bin/scylla-operator",
								"rlimits-job",
								fmt.Sprintf("--pid=%d", scyllaHostPID),
								fmt.Sprintf("--loglevel=%d", cmdutil.GetLoglevelOrDefaultOrDie()),
							},
							SecurityContext: &corev1.SecurityContext{
								Privileged: pointer.Ptr(true),
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("50Mi"),
								},
							},
						},
					},
				},
			},
		},
	}, nil
}

type containerJobData struct {
	ContainerIDs []string `json:"containerIDs"`
}

func makePerftuneJobForContainers(controllerRef *metav1.OwnerReference, namespace, nodeConfigName, nodeName string, nodeUID types.UID, image, irqMask string, dataHostPaths []string, disableWritebackCache bool, podSpec *corev1.PodSpec, ifaceNames, scyllaContainerIDs []string) (*batchv1.Job, error) {
	podSpec = podSpec.DeepCopy()

	args := []string{
		"--irq-cpu-mask", irqMask,
		"--tune=net",
	}

	for _, ifaceName := range ifaceNames {
		args = append(args, fmt.Sprintf("--nic=%s", ifaceName))
	}

	// FIXME: disk shouldn't be empty
	if len(dataHostPaths) > 0 {
		args = append(args, "--tune", "disks")
	}
	for _, hostPath := range dataHostPaths {
		args = append(args, "--dir", path.Join("/host", hostPath))
	}

	if disableWritebackCache {
		args = append(args, "--write-back-cache", "false")
	}

	jobData := containerJobData{
		ContainerIDs: scyllaContainerIDs,
	}
	jobDataBytes, err := json.Marshal(jobData)
	if err != nil {
		return nil, fmt.Errorf("can't marshal job data: %w", err)
	}

	labels := map[string]string{
		naming.NodeConfigNameLabel:          nodeConfigName,
		naming.NodeConfigJobForNodeUIDLabel: string(nodeUID),
		naming.NodeConfigJobTypeLabel:       string(naming.NodeConfigJobTypeContainerPerftune),
	}
	annotations := map[string]string{
		naming.NodeConfigJobForNodeKey: nodeName,
		naming.NodeConfigJobData:       string(jobDataBytes),
	}

	perftuneJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			// TODO: hash the name to avoid overflow
			Name:            fmt.Sprintf("perftune-containers-%s", nodeUID),
			OwnerReferences: []metav1.OwnerReference{*controllerRef},
			Labels:          labels,
			Annotations:     annotations,
		},
		Spec: batchv1.JobSpec{
			// TODO: handle failed jobs and retry.
			BackoffLimit: pointer.Ptr(int32(math.MaxInt32)),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: annotations,
				},
				Spec: corev1.PodSpec{
					Tolerations:        podSpec.Tolerations,
					NodeName:           nodeName,
					RestartPolicy:      corev1.RestartPolicyOnFailure,
					HostPID:            true,
					HostNetwork:        true,
					ServiceAccountName: naming.PerftuneServiceAccountName,
					SecurityContext: &corev1.PodSecurityContext{
						RunAsUser:  pointer.Ptr(rootUID),
						RunAsGroup: pointer.Ptr(rootGID),
					},
					Containers: []corev1.Container{
						{
							Name:            naming.PerftuneContainerName,
							Image:           image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command:         []string{"/opt/scylladb/scripts/perftune.py"},
							Args:            args,
							Env: []corev1.EnvVar{
								{
									Name:  "SYSTEMD_IGNORE_CHROOT",
									Value: "1",
								},
							},
							SecurityContext: &corev1.SecurityContext{
								Privileged: pointer.Ptr(true),
							},
							VolumeMounts: []corev1.VolumeMount{
								makeVolumeMount("hostfs", "/host", false),
								makeVolumeMount("etc-systemd", "/etc/systemd", false),
								makeVolumeMount("host-sys-class", "/sys/class", false),
								makeVolumeMount("host-sys-devices", "/sys/devices", false),
								makeVolumeMount("host-lib-systemd-system", "/lib/systemd/system", true),
								makeVolumeMount("host-var-run-dbus", "/var/run/dbus", true),
								makeVolumeMount("host-run-systemd-system", "/run/systemd/system", true),
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("50Mi"),
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						// Storage device tuning
						makeHostDirVolume("hostfs", "/"),
						makeHostDirVolume("host-sys-class", "/sys/class"),
						makeHostDirVolume("host-sys-devices", "/sys/devices"),
						// Plumb host systemd to restart irqbalancer running on host
						makeHostDirVolume("etc-systemd", "/etc/systemd"),
						makeHostDirVolume("host-lib-systemd-system", "/lib/systemd/system"),
						makeHostDirVolume("host-var-run-dbus", "/var/run/dbus"),
						makeHostDirVolume("host-run-systemd-system", "/run/systemd/system"),
					},
				},
			},
		},
	}

	// Host node might not be running irqbalance. Mount config only when it's present on the host.
	_, err = os.Stat("/etc/sysconfig/irqbalance")
	if err == nil {
		perftuneJob.Spec.Template.Spec.Volumes = append(
			perftuneJob.Spec.Template.Spec.Volumes,
			makeHostFileVolume("etc-sysconfig-irqbalance", "/etc/sysconfig/irqbalance"),
		)
		perftuneJob.Spec.Template.Spec.Containers[0].VolumeMounts = append(
			perftuneJob.Spec.Template.Spec.Containers[0].VolumeMounts,
			makeVolumeMount("etc-sysconfig-irqbalance", "/etc/sysconfig/irqbalance", false),
		)
	}

	return perftuneJob, nil
}

func makeVolumeMount(name, mountPath string, readonly bool) corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      name,
		MountPath: mountPath,
		ReadOnly:  readonly,
	}
}

func makeHostVolume(name, hostPath string, volumeType *corev1.HostPathType) corev1.Volume {
	return corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: hostPath,
				Type: volumeType,
			},
		},
	}
}

func makeHostDirVolume(name, hostPath string) corev1.Volume {
	volumeType := corev1.HostPathDirectory
	return makeHostVolume(name, hostPath, &volumeType)
}

func makeHostFileVolume(name, hostPath string) corev1.Volume {
	volumeType := corev1.HostPathFile
	return makeHostVolume(name, hostPath, &volumeType)
}
