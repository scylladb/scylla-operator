package scyllaoperatorconfig

import (
	"context"
	"errors"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configassests "github.com/scylladb/scylla-operator/assets/config"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	scyllafixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scylla"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
)

var _ = g.Describe("ScyllaOperatorConfig ", framework.Serial, func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("scyllaoperatorconfig")
	globalSOC := scyllafixture.DefaultScyllaOperatorConfig.ReadOrFail()

	g.BeforeEach(func() {
		ctx, cancel := context.WithTimeoutCause(context.Background(), 1*time.Minute, errors.New("exceeded wait timeout"))
		defer cancel()
		_, err := utils.WaitForScyllaOperatorConfigState(
			ctx,
			f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaOperatorConfigs(),
			globalSOC.Name,
			controllerhelpers.WaitForStateOptions{
				TolerateDelete: false,
			},
			func(freshSOC *scyllav1alpha1.ScyllaOperatorConfig) (bool, error) {
				return freshSOC.DeletionTimestamp == nil, nil
			},
		)
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("should project images into the status", func() {
		ctx, cancel := context.WithTimeoutCause(context.Background(), testTimeout, errors.New("exceeded test timeout"))
		defer cancel()

		var err error
		soc := scyllafixture.DefaultScyllaOperatorConfig.ReadOrFail()

		rc := framework.NewRestoringCleaner(
			ctx,
			f.AdminClientConfig(),
			f.KubeAdminClient(),
			f.DynamicAdminClient(),
			socResourceInfo,
			soc.Namespace,
			soc.Name,
			framework.RestoreStrategyUpdate,
		)
		f.AddCleaners(rc)

		framework.By("Replacing ScyllaOperatorConfig")
		err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
			var freshSOC, updatedSOC *scyllav1alpha1.ScyllaOperatorConfig
			freshSOC, err = f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaOperatorConfigs().Get(ctx, soc.GetName(), metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			obj := soc.DeepCopy()
			obj.ResourceVersion = freshSOC.ResourceVersion
			updatedSOC, err = f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaOperatorConfigs().Update(ctx, obj, metav1.UpdateOptions{})
			if err == nil {
				soc = updatedSOC
			}
			return err
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for ScyllaOperatorConfig status update (RV=%s)", soc.ResourceVersion)
		soc, err = utils.WaitForScyllaOperatorConfigStatus(ctx, f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaOperatorConfigs(), soc)
		o.Expect(err).NotTo(o.HaveOccurred())

		o.Expect(soc.Status.ObservedGeneration).NotTo(o.BeNil())
		o.Expect(*soc.Status.ObservedGeneration).To(o.BeEquivalentTo(soc.Generation))

		o.Expect(soc.Status.ScyllaDBUtilsImage).NotTo(o.BeNil())
		o.Expect(*soc.Status.ScyllaDBUtilsImage).To(o.BeEquivalentTo(configassests.Project.Operator.ScyllaDBUtilsImage))

		o.Expect(soc.Status.BashToolsImage).NotTo(o.BeNil())
		o.Expect(*soc.Status.BashToolsImage).To(o.BeEquivalentTo(configassests.Project.Operator.BashToolsImage))

		o.Expect(soc.Status.GrafanaImage).NotTo(o.BeNil())
		o.Expect(*soc.Status.GrafanaImage).To(o.BeEquivalentTo(configassests.Project.Operator.GrafanaImage))

		o.Expect(soc.Status.PrometheusVersion).NotTo(o.BeNil())
		o.Expect(*soc.Status.PrometheusVersion).To(o.BeEquivalentTo(configassests.Project.Operator.PrometheusVersion))

		framework.By("Changing the defaults and the overrides in ScyllaOperatorConfig")
		oldSOC := soc.DeepCopy()
		soc.Spec = scyllav1alpha1.ScyllaOperatorConfigSpec{
			ScyllaUtilsImage:                     "test.io/updated-scylla-utils-image",
			UnsupportedBashToolsImageOverride:    pointer.Ptr("test.io/overridden-bash-tools-image"),
			UnsupportedGrafanaImageOverride:      pointer.Ptr("test.io/overridden-grafana-image"),
			UnsupportedPrometheusVersionOverride: pointer.Ptr("v0.0.42"),
		}

		patch, err := helpers.CreateTwoWayMergePatch(oldSOC, soc)
		soc, err = f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaOperatorConfigs().Patch(ctx, soc.Name, types.MergePatchType, patch, metav1.PatchOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for ScyllaOperatorConfig status update (RV=%s)", soc.ResourceVersion)
		soc, err = utils.WaitForScyllaOperatorConfigStatus(ctx, f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaOperatorConfigs(), soc)
		o.Expect(err).NotTo(o.HaveOccurred())

		o.Expect(soc.Status.ObservedGeneration).NotTo(o.BeNil())
		o.Expect(*soc.Status.ObservedGeneration).To(o.BeEquivalentTo(soc.Generation))

		o.Expect(soc.Status.ScyllaDBUtilsImage).NotTo(o.BeNil())
		o.Expect(*soc.Status.ScyllaDBUtilsImage).To(o.BeEquivalentTo("test.io/updated-scylla-utils-image"))

		o.Expect(soc.Status.BashToolsImage).NotTo(o.BeNil())
		o.Expect(*soc.Status.BashToolsImage).To(o.BeEquivalentTo("test.io/overridden-bash-tools-image"))

		o.Expect(soc.Status.GrafanaImage).NotTo(o.BeNil())
		o.Expect(*soc.Status.GrafanaImage).To(o.BeEquivalentTo("test.io/overridden-grafana-image"))

		o.Expect(soc.Status.PrometheusVersion).NotTo(o.BeNil())
		o.Expect(*soc.Status.PrometheusVersion).To(o.BeEquivalentTo("v0.0.42"))
	})
})
