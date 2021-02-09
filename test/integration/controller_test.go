// Copyright (C) 2017 ScyllaDB

package integration

import (
	"context"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/v1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/test/integration"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Cluster controller", func() {
	var (
		ns *corev1.Namespace
	)

	BeforeEach(func() {
		var err error
		ns, err = testEnv.CreateNamespace(ctx, "ns")
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		Expect(testEnv.Delete(ctx, ns)).To(Succeed())
	})

	Context("Cluster is scaled sequentially", func() {
		It("single rack", func() {
			scylla := testEnv.SingleRackCluster(ns)

			Expect(testEnv.Create(ctx, scylla)).To(Succeed())
			defer testEnv.Delete(ctx, scylla)

			Expect(testEnv.WaitForCluster(ctx, scylla)).To(Succeed())

			Expect(testEnv.Refresh(ctx, scylla)).To(Succeed())
			sst := integration.NewStatefulSetOperatorStub(testEnv)

			// Cluster should be scaled sequentially up to 3
			for _, rack := range scylla.Spec.Datacenter.Racks {
				for _, replicas := range testEnv.ClusterScaleSteps(rack.Members) {
					Expect(testEnv.AssertRackScaled(ctx, rack, scylla, replicas)).To(Succeed())
					Expect(sst.CreatePods(ctx, scylla)).To(Succeed())
				}
			}

			Expect(assertClusterStatusReflectsSpec(ctx, scylla)).To(Succeed())
		})
	})

	It("When PVC affinity is bound to lost node, node is replaced", func() {
		scylla := singleNodeCluster(ns)

		Expect(testEnv.Create(ctx, scylla)).To(Succeed())
		defer func() {
			Expect(testEnv.Delete(ctx, scylla)).To(Succeed())
		}()

		Expect(testEnv.WaitForCluster(ctx, scylla)).To(Succeed())
		Expect(testEnv.Refresh(ctx, scylla)).To(Succeed())

		sstStub := integration.NewStatefulSetOperatorStub(testEnv)
		rack := scylla.Spec.Datacenter.Racks[0]

		pvOption := integration.WithPVNodeAffinity([]corev1.NodeSelectorRequirement{
			{
				Key:      "some-label",
				Operator: corev1.NodeSelectorOpIn,
				Values:   []string{"some-value"},
			},
		})

		// Cluster should be scaled sequentially up to member count
		for _, replicas := range testEnv.ClusterScaleSteps(rack.Members) {
			Expect(testEnv.AssertRackScaled(ctx, rack, scylla, replicas)).To(Succeed())
			Expect(sstStub.CreatePods(ctx, scylla)).To(Succeed())
			Expect(sstStub.CreatePVCs(ctx, scylla, pvOption)).To(Succeed())
		}

		Eventually(func() error {
			Expect(testEnv.Refresh(ctx, scylla)).To(Succeed())
			scylla.Spec.AutomaticOrphanedNodeCleanup = true
			return testEnv.Update(ctx, scylla)
		}, shortWait).Should(Succeed())

		services, err := rackMemberService(ns.Namespace, rack, scylla)
		Expect(err).To(BeNil())
		Expect(services).To(Not(BeEmpty()))

		service := services[0]

		Eventually(func() map[string]string {
			Expect(testEnv.Refresh(ctx, &service)).To(Succeed())

			return service.Labels
		}).Should(HaveKeyWithValue(naming.ReplaceLabel, ""))
	})

	Context("Node replace", func() {
		var (
			scylla  *scyllav1.ScyllaCluster
			sstStub *integration.StatefulSetOperatorStub
		)

		BeforeEach(func() {
			scylla = testEnv.SingleRackCluster(ns)

			Expect(testEnv.Create(ctx, scylla)).To(Succeed())
			Expect(testEnv.WaitForCluster(ctx, scylla)).To(Succeed())
			Expect(testEnv.Refresh(ctx, scylla)).To(Succeed())

			sstStub = integration.NewStatefulSetOperatorStub(testEnv)

			// Cluster should be scaled sequentially up to member count
			rack := scylla.Spec.Datacenter.Racks[0]
			for _, replicas := range testEnv.ClusterScaleSteps(rack.Members) {
				Expect(testEnv.AssertRackScaled(ctx, rack, scylla, replicas)).To(Succeed())
				Expect(sstStub.CreatePods(ctx, scylla)).To(Succeed())
				Expect(sstStub.CreatePVCs(ctx, scylla)).To(Succeed())
			}
		})

		AfterEach(func() {
			Expect(testEnv.Delete(ctx, scylla)).To(Succeed())
		})

		It("replace non seed node", func() {
			rack := scylla.Spec.Datacenter.Racks[0]

			services, err := nonSeedServices(ns.Namespace, rack, scylla)
			Expect(err).To(BeNil())
			Expect(services).To(Not(BeEmpty()))

			serviceToReplace := services[0]
			replacedServiceIP := serviceToReplace.Spec.ClusterIP

			serviceToReplace.Labels[naming.ReplaceLabel] = ""
			Expect(testEnv.Update(ctx, &serviceToReplace)).To(Succeed())

			By("Service IP should appear in ReplaceAddressFirstBoot rack status")
			Eventually(func() (map[string]string, error) {
				if err := testEnv.Refresh(ctx, scylla); err != nil {
					return nil, err
				}

				return scylla.Status.Racks[rack.Name].ReplaceAddressFirstBoot, nil
			}).Should(HaveKeyWithValue(serviceToReplace.Name, replacedServiceIP))

			By("Old PVC should be removed")
			Eventually(func() []corev1.PersistentVolumeClaim {
				pvcs := &corev1.PersistentVolumeClaimList{}

				Expect(testEnv.List(ctx, pvcs, &client.ListOptions{
					LabelSelector: naming.RackSelector(rack, scylla),
				})).To(Succeed())

				return pvcs.Items
			}, shortWait).Should(HaveLen(int(rack.Members - 1)))

			By("Old Pod should be removed")
			Eventually(func() []corev1.Pod {
				pods := &corev1.PodList{}

				Expect(testEnv.List(ctx, pods, &client.ListOptions{
					LabelSelector: naming.RackSelector(rack, scylla),
				})).To(Succeed())

				return pods.Items
			}, shortWait).Should(HaveLen(int(rack.Members - 1)))

			By("When new pod is scheduled")
			Expect(sstStub.CreatePods(ctx, scylla)).To(Succeed())

			By("New service should be created with replace label pointing to old one")
			Eventually(func() (map[string]string, error) {
				if err := testEnv.Refresh(ctx, scylla); err != nil {
					return nil, err
				}

				service := &corev1.Service{}
				key := client.ObjectKey{
					Namespace: scylla.Namespace,
					Name:      serviceToReplace.Name,
				}
				if err := testEnv.Get(ctx, key, service); err != nil {
					if apierrors.IsNotFound(err) {
						return nil, nil
					}
					return nil, err
				}

				return service.Labels, nil
			}, shortWait).Should(HaveKeyWithValue(naming.ReplaceLabel, replacedServiceIP))
		})

		It("replace seed node", func() {
			rack := scylla.Spec.Datacenter.Racks[0]

			services, err := seedServices(ns.Namespace, rack, scylla)
			Expect(err).To(BeNil())
			Expect(services).To(Not(BeEmpty()))

			By("When replace label is added to seed member service")
			service := services[0]
			service.Labels[naming.ReplaceLabel] = ""
			Expect(testEnv.Update(ctx, &service)).To(Succeed())

			By("There should be an error event generated about replace failure")
			Eventually(func() (done bool, err error) {
				events := &corev1.EventList{}
				err = testEnv.List(ctx, events, &client.ListOptions{
					Namespace: ns.Name,
				})
				Expect(err).To(BeNil())

				found := false
				for _, e := range events.Items {
					if e.Reason == naming.ErrSyncFailed && strings.Contains(e.Message, "replace") && strings.Contains(e.Message, "seed node") {
						found = true
						break
					}
				}

				return found, nil
			}, shortWait).Should(BeTrue())
		})
	})
})

func rackMemberService(namespace string, rack scyllav1.RackSpec, cluster *scyllav1.ScyllaCluster) ([]corev1.Service, error) {
	services := &corev1.ServiceList{}
	Expect(wait.PollImmediate(retryInterval, timeout, func() (bool, error) {
		err := testEnv.List(ctx, services, &client.ListOptions{
			Namespace:     namespace,
			LabelSelector: naming.RackSelector(rack, cluster),
		})
		if err != nil {
			return false, err
		}
		return len(services.Items) == int(rack.Members), nil
	})).To(Succeed())

	return services.Items, nil
}

func nonSeedServices(namespace string, rack scyllav1.RackSpec, cluster *scyllav1.ScyllaCluster) ([]corev1.Service, error) {
	services, err := rackMemberService(namespace, rack, cluster)
	if err != nil {
		return nil, err
	}

	var nonSeedServices []corev1.Service
	for _, s := range services {
		if _, ok := s.Labels[naming.SeedLabel]; !ok {
			nonSeedServices = append(nonSeedServices, s)
		}
	}

	return nonSeedServices, nil
}

func seedServices(namespace string, rack scyllav1.RackSpec, cluster *scyllav1.ScyllaCluster) ([]corev1.Service, error) {
	services, err := rackMemberService(namespace, rack, cluster)
	if err != nil {
		return nil, err
	}

	var seedServices []corev1.Service
	for _, s := range services {
		if _, ok := s.Labels[naming.SeedLabel]; ok {
			seedServices = append(seedServices, s)
		}
	}

	return seedServices, nil
}

func singleNodeCluster(ns *corev1.Namespace) *scyllav1.ScyllaCluster {
	return &scyllav1.ScyllaCluster{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-",
			Namespace:    ns.Name,
		},
		Spec: scyllav1.ClusterSpec{
			Version:       "4.2.0",
			AgentVersion:  pointer.StringPtr("2.2.0"),
			DeveloperMode: true,
			Datacenter: scyllav1.DatacenterSpec{
				Name: "dc1",
				Racks: []scyllav1.RackSpec{
					{
						Name:    "rack1",
						Members: 1,
						Storage: scyllav1.StorageSpec{
							Capacity: "10M",
						},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("1"),
								corev1.ResourceMemory: resource.MustParse("200M"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("1"),
								corev1.ResourceMemory: resource.MustParse("200M"),
							},
						},
					},
				},
			},
		},
	}
}

func assertClusterStatusReflectsSpec(ctx context.Context, spec *scyllav1.ScyllaCluster) error {
	return wait.Poll(retryInterval, timeout, func() (bool, error) {
		cluster := &scyllav1.ScyllaCluster{}
		if err := testEnv.Get(ctx, naming.NamespacedName(spec.Name, spec.Namespace), cluster); err != nil {
			return false, err
		}

		for _, r := range spec.Spec.Datacenter.Racks {
			status := cluster.Status.Racks[r.Name]
			if status.ReadyMembers != r.Members {
				return false, nil
			}
		}
		return true, nil
	})
}
