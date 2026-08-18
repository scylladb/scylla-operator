//go:build envtest

package controllers

import (
	"testing"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"k8s.io/klog/v2"
)

// Placeholder images for objects that are never rolled out in envtests, where no container ever has to be pulled,
// but the Operator's validation requires the image references to be set and parsable.
const (
	envtestScyllaDBImage             = "scylladb/scylla:envtest"
	envtestScyllaDBManagerAgentImage = "scylladb/scylla-manager-agent:envtest"
)

func init() {
	klog.InitFlags(nil)
}

func TestEnvtest(t *testing.T) {
	o.RegisterFailHandler(g.Fail)
	g.RunSpecs(t, "Controllers Suite")
}
