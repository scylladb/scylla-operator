package scylladbcluster

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	testClusterDomain = "test-cluster.local"
)

func TestMakeRemoteOwners(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name                          string
		cluster                       *scyllav1alpha1.ScyllaDBCluster
		remoteNamespaces              map[string]*corev1.Namespace
		existingRemoteOwners          map[string]map[string]*scyllav1alpha1.RemoteOwner
		expectedRemoteOwners          map[string][]*scyllav1alpha1.RemoteOwner
		expectedProgressingConditions []metav1.Condition
	}{
		{
			name: "RemoteOwner in each cluster datacenter",
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
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "scylla-abc",
					},
				},
				"dc2-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "scylla-def",
					},
				},
				"dc3-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "scylla-ghj",
					},
				},
			},
			existingRemoteOwners: nil,
			expectedRemoteOwners: map[string][]*scyllav1alpha1.RemoteOwner{
				"dc1-rkc": {
					&scyllav1alpha1.RemoteOwner{
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
				"dc2-rkc": {
					&scyllav1alpha1.RemoteOwner{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: "scylla-def",
							Name:      "cluster-1513j",
							Labels: map[string]string{
								"internal.scylla-operator.scylladb.com/remote-owner-cluster":   "dc2-rkc",
								"internal.scylla-operator.scylladb.com/remote-owner-namespace": "scylla",
								"internal.scylla-operator.scylladb.com/remote-owner-name":      "cluster",
								"internal.scylla-operator.scylladb.com/remote-owner-gvr":       "scylla.scylladb.com-v1alpha1-scylladbclusters",
								"scylla-operator.scylladb.com/managed-by-cluster":              "test-cluster.local",
								"app.kubernetes.io/managed-by":                                 "remote.scylla-operator.scylladb.com",
							},
						},
					},
				},
				"dc3-rkc": {
					&scyllav1alpha1.RemoteOwner{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: "scylla-ghj",
							Name:      "cluster-2p77z",
							Labels: map[string]string{
								"internal.scylla-operator.scylladb.com/remote-owner-cluster":   "dc3-rkc",
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
			expectedProgressingConditions: nil,
		},
		{
			name: "Progressing condition when Namespace is not yet created",
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
			remoteNamespaces:     map[string]*corev1.Namespace{},
			existingRemoteOwners: map[string]map[string]*scyllav1alpha1.RemoteOwner{},
			expectedRemoteOwners: map[string][]*scyllav1alpha1.RemoteOwner{},
			expectedProgressingConditions: []metav1.Condition{
				{
					Type:    "RemoteRemoteOwnerControllerProgressing",
					Status:  "True",
					Reason:  "WaitingForRemoteNamespace",
					Message: `Waiting for Namespace to be created in "dc1-rkc" Cluster`,
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotProgressingConditions, gotRemoteOwners, err := MakeRemoteRemoteOwners(tc.cluster, tc.remoteNamespaces, tc.existingRemoteOwners, testClusterDomain)
			if err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
			if !equality.Semantic.DeepEqual(gotRemoteOwners, tc.expectedRemoteOwners) {
				t.Errorf("expected and got remoteowners differ, diff: %s", cmp.Diff(gotRemoteOwners, tc.expectedRemoteOwners))
			}
			if !equality.Semantic.DeepEqual(gotProgressingConditions, tc.expectedProgressingConditions) {
				t.Errorf("expected and got progressing conditions differ, diff: %s", cmp.Diff(gotProgressingConditions, tc.expectedProgressingConditions))
			}
		})
	}
}

func TestMakeNamespaces(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name               string
		cluster            *scyllav1alpha1.ScyllaDBCluster
		existingNamespaces map[string]map[string]*corev1.Namespace
		expectedNamespaces map[string][]*corev1.Namespace
	}{
		{
			name: "namespace in each cluster datacenter",
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
			existingNamespaces: map[string]map[string]*corev1.Namespace{},
			expectedNamespaces: map[string][]*corev1.Namespace{
				"dc1-rkc": {{
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
				"dc2-rkc": {{
					ObjectMeta: metav1.ObjectMeta{
						Name: "scylla-1513j",
						Labels: map[string]string{
							"scylla-operator.scylladb.com/parent-scylladbcluster-name":            "cluster",
							"scylla-operator.scylladb.com/parent-scylladbcluster-namespace":       "scylla",
							"scylla-operator.scylladb.com/parent-scylladbcluster-datacenter-name": "dc2",
							"scylla-operator.scylladb.com/managed-by-cluster":                     "test-cluster.local",
							"app.kubernetes.io/managed-by":                                        "remote.scylla-operator.scylladb.com",
						},
					},
				}},
				"dc3-rkc": {{
					ObjectMeta: metav1.ObjectMeta{
						Name: "scylla-2p77z",
						Labels: map[string]string{
							"scylla-operator.scylladb.com/parent-scylladbcluster-name":            "cluster",
							"scylla-operator.scylladb.com/parent-scylladbcluster-namespace":       "scylla",
							"scylla-operator.scylladb.com/parent-scylladbcluster-datacenter-name": "dc3",
							"scylla-operator.scylladb.com/managed-by-cluster":                     "test-cluster.local",
							"app.kubernetes.io/managed-by":                                        "remote.scylla-operator.scylladb.com",
						},
					},
				}},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotNamespaces, err := MakeRemoteNamespaces(tc.cluster, tc.existingNamespaces, testClusterDomain)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !equality.Semantic.DeepEqual(gotNamespaces, tc.expectedNamespaces) {
				t.Errorf("expected and got namespaces differ, diff: %s", cmp.Diff(gotNamespaces, tc.expectedNamespaces))
			}
		})
	}
}

func TestMakeServices(t *testing.T) {
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
		name                          string
		cluster                       *scyllav1alpha1.ScyllaDBCluster
		remoteNamespaces              map[string]*corev1.Namespace
		remoteControllers             map[string]metav1.Object
		expectedServices              map[string][]*corev1.Service
		expectedProgressingConditions []metav1.Condition
	}{
		{
			name: "progressing condition when remote Namespace is not yet created",
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
			remoteNamespaces: map[string]*corev1.Namespace{},
			remoteControllers: map[string]metav1.Object{
				"dc1-rkc": &scyllav1alpha1.RemoteOwner{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-def",
						Namespace: "scylla-abc",
						UID:       "1234",
					},
				},
			},
			expectedServices: nil,
			expectedProgressingConditions: []metav1.Condition{
				{
					Type:    "RemoteServiceControllerProgressing",
					Status:  "True",
					Reason:  "WaitingForRemoteNamespace",
					Message: `Waiting for Namespace to be created in "dc1-rkc" Cluster`,
				},
			},
		},
		{
			name: "progressing condition when remote controller is not yet created",
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
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "scylla-abc",
					},
				},
			},
			remoteControllers: map[string]metav1.Object{},
			expectedServices:  nil,
			expectedProgressingConditions: []metav1.Condition{
				{
					Type:    "RemoteServiceControllerProgressing",
					Status:  "True",
					Reason:  "WaitingForRemoteController",
					Message: `Waiting for controller object to be created in "dc1-rkc" Cluster`,
				},
			},
		},
		{
			name: "no seed services for single datacenter cluster",
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
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "scylla-abc",
					},
				},
			},
			remoteControllers: map[string]metav1.Object{
				"dc1-rkc": &scyllav1alpha1.RemoteOwner{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-def",
						Namespace: "scylla-abc",
						UID:       "1234",
					},
				},
			},
			expectedServices:              nil,
			expectedProgressingConditions: nil,
		},
		{
			name: "cross datacenter seed services in each cluster datacenter",
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
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "scylla-abc",
					},
				},
				"dc2-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "scylla-def",
					},
				},
				"dc3-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "scylla-ghj",
					},
				},
			},
			remoteControllers: map[string]metav1.Object{
				"dc1-rkc": &scyllav1alpha1.RemoteOwner{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-xxx",
						Namespace: "scylla-abc",
						UID:       "1111",
					},
				},
				"dc2-rkc": &scyllav1alpha1.RemoteOwner{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-yyy",
						Namespace: "scylla-def",
						UID:       "2222",
					},
				},
				"dc3-rkc": &scyllav1alpha1.RemoteOwner{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-zzz",
						Namespace: "scylla-ghj",
						UID:       "3333",
					},
				},
			},
			expectedServices: map[string][]*corev1.Service{
				"dc1-rkc": {
					&corev1.Service{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "cluster-dc2-seed",
							Namespace: "scylla-abc",
							Labels: map[string]string{
								"scylla-operator.scylladb.com/parent-scylladbcluster-name":            "cluster",
								"scylla-operator.scylladb.com/parent-scylladbcluster-namespace":       "scylla",
								"scylla-operator.scylladb.com/parent-scylladbcluster-datacenter-name": "dc1",
								"scylla-operator.scylladb.com/managed-by-cluster":                     "test-cluster.local",
								"app.kubernetes.io/managed-by":                                        "remote.scylla-operator.scylladb.com",
							},
							OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(&scyllav1alpha1.RemoteOwner{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "cluster-xxx",
									Namespace: "scylla-abc",
									UID:       "1111",
								},
							},
								remoteControllerGVK,
							)},
						},
						Spec: makeSeedServiceSpec(),
					},
					&corev1.Service{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "cluster-dc3-seed",
							Namespace: "scylla-abc",
							Labels: map[string]string{
								"scylla-operator.scylladb.com/parent-scylladbcluster-name":            "cluster",
								"scylla-operator.scylladb.com/parent-scylladbcluster-namespace":       "scylla",
								"scylla-operator.scylladb.com/parent-scylladbcluster-datacenter-name": "dc1",
								"scylla-operator.scylladb.com/managed-by-cluster":                     "test-cluster.local",
								"app.kubernetes.io/managed-by":                                        "remote.scylla-operator.scylladb.com",
							},
							OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(&scyllav1alpha1.RemoteOwner{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "cluster-xxx",
									Namespace: "scylla-abc",
									UID:       "1111",
								},
							},
								remoteControllerGVK,
							)},
						},
						Spec: makeSeedServiceSpec(),
					},
				},
				"dc2-rkc": {
					&corev1.Service{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "cluster-dc1-seed",
							Namespace: "scylla-def",
							Labels: map[string]string{
								"scylla-operator.scylladb.com/parent-scylladbcluster-name":            "cluster",
								"scylla-operator.scylladb.com/parent-scylladbcluster-namespace":       "scylla",
								"scylla-operator.scylladb.com/parent-scylladbcluster-datacenter-name": "dc2",
								"scylla-operator.scylladb.com/managed-by-cluster":                     "test-cluster.local",
								"app.kubernetes.io/managed-by":                                        "remote.scylla-operator.scylladb.com",
							},
							OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(&scyllav1alpha1.RemoteOwner{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "cluster-yyy",
									Namespace: "scylla-def",
									UID:       "2222",
								},
							},
								remoteControllerGVK,
							)},
						},
						Spec: makeSeedServiceSpec(),
					},
					&corev1.Service{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "cluster-dc3-seed",
							Namespace: "scylla-def",
							Labels: map[string]string{
								"scylla-operator.scylladb.com/parent-scylladbcluster-name":            "cluster",
								"scylla-operator.scylladb.com/parent-scylladbcluster-namespace":       "scylla",
								"scylla-operator.scylladb.com/parent-scylladbcluster-datacenter-name": "dc2",
								"scylla-operator.scylladb.com/managed-by-cluster":                     "test-cluster.local",
								"app.kubernetes.io/managed-by":                                        "remote.scylla-operator.scylladb.com",
							},
							OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(&scyllav1alpha1.RemoteOwner{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "cluster-yyy",
									Namespace: "scylla-def",
									UID:       "2222",
								},
							},
								remoteControllerGVK,
							)},
						},
						Spec: makeSeedServiceSpec(),
					},
				},
				"dc3-rkc": {
					&corev1.Service{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "cluster-dc1-seed",
							Namespace: "scylla-ghj",
							Labels: map[string]string{
								"scylla-operator.scylladb.com/parent-scylladbcluster-name":            "cluster",
								"scylla-operator.scylladb.com/parent-scylladbcluster-namespace":       "scylla",
								"scylla-operator.scylladb.com/parent-scylladbcluster-datacenter-name": "dc3",
								"scylla-operator.scylladb.com/managed-by-cluster":                     "test-cluster.local",
								"app.kubernetes.io/managed-by":                                        "remote.scylla-operator.scylladb.com",
							},
							OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(&scyllav1alpha1.RemoteOwner{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "cluster-zzz",
									Namespace: "scylla-ghj",
									UID:       "3333",
								},
							},
								remoteControllerGVK,
							)},
						},
						Spec: makeSeedServiceSpec(),
					},
					&corev1.Service{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "cluster-dc2-seed",
							Namespace: "scylla-ghj",
							Labels: map[string]string{
								"scylla-operator.scylladb.com/parent-scylladbcluster-name":            "cluster",
								"scylla-operator.scylladb.com/parent-scylladbcluster-namespace":       "scylla",
								"scylla-operator.scylladb.com/parent-scylladbcluster-datacenter-name": "dc3",
								"scylla-operator.scylladb.com/managed-by-cluster":                     "test-cluster.local",
								"app.kubernetes.io/managed-by":                                        "remote.scylla-operator.scylladb.com",
							},
							OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(&scyllav1alpha1.RemoteOwner{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "cluster-zzz",
									Namespace: "scylla-ghj",
									UID:       "3333",
								},
							},
								remoteControllerGVK,
							)},
						},
						Spec: makeSeedServiceSpec(),
					},
				},
			},
			expectedProgressingConditions: nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotProgressingConditions, gotServices := MakeRemoteServices(tc.cluster, tc.remoteNamespaces, tc.remoteControllers, testClusterDomain)
			if !equality.Semantic.DeepEqual(gotServices, tc.expectedServices) {
				t.Errorf("expected and got services differ, diff: %s", cmp.Diff(tc.expectedServices, gotServices))
			}
			if !equality.Semantic.DeepEqual(gotProgressingConditions, tc.expectedProgressingConditions) {
				t.Errorf("expected and got progressing conditions differ, diff: %s", cmp.Diff(tc.expectedProgressingConditions, gotProgressingConditions))
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

	newBasicScyllaDBCluster := func() *scyllav1alpha1.ScyllaDBCluster {
		return &scyllav1alpha1.ScyllaDBCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cluster",
				Namespace: "scylla",
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
				Annotations:     map[string]string{},
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

	tt := []struct {
		name                          string
		cluster                       *scyllav1alpha1.ScyllaDBCluster
		remoteScyllaDBDatacenters     map[string]map[string]*scyllav1alpha1.ScyllaDBDatacenter
		remoteNamespaces              map[string]*corev1.Namespace
		remoteControllers             map[string]metav1.Object
		expectedScyllaDBDatacenters   map[string][]*scyllav1alpha1.ScyllaDBDatacenter
		expectedProgressingConditions []metav1.Condition
	}{
		{
			name:    "basic single dc cluster",
			cluster: newBasicScyllaDBCluster(),
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "scylla-abc",
					},
				},
			},
			remoteControllers: map[string]metav1.Object{
				"dc1-rkc": &scyllav1alpha1.RemoteOwner{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-111",
						Namespace: "scylla-abc",
						UID:       "1234",
					},
				},
			},
			expectedScyllaDBDatacenters: map[string][]*scyllav1alpha1.ScyllaDBDatacenter{
				"dc1-rkc": {
					newBasicScyllaDBDatacenter("dc1", "scylla-abc", []string{}),
				},
			},
		},
		{
			name: "basic three dc cluster",
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
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "scylla-aaa",
					},
				},
				"dc2-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "scylla-bbb",
					},
				},
				"dc3-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "scylla-ccc",
					},
				},
			},
			remoteControllers: map[string]metav1.Object{
				"dc1-rkc": &scyllav1alpha1.RemoteOwner{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-111",
						Namespace: "scylla-aaa",
						UID:       "1234",
					},
				},
				"dc2-rkc": &scyllav1alpha1.RemoteOwner{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-111",
						Namespace: "scylla-bbb",
						UID:       "1234",
					},
				},
				"dc3-rkc": &scyllav1alpha1.RemoteOwner{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-111",
						Namespace: "scylla-ccc",
						UID:       "1234",
					},
				},
			},
			remoteScyllaDBDatacenters: map[string]map[string]*scyllav1alpha1.ScyllaDBDatacenter{
				"dc1-rkc": {
					"cluster-dc1": makeReconciledScyllaDBDatacenter(newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{"cluster-dc2-seed.scylla-aaa.svc", "cluster-dc3-seed.scylla-aaa.svc"})),
				},
				"dc2-rkc": {
					"cluster-dc2": makeReconciledScyllaDBDatacenter(newBasicScyllaDBDatacenter("dc2", "scylla-bbb", []string{"cluster-dc1-seed.scylla-bbb.svc", "cluster-dc3-seed.scylla-bbb.svc"})),
				},
				"dc3-rkc": {
					"cluster-dc3": makeReconciledScyllaDBDatacenter(newBasicScyllaDBDatacenter("dc3", "scylla-ccc", []string{"cluster-dc1-seed.scylla-ccc.svc", "cluster-dc2-seed.scylla-ccc.svc"})),
				},
			},
			expectedScyllaDBDatacenters: map[string][]*scyllav1alpha1.ScyllaDBDatacenter{
				"dc1-rkc": {
					newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{"cluster-dc2-seed.scylla-aaa.svc", "cluster-dc3-seed.scylla-aaa.svc"}),
				},
				"dc2-rkc": {
					newBasicScyllaDBDatacenter("dc2", "scylla-bbb", []string{"cluster-dc1-seed.scylla-bbb.svc", "cluster-dc3-seed.scylla-bbb.svc"}),
				},
				"dc3-rkc": {
					newBasicScyllaDBDatacenter("dc3", "scylla-ccc", []string{"cluster-dc1-seed.scylla-ccc.svc", "cluster-dc2-seed.scylla-ccc.svc"}),
				},
			},
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
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "scylla-aaa",
					},
				},
				"dc2-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "scylla-bbb",
					},
				},
				"dc3-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "scylla-ccc",
					},
				},
			},
			remoteControllers: map[string]metav1.Object{
				"dc1-rkc": &scyllav1alpha1.RemoteOwner{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-111",
						Namespace: "scylla-aaa",
						UID:       "1234",
					},
				},
				"dc2-rkc": &scyllav1alpha1.RemoteOwner{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-111",
						Namespace: "scylla-bbb",
						UID:       "1234",
					},
				},
				"dc3-rkc": &scyllav1alpha1.RemoteOwner{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-111",
						Namespace: "scylla-ccc",
						UID:       "1234",
					},
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
			expectedScyllaDBDatacenters: map[string][]*scyllav1alpha1.ScyllaDBDatacenter{
				"dc1-rkc": {
					newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{}),
				},
				"dc2-rkc": {
					newBasicScyllaDBDatacenter("dc2", "scylla-bbb", []string{}),
				},
				"dc3-rkc": {
					newBasicScyllaDBDatacenter("dc3", "scylla-ccc", []string{}),
				},
			},
		},
		{
			name: "first out of three DCs is reconciled, seeds of DC2 and DC3 should point to DC1",
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
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "scylla-aaa",
					},
				},
				"dc2-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "scylla-bbb",
					},
				},
				"dc3-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "scylla-ccc",
					},
				},
			},
			remoteControllers: map[string]metav1.Object{
				"dc1-rkc": &scyllav1alpha1.RemoteOwner{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-111",
						Namespace: "scylla-aaa",
						UID:       "1234",
					},
				},
				"dc2-rkc": &scyllav1alpha1.RemoteOwner{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-111",
						Namespace: "scylla-bbb",
						UID:       "1234",
					},
				},
				"dc3-rkc": &scyllav1alpha1.RemoteOwner{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-111",
						Namespace: "scylla-ccc",
						UID:       "1234",
					},
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
			expectedScyllaDBDatacenters: map[string][]*scyllav1alpha1.ScyllaDBDatacenter{
				"dc1-rkc": {
					newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{}),
				},
				"dc2-rkc": {
					newBasicScyllaDBDatacenter("dc2", "scylla-bbb", []string{"cluster-dc1-seed.scylla-bbb.svc"}),
				},
				"dc3-rkc": {
					newBasicScyllaDBDatacenter("dc3", "scylla-ccc", []string{"cluster-dc1-seed.scylla-ccc.svc"}),
				},
			},
		},
		{
			name: "first two out of three DCs are reconciled, seeds of non-reconciled DC should point to reconciled ones, seeds of reconciled are cross referencing reconciled ones",
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
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "scylla-aaa",
					},
				},
				"dc2-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "scylla-bbb",
					},
				},
				"dc3-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "scylla-ccc",
					},
				},
			},
			remoteControllers: map[string]metav1.Object{
				"dc1-rkc": &scyllav1alpha1.RemoteOwner{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-111",
						Namespace: "scylla-aaa",
						UID:       "1234",
					},
				},
				"dc2-rkc": &scyllav1alpha1.RemoteOwner{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-111",
						Namespace: "scylla-bbb",
						UID:       "1234",
					},
				},
				"dc3-rkc": &scyllav1alpha1.RemoteOwner{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-111",
						Namespace: "scylla-ccc",
						UID:       "1234",
					},
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
			expectedScyllaDBDatacenters: map[string][]*scyllav1alpha1.ScyllaDBDatacenter{
				"dc1-rkc": {
					newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{"cluster-dc2-seed.scylla-aaa.svc"}),
				},
				"dc2-rkc": {
					newBasicScyllaDBDatacenter("dc2", "scylla-bbb", []string{"cluster-dc1-seed.scylla-bbb.svc"}),
				},
				"dc3-rkc": {
					newBasicScyllaDBDatacenter("dc3", "scylla-ccc", []string{"cluster-dc1-seed.scylla-ccc.svc", "cluster-dc2-seed.scylla-ccc.svc"}),
				},
			},
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
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "scylla-aaa",
					},
				},
				"dc2-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "scylla-bbb",
					},
				},
				"dc3-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "scylla-ccc",
					},
				},
			},
			remoteControllers: map[string]metav1.Object{
				"dc1-rkc": &scyllav1alpha1.RemoteOwner{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-111",
						Namespace: "scylla-aaa",
						UID:       "1234",
					},
				},
				"dc2-rkc": &scyllav1alpha1.RemoteOwner{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-111",
						Namespace: "scylla-bbb",
						UID:       "1234",
					},
				},
				"dc3-rkc": &scyllav1alpha1.RemoteOwner{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-111",
						Namespace: "scylla-ccc",
						UID:       "1234",
					},
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
			expectedScyllaDBDatacenters: map[string][]*scyllav1alpha1.ScyllaDBDatacenter{
				"dc1-rkc": {
					newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{"cluster-dc2-seed.scylla-aaa.svc", "cluster-dc3-seed.scylla-aaa.svc"}),
				},
				"dc2-rkc": {
					newBasicScyllaDBDatacenter("dc2", "scylla-bbb", []string{"cluster-dc1-seed.scylla-bbb.svc", "cluster-dc3-seed.scylla-bbb.svc"}),
				},
				"dc3-rkc": {
					newBasicScyllaDBDatacenter("dc3", "scylla-ccc", []string{"cluster-dc1-seed.scylla-ccc.svc", "cluster-dc2-seed.scylla-ccc.svc"}),
				},
			},
		},
		{
			name: "metadata from ScyllaDBCluster spec are propagated into ScyllaDBDatacenter object metadata and spec metadata",
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "scylla-aaa",
					},
				},
			},
			remoteControllers: map[string]metav1.Object{
				"dc1-rkc": &scyllav1alpha1.RemoteOwner{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-111",
						Namespace: "scylla-aaa",
						UID:       "1234",
					},
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
			expectedScyllaDBDatacenters: map[string][]*scyllav1alpha1.ScyllaDBDatacenter{
				"dc1-rkc": {
					func() *scyllav1alpha1.ScyllaDBDatacenter {
						dc := newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{})
						dc.Labels["label"] = "foo"
						dc.Annotations["annotation"] = "foo"

						dc.Spec.Metadata.Labels["label"] = "foo"
						dc.Spec.Metadata.Annotations["annotation"] = "foo"
						return dc
					}(),
				},
			},
		},
		{
			name: "metadata from database template overrides one specified on cluster level",
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "scylla-aaa",
					},
				},
			},
			remoteControllers: map[string]metav1.Object{
				"dc1-rkc": &scyllav1alpha1.RemoteOwner{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-111",
						Namespace: "scylla-aaa",
						UID:       "1234",
					},
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
			expectedScyllaDBDatacenters: map[string][]*scyllav1alpha1.ScyllaDBDatacenter{
				"dc1-rkc": {
					func() *scyllav1alpha1.ScyllaDBDatacenter {
						dc := newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{})
						dc.Labels["label"] = "bar"
						dc.Annotations["annotation"] = "bar"

						dc.Spec.Metadata.Labels["label"] = "bar"
						dc.Spec.Metadata.Annotations["annotation"] = "bar"
						return dc
					}(),
				},
			},
		},
		{
			name: "metadata from datacenter spec overrides one specified on cluster and datacenter template level",
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "scylla-aaa",
					},
				},
			},
			remoteControllers: map[string]metav1.Object{
				"dc1-rkc": &scyllav1alpha1.RemoteOwner{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-111",
						Namespace: "scylla-aaa",
						UID:       "1234",
					},
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
			expectedScyllaDBDatacenters: map[string][]*scyllav1alpha1.ScyllaDBDatacenter{
				"dc1-rkc": {
					func() *scyllav1alpha1.ScyllaDBDatacenter {
						dc := newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{})
						dc.Labels["label"] = "dar"
						dc.Annotations["annotation"] = "dar"

						dc.Spec.Metadata.Labels["label"] = "dar"
						dc.Spec.Metadata.Annotations["annotation"] = "dar"
						return dc
					}(),
				},
			},
		},
		{
			name: "forceRedeploymentReason on cluster level propagates into ScyllaDBDatacenter",
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "scylla-aaa",
					},
				},
			},
			remoteControllers: map[string]metav1.Object{
				"dc1-rkc": &scyllav1alpha1.RemoteOwner{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-111",
						Namespace: "scylla-aaa",
						UID:       "1234",
					},
				},
			},
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				cluster := newBasicScyllaDBCluster()
				cluster.Spec.ForceRedeploymentReason = pointer.Ptr("foo")
				return cluster
			}(),
			expectedScyllaDBDatacenters: map[string][]*scyllav1alpha1.ScyllaDBDatacenter{
				"dc1-rkc": {
					func() *scyllav1alpha1.ScyllaDBDatacenter {
						dc := newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{})
						dc.Spec.ForceRedeploymentReason = pointer.Ptr("foo")
						return dc
					}(),
				},
			},
		},
		{
			name: "forceRedeploymentReason on datacenter level is combined with one specified on cluster level",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				cluster := newBasicScyllaDBCluster()
				cluster.Spec.ForceRedeploymentReason = pointer.Ptr("foo")
				cluster.Spec.Datacenters[0].ForceRedeploymentReason = pointer.Ptr("bar")
				return cluster
			}(),
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "scylla-aaa",
					},
				},
			},
			remoteControllers: map[string]metav1.Object{
				"dc1-rkc": &scyllav1alpha1.RemoteOwner{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-111",
						Namespace: "scylla-aaa",
						UID:       "1234",
					},
				},
			},
			expectedScyllaDBDatacenters: map[string][]*scyllav1alpha1.ScyllaDBDatacenter{
				"dc1-rkc": {
					func() *scyllav1alpha1.ScyllaDBDatacenter {
						dc := newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{})
						dc.Spec.ForceRedeploymentReason = pointer.Ptr("foo,bar")
						return dc
					}(),
				},
			},
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
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "scylla-aaa",
					},
				},
			},
			remoteControllers: map[string]metav1.Object{
				"dc1-rkc": &scyllav1alpha1.RemoteOwner{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-111",
						Namespace: "scylla-aaa",
						UID:       "1234",
					},
				},
			},
			expectedScyllaDBDatacenters: map[string][]*scyllav1alpha1.ScyllaDBDatacenter{
				"dc1-rkc": {
					func() *scyllav1alpha1.ScyllaDBDatacenter {
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
			},
		},
		{
			name: "disableAutomaticOrphanedNodeReplacement is taken from cluster level",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				cluster := newBasicScyllaDBCluster()
				cluster.Spec.DisableAutomaticOrphanedNodeReplacement = true
				return cluster
			}(),
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "scylla-aaa",
					},
				},
			},
			remoteControllers: map[string]metav1.Object{
				"dc1-rkc": &scyllav1alpha1.RemoteOwner{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-111",
						Namespace: "scylla-aaa",
						UID:       "1234",
					},
				},
			},
			expectedScyllaDBDatacenters: map[string][]*scyllav1alpha1.ScyllaDBDatacenter{
				"dc1-rkc": {
					func() *scyllav1alpha1.ScyllaDBDatacenter {
						dc := newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{})
						dc.Spec.DisableAutomaticOrphanedNodeReplacement = pointer.Ptr(true)
						return dc
					}(),
				},
			},
		},
		{
			name: "minTerminationGracePeriodSeconds is taken from cluster level",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				cluster := newBasicScyllaDBCluster()
				cluster.Spec.MinTerminationGracePeriodSeconds = pointer.Ptr[int32](123)
				return cluster
			}(),
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "scylla-aaa",
					},
				},
			},
			remoteControllers: map[string]metav1.Object{
				"dc1-rkc": &scyllav1alpha1.RemoteOwner{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-111",
						Namespace: "scylla-aaa",
						UID:       "1234",
					},
				},
			},
			expectedScyllaDBDatacenters: map[string][]*scyllav1alpha1.ScyllaDBDatacenter{
				"dc1-rkc": {
					func() *scyllav1alpha1.ScyllaDBDatacenter {
						dc := newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{})
						dc.Spec.MinTerminationGracePeriodSeconds = pointer.Ptr[int32](123)
						return dc
					}(),
				},
			},
		},
		{
			name: "minReadySeconds is taken from cluster level",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				cluster := newBasicScyllaDBCluster()
				cluster.Spec.MinReadySeconds = pointer.Ptr[int32](123)
				return cluster
			}(),
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "scylla-aaa",
					},
				},
			},
			remoteControllers: map[string]metav1.Object{
				"dc1-rkc": &scyllav1alpha1.RemoteOwner{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-111",
						Namespace: "scylla-aaa",
						UID:       "1234",
					},
				},
			},
			expectedScyllaDBDatacenters: map[string][]*scyllav1alpha1.ScyllaDBDatacenter{
				"dc1-rkc": {
					func() *scyllav1alpha1.ScyllaDBDatacenter {
						dc := newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{})
						dc.Spec.MinReadySeconds = pointer.Ptr[int32](123)
						return dc
					}(),
				},
			},
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
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "scylla-aaa",
					},
				},
			},
			remoteControllers: map[string]metav1.Object{
				"dc1-rkc": &scyllav1alpha1.RemoteOwner{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-111",
						Namespace: "scylla-aaa",
						UID:       "1234",
					},
				},
			},
			expectedScyllaDBDatacenters: map[string][]*scyllav1alpha1.ScyllaDBDatacenter{
				"dc1-rkc": {
					func() *scyllav1alpha1.ScyllaDBDatacenter {
						dc := newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{})
						dc.Spec.ReadinessGates = []corev1.PodReadinessGate{
							{
								ConditionType: "foo",
							},
						}
						return dc
					}(),
				},
			},
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
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "scylla-aaa",
					},
				},
			},
			remoteControllers: map[string]metav1.Object{
				"dc1-rkc": &scyllav1alpha1.RemoteOwner{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-111",
						Namespace: "scylla-aaa",
						UID:       "1234",
					},
				},
			},
			expectedScyllaDBDatacenters: map[string][]*scyllav1alpha1.ScyllaDBDatacenter{
				"dc1-rkc": {
					func() *scyllav1alpha1.ScyllaDBDatacenter {
						dc := newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{})
						dc.Spec.RackTemplate.Nodes = pointer.Ptr[int32](321)
						return dc
					}(),
				},
			},
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
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "scylla-aaa",
					},
				},
			},
			remoteControllers: map[string]metav1.Object{
				"dc1-rkc": &scyllav1alpha1.RemoteOwner{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-111",
						Namespace: "scylla-aaa",
						UID:       "1234",
					},
				},
			},
			expectedScyllaDBDatacenters: map[string][]*scyllav1alpha1.ScyllaDBDatacenter{
				"dc1-rkc": {
					func() *scyllav1alpha1.ScyllaDBDatacenter {
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
			},
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
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "scylla-aaa",
					},
				},
			},
			remoteControllers: map[string]metav1.Object{
				"dc1-rkc": &scyllav1alpha1.RemoteOwner{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-111",
						Namespace: "scylla-aaa",
						UID:       "1234",
					},
				},
			},
			expectedScyllaDBDatacenters: map[string][]*scyllav1alpha1.ScyllaDBDatacenter{
				"dc1-rkc": {
					func() *scyllav1alpha1.ScyllaDBDatacenter {
						dc := newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{})
						dc.Spec.RackTemplate.TopologyLabelSelector = map[string]string{
							"foo": "dcRackTemplate",
						}
						return dc
					}(),
				},
			},
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
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "scylla-aaa",
					},
				},
			},
			remoteControllers: map[string]metav1.Object{
				"dc1-rkc": &scyllav1alpha1.RemoteOwner{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-111",
						Namespace: "scylla-aaa",
						UID:       "1234",
					},
				},
			},
			expectedScyllaDBDatacenters: map[string][]*scyllav1alpha1.ScyllaDBDatacenter{
				"dc1-rkc": {
					func() *scyllav1alpha1.ScyllaDBDatacenter {
						dc := newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{})
						dc.Spec.RackTemplate.TopologyLabelSelector = map[string]string{
							"foo": "dc",
						}
						return dc
					}(),
				},
			},
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
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "scylla-aaa",
					},
				},
			},
			remoteControllers: map[string]metav1.Object{
				"dc1-rkc": &scyllav1alpha1.RemoteOwner{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-111",
						Namespace: "scylla-aaa",
						UID:       "1234",
					},
				},
			},
			expectedScyllaDBDatacenters: map[string][]*scyllav1alpha1.ScyllaDBDatacenter{
				"dc1-rkc": {
					func() *scyllav1alpha1.ScyllaDBDatacenter {
						dc := newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{})
						dc.Spec.RackTemplate.TopologyLabelSelector = map[string]string{
							"foo": "dcTemplate",
						}
						return dc
					}(),
				},
			},
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
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "scylla-aaa",
					},
				},
			},
			remoteControllers: map[string]metav1.Object{
				"dc1-rkc": &scyllav1alpha1.RemoteOwner{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-111",
						Namespace: "scylla-aaa",
						UID:       "1234",
					},
				},
			},
			expectedScyllaDBDatacenters: map[string][]*scyllav1alpha1.ScyllaDBDatacenter{
				"dc1-rkc": {
					func() *scyllav1alpha1.ScyllaDBDatacenter {
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
			},
		},
		{
			name: "storage metadata is merged from all levels",
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "scylla-aaa",
					},
				},
			},
			remoteControllers: map[string]metav1.Object{
				"dc1-rkc": &scyllav1alpha1.RemoteOwner{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-111",
						Namespace: "scylla-aaa",
						UID:       "1234",
					},
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
			expectedScyllaDBDatacenters: map[string][]*scyllav1alpha1.ScyllaDBDatacenter{
				"dc1-rkc": {
					func() *scyllav1alpha1.ScyllaDBDatacenter {
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
			},
		},
		{
			name: "rack template capacity is taken from datacenter level when provided",
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "scylla-aaa",
					},
				},
			},
			remoteControllers: map[string]metav1.Object{
				"dc1-rkc": &scyllav1alpha1.RemoteOwner{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-111",
						Namespace: "scylla-aaa",
						UID:       "1234",
					},
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
			expectedScyllaDBDatacenters: map[string][]*scyllav1alpha1.ScyllaDBDatacenter{
				"dc1-rkc": {
					func() *scyllav1alpha1.ScyllaDBDatacenter {
						dc := newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{})
						dc.Spec.RackTemplate.ScyllaDB.Storage = &scyllav1alpha1.StorageOptions{
							Capacity: "4",
						}
						return dc
					}(),
				},
			},
		},
		{
			name: "rack template capacity is taken from datacenter level when provided and datacenter rack template is missing",
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "scylla-aaa",
					},
				},
			},
			remoteControllers: map[string]metav1.Object{
				"dc1-rkc": &scyllav1alpha1.RemoteOwner{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-111",
						Namespace: "scylla-aaa",
						UID:       "1234",
					},
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
			expectedScyllaDBDatacenters: map[string][]*scyllav1alpha1.ScyllaDBDatacenter{
				"dc1-rkc": {
					func() *scyllav1alpha1.ScyllaDBDatacenter {
						dc := newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{})
						dc.Spec.RackTemplate.ScyllaDB.Storage = &scyllav1alpha1.StorageOptions{
							Capacity: "3",
						}
						return dc
					}(),
				},
			},
		},
		{
			name: "rack template capacity is taken from datacenter template rack template level when provided and datacenter spec and datacenter rack template is missing",
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "scylla-aaa",
					},
				},
			},
			remoteControllers: map[string]metav1.Object{
				"dc1-rkc": &scyllav1alpha1.RemoteOwner{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-111",
						Namespace: "scylla-aaa",
						UID:       "1234",
					},
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
			expectedScyllaDBDatacenters: map[string][]*scyllav1alpha1.ScyllaDBDatacenter{
				"dc1-rkc": {
					func() *scyllav1alpha1.ScyllaDBDatacenter {
						dc := newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{})
						dc.Spec.RackTemplate.ScyllaDB.Storage = &scyllav1alpha1.StorageOptions{
							Capacity: "2",
						}
						return dc
					}(),
				},
			},
		},
		{
			name: "rack template capacity is taken from datacenter template level when provided all other are not provided",
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "scylla-aaa",
					},
				},
			},
			remoteControllers: map[string]metav1.Object{
				"dc1-rkc": &scyllav1alpha1.RemoteOwner{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-111",
						Namespace: "scylla-aaa",
						UID:       "1234",
					},
				},
			},
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				cluster := newBasicScyllaDBCluster()
				cluster.Spec.DatacenterTemplate.ScyllaDB.Storage = &scyllav1alpha1.StorageOptions{
					Capacity: "1",
				}
				return cluster
			}(),
			expectedScyllaDBDatacenters: map[string][]*scyllav1alpha1.ScyllaDBDatacenter{
				"dc1-rkc": {
					func() *scyllav1alpha1.ScyllaDBDatacenter {
						dc := newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{})
						dc.Spec.RackTemplate.ScyllaDB.Storage = &scyllav1alpha1.StorageOptions{
							Capacity: "1",
						}
						return dc
					}(),
				},
			},
		},
		{
			name: "rack template storageClassName is taken from datacenter level when provided",
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "scylla-aaa",
					},
				},
			},
			remoteControllers: map[string]metav1.Object{
				"dc1-rkc": &scyllav1alpha1.RemoteOwner{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-111",
						Namespace: "scylla-aaa",
						UID:       "1234",
					},
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
			expectedScyllaDBDatacenters: map[string][]*scyllav1alpha1.ScyllaDBDatacenter{
				"dc1-rkc": {
					func() *scyllav1alpha1.ScyllaDBDatacenter {
						dc := newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{})
						dc.Spec.RackTemplate.ScyllaDB.Storage = &scyllav1alpha1.StorageOptions{
							StorageClassName: pointer.Ptr("d"),
						}
						return dc
					}(),
				},
			},
		},
		{
			name: "rack template storageClassName is taken from datacenter level when provided and datacenter rack template is missing",
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "scylla-aaa",
					},
				},
			},
			remoteControllers: map[string]metav1.Object{
				"dc1-rkc": &scyllav1alpha1.RemoteOwner{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-111",
						Namespace: "scylla-aaa",
						UID:       "1234",
					},
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
			expectedScyllaDBDatacenters: map[string][]*scyllav1alpha1.ScyllaDBDatacenter{
				"dc1-rkc": {
					func() *scyllav1alpha1.ScyllaDBDatacenter {
						dc := newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{})
						dc.Spec.RackTemplate.ScyllaDB.Storage = &scyllav1alpha1.StorageOptions{
							StorageClassName: pointer.Ptr("c"),
						}
						return dc
					}(),
				},
			},
		},
		{
			name: "rack template storageClassName is taken from datacenter template rack template level when provided and datacenter spec and datacenter rack template is missing",
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "scylla-aaa",
					},
				},
			},
			remoteControllers: map[string]metav1.Object{
				"dc1-rkc": &scyllav1alpha1.RemoteOwner{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-111",
						Namespace: "scylla-aaa",
						UID:       "1234",
					},
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
			expectedScyllaDBDatacenters: map[string][]*scyllav1alpha1.ScyllaDBDatacenter{
				"dc1-rkc": {
					func() *scyllav1alpha1.ScyllaDBDatacenter {
						dc := newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{})
						dc.Spec.RackTemplate.ScyllaDB.Storage = &scyllav1alpha1.StorageOptions{
							StorageClassName: pointer.Ptr("b"),
						}
						return dc
					}(),
				},
			},
		},
		{
			name: "rack template storageClassName is taken from datacenter template level when provided all other are not provided",
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "scylla-aaa",
					},
				},
			},
			remoteControllers: map[string]metav1.Object{
				"dc1-rkc": &scyllav1alpha1.RemoteOwner{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-111",
						Namespace: "scylla-aaa",
						UID:       "1234",
					},
				},
			},
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				cluster := newBasicScyllaDBCluster()
				cluster.Spec.DatacenterTemplate.ScyllaDB.Storage = &scyllav1alpha1.StorageOptions{
					StorageClassName: pointer.Ptr("a"),
				}
				return cluster
			}(),
			expectedScyllaDBDatacenters: map[string][]*scyllav1alpha1.ScyllaDBDatacenter{
				"dc1-rkc": {
					func() *scyllav1alpha1.ScyllaDBDatacenter {
						dc := newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{})
						dc.Spec.RackTemplate.ScyllaDB.Storage = &scyllav1alpha1.StorageOptions{
							StorageClassName: pointer.Ptr("a"),
						}
						return dc
					}(),
				},
			},
		},
		{
			name: "rack template scylladb custom ConfigMap ref is taken from datacenter level when provided",
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "scylla-aaa",
					},
				},
			},
			remoteControllers: map[string]metav1.Object{
				"dc1-rkc": &scyllav1alpha1.RemoteOwner{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-111",
						Namespace: "scylla-aaa",
						UID:       "1234",
					},
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
			expectedScyllaDBDatacenters: map[string][]*scyllav1alpha1.ScyllaDBDatacenter{
				"dc1-rkc": {
					func() *scyllav1alpha1.ScyllaDBDatacenter {
						dc := newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{})
						dc.Spec.RackTemplate.ScyllaDB.CustomConfigMapRef = pointer.Ptr("d")
						return dc
					}(),
				},
			},
		},
		{
			name: "rack template scylladb custom ConfigMap ref is taken from datacenter level when provided and datacenter rack template is missing",
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "scylla-aaa",
					},
				},
			},
			remoteControllers: map[string]metav1.Object{
				"dc1-rkc": &scyllav1alpha1.RemoteOwner{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-111",
						Namespace: "scylla-aaa",
						UID:       "1234",
					},
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
			expectedScyllaDBDatacenters: map[string][]*scyllav1alpha1.ScyllaDBDatacenter{
				"dc1-rkc": {
					func() *scyllav1alpha1.ScyllaDBDatacenter {
						dc := newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{})
						dc.Spec.RackTemplate.ScyllaDB.CustomConfigMapRef = pointer.Ptr("c")
						return dc
					}(),
				},
			},
		},
		{
			name: "rack template scylladb custom ConfigMap ref is taken from datacenter template rack template level when provided and datacenter spec and datacenter rack template is missing",
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "scylla-aaa",
					},
				},
			},
			remoteControllers: map[string]metav1.Object{
				"dc1-rkc": &scyllav1alpha1.RemoteOwner{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-111",
						Namespace: "scylla-aaa",
						UID:       "1234",
					},
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
			expectedScyllaDBDatacenters: map[string][]*scyllav1alpha1.ScyllaDBDatacenter{
				"dc1-rkc": {
					func() *scyllav1alpha1.ScyllaDBDatacenter {
						dc := newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{})
						dc.Spec.RackTemplate.ScyllaDB.CustomConfigMapRef = pointer.Ptr("b")
						return dc
					}(),
				},
			},
		},
		{
			name: "rack template scylladb custom ConfigMap ref is taken from datacenter template level when provided all other are not provided",
			remoteNamespaces: map[string]*corev1.Namespace{
				"dc1-rkc": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "scylla-aaa",
					},
				},
			},
			remoteControllers: map[string]metav1.Object{
				"dc1-rkc": &scyllav1alpha1.RemoteOwner{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-111",
						Namespace: "scylla-aaa",
						UID:       "1234",
					},
				},
			},
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				cluster := newBasicScyllaDBCluster()
				cluster.Spec.DatacenterTemplate.ScyllaDB.CustomConfigMapRef = pointer.Ptr("a")
				return cluster
			}(),
			expectedScyllaDBDatacenters: map[string][]*scyllav1alpha1.ScyllaDBDatacenter{
				"dc1-rkc": {
					func() *scyllav1alpha1.ScyllaDBDatacenter {
						dc := newBasicScyllaDBDatacenter("dc1", "scylla-aaa", []string{})
						dc.Spec.RackTemplate.ScyllaDB.CustomConfigMapRef = pointer.Ptr("a")
						return dc
					}(),
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotProgressingConditions, gotDatacenters, err := MakeRemoteScyllaDBDatacenters(tc.cluster, tc.remoteScyllaDBDatacenters, tc.remoteNamespaces, tc.remoteControllers, testClusterDomain)
			if err != nil {
				t.Errorf("expected nil error, got %v", err)
			}
			if !equality.Semantic.DeepEqual(gotDatacenters, tc.expectedScyllaDBDatacenters) {
				t.Errorf("expected and got datacenters differ, diff: %s", cmp.Diff(gotDatacenters, tc.expectedScyllaDBDatacenters))
			}
			if !equality.Semantic.DeepEqual(gotProgressingConditions, tc.expectedProgressingConditions) {
				t.Errorf("expected and got progressing conditions differ, diff: %s", cmp.Diff(gotProgressingConditions, tc.expectedProgressingConditions))
			}
		})
	}
}
