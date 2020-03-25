package actions

import (
	"testing"

	"github.com/scylladb/scylla-operator/pkg/controller/cluster/resource"
	"github.com/scylladb/scylla-operator/pkg/controller/cluster/util"
	"github.com/scylladb/scylla-operator/pkg/test/unit"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestUpgradeStatefulSetScyllaImage(t *testing.T) {

	cluster := unit.NewSingleRackCluster(3)
	rack := cluster.Spec.Datacenter.Racks[0]
	sts := resource.StatefulSetForRack(rack, cluster, "sidecar")

	tests := []struct {
		name          string
		sts           *appsv1.StatefulSet
		expectedImage string
	}{
		{
			name:          "image upgrade",
			sts:           sts,
			expectedImage: "scylla/scylladb:2.3.2",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			kubeClient := fake.NewSimpleClientset(sts)
			err := util.UpgradeStatefulSetScyllaImage(test.sts, test.expectedImage, kubeClient)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			upgradedSts, err := kubeClient.AppsV1().StatefulSets(sts.Namespace).Get(sts.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			actualImage := upgradedSts.Spec.Template.Spec.Containers[0].Image
			if actualImage != test.expectedImage {
				t.Fatalf("Got image %s, expected %s.", actualImage, test.expectedImage)
			}
		})
	}
}
