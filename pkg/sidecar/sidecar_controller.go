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

package sidecar

import (
	"context"
	"fmt"
	"github.com/davecgh/go-spew/spew"
	"github.com/pkg/errors"
	"github.com/scylladb/scylla-operator/cmd/options"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/sidecar/config"
	"github.com/scylladb/scylla-operator/pkg/sidecar/identity"
	log "github.com/sirupsen/logrus"
	"github.com/yanniszark/go-nodetool/nodetool"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/informers/internalinterfaces"
	"k8s.io/client-go/kubernetes"
	"net/url"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"time"
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

	opts := options.GetSidecarOptions()
	kubeClient := kubernetes.NewForConfigOrDie(mgr.GetConfig())
	member, err := identity.Retrieve(opts.Name, opts.Namespace, kubeClient)
	if err != nil {
		log.Fatalf("Failed to get member: %+v", err)
	}
	log.Infof("Member: %v", spew.Sdump(member))

	url, err := url.Parse(fmt.Sprintf("http://127.0.0.1:%d/%s/", naming.JolokiaPort, naming.JolokiaContext))
	if err != nil {
		log.Fatalf("Failed to parse url: %+v", err)
	}

	client, err := client.New(mgr.GetConfig(), client.Options{})
	if err != nil {
		log.Fatalf("Error getting dynamic client: %+v", err)
	}

	mc := &MemberController{
		Client:     client,
		kubeClient: kubeClient,
		member:     member,
		scheme:     mgr.GetScheme(),
		nodetool:   nodetool.NewFromURL(url),
	}

	if err = mc.onStartup(); err != nil {
		log.Fatalf("Error occured during startup: %+v", err)
	}
	return mc
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {

	mc := r.(*MemberController)

	// Create a new controller
	c, err := controller.New("sidecar-controller", mgr, controller.Options{
		Reconciler:              r,
		MaxConcurrentReconciles: concurrency,
	})
	if err != nil {
		return errors.Wrap(err, "controller creation failed")
	}

	// Use an informer to do filtered lists for minimum memory overhead.
	// Replace this when filtered watches are supported:
	// https://github.com/kubernetes-sigs/controller-runtime/issues/244

	// This func will make our informer only watch resources with the name of our member
	nameFilteringFunc := internalinterfaces.TweakListOptionsFunc(
		func(listOpts *metav1.ListOptions) {
			listOpts.FieldSelector = fmt.Sprintf("metadata.name=%s", mc.member.Name)
		},
	)

	kubeInformerFactory := kubeinformers.NewSharedInformerFactoryWithOptions(
		mc.kubeClient,
		30*time.Second,
		kubeinformers.WithNamespace(mc.member.Namespace),
		kubeinformers.WithTweakListOptions(nameFilteringFunc),
	)

	// Instruct the manager to start the informers
	err = mgr.Add(manager.RunnableFunc(func(s <-chan struct{}) error {
		kubeInformerFactory.Start(s)
		<-s
		return nil
	}))
	if err != nil {
		return errors.Wrap(err, "failed to add informers to manager")
	}

	// Watch Service Resources
	err = c.Watch(
		&source.Informer{Informer: kubeInformerFactory.Core().V1().Services().Informer()},
		&handler.EnqueueRequestForObject{},
	)
	if err != nil {
		return errors.Wrap(err, "failed to watch services")
	}

	return nil
}

var _ reconcile.Reconciler = &MemberController{}

// MemberController reconciles the sidecar
type MemberController struct {
	client.Client
	kubeClient kubernetes.Interface
	member     *identity.Member
	nodetool   *nodetool.Nodetool
	scheme     *runtime.Scheme
}

// Reconcile observes the state of a Scylla Member
func (mc *MemberController) Reconcile(request reconcile.Request) (reconcile.Result, error) {

	memberService := &corev1.Service{}
	err := mc.Get(
		context.TODO(),
		naming.NamespacedName(request.Name, request.Namespace),
		memberService,
	)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			log.Infof("Object %+v not found", request.NamespacedName)
			return reconcile.Result{}, nil
		}
		return reconcile.Result{Requeue: true}, errors.Wrap(err, "failed to get service from reconcile request")
	}

	log.Info("Starting reconciliation...")
	if err := mc.sync(memberService); err != nil {
		log.Errorf("An error occured during reconciliation: %+v", err)
		return reconcile.Result{Requeue: true}, errors.WithStack(err)
	}

	return reconcile.Result{}, nil
}

func (mc *MemberController) onStartup() error {

	log.Info("Setting up HTTP Checks...")
	// HTTP Checks
	go mc.setupHTTPChecks()

	// Setup config files
	log.Info("Setting up config files")
	cmd, err := config.NewForMember(mc.member, mc.kubeClient, mc.Client).Setup()
	if err != nil {
		return errors.Wrap(err, "failed to setup config files")
	}

	// Start the scylla process
	log.Info("Starting the scylla process")
	if err = cmd.Start(); err != nil {
		return errors.Wrap(err, "error starting database daemon: %s")
	}

	return nil
}
