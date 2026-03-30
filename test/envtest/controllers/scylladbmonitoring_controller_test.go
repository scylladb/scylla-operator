//go:build envtest

package controllers

import (
	"context"
	"fmt"
	"sync"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringinformers "github.com/prometheus-operator/prometheus-operator/pkg/client/informers/externalversions"
	monitoringversionedclient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllainformers "github.com/scylladb/scylla-operator/pkg/client/scylla/informers/externalversions"
	"github.com/scylladb/scylla-operator/pkg/controller/scylladbmonitoring"
	"github.com/scylladb/scylla-operator/pkg/crypto"
	"github.com/scylladb/scylla-operator/pkg/internalapi"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/test/envtest"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
)

var _ = g.Describe("ScyllaDBMonitoringController", func() {
	var env *envtest.Environment
	g.BeforeEach(func(ctx g.SpecContext) {
		env = envtest.Setup(ctx)
	})

	g.It("reports Available=False before Grafana Deployment is rolled out (External mode)", func(ctx g.SpecContext) {
		g.By("Starting ScyllaDBMonitoring controller")
		runScyllaDBMonitoringController(ctx, env)

		g.By("Creating ScyllaOperatorConfig singleton")
		createScyllaOperatorConfig(ctx, env)

		sm := newBasicScyllaDBMonitoring("ext-not-ready", env.Namespace())
		sm = createScyllaDBMonitoring(ctx, env, sm)

		g.By("Waiting for controller to create the Grafana Deployment")
		o.Eventually(func(eg o.Gomega) {
			_, err := env.TypedKubeClient().AppsV1().Deployments(env.Namespace()).Get(ctx, fmt.Sprintf("%s-grafana", sm.Name), metav1.GetOptions{})
			eg.Expect(err).NotTo(o.HaveOccurred())
		}).WithTimeout(30 * time.Second).WithContext(ctx).Should(o.Succeed())

		g.By("Expecting Available=False and Progressing=True before Grafana Deployment is available")
		o.Eventually(func(eg o.Gomega) {
			var err error
			sm, err = env.ScyllaClient().ScyllaV1alpha1().ScyllaDBMonitorings(env.Namespace()).Get(ctx, sm.Name, metav1.GetOptions{})
			eg.Expect(err).NotTo(o.HaveOccurred())

			availCond := apimeta.FindStatusCondition(sm.Status.Conditions, scyllav1alpha1.AvailableCondition)
			eg.Expect(availCond).NotTo(o.BeNil(), "Available condition should be present")
			eg.Expect(availCond.Status).To(o.Equal(metav1.ConditionFalse), "Available should be False while Grafana Deployment has no Available condition")

			progressingCond := apimeta.FindStatusCondition(sm.Status.Conditions, scyllav1alpha1.ProgressingCondition)
			eg.Expect(progressingCond).NotTo(o.BeNil(), "Progressing condition should be present")
			eg.Expect(progressingCond.Status).To(o.Equal(metav1.ConditionTrue), "Progressing should be True while Grafana Deployment has no Progressing condition set to False")
		}).WithTimeout(30 * time.Second).WithContext(ctx).Should(o.Succeed())
	})

	g.It("reports Available=True once Grafana Deployment is rolled out (External mode)", func(ctx g.SpecContext) {
		g.By("Starting ScyllaDBMonitoring controller")
		runScyllaDBMonitoringController(ctx, env)

		g.By("Creating ScyllaOperatorConfig singleton")
		createScyllaOperatorConfig(ctx, env)

		sm := newBasicScyllaDBMonitoring("ext-ready", env.Namespace())
		sm = createScyllaDBMonitoring(ctx, env, sm)

		g.By("Waiting for controller to create the Grafana Deployment")
		var grafanaDeployment *appsv1.Deployment
		o.Eventually(func(eg o.Gomega) {
			var err error
			grafanaDeployment, err = env.TypedKubeClient().AppsV1().Deployments(env.Namespace()).Get(ctx, fmt.Sprintf("%s-grafana", sm.Name), metav1.GetOptions{})
			eg.Expect(err).NotTo(o.HaveOccurred())
		}).WithTimeout(30 * time.Second).WithContext(ctx).Should(o.Succeed())

		g.By("Patching Grafana Deployment status conditions to simulate an available Deployment")
		patchGrafanaDeploymentAsAvailable(ctx, env, grafanaDeployment)

		g.By("Expecting Available=True and Progressing=False once Grafana is available")
		o.Eventually(func(eg o.Gomega) {
			var err error
			sm, err = env.ScyllaClient().ScyllaV1alpha1().ScyllaDBMonitorings(env.Namespace()).Get(ctx, sm.Name, metav1.GetOptions{})
			eg.Expect(err).NotTo(o.HaveOccurred())

			availCond := apimeta.FindStatusCondition(sm.Status.Conditions, scyllav1alpha1.AvailableCondition)
			eg.Expect(availCond).NotTo(o.BeNil(), "Available condition should be present")
			eg.Expect(availCond.Status).To(o.Equal(metav1.ConditionTrue), "Available should be True once Grafana Deployment is available")

			progressingCond := apimeta.FindStatusCondition(sm.Status.Conditions, scyllav1alpha1.ProgressingCondition)
			eg.Expect(progressingCond).NotTo(o.BeNil(), "Progressing condition should be present")
			eg.Expect(progressingCond.Status).To(o.Equal(metav1.ConditionFalse), "Progressing should be False once Grafana Deployment is not progressing")
		}).WithTimeout(30 * time.Second).WithContext(ctx).Should(o.Succeed())
	})

	g.It("reports Available=False when Prometheus CR has not yet reported Available=True (Managed mode)", func(ctx g.SpecContext) {
		g.By("Starting ScyllaDBMonitoring controller")
		runScyllaDBMonitoringController(ctx, env)

		g.By("Creating ScyllaOperatorConfig singleton")
		createScyllaOperatorConfig(ctx, env)

		sm := newManagedScyllaDBMonitoring("managed-not-ready", env.Namespace())
		sm = createScyllaDBMonitoring(ctx, env, sm)

		g.By("Waiting for controller to create the Grafana Deployment and Prometheus CR")
		var grafanaDeployment *appsv1.Deployment
		o.Eventually(func(eg o.Gomega) {
			var err error
			grafanaDeployment, err = env.TypedKubeClient().AppsV1().Deployments(env.Namespace()).Get(ctx, fmt.Sprintf("%s-grafana", sm.Name), metav1.GetOptions{})
			eg.Expect(err).NotTo(o.HaveOccurred())
		}).WithTimeout(30 * time.Second).WithContext(ctx).Should(o.Succeed())

		g.By("Patching Grafana Deployment status conditions to simulate an available Deployment")
		patchGrafanaDeploymentAsAvailable(ctx, env, grafanaDeployment)

		g.By("Expecting Available=False and Progressing=True because Prometheus CR has no conditions")
		o.Eventually(func(eg o.Gomega) {
			var err error
			sm, err = env.ScyllaClient().ScyllaV1alpha1().ScyllaDBMonitorings(env.Namespace()).Get(ctx, sm.Name, metav1.GetOptions{})
			eg.Expect(err).NotTo(o.HaveOccurred())

			availCond := apimeta.FindStatusCondition(sm.Status.Conditions, scyllav1alpha1.AvailableCondition)
			eg.Expect(availCond).NotTo(o.BeNil(), "Available condition should be present")
			eg.Expect(availCond.Status).To(o.Equal(metav1.ConditionFalse), "Available should be False while Prometheus CR has not reported Available=True")

			progressingCond := apimeta.FindStatusCondition(sm.Status.Conditions, scyllav1alpha1.ProgressingCondition)
			eg.Expect(progressingCond).NotTo(o.BeNil(), "Progressing condition should be present")
			eg.Expect(progressingCond.Status).To(o.Equal(metav1.ConditionTrue), "Progressing should be True while Prometheus CR has not reported Reconciled=True")
		}).WithTimeout(30 * time.Second).WithContext(ctx).Should(o.Succeed())
	})

	g.It("reports Available=True when both Grafana Deployment is rolled out and Prometheus CR reports Available=True (Managed mode)", func(ctx g.SpecContext) {
		g.By("Starting ScyllaDBMonitoring controller")
		runScyllaDBMonitoringController(ctx, env)

		g.By("Creating ScyllaOperatorConfig singleton")
		createScyllaOperatorConfig(ctx, env)

		sm := newManagedScyllaDBMonitoring("managed-ready", env.Namespace())
		sm = createScyllaDBMonitoring(ctx, env, sm)

		g.By("Waiting for controller to create the Grafana Deployment")
		var grafanaDeployment *appsv1.Deployment
		o.Eventually(func(eg o.Gomega) {
			var err error
			grafanaDeployment, err = env.TypedKubeClient().AppsV1().Deployments(env.Namespace()).Get(ctx, fmt.Sprintf("%s-grafana", sm.Name), metav1.GetOptions{})
			eg.Expect(err).NotTo(o.HaveOccurred())
		}).WithTimeout(30 * time.Second).WithContext(ctx).Should(o.Succeed())

		g.By("Patching Grafana Deployment status conditions to simulate an available Deployment")
		patchGrafanaDeploymentAsAvailable(ctx, env, grafanaDeployment)

		g.By("Waiting for controller to create the Prometheus CR")
		var prometheus *monitoringv1.Prometheus
		o.Eventually(func(eg o.Gomega) {
			var err error
			prometheus, err = env.MonitoringClient().MonitoringV1().Prometheuses(env.Namespace()).Get(ctx, sm.Name, metav1.GetOptions{})
			eg.Expect(err).NotTo(o.HaveOccurred())
		}).WithTimeout(30 * time.Second).WithContext(ctx).Should(o.Succeed())

		g.By("Patching Prometheus CR status with Available=True and Reconciled=True")
		patchPrometheusAsAvailable(ctx, env, prometheus)

		g.By("Expecting Available=True and Progressing=False once both Grafana and Prometheus are ready")
		o.Eventually(func(eg o.Gomega) {
			sm, err := env.ScyllaClient().ScyllaV1alpha1().ScyllaDBMonitorings(env.Namespace()).Get(ctx, sm.Name, metav1.GetOptions{})
			eg.Expect(err).NotTo(o.HaveOccurred())

			availCond := apimeta.FindStatusCondition(sm.Status.Conditions, scyllav1alpha1.AvailableCondition)
			eg.Expect(availCond).NotTo(o.BeNil(), "Available condition should be present")
			eg.Expect(availCond.Status).To(o.Equal(metav1.ConditionTrue), "Available should be True when Grafana is available and Prometheus reports Available=True")

			progressingCond := apimeta.FindStatusCondition(sm.Status.Conditions, scyllav1alpha1.ProgressingCondition)
			eg.Expect(progressingCond).NotTo(o.BeNil(), "Progressing condition should be present")
			eg.Expect(progressingCond.Status).To(o.Equal(metav1.ConditionFalse), "Progressing should be False when Grafana is not progressing and Prometheus reports Reconciled=True")
		}).WithTimeout(30 * time.Second).WithContext(ctx).Should(o.Succeed())
	})
	g.It("reports Available=False and Progressing=True when Grafana Deployment explicitly reports Available=False (External mode)", func(ctx g.SpecContext) {
		g.By("Starting ScyllaDBMonitoring controller")
		runScyllaDBMonitoringController(ctx, env)

		g.By("Creating ScyllaOperatorConfig singleton")
		createScyllaOperatorConfig(ctx, env)

		sm := newBasicScyllaDBMonitoring("ext-grafana", env.Namespace())
		sm = createScyllaDBMonitoring(ctx, env, sm)

		g.By("Waiting for controller to create the Grafana Deployment")
		var grafanaDeployment *appsv1.Deployment
		o.Eventually(func(eg o.Gomega) {
			var err error
			grafanaDeployment, err = env.TypedKubeClient().AppsV1().Deployments(env.Namespace()).Get(ctx, fmt.Sprintf("%s-grafana", sm.Name), metav1.GetOptions{})
			eg.Expect(err).NotTo(o.HaveOccurred())
		}).WithTimeout(30 * time.Second).WithContext(ctx).Should(o.Succeed())

		g.By("Patching Grafana Deployment status conditions to simulate unavailable/progressing state")
		patchGrafanaDeploymentAsNotAvailable(ctx, env, grafanaDeployment)

		g.By("Expecting Available=False and Progressing=True when Grafana Deployment explicitly reports Available=False")
		o.Eventually(func(eg o.Gomega) {
			var err error
			sm, err = env.ScyllaClient().ScyllaV1alpha1().ScyllaDBMonitorings(env.Namespace()).Get(ctx, sm.Name, metav1.GetOptions{})
			eg.Expect(err).NotTo(o.HaveOccurred())

			availCond := apimeta.FindStatusCondition(sm.Status.Conditions, scyllav1alpha1.AvailableCondition)
			eg.Expect(availCond).NotTo(o.BeNil(), "Available condition should be present")
			eg.Expect(availCond.Status).To(o.Equal(metav1.ConditionFalse), "Available should be False when Grafana Deployment explicitly reports Available=False")

			progressingCond := apimeta.FindStatusCondition(sm.Status.Conditions, scyllav1alpha1.ProgressingCondition)
			eg.Expect(progressingCond).NotTo(o.BeNil(), "Progressing condition should be present")
			eg.Expect(progressingCond.Status).To(o.Equal(metav1.ConditionTrue), "Progressing should be True when Grafana Deployment explicitly reports Progressing=True")
		}).WithTimeout(30 * time.Second).WithContext(ctx).Should(o.Succeed())
	})

	g.It("reports Available=False and Progressing=True when Prometheus CR explicitly reports Available=False and Reconciled=False (Managed mode)", func(ctx g.SpecContext) {
		g.By("Starting ScyllaDBMonitoring controller")
		runScyllaDBMonitoringController(ctx, env)

		g.By("Creating ScyllaOperatorConfig singleton")
		createScyllaOperatorConfig(ctx, env)

		sm := newManagedScyllaDBMonitoring("managed-prometheus", env.Namespace())
		sm = createScyllaDBMonitoring(ctx, env, sm)

		g.By("Waiting for controller to create the Grafana Deployment")
		var grafanaDeployment *appsv1.Deployment
		o.Eventually(func(eg o.Gomega) {
			var err error
			grafanaDeployment, err = env.TypedKubeClient().AppsV1().Deployments(env.Namespace()).Get(ctx, fmt.Sprintf("%s-grafana", sm.Name), metav1.GetOptions{})
			eg.Expect(err).NotTo(o.HaveOccurred())
		}).WithTimeout(30 * time.Second).WithContext(ctx).Should(o.Succeed())

		g.By("Patching Grafana Deployment status conditions to simulate an available Deployment")
		patchGrafanaDeploymentAsAvailable(ctx, env, grafanaDeployment)

		g.By("Waiting for controller to create the Prometheus CR")
		var prometheus *monitoringv1.Prometheus
		o.Eventually(func(eg o.Gomega) {
			var err error
			prometheus, err = env.MonitoringClient().MonitoringV1().Prometheuses(env.Namespace()).Get(ctx, sm.Name, metav1.GetOptions{})
			eg.Expect(err).NotTo(o.HaveOccurred())
		}).WithTimeout(30 * time.Second).WithContext(ctx).Should(o.Succeed())

		g.By("Patching Prometheus CR status with Available=False and Reconciled=False")
		patchPrometheusAsNotReconciled(ctx, env, prometheus)

		g.By("Expecting Available=False and Progressing=True when Prometheus CR explicitly reports Available=False and Reconciled=False")
		o.Eventually(func(eg o.Gomega) {
			var err error
			sm, err = env.ScyllaClient().ScyllaV1alpha1().ScyllaDBMonitorings(env.Namespace()).Get(ctx, sm.Name, metav1.GetOptions{})
			eg.Expect(err).NotTo(o.HaveOccurred())

			availCond := apimeta.FindStatusCondition(sm.Status.Conditions, scyllav1alpha1.AvailableCondition)
			eg.Expect(availCond).NotTo(o.BeNil(), "Available condition should be present")
			eg.Expect(availCond.Status).To(o.Equal(metav1.ConditionFalse), "Available should be False when Prometheus CR explicitly reports Available=False")

			progressingCond := apimeta.FindStatusCondition(sm.Status.Conditions, scyllav1alpha1.ProgressingCondition)
			eg.Expect(progressingCond).NotTo(o.BeNil(), "Progressing condition should be present")
			eg.Expect(progressingCond.Status).To(o.Equal(metav1.ConditionTrue), "Progressing should be True when Prometheus CR explicitly reports Reconciled=False")
		}).WithTimeout(30 * time.Second).WithContext(ctx).Should(o.Succeed())
	})
})

// createScyllaOperatorConfig creates a ScyllaOperatorConfig singleton (name="cluster") with
// the minimal status fields required by the ScyllaDBMonitoring controller.
func createScyllaOperatorConfig(ctx context.Context, e *envtest.Environment) *scyllav1alpha1.ScyllaOperatorConfig {
	g.GinkgoHelper()

	soc := &scyllav1alpha1.ScyllaOperatorConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: naming.SingletonName,
		},
		Spec: scyllav1alpha1.ScyllaOperatorConfigSpec{
			ScyllaUtilsImage: "docker.io/scylladb/scylla:6.2.0",
		},
	}

	soc, err := e.ScyllaClient().ScyllaV1alpha1().ScyllaOperatorConfigs().Create(ctx, soc, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to create ScyllaOperatorConfig")

	// Populate the status fields that the controller reads.
	socCopy := soc.DeepCopy()
	socCopy.Status.GrafanaImage = pointer.Ptr("docker.io/grafana/grafana:10.0.0")
	socCopy.Status.BashToolsImage = pointer.Ptr("docker.io/scylladb/scylla-operator-bash-tools:latest")
	socCopy.Status.PrometheusVersion = pointer.Ptr("2.48.0")
	soc, err = e.ScyllaClient().ScyllaV1alpha1().ScyllaOperatorConfigs().UpdateStatus(ctx, socCopy, metav1.UpdateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to update ScyllaOperatorConfig status")

	return soc
}

// createScyllaDBMonitoring creates the given ScyllaDBMonitoring and returns the created object.
func createScyllaDBMonitoring(ctx context.Context, e *envtest.Environment, sm *scyllav1alpha1.ScyllaDBMonitoring) *scyllav1alpha1.ScyllaDBMonitoring {
	g.GinkgoHelper()

	g.By(fmt.Sprintf("Creating ScyllaDBMonitoring %q", sm.Name))
	sm, err := e.ScyllaClient().ScyllaV1alpha1().ScyllaDBMonitorings(e.Namespace()).Create(ctx, sm, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	return sm
}

// patchGrafanaDeploymentAsAvailable patches the Grafana Deployment's status conditions to simulate a
// fully available and non-progressing Deployment.
func patchGrafanaDeploymentAsAvailable(ctx context.Context, e *envtest.Environment, d *appsv1.Deployment) {
	g.GinkgoHelper()

	g.By(fmt.Sprintf("Patching Deployment %q status conditions to simulate available state", d.Name))

	dCopy := d.DeepCopy()
	dCopy.Status.Conditions = []appsv1.DeploymentCondition{
		{
			Type:               appsv1.DeploymentAvailable,
			Status:             corev1.ConditionTrue,
			Reason:             internalapi.AsExpectedReason,
			LastTransitionTime: metav1.Now(),
		},
		{
			Type:               appsv1.DeploymentProgressing,
			Status:             corev1.ConditionFalse,
			Reason:             "NewReplicaSetAvailable",
			LastTransitionTime: metav1.Now(),
		},
	}

	_, err := e.TypedKubeClient().AppsV1().Deployments(e.Namespace()).UpdateStatus(ctx, dCopy, metav1.UpdateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
}

// patchPrometheusAsAvailable patches the Prometheus CR status with Available=True and Reconciled=True.
func patchPrometheusAsAvailable(ctx context.Context, e *envtest.Environment, p *monitoringv1.Prometheus) {
	pCopy := p.DeepCopy()
	pCopy.Status.Conditions = []monitoringv1.Condition{
		{
			Type:               monitoringv1.Available,
			Status:             monitoringv1.ConditionTrue,
			Reason:             internalapi.AsExpectedReason,
			LastTransitionTime: metav1.Now(),
		},
		{
			Type:               monitoringv1.Reconciled,
			Status:             monitoringv1.ConditionTrue,
			Reason:             internalapi.AsExpectedReason,
			LastTransitionTime: metav1.Now(),
		},
	}
	_, err := e.MonitoringClient().MonitoringV1().Prometheuses(e.Namespace()).UpdateStatus(ctx, pCopy, metav1.UpdateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
}

// patchGrafanaDeploymentAsNotAvailable patches the Grafana Deployment's status conditions to simulate
// a rollout in progress: Available=False and Progressing=True.
func patchGrafanaDeploymentAsNotAvailable(ctx context.Context, e *envtest.Environment, d *appsv1.Deployment) {
	g.GinkgoHelper()

	g.By(fmt.Sprintf("Patching Deployment %q status conditions to simulate unavailable/progressing state", d.Name))

	dCopy := d.DeepCopy()
	dCopy.Status.Conditions = []appsv1.DeploymentCondition{
		{
			Type:               appsv1.DeploymentAvailable,
			Status:             corev1.ConditionFalse,
			Reason:             "MinimumReplicasUnavailable",
			Message:            "Deployment does not have minimum availability.",
			LastTransitionTime: metav1.Now(),
		},
		{
			Type:               appsv1.DeploymentProgressing,
			Status:             corev1.ConditionTrue,
			Reason:             "ReplicaSetUpdated",
			Message:            "ReplicaSet is being updated.",
			LastTransitionTime: metav1.Now(),
		},
	}

	_, err := e.TypedKubeClient().AppsV1().Deployments(e.Namespace()).UpdateStatus(ctx, dCopy, metav1.UpdateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
}

// patchPrometheusAsNotReconciled patches the Prometheus CR status with Available=False and Reconciled=False,
// simulating a reconciliation error from the Prometheus Operator.
func patchPrometheusAsNotReconciled(ctx context.Context, e *envtest.Environment, p *monitoringv1.Prometheus) {
	g.GinkgoHelper()

	pCopy := p.DeepCopy()
	pCopy.Status.Conditions = []monitoringv1.Condition{
		{
			Type:               monitoringv1.Available,
			Status:             monitoringv1.ConditionFalse,
			Reason:             "NoPodReady",
			Message:            "No Prometheus pods are ready.",
			LastTransitionTime: metav1.Now(),
		},
		{
			Type:               monitoringv1.Reconciled,
			Status:             monitoringv1.ConditionFalse,
			Reason:             "SyncFailed",
			Message:            "Prometheus operator failed to sync the resource.",
			LastTransitionTime: metav1.Now(),
		},
	}
	_, err := e.MonitoringClient().MonitoringV1().Prometheuses(e.Namespace()).UpdateStatus(ctx, pCopy, metav1.UpdateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
}

// newBasicScyllaDBMonitoring returns a minimal valid *scyllav1alpha1.ScyllaDBMonitoring
// with Prometheus in External mode (no Prometheus CR created, always considered available).
func newBasicScyllaDBMonitoring(name, namespace string) *scyllav1alpha1.ScyllaDBMonitoring {
	g.GinkgoHelper()

	smType := scyllav1alpha1.ScyllaDBMonitoringTypePlatform
	return &scyllav1alpha1.ScyllaDBMonitoring{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: scyllav1alpha1.ScyllaDBMonitoringSpec{
			EndpointsSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": name,
				},
			},
			Type: &smType,
			Components: &scyllav1alpha1.Components{
				Prometheus: &scyllav1alpha1.PrometheusSpec{
					Mode:      scyllav1alpha1.PrometheusModeExternal,
					Resources: corev1.ResourceRequirements{},
				},
				Grafana: &scyllav1alpha1.GrafanaSpec{
					Resources: corev1.ResourceRequirements{},
					Authentication: scyllav1alpha1.GrafanaAuthentication{
						InsecureEnableAnonymousAccess: true,
					},
				},
			},
		},
	}
}

// newManagedScyllaDBMonitoring returns a minimal valid *scyllav1alpha1.ScyllaDBMonitoring
// with Prometheus in Managed mode (a Prometheus CR is created and managed).
func newManagedScyllaDBMonitoring(name, namespace string) *scyllav1alpha1.ScyllaDBMonitoring {
	g.GinkgoHelper()

	smType := scyllav1alpha1.ScyllaDBMonitoringTypePlatform
	return &scyllav1alpha1.ScyllaDBMonitoring{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: scyllav1alpha1.ScyllaDBMonitoringSpec{
			EndpointsSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": name,
				},
			},
			Type: &smType,
			Components: &scyllav1alpha1.Components{
				Prometheus: &scyllav1alpha1.PrometheusSpec{
					Mode:      scyllav1alpha1.PrometheusModeManaged,
					Resources: corev1.ResourceRequirements{},
				},
				Grafana: &scyllav1alpha1.GrafanaSpec{
					Resources: corev1.ResourceRequirements{},
					Authentication: scyllav1alpha1.GrafanaAuthentication{
						InsecureEnableAnonymousAccess: true,
					},
				},
			},
		},
	}
}

// runScyllaDBMonitoringController creates and starts a ScyllaDBMonitoring controller against the given
// envtest environment. The controller is stopped automatically when ctx is cancelled.
func runScyllaDBMonitoringController(ctx context.Context, e *envtest.Environment) {
	g.GinkgoHelper()

	const resyncPeriod = 12 * time.Hour

	monitoringClient, err := monitoringversionedclient.NewForConfig(e.Config())
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to create monitoring versioned client")

	kubeInformers := informers.NewSharedInformerFactoryWithOptions(
		e.TypedKubeClient(),
		resyncPeriod,
		informers.WithNamespace(e.Namespace()),
	)
	scyllaInformers := scyllainformers.NewSharedInformerFactoryWithOptions(
		e.ScyllaClient(),
		resyncPeriod,
		scyllainformers.WithNamespace(e.Namespace()),
	)
	scyllaGlobalInformers := scyllainformers.NewSharedInformerFactory(
		e.ScyllaClient(),
		resyncPeriod,
	)
	monitoringInformers := monitoringinformers.NewSharedInformerFactoryWithOptions(
		monitoringClient,
		resyncPeriod,
		monitoringinformers.WithNamespace(e.Namespace()),
	)

	// RSA key generator: min=1, max=1, small key size for fast tests.
	keyGenerator, err := crypto.NewRSAKeyGenerator(1, 1, 1024, 42*time.Hour)
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to create RSA key generator")

	mc, err := scylladbmonitoring.NewController(
		e.TypedKubeClient(),
		e.ScyllaClient().ScyllaV1alpha1(),
		monitoringClient.MonitoringV1(),
		scyllaGlobalInformers.Scylla().V1alpha1().ScyllaOperatorConfigs(),
		kubeInformers.Core().V1().ConfigMaps(),
		kubeInformers.Core().V1().Secrets(),
		kubeInformers.Core().V1().Services(),
		kubeInformers.Core().V1().ServiceAccounts(),
		kubeInformers.Rbac().V1().RoleBindings(),
		kubeInformers.Policy().V1().PodDisruptionBudgets(),
		kubeInformers.Apps().V1().Deployments(),
		kubeInformers.Networking().V1().Ingresses(),
		scyllaInformers.Scylla().V1alpha1().ScyllaDBMonitorings(),
		monitoringInformers.Monitoring().V1().Prometheuses(),
		monitoringInformers.Monitoring().V1().PrometheusRules(),
		monitoringInformers.Monitoring().V1().ServiceMonitors(),
		keyGenerator,
	)
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to create ScyllaDBMonitoring controller")

	kubeInformers.Start(ctx.Done())
	scyllaInformers.Start(ctx.Done())
	scyllaGlobalInformers.Start(ctx.Done())
	monitoringInformers.Start(ctx.Done())

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		keyGenerator.Run(ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		mc.Run(ctx, 1)
	}()

	g.DeferCleanup(func() {
		kubeInformers.Shutdown()
		scyllaInformers.Shutdown()
		scyllaGlobalInformers.Shutdown()
		monitoringInformers.Shutdown()
		wg.Wait()
	})
}
