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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ScyllaClusterManagerSpec defines the desired state of ScyllaClusterManager.
type ScyllaClusterManagerSpec struct {
}

// ScyllaClusterManagerStatus defines the observed state of ScyllaClusterManager.
type ScyllaClusterManagerStatus struct {
	// observedGeneration is the most recent generation observed for this ScyllaClusterManager. It corresponds to the
	// ScyllaClusterManager's generation, which is updated on mutation by the API Server.
	// +optional
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ScyllaClusterManager defines a Scylla cluster.
type ScyllaClusterManager struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec defines the desired state of this scylla cluster manager.
	Spec ScyllaClusterManagerSpec `json:"spec,omitempty"`

	// status is the current status of this scylla cluster manager.
	Status ScyllaClusterManagerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ScyllaClusterManagerList holds a list of ScyllaClusterManagers.
type ScyllaClusterManagerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ScyllaClusterManager `json:"items"`
}
