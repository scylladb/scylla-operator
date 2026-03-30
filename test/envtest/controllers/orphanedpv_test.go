//go:build envtest

package controllers

import (
	"context"
	"fmt"
	"sync"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllainformers "github.com/scylladb/scylla-operator/pkg/client/scylla/informers/externalversions"
	"github.com/scylladb/scylla-operator/pkg/controller/orphanedpv"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/pkg/test/unit"
	"github.com/scylladb/scylla-operator/test/envtest"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = g.Describe("OrphanedPVController", func() {
	const (
		sdcName    = "test-sdc"
		dcName     = "dc1"
		rackName   = "rack1"
		pvBaseName = "pv-test"
	)

	var (
		env     *envtest.Environment
		stsName string
		svcName string
		pvcName string
	)
	g.BeforeEach(func(ctx g.SpecContext) {
		env = envtest.Setup(ctx)
		stsName = fmt.Sprintf("%s-%s-%s", sdcName, dcName, rackName)
		svcName = fmt.Sprintf("%s-0", stsName)
		pvcName = fmt.Sprintf("%s-%s", naming.PVCTemplateName, svcName)
	})

	runOrphanedPVController := func(ctx context.Context, env *envtest.Environment) {
		kubeInformers := kubeinformers.NewSharedInformerFactory(env.TypedKubeClient(), 0)
		scyllaInformers := scyllainformers.NewSharedInformerFactory(env.ScyllaClient(), 0)

		controller, err := orphanedpv.NewController(
			env.TypedKubeClient(),
			kubeInformers.Core().V1().PersistentVolumes(),
			kubeInformers.Core().V1().PersistentVolumeClaims(),
			kubeInformers.Core().V1().Nodes(),
			scyllaInformers.Scylla().V1alpha1().ScyllaDBDatacenters(),
		)
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to create orphaned PV controller")

		kubeInformers.Start(ctx.Done())
		scyllaInformers.Start(ctx.Done())

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			controller.Run(ctx, 1)
		}()

		g.DeferCleanup(func() {
			kubeInformers.Shutdown()
			scyllaInformers.Shutdown()
			wg.Wait()
		})
	}

	makeScyllaDBDatacenter := func(namespace string, disableOrphanedNodeReplacement *bool) *scyllav1alpha1.ScyllaDBDatacenter {
		return &scyllav1alpha1.ScyllaDBDatacenter{
			ObjectMeta: metav1.ObjectMeta{
				Name:      sdcName,
				Namespace: namespace,
			},
			Spec: scyllav1alpha1.ScyllaDBDatacenterSpec{
				ClusterName:    "test-cluster",
				DatacenterName: pointer.Ptr(dcName),
				ScyllaDB: scyllav1alpha1.ScyllaDB{
					Image: unit.ScyllaDBImage,
				},
				Racks: []scyllav1alpha1.RackSpec{
					{
						RackTemplate: scyllav1alpha1.RackTemplate{
							Nodes: pointer.Ptr[int32](1),
						},
						Name: rackName,
					},
				},
				DisableAutomaticOrphanedNodeReplacement: disableOrphanedNodeReplacement,
			},
		}
	}

	makePVC := func(namespace, pvName string) *corev1.PersistentVolumeClaim {
		return &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pvcName,
				Namespace: namespace,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("1Gi"),
					},
				},
				VolumeName: pvName,
			},
		}
	}

	makePV := func(pvName string, nodeAffinity *corev1.VolumeNodeAffinity) *corev1.PersistentVolume {
		pv := &corev1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name: pvName,
			},
			Spec: corev1.PersistentVolumeSpec{
				Capacity: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				PersistentVolumeSource: corev1.PersistentVolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/tmp/test",
					},
				},
				NodeAffinity: nodeAffinity,
			},
		}

		return pv
	}

	makeVolumeNodeAffinity := func(hostname string) *corev1.VolumeNodeAffinity {
		return &corev1.VolumeNodeAffinity{
			Required: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      corev1.LabelHostname,
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{hostname},
							},
						},
					},
				},
			},
		}
	}

	makeService := func(namespace string) *corev1.Service {
		return &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      svcName,
				Namespace: namespace,
				Labels: map[string]string{
					naming.DatacenterNameLabel: dcName,
					naming.RackNameLabel:       rackName,
				},
			},
			Spec: corev1.ServiceSpec{
				ClusterIP: corev1.ClusterIPNone,
			},
		}
	}

	type testCase struct {
		pvNameSuffix                   string
		disableOrphanedNodeReplacement bool
		nodeHostname                   string
		createNode                     bool
		pvNodeAffinity                 *corev1.VolumeNodeAffinity
		expectReplaceLabel             bool
	}
	g.DescribeTable("should handle Service replace labeling", func(ctx g.SpecContext, tc testCase) {
		pvName := pvBaseName + tc.pvNameSuffix

		g.By("Creating test objects")
		if tc.createNode {
			node := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: tc.nodeHostname,
					Labels: map[string]string{
						corev1.LabelHostname: tc.nodeHostname,
					},
				},
			}
			err := env.KubeClient().Create(ctx, node)
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		sdc := makeScyllaDBDatacenter(env.Namespace(), &tc.disableOrphanedNodeReplacement)
		err := env.KubeClient().Create(ctx, sdc)
		o.Expect(err).NotTo(o.HaveOccurred())

		pv := makePV(pvName, tc.pvNodeAffinity)
		err = env.KubeClient().Create(ctx, pv)
		o.Expect(err).NotTo(o.HaveOccurred())

		pvc := makePVC(env.Namespace(), pvName)
		err = env.KubeClient().Create(ctx, pvc)
		o.Expect(err).NotTo(o.HaveOccurred())

		svc := makeService(env.Namespace())
		err = env.KubeClient().Create(ctx, svc)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Running the OrphanedPVController")
		controllerCtx, controllerCancel := context.WithCancel(ctx)
		defer controllerCancel()
		runOrphanedPVController(controllerCtx, env)

		if tc.expectReplaceLabel {
			g.By("Waiting for the Service to get the replace label")
			o.Eventually(func(g o.Gomega) {
				freshSvc := &corev1.Service{}
				g.Expect(env.KubeClient().Get(ctx, client.ObjectKeyFromObject(svc), freshSvc)).To(o.Succeed())
				g.Expect(freshSvc.Labels).To(o.HaveKey(naming.ReplaceLabel))
			}).WithTimeout(30 * time.Second).WithPolling(250 * time.Millisecond).WithContext(ctx).Should(o.Succeed())
		} else {
			g.By("Ensuring the Service does not get the replace label")
			o.Consistently(func(g o.Gomega) {
				freshSvc := &corev1.Service{}
				g.Expect(env.KubeClient().Get(ctx, client.ObjectKeyFromObject(svc), freshSvc)).To(o.Succeed())
				g.Expect(freshSvc.Labels).NotTo(o.HaveKey(naming.ReplaceLabel))
			}).WithTimeout(10 * time.Second).WithPolling(250 * time.Millisecond).WithContext(ctx).Should(o.Succeed())
		}
	},
		g.Entry("when PV is bound to a non-existent node", testCase{
			pvNameSuffix:                   "-orphaned",
			disableOrphanedNodeReplacement: false,
			nodeHostname:                   "this-node-does-not-exist-42",
			createNode:                     false,
			pvNodeAffinity:                 makeVolumeNodeAffinity("this-node-does-not-exist-42"),
			expectReplaceLabel:             true,
		}),
		g.Entry("when automatic orphan replacement is disabled", testCase{
			pvNameSuffix:                   "-disabled",
			disableOrphanedNodeReplacement: true,
			nodeHostname:                   "this-node-does-not-exist-42",
			createNode:                     false,
			pvNodeAffinity:                 makeVolumeNodeAffinity("this-node-does-not-exist-42"),
			expectReplaceLabel:             false,
		}),
		g.Entry("when PV is bound to an existing node", testCase{
			pvNameSuffix:                   "-existing",
			disableOrphanedNodeReplacement: false,
			nodeHostname:                   "existing-node",
			createNode:                     true,
			pvNodeAffinity:                 makeVolumeNodeAffinity("existing-node"),
			expectReplaceLabel:             false,
		}),
		g.Entry("when PV has no NodeAffinity", testCase{
			pvNameSuffix:                   "-no-affinity",
			disableOrphanedNodeReplacement: false,
			nodeHostname:                   "",
			createNode:                     false,
			pvNodeAffinity:                 nil,
			expectReplaceLabel:             false,
		}),
	)
})
