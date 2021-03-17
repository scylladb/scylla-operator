// Copyright (C) 2017 ScyllaDB

package manager

import (
	"context"
	"net/http"
	"net/url"
	"reflect"
	"time"

	"github.com/pkg/errors"
	"github.com/scylladb/go-log"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/mermaidclient"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"go.uber.org/multierr"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	ctrl "sigs.k8s.io/controller-runtime"
	mgr "sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Reconciler reconciles a Cluster object
type Reconciler struct {
	client.Client
	UncachedClient client.Client
	ManagerClient  *mermaidclient.Client

	IgnoredNamespace string

	Scheme *runtime.Scheme
	Logger log.Logger
}

func New(ctx context.Context, mgr mgr.Manager, ignoredNamespace, namespace string, logger log.Logger) (*Reconciler, error) {
	c, err := client.New(mgr.GetConfig(), client.Options{
		Scheme: mgr.GetScheme(),
	})
	if err != nil {
		return nil, errors.Wrap(err, "get dynamic client")
	}

	manager, err := discoverManager(ctx, mgr, namespace)
	if err != nil {
		return nil, errors.Wrap(err, "discover manager")
	}

	logger.Info(ctx, "Polling Manager server")
	if err := wait.Poll(2*time.Second, 5*time.Minute, func() (bool, error) {
		_, err := manager.Version(ctx)
		return err == nil, nil
	}); err != nil {
		return nil, errors.Wrap(err, "wait for manager")
	}
	logger.Info(ctx, "Manager server ready")

	return &Reconciler{
		Client:           mgr.GetClient(),
		UncachedClient:   c,
		ManagerClient:    manager,
		IgnoredNamespace: ignoredNamespace,
		Scheme:           mgr.GetScheme(),
		Logger:           logger,
	}, nil
}

func discoverManager(ctx context.Context, mgr mgr.Manager, namespace string) (*mermaidclient.Client, error) {
	cl, err := client.New(mgr.GetConfig(), client.Options{
		Scheme: mgr.GetScheme(),
	})
	if err != nil {
		return nil, errors.Wrap(err, "get dynamic client")
	}

	svcList := &corev1.ServiceList{}
	err = cl.List(ctx, svcList, &client.ListOptions{
		LabelSelector: naming.ManagerSelector(),
		Namespace:     namespace,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to list member services")
	}

	if len(svcList.Items) == 0 {
		return nil, errors.Wrap(err, "cannot find Scylla Manager service")
	}

	apiAddress := (&url.URL{
		Scheme: "http",
		Host:   svcList.Items[0].Name,
		Path:   "/api/v1",
	}).String()

	manager, err := mermaidclient.NewClient(apiAddress, &http.Transport{})
	if err != nil {
		return nil, errors.Wrap(err, "create manager client")
	}

	return &manager, nil
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Logger.With("cluster", req.NamespacedName)

	if req.Namespace == r.IgnoredNamespace {
		logger.Debug(ctx, "ignoring reconcile")
		return ctrl.Result{}, nil
	}

	var cluster scyllav1.ScyllaCluster
	if err := r.UncachedClient.Get(ctx, req.NamespacedName, &cluster); err != nil {
		if apierrors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			logger.Info(ctx, "Object not found", "namespace", req.Namespace, "name", req.Name)
			return reconcile.Result{}, nil
		}
		return ctrl.Result{Requeue: true}, errors.Wrap(err, "get cluster")
	}

	for _, rack := range cluster.Spec.Datacenter.Racks {
		logger.Debug(ctx, "cluster spec", "racks", cluster.Spec.Datacenter.Racks)
		logger.Debug(ctx, "cluster status", "racks", cluster.Status.Racks)
		status, ok := cluster.Status.Racks[rack.Name]
		if !ok {
			logger.Debug(ctx, "Rack not found", "rack", rack.Name)
			return ctrl.Result{Requeue: true}, nil
		}
		if status.ReadyMembers != rack.Members {
			logger.Info(ctx, "Rack not ready", "rack", rack.Name)
			return ctrl.Result{Requeue: true}, nil
		}
	}

	logger.Info(ctx, "Cluster ready, syncing with Scylla Manager", "resource_version", cluster.ResourceVersion)

	clusterID := ""
	if cluster.Status.ManagerID != nil {
		clusterID = *cluster.Status.ManagerID
	}

	managerState, err := r.managerState(ctx, clusterID)
	if err != nil {
		logger.Error(ctx, "Failed to fetch manager state", "error", err)
		return reconcile.Result{}, err
	}
	r.Logger.Debug(ctx, "Manager state", "state", managerState)

	authToken, err := r.getAuthToken(ctx, &cluster)
	if err != nil {
		logger.Error(ctx, "Failed to fetch cluster auth token", "error", err)
		return reconcile.Result{}, err
	}

	clusterCopy := cluster.DeepCopy()
	actions, requeue, err := sync(ctx, &cluster, authToken, managerState)
	if err != nil {
		logger.Error(ctx, "Failed to generate sync actions", "error", err)
		return reconcile.Result{}, err
	}

	var actionErrs error
	for _, a := range actions {
		logger.Debug(ctx, "Executing action", "action", a)
		if err := a.Execute(ctx, r.ManagerClient, &cluster.Status); err != nil {
			logger.Error(ctx, "Failed to execute action", "action", a, "error", err)
			actionErrs = multierr.Append(actionErrs, err)
		}
	}

	// Update status if needed
	if !reflect.DeepEqual(cluster.Status, clusterCopy.Status) {
		logger.Debug(ctx, "Updating cluster status", "new", cluster.Status, "old", clusterCopy.Status)
		if err := r.Status().Update(ctx, &cluster); err != nil {
			logger.Error(ctx, "Failed to update cluster status", "resource_version", cluster.ResourceVersion, "error", err)
			return reconcile.Result{}, errors.WithStack(err)
		}
	}

	if actionErrs != nil {
		logger.Info(ctx, "Failed to execute actions, retrying")
		return ctrl.Result{}, actionErrs
	}

	logger.Info(ctx, "Reconcile successful", "requeue", requeue)

	return ctrl.Result{Requeue: requeue}, nil
}

func (r *Reconciler) managerState(ctx context.Context, clusterID string) (*state, error) {
	clusters, err := r.ManagerClient.ListClusters(ctx)
	if err != nil {
		return nil, err
	}
	var (
		repairTasks []*RepairTask
		backupTasks []*BackupTask
	)

	if clusterID != "" {
		clusterFound := false
		for _, c := range clusters {
			if c.ID == clusterID {
				clusterFound = true
			}
		}

		if clusterFound {
			managerRepairTasks, err := r.ManagerClient.ListTasks(ctx, clusterID, "repair", true, "")
			if err != nil {
				return nil, err
			}

			repairTasks = make([]*RepairTask, 0, len(managerRepairTasks.ExtendedTaskSlice))
			for _, managerRepairTask := range managerRepairTasks.ExtendedTaskSlice {
				rt := &RepairTask{}
				if err := rt.FromManager(managerRepairTask); err != nil {
					return nil, err
				}
				repairTasks = append(repairTasks, rt)
			}

			managerBackupTasks, err := r.ManagerClient.ListTasks(ctx, clusterID, "backup", true, "")
			if err != nil {
				return nil, err
			}

			backupTasks = make([]*BackupTask, 0, len(managerBackupTasks.ExtendedTaskSlice))
			for _, managerBackupTask := range managerBackupTasks.ExtendedTaskSlice {
				bt := &BackupTask{}
				if err := bt.FromManager(managerBackupTask); err != nil {
					return nil, err
				}
				backupTasks = append(backupTasks, bt)
			}
		}
	}

	return &state{
		Clusters:    clusters,
		BackupTasks: backupTasks,
		RepairTasks: repairTasks,
	}, nil
}

func (r *Reconciler) getAuthToken(ctx context.Context, cluster *scyllav1.ScyllaCluster) (string, error) {
	const agentConfigKey = "scylla-manager-agent.yaml"
	secret := &corev1.Secret{}
	agentSecretName := cluster.Spec.Datacenter.Racks[0].ScyllaAgentConfig
	secretKey := client.ObjectKey{
		Name:      agentSecretName,
		Namespace: cluster.Namespace,
	}
	err := r.Client.Get(ctx, secretKey, secret)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return "", nil
		}
		return "", errors.Wrapf(err, "get %s secret", agentSecretName)
	}
	agentConfig := struct {
		AuthToken string `yaml:"auth_token"`
	}{}
	if err := yaml.Unmarshal(secret.Data[agentConfigKey], &agentConfig); err != nil {
		return "", errors.Wrap(err, "cannot parse agent config")
	}

	return agentConfig.AuthToken, nil
}

func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&scyllav1.ScyllaCluster{}).
		Complete(r)
}
