package controllerhelpers

import (
	"encoding/json"
	"fmt"

	"github.com/scylladb/scylla-operator/pkg/internalapi"
	"github.com/scylladb/scylla-operator/pkg/naming"
	corev1 "k8s.io/api/core/v1"
)

func GetSidecarRuntimeConfigFromConfigMap(cm *corev1.ConfigMap) (*internalapi.SidecarRuntimeConfig, error) {
	if cm.Data == nil {
		return nil, fmt.Errorf("no data found in configmap %q", naming.ObjRef(cm))
	}

	srcData, found := cm.Data[naming.ScyllaRuntimeConfigKey]
	if !found {
		return nil, fmt.Errorf("no key %q found in configmap %q", naming.ScyllaRuntimeConfigKey, naming.ObjRef(cm))
	}

	src := &internalapi.SidecarRuntimeConfig{}
	err := json.Unmarshal([]byte(srcData), src)
	if err != nil {
		return nil, fmt.Errorf("can't unmarshal sidecar runtime config in configmap %q: %w", naming.ObjRef(cm), err)
	}

	return src, nil
}
