package scylladbcluster

import (
	"context"
	"errors"
	"fmt"
	"sort"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	utilsv1alpha1 "github.com/scylladb/scylla-operator/test/e2e/utils/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func WaitForFullQuorum(ctx context.Context, rkcClusterMap map[string]framework.ClusterInterface, sc *scyllav1alpha1.ScyllaDBCluster) error {
	allBroadcastAddresses := map[string][]string{}
	var sortedAllBroadcastAddresses []string

	var errs []error
	for _, dc := range sc.Spec.Datacenters {
		clusterClient, ok := rkcClusterMap[dc.RemoteKubernetesClusterName]
		if !ok {
			errs = append(errs, fmt.Errorf("cluster client is missing for DC %q of ScyllaDBCluster %q", dc.Name, naming.ObjRef(sc)))
			continue
		}

		dcStatus, _, ok := oslices.Find(sc.Status.Datacenters, func(dcStatus scyllav1alpha1.ScyllaDBClusterDatacenterStatus) bool {
			return dcStatus.Name == dc.Name
		})
		if !ok {
			return fmt.Errorf("can't find %q datacenter status", dc.Name)
		}
		if dcStatus.RemoteNamespaceName == nil {
			return fmt.Errorf("unexpected nil remote namespace name in %q datacenter status", dc.Name)
		}

		sdc, err := clusterClient.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBDatacenters(*dcStatus.RemoteNamespaceName).Get(ctx, naming.ScyllaDBDatacenterName(sc, &dc), metav1.GetOptions{})
		if err != nil {
			errs = append(errs, fmt.Errorf("can't get ScyllaDBDatacenter %q: %w", dc.Name, err))
			continue
		}

		hosts, err := utilsv1alpha1.GetBroadcastAddresses(ctx, clusterClient.KubeAdminClient().CoreV1(), sdc)
		if err != nil {
			return fmt.Errorf("can't get broadcast addresses for ScyllaDBDatacenter %q: %w", naming.ObjRef(sdc), err)
		}
		allBroadcastAddresses[dc.Name] = hosts
		sortedAllBroadcastAddresses = append(sortedAllBroadcastAddresses, hosts...)
	}
	err := errors.Join(errs...)
	if err != nil {
		return err
	}

	sort.Strings(sortedAllBroadcastAddresses)

	for _, dc := range sc.Spec.Datacenters {
		clusterClient, ok := rkcClusterMap[dc.RemoteKubernetesClusterName]
		if !ok {
			errs = append(errs, fmt.Errorf("cluster client is missing for DC %q of ScyllaDBCluster %q", dc.Name, naming.ObjRef(sc)))
			continue
		}

		dcStatus, _, ok := oslices.Find(sc.Status.Datacenters, func(dcStatus scyllav1alpha1.ScyllaDBClusterDatacenterStatus) bool {
			return dcStatus.Name == dc.Name
		})
		if !ok {
			return fmt.Errorf("can't find %q datacenter status", dc.Name)
		}
		if dcStatus.RemoteNamespaceName == nil {
			return fmt.Errorf("unexpected nil remote namespace name in %q datacenter status", dc.Name)
		}

		sdc, err := clusterClient.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBDatacenters(*dcStatus.RemoteNamespaceName).Get(ctx, naming.ScyllaDBDatacenterName(sc, &dc), metav1.GetOptions{})
		if err != nil {
			errs = append(errs, fmt.Errorf("can't get ScyllaDBDatacenter %q: %w", dc.Name, err))
			continue
		}

		err = utilsv1alpha1.WaitForFullQuorum(ctx, clusterClient.KubeAdminClient().CoreV1(), sdc, sortedAllBroadcastAddresses)
		if err != nil {
			errs = append(errs, err)
		}
	}

	err = errors.Join(errs...)
	if err != nil {
		return fmt.Errorf("can't wait for scylla nodes to reach status consistency: %w", err)
	}

	framework.Infof("ScyllaDB nodes have reached status consistency.")

	return nil
}
