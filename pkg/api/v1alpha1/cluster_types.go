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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ClusterSpec defines the desired state of Cluster
type ClusterSpec struct {
	// Important: Run "make" to regenerate code after modifying this file

	//TODO: add validation tags

	// Version of Scylla to use.
	Version string `json:"version"`
	// Repository to pull the image from.
	Repository *string `json:"repository,omitempty"`
	// Alternator designates this cluster an Alternator cluster
	Alternator *AlternatorSpec `json:"alternator,omitempty"`
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
	// Sysctl properties to be applied during initialization
	// given as a list of key=value pairs.
	// Example: fs.aio-max-nr=232323
	Sysctls []string `json:"sysctls,omitempty"`
	// Networking config
	Network Network `json:"network,omitempty"`
}

type Network struct {
	HostNetworking bool             `json:"hostNetworking,omitempty"`
	DNSPolicy      corev1.DNSPolicy `json:"dnsPolicy,omitempty"`
}

func (s Network) GetDNSPolicy() corev1.DNSPolicy {
	if s.DNSPolicy == "" {
		if s.HostNetworking {
			return corev1.DNSClusterFirstWithHostNet
		}
		return corev1.DNSDefault
	}
	return s.DNSPolicy
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
	Volumes   []corev1.Volume             `json:"volumes,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"name" protobuf:"bytes,1,rep,name=volumes"`
	// Volume mounts to be added to scylla nodes
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty" patchStrategy:"merge" patchMergeKey:"mountPath" protobuf:"bytes,9,rep,name=volumeMounts"`
	// Scylla config map name to customize scylla.yaml
	ScyllaConfig string `json:"scyllaConfig"`
	// Scylla config map name to customize scylla manager agent
	ScyllaAgentConfig string `json:"scyllaAgentConfig"`
}

type PlacementSpec struct {
	NodeAffinity    *corev1.NodeAffinity    `json:"nodeAffinity,omitempty"`
	PodAffinity     *corev1.PodAffinity     `json:"podAffinity,omitempty"`
	PodAntiAffinity *corev1.PodAntiAffinity `json:"podAntiAffinity,omitempty"`
	Tolerations     []corev1.Toleration     `json:"tolerations,omitempty"`
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

type AlternatorSpec struct {
	// Port on which to bind the Alternator API
	Port int32 `json:"port,omitempty"`
}

func (a *AlternatorSpec) Enabled() bool {
	return a != nil && a.Port > 0
}

// ClusterStatus defines the observed state of Cluster
type ClusterStatus struct {
	Racks map[string]RackStatus `json:"racks,omitempty"`
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

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Cluster is the Schema for the clusters API
type Cluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterSpec   `json:"spec,omitempty"`
	Status ClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterList contains a list of Cluster
type ClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Cluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Cluster{}, &ClusterList{})
}
