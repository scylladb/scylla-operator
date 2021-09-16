/*


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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ConditionType string

const (
	// ScyllaNodeConfigAvailable means the ScyllaNodeConfig is available, ie. replicas required are up and running.
	ScyllaNodeConfigAvailable ConditionType = "Available"

	// ScyllaNodeConfigScalingReason is added when ScyllaNodeConfig hasn't reached desired number of replicas.
	ScyllaNodeConfigScalingReason = "Scaling"

	// ScyllaNodeConfigReplicasReadyReason is added when all required ScyllaNodeConfig replicas are ready.
	ScyllaNodeConfigReplicasReadyReason = "ReplicasReady"
)

type Condition struct {
	// type of condition.
	Type ConditionType `json:"type"`

	// status of the condition, one of True, False, or Unknown.
	Status corev1.ConditionStatus `json:"status"`

	// lastTransitionTime is last time the condition transitioned from one status to another.
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`

	// reason for the condition's last transition.
	Reason string `json:"reason"`

	// message is a human-readable message indicating details about the transition.
	Message string `json:"message"`
}

type ReplicaCount struct {
	Desired int32 `json:"desired"`
	Actual  int32 `json:"actual"`
	Ready   int32 `json:"ready"`
}

type ScyllaNodeConfigStatus struct {
	// observedGeneration indicates the most recent generation observed by the controller.
	ObservedGeneration int64 `json:"observedGeneration"`

	// conditions represents the latest available observations of current state.
	Conditions []Condition `json:"conditions"`

	Current ReplicaCount `json:"current"`
	Updated ReplicaCount `json:"updated"`
}

type ScyllaNodeConfigPlacement struct {
	// affinity is a group of affinity scheduling rules for ScyllaNodeConfig Pods.
	Affinity corev1.Affinity `json:"affinity"`
	// tolerations is a group of tolerations ScyllaNodeConfig Pods are going to have.
	Tolerations []corev1.Toleration `json:"tolerations"`
	// nodeSelector is a selector which must be true for the ScyllaNodeConfig Pod to fit on a node.
	// Selector which must match a node's labels for the pod to be scheduled on that node.
	// +kubebuilder:default:={"scylla-operator.scylladb.com/node-pool": "scylla-pool"}
	NodeSelector map[string]string `json:"nodeSelector"`
}

type ScyllaNodeConfigSpec struct {
	// placement contains scheduling rules for ScyllaNodeConfig Pods.
	Placement ScyllaNodeConfigPlacement `json:"placement"`

	// optimizationsDisabled controls if nodes matching placement requirements
	// are going to be optimized. Turning off optimizations on already optimized
	// Nodes does not revert changes.
	DisableOptimizations bool `json:"disableOptimizations"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:resource:path=scyllanodeconfigs,scope=Cluster
// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ScyllaNodeConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ScyllaNodeConfigSpec   `json:"spec,omitempty"`
	Status ScyllaNodeConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ScyllaNodeConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ScyllaNodeConfig `json:"items"`
}
