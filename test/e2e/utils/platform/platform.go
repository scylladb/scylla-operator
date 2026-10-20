// Copyright (C) 2026 ScyllaDB

package platform

import (
	"context"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	operatorNamespace      = "scylla-operator"
	operatorDeploymentName = "scylla-operator"

	olmOwnerKindLabel = "olm.owner.kind"
	olmOwnerKindCSV   = "ClusterServiceVersion"
)

// Platform is how the operator under test was installed, which determines how tests can change its configuration.
type Platform string

const (
	// PlatformOLM is the operator installed by Operator Lifecycle Manager, e.g. on OpenShift.
	PlatformOLM Platform = "OLM"
	// PlatformOther is the operator installed in any other way, e.g. from the manifests.
	PlatformOther Platform = "Other"
)

// DetectPlatform detects the platform from the operator Deployment, which OLM labels with the kind of its owner.
func DetectPlatform(ctx context.Context, client kubernetes.Interface) Platform {
	g.GinkgoHelper()

	deploy, err := client.AppsV1().Deployments(operatorNamespace).Get(ctx, operatorDeploymentName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	if deploy.Labels[olmOwnerKindLabel] == olmOwnerKindCSV {
		return PlatformOLM
	}

	return PlatformOther
}
