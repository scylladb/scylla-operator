package operator

import (
	"reflect"
	"sort"
	"testing"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestListScyllaClustersWithNonRFC1123SubdomainTaskNames(t *testing.T) {
	tt := []struct {
		name            string
		objects         []runtime.Object
		expectedInvalid []string
		expectedErr     error
	}{
		{
			name:            "no ScyllaClusters returns empty result",
			objects:         nil,
			expectedInvalid: nil,
			expectedErr:     nil,
		},
		{
			name: "ScyllaCluster with valid task names returns empty result",
			objects: []runtime.Object{
				&scyllav1.ScyllaCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-1",
						Namespace: "ns-1",
					},
					Spec: scyllav1.ScyllaClusterSpec{
						Repairs: []scyllav1.RepairTaskSpec{
							{TaskSpec: scyllav1.TaskSpec{Name: "valid-repair"}},
						},
						Backups: []scyllav1.BackupTaskSpec{
							{TaskSpec: scyllav1.TaskSpec{Name: "valid-backup"}},
						},
					},
				},
			},
			expectedInvalid: nil,
			expectedErr:     nil,
		},
		{
			name: "ScyllaCluster with invalid repair task name is reported",
			objects: []runtime.Object{
				&scyllav1.ScyllaCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-1",
						Namespace: "ns-1",
					},
					Spec: scyllav1.ScyllaClusterSpec{
						Repairs: []scyllav1.RepairTaskSpec{
							{TaskSpec: scyllav1.TaskSpec{Name: "INVALID_REPAIR"}},
						},
					},
				},
			},
			expectedInvalid: []string{"ns-1/cluster-1"},
			expectedErr:     nil,
		},
		{
			name: "ScyllaCluster with invalid backup task name is reported",
			objects: []runtime.Object{
				&scyllav1.ScyllaCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-1",
						Namespace: "ns-1",
					},
					Spec: scyllav1.ScyllaClusterSpec{
						Backups: []scyllav1.BackupTaskSpec{
							{TaskSpec: scyllav1.TaskSpec{Name: "INVALID_BACKUP"}},
						},
					},
				},
			},
			expectedInvalid: []string{"ns-1/cluster-1"},
			expectedErr:     nil,
		},
		{
			name: "multiple ScyllaClusters with mixed valid and invalid task names returns only invalid refs",
			objects: []runtime.Object{
				&scyllav1.ScyllaCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "valid-cluster",
						Namespace: "ns-1",
					},
					Spec: scyllav1.ScyllaClusterSpec{
						Repairs: []scyllav1.RepairTaskSpec{
							{TaskSpec: scyllav1.TaskSpec{Name: "valid-repair"}},
						},
						Backups: []scyllav1.BackupTaskSpec{
							{TaskSpec: scyllav1.TaskSpec{Name: "valid-backup"}},
						},
					},
				},
				&scyllav1.ScyllaCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "invalid-cluster-1",
						Namespace: "ns-1",
					},
					Spec: scyllav1.ScyllaClusterSpec{
						Repairs: []scyllav1.RepairTaskSpec{
							{TaskSpec: scyllav1.TaskSpec{Name: "INVALID"}},
						},
					},
				},
				&scyllav1.ScyllaCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "invalid-cluster-2",
						Namespace: "ns-2",
					},
					Spec: scyllav1.ScyllaClusterSpec{
						Backups: []scyllav1.BackupTaskSpec{
							{TaskSpec: scyllav1.TaskSpec{Name: "ALSO_INVALID"}},
						},
					},
				},
			},
			expectedInvalid: []string{"ns-1/invalid-cluster-1", "ns-2/invalid-cluster-2"},
			expectedErr:     nil,
		},
		{
			name: "ScyllaCluster with both invalid repair and backup task names appears only once",
			objects: []runtime.Object{
				&scyllav1.ScyllaCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-1",
						Namespace: "ns-1",
					},
					Spec: scyllav1.ScyllaClusterSpec{
						Repairs: []scyllav1.RepairTaskSpec{
							{TaskSpec: scyllav1.TaskSpec{Name: "INVALID_REPAIR"}},
						},
						Backups: []scyllav1.BackupTaskSpec{
							{TaskSpec: scyllav1.TaskSpec{Name: "INVALID_BACKUP"}},
						},
					},
				},
			},
			expectedInvalid: []string{"ns-1/cluster-1"},
			expectedErr:     nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			fakeClient := fake.NewSimpleClientset(tc.objects...)

			got, err := listScyllaClustersWithNonRFC1123SubdomainTaskNames(t.Context(), fakeClient.ScyllaV1())
			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Errorf("expected error %v, got %v", tc.expectedErr, err)
			}

			sort.Strings(got)
			sort.Strings(tc.expectedInvalid)

			if !reflect.DeepEqual(got, tc.expectedInvalid) {
				t.Errorf("expected invalid refs %v, got %v", tc.expectedInvalid, got)
			}
		})
	}
}
