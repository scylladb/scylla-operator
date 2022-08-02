package manager

import (
	_ "embed"

	o "github.com/onsi/gomega"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/test/e2e/scheme"
)

var (
	//go:embed "manager.yaml"
	Manager ManagerBytes
)

type ManagerBytes []byte

func (m ManagerBytes) ReadOrFail() *scyllav1alpha1.ScyllaManager {
	obj, _, err := scheme.Codecs.UniversalDeserializer().Decode(m, nil, nil)
	o.Expect(err).NotTo(o.HaveOccurred())

	return obj.(*scyllav1alpha1.ScyllaManager)
}
