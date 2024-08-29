// Copyright (C) 2021 ScyllaDB

package framework

import (
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	restclient "k8s.io/client-go/rest"
)

var TestContext *TestContextType

type CleanupPolicyType string

var (
	CleanupPolicyAlways    CleanupPolicyType = "Always"
	CleanupPolicyOnSuccess CleanupPolicyType = "OnSuccess"
	CleanupPolicyNever     CleanupPolicyType = "Never"
)

type IngressController struct {
	IngressClassName  string
	Address           string
	CustomAnnotations map[string]string
}

type ScyllaClusterOptions struct {
	ExposeOptions    ExposeOptions
	StorageClassName string
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
	ObjectStorageTypeS3   ObjectStorageType = "S3"
)

type TestContextType struct {
	RestConfigs          []*restclient.Config
	ArtifactsDir         string
	CleanupPolicy        CleanupPolicyType
	IngressController    *IngressController
	ScyllaClusterOptions *ScyllaClusterOptions
	ObjectStorageType    ObjectStorageType
	ObjectStorageBucket  string
	GCSServiceAccountKey []byte
	S3CredentialsFile    []byte
}
