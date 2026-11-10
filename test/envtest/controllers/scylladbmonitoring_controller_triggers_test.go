//go:build envtest

package controllers

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controller/scylladbmonitoring"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/test/envtest"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// scyllaDBMonitoringTriggerFixture is the state the trigger rows change: a monitoring with Prometheus in External
// mode whose Grafana datasource refers to a Secret and a ConfigMap, and another one with Prometheus in Managed
// mode, which owns the Prometheus Operator objects.
type scyllaDBMonitoringTriggerFixture struct {
	triggerActions

	env      *envtest.Environment
	recorder *reconcileRecorder

	monitoring        *scyllav1alpha1.ScyllaDBMonitoring
	managedMonitoring *scyllav1alpha1.ScyllaDBMonitoring
	// tokenSecret and caConfigMap are referred to by monitoring's Grafana datasource.
	tokenSecret *corev1.Secret
	caConfigMap *corev1.ConfigMap
}

// getAnyOwnedBy returns any object of list's kind controlled by sm.
func (f *scyllaDBMonitoringTriggerFixture) getAnyOwnedBy(ctx context.Context, list client.ObjectList, sm *scyllav1alpha1.ScyllaDBMonitoring) client.Object {
	g.GinkgoHelper()

	err := f.env.KubeClient().List(ctx, list, client.InNamespace(f.env.Namespace()))
	o.Expect(err).NotTo(o.HaveOccurred())

	items, err := apimeta.ExtractList(list)
	o.Expect(err).NotTo(o.HaveOccurred())

	for _, item := range items {
		obj := item.(client.Object)
		if metav1.IsControlledBy(obj, sm) {
			return obj
		}
	}

	g.Fail(fmt.Sprintf("no object of %T is controlled by %q", list, sm.Name))
	return nil
}

// These rows check which ScyllaDBMonitorings each kind of change reconciles. The rows run in order against one
// fixture, and a failing row doesn't stop the rest, so every broken watch is reported at once.
var _ = g.Describe("ScyllaDBMonitoring controller triggers", g.Ordered, g.ContinueOnFailure, func() {
	const (
		monitoringName        = "envtest-monitoring"
		managedMonitoringName = "envtest-managed-monitoring"
	)

	var f *scyllaDBMonitoringTriggerFixture

	g.BeforeAll(func(ctx g.SpecContext) {
		env := envtest.Setup(ctx)
		recorder := &reconcileRecorder{}

		g.By("Running ScyllaDBMonitoring controller with a reconcile observer")
		// A setup node's context ends with the node; the controller runs for the whole container. Cleanups run in
		// reverse order, so the context is cancelled before the runner's cleanup waits for the controller to stop.
		controllerCtx, cancel := context.WithCancel(context.Background())
		runScyllaDBMonitoringController(controllerCtx, env, scylladbmonitoring.WithOnReconcile(recorder.observe))
		g.DeferCleanup(cancel)

		g.By("Creating ScyllaOperatorConfig singleton")
		createScyllaOperatorConfig(ctx, env)

		g.By("Creating the Secret and the ConfigMap the Grafana datasource refers to")
		tokenSecret, err := env.TypedKubeClient().CoreV1().Secrets(env.Namespace()).Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "prometheus-token",
				Namespace: env.Namespace(),
			},
			Data: map[string][]byte{
				"token": []byte("envtest"),
			},
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		caConfigMap, err := env.TypedKubeClient().CoreV1().ConfigMaps(env.Namespace()).Create(ctx, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "prometheus-ca",
				Namespace: env.Namespace(),
			},
			Data: map[string]string{
				"ca.crt": "envtest",
			},
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Creating a ScyllaDBMonitoring referring to them and exposing Grafana through an Ingress, and a ScyllaDBMonitoring with Prometheus in Managed mode")
		sm := newBasicScyllaDBMonitoring(monitoringName, env.Namespace())
		sm.Spec.Components.Grafana.ExposeOptions = &scyllav1alpha1.GrafanaExposeOptions{
			WebInterface: &scyllav1alpha1.HTTPSExposeOptions{
				Ingress: &scyllav1alpha1.IngressOptions{
					IngressClassName: "envtest",
					DNSDomains:       []string{"grafana.envtest.local"},
				},
			},
		}
		sm.Spec.Components.Grafana.Datasources[0].PrometheusOptions = &scyllav1alpha1.GrafanaPrometheusDatasourceOptions{
			TLS: &scyllav1alpha1.GrafanaDatasourceTLSSpec{
				CACertConfigMapRef: &scyllav1alpha1.LocalObjectKeySelector{
					Name: caConfigMap.Name,
					Key:  "ca.crt",
				},
			},
			Auth: &scyllav1alpha1.GrafanaPrometheusDatasourceAuthSpec{
				Type: scyllav1alpha1.GrafanaPrometheusDatasourceAuthTypeBearerToken,
				BearerTokenOptions: &scyllav1alpha1.GrafanaPrometheusDatasourceBearerTokenAuthOptions{
					SecretRef: &scyllav1alpha1.LocalObjectKeySelector{
						Name: tokenSecret.Name,
						Key:  "token",
					},
				},
			},
		}
		monitoring := createScyllaDBMonitoring(ctx, env, sm)
		managedMonitoring := createScyllaDBMonitoring(ctx, env, newManagedScyllaDBMonitoring(managedMonitoringName, env.Namespace()))

		g.By("Waiting for the Grafana Deployments, the Ingress and the Prometheus to be created")
		o.Eventually(func(eo o.Gomega, ctx context.Context) {
			for _, sm := range []*scyllav1alpha1.ScyllaDBMonitoring{monitoring, managedMonitoring} {
				_, err := env.TypedKubeClient().AppsV1().Deployments(env.Namespace()).Get(ctx, fmt.Sprintf("%s-grafana", sm.Name), metav1.GetOptions{})
				eo.Expect(err).NotTo(o.HaveOccurred())
			}

			ingresses, err := env.TypedKubeClient().NetworkingV1().Ingresses(env.Namespace()).List(ctx, metav1.ListOptions{})
			eo.Expect(err).NotTo(o.HaveOccurred())
			eo.Expect(ingresses.Items).NotTo(o.BeEmpty())

			prometheuses := &monitoringv1.PrometheusList{}
			err = env.KubeClient().List(ctx, prometheuses, client.InNamespace(env.Namespace()))
			eo.Expect(err).NotTo(o.HaveOccurred())
			eo.Expect(prometheuses.Items).NotTo(o.BeEmpty())
		}).WithContext(ctx).WithTimeout(triggerTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())

		f = &scyllaDBMonitoringTriggerFixture{
			triggerActions:    triggerActions{client: env.KubeClient()},
			env:               env,
			recorder:          recorder,
			monitoring:        monitoring,
			managedMonitoring: managedMonitoring,
			tokenSecret:       tokenSecret,
			caConfigMap:       caConfigMap,
		}
	})

	g.DescribeTable("reconciles the ScyllaDBMonitorings a change concerns",
		func(ctx g.SpecContext, row triggerRow[*scyllaDBMonitoringTriggerFixture]) {
			runTriggerRow(ctx, f.recorder, f, row)
		},

		// The rows below cover the ScyllaDBMonitoring watch and the objects a ScyllaDBMonitoring controls, which are
		// enqueued through their controllerRef. Every event of an object goes through the same handler, so one
		// change per kind is enough.
		g.Entry("ScyllaDBMonitoring update", triggerRow[*scyllaDBMonitoringTriggerFixture]{
			change: func(ctx context.Context, f *scyllaDBMonitoringTriggerFixture) {
				f.annotate(ctx, f.monitoring.DeepCopy())
			},
			expected: []string{monitoringName},
		}),
		g.Entry("other ScyllaDBMonitoring update", triggerRow[*scyllaDBMonitoringTriggerFixture]{
			change: func(ctx context.Context, f *scyllaDBMonitoringTriggerFixture) {
				f.annotate(ctx, f.managedMonitoring.DeepCopy())
			},
			expected: []string{managedMonitoringName},
		}),

		g.Entry("owned ConfigMap update", triggerRow[*scyllaDBMonitoringTriggerFixture]{
			change: func(ctx context.Context, f *scyllaDBMonitoringTriggerFixture) {
				f.annotate(ctx, f.getAnyOwnedBy(ctx, &corev1.ConfigMapList{}, f.monitoring))
			},
			expected: []string{monitoringName},
		}),
		g.Entry("owned Secret update", triggerRow[*scyllaDBMonitoringTriggerFixture]{
			change: func(ctx context.Context, f *scyllaDBMonitoringTriggerFixture) {
				f.annotate(ctx, f.getAnyOwnedBy(ctx, &corev1.SecretList{}, f.monitoring))
			},
			expected: []string{monitoringName},
		}),
		g.Entry("owned Service update", triggerRow[*scyllaDBMonitoringTriggerFixture]{
			change: func(ctx context.Context, f *scyllaDBMonitoringTriggerFixture) {
				f.annotate(ctx, f.getAnyOwnedBy(ctx, &corev1.ServiceList{}, f.monitoring))
			},
			expected: []string{monitoringName},
		}),
		g.Entry("owned ServiceAccount update", triggerRow[*scyllaDBMonitoringTriggerFixture]{
			change: func(ctx context.Context, f *scyllaDBMonitoringTriggerFixture) {
				f.annotate(ctx, f.getAnyOwnedBy(ctx, &corev1.ServiceAccountList{}, f.monitoring))
			},
			expected: []string{monitoringName},
		}),
		g.Entry("owned Deployment update", triggerRow[*scyllaDBMonitoringTriggerFixture]{
			change: func(ctx context.Context, f *scyllaDBMonitoringTriggerFixture) {
				f.annotate(ctx, f.getAnyOwnedBy(ctx, &appsv1.DeploymentList{}, f.monitoring))
			},
			expected: []string{monitoringName},
		}),
		g.Entry("owned Ingress update", triggerRow[*scyllaDBMonitoringTriggerFixture]{
			change: func(ctx context.Context, f *scyllaDBMonitoringTriggerFixture) {
				f.annotate(ctx, f.getAnyOwnedBy(ctx, &networkingv1.IngressList{}, f.monitoring))
			},
			expected: []string{monitoringName},
		}),
		g.Entry("owned Prometheus update", triggerRow[*scyllaDBMonitoringTriggerFixture]{
			change: func(ctx context.Context, f *scyllaDBMonitoringTriggerFixture) {
				f.annotate(ctx, f.getAnyOwnedBy(ctx, &monitoringv1.PrometheusList{}, f.managedMonitoring))
			},
			expected: []string{managedMonitoringName},
		}),
		g.Entry("owned PrometheusRule update", triggerRow[*scyllaDBMonitoringTriggerFixture]{
			change: func(ctx context.Context, f *scyllaDBMonitoringTriggerFixture) {
				f.annotate(ctx, f.getAnyOwnedBy(ctx, &monitoringv1.PrometheusRuleList{}, f.managedMonitoring))
			},
			expected: []string{managedMonitoringName},
		}),
		g.Entry("owned ServiceMonitor update", triggerRow[*scyllaDBMonitoringTriggerFixture]{
			change: func(ctx context.Context, f *scyllaDBMonitoringTriggerFixture) {
				f.annotate(ctx, f.getAnyOwnedBy(ctx, &monitoringv1.ServiceMonitorList{}, f.managedMonitoring))
			},
			expected: []string{managedMonitoringName},
		}),
		// The controller creates no PodDisruptionBudget, but watches them.
		g.Entry("PodDisruptionBudget controlled by a ScyllaDBMonitoring creation", triggerRow[*scyllaDBMonitoringTriggerFixture]{
			change: func(ctx context.Context, f *scyllaDBMonitoringTriggerFixture) {
				f.create(ctx, &policyv1.PodDisruptionBudget{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "envtest",
						Namespace: f.env.Namespace(),
						OwnerReferences: []metav1.OwnerReference{
							*metav1.NewControllerRef(f.monitoring, scyllav1alpha1.GroupVersion.WithKind("ScyllaDBMonitoring")),
						},
					},
					Spec: policyv1.PodDisruptionBudgetSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "envtest",
							},
						},
					},
				})
			},
			expected: []string{monitoringName},
		}),
		g.Entry("ServiceAccount controlled by another kind creation", triggerRow[*scyllaDBMonitoringTriggerFixture]{
			change: func(ctx context.Context, f *scyllaDBMonitoringTriggerFixture) {
				f.create(ctx, &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "foreign",
						Namespace:       f.env.Namespace(),
						OwnerReferences: []metav1.OwnerReference{makeEnvtestForeignControllerRef()},
					},
				})
			},
		}),

		// The rows below exercise the enqueue logic written by hand: the Secret and ConfigMap watches reaching the
		// monitorings whose Grafana datasource refers to them, and the ScyllaOperatorConfig watch reaching every
		// monitoring.
		g.Entry("datasource Secret update", triggerRow[*scyllaDBMonitoringTriggerFixture]{
			change: func(ctx context.Context, f *scyllaDBMonitoringTriggerFixture) {
				f.annotate(ctx, f.tokenSecret.DeepCopy())
			},
			expected: []string{monitoringName},
		}),
		g.Entry("datasource ConfigMap update", triggerRow[*scyllaDBMonitoringTriggerFixture]{
			change: func(ctx context.Context, f *scyllaDBMonitoringTriggerFixture) {
				f.annotate(ctx, f.caConfigMap.DeepCopy())
			},
			expected: []string{monitoringName},
		}),
		g.Entry("unrelated Secret creation", triggerRow[*scyllaDBMonitoringTriggerFixture]{
			change: func(ctx context.Context, f *scyllaDBMonitoringTriggerFixture) {
				f.create(ctx, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "unrelated",
						Namespace: f.env.Namespace(),
					},
				})
			},
		}),
		g.Entry("unrelated ConfigMap creation", triggerRow[*scyllaDBMonitoringTriggerFixture]{
			change: func(ctx context.Context, f *scyllaDBMonitoringTriggerFixture) {
				f.create(ctx, &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "unrelated",
						Namespace: f.env.Namespace(),
					},
				})
			},
		}),

		g.Entry("ScyllaOperatorConfig update", triggerRow[*scyllaDBMonitoringTriggerFixture]{
			change: func(ctx context.Context, f *scyllaDBMonitoringTriggerFixture) {
				f.annotate(ctx, &scyllav1alpha1.ScyllaOperatorConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name: naming.SingletonName,
					},
				})
			},
			expected: []string{monitoringName, managedMonitoringName},
		}),
		g.Entry("other ScyllaOperatorConfig creation", triggerRow[*scyllaDBMonitoringTriggerFixture]{
			change: func(ctx context.Context, f *scyllaDBMonitoringTriggerFixture) {
				f.create(ctx, &scyllav1alpha1.ScyllaOperatorConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name: "other",
					},
				})
			},
		}),
	)
})
