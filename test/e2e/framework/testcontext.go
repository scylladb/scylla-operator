// Copyright (C) 2021 ScyllaDB

package framework

import (
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	restclient "k8s.io/client-go/rest"
)

var TestContext *TestContextType

type DeleteTestingNSPolicyType string

var (
	DeleteTestingNSPolicyAlways    DeleteTestingNSPolicyType = "Always"
	DeleteTestingNSPolicyOnSuccess DeleteTestingNSPolicyType = "OnSuccess"
	DeleteTestingNSPolicyNever     DeleteTestingNSPolicyType = "Never"
)

type IngressController struct {
	IngressClassName  string
	Address           string
	CustomAnnotations map[string]string
}

type ScyllaClusterOptions struct {
	ExposeOptions ExposeOptions
}

type ExposeOptions struct {
	NodeServiceType             scyllav1.NodeServiceType
	NodesBroadcastAddressType   scyllav1.BroadcastAddressType
	ClientsBroadcastAddressType scyllav1.BroadcastAddressType
}

type ObjectStorageType string

const (
	ObjectStorageTypeNone ObjectStorageType = "None"
	ObjectStorageTypeGCS  ObjectStorageType = "GCS"
)

type TestContextType struct {
	RestConfigs           []*restclient.Config
	ArtifactsDir          string
	DeleteTestingNSPolicy DeleteTestingNSPolicyType
	IngressController     *IngressController
	ScyllaClusterOptions  *ScyllaClusterOptions
	ObjectStorageType     ObjectStorageType
	ObjectStorageBucket   string
	GCSServiceAccountKey  []byte
}
