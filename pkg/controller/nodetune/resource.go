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
	hashutil "github.com/scylladb/scylla-operator/pkg/util/hash"
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
	operatorImage string,
	selfPod *corev1.Pod,
	sysctlConfigMap *corev1.ConfigMap,
) ([]*batchv1.Job, error) {
	var jobs []*batchv1.Job

	sysctlsJob, err := makeSysctlsJobForNode(
		nc,
		controllerRef,
		namespace,
		nodeName,
		nodeUID,
		operatorImage,
		&selfPod.Spec,
		sysctlConfigMap,
	)
	if err != nil {
		return nil, fmt.Errorf("can't create sysctls job for node %q: %w", nodeName, err)
	}
	jobs = append(jobs, sysctlsJob)

	perftuneJob, ok := makePerftuneJobForNode(
		nc,
		controllerRef,
		namespace,
		nodeName,
		nodeUID,
		scyllaImage,
		&selfPod.Spec,
	)
	if ok {
		jobs = append(jobs, perftuneJob)
	}

	return jobs, nil
}

func makePerftuneJobForNode(nc *scyllav1alpha1.NodeConfig, controllerRef *metav1.OwnerReference, namespace, nodeName string, nodeUID types.UID, image string, podSpec *corev1.PodSpec) (*batchv1.Job, bool) {
	if nc.Spec.DisableOptimizations {
		klog.V(2).InfoS("NodeConfig's optimizations are disabled, skipping perftune Job for node")
		return nil, false
	}

	podSpec = podSpec.DeepCopy()

	args := []string{
		"--tune=system",
		"--tune-clock",
	}

	labels := labelsForNodeConfigJob(nc.GetName(), nodeUID, naming.NodeConfigJobTypeNodePerftune)
	annotations := annotationsForNodeConfigJob(nodeName)

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

	return job, true
}

// makeSysctlsJobForNode makes a Job that applies sysctls on the specified node.
func makeSysctlsJobForNode(
	nc *scyllav1alpha1.NodeConfig,
	controllerRef *metav1.OwnerReference,
	namespace string,
	nodeName string,
	nodeUID types.UID,
	image string,
	podSpec *corev1.PodSpec,
	sysctlConfigMap *corev1.ConfigMap,
) (*batchv1.Job, error) {
	name, err := naming.NodeConfigSysctlsJobForNodeName(string(nodeUID))
	if err != nil {
		return nil, fmt.Errorf("can't get sysctls job name: %w", err)
	}

	labels := labelsForNodeConfigJob(nc.GetName(), nodeUID, naming.NodeConfigJobTypeNodeSysctls)
	annotations := annotationsForNodeConfigJob(nodeName)

	inputsHash, err := hashutil.HashObjects(sysctlConfigMap)
	if err != nil {
		return nil, fmt.Errorf("can't calculate inputs hash: %w", err)
	}

	annotations[naming.InputsHashAnnotation] = inputsHash

	sysctlConfigMapMountPath := path.Join("/var/run/configmaps/", naming.SysctlConfigFileName)
	hostSysctlConfPath := path.Join("/host/etc/sysctl.d", naming.SysctlConfigFileName)
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       namespace,
			Name:            name,
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
					ServiceAccountName: naming.SysctlsServiceAccountName,
					Containers: []corev1.Container{
						{
							Name:            naming.SysctlsContainerName,
							Image:           image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command: []string{
								"/usr/bin/bash",
								"-euExo",
								"pipefail",
								"-O",
								"inherit_errexit",
								"-c",
							},
							Args: []string{
								`
cp ` + sysctlConfigMapMountPath + ` ` + hostSysctlConfPath + `
sysctl --load ` + hostSysctlConfPath + `
`,
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
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:             "hostfs",
									MountPath:        "/host",
									MountPropagation: pointer.Ptr(corev1.MountPropagationBidirectional),
								},
								{
									Name:      "sysctl-config",
									ReadOnly:  true,
									MountPath: sysctlConfigMapMountPath,
									SubPath:   naming.SysctlConfigFileName,
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "hostfs",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/",
									Type: pointer.Ptr(corev1.HostPathDirectory),
								},
							},
						},
						{
							Name: "sysctl-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: sysctlConfigMap.GetName(),
									},
								},
							},
						},
					},
				},
			},
		},
	}

	return job, nil
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

func labelsForNodeConfigJob(nodeConfigName string, nodeUID types.UID, jobType naming.NodeConfigJobType) map[string]string {
	return map[string]string{
		naming.NodeConfigNameLabel:          nodeConfigName,
		naming.NodeConfigJobForNodeUIDLabel: string(nodeUID),
		naming.NodeConfigJobTypeLabel:       string(jobType),
	}
}

func annotationsForNodeConfigJob(nodeName string) map[string]string {
	return map[string]string{
		naming.NodeConfigJobForNodeKey: nodeName,
	}
}
