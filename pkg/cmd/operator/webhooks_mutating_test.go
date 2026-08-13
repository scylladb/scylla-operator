// Copyright (c) 2026 ScyllaDB

package operator

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/scheme"
	jsonpatch "gomodules.xyz/jsonpatch/v2"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func newMutateAdmissionRequest(gvr metav1.GroupVersionResource, operation admissionv1.Operation, raw []byte) *admissionv1.AdmissionRequest {
	return &admissionv1.AdmissionRequest{
		UID:       "uid",
		Resource:  gvr,
		Operation: operation,
		Object:    runtime.RawExtension{Raw: raw},
	}
}

// Test_mutate covers the dispatcher machinery against mocked defaulters.
// The defaulters actually served by the webhook server are covered by Test_mutate_withDefaultDefaulters.
func Test_mutate(t *testing.T) {
	t.Parallel()

	podsGVR := metav1.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "pods",
	}
	configMapsGVR := metav1.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "configmaps",
	}

	podRawWithSpec := []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod","namespace":"test"},"spec":{"containers":null}}`)
	podRawWithoutSpec := []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod","namespace":"test"}}`)

	mutatingPodDefaulters := map[schema.GroupVersionResource]Defaulter{
		corev1.SchemeGroupVersion.WithResource("pods"): &GenericDefaulter[*corev1.Pod]{
			DefaultOnCreateFunc: func(pod *corev1.Pod) {
				pod.Spec.Hostname = "defaulted"
			},
		},
	}

	tt := []struct {
		name            string
		req             *admissionv1.AdmissionRequest
		defaulters      map[schema.GroupVersionResource]Defaulter
		expectedPatches []jsonpatch.Operation
		expectedError   error
	}{
		{
			name:       "create with a mutating defaulter returns the patch operations",
			req:        newMutateAdmissionRequest(podsGVR, admissionv1.Create, podRawWithSpec),
			defaulters: mutatingPodDefaulters,
			expectedPatches: []jsonpatch.Operation{
				{
					Operation: "add",
					Path:      "/spec/hostname",
					Value:     "defaulted",
				},
			},
			expectedError: nil,
		},
		{
			name: "create with a no-op defaulter returns no patch operations",
			req:  newMutateAdmissionRequest(podsGVR, admissionv1.Create, podRawWithSpec),
			defaulters: map[schema.GroupVersionResource]Defaulter{
				corev1.SchemeGroupVersion.WithResource("pods"): &GenericDefaulter[*corev1.Pod]{
					DefaultOnCreateFunc: func(pod *corev1.Pod) {},
				},
			},
			expectedPatches: nil,
			expectedError:   nil,
		},
		{
			// The patch is computed against the decoded object, whose spec always serializes, while the raw
			// payload the API server applies it to may not have one. The patch is returned regardless: the
			// API server rejects the creation when it doesn't apply, instead of the object being silently
			// persisted without its defaults.
			name:       "create whose raw payload is missing the patched field's parent still returns the patch operations",
			req:        newMutateAdmissionRequest(podsGVR, admissionv1.Create, podRawWithoutSpec),
			defaulters: mutatingPodDefaulters,
			expectedPatches: []jsonpatch.Operation{
				{
					Operation: "add",
					Path:      "/spec/hostname",
					Value:     "defaulted",
				},
			},
			expectedError: nil,
		},
		{
			name:            "unregistered GVR is rejected",
			req:             newMutateAdmissionRequest(configMapsGVR, admissionv1.Create, podRawWithSpec),
			defaulters:      mutatingPodDefaulters,
			expectedPatches: nil,
			expectedError:   fmt.Errorf(`unsupported GVR "/v1, Resource=configmaps"`),
		},
		{
			// A defaulter registered for a GVR whose payloads decode into another type must be reported, not
			// panicked on, and it has to deny: silently skipping the defaulting would let an object through
			// without the fields it's supposed to be created with.
			name: "a defaulter registered for a mismatched type is rejected",
			req:  newMutateAdmissionRequest(podsGVR, admissionv1.Create, podRawWithSpec),
			defaulters: map[schema.GroupVersionResource]Defaulter{
				corev1.SchemeGroupVersion.WithResource("pods"): &GenericDefaulter[*corev1.ConfigMap]{
					DefaultOnCreateFunc: func(cm *corev1.ConfigMap) {
						panic("unexpected call to DefaultOnCreateFunc")
					},
				},
			},
			expectedPatches: nil,
			expectedError:   fmt.Errorf(`can't default object "/v1, Resource=pods": %w`, fmt.Errorf("expected *v1.ConfigMap, got *v1.Pod")),
		},
		{
			name:            "update operation is rejected",
			req:             newMutateAdmissionRequest(podsGVR, admissionv1.Update, podRawWithSpec),
			defaulters:      mutatingPodDefaulters,
			expectedPatches: nil,
			expectedError:   fmt.Errorf(`unsupported operation "UPDATE"`),
		},
		{
			name:            "delete operation is rejected",
			req:             newMutateAdmissionRequest(podsGVR, admissionv1.Delete, podRawWithSpec),
			defaulters:      mutatingPodDefaulters,
			expectedPatches: nil,
			expectedError:   fmt.Errorf(`unsupported operation "DELETE"`),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			patches, err := mutate(tc.req, tc.defaulters)
			if !reflect.DeepEqual(tc.expectedError, err) {
				t.Fatalf("expected and actual errors differ: %s", cmp.Diff(tc.expectedError, err, cmpopts.EquateErrors()))
			}

			if !reflect.DeepEqual(tc.expectedPatches, patches) {
				t.Errorf("expected and actual patches differ: %s", cmp.Diff(tc.expectedPatches, patches))
			}
		})
	}
}

// Test_mutate_withDefaultDefaulters covers mutate against DefaultDefaulters, the registry served by the webhook
// server, asserting which resources it defaults and what the registered defaulters stamp.
// The defaulters themselves are covered by the defaulting package's tests, the dispatcher machinery by Test_mutate.
func Test_mutate_withDefaultDefaulters(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name            string
		req             *admissionv1.AdmissionRequest
		expectedPatches []jsonpatch.Operation
		expectedError   error
	}{
		{
			// Sequential is never stamped: an unset bootstrapPolicy is left unset, so that objects whose owners
			// never made a choice keep resolving it rather than being pinned to today's resolution.
			name: "a ScyllaCluster with a version not supporting parallel bootstrap is admitted unchanged",
			req: newMutateAdmissionRequest(metav1.GroupVersionResource{
				Group:    "scylla.scylladb.com",
				Version:  "v1",
				Resource: "scyllaclusters",
			}, admissionv1.Create, []byte(`{"apiVersion":"scylla.scylladb.com/v1","kind":"ScyllaCluster","metadata":{"name":"basic","namespace":"test"},"spec":{"version":"2026.1.0","agentVersion":"3.4.0","datacenter":{"name":"dc1","racks":[{"name":"rack1","members":3,"storage":{"capacity":"1Gi"}}]}}}`)),
			expectedPatches: nil,
			expectedError:   nil,
		},
		{
			name: "a ScyllaCluster with a version supporting parallel bootstrap is stamped with a Parallel bootstrapPolicy",
			req: newMutateAdmissionRequest(metav1.GroupVersionResource{
				Group:    "scylla.scylladb.com",
				Version:  "v1",
				Resource: "scyllaclusters",
			}, admissionv1.Create, []byte(`{"apiVersion":"scylla.scylladb.com/v1","kind":"ScyllaCluster","metadata":{"name":"basic","namespace":"test"},"spec":{"version":"2026.2.0","agentVersion":"3.4.0","datacenter":{"name":"dc1","racks":[{"name":"rack1","members":3,"storage":{"capacity":"1Gi"}}]}}}`)),
			expectedPatches: []jsonpatch.Operation{
				{
					Operation: "add",
					Path:      "/spec/bootstrapPolicy",
					Value:     string(scyllav1.BootstrapPolicyParallel),
				},
			},
			expectedError: nil,
		},
		{
			name: "a ScyllaCluster with an explicit bootstrapPolicy passes through unchanged",
			req: newMutateAdmissionRequest(metav1.GroupVersionResource{
				Group:    "scylla.scylladb.com",
				Version:  "v1",
				Resource: "scyllaclusters",
			}, admissionv1.Create, []byte(`{"apiVersion":"scylla.scylladb.com/v1","kind":"ScyllaCluster","metadata":{"name":"basic","namespace":"test"},"spec":{"version":"2026.2.0","agentVersion":"3.4.0","bootstrapPolicy":"Sequential","datacenter":{"name":"dc1","racks":[{"name":"rack1","members":3,"storage":{"capacity":"1Gi"}}]}}}`)),
			expectedPatches: nil,
			expectedError:   nil,
		},
		{
			name: "a ScyllaDBDatacenter with an image not supporting parallel bootstrap is admitted unchanged",
			req: newMutateAdmissionRequest(metav1.GroupVersionResource{
				Group:    "scylla.scylladb.com",
				Version:  "v1alpha1",
				Resource: "scylladbdatacenters",
			}, admissionv1.Create, []byte(`{"apiVersion":"scylla.scylladb.com/v1alpha1","kind":"ScyllaDBDatacenter","metadata":{"name":"basic","namespace":"test"},"spec":{"clusterName":"basic","scyllaDB":{"image":"docker.io/scylladb/scylla:2026.1.0"},"racks":[{"name":"rack1"}]}}`)),
			expectedPatches: nil,
			expectedError:   nil,
		},
		{
			name: "a ScyllaDBDatacenter with an image supporting parallel bootstrap is stamped with a Parallel bootstrapPolicy",
			req: newMutateAdmissionRequest(metav1.GroupVersionResource{
				Group:    "scylla.scylladb.com",
				Version:  "v1alpha1",
				Resource: "scylladbdatacenters",
			}, admissionv1.Create, []byte(`{"apiVersion":"scylla.scylladb.com/v1alpha1","kind":"ScyllaDBDatacenter","metadata":{"name":"basic","namespace":"test"},"spec":{"clusterName":"basic","scyllaDB":{"image":"docker.io/scylladb/scylla:2026.2.0"},"racks":[{"name":"rack1"}]}}`)),
			expectedPatches: []jsonpatch.Operation{
				{
					Operation: "add",
					Path:      "/spec/bootstrapPolicy",
					Value:     string(scyllav1alpha1.BootstrapPolicyParallel),
				},
			},
			expectedError: nil,
		},
		{
			name: "a ScyllaDBDatacenter with an explicit bootstrapPolicy passes through unchanged",
			req: newMutateAdmissionRequest(metav1.GroupVersionResource{
				Group:    "scylla.scylladb.com",
				Version:  "v1alpha1",
				Resource: "scylladbdatacenters",
			}, admissionv1.Create, []byte(`{"apiVersion":"scylla.scylladb.com/v1alpha1","kind":"ScyllaDBDatacenter","metadata":{"name":"basic","namespace":"test"},"spec":{"clusterName":"basic","scyllaDB":{"image":"docker.io/scylladb/scylla:2026.2.0"},"bootstrapPolicy":"Sequential","racks":[{"name":"rack1"}]}}`)),
			expectedPatches: nil,
			expectedError:   nil,
		},
		{
			// ScyllaDBClusters are deliberately not registered, as parallel bootstrap is not supported in
			// automated multi-datacenter setups.
			name: "a ScyllaDBCluster is rejected as an unsupported GVR",
			req: newMutateAdmissionRequest(metav1.GroupVersionResource{
				Group:    "scylla.scylladb.com",
				Version:  "v1alpha1",
				Resource: "scylladbclusters",
			}, admissionv1.Create, []byte(`{"apiVersion":"scylla.scylladb.com/v1alpha1","kind":"ScyllaDBCluster","metadata":{"name":"basic","namespace":"test"}}`)),
			expectedPatches: nil,
			expectedError:   fmt.Errorf(`unsupported GVR "scylla.scylladb.com/v1alpha1, Resource=scylladbclusters"`),
		},
		{
			name: "a resource outside of the ScyllaDB API group is rejected as an unsupported GVR",
			req: newMutateAdmissionRequest(metav1.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "pods",
			}, admissionv1.Create, []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod","namespace":"test"},"spec":{"containers":null}}`)),
			expectedPatches: nil,
			expectedError:   fmt.Errorf(`unsupported GVR "/v1, Resource=pods"`),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			patches, err := mutate(tc.req, DefaultDefaulters)
			if !reflect.DeepEqual(tc.expectedError, err) {
				t.Fatalf("expected and actual errors differ: %s", cmp.Diff(tc.expectedError, err, cmpopts.EquateErrors()))
			}

			if !reflect.DeepEqual(tc.expectedPatches, patches) {
				t.Errorf("expected and actual patches differ: %s", cmp.Diff(tc.expectedPatches, patches))
			}
		})
	}
}

// unmarshalableObject is a runtime.Object that always fails to serialize.
type unmarshalableObject struct {
	metav1.TypeMeta
}

func (o *unmarshalableObject) DeepCopyObject() runtime.Object {
	return &unmarshalableObject{TypeMeta: o.TypeMeta}
}

func (o *unmarshalableObject) MarshalJSON() ([]byte, error) {
	return nil, fmt.Errorf("always fails")
}

func Test_createDefaultingPatch(t *testing.T) {
	t.Parallel()

	// A payload that omits fields the Go types serialize unconditionally, i.e. one whose round trip through
	// the typed object is lossy. Any diff base other than re-encoding both sides from the decoded object
	// surfaces those fields as operations, so the expectations below pin the diff base down.
	newDecodedScyllaCluster := func(t *testing.T, specFields string) *scyllav1.ScyllaCluster {
		t.Helper()

		raw := []byte(fmt.Sprintf(`{"apiVersion":"scylla.scylladb.com/v1","kind":"ScyllaCluster",`+
			`"metadata":{"name":"basic","namespace":"test"},`+
			`"spec":{"version":"6.2.0","agentVersion":"latest",%s`+
			`"datacenter":{"name":"dc1","racks":[{"name":"rack1","members":3,"storage":{"capacity":"1Gi"}}]}}}`, specFields))

		obj, _, err := scheme.Codecs.UniversalDeserializer().Decode(raw, nil, nil)
		if err != nil {
			t.Fatal(err)
		}

		sc, ok := obj.(*scyllav1.ScyllaCluster)
		if !ok {
			t.Fatalf("expected *scyllav1.ScyllaCluster, got %T", obj)
		}

		return sc
	}

	tt := []struct {
		name                   string
		objsFunc               func(t *testing.T) (runtime.Object, runtime.Object)
		expectedPatches        []jsonpatch.Operation
		expectedErrorSubstring string
	}{
		{
			// True by construction, given both sides are re-encoded from the same decoded object. Kept as a
			// regression guard against a change of the diff base, not as a proof that patching works.
			name: "an unchanged object yields no operations",
			objsFunc: func(t *testing.T) (runtime.Object, runtime.Object) {
				sc := newDecodedScyllaCluster(t, "")
				return sc, sc.DeepCopy()
			},
			expectedPatches: []jsonpatch.Operation{},
		},
		{
			// The expectation that matters: exactly the field the defaulter set, and nothing else. Diffing
			// the raw payload instead would additionally add .spec.network, .spec.datacenter.racks[*].resources,
			// .scyllaConfig, .scyllaAgentConfig and .status here.
			name: "a set field yields only that operation",
			objsFunc: func(t *testing.T) (runtime.Object, runtime.Object) {
				sc := newDecodedScyllaCluster(t, "")
				defaulted := sc.DeepCopy()
				defaulted.Spec.ForceRedeploymentReason = "defaulted"

				return sc, defaulted
			},
			expectedPatches: []jsonpatch.Operation{
				{
					Operation: "add",
					Path:      "/spec/forceRedeploymentReason",
					Value:     "defaulted",
				},
			},
		},
		{
			name: "a changed field yields a replace operation",
			objsFunc: func(t *testing.T) (runtime.Object, runtime.Object) {
				sc := newDecodedScyllaCluster(t, "")
				defaulted := sc.DeepCopy()
				defaulted.Spec.Version = "6.3.0"

				return sc, defaulted
			},
			expectedPatches: []jsonpatch.Operation{
				{
					Operation: "replace",
					Path:      "/spec/version",
					Value:     "6.3.0",
				},
			},
		},
		{
			name: "a cleared field yields a remove operation",
			objsFunc: func(t *testing.T) (runtime.Object, runtime.Object) {
				sc := newDecodedScyllaCluster(t, `"forceRedeploymentReason":"set-by-user",`)
				defaulted := sc.DeepCopy()
				defaulted.Spec.ForceRedeploymentReason = ""

				return sc, defaulted
			},
			expectedPatches: []jsonpatch.Operation{
				{
					Operation: "remove",
					Path:      "/spec/forceRedeploymentReason",
					Value:     nil,
				},
			},
		},
		{
			name: "an appended slice element yields an add operation",
			objsFunc: func(t *testing.T) (runtime.Object, runtime.Object) {
				sc := newDecodedScyllaCluster(t, `"sysctls":["fs.aio-max-nr=1048576"],`)
				defaulted := sc.DeepCopy()
				defaulted.Spec.Sysctls = append(defaulted.Spec.Sysctls, "fs.file-max=1048576")

				return sc, defaulted
			},
			expectedPatches: []jsonpatch.Operation{
				{
					Operation: "add",
					Path:      "/spec/sysctls/1",
					Value:     "fs.file-max=1048576",
				},
			},
		},
		{
			name: "an unserializable object is rejected",
			objsFunc: func(t *testing.T) (runtime.Object, runtime.Object) {
				return &unmarshalableObject{}, newDecodedScyllaCluster(t, "")
			},
			expectedPatches:        nil,
			expectedErrorSubstring: "can't encode object:",
		},
		{
			name: "an unserializable defaulted object is rejected",
			objsFunc: func(t *testing.T) (runtime.Object, runtime.Object) {
				return newDecodedScyllaCluster(t, ""), &unmarshalableObject{}
			},
			expectedPatches:        nil,
			expectedErrorSubstring: "can't encode defaulted object:",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			obj, defaultedObj := tc.objsFunc(t)

			patches, err := createDefaultingPatch(obj, defaultedObj)
			verifyErrorSubstring(t, err, tc.expectedErrorSubstring)

			if !reflect.DeepEqual(tc.expectedPatches, patches) {
				t.Errorf("expected and actual patches differ: %s", cmp.Diff(tc.expectedPatches, patches))
			}
		})
	}
}

// verifyErrorSubstring asserts on wrapped errors, whose messages embed those of the wrapped ones and so
// can't be compared for equality.
func verifyErrorSubstring(t *testing.T, err error, expectedSubstring string) {
	t.Helper()

	if len(expectedSubstring) == 0 {
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		return
	}

	if err == nil {
		t.Fatalf("expected an error containing %q, got none", expectedSubstring)
	}

	if !strings.Contains(err.Error(), expectedSubstring) {
		t.Fatalf("expected an error containing %q, got %v", expectedSubstring, err)
	}
}
