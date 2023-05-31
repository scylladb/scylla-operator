// Copyright (c) 2023 ScyllaDB.

package upgrade

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/set/upgrade/scyllacluster"
	"github.com/scylladb/scylla-operator/test/e2e/upgrade"
	"github.com/spf13/pflag"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
)

var (
	operatorRepoPath           string
	operatorDeployScriptPath   string
	operatorTargetVersionImage string
)

func addOperatorFlags(fs *pflag.FlagSet) {
	fs.StringVarP(&operatorRepoPath, "operator-upgrade-repo-path", "", operatorRepoPath, "Path to root directory of Scylla Operator repository.")
	fs.StringVarP(&operatorDeployScriptPath, "operator-upgrade-deploy-script", "", operatorDeployScriptPath, "Path to script which deploys Scylla Operator.")
	fs.StringVarP(&operatorTargetVersionImage, "operator-upgrade-target-image", "", operatorRepoPath, "Image to which Scylla Operator will be upgraded.")
}

var upgradeTests = []upgrade.Test{
	&scyllacluster.ScyllaClusterRolloutUpgradeTest{},
	&scyllacluster.ScyllaClusterTrafficDuringUpgrade{},
}

var _ = g.Describe("Operator upgrade", framework.Upgrade, func() {
	defer g.GinkgoRecover()

	testFrameworks := upgrade.CreateUpgradeFrameworks(upgradeTests)

	g.It("doesn't break managed objects", func(ctx context.Context) {
		upgrade.RunUpgradeSuite(ctx, upgradeTests, testFrameworks, UpgradeOperatorFunc())
	})
})

func UpgradeOperatorFunc() func(ctx context.Context) error {
	return func(ctx context.Context) error {
		framework.By("Upgrading Scylla Operator")

		var featureFlags []string
		for feature := range utilfeature.DefaultMutableFeatureGate.GetAll() {
			featureFlags = append(featureFlags, fmt.Sprintf("%s=%v", feature, utilfeature.DefaultMutableFeatureGate.Enabled(feature)))
		}

		cmd := exec.CommandContext(ctx, operatorDeployScriptPath, operatorTargetVersionImage)
		cmd.Dir = operatorRepoPath
		cmd.Env = append(
			os.Environ(),
			"REENTRANT=1",
			fmt.Sprintf("SCYLLA_OPERATOR_FEATURE_GATES=%s", strings.Join(featureFlags, ",")),
		)

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()
		if err != nil {
			return fmt.Errorf("can't run operator deploy script %q with %q image: %w, stdout %q, stderr %q", operatorDeployScriptPath, operatorTargetVersionImage, err, stdout.String(), stderr.String())
		}

		return nil
	}
}
