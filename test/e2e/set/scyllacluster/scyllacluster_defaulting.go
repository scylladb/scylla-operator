// Copyright (c) 2026 ScyllaDB.

package scyllacluster

import (
	"context"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/semver"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = g.Describe("ScyllaCluster's mutating admission webhook", framework.SuiteParallel, framework.SuiteParallelOpenShift, framework.SuiteKindFast, func() {
	var f *framework.Framework

	g.BeforeEach(func(ctx context.Context) {
		f = framework.NewFramework(ctx, "scyllacluster")
	})

	g.It("should default bootstrapPolicy on creation", func(ctx g.SpecContext) {
		sc := f.GetDefaultScyllaCluster()
		// The fixture carries an explicit bootstrapPolicy when the suite is invoked with one.
		sc.Spec.BootstrapPolicy = nil

		// Parallel is only stamped for ScyllaDB versions known to support bootstrapping nodes in parallel, and
		// Sequential is never stamped, so the expected value depends on the version under test. It's deliberately
		// not hardcoded: the version can carry a digest, which makes it unparseable and hence not supporting
		// parallel bootstrap.
		var expectedBootstrapPolicy *scyllav1.BootstrapPolicy
		if semver.SupportsParallelBootstrap(sc.Spec.Version) {
			expectedBootstrapPolicy = new(scyllav1.BootstrapPolicyParallel)
		}

		framework.By("Creating a ScyllaCluster without a bootstrapPolicy")
		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sc.Spec.BootstrapPolicy).To(o.Equal(expectedBootstrapPolicy))
	})

	g.It("should preserve an explicitly set bootstrapPolicy on creation", func(ctx g.SpecContext) {
		sc := f.GetDefaultScyllaCluster()
		// Sequential is valid for every ScyllaDB version, so this holds regardless of the version under test.
		sc.Spec.BootstrapPolicy = new(scyllav1.BootstrapPolicySequential)

		framework.By("Creating a ScyllaCluster with an explicit bootstrapPolicy")
		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sc.Spec.BootstrapPolicy).To(o.Equal(new(scyllav1.BootstrapPolicySequential)))
	})
})
