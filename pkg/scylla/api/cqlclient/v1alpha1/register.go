package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	GroupName     = "cqlclient.scylla.scylladb.com"
	GroupVersion  = schema.GroupVersion{Group: GroupName, Version: "v1alpha1"}
	schemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	// Install is a function which adds this version to a scheme
	Install = schemeBuilder.AddToScheme
)

// Adds the list of known types to api.Scheme.
func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(GroupVersion,
		&CQLConnectionConfig{},
	)
	metav1.AddToGroupVersion(scheme, GroupVersion)
	return nil
}
