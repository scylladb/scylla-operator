//go:build envtest

package controllers

import (
	"context"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controller/scylladbmanagerclusterregistration"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/test/envtest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// scyllaDBManagerClusterRegistrationTriggerFixture is the state the trigger rows change: two ScyllaDBDatacenters,
// each with a global ScyllaDB Manager registration, and a ScyllaDBDatacenter with no registration. No datacenter has
// an available node, so the registrations settle waiting for one, without calling ScyllaDB Manager.
type scyllaDBManagerClusterRegistrationTriggerFixture struct {
	triggerActions

	env      *envtest.Environment
	recorder *reconcileRecorder

	datacenter             *scyllav1alpha1.ScyllaDBDatacenter
	registration           *scyllav1alpha1.ScyllaDBManagerClusterRegistration
	otherRegistration      *scyllav1alpha1.ScyllaDBManagerClusterRegistration
	unregisteredDatacenter *scyllav1alpha1.ScyllaDBDatacenter
}

// scyllaDBManagerClusterRegistrationNameFor returns the name of the registration of the ScyllaDBDatacenter named
// sdcName.
func scyllaDBManagerClusterRegistrationNameFor(sdcName string) string {
	name, err := naming.ScyllaDBManagerClusterRegistrationNameForScyllaDBDatacenter(&scyllav1alpha1.ScyllaDBDatacenter{
		ObjectMeta: metav1.ObjectMeta{
			Name: sdcName,
		},
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	return name
}

// makeEnvtestScyllaDBManagerClusterRegistration returns a registration of the ScyllaDBDatacenter named sdcName with
// the global ScyllaDB Manager instance.
func makeEnvtestScyllaDBManagerClusterRegistration(namespace, sdcName string) *scyllav1alpha1.ScyllaDBManagerClusterRegistration {
	return &scyllav1alpha1.ScyllaDBManagerClusterRegistration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      scyllaDBManagerClusterRegistrationNameFor(sdcName),
			Namespace: namespace,
			Labels:    naming.GlobalScyllaDBManagerClusterRegistrationSelectorLabels(),
		},
		Spec: scyllav1alpha1.ScyllaDBManagerClusterRegistrationSpec{
			ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
				Kind: scyllav1alpha1.ScyllaDBDatacenterGVK.Kind,
				Name: sdcName,
			},
		},
	}
}

// secretControlledBy returns a Secret named name, controlled by sdc if it isn't nil.
func (f *scyllaDBManagerClusterRegistrationTriggerFixture) secretControlledBy(name string, sdc *scyllav1alpha1.ScyllaDBDatacenter) *corev1.Secret {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: f.env.Namespace(),
		},
	}
	if sdc != nil {
		secret.OwnerReferences = []metav1.OwnerReference{
			*metav1.NewControllerRef(sdc, scyllav1alpha1.ScyllaDBDatacenterGVK),
		}
	}

	return secret
}

// These rows check which ScyllaDBManagerClusterRegistrations each kind of change reconciles. The rows run in order
// against one fixture, and a failing row doesn't stop the rest. The registration deletion row comes before the
// global ScyllaDB Manager Namespace exists: without it, the finalizer is removed without calling ScyllaDB Manager.
var _ = g.Describe("ScyllaDBManagerClusterRegistration controller triggers", g.Ordered, g.ContinueOnFailure, func() {
	const (
		datacenterName             = "envtest-sdc"
		otherDatacenterName        = "envtest-other-sdc"
		unregisteredDatacenterName = "envtest-unregistered-sdc"
	)

	var f *scyllaDBManagerClusterRegistrationTriggerFixture

	g.BeforeAll(func(ctx g.SpecContext) {
		env := envtest.Setup(ctx)
		recorder := &reconcileRecorder{}

		g.By("Running ScyllaDBManagerClusterRegistration controller with a reconcile observer")
		// A setup node's context ends with the node; the controller runs for the whole container. Cleanups run in
		// reverse order, so the context is cancelled before the runner's cleanup waits for the controller to stop.
		controllerCtx, cancel := context.WithCancel(context.Background())
		runScyllaDBManagerClusterRegistrationController(controllerCtx, env, scylladbmanagerclusterregistration.WithOnReconcile(recorder.observe))
		g.DeferCleanup(cancel)

		g.By("Creating the ScyllaDBDatacenters")
		datacenters := map[string]*scyllav1alpha1.ScyllaDBDatacenter{}
		for _, name := range []string{datacenterName, otherDatacenterName, unregisteredDatacenterName} {
			sdc := makeEnvtestScyllaDBDatacenter(env.Namespace(), []string{"rack-a"}, func(sdc *scyllav1alpha1.ScyllaDBDatacenter) {
				sdc.Name = name
				sdc.Spec.ClusterName = name
			})
			sdc, err := env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).Create(ctx, sdc, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			datacenters[name] = sdc
		}

		g.By("Creating the registrations")
		registrations := map[string]*scyllav1alpha1.ScyllaDBManagerClusterRegistration{}
		for _, smcr := range []*scyllav1alpha1.ScyllaDBManagerClusterRegistration{
			makeEnvtestScyllaDBManagerClusterRegistration(env.Namespace(), datacenterName),
			makeEnvtestScyllaDBManagerClusterRegistration(env.Namespace(), otherDatacenterName),
		} {
			smcr, err := env.ScyllaClient().ScyllaV1alpha1().ScyllaDBManagerClusterRegistrations(env.Namespace()).Create(ctx, smcr, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			registrations[smcr.Name] = smcr
		}

		g.By("Waiting for the registrations to wait for their ScyllaDBDatacenters' availability")
		for _, name := range []string{scyllaDBManagerClusterRegistrationNameFor(datacenterName), scyllaDBManagerClusterRegistrationNameFor(otherDatacenterName)} {
			o.Eventually(func(eo o.Gomega, ctx context.Context) {
				smcr, err := env.ScyllaClient().ScyllaV1alpha1().ScyllaDBManagerClusterRegistrations(env.Namespace()).Get(ctx, name, metav1.GetOptions{})
				eo.Expect(err).NotTo(o.HaveOccurred())
				eo.Expect(smcr.Status.Conditions).To(o.ContainElement(o.And(
					o.HaveField("Reason", "AwaitingScyllaDBDatacenterAvailability"),
					o.HaveField("Status", metav1.ConditionTrue),
				)))
			}).WithContext(ctx).WithTimeout(triggerTimeout).Should(o.Succeed())
		}

		f = &scyllaDBManagerClusterRegistrationTriggerFixture{
			triggerActions:         triggerActions{client: env.KubeClient()},
			env:                    env,
			recorder:               recorder,
			datacenter:             datacenters[datacenterName],
			registration:           registrations[scyllaDBManagerClusterRegistrationNameFor(datacenterName)],
			otherRegistration:      registrations[scyllaDBManagerClusterRegistrationNameFor(otherDatacenterName)],
			unregisteredDatacenter: datacenters[unregisteredDatacenterName],
		}
	})

	g.DescribeTable("reconciles the ScyllaDBManagerClusterRegistrations a change concerns",
		func(ctx g.SpecContext, row triggerRow[*scyllaDBManagerClusterRegistrationTriggerFixture]) {
			runTriggerRow(ctx, f.recorder, f, row)
		},

		g.Entry("registration update", triggerRow[*scyllaDBManagerClusterRegistrationTriggerFixture]{
			change: func(ctx context.Context, f *scyllaDBManagerClusterRegistrationTriggerFixture) {
				f.annotate(ctx, f.registration.DeepCopy())
			},
			expected: []string{scyllaDBManagerClusterRegistrationNameFor(datacenterName)},
		}),
		g.Entry("other registration update", triggerRow[*scyllaDBManagerClusterRegistrationTriggerFixture]{
			change: func(ctx context.Context, f *scyllaDBManagerClusterRegistrationTriggerFixture) {
				f.annotate(ctx, f.otherRegistration.DeepCopy())
			},
			expected: []string{scyllaDBManagerClusterRegistrationNameFor(otherDatacenterName)},
		}),

		// The rows below exercise the ScyllaDBDatacenter watch, reaching the registration named after the
		// datacenter, and the Secret watch, reaching it through the ScyllaDBDatacenter controlling the Secret.
		g.Entry("ScyllaDBDatacenter update", triggerRow[*scyllaDBManagerClusterRegistrationTriggerFixture]{
			change: func(ctx context.Context, f *scyllaDBManagerClusterRegistrationTriggerFixture) {
				f.annotate(ctx, f.datacenter.DeepCopy())
			},
			expected: []string{scyllaDBManagerClusterRegistrationNameFor(datacenterName)},
		}),
		g.Entry("unregistered ScyllaDBDatacenter update", triggerRow[*scyllaDBManagerClusterRegistrationTriggerFixture]{
			change: func(ctx context.Context, f *scyllaDBManagerClusterRegistrationTriggerFixture) {
				f.annotate(ctx, f.unregisteredDatacenter.DeepCopy())
			},
		}),
		g.Entry("Secret controlled by the ScyllaDBDatacenter creation", triggerRow[*scyllaDBManagerClusterRegistrationTriggerFixture]{
			change: func(ctx context.Context, f *scyllaDBManagerClusterRegistrationTriggerFixture) {
				f.create(ctx, f.secretControlledBy("controlled", f.datacenter))
			},
			expected: []string{scyllaDBManagerClusterRegistrationNameFor(datacenterName)},
		}),
		g.Entry("Secret controlled by the unregistered ScyllaDBDatacenter creation", triggerRow[*scyllaDBManagerClusterRegistrationTriggerFixture]{
			change: func(ctx context.Context, f *scyllaDBManagerClusterRegistrationTriggerFixture) {
				f.create(ctx, f.secretControlledBy("controlled-by-unregistered", f.unregisteredDatacenter))
			},
		}),
		g.Entry("uncontrolled Secret creation", triggerRow[*scyllaDBManagerClusterRegistrationTriggerFixture]{
			change: func(ctx context.Context, f *scyllaDBManagerClusterRegistrationTriggerFixture) {
				f.create(ctx, f.secretControlledBy("uncontrolled", nil))
			},
		}),

		g.Entry("registration deletion", triggerRow[*scyllaDBManagerClusterRegistrationTriggerFixture]{
			change: func(ctx context.Context, f *scyllaDBManagerClusterRegistrationTriggerFixture) {
				f.delete(ctx, f.otherRegistration.DeepCopy())
			},
			expected: []string{scyllaDBManagerClusterRegistrationNameFor(otherDatacenterName)},
		}),

		// The rows below exercise the Namespace watch, reaching every registration with the global ScyllaDB Manager
		// instance through its Namespace.
		g.Entry("global ScyllaDB Manager Namespace creation", triggerRow[*scyllaDBManagerClusterRegistrationTriggerFixture]{
			change: func(ctx context.Context, f *scyllaDBManagerClusterRegistrationTriggerFixture) {
				f.create(ctx, &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: naming.ScyllaManagerNamespace,
					},
				})
			},
			expected: []string{scyllaDBManagerClusterRegistrationNameFor(datacenterName)},
		}),
		g.Entry("global ScyllaDB Manager Namespace update", triggerRow[*scyllaDBManagerClusterRegistrationTriggerFixture]{
			change: func(ctx context.Context, f *scyllaDBManagerClusterRegistrationTriggerFixture) {
				f.annotate(ctx, &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: naming.ScyllaManagerNamespace,
					},
				})
			},
			expected: []string{scyllaDBManagerClusterRegistrationNameFor(datacenterName)},
		}),
		g.Entry("other Namespace creation", triggerRow[*scyllaDBManagerClusterRegistrationTriggerFixture]{
			change: func(ctx context.Context, f *scyllaDBManagerClusterRegistrationTriggerFixture) {
				f.create(ctx, &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "envtest-other",
					},
				})
			},
		}),
	)
})
