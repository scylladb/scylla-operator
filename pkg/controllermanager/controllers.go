// Copyright (c) 2026 ScyllaDB.

package controllermanager

import (
	"context"
	"fmt"

	"github.com/scylladb/scylla-operator/pkg/controller/globalscylladbmanager"
	"github.com/scylladb/scylla-operator/pkg/controller/nodeconfig"
	"github.com/scylladb/scylla-operator/pkg/controller/nodeconfigpod"
	"github.com/scylladb/scylla-operator/pkg/controller/orphanedpv"
	"github.com/scylladb/scylla-operator/pkg/controller/remotekubernetescluster"
	"github.com/scylladb/scylla-operator/pkg/controller/scyllacluster"
	"github.com/scylladb/scylla-operator/pkg/controller/scylladbcluster"
	"github.com/scylladb/scylla-operator/pkg/controller/scylladbdatacenter"
	"github.com/scylladb/scylla-operator/pkg/controller/scylladbmanagerclusterregistration"
	"github.com/scylladb/scylla-operator/pkg/controller/scylladbmanagertask"
	"github.com/scylladb/scylla-operator/pkg/controller/scylladbmonitoring"
	"github.com/scylladb/scylla-operator/pkg/controller/scyllaoperatorconfig"
	remoteclient "github.com/scylladb/scylla-operator/pkg/remoteclient/client"
	"github.com/scylladb/scylla-operator/pkg/remotecluster"
	"sigs.k8s.io/controller-runtime/pkg/controller"
)

// registerControllers wires every controller of the operator binary with the manager. The two multi-datacenter
// controllers share the set of remote clusters: the RemoteKubernetesCluster controller adds and removes them, the
// ScyllaDBCluster controller reads, writes and watches through them.
func (m *Manager) registerControllers(ctx context.Context) error {
	o := m.options
	var err error

	remoteClusters := remotecluster.New(ctx, scylladbcluster.RemoteCacheOptions())

	// The ScyllaDBDatacenter controller is a controller-runtime reconciler: it reads and writes through the manager's
	// client and is run by the manager.
	sdcc := scylladbdatacenter.NewController(
		m.mgr.GetClient(),
		m.mgr.GetAPIReader(),
		m.mgr.GetEventRecorderFor("scylladbdatacenter-controller"),
		o.OperatorImage,
		o.CQLSIngressPort,
		o.KeyGenerator,
	)
	err = sdcc.SetupWithManager(m.mgr, controller.Options{
		MaxConcurrentReconciles: o.ConcurrentSyncs,
	})
	if err != nil {
		return fmt.Errorf("can't set up scylladbdatacenter controller: %w", err)
	}

	scc := scyllacluster.NewController(
		m.mgr.GetClient(),
		m.mgr.GetAPIReader(),
		m.mgr.GetEventRecorderFor("scyllacluster-controller"),
	)
	err = scc.SetupWithManager(m.mgr, controller.Options{
		MaxConcurrentReconciles: o.ConcurrentSyncs,
	})
	if err != nil {
		return fmt.Errorf("can't set up scyllacluster controller: %w", err)
	}

	opc := orphanedpv.NewController(
		m.mgr.GetClient(),
		m.mgr.GetAPIReader(),
		m.mgr.GetEventRecorderFor("orphanedpv-controller"),
	)
	err = opc.SetupWithManager(m.mgr, orphanedpv.ControllerOptions(o.ConcurrentSyncs))
	if err != nil {
		return fmt.Errorf("can't set up orphanedpv controller: %w", err)
	}

	ncc := nodeconfig.NewController(
		m.mgr.GetClient(),
		m.mgr.GetAPIReader(),
		m.mgr.GetEventRecorderFor("NodeConfig-controller"),
		o.OperatorImage,
	)
	err = ncc.SetupWithManager(m.mgr, nodeconfig.ControllerOptions(o.ConcurrentSyncs))
	if err != nil {
		return fmt.Errorf("can't set up nodeconfig controller: %w", err)
	}

	ncpc := nodeconfigpod.NewController(
		m.mgr.GetClient(),
		m.mgr.GetAPIReader(),
		m.mgr.GetEventRecorderFor("NodeConfigCM-controller"),
	)
	err = ncpc.SetupWithManager(m.mgr, nodeconfigpod.ControllerOptions(o.ConcurrentSyncs))
	if err != nil {
		return fmt.Errorf("can't set up nodeconfigpod controller: %w", err)
	}

	socc := scyllaoperatorconfig.NewController(
		m.mgr.GetClient(),
		m.mgr.GetEventRecorderFor("scyllaoperatorconfig-controller"),
		o.ClusterDomainGetter,
	)
	err = socc.SetupWithManager(m.mgr, scyllaoperatorconfig.ControllerOptions())
	if err != nil {
		return fmt.Errorf("can't set up scyllaoperatorconfig controller: %w", err)
	}

	if o.MonitoringCRDsInstalled {
		mc := scylladbmonitoring.NewController(
			m.mgr.GetClient(),
			m.mgr.GetAPIReader(),
			m.mgr.GetEventRecorderFor("scylladbmonitoring-controller"),
			o.KeyGenerator,
		)
		err = mc.SetupWithManager(m.mgr, controller.Options{
			MaxConcurrentReconciles: o.ConcurrentSyncs,
		})
		if err != nil {
			return fmt.Errorf("can't set up scylladbmonitoring controller: %w", err)
		}
	}

	rkcc := remotekubernetescluster.NewController(
		m.mgr.GetClient(),
		m.mgr.GetAPIReader(),
		m.mgr.GetEventRecorderFor("remotekubernetescluster-controller"),
		[]remoteclient.DynamicClusterInterface{
			o.ClusterKubeClient,
			o.ClusterScyllaClient,
			remoteClusters,
		},
		o.ClusterKubeClient,
		o.ClusterScyllaClient,
	)
	err = rkcc.SetupWithManager(m.mgr, controller.Options{
		MaxConcurrentReconciles: o.ConcurrentSyncs,
	})
	if err != nil {
		return fmt.Errorf("can't set up RemoteKubernetesCluster controller: %w", err)
	}

	sdbcc := scylladbcluster.NewController(
		m.mgr.GetClient(),
		m.mgr.GetAPIReader(),
		m.mgr.GetEventRecorderFor("scylladbcluster-controller"),
		remoteClusters,
	)
	err = sdbcc.SetupWithManager(m.mgr, controller.Options{
		MaxConcurrentReconciles: o.ConcurrentSyncs,
	})
	if err != nil {
		return fmt.Errorf("can't set up ScyllaDBCluster controller: %w", err)
	}

	gsmc := globalscylladbmanager.NewController(
		m.mgr.GetClient(),
		m.mgr.GetEventRecorderFor("globalscylladbmanager-controller"),
	)
	err = gsmc.SetupWithManager(m.mgr, controller.Options{
		MaxConcurrentReconciles: 1,
	})
	if err != nil {
		return fmt.Errorf("can't set up global ScyllaDB Manager controller: %w", err)
	}

	smcrc := scylladbmanagerclusterregistration.NewController(
		m.mgr.GetClient(),
		m.mgr.GetAPIReader(),
		m.mgr.GetEventRecorderFor("scylladbmanagerclusterregistration-controller"),
	)
	err = smcrc.SetupWithManager(m.mgr, scylladbmanagerclusterregistration.ControllerOptions(o.ConcurrentSyncs))
	if err != nil {
		return fmt.Errorf("can't set up ScyllaDBManagerClusterRegistration controller: %w", err)
	}

	smtc := scylladbmanagertask.NewController(
		m.mgr.GetClient(),
		m.mgr.GetAPIReader(),
		m.mgr.GetEventRecorderFor("scylladbmanagertask-controller"),
	)
	err = smtc.SetupWithManager(m.mgr, scylladbmanagertask.ControllerOptions(o.ConcurrentSyncs))
	if err != nil {
		return fmt.Errorf("can't set up ScyllaDBManagerTask controller: %w", err)
	}

	return nil
}
