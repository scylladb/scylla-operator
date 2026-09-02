package scylladbdatacenter

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/internalapi"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/test/unit"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	corev1listers "k8s.io/client-go/listers/core/v1"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
)

const (
	testNamespace     = "default"
	testScyllaDBImage = "scylladb/scylla:latest"
)

func Test_createMissingStatefulSets(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name                string
		scyllaDBDatacenter  *scyllav1alpha1.ScyllaDBDatacenter
		required            []*appsv1.StatefulSet
		existing            map[string]*appsv1.StatefulSet
		applyStatefulSet    func(context.Context, *appsv1.StatefulSet) (*appsv1.StatefulSet, bool, error)
		expectedCreated     []*appsv1.StatefulSet
		expectedConditions  []metav1.Condition
		expectedErrorString string
	}{
		{
			name:               "skips existing StatefulSets",
			scyllaDBDatacenter: newScyllaDBDatacenter(),
			required: []*appsv1.StatefulSet{
				newStatefulSet("foo"),
			},
			existing: map[string]*appsv1.StatefulSet{
				"foo": newStatefulSet("foo"),
			},
			applyStatefulSet: func(context.Context, *appsv1.StatefulSet) (*appsv1.StatefulSet, bool, error) {
				t.Fatal("applyStatefulSet called for an existing StatefulSet")
				return nil, false, nil
			},
			expectedCreated:     []*appsv1.StatefulSet{},
			expectedConditions:  []metav1.Condition{},
			expectedErrorString: "",
		},
		{
			name: "creates first missing StatefulSet only",
			scyllaDBDatacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newScyllaDBDatacenter()
				sdc.Generation = 3
				return sdc
			}(),
			required: []*appsv1.StatefulSet{
				newStatefulSet("foo"),
				newStatefulSet("bar"),
			},
			existing: map[string]*appsv1.StatefulSet{},
			applyStatefulSet: func(context.Context, *appsv1.StatefulSet) (*appsv1.StatefulSet, bool, error) {
				return newStatefulSet("foo"), true, nil
			},
			expectedCreated: []*appsv1.StatefulSet{newStatefulSet("foo")},
			expectedConditions: []metav1.Condition{
				{
					Type:               statefulSetControllerProgressingCondition,
					Status:             metav1.ConditionTrue,
					Reason:             internalapi.ProgressingReason,
					Message:            `Progressing: Running "apply" on "apps/v1, Kind=StatefulSet"`,
					ObservedGeneration: 3,
				},
			},
			expectedErrorString: "",
		},
		{
			name:               "does not add condition when apply is unchanged",
			scyllaDBDatacenter: newScyllaDBDatacenter(),
			required: []*appsv1.StatefulSet{
				newStatefulSet("foo"),
			},
			existing: map[string]*appsv1.StatefulSet{},
			applyStatefulSet: func(context.Context, *appsv1.StatefulSet) (*appsv1.StatefulSet, bool, error) {
				return newStatefulSet("foo"), false, nil
			},
			expectedCreated:     []*appsv1.StatefulSet{},
			expectedConditions:  []metav1.Condition{},
			expectedErrorString: "",
		},
		{
			name:               "returns apply error",
			scyllaDBDatacenter: newScyllaDBDatacenter(),
			required: []*appsv1.StatefulSet{
				newStatefulSet("foo"),
			},
			existing: map[string]*appsv1.StatefulSet{},
			applyStatefulSet: func(context.Context, *appsv1.StatefulSet) (*appsv1.StatefulSet, bool, error) {
				return nil, false, errors.New("apply failed")
			},
			expectedCreated:     []*appsv1.StatefulSet{},
			expectedConditions:  []metav1.Condition{},
			expectedErrorString: `can't create missing statefulset "default/foo": apply failed`,
		},
		{
			name: "returns the StatefulSets created before an apply error with parallel node operations enabled",
			scyllaDBDatacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newScyllaDBDatacenter()
				sdc.Generation = 3
				sdc.Spec.ScyllaDB.Image = unit.ScyllaDBImageAtParallelBootstrapThreshold
				sdc.Spec.EnableParallelNodeOperations = new(true)
				return sdc
			}(),
			required: []*appsv1.StatefulSet{
				newStatefulSet("foo"),
				newStatefulSet("bar"),
				newStatefulSet("baz"),
			},
			existing: map[string]*appsv1.StatefulSet{},
			applyStatefulSet: func(_ context.Context, required *appsv1.StatefulSet) (*appsv1.StatefulSet, bool, error) {
				switch required.Name {
				case "bar":
					return nil, false, errors.New("apply failed")

				case "baz":
					t.Fatal("applyStatefulSet called after an apply error")
					return nil, false, nil

				default:
					return newStatefulSet(required.Name), true, nil
				}
			},
			expectedCreated: []*appsv1.StatefulSet{
				newStatefulSet("foo"),
			},
			expectedConditions: []metav1.Condition{
				{
					Type:               statefulSetControllerProgressingCondition,
					Status:             metav1.ConditionTrue,
					Reason:             internalapi.ProgressingReason,
					Message:            `Progressing: Running "apply" on "apps/v1, Kind=StatefulSet"`,
					ObservedGeneration: 3,
				},
			},
			expectedErrorString: `can't create missing statefulset "default/bar": apply failed`,
		},
		{
			name: "creates all missing StatefulSets with parallel node operations enabled",
			scyllaDBDatacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newScyllaDBDatacenter()
				sdc.Generation = 3
				sdc.Spec.ScyllaDB.Image = unit.ScyllaDBImageAtParallelBootstrapThreshold
				sdc.Spec.EnableParallelNodeOperations = new(true)
				return sdc
			}(),
			required: []*appsv1.StatefulSet{
				newStatefulSet("foo"),
				newStatefulSet("bar"),
			},
			existing: map[string]*appsv1.StatefulSet{},
			applyStatefulSet: func(_ context.Context, required *appsv1.StatefulSet) (*appsv1.StatefulSet, bool, error) {
				return newStatefulSet(required.Name), true, nil
			},
			expectedCreated: []*appsv1.StatefulSet{
				newStatefulSet("foo"),
				newStatefulSet("bar"),
			},
			expectedConditions: []metav1.Condition{
				{
					Type:               statefulSetControllerProgressingCondition,
					Status:             metav1.ConditionTrue,
					Reason:             internalapi.ProgressingReason,
					Message:            `Progressing: Running "apply" on "apps/v1, Kind=StatefulSet"`,
					ObservedGeneration: 3,
				},
				{
					Type:               statefulSetControllerProgressingCondition,
					Status:             metav1.ConditionTrue,
					Reason:             internalapi.ProgressingReason,
					Message:            `Progressing: Running "apply" on "apps/v1, Kind=StatefulSet"`,
					ObservedGeneration: 3,
				},
			},
			expectedErrorString: "",
		},
		{
			name: "creates only the missing StatefulSets with parallel node operations enabled",
			scyllaDBDatacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newScyllaDBDatacenter()
				sdc.Generation = 3
				sdc.Spec.ScyllaDB.Image = unit.ScyllaDBImageAtParallelBootstrapThreshold
				sdc.Spec.EnableParallelNodeOperations = new(true)
				return sdc
			}(),
			required: []*appsv1.StatefulSet{
				newStatefulSet("foo"),
				newStatefulSet("bar"),
			},
			existing: map[string]*appsv1.StatefulSet{
				"foo": newStatefulSet("foo"),
			},
			applyStatefulSet: func(_ context.Context, required *appsv1.StatefulSet) (*appsv1.StatefulSet, bool, error) {
				if required.Name != "bar" {
					t.Fatalf("applyStatefulSet called for an existing StatefulSet %q", required.Name)
				}
				return newStatefulSet(required.Name), true, nil
			},
			expectedCreated: []*appsv1.StatefulSet{
				newStatefulSet("bar"),
			},
			expectedConditions: []metav1.Condition{
				{
					Type:               statefulSetControllerProgressingCondition,
					Status:             metav1.ConditionTrue,
					Reason:             internalapi.ProgressingReason,
					Message:            `Progressing: Running "apply" on "apps/v1, Kind=StatefulSet"`,
					ObservedGeneration: 3,
				},
			},
			expectedErrorString: "",
		},
		{
			name: "returns an error with parallel node operations enabled and an unsupported ScyllaDB version",
			scyllaDBDatacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newScyllaDBDatacenter()
				sdc.Spec.ScyllaDB.Image = unit.ScyllaDBImageBelowParallelBootstrapThreshold
				sdc.Spec.EnableParallelNodeOperations = new(true)
				return sdc
			}(),
			required: []*appsv1.StatefulSet{
				newStatefulSet("foo"),
			},
			existing: map[string]*appsv1.StatefulSet{},
			applyStatefulSet: func(context.Context, *appsv1.StatefulSet) (*appsv1.StatefulSet, bool, error) {
				t.Fatal("applyStatefulSet called with an unsupported ScyllaDB version")
				return nil, false, nil
			},
			expectedCreated:     nil,
			expectedConditions:  nil,
			expectedErrorString: `can't determine effective parallel node operations enablement: parallel node operations require a semver-parseable ScyllaDB version >= 2026.2, got "2026.1.0"`,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			created, conditions, err := createMissingStatefulSets(
				context.Background(),
				tc.applyStatefulSet,
				tc.scyllaDBDatacenter,
				tc.required,
				tc.existing,
			)

			var errStr string
			if err != nil {
				errStr = err.Error()
			}
			if diff := cmp.Diff(tc.expectedErrorString, errStr); diff != "" {
				t.Errorf("expected and actual error strings differ (-want +got):\n%s", diff)
			}

			if diff := cmp.Diff(tc.expectedCreated, created, cmpopts.IgnoreFields(metav1.ObjectMeta{}, "ResourceVersion")); diff != "" {
				t.Errorf("created StatefulSets differ (-want +got):\n%s", diff)
			}

			for i := range conditions {
				conditions[i].LastTransitionTime = metav1.Time{}
			}
			if diff := cmp.Diff(tc.expectedConditions, conditions); diff != "" {
				t.Errorf("conditions differ (-want +got):\n%s", diff)
			}
		})
	}
}

func Test_ensureRackNamesInRackStatuses(t *testing.T) {
	t.Parallel()

	newRackStatefulSet := func(rackName string) *appsv1.StatefulSet {
		sts := newStatefulSet(rackName)
		sts.Labels = map[string]string{
			naming.RackNameLabel: rackName,
		}

		return sts
	}

	newPodLister := func(t *testing.T, statefulSets ...*appsv1.StatefulSet) corev1listers.PodLister {
		t.Helper()

		podCache := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
		for _, sts := range statefulSets {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: sts.Namespace,
					Name:      sts.Name + "-0",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: appsv1.SchemeGroupVersion.String(),
							Kind:       "StatefulSet",
							Name:       sts.Name,
							UID:        sts.UID,
							Controller: new(true),
						},
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  naming.ScyllaContainerName,
							Image: testScyllaDBImage,
						},
					},
				},
			}
			if err := podCache.Add(pod); err != nil {
				t.Fatal(err)
			}
		}

		return corev1listers.NewPodLister(podCache)
	}

	freshRackStatus := func(name string) scyllav1alpha1.RackStatus {
		return scyllav1alpha1.RackStatus{
			Name:           name,
			CurrentVersion: "latest",
			UpdatedVersion: "latest",
			Nodes:          new(int32(1)),
			CurrentNodes:   new(int32(0)),
			UpdatedNodes:   new(int32(0)),
			ReadyNodes:     new(int32(0)),
			AvailableNodes: new(int32(0)),
			Stale:          new(false),
		}
	}

	tt := []struct {
		name                string
		scyllaDBDatacenter  *scyllav1alpha1.ScyllaDBDatacenter
		status              *scyllav1alpha1.ScyllaDBDatacenterStatus
		statefulSets        []*appsv1.StatefulSet
		services            map[string]*corev1.Service
		expectedRacks       []scyllav1alpha1.RackStatus
		expectedErrorString string
	}{
		{
			name:               "adds status calculated from statefulset and pod",
			scyllaDBDatacenter: newScyllaDBDatacenter(),
			status:             &scyllav1alpha1.ScyllaDBDatacenterStatus{},
			statefulSets: []*appsv1.StatefulSet{
				newRackStatefulSet("foo"),
			},
			expectedRacks: []scyllav1alpha1.RackStatus{
				freshRackStatus("foo"),
			},
		},
		{
			name:               "recalculates existing rack status",
			scyllaDBDatacenter: newScyllaDBDatacenter(),
			status: &scyllav1alpha1.ScyllaDBDatacenterStatus{
				Racks: []scyllav1alpha1.RackStatus{
					{
						Name:           "foo",
						CurrentVersion: "old",
						UpdatedVersion: "old",
						Nodes:          new(int32(2)),
						CurrentNodes:   new(int32(2)),
						UpdatedNodes:   new(int32(2)),
						ReadyNodes:     new(int32(2)),
						AvailableNodes: new(int32(2)),
						Stale:          new(true),
					},
				},
			},
			statefulSets: []*appsv1.StatefulSet{
				newRackStatefulSet("foo"),
			},
			expectedRacks: []scyllav1alpha1.RackStatus{
				freshRackStatus("foo"),
			},
		},
		{
			name:               "lists nodes whose member Service carries the decommissioned label",
			scyllaDBDatacenter: newScyllaDBDatacenter(),
			status:             &scyllav1alpha1.ScyllaDBDatacenterStatus{},
			statefulSets: []*appsv1.StatefulSet{
				newRackStatefulSet("foo"),
			},
			services: map[string]*corev1.Service{
				"foo-1": {
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testNamespace,
						Name:      "foo-1",
						Labels: map[string]string{
							naming.RackNameLabel:       "foo",
							naming.DecommissionedLabel: naming.LabelValueFalse,
						},
					},
				},
			},
			expectedRacks: []scyllav1alpha1.RackStatus{
				func() scyllav1alpha1.RackStatus {
					rs := freshRackStatus("foo")
					rs.DecommissioningNodes = []scyllav1alpha1.DecommissioningNodeStatus{
						{Name: "foo-1"},
					}
					return rs
				}(),
			},
		},
		{
			name:               "keeps stale status for rack without statefulset",
			scyllaDBDatacenter: newScyllaDBDatacenter(),
			status: &scyllav1alpha1.ScyllaDBDatacenterStatus{
				Racks: []scyllav1alpha1.RackStatus{
					{
						Name:           "bar",
						CurrentVersion: "old",
						UpdatedVersion: "old",
						Stale:          new(true),
					},
				},
			},
			statefulSets: []*appsv1.StatefulSet{
				newRackStatefulSet("foo"),
			},
			expectedRacks: []scyllav1alpha1.RackStatus{
				{
					Name:           "bar",
					CurrentVersion: "old",
					UpdatedVersion: "old",
					Stale:          new(true),
				},
				freshRackStatus("foo"),
			},
		},
		{
			name:               "returns missing rack label error",
			scyllaDBDatacenter: newScyllaDBDatacenter(),
			status:             &scyllav1alpha1.ScyllaDBDatacenterStatus{},
			statefulSets: []*appsv1.StatefulSet{
				newStatefulSet("foo"),
			},
			expectedErrorString: `can't determine rack name: statefulset default/foo is missing label "scylla/rack"`,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fakePodLister := newPodLister(t, tc.statefulSets...)

			err := ensureRackNamesInRackStatuses(
				fakePodLister,
				tc.scyllaDBDatacenter,
				tc.status,
				tc.statefulSets,
				tc.services,
			)

			var errStr string
			if err != nil {
				errStr = err.Error()
			}
			if diff := cmp.Diff(tc.expectedErrorString, errStr); diff != "" {
				t.Errorf("expected and actual error strings differ (-want +got):\n%s", diff)
			}

			if diff := cmp.Diff(tc.expectedRacks, tc.status.Racks); diff != "" {
				t.Errorf("rack statuses differ (-want +got):\n%s", diff)
			}
		})
	}
}

func newScyllaDBDatacenter() *scyllav1alpha1.ScyllaDBDatacenter {
	return &scyllav1alpha1.ScyllaDBDatacenter{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
		},
		Spec: scyllav1alpha1.ScyllaDBDatacenterSpec{
			ScyllaDB: scyllav1alpha1.ScyllaDB{
				Image: testScyllaDBImage,
			},
		},
	}
}

func newStatefulSet(name string) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      name,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: scyllav1alpha1.GroupVersion.String(),
					Kind:       "ScyllaDBDatacenter",
					Name:       "basic",
					UID:        "owner",
					Controller: new(true),
				},
			},
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: new(int32(1)),
		},
	}
}

func Test_getRackScaleDownTargetNodeCount(t *testing.T) {
	t.Parallel()

	sdc := newScyllaDBDatacenter()
	sdc.Spec.Racks = []scyllav1alpha1.RackSpec{
		{Name: "a"},
	}
	stsName := naming.StatefulSetNameForRack(sdc.Spec.Racks[0], sdc)
	newRackStatefulSet := func(replicas int32) *appsv1.StatefulSet {
		return &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: stsName,
			},
			Spec: appsv1.StatefulSetSpec{
				Replicas: new(replicas),
			},
		}
	}
	newDecommissioningNodes := func(names ...string) []scyllav1alpha1.DecommissioningNodeStatus {
		var nodes []scyllav1alpha1.DecommissioningNodeStatus
		for _, name := range names {
			nodes = append(nodes, scyllav1alpha1.DecommissioningNodeStatus{Name: name})
		}
		return nodes
	}

	tt := []struct {
		name                 string
		sts                  *appsv1.StatefulSet
		specNodeCount        int32
		decommissioningNodes []scyllav1alpha1.DecommissioningNodeStatus
		expectedNodeCount    int32
		expectedErr          error
	}{
		{
			name:                 "lowest leaving ordinal",
			sts:                  newRackStatefulSet(3),
			specNodeCount:        1,
			decommissioningNodes: newDecommissioningNodes(stsName+"-1", stsName+"-2"),
			expectedNodeCount:    1,
		},
		{
			name:                 "capped at the StatefulSet replicas when the leaving nodes are already scaled down",
			sts:                  newRackStatefulSet(2),
			specNodeCount:        2,
			decommissioningNodes: newDecommissioningNodes(stsName + "-2"),
			expectedNodeCount:    2,
		},
		{
			name:                 "a raised spec node count is not followed",
			sts:                  newRackStatefulSet(3),
			specNodeCount:        3,
			decommissioningNodes: newDecommissioningNodes(stsName+"-1", stsName+"-2"),
			expectedNodeCount:    1,
		},
		{
			name:                 "a spec node count lowered below the leaving nodes is followed",
			sts:                  newRackStatefulSet(3),
			specNodeCount:        0,
			decommissioningNodes: newDecommissioningNodes(stsName+"-1", stsName+"-2"),
			expectedNodeCount:    0,
		},
		{
			name:                 "no decommissioning nodes yield the spec node count",
			sts:                  newRackStatefulSet(3),
			specNodeCount:        1,
			decommissioningNodes: nil,
			expectedNodeCount:    1,
		},
		{
			name:                 "unparsable leaving node name errors out",
			sts:                  newRackStatefulSet(3),
			specNodeCount:        3,
			decommissioningNodes: newDecommissioningNodes("foo"),
			expectedErr:          fmt.Errorf(`can't get ordinal of decommissioning node "foo" of rack "a" of ScyllaDBDatacenter %q: %w`, naming.ObjRef(sdc), fmt.Errorf(`didn't find '-' delimiter in string foo`)),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := getRackScaleDownTargetNodeCount(sdc, "a", tc.sts, tc.specNodeCount, tc.decommissioningNodes)
			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Fatalf("expected error %v, got %v", tc.expectedErr, err)
			}
			if got != tc.expectedNodeCount {
				t.Errorf("expected node count %d, got %d", tc.expectedNodeCount, got)
			}
		})
	}
}

func Test_getRackScaleDownReplicas(t *testing.T) {
	t.Parallel()

	const stsName = "foo"

	newSts := func(replicas int32) *appsv1.StatefulSet {
		return &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: stsName,
			},
			Spec: appsv1.StatefulSetSpec{
				Replicas: new(replicas),
			},
		}
	}
	// newRackServices builds the member Services of ordinals 0..len(labelValues)-1, each with the given decommissioned
	// label value, or without the label for an empty value. A nil value leaves the Service out.
	newRackServices := func(labelValues ...*string) map[string]*corev1.Service {
		rackServices := map[string]*corev1.Service{}
		for ordinal, labelValue := range labelValues {
			if labelValue == nil {
				continue
			}
			labels := map[string]string{}
			if len(*labelValue) != 0 {
				labels[naming.DecommissionedLabel] = *labelValue
			}
			rackServices[naming.MemberServiceNameForStatefulSet(stsName, ordinal)] = &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:   naming.MemberServiceNameForStatefulSet(stsName, ordinal),
					Labels: labels,
				},
			}
		}
		return rackServices
	}
	none, requested, done := new(""), new(naming.LabelValueFalse), new(naming.LabelValueTrue)

	tt := []struct {
		name                 string
		sts                  *appsv1.StatefulSet
		lowestLeavingOrdinal int32
		rackServices         map[string]*corev1.Service
		expectedReplicas     int32
	}{
		{
			name:                 "no leaving node decommissioned yet",
			sts:                  newSts(3),
			lowestLeavingOrdinal: 1,
			rackServices:         newRackServices(none, requested, requested),
			expectedReplicas:     3,
		},
		{
			name:                 "the highest node decommissioned with a lower one in flight",
			sts:                  newSts(3),
			lowestLeavingOrdinal: 1,
			rackServices:         newRackServices(none, requested, done),
			expectedReplicas:     2,
		},
		{
			name:                 "a lower node decommissioned with the highest one in flight",
			sts:                  newSts(3),
			lowestLeavingOrdinal: 1,
			rackServices:         newRackServices(none, done, requested),
			expectedReplicas:     3,
		},
		{
			name:                 "every leaving node decommissioned",
			sts:                  newSts(3),
			lowestLeavingOrdinal: 1,
			rackServices:         newRackServices(none, done, done),
			expectedReplicas:     1,
		},
		{
			name:                 "the run stops at the lowest leaving ordinal",
			sts:                  newSts(3),
			lowestLeavingOrdinal: 2,
			rackServices:         newRackServices(done, done, done),
			expectedReplicas:     2,
		},
		{
			name:                 "a node that was just requested ends the run",
			sts:                  newSts(3),
			lowestLeavingOrdinal: 0,
			rackServices:         newRackServices(none, none, done),
			expectedReplicas:     2,
		},
		{
			name:                 "a missing Service ends the run",
			sts:                  newSts(3),
			lowestLeavingOrdinal: 0,
			rackServices:         newRackServices(done, nil, done),
			expectedReplicas:     2,
		},
		{
			name:                 "the highest node missing yields the current replicas",
			sts:                  newSts(2),
			lowestLeavingOrdinal: 1,
			rackServices:         newRackServices(none, nil),
			expectedReplicas:     2,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := getRackScaleDownReplicas(tc.sts, tc.lowestLeavingOrdinal, tc.rackServices)
			if got != tc.expectedReplicas {
				t.Errorf("expected replicas %d, got %d", tc.expectedReplicas, got)
			}
		})
	}
}

func Test_syncRackDecommission(t *testing.T) {
	t.Parallel()

	const rackName = "a"

	newSDCWithNodes := func(nodes int32) *scyllav1alpha1.ScyllaDBDatacenter {
		sdc := newScyllaDBDatacenter()
		sdc.Spec.Racks = []scyllav1alpha1.RackSpec{
			{
				Name: rackName,
				RackTemplate: scyllav1alpha1.RackTemplate{
					Nodes: new(nodes),
				},
			},
		}
		return sdc
	}

	newParallelSDCWithNodes := func(nodes int32) *scyllav1alpha1.ScyllaDBDatacenter {
		sdc := newSDCWithNodes(nodes)
		sdc.Spec.ScyllaDB.Image = unit.ScyllaDBImageAtParallelBootstrapThreshold
		sdc.Spec.EnableParallelNodeOperations = new(true)
		return sdc
	}

	sdc := newSDCWithNodes(1)
	stsName := naming.StatefulSetNameForRack(sdc.Spec.Racks[0], sdc)

	newSts := func(replicas int32) *appsv1.StatefulSet {
		return &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testNamespace,
				Name:      stsName,
				Labels: map[string]string{
					naming.RackNameLabel: rackName,
				},
			},
			Spec: appsv1.StatefulSetSpec{
				Replicas: new(replicas),
			},
		}
	}
	newMemberService := func(ordinal int32, decommissionedLabelValue *string) *corev1.Service {
		labels := map[string]string{
			naming.RackNameLabel: rackName,
		}
		if decommissionedLabelValue != nil {
			labels[naming.DecommissionedLabel] = *decommissionedLabelValue
		}
		return &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testNamespace,
				Name:      naming.MemberServiceNameForStatefulSet(stsName, int(ordinal)),
				Labels:    labels,
			},
		}
	}
	newRackServices := func(services ...*corev1.Service) map[string]*corev1.Service {
		rackServices := map[string]*corev1.Service{}
		for _, svc := range services {
			rackServices[svc.Name] = svc
		}
		return rackServices
	}
	makeStampCondition := func(svc *corev1.Service) metav1.Condition {
		svcCopy := svc.DeepCopy()
		svcCopy.Labels[naming.DecommissionedLabel] = naming.LabelValueFalse
		var conditions []metav1.Condition
		controllerhelpers.AddGenericProgressingStatusCondition(&conditions, statefulSetControllerProgressingCondition, svcCopy, "update", sdc.Generation)
		return conditions[0]
	}
	makeWaitingForDecommissionCondition := func(ordinal int32) metav1.Condition {
		return metav1.Condition{
			Type:               statefulSetControllerProgressingCondition,
			Status:             metav1.ConditionTrue,
			Reason:             "WaitingForRackServiceDecommission",
			Message:            fmt.Sprintf(`Waiting for rack service "%s/%s-%d" to decommission.`, testNamespace, stsName, ordinal),
			ObservedGeneration: sdc.Generation,
		}
	}
	makeDeferringCondition := func(specNodeCount int32, decommissioningOrdinals ...int32) metav1.Condition {
		names := make([]string, 0, len(decommissioningOrdinals))
		for _, ordinal := range decommissioningOrdinals {
			names = append(names, naming.MemberServiceNameForStatefulSet(stsName, int(ordinal)))
		}
		return metav1.Condition{
			Type:               statefulSetControllerProgressingCondition,
			Status:             metav1.ConditionTrue,
			Reason:             "DeferringRackNodeCountChange",
			Message:            fmt.Sprintf(`Deferring node count change of rack %q to %d until the Services of its decommissioning nodes %q are pruned.`, rackName, specNodeCount, names),
			ObservedGeneration: sdc.Generation,
		}
	}
	makeScaleCondition := func(sts *appsv1.StatefulSet, replicas int32) metav1.Condition {
		scale := &autoscalingv1.Scale{
			ObjectMeta: metav1.ObjectMeta{
				Name:            sts.Name,
				Namespace:       sts.Namespace,
				ResourceVersion: sts.ResourceVersion,
			},
			Spec: autoscalingv1.ScaleSpec{
				Replicas: replicas,
			},
		}
		var conditions []metav1.Condition
		controllerhelpers.AddGenericProgressingStatusCondition(&conditions, statefulSetControllerProgressingCondition, scale, "updateScale", sdc.Generation)
		return conditions[0]
	}

	tt := []struct {
		name                                      string
		sdc                                       *scyllav1alpha1.ScyllaDBDatacenter
		rackName                                  string
		sts                                       *appsv1.StatefulSet
		rackServices                              map[string]*corev1.Service
		expectedConditions                        []metav1.Condition
		expectedErrorString                       string
		expectedDecommissionRequestedServiceNames []string
		expectedScaledReplicas                    *int32
	}{
		{
			name:               "idle rack yields no conditions and no actions with parallel node operations disabled",
			sdc:                newSDCWithNodes(1),
			rackName:           rackName,
			sts:                newSts(1),
			rackServices:       newRackServices(newMemberService(0, nil)),
			expectedConditions: nil,
		},
		{
			name:               "a fresh scale-down stamps the highest node with parallel node operations disabled",
			sdc:                newSDCWithNodes(1),
			rackName:           rackName,
			sts:                newSts(2),
			rackServices:       newRackServices(newMemberService(0, nil), newMemberService(1, nil)),
			expectedConditions: []metav1.Condition{makeStampCondition(newMemberService(1, nil))},
			expectedDecommissionRequestedServiceNames: []string{stsName + "-1"},
		},
		{
			name:               "a fresh multi-node scale-down stamps the highest node only with parallel node operations disabled",
			sdc:                newSDCWithNodes(1),
			rackName:           rackName,
			sts:                newSts(3),
			rackServices:       newRackServices(newMemberService(0, nil), newMemberService(1, nil), newMemberService(2, nil)),
			expectedConditions: []metav1.Condition{makeStampCondition(newMemberService(2, nil))},
			expectedDecommissionRequestedServiceNames: []string{stsName + "-2"},
		},
		{
			name:         "an in-flight decommission of the highest node is waited for with parallel node operations disabled",
			sdc:          newSDCWithNodes(1),
			rackName:     rackName,
			sts:          newSts(2),
			rackServices: newRackServices(newMemberService(0, nil), newMemberService(1, new(naming.LabelValueFalse))),
			expectedConditions: []metav1.Condition{
				{
					Type:               statefulSetControllerProgressingCondition,
					Status:             metav1.ConditionTrue,
					Reason:             "WaitingForRackServiceDecommission",
					Message:            fmt.Sprintf(`Waiting for rack service "%s/%s-1" to decommission.`, testNamespace, stsName),
					ObservedGeneration: sdc.Generation,
				},
			},
		},
		{
			name:         "an in-flight decommission below the highest node is waited for and the highest node is not stamped with parallel node operations disabled",
			sdc:          newSDCWithNodes(1),
			rackName:     rackName,
			sts:          newSts(3),
			rackServices: newRackServices(newMemberService(0, nil), newMemberService(1, new(naming.LabelValueFalse)), newMemberService(2, nil)),
			expectedConditions: []metav1.Condition{
				{
					Type:               statefulSetControllerProgressingCondition,
					Status:             metav1.ConditionTrue,
					Reason:             "WaitingForRackServiceDecommission",
					Message:            fmt.Sprintf(`Waiting for rack service "%s/%s-1" to decommission.`, testNamespace, stsName),
					ObservedGeneration: sdc.Generation,
				},
			},
		},
		{
			name:                   "a decommissioned highest node concludes by scaling the StatefulSet below it with parallel node operations disabled",
			sdc:                    newSDCWithNodes(1),
			rackName:               rackName,
			sts:                    newSts(2),
			rackServices:           newRackServices(newMemberService(0, nil), newMemberService(1, new(naming.LabelValueTrue))),
			expectedConditions:     []metav1.Condition{makeScaleCondition(newSts(2), 1)},
			expectedScaledReplicas: new(int32(1)),
		},
		{
			name:         "a node count raised mid-decommission is deferred with parallel node operations disabled",
			sdc:          newSDCWithNodes(3),
			rackName:     rackName,
			sts:          newSts(2),
			rackServices: newRackServices(newMemberService(0, nil), newMemberService(1, new(naming.LabelValueFalse))),
			expectedConditions: []metav1.Condition{
				{
					Type:               statefulSetControllerProgressingCondition,
					Status:             metav1.ConditionTrue,
					Reason:             "DeferringRackNodeCountChange",
					Message:            fmt.Sprintf(`Deferring node count change of rack %q to 3 until the Services of its decommissioning nodes ["%s-1"] are pruned.`, rackName, stsName),
					ObservedGeneration: sdc.Generation,
				},
				{
					Type:               statefulSetControllerProgressingCondition,
					Status:             metav1.ConditionTrue,
					Reason:             "WaitingForRackServiceDecommission",
					Message:            fmt.Sprintf(`Waiting for rack service "%s/%s-1" to decommission.`, testNamespace, stsName),
					ObservedGeneration: sdc.Generation,
				},
			},
		},
		{
			name:         "decommissioned nodes scaled away already are held until their Services are pruned with parallel node operations disabled",
			sdc:          newSDCWithNodes(2),
			rackName:     rackName,
			sts:          newSts(1),
			rackServices: newRackServices(newMemberService(0, nil), newMemberService(1, new(naming.LabelValueTrue))),
			expectedConditions: []metav1.Condition{
				{
					Type:               statefulSetControllerProgressingCondition,
					Status:             metav1.ConditionTrue,
					Reason:             "DeferringRackNodeCountChange",
					Message:            fmt.Sprintf(`Deferring node count change of rack %q to 2 until the Services of its decommissioning nodes ["%s-1"] are pruned.`, rackName, stsName),
					ObservedGeneration: sdc.Generation,
				},
				{
					Type:               statefulSetControllerProgressingCondition,
					Status:             metav1.ConditionTrue,
					Reason:             "WaitingForRackServicePruning",
					Message:            fmt.Sprintf(`Waiting for decommissioned service(s) ["%s-1"] of rack %q to be pruned.`, stsName, rackName),
					ObservedGeneration: sdc.Generation,
				},
			},
		},
		{
			name:         "a scale-down with the highest node's Service missing waits for it with parallel node operations disabled",
			sdc:          newSDCWithNodes(1),
			rackName:     rackName,
			sts:          newSts(2),
			rackServices: newRackServices(newMemberService(0, nil)),
			expectedConditions: []metav1.Condition{
				{
					Type:               statefulSetControllerProgressingCondition,
					Status:             metav1.ConditionTrue,
					Reason:             "WaitingForMissingService",
					Message:            fmt.Sprintf(`Statusfulset "%s/%s" is waiting for service %q to be created`, testNamespace, stsName, stsName+"-1"),
					ObservedGeneration: sdc.Generation,
				},
			},
		},
		{
			name:         "a multi-node scale-down with the highest node in flight is not reported as a deferred node count change with parallel node operations disabled",
			sdc:          newSDCWithNodes(1),
			rackName:     rackName,
			sts:          newSts(3),
			rackServices: newRackServices(newMemberService(0, nil), newMemberService(1, nil), newMemberService(2, new(naming.LabelValueFalse))),
			expectedConditions: []metav1.Condition{
				makeWaitingForDecommissionCondition(2),
			},
		},
		{
			name:               "a node count lowered further mid-decommission requests the next node once the previous one is decommissioned with parallel node operations disabled",
			sdc:                newSDCWithNodes(0),
			rackName:           rackName,
			sts:                newSts(2),
			rackServices:       newRackServices(newMemberService(0, nil), newMemberService(1, nil), newMemberService(2, new(naming.LabelValueTrue))),
			expectedConditions: []metav1.Condition{makeStampCondition(newMemberService(1, nil))},
			expectedDecommissionRequestedServiceNames: []string{stsName + "-1"},
		},
		{
			name:               "a fresh multi-node scale-down stamps all the leaving nodes at once with parallel node operations enabled",
			sdc:                newParallelSDCWithNodes(1),
			rackName:           rackName,
			sts:                newSts(3),
			rackServices:       newRackServices(newMemberService(0, nil), newMemberService(1, nil), newMemberService(2, nil)),
			expectedConditions: []metav1.Condition{makeStampCondition(newMemberService(1, nil)), makeStampCondition(newMemberService(2, nil))},
			expectedDecommissionRequestedServiceNames: []string{stsName + "-1", stsName + "-2"},
		},
		{
			name:         "a partially requested multi-node scale-down stamps the remaining leaving nodes with parallel node operations enabled",
			sdc:          newParallelSDCWithNodes(1),
			rackName:     rackName,
			sts:          newSts(3),
			rackServices: newRackServices(newMemberService(0, nil), newMemberService(1, new(naming.LabelValueFalse)), newMemberService(2, nil)),
			expectedConditions: []metav1.Condition{
				makeWaitingForDecommissionCondition(1),
				makeStampCondition(newMemberService(2, nil)),
			},
			expectedDecommissionRequestedServiceNames: []string{stsName + "-2"},
		},
		{
			name:         "a node above an in-flight node is requested even after the node count was reverted, as the StatefulSet can't remove the in-flight node alone, with parallel node operations enabled",
			sdc:          newParallelSDCWithNodes(3),
			rackName:     rackName,
			sts:          newSts(3),
			rackServices: newRackServices(newMemberService(0, nil), newMemberService(1, new(naming.LabelValueFalse)), newMemberService(2, nil)),
			expectedConditions: []metav1.Condition{
				makeDeferringCondition(3, 1),
				makeWaitingForDecommissionCondition(1),
				makeStampCondition(newMemberService(2, nil)),
			},
			expectedDecommissionRequestedServiceNames: []string{stsName + "-2"},
		},
		{
			name:         "a node count reverted after a pass cut short saves the nodes that weren't requested yet with parallel node operations enabled",
			sdc:          newParallelSDCWithNodes(3),
			rackName:     rackName,
			sts:          newSts(3),
			rackServices: newRackServices(newMemberService(0, nil), newMemberService(1, nil), newMemberService(2, new(naming.LabelValueFalse))),
			expectedConditions: []metav1.Condition{
				makeDeferringCondition(3, 2),
				makeWaitingForDecommissionCondition(2),
			},
		},
		{
			name:         "a scale-down cut short after the highest node was requested resumes on the next pass with parallel node operations enabled",
			sdc:          newParallelSDCWithNodes(1),
			rackName:     rackName,
			sts:          newSts(3),
			rackServices: newRackServices(newMemberService(0, nil), newMemberService(1, nil), newMemberService(2, new(naming.LabelValueFalse))),
			expectedConditions: []metav1.Condition{
				makeWaitingForDecommissionCondition(2),
				makeStampCondition(newMemberService(1, nil)),
			},
			expectedDecommissionRequestedServiceNames: []string{stsName + "-1"},
		},
		{
			name:         "a decommissioned highest node is scaled away while a lower leaving node is still in flight with parallel node operations enabled",
			sdc:          newParallelSDCWithNodes(1),
			rackName:     rackName,
			sts:          newSts(3),
			rackServices: newRackServices(newMemberService(0, nil), newMemberService(1, new(naming.LabelValueFalse)), newMemberService(2, new(naming.LabelValueTrue))),
			expectedConditions: []metav1.Condition{
				makeWaitingForDecommissionCondition(1),
				makeScaleCondition(newSts(3), 2),
			},
			expectedScaledReplicas: new(int32(2)),
		},
		{
			name:         "a multi-node scale-down with the lower leaving node decommissioned first is waited for with parallel node operations enabled",
			sdc:          newParallelSDCWithNodes(1),
			rackName:     rackName,
			sts:          newSts(3),
			rackServices: newRackServices(newMemberService(0, nil), newMemberService(1, new(naming.LabelValueTrue)), newMemberService(2, new(naming.LabelValueFalse))),
			expectedConditions: []metav1.Condition{
				makeWaitingForDecommissionCondition(2),
			},
		},
		{
			name:         "in-flight decommissions are waited for in a stable order with parallel node operations enabled",
			sdc:          newParallelSDCWithNodes(1),
			rackName:     rackName,
			sts:          newSts(3),
			rackServices: newRackServices(newMemberService(0, nil), newMemberService(1, new(naming.LabelValueFalse)), newMemberService(2, new(naming.LabelValueFalse))),
			expectedConditions: []metav1.Condition{
				makeWaitingForDecommissionCondition(1),
				makeWaitingForDecommissionCondition(2),
			},
		},
		{
			name:                   "a fully decommissioned multi-node scale-down concludes by scaling the StatefulSet to the target with parallel node operations enabled",
			sdc:                    newParallelSDCWithNodes(1),
			rackName:               rackName,
			sts:                    newSts(3),
			rackServices:           newRackServices(newMemberService(0, nil), newMemberService(1, new(naming.LabelValueTrue)), newMemberService(2, new(naming.LabelValueTrue))),
			expectedConditions:     []metav1.Condition{makeScaleCondition(newSts(3), 1)},
			expectedScaledReplicas: new(int32(1)),
		},
		{
			name:         "a node count lowered further mid-decommission extends the batch with parallel node operations enabled",
			sdc:          newParallelSDCWithNodes(0),
			rackName:     rackName,
			sts:          newSts(3),
			rackServices: newRackServices(newMemberService(0, nil), newMemberService(1, new(naming.LabelValueFalse)), newMemberService(2, new(naming.LabelValueFalse))),
			expectedConditions: []metav1.Condition{
				makeWaitingForDecommissionCondition(1),
				makeWaitingForDecommissionCondition(2),
				makeStampCondition(newMemberService(0, nil)),
			},
			expectedDecommissionRequestedServiceNames: []string{stsName + "-0"},
		},
		{
			name:         "a multi-node scale-down with a leaving node's Service missing waits for it before requesting any with parallel node operations enabled",
			sdc:          newParallelSDCWithNodes(1),
			rackName:     rackName,
			sts:          newSts(3),
			rackServices: newRackServices(newMemberService(0, nil), newMemberService(2, nil)),
			expectedConditions: []metav1.Condition{
				{
					Type:               statefulSetControllerProgressingCondition,
					Status:             metav1.ConditionTrue,
					Reason:             "WaitingForMissingService",
					Message:            fmt.Sprintf(`Statusfulset "%s/%s" is waiting for service %q to be created`, testNamespace, stsName, stsName+"-1"),
					ObservedGeneration: sdc.Generation,
				},
			},
		},
		{
			name: "an unsupported ScyllaDB version with parallel node operations enabled errors out",
			sdc: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newParallelSDCWithNodes(1)
				sdc.Spec.ScyllaDB.Image = unit.ScyllaDBImageBelowParallelBootstrapThreshold
				return sdc
			}(),
			rackName:            rackName,
			sts:                 newSts(2),
			rackServices:        newRackServices(newMemberService(0, nil), newMemberService(1, nil)),
			expectedErrorString: `can't determine effective parallel node operations enablement: parallel node operations require a semver-parseable ScyllaDB version >= 2026.2, got "2026.1.0"`,
		},
		{
			name:                "a rack missing from the spec errors out with parallel node operations disabled",
			sdc:                 newSDCWithNodes(1),
			rackName:            "missing",
			sts:                 newSts(1),
			rackServices:        newRackServices(),
			expectedErrorString: fmt.Sprintf(`can't get rack "missing" node count of ScyllaDBDatacenter "%[1]s/": can't find rack "missing" in rack spec of ScyllaDBDatacenter "%[1]s/"`, testNamespace),
		},
		{
			name:                "an unparsable leaving node name errors out with parallel node operations disabled",
			sdc:                 newSDCWithNodes(1),
			rackName:            rackName,
			sts:                 newSts(1),
			rackServices:        newRackServices(&corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace, Name: "foo", Labels: map[string]string{naming.RackNameLabel: rackName, naming.DecommissionedLabel: naming.LabelValueTrue}}}),
			expectedErrorString: fmt.Sprintf(`can't get scale-down target node count of rack %[1]q of ScyllaDBDatacenter "%[2]s/": can't get ordinal of decommissioning node "foo" of rack %[1]q of ScyllaDBDatacenter "%[2]s/": didn't find '-' delimiter in string foo`, rackName, testNamespace),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			objects := []runtime.Object{tc.sts}
			for _, svc := range tc.rackServices {
				objects = append(objects, svc)
			}
			client := fake.NewSimpleClientset(objects...)

			var scaledReplicas *int32
			client.PrependReactor("update", "statefulsets", func(action clienttesting.Action) (bool, runtime.Object, error) {
				updateAction := action.(clienttesting.UpdateAction)
				if updateAction.GetSubresource() != "scale" {
					return false, nil, nil
				}
				scaledReplicas = new(updateAction.GetObject().(*autoscalingv1.Scale).Spec.Replicas)
				return true, updateAction.GetObject(), nil
			})

			sdcc := &Controller{
				kubeClient: client,
			}

			gotConditions, err := sdcc.syncRackDecommission(t.Context(), tc.sdc, tc.rackName, tc.sts, tc.rackServices)

			var errString string
			if err != nil {
				errString = err.Error()
			}
			if errString != tc.expectedErrorString {
				t.Fatalf("expected error %q, got %q", tc.expectedErrorString, errString)
			}
			if !reflect.DeepEqual(gotConditions, tc.expectedConditions) {
				t.Errorf("expected and actual conditions differ: %s", cmp.Diff(tc.expectedConditions, gotConditions))
			}

			// A Service whose decommissioned label value changed is one whose decommission was requested: the flow
			// only ever sets the label to false.
			var decommissionRequestedServiceNames []string
			for name := range tc.rackServices {
				svc, err := client.CoreV1().Services(testNamespace).Get(t.Context(), name, metav1.GetOptions{})
				if err != nil {
					t.Fatal(err)
				}
				if svc.Labels[naming.DecommissionedLabel] == tc.rackServices[name].Labels[naming.DecommissionedLabel] {
					continue
				}
				if svc.Labels[naming.DecommissionedLabel] != naming.LabelValueFalse {
					t.Errorf("service %q has unexpected decommissioned label value %q", name, svc.Labels[naming.DecommissionedLabel])
				}
				decommissionRequestedServiceNames = append(decommissionRequestedServiceNames, name)
			}
			slices.Sort(decommissionRequestedServiceNames)
			if diff := cmp.Diff(tc.expectedDecommissionRequestedServiceNames, decommissionRequestedServiceNames, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("decommission requested services differ (-want +got):\n%s", diff)
			}
			if !reflect.DeepEqual(scaledReplicas, tc.expectedScaledReplicas) {
				t.Errorf("expected scaled replicas %v, got %v", tc.expectedScaledReplicas, scaledReplicas)
			}
		})
	}
}

func Test_checkExistingStatefulSetsRolloutStatus(t *testing.T) {
	t.Parallel()

	const rackName = "a"

	sdc := newScyllaDBDatacenter()
	sdc.Generation = 3

	newRackStatefulSet := func(replicas int32, rolledOut bool) *appsv1.StatefulSet {
		sts := newStatefulSet("foo")
		sts.Generation = 1
		sts.Labels = map[string]string{
			naming.RackNameLabel: rackName,
		}
		sts.Spec.Replicas = new(replicas)
		sts.Spec.UpdateStrategy = appsv1.StatefulSetUpdateStrategy{
			Type: appsv1.RollingUpdateStatefulSetStrategyType,
		}
		sts.Status = appsv1.StatefulSetStatus{
			ObservedGeneration: 1,
			Replicas:           replicas,
			ReadyReplicas:      replicas,
			AvailableReplicas:  replicas,
			UpdatedReplicas:    replicas,
			CurrentRevision:    "rev",
			UpdateRevision:     "rev",
		}
		if !rolledOut {
			sts.Status.ReadyReplicas = replicas - 1
			sts.Status.AvailableReplicas = replicas - 1
		}
		return sts
	}
	newLeavingMemberService := func(ordinal int32) *corev1.Service {
		return &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testNamespace,
				Name:      naming.MemberServiceNameForStatefulSet("foo", int(ordinal)),
				Labels: map[string]string{
					naming.RackNameLabel:       rackName,
					naming.DecommissionedLabel: naming.LabelValueFalse,
				},
			},
		}
	}
	waitingForRolloutCondition := metav1.Condition{
		Type:               statefulSetControllerProgressingCondition,
		Status:             metav1.ConditionTrue,
		Reason:             "WaitingForStatefulSetRollout",
		Message:            fmt.Sprintf(`Waiting for StatefulSet "%s/foo" to roll out.`, testNamespace),
		ObservedGeneration: sdc.Generation,
	}

	tt := []struct {
		name               string
		required           []*appsv1.StatefulSet
		existing           map[string]*appsv1.StatefulSet
		services           map[string]*corev1.Service
		expectedConditions []metav1.Condition
	}{
		{
			name:     "a missing StatefulSet is skipped",
			required: []*appsv1.StatefulSet{newRackStatefulSet(2, false)},
			existing: map[string]*appsv1.StatefulSet{},
		},
		{
			name:     "a rolled out StatefulSet yields no conditions",
			required: []*appsv1.StatefulSet{newRackStatefulSet(2, true)},
			existing: map[string]*appsv1.StatefulSet{"foo": newRackStatefulSet(2, true)},
		},
		{
			name:               "a StatefulSet that is not rolled out is waited for",
			required:           []*appsv1.StatefulSet{newRackStatefulSet(2, false)},
			existing:           map[string]*appsv1.StatefulSet{"foo": newRackStatefulSet(2, false)},
			expectedConditions: []metav1.Condition{waitingForRolloutCondition},
		},
		{
			name:     "a StatefulSet whose replicas differ from the required ones is skipped",
			required: []*appsv1.StatefulSet{newRackStatefulSet(1, false)},
			existing: map[string]*appsv1.StatefulSet{"foo": newRackStatefulSet(2, false)},
		},
		{
			name:     "a StatefulSet of a rack with leaving nodes is skipped",
			required: []*appsv1.StatefulSet{newRackStatefulSet(2, false)},
			existing: map[string]*appsv1.StatefulSet{"foo": newRackStatefulSet(2, false)},
			services: map[string]*corev1.Service{
				"foo-1": newLeavingMemberService(1),
			},
		},
		{
			name:     "leaving nodes of another rack do not skip the StatefulSet",
			required: []*appsv1.StatefulSet{newRackStatefulSet(2, false)},
			existing: map[string]*appsv1.StatefulSet{"foo": newRackStatefulSet(2, false)},
			services: map[string]*corev1.Service{
				"bar-1": func() *corev1.Service {
					svc := newLeavingMemberService(1)
					svc.Name = "bar-1"
					svc.Labels[naming.RackNameLabel] = "b"
					return svc
				}(),
			},
			expectedConditions: []metav1.Condition{waitingForRolloutCondition},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			sdcc := &Controller{}
			gotConditions, err := sdcc.checkExistingStatefulSetsRolloutStatus(t.Context(), sdc, tc.required, tc.existing, tc.services)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			if diff := cmp.Diff(tc.expectedConditions, gotConditions); diff != "" {
				t.Errorf("conditions differ (-want +got):\n%s", diff)
			}
		})
	}
}
