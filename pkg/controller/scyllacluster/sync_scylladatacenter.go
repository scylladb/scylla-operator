// Copyright (c) 2022 ScyllaDB.

package scyllacluster

import (
	"context"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllav2alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v2alpha1"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func (scc *Controller) syncScyllaDatacenters(
	ctx context.Context,
	cluster *scyllav2alpha1.ScyllaCluster,
	remoteScyllaDatacenters map[string]map[string]*scyllav1alpha1.ScyllaDatacenter,
	status *scyllav2alpha1.ScyllaClusterStatus,
) error {
	requiredScyllaDatacenters := MakeScyllaDatacenters(cluster)

	// Delete any excessive ScyllaClusters.
	// Delete has to be the fist action to avoid getting stuck on quota.
	var deletionErrors []error
	for dcName, scyllaDatacenters := range remoteScyllaDatacenters {
		for _, sd := range scyllaDatacenters {
			if sd.DeletionTimestamp != nil {
				continue
			}

			req, ok := requiredScyllaDatacenters[dcName]
			if ok && sd.Name == req.Name {
				continue
			}

			// TODO: This should first scale all racks to 0, only then remove
			propagationPolicy := metav1.DeletePropagationBackground
			dcClient, err := scc.scyllaMultiregionClient.Datacenter(fmt.Sprintf("%s/%s/%s", cluster.Namespace, cluster.Name, dcName))
			if err != nil {
				return fmt.Errorf("can't get client to %q datacenter: %w", dcName, err)
			}
			err = dcClient.ScyllaV1alpha1().ScyllaDatacenters(sd.Namespace).Delete(ctx, sd.Name, metav1.DeleteOptions{
				Preconditions: &metav1.Preconditions{
					UID: &sd.UID,
				},
				PropagationPolicy: &propagationPolicy,
			})
			deletionErrors = append(deletionErrors, err)
		}
	}
	var err error
	err = utilerrors.NewAggregate(deletionErrors)
	if err != nil {
		return fmt.Errorf("can't delete scylla datacenter(s): %w", err)
	}

	for dcName, sd := range requiredScyllaDatacenters {
		dcClient, err := scc.scyllaMultiregionClient.Datacenter(dcName)
		if err != nil {
			return fmt.Errorf("can't get datacenter client: %w", err)
		}
		sd, _, err = resourceapply.ApplyRemoteScyllaDatacenter(ctx, dcClient.ScyllaV1alpha1(), scc.scyllaDatacenterMultiregionLister.Datacenter(dcName), scc.eventRecorder, sd)
		if err != nil {
			return fmt.Errorf("can't apply scylla datacenter: %w", err)
		}

		for i, dc := range cluster.Spec.Datacenters {
			if dc.Name != dcName {
				continue
			}
			status.Datacenters[i] = *scc.calculateDatacenterStatus(cluster, dc, sd)
			break
		}
	}

	return nil
}
