package operator

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"

	"github.com/scylladb/scylla-operator/pkg/api/scylla/defaulting"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/kubeinterfaces"
	"github.com/scylladb/scylla-operator/pkg/scheme"
	jsonpatch "gomodules.xyz/jsonpatch/v2"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// DefaultDefaulters registers the create-time defaulters served on the /mutate endpoint.
// ScyllaDBClusters are deliberately not registered. Registering ScyllaDBDatacenters is safe because
// managed ScyllaDBDatacenters always arrive at admission with the defaulted fields explicitly set by
// their parent controllers, so the defaulters only ever take effect for directly created objects.
// The parent controllers' authoritative apply additionally overwrites any stray defaulted value on
// managed objects.
var DefaultDefaulters = map[schema.GroupVersionResource]Defaulter{
	scyllav1.GroupVersion.WithResource("scyllaclusters"): &GenericDefaulter[*scyllav1.ScyllaCluster]{
		DefaultOnCreateFunc: defaulting.SetDefaultsScyllaCluster,
	},
	scyllav1alpha1.GroupVersion.WithResource("scylladbdatacenters"): &GenericDefaulter[*scyllav1alpha1.ScyllaDBDatacenter]{
		DefaultOnCreateFunc: defaulting.SetDefaultsScyllaDBDatacenter,
	},
}

type Defaulter interface {
	// DefaultOnCreate sets the create time defaults on obj. It returns an error only when obj isn't of the
	// type the defaulter defaults, i.e. when the registered defaulter doesn't match the request's resource.
	DefaultOnCreate(obj runtime.Object) error
}

type DefaultableObject interface {
	kubeinterfaces.ObjectInterface
	schema.ObjectKind
}

type GenericDefaulter[T DefaultableObject] struct {
	DefaultOnCreateFunc func(obj T)
}

func (d *GenericDefaulter[T]) DefaultOnCreate(obj runtime.Object) error {
	typedObj, ok := obj.(T)
	if !ok {
		var expectedObj T
		return fmt.Errorf("expected %T, got %T", expectedObj, obj)
	}

	d.DefaultOnCreateFunc(typedObj)

	return nil
}

// NewMutatingWebhookHandler returns an admission handler that dispatches admission requests to the given defaulters.
// Only CREATE operations are supported: requests for any other operation are rejected with an error, so any webhook
// configuration routing to it must use CREATE-only rules, or, combined with failurePolicy: Fail, the intercepted
// operations would always be denied.
func NewMutatingWebhookHandler(defaulters map[schema.GroupVersionResource]Defaulter) admission.Handler {
	return admission.HandlerFunc(func(ctx context.Context, req admission.Request) admission.Response {
		patches, err := mutate(&req.AdmissionRequest, defaulters)
		if err != nil {
			klog.V(2).InfoS("Review failed", "Error", err)
			return admission.Errored(http.StatusInternalServerError, err)
		}

		return admission.Patched("", patches...)
	})
}

func mutate(req *admissionv1.AdmissionRequest, defaulters map[schema.GroupVersionResource]Defaulter) ([]jsonpatch.Operation, error) {
	gvr := schema.GroupVersionResource{
		Group:    req.Resource.Group,
		Version:  req.Resource.Version,
		Resource: req.Resource.Resource,
	}

	defaulter, ok := defaulters[gvr]
	if !ok {
		return nil, fmt.Errorf("unsupported GVR %q", gvr)
	}

	if req.Operation != admissionv1.Create {
		return nil, fmt.Errorf("unsupported operation %q", req.Operation)
	}

	deserializer := scheme.Codecs.UniversalDeserializer()

	obj, _, err := deserializer.Decode(req.Object.Raw, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("can't decode object %q: %w", gvr, err)
	}

	defaultedObj := obj.DeepCopyObject()
	err = defaulter.DefaultOnCreate(defaultedObj)
	if err != nil {
		return nil, fmt.Errorf("can't default object %q: %w", gvr, err)
	}

	// If the defaulter didn't change the object, there is no patch to compute.
	if reflect.DeepEqual(obj, defaultedObj) {
		return nil, nil
	}

	patches, err := createDefaultingPatch(obj, defaultedObj)
	if err != nil {
		return nil, fmt.Errorf("can't create patch for object %q: %w", gvr, err)
	}

	return patches, nil
}

// createDefaultingPatch returns the JSONPatch operations turning obj into defaultedObj.
//
// The patch is computed between the decoded object and its defaulted copy, not against the raw request
// payload: re-encoding the decoded object on both sides makes the round-trip serialization artifacts
// (fields without omitempty that the raw payload omits) identical, so the patch only ever contains
// the fields the defaulter set.
// This deliberately deviates from controller-runtime's defaulting webhook, which diffs the raw payload
// against the re-encoded defaulted object, and consequently stamps the zero values of every field the
// raw payload omits whenever a defaulter fires, and needs dedicated machinery to drop removals of
// explicitly set zero values of omitempty fields:
// https://github.com/kubernetes-sigs/controller-runtime/blob/v0.24.1/pkg/webhook/admission/defaulter_custom.go#L164
// https://github.com/kubernetes-sigs/controller-runtime/blob/v0.24.1/pkg/webhook/admission/defaulter_custom.go#L171
//
// Note that the API server applies the patch to the raw request payload, which, unlike the decoded object
// the patch is computed against, may be missing the parent paths of the patched fields, failing the
// creation. Defaulters must only set fields whose parents are guaranteed to be present in every payload
// that passes validation, or set the missing parent objects themselves so the patch carries them.
func createDefaultingPatch(obj, defaultedObj runtime.Object) ([]jsonpatch.Operation, error) {
	objBytes, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("can't encode object: %w", err)
	}

	defaultedObjBytes, err := json.Marshal(defaultedObj)
	if err != nil {
		return nil, fmt.Errorf("can't encode defaulted object: %w", err)
	}

	patches, err := jsonpatch.CreatePatch(objBytes, defaultedObjBytes)
	if err != nil {
		return nil, fmt.Errorf("can't create patch: %w", err)
	}

	return patches, nil
}
