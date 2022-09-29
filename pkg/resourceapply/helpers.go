package resourceapply

import (
	"fmt"
	"strings"

	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/pkg/resource"
	hashutil "github.com/scylladb/scylla-operator/pkg/util/hash"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
)

func verifyDesiredObject(obj metav1.Object) error {
	if obj.GetUID() != "" {
		return fmt.Errorf("desired objects are not allowed to have UID set")
	}

	if !pointer.Time(obj.GetCreationTimestamp()).IsZero() {
		return fmt.Errorf("desired objects are not allowed to have creationTimestamp set")
	}

	if obj.GetGeneration() != 0 {
		return fmt.Errorf("desired objects are not allowed to have generation set")
	}

	if len(obj.GetManagedFields()) != 0 {
		return fmt.Errorf("desired objects are not allowed to contain managedFields")
	}

	if len(obj.GetSelfLink()) != 0 {
		return fmt.Errorf("desired objects are not allowed to have selfLink set")
	}

	return nil
}

func SetHashAnnotation(obj metav1.Object) error {
	err := verifyDesiredObject(obj)
	if err != nil {
		return fmt.Errorf("invalid desider object %q: %w", naming.ObjRef(obj), err)
	}

	// Do not hash ResourceVersion.
	rv := obj.GetResourceVersion()
	obj.SetResourceVersion("")
	defer obj.SetResourceVersion(rv)

	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}

	// Clear annotation to have consistent hashing for the same objects.
	delete(annotations, naming.ManagedHash)

	hash, err := hashutil.HashObjects(obj)
	if err != nil {
		return err
	}

	annotations[naming.ManagedHash] = hash
	obj.SetAnnotations(annotations)

	return nil
}

func reportEvent(recorder record.EventRecorder, obj runtime.Object, operationErr error, verb string) {
	objMeta, err := meta.Accessor(obj)
	if err != nil {
		klog.ErrorS(err, "can't get object metadata")
		return
	}
	gvk, err := resource.GetObjectGVK(obj)
	if err != nil {
		klog.ErrorS(err, "can't determine object GVK", "Object", klog.KObj(objMeta))
		return
	}

	if operationErr != nil {
		recorder.Eventf(
			obj,
			corev1.EventTypeWarning,
			fmt.Sprintf("%s%sFailed", strings.Title(verb), gvk.Kind),
			"Failed to %s %s %s: %v",
			strings.ToLower(verb), gvk.Kind, naming.ObjRef(objMeta), operationErr,
		)
		return
	}
	recorder.Eventf(
		obj,
		corev1.EventTypeNormal,
		fmt.Sprintf("%s%sd", gvk.Kind, strings.Title(verb)),
		"%s %s %sd",
		gvk.Kind, naming.ObjRef(objMeta), verb,
	)
}

func ReportCreateEvent(recorder record.EventRecorder, obj runtime.Object, operationErr error) {
	if apierrors.HasStatusCause(operationErr, corev1.NamespaceTerminatingCause) {
		// If the namespace is being terminated, we don't have to do
		// anything because any creation will fail.
		return
	}

	reportEvent(recorder, obj, operationErr, "create")
}

func ReportUpdateEvent(recorder record.EventRecorder, obj runtime.Object, operationErr error) {
	reportEvent(recorder, obj, operationErr, "update")
}

func ReportDeleteEvent(recorder record.EventRecorder, obj runtime.Object, operationErr error) {
	reportEvent(recorder, obj, operationErr, "delete")
}
