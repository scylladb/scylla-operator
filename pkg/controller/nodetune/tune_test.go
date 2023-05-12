// Copyright (C) 2021 ScyllaDB

package nodetune

import (
	"context"
	"fmt"
	"os"
	"path"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/scylladb/scylla-operator/pkg/cri"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/util/cpuset"
	"github.com/scylladb/scylla-operator/pkg/util/uuid"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	testTimeout = 1 * time.Second
)

func TestStripContainerID(t *testing.T) {
	ts := []struct {
		name                string
		containerID         string
		expectedContainerID string
		expectedErr         error
	}{
		{
			name:                "Docker container",
			containerID:         "docker://5bdb9a25f236e703161902376f9377336c072a2c4fca54a5100e8e07fb7d4280",
			expectedContainerID: "5bdb9a25f236e703161902376f9377336c072a2c4fca54a5100e8e07fb7d4280",
			expectedErr:         nil,
		},
		{
			name:                "invalid",
			containerID:         "abc1234",
			expectedContainerID: "",
			expectedErr:         fmt.Errorf(`unsupported containerID format "abc1234"`),
		},
	}

	for i := range ts {
		test := ts[i]
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cid, err := stripContainerID(test.containerID)
			if !reflect.DeepEqual(err, test.expectedErr) {
				t.Errorf("expected error %s, got %s", test.expectedErr, err)
			}
			if cid != test.expectedContainerID {
				t.Errorf("expected container ID %q, got %q", test.containerID, cid)
			}
		})
	}
}

type fakeCriClient struct {
	expectations map[string]*cri.ContainerStatus
}

func (f *fakeCriClient) Inspect(ctx context.Context, containerID string) (*cri.ContainerStatus, error) {
	if cs, ok := f.expectations[containerID]; ok {
		return cs, nil
	}
	return nil, fmt.Errorf("not expected Inspect call for %q containerID", containerID)
}

func (f *fakeCriClient) Close() error {
	return nil
}

func TestScyllaContainerCpuSet(t *testing.T) {
	const (
		scyllaContainerID = "5bdb9a25f236e703161902376f9377336c072a2c4fca54a5100e8e07fb7d4280"
		podUID            = "03840283-da85-4518-a860-2d1da454207c"
	)

	containerStatusWithRuntimeInfo := &cri.ContainerStatus{
		Info: cri.ContainerStatusInfo{
			RuntimeSpec: &cri.ContainerStatusInfoRuntimeSpec{
				Linux: cri.ContainerStatusInfoRuntimeSpecLinux{
					Resources: cri.ContainerStatusInfoRuntimeSpecLinuxResources{
						CPU: cri.ContainerStatusInfoRuntimeSpecLinuxResourcesCPU{
							Cpus: "0-1",
						},
					},
				},
			},
		},
	}

	emptyContainerStatus := &cri.ContainerStatus{
		Info: cri.ContainerStatusInfo{
			RuntimeSpec: nil,
		},
	}

	saveCpuset := func(dir, cpuset string) {
		if err := os.MkdirAll(dir, 0777); err != nil {
			t.Fatal(err)
		}

		f, err := os.Create(path.Join(dir, "cpuset.cpus"))
		if err != nil {
			t.Fatal(err)
		}

		defer func() {
			if err := f.Close(); err != nil {
				t.Fatal(err)
			}
		}()

		if _, err := f.WriteString(cpuset); err != nil {
			t.Fatal(err)
		}
	}

	generateEKSCgroup := func(cpuset string) func() string {
		temp := t.TempDir()
		dir := path.Join(temp, fmt.Sprintf("/cpuset/kubepods.slice/kubepods-pod%s.slice/docker-%s.scope", strings.ReplaceAll(podUID, "-", "_"), scyllaContainerID))

		saveCpuset(dir, cpuset)

		return func() string {
			return temp
		}
	}

	generateGKECgroup := func(cpuset string) func() string {
		temp := t.TempDir()
		dir := path.Join(temp, fmt.Sprintf("/cpuset/kubepods/pod%s/%s", podUID, scyllaContainerID))

		saveCpuset(dir, cpuset)

		return func() string {
			return temp
		}
	}

	generateMinikubeCgroup := func(cpuset string) func() string {
		temp := t.TempDir()
		dir := path.Join(temp, fmt.Sprintf("/cpuset/kubepods.slice/kubepods-pod%s.slice/docker-%s.scope", strings.ReplaceAll(podUID, "-", "_"), scyllaContainerID))

		saveCpuset(dir, cpuset)

		return func() string {
			return temp
		}
	}

	ts := []struct {
		name            string
		containerStatus *cri.ContainerStatus
		expectedCpuSet  string
		expectedErr     error
		cgroupFsPath    func() string
	}{
		{
			name:            "cpuset taken from CRI",
			containerStatus: containerStatusWithRuntimeInfo,
			cgroupFsPath:    t.TempDir,
			expectedCpuSet:  "0-1",
		},
		{
			name:            "cpuset taken from cgroup with EKS pattern",
			containerStatus: emptyContainerStatus,
			expectedCpuSet:  "0-2",
			cgroupFsPath:    generateEKSCgroup("0-2"),
		},
		{
			name:            "cpuset taken from cgroup with GKE pattern",
			containerStatus: emptyContainerStatus,
			expectedCpuSet:  "0-3",
			cgroupFsPath:    generateGKECgroup("0-3"),
		},
		{
			name:            "cpuset taken from cgroup with Minikube pattern",
			containerStatus: emptyContainerStatus,
			expectedCpuSet:  "0-4",
			cgroupFsPath:    generateMinikubeCgroup("0-4"),
		},
		{
			name:            "empty CRI and missing cgroup",
			containerStatus: emptyContainerStatus,
			cgroupFsPath:    t.TempDir,
			expectedErr:     fmt.Errorf("can't find Scylla container cpuset"),
		},
	}

	for i := range ts {
		test := ts[i]
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
			defer cancel()

			criClient := &fakeCriClient{
				expectations: map[string]*cri.ContainerStatus{
					scyllaContainerID: test.containerStatus,
				},
			}

			scyllaCpuset, err := getContainerCPUs(ctx, criClient, podUID, scyllaContainerID, test.cgroupFsPath())
			if !reflect.DeepEqual(err, test.expectedErr) {
				t.Errorf("expected %v error, got %v", test.expectedErr, err)
			}

			if test.expectedCpuSet != scyllaCpuset.String() {
				t.Errorf("expected %v Scylla cpuset, got %v", test.expectedCpuSet, scyllaCpuset)
			}
		})
	}
}

func TestGetIRQCPUs(t *testing.T) {
	newScyllaPod := func(qosClass corev1.PodQOSClass) (*corev1.Pod, string) {
		cid := fmt.Sprintf("docker://%s", strings.ReplaceAll(uuid.MustRandom().String(), "-", ""))
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					naming.ClusterNameLabel: "cluster",
				},
			},
			Status: corev1.PodStatus{
				QOSClass: qosClass,
				ContainerStatuses: []corev1.ContainerStatus{
					{
						Name:        naming.ScyllaContainerName,
						ContainerID: cid,
						State: corev1.ContainerState{
							Running: &corev1.ContainerStateRunning{StartedAt: metav1.Now()},
						},
					},
				},
			},
		}
		return pod, cid
	}

	guaranteedScylla1, scylla1CID := newScyllaPod(corev1.PodQOSGuaranteed)
	guaranteedScylla2, scylla2CID := newScyllaPod(corev1.PodQOSGuaranteed)
	burstableScylla, scylla3CID := newScyllaPod(corev1.PodQOSBurstable)

	criContainerStatus := func(cpus string) *cri.ContainerStatus {
		return &cri.ContainerStatus{
			Info: cri.ContainerStatusInfo{
				RuntimeSpec: &cri.ContainerStatusInfoRuntimeSpec{
					Linux: cri.ContainerStatusInfoRuntimeSpecLinux{
						Resources: cri.ContainerStatusInfoRuntimeSpecLinuxResources{
							CPU: cri.ContainerStatusInfoRuntimeSpecLinuxResourcesCPU{
								Cpus: cpus,
							},
						},
					},
				},
			},
		}
	}

	ts := []struct {
		name              string
		containerStatuses map[string]*cri.ContainerStatus
		scyllaPods        []*corev1.Pod
		hostCpuSet        string
		expectedCpuSet    string
		expectedErr       error
	}{
		{
			name: "single guaranteed Scylla",
			containerStatuses: map[string]*cri.ContainerStatus{
				mustStripContainerID(scylla1CID): criContainerStatus("0-1"),
			},
			scyllaPods: []*corev1.Pod{
				guaranteedScylla1,
			},
			hostCpuSet:     "0-5",
			expectedCpuSet: "2-5",
		},
		{
			name: "multiple guaranteed Scylla",
			containerStatuses: map[string]*cri.ContainerStatus{
				mustStripContainerID(scylla1CID): criContainerStatus("0-1"),
				mustStripContainerID(scylla2CID): criContainerStatus("5"),
			},
			scyllaPods: []*corev1.Pod{
				guaranteedScylla1,
				guaranteedScylla2,
			},
			hostCpuSet:     "0-5",
			expectedCpuSet: "2-4",
		},
		{
			name: "burstable Scylla",
			containerStatuses: map[string]*cri.ContainerStatus{
				mustStripContainerID(scylla3CID): criContainerStatus("0-5"),
			},
			scyllaPods: []*corev1.Pod{
				burstableScylla,
			},
			hostCpuSet:     "0-5",
			expectedCpuSet: "0-5",
		},
	}
	for i := range ts {
		test := ts[i]
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
			defer cancel()

			criClient := &fakeCriClient{
				expectations: test.containerStatuses,
			}

			hostFullCpuset, err := cpuset.Parse(test.hostCpuSet)
			if err != nil {
				t.Fatal(err)
			}

			irqCpuSet, err := getIRQCPUs(ctx, criClient, test.scyllaPods, hostFullCpuset, defaultCgroupMountpoint)
			if !reflect.DeepEqual(err, test.expectedErr) {
				t.Errorf("expected %v error, got %v", test.expectedErr, err)
			}

			if test.expectedCpuSet != irqCpuSet.String() {
				t.Errorf("expected %v IRQ cpuset, got %v", test.expectedCpuSet, irqCpuSet)
			}
		})
	}
}

func mustStripContainerID(containerID string) string {
	cid, err := stripContainerID(containerID)
	if err != nil {
		panic(err)
	}
	return cid
}
