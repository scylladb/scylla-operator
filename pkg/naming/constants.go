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

type ScyllaIngressType string

const (
	ScyllaIngressTypeNode    ScyllaIngressType = "Node"
	ScyllaIngressTypeAnyNode ScyllaIngressType = "AnyNode"
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
	CleanupContainerName            = "cleanup"
	RLimitsContainerName            = "rlimits"

	PVCTemplateName = "data"

	SharedDirName = "/mnt/shared"

	ScyllaConfigDirName          = "/mnt/scylla-config"
	ScyllaAgentConfigDirName     = "/mnt/scylla-agent-config"
	ScyllaAgentConfigFileName    = "scylla-manager-agent.yaml"
	ScyllaAgentAuthTokenFileName = "auth-token.yaml"
	ScyllaAgentConfigDefaultFile = "/etc/scylla-manager-agent/scylla-manager-agent.yaml"
	ScyllaClientConfigDirName    = "/mnt/scylla-client-config"
	ScyllaDBManagedConfigDir     = "/var/run/configmaps/scylla-operator.scylladb.com/scylladb/managed-config"
	ScyllaDBSnitchConfigDir      = "/var/run/configmaps/scylla-operator.scylladb.com/scylladb/snitch-config"
	ScyllaConfigName             = "scylla.yaml"
	ScyllaDBManagedConfigName    = "scylladb-managed-config.yaml"
	ScyllaManagedConfigPath      = ScyllaDBManagedConfigDir + "/" + ScyllaDBManagedConfigName
	ScyllaRackDCPropertiesName   = "cassandra-rackdc.properties"
	ScyllaIOPropertiesName       = "io_properties.yaml"

	ScyllaDBIgnitionDonePath = SharedDirName + "/ignition.done"

	DataDir = "/var/lib/scylla"

	ReadinessProbePath         = "/readyz"
	LivenessProbePath          = "/healthz"
	ScyllaDBAPIStatusProbePort = 8080
	ScyllaDBIgnitionProbePort  = 42081
	ScyllaAPIPort              = 10000

	OperatorEnvVarPrefix = "SCYLLA_OPERATOR_"
)

const (
	ScyllaManagerNamespace      = "scylla-manager"
	ScyllaManagerServiceName    = "scylla-manager"
	ScyllaManagerDeploymentName = "scylla-manager"

	ScyllaOperatorNodeTuningNamespace = "scylla-operator-node-tuning"

	ScyllaClusterMemberClusterRoleName = "scyllacluster-member"

	SingletonName = "cluster"

	PerftuneJobPrefixName        = "perftune"
	PerftuneServiceAccountName   = "perftune"
	RlimitsJobServiceAccountName = "rlimits"
)

type NodeConfigJobType string

const (
	NodeConfigJobTypeNode                    NodeConfigJobType = "Node"
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
	ParentClusterNameLabel           = "scylla-operator.scylladb.com/parent-scylladbcluster-name"
	ParentClusterNamespaceLabel      = "scylla-operator.scylladb.com/parent-scylladbcluster-namespace"
	ParentClusterDatacenterNameLabel = "scylla-operator.scylladb.com/parent-scylladbcluster-datacenter-name"
)

const (
	RemoteOwnerClusterLabel   = "internal.scylla-operator.scylladb.com/remote-owner-cluster"
	RemoteOwnerNamespaceLabel = "internal.scylla-operator.scylladb.com/remote-owner-namespace"
	RemoteOwnerNameLabel      = "internal.scylla-operator.scylladb.com/remote-owner-name"
	RemoteOwnerGVR            = "internal.scylla-operator.scylladb.com/remote-owner-gvr"
)

const (
	ClusterEndpointsLabel = "scylla-operator.scylladb.com/cluster-endpoints"
)

const (
	KubeConfigSecretKey string = "kubeconfig"
)

const (
	OperatorAppNameWithDomain       = "scylla-operator.scylladb.com"
	RemoteOperatorAppNameWithDomain = "remote.scylla-operator.scylladb.com"
)

const (
	RemoteKubernetesClusterFinalizer = "scylla-operator.scylladb.com/remotekubernetescluster-protection"
	ScyllaDBClusterFinalizer         = "scylla-operator.scylladb.com/scylladbcluster-protection"
)

const (
	KubernetesManagedByLabel = "app.kubernetes.io/managed-by"
)

const (
	GlobalScyllaDBManagerRegistrationLabel = "scylla-operator.scylladb.com/register-with-global-scylladb-manager"
	GlobalScyllaDBManagerLabel             = "internal.scylla-operator.scylladb.com/global-scylladb-manager"

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
)
