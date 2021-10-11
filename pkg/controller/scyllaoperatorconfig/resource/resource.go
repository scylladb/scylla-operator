// Copyright (C) 2021 ScyllaDB

package resource

import (
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func DefaultScyllaOperatorConfig() *scyllav1alpha1.ScyllaOperatorConfig {
	return &scyllav1alpha1.ScyllaOperatorConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: naming.ScyllaOperatorName,
		},
		Spec: scyllav1alpha1.ScyllaOperatorConfigSpec{
			ScyllaUtilsImage: naming.DefaultScyllaUtilsImage,
		},
	}
}
