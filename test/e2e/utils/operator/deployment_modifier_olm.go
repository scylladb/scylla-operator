// Copyright (C) 2026 ScyllaDB

package operator

import (
	"context"
	"encoding/json"
	"slices"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/scylladb/scylla-operator/pkg/gather/collect"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

const (
	olmOwnerLabel          = "olm.owner"
	olmOwnerNamespaceLabel = "olm.owner.namespace"
)

var subscriptionResourceInfo = collect.ResourceInfo{
	Resource: schema.GroupVersionResource{
		Group:    "operators.coreos.com",
		Version:  "v1alpha1",
		Resource: "subscriptions",
	},
	Scope: meta.RESTScopeNamespace,
}

// olmOperatorDeploymentModifier sets the flags as environment variables in the config of the Subscription that
// installed the operator. OLM reverts direct changes to the Deployment it manages, but renders the Subscription's
// config into it.
type olmOperatorDeploymentModifier struct {
	f                     *framework.Framework
	subscriptionNamespace string
	subscriptionName      string
}

var _ OperatorDeploymentModifier = &olmOperatorDeploymentModifier{}

func newOLMOperatorDeploymentModifier(ctx context.Context, f *framework.Framework) *olmOperatorDeploymentModifier {
	g.GinkgoHelper()

	deploy, err := f.KubeAdminClient().AppsV1().Deployments(namespace).Get(ctx, deploymentName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	subscription := getOLMSubscription(ctx, f, deploy)

	addRestoringCleaner(ctx, f, subscriptionResourceInfo, subscription.GetNamespace(), subscription.GetName())

	return &olmOperatorDeploymentModifier{
		f:                     f,
		subscriptionNamespace: subscription.GetNamespace(),
		subscriptionName:      subscription.GetName(),
	}
}

// getOLMSubscription returns the Subscription that installed the ClusterServiceVersion owning the operator Deployment.
func getOLMSubscription(ctx context.Context, f *framework.Framework, deploy *appsv1.Deployment) *unstructured.Unstructured {
	g.GinkgoHelper()

	csvName := deploy.Labels[olmOwnerLabel]
	o.Expect(csvName).NotTo(o.BeEmpty(), "label %q on Deployment %q", olmOwnerLabel, deploymentName)
	csvNamespace := deploy.Labels[olmOwnerNamespaceLabel]
	o.Expect(csvNamespace).NotTo(o.BeEmpty(), "label %q on Deployment %q", olmOwnerNamespaceLabel, deploymentName)

	subscriptionList, err := f.DynamicAdminClient().Resource(subscriptionResourceInfo.Resource).Namespace(csvNamespace).List(ctx, metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	subscriptions := slices.DeleteFunc(subscriptionList.Items, func(s unstructured.Unstructured) bool {
		installedCSV, _, _ := unstructured.NestedString(s.Object, "status", "installedCSV")
		return installedCSV != csvName
	})
	o.Expect(subscriptions).To(o.HaveLen(1), "Subscriptions in namespace %q that installed ClusterServiceVersion %q", csvNamespace, csvName)

	return &subscriptions[0]
}

func (m *olmOperatorDeploymentModifier) SetFlags(ctx context.Context, flags map[string]string) {
	g.GinkgoHelper()

	m.updateFlags(ctx, flags, nil)
}

func (m *olmOperatorDeploymentModifier) UnsetFlags(ctx context.Context, names ...string) {
	g.GinkgoHelper()

	m.updateFlags(ctx, nil, names)
}

func (m *olmOperatorDeploymentModifier) updateFlags(ctx context.Context, set map[string]string, unset []string) {
	g.GinkgoHelper()

	deploy, err := m.f.KubeAdminClient().AppsV1().Deployments(namespace).Get(ctx, deploymentName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	container, err := getOperatorContainer(deploy)
	o.Expect(err).NotTo(o.HaveOccurred())

	expectFlagsNotPassedAsArgs(container, set, unset)

	subscriptionClient := m.f.DynamicAdminClient().Resource(subscriptionResourceInfo.Resource).Namespace(m.subscriptionNamespace)
	subscription, err := subscriptionClient.Get(ctx, m.subscriptionName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	unstructuredEnv, _, err := unstructured.NestedSlice(subscription.Object, "spec", "config", "env")
	o.Expect(err).NotTo(o.HaveOccurred())
	unstructuredEnvBytes, err := json.Marshal(unstructuredEnv)
	o.Expect(err).NotTo(o.HaveOccurred())
	var env []corev1.EnvVar
	err = json.Unmarshal(unstructuredEnvBytes, &env)
	o.Expect(err).NotTo(o.HaveOccurred())

	// A merge patch replaces the whole env list.
	patchBytes, err := json.Marshal(map[string]any{
		"spec": map[string]any{
			"config": map[string]any{
				"env": setFlagEnvVars(env, set, unset),
			},
		},
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	framework.By("Patching the env of Subscription %q", m.subscriptionName)
	_, err = subscriptionClient.Patch(ctx, m.subscriptionName, types.MergePatchType, patchBytes, metav1.PatchOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	// OLM renders the Subscription's config into the Deployment asynchronously, so the Deployment can still be rolled
	// out with the previous config.
	waitForOperatorFlags(ctx, m.f, set, unset)
}
