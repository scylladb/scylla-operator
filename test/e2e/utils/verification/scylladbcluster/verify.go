package scylladbcluster

import (
	"context"

	o "github.com/onsi/gomega"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scylladbclustercontroller "github.com/scylladb/scylla-operator/pkg/controller/scylladbcluster"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/internalapi"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func Verify(ctx context.Context, sc *scyllav1alpha1.ScyllaDBCluster, rkcClusterMap map[string]framework.ClusterInterface) {
	framework.By("Verifying the ScyllaDBCluster")

	sc = sc.DeepCopy()

	o.Expect(sc.CreationTimestamp).NotTo(o.BeNil())
	o.Expect(sc.Status.ObservedGeneration).NotTo(o.BeNil())
	o.Expect(*sc.Status.ObservedGeneration).To(o.BeNumerically(">=", sc.Generation))
	o.Expect(sc.Status.Datacenters).To(o.HaveLen(len(sc.Spec.Datacenters)))

	for i := range sc.Status.Conditions {
		c := &sc.Status.Conditions[i]
		o.Expect(c.LastTransitionTime).NotTo(o.BeNil())
		o.Expect(c.LastTransitionTime.Time.Before(sc.CreationTimestamp.Time)).NotTo(o.BeTrue())

		// To be able to compare the statuses we need to remove the random timestamp.
		c.LastTransitionTime = metav1.Time{}
	}

	o.Expect(sc.Status.Conditions).To(o.ConsistOf(func() []interface{} {
		type condValue struct {
			condType string
			status   metav1.ConditionStatus
		}
		condList := []condValue{
			// Aggregated conditions
			{
				condType: scyllav1alpha1.AvailableCondition,
				status:   metav1.ConditionTrue,
			},
			{
				condType: scyllav1alpha1.ProgressingCondition,
				status:   metav1.ConditionFalse,
			},
			{
				condType: scyllav1alpha1.DegradedCondition,
				status:   metav1.ConditionFalse,
			},
		}

		for _, dc := range sc.Spec.Datacenters {
			dcCondList := []condValue{
				// Datacenter aggregated conditions
				{
					condType: internalapi.MakeDatacenterConditionFunc(scyllav1alpha1.AvailableCondition)(dc.Name),
					status:   metav1.ConditionTrue,
				},
				{
					condType: internalapi.MakeDatacenterConditionFunc(scyllav1alpha1.ProgressingCondition)(dc.Name),
					status:   metav1.ConditionFalse,
				},
				{
					condType: internalapi.MakeDatacenterConditionFunc(scyllav1alpha1.DegradedCondition)(dc.Name),
					status:   metav1.ConditionFalse,
				},

				// Datacenter controller conditions
				{
					condType: scylladbclustercontroller.MakeRemoteKindControllerDatacenterConditionFunc("ScyllaDBDatacenter", scyllav1alpha1.ProgressingCondition)(dc.Name),
					status:   metav1.ConditionFalse,
				},
				{
					condType: scylladbclustercontroller.MakeRemoteKindControllerDatacenterConditionFunc("ScyllaDBDatacenter", scyllav1alpha1.DegradedCondition)(dc.Name),
					status:   metav1.ConditionFalse,
				},
				{
					condType: scylladbclustercontroller.MakeRemoteKindControllerDatacenterConditionFunc("Namespace", scyllav1alpha1.ProgressingCondition)(dc.Name),
					status:   metav1.ConditionFalse,
				},
				{
					condType: scylladbclustercontroller.MakeRemoteKindControllerDatacenterConditionFunc("Namespace", scyllav1alpha1.DegradedCondition)(dc.Name),
					status:   metav1.ConditionFalse,
				},
				{
					condType: scylladbclustercontroller.MakeRemoteKindControllerDatacenterConditionFunc("Service", scyllav1alpha1.ProgressingCondition)(dc.Name),
					status:   metav1.ConditionFalse,
				},
				{
					condType: scylladbclustercontroller.MakeRemoteKindControllerDatacenterConditionFunc("Service", scyllav1alpha1.DegradedCondition)(dc.Name),
					status:   metav1.ConditionFalse,
				},
				{
					condType: scylladbclustercontroller.MakeRemoteKindControllerDatacenterConditionFunc("EndpointSlice", scyllav1alpha1.ProgressingCondition)(dc.Name),
					status:   metav1.ConditionFalse,
				},
				{
					condType: scylladbclustercontroller.MakeRemoteKindControllerDatacenterConditionFunc("EndpointSlice", scyllav1alpha1.DegradedCondition)(dc.Name),
					status:   metav1.ConditionFalse,
				},
				{
					condType: scylladbclustercontroller.MakeRemoteKindControllerDatacenterConditionFunc("Endpoints", scyllav1alpha1.ProgressingCondition)(dc.Name),
					status:   metav1.ConditionFalse,
				},
				{
					condType: scylladbclustercontroller.MakeRemoteKindControllerDatacenterConditionFunc("Endpoints", scyllav1alpha1.DegradedCondition)(dc.Name),
					status:   metav1.ConditionFalse,
				},
				{
					condType: scylladbclustercontroller.MakeRemoteKindControllerDatacenterConditionFunc("RemoteOwner", scyllav1alpha1.ProgressingCondition)(dc.Name),
					status:   metav1.ConditionFalse,
				},
				{
					condType: scylladbclustercontroller.MakeRemoteKindControllerDatacenterConditionFunc("RemoteOwner", scyllav1alpha1.DegradedCondition)(dc.Name),
					status:   metav1.ConditionFalse,
				},
				{
					condType: scylladbclustercontroller.MakeRemoteKindControllerDatacenterConditionFunc("ConfigMap", scyllav1alpha1.ProgressingCondition)(dc.Name),
					status:   metav1.ConditionFalse,
				},
				{
					condType: scylladbclustercontroller.MakeRemoteKindControllerDatacenterConditionFunc("ConfigMap", scyllav1alpha1.DegradedCondition)(dc.Name),
					status:   metav1.ConditionFalse,
				},
				{
					condType: scylladbclustercontroller.MakeRemoteKindControllerDatacenterConditionFunc("Secret", scyllav1alpha1.ProgressingCondition)(dc.Name),
					status:   metav1.ConditionFalse,
				},
				{
					condType: scylladbclustercontroller.MakeRemoteKindControllerDatacenterConditionFunc("Secret", scyllav1alpha1.DegradedCondition)(dc.Name),
					status:   metav1.ConditionFalse,
				},
			}

			condList = append(condList, dcCondList...)
		}

		expectedConditions := make([]interface{}, 0, len(condList))
		for _, item := range condList {
			expectedConditions = append(expectedConditions, metav1.Condition{
				Type:               item.condType,
				Status:             item.status,
				Reason:             "AsExpected",
				Message:            "",
				ObservedGeneration: sc.Generation,
			})
		}

		return expectedConditions
	}()...))

	nodeCount := controllerhelpers.GetScyllaDBClusterNodeCount(sc)
	o.Expect(sc.Status.Nodes).ToNot(o.BeNil())
	o.Expect(*sc.Status.Nodes).To(o.Equal(nodeCount))

	o.Expect(sc.Status.CurrentNodes).ToNot(o.BeNil())
	o.Expect(*sc.Status.CurrentNodes).To(o.Equal(nodeCount))

	o.Expect(sc.Status.UpdatedNodes).ToNot(o.BeNil())
	o.Expect(*sc.Status.UpdatedNodes).To(o.Equal(nodeCount))

	o.Expect(sc.Status.ReadyNodes).ToNot(o.BeNil())
	o.Expect(*sc.Status.ReadyNodes).To(o.Equal(nodeCount))

	o.Expect(sc.Status.AvailableNodes).ToNot(o.BeNil())
	o.Expect(*sc.Status.AvailableNodes).To(o.Equal(nodeCount))

	for _, dc := range sc.Spec.Datacenters {
		o.Expect(rkcClusterMap).To(o.HaveKey(dc.RemoteKubernetesClusterName))
		cluster := rkcClusterMap[dc.RemoteKubernetesClusterName]
		o.Expect(cluster).NotTo(o.BeNil())

		dcStatus, _, ok := slices.Find(sc.Status.Datacenters, func(dcStatus scyllav1alpha1.ScyllaDBClusterDatacenterStatus) bool {
			return dcStatus.Name == dc.Name
		})
		o.Expect(ok).To(o.BeTrue())
		o.Expect(dcStatus.RemoteNamespaceName).NotTo(o.BeNil())

		sdc, err := cluster.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBDatacenters(*dcStatus.RemoteNamespaceName).Get(ctx, naming.ScyllaDBDatacenterName(sc, &dc), metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaDBDatacenter(sc, dc, sdc)

		dcNodeCount := controllerhelpers.GetScyllaDBClusterDatacenterNodeCount(sc, dc)

		o.Expect(dcStatus.Stale).NotTo(o.BeNil())
		o.Expect(*dcStatus.Stale).To(o.BeFalse())

		o.Expect(dcStatus.Nodes).ToNot(o.BeNil())
		o.Expect(*dcStatus.Nodes).To(o.Equal(dcNodeCount))

		o.Expect(dcStatus.CurrentNodes).ToNot(o.BeNil())
		o.Expect(*dcStatus.CurrentNodes).To(o.Equal(dcNodeCount))

		o.Expect(dcStatus.UpdatedNodes).ToNot(o.BeNil())
		o.Expect(*dcStatus.UpdatedNodes).To(o.Equal(dcNodeCount))

		o.Expect(dcStatus.ReadyNodes).ToNot(o.BeNil())
		o.Expect(*dcStatus.ReadyNodes).To(o.Equal(dcNodeCount))

		o.Expect(dcStatus.AvailableNodes).ToNot(o.BeNil())
		o.Expect(*dcStatus.AvailableNodes).To(o.Equal(dcNodeCount))

		otherDCs := slices.FilterOut(sc.Spec.Datacenters, func(otherDC scyllav1alpha1.ScyllaDBClusterDatacenter) bool {
			return dc.Name == otherDC.Name
		})

		for _, otherDC := range otherDCs {
			otherDCSeedService, err := cluster.KubeAdminClient().CoreV1().Services(*dcStatus.RemoteNamespaceName).Get(ctx, naming.SeedService(sc, &otherDC), metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			o.Expect(otherDCSeedService.Spec.Selector).To(o.BeNil())
			o.Expect(otherDCSeedService.Spec.ClusterIP).To(o.Equal(corev1.ClusterIPNone))
			o.Expect(otherDCSeedService.Spec.Type).To(o.Equal(corev1.ServiceTypeClusterIP))
			o.Expect(otherDCSeedService.Spec.Ports).To(o.ConsistOf([]corev1.ServicePort{
				{
					Name:       "inter-node",
					Protocol:   corev1.ProtocolTCP,
					Port:       7000,
					TargetPort: intstr.IntOrString{IntVal: 7000},
				},
				{
					Name:       "inter-node-ssl",
					Protocol:   corev1.ProtocolTCP,
					Port:       7001,
					TargetPort: intstr.IntOrString{IntVal: 7001},
				},
				{
					Name:       "cql",
					Protocol:   corev1.ProtocolTCP,
					Port:       9042,
					TargetPort: intstr.IntOrString{IntVal: 9042},
				},
				{
					Name:       "cql-ssl",
					Protocol:   corev1.ProtocolTCP,
					Port:       9142,
					TargetPort: intstr.IntOrString{IntVal: 9142},
				},
			}))

			seedServiceEndpointSlice, err := cluster.KubeAdminClient().DiscoveryV1().EndpointSlices(*dcStatus.RemoteNamespaceName).Get(ctx, otherDCSeedService.Name, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			o.Expect(seedServiceEndpointSlice.Ports).To(o.ConsistOf([]discoveryv1.EndpointPort{
				{
					Name:     pointer.Ptr("inter-node"),
					Protocol: pointer.Ptr(corev1.ProtocolTCP),
					Port:     pointer.Ptr(int32(7000)),
				},
				{
					Name:     pointer.Ptr("inter-node-ssl"),
					Protocol: pointer.Ptr(corev1.ProtocolTCP),
					Port:     pointer.Ptr(int32(7001)),
				},
				{
					Name:     pointer.Ptr("cql"),
					Protocol: pointer.Ptr(corev1.ProtocolTCP),
					Port:     pointer.Ptr(int32(9042)),
				},
				{
					Name:     pointer.Ptr("cql-ssl"),
					Protocol: pointer.Ptr(corev1.ProtocolTCP),
					Port:     pointer.Ptr(int32(9142)),
				},
			}))
			o.Expect(seedServiceEndpointSlice.Endpoints).To(o.HaveLen(1))

			o.Expect(seedServiceEndpointSlice.Endpoints[0].Addresses).To(o.HaveLen(int(dcNodeCount)))

			o.Expect(seedServiceEndpointSlice.Endpoints[0].Conditions.Ready).ToNot(o.BeNil())
			o.Expect(*seedServiceEndpointSlice.Endpoints[0].Conditions.Ready).To(o.BeTrue())

			o.Expect(seedServiceEndpointSlice.Endpoints[0].Conditions.Serving).ToNot(o.BeNil())
			o.Expect(*seedServiceEndpointSlice.Endpoints[0].Conditions.Serving).To(o.BeTrue())

			o.Expect(seedServiceEndpointSlice.Endpoints[0].Conditions.Terminating).ToNot(o.BeNil())
			o.Expect(*seedServiceEndpointSlice.Endpoints[0].Conditions.Terminating).To(o.BeFalse())
		}
	}
}

func verifyScyllaDBDatacenter(sc *scyllav1alpha1.ScyllaDBCluster, dc scyllav1alpha1.ScyllaDBClusterDatacenter, sdc *scyllav1alpha1.ScyllaDBDatacenter) {
	o.Expect(sdc.CreationTimestamp).NotTo(o.BeNil())
	o.Expect(sdc.Status.ObservedGeneration).NotTo(o.BeNil())
	o.Expect(*sdc.Status.ObservedGeneration).To(o.BeNumerically(">=", sc.Generation))

	dcNodeCount := controllerhelpers.GetScyllaDBClusterDatacenterNodeCount(sc, dc)

	o.Expect(sdc.Status.Nodes).NotTo(o.BeNil())
	o.Expect(*sdc.Status.Nodes).To(o.Equal(dcNodeCount))

	o.Expect(sdc.Status.ReadyNodes).NotTo(o.BeNil())
	o.Expect(*sdc.Status.ReadyNodes).To(o.Equal(dcNodeCount))

	o.Expect(sdc.Status.AvailableNodes).NotTo(o.BeNil())
	o.Expect(*sdc.Status.AvailableNodes).To(o.Equal(dcNodeCount))
}
