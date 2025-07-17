// Copyright (C) 2025 ScyllaDB

package globalscylladbmanager

import (
	"fmt"
	"maps"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func makeScyllaDBManagerClusterRegistrations(
	scyllaDBDatacenters []*scyllav1alpha1.ScyllaDBDatacenter,
	scyllaDBClusters []*scyllav1alpha1.ScyllaDBCluster,
) (map[string][]*scyllav1alpha1.ScyllaDBManagerClusterRegistration, error) {
	var errs []error

	scyllaDBManagerClusterRegistrationsForScyllaDBDatacenters, err := makeScyllaDBManagerClusterRegistrationsForScyllaDBDatacenters(scyllaDBDatacenters)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't make required ScyllaDBManagerClusterRegistration objects for ScyllaDBDatacenter(s): %w", err))
	}

	scyllaDBManagerClusterRegistrationsForScyllaDBClusters, err := makeScyllaDBManagerClusterRegistrationsForScyllaDBClusters(scyllaDBClusters)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't make required ScyllaDBManagerClusterRegistration objects for ScyllaDBClusters(s): %w", err))
	}

	err = apimachineryutilerrors.NewAggregate(errs)
	if err != nil {
		return nil, err
	}

	requiredScyllaDBManagerClusterRegistrations := map[string][]*scyllav1alpha1.ScyllaDBManagerClusterRegistration{}
	maps.Copy(requiredScyllaDBManagerClusterRegistrations, scyllaDBManagerClusterRegistrationsForScyllaDBDatacenters)
	maps.Copy(requiredScyllaDBManagerClusterRegistrations, scyllaDBManagerClusterRegistrationsForScyllaDBClusters)

	return requiredScyllaDBManagerClusterRegistrations, nil
}

func makeScyllaDBManagerClusterRegistrationsForScyllaDBDatacenters(sdcs []*scyllav1alpha1.ScyllaDBDatacenter) (map[string][]*scyllav1alpha1.ScyllaDBManagerClusterRegistration, error) {
	return makeScyllaDBManagerClusterRegistrationsForObjects(sdcs, makeScyllaDBManagerClusterRegistrationForScyllaDBDatacenter)
}

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

func makeScyllaDBManagerClusterRegistrationsForScyllaDBClusters(scs []*scyllav1alpha1.ScyllaDBCluster) (map[string][]*scyllav1alpha1.ScyllaDBManagerClusterRegistration, error) {
	return makeScyllaDBManagerClusterRegistrationsForObjects(scs, makeScyllaDBManagerClusterRegistrationForScyllaDBCluster)
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

func makeScyllaDBManagerClusterRegistrationsForObjects[S ~[]E, E metav1.Object](
	objs S,
	makeScyllaDBManagerClusterRegistrationForObject func(E) (*scyllav1alpha1.ScyllaDBManagerClusterRegistration, error),
) (map[string][]*scyllav1alpha1.ScyllaDBManagerClusterRegistration, error) {
	var errs []error
	requiredScyllaDBManagerClusterRegistrations := map[string][]*scyllav1alpha1.ScyllaDBManagerClusterRegistration{}
	for _, obj := range objs {
		required, err := makeScyllaDBManagerClusterRegistrationForObject(obj)
		if err != nil {
			errs = append(errs, fmt.Errorf("can't make ScyllaDBManagerClusterRegistration for object %q: %w", naming.ObjRef(obj), err))
			continue
		}

		requiredScyllaDBManagerClusterRegistrations[obj.GetNamespace()] = append(requiredScyllaDBManagerClusterRegistrations[obj.GetNamespace()], required)
	}

	return requiredScyllaDBManagerClusterRegistrations, apimachineryutilerrors.NewAggregate(errs)
}
