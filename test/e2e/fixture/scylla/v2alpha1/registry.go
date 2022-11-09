// Copyright (c) 2022 ScyllaDB.

package v2alpha1

import (
	_ "embed"

	o "github.com/onsi/gomega"
	scyllav2alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v2alpha1"
	"github.com/scylladb/scylla-operator/test/e2e/scheme"
)

var (
	//go:embed "basic.scyllacluster.yaml"
	SingleDCScyllaCluster ScyllaClusterBytes
)

type ScyllaClusterBytes []byte

func (sc ScyllaClusterBytes) ReadOrFail() *scyllav2alpha1.ScyllaCluster {
	obj, _, err := scheme.Codecs.UniversalDeserializer().Decode(sc, nil, nil)
	o.Expect(err).NotTo(o.HaveOccurred())

	return obj.(*scyllav2alpha1.ScyllaCluster)
}
