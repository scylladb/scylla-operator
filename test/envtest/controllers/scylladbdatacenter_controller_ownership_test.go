//go:build envtest

package controllers

import (
	"context"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/internalapi"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/test/envtest"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryutilintstr "k8s.io/apimachinery/pkg/util/intstr"
	apimachineryutiluuid "k8s.io/apimachinery/pkg/util/uuid"
)

// The controller owns every object labelled with its cluster label: the ones it created, the orphans it adopts, and
// nothing owned by another controller. These specs pin what it does with each of them.
var _ = g.Describe("ScyllaDBDatacenter controller object ownership", func() {
	const rackName = "rack-a"

	var env *envtest.Environment
	g.BeforeEach(func(ctx g.SpecContext) {
		env = envtest.Setup(ctx)
	})

	// The excess object carries the controller's labels and a controllerRef to the datacenter, as a leftover of a
	// previous shape of the datacenter would.
	g.DescribeTable("should prune an owned object that isn't required and keep the required one",
		func(ctx g.SpecContext, kind ownedObjectKind) {
			sdc := setupRolledOutRacks(ctx, env, false, []string{rackName}, 1)

			g.By(fmt.Sprintf("Creating an excess owned %s", kind.name))
			excessObject, err := kind.create(ctx, env, metav1.ObjectMeta{
				Name:            fmt.Sprintf("excess-%s", strings.ToLower(kind.name)),
				Namespace:       env.Namespace(),
				Labels:          naming.ScyllaDBDatacenterSelectorLabels(sdc),
				OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(sdc, scyllav1alpha1.ScyllaDBDatacenterGVK)},
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("Waiting for the excess %s to be pruned", kind.name))
			o.Eventually(func(eo o.Gomega, ctx context.Context) {
				_, err := kind.get(ctx, env, excessObject.GetName())
				eo.Expect(apierrors.IsNotFound(err)).To(o.BeTrue(), "%s %q should be pruned: %v", kind.name, excessObject.GetName(), err)
			}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())

			if kind.requiredName == nil {
				return
			}

			g.By(fmt.Sprintf("Verifying the required %s is kept", kind.name))
			requiredObject, err := kind.get(ctx, env, kind.requiredName(sdc))
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(metav1.IsControlledBy(requiredObject, sdc)).To(o.BeTrue())
		},
		g.Entry("PodDisruptionBudget", ownedObjectKind{
			name: "PodDisruptionBudget",
			create: func(ctx context.Context, e *envtest.Environment, objectMeta metav1.ObjectMeta) (metav1.Object, error) {
				return e.TypedKubeClient().PolicyV1().PodDisruptionBudgets(e.Namespace()).Create(ctx, &policyv1.PodDisruptionBudget{
					ObjectMeta: objectMeta,
					Spec:       makeEnvtestPodDisruptionBudgetSpec(pointer.Ptr(apimachineryutilintstr.FromInt32(3))),
				}, metav1.CreateOptions{})
			},
			get: func(ctx context.Context, e *envtest.Environment, name string) (metav1.Object, error) {
				return e.TypedKubeClient().PolicyV1().PodDisruptionBudgets(e.Namespace()).Get(ctx, name, metav1.GetOptions{})
			},
			requiredName: func(sdc *scyllav1alpha1.ScyllaDBDatacenter) string {
				return naming.PodDisruptionBudgetName(sdc)
			},
		}),
		g.Entry("RoleBinding", ownedObjectKind{
			name: "RoleBinding",
			create: func(ctx context.Context, e *envtest.Environment, objectMeta metav1.ObjectMeta) (metav1.Object, error) {
				return e.TypedKubeClient().RbacV1().RoleBindings(e.Namespace()).Create(ctx, &rbacv1.RoleBinding{
					ObjectMeta: objectMeta,
					RoleRef: rbacv1.RoleRef{
						APIGroup: rbacv1.GroupName,
						Kind:     "ClusterRole",
						Name:     "view",
					},
				}, metav1.CreateOptions{})
			},
			get: func(ctx context.Context, e *envtest.Environment, name string) (metav1.Object, error) {
				return e.TypedKubeClient().RbacV1().RoleBindings(e.Namespace()).Get(ctx, name, metav1.GetOptions{})
			},
			requiredName: func(sdc *scyllav1alpha1.ScyllaDBDatacenter) string {
				return naming.MemberServiceAccountNameForScyllaDBDatacenter(sdc.Name)
			},
		}),
		g.Entry("ServiceAccount", ownedObjectKind{
			name: "ServiceAccount",
			create: func(ctx context.Context, e *envtest.Environment, objectMeta metav1.ObjectMeta) (metav1.Object, error) {
				return e.TypedKubeClient().CoreV1().ServiceAccounts(e.Namespace()).Create(ctx, &corev1.ServiceAccount{
					ObjectMeta: objectMeta,
				}, metav1.CreateOptions{})
			},
			get: func(ctx context.Context, e *envtest.Environment, name string) (metav1.Object, error) {
				return e.TypedKubeClient().CoreV1().ServiceAccounts(e.Namespace()).Get(ctx, name, metav1.GetOptions{})
			},
			requiredName: func(sdc *scyllav1alpha1.ScyllaDBDatacenter) string {
				return naming.MemberServiceAccountNameForScyllaDBDatacenter(sdc.Name)
			},
		}),
		// No Ingress is required by a datacenter that doesn't expose its nodes.
		g.Entry("Ingress", ownedObjectKind{
			name: "Ingress",
			create: func(ctx context.Context, e *envtest.Environment, objectMeta metav1.ObjectMeta) (metav1.Object, error) {
				return e.TypedKubeClient().NetworkingV1().Ingresses(e.Namespace()).Create(ctx, &networkingv1.Ingress{
					ObjectMeta: objectMeta,
					Spec:       makeEnvtestIngressSpec("excess.envtest.scylladb.local"),
				}, metav1.CreateOptions{})
			},
			get: func(ctx context.Context, e *envtest.Environment, name string) (metav1.Object, error) {
				return e.TypedKubeClient().NetworkingV1().Ingresses(e.Namespace()).Get(ctx, name, metav1.GetOptions{})
			},
		}),
	)

	// An orphan carries no controllerRef, so its events don't enqueue the datacenter, and it is adopted by the next
	// sync that runs for any other reason. The orphans are created before the datacenter, so that its first sync
	// finds them.
	g.It("should adopt orphaned objects with matching labels", func(ctx g.SpecContext) {
		g.By("Running ScyllaDBDatacenter controller")
		runScyllaDBDatacenterController(ctx, env)

		g.By("Creating ScyllaOperatorConfig singleton")
		createScyllaOperatorConfig(ctx, env)

		sdc := makeEnvtestScyllaDBDatacenter(env.Namespace(), []string{rackName})

		orphanObjectMeta := func(name string) metav1.ObjectMeta {
			return metav1.ObjectMeta{
				Name:      name,
				Namespace: env.Namespace(),
				Labels:    naming.ScyllaDBDatacenterSelectorLabels(sdc),
			}
		}

		// A ConfigMap is adopted and then left alone, as no ConfigMap is ever pruned.
		g.By("Creating an orphaned ConfigMap with the datacenter labels")
		orphanConfigMap, err := env.TypedKubeClient().CoreV1().ConfigMaps(env.Namespace()).Create(ctx, &corev1.ConfigMap{
			ObjectMeta: orphanObjectMeta("orphan-configmap"),
			Data:       map[string]string{"key": "value"},
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		// A PodDisruptionBudget of a name that isn't required is adopted and then pruned as any other owned excess
		// object.
		g.By("Creating an orphaned excess PodDisruptionBudget with the datacenter labels")
		orphanExcessPDB, err := env.TypedKubeClient().PolicyV1().PodDisruptionBudgets(env.Namespace()).Create(ctx, &policyv1.PodDisruptionBudget{
			ObjectMeta: orphanObjectMeta("orphan-excess-pdb"),
			Spec:       makeEnvtestPodDisruptionBudgetSpec(pointer.Ptr(apimachineryutilintstr.FromInt32(3))),
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Creating a ScyllaDBDatacenter with a single rack")
		sdc, err = env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).Create(ctx, sdc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for the orphaned ConfigMap to be adopted")
		o.Eventually(func(eo o.Gomega, ctx context.Context) {
			cm, err := env.TypedKubeClient().CoreV1().ConfigMaps(env.Namespace()).Get(ctx, orphanConfigMap.Name, metav1.GetOptions{})
			eo.Expect(err).NotTo(o.HaveOccurred())
			eo.Expect(cm.UID).To(o.Equal(orphanConfigMap.UID))
			eo.Expect(metav1.IsControlledBy(cm, sdc)).To(o.BeTrue())
			eo.Expect(cm.Data).To(o.Equal(orphanConfigMap.Data))
		}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())

		g.By("Waiting for the orphaned excess PodDisruptionBudget to be adopted and pruned")
		o.Eventually(func(eo o.Gomega, ctx context.Context) {
			_, err := env.TypedKubeClient().PolicyV1().PodDisruptionBudgets(env.Namespace()).Get(ctx, orphanExcessPDB.Name, metav1.GetOptions{})
			eo.Expect(apierrors.IsNotFound(err)).To(o.BeTrue(), "PodDisruptionBudget %q should be pruned: %v", orphanExcessPDB.Name, err)
		}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())
	})

	g.It("should report the StatefulSet sync as degraded and leave a rack StatefulSet owned by another controller untouched", func(ctx g.SpecContext) {
		g.By("Running ScyllaDBDatacenter controller")
		runScyllaDBDatacenterController(ctx, env)

		g.By("Creating ScyllaOperatorConfig singleton")
		createScyllaOperatorConfig(ctx, env)

		sdc := makeEnvtestScyllaDBDatacenter(env.Namespace(), []string{rackName})
		rackStatefulSetName := naming.StatefulSetNameForRack(sdc.Spec.Racks[0], sdc)

		g.By("Creating a StatefulSet under the rack's name owned by another controller")
		foreignStatefulSet := makeEnvtestForeignStatefulSet(env.Namespace(), rackStatefulSetName, naming.ScyllaDBDatacenterSelectorLabels(sdc))
		foreignStatefulSet.OwnerReferences = []metav1.OwnerReference{makeEnvtestForeignControllerRef()}
		foreignStatefulSet, err := env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()).Create(ctx, foreignStatefulSet, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Creating a ScyllaDBDatacenter with a single rack")
		sdc, err = env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).Create(ctx, sdc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for the StatefulSet sync to be reported as degraded")
		o.Eventually(func(eo o.Gomega, ctx context.Context) {
			degradedCondition := getScyllaDBDatacenterCondition(ctx, env, sdc.Name, internalapi.MakeKindControllerCondition("StatefulSet", scyllav1alpha1.DegradedCondition))
			eo.Expect(degradedCondition).NotTo(o.BeNil())
			eo.Expect(degradedCondition.Status).To(o.Equal(metav1.ConditionTrue))
			eo.Expect(degradedCondition.Reason).To(o.Equal(internalapi.ErrorReason))

			aggregatedDegradedCondition := getScyllaDBDatacenterCondition(ctx, env, sdc.Name, scyllav1alpha1.DegradedCondition)
			eo.Expect(aggregatedDegradedCondition).NotTo(o.BeNil())
			eo.Expect(aggregatedDegradedCondition.Status).To(o.Equal(metav1.ConditionTrue))
		}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())

		g.By("Verifying the StatefulSet is left untouched")
		o.Consistently(func(co o.Gomega, ctx context.Context) {
			sts, err := env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()).Get(ctx, rackStatefulSetName, metav1.GetOptions{})
			co.Expect(err).NotTo(o.HaveOccurred())
			co.Expect(sts.UID).To(o.Equal(foreignStatefulSet.UID))
			co.Expect(sts.Generation).To(o.Equal(foreignStatefulSet.Generation))
			co.Expect(sts.OwnerReferences).To(o.Equal(foreignStatefulSet.OwnerReferences))
			co.Expect(sts.DeletionTimestamp).To(o.BeNil())
		}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultConsistentlyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())
	})

	g.It("should delete the StatefulSet of a removed rack and drop its rack status", func(ctx g.SpecContext) {
		const removedRackName = "rack-b"

		sdc := setupRolledOutRacks(ctx, env, false, []string{rackName, removedRackName}, 1)
		removedRackStatefulSetName := naming.StatefulSetNameForRack(sdc.Spec.Racks[1], sdc)
		removedRackServiceName := naming.MemberServiceName(sdc.Spec.Racks[1], sdc, 0)

		g.By(fmt.Sprintf("Scaling the %q rack down to zero nodes", removedRackName))
		scaleRack(ctx, env, sdc.Name, removedRackName, 0)

		g.By("Marking the leaving node as decommissioned in place of the sidecar")
		waitForServiceDecommissionedLabel(ctx, env, removedRackServiceName, naming.LabelValueFalse)
		setServiceDecommissionedLabel(ctx, env, removedRackServiceName, naming.LabelValueTrue)

		g.By("Waiting for the rack StatefulSet to be scaled down to zero replicas and the node to be pruned")
		waitForStatefulSetReplicas(ctx, env, removedRackStatefulSetName, 0)
		waitForServiceToBePrunedAndRecordToDrain(ctx, env, sdc.Name, removedRackName, removedRackServiceName)
		markStatefulSetAsRolledOut(ctx, env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()), removedRackStatefulSetName)

		g.By("Waiting for the rack status to report no nodes for the current generation")
		o.Eventually(func(eo o.Gomega, ctx context.Context) {
			sdc, err := env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).Get(ctx, sdc.Name, metav1.GetOptions{})
			eo.Expect(err).NotTo(o.HaveOccurred())
			eo.Expect(sdc.Status.ObservedGeneration).To(o.HaveValue(o.Equal(sdc.Generation)))

			rackStatus := getRackStatus(ctx, env, sdc.Name, removedRackName)
			eo.Expect(rackStatus).NotTo(o.BeNil())
			eo.Expect(rackStatus.Nodes).To(o.HaveValue(o.BeEquivalentTo(0)))
			eo.Expect(rackStatus.Stale).To(o.HaveValue(o.BeFalse()))
			eo.Expect(rackStatus.DecommissioningNodes).To(o.BeEmpty())
		}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())

		g.By(fmt.Sprintf("Removing the %q rack from the spec", removedRackName))
		updateScyllaDBDatacenter(ctx, env, sdc.Name, func(sdc *scyllav1alpha1.ScyllaDBDatacenter) {
			sdc.Spec.Racks = makeRackSpecs(rackName)
		})

		g.By("Waiting for the rack StatefulSet to be deleted and the rack status to be dropped")
		o.Eventually(func(eo o.Gomega, ctx context.Context) {
			_, err := env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()).Get(ctx, removedRackStatefulSetName, metav1.GetOptions{})
			eo.Expect(apierrors.IsNotFound(err)).To(o.BeTrue(), "StatefulSet %q should be deleted: %v", removedRackStatefulSetName, err)

			sdc, err := env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).Get(ctx, sdc.Name, metav1.GetOptions{})
			eo.Expect(err).NotTo(o.HaveOccurred())
			eo.Expect(sdc.Status.ObservedGeneration).To(o.HaveValue(o.Equal(sdc.Generation)))
			eo.Expect(sdc.Status.Racks).To(o.HaveLen(1))
			eo.Expect(sdc.Status.Racks[0].Name).To(o.Equal(rackName))
		}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())

		g.By("Verifying the remaining rack StatefulSet is kept")
		remainingStatefulSet, err := env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()).Get(ctx, naming.StatefulSetNameForRack(sdc.Spec.Racks[0], sdc), metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(remainingStatefulSet.DeletionTimestamp).To(o.BeNil())
	})
})

// ownedObjectKind is a kind of object the controller owns through its selector labels, with the accessors a spec
// needs to create an excess one and to look one up.
type ownedObjectKind struct {
	name   string
	create func(ctx context.Context, e *envtest.Environment, objectMeta metav1.ObjectMeta) (metav1.Object, error)
	get    func(ctx context.Context, e *envtest.Environment, name string) (metav1.Object, error)
	// requiredName returns the name of the object of this kind the datacenter requires, or is nil when it requires
	// none.
	requiredName func(sdc *scyllav1alpha1.ScyllaDBDatacenter) string
}

// getScyllaDBDatacenterCondition returns the condition of the given type from the ScyllaDBDatacenter status, or nil
// if there is none.
func getScyllaDBDatacenterCondition(ctx context.Context, e *envtest.Environment, sdcName, conditionType string) *metav1.Condition {
	g.GinkgoHelper()

	sdc, err := e.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(e.Namespace()).Get(ctx, sdcName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	return apimeta.FindStatusCondition(sdc.Status.Conditions, conditionType)
}

// makeEnvtestForeignControllerRef returns a controllerRef to a ScyllaDBDatacenter that doesn't exist, standing in for
// any controller other than the one under test.
func makeEnvtestForeignControllerRef() metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion:         scyllav1alpha1.ScyllaDBDatacenterGVK.GroupVersion().String(),
		Kind:               scyllav1alpha1.ScyllaDBDatacenterGVK.Kind,
		Name:               "foreign-owner",
		UID:                apimachineryutiluuid.NewUUID(),
		Controller:         pointer.Ptr(true),
		BlockOwnerDeletion: pointer.Ptr(true),
	}
}

// makeEnvtestForeignStatefulSet returns a valid StatefulSet of the given name and labels that isn't shaped like a rack
// StatefulSet of the controller. It has no owner; the caller sets one.
func makeEnvtestForeignStatefulSet(namespace, name string, labels map[string]string) *appsv1.StatefulSet {
	podLabels := map[string]string{"app": "foreign"}

	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    pointer.Ptr(int32(1)),
			ServiceName: name,
			Selector:    &metav1.LabelSelector{MatchLabels: podLabels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: podLabels},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "foreign",
							Image: "foreign:envtest",
						},
					},
				},
			},
		},
	}
}

func makeEnvtestPodDisruptionBudgetSpec(maxUnavailable *apimachineryutilintstr.IntOrString) policyv1.PodDisruptionBudgetSpec {
	return policyv1.PodDisruptionBudgetSpec{
		MaxUnavailable: maxUnavailable,
		Selector:       &metav1.LabelSelector{MatchLabels: map[string]string{"app": "excess"}},
	}
}

func makeEnvtestIngressSpec(host string) networkingv1.IngressSpec {
	pathType := networkingv1.PathTypePrefix

	return networkingv1.IngressSpec{
		Rules: []networkingv1.IngressRule{
			{
				Host: host,
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{
							{
								Path:     "/",
								PathType: &pathType,
								Backend: networkingv1.IngressBackend{
									Service: &networkingv1.IngressServiceBackend{
										Name: "excess",
										Port: networkingv1.ServiceBackendPort{Number: 9042},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}
