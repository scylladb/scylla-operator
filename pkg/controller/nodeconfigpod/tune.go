// Copyright (C) 2021 ScyllaDB

package nodeconfigpod

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/scylladb/go-set/strset"
	"github.com/scylladb/scylla-operator/pkg/controller/helpers"
	"github.com/scylladb/scylla-operator/pkg/cri"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/util/cpuset"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

func getIRQCPUs(ctx context.Context, criClient cri.Client, scyllaPods []*corev1.Pod, hostFullCpuset cpuset.CPUSet) (cpuset.CPUSet, error) {
	scyllaCPUs, err := getScyllaCPUs(ctx, criClient, scyllaPods)
	if err != nil {
		return cpuset.CPUSet{}, fmt.Errorf("get Scylla CPUs: %w", err)
	}

	// Use all CPUs *not* assigned to Scylla container for IRQs.
	return hostFullCpuset.Difference(scyllaCPUs), nil
}

func getScyllaCPUs(ctx context.Context, criClient cri.Client, scyllaPods []*corev1.Pod) (cpuset.CPUSet, error) {
	scyllaCpus := cpuset.NewCPUSet()
	for _, p := range scyllaPods {
		if p.GetLabels()[naming.ClusterNameLabel] != "" {
			if !helpers.IsScyllaContainerRunning(p) {
				return cpuset.CPUSet{}, fmt.Errorf("scylla container in %s pod is not running, will retry in a bit", naming.ObjRef(p))
			}
			if p.Status.QOSClass == corev1.PodQOSGuaranteed {
				containerCpuSet, err := scyllaContainerCpuSet(ctx, criClient, p, naming.HostFilesystemDirName)
				if err != nil {
					return cpuset.CPUSet{}, fmt.Errorf("failed to get cpuset of %s Pod Scylla container: %w", naming.ObjRef(p), err)
				}
				klog.V(4).InfoS("Scylla container cpuset", "cpuset", containerCpuSet.String(), "Pod", klog.KObj(p))
				scyllaCpus = scyllaCpus.Union(containerCpuSet)
			}
		}
	}

	return scyllaCpus, nil
}

func scyllaDataDirHostPaths(ctx context.Context, criClient cri.Client, scyllaPods []*corev1.Pod) ([]string, error) {
	dataDirs := strset.New()

	for _, pod := range scyllaPods {
		cid, err := scyllaContainerID(pod)
		if err != nil {
			return nil, fmt.Errorf("get Scylla container ID: %w", err)
		}

		cs, err := criClient.Inspect(ctx, cid)
		if err != nil && !errors.Is(err, cri.NotFoundErr) {
			return nil, fmt.Errorf("failed to inspect container %q, %w", cid, err)
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
		return cpuset.CPUSet{}, fmt.Errorf("failed to inspect container %q, %w", cid, err)
	}

	if cs.Info.RuntimeSpec == nil {
		return cpuset.CPUSet{}, nil
	}

	containerCpuSet, err := cpuset.Parse(cs.Info.RuntimeSpec.Linux.Resources.CPU.Cpus)
	if err != nil {
		return cpuset.CPUSet{}, fmt.Errorf("failed to parse container %q cpuset %q, %w", cid, cs.Info.RuntimeSpec.Linux.Resources.CPU.Cpus, err)
	}

	return containerCpuSet, nil
}

func scyllaContainerCpuSet(ctx context.Context, criClient cri.Client, pod *corev1.Pod, hostFilesystem string) (cpuset.CPUSet, error) {
	cid, err := scyllaContainerID(pod)
	if err != nil {
		return cpuset.CPUSet{}, fmt.Errorf("get Scylla container ID: %w", err)
	}

	containerCpuSet, err := cpusetFromCRI(ctx, criClient, cid)
	if err != nil && !errors.Is(err, cri.NotFoundErr) {
		return cpuset.CPUSet{}, fmt.Errorf("get cpuset from CRI: %w", err)
	}

	if !containerCpuSet.IsEmpty() {
		return containerCpuSet, nil
	}

	// On AWS and Minikube runtime information is not available through CRI.
	// Figure out assigned CPUs by manually reading cgroup fs.
	klog.Info("Falling back to manual cpuset discovery via cgroups")
	for _, cpusetPath := range podCpusetPaths(string(pod.UID), cid) {
		hostPath := path.Join(hostFilesystem, cpusetPath)
		if _, err := os.Stat(hostPath); err != nil {
			continue
		}
		content, err := ioutil.ReadFile(hostPath)
		if err != nil {
			return cpuset.CPUSet{}, fmt.Errorf("failed to read cgroup cpuset: %w", err)
		}
		containerCpuSet, err := cpuset.Parse(strings.TrimSpace(string(content)))
		if err != nil {
			return cpuset.CPUSet{}, fmt.Errorf("failed to parse container %q cpuset %q, %w", cid, string(content), err)
		}

		klog.V(4).InfoS("Found Scylla cpuset", "container_id", cid, "cpuset", containerCpuSet.String())
		return containerCpuSet, nil
	}

	return cpuset.CPUSet{}, fmt.Errorf("cannot find Scylla container cpuset")
}

func podCpusetPaths(podID, containerID string) []string {
	// AWS, minikube: /sys/fs/cgroup/cpuset/kubepods.slice/kubepods-pode0c9e8dc_4bfa_4d34_9e03_746a0fab90a5.slice/docker-7b4acc0e8a0d0090396906d500710f121851c487ca1a9f889215200bc377b5fb.scope/cpuset.cpus
	// GKE: /sys/fs/cgroup/cpuset/kubepods/podfc060df5-82a2-4a5e-86c0-40aec54b2a09/e3e0fc65ca5a88c9f47078aa0f097053f4c0620b2134a903c124a9b015386505/cpuset.cpus

	return []string{
		fmt.Sprintf("/sys/fs/cgroup/cpuset/kubepods.slice/kubepods-pod%s.slice/docker-%s.scope/cpuset.cpus", strings.ReplaceAll(podID, "-", "_"), containerID),
		fmt.Sprintf("/sys/fs/cgroup/cpuset/kubepods/pod%s/%s/cpuset.cpus", podID, containerID),
	}
}

func scyllaContainerID(pod *corev1.Pod) (string, error) {
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.Name != naming.ScyllaContainerName {
			continue
		}

		cid, err := stripContainerID(cs.ContainerID)
		if err != nil {
			return "", fmt.Errorf("cannot strip container ID prefix from %q, %w", cs.ContainerID, err)
		}
		return cid, nil
	}
	return "", fmt.Errorf("cannot find Scylla container ID in '%s/%s' Pod", pod.Namespace, pod.Name)
}

var containerIDRe = regexp.MustCompile(`[a-z]+://([a-z0-9]+)`)

func stripContainerID(containerID string) (string, error) {
	m := containerIDRe.FindStringSubmatch(containerID)
	if len(m) != 2 {
		return "", fmt.Errorf("unsupported containerID format %q", containerID)
	}
	return m[1], nil
}
