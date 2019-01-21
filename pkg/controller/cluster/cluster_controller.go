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
	"github.com/pkg/errors"
	"github.com/prometheus/common/log"
	"github.com/scylladb/scylla-operator/cmd/options"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/apis/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controller/cluster/util"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const concurrency = 1

// Add creates a new Cluster Controller and adds it to the Manager with default RBAC.
// The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {

	kubeClient := kubernetes.NewForConfigOrDie(mgr.GetConfig())
	uncachedClient, err := client.New(mgr.GetConfig(), client.Options{})
	if err != nil {
		log.Fatalf("Failed to get dynamic uncached client: %+v", err)
	}
	return &ClusterController{
		Client:        mgr.GetClient(),
		UncachedClient: uncachedClient,
		KubeClient:    kubeClient,
		Recorder:      mgr.GetRecorder("scylla-cluster-controller"),
		OperatorImage: getOperatorImage(kubeClient),
		scheme:        mgr.GetScheme(),
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
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
			oldCluster := e.ObjectOld.(*scyllav1alpha1.Cluster)
			newCluster := e.ObjectNew.(*scyllav1alpha1.Cluster)
			if reflect.DeepEqual(oldCluster, newCluster) {
				return false
			}
			return true
		},
	}

	err = c.Watch(
		&source.Kind{Type: &scyllav1alpha1.Cluster{}},
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
			OwnerType:    &scyllav1alpha1.Cluster{},
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
			OwnerType:    &scyllav1alpha1.Cluster{},
		},
		predicate.ResourceVersionChangedPredicate{},
	)
	if err != nil {
		return errors.Wrap(err, "services watch setup failed")
	}

	return nil
}

func getOperatorImage(kubeClient kubernetes.Interface) string {

	opts := options.GetControllerOptions()
	if opts.Image != "" {
		return opts.Image
	}

	pod, err := kubeClient.CoreV1().Pods(opts.Namespace).Get(opts.Name, metav1.GetOptions{})
	if err != nil {
		log.Fatalf("Failed to get operator image: %+v", err)
	}

	if len(pod.Spec.Containers) != 1 {
		log.Fatalf("Operator Pod must have exactly 1 container, found %d", len(pod.Spec.Containers))
	}
	return pod.Spec.Containers[0].Image
}

var _ reconcile.Reconciler = &ClusterController{}

// ClusterController reconciles a Cluster object
type ClusterController struct {
	client.Client
	UncachedClient client.Client
	Recorder      record.EventRecorder
	OperatorImage string
	// Original k8s client needed for patch ops
	// Will replace once the dynamic client adds this feature
	// https://github.com/kubernetes-sigs/controller-runtime/pull/235
	// Feature depends on server-side apply, landing in 1.14
	// https://github.com/kubernetes/enhancements/issues/555
	KubeClient kubernetes.Interface
	scheme     *runtime.Scheme
}

// Reconcile reads that state of the cluster for a Cluster object and makes changes based on the state read
// and what is in the Cluster.Spec
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=,resources=pods,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups=,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=,resources=persistentvolumeclaims,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups=,resources=events,verbs=create;update;patch
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=scylla.scylladb.com,resources=clusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=scylla.scylladb.com,resources=clusters/status,verbs=update
func (cc *ClusterController) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the Cluster instance
	c := &scyllav1alpha1.Cluster{}
	err := cc.UncachedClient.Get(context.TODO(), request.NamespacedName, c)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{Requeue: true}, err
	}
	logger := util.LoggerForCluster(c)
	copy := c.DeepCopy()
	if err = cc.sync(copy); err != nil {
		logger.Error("An error occured during cluster reconciliation. Printing stacktrace:")
		logger.Errorf("%+v", err)
		return reconcile.Result{}, errors.Wrap(err, "sync failed")
	}

	// Update status if needed
	// If there's a change in the status, update it
	if !reflect.DeepEqual(c.Status, copy.Status) {
		logger.Info("Writing cluster status.")
		err = cc.Status().Update(context.TODO(), copy)
		if err != nil {
			logger.Errorf("Failed to update cluster status: %+v", err)
			return reconcile.Result{}, errors.WithStack(err)
		}
	}

	logger.Info("Successful reconciliation")
	return reconcile.Result{}, nil
}
