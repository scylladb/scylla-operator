// Copyright (C) 2025 ScyllaDB

package nodeconfig

import (
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_makeSysctlConfigMap(t *testing.T) {
	t.Parallel()

	getTestNodeConfig := func() *scyllav1alpha1.NodeConfig {
		return &scyllav1alpha1.NodeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "test",
				UID:       "uid",
			},
			Spec: scyllav1alpha1.NodeConfigSpec{
				Sysctls: []corev1.Sysctl{
					{
						Name:  "fs.aio-max-nr",
						Value: "30000000",
					},
					{
						Name:  "fs.file-max",
						Value: "9223372036854775807",
					},
					{
						Name:  "fs.nr_open",
						Value: "1073741816",
					},
					{
						Name:  "fs.inotify.max_user_instances",
						Value: "1200",
					},
					{
						Name:  "vm.swappiness",
						Value: "1",
					},
					{
						Name:  "vm.vfs_cache_pressure",
						Value: "2000",
					},
				},
			},
		}
	}

	tt := []struct {
		name        string
		nc          *scyllav1alpha1.NodeConfig
		expected    *corev1.ConfigMap
		expectedErr error
	}{
		{
			name: "nil sysctls",
			nc: func() *scyllav1alpha1.NodeConfig {
				nc := getTestNodeConfig()
				nc.Spec.Sysctls = nil
				return nc
			}(),
			expected: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-sysctl-rh5ie",
					Namespace: "scylla-operator-node-tuning",
					Labels: map[string]string{
						"app.kubernetes.io/name":                        "scylla-node-config",
						"scylla-operator.scylladb.com/node-config-name": "test",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "NodeConfig",
							Name:               "test",
							UID:                "uid",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				Data: map[string]string{
					"99-override_scylla-operator_scylladb_com.conf": ``,
				},
			},
			expectedErr: nil,
		},
		{
			name: "empty sysctls",
			nc: &scyllav1alpha1.NodeConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
					UID:       "uid",
				},
				Spec: scyllav1alpha1.NodeConfigSpec{
					Sysctls: []corev1.Sysctl{},
				},
			},
			expected: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-sysctl-rh5ie",
					Namespace: "scylla-operator-node-tuning",
					Labels: map[string]string{
						"app.kubernetes.io/name":                        "scylla-node-config",
						"scylla-operator.scylladb.com/node-config-name": "test",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "NodeConfig",
							Name:               "test",
							UID:                "uid",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				Data: map[string]string{
					"99-override_scylla-operator_scylladb_com.conf": ``,
				},
			},
			expectedErr: nil,
		},
		{
			name: "non-empty sysctls",
			nc: &scyllav1alpha1.NodeConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
					UID:       "uid",
				},
				Spec: scyllav1alpha1.NodeConfigSpec{
					Sysctls: []corev1.Sysctl{
						{
							Name:  "fs.aio-max-nr",
							Value: "30000000",
						},
						{
							Name:  "fs.file-max",
							Value: "9223372036854775807",
						},
						{
							Name:  "fs.nr_open",
							Value: "1073741816",
						},
						{
							Name:  "fs.inotify.max_user_instances",
							Value: "1200",
						},
						{
							Name:  "vm.swappiness",
							Value: "1",
						},
						{
							Name:  "vm.vfs_cache_pressure",
							Value: "2000",
						},
					},
				},
			},
			expected: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-sysctl-rh5ie",
					Namespace: "scylla-operator-node-tuning",
					Labels: map[string]string{
						"app.kubernetes.io/name":                        "scylla-node-config",
						"scylla-operator.scylladb.com/node-config-name": "test",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "NodeConfig",
							Name:               "test",
							UID:                "uid",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				Data: map[string]string{
					"99-override_scylla-operator_scylladb_com.conf": strings.TrimPrefix(`
fs.aio-max-nr = 30000000
fs.file-max = 9223372036854775807
fs.nr_open = 1073741816
fs.inotify.max_user_instances = 1200
vm.swappiness = 1
vm.vfs_cache_pressure = 2000
`, "\n"),
				},
			},
			expectedErr: nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := makeSysctlConfigMap(tc.nc)

			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Fatalf("expected and actual errors differ: %s", cmp.Diff(tc.expectedErr, err, cmpopts.EquateErrors()))
			}

			if !equality.Semantic.DeepEqual(got, tc.expected) {
				t.Errorf("expected and actual differ: %s", cmp.Diff(tc.expected, got))
			}
		})

	}
}
