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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ObjectTemplateMetadata struct {
	// labels is a custom key value map that gets merged with managed object labels.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// annotations is a custom key value map that gets merged with managed object annotations.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// ScyllaClusterSpec defines the desired state of Cluster.
type ScyllaClusterSpec struct {
	// podMetadata controls shared metadata for all pods created based on this spec.
	// +optional
	PodMetadata *ObjectTemplateMetadata `json:"podMetadata,omitempty"`

	// version is a version tag of Scylla to use.
	Version string `json:"version"`

	// repository is the image repository to pull the Scylla image from.
	// +kubebuilder:default:="docker.io/scylladb/scylla"
	// +optional
	Repository string `json:"repository,omitempty"`

	// alternator designates this cluster an Alternator cluster.
	// +optional
	Alternator *AlternatorSpec `json:"alternator,omitempty"`

	// agentVersion indicates the version of Scylla Manager Agent to use.
	// +kubebuilder:default:="latest"
	// +optional
	AgentVersion string `json:"agentVersion"`

	// agentRepository is the repository to pull the agent image from.
	// +kubebuilder:default:="docker.io/scylladb/scylla-manager-agent"
	// +optional
	AgentRepository string `json:"agentRepository,omitempty"`

	// developerMode determines if the cluster runs in developer-mode.
	// +optional
	DeveloperMode bool `json:"developerMode,omitempty"`

	// cpuset determines if the cluster will use cpu-pinning.
	// Deprecated: `cpuset` is deprecated. It is now treated as if it is always set to true regardless of its value.
	// +optional
	CpuSet bool `json:"cpuset,omitempty"`

	// automaticOrphanedNodeCleanup controls if automatic orphan node cleanup should be performed.
	AutomaticOrphanedNodeCleanup bool `json:"automaticOrphanedNodeCleanup,omitempty"`

	// genericUpgrade allows to configure behavior of generic upgrade logic.
	// +optional
	GenericUpgrade *GenericUpgradeSpec `json:"genericUpgrade,omitempty"`

	// datacenter holds a specification of a datacenter.
	Datacenter DatacenterSpec `json:"datacenter"`

	// sysctls holds the sysctl properties to be applied during initialization given as a list of key=value pairs.
	// Example: fs.aio-max-nr=232323
	// Deprecated: `sysctls` is deprecated. Use NodeConfig to configure sysctls instead.
	// See NodeConfig resource reference for details.
	// +optional
	Sysctls []string `json:"sysctls,omitempty"`

	// scyllaArgs will be appended to Scylla binary during startup.
	// This is supported from 4.2.0 Scylla version.
	// +optional
	ScyllaArgs string `json:"scyllaArgs,omitempty"`

	// network holds the networking config.
	// +optional
	Network Network `json:"network,omitempty"`

	// repairs specify repair tasks in Scylla Manager.
	// When Scylla Manager is not installed, these will be ignored.
	// +optional
	Repairs []RepairTaskSpec `json:"repairs,omitempty"`

	// backups specifies backup tasks in Scylla Manager.
	// When Scylla Manager is not installed, these will be ignored.
	// +optional
	Backups []BackupTaskSpec `json:"backups,omitempty"`

	// forceRedeploymentReason can be used to force a rolling update of all racks by providing a unique string.
	// +optional
	ForceRedeploymentReason string `json:"forceRedeploymentReason,omitempty"`

	// imagePullSecrets is an optional list of references to secrets in the same namespace
	// used for pulling Scylla and Agent images.
	// +optional
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	// dnsDomains is a list of DNS domains this cluster is reachable by.
	// These domains are used when setting up the infrastructure, like certificates.
	// EXPERIMENTAL. Do not rely on any particular behaviour controlled by this field.
	// +optional
	DNSDomains []string `json:"dnsDomains,omitempty"`

	// IPFamily specifies the IP family for this cluster.
	// All services, broadcast addresses, and pod IPs will use this IP family.
	// +kubebuilder:validation:Enum=IPv4;IPv6
	// +kubebuilder:default="IPv4"
	// +optional
	IPFamily *corev1.IPFamily `json:"ipFamily,omitempty"`

	// exposeOptions specifies options for exposing ScyllaCluster services.
	// This field is immutable.
	// EXPERIMENTAL. Do not rely on any particular behaviour controlled by this field.
	// +optional
	ExposeOptions *ExposeOptions `json:"exposeOptions,omitempty"`

	// externalSeeds specifies the external seeds to propagate to ScyllaDB binary on startup as "seeds" parameter of seed-provider.
	ExternalSeeds []string `json:"externalSeeds,omitempty"`

	// minTerminationGracePeriodSeconds specifies minimum duration in seconds to wait before every drained node is
	// terminated. This gives time to potential load balancer in front of a node to notice that node is not ready anymore
	// and stop forwarding new requests.
	// This applies only when node is terminated gracefully.
	// If not provided, Operator will determine this value.
	// EXPERIMENTAL. Do not rely on any particular behaviour controlled by this field.
	// +optional
	MinTerminationGracePeriodSeconds *int32 `json:"minTerminationGracePeriodSeconds,omitempty"`

	// minReadySeconds is the minimum number of seconds for which a newly created ScyllaDB node should be ready
	// for it to be considered available.
	// When used to control load balanced traffic, this can give the load balancer in front of a node enough time to
	// notice that the node is ready and start forwarding traffic in time. Because it all depends on timing, the order
	// is not guaranteed and, if possible, you should use readinessGates instead.
	// If not provided, Operator will determine this value.
	// +optional
	MinReadySeconds *int32 `json:"minReadySeconds,omitempty"`

	// readinessGates specifies custom readiness gates that will be evaluated for every ScyllaDB Pod readiness.
	// It's projected into every ScyllaDB Pod as its readinessGate. Refer to upstream documentation to learn more
	// about readiness gates.
	// +optional
	ReadinessGates []corev1.PodReadinessGate `json:"readinessGates,omitempty"`
}

type PodIPSourceType string

const (
	// StatusPodIPSource specifies that the PodIP is taken from Pod.Status.PodIP
	StatusPodIPSource PodIPSourceType = "Status"
)

type PodIPInterfaceOptions struct {
	// interfaceName specifies interface name within a Pod from which address is taken from.
	InterfaceName string `json:"interfaceName,omitempty"`
}

// PodIPAddressOptions hold options related to Pod IP address.
type PodIPAddressOptions struct {
	// sourceType specifies source of the Pod IP.
	// +kubebuilder:default:="Status"
	Source PodIPSourceType `json:"source"`
}

type BroadcastAddressType string

const (
	// BroadcastAddressTypePodIP selects the IP address from Pod.
	BroadcastAddressTypePodIP BroadcastAddressType = "PodIP"

	// BroadcastAddressTypeServiceClusterIP selects the IP address from Service.spec.ClusterIP
	BroadcastAddressTypeServiceClusterIP BroadcastAddressType = "ServiceClusterIP"

	// BroadcastAddressTypeServiceLoadBalancerIngress selects the IP address or Hostname from Service.status.ingress[0]
	BroadcastAddressTypeServiceLoadBalancerIngress BroadcastAddressType = "ServiceLoadBalancerIngress"
)

// BroadcastOptions hold options related to address broadcasted by ScyllaDB node.
type BroadcastOptions struct {
	// type of the address that is broadcasted.
	Type BroadcastAddressType `json:"type"`

	// podIP holds options related to Pod IP address.
	// +optional
	PodIP *PodIPAddressOptions `json:"podIP,omitempty"`
}

// NodeBroadcastOptions hold options related to addresses broadcasted by ScyllaDB node.
type NodeBroadcastOptions struct {
	// nodes specifies options related to the address that is broadcasted for communication with other nodes.
	// This field controls the `broadcast_address` value in ScyllaDB config.
	// +kubebuilder:default:={type:"ServiceClusterIP"}
	Nodes BroadcastOptions `json:"nodes"`

	// clients specifies options related to the address that is broadcasted for communication with clients.
	// This field controls the `broadcast_rpc_address` value in ScyllaDB config.
	// +kubebuilder:default:={type:"ServiceClusterIP"}
	Clients BroadcastOptions `json:"clients"`
}

type NodeServiceType string

const (
	// NodeServiceTypeHeadless means nodes will be exposed via Headless Service.
	NodeServiceTypeHeadless NodeServiceType = "Headless"

	// NodeServiceTypeClusterIP means nodes will be exposed via ClusterIP Service.
	NodeServiceTypeClusterIP NodeServiceType = "ClusterIP"

	// NodeServiceTypeLoadBalancer means nodes will be exposed via LoadBalancer Service.
	NodeServiceTypeLoadBalancer NodeServiceType = "LoadBalancer"
)

type NodeServiceTemplate struct {
	ObjectTemplateMetadata `json:",inline"`

	// type is the Kubernetes Service type.
	// +kubebuilder:validation:Required
	Type NodeServiceType `json:"type"`

	// externalTrafficPolicy controls value of service.spec.externalTrafficPolicy of each node Service.
	// Check Kubernetes corev1.Service documentation about semantic of this field.
	// +optional
	ExternalTrafficPolicy *corev1.ServiceExternalTrafficPolicy `json:"externalTrafficPolicy,omitempty"`

	// allocateLoadBalancerNodePorts controls value of service.spec.allocateLoadBalancerNodePorts of each node Service.
	// Check Kubernetes corev1.Service documentation about semantic of this field.
	// +optional
	AllocateLoadBalancerNodePorts *bool `json:"allocateLoadBalancerNodePorts,omitempty"`

	// loadBalancerClass controls value of service.spec.loadBalancerClass of each node Service.
	// Check Kubernetes corev1.Service documentation about semantic of this field.
	// +optional
	LoadBalancerClass *string `json:"loadBalancerClass,omitempty"`

	// internalTrafficPolicy controls value of service.spec.internalTrafficPolicy of each node Service.
	// Check Kubernetes corev1.Service documentation about semantic of this field.
	// +optional
	InternalTrafficPolicy *corev1.ServiceInternalTrafficPolicy `json:"internalTrafficPolicy,omitempty"`
}

// ExposeOptions hold options related to exposing ScyllaCluster backends.
// EXPERIMENTAL. Do not rely on any particular behaviour controlled by this field.
type ExposeOptions struct {
	// cql specifies expose options for CQL SSL backend.
	// EXPERIMENTAL. Do not rely on any particular behaviour controlled by this field.
	// +optional
	CQL *CQLExposeOptions `json:"cql,omitempty"`

	// nodeService controls properties of Service dedicated for each ScyllaCluster node.
	// +kubebuilder:default:={type:"ClusterIP"}
	NodeService *NodeServiceTemplate `json:"nodeService,omitempty"`

	// BroadcastOptions defines how ScyllaDB node publishes its IP address to other nodes and clients.
	BroadcastOptions *NodeBroadcastOptions `json:"broadcastOptions,omitempty"`
}

// RackExposeOptions hold options related to exposing rack of ScyllaDBDatacenter.
type RackExposeOptions struct {
	// nodeService controls properties of Service dedicated for each ScyllaDBDatacenter node in given rack.
	// +optional
	NodeService *RackNodeServiceTemplate `json:"nodeService,omitempty"`
}

// RackNodeServiceTemplate hold options related to properties of rack Service.
type RackNodeServiceTemplate struct {
	ObjectTemplateMetadata `json:",inline"`
}

// CQLExposeOptions hold options related to exposing CQL backend.
// EXPERIMENTAL. Do not rely on any particular behaviour controlled by this field.
type CQLExposeOptions struct {
	// ingress is an Ingress configuration options.
	// EXPERIMENTAL. Do not rely on any particular behaviour controlled by this field.
	// +optional
	Ingress *IngressOptions `json:"ingress,omitempty"`
}

// IngressOptions defines configuration options for Ingress objects associated with cluster nodes.
// EXPERIMENTAL. Do not rely on any particular behaviour controlled by this field.
type IngressOptions struct {
	ObjectTemplateMetadata `json:",inline"`

	// disabled controls if Ingress object creation is disabled.
	// Unless disabled, there is an Ingress objects created for every Scylla node.
	// EXPERIMENTAL. Do not rely on any particular behaviour controlled by this field.
	// +optional
	Disabled *bool `json:"disabled,omitempty"`

	// ingressClassName specifies Ingress class name.
	// EXPERIMENTAL. Do not rely on any particular behaviour controlled by this field.
	// +optional
	IngressClassName string `json:"ingressClassName,omitempty"`
}

// GenericUpgradeFailureStrategy allows to specify how upgrade logic should handle failures.
type GenericUpgradeFailureStrategy string

const (
	// GenericUpgradeFailureStrategyRetry infinitely retries until node becomes ready.
	GenericUpgradeFailureStrategyRetry GenericUpgradeFailureStrategy = "Retry"
)

// GenericUpgradeSpec hold generic upgrade procedure parameters.
type GenericUpgradeSpec struct {
	// failureStrategy specifies which logic is executed when upgrade failure happens.
	// Currently only Retry is supported.
	// +kubebuilder:default:="Retry"
	// +optional
	FailureStrategy GenericUpgradeFailureStrategy `json:"failureStrategy,omitempty"`

	// pollInterval specifies how often upgrade logic polls on state updates.
	// Increasing this value should lower number of requests sent to apiserver, but it may affect
	// overall time spent during upgrade.
	// +kubebuilder:default:="1s"
	// +optional
	// DEPRECATED.
	PollInterval metav1.Duration `json:"pollInterval,omitempty"`
}

type SchedulerTaskSpec struct {
	// startDate specifies the task start date expressed in the RFC3339 format or now[+duration],
	// e.g. now+3d2h10m, valid units are d, h, m, s.
	// +optional
	StartDate *string `json:"startDate,omitempty"`

	// interval represents a task schedule interval e.g. 3d2h10m, valid units are d, h, m, s.
	// Deprecated: please use cron instead.
	// +optional
	Interval *string `json:"interval,omitempty"`

	// numRetries indicates how many times a scheduled task will be retried before failing.
	// +kubebuilder:default:=3
	// +optional
	NumRetries *int64 `json:"numRetries,omitempty"`

	// retryWait specifies the initial exponential backoff duration for task retries.
	// For instance, if set to 10 minutes, the first retry will be attempted after 10 minutes, the second after 20 minutes, the third after 40 minutes, and so on, up to the number of retries specified in `numRetries`.
	// If not set, the default values is left to ScyllaDB Manager to decide.
	// +optional
	RetryWait *metav1.Duration `json:"retryWait,omitempty"`

	// cron specifies the task schedule as a cron expression. It supports an extended syntax including @monthly, @weekly, @daily, @midnight, @hourly, @every X[h|m|s].
	// +optional
	Cron *string `json:"cron,omitempty"`

	// timezone specifies the timezone of cron field.
	// +optional
	Timezone *string `json:"timezone,omitempty"`
}

type TaskSpec struct {
	// name specifies the name of a task.
	Name string `json:"name"`

	SchedulerTaskSpec `json:",inline"`
}

type RepairTaskSpec struct {
	TaskSpec `json:",inline"`

	// dc is a list of datacenter glob patterns, e.g. 'dc1', '!otherdc*' used to specify the DCs
	// to include or exclude from backup.
	DC []string `json:"dc,omitempty"`

	// failFast indicates if a repair should be stopped on first error.
	// +optional
	FailFast bool `json:"failFast,omitempty"`

	// intensity indicates how many token ranges (per shard) to repair in a single Scylla repair job. By default this is 1.
	// If you set it to 0 the number of token ranges is adjusted to the maximum supported by node (see max_repair_ranges_in_parallel in Scylla logs).
	// Valid values are 0 and integers >= 1. Higher values will result in increased cluster load and slightly faster repairs.
	// Changing the intensity impacts repair granularity if you need to resume it, the higher the value the more work on resume.
	// For Scylla clusters that *do not support row-level repair*, intensity can be a decimal between (0,1).
	// In that case it specifies percent of shards that can be repaired in parallel on a repair master node.
	// For Scylla clusters that are row-level repair enabled, setting intensity below 1 has the same effect as setting intensity 1.
	// +kubebuilder:default:="1"
	// +optional
	Intensity string `json:"intensity,omitempty"`

	// parallel is the maximum number of Scylla repair jobs that can run at the same time (on different token ranges and replicas).
	// Each node can take part in at most one repair at any given moment. By default the maximum possible parallelism is used.
	// The effective parallelism depends on a keyspace replication factor (RF) and the number of nodes.
	// The formula to calculate it is as follows: number of nodes / RF, ex. for 6 node cluster with RF=3 the maximum parallelism is 2.
	// +kubebuilder:default:=0
	// +optional
	Parallel int64 `json:"parallel,omitempty"`

	// keyspace is a list of keyspace/tables glob patterns, e.g. 'keyspace,!keyspace.table_prefix_*'
	// used to include or exclude keyspaces from repair.
	Keyspace []string `json:"keyspace,omitempty"`

	// smallTableThreshold enable small table optimization for tables of size lower than given threshold.
	// Supported units [B, MiB, GiB, TiB].
	// +kubebuilder:default:="1GiB"
	// +optional
	SmallTableThreshold string `json:"smallTableThreshold,omitempty"`

	// host specifies a host to repair. If empty, all hosts are repaired.
	Host *string `json:"host,omitempty"`

	// ignoreDownHosts indicates that the nodes in down state should be ignored during repair.
	// +optional
	IgnoreDownHosts *bool `json:"ignoreDownHosts,omitempty"`
}

type BackupTaskSpec struct {
	TaskSpec `json:",inline"`

	// dc is a list of datacenter glob patterns, e.g. 'dc1,!otherdc*' used to specify the DCs
	// to include or exclude from backup.
	// +optional
	DC []string `json:"dc,omitempty"`

	// keyspace is a list of keyspace/tables glob patterns,
	// e.g. 'keyspace,!keyspace.table_prefix_*' used to include or exclude keyspaces from repair.
	// +optional
	Keyspace []string `json:"keyspace,omitempty"`

	// location is a list of backup locations in the format [<dc>:]<provider>:<name> ex. s3:my-bucket.
	// The <dc>: part is optional and is only needed when different datacenters are being used to upload data
	// to different locations. <name> must be an alphanumeric string and may contain a dash and or a dot,
	// but other characters are forbidden.
	// The only supported storage <provider> at the moment are s3 and gcs.
	Location []string `json:"location"`

	// rateLimit is a list of megabytes (MiB) per second rate limits expressed in the format [<dc>:]<limit>.
	// The <dc>: part is optional and only needed when different datacenters need different upload limits.
	// Set to 0 for no limit (default 100).
	// +optional
	RateLimit []string `json:"rateLimit,omitempty"`

	// retention is the number of backups which are to be stored.
	// +kubebuilder:default:=3
	// +optional
	Retention int64 `json:"retention,omitempty"`

	// snapshotParallel is a list of snapshot parallelism limits in the format [<dc>:]<limit>.
	// The <dc>: part is optional and allows for specifying different limits in selected datacenters.
	// If The <dc>: part is not set, the limit is global (e.g. 'dc1:2,5') the runs are parallel in n nodes (2 in dc1)
	// and n nodes in all the other datacenters.
	// +optional
	SnapshotParallel []string `json:"snapshotParallel,omitempty"`

	// uploadParallel is a list of upload parallelism limits in the format [<dc>:]<limit>.
	// The <dc>: part is optional and allows for specifying different limits in selected datacenters.
	// If The <dc>: part is not set the limit is global (e.g. 'dc1:2,5') the runs are parallel in n nodes (2 in dc1)
	// and n nodes in all the other datacenters.
	// +optional
	UploadParallel []string `json:"uploadParallel,omitempty"`
}

type Network struct {
	// hostNetworking determines if scylla uses the host's network namespace. Setting this option
	// avoids going through Kubernetes SDN and exposes scylla on node's IP.
	// Deprecated: `hostNetworking` is deprecated and may be ignored in the future.
	HostNetworking bool `json:"hostNetworking,omitempty"`

	// dnsPolicy defines how a pod's DNS will be configured.
	DNSPolicy corev1.DNSPolicy `json:"dnsPolicy,omitempty"`

	// ipFamilyPolicy specifies the IP family policy for the cluster.
	// Supports: SingleStack, PreferDualStack, RequireDualStack.
	// +kubebuilder:default:="SingleStack"
	// +optional
	IPFamilyPolicy *corev1.IPFamilyPolicy `json:"ipFamilyPolicy,omitempty"`

	// ipFamilies specifies the IP families to use.
	// Supports: IPv4, IPv6.
	// +optional
	IPFamilies []corev1.IPFamily `json:"ipFamilies,omitempty"`
}

func (s Network) GetDNSPolicy() corev1.DNSPolicy {
	if len(s.DNSPolicy) != 0 {
		return s.DNSPolicy
	}

	return corev1.DNSClusterFirstWithHostNet
}

// DatacenterSpec is the desired state for a Scylla Datacenter.
type DatacenterSpec struct {
	// name is the name of the scylla datacenter. Used in the cassandra-rackdc.properties file.
	Name string `json:"name"`

	// racks specify the racks in the datacenter.
	Racks []RackSpec `json:"racks"`
}

// RackSpec is the desired state for a Scylla Rack.
type RackSpec struct {
	// name is the name of the Scylla Rack. Used in the cassandra-rackdc.properties file.
	Name string `json:"name"`

	// members is the number of Scylla instances in this rack.
	Members int32 `json:"members"`

	// storage describes the underlying storage that Scylla will consume.
	Storage Storage `json:"storage"`

	// placement describes restrictions for the nodes Scylla is scheduled on.
	// +optional
	Placement *PlacementSpec `json:"placement,omitempty"`

	// resources the Scylla container will use.
	Resources corev1.ResourceRequirements `json:"resources"`

	// agentResources specify the resources for the Agent container.
	// +kubebuilder:default:={requests: {cpu: "50m", memory: "10M"}}
	// +optional
	AgentResources corev1.ResourceRequirements `json:"agentResources,omitempty"`

	// Volumes added to Scylla Pod.
	// +optional
	Volumes []corev1.Volume `json:"volumes,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`

	// VolumeMounts to be added to Scylla container.
	// +optional
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty" patchStrategy:"merge" patchMergeKey:"mountPath"`

	// AgentVolumeMounts to be added to Agent container.
	AgentVolumeMounts []corev1.VolumeMount `json:"agentVolumeMounts,omitempty" patchStrategy:"merge" patchMergeKey:"mountPath"`

	// Scylla config map name to customize scylla.yaml
	ScyllaConfig string `json:"scyllaConfig"`

	// Scylla config map name to customize scylla manager agent
	ScyllaAgentConfig string `json:"scyllaAgentConfig"`

	// exposeOptions specifies rack-specific parameters related to exposing ScyllaDBDatacenter backends.
	// +optional
	ExposeOptions *RackExposeOptions `json:"exposeOptions,omitempty"`
}

type PlacementSpec struct {
	// nodeAffinity describes node affinity scheduling rules for the pod.
	// +optional
	NodeAffinity *corev1.NodeAffinity `json:"nodeAffinity,omitempty"`

	// podAffinity describes pod affinity scheduling rules.
	// +optional
	PodAffinity *corev1.PodAffinity `json:"podAffinity,omitempty"`

	// podAntiAffinity describes pod anti-affinity scheduling rules.
	// +optional
	PodAntiAffinity *corev1.PodAntiAffinity `json:"podAntiAffinity,omitempty"`

	// tolerations allow the pod to tolerate any taint that matches the triple <key,value,effect>
	// using the matching operator.
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
}

type Storage struct {
	// metadata controls shared metadata for the volume claim for this rack.
	// At this point, the values are applied only for the initial claim and are not reconciled during its lifetime.
	// Note that this may get fixed in the future and this behaviour shouldn't be relied on in any way.
	// +optional
	Metadata *ObjectTemplateMetadata `json:"metadata,omitempty"`

	// capacity describes the requested size of each persistent volume.
	Capacity string `json:"capacity"`

	// storageClassName is the name of a storageClass to request.
	// +optional
	StorageClassName *string `json:"storageClassName,omitempty"`
}

type TLSCertificateType string

const (
	TLSCertificateTypeOperatorManaged TLSCertificateType = "OperatorManaged"
	TLSCertificateTypeUserManaged     TLSCertificateType = "UserManaged"
)

type UserManagedTLSCertificateOptions struct {
	// secretName references a kubernetes.io/tls type secret containing the TLS cert and key.
	SecretName string `json:"secretName"`
}

type OperatorManagedTLSCertificateOptions struct {
	// additionalDNSNames represents external DNS names that the certificates should be signed for.
	// +optional
	AdditionalDNSNames []string `json:"additionalDNSNames,omitempty"`

	// additionalIPAddresses represents external IP addresses that the certificates should be signed for.
	// +optional
	AdditionalIPAddresses []string `json:"additionalIPAddresses,omitempty"`
}

type TLSCertificate struct {
	// type determines the source of this certificate.
	// +kubebuilder:validation:Enum="OperatorManaged";"UserManaged"
	Type TLSCertificateType `json:"type"`

	// userManagedOptions specifies options for certificates manged by users.
	// +optional
	UserManagedOptions *UserManagedTLSCertificateOptions `json:"userManagedOptions,omitempty"`

	// operatorManagedOptions specifies options for certificates manged by the operator.
	// +optional
	OperatorManagedOptions *OperatorManagedTLSCertificateOptions `json:"operatorManagedOptions,omitempty"`
}

type AlternatorSpec struct {
	// port is the port number used to bind the Alternator API.
	// Deprecated: `port` is deprecated and may be ignored in the future.
	// Please make sure to avoid using hostNetworking and work with standard Kubernetes concepts like Services.
	Port int32 `json:"port,omitempty"`

	// writeIsolation indicates the isolation level.
	WriteIsolation string `json:"writeIsolation,omitempty"`

	// insecureEnableHTTP enables serving Alternator traffic also on insecure HTTP port.
	// +optional
	InsecureEnableHTTP *bool `json:"insecureEnableHTTP,omitempty"`

	// insecureDisableAuthorization disables Alternator authorization.
	// If not specified, the authorization is enabled.
	// For backwards compatibility the authorization is disabled when this field is not specified
	// and a manual port is used.
	// +optional
	InsecureDisableAuthorization *bool `json:"insecureDisableAuthorization,omitempty"`

	// servingCertificate references a TLS certificate for serving secure traffic.
	// +kubebuilder:default:={type:"OperatorManaged"}
	// +optional
	ServingCertificate *TLSCertificate `json:"servingCertificate,omitempty"`
}

type SchedulerTaskStatus struct {
	// startDate reflects the task start date expressed in the RFC3339 format
	// +optional
	StartDate *string `json:"startDate,omitempty"`

	// interval reflects a task schedule interval.
	// +optional
	Interval *string `json:"interval,omitempty"`

	// numRetries reflects how many times a scheduled task will be retried before failing.
	// +optional
	NumRetries *int64 `json:"numRetries,omitempty"`

	// retryWait reflects the initial exponential backoff duration for task retries.
	// +optional
	RetryWait *metav1.Duration `json:"retryWait,omitempty"`

	// cron reflects the task schedule as a cron expression.
	// +optional
	Cron *string `json:"cron,omitempty"`

	// timezone reflects the timezone of cron field.
	// +optional
	Timezone *string `json:"timezone,omitempty"`
}

type TaskStatus struct {
	SchedulerTaskStatus `json:",inline"`

	// name reflects the name of a task.
	Name string `json:"name,omitempty"`

	// id reflects identification number of the repair task.
	// +optional
	ID *string `json:"id,omitempty"`

	// labels reflects the labels of a task.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// error holds the task error, if any.
	// +optional
	Error *string `json:"error,omitempty"`
}

type RepairTaskStatus struct {
	TaskStatus `json:",inline"`

	// dc reflects a list of datacenter glob patterns, e.g. 'dc1', '!otherdc*' used to specify the DCs
	// to include or exclude from repair.
	// +optional
	DC []string `json:"dc,omitempty" mapstructure:"dc,omitempty"`

	// failFast indicates if a repair should be stopped on first error.
	// +optional
	FailFast *bool `json:"failFast,omitempty" mapstructure:"fail_fast,omitempty"`

	// intensity indicates how many token ranges (per shard) to repair in a single Scylla repair job. By default this is 1.
	// +optional
	Intensity *string `json:"intensity,omitempty" mapstructure:"intensity,omitempty"`

	// parallel reflects the maximum number of Scylla repair jobs that can run at the same time (on different token ranges and replicas).
	// +optional
	Parallel *int64 `json:"parallel,omitempty" mapstructure:"parallel,omitempty"`

	// keyspace reflects a list of keyspace/tables glob patterns, e.g. 'keyspace,!keyspace.table_prefix_*'
	// used to include or exclude keyspaces from repair.
	Keyspace []string `json:"keyspace,omitempty" mapstructure:"keyspace,omitempty"`

	// smallTableThreshold reflects whether small table optimization for tables, of size lower than given threshold, are enabled.
	// +optional
	SmallTableThreshold *string `json:"smallTableThreshold,omitempty" mapstructure:"small_table_threshold,omitempty"`

	// host reflects a host to repair.
	Host *string `json:"host,omitempty" mapstructure:"host,omitempty"`

	// ignoreDownHosts reflects whether the nodes in down state are ignored during repair.
	// +optional
	IgnoreDownHosts *bool `json:"ignoreDownHosts,omitempty" mapstructure:"ignore_down_hosts,omitempty"`
}

type BackupTaskStatus struct {
	TaskStatus `json:",inline"`

	// dc reflects a list of datacenter glob patterns, e.g. 'dc1,!otherdc*' used to specify the DCs
	// to include or exclude from backup.
	// +optional
	DC []string `json:"dc,omitempty" mapstructure:"dc,omitempty"`

	// keyspace reflects a list of keyspace/tables glob patterns,
	// e.g. 'keyspace,!keyspace.table_prefix_*' used to include or exclude keyspaces from repair.
	// +optional
	Keyspace []string `json:"keyspace,omitempty" mapstructure:"keyspace,omitempty"`

	// location reflects a list of backup locations in the format [<dc>:]<provider>:<name> ex. s3:my-bucket.
	Location []string `json:"location" mapstructure:"location,omitempty"`

	// rateLimit reflects a list of megabytes (MiB) per second rate limits expressed in the format [<dc>:]<limit>.
	// +optional
	RateLimit []string `json:"rateLimit,omitempty" mapstructure:"rate_limit,omitempty"`

	// retention reflects the number of backups which are to be stored.
	// +optional
	Retention *int64 `json:"retention,omitempty" mapstructure:"retention,omitempty"`

	// snapshotParallel reflects a list of snapshot parallelism limits in the format [<dc>:]<limit>.
	// +optional
	SnapshotParallel []string `json:"snapshotParallel,omitempty" mapstructure:"snapshot_parallel,omitempty"`

	// uploadParallel reflects a list of upload parallelism limits in the format [<dc>:]<limit>.
	// +optional
	UploadParallel []string `json:"uploadParallel,omitempty" mapstructure:"upload_parallel,omitempty"`
}

// ScyllaClusterStatus defines the observed state of ScyllaCluster.
type ScyllaClusterStatus struct {
	// observedGeneration is the most recent generation observed for this ScyllaCluster. It corresponds to the
	// ScyllaCluster's generation, which is updated on mutation by the API Server.
	// +optional
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// racks reflect status of cluster racks.
	Racks map[string]RackStatus `json:"racks,omitempty"`

	// members is the number of ScyllaDB members in all racks.
	// +optional
	Members *int32 `json:"members,omitempty"`

	// readyMembers is the number of ScyllaDB members in all racks that are ready.
	// +optional
	ReadyMembers *int32 `json:"readyMembers,omitempty"`

	// availableMembers is the number of ScyllaDB members in all racks that are available.
	// +optional
	AvailableMembers *int32 `json:"availableMembers,omitempty"`

	// rackCount is the number of ScyllaDB racks in this cluster.
	// +optional
	RackCount *int32 `json:"rackCount,omitempty"`

	// managerId contains ID under which cluster was registered in Scylla Manager.
	ManagerID *string `json:"managerId,omitempty"`

	// repairs reflects status of repair tasks.
	Repairs []RepairTaskStatus `json:"repairs,omitempty"`

	// backups reflects status of backup tasks.
	Backups []BackupTaskStatus `json:"backups,omitempty"`

	// upgrade reflects state of ongoing upgrade procedure.
	Upgrade *UpgradeStatus `json:"upgrade,omitempty"`

	// conditions hold conditions describing ScyllaCluster state.
	// To determine whether a cluster rollout is finished, look for Available=True,Progressing=False,Degraded=False.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

const (
	AvailableCondition   = "Available"
	ProgressingCondition = "Progressing"
	DegradedCondition    = "Degraded"
)

// UpgradeStatus contains the internal state of an ongoing upgrade procedure.
// Do not rely on these internal values externally. They are meant for keeping an internal state
// and their values are subject to change within the limits of API compatibility.
type UpgradeStatus struct {
	// state reflects current upgrade state.
	State string `json:"state"`

	// currentNode node under upgrade.
	// DEPRECATED.
	CurrentNode string `json:"currentNode,omitempty"`

	// currentRack rack under upgrade.
	// DEPRECATED.
	CurrentRack string `json:"currentRack,omitempty"`

	// fromVersion reflects from which version ScyllaCluster is being upgraded.
	FromVersion string `json:"fromVersion"`

	// toVersion reflects to which version ScyllaCluster is being upgraded.
	ToVersion string `json:"toVersion"`

	// systemSnapshotTag is the snapshot tag of system keyspaces.
	SystemSnapshotTag string `json:"systemSnapshotTag,omitempty"`

	// dataSnapshotTag is the snapshot tag of data keyspaces.
	DataSnapshotTag string `json:"dataSnapshotTag,omitempty"`
}

// RackStatus is the status of a Scylla Rack
type RackStatus struct {
	// version is the current version of Scylla in use.
	Version string `json:"version"`

	// members is the current number of members requested in the specific Rack
	Members int32 `json:"members"`

	// readyMembers is the number of ready members in the specific Rack
	ReadyMembers int32 `json:"readyMembers"`

	// availableMembers is the number of available members in the Rack.
	// +optional
	AvailableMembers *int32 `json:"availableMembers,omitempty"`

	// updatedMembers is the number of members matching the current spec.
	// +optional
	UpdatedMembers *int32 `json:"updatedMembers,omitempty"`

	// stale indicates if the current rack status is collected for a previous generation.
	// stale should eventually become false when the appropriate controller writes a fresh status.
	// +optional
	Stale *bool `json:"stale,omitempty"`

	// conditions are the latest available observations of a rack's state.
	Conditions []RackCondition `json:"conditions,omitempty"`

	// replace_address_first_boot holds addresses which should be replaced by new nodes.
	// DEPRECATED: since Scylla Operator 1.10 it's only used for deprecated replace node procedure (ScyllaDB OS <5.2, Enterprise <2023.1).
	//             With Scylla Operator 1.11+ this field may be empty.
	ReplaceAddressFirstBoot map[string]string `json:"replace_address_first_boot,omitempty"`
}

// RackCondition is an observation about the state of a rack.
type RackCondition struct {
	// type holds the condition type.
	Type RackConditionType `json:"type"`

	// status represent condition status.
	Status corev1.ConditionStatus `json:"status"`
}

type RackConditionType string

const (
	RackConditionTypeMemberLeaving         RackConditionType = "MemberLeaving"
	RackConditionTypeUpgrading             RackConditionType = "RackUpgrading"
	RackConditionTypeMemberReplacing       RackConditionType = "MemberReplacing"
	RackConditionTypeMemberDecommissioning RackConditionType = "MemberDecommissioning"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:printcolumn:name="READY",type=integer,JSONPath=".status.readyMembers"
// +kubebuilder:printcolumn:name="MEMBERS",type=integer,JSONPath=".status.members"
// +kubebuilder:printcolumn:name="RACKS",type=integer,JSONPath=".status.rackCount"
// +kubebuilder:printcolumn:name="AVAILABLE",type=string,JSONPath=".status.conditions[?(@.type=='Available')].status"
// +kubebuilder:printcolumn:name="PROGRESSING",type=string,JSONPath=".status.conditions[?(@.type=='Progressing')].status"
// +kubebuilder:printcolumn:name="DEGRADED",type=string,JSONPath=".status.conditions[?(@.type=='Degraded')].status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// ScyllaCluster defines a Scylla cluster.
type ScyllaCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec defines the desired state of this scylla cluster.
	Spec ScyllaClusterSpec `json:"spec,omitempty"`

	// status is the current status of this scylla cluster.
	Status ScyllaClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ScyllaClusterList holds a list of ScyllaClusters.
type ScyllaClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ScyllaCluster `json:"items"`
}
