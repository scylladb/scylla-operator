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
	"reflect"
	"strings"

	"github.com/scylladb/go-log"
	"github.com/scylladb/scylla-operator/pkg/cmd/scylla-operator/options"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/v1"
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

// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;delete;update
// +kubebuilder:rbac:groups="",resources=persistentvolumes,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;update;patch
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=scylla.scylladb.com,resources=scyllaclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=scylla.scylladb.com,resources=scyllaclusters/status,verbs=update;get;patch

func (cc *ClusterReconciler) Reconcile(request ctrl.Request) (ctrl.Result, error) {
	ctx := log.WithNewTraceID(context.Background())
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
	copy := c.DeepCopy()
	if err = cc.sync(copy); err != nil {
		logger.Error(ctx, "An error occurred during cluster reconciliation", "error", err)
		return reconcile.Result{}, errors.Wrap(err, "sync failed")
	}

	// Update status if needed
	// If there's a change in the status, update it
	if !reflect.DeepEqual(c.Status, copy.Status) {
		logger.Info(ctx, "Writing cluster status.")
		if err := cc.Status().Update(ctx, copy); err != nil {
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
		return errors.Wrap(err, "controller creation failed")
	}

	//////////////////////////////////
	// Watch for changes to Cluster //
	//////////////////////////////////
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
		return errors.Wrap(err, "cluster watch setup failed")
	}

	/////////////////////////////////////////////
	// Watch StatefulSets created by a Cluster //
	/////////////////////////////////////////////

	err = c.Watch(
		&source.Kind{Type: &appsv1.StatefulSet{}},
		&handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &scyllav1.ScyllaCluster{},
		},
		predicate.ResourceVersionChangedPredicate{},
	)
	if err != nil {
		return errors.Wrap(err, "statefulset watch setup failed")
	}

	/////////////////////////////////////////
	// Watch Services created by a Cluster //
	/////////////////////////////////////////

	err = c.Watch(
		&source.Kind{Type: &corev1.Service{}},
		&handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &scyllav1.ScyllaCluster{},
		},
		predicate.ResourceVersionChangedPredicate{},
	)
	if err != nil {
		return errors.Wrap(err, "services watch setup failed")
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
