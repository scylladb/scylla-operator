// Copyright (C) 2022 ScyllaDB

package helpers

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// UncachedListFunc wraps a List function and makes sure initial lists avoid watch cache on the apiserver.
// This is important for caller that need top reason about "happened after" or similar cases.
func UncachedListFunc(f func(options metav1.ListOptions) (runtime.Object, error)) func(options metav1.ListOptions) (runtime.Object, error) {
	return func(options metav1.ListOptions) (runtime.Object, error) {
		// Transform every RV="0" into RV="" that avoids watch cache.
		// By default, informers use RV="0" that goes through watch cache.
		// We need to avoid watch cache not to list older objects.
		if options.ResourceVersion == "0" {
			options.ResourceVersion = ""
		}

		return f(options)
	}
}
