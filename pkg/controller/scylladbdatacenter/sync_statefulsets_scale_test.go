package scylladbdatacenter

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubefake "k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

func Test_scaleStatefulSets(t *testing.T) {
	t.Parallel()

	const rackName = "rack-a"

	newRackStatefulSet := func(replicas int32) *appsv1.StatefulSet {
		sts := newStatefulSet("sts")
		sts.Labels = map[string]string{naming.RackNameLabel: rackName}
		sts.Spec.Replicas = new(replicas)
		return sts
	}

	newMemberService := func(ordinal int, decommissionedLabel *string) *corev1.Service {
		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testNamespace,
				Name:      "sts-" + string(rune('0'+ordinal)),
				Labels:    map[string]string{naming.RackNameLabel: rackName},
			},
		}
		if decommissionedLabel != nil {
			svc.Labels[naming.DecommissionedLabel] = *decommissionedLabel
		}
		return svc
	}

	toMap := func(services ...*corev1.Service) map[string]*corev1.Service {
		m := map[string]*corev1.Service{}
		for _, svc := range services {
			m[svc.Name] = svc
		}
		return m
	}

	tt := []struct {
		name                  string
		requiredReplicas      int32
		existing              *appsv1.StatefulSet
		services              map[string]*corev1.Service
		expectedScaleReplicas *int32
		expectedDecommission  string
		expectedReasons       []string
	}{
		{
			name:                  "does nothing when the replicas match",
			requiredReplicas:      2,
			existing:              newRackStatefulSet(2),
			services:              toMap(newMemberService(0, nil), newMemberService(1, nil)),
			expectedScaleReplicas: nil,
			expectedDecommission:  "",
			expectedReasons:       nil,
		},
		{
			name:                  "scales up directly",
			requiredReplicas:      3,
			existing:              newRackStatefulSet(2),
			services:              toMap(newMemberService(0, nil), newMemberService(1, nil)),
			expectedScaleReplicas: new(int32(3)),
			expectedDecommission:  "",
			expectedReasons:       []string{"Progressing"},
		},
		{
			name:                  "requests the decommission of the last node before scaling down",
			requiredReplicas:      1,
			existing:              newRackStatefulSet(2),
			services:              toMap(newMemberService(0, nil), newMemberService(1, nil)),
			expectedScaleReplicas: nil,
			expectedDecommission:  "sts-1",
			expectedReasons:       []string{"Progressing"},
		},
		{
			name:                  "waits while the last node is decommissioning",
			requiredReplicas:      1,
			existing:              newRackStatefulSet(2),
			services:              toMap(newMemberService(0, nil), newMemberService(1, new(naming.LabelValueFalse))),
			expectedScaleReplicas: nil,
			expectedDecommission:  "",
			expectedReasons:       []string{reasonWaitingForRackServiceDecommission},
		},
		{
			name:                  "scales down by one once the last node is decommissioned",
			requiredReplicas:      0,
			existing:              newRackStatefulSet(2),
			services:              toMap(newMemberService(0, nil), newMemberService(1, new(naming.LabelValueTrue))),
			expectedScaleReplicas: new(int32(1)),
			expectedDecommission:  "",
			expectedReasons:       []string{"Progressing"},
		},
		{
			name:                  "waits for the member Service of the last node",
			requiredReplicas:      1,
			existing:              newRackStatefulSet(2),
			services:              toMap(newMemberService(0, nil)),
			expectedScaleReplicas: nil,
			expectedDecommission:  "",
			expectedReasons:       []string{reasonWaitingForMissingService},
		},
		{
			name:                  "does not scale while another node of the rack is decommissioning",
			requiredReplicas:      3,
			existing:              newRackStatefulSet(2),
			services:              toMap(newMemberService(0, new(naming.LabelValueFalse)), newMemberService(1, nil)),
			expectedScaleReplicas: nil,
			expectedDecommission:  "",
			expectedReasons:       []string{reasonWaitingForRackServiceDecommission},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			objects := []runtime.Object{tc.existing}
			for _, svc := range tc.services {
				objects = append(objects, svc)
			}
			kubeClient := kubefake.NewSimpleClientset(objects...)

			var scaledTo *int32
			kubeClient.PrependReactor("update", "statefulsets", func(action clienttesting.Action) (bool, runtime.Object, error) {
				updateAction := action.(clienttesting.UpdateAction)
				if updateAction.GetSubresource() != "scale" {
					return false, nil, nil
				}
				scale := updateAction.GetObject().(*autoscalingv1.Scale)
				scaledTo = new(scale.Spec.Replicas)
				return true, scale, nil
			})

			required := newRackStatefulSet(tc.requiredReplicas)
			sdcc := &Controller{kubeClient: kubeClient}
			conditions, err := sdcc.scaleStatefulSets(context.Background(), &statefulSetSyncContext{
				sdc:                  newScyllaDBDatacenter(),
				requiredStatefulSets: []*appsv1.StatefulSet{required},
				existingStatefulSets: map[string]*appsv1.StatefulSet{required.Name: tc.existing},
				services:             tc.services,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			var reasons []string
			for _, cond := range conditions {
				reasons = append(reasons, cond.Reason)
			}
			if diff := cmp.Diff(tc.expectedReasons, reasons); diff != "" {
				t.Errorf("condition reasons differ (-want +got):\n%s", diff)
			}

			if diff := cmp.Diff(tc.expectedScaleReplicas, scaledTo); diff != "" {
				t.Errorf("scaled replicas differ (-want +got):\n%s", diff)
			}

			decommissionRequested := ""
			for _, action := range kubeClient.Actions() {
				updateAction, ok := action.(clienttesting.UpdateAction)
				if !ok || updateAction.GetResource().Resource != "services" {
					continue
				}
				svc := updateAction.GetObject().(*corev1.Service)
				if svc.Labels[naming.DecommissionedLabel] == naming.LabelValueFalse {
					decommissionRequested = svc.Name
				}
			}
			if decommissionRequested != tc.expectedDecommission {
				t.Errorf("expected decommission to be requested for %q, got %q", tc.expectedDecommission, decommissionRequested)
			}
		})
	}
}

func Test_servicesForRack(t *testing.T) {
	t.Parallel()

	newService := func(name string, labels map[string]string) *corev1.Service {
		return &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels}}
	}

	services := map[string]*corev1.Service{
		"a-0":       newService("a-0", map[string]string{naming.RackNameLabel: "a"}),
		"a-1":       newService("a-1", map[string]string{naming.RackNameLabel: "a"}),
		"b-0":       newService("b-0", map[string]string{naming.RackNameLabel: "b"}),
		"unlabeled": newService("unlabeled", nil),
	}

	got := servicesForRack(services, "a")
	expected := map[string]*corev1.Service{"a-0": services["a-0"], "a-1": services["a-1"]}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("rack services differ (-want +got):\n%s", diff)
	}

	if got := servicesForRack(services, ""); len(got) != 0 {
		t.Errorf("expected no services for an empty rack name, got %v", got)
	}
}
