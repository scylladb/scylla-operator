// Copyright (c) 2023 ScyllaDB

package naming

import (
	"fmt"
	"reflect"
	"testing"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_ImageToVersion(t *testing.T) {
	t.Parallel()

	tcs := []struct {
		name            string
		image           string
		expectedVersion string
		expectedError   error
	}{
		{
			name:            "tagged reference with a domain",
			image:           "docker.io/scylladb/scylla:5.1.15",
			expectedVersion: "5.1.15",
			expectedError:   nil,
		},
		{
			name:            "tagged reference without a domain",
			image:           "scylladb/scylla:5.1.15",
			expectedVersion: "5.1.15",
			expectedError:   nil,
		},
		{
			name:            "tagged reference with port in domain",
			image:           "someproxy:1234/proxy/scylladb/scylla:5.1.15",
			expectedVersion: "5.1.15",
			expectedError:   nil,
		},
		{
			name:            "tagged and digested reference",
			image:           "docker.io/scylladb/scylla:5.1.15@sha256:12a95529d94498d8f56fc4596f8ac570e961618fab7ab5cf3e3df781c2b8fd33",
			expectedVersion: "5.1.15",
			expectedError:   nil,
		},
		{
			name:          "digested reference",
			image:         "docker.io/scylladb/scylla@sha256:12a95529d94498d8f56fc4596f8ac570e961618fab7ab5cf3e3df781c2b8fd33",
			expectedError: fmt.Errorf("invalid, non-tagged image reference of type reference.canonicalReference: docker.io/scylladb/scylla@sha256:12a95529d94498d8f56fc4596f8ac570e961618fab7ab5cf3e3df781c2b8fd33"),
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			version, err := ImageToVersion(tc.image)

			if !reflect.DeepEqual(err, tc.expectedError) {
				t.Fatalf("expected error: %#v, got: %#v", tc.expectedError, err)
			}
			if !reflect.DeepEqual(version, tc.expectedVersion) {
				t.Fatalf("expected version: %s, got: %s", tc.expectedVersion, version)
			}
		})
	}
}

func Test_ScyllaDBManagerClusterRegistrationNameForScyllaDBDatacenter(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name          string
		sdc           *scyllav1alpha1.ScyllaDBDatacenter
		expectedName  string
		expectedError error
	}{
		{
			name: "not truncated",
			sdc: &scyllav1alpha1.ScyllaDBDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Name: "basic",
				},
			},
			expectedName:  "scylladbdatacenter-basic-3tpwb",
			expectedError: nil,
		},
		{
			name: "truncated",
			sdc: &scyllav1alpha1.ScyllaDBDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Name: "hzz5k3svtyvz2xza2md1mnurkzo41548jhoo22mqyuq40cdpqmga47287s5tqk7hd0zv1wizww7fn5e8nk3i4ckohflxo9tjao3zqlbxvkv724nozd267lr2r7u48dnua9jhalkjdyhoputvppmmungliyc16lqqkga2fg9ircczyp7ekjkuo35nbaevx88d72312c5s19goq6aehyun71bxrnjf95oklso5ykdvw93ya3jd15bs3gmqm71g4ncq",
				},
			},
			expectedName:  "scylladbdatacenter-hzz5k3svtyvz2xza2md1mnurkzo41548jhoo22mqyuq40cdpqmga47287s5tqk7hd0zv1wizww7fn5e8nk3i4ckohflxo9tjao3zqlbxvkv724nozd267lr2r7u48dnua9jhalkjdyhoputvppmmungliyc16lqqkga2fg9ircczyp7ekjkuo35nbaevx88d72312c5s19goq6aehyun71bxrnjf95oklso5-2c7e7",
			expectedError: nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			name, err := ScyllaDBManagerClusterRegistrationNameForScyllaDBDatacenter(tc.sdc)

			if !reflect.DeepEqual(err, tc.expectedError) {
				t.Fatalf("expected error: %#v, got: %#v", tc.expectedError, err)
			}

			if !reflect.DeepEqual(name, tc.expectedName) {
				t.Errorf("expected name: %s, got: %s", tc.expectedName, name)
			}
		})
	}
}
