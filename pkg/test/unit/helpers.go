package unit

import (
	"context"
	"encoding/hex"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/apis/scylla/v1alpha1"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"math/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewSingleRackCluster(members int32) *scyllav1alpha1.Cluster {
	return &scyllav1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "test-ns",
		},
		Spec: scyllav1alpha1.ClusterSpec{
			Version: "2.3.1",
			Datacenter: scyllav1alpha1.DatacenterSpec{
				Name: "test-dc",
				Racks: []scyllav1alpha1.RackSpec{
					{
						Name:    "test-rack",
						Members: members,
					},
				},
			},
		},
		Status: scyllav1alpha1.ClusterStatus{
			Racks: map[string]*scyllav1alpha1.RackStatus{
				"test-rack": {
					Members:      members,
					ReadyMembers: members,
				},
			},
		},
	}
}

func CreateRandomNamespace(client client.Client) string {
	namespaceNameBytes := make([]byte, 10)
	rand.Read(namespaceNameBytes)
	namespaceName := "test-" + hex.EncodeToString(namespaceNameBytes)
	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespaceName}}
	err := client.Create(context.TODO(), namespace)
	if err != nil {
		log.Fatal(err)
	}
	return namespaceName
}
