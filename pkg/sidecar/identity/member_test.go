// Copyright (C) 2021 ScyllaDB

package identity

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"sync"
	"testing"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	kubetesting "k8s.io/client-go/testing"
)

func TestMember_GetSeeds(t *testing.T) {
	t.Parallel()

	createPodAndSvc := func(name, ip string, creationTimestamp time.Time) (*corev1.Pod, *corev1.Service) {
		return createPodAndSvcInRack(name, "rack", ip, creationTimestamp)
	}

	// unresolvableSvc makes a Service whose broadcast address of ClusterIP type can't be resolved.
	// A headless Service is what a member looks like before an address is assigned to it.
	unresolvableSvc := func(svc *corev1.Service) *corev1.Service {
		svc = svc.DeepCopy()
		svc.Spec.ClusterIP = corev1.ClusterIPNone
		svc.Spec.ClusterIPs = []string{corev1.ClusterIPNone}
		return svc
	}

	// Truncated to whole seconds because metav1.Time serializes as RFC3339 with second granularity, so a
	// real API server never returns sub-second creation timestamps. The fake client skips serialization
	// and would otherwise let fixtures rank on a precision production doesn't have.
	now := time.Now().Truncate(time.Second)
	firstPod, firstService := createPodAndSvc("pod-0", "1.1.1.1", now)
	secondPod, secondService := createPodAndSvc("pod-1", "2.2.2.2", now.Add(time.Second))
	thirdPod, thirdService := createPodAndSvc("pod-2", "3.3.3.3", now.Add(2*time.Second))

	// Members spread across distinct racks, to exercise the per-rack seed selection. Ranked by the
	// creation timestamp of their Service: rackAPod, then rackBPod, then rackCPod.
	rackAPod, rackAService := createPodAndSvcInRack("rack-a-0", "rack-a", "10.1.0.1", now)
	rackBPod, rackBService := createPodAndSvcInRack("rack-b-0", "rack-b", "10.2.0.1", now.Add(time.Second))
	rackCPod, rackCService := createPodAndSvcInRack("rack-c-0", "rack-c", "10.3.0.1", now.Add(2*time.Second))

	// Two members of the same rack whose Services were created within the same second, so only their
	// names can break the tie.
	sameSecondFirstPod, sameSecondFirstService := createPodAndSvcInRack("rack-d-0", "rack-d", "10.4.0.1", now)
	sameSecondSecondPod, sameSecondSecondService := createPodAndSvcInRack("rack-d-1", "rack-d", "10.4.0.2", now)

	ts := []struct {
		name                       string
		memberService              *corev1.Service
		memberPod                  *corev1.Pod
		memberClientsBroadcastType scyllav1alpha1.BroadcastAddressType
		memberNodesBroadcastType   scyllav1alpha1.BroadcastAddressType
		ipFamily                   corev1.IPFamily
		externalSeeds              []string
		objects                    []runtime.Object
		expectSeeds                []string
		expectErrorString          string
	}{
		{
			name:                       "error when no pods are found",
			memberPod:                  firstPod,
			memberService:              firstService,
			memberClientsBroadcastType: scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			memberNodesBroadcastType:   scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			ipFamily:                   corev1.IPv4Protocol,
			objects:                    []runtime.Object{},
			expectErrorString:          `can't get seed candidates: internal error: can't find any pod for this cluster, including itself`,
		},
		{
			name:                       "bootstrap with external seeds only when cluster is empty and external seeds are provided",
			memberPod:                  firstPod,
			memberService:              firstService,
			memberClientsBroadcastType: scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			memberNodesBroadcastType:   scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			ipFamily:                   corev1.IPv4Protocol,
			objects:                    []runtime.Object{firstPod, firstService},
			externalSeeds:              []string{"10.0.1.1", "10.0.1.2"},
			expectSeeds:                []string{"10.0.1.1", "10.0.1.2"},
		},
		{
			name:                       "bootstrap with external seeds and first created UN node when external seeds are provided",
			memberPod:                  firstPod,
			memberService:              firstService,
			memberClientsBroadcastType: scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			memberNodesBroadcastType:   scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			ipFamily:                   corev1.IPv4Protocol,
			objects:                    []runtime.Object{firstPod, firstService, markPodReady(secondPod), secondService, markPodReady(thirdPod), thirdService},
			externalSeeds:              []string{"10.0.1.1", "10.0.1.2"},
			expectSeeds:                []string{"10.0.1.1", "10.0.1.2", secondService.Spec.ClusterIP},
		},
		{
			name:                       "bootstrap with external seeds only when all Pods from DC are down and the first created Pod is itself",
			memberPod:                  firstPod,
			memberService:              firstService,
			memberClientsBroadcastType: scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			memberNodesBroadcastType:   scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			ipFamily:                   corev1.IPv4Protocol,
			objects:                    []runtime.Object{firstPod, firstService, secondPod, secondService, thirdPod, thirdService},
			externalSeeds:              []string{"10.0.1.1", "10.0.1.2"},
			expectSeeds:                []string{"10.0.1.1", "10.0.1.2"},
		},
		{
			name:                       "bootstrap with external seeds and first created Pod when all Pods from DC are down and external seeds are provided",
			memberPod:                  thirdPod,
			memberService:              thirdService,
			memberClientsBroadcastType: scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			memberNodesBroadcastType:   scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			ipFamily:                   corev1.IPv4Protocol,
			objects:                    []runtime.Object{firstPod, firstService, secondPod, secondService, thirdPod, thirdService},
			externalSeeds:              []string{"10.0.1.1", "10.0.1.2"},
			expectSeeds:                []string{"10.0.1.1", "10.0.1.2", firstService.Spec.ClusterIP},
		},
		{
			name:                       "bootstraps with itself using node-to-node identifier of ClusterIP type when cluster is empty",
			memberPod:                  firstPod,
			memberService:              firstService,
			memberClientsBroadcastType: scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			memberNodesBroadcastType:   scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			ipFamily:                   corev1.IPv4Protocol,
			objects:                    []runtime.Object{firstPod, firstService},
			expectSeeds:                []string{firstService.Spec.ClusterIP},
		},
		{
			name: "bootstraps with itself using node-to-node identifier of PodIP type when cluster is empty",
			memberPod: func() *corev1.Pod {
				pod := firstPod.DeepCopy()
				pod.Status.PodIP = "1.2.3.4"
				return pod
			}(),
			memberService:              firstService,
			memberClientsBroadcastType: scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			memberNodesBroadcastType:   scyllav1alpha1.BroadcastAddressTypePodIP,
			ipFamily:                   corev1.IPv4Protocol,
			objects:                    []runtime.Object{firstPod, firstService},
			expectSeeds:                []string{"1.2.3.4"},
		},
		{
			name:      "bootstraps with itself using node-to-node identifier of LoadBalancer External IP type when cluster is empty",
			memberPod: firstPod,
			memberService: func() *corev1.Service {
				svc := firstService.DeepCopy()
				svc.Spec.Type = corev1.ServiceTypeLoadBalancer
				svc.Status.LoadBalancer = corev1.LoadBalancerStatus{
					Ingress: []corev1.LoadBalancerIngress{
						{
							IP: "4.3.2.1",
						},
					},
				}
				return svc
			}(),
			memberClientsBroadcastType: scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			memberNodesBroadcastType:   scyllav1alpha1.BroadcastAddressTypeServiceLoadBalancerIngress,
			ipFamily:                   corev1.IPv4Protocol,
			objects:                    []runtime.Object{firstPod, firstService},
			expectSeeds:                []string{"4.3.2.1"},
		},
		{
			name:      "bootstraps with itself using node-to-node identifier of LoadBalancer hostname when cluster is empty",
			memberPod: firstPod,
			memberService: func() *corev1.Service {
				svc := firstService.DeepCopy()
				svc.Spec.Type = corev1.ServiceTypeLoadBalancer
				svc.Status.LoadBalancer = corev1.LoadBalancerStatus{
					Ingress: []corev1.LoadBalancerIngress{
						{
							Hostname: "node-1-hostname.scylladb.com",
						},
					},
				}
				return svc
			}(),
			memberClientsBroadcastType: scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			memberNodesBroadcastType:   scyllav1alpha1.BroadcastAddressTypeServiceLoadBalancerIngress,
			ipFamily:                   corev1.IPv4Protocol,
			objects:                    []runtime.Object{firstPod, firstService},
			expectSeeds:                []string{"node-1-hostname.scylladb.com"},
		},
		{
			name:                       "bootstrap with first created UN node",
			memberPod:                  firstPod,
			memberService:              firstService,
			memberClientsBroadcastType: scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			memberNodesBroadcastType:   scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			ipFamily:                   corev1.IPv4Protocol,
			objects:                    []runtime.Object{firstPod, firstService, markPodReady(secondPod), secondService, markPodReady(thirdPod), thirdService},
			expectSeeds:                []string{secondService.Spec.ClusterIP},
		},
		{
			name:                       "bootstrap only with UN node",
			memberPod:                  firstPod,
			memberService:              firstService,
			memberClientsBroadcastType: scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			memberNodesBroadcastType:   scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			ipFamily:                   corev1.IPv4Protocol,
			objects:                    []runtime.Object{firstPod, firstService, secondPod, secondService, markPodReady(thirdPod), thirdService},
			expectSeeds:                []string{thirdService.Spec.ClusterIP},
		},
		{
			name:                       "bootstrap with first created Pod when all are down, which is itself",
			memberPod:                  firstPod,
			memberService:              firstService,
			memberClientsBroadcastType: scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			memberNodesBroadcastType:   scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			ipFamily:                   corev1.IPv4Protocol,
			objects:                    []runtime.Object{firstPod, firstService, secondPod, secondService, thirdPod, thirdService},
			expectSeeds:                []string{firstService.Spec.ClusterIP},
		},
		{
			name:                       "bootstrap with first created Pod when all are down, which is another node",
			memberPod:                  thirdPod,
			memberService:              thirdService,
			memberClientsBroadcastType: scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			memberNodesBroadcastType:   scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			ipFamily:                   corev1.IPv4Protocol,
			objects:                    []runtime.Object{firstPod, firstService, secondPod, secondService, thirdPod, thirdService},
			expectSeeds:                []string{firstService.Spec.ClusterIP},
		},
		{
			name: "use PodIP from status when node broadcast address type is PodIP",
			memberPod: func() *corev1.Pod {
				pod := firstPod.DeepCopy()
				pod.Status.PodIP = "10.0.0.1"
				return pod
			}(),
			memberService:              firstService,
			memberClientsBroadcastType: scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			memberNodesBroadcastType:   scyllav1alpha1.BroadcastAddressTypePodIP,
			ipFamily:                   corev1.IPv4Protocol,
			objects: []runtime.Object{
				func() runtime.Object {
					pod := firstPod.DeepCopy()
					pod.Status.PodIP = "10.0.0.1"
					return pod
				}(),
				firstService,
				func() runtime.Object {
					pod := secondPod.DeepCopy()
					pod.Status.PodIP = "1.2.3.4"
					pod = markPodReady(pod)
					return pod
				}(),
				func() runtime.Object {
					svc := secondService.DeepCopy()
					svc.Spec.ClusterIP = corev1.ClusterIPNone
					svc.Spec.ClusterIPs = []string{corev1.ClusterIPNone}
					return svc
				}(),
				thirdPod,
				thirdService,
			},
			expectSeeds: []string{"1.2.3.4"},
		},
		{
			name:                       "use ClusterIP from Service when node broadcast address type is ClusterIP",
			memberPod:                  thirdPod,
			memberService:              thirdService,
			memberClientsBroadcastType: scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			memberNodesBroadcastType:   scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			ipFamily:                   corev1.IPv4Protocol,
			objects: []runtime.Object{
				func() runtime.Object {
					svc := firstService.DeepCopy()
					svc.Spec.ClusterIP = "1.2.3.4"
					svc.Spec.ClusterIPs = []string{"1.2.3.4"}
					return svc
				}(),
				firstPod,
				secondPod,
				secondService,
				thirdPod,
				thirdService,
			},
			expectSeeds: []string{"1.2.3.4"},
		},
		{
			name:      "use preferred IP address from first Service ingress status when node broadcast address type is LoadBalancer Ingress",
			memberPod: thirdPod,
			memberService: func() *corev1.Service {
				svc := thirdService.DeepCopy()
				svc.Spec.Type = corev1.ServiceTypeLoadBalancer
				svc.Status.LoadBalancer = corev1.LoadBalancerStatus{
					Ingress: []corev1.LoadBalancerIngress{
						{
							Hostname: "third.service.scylladb.com",
						},
					},
				}
				return svc
			}(),
			memberClientsBroadcastType: scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			memberNodesBroadcastType:   scyllav1alpha1.BroadcastAddressTypeServiceLoadBalancerIngress,
			ipFamily:                   corev1.IPv4Protocol,
			objects: []runtime.Object{
				firstPod,
				func() runtime.Object {
					svc := firstService.DeepCopy()
					svc.Spec.Type = corev1.ServiceTypeLoadBalancer
					svc.Status.LoadBalancer = corev1.LoadBalancerStatus{
						Ingress: []corev1.LoadBalancerIngress{
							{
								IP:       "1.2.3.4",
								Hostname: "first.service.scylladb.com",
							},
						},
					}
					return svc
				}(),
				secondPod,
				secondService,
				thirdPod,
				func() runtime.Object {
					svc := thirdService.DeepCopy()
					svc.Spec.Type = corev1.ServiceTypeLoadBalancer
					svc.Status.LoadBalancer = corev1.LoadBalancerStatus{
						Ingress: []corev1.LoadBalancerIngress{
							{
								Hostname: "third.service.scylladb.com",
							},
						},
					}
					return svc
				}(),
			},
			expectSeeds: []string{"1.2.3.4"},
		},
		{
			name:      "use hostname from first Service ingress status when node broadcast address type is LoadBalancer Ingress and IP is not available",
			memberPod: thirdPod,
			memberService: func() *corev1.Service {
				svc := thirdService.DeepCopy()
				svc.Spec.Type = corev1.ServiceTypeLoadBalancer
				svc.Status.LoadBalancer = corev1.LoadBalancerStatus{
					Ingress: []corev1.LoadBalancerIngress{
						{
							Hostname: "third.service.scylladb.com",
						},
					},
				}
				return svc
			}(),
			memberClientsBroadcastType: scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			memberNodesBroadcastType:   scyllav1alpha1.BroadcastAddressTypeServiceLoadBalancerIngress,
			ipFamily:                   corev1.IPv4Protocol,
			objects: []runtime.Object{
				firstPod,
				func() runtime.Object {
					svc := firstService.DeepCopy()
					svc.Spec.Type = corev1.ServiceTypeLoadBalancer
					svc.Status.LoadBalancer = corev1.LoadBalancerStatus{
						Ingress: []corev1.LoadBalancerIngress{
							{
								Hostname: "first.service.scylladb.com",
							},
						},
					}
					return svc
				}(),
				secondPod,
				secondService,
				thirdPod,
				func() runtime.Object {
					svc := thirdService.DeepCopy()
					svc.Spec.Type = corev1.ServiceTypeLoadBalancer
					svc.Status.LoadBalancer = corev1.LoadBalancerStatus{
						Ingress: []corev1.LoadBalancerIngress{
							{
								Hostname: "third.service.scylladb.com",
							},
						},
					}
					return svc
				}(),
			},
			expectSeeds: []string{"first.service.scylladb.com"},
		},
		{
			name:                       "bootstraps with itself when it's the only node, regardless of its ordinal within the rack",
			memberPod:                  secondPod,
			memberService:              secondService,
			memberClientsBroadcastType: scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			memberNodesBroadcastType:   scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			ipFamily:                   corev1.IPv4Protocol,
			objects:                    []runtime.Object{secondPod, secondService},
			expectSeeds:                []string{secondService.Spec.ClusterIP},
		},
		{
			name:                       "bootstrap with the first-ranked pod when it isn't self and there are other pods",
			memberPod:                  secondPod,
			memberService:              secondService,
			memberClientsBroadcastType: scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			memberNodesBroadcastType:   scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			ipFamily:                   corev1.IPv4Protocol,
			objects:                    []runtime.Object{firstPod, firstService, secondPod, secondService},
			expectSeeds:                []string{firstService.Spec.ClusterIP},
		},
		{
			// Cleanup Job pods inherit the cluster's labels, but they have neither a rack nor a member Service.
			name:                       "ignores pods of the cluster which aren't ScyllaDB nodes",
			memberPod:                  firstPod,
			memberService:              firstService,
			memberClientsBroadcastType: scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			memberNodesBroadcastType:   scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			ipFamily:                   corev1.IPv4Protocol,
			objects: []runtime.Object{
				firstPod,
				firstService,
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cleanup-first-pod-abcde",
						Namespace: "namespace",
						Labels: map[string]string{
							"scylla/cluster":                        "my-cluster",
							"app":                                   "scylla",
							"app.kubernetes.io/name":                "scylla",
							"app.kubernetes.io/managed-by":          "scylla-operator",
							"scylla-operator.scylladb.com/pod-type": "cleanup-job",
						},
					},
				},
			},
			expectSeeds: []string{firstService.Spec.ClusterIP},
		},
		{
			name: "IPv6-only service uses IPv6 broadcast addresses",
			memberPod: func() *corev1.Pod {
				pod := firstPod.DeepCopy()
				pod.Status.PodIP = "2001:db8::1"
				pod.Status.PodIPs = []corev1.PodIP{
					{IP: "2001:db8::1"},
				}
				return pod
			}(),
			memberService: func() *corev1.Service {
				svc := firstService.DeepCopy()
				svc.Spec.ClusterIP = "fd00::1"
				svc.Spec.ClusterIPs = []string{"fd00::1"}
				svc.Spec.IPFamilies = []corev1.IPFamily{corev1.IPv6Protocol}
				return svc
			}(),
			memberClientsBroadcastType: scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			memberNodesBroadcastType:   scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			ipFamily:                   corev1.IPv6Protocol,
			objects: []runtime.Object{
				func() *corev1.Pod {
					pod := firstPod.DeepCopy()
					pod.Status.PodIP = "2001:db8::1"
					pod.Status.PodIPs = []corev1.PodIP{
						{IP: "2001:db8::1"},
					}
					return pod
				}(),
				func() *corev1.Service {
					svc := firstService.DeepCopy()
					svc.Spec.ClusterIP = "fd00::1"
					svc.Spec.ClusterIPs = []string{"fd00::1"}
					svc.Spec.IPFamilies = []corev1.IPFamily{corev1.IPv6Protocol}
					return svc
				}(),
			},
			expectSeeds: []string{"fd00::1"},
		},
		{
			name: "dual-stack service with IPv4 first uses IPv4 for ScyllaDB",
			memberPod: func() *corev1.Pod {
				pod := firstPod.DeepCopy()
				pod.Status.PodIP = "192.168.1.1"
				pod.Status.PodIPs = []corev1.PodIP{
					{IP: "192.168.1.1"},
					{IP: "2001:db8::1"},
				}
				return pod
			}(),
			memberService: func() *corev1.Service {
				svc := firstService.DeepCopy()
				svc.Spec.ClusterIP = "10.96.0.1"
				svc.Spec.ClusterIPs = []string{"10.96.0.1", "fd00::1"}
				svc.Spec.IPFamilies = []corev1.IPFamily{corev1.IPv4Protocol, corev1.IPv6Protocol}
				return svc
			}(),
			memberClientsBroadcastType: scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			memberNodesBroadcastType:   scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			ipFamily:                   corev1.IPv4Protocol,
			objects: []runtime.Object{
				func() *corev1.Pod {
					pod := firstPod.DeepCopy()
					pod.Status.PodIP = "192.168.1.1"
					pod.Status.PodIPs = []corev1.PodIP{
						{IP: "192.168.1.1"},
						{IP: "2001:db8::1"},
					}
					return pod
				}(),
				func() *corev1.Service {
					svc := firstService.DeepCopy()
					svc.Spec.ClusterIP = "10.96.0.1"
					svc.Spec.ClusterIPs = []string{"10.96.0.1", "fd00::1"}
					svc.Spec.IPFamilies = []corev1.IPFamily{corev1.IPv4Protocol, corev1.IPv6Protocol}
					return svc
				}(),
			},
			expectSeeds: []string{"10.96.0.1"},
		},
		{
			name: "IPv6 PodIP broadcast with dual-stack pod",
			memberPod: func() *corev1.Pod {
				pod := firstPod.DeepCopy()
				pod.Status.PodIP = "192.168.1.1"
				pod.Status.PodIPs = []corev1.PodIP{
					{IP: "2001:db8::1"}, // IPv6 first in PodIPs
					{IP: "192.168.1.1"},
				}
				return pod
			}(),
			memberService: func() *corev1.Service {
				svc := firstService.DeepCopy()
				svc.Spec.ClusterIP = "fd00::1"
				svc.Spec.ClusterIPs = []string{"fd00::1"}
				svc.Spec.IPFamilies = []corev1.IPFamily{corev1.IPv6Protocol}
				return svc
			}(),
			memberClientsBroadcastType: scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			memberNodesBroadcastType:   scyllav1alpha1.BroadcastAddressTypePodIP,
			ipFamily:                   corev1.IPv6Protocol,
			objects: []runtime.Object{
				func() *corev1.Pod {
					pod := firstPod.DeepCopy()
					pod.Status.PodIP = "192.168.1.1"
					pod.Status.PodIPs = []corev1.PodIP{
						{IP: "2001:db8::1"}, // IPv6 first in PodIPs
						{IP: "192.168.1.1"},
					}
					return pod
				}(),
				func() *corev1.Service {
					svc := firstService.DeepCopy()
					svc.Spec.ClusterIP = "fd00::1"
					svc.Spec.ClusterIPs = []string{"fd00::1"}
					svc.Spec.IPFamilies = []corev1.IPFamily{corev1.IPv6Protocol}
					return svc
				}(),
			},
			expectSeeds: []string{"2001:db8::1"}, // Should use IPv6 from PodIPs[0]
		},
		{
			// Ready path. Self reads ready and ranks first, but is skipped, so rack-a contributes no seed.
			name:                       "selects the first ready member of each rack, excluding itself",
			memberPod:                  rackAPod,
			memberService:              rackAService,
			memberClientsBroadcastType: scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			memberNodesBroadcastType:   scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			ipFamily:                   corev1.IPv4Protocol,
			objects: []runtime.Object{
				markPodReady(rackAPod), rackAService,
				markPodReady(rackBPod), rackBService,
				markPodReady(rackCPod), rackCService,
			},
			expectSeeds: []string{rackBService.Spec.ClusterIP, rackCService.Spec.ClusterIP},
		},
		{
			name:                       "selects a non-self ready member of its own rack",
			memberPod:                  sameSecondFirstPod,
			memberService:              sameSecondFirstService,
			memberClientsBroadcastType: scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			memberNodesBroadcastType:   scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			ipFamily:                   corev1.IPv4Protocol,
			objects: []runtime.Object{
				markPodReady(sameSecondFirstPod), sameSecondFirstService,
				markPodReady(sameSecondSecondPod), sameSecondSecondService,
			},
			expectSeeds: []string{sameSecondSecondService.Spec.ClusterIP},
		},
		{
			// Fallback path. Self reads ready, which can only mean its Pod condition hasn't flipped to
			// False yet after a scylla container restart - seed selection runs before ScyllaDB starts, so
			// self can't truly be ready. Self is skipped regardless, leaving nothing ready, so all members
			// fall back to the candidate they agree on: the first-ranked one, which here is self.
			name:                       "falls back to external seeds when the first-ranked member is itself",
			memberPod:                  rackAPod,
			memberService:              rackAService,
			memberClientsBroadcastType: scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			memberNodesBroadcastType:   scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			ipFamily:                   corev1.IPv4Protocol,
			objects: []runtime.Object{
				markPodReady(rackAPod), rackAService,
				rackBPod, rackBService,
			},
			externalSeeds: []string{"10.0.1.1"},
			expectSeeds:   []string{"10.0.1.1"},
		},
		{
			// Fallback path. The agreed candidate is self, and with no external seeds there are
			// no other addresses to name, so self is the only seed.
			name:                       "falls back to the first-ranked member, which is itself",
			memberPod:                  rackAPod,
			memberService:              rackAService,
			memberClientsBroadcastType: scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			memberNodesBroadcastType:   scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			ipFamily:                   corev1.IPv4Protocol,
			objects: []runtime.Object{
				markPodReady(rackAPod), rackAService,
				rackBPod, rackBService,
			},
			expectSeeds: []string{rackAService.Spec.ClusterIP},
		},
		{
			name:                       "skips racks whose members are all unready",
			memberPod:                  rackAPod,
			memberService:              rackAService,
			memberClientsBroadcastType: scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			memberNodesBroadcastType:   scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			ipFamily:                   corev1.IPv4Protocol,
			objects: []runtime.Object{
				markPodReady(rackAPod), rackAService,
				rackBPod, rackBService,
				markPodReady(rackCPod), rackCService,
			},
			expectSeeds: []string{rackCService.Spec.ClusterIP},
		},
		{
			name:                       "selects only the first-ranked member when none is ready, across racks",
			memberPod:                  rackCPod,
			memberService:              rackCService,
			memberClientsBroadcastType: scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			memberNodesBroadcastType:   scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			ipFamily:                   corev1.IPv4Protocol,
			objects: []runtime.Object{
				rackAPod, rackAService,
				rackBPod, rackBService,
				rackCPod, rackCService,
			},
			expectSeeds: []string{rackAService.Spec.ClusterIP},
		},
		{
			name:                       "seeds with itself when none is ready and it's the first-ranked member, across racks",
			memberPod:                  rackAPod,
			memberService:              rackAService,
			memberClientsBroadcastType: scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			memberNodesBroadcastType:   scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			ipFamily:                   corev1.IPv4Protocol,
			objects: []runtime.Object{
				rackAPod, rackAService,
				rackBPod, rackBService,
				rackCPod, rackCService,
			},
			expectSeeds: []string{rackAService.Spec.ClusterIP},
		},
		{
			name:                       "seeds with external seeds only, and never itself, when none is ready and it's the first-ranked member",
			memberPod:                  rackAPod,
			memberService:              rackAService,
			memberClientsBroadcastType: scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			memberNodesBroadcastType:   scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			ipFamily:                   corev1.IPv4Protocol,
			objects: []runtime.Object{
				rackAPod, rackAService,
				rackBPod, rackBService,
			},
			externalSeeds: []string{"10.0.1.1"},
			expectSeeds:   []string{"10.0.1.1"},
		},
		{
			name:                       "ranks members whose Services were created within the same second by name",
			memberPod:                  sameSecondSecondPod,
			memberService:              sameSecondSecondService,
			memberClientsBroadcastType: scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			memberNodesBroadcastType:   scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			ipFamily:                   corev1.IPv4Protocol,
			objects: []runtime.Object{
				sameSecondFirstPod, sameSecondFirstService,
				sameSecondSecondPod, sameSecondSecondService,
			},
			expectSeeds: []string{sameSecondFirstService.Spec.ClusterIP},
		},
		{
			// This shouldn't happen in practice: a ready member's own sidecar must already have resolved
			// its own address before it could become ready. Kept as defense-in-depth in case that
			// invariant is ever violated.
			name:                       "errors when a selected ready member's broadcast address can't be resolved",
			memberPod:                  rackAPod,
			memberService:              rackAService,
			memberClientsBroadcastType: scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			memberNodesBroadcastType:   scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			ipFamily:                   corev1.IPv4Protocol,
			objects: []runtime.Object{
				markPodReady(rackAPod), rackAService,
				markPodReady(rackBPod), unresolvableSvc(rackBService),
				markPodReady(rackCPod), rackCService,
			},
			expectErrorString: `can't resolve seed broadcast addresses: can't resolve seed broadcast address: can't get broadcast address of "rack-b-0": service "namespace/rack-b-0" does not have a ClusterIP address`,
		},
		{
			name:                       "returns an error when no selected member's broadcast address can be resolved",
			memberPod:                  rackBPod,
			memberService:              rackBService,
			memberClientsBroadcastType: scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			memberNodesBroadcastType:   scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
			ipFamily:                   corev1.IPv4Protocol,
			objects: []runtime.Object{
				markPodReady(rackAPod), unresolvableSvc(rackAService),
			},
			expectErrorString: `can't resolve seed broadcast addresses: can't resolve seed broadcast address: can't get broadcast address of "rack-a-0": service "namespace/rack-a-0" does not have a ClusterIP address`,
		},
	}

	for _, test := range ts {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			member, err := NewMember(test.memberService, test.memberPod, test.memberNodesBroadcastType, test.memberClientsBroadcastType, test.ipFamily, nil)
			if err != nil {
				t.Fatal(err)
			}

			verifyErrorString := func(t *testing.T, err error) {
				var gotErrorString string
				if err != nil {
					gotErrorString = err.Error()
				}
				if gotErrorString != test.expectErrorString {
					t.Errorf("expected error %q, got %q", test.expectErrorString, gotErrorString)
				}
			}

			fakeClient := fake.NewSimpleClientset(test.objects...)
			seeds, err := member.GetSeeds(t.Context(), fakeClient.CoreV1(), test.externalSeeds)
			verifyErrorString(t, err)
			if !reflect.DeepEqual(seeds, test.expectSeeds) {
				t.Errorf("expected seeds %v, got %v", test.expectSeeds, seeds)
			}

			// Every node selects seeds from its own view of the cluster, and the order the API server
			// happens to return the members in is not guaranteed, so the result must not depend on it.
			reversedObjects := slices.Clone(test.objects)
			slices.Reverse(reversedObjects)
			reversedFakeClient := fake.NewSimpleClientset(reversedObjects...)

			reversedSeeds, err := member.GetSeeds(t.Context(), reversedFakeClient.CoreV1(), test.externalSeeds)
			verifyErrorString(t, err)

			if !reflect.DeepEqual(reversedSeeds, test.expectSeeds) {
				t.Errorf("expected seeds %v, got %v", test.expectSeeds, reversedSeeds)
			}
		})
	}
}

// TestMember_GetSeeds_converges verifies that nodes selecting seeds independently agree on one of them.
// Two nodes each picking the other would leave them in separate clusters, and asserting the selection
// from a single node's perspective can't catch that.
// Agreement only matters when the cluster hasn't formed yet, which is the case covered here.
func TestMember_GetSeeds_converges(t *testing.T) {
	t.Parallel()

	now := time.Now().Truncate(time.Second)

	type testMember struct {
		name      string
		rack      string
		ip        string
		createdAt time.Time
	}

	ts := []struct {
		name        string
		members     []testMember
		expectSeeds []string
	}{
		{
			name: "ranked by the creation timestamp of their Service, across racks",
			members: []testMember{
				{name: "rack-c-0", rack: "rack-c", ip: "10.3.0.1", createdAt: now.Add(2 * time.Second)},
				{name: "rack-a-0", rack: "rack-a", ip: "10.1.0.1", createdAt: now},
				{name: "rack-b-0", rack: "rack-b", ip: "10.2.0.1", createdAt: now.Add(time.Second)},
			},
			expectSeeds: []string{"10.1.0.1"},
		},
		{
			// A real API server stores creation timestamps at second granularity, so members created
			// moments apart tie and only the name orders them. That makes the tiebreak, not the timestamp,
			// what every node agrees on here.
			name: "ranked by name when the creation timestamps of their Services tie",
			members: []testMember{
				{name: "rack-a-2", rack: "rack-a", ip: "10.1.0.3", createdAt: now},
				{name: "rack-a-0", rack: "rack-a", ip: "10.1.0.1", createdAt: now},
				{name: "rack-a-1", rack: "rack-a", ip: "10.1.0.2", createdAt: now},
			},
			expectSeeds: []string{"10.1.0.1"},
		},
	}

	for _, test := range ts {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			type member struct {
				pod *corev1.Pod
				svc *corev1.Service
			}
			var members []member
			for _, m := range test.members {
				pod, svc := createPodAndSvcInRack(m.name, m.rack, m.ip, m.createdAt)
				members = append(members, member{pod: pod, svc: svc})
			}

			var objects []runtime.Object
			for _, m := range members {
				objects = append(objects, m.pod, m.svc)
			}
			fakeClient := fake.NewSimpleClientset(objects...)

			seedsBySelf := map[string][]string{}
			for _, self := range members {
				m, err := NewMember(self.svc, self.pod, scyllav1alpha1.BroadcastAddressTypeServiceClusterIP, scyllav1alpha1.BroadcastAddressTypeServiceClusterIP, corev1.IPv4Protocol, nil)
				if err != nil {
					t.Fatal(fmt.Errorf("error creating member: %w", err))
				}

				seeds, err := m.GetSeeds(t.Context(), fakeClient.CoreV1(), nil)
				if err != nil {
					t.Fatal(fmt.Errorf("error getting seeds: %w", err))
				}

				seedsBySelf[self.pod.Name] = seeds
			}

			// Nothing is ready, so every node has to name the same single member: the first-ranked one.
			for name, seeds := range seedsBySelf {
				if !reflect.DeepEqual(seeds, test.expectSeeds) {
					t.Errorf("expected node %q to select %v, got %v", name, test.expectSeeds, seeds)
				}
			}
		})
	}
}

// TestMember_GetSeeds_fallbackWaitsForResolution verifies that when no candidate is ready and the
// resulting fallback candidate isn't self, GetSeeds waits for its broadcast address to become
// resolvable instead of failing immediately.
func TestMember_GetSeeds_fallbackWaitsForResolution(t *testing.T) {
	t.Parallel()

	now := time.Now().Truncate(time.Second)
	rackAPod, rackAService := createPodAndSvcInRack("rack-a-0", "rack-a", "10.1.0.1", now)
	rackBPod, rackBService := createPodAndSvcInRack("rack-b-0", "rack-b", "10.2.0.1", now.Add(time.Second))

	unresolvableSvc := func(svc *corev1.Service) *corev1.Service {
		svc = svc.DeepCopy()
		svc.Spec.ClusterIP = corev1.ClusterIPNone
		svc.Spec.ClusterIPs = []string{corev1.ClusterIPNone}
		return svc
	}

	t.Run("gives up once the context is done", func(t *testing.T) {
		t.Parallel()

		var wg sync.WaitGroup
		defer wg.Wait()

		ctx, ctxCancel := context.WithCancel(t.Context())
		defer ctxCancel()

		member, err := NewMember(rackBService, rackBPod, scyllav1alpha1.BroadcastAddressTypeServiceClusterIP, scyllav1alpha1.BroadcastAddressTypeServiceClusterIP, corev1.IPv4Protocol, nil)
		if err != nil {
			t.Fatal(fmt.Errorf("error creating member: %w", err))
		}

		fakeClient := fake.NewSimpleClientset(rackAPod, unresolvableSvc(rackAService), rackBPod, rackBService)
		polled := observeServiceGet(fakeClient, rackAService.Name)

		wg.Go(func() {
			defer ctxCancel()

			// Cancel only once GetSeeds has observed the unresolvable Service, so the error can only come
			// from the poll giving up rather than from it never having polled at all.
			select {
			case <-polled:
			case <-ctx.Done():
			}
		})

		_, err = member.GetSeeds(ctx, fakeClient.CoreV1(), nil)

		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected error to be %v, got %v", context.Canceled, err)
		}
	})

	t.Run("succeeds once the address becomes resolvable", func(t *testing.T) {
		t.Parallel()

		var wg sync.WaitGroup
		defer wg.Wait()

		ctx, ctxCancel := context.WithCancel(t.Context())
		defer ctxCancel()

		member, err := NewMember(rackBService, rackBPod, scyllav1alpha1.BroadcastAddressTypeServiceClusterIP, scyllav1alpha1.BroadcastAddressTypeServiceClusterIP, corev1.IPv4Protocol, nil)
		if err != nil {
			t.Fatal(fmt.Errorf("error creating member: %w", err))
		}

		fakeClient := fake.NewSimpleClientset(rackAPod, unresolvableSvc(rackAService), rackBPod, rackBService)
		polled := observeServiceGet(fakeClient, rackAService.Name)

		doneCh := make(chan struct{}, 1)
		errCh := make(chan error, 1)
		wg.Go(func() {
			defer close(doneCh)

			// Make the address resolvable only once GetSeeds has observed it unresolvable, so it has to
			// wait for the update rather than resolving on its first attempt.
			select {
			case <-polled:
			case <-ctx.Done():
				return
			}

			// The update itself deliberately doesn't use ctx: it's canceled as this subtest returns, and a
			// cancellation racing the update would surface as a fixture error rather than the real failure.
			svc, err := fakeClient.CoreV1().Services(rackAService.Namespace).Get(context.Background(), rackAService.Name, metav1.GetOptions{})
			if err != nil {
				errCh <- fmt.Errorf("error getting service: %w", err)
				return
			}

			svc.Spec.ClusterIP = rackAService.Spec.ClusterIP
			svc.Spec.ClusterIPs = rackAService.Spec.ClusterIPs
			_, err = fakeClient.CoreV1().Services(rackAService.Namespace).Update(context.Background(), svc, metav1.UpdateOptions{})
			if err != nil {
				errCh <- fmt.Errorf("error updating service: %w", err)
			}
		})

		seeds, err := member.GetSeeds(ctx, fakeClient.CoreV1(), nil)
		if err != nil {
			t.Fatal(fmt.Errorf("error getting seeds: %w", err))
		}

		select {
		case err := <-errCh:
			t.Fatal(err)
		case <-doneCh:
			break
		}

		expected := []string{rackAService.Spec.ClusterIP}
		if !reflect.DeepEqual(seeds, expected) {
			t.Errorf("expected seeds %v, got %v", expected, seeds)
		}
	})
}

// observeServiceGet returns a channel which is closed the first time the named Service is fetched.
func observeServiceGet(fakeClient *fake.Clientset, svcName string) <-chan struct{} {
	observed := make(chan struct{})

	// The poll fetches the Service on every tick, and closing a closed channel panics.
	var once sync.Once

	fakeClient.PrependReactor("get", "services", func(action kubetesting.Action) (bool, runtime.Object, error) {
		getAction, ok := action.(kubetesting.GetAction)
		if !ok || getAction.GetName() != svcName {
			return false, nil, nil
		}

		once.Do(func() {
			close(observed)
		})

		return false, nil, nil
	})

	return observed
}

// createPodAndSvcInRack makes a member of "my-cluster" in the given rack, with its Pod and Service sharing
// a name and a creation timestamp, as the operator creates them.
func createPodAndSvcInRack(name, rack, ip string, creationTimestamp time.Time) (*corev1.Pod, *corev1.Service) {
	// Member Services are selected by the cluster's labels, same as their Pods.
	clusterLabels := map[string]string{
		"scylla/cluster":               "my-cluster",
		"app":                          "scylla",
		"app.kubernetes.io/name":       "scylla",
		"app.kubernetes.io/managed-by": "scylla-operator",
		"scylla/rack":                  rack,
	}

	podLabels := maps.Clone(clusterLabels)
	podLabels["scylla/rack-ordinal"] = "0"
	podLabels["scylla-operator.scylladb.com/pod-type"] = "scylladb-node"

	svcLabels := maps.Clone(clusterLabels)
	svcLabels["scylla-operator.scylladb.com/scylla-service-type"] = "member"

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         "namespace",
			Labels:            podLabels,
			CreationTimestamp: metav1.NewTime(creationTimestamp),
		},
	}
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         "namespace",
			Labels:            svcLabels,
			CreationTimestamp: metav1.NewTime(creationTimestamp),
		},
		Spec: corev1.ServiceSpec{
			ClusterIP:  ip,
			ClusterIPs: []string{ip},
		},
	}

	return pod, svc
}

func markPodReady(pod *corev1.Pod) *corev1.Pod {
	p := pod.DeepCopy()
	cond := controllerhelpers.GetPodCondition(p.Status.Conditions, corev1.PodReady)
	if cond != nil {
		cond.Status = corev1.ConditionTrue
		return p
	}

	p.Status.Conditions = append(p.Status.Conditions, corev1.PodCondition{
		Type:   corev1.PodReady,
		Status: corev1.ConditionTrue,
	})

	return p
}
