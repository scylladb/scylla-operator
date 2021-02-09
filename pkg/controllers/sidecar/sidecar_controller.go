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
	"time"

	"github.com/pkg/errors"
	"github.com/scylladb/go-log"
	"github.com/scylladb/scylla-operator/pkg/cmd/scylla-operator/options"
	"github.com/scylladb/scylla-operator/pkg/controllers/sidecar/config"
	"github.com/scylladb/scylla-operator/pkg/controllers/sidecar/identity"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/scyllaclient"
	"github.com/scylladb/scylla-operator/pkg/util/cfgutil"
	"github.com/scylladb/scylla-operator/pkg/util/network"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/informers/internalinterfaces"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const concurrency = 1

var _ reconcile.Reconciler = &MemberReconciler{}

// MemberReconciler reconciles the sidecar
type MemberReconciler struct {
	client.Client
	kubeClient   kubernetes.Interface
	member       *identity.Member
	scyllaClient *scyllaclient.Client
	scheme       *runtime.Scheme
	logger       log.Logger
}

// Reconcile observes the state of a Scylla Member
func (mc *MemberReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	ctx := log.WithNewTraceID(context.Background())
	memberService := &corev1.Service{}
	err := mc.Get(ctx, naming.NamespacedName(request.Name, request.Namespace), memberService)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			mc.logger.Info(ctx, "Object not found", "namespace", request.Namespace, "name", request.Name)
			return reconcile.Result{}, nil
		}
		return reconcile.Result{Requeue: true}, errors.Wrap(err, "failed to get service from reconcile request")
	}

	mc.logger.Info(ctx, "Starting reconciliation...")

	requeue, err := mc.sync(ctx, memberService)
	if err != nil {
		mc.logger.Error(ctx, "An error occurred during reconciliation", "error", err)
		return reconcile.Result{Requeue: requeue}, errors.WithStack(err)
	}
	return reconcile.Result{Requeue: requeue}, nil
}

// newReconciler returns a new reconcile.Reconciler
func New(ctx context.Context, mgr manager.Manager, logger log.Logger) (*MemberReconciler, error) {
	opts := options.GetSidecarOptions()
	kubeClient := kubernetes.NewForConfigOrDie(mgr.GetConfig())
	member, err := identity.Retrieve(ctx, opts.Name, opts.Namespace, kubeClient)
	if err != nil {
		return nil, errors.Wrap(err, "get member")
	}
	logger.Info(ctx, "Member loaded", "member", member)

	c, err := client.New(mgr.GetConfig(), client.Options{
		Scheme: mgr.GetScheme(),
	})
	if err != nil {
		return nil, errors.Wrap(err, "get dynamic client")
	}

	host, err := network.FindFirstNonLocalIP()
	if err != nil {
		return nil, errors.Wrap(err, "get scylla address")
	}

	cfg := scyllaclient.DefaultConfig(host.String())
	if err := cfgutil.ParseYAML(&cfg, naming.ScyllaClientConfigDirName+"/"+naming.ScyllaClientConfigFileName); err != nil {
		return nil, errors.Wrap(err, "parse scylla agent config")
	}
	scyllaClient, err := scyllaclient.NewClient(cfg, logger.Named("scylla_client"))
	if err != nil {
		return nil, errors.Wrap(err, "create scylla client")
	}
	mc := &MemberReconciler{
		Client:       c,
		kubeClient:   kubeClient,
		member:       member,
		scheme:       mgr.GetScheme(),
		scyllaClient: scyllaClient,
		logger:       logger,
	}

	if err = mc.onStartup(ctx); err != nil {
		return nil, errors.Wrap(err, "startup")
	}

	return mc, nil
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func (mc *MemberReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Create a new controller
	c, err := controller.New("sidecar-controller", mgr, controller.Options{
		Reconciler:              mc,
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

func (mc *MemberReconciler) onStartup(ctx context.Context) error {
	mc.logger.Info(ctx, "Setting up HTTP Checks...")
	// HTTP Checks
	go mc.setupHTTPChecks(ctx)

	// Setup config files
	mc.logger.Info(ctx, "Setting up config files")
	cfg := config.NewForMember(mc.member, mc.kubeClient, mc.Client, mc.logger)
	cmd, err := cfg.Setup(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to setup config files")
	}

	// Start the scylla process
	mc.logger.Info(ctx, "Starting the scylla process")
	if err = cmd.Start(); err != nil {
		return errors.Wrap(err, "error starting database daemon: %s")
	}
	return nil
}
