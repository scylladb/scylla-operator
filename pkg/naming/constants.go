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
	// SeedLabel determines if a member is a seed or not.
	SeedLabel = "scylla/seed"

	// DecommissionLabel expresses the intent to decommission
	// the specific member. The presence of the label expresses
	// the intent to decommission. If the value is true, it means
	// the member has finished decommissioning.
	// Values: {true, false}
	DecommissionLabel = "scylla/decommissioned"

	LabelValueTrue  = "true"
	LabelValueFalse = "false"
)

// Generic Labels used on objects created by the operator.
const (
	ClusterNameLabel    = "scylla/cluster"
	DatacenterNameLabel = "scylla/datacenter"
	RackNameLabel       = "scylla/rack"

	AppName         = "scylla"
	OperatorAppName = "scylla-operator"

	PrometheusScrapeAnnotation = "prometheus.io/scrape"
	PrometheusPortAnnotation   = "prometheus.io/port"
)

// Environment Variables
const (
	EnvVarEnvVarPodName = "POD_NAME"
	EnvVarPodNamespace  = "POD_NAMESPACE"
	EnvVarCPU           = "CPU"
)

// Recorder Values
const (
	// SuccessSynced is used as part of the Event 'reason' when a Cluster is
	// synced.
	SuccessSynced = "Synced"
	// ErrResourceExists is used as part of the Event 'reason' when a
	// Cluster fails to sync due to a resource of the same name already
	// existing.
	ErrSyncFailed = "ErrSyncFailed"
)

// Configuration Values
const (
	ScyllaContainerName = "scylla"

	PVCTemplateName = "data"

	SharedDirName = "/mnt/shared"

	ScyllaConfigDirName        = "/mnt/scylla-config"
	ScyllaAgentConfigDirName   = "/mnt/scylla-agent-config"
	ScyllaConfigName           = "scylla.yaml"
	ScyllaRackDCPropertiesName = "cassandra-rackdc.properties"

	DataDir = "/var/lib/scylla"

	ReadinessProbePath = "/readyz"
	LivenessProbePath  = "/healthz"
	ProbePort          = 8080
)
