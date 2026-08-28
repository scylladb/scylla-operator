package scylladbdatacenter

import (
	"context"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/scylladb/scylla-operator/pkg/internalapi"
	"github.com/scylladb/scylla-operator/pkg/naming"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
)

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

			got, err := decodeUpgradeContext(tc.configMap)

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
