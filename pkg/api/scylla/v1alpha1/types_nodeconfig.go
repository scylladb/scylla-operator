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

type NodeConfigConditionType string

const (
	// Reconciled indicates that the NodeConfig is fully deployed and available.
	NodeConfigReconciledConditionType NodeConfigConditionType = "Reconciled"
)

type NodeConfigCondition struct {
	// type is the type of the NodeConfig condition.
	Type NodeConfigConditionType `json:"type"`

	// status represents the state of the condition, one of True, False, or Unknown.
	Status corev1.ConditionStatus `json:"status"`

	// lastTransitionTime is last time the condition transitioned from one status to another.
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`

	// reason is the reason for condition's last transition.
	Reason string `json:"reason"`

	// message is a human-readable message indicating details about the transition.
	Message string `json:"message"`
}

type NodeConfigNodeStatus struct {
	Name            string   `json:"name"`
	TunedNode       bool     `json:"tunedNode"`
	TunedContainers []string `json:"tunedContainers"`
}

type NodeConfigStatus struct {
	// observedGeneration indicates the most recent generation observed by the controller.
	ObservedGeneration int64 `json:"observedGeneration"`

	// conditions represents the latest available observations of current state.
	// +optional
	Conditions []NodeConfigCondition `json:"conditions"`

	// nodeStatuses hold the status for each tuned node.
	NodeStatuses []NodeConfigNodeStatus `json:"nodeStatuses"`
}

type NodeConfigPlacement struct {
	// affinity is a group of affinity scheduling rules for NodeConfig Pods.
	Affinity corev1.Affinity `json:"affinity"`

	// tolerations is a group of tolerations NodeConfig Pods are going to have.
	Tolerations []corev1.Toleration `json:"tolerations"`

	// nodeSelector is a selector which must be true for the NodeConfig Pod to fit on a node.
	// Selector which must match a node's labels for the pod to be scheduled on that node.
	// +kubebuilder:validation:Required
	NodeSelector map[string]string `json:"nodeSelector"`
}

type NodeConfigSpec struct {
	// placement contains scheduling rules for NodeConfig Pods.
	// +kubebuilder:validation:Required
	Placement NodeConfigPlacement `json:"placement"`

	// disableOptimizations controls if nodes matching placement requirements
	// are going to be optimized. Turning off optimizations on already optimized
	// Nodes does not revert changes.
	DisableOptimizations bool `json:"disableOptimizations"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:resource:path=nodeconfigs,scope=Cluster
// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type NodeConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NodeConfigSpec   `json:"spec,omitempty"`
	Status NodeConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type NodeConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NodeConfig `json:"items"`
}
