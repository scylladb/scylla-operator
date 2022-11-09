// Copyright (c) 2022 ScyllaDB.

package informers

import (
	"reflect"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

func ResourceConvertingEventHandler(scheme *runtime.Scheme, obj runtime.Object, handler cache.ResourceEventHandler) cache.ResourceEventHandler {
	typeOf := reflect.TypeOf(obj)
	convert := func(unstructured any) (any, error) {
		o := reflect.New(typeOf.Elem()).Interface()
		if err := scheme.Convert(unstructured, o, nil); err != nil {
			return nil, err
		}

		return o, nil
	}

	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(unstructured interface{}) {
			convertedObj, err := convert(unstructured)
			if err != nil {
				klog.Warningf("can't convert unstructured object to %T, skipping event: %s", obj, err)
				return
			}

			handler.OnAdd(convertedObj)
		},
		UpdateFunc: func(unstructuredOld, unstructuredNew interface{}) {
			oldObj, err := convert(unstructuredOld)
			if err != nil {
				klog.Warningf("can't convert unstructured object to %T, skipping event: %s", obj, err)
				return
			}
			newObj, err := convert(unstructuredNew)
			if err != nil {
				klog.Warningf("can't convert unstructured object to %T, skipping event: %s", obj, err)
				return
			}

			handler.OnUpdate(oldObj, newObj)
		},
		DeleteFunc: func(unstructured interface{}) {
			convertedObj, err := convert(unstructured)
			if err != nil {
				klog.Warningf("can't convert unstructured object to %T, skipping event: %s", obj, err)
				return
			}

			handler.OnDelete(convertedObj)
		},
	}
}
