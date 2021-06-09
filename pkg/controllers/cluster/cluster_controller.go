/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cluster

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/pkg/errors"
	"github.com/scylladb/go-log"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/cmd/operator/options"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/policy/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const concurrency = 1

// ClusterReconciler reconciles a Cluster object
type ClusterReconciler struct {
	client.Client

	// Original k8s client needed for patch ops
	// Will replace once the dynamic client adds this feature
	// https://github.com/kubernetes-sigs/controller-runtime/pull/235
	// Feature depends on server-side apply, landing in 1.14
	// https://github.com/kubernetes/enhancements/issues/555
	KubeClient     kubernetes.Interface
	UncachedClient client.Client
	Recorder       record.EventRecorder
	OperatorImage  string

	Scheme *runtime.Scheme
	Logger log.Logger
}

var _ reconcile.Reconciler = &ClusterReconciler{}

func New(ctx context.Context, mgr ctrl.Manager, logger log.Logger) (*ClusterReconciler, error) {
	kubeClient := kubernetes.NewForConfigOrDie(mgr.GetConfig())
	uncachedClient, err := client.New(mgr.GetConfig(), client.Options{
		Scheme: mgr.GetScheme(),
	})
	if err != nil {
		return nil, errors.Wrap(err, "get dynamic uncached client")
	}

	operatorImage, err := GetOperatorImage(ctx, kubeClient, options.GetOperatorOptions())
	if err != nil {
		return nil, errors.Wrap(err, "get operator image")
	}

	return &ClusterReconciler{
		Client:         mgr.GetClient(),
		UncachedClient: uncachedClient,
		KubeClient:     kubeClient,
		Recorder:       mgr.GetEventRecorderFor("scylla-cluster-controller"),
		OperatorImage:  operatorImage,
		Scheme:         mgr.GetScheme(),
		Logger:         logger,
	}, nil
}

func (cc *ClusterReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	ctx = log.WithNewTraceID(ctx)
	cc.Logger.Debug(ctx, "Reconcile request", "request", request.String())
	// Fetch the Cluster instance
	c := &scyllav1.ScyllaCluster{}
	err := cc.UncachedClient.Get(ctx, request.NamespacedName, c)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			cc.Logger.Debug(ctx, "Cluster not found", "namespace", request.NamespacedName)
			return reconcile.Result{}, nil
		}
		cc.Logger.Debug(ctx, "Error during getting clusters", "error", err)
		// Error reading the object - requeue the request.
		return reconcile.Result{Requeue: true}, err
	}

	logger := cc.Logger.With("cluster", c.Namespace+"/"+c.Name, "resourceVersion", c.ResourceVersion)
	clusterCopy := c.DeepCopy()
	if c.DeletionTimestamp != nil {
		logger.Info(ctx, "Updating cluster status for deleted cluster")
		if err := cc.updateStatus(ctx, clusterCopy); err != nil {
			cc.Recorder.Event(c, corev1.EventTypeWarning, naming.ErrSyncFailed, fmt.Sprintf(MessageUpdateStatusFailed, err))
			return reconcile.Result{Requeue: true}, errors.Wrap(err, "failed to update status")
		}
	} else {
		if err = cc.sync(clusterCopy); err != nil {
			logger.Error(ctx, "An error occurred during cluster reconciliation", "error", err)
			return reconcile.Result{}, errors.Wrap(err, "sync failed")
		}
	}

	// Update status if needed
	// If there's a change in the status, update it
	if !reflect.DeepEqual(c.Status, clusterCopy.Status) {
		logger.Info(ctx, "Writing cluster status.")
		if err := cc.Status().Update(ctx, clusterCopy); err != nil {
			if apierrors.IsConflict(err) {
				logger.Info(ctx, "Failed to update cluster status", "error", err)
			} else {
				logger.Error(ctx, "Failed to update cluster status", "error", err)
			}
			return reconcile.Result{}, errors.WithStack(err)
		}
	}

	logger.Info(ctx, "Reconciliation successful")
	return reconcile.Result{}, nil
}

func (r *ClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Create a new controller
	c, err := controller.New("cluster-controller", mgr, controller.Options{
		Reconciler:              r,
		MaxConcurrentReconciles: concurrency,
	})
	if err != nil {
		return errors.Wrap(err, "controller creation")
	}

	// Watch for changes to ScyllaCluster
	clusterSpecChangedPredicate := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldCluster := e.ObjectOld.(*scyllav1.ScyllaCluster)
			newCluster := e.ObjectNew.(*scyllav1.ScyllaCluster)
			if reflect.DeepEqual(oldCluster, newCluster) {
				return false
			}
			return true
		},
	}

	err = c.Watch(
		&source.Kind{Type: &scyllav1.ScyllaCluster{}},
		&handler.EnqueueRequestForObject{},
		predicate.ResourceVersionChangedPredicate{},
		clusterSpecChangedPredicate,
	)
	if err != nil {
		return errors.Wrap(err, "cluster watch setup")
	}

	// Watch StatefulSets created by a ScyllaCluster
	err = c.Watch(
		&source.Kind{Type: &appsv1.StatefulSet{}},
		&handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &scyllav1.ScyllaCluster{},
		},
		predicate.ResourceVersionChangedPredicate{},
	)
	if err != nil {
		return errors.Wrap(err, "statefulset watch setup")
	}

	// Watch Services created by a ScyllaCluster.
	err = c.Watch(
		&source.Kind{Type: &corev1.Service{}},
		&handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &scyllav1.ScyllaCluster{},
		},
		predicate.ResourceVersionChangedPredicate{},
	)
	if err != nil {
		return errors.Wrap(err, "services watch setup")
	}

	// Watch PDBs created for ScyllaCluster
	err = c.Watch(
		&source.Kind{Type: &v1beta1.PodDisruptionBudget{}},
		&handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &scyllav1.ScyllaCluster{},
		},
		predicate.ResourceVersionChangedPredicate{},
	)
	if err != nil {
		return errors.Wrap(err, "pod disruption budget watch setup")
	}

	// Watch Pods created by a STS
	scyllaOnlyPods := func(meta metav1.Object) bool {
		if meta.GetLabels()["app.kubernetes.io/managed-by"] != "scylla-operator" {
			return false
		}
		return true
	}
	scyllaOnlyPodsPredicate := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return scyllaOnlyPods(e.Object.(metav1.Object))
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return scyllaOnlyPods(e.ObjectNew.(metav1.Object))
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return scyllaOnlyPods(e.Object.(metav1.Object))
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return scyllaOnlyPods(e.Object.(metav1.Object))
		},
	}

	err = c.Watch(
		&source.Kind{Type: &corev1.Pod{}},
		&EnqueueRequestForPod{
			KubeClient: r.KubeClient,
		},
		predicate.ResourceVersionChangedPredicate{},
		scyllaOnlyPodsPredicate,
	)
	if err != nil {
		return errors.Wrap(err, "pods watch setup failed")
	}

	// Watch Secrets created for ScyllaCluster
	err = c.Watch(
		&source.Kind{Type: &corev1.Secret{}},
		&handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &scyllav1.ScyllaCluster{},
		},
		predicate.ResourceVersionChangedPredicate{},
	)
	if err != nil {
		return errors.Wrap(err, "secret watch setup")
	}

	return nil
}

func GetOperatorImage(ctx context.Context, kubeClient kubernetes.Interface, opts *options.OperatorOptions) (string, error) {
	if opts.Image != "" {
		return opts.Image, nil
	}

	pod, err := kubeClient.CoreV1().Pods(opts.Namespace).Get(ctx, opts.Name, metav1.GetOptions{})
	if err != nil {
		return "", errors.Wrap(err, "list pods")
	}

	// Scylla Operator image must contain two words: "scylla" and "operator"
	for _, c := range pod.Spec.Containers {
		img := strings.ToLower(c.Image)
		if strings.Contains(img, "scylla") && strings.Contains(img, "operator") {
			return c.Image, nil
		}
	}

	return "", errors.New("cannot find scylla operator container in pod spec")
}
