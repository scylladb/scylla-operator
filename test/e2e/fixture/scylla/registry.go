package scylla

import (
	_ "embed"

	o "github.com/onsi/gomega"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/test/e2e/scheme"
)

var (
	//go:embed "basic.scyllacluster.yaml"
	BasicScyllaCluster ScyllaClusterBytes

	//go:embed "nodeconfig.yaml"
	NodeConfig NodeConfigBytes
)

type ScyllaClusterBytes []byte

func (sc ScyllaClusterBytes) ReadOrFail() *scyllav1.ScyllaCluster {
	obj, _, err := scheme.Codecs.UniversalDeserializer().Decode(sc, nil, nil)
	o.Expect(err).NotTo(o.HaveOccurred())

	return obj.(*scyllav1.ScyllaCluster)
}

type NodeConfigBytes []byte

func (sc NodeConfigBytes) ReadOrFail() *scyllav1alpha1.NodeConfig {
	obj, _, err := scheme.Codecs.UniversalDeserializer().Decode(sc, nil, nil)
	o.Expect(err).NotTo(o.HaveOccurred())

	return obj.(*scyllav1alpha1.NodeConfig)
}
