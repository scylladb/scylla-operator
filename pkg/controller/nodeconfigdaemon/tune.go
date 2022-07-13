// Copyright (C) 2021 ScyllaDB

package nodeconfigdaemon

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/scylladb/go-set/strset"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/cri"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/util/cpuset"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

const defaultCgroupMountpoint = "/sys/fs/cgroup"

func getIRQCPUs(ctx context.Context, criClient cri.Client, scyllaPods []*corev1.Pod, hostFullCpuset cpuset.CPUSet, cgroupMountpoint string) (cpuset.CPUSet, error) {
	scyllaCPUs := cpuset.CPUSet{}
	for _, scyllaPod := range scyllaPods {
		if scyllaPod.Status.QOSClass != corev1.PodQOSGuaranteed {
			continue
		}

		containerID, err := getScyllaContainerIDInCRIFormat(scyllaPod)
		if err != nil {
			return cpuset.CPUSet{}, fmt.Errorf("can't find Scylla containerID in Pod %q: %w", naming.ObjRef(scyllaPod), err)
		}
		cs, err := getContainerCPUs(ctx, criClient, string(scyllaPod.UID), containerID, cgroupMountpoint)
		if err != nil {
			return cpuset.CPUSet{}, fmt.Errorf("can't get CPUs for container %q in Pod %q: %w", containerID, naming.ObjRef(scyllaPod), err)
		}
		scyllaCPUs = scyllaCPUs.Union(cs)
	}

	// Use all CPUs *not* assigned to Scylla container for IRQs.
	return hostFullCpuset.Difference(scyllaCPUs), nil
}

func getContainerCPUs(ctx context.Context, criClient cri.Client, podUID, containerID, cgroupMountpoint string) (cpuset.CPUSet, error) {
	containerCpuSet, err := cpusetFromCRI(ctx, criClient, containerID)
	if err != nil {
		return cpuset.CPUSet{}, fmt.Errorf("can't get cpuset from CRI: %w", err)
	}

	if !containerCpuSet.IsEmpty() {
		return containerCpuSet, nil
	}

	// On AWS and Minikube runtime information is not available through CRI.
	// Figure out assigned CPUs by manually reading cgroup fs.
	klog.Info("Falling back to manual cpuset discovery via cgroups")
	for _, cpusetPath := range podCpusetPaths(podUID, containerID, cgroupMountpoint) {
		_, err := os.Stat(cpusetPath)
		if err != nil {
			klog.V(4).InfoS("Cpuset path unsuccessful", "Path", cpusetPath, "Error", err)
			continue
		}

		klog.V(4).InfoS("Cpuset path successful", "Path", cpusetPath)

		content, err := ioutil.ReadFile(cpusetPath)
		if err != nil {
			return cpuset.CPUSet{}, fmt.Errorf("can't read cgroup cpuset: %w", err)
		}

		containerCpuSet, err := cpuset.Parse(strings.TrimSpace(string(content)))
		if err != nil {
			return cpuset.CPUSet{}, fmt.Errorf("can't parse container %q cpuset %q, %w", containerID, string(content), err)
		}

		klog.V(4).InfoS("Found Scylla cpuset", "ContainerID", containerID, "Cpuset", containerCpuSet.String())
		return containerCpuSet, nil
	}

	return cpuset.CPUSet{}, fmt.Errorf("can't find Scylla container cpuset")
}

func scyllaDataDirMountHostPaths(ctx context.Context, criClient cri.Client, scyllaPods []*corev1.Pod) ([]string, error) {
	dataDirs := strset.New()

	for _, pod := range scyllaPods {
		cid, err := getScyllaContainerIDInCRIFormat(pod)
		if err != nil {
			return nil, fmt.Errorf("get Scylla container ID: %w", err)
		}

		cs, err := criClient.Inspect(ctx, cid)
		if err != nil {
			return nil, fmt.Errorf("can't inspect container %q: %w", cid, err)
		}

		if cs != nil {
			for _, mount := range cs.Status.GetMounts() {
				if mount.ContainerPath != naming.DataDir {
					continue
				}
				dataDirs.Add(mount.HostPath)
			}
		}
	}

	return dataDirs.List(), nil
}

func cpusetFromCRI(ctx context.Context, client cri.Client, cid string) (cpuset.CPUSet, error) {
	cs, err := client.Inspect(ctx, cid)
	if err != nil {
		return cpuset.CPUSet{}, fmt.Errorf("can't inspect container %q, %w", cid, err)
	}

	if cs.Info.RuntimeSpec == nil {
		klog.V(2).InfoS("No container status available", "ContainerID", cid)
		return cpuset.CPUSet{}, nil
	}

	containerCpuSet, err := cpuset.Parse(cs.Info.RuntimeSpec.Linux.Resources.CPU.Cpus)
	if err != nil {
		return cpuset.CPUSet{}, fmt.Errorf("can't parse container %q cpuset %q, %w", cid, cs.Info.RuntimeSpec.Linux.Resources.CPU.Cpus, err)
	}

	return containerCpuSet, nil
}

func podCpusetPaths(podID, containerID, cgroupMountpoint string) []string {
	// AWS, minikube (docker): /sys/fs/cgroup/cpuset/kubepods.slice/kubepods-pode0c9e8dc_4bfa_4d34_9e03_746a0fab90a5.slice/docker-7b4acc0e8a0d0090396906d500710f121851c487ca1a9f889215200bc377b5fb.scope/cpuset.cpus
	// GKE: /sys/fs/cgroup/cpuset/kubepods/podfc060df5-82a2-4a5e-86c0-40aec54b2a09/e3e0fc65ca5a88c9f47078aa0f097053f4c0620b2134a903c124a9b015386505/cpuset.cpus
	// minikube (cri-o): /sys/fs/cgroup/cpuset/kubepods.slice/kubepods-pod74298416_16b6_47bb_ae8f_01ee4f91c525.slice/crio-4a839ac8074c771bd0f6d3732e4d5acd3e27acb818ae6895d767f06198a19596.scope/cpuset.cpus

	return []string{
		path.Join(cgroupMountpoint, fmt.Sprintf("/cpuset/kubepods.slice/kubepods-pod%s.slice/docker-%s.scope/cpuset.cpus", strings.ReplaceAll(podID, "-", "_"), containerID)),
		path.Join(cgroupMountpoint, fmt.Sprintf("/cpuset/kubepods/pod%s/%s/cpuset.cpus", podID, containerID)),
		path.Join(cgroupMountpoint, fmt.Sprintf("/cpuset/kubepods.slice/kubepods-pod%s.slice/crio-%s.scope/cpuset.cpus", strings.ReplaceAll(podID, "-", "_"), containerID)),
	}
}

func getScyllaContainerIDInCRIFormat(pod *corev1.Pod) (string, error) {
	cidURI, err := controllerhelpers.GetScyllaContainerID(pod)
	if err != nil {
		return "", err
	}

	cid, err := stripContainerID(cidURI)
	if err != nil {
		return "", fmt.Errorf("can't strip container ID prefix from %q: %w", cidURI, err)
	}

	return cid, nil
}

var containerIDRe = regexp.MustCompile(`[a-z]+://([a-z0-9]+)`)

func stripContainerID(containerID string) (string, error) {
	m := containerIDRe.FindStringSubmatch(containerID)
	if len(m) != 2 {
		return "", fmt.Errorf("unsupported containerID format %q", containerID)
	}
	return m[1], nil
}
