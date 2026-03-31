package coredumps

import (
	_ "embed"

	o "github.com/onsi/gomega"
	"github.com/scylladb/scylla-operator/test/e2e/scheme"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

var (
	//go:embed coredump-conf.configmap.yaml
	CoredumpConfConfigMap CoredumpConfConfigMapBytes

	//go:embed setup-systemd-coredump.daemonset.yaml
	SetupSystemdCoredumpDaemonSet SetupSystemdCoredumpDaemonSetBytes
)

type CoredumpConfConfigMapBytes []byte

func (b CoredumpConfConfigMapBytes) ReadOrFail() *corev1.ConfigMap {
	obj, _, err := scheme.Codecs.UniversalDeserializer().Decode(b, nil, nil)
	o.Expect(err).NotTo(o.HaveOccurred())

	return obj.(*corev1.ConfigMap)
}

type SetupSystemdCoredumpDaemonSetBytes []byte

func (b SetupSystemdCoredumpDaemonSetBytes) ReadOrFail() *appsv1.DaemonSet {
	obj, _, err := scheme.Codecs.UniversalDeserializer().Decode(b, nil, nil)
	o.Expect(err).NotTo(o.HaveOccurred())

	return obj.(*appsv1.DaemonSet)
}
