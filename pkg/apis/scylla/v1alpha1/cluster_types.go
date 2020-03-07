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

// Cluster is the Schema for the clusters API
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
type Cluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterSpec   `json:"spec,omitempty"`
	Status ClusterStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterList contains a list of Cluster
type ClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Cluster `json:"items"`
}

// ClusterSpec is the desired state for a Scylla Cluster.
type ClusterSpec struct {
	// Version of Scylla to use.
	Version string `json:"version"`
	// Repository to pull the image from.
	Repository *string `json:"repository,omitempty"`
	// Version of Scylla Manager Agent to use. Defaults to "latest".
	AgentVersion *string `json:"agentVersion"`
	// Repository to pull the agent image from. Defaults to "scylladb/scylla-manager-agent".
	AgentRepository *string `json:"agentRepository,omitempty"`
	// DeveloperMode determines if the cluster runs in developer-mode.
	DeveloperMode bool `json:"developerMode,omitempty"`
	// CpuSet determines if the cluster will use cpu-pinning for max performance.
	CpuSet bool `json:"cpuset,omitempty"`
	// Datacenter that will make up this cluster.
	Datacenter DatacenterSpec `json:"datacenter"`
	// User-provided image for the sidecar that replaces default.
	SidecarImage *ImageSpec `json:"sidecarImage,omitempty"`
}

// DatacenterSpec is the desired state for a Scylla Datacenter.
type DatacenterSpec struct {
	// Name of the Scylla Datacenter. Used in the cassandra-rackdc.properties file.
	Name string `json:"name"`
	// Racks of the specific Datacenter.
	Racks []RackSpec `json:"racks"`
}

// RackSpec is the desired state for a Scylla Rack.
type RackSpec struct {
	// Name of the Scylla Rack. Used in the cassandra-rackdc.properties file.
	Name string `json:"name"`
	// Members is the number of Scylla instances in this rack.
	Members int32 `json:"members"`
	// Storage describes the underlying storage that Scylla will consume.
	Storage StorageSpec `json:"storage"`
	// Placement describes restrictions for the nodes Scylla is scheduled on.
	Placement *PlacementSpec `json:"placement,omitempty"`
	// Resources the Scylla Pods will use.
	Resources corev1.ResourceRequirements `json:"resources"`
	// Scylla config map name to customize scylla.yaml
	ScyllaConfig string `json:"scyllaConfig"`
	// Scylla config map name to customize scylla manager agent
	ScyllaAgentConfig string `json:"scyllaAgentConfig"`
}

type PlacementSpec struct {
	corev1.Affinity
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
}

// ImageSpec is the desired state for a container image.
type ImageSpec struct {
	// Version of the image.
	Version string `json:"version"`
	// Repository to pull the image from.
	Repository string `json:"repository"`
}

type StorageSpec struct {
	// Capacity of each member's volume
	Capacity string `json:"capacity"`
	// Name of storageClass to request
	StorageClassName *string `json:"storageClassName,omitempty"`
}

// ClusterStatus is the status of a Scylla Cluster
type ClusterStatus struct {
	Racks map[string]*RackStatus `json:"racks,omitempty"`
}

// RackStatus is the status of a Scylla Rack
type RackStatus struct {
	// Version is the current version of Scylla in use.
	Version string `json:"version"`
	// Members is the current number of members requested in the specific Rack
	Members int32 `json:"members"`
	// ReadyMembers is the number of ready members in the specific Rack
	ReadyMembers int32 `json:"readyMembers"`
	// Conditions are the latest available observations of a rack's state.
	Conditions []RackCondition `json:"conditions,omitempty"`
}

// RackCondition is an observation about the state of a rack.
type RackCondition struct {
	Type   RackConditionType      `json:"type"`
	Status corev1.ConditionStatus `json:"status"`
}

type RackConditionType string

const (
	RackConditionTypeMemberLeaving RackConditionType = "MemberLeaving"
	RackConditionTypeUpgrading     RackConditionType = "RackUpgrading"
)

func init() {
	SchemeBuilder.Register(&Cluster{}, &ClusterList{})
}
