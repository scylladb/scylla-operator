// Copyright (c) 2026 ScyllaDB.

package scylladbdatacenter

import (
	"context"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/semver"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = g.Describe("ScyllaDBDatacenter's mutating admission webhook", framework.SuiteParallel, framework.SuiteParallelOpenShift, framework.SuiteKindFast, func() {
	var f *framework.Framework

	g.BeforeEach(func(ctx context.Context) {
		f = framework.NewFramework(ctx, "scylladbdatacenter")
	})

	g.It("should default bootstrapPolicy on creation", func(ctx g.SpecContext) {
		sdc := f.GetDefaultScyllaDBDatacenter()
		// The fixture carries an explicit bootstrapPolicy when the suite is invoked with one.
		sdc.Spec.BootstrapPolicy = nil

		// Parallel is only stamped for ScyllaDB versions known to support bootstrapping nodes in parallel, and
		// Sequential is never stamped, so the expected value depends on the image under test. It's deliberately not
		// hardcoded: the tag the tests run against doesn't have to be a semver-parseable version.
		scyllaDBVersion, err := naming.ImageToVersion(sdc.Spec.ScyllaDB.Image)
		o.Expect(err).NotTo(o.HaveOccurred())

		var expectedBootstrapPolicy *scyllav1alpha1.BootstrapPolicy
		if semver.SupportsParallelBootstrap(scyllaDBVersion) {
			expectedBootstrapPolicy = new(scyllav1alpha1.BootstrapPolicyParallel)
		}

		framework.By("Creating a ScyllaDBDatacenter without a bootstrapPolicy")
		sdc, err = f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBDatacenters(f.Namespace()).Create(ctx, sdc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sdc.Spec.BootstrapPolicy).To(o.Equal(expectedBootstrapPolicy))
	})

	g.It("should preserve an explicitly set bootstrapPolicy on creation", func(ctx g.SpecContext) {
		sdc := f.GetDefaultScyllaDBDatacenter()
		// Sequential is valid for every ScyllaDB version, so this holds regardless of the image under test.
		sdc.Spec.BootstrapPolicy = new(scyllav1alpha1.BootstrapPolicySequential)

		framework.By("Creating a ScyllaDBDatacenter with an explicit bootstrapPolicy")
		sdc, err := f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBDatacenters(f.Namespace()).Create(ctx, sdc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sdc.Spec.BootstrapPolicy).To(o.Equal(new(scyllav1alpha1.BootstrapPolicySequential)))
	})
})
