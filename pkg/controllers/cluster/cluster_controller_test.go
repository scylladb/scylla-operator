// Copyright (C) 2017 ScyllaDB

package cluster

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/scylladb/scylla-operator/pkg/cmd/options"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

var _ = Describe("Cluster controller", func() {

	DescribeTable("Operator image", func(containerImages []string, expectedImage string) {
		const (
			namespace = "ns"
			name      = "name"
		)
		opts := options.GetOperatorOptions()
		opts.Namespace = namespace
		opts.Name = name

		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      name,
			},
		}
		for _, ci := range containerImages {
			pod.Spec.Containers = append(pod.Spec.Containers, corev1.Container{
				Image: ci,
			})
		}

		kubeClientFake := kubefake.NewSimpleClientset(pod)
		image, err := getOperatorImage(context.Background(), kubeClientFake)
		if expectedImage == "" {
			Expect(err).To(HaveOccurred())
		} else {
			Expect(err).ToNot(HaveOccurred())
			Expect(image).To(Equal(expectedImage))
		}
	},
		Entry("single correct", []string{"scylladb/scylla-operator:0.3.0"}, "scylladb/scylla-operator:0.3.0"),
		Entry("operator with sidecar", []string{"scylladb/scylla-operator:0.3.0", "random/sidecar:latest"}, "scylladb/scylla-operator:0.3.0"),
		Entry("custom operator", []string{"random/my-scylla-operator:0.3.0"}, "random/my-scylla-operator:0.3.0"),
		Entry("incorrect operator", []string{"random/my-operator:0.3.0"}, ""),
	)

})
