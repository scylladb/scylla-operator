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

package v1

import (
	"github.com/blang/semver"
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
	// AutomaticOrphanedNodeCleanup controls if automatic orphan node cleanup should be performed.
	AutomaticOrphanedNodeCleanup bool `json:"automaticOrphanedNodeCleanup,omitempty"`
	// GenericUpgrade allows to configure behavior of generic upgrade logic.
	GenericUpgrade *GenericUpgradeSpec `json:"genericUpgrade,omitempty"`
	// Datacenter that will make up this cluster.
	Datacenter DatacenterSpec `json:"datacenter"`
	// Sysctl properties to be applied during initialization
	// given as a list of key=value pairs.
	// Example: fs.aio-max-nr=232323
	Sysctls    []string `json:"sysctls,omitempty"`
	ScyllaArgs string   `json:"scyllaArgs,omitempty"`
	// Networking config
	Network Network `json:"network,omitempty"`
	// Repairs specifies repair task in Scylla Manager.
	// When Scylla Manager is not installed, these will be ignored.
	Repairs []RepairTaskSpec `json:"repairs,omitempty"`
	// Backups specifies backup task in Scylla Manager.
	// When Scylla Manager is not installed, these will be ignored.
	Backups []BackupTaskSpec `json:"backups,omitempty"`
}

// GenericUpgradeFailureStrategy allows to specify how upgrade logic should handle failures.
type GenericUpgradeFailureStrategy string

const (
	// GenericUpgradeFailureStrategyRetry infinitely retry until node becomes ready.
	GenericUpgradeFailureStrategyRetry GenericUpgradeFailureStrategy = "Retry"
)

// GenericUpgradeSpec hold generic upgrade procedure parameters.
type GenericUpgradeSpec struct {
	// FailureStrategy specifies which logic is executed when upgrade failure happens.
	// Currently only Retry is supported.
	FailureStrategy GenericUpgradeFailureStrategy `json:"failureStrategy,omitempty"`
	// PollInterval specifies how often upgrade logic polls on state updates.
	// Increasing this value should lower number of requests sent to apiserver, but it may affect
	// overall time spent during upgrade.
	PollInterval *metav1.Duration `json:"pollInterval,omitempty"`
}

type SchedulerTaskSpec struct {
	// Name of a task, it must be unique across all tasks.
	Name string `json:"name"`
	// StartDate specifies the task start date expressed in the RFC3339 format or now[+duration],
	// e.g. now+3d2h10m, valid units are d, h, m, s (default "now").
	StartDate *string `json:"startDate,omitempty"`
	// Interval task schedule interval e.g. 3d2h10m, valid units are d, h, m, s (default "0").
	Interval *string `json:"interval,omitempty"`
	// NumRetries the number of times a scheduled task will retry to run before failing (default 3).
	NumRetries *int64 `json:"numRetries,omitempty"`
}

type RepairTaskSpec struct {
	SchedulerTaskSpec `json:",inline"`
	// DC list of datacenter glob patterns, e.g. 'dc1', '!otherdc*' used to specify the DCs
	// to include or exclude from backup.
	DC []string `json:"dc,omitempty" mapstructure:"dc,omitempty"`
	// FailFast stop repair on first error.
	FailFast *bool `json:"failFast,omitempty" mapstructure:"fail_fast,omitempty"`
	// Intensity how many token ranges (per shard) to repair in a single Scylla repair job. By default this is 1.
	// If you set it to 0 the number of token ranges is adjusted to the maximum supported by node (see max_repair_ranges_in_parallel in Scylla logs).
	// Valid values are 0 and integers >= 1. Higher values will result in increased cluster load and slightly faster repairs.
	// Changing the intensity impacts repair granularity if you need to resume it, the higher the value the more work on resume.
	// For Scylla clusters that *do not support row-level repair*, intensity can be a decimal between (0,1).
	// In that case it specifies percent of shards that can be repaired in parallel on a repair master node.
	// For Scylla clusters that are row-level repair enabled, setting intensity below 1 has the same effect as setting intensity 1.
	Intensity *string `json:"intensity,omitempty" mapstructure:"intensity,omitempty"`
	// Parallel the maximum number of Scylla repair jobs that can run at the same time (on different token ranges and replicas).
	// Each node can take part in at most one repair at any given moment. By default the maximum possible parallelism is used.
	// The effective parallelism depends on a keyspace replication factor (RF) and the number of nodes.
	// The formula to calculate it is as follows: number of nodes / RF, ex. for 6 node cluster with RF=3 the maximum parallelism is 2.
	Parallel *int64 `json:"parallel,omitempty" mapstructure:"parallel,omitempty"`
	// Keyspace a list of keyspace/tables glob patterns, e.g. 'keyspace,!keyspace.table_prefix_*'
	// used to include or exclude keyspaces from repair.
	Keyspace []string `json:"keyspace,omitempty" mapstructure:"keyspace,omitempty"`
	// SmallTableThreshold enable small table optimization for tables of size lower than given threshold.
	// Supported units [B, MiB, GiB, TiB] (default "1GiB").
	SmallTableThreshold *string `json:"smallTableThreshold,omitempty" mapstructure:"small_table_threshold,omitempty"`
	// Host to repair, by default all hosts are repaired
	Host *string `json:"host,omitempty" mapstructure:"host,omitempty"`
}

type BackupTaskSpec struct {
	SchedulerTaskSpec `json:",inline"`
	// DC a list of datacenter glob patterns, e.g. 'dc1,!otherdc*' used to specify the DCs
	// to include or exclude from backup.
	DC []string `json:"dc,omitempty" mapstructure:"dc,omitempty"`
	// Keyspace a list of keyspace/tables glob patterns,
	// e.g. 'keyspace,!keyspace.table_prefix_*' used to include or exclude keyspaces from repair.
	Keyspace []string `json:"keyspace,omitempty" mapstructure:"keyspace,omitempty"`
	// Location a list of backup locations in the format [<dc>:]<provider>:<name> ex. s3:my-bucket.
	// The <dc>: part is optional and is only needed when different datacenters are being used to upload data
	// to different locations. <name> must be an alphanumeric string and may contain a dash and or a dot,
	// but other characters are forbidden.
	// The only supported storage <provider> at the moment are s3 and gcs.
	Location []string `json:"location" mapstructure:"location,omitempty"`
	// RateLimit a list of megabytes (MiB) per second rate limits expressed in the format [<dc>:]<limit>.
	// The <dc>: part is optional and only needed when different datacenters need different upload limits.
	// Set to 0 for no limit (default 100).
	RateLimit []string `json:"rateLimit,omitempty" mapstructure:"rate_limit,omitempty"`
	// Retention The number of backups which are to be stored (default 3).
	Retention *int64 `json:"retention,omitempty" mapstructure:"retention,omitempty"`
	// SnapshotParallel a list of snapshot parallelism limits in the format [<dc>:]<limit>.
	// The <dc>: part is optional and allows for specifying different limits in selected datacenters.
	// If The <dc>: part is not set, the limit is global (e.g. 'dc1:2,5') the runs are parallel in n nodes (2 in dc1)
	// and n nodes in all the other datacenters.
	SnapshotParallel []string `json:"snapshotParallel,omitempty" mapstructure:"snapshot_parallel,omitempty"`
	// UploadParallel a list of upload parallelism limits in the format [<dc>:]<limit>.
	// The <dc>: part is optional and allows for specifying different limits in selected datacenters.
	// If The <dc>: part is not set the limit is global (e.g. 'dc1:2,5') the runs are parallel in n nodes (2 in dc1)
	// and n nodes in all the other datacenters.
	UploadParallel []string `json:"uploadParallel,omitempty" mapstructure:"upload_parallel,omitempty"`
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
	// Resources the Scylla container will use.
	Resources corev1.ResourceRequirements `json:"resources"`
	// AgentResources which Agent container will use.
	AgentResources corev1.ResourceRequirements `json:"agentResources,omitempty"`
	// Volumes added to Scylla Pod.
	Volumes []corev1.Volume `json:"volumes,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`
	// VolumeMounts to be added to Scylla container.
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty" patchStrategy:"merge" patchMergeKey:"mountPath"`
	// AgentVolumeMounts to be added to Agent container.
	AgentVolumeMounts []corev1.VolumeMount `json:"agentVolumeMounts,omitempty" patchStrategy:"merge" patchMergeKey:"mountPath"`
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
	Port           int32  `json:"port,omitempty"`
	WriteIsolation string `json:"writeIsolation,omitempty"`
}

func (a *AlternatorSpec) Enabled() bool {
	return a != nil && a.Port > 0
}

type RepairTaskStatus struct {
	RepairTaskSpec `json:",inline" mapstructure:",squash"`
	ID             string `json:"id"`
	Error          string `json:"error"`
}

type BackupTaskStatus struct {
	BackupTaskSpec `json:",inline"`
	ID             string `json:"id"`
	Error          string `json:"error"`
}

// ClusterStatus defines the observed state of ScyllaCluster
type ClusterStatus struct {
	// Racks reflect status of cluster racks.
	Racks map[string]RackStatus `json:"racks,omitempty"`
	// ManagerID contains ID under which cluster was registered in Scylla Manager.
	ManagerID *string `json:"managerId,omitempty"`
	// Repairs reflects status of repair tasks.
	Repairs []RepairTaskStatus `json:"repairs,omitempty"`
	// Backups reflects status of backup tasks.
	Backups []BackupTaskStatus `json:"backups,omitempty"`
	// Upgrade reflects state of ongoing upgrade procedure.
	Upgrade *UpgradeStatus `json:"upgrade,omitempty"`
}

// UpgradeStatus contains state of ongoing upgrade procedure.
type UpgradeStatus struct {
	// State reflects current upgrade state.
	State string `json:"state"`
	// CurrentNode node under upgrade.
	CurrentNode string `json:"currentNode,omitempty"`
	// CurrentRack rack under upgrade.
	CurrentRack string `json:"currentRack,omitempty"`
	// FromVersion reflects from which version ScyllaCluster is being upgraded.
	FromVersion string `json:"fromVersion"`
	// ToVersion reflects to which version ScyllaCluster is being upgraded.
	ToVersion string `json:"toVersion"`
	// SystemSnapshotTag snapshot tag of system keyspaces.
	SystemSnapshotTag string `json:"systemSnapshotTag,omitempty"`
	// DataSnapshotTag snapshot tag of data keyspaces.
	DataSnapshotTag string `json:"dataSnapshotTag,omitempty"`
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
	// Pool of addresses which should be replaced by new nodes.
	ReplaceAddressFirstBoot map[string]string `json:"replace_address_first_boot,omitempty"`
}

// RackCondition is an observation about the state of a rack.
type RackCondition struct {
	Type   RackConditionType      `json:"type"`
	Status corev1.ConditionStatus `json:"status"`
}

type RackConditionType string

const (
	RackConditionTypeMemberLeaving   RackConditionType = "MemberLeaving"
	RackConditionTypeUpgrading       RackConditionType = "RackUpgrading"
	RackConditionTypeMemberReplacing RackConditionType = "MemberReplacing"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion

// Cluster is the Schema for the clusters API
type ScyllaCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterSpec   `json:"spec,omitempty"`
	Status ClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterList contains a list of Cluster
type ScyllaClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ScyllaCluster `json:"items"`
}

// Version of scylla docker starting from which passing arguments via entry-point is supported
var ScyllaVersionThatSupportsArgsText = "4.2.0"
var ScyllaVersionThatSupportsArgs semver.Version

func init() {
	var err error
	if ScyllaVersionThatSupportsArgs, err = semver.Parse(ScyllaVersionThatSupportsArgsText); err != nil {
		panic(err)
	}
	SchemeBuilder.Register(&ScyllaCluster{}, &ScyllaClusterList{})
}
