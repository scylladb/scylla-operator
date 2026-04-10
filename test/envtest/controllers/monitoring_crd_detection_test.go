//go:build envtest

package controllers

import (
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/test/envtest"
)

var _ = g.Describe("Monitoring CRD detection", func() {
	g.It("reports monitoring.coreos.com/v1 as unavailable when Prometheus Operator CRDs are not installed", func(ctx g.SpecContext) {
		env := envtest.Setup(ctx, envtest.WithoutMonitoringCRDs())

		available, err := helpers.IsAPIGroupVersionAvailable(env.TypedKubeClient().Discovery(), "monitoring.coreos.com/v1")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(available).To(o.BeFalse(), "monitoring.coreos.com/v1 should not be available when Prometheus Operator CRDs are not installed")
	})

	g.It("reports monitoring.coreos.com/v1 as available when Prometheus Operator CRDs are installed", func(ctx g.SpecContext) {
		env := envtest.Setup(ctx)

		available, err := helpers.IsAPIGroupVersionAvailable(env.TypedKubeClient().Discovery(), "monitoring.coreos.com/v1")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(available).To(o.BeTrue(), "monitoring.coreos.com/v1 should be available when Prometheus Operator CRDs are installed")
	})
})
