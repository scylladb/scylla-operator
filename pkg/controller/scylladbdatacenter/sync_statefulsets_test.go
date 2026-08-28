package scylladbdatacenter

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/internalapi"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/test/unit"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kubefake "k8s.io/client-go/kubernetes/fake"
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

func Test_pruneStatefulSets(t *testing.T) {
	t.Parallel()

	newRackStatefulSet := func(name, rackName string, uid types.UID) *appsv1.StatefulSet {
		sts := newStatefulSet(name)
		sts.UID = uid
		if len(rackName) != 0 {
			sts.Labels = map[string]string{naming.RackNameLabel: rackName}
		}
		return sts
	}

	tt := []struct {
		name                  string
		required              []*appsv1.StatefulSet
		existing              map[string]*appsv1.StatefulSet
		rackStatuses          []scyllav1alpha1.RackStatus
		expectedDeleted       []string
		expectedRackStatuses  []scyllav1alpha1.RackStatus
		expectedConditionsLen int
	}{
		{
			name: "keeps required StatefulSets",
			required: []*appsv1.StatefulSet{
				newRackStatefulSet("foo", "a", "1"),
			},
			existing: map[string]*appsv1.StatefulSet{
				"foo": newRackStatefulSet("foo", "a", "1"),
			},
			rackStatuses:          []scyllav1alpha1.RackStatus{{Name: "a"}},
			expectedDeleted:       nil,
			expectedRackStatuses:  []scyllav1alpha1.RackStatus{{Name: "a"}},
			expectedConditionsLen: 0,
		},
		{
			name: "deletes an excessive StatefulSet and drops its rack status",
			required: []*appsv1.StatefulSet{
				newRackStatefulSet("foo", "a", "1"),
			},
			existing: map[string]*appsv1.StatefulSet{
				"foo": newRackStatefulSet("foo", "a", "1"),
				"bar": newRackStatefulSet("bar", "b", "2"),
			},
			rackStatuses:          []scyllav1alpha1.RackStatus{{Name: "a"}, {Name: "b"}},
			expectedDeleted:       []string{"bar"},
			expectedRackStatuses:  []scyllav1alpha1.RackStatus{{Name: "a"}},
			expectedConditionsLen: 1,
		},
		{
			name:     "skips StatefulSets that are already being deleted",
			required: []*appsv1.StatefulSet{},
			existing: map[string]*appsv1.StatefulSet{
				"bar": func() *appsv1.StatefulSet {
					sts := newRackStatefulSet("bar", "b", "2")
					sts.DeletionTimestamp = &metav1.Time{Time: time.Now()}
					return sts
				}(),
			},
			rackStatuses:          []scyllav1alpha1.RackStatus{{Name: "b"}},
			expectedDeleted:       nil,
			expectedRackStatuses:  []scyllav1alpha1.RackStatus{{Name: "b"}},
			expectedConditionsLen: 0,
		},
		{
			name:     "deletes an excessive StatefulSet without a rack label and keeps the statuses",
			required: []*appsv1.StatefulSet{},
			existing: map[string]*appsv1.StatefulSet{
				"bar": newRackStatefulSet("bar", "", "2"),
			},
			rackStatuses:          []scyllav1alpha1.RackStatus{{Name: "b"}},
			expectedDeleted:       []string{"bar"},
			expectedRackStatuses:  []scyllav1alpha1.RackStatus{{Name: "b"}},
			expectedConditionsLen: 1,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			existingObjects := make([]runtime.Object, 0, len(tc.existing))
			for _, sts := range tc.existing {
				existingObjects = append(existingObjects, sts)
			}
			kubeClient := kubefake.NewSimpleClientset(existingObjects...)

			sdcc := &Controller{kubeClient: kubeClient}
			status := &scyllav1alpha1.ScyllaDBDatacenterStatus{Racks: tc.rackStatuses}

			conditions, err := sdcc.pruneStatefulSets(context.Background(), newScyllaDBDatacenter(), status, tc.required, tc.existing)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(conditions) != tc.expectedConditionsLen {
				t.Errorf("expected %d progressing condition(s), got %d: %v", tc.expectedConditionsLen, len(conditions), conditions)
			}

			var deleted []string
			for _, action := range kubeClient.Actions() {
				if deleteAction, ok := action.(clienttesting.DeleteAction); ok {
					deleted = append(deleted, deleteAction.GetName())
				}
			}
			sort.Strings(deleted)
			if !cmp.Equal(deleted, tc.expectedDeleted) {
				t.Errorf("deleted StatefulSets differ: expected %v, got %v", tc.expectedDeleted, deleted)
			}

			if diff := cmp.Diff(tc.expectedRackStatuses, status.Racks); diff != "" {
				t.Errorf("rack statuses differ (-want +got):\n%s", diff)
			}
		})
	}
}

func Test_checkExistingStatefulSetsRolloutStatus(t *testing.T) {
	t.Parallel()

	newRollingStatefulSet := func(name string, replicas int32, rolledOut bool) *appsv1.StatefulSet {
		sts := newStatefulSet(name)
		sts.Generation = 2
		sts.Spec.Replicas = new(replicas)
		sts.Spec.UpdateStrategy.Type = appsv1.RollingUpdateStatefulSetStrategyType
		sts.Status.ObservedGeneration = 1
		if rolledOut {
			sts.Status.ObservedGeneration = 2
			sts.Status.Replicas = replicas
			sts.Status.ReadyReplicas = replicas
			sts.Status.AvailableReplicas = replicas
			sts.Status.UpdatedReplicas = replicas
			sts.Status.CurrentRevision = "rev"
			sts.Status.UpdateRevision = "rev"
		}
		return sts
	}

	tt := []struct {
		name               string
		required           []*appsv1.StatefulSet
		existing           map[string]*appsv1.StatefulSet
		expectedConditions []metav1.Condition
	}{
		{
			name:               "ignores missing StatefulSets",
			required:           []*appsv1.StatefulSet{newRollingStatefulSet("foo", 1, false)},
			existing:           map[string]*appsv1.StatefulSet{},
			expectedConditions: nil,
		},
		{
			name:               "no condition for a rolled out StatefulSet",
			required:           []*appsv1.StatefulSet{newRollingStatefulSet("foo", 1, true)},
			existing:           map[string]*appsv1.StatefulSet{"foo": newRollingStatefulSet("foo", 1, true)},
			expectedConditions: nil,
		},
		{
			name:     "waits for a StatefulSet that is not rolled out",
			required: []*appsv1.StatefulSet{newRollingStatefulSet("foo", 1, true)},
			existing: map[string]*appsv1.StatefulSet{"foo": newRollingStatefulSet("foo", 1, false)},
			expectedConditions: []metav1.Condition{
				{
					Type:               statefulSetControllerProgressingCondition,
					Status:             metav1.ConditionTrue,
					Reason:             "WaitingForStatefulSetRollout",
					Message:            `Waiting for StatefulSet "default/foo" to roll out.`,
					ObservedGeneration: 0,
				},
			},
		},
		{
			name:               "skips a StatefulSet that is about to be scaled",
			required:           []*appsv1.StatefulSet{newRollingStatefulSet("foo", 2, true)},
			existing:           map[string]*appsv1.StatefulSet{"foo": newRollingStatefulSet("foo", 1, false)},
			expectedConditions: nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			sdcc := &Controller{}
			conditions, err := sdcc.checkExistingStatefulSetsRolloutStatus(context.Background(), newScyllaDBDatacenter(), tc.required, tc.existing)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if diff := cmp.Diff(tc.expectedConditions, conditions); diff != "" {
				t.Errorf("conditions differ (-want +got):\n%s", diff)
			}
		})
	}
}

func Test_setStatefulSetsAvailableStatusCondition(t *testing.T) {
	t.Parallel()

	const image = "docker.io/scylladb/scylla:6.2.0"

	newSDC := func(nodes int32, racks ...string) *scyllav1alpha1.ScyllaDBDatacenter {
		sdc := newScyllaDBDatacenter()
		sdc.Generation = 7
		sdc.Spec.ScyllaDB.Image = image
		sdc.Spec.RackTemplate = &scyllav1alpha1.RackTemplate{Nodes: new(nodes)}
		for _, rack := range racks {
			sdc.Spec.Racks = append(sdc.Spec.Racks, scyllav1alpha1.RackSpec{Name: rack})
		}
		return sdc
	}

	newRackStatus := func(name, version string, ready, updated int32, stale bool) scyllav1alpha1.RackStatus {
		return scyllav1alpha1.RackStatus{
			Name:           name,
			CurrentVersion: version,
			ReadyNodes:     new(ready),
			UpdatedNodes:   new(updated),
			Stale:          new(stale),
		}
	}

	tt := []struct {
		name              string
		sdc               *scyllav1alpha1.ScyllaDBDatacenter
		racks             []scyllav1alpha1.RackStatus
		expectedCondition metav1.Condition
	}{
		{
			name: "available when all racks are at the desired version, updated and ready",
			sdc:  newSDC(2, "a", "b"),
			racks: []scyllav1alpha1.RackStatus{
				newRackStatus("a", "6.2.0", 2, 2, false),
				newRackStatus("b", "6.2.0", 2, 2, false),
			},
			expectedCondition: metav1.Condition{
				Type:               statefulSetControllerAvailableCondition,
				Status:             metav1.ConditionTrue,
				Reason:             internalapi.AsExpectedReason,
				Message:            "",
				ObservedGeneration: 7,
			},
		},
		{
			name: "not available when a rack is at a different version",
			sdc:  newSDC(2, "a", "b"),
			racks: []scyllav1alpha1.RackStatus{
				newRackStatus("a", "6.2.0", 2, 2, false),
				newRackStatus("b", "6.1.0", 2, 2, false),
			},
			expectedCondition: metav1.Condition{
				Type:               statefulSetControllerAvailableCondition,
				Status:             metav1.ConditionFalse,
				Reason:             "RacksNotAtDesiredVersion",
				Message:            `Racks "b" are not in the desired version`,
				ObservedGeneration: 7,
			},
		},
		{
			name: "not available when members are not updated",
			sdc:  newSDC(2, "a"),
			racks: []scyllav1alpha1.RackStatus{
				newRackStatus("a", "6.2.0", 2, 1, false),
			},
			expectedCondition: metav1.Condition{
				Type:               statefulSetControllerAvailableCondition,
				Status:             metav1.ConditionFalse,
				Reason:             "MembersNotUpdated",
				Message:            "Only 1 out of 2 member(s) have been updated",
				ObservedGeneration: 7,
			},
		},
		{
			name: "not available when members are not ready",
			sdc:  newSDC(2, "a"),
			racks: []scyllav1alpha1.RackStatus{
				newRackStatus("a", "6.2.0", 1, 2, false),
			},
			expectedCondition: metav1.Condition{
				Type:               statefulSetControllerAvailableCondition,
				Status:             metav1.ConditionFalse,
				Reason:             "MembersNotReady",
				Message:            "Only 1 out of 2 member(s) are ready",
				ObservedGeneration: 7,
			},
		},
		{
			name: "stale rack statuses don't count towards updated members",
			sdc:  newSDC(2, "a"),
			racks: []scyllav1alpha1.RackStatus{
				newRackStatus("a", "6.2.0", 2, 2, true),
			},
			expectedCondition: metav1.Condition{
				Type:               statefulSetControllerAvailableCondition,
				Status:             metav1.ConditionFalse,
				Reason:             "MembersNotUpdated",
				Message:            "Only 0 out of 2 member(s) have been updated",
				ObservedGeneration: 7,
			},
		},
		{
			name:  "a rack without a status is skipped",
			sdc:   newSDC(2, "a"),
			racks: nil,
			expectedCondition: metav1.Condition{
				Type:               statefulSetControllerAvailableCondition,
				Status:             metav1.ConditionFalse,
				Reason:             "MembersNotUpdated",
				Message:            "Only 0 out of 2 member(s) have been updated",
				ObservedGeneration: 7,
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			sdcc := &Controller{}
			status := &scyllav1alpha1.ScyllaDBDatacenterStatus{Racks: tc.racks}
			sdcc.setStatefulSetsAvailableStatusCondition(tc.sdc, status)

			if len(status.Conditions) != 1 {
				t.Fatalf("expected exactly one condition, got %v", status.Conditions)
			}
			got := status.Conditions[0]
			got.LastTransitionTime = metav1.Time{}
			if diff := cmp.Diff(tc.expectedCondition, got); diff != "" {
				t.Errorf("condition differs (-want +got):\n%s", diff)
			}
		})
	}
}

func Test_decodeUpgradeContext(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name                string
		configMap           *corev1.ConfigMap
		expected            *internalapi.DatacenterUpgradeContext
		expectedErrorString string
	}{
		{
			name: "decodes a valid upgrade context",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace, Name: "uc"},
				Data: map[string]string{
					naming.UpgradeContextConfigMapKey: `{"state":"RolloutRun","fromVersion":"6.1.0","toVersion":"6.2.0","systemSnapshotTag":"s","dataSnapshotTag":"d"}`,
				},
			},
			expected: &internalapi.DatacenterUpgradeContext{
				State:             internalapi.RolloutRunUpgradePhase,
				FromVersion:       "6.1.0",
				ToVersion:         "6.2.0",
				SystemSnapshotTag: "s",
				DataSnapshotTag:   "d",
			},
			expectedErrorString: "",
		},
		{
			name: "fails on a missing key",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace, Name: "uc"},
				Data:       map[string]string{},
			},
			expected:            nil,
			expectedErrorString: `upgrade context ConfigMap "default/uc" is missing "upgrade-context.json" key`,
		},
		{
			name: "fails on malformed data",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace, Name: "uc"},
				Data: map[string]string{
					naming.UpgradeContextConfigMapKey: `{`,
				},
			},
			expected:            nil,
			expectedErrorString: `can't decode ugprade context from ConfigMap "default/uc": can't json decode ugprade context: unexpected EOF`,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			sdcc := &Controller{}
			got, err := sdcc.decodeUpgradeContext(tc.configMap)

			gotErrorString := ""
			if err != nil {
				gotErrorString = err.Error()
			}
			if gotErrorString != tc.expectedErrorString {
				t.Fatalf("expected error %q, got %q", tc.expectedErrorString, gotErrorString)
			}

			if diff := cmp.Diff(tc.expected, got); diff != "" {
				t.Errorf("upgrade context differs (-want +got):\n%s", diff)
			}
		})
	}
}
