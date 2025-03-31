package scylladbcluster

import (
	"context"
	"fmt"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllaclient "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/controllertools"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/naming"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

func (scc *Controller) sync(ctx context.Context, key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		klog.ErrorS(err, "Failed to split meta namespace cache key", "cacheKey", key)
		return err
	}

	startTime := time.Now()
	klog.V(4).InfoS("Started syncing ScyllaDBCluster", "ScyllaDBCluster", klog.KRef(namespace, name), "startTime", startTime)
	defer func() {
		klog.V(4).InfoS("Finished syncing ScyllaDBCluster", "ScyllaDBCluster", klog.KRef(namespace, name), "duration", time.Since(startTime))
	}()

	sc, err := scc.scyllaDBClusterLister.ScyllaDBClusters(namespace).Get(name)
	if errors.IsNotFound(err) {
		klog.V(2).InfoS("ScyllaDBCluster has been deleted", "ScyllaDBCluster", klog.KRef(namespace, name))
		return nil
	}
	if err != nil {
		return fmt.Errorf("can't get ScyllaDBCluster %q: %w", naming.ManualRef(namespace, name), err)
	}

	soc, err := scc.scyllaOperatorConfigLister.Get(naming.SingletonName)
	if err != nil {
		return fmt.Errorf("can't get ScyllaOperatorConfig %q: %w", naming.SingletonName, err)
	}

	scRemoteSelector := naming.ScyllaDBClusterSelector(sc)

	// OS Operator rewrites ScyllaDBDatacenter labels into managed Service objects, and
	// Kubernetes controller reconciling Endpoints for Services rewrites them to Endpoints.
	// As a result, we can't distinguish Endpoints managed by us from these managed by Kubernetes.
	// To overcome this, the selector we use for Endpoints is a superset of the selector of other managed objects.
	scRemoteEndpointsSelector := naming.ScyllaDBClusterEndpointsSelector(sc)

	// Operator reconciles objects in remote Kubernetes clusters, hence we can't set up a OwnerReference to ScyllaDBCluster
	// because it's not there. Instead, we will manage dependent object ownership via a RemoteOwner.
	type remoteCT = *scyllav1alpha1.RemoteOwner
	var objectErrs []error

	remoteClusterNames := slices.ConvertSlice(sc.Spec.Datacenters, func(dc scyllav1alpha1.ScyllaDBClusterDatacenter) string {
		return dc.RemoteKubernetesClusterName
	})

	remoteNamespaceMap, err := scc.getRemoteNamespacesMap(sc)
	if err != nil {
		objectErrs = append(objectErrs, fmt.Errorf("can't get remote namespaces: %w", err))
	}

	remoteNamespaces, err := scc.chooseRemoteNamespaces(sc, remoteNamespaceMap)
	if err != nil {
		objectErrs = append(objectErrs, fmt.Errorf("can't choose remote namespaces: %w", err))
	}

	remoteRemoteOwnerMap, err := scc.getRemoteRemoteOwners(sc, remoteNamespaces)
	if err != nil {
		objectErrs = append(objectErrs, fmt.Errorf("can't get remote remoteowners: %w", err))
	}

	remoteControllers, err := scc.chooseRemoteControllers(sc, remoteRemoteOwnerMap)
	if err != nil {
		objectErrs = append(objectErrs, fmt.Errorf("can't choose remote controllers: %w", err))
	}

	remoteServiceMap, err := controllerhelpers.GetRemoteObjects[remoteCT, *corev1.Service](ctx, remoteClusterNames, remoteControllers, remoteControllerGVK, scRemoteSelector, &controllerhelpers.ClusterControlleeManagerGetObjectsFuncs[remoteCT, *corev1.Service]{
		ClusterFunc: func(clusterName string) (controllerhelpers.ControlleeManagerGetObjectsInterface[remoteCT, *corev1.Service], error) {
			ns, ok := remoteNamespaces[clusterName]
			if !ok {
				return nil, nil
			}

			kubeClusterClient, scyllaClusterClient, err := scc.getClusterClients(clusterName)
			if err != nil {
				return nil, fmt.Errorf("can't get cluster %q clients: %w", clusterName, err)
			}

			return &controllerhelpers.ControlleeManagerGetObjectsFuncs[remoteCT, *corev1.Service]{
				GetControllerUncachedFunc: scyllaClusterClient.ScyllaV1alpha1().RemoteOwners(ns.Name).Get,
				ListObjectsFunc:           scc.remoteServiceLister.Cluster(clusterName).Services(ns.Name).List,
				PatchObjectFunc:           kubeClusterClient.CoreV1().Services(ns.Name).Patch,
			}, nil
		},
	})
	if err != nil {
		objectErrs = append(objectErrs, fmt.Errorf("can't get remote services: %w", err))
	}

	remoteEndpointSlicesMap, err := controllerhelpers.GetRemoteObjects[remoteCT, *discoveryv1.EndpointSlice](ctx, remoteClusterNames, remoteControllers, remoteControllerGVK, scRemoteSelector, &controllerhelpers.ClusterControlleeManagerGetObjectsFuncs[remoteCT, *discoveryv1.EndpointSlice]{
		ClusterFunc: func(clusterName string) (controllerhelpers.ControlleeManagerGetObjectsInterface[remoteCT, *discoveryv1.EndpointSlice], error) {
			ns, ok := remoteNamespaces[clusterName]
			if !ok {
				return nil, nil
			}

			kubeClusterClient, scyllaClusterClient, err := scc.getClusterClients(clusterName)
			if err != nil {
				return nil, fmt.Errorf("can't get cluster %q clients: %w", clusterName, err)
			}

			return &controllerhelpers.ControlleeManagerGetObjectsFuncs[remoteCT, *discoveryv1.EndpointSlice]{
				GetControllerUncachedFunc: scyllaClusterClient.ScyllaV1alpha1().RemoteOwners(ns.Name).Get,
				ListObjectsFunc:           scc.remoteEndpointSliceLister.Cluster(clusterName).EndpointSlices(ns.Name).List,
				PatchObjectFunc:           kubeClusterClient.DiscoveryV1().EndpointSlices(ns.Name).Patch,
			}, nil
		},
	})
	if err != nil {
		objectErrs = append(objectErrs, fmt.Errorf("can't get remote endpointslices: %w", err))
	}

	// GKE DNS doesn't understand EndpointSlices hence we have to reconcile both Endpoints and EndpointSlices.
	// https://github.com/kubernetes/kubernetes/issues/107742
	remoteEndpointsMap, err := controllerhelpers.GetRemoteObjects[remoteCT, *corev1.Endpoints](ctx, remoteClusterNames, remoteControllers, remoteControllerGVK, scRemoteEndpointsSelector, &controllerhelpers.ClusterControlleeManagerGetObjectsFuncs[remoteCT, *corev1.Endpoints]{
		ClusterFunc: func(clusterName string) (controllerhelpers.ControlleeManagerGetObjectsInterface[remoteCT, *corev1.Endpoints], error) {
			ns, ok := remoteNamespaces[clusterName]
			if !ok {
				return nil, nil
			}

			kubeClusterClient, scyllaClusterClient, err := scc.getClusterClients(clusterName)
			if err != nil {
				return nil, fmt.Errorf("can't get cluster %q clients: %w", clusterName, err)
			}

			return &controllerhelpers.ControlleeManagerGetObjectsFuncs[remoteCT, *corev1.Endpoints]{
				GetControllerUncachedFunc: scyllaClusterClient.ScyllaV1alpha1().RemoteOwners(ns.Name).Get,
				ListObjectsFunc:           scc.remoteEndpointsLister.Cluster(clusterName).Endpoints(ns.Name).List,
				PatchObjectFunc:           kubeClusterClient.CoreV1().Endpoints(ns.Name).Patch,
			}, nil
		},
	})
	if err != nil {
		objectErrs = append(objectErrs, fmt.Errorf("can't get remote endpoints: %w", err))
	}

	remoteConfigMapMap, err := controllerhelpers.GetRemoteObjects[remoteCT, *corev1.ConfigMap](ctx, remoteClusterNames, remoteControllers, remoteControllerGVK, scRemoteSelector, &controllerhelpers.ClusterControlleeManagerGetObjectsFuncs[remoteCT, *corev1.ConfigMap]{
		ClusterFunc: func(clusterName string) (controllerhelpers.ControlleeManagerGetObjectsInterface[remoteCT, *corev1.ConfigMap], error) {
			ns, ok := remoteNamespaces[clusterName]
			if !ok {
				return nil, nil
			}

			kubeClusterClient, scyllaClusterClient, err := scc.getClusterClients(clusterName)
			if err != nil {
				return nil, fmt.Errorf("can't get cluster %q clients: %w", clusterName, err)
			}

			return &controllerhelpers.ControlleeManagerGetObjectsFuncs[remoteCT, *corev1.ConfigMap]{
				GetControllerUncachedFunc: scyllaClusterClient.ScyllaV1alpha1().RemoteOwners(ns.Name).Get,
				ListObjectsFunc:           scc.remoteConfigMapLister.Cluster(clusterName).ConfigMaps(ns.Name).List,
				PatchObjectFunc:           kubeClusterClient.CoreV1().ConfigMaps(ns.Name).Patch,
			}, nil
		},
	})
	if err != nil {
		objectErrs = append(objectErrs, fmt.Errorf("can't get remote configmaps: %w", err))
	}

	remoteSecretMap, err := controllerhelpers.GetRemoteObjects[remoteCT, *corev1.Secret](ctx, remoteClusterNames, remoteControllers, remoteControllerGVK, scRemoteSelector, &controllerhelpers.ClusterControlleeManagerGetObjectsFuncs[remoteCT, *corev1.Secret]{
		ClusterFunc: func(clusterName string) (controllerhelpers.ControlleeManagerGetObjectsInterface[remoteCT, *corev1.Secret], error) {
			ns, ok := remoteNamespaces[clusterName]
			if !ok {
				return nil, nil
			}

			kubeClusterClient, scyllaClusterClient, err := scc.getClusterClients(clusterName)
			if err != nil {
				return nil, fmt.Errorf("can't get cluster %q clients: %w", clusterName, err)
			}

			return &controllerhelpers.ControlleeManagerGetObjectsFuncs[remoteCT, *corev1.Secret]{
				GetControllerUncachedFunc: scyllaClusterClient.ScyllaV1alpha1().RemoteOwners(ns.Name).Get,
				ListObjectsFunc:           scc.remoteSecretLister.Cluster(clusterName).Secrets(ns.Name).List,
				PatchObjectFunc:           kubeClusterClient.CoreV1().Secrets(ns.Name).Patch,
			}, nil
		},
	})
	if err != nil {
		objectErrs = append(objectErrs, fmt.Errorf("can't get remote configmaps: %w", err))
	}

	remoteScyllaDBDatacenterMap, err := controllerhelpers.GetRemoteObjects[remoteCT, *scyllav1alpha1.ScyllaDBDatacenter](ctx, remoteClusterNames, remoteControllers, remoteControllerGVK, scRemoteSelector, &controllerhelpers.ClusterControlleeManagerGetObjectsFuncs[remoteCT, *scyllav1alpha1.ScyllaDBDatacenter]{
		ClusterFunc: func(clusterName string) (controllerhelpers.ControlleeManagerGetObjectsInterface[remoteCT, *scyllav1alpha1.ScyllaDBDatacenter], error) {
			ns, ok := remoteNamespaces[clusterName]
			if !ok {
				return nil, nil
			}

			_, scyllaClusterClient, err := scc.getClusterClients(clusterName)
			if err != nil {
				return nil, fmt.Errorf("can't get cluster %q clients: %w", clusterName, err)
			}

			return &controllerhelpers.ControlleeManagerGetObjectsFuncs[remoteCT, *scyllav1alpha1.ScyllaDBDatacenter]{
				GetControllerUncachedFunc: scyllaClusterClient.ScyllaV1alpha1().RemoteOwners(ns.Name).Get,
				ListObjectsFunc:           scc.remoteScyllaDBDatacenterLister.Cluster(clusterName).ScyllaDBDatacenters(ns.Name).List,
				PatchObjectFunc:           scyllaClusterClient.ScyllaV1alpha1().ScyllaDBDatacenters(ns.Name).Patch,
			}, nil
		},
	})
	if err != nil {
		objectErrs = append(objectErrs, fmt.Errorf("can't get remote scyllaclusters: %w", err))
	}

	objectErr := utilerrors.NewAggregate(objectErrs)
	if objectErr != nil {
		return objectErr
	}

	status := scc.calculateStatus(sc, remoteScyllaDBDatacenterMap)

	if sc.DeletionTimestamp != nil {
		err = controllerhelpers.RunSync(
			&status.Conditions,
			scyllaDBClusterFinalizerProgressingCondition,
			scyllaDBClusterFinalizerDegradedCondition,
			sc.Generation,
			func() ([]metav1.Condition, error) {
				return scc.syncFinalizer(ctx, sc, remoteNamespaces)
			},
		)
		if err != nil {
			return fmt.Errorf("can't finalize: %w", err)
		}
		return scc.updateStatus(ctx, sc, status)
	}

	if soc.Status.ClusterDomain == nil || len(*soc.Status.ClusterDomain) == 0 {
		scc.eventRecorder.Event(sc, corev1.EventTypeNormal, "MissingClusterDomain", "ScyllaOperatorConfig doesn't yet have clusterDomain available in the status.")
		return controllertools.NewNonRetriable("ScyllaOperatorConfig doesn't yet have clusterDomain available in the status")
	}
	managingClusterDomain := *soc.Status.ClusterDomain

	if !scc.hasFinalizer(sc.GetFinalizers()) {
		err = scc.addFinalizer(ctx, sc)
		if err != nil {
			return fmt.Errorf("can't add finalizer: %w", err)
		}
		return nil
	}

	var errs []error

	err = controllerhelpers.RunSync(
		&status.Conditions,
		remoteNamespaceControllerProgressingCondition,
		remoteNamespaceControllerDegradedCondition,
		sc.Generation,
		func() ([]metav1.Condition, error) {
			return scc.syncNamespaces(ctx, sc, remoteNamespaceMap, managingClusterDomain)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync remote namespaces: %w", err))
	}

	err = controllerhelpers.RunSync(
		&status.Conditions,
		remoteRemoteOwnerControllerProgressingCondition,
		remoteRemoteOwnerControllerDegradedCondition,
		sc.Generation,
		func() ([]metav1.Condition, error) {
			return scc.syncRemoteOwners(ctx, sc, remoteNamespaces, remoteRemoteOwnerMap, managingClusterDomain)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync remote remoteowners: %w", err))
	}

	err = controllerhelpers.RunSync(
		&status.Conditions,
		remoteServiceControllerProgressingCondition,
		remoteServiceControllerDegradedCondition,
		sc.Generation,
		func() ([]metav1.Condition, error) {
			return scc.syncServices(ctx, sc, remoteNamespaces, remoteControllers, remoteServiceMap, managingClusterDomain)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync remote services: %w", err))
	}

	err = controllerhelpers.RunSync(
		&status.Conditions,
		remoteEndpointSliceControllerProgressingCondition,
		remoteEndpointSliceControllerDegradedCondition,
		sc.Generation,
		func() ([]metav1.Condition, error) {
			return scc.syncEndpointSlices(ctx, sc, remoteNamespaces, remoteControllers, remoteEndpointSlicesMap, managingClusterDomain)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync remote endpointslices: %w", err))
	}

	err = controllerhelpers.RunSync(
		&status.Conditions,
		remoteEndpointsControllerProgressingCondition,
		remoteEndpointsControllerDegradedCondition,
		sc.Generation,
		func() ([]metav1.Condition, error) {
			return scc.syncEndpoints(ctx, sc, remoteNamespaces, remoteControllers, remoteEndpointsMap, managingClusterDomain)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync remote endpoints: %w", err))
	}

	err = controllerhelpers.RunSync(
		&status.Conditions,
		remoteConfigMapControllerProgressingCondition,
		remoteConfigMapControllerDegradedCondition,
		sc.Generation,
		func() ([]metav1.Condition, error) {
			return scc.syncConfigMaps(ctx, sc, remoteNamespaces, remoteControllers, remoteConfigMapMap, managingClusterDomain)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync remote configmaps: %w", err))
	}

	err = controllerhelpers.RunSync(
		&status.Conditions,
		remoteSecretControllerProgressingCondition,
		remoteSecretControllerDegradedCondition,
		sc.Generation,
		func() ([]metav1.Condition, error) {
			return scc.syncSecrets(ctx, sc, remoteNamespaces, remoteControllers, remoteSecretMap, managingClusterDomain)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync remote secrets: %w", err))
	}

	err = controllerhelpers.RunSync(
		&status.Conditions,
		remoteScyllaDBDatacenterControllerProgressingCondition,
		remoteScyllaDBDatacenterControllerDegradedCondition,
		sc.Generation,
		func() ([]metav1.Condition, error) {
			return scc.syncScyllaDBDatacenters(ctx, sc, remoteNamespaces, remoteControllers, remoteScyllaDBDatacenterMap, managingClusterDomain)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync remote scylladbdatacenters: %w", err))
	}

	// Aggregate conditions.
	err = controllerhelpers.SetAggregatedWorkloadConditions(&status.Conditions, sc.Generation)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't aggregate workload conditions: %w", err))
	} else {
		err = scc.updateStatus(ctx, sc, status)
		errs = append(errs, err)
	}

	return utilerrors.NewAggregate(errs)
}

func (scc *Controller) chooseRemoteControllers(sc *scyllav1alpha1.ScyllaDBCluster, remoteRemoteOwnersMap map[string]map[string]*scyllav1alpha1.RemoteOwner) (map[string]metav1.Object, error) {
	remoteControllers := make(map[string]metav1.Object)

	for _, dc := range sc.Spec.Datacenters {
		remoteOwnersMap, ok := remoteRemoteOwnersMap[dc.RemoteKubernetesClusterName]
		if !ok || len(remoteOwnersMap) == 0 {
			// Might not be created yet
			continue
		}

		remoteOwners := helpers.GetMapValues(remoteOwnersMap)
		if len(remoteOwners) > 1 {
			klog.InfoS("Found more than one RemoteOwner pointing to ScyllaDBCluster of the same UID, ignoring both to prune the extra one", "Cluster", dc.RemoteKubernetesClusterName, "ScyllaDBCluster", naming.ObjRef(sc), "UID", sc.UID)
			continue
		}

		remoteControllers[dc.RemoteKubernetesClusterName] = remoteOwners[0]
	}

	return remoteControllers, nil
}

func (scc *Controller) getRemoteRemoteOwners(sc *scyllav1alpha1.ScyllaDBCluster, remoteNamespaces map[string]*corev1.Namespace) (map[string]map[string]*scyllav1alpha1.RemoteOwner, error) {
	remoteOwnerMap := make(map[string]map[string]*scyllav1alpha1.RemoteOwner, len(sc.Spec.Datacenters))
	for _, dc := range sc.Spec.Datacenters {
		ns, ok := remoteNamespaces[dc.RemoteKubernetesClusterName]
		if !ok {
			// Might not exist yet
			continue
		}

		selector := labels.SelectorFromSet(naming.RemoteOwnerSelectorLabels(sc, &dc))
		remoteOwners, err := scc.remoteRemoteOwnerLister.Cluster(dc.RemoteKubernetesClusterName).RemoteOwners(ns.Name).List(selector)
		if err != nil {
			return nil, fmt.Errorf("can't list remote remoteowners in %q cluster: %w", dc.RemoteKubernetesClusterName, err)
		}

		roMap := make(map[string]*scyllav1alpha1.RemoteOwner, len(remoteOwners))

		for _, ro := range remoteOwners {
			if len(ro.OwnerReferences) > 0 {
				return nil, fmt.Errorf("unexpected RemoteOwner %q matching our selector and having an OwnerReference in %q cluster", naming.ObjRef(ro), dc.RemoteKubernetesClusterName)
			}

			roMap[ro.Name] = ro
		}

		remoteOwnerMap[dc.RemoteKubernetesClusterName] = roMap
	}

	return remoteOwnerMap, nil
}

// chooseRemoteNamespaces returns a map of namespaces used for reconciled objects per each remote.
func (scc *Controller) chooseRemoteNamespaces(sc *scyllav1alpha1.ScyllaDBCluster, remoteNamespacesMap map[string]map[string]*corev1.Namespace) (map[string]*corev1.Namespace, error) {
	remoteNamespaces := make(map[string]*corev1.Namespace)

	for _, dc := range sc.Spec.Datacenters {
		rnss, ok := remoteNamespacesMap[dc.RemoteKubernetesClusterName]
		if !ok || len(rnss) == 0 {
			// Might not be created yet
			continue
		}

		remoteNss := helpers.GetMapValues(rnss)
		if len(remoteNss) > 1 {
			klog.InfoS("Found more than one Namespace pointing to ScyllaDBCluster of the same UID, ignoring both to prune the extra one", "Cluster", dc.RemoteKubernetesClusterName, "ScyllaDBCluster", naming.ObjRef(sc), "UID", sc.UID)
			continue
		}

		remoteNamespaces[dc.RemoteKubernetesClusterName] = remoteNss[0]
	}

	return remoteNamespaces, nil
}

// getRemoteNamespacesMap returns a map of remote namespaces matching provided selector.
func (scc *Controller) getRemoteNamespacesMap(sc *scyllav1alpha1.ScyllaDBCluster) (map[string]map[string]*corev1.Namespace, error) {
	namespacesMap := make(map[string]map[string]*corev1.Namespace, len(sc.Spec.Datacenters))

	for _, dc := range sc.Spec.Datacenters {
		selector := labels.SelectorFromSet(naming.ScyllaDBClusterDatacenterSelectorLabels(sc, &dc))
		remoteNamespaces, err := scc.remoteNamespaceLister.Cluster(dc.RemoteKubernetesClusterName).List(selector)
		if err != nil {
			return nil, fmt.Errorf("can't list remote namespaces in %q cluster: %w", dc.RemoteKubernetesClusterName, err)
		}

		nsMap := make(map[string]*corev1.Namespace, len(remoteNamespaces))
		for _, rns := range remoteNamespaces {
			if len(rns.OwnerReferences) != 0 {
				return nil, fmt.Errorf("unexpected Namespace %q matching our selector and having an OwnerReference in %q cluster", rns.Name, dc.RemoteKubernetesClusterName)
			}

			nsMap[rns.Name] = rns
		}

		namespacesMap[dc.RemoteKubernetesClusterName] = nsMap
	}

	return namespacesMap, nil
}

func (scc *Controller) getClusterClients(clusterName string) (kubernetes.Interface, scyllaclient.Interface, error) {
	kubeClusterClient, err := scc.kubeRemoteClient.Cluster(clusterName)
	if err != nil {
		return nil, nil, fmt.Errorf("can't get kube cluster %q client: %w", clusterName, err)
	}

	scyllaClusterClient, err := scc.scyllaRemoteClient.Cluster(clusterName)
	if err != nil {
		return nil, nil, fmt.Errorf("can't get scylla cluster %q client: %w", clusterName, err)
	}

	return kubeClusterClient, scyllaClusterClient, nil
}
