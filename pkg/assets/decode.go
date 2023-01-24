// Copyright (C) 2023 ScyllaDB

package assets

import (
	"fmt"
	"text/template"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
)

func Decode[T any](data []byte, decoder runtime.Decoder) (T, error) {
	obj, _, err := decoder.Decode(data, nil, nil)
	if err != nil {
		return *new(T), fmt.Errorf("can't decode object: %w", err)
	}

	typedObj, ok := obj.(T)
	if !ok {
		return *new(T), fmt.Errorf("can't cast decoded object of type %t: %w", obj, err)
	}

	return typedObj, nil
}

func RenderAndDecode[T runtime.Object](tmpl *template.Template, inputs any, decoder runtime.Decoder) (T, string, error) {
	renderedBytes, err := RenderTemplate(tmpl, inputs)
	if err != nil {
		return *new(T), "", fmt.Errorf("can't render template: %w", err)
	}

	obj, err := Decode[T](renderedBytes, decoder)
	if err != nil {
		// Rendered templates can contain secret data that we can't log in the regular flow.
		var redactedString string
		switch runtime.Object(*new(T)).(type) {
		case *corev1.Secret:
			redactedString = "<redacted secret bytes>"
		default:
			redactedString = string(renderedBytes)
		}
		klog.Errorf("Can't decode rendered template %q: %v. Template:\n%s", tmpl.Name(), err, redactedString)
		return *new(T), string(renderedBytes), fmt.Errorf("can't decode rendered template %q: %w", tmpl.Name(), err)
	}

	return obj, string(renderedBytes), nil
}
