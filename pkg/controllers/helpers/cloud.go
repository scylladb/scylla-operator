// Copyright (C) 2021 ScyllaDB

package helpers

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
)

type CloudPlatform string

const (
	AKSPlatform     CloudPlatform = "AKS"
	GKEPlatform     CloudPlatform = "GKE"
	EKSPlatform     CloudPlatform = "EKS"
	UnknownPlatform CloudPlatform = "Unknown"
)

func GetCloudPlatform(node *corev1.Node) CloudPlatform {
	if strings.HasSuffix(node.Status.NodeInfo.KernelVersion, "gke") {
		return GKEPlatform
	}
	if strings.HasSuffix(node.Status.NodeInfo.KernelVersion, "amzn2.x86_64") {
		return EKSPlatform
	}
	if strings.HasSuffix(node.Status.NodeInfo.KernelVersion, "azure") {
		return AKSPlatform
	}

	return UnknownPlatform
}
