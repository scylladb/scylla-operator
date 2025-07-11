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
	oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/internalapi"
	"github.com/scylladb/scylla-operator/pkg/naming"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
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

	scLocalSelector := naming.ScyllaDBClusterLocalSelector(sc)

	// Kubernetes' controller-manager copies Service's labels into managed Endpoints.
	// As a result, we can't distinguish Endpoints managed by us from these managed by Kubernetes.
	// To overcome this, the selector we use for Endpoints is a superset of the selector of other managed objects.
	scLocalEndpointsSelector := naming.ScyllaDBClusterEndpointsSelector(sc)

	type localCT = *scyllav1alpha1.ScyllaDBCluster
	var localObjectErrs []error

	localServiceMap, err := controllerhelpers.GetObjects[localCT, *corev1.Service](
		ctx,
		sc,
		scyllav1alpha1.ScyllaDBClusterGVK,
		scLocalSelector,
		controllerhelpers.ControlleeManagerGetObjectsFuncs[localCT, *corev1.Service]{
			GetControllerUncachedFunc: scc.scyllaClient.ScyllaV1alpha1().ScyllaDBClusters(sc.Namespace).Get,
			ListObjectsFunc:           scc.serviceLister.Services(sc.Namespace).List,
			PatchObjectFunc:           scc.kubeClient.CoreV1().Services(sc.Namespace).Patch,
		},
	)
	if err != nil {
		localObjectErrs = append(localObjectErrs, fmt.Errorf("can't get services: %w", err))
	}

	localEndpointSlicesMap, err := controllerhelpers.GetObjects[localCT, *discoveryv1.EndpointSlice](
		ctx,
		sc,
		scyllav1alpha1.ScyllaDBClusterGVK,
		scLocalEndpointsSelector,
		controllerhelpers.ControlleeManagerGetObjectsFuncs[localCT, *discoveryv1.EndpointSlice]{
			GetControllerUncachedFunc: scc.scyllaClient.ScyllaV1alpha1().ScyllaDBClusters(sc.Namespace).Get,
			ListObjectsFunc:           scc.endpointSliceLister.EndpointSlices(sc.Namespace).List,
			PatchObjectFunc:           scc.kubeClient.DiscoveryV1().EndpointSlices(sc.Namespace).Patch,
		},
	)
	if err != nil {
		localObjectErrs = append(localObjectErrs, fmt.Errorf("can't get endpointslices: %w", err))
	}

	localEndpointsMap, err := controllerhelpers.GetObjects[localCT, *corev1.Endpoints](
		ctx,
		sc,
		scyllav1alpha1.ScyllaDBClusterGVK,
		scLocalEndpointsSelector,
		controllerhelpers.ControlleeManagerGetObjectsFuncs[localCT, *corev1.Endpoints]{
			GetControllerUncachedFunc: scc.scyllaClient.ScyllaV1alpha1().ScyllaDBClusters(sc.Namespace).Get,
			ListObjectsFunc:           scc.endpointsLister.Endpoints(sc.Namespace).List,
			PatchObjectFunc:           scc.kubeClient.CoreV1().Endpoints(sc.Namespace).Patch,
		},
	)
	if err != nil {
		localObjectErrs = append(localObjectErrs, fmt.Errorf("can't get endpoints: %w", err))
	}

	if err = apimachineryutilerrors.NewAggregate(localObjectErrs); err != nil {
		return err
	}

	scRemoteSelector := naming.ScyllaDBClusterRemoteSelector(sc)

	// OS Operator rewrites ScyllaDBDatacenter labels into managed Service objects, and
	// Kubernetes controller reconciling Endpoints for Services rewrites them to Endpoints.
	// As a result, we can't distinguish Endpoints managed by us from these managed by Kubernetes.
	// To overcome this, the selector we use for Endpoints is a superset of the selector of other managed objects.
	scRemoteEndpointsSelector := naming.ScyllaDBClusterRemoteEndpointsSelector(sc)

	// Operator reconciles objects in remote Kubernetes clusters, hence we can't set up a OwnerReference to ScyllaDBCluster
	// because it's not there. Instead, we will manage dependent object ownership via a RemoteOwner.
	type remoteCT = *scyllav1alpha1.RemoteOwner
	var objectErrMaps map[string][]error

	remoteClusterNames := oslices.ConvertSlice(sc.Spec.Datacenters, func(dc scyllav1alpha1.ScyllaDBClusterDatacenter) string {
		return dc.RemoteKubernetesClusterName
	})

	remoteNamespaceMap, errMap := scc.getRemoteNamespacesMap(sc)
	for remoteClusterName, err := range errMap {
		objectErrMaps[remoteClusterName] = append(objectErrMaps[remoteClusterName], fmt.Errorf("cant get remote namespaces for %q remote cluster: %w", remoteClusterName, err))
	}

	remoteNamespaces := scc.chooseRemoteNamespaces(sc, remoteNamespaceMap)

	remoteRemoteOwnerMap, errMap := scc.getRemoteRemoteOwners(sc, remoteNamespaces)
	for remoteClusterName, err := range errMap {
		objectErrMaps[remoteClusterName] = append(objectErrMaps[remoteClusterName], fmt.Errorf("can't get remote remoteowners for %q remote cluster: %w", remoteClusterName, err))
	}

	remoteControllers := scc.chooseRemoteControllers(sc, remoteRemoteOwnerMap)

	remoteServiceMap, errMap := controllerhelpers.GetRemoteObjects[remoteCT, *corev1.Service](ctx, remoteClusterNames, remoteControllers, remoteControllerGVK, scRemoteSelector, &controllerhelpers.ClusterControlleeManagerGetObjectsFuncs[remoteCT, *corev1.Service]{
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
	for remoteClusterName, err := range errMap {
		objectErrMaps[remoteClusterName] = append(objectErrMaps[remoteClusterName], fmt.Errorf("can't get remote services for %q remote cluster: %w", remoteClusterName, err))
	}

	remoteEndpointSlicesMap, errMap := controllerhelpers.GetRemoteObjects[remoteCT, *discoveryv1.EndpointSlice](ctx, remoteClusterNames, remoteControllers, remoteControllerGVK, scRemoteSelector, &controllerhelpers.ClusterControlleeManagerGetObjectsFuncs[remoteCT, *discoveryv1.EndpointSlice]{
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
	for remoteClusterName, err := range errMap {
		objectErrMaps[remoteClusterName] = append(objectErrMaps[remoteClusterName], fmt.Errorf("can't get remote endpointslices for %q remote cluster: %w", remoteClusterName, err))
	}

	// GKE DNS doesn't understand EndpointSlices hence we have to reconcile both Endpoints and EndpointSlices.
	// https://github.com/kubernetes/kubernetes/issues/107742
	remoteEndpointsMap, errMap := controllerhelpers.GetRemoteObjects[remoteCT, *corev1.Endpoints](ctx, remoteClusterNames, remoteControllers, remoteControllerGVK, scRemoteEndpointsSelector, &controllerhelpers.ClusterControlleeManagerGetObjectsFuncs[remoteCT, *corev1.Endpoints]{
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
	for remoteClusterName, err := range errMap {
		objectErrMaps[remoteClusterName] = append(objectErrMaps[remoteClusterName], fmt.Errorf("can't get remote endpoints for %q remote cluster: %w", remoteClusterName, err))
	}

	remoteConfigMapMap, errMap := controllerhelpers.GetRemoteObjects[remoteCT, *corev1.ConfigMap](ctx, remoteClusterNames, remoteControllers, remoteControllerGVK, scRemoteSelector, &controllerhelpers.ClusterControlleeManagerGetObjectsFuncs[remoteCT, *corev1.ConfigMap]{
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
	for remoteClusterName, err := range errMap {
		objectErrMaps[remoteClusterName] = append(objectErrMaps[remoteClusterName], fmt.Errorf("can't get remote configmaps for %q remote cluster: %w", remoteClusterName, err))
	}

	remoteSecretMap, errMap := controllerhelpers.GetRemoteObjects[remoteCT, *corev1.Secret](ctx, remoteClusterNames, remoteControllers, remoteControllerGVK, scRemoteSelector, &controllerhelpers.ClusterControlleeManagerGetObjectsFuncs[remoteCT, *corev1.Secret]{
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
	for remoteClusterName, err := range errMap {
		objectErrMaps[remoteClusterName] = append(objectErrMaps[remoteClusterName], fmt.Errorf("can't get remote secrets for %q remote cluster: %w", remoteClusterName, err))
	}

	remoteScyllaDBDatacenterMap, errMap := controllerhelpers.GetRemoteObjects[remoteCT, *scyllav1alpha1.ScyllaDBDatacenter](ctx, remoteClusterNames, remoteControllers, remoteControllerGVK, scRemoteSelector, &controllerhelpers.ClusterControlleeManagerGetObjectsFuncs[remoteCT, *scyllav1alpha1.ScyllaDBDatacenter]{
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
	for remoteClusterName, err := range errMap {
		objectErrMaps[remoteClusterName] = append(objectErrMaps[remoteClusterName], fmt.Errorf("can't get remote scylladbdatacenters for %q remote cluster: %w", remoteClusterName, err))
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

	type remoteNamespacedOwnedResourceSyncParameters struct {
		kind                 string
		progressingCondition string
		degradedCondition    string
		syncFn               func(remoteNamespace *corev1.Namespace, remoteController metav1.Object) ([]metav1.Condition, error)
	}

	var errs []error

	for _, dc := range sc.Spec.Datacenters {
		objectErrs := objectErrMaps[dc.RemoteKubernetesClusterName]

		var err error
		err = apimachineryutilerrors.NewAggregate(objectErrs)
		if err != nil {
			errs = append(errs, fmt.Errorf("can't sync remote datacenter %q: %w", dc.RemoteKubernetesClusterName, err))
			continue
		}

		err = controllerhelpers.RunSync(
			&status.Conditions,
			makeRemoteNamespaceControllerDatacenterProgressingCondition(dc.Name),
			makeRemoteNamespaceControllerDatacenterDegradedCondition(dc.Name),
			sc.Generation,
			func() ([]metav1.Condition, error) {
				return scc.syncRemoteNamespaces(ctx, sc, &dc, remoteNamespaceMap[dc.RemoteKubernetesClusterName], managingClusterDomain)
			},
		)
		if err != nil {
			errs = append(errs, fmt.Errorf("can't sync remote namespaces: %w", err))
		}

		// RemoteOwner resource is namespaced but not owned by anyone. It becomes our controllerRef handle for other remotely reconciled objects.
		err = controllerhelpers.RunSync(
			&status.Conditions,
			makeRemoteRemoteOwnerControllerDatacenterProgressingCondition(dc.Name),
			makeRemoteRemoteOwnerControllerDatacenterDegradedCondition(dc.Name),
			sc.Generation,
			func() ([]metav1.Condition, error) {
				var progressingConditions []metav1.Condition

				remoteNamespace := remoteNamespaces[dc.RemoteKubernetesClusterName]
				if remoteNamespace == nil {
					progressingConditions = append(progressingConditions, metav1.Condition{
						Type:               makeRemoteRemoteOwnerControllerDatacenterProgressingCondition(dc.Name),
						Status:             metav1.ConditionTrue,
						Reason:             "WaitingForRemoteNamespace",
						Message:            fmt.Sprintf("Waiting for Namespace to be created in %q Cluster", dc.RemoteKubernetesClusterName),
						ObservedGeneration: sc.Generation,
					})
					return progressingConditions, nil
				}

				return scc.syncRemoteRemoteOwners(ctx, sc, &dc, remoteNamespace, remoteRemoteOwnerMap[dc.RemoteKubernetesClusterName], managingClusterDomain)
			},
		)
		if err != nil {
			errs = append(errs, fmt.Errorf("can't sync remote remoteowners: %w", err))
		}

		remoteNamespacedOwnedSyncParams := []remoteNamespacedOwnedResourceSyncParameters{
			{
				kind:                 "Service",
				progressingCondition: makeRemoteServiceControllerDatacenterProgressingCondition(dc.Name),
				degradedCondition:    makeRemoteServiceControllerDatacenterDegradedCondition(dc.Name),
				syncFn: func(remoteNamespace *corev1.Namespace, remoteController metav1.Object) ([]metav1.Condition, error) {
					return scc.syncRemoteServices(ctx, sc, &dc, remoteNamespace, remoteController, remoteServiceMap[dc.RemoteKubernetesClusterName], managingClusterDomain)
				},
			},
			{
				kind:                 "EndpointSlice",
				progressingCondition: makeRemoteEndpointSliceControllerDatacenterProgressingCondition(dc.Name),
				degradedCondition:    makeRemoteEndpointSliceControllerDatacenterDegradedCondition(dc.Name),
				syncFn: func(remoteNamespace *corev1.Namespace, remoteController metav1.Object) ([]metav1.Condition, error) {
					return scc.syncRemoteEndpointSlices(ctx, sc, &dc, remoteNamespace, remoteController, remoteEndpointSlicesMap[dc.RemoteKubernetesClusterName], remoteNamespaces, managingClusterDomain)
				},
			},
			{
				kind:                 "Endpoints",
				progressingCondition: makeRemoteEndpointsControllerDatacenterProgressingCondition(dc.Name),
				degradedCondition:    makeRemoteEndpointsControllerDatacenterDegradedCondition(dc.Name),
				syncFn: func(remoteNamespace *corev1.Namespace, remoteController metav1.Object) ([]metav1.Condition, error) {
					return scc.syncRemoteEndpoints(ctx, sc, &dc, remoteNamespace, remoteController, remoteEndpointsMap[dc.RemoteKubernetesClusterName], remoteNamespaces, managingClusterDomain)
				},
			},
			{
				kind:                 "ConfigMap",
				progressingCondition: makeRemoteConfigMapControllerDatacenterProgressingCondition(dc.Name),
				degradedCondition:    makeRemoteConfigMapControllerDatacenterDegradedCondition(dc.Name),
				syncFn: func(remoteNamespace *corev1.Namespace, remoteController metav1.Object) ([]metav1.Condition, error) {
					return scc.syncRemoteConfigMaps(ctx, sc, &dc, remoteNamespace, remoteController, remoteConfigMapMap[dc.RemoteKubernetesClusterName], managingClusterDomain)
				},
			},
			{
				kind:                 "Secret",
				progressingCondition: makeRemoteSecretControllerDatacenterProgressingCondition(dc.Name),
				degradedCondition:    makeRemoteSecretControllerDatacenterDegradedCondition(dc.Name),
				syncFn: func(remoteNamespace *corev1.Namespace, remoteController metav1.Object) ([]metav1.Condition, error) {
					return scc.syncRemoteSecrets(ctx, sc, &dc, remoteNamespace, remoteController, remoteSecretMap[dc.RemoteKubernetesClusterName], managingClusterDomain)
				},
			},
			{
				kind:                 "ScyllaDBDatacenter",
				progressingCondition: makeRemoteScyllaDBDatacenterControllerDatacenterProgressingCondition(dc.Name),
				degradedCondition:    makeRemoteScyllaDBDatacenterControllerDatacenterDegradedCondition(dc.Name),
				syncFn: func(remoteNamespace *corev1.Namespace, remoteController metav1.Object) ([]metav1.Condition, error) {
					return scc.syncRemoteScyllaDBDatacenters(ctx, sc, &dc, status, remoteNamespace, remoteController, remoteScyllaDBDatacenterMap, managingClusterDomain)
				},
			},
		}

		for _, syncParams := range remoteNamespacedOwnedSyncParams {
			err := controllerhelpers.SyncRemoteNamespacedObject(
				&status.Conditions,
				syncParams.progressingCondition,
				syncParams.degradedCondition,
				sc.Generation,
				dc.RemoteKubernetesClusterName,
				remoteNamespaces[dc.RemoteKubernetesClusterName],
				remoteControllers[dc.RemoteKubernetesClusterName],
				syncParams.syncFn,
			)
			if err != nil {
				errs = append(errs, fmt.Errorf("can't sync remote %s: %w", syncParams.kind, err))
			}
		}

		// Aggregate datacenter conditions.
		err = controllerhelpers.SetAggregatedWorkloadConditionsBySuffixes(
			internalapi.MakeDatacenterAvailableCondition(dc.Name),
			internalapi.MakeDatacenterProgressingCondition(dc.Name),
			internalapi.MakeDatacenterDegradedCondition(dc.Name),
			&status.Conditions,
			sc.Generation,
		)
		if err != nil {
			errs = append(errs, fmt.Errorf("can't aggregate datacenter %q workload conditions: %w", dc.Name, err))
		}
	}

	localSyncParameters := []struct {
		kind                 string
		progressingCondition string
		degradedCondition    string
		syncFn               func() ([]metav1.Condition, error)
	}{
		{
			kind:                 "Service",
			progressingCondition: serviceControllerProgressingCondition,
			degradedCondition:    serviceControllerDegradedCondition,
			syncFn: func() ([]metav1.Condition, error) {
				return scc.syncLocalServices(ctx, sc, localServiceMap)
			},
		},
		{
			kind:                 "EndpointSlice",
			progressingCondition: endpointSliceControllerProgressingCondition,
			degradedCondition:    endpointSliceControllerDegradedCondition,
			syncFn: func() ([]metav1.Condition, error) {
				return scc.syncLocalEndpointSlices(ctx, sc, localEndpointSlicesMap, remoteNamespaces)
			},
		},
		{
			kind:                 "Endpoints",
			progressingCondition: endpointsControllerProgressingCondition,
			degradedCondition:    endpointsControllerDegradedCondition,
			syncFn: func() ([]metav1.Condition, error) {
				return scc.syncLocalEndpoints(ctx, sc, localEndpointsMap, remoteNamespaces)
			},
		},
	}

	for _, syncParams := range localSyncParameters {
		err = controllerhelpers.RunSync(
			&status.Conditions,
			syncParams.progressingCondition,
			syncParams.degradedCondition,
			sc.Generation,
			syncParams.syncFn,
		)
		if err != nil {
			errs = append(errs, fmt.Errorf("can't sync local %s: %w", syncParams.kind, err))
		}
	}

	// Aggregate conditions.
	err = controllerhelpers.SetAggregatedWorkloadConditions(&status.Conditions, sc.Generation)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't aggregate workload conditions: %w", err))
	} else {
		err = scc.updateStatus(ctx, sc, status)
		errs = append(errs, err)
	}

	return apimachineryutilerrors.NewAggregate(errs)
}

func (scc *Controller) chooseRemoteControllers(sc *scyllav1alpha1.ScyllaDBCluster, remoteRemoteOwnersMap map[string]map[string]*scyllav1alpha1.RemoteOwner) map[string]metav1.Object {
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

	return remoteControllers
}

func (scc *Controller) getRemoteRemoteOwners(sc *scyllav1alpha1.ScyllaDBCluster, remoteNamespaces map[string]*corev1.Namespace) (map[string]map[string]*scyllav1alpha1.RemoteOwner, map[string]error) {
	remoteOwnerMap := make(map[string]map[string]*scyllav1alpha1.RemoteOwner, len(sc.Spec.Datacenters))
	errMap := make(map[string]error, len(sc.Spec.Datacenters))
	for _, dc := range sc.Spec.Datacenters {
		ns, ok := remoteNamespaces[dc.RemoteKubernetesClusterName]
		if !ok {
			// Might not exist yet
			continue
		}

		selector := labels.SelectorFromSet(naming.RemoteOwnerSelectorLabels(sc, &dc))
		remoteOwners, err := scc.remoteRemoteOwnerLister.Cluster(dc.RemoteKubernetesClusterName).RemoteOwners(ns.Name).List(selector)
		if err != nil {
			errMap[dc.RemoteKubernetesClusterName] = fmt.Errorf("can't list remote remoteowners in %q cluster: %w", dc.RemoteKubernetesClusterName, err)
			continue
		}

		roMap := make(map[string]*scyllav1alpha1.RemoteOwner, len(remoteOwners))

		for _, ro := range remoteOwners {
			if len(ro.OwnerReferences) > 0 {
				errMap[dc.RemoteKubernetesClusterName] = fmt.Errorf("unexpected RemoteOwner %q matching our selector and having an OwnerReference in %q cluster", naming.ObjRef(ro), dc.RemoteKubernetesClusterName)
				continue
			}

			roMap[ro.Name] = ro
		}

		remoteOwnerMap[dc.RemoteKubernetesClusterName] = roMap
	}

	return remoteOwnerMap, errMap
}

// chooseRemoteNamespaces returns a map of namespaces used for reconciled objects per each remote.
func (scc *Controller) chooseRemoteNamespaces(sc *scyllav1alpha1.ScyllaDBCluster, remoteNamespacesMap map[string]map[string]*corev1.Namespace) map[string]*corev1.Namespace {
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

	return remoteNamespaces
}

// getRemoteNamespacesMap returns a map of remote namespaces matching provided selector.
func (scc *Controller) getRemoteNamespacesMap(sc *scyllav1alpha1.ScyllaDBCluster) (map[string]map[string]*corev1.Namespace, map[string]error) {
	namespacesMap := make(map[string]map[string]*corev1.Namespace, len(sc.Spec.Datacenters))
	errMap := make(map[string]error, len(sc.Spec.Datacenters))
	for _, dc := range sc.Spec.Datacenters {
		selector := labels.SelectorFromSet(naming.ScyllaDBClusterDatacenterSelectorLabels(sc, &dc))
		remoteNamespaces, err := scc.remoteNamespaceLister.Cluster(dc.RemoteKubernetesClusterName).List(selector)
		if err != nil {
			errMap[dc.RemoteKubernetesClusterName] = fmt.Errorf("can't list remote namespaces in %q cluster: %w", dc.RemoteKubernetesClusterName, err)
			continue
		}

		nsMap := make(map[string]*corev1.Namespace, len(remoteNamespaces))
		for _, rns := range remoteNamespaces {
			if len(rns.OwnerReferences) != 0 {
				errMap[dc.RemoteKubernetesClusterName] = fmt.Errorf("unexpected Namespace %q matching our selector and having an OwnerReference in %q cluster", rns.Name, dc.RemoteKubernetesClusterName)
				continue
			}

			nsMap[rns.Name] = rns
		}

		namespacesMap[dc.RemoteKubernetesClusterName] = nsMap
	}

	return namespacesMap, errMap
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
