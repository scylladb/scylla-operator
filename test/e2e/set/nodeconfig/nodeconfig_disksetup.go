// Copyright (c) 2022-2023 ScyllaDB.

package nodeconfig

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	scyllafixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scylla"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	"github.com/scylladb/scylla-operator/test/e2e/utils/image"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	cacheddiscovery "k8s.io/client-go/discovery/cached/memory"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
)

var _ = g.Describe("Node Setup", framework.Serial, func() {
	defer g.GinkgoRecover()
	f := framework.NewFramework("nodesetup")

	ncTemplate := scyllafixture.NodeConfig.ReadOrFail()
	var matchingNodes []*corev1.Node

	preconditionSuccessful := false
	g.JustBeforeEach(func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Make sure the NodeConfig is not present.
		framework.By("Making sure NodeConfig %q, doesn't exist", naming.ObjRef(ncTemplate))
		_, err := f.ScyllaAdminClient().ScyllaV1alpha1().NodeConfigs().Get(ctx, ncTemplate.Name, metav1.GetOptions{})
		if err == nil {
			framework.Failf("NodeConfig %q can't be present before running this test", naming.ObjRef(ncTemplate))
		} else if !apierrors.IsNotFound(err) {
			framework.Failf("Can't get NodeConfig %q: %v", naming.ObjRef(ncTemplate), err)
		}

		preconditionSuccessful = true

		g.By("Verifying there is at least one scylla node")
		matchingNodes, err = utils.GetMatchingNodesForNodeConfig(ctx, f.KubeAdminClient().CoreV1(), ncTemplate)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(matchingNodes).NotTo(o.HaveLen(0))
		framework.Infof("There are %d scylla nodes", len(matchingNodes))
	})

	g.JustAfterEach(func() {
		if !preconditionSuccessful {
			return
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		if len(framework.TestContext.ArtifactsDir) > 0 {
			framework.By("Collecting NodeConfig namespace dump")

			dir := path.Join(framework.TestContext.ArtifactsDir, "nodeconfig-related", f.Namespace())
			err := os.MkdirAll(dir, 0777)
			if err != nil && !os.IsExist(err) {
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			err = framework.DumpNamespace(ctx, cacheddiscovery.NewMemCacheClient(f.KubeAdminClient().Discovery()), f.DynamicAdminClient(), f.KubeAdminClient().CoreV1(), dir, naming.ScyllaOperatorNodeTuningNamespace)
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		framework.By("Deleting NodeConfig %q, if it exists", naming.ObjRef(ncTemplate))
		err := f.ScyllaAdminClient().ScyllaV1alpha1().NodeConfigs().Delete(context.Background(), ncTemplate.Name, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			framework.Failf("Can't delete NodeConfig %q: %v", naming.ObjRef(ncTemplate), err)
		}
		if !apierrors.IsNotFound(err) {
			err = framework.WaitForObjectDeletion(context.Background(), f.DynamicAdminClient(), scyllav1alpha1.GroupVersion.WithResource("nodeconfigs"), ncTemplate.Namespace, ncTemplate.Name, nil)
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	})

	g.DescribeTable("should make RAID0, format it to XFS, and mount at desired location", func(numberOfDevices int) {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		nc := ncTemplate.DeepCopy()

		framework.By("Creating a client Pod")
		clientPod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "client",
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "client",
						Image: image.GetE2EImage(image.OperatorNodeSetup),
						Command: []string{
							"/bin/sh",
							"-c",
							"sleep 3600",
						},
						SecurityContext: &corev1.SecurityContext{
							Privileged: pointer.Ptr(true),
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:             "host",
								MountPath:        "/host",
								MountPropagation: pointer.Ptr(corev1.MountPropagationBidirectional),
							},
						},
					},
				},
				Tolerations:  nc.Spec.Placement.Tolerations,
				NodeSelector: nc.Spec.Placement.NodeSelector,
				Affinity:     &nc.Spec.Placement.Affinity,
				Volumes: []corev1.Volume{
					{
						Name: "host",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: "/",
							},
						},
					},
				},
				TerminationGracePeriodSeconds: pointer.Ptr(int64(1)),
				RestartPolicy:                 corev1.RestartPolicyNever,
			},
		}

		clientPod, err := f.KubeClient().CoreV1().Pods(f.Namespace()).Create(ctx, clientPod, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		waitCtx1, waitCtx1Cancel := utils.ContextForPodStartup(ctx)
		defer waitCtx1Cancel()
		clientPod, err = utils.WaitForPodState(waitCtx1, f.KubeClient().CoreV1().Pods(clientPod.Namespace), clientPod.Name, utils.WaitForStateOptions{}, utils.PodIsRunning)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Creating %d loop devices", numberOfDevices)
		staticDevices, cleanup, err := makeLoopDevices(f.KubeClient().CoreV1(), clientPod, numberOfDevices)
		o.Expect(err).NotTo(o.HaveOccurred())

		defer func() {
			err := cleanup()
			o.Expect(err).NotTo(o.HaveOccurred())
		}()

		raidName := rand.String(8)
		mountPath := fmt.Sprintf("/mnt/disk-setup-%s", f.Namespace())
		hostMountPath := path.Join("/host", mountPath)

		filesystem := scyllav1alpha1.XFSFilesystem
		mountOptions := []string{"prjquota"}

		nc.Spec.LocalDiskSetup = &scyllav1alpha1.LocalDiskSetup{
			RAIDs: []scyllav1alpha1.RAIDConfiguration{
				{
					Name: raidName,
					Type: scyllav1alpha1.RAID0Type,
					RAID0: &scyllav1alpha1.RAID0Options{
						Devices: scyllav1alpha1.DeviceDiscovery{
							NameRegex:  strings.Join(staticDevices, "|"),
							ModelRegex: ".*",
						},
					},
				},
			},
			Mounts: []scyllav1alpha1.MountConfiguration{
				{
					Device:             raidName,
					MountPoint:         mountPath,
					FSType:             string(filesystem),
					UnsupportedOptions: mountOptions,
				},
			},
			Filesystems: []scyllav1alpha1.FilesystemConfiguration{
				{
					Device: raidName,
					Type:   filesystem,
				},
			},
		}

		g.By("Creating a NodeConfig")
		nc, err = f.ScyllaAdminClient().ScyllaV1alpha1().NodeConfigs().Create(ctx, nc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the NodeConfig to deploy")
		ctx1, ctx1Cancel := context.WithTimeout(ctx, nodeConfigRolloutTimeout)
		defer ctx1Cancel()
		nc, err = utils.WaitForNodeConfigState(
			ctx1,
			f.ScyllaAdminClient().ScyllaV1alpha1().NodeConfigs(),
			nc.Name,
			utils.WaitForStateOptions{TolerateDelete: false},
			utils.IsNodeConfigRolledOut,
			utils.IsNodeConfigDoneWithNodes(matchingNodes),
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		stdout, stderr, err := executeInPod(f.KubeClient().CoreV1(), clientPod, "findmnt", "--raw", "--output=SOURCE", "--noheadings", hostMountPath)
		o.Expect(err).NotTo(o.HaveOccurred(), stdout, stderr)

		discoveredRaidDevice := strings.TrimSpace(stdout)
		discoveredRaidDeviceOnHost := path.Join("/host", discoveredRaidDevice)

		framework.By("Checking if RAID device has been created at %q", discoveredRaidDevice)
		o.Eventually(func(g o.Gomega) {
			stdout, stderr, err := executeInPod(f.KubeClient().CoreV1(), clientPod, "stat", discoveredRaidDeviceOnHost)
			g.Expect(err).NotTo(o.HaveOccurred(), stdout, stderr)

			stdout, stderr, err = executeInPod(f.KubeClient().CoreV1(), clientPod, "readlink", "-f", discoveredRaidDeviceOnHost)
			g.Expect(err).NotTo(o.HaveOccurred(), stdout, stderr)

			raidDeviceName := path.Base(discoveredRaidDeviceOnHost)

			stdout, stderr, err = executeInPod(f.KubeClient().CoreV1(), clientPod, "cat", fmt.Sprintf("/sys/block/%s/md/level", raidDeviceName))
			g.Expect(err).NotTo(o.HaveOccurred(), stdout, stderr)

			raidLevel := strings.TrimSpace(stdout)
			g.Expect(raidLevel).To(o.Equal("raid0"))
		}).WithPolling(1 * time.Second).WithTimeout(3 * time.Minute).Should(o.Succeed())

		defer func() {
			framework.By("Stopping RAID device at %q", discoveredRaidDeviceOnHost)
			stdout, stderr, err := executeInPod(f.KubeClient().CoreV1(), clientPod, "mdadm", "--stop", discoveredRaidDeviceOnHost)
			o.Expect(err).NotTo(o.HaveOccurred(), stdout, stderr)
		}()

		framework.By("Checking if RAID device has been formatted")
		o.Eventually(func(g o.Gomega) {
			stdout, stderr, err := executeInPod(f.KubeClient().CoreV1(), clientPod, "blkid", "--output=value", "--match-tag=TYPE", discoveredRaidDeviceOnHost)
			g.Expect(err).NotTo(o.HaveOccurred(), stderr)

			g.Expect(strings.TrimSpace(stdout)).To(o.Equal("xfs"))
		}).WithPolling(1 * time.Second).WithTimeout(3 * time.Minute).Should(o.Succeed())

		framework.By("Checking if RAID was mounted at the provided location with correct options")
		o.Eventually(func(g o.Gomega) {
			stdout, stderr, err := executeInPod(f.KubeClient().CoreV1(), clientPod, "mount")
			g.Expect(err).NotTo(o.HaveOccurred(), stderr)

			// mount output format
			// /dev/md337 on /host/mnt/persistent-volume type xfs (rw,relatime,attr2,inode64,logbufs=8,logbsize=32k,sunit=2048,swidth=2048,prjquota)
			g.Expect(stdout).To(o.MatchRegexp(`%s on %s type %s \(.*%s.*\)`, discoveredRaidDevice, hostMountPath, filesystem, mountOptions[0]))
		}).WithPolling(1 * time.Second).WithTimeout(3 * time.Minute).Should(o.Succeed())

		// Disable disk setup before cleanup to not fight over resources
		// TODO: can be removed once we support cleanup of filesystem and raid array
		nc.Spec.LocalDiskSetup = nil
		nc, err = f.ScyllaAdminClient().ScyllaV1alpha1().NodeConfigs().Update(ctx, nc, metav1.UpdateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the NodeConfig to deploy")
		ctx2, ctx2Cancel := context.WithTimeout(ctx, nodeConfigRolloutTimeout)
		defer ctx2Cancel()
		nc, err = utils.WaitForNodeConfigState(
			ctx2,
			f.ScyllaAdminClient().ScyllaV1alpha1().NodeConfigs(),
			nc.Name,
			utils.WaitForStateOptions{TolerateDelete: false},
			utils.IsNodeConfigRolledOut,
			utils.IsNodeConfigDoneWithNodes(matchingNodes),
		)
		o.Expect(err).NotTo(o.HaveOccurred())
	},
		g.Entry("out of one loop device", 1),
		g.Entry("out of three loop devices", 3),
	)
})

func makeLoopDevices(client corev1client.CoreV1Interface, pod *corev1.Pod, disks int) ([]string, func() error, error) {
	loopDevices := make([]string, 0, disks)
	diskImages := make([]string, 0, disks)

	for i := 0; i < disks; i++ {
		diskImage := fmt.Sprintf("/host/mnt/disk-%s.img", rand.String(6))
		stdout, stderr, err := executeInPod(client, pod, "dd", "if=/dev/zero", fmt.Sprintf("of=%s", diskImage), "bs=1M", "count=32")
		if err != nil {
			return nil, nil, fmt.Errorf("can't create disk image: %w, stdout: %q, stderr: %q", err, stdout, stderr)
		}

		diskImages = append(diskImages, diskImage)

		stdout, stderr, err = executeInPod(client, pod, "losetup", "--show", "--find", diskImage)
		if err != nil {
			return nil, nil, fmt.Errorf("can't create loop device: %w, stdout: %q, stderr: %q", err, stdout, stderr)
		}

		loopDevice := strings.TrimSpace(stdout)
		loopDevices = append(loopDevices, loopDevice)
	}

	if len(loopDevices) != disks {
		return nil, nil, fmt.Errorf("expected to create %d devices, got %d", disks, len(loopDevices))
	}

	cleanup := func() error {
		return detachDevices(client, pod, loopDevices, diskImages)
	}

	return loopDevices, cleanup, nil
}

func detachDevices(client corev1client.CoreV1Interface, pod *corev1.Pod, devices []string, diskImages []string) error {
	for i, device := range devices {
		stdout, stderr, err := executeInPod(client, pod, "losetup", "--detach", device)
		if err != nil {
			return fmt.Errorf("can't detach loop device %q: %w, stdout: %q, stderr: %q", device, err, stdout, stderr)
		}
		stdout, stderr, err = executeInPod(client, pod, "rm", diskImages[i])
		if err != nil {
			return fmt.Errorf("can't remove disk image %q: %w, stdout: %q, stderr: %q", diskImages[i], err, stdout, stderr)
		}
	}

	return nil
}

func executeInPod(client corev1client.CoreV1Interface, pod *corev1.Pod, command string, args ...string) (string, string, error) {
	return utils.ExecWithOptions(client, utils.ExecOptions{
		Command:       append([]string{command}, args...),
		Namespace:     pod.Namespace,
		PodName:       pod.Name,
		ContainerName: pod.Spec.Containers[0].Name,
		CaptureStdout: true,
		CaptureStderr: true,
	})
}
