package scylladbcluster

import (
	"fmt"
	"reflect"
	"slices"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllav1alpha1listers "github.com/scylladb/scylla-operator/pkg/client/scylla/listers/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	remotelister "github.com/scylladb/scylla-operator/pkg/remoteclient/lister"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryruntime "k8s.io/apimachinery/pkg/runtime"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

const (
	testClusterDomain = "test-cluster.local"
)

func TestMakeRemoteOwners(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name                 string
		cluster              *scyllav1alpha1.ScyllaDBCluster
		datacenter           *scyllav1alpha1.ScyllaDBClusterDatacenter
		remoteNamespace      *corev1.Namespace
		expectedRemoteOwners []*scyllav1alpha1.RemoteOwner
	}{
		{
			name: "RemoteOwner for remote datacenter",
			cluster: &scyllav1alpha1.ScyllaDBCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster",
					Namespace: "scylla",
				},
				Spec: scyllav1alpha1.ScyllaDBClusterSpec{
					Datacenters: []scyllav1alpha1.ScyllaDBClusterDatacenter{
						{
							Name:                        "dc1",
							RemoteKubernetesClusterName: "dc1-rkc",
						},
					},
				},
			},
			datacenter: &scyllav1alpha1.ScyllaDBClusterDatacenter{
				Name:                        "dc1",
				RemoteKubernetesClusterName: "dc1-rkc",
			},
			remoteNamespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "scylla-abc",
				},
			},
			expectedRemoteOwners: []*scyllav1alpha1.RemoteOwner{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "scylla-abc",
						Name:      "cluster-28t03",
						Labels: map[string]string{
							"internal.scylla-operator.scylladb.com/remote-owner-cluster":   "dc1-rkc",
							"internal.scylla-operator.scylladb.com/remote-owner-namespace": "scylla",
							"internal.scylla-operator.scylladb.com/remote-owner-name":      "cluster",
							"internal.scylla-operator.scylladb.com/remote-owner-gvr":       "scylla.scylladb.com-v1alpha1-scylladbclusters",
							"scylla-operator.scylladb.com/managed-by-cluster":              "test-cluster.local",
							"app.kubernetes.io/managed-by":                                 "remote.scylla-operator.scylladb.com",
						},
					},
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			remoteOwners, err := MakeRemoteRemoteOwners(
				tc.cluster,
				tc.datacenter,
				tc.remoteNamespace,
				testClusterDomain,
			)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !equality.Semantic.DeepEqual(remoteOwners, tc.expectedRemoteOwners) {
				t.Errorf("expected remote owners %v, got %v", tc.expectedRemoteOwners, remoteOwners)
			}
		})
	}
}

func TestMakeNamespaces(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name               string
		cluster            *scyllav1alpha1.ScyllaDBCluster
		datacenter         *scyllav1alpha1.ScyllaDBClusterDatacenter
		expectedNamespaces []*corev1.Namespace
	}{
		{
			name: "remote namespace for datacenter",
			cluster: &scyllav1alpha1.ScyllaDBCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster",
					Namespace: "scylla",
				},
				Spec: scyllav1alpha1.ScyllaDBClusterSpec{
					Datacenters: []scyllav1alpha1.ScyllaDBClusterDatacenter{
						{
							Name:                        "dc1",
							RemoteKubernetesClusterName: "dc1-rkc",
						},
					},
				},
			},
			datacenter: &scyllav1alpha1.ScyllaDBClusterDatacenter{
				Name:                        "dc1",
				RemoteKubernetesClusterName: "dc1-rkc",
			},
			expectedNamespaces: []*corev1.Namespace{{
				ObjectMeta: metav1.ObjectMeta{
					Name: "scylla-28t03",
					Labels: map[string]string{
						"scylla-operator.scylladb.com/parent-scylladbcluster-name":            "cluster",
						"scylla-operator.scylladb.com/parent-scylladbcluster-namespace":       "scylla",
						"scylla-operator.scylladb.com/parent-scylladbcluster-datacenter-name": "dc1",
						"scylla-operator.scylladb.com/managed-by-cluster":                     "test-cluster.local",
						"app.kubernetes.io/managed-by":                                        "remote.scylla-operator.scylladb.com",
					},
				},
			}},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotNamespaces, err := MakeRemoteNamespaces(tc.cluster, tc.datacenter, testClusterDomain)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !equality.Semantic.DeepEqual(gotNamespaces, tc.expectedNamespaces) {
				t.Errorf("expected namespaces %v, got %v", tc.expectedNamespaces, gotNamespaces)
			}
		})
	}
}

func TestMakeRemoteServices(t *testing.T) {
	t.Parallel()

	makeSeedServiceSpec := func() corev1.ServiceSpec {
		return corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:     "inter-node",
					Protocol: corev1.ProtocolTCP,
					Port:     7000,
				},
				{
					Name:     "inter-node-ssl",
					Protocol: corev1.ProtocolTCP,
					Port:     7001,
				},
				{
					Name:     "cql",
					Protocol: corev1.ProtocolTCP,
					Port:     9042,
				},
				{
					Name:     "cql-ssl",
					Protocol: corev1.ProtocolTCP,
					Port:     9142,
				},
			},
			Selector:  nil,
			ClusterIP: corev1.ClusterIPNone,
			Type:      corev1.ServiceTypeClusterIP,
		}
	}

	tt := []struct {
		name             string
		cluster          *scyllav1alpha1.ScyllaDBCluster
		datacenter       *scyllav1alpha1.ScyllaDBClusterDatacenter
		remoteNamespace  *corev1.Namespace
		remoteController metav1.Object
		expectedServices []*corev1.Service
	}{
		{
			name: "cross-dc seed services between datacenters",
			cluster: &scyllav1alpha1.ScyllaDBCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster",
					Namespace: "scylla",
				},
				Spec: scyllav1alpha1.ScyllaDBClusterSpec{
					Datacenters: []scyllav1alpha1.ScyllaDBClusterDatacenter{
						{
							Name:                        "dc1",
							RemoteKubernetesClusterName: "dc1-rkc",
						},
						{
							Name:                        "dc2",
							RemoteKubernetesClusterName: "dc2-rkc",
						},
						{
							Name:                        "dc3",
							RemoteKubernetesClusterName: "dc3-rkc",
						},
					},
				},
			},
			datacenter: &scyllav1alpha1.ScyllaDBClusterDatacenter{
				Name:                        "dc1",
				RemoteKubernetesClusterName: "dc1-rkc",
			},
			remoteNamespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "scylla-28t03",
				},
			},
			remoteController: &scyllav1alpha1.RemoteOwner{
				ObjectMeta: metav1.ObjectMeta{
					Name: "remote-owner",
					UID:  "1234",
				},
			},
			expectedServices: []*corev1.Service{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-dc2-seed",
						Namespace: "scylla-28t03",
						Labels: map[string]string{
							"scylla-operator.scylladb.com/parent-scylladbcluster-name":            "cluster",
							"scylla-operator.scylladb.com/parent-scylladbcluster-namespace":       "scylla",
							"scylla-operator.scylladb.com/parent-scylladbcluster-datacenter-name": "dc1",
							"scylla-operator.scylladb.com/managed-by-cluster":                     "test-cluster.local",
							"app.kubernetes.io/managed-by":                                        "remote.scylla-operator.scylladb.com",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "scylla.scylladb.com/v1alpha1",
								Kind:               "RemoteOwner",
								Name:               "remote-owner",
								UID:                "1234",
								Controller:         pointer.Ptr(true),
								BlockOwnerDeletion: pointer.Ptr(true),
							},
						},
					},
					Spec: makeSeedServiceSpec(),
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-dc3-seed",
						Namespace: "scylla-28t03",
						Labels: map[string]string{
							"scylla-operator.scylladb.com/parent-scylladbcluster-name":            "cluster",
							"scylla-operator.scylladb.com/parent-scylladbcluster-namespace":       "scylla",
							"scylla-operator.scylladb.com/parent-scylladbcluster-datacenter-name": "dc1",
							"scylla-operator.scylladb.com/managed-by-cluster":                     "test-cluster.local",
							"app.kubernetes.io/managed-by":                                        "remote.scylla-operator.scylladb.com",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "scylla.scylladb.com/v1alpha1",
								Kind:               "RemoteOwner",
								Name:               "remote-owner",
								UID:                "1234",
								Controller:         pointer.Ptr(true),
								BlockOwnerDeletion: pointer.Ptr(true),
							},
						},
					},
					Spec: makeSeedServiceSpec(),
				},
			},
		},
		{
			name: "no services when no other datacenters",
			cluster: &scyllav1alpha1.ScyllaDBCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster",
					Namespace: "scylla",
				},
				Spec: scyllav1alpha1.ScyllaDBClusterSpec{
					Datacenters: []scyllav1alpha1.ScyllaDBClusterDatacenter{
						{
							Name:                        "dc1",
							RemoteKubernetesClusterName: "dc1-rkc",
						},
					},
				},
			},
			datacenter: &scyllav1alpha1.ScyllaDBClusterDatacenter{
				Name:                        "dc1",
				RemoteKubernetesClusterName: "dc1-rkc",
			},
			remoteNamespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "scylla-28t03",
				},
			},
			remoteController: &scyllav1alpha1.RemoteOwner{
				ObjectMeta: metav1.ObjectMeta{
					Name: "remote-owner",
				},
			},
			expectedServices: nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			services := MakeRemoteServices(
				tc.cluster,
				tc.datacenter,
				tc.remoteNamespace,
				tc.remoteController,
				testClusterDomain,
			)

			if !equality.Semantic.DeepEqual(services, tc.expectedServices) {
				t.Errorf("expected services %v, got %v", tc.expectedServices, services)
			}
		})
	}
}

func TestMakeScyllaDBDatacenters(t *testing.T) {
	t.Parallel()

	makePlacement := func(uniqueValue string) *scyllav1alpha1.Placement {
		return &scyllav1alpha1.Placement{
			NodeAffinity: &corev1.NodeAffinity{
				PreferredDuringSchedulingIgnoredDuringExecution: []corev1.PreferredSchedulingTerm{
					{
						Weight: 111,
						Preference: corev1.NodeSelectorTerm{
							MatchFields: []corev1.NodeSelectorRequirement{
								{
									Key:      uniqueValue,
									Operator: corev1.NodeSelectorOpIn,
									Values:   []string{uniqueValue},
								},
							},
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{
									Key:      uniqueValue,
									Operator: corev1.NodeSelectorOpIn,
									Values:   []string{uniqueValue},
								},
							},
						},
					},
				},
				RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchFields: []corev1.NodeSelectorRequirement{
								{
									Key:      uniqueValue,
									Operator: corev1.NodeSelectorOpIn,
									Values:   []string{uniqueValue},
								},
							},
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{
									Key:      uniqueValue,
									Operator: corev1.NodeSelectorOpIn,
									Values:   []string{uniqueValue},
								},
							},
						},
					},
				},
			},
			PodAffinity: &corev1.PodAffinity{
				PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
					{
						Weight: 222,
						PodAffinityTerm: corev1.PodAffinityTerm{
							LabelSelector: metav1.SetAsLabelSelector(map[string]string{
								uniqueValue: uniqueValue,
							}),
							Namespaces:  []string{uniqueValue},
							TopologyKey: uniqueValue,
							NamespaceSelector: metav1.SetAsLabelSelector(map[string]string{
								uniqueValue: uniqueValue,
							}),
							MatchLabelKeys:    []string{uniqueValue},
							MismatchLabelKeys: []string{uniqueValue},
						},
					},
				},
				RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
					{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								uniqueValue: uniqueValue,
							},
						},
						TopologyKey: uniqueValue,
					},
				},
			},
			PodAntiAffinity: &corev1.PodAntiAffinity{
				PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
					{
						Weight: 333,
						PodAffinityTerm: corev1.PodAffinityTerm{
							LabelSelector: metav1.SetAsLabelSelector(map[string]string{
								uniqueValue: uniqueValue,
							}),
							Namespaces:  []string{uniqueValue},
							TopologyKey: uniqueValue,
							NamespaceSelector: metav1.SetAsLabelSelector(map[string]string{
								uniqueValue: uniqueValue,
							}),
							MatchLabelKeys:    []string{uniqueValue},
							MismatchLabelKeys: []string{uniqueValue},
						},
					},
				},
				RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
					{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								uniqueValue: uniqueValue,
							},
						},
						TopologyKey: uniqueValue,
					},
				},
			},
			Tolerations: []corev1.Toleration{
				{
					Key:      uniqueValue,
					Operator: corev1.TolerationOpEqual,
					Value:    uniqueValue,
					Effect:   corev1.TaintEffectNoSchedule,
				},
			},
		}
	}

	newBasicOwnerReference := func(namespace string) []metav1.OwnerReference {
		return []metav1.OwnerReference{
			*metav1.NewControllerRef(&scyllav1alpha1.RemoteOwner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-111",
					Namespace: namespace,
					UID:       "1234",
				},
			}, remoteControllerGVK),
		}
	}

	newBasicScyllaDBDatacenter := func(dcName string, namespace string, seeds []string) *scyllav1alpha1.ScyllaDBDatacenter {
		return &scyllav1alpha1.ScyllaDBDatacenter{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("cluster-%s", dcName),
				Namespace: namespace,
				Labels: map[string]string{
					"scylla-operator.scylladb.com/parent-scylladbcluster-datacenter-name": dcName,
					"scylla-operator.scylladb.com/parent-scylladbcluster-name":            "cluster",
					"scylla-operator.scylladb.com/parent-scylladbcluster-namespace":       "scylla",
					"scylla-operator.scylladb.com/managed-by-cluster":                     "test-cluster.local",
					"app.kubernetes.io/managed-by":                                        "remote.scylla-operator.scylladb.com",
				},
				Annotations: map[string]string{
					"internal.scylla-operator.scylladb.com/scylladb-manager-agent-auth-token-override-secret-ref": "cluster-auth-token-2s75a",
				},
				OwnerReferences: newBasicOwnerReference(namespace),
			},
			Spec: scyllav1alpha1.ScyllaDBDatacenterSpec{
				Metadata: &scyllav1alpha1.ObjectTemplateMetadata{
					Labels: map[string]string{
						"scylla-operator.scylladb.com/parent-scylladbcluster-name":      "cluster",
						"scylla-operator.scylladb.com/parent-scylladbcluster-namespace": "scylla",
					},
					Annotations: map[string]string{},
				},
				ClusterName:                             "cluster",
				DatacenterName:                          pointer.Ptr(dcName),
				DisableAutomaticOrphanedNodeReplacement: pointer.Ptr(false),
				ScyllaDB: scyllav1alpha1.ScyllaDB{
					Image:         "repo/scylla:version",
					ExternalSeeds: seeds,
				},
				ScyllaDBManagerAgent: &scyllav1alpha1.ScyllaDBManagerAgent{
					Image: pointer.Ptr("repo/agent:version"),
				},
				ExposeOptions: &scyllav1alpha1.ExposeOptions{
					NodeService: &scyllav1alpha1.NodeServiceTemplate{
						Type: scyllav1alpha1.NodeServiceTypeHeadless,
					},
					BroadcastOptions: &scyllav1alpha1.NodeBroadcastOptions{
						Nodes: scyllav1alpha1.BroadcastOptions{
							Type: scyllav1alpha1.BroadcastAddressTypePodIP,
						},
						Clients: scyllav1alpha1.BroadcastOptions{
							Type: scyllav1alpha1.BroadcastAddressTypePodIP,
						},
					},
				},
				RackTemplate: &scyllav1alpha1.RackTemplate{
					ScyllaDB: &scyllav1alpha1.ScyllaDBTemplate{
						Resources: &corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("2"),
								corev1.ResourceMemory: resource.MustParse("2Gi"),
							},
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("1"),
								corev1.ResourceMemory: resource.MustParse("1Gi"),
							},
						},
						Storage: &scyllav1alpha1.StorageOptions{
							Capacity: "100Gi",
						},
					},
					ScyllaDBManagerAgent: &scyllav1alpha1.ScyllaDBManagerAgentTemplate{
						Resources: &corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("4"),
								corev1.ResourceMemory: resource.MustParse("4Gi"),
							},
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("3"),
								corev1.ResourceMemory: resource.MustParse("3Gi"),
							},
						},
					},
					Nodes: pointer.Ptr[int32](3),
				},
				Racks: []scyllav1alpha1.RackSpec{
					{
						Name: "a",
					},
					{
						Name: "b",
					},
					{
						Name: "c",
					},
				},
			},
		}
	}

	makeReconciledScyllaDBDatacenter := func(sdc *scyllav1alpha1.ScyllaDBDatacenter) *scyllav1alpha1.ScyllaDBDatacenter {
		sdc.Generation = 123
		sdc.Status = scyllav1alpha1.ScyllaDBDatacenterStatus{
			ObservedGeneration: pointer.Ptr[int64](123),
			Conditions: []metav1.Condition{
				{
					Type:               scyllav1alpha1.AvailableCondition,
					Status:             metav1.ConditionTrue,
					ObservedGeneration: 123,
				},
				{
					Type:               scyllav1alpha1.ProgressingCondition,
					Status:             metav1.ConditionFalse,
					ObservedGeneration: 123,
				},
				{
					Type:               scyllav1alpha1.DegradedCondition,
					Status:             metav1.ConditionFalse,
					ObservedGeneration: 123,
				},
			},
		}
		return sdc
	}

	dcFromSpec := func(idx int) func(cluster *scyllav1alpha1.ScyllaDBCluster) *scyllav1alpha1.ScyllaDBClusterDatacenter {
		return func(cluster *scyllav1alpha1.ScyllaDBCluster) *scyllav1alpha1.ScyllaDBClusterDatacenter {
			return pointer.Ptr(cluster.Spec.Datacenters[idx])
		}
	}

	tt := []struct {
		name                        string
		cluster                     *scyllav1alpha1.ScyllaDBCluster
		datacenter                  func(*scyllav1alpha1.ScyllaDBCluster) *scyllav1alpha1.ScyllaDBClusterDatacenter
		remoteScyllaDBDatacenters   map[string]map[string]*scyllav1alpha1.ScyllaDBDatacenter
		remoteNamespace             *corev1.Namespace
		remoteController            metav1.Object
		expectedScyllaDBDatacenters *scyllav1alpha1.ScyllaDBDatacenter
	}{
		{
			name:       "basic single dc cluster",
			cluster:    newBasicScyllaDBCluster(),
			datacenter: dcFromSpec(0),
			remoteNamespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "scylla-abc",
				},
			},
			remoteController: &scyllav1alpha1.RemoteOwner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-111",
					Namespace: "scylla-abc",
					UID:       "1234",
				},
			},
			expectedScyllaDBDatacenters: newBasicScyllaDBDatacenter("dc1", "scylla-abc", []string{}),
		},
		{
			name: "empty seeds when all three DCs are not reconciled",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				cluster := newBasicScyllaDBCluster()
				cluster.Spec.Datacenters = []scyllav1alpha1.ScyllaDBClusterDatacenter{
					{
						Name:                        "dc1",
						RemoteKubernetesClusterName: "dc1-rkc",
					},
					{
						Name:                        "dc2",
						RemoteKubernetesClusterName: "dc2-rkc",
					},
					{
						Name:                        "dc3",
						RemoteKubernetesClusterName: "dc3-rkc",
					},
				}

				return cluster
			}(),
			datacenter: dcFromSpec(0),
			remoteNamespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "scylla-aaa",
				},
			},
			remoteController: &scyllav1alpha1.RemoteOwner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-111",
					Namespace: "scylla-aaa",
					UID:       "1234",
				},
			},
			remoteScyllaDBDatacenters: map[string]map[string]*scyllav1alpha1.ScyllaDBDatacenter{
				"dc1-rkc": {
					"cluster-dc1": newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{}),
				},
				"dc2-rkc": {
					"cluster-dc2": newBasicScyllaDBDatacenter("dc2", "scylla-bbb", []string{}),
				},
				"dc3-rkc": {
					"cluster-dc3": newBasicScyllaDBDatacenter("dc3", "scylla-ccc", []string{}),
				},
			},
			expectedScyllaDBDatacenters: newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{}),
		},
		{
			name: "only first out of three DCs is reconciled, seeds of DC2 should point to DC1",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				cluster := newBasicScyllaDBCluster()
				cluster.Spec.Datacenters = []scyllav1alpha1.ScyllaDBClusterDatacenter{
					{
						Name:                        "dc1",
						RemoteKubernetesClusterName: "dc1-rkc",
					},
					{
						Name:                        "dc2",
						RemoteKubernetesClusterName: "dc2-rkc",
					},
					{
						Name:                        "dc3",
						RemoteKubernetesClusterName: "dc3-rkc",
					},
				}

				return cluster
			}(),
			datacenter: dcFromSpec(1),
			remoteNamespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "scylla-bbb",
				},
			},
			remoteController: &scyllav1alpha1.RemoteOwner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-111",
					Namespace: "scylla-bbb",
					UID:       "1234",
				},
			},
			remoteScyllaDBDatacenters: map[string]map[string]*scyllav1alpha1.ScyllaDBDatacenter{
				"dc1-rkc": {
					"cluster-dc1": makeReconciledScyllaDBDatacenter(newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{})),
				},
				"dc2-rkc": {
					"cluster-dc2": newBasicScyllaDBDatacenter("dc2", "scylla-bbb", []string{}),
				},
				"dc3-rkc": {
					"cluster-dc3": newBasicScyllaDBDatacenter("dc3", "scylla-ccc", []string{}),
				},
			},
			expectedScyllaDBDatacenters: newBasicScyllaDBDatacenter("dc2", "scylla-bbb", []string{"cluster-dc1-seed.scylla-bbb.svc"}),
		},
		{
			name: "first two out of three DCs are reconciled, seeds of non-reconciled DC should point to reconciled ones",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				cluster := newBasicScyllaDBCluster()
				cluster.Spec.Datacenters = []scyllav1alpha1.ScyllaDBClusterDatacenter{
					{
						Name:                        "dc1",
						RemoteKubernetesClusterName: "dc1-rkc",
					},
					{
						Name:                        "dc2",
						RemoteKubernetesClusterName: "dc2-rkc",
					},
					{
						Name:                        "dc3",
						RemoteKubernetesClusterName: "dc3-rkc",
					},
				}

				return cluster
			}(),
			datacenter: dcFromSpec(2),
			remoteNamespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "scylla-ccc",
				},
			},
			remoteController: &scyllav1alpha1.RemoteOwner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-111",
					Namespace: "scylla-ccc",
					UID:       "1234",
				},
			},
			remoteScyllaDBDatacenters: map[string]map[string]*scyllav1alpha1.ScyllaDBDatacenter{
				"dc1-rkc": {
					"cluster-dc1": makeReconciledScyllaDBDatacenter(newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{})),
				},
				"dc2-rkc": {
					"cluster-dc2": makeReconciledScyllaDBDatacenter(newBasicScyllaDBDatacenter("dc2", "scylla-bbb", []string{})),
				},
				"dc3-rkc": {
					"cluster-dc3": newBasicScyllaDBDatacenter("dc3", "scylla-ccc", []string{}),
				},
			},
			expectedScyllaDBDatacenters: newBasicScyllaDBDatacenter("dc3", "scylla-ccc", []string{"cluster-dc1-seed.scylla-ccc.svc", "cluster-dc2-seed.scylla-ccc.svc"}),
		},
		{
			name: "not fully reconciled DC is part of other DC seeds if existing ScyllaDBDatacenters are referencing it",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				cluster := newBasicScyllaDBCluster()
				cluster.Spec.Datacenters = []scyllav1alpha1.ScyllaDBClusterDatacenter{
					{
						Name:                        "dc1",
						RemoteKubernetesClusterName: "dc1-rkc",
					},
					{
						Name:                        "dc2",
						RemoteKubernetesClusterName: "dc2-rkc",
					},
					{
						Name:                        "dc3",
						RemoteKubernetesClusterName: "dc3-rkc",
					},
				}

				return cluster
			}(),
			datacenter: dcFromSpec(0),
			remoteNamespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "scylla-aaa",
				},
			},
			remoteController: &scyllav1alpha1.RemoteOwner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-111",
					Namespace: "scylla-aaa",
					UID:       "1234",
				},
			},
			remoteScyllaDBDatacenters: map[string]map[string]*scyllav1alpha1.ScyllaDBDatacenter{
				"dc1-rkc": {
					"cluster-dc1": makeReconciledScyllaDBDatacenter(newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{"cluster-dc2-seed.scylla-aaa.svc", "cluster-dc3-seed.scylla-aaa.svc"})),
				},
				"dc2-rkc": {
					"cluster-dc2": newBasicScyllaDBDatacenter("dc2", "scylla-bbb", []string{"cluster-dc1-seed.scylla-bbb.svc", "cluster-dc3-seed.scylla-bbb.svc"}),
				},
				"dc3-rkc": {
					"cluster-dc3": makeReconciledScyllaDBDatacenter(newBasicScyllaDBDatacenter("dc3", "scylla-ccc", []string{"cluster-dc1-seed.scylla-ccc.svc", "cluster-dc2-seed.scylla-ccc.svc"})),
				},
			},
			expectedScyllaDBDatacenters: newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{"cluster-dc2-seed.scylla-aaa.svc", "cluster-dc3-seed.scylla-aaa.svc"}),
		},
		{
			name:       "metadata from ScyllaDBCluster spec are propagated into ScyllaDBDatacenter object metadata and spec metadata",
			datacenter: dcFromSpec(0),
			remoteNamespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "scylla-aaa",
				},
			},
			remoteController: &scyllav1alpha1.RemoteOwner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-111",
					Namespace: "scylla-aaa",
					UID:       "1234",
				},
			},
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				cluster := newBasicScyllaDBCluster()
				cluster.Spec.Metadata = &scyllav1alpha1.ObjectTemplateMetadata{
					Labels: map[string]string{
						"label": "foo",
					},
					Annotations: map[string]string{
						"annotation": "foo",
					},
				}
				return cluster
			}(),
			expectedScyllaDBDatacenters: func() *scyllav1alpha1.ScyllaDBDatacenter {
				dc := newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{})
				dc.Labels["label"] = "foo"
				dc.Annotations["annotation"] = "foo"

				dc.Spec.Metadata.Labels["label"] = "foo"
				dc.Spec.Metadata.Annotations["annotation"] = "foo"
				return dc
			}(),
		},
		{
			name:       "metadata from database template overrides one specified on cluster level",
			datacenter: dcFromSpec(0),
			remoteNamespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "scylla-aaa",
				},
			},
			remoteController: &scyllav1alpha1.RemoteOwner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-111",
					Namespace: "scylla-aaa",
					UID:       "1234",
				},
			},
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				cluster := newBasicScyllaDBCluster()
				cluster.Spec.Metadata = &scyllav1alpha1.ObjectTemplateMetadata{
					Labels: map[string]string{
						"label": "foo",
					},
					Annotations: map[string]string{
						"annotation": "foo",
					},
				}
				cluster.Spec.DatacenterTemplate.Metadata = &scyllav1alpha1.ObjectTemplateMetadata{
					Labels: map[string]string{
						"label": "bar",
					},
					Annotations: map[string]string{
						"annotation": "bar",
					},
				}
				return cluster
			}(),
			expectedScyllaDBDatacenters: func() *scyllav1alpha1.ScyllaDBDatacenter {
				dc := newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{})
				dc.Labels["label"] = "bar"
				dc.Annotations["annotation"] = "bar"

				dc.Spec.Metadata.Labels["label"] = "bar"
				dc.Spec.Metadata.Annotations["annotation"] = "bar"
				return dc
			}(),
		},
		{
			name:       "metadata from datacenter spec overrides one specified on cluster and datacenter template level",
			datacenter: dcFromSpec(0),
			remoteNamespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "scylla-aaa",
				},
			},
			remoteController: &scyllav1alpha1.RemoteOwner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-111",
					Namespace: "scylla-aaa",
					UID:       "1234",
				},
			},
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				cluster := newBasicScyllaDBCluster()
				cluster.Spec.Metadata = &scyllav1alpha1.ObjectTemplateMetadata{
					Labels: map[string]string{
						"label": "foo",
					},
					Annotations: map[string]string{
						"annotation": "foo",
					},
				}
				cluster.Spec.DatacenterTemplate.Metadata = &scyllav1alpha1.ObjectTemplateMetadata{
					Labels: map[string]string{
						"label": "bar",
					},
					Annotations: map[string]string{
						"annotation": "bar",
					},
				}
				cluster.Spec.Datacenters[0].Metadata = &scyllav1alpha1.ObjectTemplateMetadata{
					Labels: map[string]string{
						"label": "dar",
					},
					Annotations: map[string]string{
						"annotation": "dar",
					},
				}
				return cluster
			}(),
			expectedScyllaDBDatacenters: func() *scyllav1alpha1.ScyllaDBDatacenter {
				dc := newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{})
				dc.Labels["label"] = "dar"
				dc.Annotations["annotation"] = "dar"

				dc.Spec.Metadata.Labels["label"] = "dar"
				dc.Spec.Metadata.Annotations["annotation"] = "dar"
				return dc
			}(),
		},
		{
			name:       "forceRedeploymentReason on cluster level propagates into ScyllaDBDatacenter",
			datacenter: dcFromSpec(0),
			remoteNamespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "scylla-aaa",
				},
			},
			remoteController: &scyllav1alpha1.RemoteOwner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-111",
					Namespace: "scylla-aaa",
					UID:       "1234",
				},
			},
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				cluster := newBasicScyllaDBCluster()
				cluster.Spec.ForceRedeploymentReason = pointer.Ptr("foo")
				return cluster
			}(),
			expectedScyllaDBDatacenters: func() *scyllav1alpha1.ScyllaDBDatacenter {
				dc := newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{})
				dc.Spec.ForceRedeploymentReason = pointer.Ptr("foo")
				return dc
			}(),
		},
		{
			name: "forceRedeploymentReason on datacenter level is combined with one specified on cluster level",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				cluster := newBasicScyllaDBCluster()
				cluster.Spec.ForceRedeploymentReason = pointer.Ptr("foo")
				cluster.Spec.Datacenters[0].ForceRedeploymentReason = pointer.Ptr("bar")
				return cluster
			}(),
			datacenter: dcFromSpec(0),
			remoteNamespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "scylla-aaa",
				},
			},
			remoteController: &scyllav1alpha1.RemoteOwner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-111",
					Namespace: "scylla-aaa",
					UID:       "1234",
				},
			},
			expectedScyllaDBDatacenters: func() *scyllav1alpha1.ScyllaDBDatacenter {
				dc := newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{})
				dc.Spec.ForceRedeploymentReason = pointer.Ptr("foo,bar")
				return dc
			}(),
		},
		{
			name: "exposeOptions are are propagated into ScyllaDBDatacenter",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				cluster := newBasicScyllaDBCluster()
				cluster.Spec.ExposeOptions = &scyllav1alpha1.ScyllaDBClusterExposeOptions{
					NodeService: &scyllav1alpha1.NodeServiceTemplate{
						ObjectTemplateMetadata: scyllav1alpha1.ObjectTemplateMetadata{
							Labels: map[string]string{
								"label": "foo",
							},
							Annotations: map[string]string{
								"annotation": "foo",
							},
						},
						Type:                          scyllav1alpha1.NodeServiceTypeHeadless,
						ExternalTrafficPolicy:         pointer.Ptr(corev1.ServiceExternalTrafficPolicyCluster),
						AllocateLoadBalancerNodePorts: pointer.Ptr(true),
						LoadBalancerClass:             pointer.Ptr("load-balancer-class"),
						InternalTrafficPolicy:         pointer.Ptr(corev1.ServiceInternalTrafficPolicyCluster),
					},
					BroadcastOptions: &scyllav1alpha1.ScyllaDBClusterNodeBroadcastOptions{
						Nodes: scyllav1alpha1.BroadcastOptions{
							Type: scyllav1alpha1.BroadcastAddressTypePodIP,
							PodIP: &scyllav1alpha1.PodIPAddressOptions{
								Source: scyllav1alpha1.StatusPodIPSource,
							},
						},
						Clients: scyllav1alpha1.BroadcastOptions{
							Type: scyllav1alpha1.BroadcastAddressTypeServiceLoadBalancerIngress,
							PodIP: &scyllav1alpha1.PodIPAddressOptions{
								Source: scyllav1alpha1.StatusPodIPSource,
							},
						},
					},
				}
				return cluster
			}(),
			datacenter: dcFromSpec(0),
			remoteNamespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "scylla-aaa",
				},
			},
			remoteController: &scyllav1alpha1.RemoteOwner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-111",
					Namespace: "scylla-aaa",
					UID:       "1234",
				},
			},
			expectedScyllaDBDatacenters: func() *scyllav1alpha1.ScyllaDBDatacenter {
				dc := newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{})
				dc.Spec.ExposeOptions = &scyllav1alpha1.ExposeOptions{
					NodeService: &scyllav1alpha1.NodeServiceTemplate{
						ObjectTemplateMetadata: scyllav1alpha1.ObjectTemplateMetadata{
							Labels: map[string]string{
								"label": "foo",
							},
							Annotations: map[string]string{
								"annotation": "foo",
							},
						},
						Type:                          scyllav1alpha1.NodeServiceTypeHeadless,
						ExternalTrafficPolicy:         pointer.Ptr(corev1.ServiceExternalTrafficPolicyCluster),
						AllocateLoadBalancerNodePorts: pointer.Ptr(true),
						LoadBalancerClass:             pointer.Ptr("load-balancer-class"),
						InternalTrafficPolicy:         pointer.Ptr(corev1.ServiceInternalTrafficPolicyCluster),
					},
					BroadcastOptions: &scyllav1alpha1.NodeBroadcastOptions{
						Nodes: scyllav1alpha1.BroadcastOptions{
							Type: scyllav1alpha1.BroadcastAddressTypePodIP,
							PodIP: &scyllav1alpha1.PodIPAddressOptions{
								Source: scyllav1alpha1.StatusPodIPSource,
							},
						},
						Clients: scyllav1alpha1.BroadcastOptions{
							Type: scyllav1alpha1.BroadcastAddressTypeServiceLoadBalancerIngress,
							PodIP: &scyllav1alpha1.PodIPAddressOptions{
								Source: scyllav1alpha1.StatusPodIPSource,
							},
						},
					},
				}
				return dc
			}(),
		},
		{
			name: "disableAutomaticOrphanedNodeReplacement is taken from cluster level",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				cluster := newBasicScyllaDBCluster()
				cluster.Spec.DisableAutomaticOrphanedNodeReplacement = true
				return cluster
			}(),
			datacenter: dcFromSpec(0),
			remoteNamespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "scylla-aaa",
				},
			},
			remoteController: &scyllav1alpha1.RemoteOwner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-111",
					Namespace: "scylla-aaa",
					UID:       "1234",
				},
			},
			expectedScyllaDBDatacenters: func() *scyllav1alpha1.ScyllaDBDatacenter {
				dc := newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{})
				dc.Spec.DisableAutomaticOrphanedNodeReplacement = pointer.Ptr(true)
				return dc
			}(),
		},
		{
			name: "minTerminationGracePeriodSeconds is taken from cluster level",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				cluster := newBasicScyllaDBCluster()
				cluster.Spec.MinTerminationGracePeriodSeconds = pointer.Ptr[int32](123)
				return cluster
			}(),
			datacenter: dcFromSpec(0),
			remoteNamespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "scylla-aaa",
				},
			},
			remoteController: &scyllav1alpha1.RemoteOwner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-111",
					Namespace: "scylla-aaa",
					UID:       "1234",
				},
			},
			expectedScyllaDBDatacenters: func() *scyllav1alpha1.ScyllaDBDatacenter {
				dc := newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{})
				dc.Spec.MinTerminationGracePeriodSeconds = pointer.Ptr[int32](123)
				return dc
			}(),
		},
		{
			name: "minReadySeconds is taken from cluster level",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				cluster := newBasicScyllaDBCluster()
				cluster.Spec.MinReadySeconds = pointer.Ptr[int32](123)
				return cluster
			}(),
			datacenter: dcFromSpec(0),
			remoteNamespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "scylla-aaa",
				},
			},
			remoteController: &scyllav1alpha1.RemoteOwner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-111",
					Namespace: "scylla-aaa",
					UID:       "1234",
				},
			},
			expectedScyllaDBDatacenters: func() *scyllav1alpha1.ScyllaDBDatacenter {
				dc := newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{})
				dc.Spec.MinReadySeconds = pointer.Ptr[int32](123)
				return dc
			}(),
		},
		{
			name: "readinessGates is taken from cluster level",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				cluster := newBasicScyllaDBCluster()
				cluster.Spec.ReadinessGates = []corev1.PodReadinessGate{
					{
						ConditionType: "foo",
					},
				}
				return cluster
			}(),
			datacenter: dcFromSpec(0),
			remoteNamespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "scylla-aaa",
				},
			},
			remoteController: &scyllav1alpha1.RemoteOwner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-111",
					Namespace: "scylla-aaa",
					UID:       "1234",
				},
			},
			expectedScyllaDBDatacenters: func() *scyllav1alpha1.ScyllaDBDatacenter {
				dc := newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{})
				dc.Spec.ReadinessGates = []corev1.PodReadinessGate{
					{
						ConditionType: "foo",
					},
				}
				return dc
			}(),
		},
		{
			name: "nodes in rack template in datacenter spec overrides ones specified in datacenter template",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				cluster := newBasicScyllaDBCluster()
				cluster.Spec.DatacenterTemplate.RackTemplate = &scyllav1alpha1.RackTemplate{
					Nodes: pointer.Ptr[int32](123),
				}
				cluster.Spec.Datacenters[0].RackTemplate = &scyllav1alpha1.RackTemplate{
					Nodes: pointer.Ptr[int32](321),
				}

				return cluster
			}(),
			datacenter: dcFromSpec(0),
			remoteNamespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "scylla-aaa",
				},
			},
			remoteController: &scyllav1alpha1.RemoteOwner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-111",
					Namespace: "scylla-aaa",
					UID:       "1234",
				},
			},
			expectedScyllaDBDatacenters: func() *scyllav1alpha1.ScyllaDBDatacenter {
				dc := newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{})
				dc.Spec.RackTemplate.Nodes = pointer.Ptr[int32](321)
				return dc
			}(),
		},
		{
			name: "topologyLabelSelector is merged from ones specified on each level",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				cluster := newBasicScyllaDBCluster()
				cluster.Spec.DatacenterTemplate.RackTemplate.TopologyLabelSelector = map[string]string{
					"dcTemplateRackTemplate": "foo",
				}
				cluster.Spec.DatacenterTemplate.TopologyLabelSelector = map[string]string{
					"dcTemplate": "foo",
				}
				cluster.Spec.Datacenters[0].TopologyLabelSelector = map[string]string{
					"dc": "foo",
				}
				cluster.Spec.Datacenters[0].RackTemplate = &scyllav1alpha1.RackTemplate{
					TopologyLabelSelector: map[string]string{
						"dcRackTemplate": "foo",
					},
				}

				return cluster
			}(),
			datacenter: dcFromSpec(0),
			remoteNamespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "scylla-aaa",
				},
			},
			remoteController: &scyllav1alpha1.RemoteOwner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-111",
					Namespace: "scylla-aaa",
					UID:       "1234",
				},
			},
			expectedScyllaDBDatacenters: func() *scyllav1alpha1.ScyllaDBDatacenter {
				dc := newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{})
				dc.Spec.RackTemplate.TopologyLabelSelector = map[string]string{
					"dcTemplateRackTemplate": "foo",
					"dc":                     "foo",
					"dcTemplate":             "foo",
					"dcRackTemplate":         "foo",
				}
				return dc
			}(),
		},
		{
			name: "collision on topologyLabelSelector key, datacenter rackTemplate takes precedence over all others",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				cluster := newBasicScyllaDBCluster()
				cluster.Spec.DatacenterTemplate.RackTemplate.TopologyLabelSelector = map[string]string{
					"foo": "dcTemplateRackTemplate",
				}
				cluster.Spec.DatacenterTemplate.TopologyLabelSelector = map[string]string{
					"foo": "dcTemplate",
				}
				cluster.Spec.Datacenters[0].TopologyLabelSelector = map[string]string{
					"foo": "dc",
				}
				cluster.Spec.Datacenters[0].RackTemplate = &scyllav1alpha1.RackTemplate{
					TopologyLabelSelector: map[string]string{
						"foo": "dcRackTemplate",
					},
				}

				return cluster
			}(),
			datacenter: dcFromSpec(0),
			remoteNamespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "scylla-aaa",
				},
			},
			remoteController: &scyllav1alpha1.RemoteOwner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-111",
					Namespace: "scylla-aaa",
					UID:       "1234",
				},
			},
			expectedScyllaDBDatacenters: func() *scyllav1alpha1.ScyllaDBDatacenter {
				dc := newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{})
				dc.Spec.RackTemplate.TopologyLabelSelector = map[string]string{
					"foo": "dcRackTemplate",
				}
				return dc
			}(),
		},
		{
			name: "collision on topologyLabelSelector key, datacenter takes precedence when datacenter rackTemplate is missing",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				cluster := newBasicScyllaDBCluster()
				cluster.Spec.DatacenterTemplate.RackTemplate.TopologyLabelSelector = map[string]string{
					"foo": "dcTemplateRackTemplate",
				}
				cluster.Spec.DatacenterTemplate.TopologyLabelSelector = map[string]string{
					"foo": "dcTemplate",
				}
				cluster.Spec.Datacenters[0].TopologyLabelSelector = map[string]string{
					"foo": "dc",
				}

				return cluster
			}(),
			datacenter: dcFromSpec(0),
			remoteNamespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "scylla-aaa",
				},
			},
			remoteController: &scyllav1alpha1.RemoteOwner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-111",
					Namespace: "scylla-aaa",
					UID:       "1234",
				},
			},
			expectedScyllaDBDatacenters: func() *scyllav1alpha1.ScyllaDBDatacenter {
				dc := newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{})
				dc.Spec.RackTemplate.TopologyLabelSelector = map[string]string{
					"foo": "dc",
				}
				return dc
			}(),
		},
		{
			name: "in case of collision on topologyLabelSelector key, datacenter template takes precedence when datacenter and datacenter rackTemplate is missing",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				cluster := newBasicScyllaDBCluster()
				cluster.Spec.DatacenterTemplate.RackTemplate.TopologyLabelSelector = map[string]string{
					"foo": "dcTemplateRackTemplate",
				}
				cluster.Spec.DatacenterTemplate.TopologyLabelSelector = map[string]string{
					"foo": "dcTemplate",
				}

				return cluster
			}(),
			datacenter: dcFromSpec(0),
			remoteNamespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "scylla-aaa",
				},
			},
			remoteController: &scyllav1alpha1.RemoteOwner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-111",
					Namespace: "scylla-aaa",
					UID:       "1234",
				},
			},
			expectedScyllaDBDatacenters: func() *scyllav1alpha1.ScyllaDBDatacenter {
				dc := newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{})
				dc.Spec.RackTemplate.TopologyLabelSelector = map[string]string{
					"foo": "dcTemplate",
				}
				return dc
			}(),
		},
		{
			name: "rackTemplate placement is merged from all levels",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				cluster := newBasicScyllaDBCluster()
				cluster.Spec.DatacenterTemplate.Placement = makePlacement("foo")
				cluster.Spec.DatacenterTemplate.RackTemplate.Placement = makePlacement("bar")
				cluster.Spec.Datacenters[0].Placement = makePlacement("dar")
				cluster.Spec.Datacenters[0].RackTemplate = &scyllav1alpha1.RackTemplate{
					Placement: makePlacement("zar"),
				}

				return cluster
			}(),
			datacenter: dcFromSpec(0),
			remoteNamespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "scylla-aaa",
				},
			},
			remoteController: &scyllav1alpha1.RemoteOwner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-111",
					Namespace: "scylla-aaa",
					UID:       "1234",
				},
			},
			expectedScyllaDBDatacenters: func() *scyllav1alpha1.ScyllaDBDatacenter {
				dc := newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{})
				dc.Spec.RackTemplate.Placement = &scyllav1alpha1.Placement{
					NodeAffinity: &corev1.NodeAffinity{
						PreferredDuringSchedulingIgnoredDuringExecution: []corev1.PreferredSchedulingTerm{
							{
								Weight: 111,
								Preference: corev1.NodeSelectorTerm{
									MatchFields: []corev1.NodeSelectorRequirement{
										{
											Key:      "foo",
											Operator: corev1.NodeSelectorOpIn,
											Values:   []string{"foo"},
										},
									},
									MatchExpressions: []corev1.NodeSelectorRequirement{
										{
											Key:      "foo",
											Operator: corev1.NodeSelectorOpIn,
											Values:   []string{"foo"},
										},
									},
								},
							},
							{
								Weight: 111,
								Preference: corev1.NodeSelectorTerm{
									MatchFields: []corev1.NodeSelectorRequirement{
										{
											Key:      "bar",
											Operator: corev1.NodeSelectorOpIn,
											Values:   []string{"bar"},
										},
									},
									MatchExpressions: []corev1.NodeSelectorRequirement{
										{
											Key:      "bar",
											Operator: corev1.NodeSelectorOpIn,
											Values:   []string{"bar"},
										},
									},
								},
							},
							{
								Weight: 111,
								Preference: corev1.NodeSelectorTerm{
									MatchFields: []corev1.NodeSelectorRequirement{
										{
											Key:      "dar",
											Operator: corev1.NodeSelectorOpIn,
											Values:   []string{"dar"},
										},
									},
									MatchExpressions: []corev1.NodeSelectorRequirement{
										{
											Key:      "dar",
											Operator: corev1.NodeSelectorOpIn,
											Values:   []string{"dar"},
										},
									},
								},
							},
							{
								Weight: 111,
								Preference: corev1.NodeSelectorTerm{
									MatchFields: []corev1.NodeSelectorRequirement{
										{
											Key:      "zar",
											Operator: corev1.NodeSelectorOpIn,
											Values:   []string{"zar"},
										},
									},
									MatchExpressions: []corev1.NodeSelectorRequirement{
										{
											Key:      "zar",
											Operator: corev1.NodeSelectorOpIn,
											Values:   []string{"zar"},
										},
									},
								},
							},
						},
						RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
							NodeSelectorTerms: []corev1.NodeSelectorTerm{
								{
									MatchFields: []corev1.NodeSelectorRequirement{
										{
											Key:      "foo",
											Operator: corev1.NodeSelectorOpIn,
											Values:   []string{"foo"},
										},
									},
									MatchExpressions: []corev1.NodeSelectorRequirement{
										{
											Key:      "foo",
											Operator: corev1.NodeSelectorOpIn,
											Values:   []string{"foo"},
										},
									},
								},
								{
									MatchFields: []corev1.NodeSelectorRequirement{
										{
											Key:      "bar",
											Operator: corev1.NodeSelectorOpIn,
											Values:   []string{"bar"},
										},
									},
									MatchExpressions: []corev1.NodeSelectorRequirement{
										{
											Key:      "bar",
											Operator: corev1.NodeSelectorOpIn,
											Values:   []string{"bar"},
										},
									},
								},
								{
									MatchFields: []corev1.NodeSelectorRequirement{
										{
											Key:      "dar",
											Operator: corev1.NodeSelectorOpIn,
											Values:   []string{"dar"},
										},
									},
									MatchExpressions: []corev1.NodeSelectorRequirement{
										{
											Key:      "dar",
											Operator: corev1.NodeSelectorOpIn,
											Values:   []string{"dar"},
										},
									},
								},
								{
									MatchFields: []corev1.NodeSelectorRequirement{
										{
											Key:      "zar",
											Operator: corev1.NodeSelectorOpIn,
											Values:   []string{"zar"},
										},
									},
									MatchExpressions: []corev1.NodeSelectorRequirement{
										{
											Key:      "zar",
											Operator: corev1.NodeSelectorOpIn,
											Values:   []string{"zar"},
										},
									},
								},
							},
						},
					},
					PodAffinity: &corev1.PodAffinity{
						PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
							{
								Weight: 222,
								PodAffinityTerm: corev1.PodAffinityTerm{
									LabelSelector: metav1.SetAsLabelSelector(map[string]string{
										"foo": "foo",
									}),
									Namespaces:  []string{"foo"},
									TopologyKey: "foo",
									NamespaceSelector: metav1.SetAsLabelSelector(map[string]string{
										"foo": "foo",
									}),
									MatchLabelKeys:    []string{"foo"},
									MismatchLabelKeys: []string{"foo"},
								},
							},
							{
								Weight: 222,
								PodAffinityTerm: corev1.PodAffinityTerm{
									LabelSelector: metav1.SetAsLabelSelector(map[string]string{
										"bar": "bar",
									}),
									Namespaces:  []string{"bar"},
									TopologyKey: "bar",
									NamespaceSelector: metav1.SetAsLabelSelector(map[string]string{
										"bar": "bar",
									}),
									MatchLabelKeys:    []string{"bar"},
									MismatchLabelKeys: []string{"bar"},
								},
							},
							{
								Weight: 222,
								PodAffinityTerm: corev1.PodAffinityTerm{
									LabelSelector: metav1.SetAsLabelSelector(map[string]string{
										"dar": "dar",
									}),
									Namespaces:  []string{"dar"},
									TopologyKey: "dar",
									NamespaceSelector: metav1.SetAsLabelSelector(map[string]string{
										"dar": "dar",
									}),
									MatchLabelKeys:    []string{"dar"},
									MismatchLabelKeys: []string{"dar"},
								},
							},
							{
								Weight: 222,
								PodAffinityTerm: corev1.PodAffinityTerm{
									LabelSelector: metav1.SetAsLabelSelector(map[string]string{
										"zar": "zar",
									}),
									Namespaces:  []string{"zar"},
									TopologyKey: "zar",
									NamespaceSelector: metav1.SetAsLabelSelector(map[string]string{
										"zar": "zar",
									}),
									MatchLabelKeys:    []string{"zar"},
									MismatchLabelKeys: []string{"zar"},
								},
							},
						},
						RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
							{
								LabelSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										"foo": "foo",
									},
								},
								TopologyKey: "foo",
							},
							{
								LabelSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										"bar": "bar",
									},
								},
								TopologyKey: "bar",
							},
							{
								LabelSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										"dar": "dar",
									},
								},
								TopologyKey: "dar",
							},
							{
								LabelSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										"zar": "zar",
									},
								},
								TopologyKey: "zar",
							},
						},
					},
					PodAntiAffinity: &corev1.PodAntiAffinity{
						PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
							{
								Weight: 333,
								PodAffinityTerm: corev1.PodAffinityTerm{
									LabelSelector: metav1.SetAsLabelSelector(map[string]string{
										"foo": "foo",
									}),
									Namespaces:  []string{"foo"},
									TopologyKey: "foo",
									NamespaceSelector: metav1.SetAsLabelSelector(map[string]string{
										"foo": "foo",
									}),
									MatchLabelKeys:    []string{"foo"},
									MismatchLabelKeys: []string{"foo"},
								},
							},
							{
								Weight: 333,
								PodAffinityTerm: corev1.PodAffinityTerm{
									LabelSelector: metav1.SetAsLabelSelector(map[string]string{
										"bar": "bar",
									}),
									Namespaces:  []string{"bar"},
									TopologyKey: "bar",
									NamespaceSelector: metav1.SetAsLabelSelector(map[string]string{
										"bar": "bar",
									}),
									MatchLabelKeys:    []string{"bar"},
									MismatchLabelKeys: []string{"bar"},
								},
							},
							{
								Weight: 333,
								PodAffinityTerm: corev1.PodAffinityTerm{
									LabelSelector: metav1.SetAsLabelSelector(map[string]string{
										"dar": "dar",
									}),
									Namespaces:  []string{"dar"},
									TopologyKey: "dar",
									NamespaceSelector: metav1.SetAsLabelSelector(map[string]string{
										"dar": "dar",
									}),
									MatchLabelKeys:    []string{"dar"},
									MismatchLabelKeys: []string{"dar"},
								},
							},
							{
								Weight: 333,
								PodAffinityTerm: corev1.PodAffinityTerm{
									LabelSelector: metav1.SetAsLabelSelector(map[string]string{
										"zar": "zar",
									}),
									Namespaces:  []string{"zar"},
									TopologyKey: "zar",
									NamespaceSelector: metav1.SetAsLabelSelector(map[string]string{
										"zar": "zar",
									}),
									MatchLabelKeys:    []string{"zar"},
									MismatchLabelKeys: []string{"zar"},
								},
							},
						},
						RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
							{
								LabelSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										"foo": "foo",
									},
								},
								TopologyKey: "foo",
							},
							{
								LabelSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										"bar": "bar",
									},
								},
								TopologyKey: "bar",
							},
							{
								LabelSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										"dar": "dar",
									},
								},
								TopologyKey: "dar",
							},
							{
								LabelSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										"zar": "zar",
									},
								},
								TopologyKey: "zar",
							},
						},
					},
					Tolerations: []corev1.Toleration{
						{
							Key:      "foo",
							Operator: corev1.TolerationOpEqual,
							Value:    "foo",
							Effect:   corev1.TaintEffectNoSchedule,
						},
						{
							Key:      "bar",
							Operator: corev1.TolerationOpEqual,
							Value:    "bar",
							Effect:   corev1.TaintEffectNoSchedule,
						},
						{
							Key:      "dar",
							Operator: corev1.TolerationOpEqual,
							Value:    "dar",
							Effect:   corev1.TaintEffectNoSchedule,
						},
						{
							Key:      "zar",
							Operator: corev1.TolerationOpEqual,
							Value:    "zar",
							Effect:   corev1.TaintEffectNoSchedule,
						},
					},
				}
				return dc
			}(),
		},
		{
			name:       "storage metadata is merged from all levels",
			datacenter: dcFromSpec(0),
			remoteNamespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "scylla-aaa",
				},
			},
			remoteController: &scyllav1alpha1.RemoteOwner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-111",
					Namespace: "scylla-aaa",
					UID:       "1234",
				},
			},
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				cluster := newBasicScyllaDBCluster()
				cluster.Spec.DatacenterTemplate.ScyllaDB.Storage = &scyllav1alpha1.StorageOptions{
					Metadata: &scyllav1alpha1.ObjectTemplateMetadata{
						Labels: map[string]string{
							"foo": "foo",
						},
						Annotations: map[string]string{
							"foo": "foo",
						},
					},
				}
				cluster.Spec.DatacenterTemplate.RackTemplate.ScyllaDB = &scyllav1alpha1.ScyllaDBTemplate{
					Storage: &scyllav1alpha1.StorageOptions{
						Metadata: &scyllav1alpha1.ObjectTemplateMetadata{
							Labels: map[string]string{
								"bar": "bar",
							},
							Annotations: map[string]string{
								"bar": "bar",
							},
						},
					},
				}
				cluster.Spec.Datacenters[0].ScyllaDB = &scyllav1alpha1.ScyllaDBTemplate{
					Storage: &scyllav1alpha1.StorageOptions{
						Metadata: &scyllav1alpha1.ObjectTemplateMetadata{
							Labels: map[string]string{
								"dar": "dar",
							},
							Annotations: map[string]string{
								"dar": "dar",
							},
						},
					},
				}
				cluster.Spec.Datacenters[0].RackTemplate = &scyllav1alpha1.RackTemplate{
					ScyllaDB: &scyllav1alpha1.ScyllaDBTemplate{
						Storage: &scyllav1alpha1.StorageOptions{
							Metadata: &scyllav1alpha1.ObjectTemplateMetadata{
								Labels: map[string]string{
									"zar": "zar",
								},
								Annotations: map[string]string{
									"zar": "zar",
								},
							},
						},
					},
				}
				return cluster
			}(),
			expectedScyllaDBDatacenters: func() *scyllav1alpha1.ScyllaDBDatacenter {
				dc := newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{})
				dc.Spec.RackTemplate.ScyllaDB.Storage = &scyllav1alpha1.StorageOptions{
					Metadata: &scyllav1alpha1.ObjectTemplateMetadata{
						Labels: map[string]string{
							"foo": "foo",
							"bar": "bar",
							"dar": "dar",
							"zar": "zar",
						},
						Annotations: map[string]string{
							"foo": "foo",
							"bar": "bar",
							"dar": "dar",
							"zar": "zar",
						},
					},
				}
				return dc
			}(),
		},
		{
			name:       "rack template capacity is taken from datacenter level when provided",
			datacenter: dcFromSpec(0),
			remoteNamespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "scylla-aaa",
				},
			},
			remoteController: &scyllav1alpha1.RemoteOwner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-111",
					Namespace: "scylla-aaa",
					UID:       "1234",
				},
			},
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				cluster := newBasicScyllaDBCluster()
				cluster.Spec.DatacenterTemplate.ScyllaDB.Storage = &scyllav1alpha1.StorageOptions{
					Capacity: "1",
				}
				cluster.Spec.DatacenterTemplate.RackTemplate.ScyllaDB = &scyllav1alpha1.ScyllaDBTemplate{
					Storage: &scyllav1alpha1.StorageOptions{
						Capacity: "2",
					},
				}
				cluster.Spec.Datacenters[0].ScyllaDB = &scyllav1alpha1.ScyllaDBTemplate{
					Storage: &scyllav1alpha1.StorageOptions{
						Capacity: "3",
					},
				}
				cluster.Spec.Datacenters[0].RackTemplate = &scyllav1alpha1.RackTemplate{
					ScyllaDB: &scyllav1alpha1.ScyllaDBTemplate{
						Storage: &scyllav1alpha1.StorageOptions{
							Capacity: "4",
						},
					},
				}
				return cluster
			}(),
			expectedScyllaDBDatacenters: func() *scyllav1alpha1.ScyllaDBDatacenter {
				dc := newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{})
				dc.Spec.RackTemplate.ScyllaDB.Storage = &scyllav1alpha1.StorageOptions{
					Capacity: "4",
				}
				return dc
			}(),
		},
		{
			name:       "rack template capacity is taken from datacenter level when provided and datacenter rack template is missing",
			datacenter: dcFromSpec(0),
			remoteNamespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "scylla-aaa",
				},
			},
			remoteController: &scyllav1alpha1.RemoteOwner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-111",
					Namespace: "scylla-aaa",
					UID:       "1234",
				},
			},
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				cluster := newBasicScyllaDBCluster()
				cluster.Spec.DatacenterTemplate.ScyllaDB.Storage = &scyllav1alpha1.StorageOptions{
					Capacity: "1",
				}
				cluster.Spec.DatacenterTemplate.RackTemplate.ScyllaDB = &scyllav1alpha1.ScyllaDBTemplate{
					Storage: &scyllav1alpha1.StorageOptions{
						Capacity: "2",
					},
				}
				cluster.Spec.Datacenters[0].ScyllaDB = &scyllav1alpha1.ScyllaDBTemplate{
					Storage: &scyllav1alpha1.StorageOptions{
						Capacity: "3",
					},
				}
				return cluster
			}(),
			expectedScyllaDBDatacenters: func() *scyllav1alpha1.ScyllaDBDatacenter {
				dc := newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{})
				dc.Spec.RackTemplate.ScyllaDB.Storage = &scyllav1alpha1.StorageOptions{
					Capacity: "3",
				}
				return dc
			}(),
		},
		{
			name:       "rack template capacity is taken from datacenter template rack template level when provided and datacenter spec and datacenter rack template is missing",
			datacenter: dcFromSpec(0),
			remoteNamespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "scylla-aaa",
				},
			},
			remoteController: &scyllav1alpha1.RemoteOwner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-111",
					Namespace: "scylla-aaa",
					UID:       "1234",
				},
			},
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				cluster := newBasicScyllaDBCluster()
				cluster.Spec.DatacenterTemplate.ScyllaDB.Storage = &scyllav1alpha1.StorageOptions{
					Capacity: "1",
				}
				cluster.Spec.DatacenterTemplate.RackTemplate.ScyllaDB = &scyllav1alpha1.ScyllaDBTemplate{
					Storage: &scyllav1alpha1.StorageOptions{
						Capacity: "2",
					},
				}
				return cluster
			}(),
			expectedScyllaDBDatacenters: func() *scyllav1alpha1.ScyllaDBDatacenter {
				dc := newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{})
				dc.Spec.RackTemplate.ScyllaDB.Storage = &scyllav1alpha1.StorageOptions{
					Capacity: "2",
				}
				return dc
			}(),
		},
		{
			name:       "rack template capacity is taken from datacenter template level when provided all other are not provided",
			datacenter: dcFromSpec(0),
			remoteNamespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "scylla-aaa",
				},
			},
			remoteController: &scyllav1alpha1.RemoteOwner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-111",
					Namespace: "scylla-aaa",
					UID:       "1234",
				},
			},
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				cluster := newBasicScyllaDBCluster()
				cluster.Spec.DatacenterTemplate.ScyllaDB.Storage = &scyllav1alpha1.StorageOptions{
					Capacity: "1",
				}
				return cluster
			}(),
			expectedScyllaDBDatacenters: func() *scyllav1alpha1.ScyllaDBDatacenter {
				dc := newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{})
				dc.Spec.RackTemplate.ScyllaDB.Storage = &scyllav1alpha1.StorageOptions{
					Capacity: "1",
				}
				return dc
			}(),
		},
		{
			name:       "rack template storageClassName is taken from datacenter level when provided",
			datacenter: dcFromSpec(0),
			remoteNamespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "scylla-aaa",
				},
			},
			remoteController: &scyllav1alpha1.RemoteOwner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-111",
					Namespace: "scylla-aaa",
					UID:       "1234",
				},
			},
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				cluster := newBasicScyllaDBCluster()
				cluster.Spec.DatacenterTemplate.ScyllaDB.Storage = &scyllav1alpha1.StorageOptions{
					StorageClassName: pointer.Ptr("a"),
				}
				cluster.Spec.DatacenterTemplate.RackTemplate.ScyllaDB = &scyllav1alpha1.ScyllaDBTemplate{
					Storage: &scyllav1alpha1.StorageOptions{
						StorageClassName: pointer.Ptr("b"),
					},
				}
				cluster.Spec.Datacenters[0].ScyllaDB = &scyllav1alpha1.ScyllaDBTemplate{
					Storage: &scyllav1alpha1.StorageOptions{
						StorageClassName: pointer.Ptr("c"),
					},
				}
				cluster.Spec.Datacenters[0].RackTemplate = &scyllav1alpha1.RackTemplate{
					ScyllaDB: &scyllav1alpha1.ScyllaDBTemplate{
						Storage: &scyllav1alpha1.StorageOptions{
							StorageClassName: pointer.Ptr("d"),
						},
					},
				}
				return cluster
			}(),
			expectedScyllaDBDatacenters: func() *scyllav1alpha1.ScyllaDBDatacenter {
				dc := newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{})
				dc.Spec.RackTemplate.ScyllaDB.Storage = &scyllav1alpha1.StorageOptions{
					StorageClassName: pointer.Ptr("d"),
				}
				return dc
			}(),
		},
		{
			name:       "rack template storageClassName is taken from datacenter level when provided and datacenter rack template is missing",
			datacenter: dcFromSpec(0),
			remoteNamespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "scylla-aaa",
				},
			},
			remoteController: &scyllav1alpha1.RemoteOwner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-111",
					Namespace: "scylla-aaa",
					UID:       "1234",
				},
			},
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				cluster := newBasicScyllaDBCluster()
				cluster.Spec.DatacenterTemplate.ScyllaDB.Storage = &scyllav1alpha1.StorageOptions{
					StorageClassName: pointer.Ptr("a"),
				}
				cluster.Spec.DatacenterTemplate.RackTemplate.ScyllaDB = &scyllav1alpha1.ScyllaDBTemplate{
					Storage: &scyllav1alpha1.StorageOptions{
						StorageClassName: pointer.Ptr("b"),
					},
				}
				cluster.Spec.Datacenters[0].ScyllaDB = &scyllav1alpha1.ScyllaDBTemplate{
					Storage: &scyllav1alpha1.StorageOptions{
						StorageClassName: pointer.Ptr("c"),
					},
				}
				return cluster
			}(),
			expectedScyllaDBDatacenters: func() *scyllav1alpha1.ScyllaDBDatacenter {
				dc := newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{})
				dc.Spec.RackTemplate.ScyllaDB.Storage = &scyllav1alpha1.StorageOptions{
					StorageClassName: pointer.Ptr("c"),
				}
				return dc
			}(),
		},
		{
			name:       "rack template storageClassName is taken from datacenter template rack template level when provided and datacenter spec and datacenter rack template is missing",
			datacenter: dcFromSpec(0),
			remoteNamespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "scylla-aaa",
				},
			},
			remoteController: &scyllav1alpha1.RemoteOwner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-111",
					Namespace: "scylla-aaa",
					UID:       "1234",
				},
			},
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				cluster := newBasicScyllaDBCluster()
				cluster.Spec.DatacenterTemplate.ScyllaDB.Storage = &scyllav1alpha1.StorageOptions{
					StorageClassName: pointer.Ptr("a"),
				}
				cluster.Spec.DatacenterTemplate.RackTemplate.ScyllaDB = &scyllav1alpha1.ScyllaDBTemplate{
					Storage: &scyllav1alpha1.StorageOptions{
						StorageClassName: pointer.Ptr("b"),
					},
				}
				return cluster
			}(),
			expectedScyllaDBDatacenters: func() *scyllav1alpha1.ScyllaDBDatacenter {
				dc := newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{})
				dc.Spec.RackTemplate.ScyllaDB.Storage = &scyllav1alpha1.StorageOptions{
					StorageClassName: pointer.Ptr("b"),
				}
				return dc
			}(),
		},
		{
			name:       "rack template storageClassName is taken from datacenter template level when provided all other are not provided",
			datacenter: dcFromSpec(0),
			remoteNamespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "scylla-aaa",
				},
			},
			remoteController: &scyllav1alpha1.RemoteOwner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-111",
					Namespace: "scylla-aaa",
					UID:       "1234",
				},
			},
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				cluster := newBasicScyllaDBCluster()
				cluster.Spec.DatacenterTemplate.ScyllaDB.Storage = &scyllav1alpha1.StorageOptions{
					StorageClassName: pointer.Ptr("a"),
				}
				return cluster
			}(),
			expectedScyllaDBDatacenters: func() *scyllav1alpha1.ScyllaDBDatacenter {
				dc := newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{})
				dc.Spec.RackTemplate.ScyllaDB.Storage = &scyllav1alpha1.StorageOptions{
					StorageClassName: pointer.Ptr("a"),
				}
				return dc
			}(),
		},
		{
			name:       "rack template scylladb custom ConfigMap ref is taken from datacenter level when provided",
			datacenter: dcFromSpec(0),
			remoteNamespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "scylla-aaa",
				},
			},
			remoteController: &scyllav1alpha1.RemoteOwner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-111",
					Namespace: "scylla-aaa",
					UID:       "1234",
				},
			},
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				cluster := newBasicScyllaDBCluster()
				cluster.Spec.DatacenterTemplate.ScyllaDB.CustomConfigMapRef = pointer.Ptr("a")
				cluster.Spec.DatacenterTemplate.RackTemplate.ScyllaDB = &scyllav1alpha1.ScyllaDBTemplate{
					CustomConfigMapRef: pointer.Ptr("b"),
				}
				cluster.Spec.Datacenters[0].ScyllaDB = &scyllav1alpha1.ScyllaDBTemplate{
					CustomConfigMapRef: pointer.Ptr("c"),
				}
				cluster.Spec.Datacenters[0].RackTemplate = &scyllav1alpha1.RackTemplate{
					ScyllaDB: &scyllav1alpha1.ScyllaDBTemplate{
						CustomConfigMapRef: pointer.Ptr("d"),
					},
				}
				return cluster
			}(),
			expectedScyllaDBDatacenters: func() *scyllav1alpha1.ScyllaDBDatacenter {
				dc := newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{})
				dc.Spec.RackTemplate.ScyllaDB.CustomConfigMapRef = pointer.Ptr("d")
				return dc
			}(),
		},
		{
			name:       "rack template scylladb custom ConfigMap ref is taken from datacenter level when provided and datacenter rack template is missing",
			datacenter: dcFromSpec(0),
			remoteNamespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "scylla-aaa",
				},
			},
			remoteController: &scyllav1alpha1.RemoteOwner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-111",
					Namespace: "scylla-aaa",
					UID:       "1234",
				},
			},
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				cluster := newBasicScyllaDBCluster()
				cluster.Spec.DatacenterTemplate.ScyllaDB.CustomConfigMapRef = pointer.Ptr("a")
				cluster.Spec.DatacenterTemplate.RackTemplate.ScyllaDB = &scyllav1alpha1.ScyllaDBTemplate{
					CustomConfigMapRef: pointer.Ptr("b"),
				}
				cluster.Spec.Datacenters[0].ScyllaDB = &scyllav1alpha1.ScyllaDBTemplate{
					CustomConfigMapRef: pointer.Ptr("c"),
				}
				return cluster
			}(),
			expectedScyllaDBDatacenters: func() *scyllav1alpha1.ScyllaDBDatacenter {
				dc := newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{})
				dc.Spec.RackTemplate.ScyllaDB.CustomConfigMapRef = pointer.Ptr("c")
				return dc
			}(),
		},
		{
			name:       "rack template scylladb custom ConfigMap ref is taken from datacenter template rack template level when provided and datacenter spec and datacenter rack template is missing",
			datacenter: dcFromSpec(0),
			remoteNamespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "scylla-aaa",
				},
			},
			remoteController: &scyllav1alpha1.RemoteOwner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-111",
					Namespace: "scylla-aaa",
					UID:       "1234",
				},
			},
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				cluster := newBasicScyllaDBCluster()
				cluster.Spec.DatacenterTemplate.ScyllaDB.CustomConfigMapRef = pointer.Ptr("a")
				cluster.Spec.DatacenterTemplate.RackTemplate.ScyllaDB = &scyllav1alpha1.ScyllaDBTemplate{
					CustomConfigMapRef: pointer.Ptr("b"),
				}
				return cluster
			}(),
			expectedScyllaDBDatacenters: func() *scyllav1alpha1.ScyllaDBDatacenter {
				dc := newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{})
				dc.Spec.RackTemplate.ScyllaDB.CustomConfigMapRef = pointer.Ptr("b")
				return dc
			}(),
		},
		{
			name: "rack template scylladb custom ConfigMap ref is taken from datacenter template level when provided all other are not provided",
			remoteNamespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "scylla-aaa",
				},
			},
			remoteController: &scyllav1alpha1.RemoteOwner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-111",
					Namespace: "scylla-aaa",
					UID:       "1234",
				},
			},
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				cluster := newBasicScyllaDBCluster()
				cluster.Spec.DatacenterTemplate.ScyllaDB.CustomConfigMapRef = pointer.Ptr("a")
				return cluster
			}(),
			datacenter: dcFromSpec(0),
			expectedScyllaDBDatacenters: func() *scyllav1alpha1.ScyllaDBDatacenter {
				dc := newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{})
				dc.Spec.RackTemplate.ScyllaDB.CustomConfigMapRef = pointer.Ptr("a")
				return dc
			}(),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dc := tc.datacenter(tc.cluster)

			gotDatacenters, err := MakeRemoteScyllaDBDatacenters(
				tc.cluster,
				dc,
				tc.remoteScyllaDBDatacenters,
				tc.remoteNamespace,
				tc.remoteController,
				testClusterDomain,
			)
			if err != nil {
				t.Errorf("expected nil error, got %v", err)
			}
			if !equality.Semantic.DeepEqual(gotDatacenters, tc.expectedScyllaDBDatacenters) {
				t.Errorf("expected and got datacenters differ, diff: %s", cmp.Diff(gotDatacenters, tc.expectedScyllaDBDatacenters))
			}
		})
	}
}

func Test_makeLocalIdentityService(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name            string
		sc              *scyllav1alpha1.ScyllaDBCluster
		expectedService *corev1.Service
		expectedErr     error
	}{
		{
			name: "basic identity service for ScyllaDBCluster",
			sc:   newBasicScyllaDBCluster(),
			expectedService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-client-bowxv",
					Namespace: "scylla",
					Labels: map[string]string{
						"app.kubernetes.io/name":                                          "scylla.scylladb.com",
						"app.kubernetes.io/managed-by":                                    "scylla-operator.scylladb.com",
						"scylla-operator.scylladb.com/scylladbcluster-name":               "cluster",
						"scylla-operator.scylladb.com/scylladbcluster-local-service-type": "identity",
					},
					Annotations: map[string]string{},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "ScyllaDBCluster",
							Name:               "cluster",
							UID:                "uid",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Name:     "agent-api",
							Protocol: corev1.ProtocolTCP,
							Port:     10001,
						},
						{
							Name:     "alternator",
							Protocol: corev1.ProtocolTCP,
							Port:     8000,
						},
						{
							Name:     "alternator-tls",
							Protocol: corev1.ProtocolTCP,
							Port:     8043,
						},
						{
							Name:     "cql",
							Protocol: corev1.ProtocolTCP,
							Port:     9042,
						},
						{
							Name:     "cql-ssl",
							Protocol: corev1.ProtocolTCP,
							Port:     9142,
						},
					},
					Selector:  nil,
					ClusterIP: corev1.ClusterIPNone,
					Type:      corev1.ServiceTypeClusterIP,
				},
			},
			expectedErr: nil,
		},
		{
			name: "identity service with custom labels",
			sc: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newBasicScyllaDBCluster()
				sc.Spec.Metadata = &scyllav1alpha1.ObjectTemplateMetadata{
					Labels: map[string]string{
						"custom-label1": "value1",
						"custom-label2": "value2",
					},
				}
				return sc
			}(),
			expectedService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-client-bowxv",
					Namespace: "scylla",
					Labels: map[string]string{
						"app.kubernetes.io/name":                                          "scylla.scylladb.com",
						"app.kubernetes.io/managed-by":                                    "scylla-operator.scylladb.com",
						"scylla-operator.scylladb.com/scylladbcluster-name":               "cluster",
						"scylla-operator.scylladb.com/scylladbcluster-local-service-type": "identity",
						"custom-label1": "value1",
						"custom-label2": "value2",
					},
					Annotations: map[string]string{},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "ScyllaDBCluster",
							Name:               "cluster",
							UID:                "uid",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Name:     "agent-api",
							Protocol: corev1.ProtocolTCP,
							Port:     10001,
						},
						{
							Name:     "alternator",
							Protocol: corev1.ProtocolTCP,
							Port:     8000,
						},
						{
							Name:     "alternator-tls",
							Protocol: corev1.ProtocolTCP,
							Port:     8043,
						},
						{
							Name:     "cql",
							Protocol: corev1.ProtocolTCP,
							Port:     9042,
						},
						{
							Name:     "cql-ssl",
							Protocol: corev1.ProtocolTCP,
							Port:     9142,
						},
					},
					Selector:  nil,
					ClusterIP: corev1.ClusterIPNone,
					Type:      corev1.ServiceTypeClusterIP,
				},
			},
			expectedErr: nil,
		},
		{
			name: "identity service with custom annotations",
			sc: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newBasicScyllaDBCluster()
				sc.Spec.Metadata = &scyllav1alpha1.ObjectTemplateMetadata{
					Annotations: map[string]string{
						"custom.annotation/one": "value1",
						"custom.annotation/two": "value2",
					},
				}
				return sc
			}(),
			expectedService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-client-bowxv",
					Namespace: "scylla",
					Labels: map[string]string{
						"app.kubernetes.io/name":                                          "scylla.scylladb.com",
						"app.kubernetes.io/managed-by":                                    "scylla-operator.scylladb.com",
						"scylla-operator.scylladb.com/scylladbcluster-name":               "cluster",
						"scylla-operator.scylladb.com/scylladbcluster-local-service-type": "identity",
					},
					Annotations: map[string]string{
						"custom.annotation/one": "value1",
						"custom.annotation/two": "value2",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "ScyllaDBCluster",
							Name:               "cluster",
							UID:                "uid",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Name:     "agent-api",
							Protocol: corev1.ProtocolTCP,
							Port:     10001,
						},
						{
							Name:     "alternator",
							Protocol: corev1.ProtocolTCP,
							Port:     8000,
						},
						{
							Name:     "alternator-tls",
							Protocol: corev1.ProtocolTCP,
							Port:     8043,
						},
						{
							Name:     "cql",
							Protocol: corev1.ProtocolTCP,
							Port:     9042,
						},
						{
							Name:     "cql-ssl",
							Protocol: corev1.ProtocolTCP,
							Port:     9142,
						},
					},
					Selector:  nil,
					ClusterIP: corev1.ClusterIPNone,
					Type:      corev1.ServiceTypeClusterIP,
				},
			},
			expectedErr: nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			service, err := makeLocalIdentityService(tc.sc)

			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Fatalf("expected error %v, got %v", tc.expectedErr, err)
			}

			if !equality.Semantic.DeepEqual(service, tc.expectedService) {
				t.Errorf("expected and got services differ, diff: %s", cmp.Diff(tc.expectedService, service))
			}
		})
	}
}

func Test_makeEndpointSliceForIdentityService(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name                          string
		sc                            *scyllav1alpha1.ScyllaDBCluster
		remoteNamespaces              map[string]*corev1.Namespace
		existing                      map[string][]apimachineryruntime.Object
		expected                      *discoveryv1.EndpointSlice
		expectedProgressingConditions []metav1.Condition
		expectedErr                   error
	}{
		{
			name:             "remote namespace missing",
			sc:               newBasicScyllaDBCluster(),
			remoteNamespaces: map[string]*corev1.Namespace{},
			existing:         map[string][]apimachineryruntime.Object{},
			expected: &discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-client-bowxv",
					Namespace: "scylla",
					Labels: map[string]string{
						"app.kubernetes.io/name":                            "scylla.scylladb.com",
						"app.kubernetes.io/managed-by":                      "scylla-operator.scylladb.com",
						"endpointslice.kubernetes.io/managed-by":            "scylla-operator.scylladb.com",
						"kubernetes.io/service-name":                        "cluster-client-bowxv",
						"scylla-operator.scylladb.com/cluster-endpoints":    "cluster",
						"scylla-operator.scylladb.com/scylladbcluster-name": "cluster",
					},
					Annotations: map[string]string{},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "ScyllaDBCluster",
							Name:               "cluster",
							UID:                "uid",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				AddressType: discoveryv1.AddressTypeIPv4,
				Endpoints:   nil,
				Ports: []discoveryv1.EndpointPort{
					{
						Name:     pointer.Ptr("agent-api"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](10001),
					},
					{
						Name:     pointer.Ptr("alternator"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](8000),
					},
					{
						Name:     pointer.Ptr("alternator-tls"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](8043),
					},
					{
						Name:     pointer.Ptr("cql"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](9042),
					},
					{
						Name:     pointer.Ptr("cql-ssl"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](9142),
					},
				},
			},
			expectedProgressingConditions: []metav1.Condition{
				{
					Type:    "EndpointSliceControllerProgressing",
					Status:  "True",
					Reason:  "WaitingForRemoteNamespace",
					Message: `Waiting for Namespace to be created in "dc1-rkc" Cluster`,
				},
			},
			expectedErr: nil,
		},
		{
			name: "client broadcast address type podIP, no pods",
			sc:   newBasicScyllaDBCluster(),
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "dc1-rkc-ns",
					},
				},
			},
			existing: map[string][]apimachineryruntime.Object{
				"dc1-rkc": {},
			},
			expected: &discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-client-bowxv",
					Namespace: "scylla",
					Labels: map[string]string{
						"app.kubernetes.io/name":                            "scylla.scylladb.com",
						"app.kubernetes.io/managed-by":                      "scylla-operator.scylladb.com",
						"endpointslice.kubernetes.io/managed-by":            "scylla-operator.scylladb.com",
						"kubernetes.io/service-name":                        "cluster-client-bowxv",
						"scylla-operator.scylladb.com/cluster-endpoints":    "cluster",
						"scylla-operator.scylladb.com/scylladbcluster-name": "cluster",
					},
					Annotations: map[string]string{},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "ScyllaDBCluster",
							Name:               "cluster",
							UID:                "uid",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				AddressType: discoveryv1.AddressTypeIPv4,
				Endpoints:   nil,
				Ports: []discoveryv1.EndpointPort{
					{
						Name:     pointer.Ptr("agent-api"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](10001),
					},
					{
						Name:     pointer.Ptr("alternator"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](8000),
					},
					{
						Name:     pointer.Ptr("alternator-tls"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](8043),
					},
					{
						Name:     pointer.Ptr("cql"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](9042),
					},
					{
						Name:     pointer.Ptr("cql-ssl"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](9142),
					},
				},
			},
			expectedProgressingConditions: []metav1.Condition{},
			expectedErr:                   nil,
		},
		{
			name: "client broadcast address type podIP, custom labels, no pods",
			sc: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newBasicScyllaDBCluster()
				sc.Spec.Metadata = &scyllav1alpha1.ObjectTemplateMetadata{
					Labels: map[string]string{
						"custom-label1": "value1",
						"custom-label2": "value2",
					},
				}
				return sc
			}(),
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "dc1-rkc-ns",
					},
				},
			},
			existing: map[string][]apimachineryruntime.Object{
				"dc1-rkc": {},
			},
			expected: &discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-client-bowxv",
					Namespace: "scylla",
					Labels: map[string]string{
						"app.kubernetes.io/name":                            "scylla.scylladb.com",
						"app.kubernetes.io/managed-by":                      "scylla-operator.scylladb.com",
						"endpointslice.kubernetes.io/managed-by":            "scylla-operator.scylladb.com",
						"kubernetes.io/service-name":                        "cluster-client-bowxv",
						"scylla-operator.scylladb.com/cluster-endpoints":    "cluster",
						"scylla-operator.scylladb.com/scylladbcluster-name": "cluster",
						"custom-label1":                                     "value1",
						"custom-label2":                                     "value2",
					},
					Annotations: map[string]string{},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "ScyllaDBCluster",
							Name:               "cluster",
							UID:                "uid",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				AddressType: discoveryv1.AddressTypeIPv4,
				Endpoints:   nil,
				Ports: []discoveryv1.EndpointPort{
					{
						Name:     pointer.Ptr("agent-api"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](10001),
					},
					{
						Name:     pointer.Ptr("alternator"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](8000),
					},
					{
						Name:     pointer.Ptr("alternator-tls"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](8043),
					},
					{
						Name:     pointer.Ptr("cql"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](9042),
					},
					{
						Name:     pointer.Ptr("cql-ssl"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](9142),
					},
				},
			},
			expectedProgressingConditions: []metav1.Condition{},
			expectedErr:                   nil,
		},
		{
			name: "client broadcast address type podIP, custom annotations, no pods",
			sc: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newBasicScyllaDBCluster()
				sc.Spec.Metadata = &scyllav1alpha1.ObjectTemplateMetadata{
					Annotations: map[string]string{
						"custom.annotation/one": "value1",
						"custom.annotation/two": "value2",
					},
				}
				return sc
			}(),
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "dc1-rkc-ns",
					},
				},
			},
			existing: map[string][]apimachineryruntime.Object{
				"dc1-rkc": {},
			},
			expected: &discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-client-bowxv",
					Namespace: "scylla",
					Labels: map[string]string{
						"app.kubernetes.io/name":                            "scylla.scylladb.com",
						"app.kubernetes.io/managed-by":                      "scylla-operator.scylladb.com",
						"endpointslice.kubernetes.io/managed-by":            "scylla-operator.scylladb.com",
						"kubernetes.io/service-name":                        "cluster-client-bowxv",
						"scylla-operator.scylladb.com/cluster-endpoints":    "cluster",
						"scylla-operator.scylladb.com/scylladbcluster-name": "cluster",
					},
					Annotations: map[string]string{
						"custom.annotation/one": "value1",
						"custom.annotation/two": "value2",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "ScyllaDBCluster",
							Name:               "cluster",
							UID:                "uid",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				AddressType: discoveryv1.AddressTypeIPv4,
				Endpoints:   nil,
				Ports: []discoveryv1.EndpointPort{
					{
						Name:     pointer.Ptr("agent-api"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](10001),
					},
					{
						Name:     pointer.Ptr("alternator"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](8000),
					},
					{
						Name:     pointer.Ptr("alternator-tls"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](8043),
					},
					{
						Name:     pointer.Ptr("cql"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](9042),
					},
					{
						Name:     pointer.Ptr("cql-ssl"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](9142),
					},
				},
			},
			expectedProgressingConditions: []metav1.Condition{},
			expectedErr:                   nil,
		},
		{
			name: "client broadcast address type podIP, pod not ready, not serving, not terminating",
			sc:   newBasicScyllaDBCluster(),
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "dc1-rkc-ns",
					},
				},
			},
			existing: map[string][]apimachineryruntime.Object{
				"dc1-rkc": {
					&corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "dc1-rkc-a-0",
							Namespace: "dc1-rkc-ns",
							Labels: map[string]string{
								"app":                          "scylla",
								"app.kubernetes.io/name":       "scylla",
								"app.kubernetes.io/managed-by": "scylla-operator",
								"scylla/cluster":               "cluster-dc1",
							},
							DeletionTimestamp: nil,
						},
						Status: corev1.PodStatus{
							PodIP: "10.0.0.1",
							Conditions: []corev1.PodCondition{
								{
									Type:   corev1.PodReady,
									Status: corev1.ConditionFalse,
								},
							},
						},
					},
				},
			},
			expected: &discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-client-bowxv",
					Namespace: "scylla",
					Labels: map[string]string{
						"app.kubernetes.io/name":                            "scylla.scylladb.com",
						"app.kubernetes.io/managed-by":                      "scylla-operator.scylladb.com",
						"endpointslice.kubernetes.io/managed-by":            "scylla-operator.scylladb.com",
						"kubernetes.io/service-name":                        "cluster-client-bowxv",
						"scylla-operator.scylladb.com/cluster-endpoints":    "cluster",
						"scylla-operator.scylladb.com/scylladbcluster-name": "cluster",
					},
					Annotations: map[string]string{},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "ScyllaDBCluster",
							Name:               "cluster",
							UID:                "uid",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				AddressType: discoveryv1.AddressTypeIPv4,
				Endpoints: []discoveryv1.Endpoint{
					{
						Addresses: []string{"10.0.0.1"},
						Conditions: discoveryv1.EndpointConditions{
							Ready:       pointer.Ptr(false),
							Serving:     pointer.Ptr(false),
							Terminating: pointer.Ptr(false),
						},
					},
				},
				Ports: []discoveryv1.EndpointPort{
					{
						Name:     pointer.Ptr("agent-api"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](10001),
					},
					{
						Name:     pointer.Ptr("alternator"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](8000),
					},
					{
						Name:     pointer.Ptr("alternator-tls"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](8043),
					},
					{
						Name:     pointer.Ptr("cql"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](9042),
					},
					{
						Name:     pointer.Ptr("cql-ssl"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](9142),
					},
				},
			},
			expectedProgressingConditions: []metav1.Condition{},
			expectedErr:                   nil,
		},
		{
			name: "client broadcast address type podIP, pod ready, serving, not terminating",
			sc:   newBasicScyllaDBCluster(),
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "dc1-rkc-ns",
					},
				},
			},
			existing: map[string][]apimachineryruntime.Object{
				"dc1-rkc": {
					&corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "dc1-rkc-a-0",
							Namespace: "dc1-rkc-ns",
							Labels: map[string]string{
								"app":                          "scylla",
								"app.kubernetes.io/name":       "scylla",
								"app.kubernetes.io/managed-by": "scylla-operator",
								"scylla/cluster":               "cluster-dc1",
							},
						},
						Status: corev1.PodStatus{
							PodIP: "10.0.0.1",
							Conditions: []corev1.PodCondition{
								{
									Type:   corev1.PodReady,
									Status: corev1.ConditionTrue,
								},
							},
						},
					},
				},
			},
			expected: &discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-client-bowxv",
					Namespace: "scylla",
					Labels: map[string]string{
						"app.kubernetes.io/name":                            "scylla.scylladb.com",
						"app.kubernetes.io/managed-by":                      "scylla-operator.scylladb.com",
						"endpointslice.kubernetes.io/managed-by":            "scylla-operator.scylladb.com",
						"kubernetes.io/service-name":                        "cluster-client-bowxv",
						"scylla-operator.scylladb.com/cluster-endpoints":    "cluster",
						"scylla-operator.scylladb.com/scylladbcluster-name": "cluster",
					},
					Annotations: map[string]string{},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "ScyllaDBCluster",
							Name:               "cluster",
							UID:                "uid",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				AddressType: discoveryv1.AddressTypeIPv4,
				Endpoints: []discoveryv1.Endpoint{
					{
						Addresses: []string{"10.0.0.1"},
						Conditions: discoveryv1.EndpointConditions{
							Ready:       pointer.Ptr(true),
							Serving:     pointer.Ptr(true),
							Terminating: pointer.Ptr(false),
						},
					},
				},
				Ports: []discoveryv1.EndpointPort{
					{
						Name:     pointer.Ptr("agent-api"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](10001),
					},
					{
						Name:     pointer.Ptr("alternator"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](8000),
					},
					{
						Name:     pointer.Ptr("alternator-tls"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](8043),
					},
					{
						Name:     pointer.Ptr("cql"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](9042),
					},
					{
						Name:     pointer.Ptr("cql-ssl"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](9142),
					},
				},
			},
			expectedProgressingConditions: []metav1.Condition{},
			expectedErr:                   nil,
		},
		{
			name: "client broadcast address type podIP, pod not ready, not serving, terminating",
			sc:   newBasicScyllaDBCluster(),
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "dc1-rkc-ns",
					},
				},
			},
			existing: map[string][]apimachineryruntime.Object{
				"dc1-rkc": {
					&corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "dc1-rkc-a-0",
							Namespace: "dc1-rkc-ns",
							Labels: map[string]string{
								"app":                          "scylla",
								"app.kubernetes.io/name":       "scylla",
								"app.kubernetes.io/managed-by": "scylla-operator",
								"scylla/cluster":               "cluster-dc1",
							},
							DeletionTimestamp: &metav1.Time{},
						},
						Status: corev1.PodStatus{
							PodIP: "10.0.0.1",
							Conditions: []corev1.PodCondition{
								{
									Type:   corev1.PodReady,
									Status: corev1.ConditionFalse,
								},
							},
						},
					},
				},
			},
			expected: &discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-client-bowxv",
					Namespace: "scylla",
					Labels: map[string]string{
						"app.kubernetes.io/name":                            "scylla.scylladb.com",
						"app.kubernetes.io/managed-by":                      "scylla-operator.scylladb.com",
						"endpointslice.kubernetes.io/managed-by":            "scylla-operator.scylladb.com",
						"kubernetes.io/service-name":                        "cluster-client-bowxv",
						"scylla-operator.scylladb.com/cluster-endpoints":    "cluster",
						"scylla-operator.scylladb.com/scylladbcluster-name": "cluster",
					},
					Annotations: map[string]string{},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "ScyllaDBCluster",
							Name:               "cluster",
							UID:                "uid",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				AddressType: discoveryv1.AddressTypeIPv4,
				Endpoints: []discoveryv1.Endpoint{
					{
						Addresses: []string{"10.0.0.1"},
						Conditions: discoveryv1.EndpointConditions{
							Ready:       pointer.Ptr(false),
							Serving:     pointer.Ptr(false),
							Terminating: pointer.Ptr(true),
						},
					},
				},
				Ports: []discoveryv1.EndpointPort{
					{
						Name:     pointer.Ptr("agent-api"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](10001),
					},
					{
						Name:     pointer.Ptr("alternator"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](8000),
					},
					{
						Name:     pointer.Ptr("alternator-tls"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](8043),
					},
					{
						Name:     pointer.Ptr("cql"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](9042),
					},
					{
						Name:     pointer.Ptr("cql-ssl"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](9142),
					},
				},
			},
			expectedProgressingConditions: []metav1.Condition{},
			expectedErr:                   nil,
		},
		{
			name: "client broadcast address type ServiceClusterIP, no member services",
			sc: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newBasicScyllaDBCluster()
				sc.Spec.ExposeOptions = &scyllav1alpha1.ScyllaDBClusterExposeOptions{
					BroadcastOptions: &scyllav1alpha1.ScyllaDBClusterNodeBroadcastOptions{
						Clients: scyllav1alpha1.BroadcastOptions{
							Type: scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
						},
					},
				}
				return sc
			}(),
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "dc1-rkc-ns",
					},
				},
			},
			existing: map[string][]apimachineryruntime.Object{
				"dc1-rkc": {
					&corev1.Service{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "dc1-rkc-a-0",
							Namespace: "dc1-rkc-ns",
							Labels: map[string]string{
								"app":                          "scylla",
								"app.kubernetes.io/name":       "scylla",
								"app.kubernetes.io/managed-by": "scylla-operator",
								"scylla/cluster":               "cluster-dc1",
								"scylla-operator.scylladb.com/scylla-service-type": "identity",
							},
						},
						Spec: corev1.ServiceSpec{
							ClusterIP: "172.17.17.1",
							ClusterIPs: []string{
								"172.17.17.1",
							},
							Type: corev1.ServiceTypeClusterIP,
							IPFamilies: []corev1.IPFamily{
								"IPv4",
							},
							IPFamilyPolicy: pointer.Ptr(corev1.IPFamilyPolicySingleStack),
						},
						Status: corev1.ServiceStatus{},
					},
				},
			},
			expected: &discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-client-bowxv",
					Namespace: "scylla",
					Labels: map[string]string{
						"app.kubernetes.io/name":                            "scylla.scylladb.com",
						"app.kubernetes.io/managed-by":                      "scylla-operator.scylladb.com",
						"endpointslice.kubernetes.io/managed-by":            "scylla-operator.scylladb.com",
						"kubernetes.io/service-name":                        "cluster-client-bowxv",
						"scylla-operator.scylladb.com/cluster-endpoints":    "cluster",
						"scylla-operator.scylladb.com/scylladbcluster-name": "cluster",
					},
					Annotations: map[string]string{},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "ScyllaDBCluster",
							Name:               "cluster",
							UID:                "uid",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				AddressType: discoveryv1.AddressTypeIPv4,
				Endpoints:   nil,
				Ports: []discoveryv1.EndpointPort{
					{
						Name:     pointer.Ptr("agent-api"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](10001),
					},
					{
						Name:     pointer.Ptr("alternator"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](8000),
					},
					{
						Name:     pointer.Ptr("alternator-tls"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](8043),
					},
					{
						Name:     pointer.Ptr("cql"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](9042),
					},
					{
						Name:     pointer.Ptr("cql-ssl"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](9142),
					},
				},
			},
			expectedProgressingConditions: []metav1.Condition{},
			expectedErr:                   nil,
		},
		{
			name: "client broadcast address type ServiceClusterIP, service with clusterIP none",
			sc: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newBasicScyllaDBCluster()
				sc.Spec.ExposeOptions = &scyllav1alpha1.ScyllaDBClusterExposeOptions{
					BroadcastOptions: &scyllav1alpha1.ScyllaDBClusterNodeBroadcastOptions{
						Clients: scyllav1alpha1.BroadcastOptions{
							Type: scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
						},
					},
				}
				return sc
			}(),
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "dc1-rkc-ns",
					},
				},
			},
			existing: map[string][]apimachineryruntime.Object{
				"dc1-rkc": {
					&corev1.Service{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "dc1-rkc-a-0",
							Namespace: "dc1-rkc-ns",
							Labels: map[string]string{
								"app":                          "scylla",
								"app.kubernetes.io/name":       "scylla",
								"app.kubernetes.io/managed-by": "scylla-operator",
								"scylla/cluster":               "cluster-dc1",
								"scylla-operator.scylladb.com/scylla-service-type": "member",
							},
						},
						Spec: corev1.ServiceSpec{
							ClusterIP: "None",
							ClusterIPs: []string{
								"None",
							},
							Type: corev1.ServiceTypeClusterIP,
							IPFamilies: []corev1.IPFamily{
								"IPv4",
							},
							IPFamilyPolicy: pointer.Ptr(corev1.IPFamilyPolicySingleStack),
						},
						Status: corev1.ServiceStatus{},
					},
				},
			},
			expected: &discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-client-bowxv",
					Namespace: "scylla",
					Labels: map[string]string{
						"app.kubernetes.io/name":                            "scylla.scylladb.com",
						"app.kubernetes.io/managed-by":                      "scylla-operator.scylladb.com",
						"endpointslice.kubernetes.io/managed-by":            "scylla-operator.scylladb.com",
						"kubernetes.io/service-name":                        "cluster-client-bowxv",
						"scylla-operator.scylladb.com/cluster-endpoints":    "cluster",
						"scylla-operator.scylladb.com/scylladbcluster-name": "cluster",
					},
					Annotations: map[string]string{},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "ScyllaDBCluster",
							Name:               "cluster",
							UID:                "uid",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				AddressType: discoveryv1.AddressTypeIPv4,
				Endpoints:   nil,
				Ports: []discoveryv1.EndpointPort{
					{
						Name:     pointer.Ptr("agent-api"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](10001),
					},
					{
						Name:     pointer.Ptr("alternator"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](8000),
					},
					{
						Name:     pointer.Ptr("alternator-tls"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](8043),
					},
					{
						Name:     pointer.Ptr("cql"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](9042),
					},
					{
						Name:     pointer.Ptr("cql-ssl"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](9142),
					},
				},
			},
			expectedProgressingConditions: []metav1.Condition{},
			expectedErr:                   nil,
		},
		{
			name: "client broadcast address type ServiceClusterIP, single stack cluster ip",
			sc: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newBasicScyllaDBCluster()
				sc.Spec.ExposeOptions = &scyllav1alpha1.ScyllaDBClusterExposeOptions{
					BroadcastOptions: &scyllav1alpha1.ScyllaDBClusterNodeBroadcastOptions{
						Clients: scyllav1alpha1.BroadcastOptions{
							Type: scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
						},
					},
				}
				return sc
			}(),
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "dc1-rkc-ns",
					},
				},
			},
			existing: map[string][]apimachineryruntime.Object{
				"dc1-rkc": {
					&corev1.Service{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "dc1-rkc-a-0",
							Namespace: "dc1-rkc-ns",
							Labels: map[string]string{
								"app":                          "scylla",
								"app.kubernetes.io/name":       "scylla",
								"app.kubernetes.io/managed-by": "scylla-operator",
								"scylla/cluster":               "cluster-dc1",
								"scylla-operator.scylladb.com/scylla-service-type": "member",
							},
						},
						Spec: corev1.ServiceSpec{
							ClusterIP: "172.17.17.1",
							ClusterIPs: []string{
								"172.17.17.1",
							},
							Type: corev1.ServiceTypeClusterIP,
							IPFamilies: []corev1.IPFamily{
								"IPv4",
							},
							IPFamilyPolicy: pointer.Ptr(corev1.IPFamilyPolicySingleStack),
						},
						Status: corev1.ServiceStatus{},
					},
				},
			},
			expected: &discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-client-bowxv",
					Namespace: "scylla",
					Labels: map[string]string{
						"app.kubernetes.io/name":                            "scylla.scylladb.com",
						"app.kubernetes.io/managed-by":                      "scylla-operator.scylladb.com",
						"endpointslice.kubernetes.io/managed-by":            "scylla-operator.scylladb.com",
						"kubernetes.io/service-name":                        "cluster-client-bowxv",
						"scylla-operator.scylladb.com/cluster-endpoints":    "cluster",
						"scylla-operator.scylladb.com/scylladbcluster-name": "cluster",
					},
					Annotations: map[string]string{},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "ScyllaDBCluster",
							Name:               "cluster",
							UID:                "uid",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				AddressType: discoveryv1.AddressTypeIPv4,
				Endpoints: []discoveryv1.Endpoint{
					{
						Addresses: []string{"172.17.17.1"},
						Conditions: discoveryv1.EndpointConditions{
							Ready:       pointer.Ptr(true),
							Serving:     pointer.Ptr(true),
							Terminating: pointer.Ptr(false),
						},
					},
				},
				Ports: []discoveryv1.EndpointPort{
					{
						Name:     pointer.Ptr("agent-api"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](10001),
					},
					{
						Name:     pointer.Ptr("alternator"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](8000),
					},
					{
						Name:     pointer.Ptr("alternator-tls"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](8043),
					},
					{
						Name:     pointer.Ptr("cql"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](9042),
					},
					{
						Name:     pointer.Ptr("cql-ssl"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](9142),
					},
				},
			},
			expectedProgressingConditions: []metav1.Condition{},
			expectedErr:                   nil,
		},
		{
			name: "client broadcast address type ServiceClusterIP, single stack cluster ip, terminating",
			sc: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newBasicScyllaDBCluster()
				sc.Spec.ExposeOptions = &scyllav1alpha1.ScyllaDBClusterExposeOptions{
					BroadcastOptions: &scyllav1alpha1.ScyllaDBClusterNodeBroadcastOptions{
						Clients: scyllav1alpha1.BroadcastOptions{
							Type: scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
						},
					},
				}
				return sc
			}(),
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "dc1-rkc-ns",
					},
				},
			},
			existing: map[string][]apimachineryruntime.Object{
				"dc1-rkc": {
					&corev1.Service{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "dc1-rkc-a-0",
							Namespace: "dc1-rkc-ns",
							Labels: map[string]string{
								"app":                          "scylla",
								"app.kubernetes.io/name":       "scylla",
								"app.kubernetes.io/managed-by": "scylla-operator",
								"scylla/cluster":               "cluster-dc1",
								"scylla-operator.scylladb.com/scylla-service-type": "member",
							},
							DeletionTimestamp: &metav1.Time{},
						},
						Spec: corev1.ServiceSpec{
							ClusterIP: "172.17.17.1",
							ClusterIPs: []string{
								"172.17.17.1",
							},
							Type: corev1.ServiceTypeClusterIP,
							IPFamilies: []corev1.IPFamily{
								"IPv4",
							},
							IPFamilyPolicy: pointer.Ptr(corev1.IPFamilyPolicySingleStack),
						},
						Status: corev1.ServiceStatus{},
					},
				},
			},
			expected: &discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-client-bowxv",
					Namespace: "scylla",
					Labels: map[string]string{
						"app.kubernetes.io/name":                            "scylla.scylladb.com",
						"app.kubernetes.io/managed-by":                      "scylla-operator.scylladb.com",
						"endpointslice.kubernetes.io/managed-by":            "scylla-operator.scylladb.com",
						"kubernetes.io/service-name":                        "cluster-client-bowxv",
						"scylla-operator.scylladb.com/cluster-endpoints":    "cluster",
						"scylla-operator.scylladb.com/scylladbcluster-name": "cluster",
					},
					Annotations: map[string]string{},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "ScyllaDBCluster",
							Name:               "cluster",
							UID:                "uid",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				AddressType: discoveryv1.AddressTypeIPv4,
				Endpoints: []discoveryv1.Endpoint{
					{
						Addresses: []string{"172.17.17.1"},
						Conditions: discoveryv1.EndpointConditions{
							Ready:       pointer.Ptr(true),
							Serving:     pointer.Ptr(true),
							Terminating: pointer.Ptr(true),
						},
					},
				},
				Ports: []discoveryv1.EndpointPort{
					{
						Name:     pointer.Ptr("agent-api"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](10001),
					},
					{
						Name:     pointer.Ptr("alternator"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](8000),
					},
					{
						Name:     pointer.Ptr("alternator-tls"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](8043),
					},
					{
						Name:     pointer.Ptr("cql"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](9042),
					},
					{
						Name:     pointer.Ptr("cql-ssl"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](9142),
					},
				},
			},
			expectedProgressingConditions: []metav1.Condition{},
			expectedErr:                   nil,
		},
		{
			name: "client broadcast address type ServiceClusterIP, dual stack cluster ip",
			sc: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newBasicScyllaDBCluster()
				sc.Spec.ExposeOptions = &scyllav1alpha1.ScyllaDBClusterExposeOptions{
					BroadcastOptions: &scyllav1alpha1.ScyllaDBClusterNodeBroadcastOptions{
						Clients: scyllav1alpha1.BroadcastOptions{
							Type: scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
						},
					},
				}
				return sc
			}(),
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "dc1-rkc-ns",
					},
				},
			},
			existing: map[string][]apimachineryruntime.Object{
				"dc1-rkc": {
					&corev1.Service{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "dc1-rkc-a-0",
							Namespace: "dc1-rkc-ns",
							Labels: map[string]string{
								"app":                          "scylla",
								"app.kubernetes.io/name":       "scylla",
								"app.kubernetes.io/managed-by": "scylla-operator",
								"scylla/cluster":               "cluster-dc1",
								"scylla-operator.scylladb.com/scylla-service-type": "member",
							},
						},
						Spec: corev1.ServiceSpec{
							ClusterIP: "172.17.17.1",
							ClusterIPs: []string{
								"172.17.17.1",
								"6cfb:276f:b9a0:f498:6d72:75fb:7157:c4e7",
							},
							Type: corev1.ServiceTypeClusterIP,
							IPFamilies: []corev1.IPFamily{
								"IPv4",
								"IPv6",
							},
							IPFamilyPolicy: pointer.Ptr(corev1.IPFamilyPolicyPreferDualStack),
						},
						Status: corev1.ServiceStatus{},
					},
				},
			},
			expected: &discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-client-bowxv",
					Namespace: "scylla",
					Labels: map[string]string{
						"app.kubernetes.io/name":                            "scylla.scylladb.com",
						"app.kubernetes.io/managed-by":                      "scylla-operator.scylladb.com",
						"endpointslice.kubernetes.io/managed-by":            "scylla-operator.scylladb.com",
						"kubernetes.io/service-name":                        "cluster-client-bowxv",
						"scylla-operator.scylladb.com/cluster-endpoints":    "cluster",
						"scylla-operator.scylladb.com/scylladbcluster-name": "cluster",
					},
					Annotations: map[string]string{},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "ScyllaDBCluster",
							Name:               "cluster",
							UID:                "uid",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				AddressType: discoveryv1.AddressTypeIPv4,
				Endpoints: []discoveryv1.Endpoint{
					{
						Addresses: []string{
							"172.17.17.1",
						},
						Conditions: discoveryv1.EndpointConditions{
							Ready:       pointer.Ptr(true),
							Serving:     pointer.Ptr(true),
							Terminating: pointer.Ptr(false),
						},
					},
				},
				Ports: []discoveryv1.EndpointPort{
					{
						Name:     pointer.Ptr("agent-api"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](10001),
					},
					{
						Name:     pointer.Ptr("alternator"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](8000),
					},
					{
						Name:     pointer.Ptr("alternator-tls"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](8043),
					},
					{
						Name:     pointer.Ptr("cql"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](9042),
					},
					{
						Name:     pointer.Ptr("cql-ssl"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](9142),
					},
				},
			},
			expectedProgressingConditions: []metav1.Condition{},
			expectedErr:                   nil,
		},
		{
			name: "client broadcast address type ServiceLoadBalancerIngress, no member services",
			sc: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newBasicScyllaDBCluster()
				sc.Spec.ExposeOptions = &scyllav1alpha1.ScyllaDBClusterExposeOptions{
					BroadcastOptions: &scyllav1alpha1.ScyllaDBClusterNodeBroadcastOptions{
						Clients: scyllav1alpha1.BroadcastOptions{
							Type: scyllav1alpha1.BroadcastAddressTypeServiceLoadBalancerIngress,
						},
					},
				}
				return sc
			}(),
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "dc1-rkc-ns",
					},
				},
			},
			existing: map[string][]apimachineryruntime.Object{
				"dc1-rkc": {
					&corev1.Service{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "dc1-rkc-a-0",
							Namespace: "dc1-rkc-ns",
							Labels: map[string]string{
								"app":                          "scylla",
								"app.kubernetes.io/name":       "scylla",
								"app.kubernetes.io/managed-by": "scylla-operator",
								"scylla/cluster":               "cluster-dc1",
								"scylla-operator.scylladb.com/scylla-service-type": "identity",
							},
						},
						Spec: corev1.ServiceSpec{
							ClusterIP: "172.17.17.1",
							ClusterIPs: []string{
								"172.17.17.1",
							},
							Type: corev1.ServiceTypeClusterIP,
							IPFamilies: []corev1.IPFamily{
								"IPv4",
							},
							IPFamilyPolicy: pointer.Ptr(corev1.IPFamilyPolicySingleStack),
						},
						Status: corev1.ServiceStatus{},
					},
				},
			},
			expected: &discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-client-bowxv",
					Namespace: "scylla",
					Labels: map[string]string{
						"app.kubernetes.io/name":                            "scylla.scylladb.com",
						"app.kubernetes.io/managed-by":                      "scylla-operator.scylladb.com",
						"endpointslice.kubernetes.io/managed-by":            "scylla-operator.scylladb.com",
						"kubernetes.io/service-name":                        "cluster-client-bowxv",
						"scylla-operator.scylladb.com/cluster-endpoints":    "cluster",
						"scylla-operator.scylladb.com/scylladbcluster-name": "cluster",
					},
					Annotations: map[string]string{},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "ScyllaDBCluster",
							Name:               "cluster",
							UID:                "uid",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				AddressType: discoveryv1.AddressTypeIPv4,
				Endpoints:   nil,
				Ports: []discoveryv1.EndpointPort{
					{
						Name:     pointer.Ptr("agent-api"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](10001),
					},
					{
						Name:     pointer.Ptr("alternator"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](8000),
					},
					{
						Name:     pointer.Ptr("alternator-tls"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](8043),
					},
					{
						Name:     pointer.Ptr("cql"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](9042),
					},
					{
						Name:     pointer.Ptr("cql-ssl"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](9142),
					},
				},
			},
			expectedProgressingConditions: []metav1.Condition{},
			expectedErr:                   nil,
		},
		{
			name: "client broadcast address type ServiceLoadBalancerIngress, service without load balancer status",
			sc: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newBasicScyllaDBCluster()
				sc.Spec.ExposeOptions = &scyllav1alpha1.ScyllaDBClusterExposeOptions{
					BroadcastOptions: &scyllav1alpha1.ScyllaDBClusterNodeBroadcastOptions{
						Clients: scyllav1alpha1.BroadcastOptions{
							Type: scyllav1alpha1.BroadcastAddressTypeServiceLoadBalancerIngress,
						},
					},
				}
				return sc
			}(),
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "dc1-rkc-ns",
					},
				},
			},
			existing: map[string][]apimachineryruntime.Object{
				"dc1-rkc": {
					&corev1.Service{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "dc1-rkc-a-0",
							Namespace: "dc1-rkc-ns",
							Labels: map[string]string{
								"app":                          "scylla",
								"app.kubernetes.io/name":       "scylla",
								"app.kubernetes.io/managed-by": "scylla-operator",
								"scylla/cluster":               "cluster-dc1",
								"scylla-operator.scylladb.com/scylla-service-type": "member",
							},
						},
						Spec: corev1.ServiceSpec{
							Type: corev1.ServiceTypeLoadBalancer,
						},
						Status: corev1.ServiceStatus{},
					},
				},
			},
			expected: &discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-client-bowxv",
					Namespace: "scylla",
					Labels: map[string]string{
						"app.kubernetes.io/name":                            "scylla.scylladb.com",
						"app.kubernetes.io/managed-by":                      "scylla-operator.scylladb.com",
						"endpointslice.kubernetes.io/managed-by":            "scylla-operator.scylladb.com",
						"kubernetes.io/service-name":                        "cluster-client-bowxv",
						"scylla-operator.scylladb.com/cluster-endpoints":    "cluster",
						"scylla-operator.scylladb.com/scylladbcluster-name": "cluster",
					},
					Annotations: map[string]string{},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "ScyllaDBCluster",
							Name:               "cluster",
							UID:                "uid",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				AddressType: discoveryv1.AddressTypeIPv4,
				Endpoints:   nil,
				Ports: []discoveryv1.EndpointPort{
					{
						Name:     pointer.Ptr("agent-api"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](10001),
					},
					{
						Name:     pointer.Ptr("alternator"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](8000),
					},
					{
						Name:     pointer.Ptr("alternator-tls"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](8043),
					},
					{
						Name:     pointer.Ptr("cql"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](9042),
					},
					{
						Name:     pointer.Ptr("cql-ssl"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](9142),
					},
				},
			},
			expectedProgressingConditions: []metav1.Condition{},
			expectedErr:                   nil,
		},
		{
			name: "client broadcast address type ServiceLoadBalancerIngress, service with load balancer IP",
			sc: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newBasicScyllaDBCluster()
				sc.Spec.ExposeOptions = &scyllav1alpha1.ScyllaDBClusterExposeOptions{
					BroadcastOptions: &scyllav1alpha1.ScyllaDBClusterNodeBroadcastOptions{
						Clients: scyllav1alpha1.BroadcastOptions{
							Type: scyllav1alpha1.BroadcastAddressTypeServiceLoadBalancerIngress,
						},
					},
				}
				return sc
			}(),
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "dc1-rkc-ns",
					},
				},
			},
			existing: map[string][]apimachineryruntime.Object{
				"dc1-rkc": {
					&corev1.Service{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "dc1-rkc-a-0",
							Namespace: "dc1-rkc-ns",
							Labels: map[string]string{
								"app":                          "scylla",
								"app.kubernetes.io/name":       "scylla",
								"app.kubernetes.io/managed-by": "scylla-operator",
								"scylla/cluster":               "cluster-dc1",
								"scylla-operator.scylladb.com/scylla-service-type": "member",
							},
						},
						Spec: corev1.ServiceSpec{
							Type: corev1.ServiceTypeLoadBalancer,
						},
						Status: corev1.ServiceStatus{
							LoadBalancer: corev1.LoadBalancerStatus{
								Ingress: []corev1.LoadBalancerIngress{
									{
										IP: "192.168.1.100",
									},
								},
							},
						},
					},
				},
			},
			expected: &discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-client-bowxv",
					Namespace: "scylla",
					Labels: map[string]string{
						"app.kubernetes.io/name":                            "scylla.scylladb.com",
						"app.kubernetes.io/managed-by":                      "scylla-operator.scylladb.com",
						"endpointslice.kubernetes.io/managed-by":            "scylla-operator.scylladb.com",
						"kubernetes.io/service-name":                        "cluster-client-bowxv",
						"scylla-operator.scylladb.com/cluster-endpoints":    "cluster",
						"scylla-operator.scylladb.com/scylladbcluster-name": "cluster",
					},
					Annotations: map[string]string{},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "ScyllaDBCluster",
							Name:               "cluster",
							UID:                "uid",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				AddressType: discoveryv1.AddressTypeIPv4,
				Endpoints: []discoveryv1.Endpoint{
					{
						Addresses: []string{"192.168.1.100"},
						Conditions: discoveryv1.EndpointConditions{
							Ready:       pointer.Ptr(true),
							Serving:     pointer.Ptr(true),
							Terminating: pointer.Ptr(false),
						},
					},
				},
				Ports: []discoveryv1.EndpointPort{
					{
						Name:     pointer.Ptr("agent-api"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](10001),
					},
					{
						Name:     pointer.Ptr("alternator"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](8000),
					},
					{
						Name:     pointer.Ptr("alternator-tls"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](8043),
					},
					{
						Name:     pointer.Ptr("cql"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](9042),
					},
					{
						Name:     pointer.Ptr("cql-ssl"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](9142),
					},
				},
			},
			expectedProgressingConditions: []metav1.Condition{},
			expectedErr:                   nil,
		},
		{
			name: "client broadcast address type ServiceLoadBalancerIngress, service with load balancer hostname",
			sc: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newBasicScyllaDBCluster()
				sc.Spec.ExposeOptions = &scyllav1alpha1.ScyllaDBClusterExposeOptions{
					BroadcastOptions: &scyllav1alpha1.ScyllaDBClusterNodeBroadcastOptions{
						Clients: scyllav1alpha1.BroadcastOptions{
							Type: scyllav1alpha1.BroadcastAddressTypeServiceLoadBalancerIngress,
						},
					},
				}
				return sc
			}(),
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "dc1-rkc-ns",
					},
				},
			},
			existing: map[string][]apimachineryruntime.Object{
				"dc1-rkc": {
					&corev1.Service{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "dc1-rkc-a-0",
							Namespace: "dc1-rkc-ns",
							Labels: map[string]string{
								"app":                          "scylla",
								"app.kubernetes.io/name":       "scylla",
								"app.kubernetes.io/managed-by": "scylla-operator",
								"scylla/cluster":               "cluster-dc1",
								"scylla-operator.scylladb.com/scylla-service-type": "member",
							},
						},
						Spec: corev1.ServiceSpec{
							Type: corev1.ServiceTypeLoadBalancer,
						},
						Status: corev1.ServiceStatus{
							LoadBalancer: corev1.LoadBalancerStatus{
								Ingress: []corev1.LoadBalancerIngress{
									{
										Hostname: "example.com",
									},
								},
							},
						},
					},
				},
			},
			expected: &discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-client-bowxv",
					Namespace: "scylla",
					Labels: map[string]string{
						"app.kubernetes.io/name":                            "scylla.scylladb.com",
						"app.kubernetes.io/managed-by":                      "scylla-operator.scylladb.com",
						"endpointslice.kubernetes.io/managed-by":            "scylla-operator.scylladb.com",
						"kubernetes.io/service-name":                        "cluster-client-bowxv",
						"scylla-operator.scylladb.com/cluster-endpoints":    "cluster",
						"scylla-operator.scylladb.com/scylladbcluster-name": "cluster",
					},
					Annotations: map[string]string{},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "ScyllaDBCluster",
							Name:               "cluster",
							UID:                "uid",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				AddressType: discoveryv1.AddressTypeIPv4,
				Endpoints: []discoveryv1.Endpoint{
					{
						Addresses: []string{"example.com"},
						Conditions: discoveryv1.EndpointConditions{
							Ready:       pointer.Ptr(true),
							Serving:     pointer.Ptr(true),
							Terminating: pointer.Ptr(false),
						},
					},
				},
				Ports: []discoveryv1.EndpointPort{
					{
						Name:     pointer.Ptr("agent-api"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](10001),
					},
					{
						Name:     pointer.Ptr("alternator"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](8000),
					},
					{
						Name:     pointer.Ptr("alternator-tls"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](8043),
					},
					{
						Name:     pointer.Ptr("cql"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](9042),
					},
					{
						Name:     pointer.Ptr("cql-ssl"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](9142),
					},
				},
			},
			expectedProgressingConditions: []metav1.Condition{},
			expectedErr:                   nil,
		},
		{
			name: "client broadcast address type podIP, IPv6 pod",
			sc:   newBasicScyllaDBCluster(),
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "dc1-rkc-ns",
					},
				},
			},
			existing: map[string][]apimachineryruntime.Object{
				"dc1-rkc": {
					&corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "dc1-rkc-a-0",
							Namespace: "dc1-rkc-ns",
							Labels: map[string]string{
								"app":                          "scylla",
								"app.kubernetes.io/name":       "scylla",
								"app.kubernetes.io/managed-by": "scylla-operator",
								"scylla/cluster":               "cluster-dc1",
							},
						},
						Status: corev1.PodStatus{
							PodIP: "2001:db8::1",
							Conditions: []corev1.PodCondition{
								{
									Type:   corev1.PodReady,
									Status: corev1.ConditionTrue,
								},
							},
						},
					},
				},
			},
			expected: &discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-client-bowxv",
					Namespace: "scylla",
					Labels: map[string]string{
						"app.kubernetes.io/name":                            "scylla.scylladb.com",
						"app.kubernetes.io/managed-by":                      "scylla-operator.scylladb.com",
						"endpointslice.kubernetes.io/managed-by":            "scylla-operator.scylladb.com",
						"kubernetes.io/service-name":                        "cluster-client-bowxv",
						"scylla-operator.scylladb.com/cluster-endpoints":    "cluster",
						"scylla-operator.scylladb.com/scylladbcluster-name": "cluster",
					},
					Annotations: map[string]string{},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "ScyllaDBCluster",
							Name:               "cluster",
							UID:                "uid",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				AddressType: discoveryv1.AddressTypeIPv6,
				Endpoints: []discoveryv1.Endpoint{
					{
						Addresses: []string{"2001:db8::1"},
						Conditions: discoveryv1.EndpointConditions{
							Ready:       pointer.Ptr(true),
							Serving:     pointer.Ptr(true),
							Terminating: pointer.Ptr(false),
						},
					},
				},
				Ports: []discoveryv1.EndpointPort{
					{
						Name:     pointer.Ptr("agent-api"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](10001),
					},
					{
						Name:     pointer.Ptr("alternator"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](8000),
					},
					{
						Name:     pointer.Ptr("alternator-tls"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](8043),
					},
					{
						Name:     pointer.Ptr("cql"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](9042),
					},
					{
						Name:     pointer.Ptr("cql-ssl"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](9142),
					},
				},
			},
			expectedProgressingConditions: []metav1.Condition{},
			expectedErr:                   nil,
		},
		{
			name: "client broadcast address type ServiceClusterIP, IPv6 service",
			sc: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newBasicScyllaDBCluster()
				sc.Spec.ExposeOptions = &scyllav1alpha1.ScyllaDBClusterExposeOptions{
					BroadcastOptions: &scyllav1alpha1.ScyllaDBClusterNodeBroadcastOptions{
						Clients: scyllav1alpha1.BroadcastOptions{
							Type: scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
						},
					},
				}
				return sc
			}(),
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "dc1-rkc-ns",
					},
				},
			},
			existing: map[string][]apimachineryruntime.Object{
				"dc1-rkc": {
					&corev1.Service{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "dc1-rkc-a-0",
							Namespace: "dc1-rkc-ns",
							Labels: map[string]string{
								"app":                          "scylla",
								"app.kubernetes.io/name":       "scylla",
								"app.kubernetes.io/managed-by": "scylla-operator",
								"scylla/cluster":               "cluster-dc1",
								"scylla-operator.scylladb.com/scylla-service-type": "member",
							},
						},
						Spec: corev1.ServiceSpec{
							ClusterIP: "fd00::1",
							ClusterIPs: []string{
								"fd00::1",
							},
							Type: corev1.ServiceTypeClusterIP,
							IPFamilies: []corev1.IPFamily{
								corev1.IPv6Protocol,
							},
							IPFamilyPolicy: pointer.Ptr(corev1.IPFamilyPolicySingleStack),
						},
						Status: corev1.ServiceStatus{},
					},
				},
			},
			expected: &discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-client-bowxv",
					Namespace: "scylla",
					Labels: map[string]string{
						"app.kubernetes.io/name":                            "scylla.scylladb.com",
						"app.kubernetes.io/managed-by":                      "scylla-operator.scylladb.com",
						"endpointslice.kubernetes.io/managed-by":            "scylla-operator.scylladb.com",
						"kubernetes.io/service-name":                        "cluster-client-bowxv",
						"scylla-operator.scylladb.com/cluster-endpoints":    "cluster",
						"scylla-operator.scylladb.com/scylladbcluster-name": "cluster",
					},
					Annotations: map[string]string{},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "ScyllaDBCluster",
							Name:               "cluster",
							UID:                "uid",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				AddressType: discoveryv1.AddressTypeIPv6,
				Endpoints: []discoveryv1.Endpoint{
					{
						Addresses: []string{"fd00::1"},
						Conditions: discoveryv1.EndpointConditions{
							Ready:       pointer.Ptr(true),
							Serving:     pointer.Ptr(true),
							Terminating: pointer.Ptr(false),
						},
					},
				},
				Ports: []discoveryv1.EndpointPort{
					{
						Name:     pointer.Ptr("agent-api"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](10001),
					},
					{
						Name:     pointer.Ptr("alternator"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](8000),
					},
					{
						Name:     pointer.Ptr("alternator-tls"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](8043),
					},
					{
						Name:     pointer.Ptr("cql"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](9042),
					},
					{
						Name:     pointer.Ptr("cql-ssl"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](9142),
					},
				},
			},
			expectedProgressingConditions: []metav1.Condition{},
			expectedErr:                   nil,
		},
		{
			name: "client broadcast address type ServiceLoadBalancerIngress, IPv6 load balancer",
			sc: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newBasicScyllaDBCluster()
				sc.Spec.ExposeOptions = &scyllav1alpha1.ScyllaDBClusterExposeOptions{
					BroadcastOptions: &scyllav1alpha1.ScyllaDBClusterNodeBroadcastOptions{
						Clients: scyllav1alpha1.BroadcastOptions{
							Type: scyllav1alpha1.BroadcastAddressTypeServiceLoadBalancerIngress,
						},
					},
				}
				return sc
			}(),
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "dc1-rkc-ns",
					},
				},
			},
			existing: map[string][]apimachineryruntime.Object{
				"dc1-rkc": {
					&corev1.Service{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "dc1-rkc-a-0",
							Namespace: "dc1-rkc-ns",
							Labels: map[string]string{
								"app":                          "scylla",
								"app.kubernetes.io/name":       "scylla",
								"app.kubernetes.io/managed-by": "scylla-operator",
								"scylla/cluster":               "cluster-dc1",
								"scylla-operator.scylladb.com/scylla-service-type": "member",
							},
						},
						Spec: corev1.ServiceSpec{
							Type: corev1.ServiceTypeLoadBalancer,
						},
						Status: corev1.ServiceStatus{
							LoadBalancer: corev1.LoadBalancerStatus{
								Ingress: []corev1.LoadBalancerIngress{
									{
										IP: "2001:db8:1::100",
									},
								},
							},
						},
					},
				},
			},
			expected: &discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-client-bowxv",
					Namespace: "scylla",
					Labels: map[string]string{
						"app.kubernetes.io/name":                            "scylla.scylladb.com",
						"app.kubernetes.io/managed-by":                      "scylla-operator.scylladb.com",
						"endpointslice.kubernetes.io/managed-by":            "scylla-operator.scylladb.com",
						"kubernetes.io/service-name":                        "cluster-client-bowxv",
						"scylla-operator.scylladb.com/cluster-endpoints":    "cluster",
						"scylla-operator.scylladb.com/scylladbcluster-name": "cluster",
					},
					Annotations: map[string]string{},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "ScyllaDBCluster",
							Name:               "cluster",
							UID:                "uid",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				AddressType: discoveryv1.AddressTypeIPv6,
				Endpoints: []discoveryv1.Endpoint{
					{
						Addresses: []string{"2001:db8:1::100"},
						Conditions: discoveryv1.EndpointConditions{
							Ready:       pointer.Ptr(true),
							Serving:     pointer.Ptr(true),
							Terminating: pointer.Ptr(false),
						},
					},
				},
				Ports: []discoveryv1.EndpointPort{
					{
						Name:     pointer.Ptr("agent-api"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](10001),
					},
					{
						Name:     pointer.Ptr("alternator"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](8000),
					},
					{
						Name:     pointer.Ptr("alternator-tls"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](8043),
					},
					{
						Name:     pointer.Ptr("cql"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](9042),
					},
					{
						Name:     pointer.Ptr("cql-ssl"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr[int32](9142),
					},
				},
			},
			expectedProgressingConditions: []metav1.Condition{},
			expectedErr:                   nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fakeClusterIndexer := newFakeClusterIndexer(t, tc.existing)
			remoteServiceLister := remotelister.NewClusterLister(corev1listers.NewServiceLister, fakeClusterIndexer)
			remotePodLister := remotelister.NewClusterLister(corev1listers.NewPodLister, fakeClusterIndexer)

			progressingConditions, endpointSlice, err := makeEndpointSliceForLocalIdentityService(
				tc.sc,
				tc.remoteNamespaces,
				remoteServiceLister,
				remotePodLister,
			)

			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Fatalf("expected and got errors differ:\n%s\n", cmp.Diff(tc.expectedErr, err, cmpopts.EquateErrors()))
			}

			if !equality.Semantic.DeepEqual(progressingConditions, tc.expectedProgressingConditions) {
				t.Errorf("expected and got progressing conditions differ: %s", cmp.Diff(tc.expectedProgressingConditions, progressingConditions))
			}

			if !equality.Semantic.DeepEqual(endpointSlice, tc.expected) {
				t.Errorf("expected and got endpointslice differs: %s", cmp.Diff(tc.expected, endpointSlice))
			}
		})
	}
}

func Test_makeEndpointSlicesForSeedService(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name                          string
		sc                            *scyllav1alpha1.ScyllaDBCluster
		dc                            *scyllav1alpha1.ScyllaDBClusterDatacenter
		remoteNamespace               *corev1.Namespace
		remoteController              metav1.Object
		remoteNamespaces              map[string]*corev1.Namespace
		existing                      map[string][]apimachineryruntime.Object
		managingClusterDomain         string
		expected                      []*discoveryv1.EndpointSlice
		expectedProgressingConditions []metav1.Condition
		expectedErr                   error
	}{
		{
			name: "single datacenter cluster, no remote endpoints",
			sc: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newBasicScyllaDBCluster()
				sc.Spec.Datacenters = []scyllav1alpha1.ScyllaDBClusterDatacenter{
					{
						Name:                        "dc1",
						RemoteKubernetesClusterName: "dc1-rkc",
					},
				}
				return sc
			}(),
			dc: &scyllav1alpha1.ScyllaDBClusterDatacenter{
				Name:                        "dc1",
				RemoteKubernetesClusterName: "dc1-rkc",
			},
			remoteNamespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "scylla",
				},
			},
			remoteController: &scyllav1alpha1.ScyllaDBDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-dc1",
					Namespace: "scylla",
					UID:       "dc-uid",
				},
			},
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "dc1-rkc-ns",
					},
				},
			},
			existing:                      map[string][]apimachineryruntime.Object{},
			managingClusterDomain:         "cluster.local",
			expected:                      []*discoveryv1.EndpointSlice{},
			expectedProgressingConditions: []metav1.Condition{},
			expectedErr:                   nil,
		},
		{
			name: "multi-datacenter cluster, IPv4 pods",
			sc: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newBasicScyllaDBCluster()
				sc.Spec.Datacenters = []scyllav1alpha1.ScyllaDBClusterDatacenter{
					{
						Name:                        "dc1",
						RemoteKubernetesClusterName: "dc1-rkc",
					},
					{
						Name:                        "dc2",
						RemoteKubernetesClusterName: "dc2-rkc",
					},
				}
				return sc
			}(),
			dc: &scyllav1alpha1.ScyllaDBClusterDatacenter{
				Name:                        "dc1",
				RemoteKubernetesClusterName: "dc1-rkc",
			},
			remoteNamespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "scylla",
				},
			},
			remoteController: &scyllav1alpha1.ScyllaDBDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-dc1",
					Namespace: "scylla",
					UID:       "dc-uid",
				},
			},
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "dc1-rkc-ns",
					},
				},
				"dc2-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "dc2-rkc-ns",
					},
				},
			},
			existing: map[string][]apimachineryruntime.Object{
				"dc2-rkc": {
					&corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "dc2-rkc-a-0",
							Namespace: "dc2-rkc-ns",
							Labels: map[string]string{
								"app":                          "scylla",
								"app.kubernetes.io/name":       "scylla",
								"app.kubernetes.io/managed-by": "scylla-operator",
								"scylla/cluster":               "cluster-dc2",
							},
						},
						Status: corev1.PodStatus{
							PodIP: "10.0.1.1",
							Conditions: []corev1.PodCondition{
								{
									Type:   corev1.PodReady,
									Status: corev1.ConditionTrue,
								},
							},
						},
					},
				},
			},
			managingClusterDomain: "cluster.local",
			expected: []*discoveryv1.EndpointSlice{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-dc2-seed",
						Namespace: "scylla",
						Labels: map[string]string{
							"app.kubernetes.io/managed-by":                                        "remote.scylla-operator.scylladb.com",
							"endpointslice.kubernetes.io/managed-by":                              "scylla-operator.scylladb.com",
							"kubernetes.io/service-name":                                          "cluster-dc2-seed",
							"scylla-operator.scylladb.com/managed-by-cluster":                     "cluster.local",
							"scylla-operator.scylladb.com/parent-scylladbcluster-datacenter-name": "dc1",
							"scylla-operator.scylladb.com/parent-scylladbcluster-name":            "cluster",
							"scylla-operator.scylladb.com/parent-scylladbcluster-namespace":       "scylla",
							"scylla-operator.scylladb.com/remote-cluster-endpoints":               "cluster",
						},
						Annotations: map[string]string{},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "scylla.scylladb.com/v1alpha1",
								Kind:               "RemoteOwner",
								Name:               "cluster-dc1",
								UID:                "dc-uid",
								Controller:         pointer.Ptr(true),
								BlockOwnerDeletion: pointer.Ptr(true),
							},
						},
					},
					AddressType: discoveryv1.AddressTypeIPv4,
					Endpoints: []discoveryv1.Endpoint{
						{
							Addresses: []string{"10.0.1.1"},
							Conditions: discoveryv1.EndpointConditions{
								Ready:       pointer.Ptr(true),
								Serving:     pointer.Ptr(true),
								Terminating: pointer.Ptr(false),
							},
						},
					},
					Ports: []discoveryv1.EndpointPort{
						{
							Name:     pointer.Ptr("inter-node"),
							Protocol: pointer.Ptr(corev1.ProtocolTCP),
							Port:     pointer.Ptr[int32](7000),
						},
						{
							Name:     pointer.Ptr("inter-node-ssl"),
							Protocol: pointer.Ptr(corev1.ProtocolTCP),
							Port:     pointer.Ptr[int32](7001),
						},
						{
							Name:     pointer.Ptr("cql"),
							Protocol: pointer.Ptr(corev1.ProtocolTCP),
							Port:     pointer.Ptr[int32](9042),
						},
						{
							Name:     pointer.Ptr("cql-ssl"),
							Protocol: pointer.Ptr(corev1.ProtocolTCP),
							Port:     pointer.Ptr[int32](9142),
						},
					},
				},
			},
			expectedProgressingConditions: []metav1.Condition{},
			expectedErr:                   nil,
		},
		{
			name: "multi-datacenter cluster, IPv6 pods",
			sc: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newBasicScyllaDBCluster()
				sc.Spec.Datacenters = []scyllav1alpha1.ScyllaDBClusterDatacenter{
					{
						Name:                        "dc1",
						RemoteKubernetesClusterName: "dc1-rkc",
					},
					{
						Name:                        "dc2",
						RemoteKubernetesClusterName: "dc2-rkc",
					},
				}
				return sc
			}(),
			dc: &scyllav1alpha1.ScyllaDBClusterDatacenter{
				Name:                        "dc1",
				RemoteKubernetesClusterName: "dc1-rkc",
			},
			remoteNamespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "scylla",
				},
			},
			remoteController: &scyllav1alpha1.ScyllaDBDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-dc1",
					Namespace: "scylla",
					UID:       "dc-uid",
				},
			},
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "dc1-rkc-ns",
					},
				},
				"dc2-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "dc2-rkc-ns",
					},
				},
			},
			existing: map[string][]apimachineryruntime.Object{
				"dc2-rkc": {
					&corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "dc2-rkc-a-0",
							Namespace: "dc2-rkc-ns",
							Labels: map[string]string{
								"app":                          "scylla",
								"app.kubernetes.io/name":       "scylla",
								"app.kubernetes.io/managed-by": "scylla-operator",
								"scylla/cluster":               "cluster-dc2",
							},
						},
						Status: corev1.PodStatus{
							PodIP: "2001:db8::1",
							Conditions: []corev1.PodCondition{
								{
									Type:   corev1.PodReady,
									Status: corev1.ConditionTrue,
								},
							},
						},
					},
				},
			},
			managingClusterDomain: "cluster.local",
			expected: []*discoveryv1.EndpointSlice{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-dc2-seed",
						Namespace: "scylla",
						Labels: map[string]string{
							"app.kubernetes.io/managed-by":                                        "remote.scylla-operator.scylladb.com",
							"endpointslice.kubernetes.io/managed-by":                              "scylla-operator.scylladb.com",
							"kubernetes.io/service-name":                                          "cluster-dc2-seed",
							"scylla-operator.scylladb.com/managed-by-cluster":                     "cluster.local",
							"scylla-operator.scylladb.com/parent-scylladbcluster-datacenter-name": "dc1",
							"scylla-operator.scylladb.com/parent-scylladbcluster-name":            "cluster",
							"scylla-operator.scylladb.com/parent-scylladbcluster-namespace":       "scylla",
							"scylla-operator.scylladb.com/remote-cluster-endpoints":               "cluster",
						},
						Annotations: map[string]string{},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "scylla.scylladb.com/v1alpha1",
								Kind:               "RemoteOwner",
								Name:               "cluster-dc1",
								UID:                "dc-uid",
								Controller:         pointer.Ptr(true),
								BlockOwnerDeletion: pointer.Ptr(true),
							},
						},
					},
					AddressType: discoveryv1.AddressTypeIPv6,
					Endpoints: []discoveryv1.Endpoint{
						{
							Addresses: []string{"2001:db8::1"},
							Conditions: discoveryv1.EndpointConditions{
								Ready:       pointer.Ptr(true),
								Serving:     pointer.Ptr(true),
								Terminating: pointer.Ptr(false),
							},
						},
					},
					Ports: []discoveryv1.EndpointPort{
						{
							Name:     pointer.Ptr("inter-node"),
							Protocol: pointer.Ptr(corev1.ProtocolTCP),
							Port:     pointer.Ptr[int32](7000),
						},
						{
							Name:     pointer.Ptr("inter-node-ssl"),
							Protocol: pointer.Ptr(corev1.ProtocolTCP),
							Port:     pointer.Ptr[int32](7001),
						},
						{
							Name:     pointer.Ptr("cql"),
							Protocol: pointer.Ptr(corev1.ProtocolTCP),
							Port:     pointer.Ptr[int32](9042),
						},
						{
							Name:     pointer.Ptr("cql-ssl"),
							Protocol: pointer.Ptr(corev1.ProtocolTCP),
							Port:     pointer.Ptr[int32](9142),
						},
					},
				},
			},
			expectedProgressingConditions: []metav1.Condition{},
			expectedErr:                   nil,
		},
		{
			name: "remote namespace missing",
			sc: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newBasicScyllaDBCluster()
				sc.Spec.Datacenters = []scyllav1alpha1.ScyllaDBClusterDatacenter{
					{
						Name:                        "dc1",
						RemoteKubernetesClusterName: "dc1-rkc",
					},
					{
						Name:                        "dc2",
						RemoteKubernetesClusterName: "dc2-rkc",
					},
				}
				return sc
			}(),
			dc: &scyllav1alpha1.ScyllaDBClusterDatacenter{
				Name:                        "dc1",
				RemoteKubernetesClusterName: "dc1-rkc",
			},
			remoteNamespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "scylla",
				},
			},
			remoteController: &scyllav1alpha1.ScyllaDBDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-dc1",
					Namespace: "scylla",
					UID:       "dc-uid",
				},
			},
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "dc1-rkc-ns",
					},
				},
				// dc2-rkc namespace is missing
			},
			existing:              map[string][]apimachineryruntime.Object{},
			managingClusterDomain: "cluster.local",
			expected:              []*discoveryv1.EndpointSlice{},
			expectedProgressingConditions: []metav1.Condition{
				{
					Type:    "RemoteEndpointSliceControllerDatacenterdc1Progressing",
					Status:  "True",
					Reason:  "WaitingForRemoteNamespace",
					Message: `Waiting for Namespace to be created in "dc2-rkc" Cluster`,
				},
			},
			expectedErr: nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fakeClusterIndexer := newFakeClusterIndexer(t, tc.existing)
			remoteServiceLister := remotelister.NewClusterLister(corev1listers.NewServiceLister, fakeClusterIndexer)
			remotePodLister := remotelister.NewClusterLister(corev1listers.NewPodLister, fakeClusterIndexer)

			progressingConditions, endpointSlices, err := makeEndpointSlicesForSeedService(
				tc.sc,
				tc.dc,
				tc.remoteNamespace,
				tc.remoteController,
				tc.remoteNamespaces,
				remoteServiceLister,
				remotePodLister,
				tc.managingClusterDomain,
			)

			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Fatalf("expected and got errors differ:\n%s\n", cmp.Diff(tc.expectedErr, err, cmpopts.EquateErrors()))
			}

			if !equality.Semantic.DeepEqual(progressingConditions, tc.expectedProgressingConditions) {
				t.Errorf("expected and got progressing conditions differ: %s", cmp.Diff(tc.expectedProgressingConditions, progressingConditions))
			}

			if !equality.Semantic.DeepEqual(endpointSlices, tc.expected) {
				t.Errorf("expected and got endpointslices differ: %s", cmp.Diff(tc.expected, endpointSlices))
			}
		})
	}
}

func newBasicScyllaDBCluster() *scyllav1alpha1.ScyllaDBCluster {
	return &scyllav1alpha1.ScyllaDBCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster",
			Namespace: "scylla",
			UID:       "uid",
		},
		Spec: scyllav1alpha1.ScyllaDBClusterSpec{
			Metadata:    nil,
			ClusterName: pointer.Ptr("cluster"),
			ScyllaDB: scyllav1alpha1.ScyllaDB{
				Image: "repo/scylla:version",
			},
			ScyllaDBManagerAgent: &scyllav1alpha1.ScyllaDBManagerAgent{
				Image: pointer.Ptr("repo/agent:version"),
			},
			DatacenterTemplate: &scyllav1alpha1.ScyllaDBClusterDatacenterTemplate{
				ScyllaDB: &scyllav1alpha1.ScyllaDBTemplate{
					Resources: &corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("2"),
							corev1.ResourceMemory: resource.MustParse("2Gi"),
						},
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
					},
					Storage: &scyllav1alpha1.StorageOptions{
						Capacity: "100Gi",
					},
				},
				ScyllaDBManagerAgent: &scyllav1alpha1.ScyllaDBManagerAgentTemplate{
					Resources: &corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("4"),
							corev1.ResourceMemory: resource.MustParse("4Gi"),
						},
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("3"),
							corev1.ResourceMemory: resource.MustParse("3Gi"),
						},
					},
				},
				RackTemplate: &scyllav1alpha1.RackTemplate{
					Nodes: pointer.Ptr[int32](3),
				},
				Racks: []scyllav1alpha1.RackSpec{
					{
						Name: "a",
					},
					{
						Name: "b",
					},
					{
						Name: "c",
					},
				},
			},
			Datacenters: []scyllav1alpha1.ScyllaDBClusterDatacenter{
				{
					Name:                        "dc1",
					RemoteKubernetesClusterName: "dc1-rkc",
				},
			},
		},
		Status: scyllav1alpha1.ScyllaDBClusterStatus{
			Datacenters: []scyllav1alpha1.ScyllaDBClusterDatacenterStatus{
				{
					Name:  "dc1",
					Nodes: pointer.Ptr[int32](3),
				},
			},
		},
	}
}

func newFakeClusterIndexer(t *testing.T, clusterObjectMap map[string][]apimachineryruntime.Object) func(name string) cache.Indexer {
	t.Helper()

	newIndexer := func() cache.Indexer {
		return cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	}

	cacheMap := map[string]cache.Indexer{}
	for name, objs := range clusterObjectMap {
		clusterCache := newIndexer()

		for _, obj := range objs {
			err := clusterCache.Add(obj)
			if err != nil {
				t.Fatal(err)
			}
		}

		cacheMap[name] = clusterCache
	}

	return func(name string) cache.Indexer {
		c, ok := cacheMap[name]
		if !ok {
			return newIndexer()
		}

		return c
	}
}

func Test_mergePortSpecSlices(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name     string
		xs       [][]portSpec
		expected []portSpec
	}{
		{
			name:     "no slices",
			xs:       [][]portSpec{},
			expected: nil,
		},
		{
			name: "single empty slice",
			xs: [][]portSpec{
				{},
			},
			expected: []portSpec{},
		},
		{
			name: "single non-empty slice",
			xs: [][]portSpec{
				{
					{name: "https", port: 443, protocol: corev1.ProtocolTCP},
					{name: "http", port: 80, protocol: corev1.ProtocolTCP},
				},
			},
			expected: []portSpec{
				{name: "http", port: 80, protocol: corev1.ProtocolTCP},
				{name: "https", port: 443, protocol: corev1.ProtocolTCP},
			},
		},
		{
			name: "multiple slices with unique specs",
			xs: [][]portSpec{
				{
					{name: "http", port: 80, protocol: corev1.ProtocolTCP},
				},
				{
					{name: "https", port: 443, protocol: corev1.ProtocolTCP},
				},
				{
					{name: "dns", port: 53, protocol: corev1.ProtocolUDP},
				},
			},
			expected: []portSpec{
				{name: "dns", port: 53, protocol: corev1.ProtocolUDP},
				{name: "http", port: 80, protocol: corev1.ProtocolTCP},
				{name: "https", port: 443, protocol: corev1.ProtocolTCP},
			},
		},
		{
			name: "multiple slices with duplicate specs",
			xs: [][]portSpec{
				{
					{name: "https", port: 443, protocol: corev1.ProtocolTCP},
					{name: "http", port: 80, protocol: corev1.ProtocolTCP},
				},
				{
					{name: "https", port: 443, protocol: corev1.ProtocolTCP},
				},
			},
			expected: []portSpec{
				{name: "http", port: 80, protocol: corev1.ProtocolTCP},
				{name: "https", port: 443, protocol: corev1.ProtocolTCP},
			},
		},
		{
			name: "slice with duplicate names but different specs",
			xs: [][]portSpec{
				{
					{name: "test", port: 443, protocol: corev1.ProtocolTCP},
					{name: "test", port: 80, protocol: corev1.ProtocolTCP},
				},
			},
			expected: []portSpec{
				{name: "test", port: 443, protocol: corev1.ProtocolTCP},
			},
		},
		{
			name: "multiple slices with duplicate names but different specs",
			xs: [][]portSpec{
				{
					{name: "test", port: 80, protocol: corev1.ProtocolTCP},
				},
				{
					{name: "test", port: 443, protocol: corev1.ProtocolTCP},
				},
			},
			expected: []portSpec{
				{name: "test", port: 80, protocol: corev1.ProtocolTCP},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := mergeAndCompactPortSpecSlices(tc.xs...)
			if !reflect.DeepEqual(got, tc.expected) {
				t.Errorf("expected and got port specs differ: %s", cmp.Diff(tc.expected, got, cmpopts.EquateComparable(portSpec{})))
			}
		})
	}
}

func Test_getConfigMapsAndSecretsToMirrorForAllDCs(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name                   string
		sc                     *scyllav1alpha1.ScyllaDBCluster
		expectedSecretNames    []string
		expectedConfigMapNames []string
		expectedErr            error
	}{
		{
			name: "Operator-managed secrets to mirror only",
			sc: &scyllav1alpha1.ScyllaDBCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "scylla",
				},
			},
			expectedSecretNames: []string{
				"scylla-auth-token-1lt9p",
			},
			expectedConfigMapNames: []string{},
			expectedErr:            nil,
		},
		{
			name: "All possible secrets and configmaps from custom config",
			sc: &scyllav1alpha1.ScyllaDBCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "scylla",
				},
				Spec: scyllav1alpha1.ScyllaDBClusterSpec{
					DatacenterTemplate: &scyllav1alpha1.ScyllaDBClusterDatacenterTemplate{
						ScyllaDB: &scyllav1alpha1.ScyllaDBTemplate{
							CustomConfigMapRef: pointer.Ptr("configmap-datacenterTemplate.scyllaDB.customConfigMapRef"),
						},
						ScyllaDBManagerAgent: &scyllav1alpha1.ScyllaDBManagerAgentTemplate{
							CustomConfigSecretRef: pointer.Ptr("secret-datacenterTemplate.scyllaDBManagerAgent.customConfigSecretRef"),
						},
						RackTemplate: &scyllav1alpha1.RackTemplate{
							ScyllaDB: &scyllav1alpha1.ScyllaDBTemplate{
								CustomConfigMapRef: pointer.Ptr("configmap-datacenterTemplate.rackTemplate.scyllaDB.customConfigMapRef"),
							},
							ScyllaDBManagerAgent: &scyllav1alpha1.ScyllaDBManagerAgentTemplate{
								CustomConfigSecretRef: pointer.Ptr("secret-datacenterTemplate.rackTemplate.scyllaDBManagerAgent.customConfigSecretRef"),
							},
						},
						Racks: []scyllav1alpha1.RackSpec{
							{
								RackTemplate: scyllav1alpha1.RackTemplate{
									ScyllaDB: &scyllav1alpha1.ScyllaDBTemplate{
										CustomConfigMapRef: pointer.Ptr("configmap-datacenterTemplate.racks[].scyllaDB.customConfigMapRef"),
									},
									ScyllaDBManagerAgent: &scyllav1alpha1.ScyllaDBManagerAgentTemplate{
										CustomConfigSecretRef: pointer.Ptr("secret-datacenterTemplate.racks[].scyllaDBManagerAgent.customConfigSecretRef"),
									},
								},
							},
						},
					},
				},
			},
			expectedSecretNames: []string{
				"scylla-auth-token-1lt9p",
				"secret-datacenterTemplate.scyllaDBManagerAgent.customConfigSecretRef",
				"secret-datacenterTemplate.rackTemplate.scyllaDBManagerAgent.customConfigSecretRef",
				"secret-datacenterTemplate.racks[].scyllaDBManagerAgent.customConfigSecretRef",
			},
			expectedConfigMapNames: []string{
				"configmap-datacenterTemplate.scyllaDB.customConfigMapRef",
				"configmap-datacenterTemplate.rackTemplate.scyllaDB.customConfigMapRef",
				"configmap-datacenterTemplate.racks[].scyllaDB.customConfigMapRef",
			},
		},
		{
			name: "All possible secrets and configmaps from Volumes",
			sc: &scyllav1alpha1.ScyllaDBCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "scylla",
				},
				Spec: scyllav1alpha1.ScyllaDBClusterSpec{
					DatacenterTemplate: &scyllav1alpha1.ScyllaDBClusterDatacenterTemplate{
						ScyllaDB: &scyllav1alpha1.ScyllaDBTemplate{
							Volumes: slices.Concat(
								makeVolumesReferringSecret("secret-scylladb.volumes"),
								makeVolumesReferringConfigMap("configmap-scylladb.volumes"),
							),
						},
						ScyllaDBManagerAgent: &scyllav1alpha1.ScyllaDBManagerAgentTemplate{
							Volumes: slices.Concat(
								makeVolumesReferringSecret("secret-scyllaDBManagerAgent.volumes"),
								makeVolumesReferringConfigMap("configmap-scyllaDBManagerAgent.volumes"),
							),
						},
						RackTemplate: &scyllav1alpha1.RackTemplate{
							ScyllaDB: &scyllav1alpha1.ScyllaDBTemplate{
								Volumes: slices.Concat(
									makeVolumesReferringSecret("secret-rackTemplate.scyllaDB.volumes"),
									makeVolumesReferringConfigMap("configmap-rackTemplate.scyllaDB.volumes"),
								),
							},
							ScyllaDBManagerAgent: &scyllav1alpha1.ScyllaDBManagerAgentTemplate{
								Volumes: slices.Concat(
									makeVolumesReferringSecret("secret-rackTemplate.scyllaDBManagerAgent.volumes"),
									makeVolumesReferringConfigMap("configmap-rackTemplate.scyllaDBManagerAgent.volumes"),
								),
							},
						},
						Racks: []scyllav1alpha1.RackSpec{
							{
								RackTemplate: scyllav1alpha1.RackTemplate{
									ScyllaDB: &scyllav1alpha1.ScyllaDBTemplate{
										Volumes: slices.Concat(
											makeVolumesReferringSecret("secret-racks[].scyllaDB.volumes"),
											makeVolumesReferringConfigMap("configmap-racks[].scyllaDB.volumes"),
										),
									},
									ScyllaDBManagerAgent: &scyllav1alpha1.ScyllaDBManagerAgentTemplate{
										Volumes: slices.Concat(
											makeVolumesReferringSecret("secret-racks[].scyllaDBManagerAgent.volumes"),
											makeVolumesReferringConfigMap("configmap-racks[].scyllaDBManagerAgent.volumes"),
										),
									},
								},
							},
						},
					},
				},
			},
			expectedSecretNames: []string{
				"scylla-auth-token-1lt9p",
				"secret-scylladb.volumes",
				"secret-scyllaDBManagerAgent.volumes",
				"secret-rackTemplate.scyllaDB.volumes",
				"secret-rackTemplate.scyllaDBManagerAgent.volumes",
				"secret-racks[].scyllaDB.volumes",
				"secret-racks[].scyllaDBManagerAgent.volumes",
			},
			expectedConfigMapNames: []string{
				"configmap-scylladb.volumes",
				"configmap-scyllaDBManagerAgent.volumes",
				"configmap-rackTemplate.scyllaDB.volumes",
				"configmap-rackTemplate.scyllaDBManagerAgent.volumes",
				"configmap-racks[].scyllaDB.volumes",
				"configmap-racks[].scyllaDBManagerAgent.volumes",
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotConfigMapNames, gotSecretNames, err := getConfigMapsAndSecretsToMirrorForAllDCs(tc.sc)

			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Fatalf("expected and got errors differ: %s", cmp.Diff(tc.expectedErr, err, cmpopts.EquateErrors()))
			}

			// Sort before comparison to ensure order does not affect the test.
			sort.Strings(tc.expectedConfigMapNames)
			sort.Strings(tc.expectedSecretNames)
			sort.Strings(gotConfigMapNames)
			sort.Strings(gotSecretNames)

			if !slices.Equal(gotConfigMapNames, tc.expectedConfigMapNames) {
				t.Errorf("expected and got config map names differ: %s", cmp.Diff(gotConfigMapNames, tc.expectedConfigMapNames))
			}

			if !slices.Equal(gotSecretNames, tc.expectedSecretNames) {
				t.Errorf("expected and got secret names differ: %s", cmp.Diff(gotSecretNames, tc.expectedSecretNames))
			}
		})
	}
}

func Test_getSecretsToMirrorForDC(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name                   string
		dc                     *scyllav1alpha1.ScyllaDBClusterDatacenterTemplate
		expectedSecretNames    []string
		expectedConfigMapNames []string
	}{
		{
			name:                "No secrets nor configmaps to mirror",
			dc:                  &scyllav1alpha1.ScyllaDBClusterDatacenterTemplate{},
			expectedSecretNames: nil,
		},
		{
			name: "All possible secrets and configmaps from Volumes",
			dc: &scyllav1alpha1.ScyllaDBClusterDatacenterTemplate{
				ScyllaDB: &scyllav1alpha1.ScyllaDBTemplate{
					Volumes: slices.Concat(
						makeVolumesReferringSecret("secret-scylladb.volumes"),
						makeVolumesReferringConfigMap("configmap-scylladb.volumes"),
					),
				},
				ScyllaDBManagerAgent: &scyllav1alpha1.ScyllaDBManagerAgentTemplate{
					Volumes: slices.Concat(
						makeVolumesReferringSecret("secret-scyllaDBManagerAgent.volumes"),
						makeVolumesReferringConfigMap("configmap-scyllaDBManagerAgent.volumes"),
					),
				},
				RackTemplate: &scyllav1alpha1.RackTemplate{
					ScyllaDB: &scyllav1alpha1.ScyllaDBTemplate{
						Volumes: slices.Concat(
							makeVolumesReferringSecret("secret-rackTemplate.scyllaDB.volumes"),
							makeVolumesReferringConfigMap("configmap-rackTemplate.scyllaDB.volumes"),
						),
					},
					ScyllaDBManagerAgent: &scyllav1alpha1.ScyllaDBManagerAgentTemplate{
						Volumes: slices.Concat(
							makeVolumesReferringSecret("secret-rackTemplate.scyllaDBManagerAgent.volumes"),
							makeVolumesReferringConfigMap("configmap-rackTemplate.scyllaDBManagerAgent.volumes"),
						),
					},
				},
				Racks: []scyllav1alpha1.RackSpec{
					{
						RackTemplate: scyllav1alpha1.RackTemplate{
							ScyllaDB: &scyllav1alpha1.ScyllaDBTemplate{
								Volumes: slices.Concat(
									makeVolumesReferringSecret("secret-racks[].scyllaDB.volumes"),
									makeVolumesReferringConfigMap("configmap-racks[].scyllaDB.volumes"),
								),
							},
							ScyllaDBManagerAgent: &scyllav1alpha1.ScyllaDBManagerAgentTemplate{
								Volumes: slices.Concat(
									makeVolumesReferringSecret("secret-racks[].scyllaDBManagerAgent.volumes"),
									makeVolumesReferringConfigMap("configmap-racks[].scyllaDBManagerAgent.volumes"),
								),
							},
						},
					},
				},
			},
			expectedSecretNames: []string{
				"secret-scylladb.volumes",
				"secret-scyllaDBManagerAgent.volumes",
				"secret-rackTemplate.scyllaDB.volumes",
				"secret-rackTemplate.scyllaDBManagerAgent.volumes",
				"secret-racks[].scyllaDB.volumes",
				"secret-racks[].scyllaDBManagerAgent.volumes",
			},
			expectedConfigMapNames: []string{
				"configmap-scylladb.volumes",
				"configmap-scyllaDBManagerAgent.volumes",
				"configmap-rackTemplate.scyllaDB.volumes",
				"configmap-rackTemplate.scyllaDBManagerAgent.volumes",
				"configmap-racks[].scyllaDB.volumes",
				"configmap-racks[].scyllaDBManagerAgent.volumes",
			},
		},
		{
			name: "All possible secrets from CustomConfigSecretRef",
			dc: &scyllav1alpha1.ScyllaDBClusterDatacenterTemplate{
				ScyllaDBManagerAgent: &scyllav1alpha1.ScyllaDBManagerAgentTemplate{
					CustomConfigSecretRef: pointer.Ptr("secret-scyllaDBManagerAgent.customConfigSecretRef"),
				},
				RackTemplate: &scyllav1alpha1.RackTemplate{
					ScyllaDBManagerAgent: &scyllav1alpha1.ScyllaDBManagerAgentTemplate{
						CustomConfigSecretRef: pointer.Ptr("secret-rackTemplate.scyllaDBManagerAgent.customConfigSecretRef"),
					},
				},
				Racks: []scyllav1alpha1.RackSpec{
					{
						RackTemplate: scyllav1alpha1.RackTemplate{
							ScyllaDBManagerAgent: &scyllav1alpha1.ScyllaDBManagerAgentTemplate{
								CustomConfigSecretRef: pointer.Ptr("secret-racks[].scyllaDBManagerAgent.customConfigSecretRef"),
							},
						},
					},
				},
			},
			expectedSecretNames: []string{
				"secret-scyllaDBManagerAgent.customConfigSecretRef",
				"secret-rackTemplate.scyllaDBManagerAgent.customConfigSecretRef",
				"secret-racks[].scyllaDBManagerAgent.customConfigSecretRef",
			},
			expectedConfigMapNames: []string{},
		},
		{
			name: "All possible configmaps from CustomConfigMapRef",
			dc: &scyllav1alpha1.ScyllaDBClusterDatacenterTemplate{
				ScyllaDB: &scyllav1alpha1.ScyllaDBTemplate{
					CustomConfigMapRef: pointer.Ptr("configmap-scyllaDB.customConfigMapRef"),
				},
				RackTemplate: &scyllav1alpha1.RackTemplate{
					ScyllaDB: &scyllav1alpha1.ScyllaDBTemplate{
						CustomConfigMapRef: pointer.Ptr("configmap-rackTemplate.scyllaDB.customConfigMapRef"),
					},
				},
				Racks: []scyllav1alpha1.RackSpec{
					{
						RackTemplate: scyllav1alpha1.RackTemplate{
							ScyllaDB: &scyllav1alpha1.ScyllaDBTemplate{
								CustomConfigMapRef: pointer.Ptr("configmap-racks[].scyllaDB.customConfigMapRef"),
							},
						},
					},
				},
			},
			expectedSecretNames: []string{},
			expectedConfigMapNames: []string{
				"configmap-scyllaDB.customConfigMapRef",
				"configmap-rackTemplate.scyllaDB.customConfigMapRef",
				"configmap-racks[].scyllaDB.customConfigMapRef",
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotConfigMapNames, gotSecretNames := getConfigMapsAndSecretsToMirrorForDC(tc.dc)

			// Sort before comparison to ensure order does not affect the test.
			sort.Strings(tc.expectedConfigMapNames)
			sort.Strings(tc.expectedSecretNames)
			sort.Strings(gotConfigMapNames)
			sort.Strings(gotSecretNames)

			if !slices.Equal(gotConfigMapNames, tc.expectedConfigMapNames) {
				t.Errorf("expected and got config map names differ: %s", cmp.Diff(gotConfigMapNames, tc.expectedConfigMapNames))
			}
			if !slices.Equal(gotSecretNames, tc.expectedSecretNames) {
				t.Errorf("expected and got secret names differ: %s", cmp.Diff(gotSecretNames, tc.expectedSecretNames))
			}
		})
	}
}

func Test_collectSecretNamesFromVolumes(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name     string
		volumes  []corev1.Volume
		expected []string
	}{
		{
			name:     "No volumes",
			volumes:  []corev1.Volume{},
			expected: []string{},
		},
		{
			name: "Volumes with secret",
			volumes: []corev1.Volume{
				{
					Name: "test-volume",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: "s1",
						},
					},
				},
				{
					Name: "test-volume",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: "s2",
						},
					},
				},
			},
			expected: []string{"s1", "s2"},
		},
		{
			name: "Volume with no secret",
			volumes: []corev1.Volume{
				{
					Name: "test-volume",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			},
			expected: []string{},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := collectSecretNamesFromVolumes(tc.volumes)

			// Sort before comparison to ensure order does not affect the test.
			sort.Strings(tc.expected)
			sort.Strings(got)

			if !slices.Equal(got, tc.expected) {
				t.Errorf("expected and got secret names differ: %s", cmp.Diff(tc.expected, got))
			}
		})
	}
}

func makeVolumesReferringSecret(secretName string) []corev1.Volume {
	return []corev1.Volume{
		{
			Name: "test-volume",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secretName,
				},
			},
		},
	}
}

func makeVolumesReferringConfigMap(configMapName string) []corev1.Volume {
	return []corev1.Volume{
		{
			Name: "test-volume",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: configMapName,
					},
				},
			},
		},
	}
}

func Test_makeLocalScyllaDBManagerAgentAuthTokenSecret(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name        string
		sc          *scyllav1alpha1.ScyllaDBCluster
		authToken   string
		expected    *corev1.Secret
		expectedErr error
	}{
		{
			name:      "basic",
			sc:        newBasicScyllaDBCluster(),
			authToken: "test-auth-token",
			expected: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-auth-token-2s75a",
					Namespace: "scylla",
					Labels: map[string]string{
						"app.kubernetes.io/name":                            "scylla.scylladb.com",
						"app.kubernetes.io/managed-by":                      "scylla-operator.scylladb.com",
						"scylla-operator.scylladb.com/scylladbcluster-name": "cluster",
					},
					Annotations: map[string]string{},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "ScyllaDBCluster",
							Name:               "cluster",
							UID:                "uid",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{
					"auth-token.yaml": []byte("auth_token: test-auth-token\n"),
				},
			},
			expectedErr: nil,
		},
		{
			name: "with custom labels",
			sc: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newBasicScyllaDBCluster()
				sc.Spec.Metadata = &scyllav1alpha1.ObjectTemplateMetadata{
					Labels: map[string]string{
						"custom-label1": "value1",
						"custom-label2": "value2",
					},
				}
				return sc
			}(),
			authToken: "test-auth-token",
			expected: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-auth-token-2s75a",
					Namespace: "scylla",
					Labels: map[string]string{
						"app.kubernetes.io/name":                            "scylla.scylladb.com",
						"app.kubernetes.io/managed-by":                      "scylla-operator.scylladb.com",
						"scylla-operator.scylladb.com/scylladbcluster-name": "cluster",
						"custom-label1":                                     "value1",
						"custom-label2":                                     "value2",
					},
					Annotations: map[string]string{},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "ScyllaDBCluster",
							Name:               "cluster",
							UID:                "uid",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{
					"auth-token.yaml": []byte("auth_token: test-auth-token\n"),
				},
			},
			expectedErr: nil,
		},
		{
			name: "with custom annotations",
			sc: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newBasicScyllaDBCluster()
				sc.Spec.Metadata = &scyllav1alpha1.ObjectTemplateMetadata{
					Annotations: map[string]string{
						"custom-annotation1": "value1",
						"custom-annotation2": "value2",
					},
				}
				return sc
			}(),
			authToken: "test-auth-token",
			expected: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-auth-token-2s75a",
					Namespace: "scylla",
					Labels: map[string]string{
						"app.kubernetes.io/name":                            "scylla.scylladb.com",
						"app.kubernetes.io/managed-by":                      "scylla-operator.scylladb.com",
						"scylla-operator.scylladb.com/scylladbcluster-name": "cluster",
					},
					Annotations: map[string]string{
						"custom-annotation1": "value1",
						"custom-annotation2": "value2",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "ScyllaDBCluster",
							Name:               "cluster",
							UID:                "uid",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{
					"auth-token.yaml": []byte("auth_token: test-auth-token\n"),
				},
			},
			expectedErr: nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := makeLocalScyllaDBManagerAgentAuthTokenSecret(tc.sc, tc.authToken)

			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Fatalf("expected and got errors differ:\n%s\n", cmp.Diff(tc.expectedErr, err, cmpopts.EquateErrors()))
			}

			if !apiequality.Semantic.DeepEqual(got, tc.expected) {
				t.Errorf("expected and got secret differ:\n%s\n", cmp.Diff(tc.expected, got))
			}
		})
	}
}

func Test_makeExternalScyllaDBDatacenterNodesStatusReports(t *testing.T) {
	t.Parallel()

	newMockMultiDCScyllaDBCluster := func() *scyllav1alpha1.ScyllaDBCluster {
		return &scyllav1alpha1.ScyllaDBCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cluster",
				Namespace: "scylla",
			},
			Spec: scyllav1alpha1.ScyllaDBClusterSpec{
				Datacenters: []scyllav1alpha1.ScyllaDBClusterDatacenter{
					{
						Name:                        "dc1",
						RemoteKubernetesClusterName: "dc1-rkc",
					},
					{
						Name:                        "dc2",
						RemoteKubernetesClusterName: "dc2-rkc",
					},
					{
						Name:                        "dc3",
						RemoteKubernetesClusterName: "dc3-rkc",
					},
				},
			},
		}
	}

	newMockScyllaDBClusterDatacenter := func() *scyllav1alpha1.ScyllaDBClusterDatacenter {
		return &scyllav1alpha1.ScyllaDBClusterDatacenter{
			Name:                        "dc1",
			RemoteKubernetesClusterName: "dc1-rkc",
		}
	}

	newMockScyllaDBClusterDatacenterRemoteNamespace := func() *corev1.Namespace {
		return &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "dc1-rkc-remote-namespace",
			},
		}
	}

	newMockRemoteController := func() *scyllav1alpha1.RemoteOwner {
		return &scyllav1alpha1.RemoteOwner{
			ObjectMeta: metav1.ObjectMeta{
				Name: "remote-owner",
				UID:  "1234",
			},
		}
	}

	newMockRemoteNamespaces := func() map[string]*corev1.Namespace {
		return map[string]*corev1.Namespace{
			"dc1-rkc": {
				ObjectMeta: metav1.ObjectMeta{
					Name: "dc1-rkc-remote-namespace",
				},
			},
			"dc2-rkc": {
				ObjectMeta: metav1.ObjectMeta{
					Name: "dc2-rkc-remote-namespace",
				},
			},
			"dc3-rkc": {
				ObjectMeta: metav1.ObjectMeta{
					Name: "dc3-rkc-remote-namespace",
				},
			},
		}
	}

	newMockRemoteScyllaDBDatacenters := func() map[string]map[string]*scyllav1alpha1.ScyllaDBDatacenter {
		return map[string]map[string]*scyllav1alpha1.ScyllaDBDatacenter{
			"dc1-rkc": {
				"cluster-dc1": {
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-dc1",
						Namespace: "dc1-rkc-remote-namespace",
					},
				},
			},
			"dc2-rkc": {
				"cluster-dc2": {
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-dc2",
						Namespace: "dc2-rkc-remote-namespace",
					},
				},
			},
			"dc3-rkc": {
				"cluster-dc3": {
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-dc3",
						Namespace: "dc3-rkc-remote-namespace",
					},
				},
			},
		}
	}

	tt := []struct {
		name                          string
		sc                            *scyllav1alpha1.ScyllaDBCluster
		dc                            *scyllav1alpha1.ScyllaDBClusterDatacenter
		remoteNamespace               *corev1.Namespace
		remoteController              metav1.Object
		remoteNamespaces              map[string]*corev1.Namespace
		remoteScyllaDBDatacenters     map[string]map[string]*scyllav1alpha1.ScyllaDBDatacenter
		existing                      map[string][]apimachineryruntime.Object
		expected                      []*scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport
		expectedProgressingConditions []metav1.Condition
		expectedErr                   error
	}{
		{
			name: "self dc",
			sc: &scyllav1alpha1.ScyllaDBCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster",
					Namespace: "scylla",
				},
				Spec: scyllav1alpha1.ScyllaDBClusterSpec{
					Datacenters: []scyllav1alpha1.ScyllaDBClusterDatacenter{
						{
							Name:                        "dc1",
							RemoteKubernetesClusterName: "dc1-rkc",
						},
					},
				},
			},
			dc:               newMockScyllaDBClusterDatacenter(),
			remoteNamespace:  newMockScyllaDBClusterDatacenterRemoteNamespace(),
			remoteController: newMockRemoteController(),
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "dc1-rkc-remote-namespace",
					},
				},
			},
			remoteScyllaDBDatacenters:     map[string]map[string]*scyllav1alpha1.ScyllaDBDatacenter{},
			existing:                      map[string][]apimachineryruntime.Object{},
			expected:                      []*scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport{},
			expectedProgressingConditions: []metav1.Condition{},
			expectedErr:                   nil,
		},
		{
			name:             "remote namespaces of other DCs don't exist",
			sc:               newMockMultiDCScyllaDBCluster(),
			dc:               newMockScyllaDBClusterDatacenter(),
			remoteNamespace:  newMockScyllaDBClusterDatacenterRemoteNamespace(),
			remoteController: newMockRemoteController(),
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "dc1-rkc-remote-namespace",
					},
				},
			},
			remoteScyllaDBDatacenters:     map[string]map[string]*scyllav1alpha1.ScyllaDBDatacenter{},
			existing:                      map[string][]apimachineryruntime.Object{},
			expected:                      []*scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport{},
			expectedProgressingConditions: []metav1.Condition{},
			expectedErr:                   nil,
		},
		{
			name:                          "remote SDCs for other DCs don't exist",
			sc:                            newMockMultiDCScyllaDBCluster(),
			dc:                            newMockScyllaDBClusterDatacenter(),
			remoteNamespace:               newMockScyllaDBClusterDatacenterRemoteNamespace(),
			remoteController:              newMockRemoteController(),
			remoteNamespaces:              newMockRemoteNamespaces(),
			remoteScyllaDBDatacenters:     map[string]map[string]*scyllav1alpha1.ScyllaDBDatacenter{},
			existing:                      map[string][]apimachineryruntime.Object{},
			expected:                      []*scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport{},
			expectedProgressingConditions: []metav1.Condition{},
			expectedErr:                   nil,
		},
		{
			name:                      "remote SDCs exist but their ScyllaDBNodesStatusReports do not",
			sc:                        newMockMultiDCScyllaDBCluster(),
			dc:                        newMockScyllaDBClusterDatacenter(),
			remoteNamespace:           newMockScyllaDBClusterDatacenterRemoteNamespace(),
			remoteController:          newMockRemoteController(),
			remoteNamespaces:          newMockRemoteNamespaces(),
			remoteScyllaDBDatacenters: newMockRemoteScyllaDBDatacenters(),
			existing:                  map[string][]apimachineryruntime.Object{},
			expected:                  []*scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport{},
			expectedProgressingConditions: []metav1.Condition{
				{
					Type:    "RemoteScyllaDBDatacenterNodesStatusReportControllerDatacenterdc1Progressing",
					Status:  "True",
					Reason:  "WaitingForRemoteScyllaDBDatacenterNodesStatusReport",
					Message: `Waiting for ScyllaDBDatacenterNodesStatusReport "dc2-rkc-remote-namespace/cluster-dc2-2n4hb" to be created in "dc2-rkc" Cluster`,
				},
				{
					Type:    "RemoteScyllaDBDatacenterNodesStatusReportControllerDatacenterdc1Progressing",
					Status:  "True",
					Reason:  "WaitingForRemoteScyllaDBDatacenterNodesStatusReport",
					Message: `Waiting for ScyllaDBDatacenterNodesStatusReport "dc3-rkc-remote-namespace/cluster-dc3-11k4m" to be created in "dc3-rkc" Cluster`,
				},
			},
			expectedErr: nil,
		},
		{
			name:                      "remote SDCs and their ScyllaDBNodesStatusReports exist",
			sc:                        newMockMultiDCScyllaDBCluster(),
			dc:                        newMockScyllaDBClusterDatacenter(),
			remoteNamespace:           newMockScyllaDBClusterDatacenterRemoteNamespace(),
			remoteController:          newMockRemoteController(),
			remoteNamespaces:          newMockRemoteNamespaces(),
			remoteScyllaDBDatacenters: newMockRemoteScyllaDBDatacenters(),
			existing: map[string][]apimachineryruntime.Object{
				"dc1-rkc": {
					&scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "cluster-dc1-7p9ut",
							Namespace: "dc1-rkc-remote-namespace",
						},
						DatacenterName: "dc1",
						Racks: []scyllav1alpha1.RackNodesStatusReport{
							{
								Name: "rack1",
								Nodes: []scyllav1alpha1.NodeStatusReport{
									{
										Ordinal: 0,
										HostID:  pointer.Ptr("host-id-dc1-rack1-0"),
										ObservedNodes: []scyllav1alpha1.ObservedNodeStatus{
											{
												HostID: "host-id-dc1-rack1-0",
												Status: scyllav1alpha1.NodeStatusUp,
											},
											{
												HostID: "host-id-dc2-rack1-0",
												Status: scyllav1alpha1.NodeStatusUp,
											},
											{
												HostID: "host-id-dc3-rack1-0",
												Status: scyllav1alpha1.NodeStatusUp,
											},
										},
									},
								},
							},
						},
					},
				},
				"dc2-rkc": {
					&scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "cluster-dc2-2n4hb",
							Namespace: "dc2-rkc-remote-namespace",
						},
						DatacenterName: "dc2",
						Racks: []scyllav1alpha1.RackNodesStatusReport{
							{
								Name: "rack1",
								Nodes: []scyllav1alpha1.NodeStatusReport{
									{
										Ordinal: 0,
										HostID:  pointer.Ptr("host-id-dc2-rack1-0"),
										ObservedNodes: []scyllav1alpha1.ObservedNodeStatus{
											{
												HostID: "host-id-dc1-rack1-0",
												Status: scyllav1alpha1.NodeStatusUp,
											},
											{
												HostID: "host-id-dc2-rack1-0",
												Status: scyllav1alpha1.NodeStatusUp,
											},
											{
												HostID: "host-id-dc3-rack1-0",
												Status: scyllav1alpha1.NodeStatusUp,
											},
										},
									},
								},
							},
						},
					},
				},
				"dc3-rkc": {
					&scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "cluster-dc3-11k4m",
							Namespace: "dc3-rkc-remote-namespace",
						},
						DatacenterName: "dc3",
						Racks: []scyllav1alpha1.RackNodesStatusReport{
							{
								Name: "rack1",
								Nodes: []scyllav1alpha1.NodeStatusReport{
									{
										Ordinal: 0,
										HostID:  pointer.Ptr("host-id-dc3-rack1-0"),
										ObservedNodes: []scyllav1alpha1.ObservedNodeStatus{
											{
												HostID: "host-id-dc1-rack1-0",
												Status: scyllav1alpha1.NodeStatusUp,
											},
											{
												HostID: "host-id-dc2-rack1-0",
												Status: scyllav1alpha1.NodeStatusUp,
											},
											{
												HostID: "host-id-dc3-rack1-0",
												Status: scyllav1alpha1.NodeStatusUp,
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expected: []*scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-dc2-external-hglri",
						Namespace: "dc1-rkc-remote-namespace",
						Labels: map[string]string{
							"app.kubernetes.io/managed-by":                                                        "remote.scylla-operator.scylladb.com",
							"scylla-operator.scylladb.com/managed-by-cluster":                                     "test-cluster.local",
							"scylla-operator.scylladb.com/parent-scylladbcluster-datacenter-name":                 "dc1",
							"scylla-operator.scylladb.com/parent-scylladbcluster-name":                            "cluster",
							"scylla-operator.scylladb.com/parent-scylladbcluster-namespace":                       "scylla",
							"scylla-operator.scylladb.com/remote-cluster-scylladb-datacenter-nodes-status-report": "cluster",
							"scylla-operator.scylladb.com/scylladb-datacenter-nodes-status-report-selector":       "cluster-dc1",
						},
						Annotations: map[string]string{},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "scylla.scylladb.com/v1alpha1",
								Kind:               "RemoteOwner",
								Name:               "remote-owner",
								UID:                "1234",
								Controller:         pointer.Ptr(true),
								BlockOwnerDeletion: pointer.Ptr(true),
							},
						},
					},
					DatacenterName: "dc2",
					Racks: []scyllav1alpha1.RackNodesStatusReport{
						{
							Name: "rack1",
							Nodes: []scyllav1alpha1.NodeStatusReport{
								{
									Ordinal: 0,
									HostID:  pointer.Ptr("host-id-dc2-rack1-0"),
									ObservedNodes: []scyllav1alpha1.ObservedNodeStatus{
										{
											HostID: "host-id-dc1-rack1-0",
											Status: scyllav1alpha1.NodeStatusUp,
										},
										{
											HostID: "host-id-dc2-rack1-0",
											Status: scyllav1alpha1.NodeStatusUp,
										},
										{
											HostID: "host-id-dc3-rack1-0",
											Status: scyllav1alpha1.NodeStatusUp,
										},
									},
								},
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-dc3-external-617i6",
						Namespace: "dc1-rkc-remote-namespace",
						Labels: map[string]string{
							"app.kubernetes.io/managed-by":                                                        "remote.scylla-operator.scylladb.com",
							"scylla-operator.scylladb.com/managed-by-cluster":                                     "test-cluster.local",
							"scylla-operator.scylladb.com/parent-scylladbcluster-datacenter-name":                 "dc1",
							"scylla-operator.scylladb.com/parent-scylladbcluster-name":                            "cluster",
							"scylla-operator.scylladb.com/parent-scylladbcluster-namespace":                       "scylla",
							"scylla-operator.scylladb.com/remote-cluster-scylladb-datacenter-nodes-status-report": "cluster",
							"scylla-operator.scylladb.com/scylladb-datacenter-nodes-status-report-selector":       "cluster-dc1",
						},
						Annotations: map[string]string{},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "scylla.scylladb.com/v1alpha1",
								Kind:               "RemoteOwner",
								Name:               "remote-owner",
								UID:                "1234",
								Controller:         pointer.Ptr(true),
								BlockOwnerDeletion: pointer.Ptr(true),
							},
						},
					},
					DatacenterName: "dc3",
					Racks: []scyllav1alpha1.RackNodesStatusReport{
						{
							Name: "rack1",
							Nodes: []scyllav1alpha1.NodeStatusReport{
								{
									Ordinal: 0,
									HostID:  pointer.Ptr("host-id-dc3-rack1-0"),
									ObservedNodes: []scyllav1alpha1.ObservedNodeStatus{
										{
											HostID: "host-id-dc1-rack1-0",
											Status: scyllav1alpha1.NodeStatusUp,
										},
										{
											HostID: "host-id-dc2-rack1-0",
											Status: scyllav1alpha1.NodeStatusUp,
										},
										{
											HostID: "host-id-dc3-rack1-0",
											Status: scyllav1alpha1.NodeStatusUp,
										},
									},
								},
							},
						},
					},
				},
			},
			expectedProgressingConditions: []metav1.Condition{},
			expectedErr:                   nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fakeClusterIndexer := newFakeClusterIndexer(t, tc.existing)
			remoteScyllaDBDatacenterNodesStatusReportLister := remotelister.NewClusterLister(scyllav1alpha1listers.NewScyllaDBDatacenterNodesStatusReportLister, fakeClusterIndexer)

			progressingConditions, scyllaDBDatacenterNodesStatusReports, err := makeExternalScyllaDBDatacenterNodesStatusReports(
				tc.sc,
				tc.dc,
				tc.remoteNamespace,
				tc.remoteController,
				tc.remoteNamespaces,
				tc.remoteScyllaDBDatacenters,
				remoteScyllaDBDatacenterNodesStatusReportLister,
				testClusterDomain,
			)

			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Fatalf("expected and got errors differ:\n%s\n", cmp.Diff(tc.expectedErr, err, cmpopts.EquateErrors()))
			}

			if !equality.Semantic.DeepEqual(progressingConditions, tc.expectedProgressingConditions) {
				t.Errorf("expected and got progressing conditions differ: %s", cmp.Diff(tc.expectedProgressingConditions, progressingConditions))
			}

			if !equality.Semantic.DeepEqual(scyllaDBDatacenterNodesStatusReports, tc.expected) {
				t.Errorf("expected and got ScyllaDBDatacenterNodesStatusReports differ: %s", cmp.Diff(tc.expected, scyllaDBDatacenterNodesStatusReports))
			}
		})
	}
}
