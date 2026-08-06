// Copyright (C) 2026 ScyllaDB

package operatorupgrade

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/containers/image/v5/docker/reference"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configassets "github.com/scylladb/scylla-operator/assets/config"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	operatorName = "scylla-operator"

	// releaseDeployScript deploys the released manifests/CRDs; masterDeployScript deploys the current checkout's bundle.
	releaseDeployScript = "hack/ci-deploy-release.sh"
	masterDeployScript  = "hack/ci-deploy.sh"
)

var _ = g.Describe("Operator Upgrade", framework.SuiteOperatorUpgrade, func() {
	var f *framework.Framework

	g.BeforeEach(func() {
		f = framework.NewFramework("operator-upgrade")
	})

	g.It("should upgrade the operator between the configured versions", g.SpecTimeout(testTimeout), func(ctx g.SpecContext) {
		// The upgrade endpoints are independent: each is either a released version (resolved against the released
		// operator repository) or a full image ref (e.g. a locally built image pushed to the kind registry), so the
		// test covers released -> current checkout as well as released -> released upgrades.
		upgradeFromImageRef := getOperatorImageRef(framework.TestContext.OperatorUpgradeFrom)
		upgradeToImageRef := getOperatorImageRef(framework.TestContext.OperatorUpgradeTo)

		// The deploy scripts write the rendered manifests under the given dir; use a per-phase sub-dir of the
		// suite's artifacts dir (--artifacts-dir) so both the initial and upgraded manifests are preserved.
		// Absolute, so the bundles land there regardless of the per-deploy working directory.
		artifactsDir, err := filepath.Abs(framework.TestContext.ArtifactsDir)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Verifying the operator is not deployed yet")
		_, err = f.KubeAdminClient().AppsV1().Deployments(operatorName).Get(ctx, operatorName, metav1.GetOptions{})
		o.Expect(apierrors.IsNotFound(err)).To(o.BeTrue(), "expected no operator deployment before the test deploys it, got err=%v", err)

		framework.By("Deploying the released operator stack (%s)", upgradeFromImageRef)
		ciDeploy(ctx, upgradeFromImageRef, framework.TestContext.OperatorUpgradeFromDeployDir, filepath.Join(artifactsDir, "initial-manifest-bundle"))
		o.Expect(getOperatorImage(ctx, f)).To(o.Equal(upgradeFromImageRef))

		framework.By("Creating a ScyllaCluster")
		sc := f.GetDefaultScyllaCluster()
		sc, err = f.ScyllaAdminClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to roll out (RV=%s)", sc.ResourceVersion)
		initialRolloutCtx, initialRolloutCtxCancel := utils.ContextForRollout(ctx, sc)
		defer initialRolloutCtxCancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(initialRolloutCtx, f.ScyllaAdminClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Upgrading the operator stack to image %s", upgradeToImageRef)
		ciDeploy(ctx, upgradeToImageRef, framework.TestContext.OperatorUpgradeToDeployDir, filepath.Join(artifactsDir, "upgraded-manifest-bundle"))
		o.Expect(getOperatorImage(ctx, f)).To(o.Equal(upgradeToImageRef))

		// Trigger a reconciliation by the upgraded operator and make sure it rolls the ScyllaCluster back to a stable state.
		framework.By("Triggering reconciliation by the upgraded operator")
		sc, err = utils.PatchScyllaClusterForceRedeploymentReason(ctx, f.ScyllaAdminClient().ScyllaV1().ScyllaClusters(f.Namespace()), sc.Name, "operator-upgrade")
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to stabilize after the upgrade (RV=%s)", sc.ResourceVersion)
		postUpgradeRolloutCtx, postUpgradeRolloutCtxCancel := utils.ContextForRollout(ctx, sc)
		defer postUpgradeRolloutCtxCancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(postUpgradeRolloutCtx, f.ScyllaAdminClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		// Verify CQL connectivity via `kubectl exec`-style access through the API server (cqlsh to localhost inside the
		// pod), rather than dialing the node's broadcast (Pod IP) address, which is not routable from the host this test runs on.
		framework.By("Verifying CQL connectivity")
		pod, err := f.KubeAdminClient().CoreV1().Pods(f.Namespace()).Get(ctx, utils.GetNodeName(sc, 0), metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		stdout, stderr, err := utils.ExecuteInPod(ctx, f.AdminClientConfig(), f.KubeAdminClient().CoreV1(), pod, naming.ScyllaContainerName, "cqlsh", "localhost", "-e", "SELECT key, cluster_name, data_center FROM system.local")
		o.Expect(err).NotTo(o.HaveOccurred(), "cqlsh failed: stdout=%q stderr=%q", stdout, stderr)
	})
})

// ciDeploy deploys the operator stack with the given operator image via the deploy script matching the ref. The
// script path is relative, so it resolves within deployDir — the repository tree whose scripts and manifests are
// deployed (empty means the current directory, i.e. the repository root). The script sets the operator image from
// the ref and blocks on `kubectl rollout status`, so on return the operator is running the target image and available.
func ciDeploy(ctx context.Context, operatorImageRef, deployDir, artifactsDir string) {
	deployScript := getDeployScriptForImageRef(operatorImageRef)

	cmd := exec.CommandContext(ctx, deployScript, operatorImageRef)
	cmd.Dir = deployDir
	cmd.Env = append(os.Environ(), "REENTRANT=true", fmt.Sprintf("ARTIFACTS=%s", artifactsDir))
	output, err := cmd.CombinedOutput()
	o.Expect(err).NotTo(o.HaveOccurred(), "%s failed: %s", deployScript, string(output))
}

// getDeployScriptForImageRef picks the deploy script for the given operator image. A released image (explicit tag
// other than "latest") deploys via releaseDeployScript, which resolves the manifests from the image's
// org.opencontainers.image.{source,revision} labels, so the deployed manifests belong to that exact version.
// Anything else — a "latest" tag or an untagged/digest-only ref, like a locally built image pushed to the kind
// registry — deploys the current checkout's manifests via masterDeployScript.
func getDeployScriptForImageRef(operatorImageRef string) string {
	ref, err := reference.ParseAnyReference(operatorImageRef)
	o.Expect(err).NotTo(o.HaveOccurred(), "invalid operator image reference %q", operatorImageRef)

	if tagged, ok := ref.(reference.Tagged); ok && tagged.Tag() != "latest" {
		return releaseDeployScript
	}
	return masterDeployScript
}

// getOperatorImageRef resolves an operator upgrade version flag value into a fully qualified image ref: a full image
// ref (contains a repository path) is used verbatim; a bare version resolves against the released operator repository.
func getOperatorImageRef(versionOrRef string) string {
	if strings.Contains(versionOrRef, "/") {
		return versionOrRef
	}
	return fmt.Sprintf("%s:%s", configassets.OperatorImageRepository, versionOrRef)
}

// getOperatorImage returns the image of the deployed operator container, failing the spec if the deployment or the
// container is missing.
func getOperatorImage(ctx context.Context, f *framework.Framework) string {
	deployment, err := f.KubeAdminClient().AppsV1().Deployments(operatorName).Get(ctx, operatorName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	for _, c := range deployment.Spec.Template.Spec.Containers {
		if c.Name == operatorName {
			return c.Image
		}
	}
	g.Fail(fmt.Sprintf("container %q not found in deployment %q", operatorName, naming.ManualRef(deployment.Namespace, deployment.Name)))
	return ""
}
