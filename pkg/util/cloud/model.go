// Copyright (C) 2021 ScyllaDB

package cloud

type CloudProvider string

const (
	EKSCloud     CloudProvider = "EKS"
	GKECloud     CloudProvider = "GKE"
	UnknownCloud CloudProvider = "Unknown"
)

type InstanceMetadata struct {
	InstanceType  string
	CloudProvider CloudProvider
}
