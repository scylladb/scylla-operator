package scylladbdatacenter

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/internalapi"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
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
			expectedCreated:     nil,
			expectedConditions:  nil,
			expectedErrorString: `can't create missing statefulset "default/foo": apply failed`,
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
