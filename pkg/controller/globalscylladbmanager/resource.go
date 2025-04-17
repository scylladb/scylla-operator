// Copyright (C) 2025 ScyllaDB

package globalscylladbmanager

import (
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func makeScyllaDBManagerClusterRegistrationForScyllaDBDatacenter(sdc *scyllav1alpha1.ScyllaDBDatacenter) (*scyllav1alpha1.ScyllaDBManagerClusterRegistration, error) {
	name, err := naming.ScyllaDBManagerClusterRegistrationNameForScyllaDBDatacenter(sdc)
	if err != nil {
		return nil, fmt.Errorf("can't get ScyllaDBManagerClusterRegistration name: %w", err)
	}

	smcr := &scyllav1alpha1.ScyllaDBManagerClusterRegistration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: sdc.Namespace,
			Labels: map[string]string{
				naming.GlobalScyllaDBManagerLabel: naming.LabelValueTrue,
			},
			Annotations: map[string]string{},
		},
		Spec: scyllav1alpha1.ScyllaDBManagerClusterRegistrationSpec{
			ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
				Kind: naming.ScyllaDBDatacenterKind,
				Name: sdc.Name,
			},
		},
	}

	nameOverrideAnnotationValue, hasNameOverrideAnnotation := sdc.Annotations[naming.ScyllaDBManagerClusterRegistrationNameOverrideAnnotation]
	if hasNameOverrideAnnotation {
		smcr.Annotations[naming.ScyllaDBManagerClusterRegistrationNameOverrideAnnotation] = nameOverrideAnnotationValue
	}

	return smcr, nil
}
