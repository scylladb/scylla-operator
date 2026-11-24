//go:build envtest

package controllers

import (
	"context"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controller/scylladbmanagertask"
	"github.com/scylladb/scylla-operator/test/envtest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// scyllaDBManagerTaskTriggerFixture is the state the trigger rows change: a task whose ScyllaDBDatacenter has a
// registration, and another task whose ScyllaDBDatacenter has none yet. The registration has no ScyllaDB Manager
// cluster ID, so both tasks settle waiting, without calling ScyllaDB Manager.
type scyllaDBManagerTaskTriggerFixture struct {
	triggerActions

	env      *envtest.Environment
	recorder *reconcileRecorder

	task         *scyllav1alpha1.ScyllaDBManagerTask
	otherTask    *scyllav1alpha1.ScyllaDBManagerTask
	registration *scyllav1alpha1.ScyllaDBManagerClusterRegistration
}

// makeEnvtestScyllaDBManagerTask returns a repair task named name for the ScyllaDBDatacenter named sdcName.
func makeEnvtestScyllaDBManagerTask(namespace, name, sdcName string) *scyllav1alpha1.ScyllaDBManagerTask {
	return &scyllav1alpha1.ScyllaDBManagerTask{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
			ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
				Kind: scyllav1alpha1.ScyllaDBDatacenterGVK.Kind,
				Name: sdcName,
			},
			Type:   scyllav1alpha1.ScyllaDBManagerTaskTypeRepair,
			Repair: &scyllav1alpha1.ScyllaDBManagerRepairTaskOptions{},
		},
	}
}

// These rows check which ScyllaDBManagerTasks each kind of change reconciles. The rows run in order against one
// fixture, and a failing row doesn't stop the rest. The registration rows create, and then delete, the other task's
// registration; the other task is deleted last, once nothing keeps its finalizer.
var _ = g.Describe("ScyllaDBManagerTask controller triggers", g.Ordered, g.ContinueOnFailure, func() {
	const (
		taskName                  = "envtest-task"
		otherTaskName             = "envtest-other-task"
		datacenterName            = "envtest-sdc"
		otherDatacenterName       = "envtest-other-sdc"
		datacenterWithoutTaskName = "envtest-sdc-without-task"
		otherNamespaceName        = "envtest-other"
	)

	var f *scyllaDBManagerTaskTriggerFixture

	g.BeforeAll(func(ctx g.SpecContext) {
		env := envtest.Setup(ctx)
		recorder := &reconcileRecorder{}

		g.By("Running ScyllaDBManagerTask controller with a reconcile observer")
		// A setup node's context ends with the node; the controller runs for the whole container. Cleanups run in
		// reverse order, so the context is cancelled before the runner's cleanup waits for the controller to stop.
		controllerCtx, cancel := context.WithCancel(context.Background())
		runScyllaDBManagerTaskController(controllerCtx, env, scylladbmanagertask.WithOnReconcile(recorder.observe))
		g.DeferCleanup(cancel)

		g.By("Creating the task's registration")
		registration, err := env.ScyllaClient().ScyllaV1alpha1().ScyllaDBManagerClusterRegistrations(env.Namespace()).Create(ctx, makeEnvtestScyllaDBManagerClusterRegistration(env.Namespace(), datacenterName), metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Creating the tasks")
		task, err := env.ScyllaClient().ScyllaV1alpha1().ScyllaDBManagerTasks(env.Namespace()).Create(ctx, makeEnvtestScyllaDBManagerTask(env.Namespace(), taskName, datacenterName), metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		otherTask, err := env.ScyllaClient().ScyllaV1alpha1().ScyllaDBManagerTasks(env.Namespace()).Create(ctx, makeEnvtestScyllaDBManagerTask(env.Namespace(), otherTaskName, otherDatacenterName), metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for the tasks to wait for their registrations")
		for name, reason := range map[string]string{
			taskName:      "AwaitingScyllaDBManagerClusterRegistrationClusterIDPropagation",
			otherTaskName: "AwaitingScyllaDBManagerClusterRegistrationCreation",
		} {
			o.Eventually(func(eo o.Gomega, ctx context.Context) {
				smt, err := env.ScyllaClient().ScyllaV1alpha1().ScyllaDBManagerTasks(env.Namespace()).Get(ctx, name, metav1.GetOptions{})
				eo.Expect(err).NotTo(o.HaveOccurred())
				eo.Expect(smt.Status.Conditions).To(o.ContainElement(o.And(
					o.HaveField("Reason", reason),
					o.HaveField("Status", metav1.ConditionTrue),
				)))
			}).WithContext(ctx).WithTimeout(triggerTimeout).Should(o.Succeed())
		}

		g.By("Creating another Namespace")
		_, err = env.TypedKubeClient().CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: otherNamespaceName,
			},
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		f = &scyllaDBManagerTaskTriggerFixture{
			triggerActions: triggerActions{client: env.KubeClient()},
			env:            env,
			recorder:       recorder,
			task:           task,
			otherTask:      otherTask,
			registration:   registration,
		}
	})

	g.DescribeTable("reconciles the ScyllaDBManagerTasks a change concerns",
		func(ctx g.SpecContext, row triggerRow[*scyllaDBManagerTaskTriggerFixture]) {
			runTriggerRow(ctx, f.recorder, f, row)
		},

		g.Entry("task update", triggerRow[*scyllaDBManagerTaskTriggerFixture]{
			change: func(ctx context.Context, f *scyllaDBManagerTaskTriggerFixture) {
				f.annotate(ctx, f.task.DeepCopy())
			},
			expected: []string{taskName},
		}),
		g.Entry("other task update", triggerRow[*scyllaDBManagerTaskTriggerFixture]{
			change: func(ctx context.Context, f *scyllaDBManagerTaskTriggerFixture) {
				f.annotate(ctx, f.otherTask.DeepCopy())
			},
			expected: []string{otherTaskName},
		}),

		// The rows below exercise the registration watch, reaching the tasks in the registration's namespace whose
		// ScyllaDBDatacenter the registration is named after.
		g.Entry("registration update", triggerRow[*scyllaDBManagerTaskTriggerFixture]{
			change: func(ctx context.Context, f *scyllaDBManagerTaskTriggerFixture) {
				f.annotate(ctx, f.registration.DeepCopy())
			},
			expected: []string{taskName},
		}),
		g.Entry("other task's registration creation", triggerRow[*scyllaDBManagerTaskTriggerFixture]{
			change: func(ctx context.Context, f *scyllaDBManagerTaskTriggerFixture) {
				f.create(ctx, makeEnvtestScyllaDBManagerClusterRegistration(f.env.Namespace(), otherDatacenterName))
			},
			expected: []string{otherTaskName},
		}),
		g.Entry("registration of a ScyllaDBDatacenter with no task creation", triggerRow[*scyllaDBManagerTaskTriggerFixture]{
			change: func(ctx context.Context, f *scyllaDBManagerTaskTriggerFixture) {
				f.create(ctx, makeEnvtestScyllaDBManagerClusterRegistration(f.env.Namespace(), datacenterWithoutTaskName))
			},
		}),
		g.Entry("registration with the task's name in another Namespace creation", triggerRow[*scyllaDBManagerTaskTriggerFixture]{
			change: func(ctx context.Context, f *scyllaDBManagerTaskTriggerFixture) {
				f.create(ctx, makeEnvtestScyllaDBManagerClusterRegistration(otherNamespaceName, datacenterName))
			},
		}),
		g.Entry("other task's registration deletion", triggerRow[*scyllaDBManagerTaskTriggerFixture]{
			change: func(ctx context.Context, f *scyllaDBManagerTaskTriggerFixture) {
				f.delete(ctx, makeEnvtestScyllaDBManagerClusterRegistration(f.env.Namespace(), otherDatacenterName))
			},
			expected: []string{otherTaskName},
		}),

		// With no registration, the controller removes the task's finalizer without calling ScyllaDB Manager.
		g.Entry("other task deletion", triggerRow[*scyllaDBManagerTaskTriggerFixture]{
			change: func(ctx context.Context, f *scyllaDBManagerTaskTriggerFixture) {
				f.delete(ctx, f.otherTask.DeepCopy())
			},
			expected: []string{otherTaskName},
		}),
	)
})
