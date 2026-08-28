package scylladbdatacenter

import (
	"context"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/scylladb/scylla-operator/pkg/internalapi"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
)

func Test_detectVersionUpgrade(t *testing.T) {
	t.Parallel()

	newVersionedStatefulSet := func(version string) *appsv1.StatefulSet {
		sts := newStatefulSet("sts")
		if len(version) != 0 {
			sts.Labels = map[string]string{naming.ScyllaVersionLabel: version}
		}
		return sts
	}

	tt := []struct {
		name            string
		required        string
		existing        string
		expectedUpgrade bool
		expectedFrom    string
		expectedTo      string
		expectedErr     bool
	}{
		{
			name:            "no upgrade without version labels",
			required:        "",
			existing:        "6.2.0",
			expectedUpgrade: false,
		},
		{
			name:            "no upgrade for the same version",
			required:        "6.2.0",
			existing:        "6.2.0",
			expectedUpgrade: false,
		},
		{
			name:            "no upgrade for a patch version change",
			required:        "6.2.1",
			existing:        "6.2.0",
			expectedUpgrade: false,
		},
		{
			name:            "upgrade for a minor version change",
			required:        "6.3.0",
			existing:        "6.2.0",
			expectedUpgrade: true,
			expectedFrom:    "6.2.0",
			expectedTo:      "6.3.0",
		},
		{
			name:            "upgrade for a major version change",
			required:        "2025.1.0",
			existing:        "6.2.0",
			expectedUpgrade: true,
			expectedFrom:    "6.2.0",
			expectedTo:      "2025.1.0",
		},
		{
			name:        "fails on an unparsable version",
			required:    "latest",
			existing:    "6.2.0",
			expectedErr: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			upgrade, from, to, err := detectVersionUpgrade(newVersionedStatefulSet(tc.required), newVersionedStatefulSet(tc.existing))
			if (err != nil) != tc.expectedErr {
				t.Fatalf("expected error: %t, got: %v", tc.expectedErr, err)
			}
			if upgrade != tc.expectedUpgrade || from != tc.expectedFrom || to != tc.expectedTo {
				t.Errorf("expected (%t, %q, %q), got (%t, %q, %q)", tc.expectedUpgrade, tc.expectedFrom, tc.expectedTo, upgrade, from, to)
			}
		})
	}
}

func Test_transitionUpgradePhase(t *testing.T) {
	t.Parallel()

	sdc := newScyllaDBDatacenter()
	sdc.Name = "basic"
	sdc.UID = "owner"
	sdc.Generation = 4

	upgradeContext := &internalapi.DatacenterUpgradeContext{
		State:             internalapi.PreHooksUpgradePhase,
		FromVersion:       "6.1.0",
		ToVersion:         "6.2.0",
		SystemSnapshotTag: "system",
		DataSnapshotTag:   "data",
	}

	existingCM, err := MakeUpgradeContextConfigMap(sdc, upgradeContext)
	if err != nil {
		t.Fatal(err)
	}

	kubeClient := kubefake.NewSimpleClientset(existingCM)
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	if err := indexer.Add(existingCM); err != nil {
		t.Fatal(err)
	}

	sdcc := &Controller{
		kubeClient:      kubeClient,
		configMapLister: corev1listers.NewConfigMapLister(indexer),
		eventRecorder:   record.NewFakeRecorder(10),
	}

	conditions, err := sdcc.transitionUpgradePhase(context.Background(), sdc, upgradeContext, internalapi.RolloutInitUpgradePhase)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if upgradeContext.State != internalapi.RolloutInitUpgradePhase {
		t.Errorf("expected the upgrade context to be in phase %q, got %q", internalapi.RolloutInitUpgradePhase, upgradeContext.State)
	}

	if len(conditions) != 1 || conditions[0].Reason != internalapi.ProgressingReason || conditions[0].ObservedGeneration != 4 {
		t.Errorf("expected a single progressing condition for generation 4, got %v", conditions)
	}

	cm, err := kubeClient.CoreV1().ConfigMaps(sdc.Namespace).Get(context.Background(), naming.UpgradeContextConfigMapName(sdc), metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}

	recorded := &internalapi.DatacenterUpgradeContext{}
	if err := recorded.Decode(strings.NewReader(cm.Data[naming.UpgradeContextConfigMapKey])); err != nil {
		t.Fatal(err)
	}

	expected := &internalapi.DatacenterUpgradeContext{
		State:             internalapi.RolloutInitUpgradePhase,
		FromVersion:       "6.1.0",
		ToVersion:         "6.2.0",
		SystemSnapshotTag: "system",
		DataSnapshotTag:   "data",
	}
	if diff := cmp.Diff(expected, recorded); diff != "" {
		t.Errorf("recorded upgrade context differs (-want +got):\n%s", diff)
	}
}

func Test_syncUpgrade_noUpgradeInProgress(t *testing.T) {
	t.Parallel()

	sdcc := &Controller{}
	conditions, err := sdcc.syncUpgrade(context.Background(), &statefulSetSyncContext{
		sdc:        newScyllaDBDatacenter(),
		configMaps: map[string]*corev1.ConfigMap{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(conditions) != 0 {
		t.Errorf("expected no conditions, got %v", conditions)
	}
}
