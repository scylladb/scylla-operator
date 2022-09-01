// Copyright (C) 2021 ScyllaDB

package framework

import (
	"github.com/spf13/cobra"
	restclient "k8s.io/client-go/rest"
)

var TestContext *TestContextType

type DeleteTestingNSPolicyType string

var (
	DeleteTestingNSPolicyAlways    DeleteTestingNSPolicyType = "Always"
	DeleteTestingNSPolicyOnSuccess DeleteTestingNSPolicyType = "OnSuccess"
	DeleteTestingNSPolicyNever     DeleteTestingNSPolicyType = "Never"
)

type ExternalStorageOptions struct {
	AccessKeyID     string
	SecretAccessKey string
	Provider        string
	Region          string
	Endpoint        string
}

func (o *ExternalStorageOptions) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&o.AccessKeyID, "storage-access-key-id", "", o.AccessKeyID, "Login to the external storage.")
	cmd.Flags().StringVarP(&o.SecretAccessKey, "storage-secret-access-key", "", o.SecretAccessKey, "Password to the external storage.")
	cmd.Flags().StringVarP(&o.Provider, "storage-provider", "", o.Provider, "Provider of the external storage.")
	cmd.Flags().StringVarP(&o.Region, "storage-region", "", o.Region, "Region of the external storage.")
	cmd.Flags().StringVarP(&o.Endpoint, "storage-endpoint", "", o.Endpoint, "Address of the external storage.")
}

type TestContextType struct {
	RestConfig            *restclient.Config
	ArtifactsDir          string
	DeleteTestingNSPolicy DeleteTestingNSPolicyType
	ExternalStorage       ExternalStorageOptions
}
