// Copyright (c) 2022 ScyllaDB

package scyllacluster

import (
	"context"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	scyllafixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scylla"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

var _ = g.Describe("ScyllaCluster Ingress", func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("scyllacluster")

	g.It("should create ingress objects when ingress exposeOptions are provided", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sc := scyllafixture.BasicScyllaCluster.ReadOrFail()
		sc.Spec.Datacenter.Racks[0].Members = 1
		sc.Spec.DNSDomains = []string{"private.nodes.scylladb.com", "public.nodes.scylladb.com"}
		sc.Spec.ExposeOptions = &scyllav1.ExposeOptions{
			CQL: &scyllav1.CQLExposeOptions{
				Ingress: &scyllav1.IngressOptions{
					IngressClassName: "my-cql-ingress-class",
					Annotations: map[string]string{
						"my-cql-annotation-key": "my-cql-annotation-value",
					},
				},
			},
		}

		framework.By("Creating a ScyllaCluster")
		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		// TODO: In theory Ingresses may not be created when ScyllaCluster is rolled out making this test flaky.
		//       Wait for condition when it's available.
		framework.By("Waiting for ScyllaCluster to deploy")
		waitCtx, waitCtxCancel := utils.ContextForRollout(ctx, sc)
		defer waitCtxCancel()

		sc, err = utils.WaitForScyllaClusterState(waitCtx, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), sc, nil)

		framework.By("Verifying AnyNode Ingresses")
		services, err := f.KubeClient().CoreV1().Services(sc.Namespace).List(ctx, metav1.ListOptions{
			LabelSelector: labels.SelectorFromSet(map[string]string{
				naming.ScyllaServiceTypeLabel: string(naming.ScyllaServiceTypeIdentity),
			}).String(),
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(services.Items).To(o.HaveLen(1))

		ingresses, err := f.KubeClient().NetworkingV1().Ingresses(sc.Namespace).List(ctx, metav1.ListOptions{
			LabelSelector: labels.SelectorFromSet(map[string]string{
				naming.ScyllaIngressTypeLabel: string(naming.ScyllaIngressTypeAnyNode),
			}).String(),
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(ingresses.Items).To(o.HaveLen(1))

		verifyIngress(
			&ingresses.Items[0],
			fmt.Sprintf("%s-cql", services.Items[0].Name),
			sc.Spec.ExposeOptions.CQL.Ingress.Annotations,
			sc.Spec.ExposeOptions.CQL.Ingress.IngressClassName,
			&services.Items[0],
			[]string{
				"any.cql.private.nodes.scylladb.com",
				"any.cql.public.nodes.scylladb.com",
			},
			"cql-ssl",
		)

		framework.By("Verifying Node Ingresses")
		services, err = f.KubeClient().CoreV1().Services(sc.Namespace).List(ctx, metav1.ListOptions{
			LabelSelector: labels.SelectorFromSet(map[string]string{
				naming.ScyllaServiceTypeLabel: string(naming.ScyllaServiceTypeMember),
			}).String(),
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(services.Items).To(o.HaveLen(1))

		var nodeHostIDs []string
		for _, svc := range services.Items {
			o.Expect(svc.Annotations).To(o.HaveKey(naming.HostIDAnnotation))
			nodeHostIDs = append(nodeHostIDs, svc.Annotations[naming.HostIDAnnotation])
		}

		ingresses, err = f.KubeClient().NetworkingV1().Ingresses(sc.Namespace).List(ctx, metav1.ListOptions{
			LabelSelector: labels.SelectorFromSet(map[string]string{
				naming.ScyllaIngressTypeLabel: string(naming.ScyllaIngressTypeNode),
			}).String(),
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(ingresses.Items).To(o.HaveLen(1))

		verifyIngress(
			&ingresses.Items[0],
			fmt.Sprintf("%s-cql", services.Items[0].Name),
			sc.Spec.ExposeOptions.CQL.Ingress.Annotations,
			sc.Spec.ExposeOptions.CQL.Ingress.IngressClassName,
			&services.Items[0],
			[]string{
				fmt.Sprintf("%s.cql.private.nodes.scylladb.com", nodeHostIDs[0]),
				fmt.Sprintf("%s.cql.public.nodes.scylladb.com", nodeHostIDs[0]),
			},
			"cql-ssl",
		)

		// TODO: extend the test with a connection to ScyllaCluster via these Ingresses.
		//       Ref: https://github.com/scylladb/scylla-operator/issues/1015
	})
})

func verifyIngress(ingress *networkingv1.Ingress, name string, annotations map[string]string, className string, service *corev1.Service, hosts []string, servicePort string) {
	o.Expect(ingress.Name).To(o.Equal(name))
	for k, v := range annotations {
		o.Expect(ingress.Annotations).To(o.HaveKeyWithValue(k, v))
	}

	o.Expect(ingress.Spec.IngressClassName).ToNot(o.BeNil())
	o.Expect(*ingress.Spec.IngressClassName).To(o.Equal(className))
	o.Expect(ingress.Spec.Rules).To(o.HaveLen(len(hosts)))
	for i := range hosts {
		o.Expect(ingress.Spec.Rules[i].Host).To(o.Equal(hosts[i]))
	}
	for _, rule := range ingress.Spec.Rules {
		o.Expect(rule.HTTP.Paths).To(o.HaveLen(1))
		o.Expect(rule.HTTP.Paths[0].Path).To(o.Equal("/"))
		o.Expect(rule.HTTP.Paths[0].PathType).ToNot(o.BeNil())
		o.Expect(*rule.HTTP.Paths[0].PathType).To(o.Equal(networkingv1.PathTypePrefix))
		o.Expect(rule.HTTP.Paths[0].Backend.Service).ToNot(o.BeNil())
		o.Expect(rule.HTTP.Paths[0].Backend.Service.Name).To(o.Equal(service.Name))
		o.Expect(rule.HTTP.Paths[0].Backend.Service.Port.Name).To(o.Equal(servicePort))
	}
}
