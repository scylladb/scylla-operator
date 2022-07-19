package v1alpha1

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type SchedulerTaskSpec struct {
	// name is a unique name of a task.
	Name string `json:"name"`

	// cron describes the scheduler when a given task shall be run.
	// It supports the extended syntax including @monthly, @weekly, @daily, @midnight, @hourly, @every <time.Duration>.
	// +optional
	Cron string `json:"cron"`

	// numRetries indicates how many times a scheduled task will be retried before failing.
	// +kubebuilder:default:=3
	// +optional
	NumRetries *int64 `json:"numRetries,omitempty"`

	// TimeWindow indicates when the task can be run.
	// +optional
	TimeWindow string `json:"timeWindow,omitempty"`

	// The time zone in which the task will be run.
	// +optional
	Timezone string `json:"timezone,omitempty"`
}

type RepairTaskSpec struct {
	SchedulerTaskSpec `json:",inline"`

	// Disable flag allows to disable task manually.
	// +optional
	Disable bool `json:"disable,omitempty"`

	// DC is a list of datacenter glob patterns, e.g. 'dc1', '!otherdc*' used to specify the DCs
	// to include or exclude from backup.
	DC []string `json:"dc,omitempty" mapstructure:"dc,omitempty"`

	// FailFast indicates if a repair should be stopped on first error.
	// +optional
	FailFast bool `json:"failFast,omitempty" mapstructure:"fail_fast,omitempty"`

	// Intensity indicates how many token ranges (per shard) to repair in a single Scylla repair job. By default this is 1.
	// If you set it to 0 the number of token ranges is adjusted to the maximum supported by node (see max_repair_ranges_in_parallel in Scylla logs).
	// Valid values are 0 and integers >= 1. Higher values will result in increased cluster load and slightly faster repairs.
	// Changing the intensity impacts repair granularity if you need to resume it, the higher the value the more work on resume.
	// For Scylla clusters that *do not support row-level repair*, intensity can be a decimal between (0,1).
	// In that case it specifies percent of shards that can be repaired in parallel on a repair master node.
	// For Scylla clusters that are row-level repair enabled, setting intensity below 1 has the same effect as setting intensity 1.
	// +kubebuilder:default:="1"
	// +optional
	Intensity string `json:"intensity,omitempty" mapstructure:"intensity,omitempty"`

	// Parallel is the maximum number of Scylla repair jobs that can run at the same time (on different token ranges and replicas).
	// Each node can take part in at most one repair at any given moment. By default the maximum possible parallelism is used.
	// The effective parallelism depends on a keyspace replication factor (RF) and the number of nodes.
	// The formula to calculate it is as follows: number of nodes / RF, ex. for 6 node cluster with RF=3 the maximum parallelism is 2.
	// +kubebuilder:default:=0
	// +optional
	Parallel int64 `json:"parallel,omitempty"`

	// Keyspace is a list of keyspace/tables glob patterns, e.g. 'keyspace,!keyspace.table_prefix_*'
	// used to include or exclude keyspaces from repair.
	Keyspace []string `json:"keyspace,omitempty"`

	// SmallTableThreshold enables small table optimization for tables of size lower than given threshold.
	// Supported units [B, MiB, GiB, TiB].
	// +kubebuilder:default:="1GiB"
	// +optional
	SmallTableThreshold string `json:"smallTableThreshold,omitempty" mapstructure:"small_table_threshold,omitempty"`

	// Host specifies a host to repair. If empty, all hosts are repaired.
	Host *string `json:"host,omitempty" mapstructure:"host,omitempty"`
}

type BackupTaskSpec struct {
	SchedulerTaskSpec `json:",inline"`

	// Disable flag allows to disable task manually.
	// +optional
	Disable bool `json:"disable,omitempty"`

	// DC is a list of datacenter glob patterns, e.g. 'dc1,!otherdc*' used to specify the DCs
	// to include or exclude from backup.
	// +optional
	DC []string `json:"dc,omitempty" mapstructure:"dc,omitempty"`

	// Keyspace is a list of keyspace/tables glob patterns,
	// e.g. 'keyspace,!keyspace.table_prefix_*' used to include or exclude keyspaces from repair.
	// +optional
	Keyspace []string `json:"keyspace,omitempty" mapstructure:"keyspace,omitempty"`

	// Location is a list of backup locations in the format [<dc>:]<provider>:<name> ex. s3:my-bucket.
	// The <dc>: part is optional and is only needed when different datacenters are being used to upload data
	// to different locations. <name> must be an alphanumeric string and may contain a dash and or a dot,
	// but other characters are forbidden.
	// The only supported storage <provider> at the moment are s3 and gcs.
	Location []string `json:"location" mapstructure:"location,omitempty"`

	// RateLimit is a list of megabytes (MiB) per second rate limits expressed in the format [<dc>:]<limit>.
	// The <dc>: part is optional and only needed when different datacenters need different upload limits.
	// Set to 0 for no limit (default 100).
	// +optional
	RateLimit []string `json:"rateLimit,omitempty" mapstructure:"rate_limit,omitempty"`

	// Retention is the number of backups which are to be stored.
	// +kubebuilder:default:=3
	// +optional
	Retention int64 `json:"retention,omitempty" mapstructure:"retention,omitempty"`

	// SnapshotParallel is a list of snapshot parallelism limits in the format [<dc>:]<limit>.
	// The <dc>: part is optional and allows for specifying different limits in selected datacenters.
	// If The <dc>: part is not set, the limit is global (e.g. 'dc1:2,5') the runs are parallel in n nodes (2 in dc1)
	// and n nodes in all the other datacenters.
	// +optional
	SnapshotParallel []string `json:"snapshotParallel,omitempty" mapstructure:"snapshot_parallel,omitempty"`

	// UploadParallel is a list of upload parallelism limits in the format [<dc>:]<limit>.
	// The <dc>: part is optional and allows for specifying different limits in selected datacenters.
	// If The <dc>: part is not set the limit is global (e.g. 'dc1:2,5') the runs are parallel in n nodes (2 in dc1)
	// and n nodes in all the other datacenters.
	// +optional
	UploadParallel []string `json:"uploadParallel,omitempty" mapstructure:"upload_parallel,omitempty"`
}

type KeyspaceOptions struct {

	// +optional
	KeyspacePrefix string `json:"keyspacePrefix"`

	// ReplicationFactor of a keyspace.
	// +optional
	ReplicationFactor *int32 `json:"replicationFactor"`
}

type ClientOptions struct {
	LocalDatacenter string `json:"localDatacenter"`
}

type TLSConfig struct {
	CertificateAuthorityConfigMapRef corev1.LocalObjectReference `json:"certificateAuthorityConfigMapRef"`

	ClientCertSecretRef corev1.LocalObjectReference `json:"clientCertSecretRef"`

	// +optional
	InsecureSkipTLSVerify *bool `json:"insecureSkipTlsVerify,omitempty"`
}

type ConnectionSpec struct {
	// Address of a database server.
	Server string `json:"server"`

	// +optional
	TLSServerName string `json:"tlsServerName,omitempty"`

	// Configuration of a TLS.
	// +optional
	TLSConfig *TLSConfig `json:"tlsConfig,omitempty"`

	// Username is the username for basic authentication to the Scylla cluster.
	// +optional
	Username string `json:"username,omitempty"`

	// Password is the password for basic authentication to the Scylla cluster.
	// +optional
	Password string `json:"password,omitempty"`
}

type DatabaseSpec struct {
	// +optional
	KeyspaceOptions *KeyspaceOptions `json:"keyspaceOptions"`
	// +optional
	ClientOption *ClientOptions `json:"clientOptions"`

	Connection *ConnectionSpec `json:"connection,omitempty"`
}

type ServerOptions struct {
	// +optional
	ClientCAConfigMapRef *corev1.LocalObjectReference `json:"clientCaConfigMapRef"`
	// +optional
	ServingSecretRef *corev1.LocalObjectReference `json:"servingSecretRef"`
}

// ScyllaManagerSpec defines the desired state of Scylla Manager.
type ScyllaManagerSpec struct {
	// Image is an image of Scylla Manager to use.
	Image string `json:"image"`

	// Replicas is a total number of desired Scylla Managers.
	Replicas int32 `json:"replicas"`

	// Resources per one Scylla Manager deployment.
	Resources corev1.ResourceRequirements `json:"resource"`

	// ImagePullSecrets is an optional list of references to secrets in the same namespace
	// used for pulling Scylla Manager and Agent images.
	// +optional
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	// ScyllaClusterSelector targets clusters which will be watched by this Scylla Manager.
	ScyllaClusterSelector *metav1.LabelSelector `json:"scyllaClusterSelector"`

	// The internal Database of this Scylla Manager.
	Database      *DatabaseSpec `json:"database"`
	ServerOptions ServerOptions `json:"serverOptions"`

	// Repair tasks to be executed by manager.
	Repairs []RepairTaskSpec `json:"repairs"`

	// Backup tasks to be executed by manager.
	Backups []BackupTaskSpec `json:"backups"`

	// TaskStatusRefreshTime indicates how often the status of a task will be updated during run.
	// +kubebuilder:default:=15000000000
	// +optional
	TaskStatusRefreshTime time.Duration `json:"taskStatusRefreshTime"`
}

type TaskStatusType string

const (
	TaskStatusNew      TaskStatusType = "NEW"
	TaskStatusRunning  TaskStatusType = "RUNNING"
	TaskStatusStopping TaskStatusType = "STOPPING"
	TaskStatusStopped  TaskStatusType = "STOPPED"
	TaskStatusWaiting  TaskStatusType = "WAITING"
	TaskStatusDone     TaskStatusType = "DONE"
	TaskStatusError    TaskStatusType = "ERROR"
	TaskStatusAborted  TaskStatusType = "ABORTED"
)

type TaskStatus struct {
	// ID of a task in ScyllaManager
	// +optional
	ID string `json:"id,omitempty"`

	// Name of a task in ScyllaManager
	Name string `json:"name"`

	// Status of a task from ScyllaManager.
	Status TaskStatusType `json:"status,omitempty"`

	// +optional
	Reason string `json:"reason,omitempty"`
}

const (
	ClusterRegistered = "Registered"
)

type ManagedCluster struct {
	// ID is assigned to cluster by manager.
	// +optional
	ID string `json:"id,omitempty"`

	// Name of this Scylla Cluster.
	Name string `json:"name"`

	// Conditions of a ManagedCluster.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Statuses of all the repairs in this cluster.
	Repairs []*TaskStatus `json:"repairs"`

	// Statuses of all the backups in this cluster.
	Backups []*TaskStatus `json:"backups"`
}

const (
	// ScyllaManagerAvailable is True when at least one replica of Scylla Manager is ready.
	ScyllaManagerAvailable = "Available"

	// ScyllaManagerDegraded is True if the total number of UpdatedReplicas
	// does not equal the total number of Replicas.
	ScyllaManagerDegraded = "Degraded"
)

type ScyllaManagerStatus struct {
	// observedGeneration is the most recent generation observed for this ScyllaManager. It corresponds to the
	// ScyllaManager's generation, which is updated on mutation by the API Server.
	// +optional
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// Statuses of a clusters, that match this manager's selector.
	ManagedClusters []*ManagedCluster `json:"managedClusters"`

	// Conditions of a ScyllaManager.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Total number of desired Scylla Managers.
	Replicas int32 `json:"replicas"`

	// Total number of Scylla Managers that have the desired template spec.
	UpdatedReplicas int32 `json:"updatedReplicas"`

	// Total number of ready Scylla Managers.
	ReadyReplicas int32 `json:"readyReplicas"`

	// Total number of Scylla Managers that are not ready.
	UnavailableReplicas int32 `json:"unavailableReplicas"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:resource:path=scyllamanagers,scope=Namespaced
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ScyllaManager struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec defines the desired state of the operator.
	Spec ScyllaManagerSpec `json:"spec,omitempty"`

	// status defines the observed state of the operator.
	Status ScyllaManagerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ScyllaManagerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ScyllaManager `json:"items"`
}
