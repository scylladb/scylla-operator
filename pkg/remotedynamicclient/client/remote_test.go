// Copyright (c) 2022 ScyllaDB.

package client

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/fake"
)

func TestRemoteDynamicClient_RefreshOnRegionConfigChange(t *testing.T) {
	tss := []struct {
		region                 string
		regionConfig           string
		expectedConfigCreation bool
	}{
		{
			region:                 "region-1",
			regionConfig:           "region-1-config-1",
			expectedConfigCreation: true,
		},
		{
			region:                 "region-1",
			regionConfig:           "region-1-config-1",
			expectedConfigCreation: false,
		},
		{
			region:                 "region-1",
			regionConfig:           "region-1-config-2",
			expectedConfigCreation: true,
		},
		{
			region:                 "region-1",
			regionConfig:           "region-1-config-1",
			expectedConfigCreation: true,
		},
		{
			region:                 "region-2",
			regionConfig:           "region-2-config-1",
			expectedConfigCreation: true,
		},
		{
			region:                 "region-2",
			regionConfig:           "region-1-config-1",
			expectedConfigCreation: true,
		},
		{
			region:                 "region-2",
			regionConfig:           "region-2-config-1",
			expectedConfigCreation: false,
		},
	}

	newClientCh := make(chan struct{}, 1)
	remoteClient := NewRemoteDynamicClient(WithCustomClientFactory(func(config []byte) (dynamic.Interface, error) {
		newClientCh <- struct{}{}
		return fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), nil), nil
	}))

	for i, ts := range tss {
		if err := remoteClient.Update(ts.region, []byte(ts.regionConfig)); err != nil {
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
