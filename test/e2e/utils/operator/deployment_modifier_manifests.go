// Copyright (C) 2026 ScyllaDB

package operator

import (
	"context"
	"encoding/json"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/scylladb/scylla-operator/pkg/gather/collect"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var deploymentResourceInfo = collect.ResourceInfo{
	Resource: appsv1.SchemeGroupVersion.WithResource("deployments"),
	Scope:    meta.RESTScopeNamespace,
}

// manifestsOperatorDeploymentModifier sets the flags in the env of the operator Deployment, which nothing else
// reconciles when the operator is installed from the manifests.
type manifestsOperatorDeploymentModifier struct {
	f *framework.Framework
}

var _ OperatorDeploymentModifier = &manifestsOperatorDeploymentModifier{}

func newManifestsOperatorDeploymentModifier(ctx context.Context, f *framework.Framework) *manifestsOperatorDeploymentModifier {
	g.GinkgoHelper()

	addRestoringCleaner(ctx, f, deploymentResourceInfo, namespace, deploymentName)

	return &manifestsOperatorDeploymentModifier{f: f}
}

func (m *manifestsOperatorDeploymentModifier) SetFlags(ctx context.Context, flags map[string]string) {
	g.GinkgoHelper()

	m.updateFlags(ctx, flags, nil)
}

func (m *manifestsOperatorDeploymentModifier) UnsetFlags(ctx context.Context, names ...string) {
	g.GinkgoHelper()

	m.updateFlags(ctx, nil, names)
}

func (m *manifestsOperatorDeploymentModifier) updateFlags(ctx context.Context, set map[string]string, unset []string) {
	g.GinkgoHelper()

	deploy, err := m.f.KubeAdminClient().AppsV1().Deployments(namespace).Get(ctx, deploymentName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	containerIdx, err := getOperatorContainerIndex(deploy)
	o.Expect(err).NotTo(o.HaveOccurred())
	container := &deploy.Spec.Template.Spec.Containers[containerIdx]

	expectFlagsNotPassedAsArgs(container, set, unset)

	patchBytes, err := json.Marshal([]map[string]any{
		{
			"op":    "add",
			"path":  fmt.Sprintf("/spec/template/spec/containers/%d/env", containerIdx),
			"value": setFlagEnvVars(container.Env, set, unset),
		},
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	framework.By("Patching the env of the operator Deployment")
	_, err = m.f.KubeAdminClient().AppsV1().Deployments(namespace).Patch(ctx, deploymentName, types.JSONPatchType, patchBytes, metav1.PatchOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	waitForOperatorFlagEnvVars(ctx, m.f, set, unset)
}
