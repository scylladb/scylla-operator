package scylladbdatacenter

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kubefake "k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

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

			res, err := sdcc.pruneStatefulSets(context.Background(), &statefulSetSyncContext{
				sdc:                  newScyllaDBDatacenter(),
				status:               status,
				requiredStatefulSets: tc.required,
				existingStatefulSets: tc.existing,
			})
			conditions := res.progressingConditions
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
