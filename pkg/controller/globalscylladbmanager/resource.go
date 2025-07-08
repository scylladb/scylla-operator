// Copyright (C) 2025 ScyllaDB

package globalscylladbmanager

import (
	"fmt"
	"maps"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func makeScyllaDBManagerClusterRegistrationForScyllaDBDatacenter(sdc *scyllav1alpha1.ScyllaDBDatacenter) (*scyllav1alpha1.ScyllaDBManagerClusterRegistration, error) {
	name, err := naming.ScyllaDBManagerClusterRegistrationNameForScyllaDBDatacenter(sdc)
	if err != nil {
		return nil, fmt.Errorf("can't get ScyllaDBManagerClusterRegistration name: %w", err)
	}

	labels := map[string]string{}
	maps.Copy(labels, naming.GlobalScyllaDBManagerClusterRegistrationSelectorLabels())

	annotations := map[string]string{}
	nameOverrideAnnotationValue, hasNameOverrideAnnotation := sdc.Annotations[naming.ScyllaDBManagerClusterRegistrationNameOverrideAnnotation]
	if hasNameOverrideAnnotation {
		annotations[naming.ScyllaDBManagerClusterRegistrationNameOverrideAnnotation] = nameOverrideAnnotationValue
	}

	return &scyllav1alpha1.ScyllaDBManagerClusterRegistration{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   sdc.Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: scyllav1alpha1.ScyllaDBManagerClusterRegistrationSpec{
			ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
				Kind: scyllav1alpha1.ScyllaDBDatacenterGVK.Kind,
				Name: sdc.Name,
			},
		},
	}, nil
}

func makeScyllaDBManagerClusterRegistrationForScyllaDBCluster(sc *scyllav1alpha1.ScyllaDBCluster) (*scyllav1alpha1.ScyllaDBManagerClusterRegistration, error) {
	name, err := naming.ScyllaDBManagerClusterRegistrationNameForScyllaDBCluster(sc)
	if err != nil {
		return nil, fmt.Errorf("can't get ScyllaDBManagerClusterRegistration name: %w", err)
	}

	labels := map[string]string{}
	maps.Copy(labels, naming.GlobalScyllaDBManagerClusterRegistrationSelectorLabels())

	annotations := map[string]string{}
	nameOverrideAnnotationValue, hasNameOverrideAnnotation := sc.Annotations[naming.ScyllaDBManagerClusterRegistrationNameOverrideAnnotation]
	if hasNameOverrideAnnotation {
		annotations[naming.ScyllaDBManagerClusterRegistrationNameOverrideAnnotation] = nameOverrideAnnotationValue
	}

	return &scyllav1alpha1.ScyllaDBManagerClusterRegistration{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   sc.Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: scyllav1alpha1.ScyllaDBManagerClusterRegistrationSpec{
			ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
				Kind: scyllav1alpha1.ScyllaDBClusterGVK.Kind,
				Name: sc.Name,
			},
		},
	}, nil
}
