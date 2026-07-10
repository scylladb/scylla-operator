// Copyright (C) 2026 ScyllaDB

package scylladbdatacenter

import (
	"context"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	utilsv1alpha1 "github.com/scylladb/scylla-operator/test/e2e/utils/v1alpha1"
	scylladbdatacenterverification "github.com/scylladb/scylla-operator/test/e2e/utils/verification/scylladbdatacenter"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = g.Describe("ScyllaDBDatacenter", framework.SuiteParallel, framework.SuiteParallelOpenShift, framework.SuiteKindFast, func() {
	var f *framework.Framework

	g.BeforeEach(func(ctx context.Context) {
		f = framework.NewFramework(ctx, "scylladbdatacenter")
	})

	waitForRollout := func(ctx context.Context, sdc *scyllav1alpha1.ScyllaDBDatacenter) *scyllav1alpha1.ScyllaDBDatacenter {
		rolloutCtx, rolloutCtxCancel := utilsv1alpha1.ContextForRollout(ctx, sdc)
		defer rolloutCtxCancel()

		rolledOutSDC, err := controllerhelpers.WaitForScyllaDBDatacenterState(
			rolloutCtx,
			f.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(sdc.Namespace),
			sdc.Name,
			controllerhelpers.WaitForStateOptions{},
			utilsv1alpha1.IsScyllaDBDatacenterRolledOut,
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		return rolledOutSDC
	}

	assertReadRequestTimeout := func(ctx context.Context, sdc *scyllav1alpha1.ScyllaDBDatacenter, expected int64) {
		o.Expect(sdc.Spec.Racks).ToNot(o.BeEmpty())

		svc, err := f.KubeClient().CoreV1().Services(sdc.Namespace).Get(ctx, naming.MemberServiceName(sdc.Spec.Racks[0], sdc, 0), metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		pod, err := f.KubeClient().CoreV1().Pods(sdc.Namespace).Get(ctx, naming.PodNameFromService(svc), metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		host, err := controllerhelpers.GetScyllaHost(sdc, svc, pod)
		o.Expect(err).NotTo(o.HaveOccurred())

		configClient, err := utilsv1alpha1.GetScyllaConfigClient(ctx, f.KubeClient().CoreV1(), sdc, host)
		o.Expect(err).NotTo(o.HaveOccurred())

		actual, err := configClient.ReadRequestTimeoutInMs(ctx)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(actual).To(o.Equal(expected))
	}

	g.It("should reconcile custom ScyllaDB config from ConfigMap", func(ctx g.SpecContext) {
		const (
			customConfigMapName      = "custom-scylla-config"
			overriddenYamlFieldName  = "read_request_timeout_in_ms"
			overriddenYamlFieldValue = int64(20000)
		)

		framework.By("Creating a custom ScyllaDB config ConfigMap")
		_, err := f.KubeClient().CoreV1().ConfigMaps(f.Namespace()).Create(ctx, getScyllaConfigMapForValues(customConfigMapName,
			overriddenYamlFieldName, overriddenYamlFieldValue), metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		sdc := f.GetDefaultScyllaDBDatacenter()
		sdc.Spec.RackTemplate.ScyllaDB.CustomConfigMapRef = new(customConfigMapName)

		framework.By("Creating a ScyllaDBDatacenter")
		sdc, err = f.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(f.Namespace()).Create(ctx, sdc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for ScyllaDBDatacenter to roll out (RV=%s)", sdc.ResourceVersion)
		sdc = waitForRollout(ctx, sdc)
		scylladbdatacenterverification.Verify(ctx, f.KubeClient(), f.ScyllaClient(), sdc)

		framework.By("Verifying that ScyllaDB picked up the config YAML override")
		assertReadRequestTimeout(ctx, sdc, overriddenYamlFieldValue)
	})
})

func getScyllaConfigMapForValues(name, fieldName string, value int64) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Data: map[string]string{
			naming.ScyllaConfigName: fmt.Sprintf("%s: %d\n", fieldName, value),
		},
	}
}
