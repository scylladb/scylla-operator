// Copyright (c) 2026 ScyllaDB.

//go:build envtest

package controllers

import (
	"context"
	"sync/atomic"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	operatorcmd "github.com/scylladb/scylla-operator/pkg/cmd/operator"
	"github.com/scylladb/scylla-operator/test/envtest"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var _ = g.Describe("Mutating admission webhook", func() {
	var env *envtest.Environment

	g.BeforeEach(func(ctx g.SpecContext) {
		// The spec installs the webhook itself, wrapping the handler to count the invocations, and observes the
		// behavior from before it was installed, so the default install has to be opted out of.
		env = envtest.Setup(ctx, envtest.WithoutMonitoringCRDs(), envtest.WithoutMutatingWebhook())
	})

	type entry struct {
		// resource is the resource of the tested kind, as matched by the MutatingWebhookConfiguration rules.
		resource string
		// expectedIntercepted tells whether the shipped rules are expected to route CREATEs of the kind to the webhook.
		expectedIntercepted bool
		// newObject returns an object of the tested kind, valid enough to be created against the installed CRDs.
		newObject func(name, namespace string) client.Object
	}

	g.DescribeTable("with the shipped MutatingWebhookConfiguration installed", func(ctx g.SpecContext, e *entry) {
		g.By("Verifying no mutating webhook is installed yet")
		mutatingWebhookConfigurations, err := env.TypedKubeClient().AdmissionregistrationV1().MutatingWebhookConfigurations().List(ctx, metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(mutatingWebhookConfigurations.Items).To(o.BeEmpty(), "the baseline object has to be created before any mutating webhook is installed")

		g.By("Creating a baseline object before the mutating webhook is installed")
		baseline := e.newObject("baseline", env.Namespace())
		err = env.KubeClient().Create(ctx, baseline)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Installing the mutating webhook dispatching to the operator's registered defaulters")
		var resourceInvocations, totalInvocations atomic.Int64
		mutatingHandler := operatorcmd.NewMutatingWebhookHandler(operatorcmd.DefaultDefaulters)
		envtest.SetupOperatorMutatingWebhook(ctx, env, admission.HandlerFunc(func(ctx context.Context, req admission.Request) admission.Response {
			totalInvocations.Add(1)
			if req.Resource.Resource == e.resource {
				resourceInvocations.Add(1)
			}

			return mutatingHandler.Handle(ctx, req)
		}))

		g.By("Creating an object with the mutating webhook installed")
		obj := e.newObject("mutated", env.Namespace())
		err = env.KubeClient().Create(ctx, obj)
		o.Expect(err).NotTo(o.HaveOccurred())

		if !e.expectedIntercepted {
			o.Expect(resourceInvocations.Load()).To(o.BeZero(), "the shipped rules shouldn't match %q", e.resource)

			g.By("Creating an intercepted object to verify the webhook server is serving")
			err = env.KubeClient().Create(ctx, newBasicScyllaCluster("liveness", env.Namespace()))
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(totalInvocations.Load()).To(o.BeNumerically(">=", 1), "the webhook server should be serving, otherwise the assertion above holds vacuously")

			return
		}

		o.Expect(resourceInvocations.Load()).To(o.BeNumerically(">=", 1), "the mutating webhook should have been invoked on CREATE")
		o.Expect(getUnstructuredSpec(obj)).To(o.Equal(getUnstructuredSpec(baseline)), "the spec should be admitted unchanged")
		o.Expect(obj.GetLabels()).To(o.Equal(baseline.GetLabels()), "the labels should be admitted unchanged")
		o.Expect(obj.GetAnnotations()).To(o.Equal(baseline.GetAnnotations()), "the annotations should be admitted unchanged")

		g.By("Updating the object and expecting the mutating webhook not to be invoked")
		resourceInvocationsBeforeUpdate := resourceInvocations.Load()
		labels := obj.GetLabels()
		if labels == nil {
			labels = map[string]string{}
		}
		labels["update-trigger"] = "true"
		obj.SetLabels(labels)
		err = env.KubeClient().Update(ctx, obj)
		o.Expect(err).NotTo(o.HaveOccurred())

		o.Expect(resourceInvocations.Load()).To(o.Equal(resourceInvocationsBeforeUpdate), "the shipped rules should be CREATE-only")
	},
		g.Entry("admits a ScyllaCluster unchanged", &entry{
			resource:            "scyllaclusters",
			expectedIntercepted: true,
			newObject: func(name, namespace string) client.Object {
				return newBasicScyllaCluster(name, namespace)
			},
		}),
		g.Entry("admits a ScyllaDBDatacenter unchanged", &entry{
			resource:            "scylladbdatacenters",
			expectedIntercepted: true,
			newObject: func(name, namespace string) client.Object {
				sdc := makeEnvtestScyllaDBDatacenter(namespace, []string{"rack1"})
				sdc.Name = name
				return sdc
			},
		}),
		// ScyllaDBClusters are deliberately left out of the shipped rules, as parallel bootstrap is not
		// supported in automated multi-datacenter setups.
		g.Entry("doesn't intercept a ScyllaDBCluster", &entry{
			resource:            "scylladbclusters",
			expectedIntercepted: false,
			newObject: func(name, namespace string) client.Object {
				return &scyllav1alpha1.ScyllaDBCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: namespace,
					},
				}
			},
		}),
	)
})

// The spec above installs the webhook itself, so it can't catch the default install regressing: every spec relying on
// it would silently stop being defaulted, rather than fail.
var _ = g.Describe("Mutating admission webhook installed by the default setup", func() {
	var env *envtest.Environment

	g.BeforeEach(func(ctx g.SpecContext) {
		env = envtest.Setup(ctx, envtest.WithoutMonitoringCRDs())
	})

	g.It("intercepts CREATEs without a spec installing it", func(ctx g.SpecContext) {
		g.By("Verifying the shipped MutatingWebhookConfiguration is installed")
		mutatingWebhookConfigurations, err := env.TypedKubeClient().AdmissionregistrationV1().MutatingWebhookConfigurations().List(ctx, metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(mutatingWebhookConfigurations.Items).NotTo(o.BeEmpty())

		// The shipped configuration uses failurePolicy: Fail, so an intercepted kind can only be created if the
		// webhook server the default setup started is serving.
		g.By("Creating an intercepted object to verify the webhook server is serving")
		err = env.KubeClient().Create(ctx, newBasicScyllaCluster("default-install", env.Namespace()))
		o.Expect(err).NotTo(o.HaveOccurred())
	})
})

// getUnstructuredSpec returns the spec of obj converted to an unstructured value, so that specs of different kinds
// can be compared without the test knowing their Go types.
func getUnstructuredSpec(obj client.Object) any {
	g.GinkgoHelper()

	u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to convert object to unstructured")

	return u["spec"]
}
