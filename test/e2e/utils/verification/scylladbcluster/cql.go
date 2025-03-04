package scylladbcluster

import (
	"context"
	"errors"
	"fmt"
	"github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	v1alpha2 "github.com/scylladb/scylla-operator/test/e2e/utils/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	v2 "k8s.io/client-go/kubernetes/typed/core/v1"
	"reflect"
	"sort"
	"strings"
	"time"
)

func WaitForFullQuorum(ctx context.Context, rkcClusterMap map[string]framework.ClusterInterface, sc *v1alpha1.ScyllaDBCluster) error {
	allBroadcastAddresses := map[string][]string{}
	var sortedAllBroadcastAddresses []string

	var errs []error
	for _, dc := range sc.Spec.Datacenters {
		clusterClient, ok := rkcClusterMap[dc.RemoteKubernetesClusterName]
		if !ok {
			errs = append(errs, fmt.Errorf("cluster client is missing for DC %q of ScyllaDBCluster %q", dc.Name, naming.ObjRef(sc)))
			continue
		}

		dcStatus, _, ok := slices.Find(sc.Status.Datacenters, func(dcStatus v1alpha1.ScyllaDBClusterDatacenterStatus) bool {
			return dcStatus.Name == dc.Name
		})
		if !ok {
			return fmt.Errorf("can't find %q datacenter status", dc.Name)
		}
		if dcStatus.RemoteNamespaceName == nil {
			return fmt.Errorf("unexpected nil remote namespace name in %q datacenter status", dc.Name)
		}

		sdc, err := clusterClient.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBDatacenters(*dcStatus.RemoteNamespaceName).Get(ctx, naming.ScyllaDBDatacenterName(sc, &dc), v1.GetOptions{})
		if err != nil {
			errs = append(errs, fmt.Errorf("can't get ScyllaDBDatacenter %q: %w", dc.Name, err))
			continue
		}

		hosts, err := v1alpha2.GetBroadcastAddresses(ctx, clusterClient.KubeAdminClient().CoreV1(), sdc)
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

		dcStatus, _, ok := slices.Find(sc.Status.Datacenters, func(dcStatus v1alpha1.ScyllaDBClusterDatacenterStatus) bool {
			return dcStatus.Name == dc.Name
		})
		if !ok {
			return fmt.Errorf("can't find %q datacenter status", dc.Name)
		}
		if dcStatus.RemoteNamespaceName == nil {
			return fmt.Errorf("unexpected nil remote namespace name in %q datacenter status", dc.Name)
		}

		sdc, err := clusterClient.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBDatacenters(*dcStatus.RemoteNamespaceName).Get(ctx, naming.ScyllaDBDatacenterName(sc, &dc), v1.GetOptions{})
		if err != nil {
			errs = append(errs, fmt.Errorf("can't get ScyllaDBDatacenter %q: %w", dc.Name, err))
			continue
		}

		err = waitForFullQuorum(ctx, clusterClient.KubeAdminClient().CoreV1(), sdc, sortedAllBroadcastAddresses)
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

// TODO: Should be unified with function coming from test/helpers once e2e's there starts using ScyllaDBDatacenter API.
func waitForFullQuorum(ctx context.Context, client v2.CoreV1Interface, sdc *v1alpha1.ScyllaDBDatacenter, sortedExpectedHosts []string) error {
	scyllaClient, hosts, err := v1alpha2.GetScyllaClient(ctx, client, sdc)
	if err != nil {
		return fmt.Errorf("can't get scylla client: %w", err)
	}
	defer scyllaClient.Close()

	// Wait for node status to propagate and reach consistency.
	// This can take a while so let's set a large enough timeout to avoid flakes.
	return wait.PollImmediateWithContext(ctx, 1*time.Second, 5*time.Minute, func(ctx context.Context) (done bool, err error) {
		allSeeAllAsUN := true
		infoMessages := make([]string, 0, len(hosts))
		var errs []error
		for _, h := range hosts {
			s, err := scyllaClient.Status(ctx, h)
			if err != nil {
				return true, fmt.Errorf("can't get scylla status on node %q: %w", h, err)
			}

			sHosts := s.Hosts()
			sort.Strings(sHosts)
			if !reflect.DeepEqual(sHosts, sortedExpectedHosts) {
				errs = append(errs, fmt.Errorf("node %q thinks the cluster consists of different nodes: %s", h, sHosts))
			}

			downHosts := s.DownHosts()
			infoMessages = append(infoMessages, fmt.Sprintf("Node %q, down: %q, up: %q", h, strings.Join(downHosts, "\n"), strings.Join(s.LiveHosts(), ",")))

			if len(downHosts) != 0 {
				allSeeAllAsUN = false
			}
		}

		if !allSeeAllAsUN {
			framework.Infof("ScyllaDB nodes have not reached status consistency yet. Statuses:\n%s", strings.Join(infoMessages, ","))
		}

		err = errors.Join(errs...)
		if err != nil {
			framework.Infof("ScyllaDB nodes encountered an error. Statuses:\n%s", strings.Join(infoMessages, ","))
			return true, err
		}

		return allSeeAllAsUN, nil
	})
}
