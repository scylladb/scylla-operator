// Copyright (c) 2024 ScyllaDB.

package scylladbcluster

import (
	"context"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/storage/names"
)

var _ = g.Describe("ScyllaDBCluster webhook", func() {
	f := framework.NewFramework("scylladbcluster")

	g.It("should forbid invalid requests", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		rkcs, _, err := utils.SetUpRemoteKubernetesClustersFromRestConfigs(ctx, framework.TestContext.RestConfigs, f)
		o.Expect(err).NotTo(o.HaveOccurred())

		validSC := f.GetDefaultScyllaDBCluster(rkcs)
		validSC.Name = names.SimpleNameGenerator.GenerateName(validSC.GenerateName)

		framework.By("Rejecting a creation of ScyllaDBCluster with duplicated datacenters")
		duplicatedDatacentersSC := validSC.DeepCopy()
		duplicatedDatacentersSC.Spec.Datacenters = append(duplicatedDatacentersSC.Spec.Datacenters, *duplicatedDatacentersSC.Spec.Datacenters[0].DeepCopy())
		_, err = f.ScyllaClient().ScyllaV1alpha1().ScyllaDBClusters(f.Namespace()).Create(ctx, duplicatedDatacentersSC, metav1.CreateOptions{})
		o.Expect(err).To(o.Equal(&errors.StatusError{ErrStatus: metav1.Status{
			Status:  "Failure",
			Message: fmt.Sprintf(`admission webhook "webhook.scylla.scylladb.com" denied the request: ScyllaDBCluster.scylla.scylladb.com %q is invalid: spec.datacenters[1].name: Duplicate value: %q`, duplicatedDatacentersSC.Name, duplicatedDatacentersSC.Spec.Datacenters[1].Name),
			Reason:  "Invalid",
			Details: &metav1.StatusDetails{
				Name:  duplicatedDatacentersSC.Name,
				Group: "scylla.scylladb.com",
				Kind:  "ScyllaDBCluster",
				UID:   "",
				Causes: []metav1.StatusCause{
					{
						Type:    "FieldValueDuplicate",
						Message: fmt.Sprintf(`Duplicate value: %q`, duplicatedDatacentersSC.Spec.Datacenters[1].Name),
						Field:   "spec.datacenters[1].name",
					},
				},
			},
			Code: 422,
		}}))

		framework.By("Accepting a creation of valid ScyllaCluster")
		validSC, err = f.ScyllaClient().ScyllaV1alpha1().ScyllaDBClusters(f.Namespace()).Create(ctx, validSC, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
	})
})
