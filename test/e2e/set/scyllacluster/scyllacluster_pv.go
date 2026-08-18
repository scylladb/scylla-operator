// Copyright (C) 2021 ScyllaDB

package scyllacluster

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configassests "github.com/scylladb/scylla-operator/assets/config"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	"github.com/scylladb/scylla-operator/test/e2e/utils/verification"
	scyllaclusterverification "github.com/scylladb/scylla-operator/test/e2e/utils/verification/scyllacluster"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	apimachineryutilwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	watchtools "k8s.io/client-go/tools/watch"
	"k8s.io/client-go/util/retry"
	"k8s.io/component-helpers/storage/volume"
)

var _ = g.Describe("ScyllaCluster Orphaned PV controller", framework.SuiteParallel, framework.SuiteParallelOpenShift, framework.SuiteKindFast, framework.SuiteKindClusterTopology, func() {
	var f *framework.Framework

	g.BeforeEach(func(ctx context.Context) {
		f = framework.NewFramework(ctx, "scyllacluster")
	})

	const cloneLabelKey = "e2e.operator.scylladb.com/orphaned-pv-test"

	g.JustBeforeEach(func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		testStorageClass := &storagev1.StorageClass{
			ObjectMeta: metav1.ObjectMeta{
				Name:   f.Namespace(),
				Labels: f.CommonLabels(),
			},
			Provisioner:       volume.NotSupportedProvisioner,
			ReclaimPolicy:     new(corev1.PersistentVolumeReclaimDelete),
			VolumeBindingMode: new(storagev1.VolumeBindingWaitForFirstConsumer),
		}

		_, err := f.KubeAdminClient().StorageV1().StorageClasses().Create(ctx, testStorageClass, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.JustAfterEach(func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		errs := make([]error, 0, 2)
		errs = append(errs, f.KubeAdminClient().StorageV1().StorageClasses().Delete(ctx, f.Namespace(), metav1.DeleteOptions{}))
		errs = append(errs, f.KubeAdminClient().CoreV1().PersistentVolumes().DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{
			LabelSelector: labels.SelectorFromSet(map[string]string{
				cloneLabelKey: f.Namespace(),
			}).String(),
		}))

		o.Expect(apimachineryutilerrors.NewAggregate(errs)).NotTo(o.HaveOccurred())
	})

	g.It("should replace a node with orphaned PV", func(ctx g.SpecContext) {
		testStorageClassName := f.Namespace()

		sc := f.GetDefaultScyllaCluster()
		sc.Spec.AutomaticOrphanedNodeCleanup = true
		sc.Spec.Datacenter.Racks[0].Members = 3
		sc.Spec.Datacenter.Racks[0].Storage.StorageClassName = new(testStorageClassName)
		if sc.Spec.Datacenter.Racks[0].Placement == nil {
			sc.Spec.Datacenter.Racks[0].Placement = &scyllav1.PlacementSpec{}
		}

		defaultScyllaClusterPlacement := sc.Spec.Datacenter.Racks[0].Placement.DeepCopy()

		if sc.Spec.Datacenter.Racks[0].Placement.PodAffinity == nil {
			sc.Spec.Datacenter.Racks[0].Placement.PodAffinity = &corev1.PodAffinity{}
		}
		// Every ScyllaDB Pod has to be scheduled onto the Node serving the consumer Pod of its own PVC clone, because
		// that's the only Node holding its data. The clone PVs carry no NodeAffinity - it's immutable once set, and the
		// test needs to set it later to simulate the orphan - so this PodAffinity is what ties a Pod to its data.
		//
		// The term has to resolve per ordinal, while rack placement is shared by the whole rack. MatchLabelKeys provides
		// that: the scheduler looks the listed keys up in the incoming Pod's own labels and merges them into the selector
		// as `key in (value)`. ScyllaDB Pods get apps.kubernetes.io/pod-index from the StatefulSet controller, and the
		// consumer Pods below are labeled with the ordinal of the PVC they back, so each ScyllaDB Pod only matches its
		// own consumer Pod.
		sc.Spec.Datacenter.Racks[0].Placement.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution = append(
			sc.Spec.Datacenter.Racks[0].Placement.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution,
			corev1.PodAffinityTerm{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						cloneLabelKey: f.Namespace(),
					},
				},
				MatchLabelKeys: []string{appsv1.PodIndexLabel},
				TopologyKey:    corev1.LabelHostname,
			},
		)

		// This test to trigger the orphaned PV cleanup is updating NodeAffinity of PV used by ScyllaCluster to not exiting Node.
		// The problem is, that local volume storage provisioners which we use in CI, may set this field, which is immutable.
		// To overcome this, the ScyllaCluster is using custom StorageClass not backed by any provisioner running in the cluster.
		// ScyllaCluster is going to request a PVC from that StorageClass, and the test is going to request a clone of the original PVC
		// from the default StorageClass to get any storage. Then the bound PV is rebounded to the original PVC
		// but with empty NodeAffinity. This allows the test to trigger the orphaned PV cleanup logic.
		var wg sync.WaitGroup
		defer wg.Wait()

		provisionerCtx, provisionerCancel := context.WithCancel(context.Background())
		defer provisionerCancel()

		wg.Go(func() {
			defer g.GinkgoRecover()

			lw := &cache.ListWatch{
				ListFunc: helpers.UncachedListFunc(func(options metav1.ListOptions) (runtime.Object, error) {
					return f.KubeClient().CoreV1().PersistentVolumeClaims(f.Namespace()).List(ctx, options)
				}),
				WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
					return f.KubeClient().CoreV1().PersistentVolumeClaims(f.Namespace()).Watch(ctx, options)
				},
			}

			_, err := watchtools.UntilWithSync(provisionerCtx, lw, &corev1.PersistentVolumeClaim{}, nil, func(e watch.Event) (bool, error) {
				switch t := e.Type; t {
				case watch.Added:
					pvc := e.Object.(*corev1.PersistentVolumeClaim)
					if pvc.Spec.StorageClassName == nil || *pvc.Spec.StorageClassName != testStorageClassName {
						return false, nil
					}

					pvcClone := &corev1.PersistentVolumeClaim{
						ObjectMeta: metav1.ObjectMeta{
							Name: fmt.Sprintf("clone-%s", pvc.Name),
						},
						Spec:   *pvc.Spec.DeepCopy(),
						Status: *pvc.Status.DeepCopy(),
					}
					pvcClone.Spec.VolumeName = ""
					pvcClone.Spec.StorageClassName = new(framework.TestContext.ScyllaClusterOptions.StorageClassName)

					framework.Infof("Creating clone PVC for %q", pvc.Name)
					pvcClone, _, err := resourceapply.ApplyPersistentVolumeClaimWithControl(
						ctx,
						resourceapply.ApplyControlFuncs[*corev1.PersistentVolumeClaim]{
							GetCachedFunc: func(name string) (*corev1.PersistentVolumeClaim, error) {
								return f.KubeClient().CoreV1().PersistentVolumeClaims(f.Namespace()).Get(ctx, name, metav1.GetOptions{})
							},
							CreateFunc: f.KubeClient().CoreV1().PersistentVolumeClaims(f.Namespace()).Create,
							UpdateFunc: f.KubeClient().CoreV1().PersistentVolumeClaims(f.Namespace()).Update,
							DeleteFunc: f.KubeClient().CoreV1().PersistentVolumeClaims(f.Namespace()).Delete,
						},
						&record.FakeRecorder{},
						pvcClone,
						resourceapply.ApplyOptions{
							AllowMissingControllerRef: true,
						},
					)
					o.Expect(err).NotTo(o.HaveOccurred())

					// The consumer Pod is labelled with the ordinal of the ScyllaDB Pod whose PVC it clones, so that the
					// rack's PodAffinity term can pair the two through MatchLabelKeys. The ordinal comes from the PVC
					// name, which the StatefulSet derives from its Pod's name.
					ordinal, err := naming.IndexFromName(pvc.Name)
					o.Expect(err).NotTo(o.HaveOccurred())

					framework.Infof("Creating PVC clone consumer Pod")
					consumerPodName := fmt.Sprintf("consumer-%s", pvcClone.Name)
					consumerPod := &corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name: consumerPodName,
							Labels: map[string]string{
								cloneLabelKey:        f.Namespace(),
								appsv1.PodIndexLabel: fmt.Sprintf("%d", ordinal),
							},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "consumer",
									Image: configassests.Project.Operator.BashToolsImage,
									Command: []string{
										"/bin/sh",
										"-c",
										"sleep 3600",
									},
								},
							},
							Volumes: []corev1.Volume{
								{
									Name: "data",
									VolumeSource: corev1.VolumeSource{
										PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
											ClaimName: pvcClone.Name,
										},
									},
								},
							},
							TerminationGracePeriodSeconds: new(int64(1)),
							RestartPolicy:                 corev1.RestartPolicyNever,
							Tolerations:                   defaultScyllaClusterPlacement.Tolerations,
							Affinity: &corev1.Affinity{
								NodeAffinity:    defaultScyllaClusterPlacement.NodeAffinity,
								PodAffinity:     defaultScyllaClusterPlacement.PodAffinity,
								PodAntiAffinity: defaultScyllaClusterPlacement.PodAntiAffinity,
							},
						},
					}

					_, _, err = resourceapply.ApplyPodWithControl(
						ctx,
						resourceapply.ApplyControlFuncs[*corev1.Pod]{
							GetCachedFunc: func(name string) (*corev1.Pod, error) {
								return f.KubeClient().CoreV1().Pods(f.Namespace()).Get(ctx, name, metav1.GetOptions{})
							},
							CreateFunc: f.KubeClient().CoreV1().Pods(f.Namespace()).Create,
							UpdateFunc: f.KubeClient().CoreV1().Pods(f.Namespace()).Update,
							DeleteFunc: f.KubeClient().CoreV1().Pods(f.Namespace()).Delete,
						},
						&record.FakeRecorder{},
						consumerPod,
						resourceapply.ApplyOptions{
							AllowMissingControllerRef: true,
						},
					)
					o.Expect(err).NotTo(o.HaveOccurred())
				case watch.Modified:
					pvc := e.Object.(*corev1.PersistentVolumeClaim)
					if pvc.Spec.StorageClassName == nil {
						return false, nil
					}
					if pvc.DeletionTimestamp != nil {
						return false, nil
					}

					if *pvc.Spec.StorageClassName != testStorageClassName && len(pvc.Spec.VolumeName) != 0 {
						pv, err := f.KubeAdminClient().CoreV1().PersistentVolumes().Get(ctx, pvc.Spec.VolumeName, metav1.GetOptions{})
						o.Expect(err).NotTo(o.HaveOccurred())

						realPVCName := strings.TrimPrefix(pvc.Name, "clone-")

						pvClone := &corev1.PersistentVolume{
							ObjectMeta: metav1.ObjectMeta{
								Name:   fmt.Sprintf("clone-%s", pv.Name),
								Labels: f.CommonLabels(),
							},
							Spec: *pv.Spec.DeepCopy(),
						}
						pvClone.Labels[cloneLabelKey] = f.Namespace()
						pvClone.Spec.StorageClassName = testStorageClassName
						pvClone.Spec.NodeAffinity = nil
						// Reserve the clone PV for the PVC that it's meant for. A PVC's spec.volumeName is only a request,
						// while the PV's spec.claimRef is what reserves it, so an unclaimed PV can be bound by the PV
						// controller to any other unbound PVC of the same StorageClass.
						// UID is deliberately left unset. This test replaces the PVC, and a claimRef holding only a
						// namespace and a name keeps matching it across the replacement.
						pvClone.Spec.ClaimRef = &corev1.ObjectReference{
							Namespace: f.Namespace(),
							Name:      realPVCName,
						}
						// The clone PV is a fabricated object with no lifecycle of its own, and it shares the backing volume
						// with the clone PVC's PV. Reclaiming it would make the provisioner delete storage which is still in
						// use, so it's retained and deleted explicitly on cleanup.
						pvClone.Spec.PersistentVolumeReclaimPolicy = corev1.PersistentVolumeReclaimRetain
						if pvClone.Spec.PersistentVolumeSource.Local != nil {
							pvClone.Spec.PersistentVolumeSource.HostPath = &corev1.HostPathVolumeSource{
								Path: pvClone.Spec.PersistentVolumeSource.Local.Path,
							}
							pvClone.Spec.PersistentVolumeSource.Local = nil
						}

						framework.Infof("Creating clone PV for %q", pv.Name)
						_, _, err = resourceapply.ApplyPersistentVolumeWithControl(
							ctx,
							resourceapply.ApplyControlFuncs[*corev1.PersistentVolume]{
								GetCachedFunc: func(name string) (*corev1.PersistentVolume, error) {
									return f.KubeAdminClient().CoreV1().PersistentVolumes().Get(ctx, name, metav1.GetOptions{})
								},
								CreateFunc: f.KubeAdminClient().CoreV1().PersistentVolumes().Create,
								UpdateFunc: f.KubeAdminClient().CoreV1().PersistentVolumes().Update,
								DeleteFunc: f.KubeAdminClient().CoreV1().PersistentVolumes().Delete,
							},
							&record.FakeRecorder{},
							pvClone,
							resourceapply.ApplyOptions{
								AllowMissingControllerRef: true,
							},
						)
						o.Expect(err).NotTo(o.HaveOccurred())

						err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
							realPvc, err := f.KubeClient().CoreV1().PersistentVolumeClaims(f.Namespace()).Get(ctx, realPVCName, metav1.GetOptions{})
							if err != nil {
								return err
							}

							if len(realPvc.Spec.VolumeName) != 0 {
								return nil
							}

							realPvcCopy := realPvc.DeepCopy()
							realPvcCopy.Spec.VolumeName = pvClone.Name

							framework.Infof("Binding ScyllaCluster PVC %q with clone PV %q", realPvc.Name, pvClone.Name)
							_, err = f.KubeClient().CoreV1().PersistentVolumeClaims(realPvc.Namespace).Update(ctx, realPvcCopy, metav1.UpdateOptions{})
							return err
						})
						o.Expect(err).NotTo(o.HaveOccurred())
					}
				case watch.Deleted:
					// If original PVC is deleted, delete the clone too.
					pvc := e.Object.(*corev1.PersistentVolumeClaim)
					if strings.HasPrefix(pvc.Name, "clone-") {
						return false, nil
					}

					clonePVCName := fmt.Sprintf("clone-%s", pvc.Name)
					clonePVC, err := f.KubeClient().CoreV1().PersistentVolumeClaims(pvc.Namespace).Get(ctx, clonePVCName, metav1.GetOptions{})
					if apierrors.IsNotFound(err) {
						return false, nil
					}
					o.Expect(err).NotTo(o.HaveOccurred())

					framework.Infof("Deleting clone PVC %q", clonePVCName)
					err = f.KubeClient().CoreV1().PersistentVolumeClaims(f.Namespace()).Delete(ctx, clonePVCName, metav1.DeleteOptions{})
					o.Expect(err).To(framework.NotHaveOccurredExceptNotFound())

					consumerPodName := fmt.Sprintf("consumer-%s", clonePVCName)
					framework.Infof("Deleting Pod consumer of clone PVC %q", consumerPodName)

					err = f.KubeClient().CoreV1().Pods(f.Namespace()).Delete(ctx, consumerPodName, metav1.DeleteOptions{})
					o.Expect(err).To(framework.NotHaveOccurredExceptNotFound())

					err = framework.WaitForObjectDeletion(ctx, f.DynamicAdminClient(), corev1.SchemeGroupVersion.WithResource("persistentvolumeclaims"), f.Namespace(), clonePVC.Name, &clonePVC.UID)
					if err != nil {
						return false, fmt.Errorf("couldn't wait for clone PVC %q to be deleted: %v", naming.ManualRef(f.Namespace(), clonePVCName), err)
					}
				}
				return false, nil
			})
			if apimachineryutilwait.Interrupted(err) && errors.Is(provisionerCtx.Err(), context.Canceled) {
				return
			}
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		framework.By("Creating a ScyllaCluster")
		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to roll out (RV=%s)", sc.ResourceVersion)
		initialRolloutCtx, initialRolloutCtxCancel := utils.ContextForRollout(ctx, sc)
		defer initialRolloutCtxCancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(initialRolloutCtx, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scyllaclusterverification.Verify(ctx, f.KubeClient(), f.ScyllaClient(), sc)
		scyllaclusterverification.WaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)

		// Assert that every ScyllaDB Pod actually landed on the Node serving the consumer Pod of its own PVC clone.
		// The pairing relies on MatchLabelKeys, which is silently ignored when the corresponding feature gate is
		// disabled, and on apps.kubernetes.io/pod-index being stamped on StatefulSet Pods. Both would degrade the
		// PodAffinity term into one satisfied by any consumer Pod, without failing anything, so verify the placement
		// rather than trusting the term was honoured.
		framework.By("Verifying that ScyllaDB Pods are co-located with the consumer Pods of their PVC clones")
		verifyScyllaDBPodsColocatedWithConsumerPods(ctx, f, sc)

		hosts, _, err := utils.GetBroadcastRPCAddressesAndUUIDs(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(hosts).To(o.HaveLen(3))
		di := verification.InsertAndVerifyCQLData(ctx, hosts)
		defer di.Close()

		framework.By("Simulating a PV on node that's gone")
		stsName := naming.StatefulSetNameForRackForScyllaCluster(sc.Spec.Datacenter.Racks[0], sc)
		podName := fmt.Sprintf("%s-%d", stsName, sc.Spec.Datacenter.Racks[0].Members-1)
		pvcName := naming.PVCNameForPod(podName)

		pvc, err := f.KubeClient().CoreV1().PersistentVolumeClaims(f.Namespace()).Get(ctx, pvcName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(pvc.Spec.VolumeName).NotTo(o.BeEmpty())

		pv, err := f.KubeAdminClient().CoreV1().PersistentVolumes().Get(ctx, pvc.Spec.VolumeName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		pvCopy := pv.DeepCopy()
		pvCopy.Spec.NodeAffinity = &corev1.VolumeNodeAffinity{
			Required: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      corev1.LabelHostname,
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{"this-node-does-not-exist-42"},
							},
						},
					},
				},
			},
		}

		patchData, err := controllerhelpers.GenerateMergePatch(pv, pvCopy)
		o.Expect(err).NotTo(o.HaveOccurred())

		pv, err = f.KubeAdminClient().CoreV1().PersistentVolumes().Patch(ctx, pv.Name, types.MergePatchType, patchData, metav1.PatchOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(pv.Spec.NodeAffinity).NotTo(o.BeNil())

		// We are not listening to PV changes, so we will make a dummy edit on the ScyllaCluster.
		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(
			ctx,
			sc.Name,
			types.MergePatchType,
			[]byte(`{"metadata": {"annotations": {"foo": "bar"} } }`),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the PVC to be replaced")
		pvcReplacementCtx, pvcReplacementCtxCancel := utils.ContextForRollout(ctx, sc)
		defer pvcReplacementCtxCancel()
		pvc, err = controllerhelpers.WaitForPVCState(pvcReplacementCtx, f.KubeClient().CoreV1().PersistentVolumeClaims(pvc.Namespace), pvc.Name, controllerhelpers.WaitForStateOptions{TolerateDelete: true}, func(freshPVC *corev1.PersistentVolumeClaim) (bool, error) {
			return freshPVC.UID != pvc.UID, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to observe the degradation")
		degradationCtx, degradationCtxCancel := utils.ContextForRollout(ctx, sc)
		defer degradationCtxCancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(degradationCtx, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, func(sc *scyllav1.ScyllaCluster) (bool, error) {
			rolledOut, err := utils.IsScyllaClusterRolledOut(sc)
			return !rolledOut, err
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to roll out (RV=%s)", sc.ResourceVersion)
		postReplacementRolloutCtx, postReplacementRolloutCtxCancel := utils.ContextForRollout(ctx, sc)
		defer postReplacementRolloutCtxCancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(postReplacementRolloutCtx, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scyllaclusterverification.Verify(ctx, f.KubeClient(), f.ScyllaClient(), sc)
		scyllaclusterverification.WaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)

		hosts, err = utils.GetBroadcastRPCAddresses(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(hosts).To(o.HaveLen(3))
		verification.VerifyCQLData(ctx, di)

		// Stop fake provisioner
		provisionerCancel()
		wg.Wait()
	})
})

// verifyScyllaDBPodsColocatedWithConsumerPods asserts that every ScyllaDB Pod of the first rack runs on the same Node as
// the consumer Pod holding the clone PV bound to that Pod's own PVC.
func verifyScyllaDBPodsColocatedWithConsumerPods(ctx context.Context, f *framework.Framework, sc *scyllav1.ScyllaCluster) {
	g.GinkgoHelper()

	for i := int32(0); i < sc.Spec.Datacenter.Racks[0].Members; i++ {
		podName := naming.PodNameForScyllaCluster(sc.Spec.Datacenter.Racks[0], sc, int(i))
		pod, err := f.KubeClient().CoreV1().Pods(f.Namespace()).Get(ctx, podName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(pod.Spec.NodeName).NotTo(o.BeEmpty())

		consumerPodName := fmt.Sprintf("consumer-clone-%s", naming.PVCNameForPod(podName))
		consumerPod, err := f.KubeClient().CoreV1().Pods(f.Namespace()).Get(ctx, consumerPodName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(consumerPod.Spec.NodeName).NotTo(o.BeEmpty())

		o.Expect(pod.Spec.NodeName).To(
			o.Equal(consumerPod.Spec.NodeName),
			"ScyllaDB Pod %q runs on Node %q, but the consumer Pod %q holding its data runs on Node %q",
			podName, pod.Spec.NodeName, consumerPodName, consumerPod.Spec.NodeName,
		)
	}
}
