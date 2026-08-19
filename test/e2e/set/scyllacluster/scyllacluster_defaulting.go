// Copyright (c) 2026 ScyllaDB.

package scyllacluster

import (
	"context"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/scylladb/scylla-operator/pkg/semver"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = g.Describe("ScyllaCluster's mutating admission webhook", framework.SuiteParallel, framework.SuiteParallelOpenShift, framework.SuiteKindFast, func() {
	var f *framework.Framework

	g.BeforeEach(func(ctx context.Context) {
		f = framework.NewFramework(ctx, "scyllacluster")
	})

	g.It("should default enableParallelNodeOperations on creation", func(ctx g.SpecContext) {
		sc := f.GetDefaultScyllaCluster()
		// The fixture carries an explicit enableParallelNodeOperations when the suite is invoked with one.
		sc.Spec.EnableParallelNodeOperations = nil

		// true is only stamped for ScyllaDB versions known to support bootstrapping nodes in parallel, and false is
		// never stamped, so the expected value depends on the version under test. It's deliberately not hardcoded:
		// the version can carry a digest, which makes it unparseable and hence not supporting parallel bootstrap.
		var expectedEnableParallelNodeOperations *bool
		if semver.SupportsParallelBootstrap(sc.Spec.Version) {
			expectedEnableParallelNodeOperations = new(true)
		}

		framework.By("Creating a ScyllaCluster without enableParallelNodeOperations")
		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sc.Spec.EnableParallelNodeOperations).To(o.Equal(expectedEnableParallelNodeOperations))
	})

	g.It("should preserve an explicitly set enableParallelNodeOperations on creation", func(ctx g.SpecContext) {
		sc := f.GetDefaultScyllaCluster()
		// false is valid for every ScyllaDB version, so this holds regardless of the version under test.
		sc.Spec.EnableParallelNodeOperations = new(false)

		framework.By("Creating a ScyllaCluster with an explicit enableParallelNodeOperations")
		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sc.Spec.EnableParallelNodeOperations).To(o.Equal(new(false)))
	})
})
