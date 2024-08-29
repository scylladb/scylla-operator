// Copyright (c) 2022-2023 ScyllaDB.

package nodeconfig

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configassests "github.com/scylladb/scylla-operator/assets/config"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/internalapi"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	scyllafixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scylla"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"
)

var _ = g.Describe("Node Setup", framework.Serial, func() {
	f := framework.NewFramework("nodesetup")

	ncTemplate := scyllafixture.NodeConfig.ReadOrFail()
	var matchingNodes []*corev1.Node

	g.JustBeforeEach(func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		g.By("Verifying there is at least one scylla node")
		var err error
		matchingNodes, err = utils.GetMatchingNodesForNodeConfig(ctx, f.KubeAdminClient().CoreV1(), ncTemplate)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(matchingNodes).NotTo(o.HaveLen(0))
		framework.Infof("There are %d scylla nodes", len(matchingNodes))
	})

	g.DescribeTable("should make RAID0 array out of loop devices, format it to XFS, and mount at desired location", func(numberOfDevices int) {
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
						Image: configassests.Project.OperatorTests.NodeSetupImage,
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
		clientPod, err = controllerhelpers.WaitForPodState(waitCtx1, f.KubeClient().CoreV1().Pods(clientPod.Namespace), clientPod.Name, controllerhelpers.WaitForStateOptions{}, utils.PodIsRunning)
		o.Expect(err).NotTo(o.HaveOccurred())

		raidName := rand.String(8)
		mountPath := fmt.Sprintf("/mnt/disk-setup-%s", f.Namespace())
		hostMountPath := path.Join("/host", mountPath)

		filesystem := scyllav1alpha1.XFSFilesystem
		mountOptions := []string{"prjquota"}

		loopDeviceNames := make([]string, 0, numberOfDevices)
		for i := range numberOfDevices {
			loopDeviceNames = append(loopDeviceNames, fmt.Sprintf("disk-%s-%d", f.Namespace(), i))
		}

		nc.Spec.LocalDiskSetup = &scyllav1alpha1.LocalDiskSetup{
			LoopDevices: func() []scyllav1alpha1.LoopDeviceConfiguration {
				var ldcs []scyllav1alpha1.LoopDeviceConfiguration

				for _, ldName := range loopDeviceNames {
					ldcs = append(ldcs, scyllav1alpha1.LoopDeviceConfiguration{
						Name:      ldName,
						ImagePath: fmt.Sprintf("/mnt/%s-%s.img", ldName, f.Namespace()),
						Size:      resource.MustParse("32M"),
					})
				}

				return ldcs
			}(),
			RAIDs: []scyllav1alpha1.RAIDConfiguration{
				{
					Name: raidName,
					Type: scyllav1alpha1.RAID0Type,
					RAID0: &scyllav1alpha1.RAID0Options{
						Devices: scyllav1alpha1.DeviceDiscovery{
							NameRegex:  fmt.Sprintf(`^/dev/loops/disk-%s-\d+$`, f.Namespace()),
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

		rc := framework.NewRestoringCleaner(
			ctx,
			f.KubeAdminClient(),
			f.DynamicAdminClient(),
			nodeConfigResourceInfo,
			nc.Namespace,
			nc.Name,
			framework.RestoreStrategyRecreate,
		)
		f.AddCleaners(rc)
		rc.DeleteObject(ctx, true)

		g.By("Creating a NodeConfig")
		nc, err = f.ScyllaAdminClient().ScyllaV1alpha1().NodeConfigs().Create(ctx, nc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the NodeConfig to deploy")
		ctx1, ctx1Cancel := context.WithTimeout(ctx, nodeConfigRolloutTimeout)
		defer ctx1Cancel()
		nc, err = controllerhelpers.WaitForNodeConfigState(
			ctx1,
			f.ScyllaAdminClient().ScyllaV1alpha1().NodeConfigs(),
			nc.Name,
			controllerhelpers.WaitForStateOptions{TolerateDelete: false},
			utils.IsNodeConfigRolledOut,
			utils.IsNodeConfigDoneWithNodeTuningFunc(matchingNodes),
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyNodeConfig(ctx, f.KubeAdminClient(), nc)

		hostLoopsDir := "/host/dev/loops"
		framework.By("Checking if loop devices has been created at %q", hostLoopsDir)
		o.Eventually(func(g o.Gomega) {
			for _, ldName := range loopDeviceNames {
				loopDevicePath := path.Join(hostLoopsDir, ldName)
				stdout, stderr, err := executeInPod(f.ClientConfig(), f.KubeClient().CoreV1(), clientPod, "stat", loopDevicePath)
				g.Expect(err).NotTo(o.HaveOccurred(), stdout, stderr)
			}
		}).WithPolling(1 * time.Second).WithTimeout(3 * time.Minute).Should(o.Succeed())

		var findmntOutput string
		o.Eventually(func(g o.Gomega) {
			stdout, stderr, err := executeInPod(f.ClientConfig(), f.KubeClient().CoreV1(), clientPod, "findmnt", "--raw", "--output=SOURCE", "--noheadings", hostMountPath)
			g.Expect(err).NotTo(o.HaveOccurred(), stdout, stderr)
			findmntOutput = stdout
		}).WithPolling(10 * time.Second).WithTimeout(3 * time.Minute).Should(o.Succeed())
		o.Expect(findmntOutput).NotTo(o.BeEmpty())

		discoveredRaidDevice := strings.TrimSpace(findmntOutput)
		o.Expect(discoveredRaidDevice).NotTo(o.BeEmpty())
		discoveredRaidDeviceOnHost := path.Join("/host", discoveredRaidDevice)

		framework.By("Checking if RAID device has been created at %q", discoveredRaidDevice)
		o.Eventually(func(g o.Gomega) {
			stdout, stderr, err := executeInPod(f.ClientConfig(), f.KubeClient().CoreV1(), clientPod, "stat", discoveredRaidDeviceOnHost)
			g.Expect(err).NotTo(o.HaveOccurred(), stdout, stderr)

			stdout, stderr, err = executeInPod(f.ClientConfig(), f.KubeClient().CoreV1(), clientPod, "readlink", "-f", discoveredRaidDeviceOnHost)
			g.Expect(err).NotTo(o.HaveOccurred(), stdout, stderr)

			raidDeviceName := path.Base(discoveredRaidDeviceOnHost)

			stdout, stderr, err = executeInPod(f.ClientConfig(), f.KubeClient().CoreV1(), clientPod, "cat", fmt.Sprintf("/sys/block/%s/md/level", raidDeviceName))
			g.Expect(err).NotTo(o.HaveOccurred(), stdout, stderr)

			raidLevel := strings.TrimSpace(stdout)
			g.Expect(raidLevel).To(o.Equal("raid0"))
		}).WithPolling(1 * time.Second).WithTimeout(3 * time.Minute).Should(o.Succeed())

		framework.By("Checking if RAID device has been formatted")
		o.Eventually(func(g o.Gomega) {
			stdout, stderr, err := executeInPod(f.ClientConfig(), f.KubeClient().CoreV1(), clientPod, "blkid", "--output=value", "--match-tag=TYPE", discoveredRaidDeviceOnHost)
			g.Expect(err).NotTo(o.HaveOccurred(), stderr)

			g.Expect(strings.TrimSpace(stdout)).To(o.Equal("xfs"))
		}).WithPolling(1 * time.Second).WithTimeout(3 * time.Minute).Should(o.Succeed())

		framework.By("Checking if RAID was mounted at the provided location with correct options")
		o.Eventually(func(g o.Gomega) {
			stdout, stderr, err := executeInPod(f.ClientConfig(), f.KubeClient().CoreV1(), clientPod, "mount")
			g.Expect(err).NotTo(o.HaveOccurred(), stderr)

			// mount output format
			// /dev/md337 on /host/mnt/persistent-volume type xfs (rw,relatime,attr2,inode64,logbufs=8,logbsize=32k,sunit=2048,swidth=2048,prjquota)
			g.Expect(stdout).To(o.MatchRegexp(`%s on %s type %s \(.*%s.*\)`, discoveredRaidDevice, hostMountPath, filesystem, mountOptions[0]))
		}).WithPolling(1 * time.Second).WithTimeout(3 * time.Minute).Should(o.Succeed())

		// Disable disk setup before cleanup to not fight over resources
		// TODO: can be removed once we support cleanup of filesystem and raid array
		err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
			var freshNC, updatedNC *scyllav1alpha1.NodeConfig
			freshNC, err = f.ScyllaAdminClient().ScyllaV1alpha1().NodeConfigs().Get(ctx, nc.Name, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			freshNC.Spec.LocalDiskSetup = nil
			updatedNC, err = f.ScyllaAdminClient().ScyllaV1alpha1().NodeConfigs().Update(ctx, freshNC, metav1.UpdateOptions{})
			if err == nil {
				nc = updatedNC
			}
			return err
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the NodeConfig to deploy")
		ctx2, ctx2Cancel := context.WithTimeout(ctx, nodeConfigRolloutTimeout)
		defer ctx2Cancel()
		nc, err = controllerhelpers.WaitForNodeConfigState(
			ctx2,
			f.ScyllaAdminClient().ScyllaV1alpha1().NodeConfigs(),
			nc.Name,
			controllerhelpers.WaitForStateOptions{TolerateDelete: false},
			utils.IsNodeConfigRolledOut,
			utils.IsNodeConfigDoneWithNodeTuningFunc(matchingNodes),
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyNodeConfig(ctx, f.KubeAdminClient(), nc)
	},
		g.Entry("out of one loop device", 1),
		g.Entry("out of three loop devices", 3),
	)

	g.It("should propagate degraded conditions for invalid configuration", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		nc := ncTemplate.DeepCopy()

		nc.Spec.LocalDiskSetup = &scyllav1alpha1.LocalDiskSetup{
			Mounts: []scyllav1alpha1.MountConfiguration{
				{
					Device:             fmt.Sprintf("/dev/%s", f.Namespace()),
					MountPoint:         fmt.Sprintf("/mnt/%s", f.Namespace()),
					FSType:             string(scyllav1alpha1.XFSFilesystem),
					UnsupportedOptions: []string{"prjquota"},
				},
			},
		}

		g.By("Creating NodeConfig")
		nc, err := f.ScyllaAdminClient().ScyllaV1alpha1().NodeConfigs().Create(ctx, nc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		isNodeConfigMountControllerDegraded := func(nc *scyllav1alpha1.NodeConfig) (bool, error) {
			statusConditions := nc.Status.Conditions.ToMetaV1Conditions()

			if !helpers.IsStatusConditionPresentAndTrue(statusConditions, scyllav1alpha1.AvailableCondition, nc.Generation) {
				return false, nil
			}

			if !helpers.IsStatusConditionPresentAndFalse(statusConditions, scyllav1alpha1.ProgressingCondition, nc.Generation) {
				return false, nil
			}

			if !helpers.IsStatusConditionPresentAndTrue(statusConditions, scyllav1alpha1.DegradedCondition, nc.Generation) {
				return false, nil
			}

			for _, n := range matchingNodes {
				nodeDegradedConditionType := fmt.Sprintf(internalapi.NodeDegradedConditionFormat, n.GetName())
				if !helpers.IsStatusConditionPresentAndTrue(statusConditions, nodeDegradedConditionType, nc.Generation) {
					return false, nil
				}

				nodeRaidControllerConditionType := fmt.Sprintf("MountControllerNode%sDegraded", n.GetName())
				if !helpers.IsStatusConditionPresentAndTrue(statusConditions, nodeRaidControllerConditionType, nc.Generation) {
					return false, nil
				}
			}

			return true, nil
		}

		framework.By("Waiting for NodeConfig to be in a degraded state")
		ctx1, ctx1Cancel := context.WithTimeout(ctx, nodeConfigRolloutTimeout)
		defer ctx1Cancel()
		nc, err = controllerhelpers.WaitForNodeConfigState(
			ctx1,
			f.ScyllaAdminClient().ScyllaV1alpha1().NodeConfigs(),
			nc.Name,
			controllerhelpers.WaitForStateOptions{TolerateDelete: false},
			utils.IsNodeConfigDoneWithNodeTuningFunc(matchingNodes),
			isNodeConfigMountControllerDegraded,
		)
		o.Expect(err).NotTo(o.HaveOccurred())
	})
})

func executeInPod(config *rest.Config, client corev1client.CoreV1Interface, pod *corev1.Pod, command string, args ...string) (string, string, error) {
	return utils.ExecWithOptions(config, client, utils.ExecOptions{
		Command:       append([]string{command}, args...),
		Namespace:     pod.Namespace,
		PodName:       pod.Name,
		ContainerName: pod.Spec.Containers[0].Name,
		CaptureStdout: true,
		CaptureStderr: true,
	})
}
