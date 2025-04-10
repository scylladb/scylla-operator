# ScyllaDBCluster integration with ScyllaDB Manager: cluster registration, backup and repair scheduling

## Summary

This proposal aims at introducing initial support for ScyllaDBCluster integration with ScyllaDB Manager.
The initial integration covers support for registration of managed, multi-datacenter clusters, and scheduling backup and repair tasks.

## Motivation

We want to introduce ScyllaDB Manager integration with ScyllaDBCluster to extend our existing integration to automated multi-datacenter clusters.
We want to support all the operations we currently support with single-datacenter clusters to at least the same extent.
This also allows us to deal with some of the issues and inconsistencies existing in ScyllaCluster's API related to ScyllaDB Manager integration.

### Goals

- Provide a suggested deployment mechanism for unmanaged ScyllaDB Manager server deployment in multi-datacenter Kubernetes environments.
- Introduce support for registration of ScyllaDBDatacenters with ScyllaDB Manager.
- Introduce support for registration of ScyllaDBClusters with ScyllaDB Manager.
- Introduce support for scheduling of backup tasks for ScyllaDBDatacenters and ScyllaDBClusters with ScyllaDB Manager.
- Introduce support for scheduling of repair tasks for ScyllaDBDatacenters and ScyllaDBClusters with ScyllaDB Manager.
- Maintain support for registering ScyllaClusters and scheduling backup and repair tasks for ScyllaClusters with ScyllaDB Manager.
- Introduce support for removing ScyllaDBClusters, ScyllaDBDatacenters and, transitively, ScyllaClusters from ScyllaDB Manager state on objects' deletion [#1930][2].

### Non-Goals

- Introduce support for managed ScyllaDB Manager server deployments.
- Make adjustments to communication channel encryption or means of authentication.
- Fix backup task scheduling regression with AuthN enabled [#2548][4].
- Introduce support for automated scheduling of restore tasks for either API kind.
- Introduce a procedure for manual scheduling of restore tasks for ScyllaDBClusters.
- Propagate manager-controller status conditions to ScyllaDBCluster or ScyllaCluster [#1921][3].

## Proposal

I propose to introduce support for ScyllaDBClusters' (scylla.scylladb.com/v1alpha1) and ScyllaDBDatacenters' (scylla.scylladb.com/v1alpha1) registration with ScyllaDBManager with no changes to ScyllaDBCluster nor ScyllaDBDatacenter API.
For this measure, I propose adding a new API object: `ScyllaDBManagerClusterRegistration.scylla.scylladb.com/v1alpha1`. Other than keeping the metadata involved in cluster registration with ScyllaDB Manager, the purpose of the intermediate resource is future-proofing the approach for introduction of managed ScyllaDB Manager instances, in which case the new API can also be extended to serve as a reverse reference to a ScyllaDB Manager instance.
It is an internal resource managed by Scylla Operator and users are not expected to perform any operations involving it.

To support task scheduling, I propose to add a new API object: `ScyllaDBManagerTask.scylla.scylladb.com/v1alpha1`.
The new API will allow for referencing both ScyllaDBClusters (scylla.scylladb.com/v1alpha1) and ScyllaDBDatacenters (scylla.scylladb.com/v1alpha1).

Manager-controller, today deployed alongside ScyllaDB Manager server, will be removed in favour of a new set of controllers, extending Scylla Operator, responsible for registering ScyllaDB clusters and scheduling tasks with unmanaged ScyllaDB Manager server.
This change is designed to decommission the external component (manager-controller) while maintaining full configuration compatibility on the Kubernetes object level.

To support the legacy API without duplicating the reconciliation logic, the existing controller responsible for ScyllaCluster migration will translate tasks defined in `scyllaclusters.scylla.scylladb.com/v1.spec.backups` and `scyllaclusters.scylla.scylladb.com/v1.spec.repairs` into the new API objects.
Correspondingly, all existing ScyllaCluster status fields related to ScyllaDB Manager will be updated by the translation controller. 

### User Stories

#### Unmanaged deployment of ScyllaDB Manager server in multi-datacenter environments

As a user, I want to deploy ScyllaDB Manager server deployment in a multi-datacenter environment, to work with automated, multi-datacenter ScyllaDB clusters.
To do that, I deploy the global ScyllaDB Manager server instance in `scylla-manager` namespace in a single Kubernetes cluster, in which the ScyllaDBCluster object is deployed.

#### Multi-datacenter ScyllaDB cluster registration with unmanaged ScyllaDB Manager

As a user, I want to register multi-datacenter ScyllaDB clusters, reflected in Kubernetes by ScyllaDBClusters (scylla.scylladb.com/v1alpha1), with ScyllaDB Manager.
To do that, I mark the ScyllaDBCluster object with a predefined label. 

#### ScyllaDB Manager backup and repair task scheduling for multi-datacenter ScyllaDB clusters registered with ScyllaDB Manager

As a user, I want to schedule backup and repair tasks for multi-datacenter ScyllaDB clusters registered with ScyllaDB Manager.
To do that, I create ScyllaDBManagerTask (scylla.scylladb.com/v1alpha1) objects referencing a ScyllaDBCluster.

#### ScyllaDB Manager backup and repair task scheduling for single-datacenter ScyllaDB clusters registered with ScyllaDB Manager

As a user, I want to maintain the possibility of scheduling backup and repair tasks for single-datacenter ScyllaDB clusters registered with ScyllaDB Manager.
To do that, I either:
- Use the legacy method: configure the legacy ScyllaCluster (scylla.scylladb.com/v1) object with `scyllaclusters.scylla.scylladb.com/v1.spec.backups` and `scyllaclusters.scylla.scylladb.com/v1.spec.repairs`.
- Use the new method: create ScyllaDBManagerTask (scylla.scylladb.com/v1alpha1) objects referencing a single-datacenter ScyllaDBCluster.
- Use the new method: create ScyllaDBManagerTask (scylla.scylladb.com/v1alpha1) objects referencing a ScyllaDBDatacenter.

### Risks and Mitigations

#### Insecure communication channels

There are multiple insecure communication channels in the existing ScyllaDB Manager integration, including:
- ScyllaDB Manager server to ScyllaDB: uses CQL without SSL
- manager-controller to ScyllaDB Manager server communication: HTTP instead of HTTPS
- ScyllaDB Manager server to ScyllaDB Manager agent: opportunistic TLS + authentication token (debatable)

With this proposal we do not make any adjustments to the above. We do not, however, introduce any regressions in that area and the adjustments can be made independently.

#### Manager-controller to in-operator controllers migration

Migrating the ScyllaDB Manager integration logic from the standalone manager-controller application to in-operator controllers carries a risk of disrupting ScyllaDB Manager state.
Clusters or tasks already existing in ScyllaDB Manager state should not be abandoned nor registered/scheduled multiple times.

To mitigate this, we override the naming convention for clusters and tasks defined with legacy ScyllaCluster (scylla.scylladb.com/v1) API.

#### Multiple ScyllaDB Manager server deployments reconciling the same cluster with a manual multi-datacenter procedure

Users following the manual procedure for deploying multi-datacenter ScyllaDB clusters in interconnected Kubernetes clusters (["Deploy a multi-datacenter ScyllaDB cluster in multiple interconnected Kubernetes clusters"][5]) may end up with multiple ScyllaDB Manager server instances registering and working with the cluster if they deploy ScyllaDB Manager in multiple Kubernetes clusters [#2119][6].
Users will be expected to deploy ScyllaDB Manager in only one datacenter and/or switch to the automated multi-datacenter ScyllaDB clusters.

## Design Details

### Deployment mechanism for ScyllaDB Manager

#### Changes to the deployment mechanism in single-datacenter environments

The existing deployment mechanism in single-datacenter environments is modified.
The standalone manager-controller must no longer be deployed. Its functionality is replaced by a new set of controllers extending Scylla Operator.

#### Deployment mechanism in multi-datacenter environments

We introduce a new deployment mechanism for unmanaged ScyllaDB Manager server deployments in multi-datacenter environments.
To work with ScyllaDBClusters in multi-datacenter environments, ScyllaDB Manager server must be deployed in the "control plane" cluster, in which the ScyllaDBCluster, that the ScyllaDB Manager server should integrate with, is deployed.
ScyllaDB Manager server must be deployed in `scylla-manager` namespace.
The standalone manager-controller must not be deployed.

### Registration of ScyllaDB clusters with ScyllaDB Manager

To support ScyllaDB cluster registration of both ScyllaDBClusters (scylla.scylladb.com/v1alpha1) and ScyllaDBDatacenters (scylla.scylladb.com/v1alpha1), we introduce a new API: `ScyllaDBManagerClusterRegistration.scylla.scylladb.com/v1alpha1`.
We extend Scylla Operator with a new controller: GlobalScyllaDBManager controller. The controller does not correspond to any existing Kubernetes resources but the "virtual" ScyllaDB Manager instance.
GlobalScyllaDBManager controller will create a ScyllaDBManagerClusterRegistration all ScyllaDBCluster and ScyllaDBDatacenter objects across all namespaces with `"scylla-operator.scylladb.com/register-with-global-scylladb-manager": "true"` label in their corresponding namespaces.
No other clusters in ScyllaDB Manager's state will be subject to reconciliation.

The GlobalScyllaDBManager controller will not create any ScyllaDBManagerClusterRegistration objects if `scylla-manager` namespace is not present.

The ScyllaDBManagerClusterRegistration created by GlobalScyllaDBManager controller will have an internal label set up `"internal.scylla-operator.scylladb.com/global-scylladb-manager": "true"`.

The ScyllaDBManagerClusterRegistration objects created for ScyllaDBDatacenters and ScyllaDBClusters matched by the global ScyllaDBManager instance will adhere to the following declarative naming convention to avoid name collisions and exceeding max DNS1123 subdomain length:
```
concatenate(
  truncate(
    concatenate(toLower(SCYLLADB_CLUSTER_KIND), '-', SCYLLADB_CLUSTER_NAME), 
    min(len(concatenate(SCYLLADB_CLUSTER_KIND, '-', SCYLLADB_CLUSTER_NAME)), DNS1123SubdomainMaxLength - 5)
  ),
  truncate(FNV-1a(SCYLLADB_CLUSTER_KIND, SCYLLADB_CLUSTER_NAME), 5)
)
```
where `FNV-1a` computes a base-36 string representation of a 64-bit FNV-1a hash. `SCYLLADB_CLUSTER_*` parameters refer to ScyllaDBDatacenter or ScyllaDBCluster objects. 

Scylla Operator will be extended with a controller reconciling ScyllaDBManagerClusterRegistration, responsible for registering ScyllaDBDatacenter and ScyllaDBCluster objects with a ScyllaDB Manager instance.
For the time being, only ScyllaDBManagerClusterRegistration objects with the internal label `"internal.scylla-operator.scylladb.com/global-scylladb-manager": "true"` will be reconciled.
The internal label will be used as a selector by GlobalScyllaDBManager controller.

To mark ScyllaDBClusters or ScyllaDBDatacenters for registration with the global ScyllaDB Manager server, users will label the objects with a dedicated label: `"scylla-operator.scylladb.com/register-with-global-scylladb-manager": "true"`.
To avoid ScyllaDB manager registering the same cluster through both ScyllaDBCluster and the underlying ScyllaDBDatacenters, the ScyllaDBCluster controller will not propagate the label to the created ScyllaDBDatacenters.

#### Naming convention in ScyllaDB Manager state

ScyllaDBManagerClusterRegistration controller create clusters in ScyllaDB Manager state with the following name: `<scylladbmanagerclusterregistration.scylla.scylladb.com/v1alpha1.spec.scyllaDBClusterRef.Kind>/<scylladbmanagerclusterregistration.scylla.scylladb.com/v1alpha1.spec.scyllaDBClusterRef.Name>`
ScyllaDB Manager clusters created for ScyllaDBManagerClusterRegistration objects with `"internal.scylla-operator.scylladb.com/global-scylladb-manager": "true"` label will additionally be prefixed with `<scylladbmanagerclusterregistration.scylla.scylladb.com/v1alpha1.metadata.namespace>/`.

To ensure continuity with the existing state of global ScyllaDB Manager instance, we will enable overriding the name with an internal annotation `"internal.scylla-operator.scylladb.com/scylladb-manager-cluster-name-override"`.
GlobalScyllaDBManager controller will propagate the annotation from ScyllaDBDatacenter and ScyllaDBCluster object to ScyllaDBManagerClusterRegistration objects.

#### Definition of `ScyllaDBManagerClusterRegistration.scylla.scylladb.com/v1alpha1`

```go
type LocalScyllaDBReference struct {
	// kind specifies the type of the resource.
	Kind string `json:"kind"`
	// name specifies the name of the resource.
	Name string `json:"name"`
}

type ScyllaDBManagerClusterRegistrationSpec struct {
	// scyllaDBClusterRef specifies the typed reference to the local ScyllaDB cluster.
	// Supported kinds are ScyllaDBCluster and ScyllaDBDatacenter in scylla.scylladb.com group.
	ScyllaDBClusterRef LocalScyllaDBReference `json:"scyllaDBClusterRef"`
}

type ScyllaDBManagerClusterRegistrationStatus struct {
	// observedGeneration is the most recent generation observed for this ScyllaDBManagerClusterRegistration. It corresponds to the
	// ScyllaDBManagerTask's generation, which is updated on mutation by the API Server.
	// +optional
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// conditions hold conditions describing ScyllaDBManagerClusterRegistration state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	
	// clusterID reflects the internal identification number of the cluster in ScyllaDB Manager state.
	// +optional
	ClusterID *string `json:"clusterID,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:printcolumn:name="PROGRESSING",type=string,JSONPath=".status.conditions[?(@.type=='Progressing')].status"
// +kubebuilder:printcolumn:name="DEGRADED",type=string,JSONPath=".status.conditions[?(@.type=='Degraded')].status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

type ScyllaDBManagerClusterRegistration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec defines the desired state of ScyllaDBManagerClusterRegistration.
	Spec ScyllaDBManagerClusterRegistrationSpec `json:"spec,omitempty"`

	// status reflects the observed state of ScyllaDBManagerClusterRegistration.
	Status ScyllaDBManagerClusterRegistrationStatus `json:"status,omitempty"`
}
```

#### `ScyllaCluster.scylla.scylladb.com/v1` conversion mechanism

The existing ScyllaCluster (scylla.scylladb.com/v1) controller, running as part of the standalone manager-controller, will be removed.
To ensure that users delete the `manager-controller` component, Scylla Operator will verify that the corresponding Deployment is not present in `scylla-manager` namespace. Otherwise, the command will fail to start and log a message with the reason and instructions explaining that `manager-controller` Deployment should be removed.

To ensure backwards compatibility and keep reconciling all ScyllaClusters in the Kubernetes cluster with the global ScyllaDB Manager instance, the migration controller by default labels all ScyllaDBDatacenters created as a result of migration from ScyllaClusters with the label: `"scylla-operator.scylladb.com/register-with-global-scylladb-manager": "true"`. 
Conversely, the migration controller will translate the status of ScyllaDBManagerClusterRegistration object created for the underlying ScyllaDBDatacenter into ScyllaCluster's status and update it.

The translation controller will set the internal annotation `"internal.scylla-operator.scylladb.com/name-override": "<scyllaclusters.scylla.scylladb.com/v1.metadata.namespace>/<scyllaclusters.scylla.scylladb.com/v1.metadata.name>"` on all ScyllaDBDatacenters created as a result of translation.

#### Auth tokens

ScyllaDBCluster controller, running as part of the operator, will generate the auth token secret and configure the created ScyllaDBDatacenters with an internal annotation `internal.scylla-operator.scylladb.com/scylladb-manager-agent-auth-token-override-secret-ref`.
ScyllaDBDatacenter controller will first try to get the auth token from the custom config in an unchanged manner. If the custom config is absent or does not specify the auth token, it will try to get the auth token from the Secret specified in the annotation. 
In case the annotation is absent, it will retain the already generated token or generate it in an unchanged manner.

#### Finalizers

ScyllaDBManagerClusterRegistration controller, running as part of the operator, will set a `scylla-operator.scylladb.com/scylladbmanagerclusterregistration-deletion` finalizer on the reconciled object so that the clusters are deleted from ScyllaDB Manager state on deletion [#1930][2].
ScyllaDBManagerClusterRegistration objects registering with global ScyllaDB Manager instance will be considered deregistered if `scylla-manager` namespace no longer exists or has a deletion timestamp set.

#### Status conditions

ScyllaDBManagerClusterRegistration controller, running as part of the operator, will propagate ScyllaDB Manager client errors to `Degraded` status condition of ScyllaDBManagerClusterRegistration object.

#### `ScyllaDBManagerClusterRegistration.scylla.scylladb.com/v1alpha1` validation

A validating admission webhook will be set up for `ScyllaDBManagerClusterRegistration.scylla.scylladb.com/v1alpha1`.
All fields will be validated. Only the objects with `"internal.scylla-operator.scylladb.com/global-scylladb-manager": "true"` internal label will be admitted.

### Scheduling of backup and repair tasks with ScyllaDB Manager

We introduce a new API, `ScyllaDBManagerTask.scylla.scylladb.com/v1alpha1`, which allows users to schedule backup and repair tasks for ScyllaDBClusters and ScyllaDBDatacenters registered with the global ScyllaDB Manager instance.
Scylla Operator will be extended with a dedicated controller reconciling ScyllaDBManagerTask objects, responsible for scheduling the specified tasks with ScyllaDB Manager.
No other tasks in ScyllaDB Manager's state will be subject to reconciliation.

ScyllaDBManagerTask controller will retrieve the ScyllaDBManager instance that it should schedule the tasks with for the ScyllaDBDatacenter/ScyllaDBCluster object that it refers to through the use of the declaratively named ScyllaDBManagerClusterRegistration object created by GlobalScyllaDBManager controller. 
For ScyllaDBDatacenter/ScyllaDBCluster objects with `scylla-operator.scylladb.com/register-with-global-scylladb-manager: "true"` label, ScyllaDBManagerTask controller will retrieve the intermediate ScyllaDBManagerClusterRegistration objects from `scylla-manager` namespace.
For the time being, only ScyllaDBDatacenter/ScyllaDBCluster objects with `scylla-operator.scylladb.com/register-with-global-scylladb-manager: "true"` label will be supported.

#### Naming convention in ScyllaDB Manager state

ScyllaDBManagerTask controller will set the name of the clusters registered in the global ScyllaDB Manager's state to `scylladbmanagertask.scylla.scylladb.com/v1alpha1.metadata.name`.

To ensure continuity with the existing state of global ScyllaDB Manager instance, we will enable overriding the name with an internal annotation `"internal.scylla-operator.scylladb.com/name-override"`.

#### Definition of `ScyllaDBManagerTask.scylla.scylladb.com/v1alpha1`

```go
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

	// dc specifies a list of datacenter `glob` patterns separated by commas, e.g. `dc1,!otherdc*`, determining the datacenters to include or exclude from backup.
	// +optional
	DC []string `json:"dc,omitempty"`

	// keyspace specifies a list of `glob` patterns separated by commas used to include or exclude tables from backup.
	// The patterns match keyspaces and tables. Keyspace names are separated from table names with a dot e.g. `keyspace,!keyspace.table_prefix_*`.
	// +optional
	Keyspace []string `json:"keyspace,omitempty"`

	// location specifies a list of backup locations in the following format: `[<dc>:]<provider>:<name>`.
	// `<dc>:` is optional and allows to specify the location for a datacenter in a multi-datacenter cluster.
	// `<provider>` specifies the storage provider.
	// `<name>` specifies a bucket name and must be an alphanumeric string which may contain a dash and or a dot, but other characters are forbidden.
	Location []string `json:"location"`

	// rateLimit specifies the limit for the upload rate, expressed in  megabytes (MiB) per second, at which the snapshot files can be uploaded from a ScyllaDB node to its backup destination, in the following format: `[<dc>:]<limit>`.
	// `<dc>:` is optional and allow for specifying different upload limits in selected datacenters.
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

	// dc specifies a list of datacenter `glob` patterns separated by commas, e.g. `dc1,!otherdc*`, determining the datacenters to include or exclude from repair.
	// +optional
	DC []string `json:"dc,omitempty"`

	// keyspace specifies a list of `glob` patterns separated by commas used to include or exclude tables from repair.
	// The patterns match keyspaces and tables. Keyspace names are separated from table names with a dot e.g. `keyspace,!keyspace.table_prefix_*`.
	// +optional
	Keyspace []string `json:"keyspace,omitempty"`

	// failFast indicates that a repair should be stopped on first encountered error.
	// +optional
	FailFast *bool `json:"failFast,omitempty"`

	// host specifies the IPv4 or IPv6 address of a node to repair.
	// Specifying this field limits repair to token ranges replicated by a given node.
	// When used in conjunction with `dc`, the node must belong to the specified datacenters.
	// If not set, all hosts are repaired.
	// +optional
	Host *string `json:"host,omitempty"`

	// intensity specifies the number of token ranges to repair in a single ScyllaDB node at the same time.
	// Changing the intensity impacts the repair granularity in case it is resumed. The higher the value the more work on resumption.
	// When set to zero, the number of token ranges is adjusted to the maximum supported number.
	// When set to a value greater than the maximum supported by the node, intensity is capped at the maximum supported value.
	// Refer to repair documentation for details.
	// +optional
	Intensity *int64 `json:"intensity,omitempty"`

	// parallel specifies the maximum number of ScyllaDB repair jobs that can run at the same time (on different token ranges and replicas).
	// Each node can take part in at most one repair at any given moment. By default, the maximum possible parallelism is used.
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
    // Supported kinds are ScyllaDBCluster (scylla.scylladb.com/v1alpha1) and ScyllaDBDatacenter (scylla.scylladb.com/v1alpha1).
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
```

#### `ScyllaCluster.scylla.scylladb.com/v1` conversion mechanism

The existing ScyllaCluster (scylla.scylladb.com/v1) controller, running as part of the standalone manager-controller, will be removed. Backup and repair tasks defined in `scyllaclusters.scylla.scylladb.com/v1.spec.backups` and `scyllaclusters.scylla.scylladb.com/v1.spec.repairs` will no longer be reconciled directly.
Instead, the existing ScyllaCluster migration controller, running as part of the operator, is extended to convert the backup and repair tasks defined in `scyllaclusters.scylla.scylladb.com/v1.spec.backups` and `scyllaclusters.scylla.scylladb.com/v1.spec.repairs` into the ScyllaDBManagerTask objects and apply them.

ScyllaDBManagerTask objects, created by the translation controller, adhere to the following naming convention to avoid name collisions and exceeding max DNS1123 subdomain length:
```
concatenate(
  truncate(
    concatenate(SCYLLA_CLUSTER_NAME, '-', TASK_TYPE, '-', TASK_NAME), 
    min(len(concatenate(SCYLLA_CLUSTER_NAME, '-', TASK_TYPE, '-', TASK_NAME)), DNS1123SubdomainMaxLength - 5)
  ),
  truncate(FNV-1a(SCYLLA_CLUSTER_NAME, TASK_TYPE, TASK_NAME), 5)
)
```
where `FNV-1a` computes a base-36 string representation of a 64-bit FNV-1a hash.

To maintain backwards compatibility, we keep reflecting the tasks' state in ScyllaDB Manager in `scyllaclusters.scylla.scylladb.com/v1.status.backups` and `scyllaclusters.scylla.scylladb.com/v1.status.repairs`.
The new ScyllaDBManagerTask controller will update ScyllaDBManagerTask objects with an internal annotation `internal.scylla-operator.scylladb.com/scylladb-manager-task-status` in respective JSON encoded structs.
The translating controller will use the annotation values to update the ScyllaCluster status.

The translation controller will set the internal annotation `"internal.scylla-operator.scylladb.com/scylladb-manager-task-name-override"` on all ScyllaDBManagerTasks created as a result of translation.
The annotation value will be set to `"<scyllaclusters.scylla.scylladb.com/v1.spec.backups[].name>"` for backup tasks and `"<scyllaclusters.scylla.scylladb.com/v1.spec.repairs[].name>"` for repair tasks created.

#### Notable changes between `ScyllaDBManagerTask.scylla.scylladb.com/v1alpha1` and `ScyllaClusters.scylla.scylladb.com/v1`

##### Status

The new API drops all status propagation but the internal identification number of the given task in ScyllaDB Manager state, also referred to as TaskID.
This is due to the fact that the new ScyllaDBCluster, ScyllaDBDatacenter and ScyllaDBManagerTask controllers will no longer use the objects' statuses for performing updates.

The dedicated controller will propagate any errors returned by the ScyllaDB Manager client to the object's status conditions.

##### Spec
The notable changes, as compared to the existing `scyllaclusters.scylla.scylladb.com/v1.spec.backups` and `scyllaclusters.scylla.scylladb.com/v1.spec.repairs`, and the corresponding conversion mechanisms are described below.

- Schedule (shared by backup and repair specifications):
  - The deprecated `Interval` field is dropped from the new API.
    To maintain backwards compatibility, the translating controller will set an internal annotation `internal.scylla-operator.scylladb.com/scylladb-manager-task-schedule-interval-override` on applied ScyllaDBManagerTask objects.
    The ScyllaDBManagerTask controller propagates the value of the annotation on task update/creation with ScyllaDB Manager.
  - The type of `StartDate` field is changed to `*metav1.Time`. With that we no longer support the special `now[+duration]` syntax in the new API.
    This is due to the existing integration workarounds necessary with the declarative API and idempotent operations.
    To schedule tasks immediately, users will set `StartDate` to a nil value.
    To maintain backwards compatibility with `now[+duration]` syntax, the translating controller will set an internal annotation `internal.scylla-operator.scylladb.com/scylladb-manager-task-schedule-start-date-override` on applied ScyllaDBManagerTask objects.
    The ScyllaDBManagerTask controller will then use the annotation value and the task's in-manager state to calculate a start date on task update/creation with ScyllaDB Manager.
  - The `Timezone` field is dropped from the new API. This is due to the unclear semantics of the parameter [scylladb/scylla-manager#3818][1].
    To maintain backwards compatibility, the translating controller will set an internal annotation `internal.scylla-operator.scylladb.com/scylladb-manager-task-schedule-timezone-override` on applied ScyllaDBManagerTask objects.
    The ScyllaDBManagerTask controller propagates the value of the annotation on task update/creation with ScyllaDB Manager.
    The field, as well as support for `TZ=` and `CRON_TZ=` prefixes in `Cron`, can be introduced once [scylladb/scylla-manager#3818][1] is resolved. For now, we make the validation as strict as possible.
  - The default value for `NumRetries` is removed from the new API. The defaults for not provided parameters should be handled by ScyllaDB Manager server.
- Backup:
  - The default value for `Retention` is removed from the new API. The ScyllaDB Manager server assumes a default value if the parameter is no set explicitly.
- Repair:
  - The type of `SmallTableThreshold` field is changed to `*resource.Quantity` to align with in-tree Kubernetes objects. Validation will enforce non-decimal values.
  - The type of `Intensity` field is changed to `int64` to align with ScyllaDB Manager server.
    ScyllaDB Manager API server accepts a float64 value, but overrides any values in (0,1) range. It responds with a 400 status on decimal values > 1. Unfortunately, ScyllaCluster validation allows for decimal values > 1.
    As we can not make the validation stricter on translation, to maintain backwards compatibility, the translating controller will set an internal annotation `internal.scylla-operator.scylladb.com/scylladb-manager-task-repair-intensity-override` on applied ScyllaDBManagerTask objects.
    ScyllaDBManagerTask controller will use it to override the value set in the object's specification.
  - The default value for `FailFast` is removed from the new API. The ScyllaDB Manager server assumes a default value if the parameter is no set explicitly.
  - The default value for `SmallTableThreshold` is removed from the new API. The ScyllaDB Manager server assumes a default value if the parameter is no set explicitly.
  - The default value for `Retention` is removed from the new API. The ScyllaDB Manager server assumes a default value if the parameter is no set explicitly.
  - The default value for `Parallel` is removed from the new API. The ScyllaDB Manager server assumes a default value if the parameter is no set explicitly.
  - ScyllaDBManagerTask controller will no longer override `NumRetries` when `FailFast` is set to true. If required, such behaviour should be enforced by ScyllaDB Manager server.
    The translating controller will maintain this behaviour when applying ScyllaDBManagerTasks.

The translating controller will keep respecting the default values defined in ScyllaCluster API and propagate them to applied ScyllaDBManagerTasks.

#### Finalizers

The new ScyllaDBManagerTask controller will set a `scylla-operator.scylladb.com/scylladbmanagertask-deletion` finalizer on ScyllaDBManagerTask objects so that the tasks are deleted from ScyllaDB Manager state on object deletion.

#### Status conditions

The new ScyllaDBManagerTask controller will propagate ScyllaDB Manager client errors to `Degraded` status condition.

The translating controller will propagate the error to a corresponding task's status in ScyllaCluster object.

#### Owner references

The translating controller will set the owner references of the created ScyllaDBManagerTask objects to a corresponding ScyllaCluster. The created ScyllaDBManagerTask objects will be garbage collected on ScyllaCluster's deletion according to the deletion policy.

#### Providing credentials for object storage access by ScyllaDB Manager agent sidecar

ScyllaDB Manager agent containers require credentials to the corresponding regional object storage.
Users will be responsible for configuring the credentials for each datacenter by defining `scylladbclusters.scylla.scylladb.com/v1alpha1.spec.datacenters[].scyllaDBManagerAgent.volumes` and `scylladbclusters.scylla.scylladb.com/v1alpha1.spec.datacenters[].scyllaDBManagerAgent.volumeMounts`.

### Test Plan

The existing set of E2E tests related to ScyllaCluster integration with ScyllaDB Manager will be maintained in an unchanged form.
The tests will cover correctness of ScyllaCluster's task spec conversion to ScyllaDBManagerTasks' specs and ScyllaDBManagerTasks' statuses to ScyllaClusters' task statuses.

Conversion logic between ScyllaCluster's (scylla.scylladb.com/v1) in-manager state, reflected in its status, and ScyllaDBManagerClusterRegistration (scylla.scylladb.com/v1alpha1) will be covered by unit tests.
Conversion logic between ScyllaCluster's (scylla.scylladb.com/v1) task specification and ScyllaDBManagerTasks (scylla.scylladb.com/v1alpha1) will be covered by unit tests.

A set of new E2E tests will be introduced to test ScyllaDBCluster and ScyllaDBDatacenter integration, including:
- Registering ScyllaDBDatacenters with global ScyllaDB Manager.
- Registering ScyllaDBClusters with global ScyllaDB Manager.
- Scheduling backup and repair tasks, defined by ScyllaDBManagerTask objects, with global ScyllaDB Manager.
  ScyllaDBManagerTasks will be tested with references to ScyllaDBClusters and ScyllaDBDatacenters.
  Tests for backup task scheduling will not cover the workaround for when AuthN is enabled [#2548][4].

In order to set up E2E tests covering backup task scheduling for ScyllaDBClusters in multi-datacenter environments, the test-runner and related scripts will be extended to accept object storage bucket location and credentials for multiple datacenters.
The relevant CI job definitions will also be adjusted.
No changes are expected in the CI tooling.

### Upgrade / Downgrade Strategy

The new ScyllaDBManagerClusterRegistration (scylla.scylladb.com/v1alpha1) and ScyllaDBManagerTask (scylla.scylladb.com/v1alpha1) CRDs will have to be installed ahead of Scylla Operator upgrade. 
Our documentation and release notes cover this requirement for both GitOps and Helm installation paths on the regular basis.

As the existing manager-controller deployment is dropped with this proposal, users following the Helm installation path will have to upgrade their scylla-manager release ahead of Scylla Operator upgrade. 
Users following the GitOps installation path will have to manually delete the manager-controller deployment from `scylla-manager` namespace, and perform the regular update of ScyllaDB Manager manifests ahead of Scylla Operator upgrade.
Release notes should explicitly mention and highlight the manual steps required on the GitOps path. 

On a downgrade, users following the Helm installation path will have to downgrade their scylla-manager release after they downgrade Scylla Operator.
Similarly, users following the GitOps path will have to apply ScyllaDB Manager manifests after they downgrade Scylla Operator.
Users will have to return to using task specifications in ScyllaCluster (scylla.scylladb.com/v1). The new CRs will not be migrated back. 
The new CR objects will be left behind and users might have to delete them manually from Kubernetes, as well as remove their corresponding objects from ScyllaDB Manager state.

### Version Skew Strategy

#### New Scylla Operator with current global ScyllaDB Manager deployment mechanism (manager-controller not removed)

The new set of controllers extending Scylla Operator will register each ScyllaCluster (scylla.scylladb.com/v1) and schedule its specified tasks with the global ScyllaDB Manager instance.
The migration controller will use the name overriding mechanism to ensure clusters and tasks already created in ScyllaDB Manager state by manager-controller are "adopted" by the new controllers. 

Not deleting the manager-controller instance could lead to conflicts between it and the new controllers. 
To mitigate this, Scylla Operator will verify that `manager-controller` Deployment is not present in `scylla-manager` namespace. Otherwise, the command will fail and log a message with a reason and instructions explaining that `manager-controller` Deployment must be removed from `scylla-manager` namespace. 

#### Current Scylla Operator with new global ScyllaDB Manager deployment mechanism (manager-controller removed)

No component will perform cluster registration or task scheduling with ScyllaDB Manager. ScyllaDB Manager state will not be lost.
The manager-controller will not reconcile the tasks defined in ScyllaCluster (scylla.scylladb.com/v1).

## Implementation History

- 2024-03-17: Initial enhancement proposal.
- 2024-04-02: Proposal extended with ScyllaDBManagerClusterRegistration (scylla.scylladb.com/v1alpha1) in an effort to future-proof the implementation for a move to managed ScyllaDB Manager deployments.

## Drawbacks

### No opt-out mechanism for ScyllaDB Manager integration

Introducing a controller of a "virtual" global ScyllaDB Manager instance comes with a drawback of no mechanism to opt-out of ScyllaDB Manager integration, up to this point achieved by not deploying the standalone manager-controller.
Users can still opt out of using ScyllaDB Manager with Scylla Operator by not deploying the global ScyllaDB Manager server instance. GlobalScyllaDBManager controller will not create any ScyllaDBClusterRegistration objects if `scylla-manager` namespace is not present. 
Any existing ScyllaDBClusterRegistration objects created by GlobalScyllaDBManager controller will be pruned on `scylla-manager` namespace deletion.

### No owner references for internal ScyllaDBManagerClusterRegistration (scylla.scylladb.com/v1alpha1)

As the GlobalScyllaDBManager controller reconciles a "virtual" global ScyllaDB Manager instance, and not an actual Kubernetes object, ScyllaDBManagerClusterRegistration objects created can't have an owner set up.
To address this, GlobalScyllaDBManager controller can be maintained after the global ScyllaDB Manager instance is sunsetted to prune all managed ScyllaDBManagerClusterRegistration objects.

### ScyllaDBManagerClusterRegistration (scylla.scylladb.com/v1alpha1) status conditions

With ScyllaDBManagerClusterRegistration (scylla.scylladb.com/v1alpha1) being introduced as an internal resource, status conditions related to cluster's state in ScyllaDB Manager set on the objects will not be intuitively accessible to the end user.
This is not a regression compared to the existing manager-controller implementation with ScyllaCluster (scylla.scylladb.com/v1) API and the status conditions related to cluster registration can be later be aggregated when a custom resource representing managed ScyllaDB Manager server deployments is introduced.

### Task run status propagation

The proposed API does not, in its current form, allow for propagating the status of the task runs, only the task creation/update operations.
To learn about the task runs and their state, users need to communicate with ScyllaDB Manager server directly through the use of `sctool`, which requires execing into the ScyllaDB Manager server container.
Unfortunately, ScyllaDB Manager does not provide a push API, and so the controllers would not have an inexpensive and reliable way of synchronising the manager state with Kubernetes object's statuses.
Therefore, the integration only covers operations on tasks, not task runs. The proposed API can, however, be extended in the future. 

## Alternatives

### Managed deployments
We considered introducing managed ScyllaDB Manager server deployments first, but with the proposed approach, the two features are mostly orthogonal.

### Preemptive introduction of ScyllaDBManager API

We considered a preemptive introduction of ScyllaDBManager API, the purpose of which would be to eventually serve as a declaration of intent of creating ScyllaDB Manager infrastructure, while also specifying ScyllaDB clusters to be registered with ScyllaDB Manager server instance.
Temporarily, ScyllaDBManager (scylla.scylladb.com/v1alpha1) would not involve infrastructure-related operations, but serve as a reflection of the unmanaged ScyllaDB Manager server.
ScyllaDBManagerClusterRegistration (scylla.scylladb.com/v1alpha1) would reference a ScyllaDBManager object.

We decided that the preemptive introduction of ScyllaDBManager (scylla.scylladb.com/v1alpha1) would be confusing without the infrastructure behind it, and wouldn't bring any value other than bringing the API closer to its eventual state, while also forcing us to make some decisions ahead of time.

### Addressing backup with AuthN regression
We considered addressing the automated backup with AuthN regression within this proposal. However, a viable solution would require adjusting the communication channels encryption and means of authentication, including setting up the ScyllaDB Manager server to ScyllaDB CQL over SSL communication and switching from HTTP to HTTPS in manager-controller to ScyllaDB Manager server communication, as well as automated provisioning of CQL credentials to ScyllaDB Manager server, which would require implementing the managed ScyllaDB Manager server deployments first. Therefore, we decided to take an iterative approach.

### Dedicated API field for auth token propagation from ScyllaDBClusters

We considered extending `ScyllaDBManagerAgentTemplate` in ScyllaDBDatacenter (scylla.scylladb.com/v1alpha1) with a dedicated field allowing for specifying a reference to a Secret containing a reserved key with the auth token as value.
ScyllaDBCluster controller could use this field to propagate the generated auth token to all created ScyllaDBDatacenters. On a positive side, ScyllaDBCluster controller wouldn't have to create a dedicated Secret in each datacenter.
Unfortunately, the user experience would become more convoluted as the auth token could be specified in both the dedicated Secret and the custom config Secret. Specifying the custom config Secret reference without specifying the auth token Secret reference would result in different behaviour in ScyllaDBDatacenters created by ScyllaDBCluster controller and standalone ScyllaDBDatacenters.

[1]: <https://github.com/scylladb/scylla-manager/issues/3818>
[2]: <https://github.com/scylladb/scylla-operator/issues/1930>
[3]: <https://github.com/scylladb/scylla-operator/issues/1921>
[4]: <https://github.com/scylladb/scylla-operator/issues/2548>
[5]: <https://operator.docs.scylladb.com/stable/resources/scyllaclusters/multidc/multidc.html>
[6]: <https://github.com/scylladb/scylla-operator/issues/2119>