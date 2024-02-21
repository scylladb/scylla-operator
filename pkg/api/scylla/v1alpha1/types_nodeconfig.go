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
	"k8s.io/apimachinery/pkg/api/resource"
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

	// observedGeneration represents the .metadata.generation that the condition was set based upon.
	// For instance, if .metadata.generation is currently 12, but the .status.conditions[x].observedGeneration is 9, the condition is out of date
	// with respect to the current state of the instance.
	// +optional
	// +kubebuilder:validation:Minimum=0
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

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

// DeviceDiscovery specifies options for device discovery.
type DeviceDiscovery struct {
	// nameRegex is a regular expression filtering devices by their name.
	// +optional
	NameRegex string `json:"nameRegex"`

	// modelRegex is a regular expression filtering devices by their model name.
	// +optional
	ModelRegex string `json:"modelRegex"`
}

// RAID0Options specifies raid0 options.
type RAID0Options struct {
	// devices defines which devices constitute the raid array.
	Devices DeviceDiscovery `json:"devices"`
}

// RAIDType is a raid array type.
type RAIDType string

const (
	// RAID0Type represents RAID0 array type.
	RAID0Type RAIDType = "RAID0"
)

// RAIDConfiguration is a configuration of a raid array.
type RAIDConfiguration struct {
	// name specifies the name of the raid device to be created under in `/dev/md/`.
	Name string `json:"name"`

	// type is a type of raid array.
	Type RAIDType `json:"type"`

	// RAID0 specifies RAID0 options.
	// +optional
	RAID0 *RAID0Options `json:"RAID0,omitempty"`
}

// FilesystemType is a type of filesystem.
type FilesystemType string

const (
	// XFSFilesystem represents an XFS filesystem type.
	XFSFilesystem FilesystemType = "xfs"
)

// FilesystemConfiguration specifies filesystem configuration options.
type FilesystemConfiguration struct {
	// device is a path to the device where the desired filesystem should be created.
	Device string `json:"device"`

	// type is a desired filesystem type.
	Type FilesystemType `json:"type"`
}

// MountConfiguration specifies mount configuration options.
type MountConfiguration struct {
	// device is path to a device that should be mounted.
	Device string `json:"device"`

	// mountPoint is a path where the device should be mounted at.
	MountPoint string `json:"mountPoint"`

	// fsType specifies the filesystem on the device.
	FSType string `json:"fsType"`

	// unsupportedOptions is a list of mount options used during device mounting.
	// unsupported in this field name means that we won't support all the available options passed down using this field.
	// +optional
	UnsupportedOptions []string `json:"unsupportedOptions"`
}

// LoopDeviceConfiguration specifies loop device configuration options.
type LoopDeviceConfiguration struct {
	// name specifies the name of the symlink that will point to actual loop device, created under `/dev/loops/`.
	Name string `json:"name"`

	// imagePath specifies path on host where backing image file for loop device should be located.
	ImagePath string `json:"imagePath"`

	// size specifies the size of the loop device.
	Size resource.Quantity `json:"size"`
}

// LocalDiskSetup specifies configuration of local disk setup.
type LocalDiskSetup struct {
	// loops is a list of loop device configurations.
	LoopDevices []LoopDeviceConfiguration `json:"loopDevices"`

	// raids is a list of raid configurations.
	RAIDs []RAIDConfiguration `json:"raids"`

	// filesystems is a list of filesystem configurations.
	Filesystems []FilesystemConfiguration `json:"filesystems"`

	// mounts is a list of mount configuration.
	Mounts []MountConfiguration `json:"mounts"`
}

type NodeConfigSpec struct {
	// placement contains scheduling rules for NodeConfig Pods.
	// +kubebuilder:validation:Required
	Placement NodeConfigPlacement `json:"placement"`

	// disableOptimizations controls if nodes matching placement requirements
	// are going to be optimized. Turning off optimizations on already optimized
	// Nodes does not revert changes.
	DisableOptimizations bool `json:"disableOptimizations"`

	// localDiskSetup contains options of automatic local disk setup.
	// +optional
	LocalDiskSetup *LocalDiskSetup `json:"localDiskSetup"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:resource:path=nodeconfigs,scope=Cluster
// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

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
