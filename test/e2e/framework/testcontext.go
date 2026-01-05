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
	ReactorBackend   string
}

func (o *ScyllaClusterOptions) ScyllaArgs() []string {
	var args []string

	if o.ReactorBackend != "" {
		args = append(args, "--reactor-backend="+o.ReactorBackend)
	}

	return args
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
	// RestConfig is the restclient.Config for the main cluster.
	// It also represents the "meta" or "control-plane" cluster in multi-datacenter setups.
	RestConfig *restclient.Config
	// WorkerRestConfigs contains a map of restclient.Configs for each worker cluster, keyed by the cluster identifier. Used in multi-datacenter setups.
	WorkerRestConfigs                  map[string]*restclient.Config
	ArtifactsDir                       string
	CleanupPolicy                      CleanupPolicyType
	IngressController                  *IngressController
	ScyllaClusterOptions               *ScyllaClusterOptions
	ScyllaDBVersion                    string
	ScyllaDBManagerVersion             string
	ScyllaDBManagerAgentVersion        string
	ScyllaDBUpdateFrom                 string
	ScyllaDBUpgradeFrom                string
	WorkerClusterObjectStorageSettings map[string]ClusterObjectStorageSettings
	ClusterObjectStorageSettings       *ClusterObjectStorageSettings
}
