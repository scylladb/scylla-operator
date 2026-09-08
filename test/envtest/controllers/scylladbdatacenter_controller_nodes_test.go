//go:build envtest

package controllers

import (
	"context"
	"fmt"
	"maps"
	"strconv"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configassets "github.com/scylladb/scylla-operator/assets/config"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/internalapi"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/test/unit"
	"github.com/scylladb/scylla-operator/test/envtest"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

const (
	// envtestReplacedNodeHostID and envtestReplacingNodeHostID are the host IDs the sidecar would annotate the member
	// Service with before and after a node is replaced.
	envtestReplacedNodeHostID  = "0e6bd9d4-8a6b-4bd0-8a4b-6d5c2a9f0001"
	envtestReplacingNodeHostID = "0e6bd9d4-8a6b-4bd0-8a4b-6d5c2a9f0002"

	// envtestTokenRingHash stands in for the token ring hash the sidecar annotates the member Services with. Set as
	// both the current and the last cleaned up hash, it marks a node that needs no cleanup.
	envtestTokenRingHash = "envtest-token-ring"

	// nodeReplacementEventuallyTimeout covers the fixed wait the controller takes between deleting the PVC and
	// evicting the Pod of a replaced node.
	nodeReplacementEventuallyTimeout = 30 * time.Second

	// nodeReplacementPodRetentionWindow is the window in which the Pod of a replaced node is expected to stay after
	// its PVC is deleted, well within the fixed wait the controller takes.
	nodeReplacementPodRetentionWindow = 2 * time.Second
)

// Envtest runs no StatefulSet controller, so no Pod ever exists unless a spec creates one. These specs create the
// member Pods themselves to reach the paths that read them: the node replacement and the status aggregation.
var _ = g.Describe("ScyllaDBDatacenter controller member nodes", func() {
	const rackName = "rack-a"

	var env *envtest.Environment
	g.BeforeEach(func(ctx g.SpecContext) {
		env = envtest.Setup(ctx)
	})

	g.It("should replace a node using its host ID", func(ctx g.SpecContext) {
		sdc := setupRolledOutRacks(ctx, env, false, []string{rackName}, 1)
		rackStatefulSet, err := env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()).Get(ctx, naming.StatefulSetNameForRack(sdc.Spec.Racks[0], sdc), metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		memberServiceName := naming.MemberServiceName(sdc.Spec.Racks[0], sdc, 0)

		g.By("Creating the member Pod and its PVC in place of the StatefulSet controller")
		memberPod := createMemberPod(ctx, env, rackStatefulSet, 0, sdc.Spec.ScyllaDB.Image)
		memberPVC := createMemberPVC(ctx, env, memberPod.Name)

		g.By("Labelling the member Service for replacement without a host ID annotation")
		setServiceLabel(ctx, env, memberServiceName, naming.ReplaceLabel, "")

		g.By("Waiting for the replacement to wait for the host ID annotation")
		waitForServiceControllerProgressingReason(ctx, env, sdc.Name, "WaitingForHostIDAnnotationBeforeReplacement")

		g.By("Verifying nothing is deleted before the host ID is known")
		o.Consistently(func(co o.Gomega, ctx context.Context) {
			pod, err := env.TypedKubeClient().CoreV1().Pods(env.Namespace()).Get(ctx, memberPod.Name, metav1.GetOptions{})
			co.Expect(err).NotTo(o.HaveOccurred())
			co.Expect(pod.DeletionTimestamp).To(o.BeNil())

			pvc, err := env.TypedKubeClient().CoreV1().PersistentVolumeClaims(env.Namespace()).Get(ctx, memberPVC.Name, metav1.GetOptions{})
			co.Expect(err).NotTo(o.HaveOccurred())
			co.Expect(pvc.DeletionTimestamp).To(o.BeNil())
		}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultConsistentlyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())

		g.By("Annotating the member Service with the host ID of the node in place of the sidecar")
		setServiceAnnotation(ctx, env, memberServiceName, naming.HostIDAnnotation, envtestReplacedNodeHostID)

		// The PVC has to go before the Pod, or the StatefulSet controller would recreate the Pod on the old PVC, and
		// the controller waits a fixed 10 seconds between the two for the StatefulSet controller to observe the PVC
		// deletion. The PVC is protected by the in-use finalizer, which nothing removes in envtest, so its deletion
		// only marks it.
		g.By("Waiting for the PVC to be deleted")
		o.Eventually(func(eo o.Gomega, ctx context.Context) {
			pvc, err := env.TypedKubeClient().CoreV1().PersistentVolumeClaims(env.Namespace()).Get(ctx, memberPVC.Name, metav1.GetOptions{})
			eo.Expect(err).NotTo(o.HaveOccurred())
			eo.Expect(pvc.DeletionTimestamp).NotTo(o.BeNil())
		}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())

		g.By("Verifying the Pod is kept while the StatefulSet controller is given time to observe the PVC deletion")
		o.Consistently(func(co o.Gomega, ctx context.Context) {
			pod, err := env.TypedKubeClient().CoreV1().Pods(env.Namespace()).Get(ctx, memberPod.Name, metav1.GetOptions{})
			co.Expect(err).NotTo(o.HaveOccurred())
			co.Expect(pod.UID).To(o.Equal(memberPod.UID))
			co.Expect(pod.DeletionTimestamp).To(o.BeNil())
		}).WithContext(ctx).WithTimeout(nodeReplacementPodRetentionWindow).WithPolling(100 * time.Millisecond).Should(o.Succeed())

		g.By("Waiting for the Pod to be evicted and the member Service to record the host ID being replaced")
		o.Eventually(func(eo o.Gomega, ctx context.Context) {
			_, err := env.TypedKubeClient().CoreV1().Pods(env.Namespace()).Get(ctx, memberPod.Name, metav1.GetOptions{})
			eo.Expect(apierrors.IsNotFound(err)).To(o.BeTrue(), "Pod %q should be evicted: %v", memberPod.Name, err)

			svc, err := env.TypedKubeClient().CoreV1().Services(env.Namespace()).Get(ctx, memberServiceName, metav1.GetOptions{})
			eo.Expect(err).NotTo(o.HaveOccurred())
			eo.Expect(svc.Labels).To(o.HaveKeyWithValue(naming.ReplacingNodeHostIDLabel, envtestReplacedNodeHostID))
			eo.Expect(svc.Labels).To(o.HaveKey(naming.ReplaceLabel))
		}).WithContext(ctx).WithTimeout(nodeReplacementEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())

		g.By("Waiting for the replacement to wait for the Pod to be recreated")
		waitForServiceControllerProgressingReason(ctx, env, sdc.Name, "WaitingForPodRecreation")

		g.By("Recreating the member Pod in place of the StatefulSet controller")
		createMemberPod(ctx, env, rackStatefulSet, 0, sdc.Spec.ScyllaDB.Image)

		g.By("Waiting for the replacement to wait for the new host ID")
		waitForServiceControllerProgressingReason(ctx, env, sdc.Name, "WaitingForUpdatedHostIDAnnotation")

		g.By("Annotating the member Service with the host ID of the new node in place of the sidecar")
		setServiceAnnotation(ctx, env, memberServiceName, naming.HostIDAnnotation, envtestReplacingNodeHostID)

		g.By("Waiting for the replacement to finish")
		o.Eventually(func(eo o.Gomega, ctx context.Context) {
			svc, err := env.TypedKubeClient().CoreV1().Services(env.Namespace()).Get(ctx, memberServiceName, metav1.GetOptions{})
			eo.Expect(err).NotTo(o.HaveOccurred())
			eo.Expect(svc.Labels).NotTo(o.HaveKey(naming.ReplaceLabel))
			eo.Expect(svc.Labels).NotTo(o.HaveKey(naming.ReplacingNodeHostIDLabel))
			eo.Expect(svc.Annotations).To(o.HaveKeyWithValue(naming.HostIDAnnotation, envtestReplacingNodeHostID))

			progressingCondition := getScyllaDBDatacenterCondition(ctx, env, sdc.Name, internalapi.MakeKindControllerCondition("Service", scyllav1alpha1.ProgressingCondition))
			eo.Expect(progressingCondition).NotTo(o.BeNil())
			eo.Expect(progressingCondition.Status).To(o.Equal(metav1.ConditionFalse))
		}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())
	})

	g.It("should aggregate the node counts, versions and availability of the racks in the status", func(ctx g.SpecContext) {
		const (
			otherRackName = "rack-b"
			nodes         = int32(2)
		)

		sdc := setupRolledOutRacks(ctx, env, true, []string{rackName, otherRackName}, nodes)
		expectedVersion, err := naming.ImageToVersion(sdc.Spec.ScyllaDB.Image)
		o.Expect(err).NotTo(o.HaveOccurred())
		otherRackStatefulSetName := naming.StatefulSetNameForRack(sdc.Spec.Racks[1], sdc)

		g.By("Waiting for the racks to be reported at no version with no member Pod")
		o.Eventually(func(eo o.Gomega, ctx context.Context) {
			availableCondition := getScyllaDBDatacenterCondition(ctx, env, sdc.Name, internalapi.MakeKindControllerCondition("StatefulSet", scyllav1alpha1.AvailableCondition))
			eo.Expect(availableCondition).NotTo(o.BeNil())
			eo.Expect(availableCondition.Status).To(o.Equal(metav1.ConditionFalse))
			eo.Expect(availableCondition.Reason).To(o.Equal("RacksNotAtDesiredVersion"))

			for _, rack := range sdc.Spec.Racks {
				rackStatus := getRackStatus(ctx, env, sdc.Name, rack.Name)
				eo.Expect(rackStatus).NotTo(o.BeNil())
				eo.Expect(rackStatus.CurrentVersion).To(o.BeEmpty())
				eo.Expect(rackStatus.UpdatedVersion).To(o.Equal(expectedVersion))
			}
		}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())

		// The serving certificate needs the host ID of every node and the cleanup Jobs its token ring hash, both of
		// which the sidecars annotate the member Services with. Equal current and last cleaned up hashes mean no
		// cleanup is due, so the Job sync has nothing to wait for and the datacenter can stop progressing.
		g.By("Creating the member Pods in place of the StatefulSet controller and annotating the member Services in place of the sidecars")
		for _, rack := range sdc.Spec.Racks {
			rackStatefulSet, err := env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()).Get(ctx, naming.StatefulSetNameForRack(rack, sdc), metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			for ordinal := range int(nodes) {
				createMemberPod(ctx, env, rackStatefulSet, ordinal, sdc.Spec.ScyllaDB.Image)

				memberServiceName := naming.MemberServiceName(rack, sdc, ordinal)
				setServiceAnnotations(ctx, env, memberServiceName, map[string]string{
					naming.HostIDAnnotation:                     fmt.Sprintf("%s-%d", rack.Name, ordinal),
					naming.CurrentTokenRingHashAnnotation:       envtestTokenRingHash,
					naming.LastCleanedUpTokenRingHashAnnotation: envtestTokenRingHash,
				})
			}
		}

		g.By("Waiting for the datacenter to be reported as available at the desired version")
		o.Eventually(func(eo o.Gomega, ctx context.Context) {
			sdc, err := env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).Get(ctx, sdc.Name, metav1.GetOptions{})
			eo.Expect(err).NotTo(o.HaveOccurred())
			eo.Expect(sdc.Status.ObservedGeneration).To(o.HaveValue(o.Equal(sdc.Generation)))
			eo.Expect(sdc.Status.Nodes).To(o.HaveValue(o.Equal(2 * nodes)))
			eo.Expect(sdc.Status.ReadyNodes).To(o.HaveValue(o.Equal(2 * nodes)))
			eo.Expect(sdc.Status.AvailableNodes).To(o.HaveValue(o.Equal(2 * nodes)))

			eo.Expect(sdc.Status.Racks).To(o.HaveLen(2))
			for _, rackStatus := range sdc.Status.Racks {
				eo.Expect(rackStatus.Nodes).To(o.HaveValue(o.Equal(nodes)), "rack %q", rackStatus.Name)
				eo.Expect(rackStatus.ReadyNodes).To(o.HaveValue(o.Equal(nodes)), "rack %q", rackStatus.Name)
				eo.Expect(rackStatus.AvailableNodes).To(o.HaveValue(o.Equal(nodes)), "rack %q", rackStatus.Name)
				eo.Expect(rackStatus.UpdatedNodes).To(o.HaveValue(o.Equal(nodes)), "rack %q", rackStatus.Name)
				eo.Expect(rackStatus.Stale).To(o.HaveValue(o.BeFalse()), "rack %q", rackStatus.Name)
				eo.Expect(rackStatus.CurrentVersion).To(o.Equal(expectedVersion), "rack %q", rackStatus.Name)
				eo.Expect(rackStatus.UpdatedVersion).To(o.Equal(expectedVersion), "rack %q", rackStatus.Name)
			}

			expectConditionStatus(eo, sdc, internalapi.MakeKindControllerCondition("StatefulSet", scyllav1alpha1.AvailableCondition), metav1.ConditionTrue)
			expectConditionStatus(eo, sdc, scyllav1alpha1.AvailableCondition, metav1.ConditionTrue)
			expectConditionStatus(eo, sdc, scyllav1alpha1.ProgressingCondition, metav1.ConditionFalse)
			expectConditionStatus(eo, sdc, scyllav1alpha1.DegradedCondition, metav1.ConditionFalse)
		}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())

		g.By(fmt.Sprintf("Running the first node of the %q rack at an older ScyllaDB version", otherRackName))
		olderImage := unit.ScyllaDBImageRepository + ":" + configassets.Project.OperatorTests.ScyllaDBVersions.UpgradeFrom
		olderVersion, err := naming.ImageToVersion(olderImage)
		o.Expect(err).NotTo(o.HaveOccurred())
		setPodScyllaDBImage(ctx, env, naming.MemberServiceName(sdc.Spec.Racks[1], sdc, 0), olderImage)

		g.By(fmt.Sprintf("Waiting for the %q rack to be reported at the older version and the datacenter as unavailable", otherRackName))
		o.Eventually(func(eo o.Gomega, ctx context.Context) {
			rackStatus := getRackStatus(ctx, env, sdc.Name, otherRackName)
			eo.Expect(rackStatus).NotTo(o.BeNil())
			eo.Expect(rackStatus.CurrentVersion).To(o.Equal(olderVersion))
			eo.Expect(rackStatus.UpdatedVersion).To(o.Equal(expectedVersion))

			availableCondition := getScyllaDBDatacenterCondition(ctx, env, sdc.Name, internalapi.MakeKindControllerCondition("StatefulSet", scyllav1alpha1.AvailableCondition))
			eo.Expect(availableCondition).NotTo(o.BeNil())
			eo.Expect(availableCondition.Status).To(o.Equal(metav1.ConditionFalse))
			eo.Expect(availableCondition.Reason).To(o.Equal("RacksNotAtDesiredVersion"))
			eo.Expect(availableCondition.Message).To(o.ContainSubstring(otherRackName))
			eo.Expect(availableCondition.Message).NotTo(o.ContainSubstring(rackName))
		}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())

		g.By(fmt.Sprintf("Running the first node of the %q rack at the desired ScyllaDB version again", otherRackName))
		setPodScyllaDBImage(ctx, env, naming.MemberServiceName(sdc.Spec.Racks[1], sdc, 0), sdc.Spec.ScyllaDB.Image)

		g.By(fmt.Sprintf("Marking one node of the %q rack as not ready in place of the StatefulSet controller", otherRackName))
		markStatefulSetNodesAsNotReady(ctx, env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()), otherRackStatefulSetName, 1)

		g.By("Waiting for the not ready node to be reported")
		o.Eventually(func(eo o.Gomega, ctx context.Context) {
			sdc, err := env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).Get(ctx, sdc.Name, metav1.GetOptions{})
			eo.Expect(err).NotTo(o.HaveOccurred())
			eo.Expect(sdc.Status.Nodes).To(o.HaveValue(o.Equal(2 * nodes)))
			eo.Expect(sdc.Status.ReadyNodes).To(o.HaveValue(o.Equal(2*nodes - 1)))
			eo.Expect(sdc.Status.AvailableNodes).To(o.HaveValue(o.Equal(2*nodes - 1)))

			rackStatus := getRackStatus(ctx, env, sdc.Name, otherRackName)
			eo.Expect(rackStatus).NotTo(o.BeNil())
			eo.Expect(rackStatus.ReadyNodes).To(o.HaveValue(o.Equal(nodes - 1)))
			eo.Expect(rackStatus.AvailableNodes).To(o.HaveValue(o.Equal(nodes - 1)))
			eo.Expect(rackStatus.Stale).To(o.HaveValue(o.BeFalse()))

			availableCondition := getScyllaDBDatacenterCondition(ctx, env, sdc.Name, internalapi.MakeKindControllerCondition("StatefulSet", scyllav1alpha1.AvailableCondition))
			eo.Expect(availableCondition).NotTo(o.BeNil())
			eo.Expect(availableCondition.Status).To(o.Equal(metav1.ConditionFalse))
			eo.Expect(availableCondition.Reason).To(o.Equal("MembersNotReady"))
			eo.Expect(availableCondition.Message).To(o.Equal(fmt.Sprintf("Only %d out of %d member(s) are ready", 2*nodes-1, 2*nodes)))
			expectConditionStatus(eo, sdc, scyllav1alpha1.AvailableCondition, metav1.ConditionFalse)
		}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())

		g.By(fmt.Sprintf("Marking the %q rack StatefulSet as not rolled out in place of the StatefulSet controller", otherRackName))
		markStatefulSetAsNotRolledOut(ctx, env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()), otherRackStatefulSetName)

		// A stale rack doesn't count towards the updated and ready members, so the rollout is reported before the
		// readiness.
		g.By("Waiting for the stale rack to be reported")
		o.Eventually(func(eo o.Gomega, ctx context.Context) {
			sdc, err := env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).Get(ctx, sdc.Name, metav1.GetOptions{})
			eo.Expect(err).NotTo(o.HaveOccurred())

			rackStatus := getRackStatus(ctx, env, sdc.Name, otherRackName)
			eo.Expect(rackStatus).NotTo(o.BeNil())
			eo.Expect(rackStatus.Stale).To(o.HaveValue(o.BeTrue()))

			availableCondition := getScyllaDBDatacenterCondition(ctx, env, sdc.Name, internalapi.MakeKindControllerCondition("StatefulSet", scyllav1alpha1.AvailableCondition))
			eo.Expect(availableCondition).NotTo(o.BeNil())
			eo.Expect(availableCondition.Status).To(o.Equal(metav1.ConditionFalse))
			eo.Expect(availableCondition.Reason).To(o.Equal("MembersNotUpdated"))
			eo.Expect(availableCondition.Message).To(o.Equal(fmt.Sprintf("Only %d out of %d member(s) have been updated", nodes, 2*nodes)))

			progressingCondition := getStatefulSetControllerProgressingCondition(ctx, env, sdc.Name)
			eo.Expect(progressingCondition).NotTo(o.BeNil())
			eo.Expect(progressingCondition.Status).To(o.Equal(metav1.ConditionTrue))
			eo.Expect(progressingCondition.Reason).To(o.Equal("WaitingForStatefulSetRollout"))
			expectConditionStatus(eo, sdc, scyllav1alpha1.ProgressingCondition, metav1.ConditionTrue)
		}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())
	})
})

// expectConditionStatus expects the condition of the given type to be present with the given status.
func expectConditionStatus(eo o.Gomega, sdc *scyllav1alpha1.ScyllaDBDatacenter, conditionType string, status metav1.ConditionStatus) {
	g.GinkgoHelper()

	condition := apimeta.FindStatusCondition(sdc.Status.Conditions, conditionType)
	eo.Expect(condition).NotTo(o.BeNil(), "condition %q is missing", conditionType)
	eo.Expect(condition.Status).To(o.Equal(status), "condition %q: %s", conditionType, condition.Message)
}

// waitForServiceControllerProgressingReason waits for the Service sync to report progressing with the given reason.
func waitForServiceControllerProgressingReason(ctx context.Context, e *envtest.Environment, sdcName, reason string) {
	g.GinkgoHelper()

	o.Eventually(func(eo o.Gomega, ctx context.Context) {
		progressingCondition := getScyllaDBDatacenterCondition(ctx, e, sdcName, internalapi.MakeKindControllerCondition("Service", scyllav1alpha1.ProgressingCondition))
		eo.Expect(progressingCondition).NotTo(o.BeNil())
		eo.Expect(progressingCondition.Status).To(o.Equal(metav1.ConditionTrue))
		eo.Expect(progressingCondition.Reason).To(o.Equal(reason))
	}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())
}

// createMemberPod creates the Pod of the given ordinal of the rack StatefulSet in place of the StatefulSet
// controller, which envtest doesn't run. The Pod is owned by the StatefulSet, carries the labels of its Pod template,
// runs ScyllaDB from the given image and is reported ready with an IP address. It is left unscheduled on purpose: the
// API server deletes an unscheduled Pod right away without a kubelet, and evicts a pending one without consulting the
// PodDisruptionBudget, whose status nothing maintains in envtest.
func createMemberPod(ctx context.Context, e *envtest.Environment, sts *appsv1.StatefulSet, ordinal int, scyllaDBImage string) *corev1.Pod {
	g.GinkgoHelper()

	name := naming.MemberServiceNameForStatefulSet(sts.Name, ordinal)

	labels := maps.Clone(sts.Spec.Template.Labels)
	maps.Copy(labels, naming.StatefulSetPodLabel(name))

	pod, err := e.TypedKubeClient().CoreV1().Pods(e.Namespace()).Create(ctx, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: e.Namespace(),
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(sts, appsv1.SchemeGroupVersion.WithKind("StatefulSet")),
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  naming.ScyllaContainerName,
					Image: scyllaDBImage,
				},
			},
		},
	}, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	// The IP address is unique in the datacenter: the serving certificate lists the address of every Pod.
	rackOrdinal, err := strconv.Atoi(labels[naming.RackOrdinalLabel])
	o.Expect(err).NotTo(o.HaveOccurred())
	podIP := fmt.Sprintf("10.244.%d.%d", rackOrdinal, ordinal+1)
	pod.Status = corev1.PodStatus{
		Phase: corev1.PodPending,
		Conditions: []corev1.PodCondition{
			{
				Type:   corev1.PodReady,
				Status: corev1.ConditionTrue,
			},
		},
		PodIP:  podIP,
		PodIPs: []corev1.PodIP{{IP: podIP}},
	}
	pod, err = e.TypedKubeClient().CoreV1().Pods(e.Namespace()).UpdateStatus(ctx, pod, metav1.UpdateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	return pod
}

// createMemberPVC creates the data PVC of the given member Pod in place of the StatefulSet controller.
func createMemberPVC(ctx context.Context, e *envtest.Environment, podName string) *corev1.PersistentVolumeClaim {
	g.GinkgoHelper()

	pvc, err := e.TypedKubeClient().CoreV1().PersistentVolumeClaims(e.Namespace()).Create(ctx, &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      naming.PVCNameForPod(podName),
			Namespace: e.Namespace(),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
		},
	}, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	return pvc
}

// setPodScyllaDBImage sets the image of the ScyllaDB container of the named Pod.
func setPodScyllaDBImage(ctx context.Context, e *envtest.Environment, podName, image string) {
	g.GinkgoHelper()

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		pod, err := e.TypedKubeClient().CoreV1().Pods(e.Namespace()).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("can't get Pod %q: %w", naming.ManualRef(e.Namespace(), podName), err)
		}

		idx, err := naming.FindScyllaContainer(pod.Spec.Containers)
		if err != nil {
			return fmt.Errorf("can't find ScyllaDB container in Pod %q: %w", naming.ObjRef(pod), err)
		}

		pod.Spec.Containers[idx].Image = image
		_, err = e.TypedKubeClient().CoreV1().Pods(e.Namespace()).Update(ctx, pod, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("can't update Pod %q: %w", naming.ObjRef(pod), err)
		}

		return nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func setServiceAnnotation(ctx context.Context, e *envtest.Environment, name, key, value string) {
	g.GinkgoHelper()

	setServiceAnnotations(ctx, e, name, map[string]string{key: value})
}

func setServiceAnnotations(ctx context.Context, e *envtest.Environment, name string, annotations map[string]string) {
	g.GinkgoHelper()

	updateService(ctx, e, name, func(svc *corev1.Service) {
		if svc.Annotations == nil {
			svc.Annotations = map[string]string{}
		}
		maps.Copy(svc.Annotations, annotations)
	})
}
