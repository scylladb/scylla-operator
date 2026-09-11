package scylladbdatacenter

import (
	"context"
	"fmt"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/controllertools"
	"github.com/scylladb/scylla-operator/pkg/ctrlclient"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func (sdcc *Controller) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	key := req.NamespacedName
	rq := &controllertools.Requeue{}

	namespace, name := key.Namespace, key.Name

	startTime := time.Now()
	klog.V(4).InfoS("Started syncing ScyllaCluster", "ScyllaDBDatacenter", klog.KRef(namespace, name), "startTime", startTime)
	defer func() {
		klog.V(4).InfoS("Finished syncing ScyllaCluster", "ScyllaDBDatacenter", klog.KRef(namespace, name), "duration", time.Since(startTime))
	}()

	sdc, err := ctrlclient.Get[scyllav1alpha1.ScyllaDBDatacenter](ctx, sdcc.client, namespace, name)
	if errors.IsNotFound(err) {
		klog.V(2).InfoS("ScyllaCluster has been deleted", "ScyllaDBDatacenter", klog.KObj(sdc))
		return rq.Result(), nil
	}
	if err != nil {
		return rq.Result(), err
	}

	soc, err := ctrlclient.Get[scyllav1alpha1.ScyllaOperatorConfig](ctx, sdcc.client, "", naming.SingletonName)
	if err != nil {
		return rq.Result(), fmt.Errorf("can't get ScyllaOperatorConfig %q: %w", naming.SingletonName, err)
	}

	sdcSelector := labels.SelectorFromSet(labels.Set{
		naming.ClusterNameLabel: sdc.Name,
	})

	type CT = *scyllav1alpha1.ScyllaDBDatacenter
	var objectErrs []error

	statefulSetMap, err := controllerhelpers.GetObjects[CT, *appsv1.StatefulSet](
		ctx,
		sdc,
		scyllav1alpha1.ScyllaDBDatacenterGVK,
		sdcSelector,
		ctrlclient.GetObjectsControl[scyllav1alpha1.ScyllaDBDatacenter, appsv1.StatefulSet](ctx, sdcc.client, sdcc.apiReader, sdc.Namespace),
	)
	if err != nil {
		objectErrs = append(objectErrs, err)
	}

	serviceMap, err := controllerhelpers.GetObjects[CT, *corev1.Service](
		ctx,
		sdc,
		scyllav1alpha1.ScyllaDBDatacenterGVK,
		sdcSelector,
		ctrlclient.GetObjectsControl[scyllav1alpha1.ScyllaDBDatacenter, corev1.Service](ctx, sdcc.client, sdcc.apiReader, sdc.Namespace),
	)
	if err != nil {
		objectErrs = append(objectErrs, err)
	}

	secretMap, err := controllerhelpers.GetObjects[CT, *corev1.Secret](
		ctx,
		sdc,
		scyllav1alpha1.ScyllaDBDatacenterGVK,
		sdcSelector,
		ctrlclient.GetObjectsControl[scyllav1alpha1.ScyllaDBDatacenter, corev1.Secret](ctx, sdcc.client, sdcc.apiReader, sdc.Namespace),
	)
	if err != nil {
		objectErrs = append(objectErrs, err)
	}

	configMapMap, err := controllerhelpers.GetObjects[CT, *corev1.ConfigMap](
		ctx,
		sdc,
		scyllav1alpha1.ScyllaDBDatacenterGVK,
		sdcSelector,
		ctrlclient.GetObjectsControl[scyllav1alpha1.ScyllaDBDatacenter, corev1.ConfigMap](ctx, sdcc.client, sdcc.apiReader, sdc.Namespace),
	)
	if err != nil {
		objectErrs = append(objectErrs, err)
	}

	serviceAccounts, err := controllerhelpers.GetObjects[CT, *corev1.ServiceAccount](
		ctx,
		sdc,
		scyllav1alpha1.ScyllaDBDatacenterGVK,
		sdcSelector,
		ctrlclient.GetObjectsControl[scyllav1alpha1.ScyllaDBDatacenter, corev1.ServiceAccount](ctx, sdcc.client, sdcc.apiReader, sdc.Namespace),
	)
	if err != nil {
		objectErrs = append(objectErrs, err)
	}

	roleBindings, err := controllerhelpers.GetObjects[CT, *rbacv1.RoleBinding](
		ctx,
		sdc,
		scyllav1alpha1.ScyllaDBDatacenterGVK,
		sdcSelector,
		ctrlclient.GetObjectsControl[scyllav1alpha1.ScyllaDBDatacenter, rbacv1.RoleBinding](ctx, sdcc.client, sdcc.apiReader, sdc.Namespace),
	)
	if err != nil {
		objectErrs = append(objectErrs, err)
	}

	pdbMap, err := controllerhelpers.GetObjects[CT, *policyv1.PodDisruptionBudget](
		ctx,
		sdc,
		scyllav1alpha1.ScyllaDBDatacenterGVK,
		sdcSelector,
		ctrlclient.GetObjectsControl[scyllav1alpha1.ScyllaDBDatacenter, policyv1.PodDisruptionBudget](ctx, sdcc.client, sdcc.apiReader, sdc.Namespace),
	)
	if err != nil {
		objectErrs = append(objectErrs, err)
	}

	ingressMap, err := controllerhelpers.GetObjects[CT, *networkingv1.Ingress](
		ctx,
		sdc,
		scyllav1alpha1.ScyllaDBDatacenterGVK,
		sdcSelector,
		ctrlclient.GetObjectsControl[scyllav1alpha1.ScyllaDBDatacenter, networkingv1.Ingress](ctx, sdcc.client, sdcc.apiReader, sdc.Namespace),
	)
	if err != nil {
		objectErrs = append(objectErrs, err)
	}

	jobMap, err := controllerhelpers.GetObjects[CT, *batchv1.Job](
		ctx,
		sdc,
		scyllav1alpha1.ScyllaDBDatacenterGVK,
		sdcSelector,
		ctrlclient.GetObjectsControl[scyllav1alpha1.ScyllaDBDatacenter, batchv1.Job](ctx, sdcc.client, sdcc.apiReader, sdc.Namespace),
	)
	if err != nil {
		objectErrs = append(objectErrs, err)
	}

	scyllaDBDatacenterNodesStatusReportMap, err := controllerhelpers.GetObjects[CT, *scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport](
		ctx,
		sdc,
		scyllav1alpha1.ScyllaDBDatacenterGVK,
		sdcSelector,
		ctrlclient.GetObjectsControl[scyllav1alpha1.ScyllaDBDatacenter, scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport](ctx, sdcc.client, sdcc.apiReader, sdc.Namespace),
	)
	if err != nil {
		objectErrs = append(objectErrs, err)
	}

	objectErr := apimachineryutilerrors.NewAggregate(objectErrs)
	if objectErr != nil {
		return rq.Result(), objectErr
	}

	status := sdcc.calculateStatus(ctx, sdc, statefulSetMap, serviceMap)

	if sdc.DeletionTimestamp != nil {
		return rq.Result(), sdcc.updateStatus(ctx, sdc, status)
	}

	var errs []error

	err = controllerhelpers.RunSync(
		&status.Conditions,
		serviceAccountControllerProgressingCondition,
		serviceAccountControllerDegradedCondition,
		sdc.Generation,
		func() ([]metav1.Condition, error) {
			return sdcc.syncServiceAccounts(ctx, sdc, serviceAccounts)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync service accounts: %w", err))
	}

	err = controllerhelpers.RunSync(
		&status.Conditions,
		roleBindingControllerProgressingCondition,
		roleBindingControllerDegradedCondition,
		sdc.Generation,
		func() ([]metav1.Condition, error) {
			return sdcc.syncRoleBindings(ctx, sdc, roleBindings)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync role bindings: %w", err))
	}

	err = controllerhelpers.RunSync(
		&status.Conditions,
		agentTokenControllerProgressingCondition,
		agentTokenControllerDegradedCondition,
		sdc.Generation,
		func() ([]metav1.Condition, error) {
			return sdcc.syncAgentToken(ctx, sdc, secretMap)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync agent token: %w", err))
	}

	err = controllerhelpers.RunSync(
		&status.Conditions,
		certControllerProgressingCondition,
		certControllerDegradedCondition,
		sdc.Generation,
		func() ([]metav1.Condition, error) {
			return sdcc.syncCerts(ctx, sdc, secretMap, configMapMap, serviceMap)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync certificates: %w", err))
	}

	err = controllerhelpers.RunSync(
		&status.Conditions,
		configControllerProgressingCondition,
		configControllerDegradedCondition,
		sdc.Generation,
		func() ([]metav1.Condition, error) {
			return sdcc.syncConfigs(ctx, sdc)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync configs: %w", err))
	}

	err = controllerhelpers.RunSync(
		&status.Conditions,
		scyllaDBDatacenterNodesStatusReportControllerProgressingCondition,
		scyllaDBDatacenterNodesStatusReportControllerDegradedCondition,
		sdc.Generation,
		func() ([]metav1.Condition, error) {
			return sdcc.syncScyllaDBDatacenterNodesStatusReports(ctx, sdc, serviceMap, scyllaDBDatacenterNodesStatusReportMap)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync ScyllaDBDatacenter nodes status reports: %w", err))
	}

	err = controllerhelpers.RunSync(
		&status.Conditions,
		statefulSetControllerProgressingCondition,
		statefulSetControllerDegradedCondition,
		sdc.Generation,
		func() ([]metav1.Condition, error) {
			return sdcc.syncStatefulSets(ctx, rq, sdc, soc, status, statefulSetMap, serviceMap, configMapMap)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync stateful sets: %w", err))
	}
	// Ideally, this would be projected in calculateStatus but because we are updating the status based on applied
	// StatefulSets, the rack status can change afterwards. Overtime we should consider adding a status.progressing
	// field (to allow determining cluster status without conditions) and wait for the status to be updated
	// in a single place, on the next resync.
	sdcc.setStatefulSetsAvailableStatusCondition(sdc, status)

	err = controllerhelpers.RunSync(
		&status.Conditions,
		serviceControllerProgressingCondition,
		serviceControllerDegradedCondition,
		sdc.Generation,
		func() ([]metav1.Condition, error) {
			return sdcc.syncServices(ctx, sdc, status, serviceMap, statefulSetMap, jobMap)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync services: %w", err))
	}

	err = controllerhelpers.RunSync(
		&status.Conditions,
		pdbControllerProgressingCondition,
		pdbControllerDegradedCondition,
		sdc.Generation,
		func() ([]metav1.Condition, error) {
			return sdcc.syncPodDisruptionBudgets(ctx, sdc, pdbMap)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync pdbs: %w", err))
	}

	err = controllerhelpers.RunSync(
		&status.Conditions,
		ingressControllerProgressingCondition,
		ingressControllerDegradedCondition,
		sdc.Generation,
		func() ([]metav1.Condition, error) {
			return sdcc.syncIngresses(ctx, sdc, ingressMap, serviceMap)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync ingresses: %w", err))
	}

	err = controllerhelpers.RunSync(
		&status.Conditions,
		jobControllerProgressingCondition,
		jobControllerDegradedCondition,
		sdc.Generation,
		func() ([]metav1.Condition, error) {
			return sdcc.syncJobs(ctx, sdc, serviceMap, jobMap)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync jobs: %w", err))
	}

	// Aggregate conditions.
	err = controllerhelpers.SetAggregatedWorkloadConditions(&status.Conditions, sdc.Generation)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't aggregate workload conditions: %w", err))
	} else {
		err = sdcc.updateStatus(ctx, sdc, status)
		errs = append(errs, err)
	}

	return rq.Result(), apimachineryutilerrors.NewAggregate(errs)
}
