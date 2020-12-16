// Copyright (C) 2017 ScyllaDB

package slices

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
)

var _ = Describe("Slices tests", func() {
	DescribeTable("ContainsString", func(v string, arr []string, matcher types.GomegaMatcher) {
		Expect(ContainsString(v, arr)).To(matcher)
	},
		Entry("empty", "a", []string{}, BeFalse()),
		Entry("contain element", "a", []string{"a", "b"}, BeTrue()),
		Entry("at the end", "b", []string{"a", "b"}, BeTrue()),
		Entry("does not contain element", "c", []string{"a", "b"}, BeFalse()),
	)
})

func TestSlices(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Slices Suite",
		[]Reporter{printer.NewlineReporter{}})
}
