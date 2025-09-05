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
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	"github.com/scylladb/scylla-operator/test/e2e/utils/verification"
	scyllaclusterverification "github.com/scylladb/scylla-operator/test/e2e/utils/verification/scyllacluster"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	apimachineryutilwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	watchtools "k8s.io/client-go/tools/watch"
	"k8s.io/component-helpers/storage/volume"
)

var _ = g.Describe("ScyllaCluster Orphaned PV controller", func() {
	f := framework.NewFramework("scyllacluster")

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
			ReclaimPolicy:     pointer.Ptr(corev1.PersistentVolumeReclaimDelete),
			VolumeBindingMode: pointer.Ptr(storagev1.VolumeBindingWaitForFirstConsumer),
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

	g.It("should replace a node with orphaned PV", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		testStorageClassName := f.Namespace()

		sc := f.GetDefaultScyllaCluster()
		sc.Spec.AutomaticOrphanedNodeCleanup = true
		sc.Spec.Datacenter.Racks[0].Members = 3
		sc.Spec.Datacenter.Racks[0].Storage.StorageClassName = pointer.Ptr(testStorageClassName)
		if sc.Spec.Datacenter.Racks[0].Placement == nil {
			sc.Spec.Datacenter.Racks[0].Placement = &scyllav1.PlacementSpec{}
		}

		defaultScyllaClusterPlacement := sc.Spec.Datacenter.Racks[0].Placement.DeepCopy()

		if sc.Spec.Datacenter.Racks[0].Placement.PodAffinity == nil {
			sc.Spec.Datacenter.Racks[0].Placement.PodAffinity = &corev1.PodAffinity{}
		}
		sc.Spec.Datacenter.Racks[0].Placement.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution = append(
			sc.Spec.Datacenter.Racks[0].Placement.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution,
			corev1.PodAffinityTerm{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						cloneLabelKey: f.Namespace(),
					},
				},
				TopologyKey: corev1.LabelHostname,
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

		wg.Add(1)
		go func() {
			defer wg.Done()
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
					pvcClone.Spec.StorageClassName = nil

					framework.Infof("Creating clone PVC for %q", pvc.Name)
					pvcClone, err := f.KubeClient().CoreV1().PersistentVolumeClaims(f.Namespace()).Create(ctx, pvcClone, metav1.CreateOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())

					framework.Infof("Creating PVC clone consumer Pod")
					consumerPodName := fmt.Sprintf("consumer-%s", pvcClone.Name)
					consumerPod := &corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name: consumerPodName,
							Labels: map[string]string{
								cloneLabelKey: f.Namespace(),
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
							TerminationGracePeriodSeconds: pointer.Ptr(int64(1)),
							RestartPolicy:                 corev1.RestartPolicyNever,
							Tolerations:                   defaultScyllaClusterPlacement.Tolerations,
							Affinity: &corev1.Affinity{
								NodeAffinity:    defaultScyllaClusterPlacement.NodeAffinity,
								PodAffinity:     defaultScyllaClusterPlacement.PodAffinity,
								PodAntiAffinity: defaultScyllaClusterPlacement.PodAntiAffinity,
							},
						},
					}

					_, err = f.KubeClient().CoreV1().Pods(f.Namespace()).Create(ctx, consumerPod, metav1.CreateOptions{})
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
						realPvc, err := f.KubeClient().CoreV1().PersistentVolumeClaims(f.Namespace()).Get(ctx, strings.TrimPrefix(pvc.Name, "clone-"), metav1.GetOptions{})
						o.Expect(err).NotTo(o.HaveOccurred())

						if len(realPvc.Spec.VolumeName) != 0 {
							return false, nil
						}

						pv, err := f.KubeAdminClient().CoreV1().PersistentVolumes().Get(ctx, pvc.Spec.VolumeName, metav1.GetOptions{})
						o.Expect(err).NotTo(o.HaveOccurred())

						pvClone := &corev1.PersistentVolume{
							ObjectMeta: metav1.ObjectMeta{
								Name:   fmt.Sprintf("clone-%s", pv.Name),
								Labels: f.CommonLabels(),
							},
							Spec:   *pv.Spec.DeepCopy(),
							Status: *pv.Status.DeepCopy(),
						}
						pvClone.Labels[cloneLabelKey] = f.Namespace()
						pvClone.Spec.StorageClassName = testStorageClassName
						pvClone.Spec.NodeAffinity = nil
						pvClone.Spec.ClaimRef = nil
						if pvClone.Spec.PersistentVolumeSource.Local != nil {
							pvClone.Spec.PersistentVolumeSource.HostPath = &corev1.HostPathVolumeSource{
								Path: pvClone.Spec.PersistentVolumeSource.Local.Path,
							}
							pvClone.Spec.PersistentVolumeSource.Local = nil
						}

						framework.Infof("Creating clone PV for %q", pv.Name)
						_, err = f.KubeAdminClient().CoreV1().PersistentVolumes().Create(ctx, pvClone, metav1.CreateOptions{})
						o.Expect(err).NotTo(o.HaveOccurred())

						realPvcCopy := realPvc.DeepCopy()
						realPvcCopy.Spec.VolumeName = pvClone.Name

						framework.Infof("Binding ScyllaCluster PVC %q with clone PV %q", realPvc.Name, pvClone.Name)
						_, err = f.KubeClient().CoreV1().PersistentVolumeClaims(realPvc.Namespace).Update(ctx, realPvcCopy, metav1.UpdateOptions{})
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

					framework.Infof("Deleting clone PVC %q", clonePVCName)
					err = f.KubeClient().CoreV1().PersistentVolumeClaims(f.Namespace()).Delete(ctx, clonePVCName, metav1.DeleteOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())

					consumerPodName := fmt.Sprintf("consumer-%s", clonePVCName)
					framework.Infof("Deleting Pod consumer of clone PVC %q", consumerPodName)

					err = f.KubeClient().CoreV1().Pods(f.Namespace()).Delete(ctx, consumerPodName, metav1.DeleteOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())

					err = framework.WaitForObjectDeletion(ctx, f.DynamicAdminClient(), corev1.SchemeGroupVersion.WithResource("persistentvolumeclaims"), f.Namespace(), clonePVC.Name, &clonePVC.UID)
				}
				return false, nil
			})
			if apimachineryutilwait.Interrupted(err) && errors.Is(provisionerCtx.Err(), context.Canceled) {
				return
			}
			o.Expect(err).NotTo(o.HaveOccurred())
		}()

		framework.By("Creating a ScyllaCluster")
		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to roll out (RV=%s)", sc.ResourceVersion)
		waitCtx1, waitCtx1Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx1Cancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx1, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scyllaclusterverification.Verify(ctx, f.KubeClient(), f.ScyllaClient(), sc)
		scyllaclusterverification.WaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)

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
		waitCtx2, waitCtx2Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx2Cancel()
		pvc, err = controllerhelpers.WaitForPVCState(waitCtx2, f.KubeClient().CoreV1().PersistentVolumeClaims(pvc.Namespace), pvc.Name, controllerhelpers.WaitForStateOptions{TolerateDelete: true}, func(freshPVC *corev1.PersistentVolumeClaim) (bool, error) {
			return freshPVC.UID != pvc.UID, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to observe the degradation")
		waitCtx3, waitCtx3Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx3Cancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx3, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, func(sc *scyllav1.ScyllaCluster) (bool, error) {
			rolledOut, err := utils.IsScyllaClusterRolledOut(sc)
			return !rolledOut, err
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to roll out (RV=%s)", sc.ResourceVersion)
		waitCtx4, waitCtx4Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx4Cancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx4, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
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
