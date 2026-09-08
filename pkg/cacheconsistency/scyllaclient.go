package cacheconsistency

import (
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllav1alpha1client "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned/typed/scylla/v1alpha1"
)

// Kinds the recording Scylla client records writes for.
var (
	scyllaDBDatacenterGVK                  = scyllav1alpha1.ScyllaDBDatacenterGVK
	scyllaDBDatacenterNodesStatusReportGVK = scyllav1alpha1.GroupVersion.WithKind("ScyllaDBDatacenterNodesStatusReport")
)

// NewRecordingScyllaV1alpha1Client returns a ScyllaV1alpha1Interface that records every write it makes to the kinds registered
// in store, so that WaitReady on the store covers them. See NewRecordingKubeClient for what passes through unrecorded.
//
// The wrapped kinds are ScyllaDBDatacenters and ScyllaDBDatacenterNodesStatusReports.
func NewRecordingScyllaV1alpha1Client(client scyllav1alpha1client.ScyllaV1alpha1Interface, store *ConsistencyStore) scyllav1alpha1client.ScyllaV1alpha1Interface {
	return &scyllaV1alpha1Client{
		ScyllaV1alpha1Interface: client,
		store:                   store,
	}
}

type scyllaV1alpha1Client struct {
	scyllav1alpha1client.ScyllaV1alpha1Interface
	store *ConsistencyStore
}

func (c *scyllaV1alpha1Client) ScyllaDBDatacenters(namespace string) scyllav1alpha1client.ScyllaDBDatacenterInterface {
	return NewRecordingClientWithStatus[*scyllav1alpha1.ScyllaDBDatacenter, *scyllav1alpha1.ScyllaDBDatacenterList](c.ScyllaV1alpha1Interface.ScyllaDBDatacenters(namespace), scyllaDBDatacenterGVK, namespace, c.store)
}

func (c *scyllaV1alpha1Client) ScyllaDBDatacenterNodesStatusReports(namespace string) scyllav1alpha1client.ScyllaDBDatacenterNodesStatusReportInterface {
	return NewRecordingClient[*scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport, *scyllav1alpha1.ScyllaDBDatacenterNodesStatusReportList](c.ScyllaV1alpha1Interface.ScyllaDBDatacenterNodesStatusReports(namespace), scyllaDBDatacenterNodesStatusReportGVK, namespace, c.store)
}
