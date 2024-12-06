// Copyright (c) 2024 ScyllaDB.

package client

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/fake"
)

func TestClusterClient_RefreshOnClusterConfigChange(t *testing.T) {
	tss := []struct {
		clusterName            string
		clusterConfig          string
		expectedConfigCreation bool
	}{
		{
			clusterName:            "cluster-1",
			clusterConfig:          "cluster-1-config-1",
			expectedConfigCreation: true,
		},
		{
			clusterName:            "cluster-1",
			clusterConfig:          "cluster-1-config-1",
			expectedConfigCreation: false,
		},
		{
			clusterName:            "cluster-1",
			clusterConfig:          "cluster-1-config-2",
			expectedConfigCreation: true,
		},
		{
			clusterName:            "cluster-1",
			clusterConfig:          "cluster-1-config-1",
			expectedConfigCreation: true,
		},
		{
			clusterName:            "cluster-2",
			clusterConfig:          "cluster-2-config-1",
			expectedConfigCreation: true,
		},
		{
			clusterName:            "cluster-2",
			clusterConfig:          "cluster-1-config-1",
			expectedConfigCreation: true,
		},
		{
			clusterName:            "cluster-2",
			clusterConfig:          "cluster-2-config-1",
			expectedConfigCreation: false,
		},
	}

	newClientCh := make(chan struct{}, 1)
	remoteClient := NewClusterClient(func(config []byte) (dynamic.Interface, error) {
		newClientCh <- struct{}{}
		return fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), nil), nil
	})

	for i, ts := range tss {
		err := remoteClient.UpdateCluster(ts.clusterName, []byte(ts.clusterConfig))
		if err != nil {
			t.Fatal(err)
		}
		if ts.expectedConfigCreation {
			select {
			case <-newClientCh:
			default:
				t.Fatalf("expected new config to be created at step %d but it wasn't", i)
			}
		}
	}

}

func TestClusterClient_AddDeleteCluster(t *testing.T) {
	newClientCh := make(chan struct{}, 1)
	clusterClient := NewClusterClient(func(config []byte) (dynamic.Interface, error) {
		newClientCh <- struct{}{}
		return fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), nil), nil
	})

	clusterName := "foo"
	err := clusterClient.UpdateCluster(clusterName, []byte("bar"))
	if err != nil {
		t.Fatal(err)
	}

	select {
	case <-newClientCh:
	default:
		t.Error("expected new config to be created but it wasn't")
	}

	_, err = clusterClient.Cluster(clusterName)
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}

	clusterClient.DeleteCluster(clusterName)
	_, err = clusterClient.Cluster(clusterName)
	if err == nil {
		t.Errorf("expected non-nil error, got nil")
	}
}
