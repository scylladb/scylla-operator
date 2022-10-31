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

	// NodeMaintenanceLabel means that node is under maintenance.
	// Readiness check will always fail when this label is added to member service.
	NodeMaintenanceLabel = "scylla/node-maintenance"

	LabelValueTrue  = "true"
	LabelValueFalse = "false"
)

// Annotations used internally by sidecar controller.
const (
	// HostIDAnnotation reflects the host_id of the scylla node.
	HostIDAnnotation = "internal.scylla-operator.scylladb.com/host-id"
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
	ClusterNameLabel             = "scylla/cluster"
	DatacenterNameLabel          = "scylla/datacenter"
	RackNameLabel                = "scylla/rack"
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

	AppName           = "scylla"
	OperatorAppName   = "scylla-operator"
	ManagerAppName    = "scylla-manager"
	NodeConfigAppName = "scylla-node-config"

	PrometheusScrapeAnnotation = "prometheus.io/scrape"
	PrometheusPortAnnotation   = "prometheus.io/port"

	ForceRedeploymentReasonAnnotation = "scylla-operator.scylladb.com/force-redeployment-reason"
)

const (
	NodeConfigJobForNodeKey = "scylla-operator.scylladb.com/node-config-job-for-node"
)

// Configuration Values
const (
	ScyllaContainerName          = "scylla"
	SidecarInjectorContainerName = "sidecar-injection"
	PerftuneContainerName        = "perftune"

	PVCTemplateName = "data"

	SharedDirName = "/mnt/shared"

	ScyllaConfigDirName          = "/mnt/scylla-config"
	ScyllaAgentConfigDirName     = "/mnt/scylla-agent-config"
	ScyllaAgentConfigFileName    = "scylla-manager-agent.yaml"
	ScyllaAgentAuthTokenFileName = "auth-token.yaml"
	ScyllaAgentConfigDefaultFile = "/etc/scylla-manager-agent/scylla-manager-agent.yaml"
	ScyllaClientConfigDirName    = "/mnt/scylla-client-config"
	ScyllaConfigName             = "scylla.yaml"
	ScyllaRackDCPropertiesName   = "cassandra-rackdc.properties"
	ScyllaIOPropertiesName       = "io_properties.yaml"

	DataDir = "/var/lib/scylla"

	ReadinessProbePath = "/readyz"
	LivenessProbePath  = "/healthz"
	ProbePort          = 8080
	ScyllaAPIPort      = 10000

	OperatorEnvVarPrefix = "SCYLLA_OPERATOR_"
)

const (
	ScyllaManagerNamespace   = "scylla-manager"
	ScyllaManagerServiceName = "scylla-manager"

	ScyllaOperatorNodeTuningNamespace = "scylla-operator-node-tuning"

	ScyllaClusterMemberClusterRoleName = "scyllacluster-member"

	SingletonName = "cluster"

	PerftuneJobPrefixName = "perftune"

	// TODO: Make sure this doesn't get out of date.
	DefaultScyllaUtilsImage = "docker.io/scylladb/scylla:4.6.3"
)

type NodeConfigJobType string

const (
	NodeConfigJobTypeNode       NodeConfigJobType = "Node"
	NodeConfigJobTypeContainers NodeConfigJobType = "Containers"
)

type ConfigMapType string

const (
	NodeConfigDataConfigMapType ConfigMapType = "NodeConfigData"
)

const (
	ScyllaRuntimeConfigKey string = "ScyllaRuntimeConfig"
)
