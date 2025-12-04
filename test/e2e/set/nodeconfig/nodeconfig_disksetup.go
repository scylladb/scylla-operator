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
	"k8s.io/apimachinery/pkg/types"
	apimachineryutilrand "k8s.io/apimachinery/pkg/util/rand"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"
)

var (
	// xfsVolumeSize is a size of a default xfs filesystem we create.
	// Beware that `mkfs.xfs` fails unless it has at least 300MB available.
	xfsVolumeSize = resource.MustParse("320M")
)

var _ = g.Describe("Node Setup", framework.Serial, func() {
	f := framework.NewFramework("nodesetup")

	ncTemplate := scyllafixture.NodeConfig.ReadOrFail()
	var nodeUnderTest *corev1.Node

	g.JustBeforeEach(func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		g.By("Verifying there is at least one scylla node")
		var err error
		matchingNodes, err := utils.GetMatchingNodesForNodeConfig(ctx, f.KubeAdminClient().CoreV1(), ncTemplate)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(matchingNodes).NotTo(o.HaveLen(0))
		nodeUnderTest = matchingNodes[0]
		framework.Infof("There are %d scylla nodes. %s will be used in tests.", len(matchingNodes), nodeUnderTest.GetName())
	})

	g.DescribeTable("should make RAID0 array out of loop devices, format it to XFS, and mount at desired location", func(numberOfDevices int) {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		nc := ncTemplate.DeepCopy()

		framework.By("Creating a client Pod")
		clientPod := newClientPod(nc, nodeUnderTest)

		clientPod, err := f.KubeClient().CoreV1().Pods(f.Namespace()).Create(ctx, clientPod, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		waitCtx1, waitCtx1Cancel := utils.ContextForPodStartup(ctx)
		defer waitCtx1Cancel()
		clientPod, err = controllerhelpers.WaitForPodState(waitCtx1, f.KubeClient().CoreV1().Pods(clientPod.Namespace), clientPod.Name, controllerhelpers.WaitForStateOptions{}, utils.PodIsRunning)
		o.Expect(err).NotTo(o.HaveOccurred())

		raidName := apimachineryutilrand.String(8)
		mountPath := fmt.Sprintf("/var/lib/disk-setup-%s", f.Namespace())
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
						ImagePath: fmt.Sprintf("/var/lib/%s-%s.img", ldName, f.Namespace()),
						Size:      xfsVolumeSize,
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
			f.AdminClientConfig(),
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
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyNodeConfig(ctx, f.KubeAdminClient(), nc)

		hostLoopsDir := "/host/dev/loops"
		framework.By("Checking if loop devices has been created at %q", hostLoopsDir)
		o.Eventually(func(g o.Gomega) {
			for _, ldName := range loopDeviceNames {
				loopDevicePath := path.Join(hostLoopsDir, ldName)
				stdout, stderr, err := utils.ExecuteInPod(ctx, f.ClientConfig(), f.KubeClient().CoreV1(), clientPod, "", "stat", loopDevicePath)
				g.Expect(err).NotTo(o.HaveOccurred(), stdout, stderr)
			}
		}).WithPolling(1 * time.Second).WithTimeout(3 * time.Minute).Should(o.Succeed())

		var findmntOutput string
		o.Eventually(func(g o.Gomega) {
			stdout, stderr, err := utils.ExecuteInPod(ctx, f.ClientConfig(), f.KubeClient().CoreV1(), clientPod, "", "findmnt", "--raw", "--output=SOURCE", "--noheadings", hostMountPath)
			g.Expect(err).NotTo(o.HaveOccurred(), stdout, stderr)
			findmntOutput = stdout
		}).WithPolling(10 * time.Second).WithTimeout(3 * time.Minute).Should(o.Succeed())
		o.Expect(findmntOutput).NotTo(o.BeEmpty())

		discoveredRaidDevice := strings.TrimSpace(findmntOutput)
		o.Expect(discoveredRaidDevice).NotTo(o.BeEmpty())
		discoveredRaidDeviceOnHost := path.Join("/host", discoveredRaidDevice)

		framework.By("Checking if RAID device has been created at %q", discoveredRaidDevice)
		o.Eventually(func(g o.Gomega) {
			stdout, stderr, err := utils.ExecuteInPod(ctx, f.ClientConfig(), f.KubeClient().CoreV1(), clientPod, "", "stat", discoveredRaidDeviceOnHost)
			g.Expect(err).NotTo(o.HaveOccurred(), stdout, stderr)

			stdout, stderr, err = utils.ExecuteInPod(ctx, f.ClientConfig(), f.KubeClient().CoreV1(), clientPod, "", "readlink", "-f", discoveredRaidDeviceOnHost)
			g.Expect(err).NotTo(o.HaveOccurred(), stdout, stderr)

			raidDeviceName := path.Base(discoveredRaidDeviceOnHost)

			stdout, stderr, err = utils.ExecuteInPod(ctx, f.ClientConfig(), f.KubeClient().CoreV1(), clientPod, "", "cat", fmt.Sprintf("/sys/block/%s/md/level", raidDeviceName))
			g.Expect(err).NotTo(o.HaveOccurred(), stdout, stderr)

			raidLevel := strings.TrimSpace(stdout)
			g.Expect(raidLevel).To(o.Equal("raid0"))
		}).WithPolling(1 * time.Second).WithTimeout(3 * time.Minute).Should(o.Succeed())

		framework.By("Checking if RAID device has been formatted")
		o.Eventually(func(g o.Gomega) {
			stdout, stderr, err := utils.ExecuteInPod(ctx, f.ClientConfig(), f.KubeClient().CoreV1(), clientPod, "", "blkid", "--output=value", "--match-tag=TYPE", discoveredRaidDeviceOnHost)
			g.Expect(err).NotTo(o.HaveOccurred(), stderr)

			g.Expect(strings.TrimSpace(stdout)).To(o.Equal("xfs"))
		}).WithPolling(1 * time.Second).WithTimeout(3 * time.Minute).Should(o.Succeed())

		framework.By("Checking if RAID was mounted at the provided location with correct options")
		o.Eventually(func(g o.Gomega) {
			stdout, stderr, err := utils.ExecuteInPod(ctx, f.ClientConfig(), f.KubeClient().CoreV1(), clientPod, "", "mount")
			g.Expect(err).NotTo(o.HaveOccurred(), stderr)

			// mount output format
			// /dev/md337 on /host/var/lib/disk-setup-* type xfs (rw,relatime,attr2,inode64,logbufs=8,logbsize=32k,sunit=2048,swidth=2048,prjquota)
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
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyNodeConfig(ctx, f.KubeAdminClient(), nc)
	},
		g.Entry("out of one loop device", 1),
		g.Entry("out of three loop devices", 3),
	)

	type degradedEntry struct {
		nodeConfigFunc             func() *scyllav1alpha1.NodeConfig
		preNodeConfigCreationFunc  func(ctx context.Context, nc *scyllav1alpha1.NodeConfig, nodeUnderTest *corev1.Node) func(context.Context)
		postNodeConfigCreationFunc func(ctx context.Context, nc *scyllav1alpha1.NodeConfig, nodeUnderTest *corev1.Node) func(context.Context)
	}

	g.DescribeTable("should propagate mount controller's degraded condition", func(specCtx g.SpecContext, e *degradedEntry) {
		ctx, cancel := context.WithTimeout(specCtx, testTimeout)
		defer cancel()

		nc := e.nodeConfigFunc()

		if e.preNodeConfigCreationFunc != nil {
			cleanupFunc := e.preNodeConfigCreationFunc(ctx, nc, nodeUnderTest)
			defer cleanupFunc(specCtx)
		}

		rc := framework.NewRestoringCleaner(
			ctx,
			f.AdminClientConfig(),
			f.KubeAdminClient(),
			f.DynamicAdminClient(),
			nodeConfigResourceInfo,
			nc.Namespace,
			nc.Name,
			framework.RestoreStrategyRecreate,
		)
		f.AddCleaners(rc)
		rc.DeleteObject(ctx, true)

		g.By("Creating NodeConfig")
		nc, err := f.ScyllaAdminClient().ScyllaV1alpha1().NodeConfigs().Create(ctx, nc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		if e.postNodeConfigCreationFunc != nil {
			cleanupFunc := e.postNodeConfigCreationFunc(ctx, nc, nodeUnderTest)
			defer cleanupFunc(specCtx)
		}

		framework.By("Waiting for NodeConfig to be in a degraded state")
		ctx1, ctx1Cancel := context.WithTimeout(ctx, nodeConfigRolloutTimeout)
		defer ctx1Cancel()
		nc, err = controllerhelpers.WaitForNodeConfigState(
			ctx1,
			f.ScyllaAdminClient().ScyllaV1alpha1().NodeConfigs(),
			nc.Name,
			controllerhelpers.WaitForStateOptions{TolerateDelete: false},
			isNodeConfigMountControllerNodeSetupDegradedFunc(nodeUnderTest),
		)
		o.Expect(err).NotTo(o.HaveOccurred())

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
		)
		o.Expect(err).NotTo(o.HaveOccurred())
	},
		g.Entry("for mount unit configured with a nonexistent device", &degradedEntry{
			nodeConfigFunc: func() *scyllav1alpha1.NodeConfig {
				nc := ncTemplate.DeepCopy()

				nc.Spec.LocalDiskSetup = &scyllav1alpha1.LocalDiskSetup{
					Mounts: []scyllav1alpha1.MountConfiguration{
						{
							Device:             fmt.Sprintf("/dev/%s", f.Namespace()),
							MountPoint:         fmt.Sprintf("/var/lib/%s", f.Namespace()),
							FSType:             string(scyllav1alpha1.XFSFilesystem),
							UnsupportedOptions: []string{"prjquota"},
						},
					},
				}

				return nc
			},
			preNodeConfigCreationFunc:  nil,
			postNodeConfigCreationFunc: nil,
		}),
		g.Entry("for mount unit configured with a device with an invalid filesystem type", &degradedEntry{
			nodeConfigFunc: func() *scyllav1alpha1.NodeConfig {
				nc := ncTemplate.DeepCopy()

				nc.Spec.LocalDiskSetup = &scyllav1alpha1.LocalDiskSetup{
					LoopDevices: []scyllav1alpha1.LoopDeviceConfiguration{
						{
							Name:      "disk",
							ImagePath: fmt.Sprintf("/var/lib/%s.img", f.Namespace()),
							Size:      xfsVolumeSize,
						},
					},
					Mounts: []scyllav1alpha1.MountConfiguration{
						{
							Device:             "/dev/loops/disk",
							MountPoint:         fmt.Sprintf("/var/lib/%s/mount", f.Namespace()),
							FSType:             string(scyllav1alpha1.XFSFilesystem),
							UnsupportedOptions: []string{"prjquota"},
						},
					},
				}

				return nc
			},
			preNodeConfigCreationFunc:  nil,
			postNodeConfigCreationFunc: nil,
		}),
		g.Entry("for mount unit configured to overwrite an existing mount unit", &degradedEntry{
			nodeConfigFunc: func() *scyllav1alpha1.NodeConfig {
				nc := ncTemplate.DeepCopy()

				nc.Spec.LocalDiskSetup = &scyllav1alpha1.LocalDiskSetup{
					LoopDevices: []scyllav1alpha1.LoopDeviceConfiguration{
						{
							Name:      "disk",
							ImagePath: fmt.Sprintf("/var/lib/%s.img", f.Namespace()),
							Size:      xfsVolumeSize,
						},
					},
					Filesystems: []scyllav1alpha1.FilesystemConfiguration{
						{
							Device: "/dev/loops/disk",
							Type:   scyllav1alpha1.XFSFilesystem,
						},
					},
					Mounts: []scyllav1alpha1.MountConfiguration{
						{
							Device:             "/dev/loops/disk",
							MountPoint:         fmt.Sprintf("/var/lib/%s", f.Namespace()),
							FSType:             string(scyllav1alpha1.XFSFilesystem),
							UnsupportedOptions: []string{"prjquota"},
						},
					},
				}

				return nc
			},
			preNodeConfigCreationFunc: func(ctx context.Context, nc *scyllav1alpha1.NodeConfig, nodeUnderTest *corev1.Node) func(context.Context) {
				hostMountPath := fmt.Sprintf("/host/var/lib/%s", f.Namespace())

				framework.By("Creating a client Pod")
				clientPod := newClientPod(nc, nodeUnderTest)
				clientPod.Spec.NodeName = nodeUnderTest.GetName()

				clientPod, err := f.KubeClient().CoreV1().Pods(f.Namespace()).Create(ctx, clientPod, metav1.CreateOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				waitCtx1, waitCtx1Cancel := utils.ContextForPodStartup(ctx)
				defer waitCtx1Cancel()
				clientPod, err = controllerhelpers.WaitForPodState(waitCtx1, f.KubeClient().CoreV1().Pods(clientPod.Namespace), clientPod.Name, controllerhelpers.WaitForStateOptions{}, utils.PodIsRunning)
				o.Expect(err).NotTo(o.HaveOccurred())

				framework.By("Creating a temp directory on host")
				stdout, stderr, err := utils.ExecuteInPod(ctx, f.ClientConfig(), f.KubeClient().CoreV1(), clientPod, "", "mktemp", "--tmpdir=/host/tmp/", "--directory")
				o.Expect(err).NotTo(o.HaveOccurred(), stdout, stderr)

				hostTMPDir := strings.TrimSpace(stdout)

				framework.By("Creating the target mount point on host")
				stdout, stderr, err = utils.ExecuteInPod(ctx, f.ClientConfig(), f.KubeClient().CoreV1(), clientPod, "", "mkdir", hostMountPath)
				o.Expect(err).NotTo(o.HaveOccurred(), stdout, stderr)

				framework.By("Bind mounting temp directory on host to target mount point")
				stdout, stderr, err = utils.ExecuteInPod(ctx, f.ClientConfig(), f.KubeClient().CoreV1(), clientPod, "", "mount", "--bind", hostTMPDir, hostMountPath)
				o.Expect(err).NotTo(o.HaveOccurred(), stdout, stderr)

				cleanupFunc := func(ctx context.Context) {
					framework.By("Unmounting bind mounted target")
					stdout, stderr, err = utils.ExecuteInPod(ctx, f.ClientConfig(), f.KubeClient().CoreV1(), clientPod, "", "umount", hostMountPath)
					o.Expect(err).NotTo(o.HaveOccurred(), stdout, stderr)
				}

				return cleanupFunc
			},
			postNodeConfigCreationFunc: nil,
		}),
		g.Entry("for mount unit configured with a corrupted filesystem", &degradedEntry{
			nodeConfigFunc: func() *scyllav1alpha1.NodeConfig {
				nc := ncTemplate.DeepCopy()
				nc.Spec.LocalDiskSetup = &scyllav1alpha1.LocalDiskSetup{
					LoopDevices: []scyllav1alpha1.LoopDeviceConfiguration{
						{
							Name:      "disk",
							ImagePath: fmt.Sprintf("/var/lib/%s.img", f.Namespace()),
							Size:      xfsVolumeSize,
						},
					},
					Filesystems: []scyllav1alpha1.FilesystemConfiguration{
						{
							Device: "/dev/loops/disk",
							Type:   scyllav1alpha1.XFSFilesystem,
						},
					},
				}
				return nc
			},
			preNodeConfigCreationFunc: nil,
			postNodeConfigCreationFunc: func(ctx context.Context, nc *scyllav1alpha1.NodeConfig, nodeUnderTest *corev1.Node) func(context.Context) {
				hostDevicePath := "/host/dev/loops/disk"

				framework.By("Creating a client Pod")
				clientPod := newClientPod(nc, nodeUnderTest)
				clientPod.Spec.NodeName = nodeUnderTest.GetName()

				clientPod, err := f.KubeClient().CoreV1().Pods(f.Namespace()).Create(ctx, clientPod, metav1.CreateOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				framework.By("Waiting for client Pod to be in a running state")
				waitCtx1, waitCtx1Cancel := utils.ContextForPodStartup(ctx)
				defer waitCtx1Cancel()
				clientPod, err = controllerhelpers.WaitForPodState(waitCtx1, f.KubeClient().CoreV1().Pods(clientPod.Namespace), clientPod.Name, controllerhelpers.WaitForStateOptions{}, utils.PodIsRunning)
				o.Expect(err).NotTo(o.HaveOccurred())

				framework.By("Waiting for NodeConfig to roll out")
				ctx1, ctx1Cancel := context.WithTimeout(ctx, nodeConfigRolloutTimeout)
				defer ctx1Cancel()
				nc, err = controllerhelpers.WaitForNodeConfigState(
					ctx1,
					f.ScyllaAdminClient().ScyllaV1alpha1().NodeConfigs(),
					nc.Name,
					controllerhelpers.WaitForStateOptions{TolerateDelete: false},
					utils.IsNodeConfigRolledOut,
				)
				o.Expect(err).NotTo(o.HaveOccurred())

				framework.By("Resolving the host device path symbolic link")
				devicePathInContainer, err := resolveHostSymlinkToContainerPath(ctx, f.ClientConfig(), f.KubeClient().CoreV1(), clientPod, hostDevicePath)
				o.Expect(err).NotTo(o.HaveOccurred())

				framework.By("Verifying XFS filesystem integrity")
				stdout, stderr, err := utils.ExecuteInPod(ctx, f.ClientConfig(), f.KubeClient().CoreV1(), clientPod, "", "xfs_repair", "-o", "force_geometry", "-f", "-n", devicePathInContainer)
				o.Expect(err).NotTo(o.HaveOccurred(), stdout, stderr)

				framework.By("Corrupting XFS filesystem")
				stdout, stderr, err = utils.ExecuteInPod(ctx, f.ClientConfig(), f.KubeClient().CoreV1(), clientPod, "", "xfs_db", "-x", "-c", "blockget", "-c", "blocktrash -s 12345678 -n 1000", devicePathInContainer)
				o.Expect(err).NotTo(o.HaveOccurred(), stdout, stderr)

				framework.By("Verifying that XFS filesystem is corrupted")
				stdout, stderr, err = utils.ExecuteInPod(ctx, f.ClientConfig(), f.KubeClient().CoreV1(), clientPod, "", "xfs_repair", "-o", "force_geometry", "-f", "-n", devicePathInContainer)
				o.Expect(err).To(o.HaveOccurred())

				framework.By("Patching NodeConfig's mount configuration with a mount over a corrupted filesystem")
				ncCopy := nc.DeepCopy()
				ncCopy.Spec.LocalDiskSetup.Mounts = []scyllav1alpha1.MountConfiguration{
					{
						Device:             "/dev/loops/disk",
						MountPoint:         fmt.Sprintf("/var/lib/%s", f.Namespace()),
						FSType:             string(scyllav1alpha1.XFSFilesystem),
						UnsupportedOptions: []string{"prjquota"},
					},
				}
				patch, err := helpers.CreateTwoWayMergePatch(nc, ncCopy)
				o.Expect(err).NotTo(o.HaveOccurred())

				_, err = f.ScyllaAdminClient().ScyllaV1alpha1().NodeConfigs().Patch(ctx, nc.Name, types.MergePatchType, patch, metav1.PatchOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				// Nothing to clean up.
				return func(context.Context) {}
			},
		}),
	)

	g.It("should successfully start a failed mount unit with a corrupted filesystem when it's overwritten with a clean one", func(ctx context.Context) {
		devicePath := fmt.Sprintf("/dev/loops/%s", f.Namespace())
		hostDevicePath := path.Join("/host", devicePath)

		nc := ncTemplate.DeepCopy()
		nc.Spec.LocalDiskSetup = &scyllav1alpha1.LocalDiskSetup{
			LoopDevices: []scyllav1alpha1.LoopDeviceConfiguration{
				{
					Name:      f.Namespace(),
					ImagePath: fmt.Sprintf("/var/lib/%s.img", f.Namespace()),
					Size:      xfsVolumeSize,
				},
			},
			Filesystems: []scyllav1alpha1.FilesystemConfiguration{
				{
					Device: devicePath,
					Type:   scyllav1alpha1.XFSFilesystem,
				},
			},
		}

		rc := framework.NewRestoringCleaner(
			ctx,
			f.AdminClientConfig(),
			f.KubeAdminClient(),
			f.DynamicAdminClient(),
			nodeConfigResourceInfo,
			nc.Namespace,
			nc.Name,
			framework.RestoreStrategyRecreate,
		)
		f.AddCleaners(rc)
		rc.DeleteObject(ctx, true)

		g.By("Creating NodeConfig")
		nc, err := f.ScyllaAdminClient().ScyllaV1alpha1().NodeConfigs().Create(ctx, nc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Creating a client Pod")
		clientPod := newClientPod(nc, nodeUnderTest)

		clientPod, err = f.KubeClient().CoreV1().Pods(f.Namespace()).Create(ctx, clientPod, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for client Pod to be in a running state")
		waitCtx1, waitCtx1Cancel := utils.ContextForPodStartup(ctx)
		defer waitCtx1Cancel()
		clientPod, err = controllerhelpers.WaitForPodState(waitCtx1, f.KubeClient().CoreV1().Pods(clientPod.Namespace), clientPod.Name, controllerhelpers.WaitForStateOptions{}, utils.PodIsRunning)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for NodeConfig to roll out")
		ctx1, ctx1Cancel := context.WithTimeout(ctx, nodeConfigRolloutTimeout)
		defer ctx1Cancel()
		nc, err = controllerhelpers.WaitForNodeConfigState(
			ctx1,
			f.ScyllaAdminClient().ScyllaV1alpha1().NodeConfigs(),
			nc.Name,
			controllerhelpers.WaitForStateOptions{TolerateDelete: false},
			utils.IsNodeConfigRolledOut,
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Resolving the host device path symbolic link")
		devicePathInContainer, err := resolveHostSymlinkToContainerPath(ctx, f.ClientConfig(), f.KubeClient().CoreV1(), clientPod, hostDevicePath)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Verifying the filesystem's integrity")
		stdout, stderr, err := utils.ExecuteInPod(ctx, f.ClientConfig(), f.KubeClient().CoreV1(), clientPod, "", "xfs_repair", "-o", "force_geometry", "-f", "-n", devicePathInContainer)
		o.Expect(err).NotTo(o.HaveOccurred(), stdout, stderr)

		framework.By("Getting the filesystem's block size")
		stdout, stderr, err = utils.ExecuteInPod(ctx, f.ClientConfig(), f.KubeClient().CoreV1(), clientPod, "", "stat", "--file-system", "--format=%s", devicePathInContainer)
		o.Expect(err).NotTo(o.HaveOccurred(), stdout, stderr)
		blockSize := strings.TrimSpace(stdout)

		framework.By("Corrupting the filesystem")
		stdout, stderr, err = utils.ExecuteInPod(ctx, f.ClientConfig(), f.KubeClient().CoreV1(), clientPod, "", "xfs_db", "-x", "-c", "blockget", "-c", "blocktrash -s 12345678 -n 1000", devicePathInContainer)
		o.Expect(err).NotTo(o.HaveOccurred(), stdout, stderr)

		framework.By("Verifying that the filesystem is corrupted")
		stdout, stderr, err = utils.ExecuteInPod(ctx, f.ClientConfig(), f.KubeClient().CoreV1(), clientPod, "", "xfs_repair", "-o", "force_geometry", "-f", "-n", devicePathInContainer)
		o.Expect(err).To(o.HaveOccurred())

		framework.By("Patching NodeConfig's mount configuration with a mount over a corrupted filesystem")
		ncCopy := nc.DeepCopy()
		ncCopy.Spec.LocalDiskSetup.Mounts = []scyllav1alpha1.MountConfiguration{
			{
				Device:             devicePath,
				MountPoint:         fmt.Sprintf("/var/lib/%s", f.Namespace()),
				FSType:             string(scyllav1alpha1.XFSFilesystem),
				UnsupportedOptions: []string{"prjquota"},
			},
		}
		patch, err := helpers.CreateTwoWayMergePatch(nc, ncCopy)
		o.Expect(err).NotTo(o.HaveOccurred())

		_, err = f.ScyllaAdminClient().ScyllaV1alpha1().NodeConfigs().Patch(ctx, nc.Name, types.MergePatchType, patch, metav1.PatchOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for NodeConfig to be in a degraded state")
		ctx2, ctx2Cancel := context.WithTimeout(ctx, nodeConfigRolloutTimeout)
		defer ctx2Cancel()
		nc, err = controllerhelpers.WaitForNodeConfigState(
			ctx2,
			f.ScyllaAdminClient().ScyllaV1alpha1().NodeConfigs(),
			nc.Name,
			controllerhelpers.WaitForStateOptions{TolerateDelete: false},
			isNodeConfigMountControllerNodeSetupDegradedFunc(nodeUnderTest),
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		// We're running this command until it eventually succeeds because sometimes it can fail with:
		//	mkfs.xfs: cannot open /host/dev/loop1: Device or resource busy (or similar)
		// It happens only on OpenShift and most likely is a race between the device being released by _some_ part of the
		// system and mkfs trying to open it. In virtually all observed cases, one retry after a short delay was enough to
		// succeed.
		framework.By("Overwriting the corrupted filesystem")
		o.Eventually(func(eo o.Gomega) {
			stdout, stderr, err = utils.ExecuteInPod(ctx, f.ClientConfig(), f.KubeClient().CoreV1(), clientPod, "", "mkfs", "-t", string(scyllav1alpha1.XFSFilesystem), "-b", fmt.Sprintf("size=%s", blockSize), "-K", "-f", devicePathInContainer)
			eo.Expect(err).NotTo(o.HaveOccurred(), stdout, stderr)
		}).WithPolling(time.Second * 5).WithTimeout(1 * time.Minute).Should(o.Succeed())

		framework.By("Waiting for NodeConfig to roll out")
		ctx3, ctx3Cancel := context.WithTimeout(ctx, nodeConfigRolloutTimeout)
		defer ctx3Cancel()
		nc, err = controllerhelpers.WaitForNodeConfigState(
			ctx3,
			f.ScyllaAdminClient().ScyllaV1alpha1().NodeConfigs(),
			nc.Name,
			controllerhelpers.WaitForStateOptions{TolerateDelete: false},
			utils.IsNodeConfigRolledOut,
		)
		o.Expect(err).NotTo(o.HaveOccurred())

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

		framework.By("Waiting for NodeConfig to deploy")
		ctx4, ctx4Cancel := context.WithTimeout(ctx, nodeConfigRolloutTimeout)
		defer ctx4Cancel()
		nc, err = controllerhelpers.WaitForNodeConfigState(
			ctx4,
			f.ScyllaAdminClient().ScyllaV1alpha1().NodeConfigs(),
			nc.Name,
			controllerhelpers.WaitForStateOptions{TolerateDelete: false},
			utils.IsNodeConfigRolledOut,
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyNodeConfig(ctx, f.KubeAdminClient(), nc)
	}, g.NodeTimeout(testTimeout))
})

func newClientPod(nc *scyllav1alpha1.NodeConfig, nodeUnderTest *corev1.Node) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "client",
		},
		Spec: corev1.PodSpec{
			// We want to schedule the Pod specifically on the node that is under test as we will perform operations
			// on the host filesystem through that Pod.
			NodeName: nodeUnderTest.Name,
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
}

// resolveHostSymlinkToContainerPath resolves a symlink on the host to its actual path in the container.
//
// It can be useful to get container paths to loop devices. Each loop device created by NodeConfig has a symlink in
// `/dev/loops/<name>` on the host. We need to resolve the symlink to get the actual device path (e.g.,
// `/dev/loops/disk -> /dev/loop1`). To access that path from the Pod, we need to prepend the resolved link with `/host`
// (as that's where the host root filesystem is mounted in the Pod).
func resolveHostSymlinkToContainerPath(
	ctx context.Context,
	config *rest.Config,
	client corev1client.CoreV1Interface,
	pod *corev1.Pod,
	hostPath string,
) (string, error) {
	stdout, stderr, err := utils.ExecuteInPod(ctx, config, client, pod, "", "readlink", "-f", hostPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve symlink for %s: %v\nstdout: %s\nstderr: %s", hostPath, err, stdout, stderr)
	}
	return path.Join("/host", strings.TrimSpace(stdout)), nil
}

func isNodeConfigMountControllerNodeSetupDegradedFunc(node *corev1.Node) func(nc *scyllav1alpha1.NodeConfig) (bool, error) {
	return func(nc *scyllav1alpha1.NodeConfig) (bool, error) {
		statusConditions := nc.Status.Conditions.ToMetaV1Conditions()

		if !helpers.IsStatusConditionPresentAndTrue(statusConditions, scyllav1alpha1.DegradedCondition, nc.Generation) {
			return false, nil
		}

		nodeDegradedConditionType := fmt.Sprintf(internalapi.NodeDegradedConditionFormat, node.GetName())
		if !helpers.IsStatusConditionPresentAndTrue(statusConditions, nodeDegradedConditionType, nc.Generation) {
			return false, nil
		}

		nodeSetupDegradedConditionType := fmt.Sprintf(internalapi.NodeSetupDegradedConditionFormat, node.GetName())
		if !helpers.IsStatusConditionPresentAndTrue(statusConditions, nodeSetupDegradedConditionType, nc.Generation) {
			return false, nil
		}

		nodeMountControllerConditionType := fmt.Sprintf("MountControllerNodeSetup%sDegraded", node.GetName())
		if !helpers.IsStatusConditionPresentAndTrue(statusConditions, nodeMountControllerConditionType, nc.Generation) {
			return false, nil
		}

		return true, nil
	}
}
