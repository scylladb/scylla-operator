/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package util

import (
	v1helper "github.com/scylladb/scylla-operator/pkg/thirdparty/k8s.io/kubernetes/pkg/apis/core/v1/helper"
	v1 "k8s.io/api/core/v1"
)

// AddOrUpdateDaemonPodTolerations apply necessary tolerations to DaemonSet Pods, e.g. node.kubernetes.io/not-ready:NoExecute.
func AddOrUpdateDaemonPodTolerations(spec *v1.PodSpec) {
	// DaemonSet pods shouldn't be deleted by NodeController in case of node problems.
	// Add infinite toleration for taint notReady:NoExecute here
	// to survive taint-based eviction enforced by NodeController
	// when node turns not ready.
	v1helper.AddOrUpdateTolerationInPodSpec(spec, &v1.Toleration{
		Key:      v1.TaintNodeNotReady,
		Operator: v1.TolerationOpExists,
		Effect:   v1.TaintEffectNoExecute,
	})

	// DaemonSet pods shouldn't be deleted by NodeController in case of node problems.
	// Add infinite toleration for taint unreachable:NoExecute here
	// to survive taint-based eviction enforced by NodeController
	// when node turns unreachable.
	v1helper.AddOrUpdateTolerationInPodSpec(spec, &v1.Toleration{
		Key:      v1.TaintNodeUnreachable,
		Operator: v1.TolerationOpExists,
		Effect:   v1.TaintEffectNoExecute,
	})

	// According to TaintNodesByCondition feature, all DaemonSet pods should tolerate
	// MemoryPressure, DiskPressure, PIDPressure, Unschedulable and NetworkUnavailable taints.
	v1helper.AddOrUpdateTolerationInPodSpec(spec, &v1.Toleration{
		Key:      v1.TaintNodeDiskPressure,
		Operator: v1.TolerationOpExists,
		Effect:   v1.TaintEffectNoSchedule,
	})

	v1helper.AddOrUpdateTolerationInPodSpec(spec, &v1.Toleration{
		Key:      v1.TaintNodeMemoryPressure,
		Operator: v1.TolerationOpExists,
		Effect:   v1.TaintEffectNoSchedule,
	})

	v1helper.AddOrUpdateTolerationInPodSpec(spec, &v1.Toleration{
		Key:      v1.TaintNodePIDPressure,
		Operator: v1.TolerationOpExists,
		Effect:   v1.TaintEffectNoSchedule,
	})

	v1helper.AddOrUpdateTolerationInPodSpec(spec, &v1.Toleration{
		Key:      v1.TaintNodeUnschedulable,
		Operator: v1.TolerationOpExists,
		Effect:   v1.TaintEffectNoSchedule,
	})

	if spec.HostNetwork {
		v1helper.AddOrUpdateTolerationInPodSpec(spec, &v1.Toleration{
			Key:      v1.TaintNodeNetworkUnavailable,
			Operator: v1.TolerationOpExists,
			Effect:   v1.TaintEffectNoSchedule,
		})
	}
}
