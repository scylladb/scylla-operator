// Copyright (c) 2023 ScyllaDB.

package unit

import (
	_ "embed"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/scheme"
)

var (
	//go:embed "valid.nodeconfig.yaml"
	ValidNodeConfig NodeConfigBytes
)

type NodeConfigBytes []byte

func (sc NodeConfigBytes) ReadOrFail() *scyllav1alpha1.NodeConfig {
	obj, _, err := scheme.Codecs.UniversalDeserializer().Decode(sc, nil, nil)
	if err != nil {
		panic(err)
	}

	return obj.(*scyllav1alpha1.NodeConfig)
}
