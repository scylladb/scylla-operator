// Copyright (C) 2021 ScyllaDB

package resourceapply

import (
	"fmt"
	"strings"

	"github.com/scylladb/scylla-operator/pkg/util/resource"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
)

const unknown = "unknown"

func objectReference(obj metav1.Object) string {
	namespace := obj.GetNamespace()
	if len(namespace) == 0 {
		return obj.GetName()
	}

	return fmt.Sprintf("%s/%s", obj.GetNamespace(), obj.GetName())
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
			"Failed to %s %s: %v",
			strings.ToLower(verb), objectReference(objMeta), operationErr,
		)
		return
	}
	recorder.Eventf(
		obj,
		corev1.EventTypeNormal,
		fmt.Sprintf("%s%sd", gvk.Kind, strings.Title(verb)),
		"%s %sd",
		objectReference(objMeta), strings.Title(verb),
	)
}

func ReportCreateEvent(recorder record.EventRecorder, obj runtime.Object, operationErr error) {
	reportEvent(recorder, obj, operationErr, "create")
}

func ReportUpdateEvent(recorder record.EventRecorder, obj runtime.Object, operationErr error) {
	reportEvent(recorder, obj, operationErr, "update")
}

func ReportDeleteEvent(recorder record.EventRecorder, obj runtime.Object, operationErr error) {
	reportEvent(recorder, obj, operationErr, "delete")
}
