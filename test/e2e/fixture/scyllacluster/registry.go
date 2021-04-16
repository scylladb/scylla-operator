package scyllacluster

import (
	_ "embed"

	o "github.com/onsi/gomega"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/test/e2e/scheme"
)

var (
	//go:embed "basic.scyllacluster.yaml"
	BasicScyllaCluster ScyllaCluster

	// BasicFastScyllaCluster contain several Scylla parameters which speed up Scylla bootstrapping.
	//go:embed "basic-fast.scyllacluster.yaml"
	BasicFastScyllaCluster ScyllaCluster
)

type ScyllaCluster []byte

func (sc ScyllaCluster) ReadOrFail() *scyllav1.ScyllaCluster {
	obj, _, err := scheme.Codecs.UniversalDeserializer().Decode(sc, nil, nil)
	o.Expect(err).NotTo(o.HaveOccurred())

	return obj.(*scyllav1.ScyllaCluster)
}
