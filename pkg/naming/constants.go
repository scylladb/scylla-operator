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

// Generic Labels used on objects created by the operator.
const (
	ClusterNameLabel          = "scylla/cluster"
	DatacenterNameLabel       = "scylla/datacenter"
	RackNameLabel             = "scylla/rack"
	ScyllaVersionLabel        = "scylla/scylla-version"
	ManagedHash               = "scylla-operator.scylladb.com/managed-hash"
	NodeConfigNameLabel       = "scylla-operator.scylladb.com/node-config-name"
	NodeConfigControllerLabel = "scylla-operator.scylladb.com/node-config-controller"
	NodePoolLabel             = "scylla-operator.scylladb.com/node-pool"

	AppName           = "scylla"
	OperatorAppName   = "scylla-operator"
	ManagerAppName    = "scylla-manager"
	NodeConfigAppName = "scylla-node-config"

	PrometheusScrapeAnnotation = "prometheus.io/scrape"
	PrometheusPortAnnotation   = "prometheus.io/port"

	ForceRedeploymentReasonAnnotation = "scylla-operator.scylladb.com/force-redeployment-reason"
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
	ScyllaClientConfigFileName   = "scylla-client.yaml"
	ScyllaConfigName             = "scylla.yaml"
	ScyllaRackDCPropertiesName   = "cassandra-rackdc.properties"
	ScyllaIOPropertiesName       = "io_properties.yaml"
	PerftuneCommandName          = "perftune.cmd"
	HostFilesystemDirName        = "/mnt/hostfs"

	DataDir = "/var/lib/scylla"

	ReadinessProbePath = "/readyz"
	LivenessProbePath  = "/healthz"
	ProbePort          = 8080
	MetricsPort        = 8081

	OperatorEnvVarPrefix = "SCYLLA_OPERATOR_"
)

const (
	ScyllaManagerNamespace   = "scylla-manager"
	ScyllaManagerServiceName = "scylla-manager"

	ScyllaOperatorName      = "scylla-operator"
	ScyllaOperatorNamespace = "scylla-operator"

	ScyllaOperatorNodeTuningNamespace = "scylla-operator-node-tuning"

	PerftuneJobPrefixName = "perftune"
)
