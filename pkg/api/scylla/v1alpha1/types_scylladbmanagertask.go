// Copyright (C) 2025 ScyllaDB

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ScyllaDBManagerTaskType string

const (
	ScyllaDBManagerTaskTypeBackup ScyllaDBManagerTaskType = "Backup"
	ScyllaDBManagerTaskTypeRepair ScyllaDBManagerTaskType = "Repair"
)

type ScyllaDBManagerTaskSchedule struct {
	// cron specifies the task schedule as a cron expression.
	// It supports the "standard" cron syntax `MIN HOUR DOM MON DOW`, as used by the Linux utility, as well as a set of non-standard macros: "@yearly", "@annually", "@monthly", "@weekly", "@daily", "@midnight", "@hourly", "@every [+-]?<duration>".
	// +optional
	Cron *string `json:"cron,omitempty"`

	// numRetries specifies how many times a scheduled task should be retried before failing.
	// +optional
	NumRetries *int64 `json:"numRetries,omitempty"`

	// startDate specifies the start date of the task.
	// It is represented in RFC3339 form and is in UTC.
	// If not set, the task is started immediately.
	// +optional
	StartDate *metav1.Time `json:"startDate,omitempty"`
}

type ScyllaDBManagerBackupTaskOptions struct {
	// schedule specifies the schedule on which the backup task is run.
	ScyllaDBManagerTaskSchedule `json:",inline"`

	// dc specifies a list of datacenter `glob` patterns, e.g. `dc1`, `!otherdc*`, determining the datacenters to include or exclude from backup.
	// +optional
	DC []string `json:"dc,omitempty"`

	// keyspace specifies a list of `glob` patterns used to include or exclude tables from backup.
	// The patterns match keyspaces and tables. Keyspace names are separated from table names with a dot e.g. `!keyspace.table_prefix_*`.
	// +optional
	Keyspace []string `json:"keyspace,omitempty"`

	// location specifies a list of backup locations in the following format: `[<dc>:]<provider>:<name>`.
	// `<dc>:` is optional and allows to specify the location for a datacenter in a multi-datacenter cluster.
	// `<provider>` specifies the storage provider.
	// `<name>` specifies a bucket name and must be an alphanumeric string which may contain a dash and or a dot, but other characters are forbidden.
	Location []string `json:"location"`

	// rateLimit specifies the limit for the upload rate, expressed in mebibytes (MiB) per second, at which the snapshot files can be uploaded from a ScyllaDB node to its backup destination, in the following format: `[<dc>:]<limit>`.
	// `<dc>:` is optional and allows for specifying different upload limits in selected datacenters.
	// +optional
	RateLimit []string `json:"rateLimit,omitempty"`

	// retention specifies the number of backups to store.
	// +optional
	Retention *int64 `json:"retention,omitempty"`

	// snapshotParallel specifies a list of snapshot parallelism limits in the following format:  `[<dc>:]<limit>`.
	// `<dc>:` is optional and allows for specifying different limits in selected datacenters. If `<dc>:` is not set, the limit is global.
	// For instance, `[]string{"dc1:2", "5"}` corresponds to two parallel nodes in `dc1` datacenter and five parallel nodes in the other datacenters.
	// +optional
	SnapshotParallel []string `json:"snapshotParallel,omitempty"`

	// uploadParallel specifies a list of upload parallelism limits in the following format: `[<dc>:]<limit>`.
	// `<dc>:` is optional and allows for specifying different limits in selected datacenters. If `<dc>:` is not set, the limit is global.
	// For instance, `[]string{"dc1:2", "5"}` corresponds to two parallel nodes in `dc1` datacenter and five parallel nodes in the other datacenters.
	// +optional
	UploadParallel []string `json:"uploadParallel,omitempty"`
}

type ScyllaDBManagerRepairTaskOptions struct {
	// schedule specifies the schedule on which the repair task is run.
	ScyllaDBManagerTaskSchedule `json:",inline"`

	// dc specifies a list of datacenter `glob` patterns, e.g. `dc1`, `!otherdc*`, determining the datacenters to include or exclude from repair.
	// +optional
	DC []string `json:"dc,omitempty"`

	// failFast indicates that a repair should be stopped on first encountered error.
	// +optional
	FailFast *bool `json:"failFast,omitempty"`

	// host specifies the IPv4 or IPv6 address of a node to repair.
	// Specifying this field limits repair to token ranges replicated by a given node.
	// When used in conjunction with `dc`, the node must belong to the specified datacenters.
	// If not set, all hosts are repaired.
	// +optional
	Host *string `json:"host,omitempty"`

	// ignoreDownHosts indicates that the nodes in down state should be ignored during repair.
	// +optional
	IgnoreDownHosts *bool `json:"ignoreDownHosts,omitempty"`

	// intensity specifies the number of token ranges to repair in a single ScyllaDB node at the same time.
	// Changing the intensity impacts the repair granularity in case it is resumed. The higher the value, the more work on resumption.
	// When set to zero, the number of token ranges is adjusted to the maximum supported number.
	// When set to a value greater than the maximum supported by the node, intensity is capped at the maximum supported value.
	// Refer to repair documentation for details.
	// +optional
	Intensity *int64 `json:"intensity,omitempty"`

	// keyspace specifies a list of `glob` patterns used to include or exclude tables from repair.
	// The patterns match keyspaces and tables. Keyspace names are separated from table names with a dot e.g. `!keyspace.table_prefix_*`.
	// +optional
	Keyspace []string `json:"keyspace,omitempty"`

	// parallel specifies the maximum number of ScyllaDB repair jobs that can run at the same time (on different token ranges and replicas).
	// Each node can take part in at most one repair at any given moment. By default, or when set to zero, the maximum possible parallelism is used.
	// The maximal effective parallelism depends on keyspace replication strategy and cluster topology.
	// When set to a value greater than the maximum supported by the node, parallel is capped at the maximum supported value.
	// Refer to repair documentation for details.
	// +optional
	Parallel *int64 `json:"parallel,omitempty"`

	// smallTableThreshold enables small table optimization for tables of size lower than the given threshold.
	// +optional
	SmallTableThreshold *resource.Quantity `json:"smallTableThreshold,omitempty"`
}

type ScyllaDBManagerTaskSpec struct {
	// scyllaDBClusterRef is a typed reference to the target cluster in the same namespace.
	// Supported kind is ScyllaDBDatacenter in scylla.scylladb.com group.
	ScyllaDBClusterRef LocalScyllaDBReference `json:"scyllaDBClusterRef"`

	// type specifies the type of the task.
	Type ScyllaDBManagerTaskType `json:"type"`

	// backup specifies the options for a backup task.
	// +optional
	Backup *ScyllaDBManagerBackupTaskOptions `json:"backup,omitempty"`

	// repair specifies the options for a repair task.
	// +optional
	Repair *ScyllaDBManagerRepairTaskOptions `json:"repair,omitempty"`
}

type ScyllaDBManagerTaskStatus struct {
	// observedGeneration is the most recent generation observed for this ScyllaDBManagerTask. It corresponds to the
	// ScyllaDBManagerTask's generation, which is updated on mutation by the API Server.
	// +optional
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// conditions hold conditions describing ScyllaDBManagerTask state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// taskID reflects the internal identification number of the task in ScyllaDB Manager state.
	// It can be used to identify the task when interacting directly with ScyllaDB Manager.
	// +optional
	TaskID *string `json:"taskID,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:printcolumn:name="PROGRESSING",type=string,JSONPath=".status.conditions[?(@.type=='Progressing')].status"
// +kubebuilder:printcolumn:name="DEGRADED",type=string,JSONPath=".status.conditions[?(@.type=='Degraded')].status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

type ScyllaDBManagerTask struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec defines the desired state of ScyllaDBManagerTask.
	Spec ScyllaDBManagerTaskSpec `json:"spec,omitempty"`

	// status reflects the observed state of ScyllaDBManagerTask.
	Status ScyllaDBManagerTaskStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ScyllaDBManagerTaskList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ScyllaDBManagerTask `json:"items"`
}
