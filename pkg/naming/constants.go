package naming

// These labels are only used on the ClusterIP services
// acting as each member's identity (static ip).
// Each of these labels is a record of intent to do
// something. The controller sets these labels and each
// member watches for them and takes the appropriate
// actions.
//
// See the sidecar design doc for more details.
const (
	// DecommissionedLabel expresses the intent to decommission
	// the specific member. The presence of the label expresses
	// the intent to decommission. If the value is true, it means
	// the member has finished decommissioning.
	// Values: {true, false}
	DecommissionedLabel = "scylla/decommissioned"

	// ReplaceLabel express the intent to replace pod under the specific member.
	ReplaceLabel = "scylla/replace"

	// ReplacingNodeHostIDLabel contains the Host ID of node labelled node is replacing.
	ReplacingNodeHostIDLabel = "internal.scylla-operator.scylladb.com/replacing-node-hostid"

	// NodeMaintenanceLabel means that node is under maintenance.
	// Readiness check will always fail when this label is added to member service.
	NodeMaintenanceLabel = "scylla/node-maintenance"

	// ForceIgnitionValueAnnotation allows to force ignition state. The value can be either "true" or "false".
	ForceIgnitionValueAnnotation = "internal.scylla-operator.scylladb.com/force-ignition-value"

	LabelValueTrue  = "true"
	LabelValueFalse = "false"
)

// Annotations used internally.
const (
	// HostIDAnnotation reflects the host_id of the scylla node.
	HostIDAnnotation = "internal.scylla-operator.scylladb.com/host-id"

	// CurrentTokenRingHashAnnotation reflects the current hash of token ring of the scylla node.
	CurrentTokenRingHashAnnotation = "internal.scylla-operator.scylladb.com/current-token-ring-hash"

	// LastCleanedUpTokenRingHashAnnotation reflects the last cleaned up hash of token ring of the scylla node.
	LastCleanedUpTokenRingHashAnnotation = "internal.scylla-operator.scylladb.com/last-cleaned-up-token-ring-hash"

	// CleanupJobTokenRingHashAnnotation reflects which version of token ring cleanup Job is cleaning.
	CleanupJobTokenRingHashAnnotation = "internal.scylla-operator.scylladb.com/cleanup-token-ring-hash"

	// NodeStatusReportAnnotation reflects the current status report from the ScyllaDB node.
	NodeStatusReportAnnotation = "internal.scylla.scylladb.com/scylladb-node-status-report"
)

// Annotations used for feature backward compatibility between v1.ScyllaCluster and v1alpha1.ScyllaDBDatacenter
const (
	TransformScyllaClusterToScyllaDBDatacenterHostNetworkingAnnotation               = "internal.scylla-operator.scylladb.com/host-networking"
	TransformScyllaClusterToScyllaDBDatacenterAlternatorPortAnnotation               = "internal.scylla-operator.scylladb.com/alternator-port"
	TransformScyllaClusterToScyllaDBDatacenterInsecureEnableHTTPAnnotation           = "internal.scylla-operator.scylladb.com/alternator-insecure-enable-http"
	TransformScyllaClusterToScyllaDBDatacenterInsecureDisableAuthorizationAnnotation = "internal.scylla-operator.scylladb.com/alternator-insecure-disable-authorization"
	TransformScyllaClusterToScyllaDBDatacenterSysctlsAnnotation                      = "internal.scylla-operator.scylladb.com/sysctls"
)

type ScyllaServiceType string

const (
	ScyllaServiceTypeIdentity ScyllaServiceType = "identity"
	ScyllaServiceTypeMember   ScyllaServiceType = "member"
)

type ScyllaDBClusterLocalServiceType string

const (
	ScyllaDBClusterLocalServiceTypeIdentity ScyllaDBClusterLocalServiceType = "identity"
)

type ScyllaIngressType string

const (
	ScyllaIngressTypeNode    ScyllaIngressType = "Node"
	ScyllaIngressTypeAnyNode ScyllaIngressType = "AnyNode"
)

type PodType string

const (
	// PodTypeScyllaDBNode indicates that the pod is a ScyllaDB node pod.
	PodTypeScyllaDBNode PodType = "scylladb-node"

	// PodTypeCleanupJob indicates that the pod is a cleanup job pod.
	PodTypeCleanupJob PodType = "cleanup-job"

	// PodTypeNodePerftuneJob indicates that the pod is a node perftune job pod.
	PodTypeNodePerftuneJob PodType = "node-perftune-job"

	// PodTypeNodeSysctlsJob indicates that the pod is a node sysctls job pod.
	PodTypeNodeSysctlsJob PodType = "node-sysctls-job"

	// PodTypeContainerPerftuneJob indicates that the pod is a container perftune job pod.
	PodTypeContainerPerftuneJob PodType = "container-perftune-job"

	// PodTypeContainerRLimitsJob indicates that the pod is a container resource limits job pod.
	PodTypeContainerRLimitsJob PodType = "container-rlimits-job"
)

// Generic Labels used on objects created by the operator.
const (
	// ClusterNameLabel specifies ScyllaCluster name. It's used as a selector by both ScyllaCluster and ScyllaDBDatacenter controllers.
	// As controller selector cannot be changed, this may be misleading in the context of ScyllaDBDatacenter. It should
	// specify the ScyllaCluster object name ScyllaDBDatacenter originated from or object name in case it wasn't migrated.
	ClusterNameLabel             = "scylla/cluster"
	DatacenterNameLabel          = "scylla/datacenter"
	RackNameLabel                = "scylla/rack"
	RackOrdinalLabel             = "scylla/rack-ordinal"
	ScyllaVersionLabel           = "scylla/scylla-version"
	ScyllaServiceTypeLabel       = "scylla-operator.scylladb.com/scylla-service-type"
	ScyllaIngressTypeLabel       = "scylla-operator.scylladb.com/scylla-ingress-type"
	ManagedHash                  = "scylla-operator.scylladb.com/managed-hash"
	NodeConfigJobForNodeUIDLabel = "scylla-operator.scylladb.com/node-config-job-for-node-uid"
	NodeConfigJobTypeLabel       = "scylla-operator.scylladb.com/node-config-job-type"
	NodeConfigJobData            = "scylla-operator.scylladb.com/node-config-job-data"
	NodeConfigNameLabel          = "scylla-operator.scylladb.com/node-config-name"
	ConfigMapTypeLabel           = "scylla-operator.scylladb.com/config-map-type"
	OwnerUIDLabel                = "scylla-operator.scylladb.com/owner-uid"
	ScyllaDBMonitoringNameLabel  = "scylla-operator.scylladb.com/scylladbmonitoring-name"
	ControllerNameLabel          = "scylla-operator.scylladb.com/controller-name"
	NodeJobLabel                 = "scylla-operator.scylladb.com/node-job"
	NodeJobTypeLabel             = "scylla-operator.scylladb.com/node-job-type"
	// PodTypeLabel specifies the type of the pod (e.g., ScyllaDB node). It's assigned to pods managed by the operator.
	PodTypeLabel = "scylla-operator.scylladb.com/pod-type"

	AppName           = "scylla"
	OperatorAppName   = "scylla-operator"
	ManagerAppName    = "scylla-manager"
	NodeConfigAppName = "scylla-node-config"

	PrometheusScrapeAnnotation = "prometheus.io/scrape"
	PrometheusPortAnnotation   = "prometheus.io/port"

	ForceRedeploymentReasonAnnotation = "scylla-operator.scylladb.com/force-redeployment-reason"
	InputsHashAnnotation              = "scylla-operator.scylladb.com/inputs-hash"
)

const (
	NodeConfigJobForNodeKey = "scylla-operator.scylladb.com/node-config-job-for-node"
)

// Configuration Values
const (
	ScyllaContainerName             = "scylla"
	ScyllaDBIgnitionContainerName   = "scylladb-ignition"
	ScyllaManagerAgentContainerName = "scylla-manager-agent"
	SidecarInjectorContainerName    = "sidecar-injection"
	PerftuneContainerName           = "perftune"
	SysctlsContainerName            = "sysctls"
	CleanupContainerName            = "cleanup"
	RLimitsContainerName            = "rlimits"

	PVCTemplateName = "data"

	SharedDirName = "/mnt/shared"

	ScyllaConfigDirName             = "/mnt/scylla-config"
	ScyllaAgentConfigDirName        = "/mnt/scylla-agent-config"
	ScyllaManagedAgentConfigDirName = "/mnt/scylla-managed-agent-config"
	ScyllaAgentConfigFileName       = "scylla-manager-agent.yaml"
	ScyllaAgentAuthTokenFileName    = "auth-token.yaml"
	ScyllaAgentConfigDefaultFile    = "/etc/scylla-manager-agent/scylla-manager-agent.yaml"
	ScyllaClientConfigDirName       = "/mnt/scylla-client-config"
	ScyllaDBManagedConfigDir        = "/var/run/configmaps/scylla-operator.scylladb.com/scylladb/managed-config"
	ScyllaDBSnitchConfigDir         = "/var/run/configmaps/scylla-operator.scylladb.com/scylladb/snitch-config"
	ScyllaConfigName                = "scylla.yaml"
	ScyllaDBManagedConfigName       = "scylladb-managed-config.yaml"
	ScyllaManagedConfigPath         = ScyllaDBManagedConfigDir + "/" + ScyllaDBManagedConfigName
	ScyllaRackDCPropertiesName      = "cassandra-rackdc.properties"
	ScyllaIOPropertiesName          = "io_properties.yaml"

	ScyllaDBIgnitionDonePath = SharedDirName + "/ignition.done"

	ScyllaDBSSTableBootstrapQueryResultPath = SharedDirName + "/sstable-bootstrap-query-result.json"

	DataDir = "/var/lib/scylla"

	ReadinessProbePath         = "/readyz"
	LivenessProbePath          = "/healthz"
	ScyllaDBAPIStatusProbePort = 8080
	ScyllaDBIgnitionProbePort  = 42081
	ScyllaAPIPort              = 10000

	OperatorEnvVarPrefix = "SCYLLA_OPERATOR_"
)

const (
	ScyllaManagerNamespace                  = "scylla-manager"
	ScyllaManagerServiceName                = "scylla-manager"
	StandaloneScyllaDBManagerControllerName = "scylla-manager-controller"

	ScyllaOperatorNodeTuningNamespace = "scylla-operator-node-tuning"

	ScyllaClusterMemberClusterRoleName = "scyllacluster-member"

	SingletonName = "cluster"

	PerftuneJobPrefixName        = "perftune"
	PerftuneServiceAccountName   = "perftune"
	SysctlsServiceAccountName    = "sysctls"
	RlimitsJobServiceAccountName = "rlimits"

	SysctlConfigFileName = "99-override_scylla-operator_scylladb_com.conf"
)

type NodeConfigJobType string

const (
	NodeConfigJobTypeNodePerftune            NodeConfigJobType = "NodePerftune"
	NodeConfigJobTypeNodeSysctls             NodeConfigJobType = "NodeSysctls"
	NodeConfigJobTypeContainerPerftune       NodeConfigJobType = "ContainerPerftune"
	NodeConfigJobTypeContainerResourceLimits NodeConfigJobType = "ContainerResourceLimits"
)

type ConfigMapType string

const (
	NodeConfigDataConfigMapType ConfigMapType = "NodeConfigData"
)

const (
	ScyllaRuntimeConfigKey string = "ScyllaRuntimeConfig"
)

type ProtocolDNSLabel string

const (
	CQLProtocolDNSLabel = "cql"
)

type NodeJobType string

const (
	JobTypeCleanup NodeJobType = "Cleanup"
)

const (
	DevLoopDeviceDirectoryName = "loops"
)

const (
	UpgradeContextConfigMapKey = "upgrade-context.json"
)

const (
	ManagedByClusterLabel = "scylla-operator.scylladb.com/managed-by-cluster"
)

const (
	ScyllaDBClusterNameLabel         = "scylla-operator.scylladb.com/scylladbcluster-name"
	ParentClusterNameLabel           = "scylla-operator.scylladb.com/parent-scylladbcluster-name"
	ParentClusterNamespaceLabel      = "scylla-operator.scylladb.com/parent-scylladbcluster-namespace"
	ParentClusterDatacenterNameLabel = "scylla-operator.scylladb.com/parent-scylladbcluster-datacenter-name"

	ScyllaDBClusterLocalServiceTypeLabel = "scylla-operator.scylladb.com/scylladbcluster-local-service-type"
)

const (
	RemoteOwnerClusterLabel   = "internal.scylla-operator.scylladb.com/remote-owner-cluster"
	RemoteOwnerNamespaceLabel = "internal.scylla-operator.scylladb.com/remote-owner-namespace"
	RemoteOwnerNameLabel      = "internal.scylla-operator.scylladb.com/remote-owner-name"
	RemoteOwnerGVR            = "internal.scylla-operator.scylladb.com/remote-owner-gvr"
)

const (
	ClusterEndpointsLabel       = "scylla-operator.scylladb.com/cluster-endpoints"
	RemoteClusterEndpointsLabel = "scylla-operator.scylladb.com/remote-cluster-endpoints"
)

const (
	RemoteClusterScyllaDBDatacenterNodesStatusReportLabel = "scylla-operator.scylladb.com/remote-cluster-scylladb-datacenter-nodes-status-report"
)

const (
	KubeConfigSecretKey string = "kubeconfig"
)

const (
	AppNameWithDomain = "scylla.scylladb.com"

	OperatorAppNameWithDomain       = "scylla-operator.scylladb.com"
	RemoteOperatorAppNameWithDomain = "remote.scylla-operator.scylladb.com"
)

const (
	RemoteKubernetesClusterFinalizer = "scylla-operator.scylladb.com/remotekubernetescluster-protection"
	ScyllaDBClusterFinalizer         = "scylla-operator.scylladb.com/scylladbcluster-protection"
)

const (
	KubernetesNameLabel      = "app.kubernetes.io/name"
	KubernetesManagedByLabel = "app.kubernetes.io/managed-by"
)

const (
	GlobalScyllaDBManagerRegistrationLabel = "scylla-operator.scylladb.com/register-with-global-scylladb-manager"
	GlobalScyllaDBManagerLabel             = "internal.scylla-operator.scylladb.com/global-scylladb-manager"

	// DisableGlobalScyllaDBManagerIntegrationAnnotation can be set to `true` to disable v1.ScyllaCluster integration with the global ScyllaDB Manager which is enabled by default.
	DisableGlobalScyllaDBManagerIntegrationAnnotation = "scylla-operator.scylladb.com/disable-global-scylladb-manager-integration"

	// ScyllaDBManagerAgentAuthTokenOverrideSecretRefAnnotation is used to override the auth tokens generated for specific ScyllaDBDatacenters with a shared one, common for the entire ScyllaDBCluster.
	ScyllaDBManagerAgentAuthTokenOverrideSecretRefAnnotation = "internal.scylla-operator.scylladb.com/scylladb-manager-agent-auth-token-override-secret-ref"

	ScyllaDBManagerClusterRegistrationFinalizer              = "scylla-operator.scylladb.com/scylladbmanagerclusterregistration-deletion"
	ScyllaDBManagerClusterRegistrationNameOverrideAnnotation = "internal.scylla-operator.scylladb.com/scylladb-manager-cluster-name-override"

	ScyllaDBManagerTaskFinalizer = "scylla-operator.scylladb.com/scylladbmanagertask-deletion"
	// ScyllaDBManagerTaskMissingOwnerUIDForceAdoptAnnotation is used to annotate a ScyllaDBManagerTask to force adoption of a matching task in ScyllaDB Manager state that is missing an owner UID label.
	ScyllaDBManagerTaskMissingOwnerUIDForceAdoptAnnotation         = "internal.scylla-operator.scylladb.com/scylladb-manager-task-missing-owner-uid-force-adopt"
	ScyllaDBManagerTaskNameOverrideAnnotation                      = "internal.scylla-operator.scylladb.com/scylladb-manager-task-name-override"
	ScyllaDBManagerTaskScheduleIntervalOverrideAnnotation          = "internal.scylla-operator.scylladb.com/scylladb-manager-task-schedule-interval-override"
	ScyllaDBManagerTaskScheduleStartDateOverrideAnnotation         = "internal.scylla-operator.scylladb.com/scylladb-manager-task-schedule-start-date-override"
	ScyllaDBManagerTaskScheduleTimezoneOverrideAnnotation          = "internal.scylla-operator.scylladb.com/scylladb-manager-task-schedule-timezone-override"
	ScyllaDBManagerTaskBackupDCNoValidateAnnotation                = "internal.scylla-operator.scylladb.com/scylladb-manager-task-backup-dc-no-validate"
	ScyllaDBManagerTaskBackupKeyspaceNoValidateAnnotation          = "internal.scylla-operator.scylladb.com/scylladb-manager-task-backup-keyspace-no-validate"
	ScyllaDBManagerTaskBackupLocationNoValidateAnnotation          = "internal.scylla-operator.scylladb.com/scylladb-manager-task-backup-location-no-validate"
	ScyllaDBManagerTaskBackupRateLimitNoValidateAnnotation         = "internal.scylla-operator.scylladb.com/scylladb-manager-task-backup-rate-limit-no-validate"
	ScyllaDBManagerTaskBackupRetentionNoValidateAnnotation         = "internal.scylla-operator.scylladb.com/scylladb-manager-task-backup-retention-no-validate"
	ScyllaDBManagerTaskBackupSnapshotParallelNoValidateAnnotation  = "internal.scylla-operator.scylladb.com/scylladb-manager-task-backup-snapshot-parallel-no-validate"
	ScyllaDBManagerTaskBackupUploadParallelNoValidateAnnotation    = "internal.scylla-operator.scylladb.com/scylladb-manager-task-backup-upload-parallel-no-validate"
	ScyllaDBManagerTaskRepairParallelNoValidateAnnotation          = "internal.scylla-operator.scylladb.com/scylladb-manager-task-repair-parallel-no-validate"
	ScyllaDBManagerTaskRepairDCNoValidateAnnotation                = "internal.scylla-operator.scylladb.com/scylladb-manager-task-repair-dc-no-validate"
	ScyllaDBManagerTaskRepairKeyspaceNoValidateAnnotation          = "internal.scylla-operator.scylladb.com/scylladb-manager-task-repair-keyspace-no-validate"
	ScyllaDBManagerTaskRepairIntensityOverrideAnnotation           = "internal.scylla-operator.scylladb.com/scylladb-manager-task-repair-intensity-override"
	ScyllaDBManagerTaskRepairSmallTableThresholdOverrideAnnotation = "internal.scylla-operator.scylladb.com/scylladb-manager-task-repair-small-table-threshold-override"
	ScyllaDBManagerTaskStatusAnnotation                            = "internal.scylla-operator.scylladb.com/scylladb-manager-task-status"
)

const (
	// ScyllaDBDatacenterNodesStatusReportSelectorLabel is used to uniformly label nodes status reports created for a ScyllaDB cluster.
	// It allows easy selection of all datacenter reports for a given cluster.
	ScyllaDBDatacenterNodesStatusReportSelectorLabel = "scylla-operator.scylladb.com/scylladb-datacenter-nodes-status-report-selector"

	// ForceProceedToBootstrapAnnotation allows to force the bootstrap barrier to proceed without waiting for normal preconditions to be satisfied.
	// If set to "true", bootstrap barrier will proceed to bootstrap immediately.
	// This can be used on a particular member Service to override the normal bootstrap precondition checks, or on the entire ScyllaDBDatacenter to override the default behavior for all members.
	ForceProceedToBootstrapAnnotation = "scylla-operator.scylladb.com/force-proceed-to-bootstrap"
)
