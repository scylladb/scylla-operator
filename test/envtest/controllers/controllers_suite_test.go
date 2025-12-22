//go:build envtest

package controllers

import (
	"testing"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
)

func TestEnvtest(t *testing.T) {
	o.RegisterFailHandler(g.Fail)
	g.RunSpecs(t, "Controllers Suite")
}
