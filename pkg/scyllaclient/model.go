package scyllaclient

import (
	"fmt"
	"strings"

	"github.com/hashicorp/go-version"
	"github.com/scylladb/go-set/strset"
)

// NodeStatus represents nodetool Status=Up/Down.
type NodeStatus bool

// NodeStatus enumeration
const (
	NodeStatusUp   NodeStatus = true
	NodeStatusDown NodeStatus = false
)

func (s NodeStatus) String() string {
	if s {
		return "U"
	}
	return "D"
}

// NodeState represents nodetool State=Normal/Leaving/Joining/Moving
type NodeState string

// NodeState enumeration
const (
	NodeStateNormal  NodeState = ""
	NodeStateLeaving NodeState = "LEAVING"
	NodeStateJoining NodeState = "JOINING"
	NodeStateMoving  NodeState = "MOVING"
)

func (s NodeState) String() string {
	switch s {
	case NodeStateNormal:
		return "N"
	case NodeStateLeaving:
		return "L"
	case NodeStateJoining:
		return "J"
	case NodeStateMoving:
		return "M"
	}
	return ""
}

type OperationalMode string

const (
	OperationalModeClient          OperationalMode = "CLIENT"
	OperationalModeDecommissioned  OperationalMode = "DECOMMISSIONED"
	OperationalModeDecommissioning OperationalMode = "DECOMMISSIONING"
	OperationalModeJoining         OperationalMode = "JOINING"
	OperationalModeLeaving         OperationalMode = "LEAVING"
	OperationalModeNormal          OperationalMode = "NORMAL"
	OperationalModeDrained         OperationalMode = "DRAINED"
	OperationalModeDraining        OperationalMode = "DRAINING"
	OperationalModeUnknown         OperationalMode = "UNKNOWN"
)

var (
	operationalModeMap = map[string]OperationalMode{
		"CLIENT":          OperationalModeClient,
		"DECOMMISSIONED":  OperationalModeDecommissioned,
		"DECOMMISSIONING": OperationalModeDecommissioning,
		"JOINING":         OperationalModeJoining,
		"LEAVING":         OperationalModeLeaving,
		"DRAINED":         OperationalModeDrained,
		"DRAINING":        OperationalModeDraining,
		"NORMAL":          OperationalModeNormal,
	}
)

func (o OperationalMode) String() string {
	switch o {
	case "CLIENT", "DECOMMISSIONED", "DECOMMISSIONING", "JOINING", "LEAVING", "NORMAL", "DRAINED", "DRAINING":
		return string(o)
	default:
		return "UNKNOWN"
	}
}

func (o OperationalMode) IsDecommissioned() bool {
	return o == OperationalModeDecommissioned
}

func (o OperationalMode) IsDecommissioning() bool {
	return o == OperationalModeDecommissioned
}

func (o OperationalMode) IsLeaving() bool {
	return o == OperationalModeLeaving
}

func (o OperationalMode) IsNormal() bool {
	return o == OperationalModeNormal
}

func (o OperationalMode) IsJoining() bool {
	return o == OperationalModeJoining
}

func (o OperationalMode) IsDrained() bool {
	return o == OperationalModeDrained
}

func (o OperationalMode) IsDraining() bool {
	return o == OperationalModeDraining
}

func operationalModeFromString(str string) OperationalMode {
	if om, ok := operationalModeMap[strings.ToUpper(str)]; ok {
		return om
	}
	return OperationalModeUnknown
}

// NodeStatusInfo represents a nodetool status line.
type NodeStatusInfo struct {
	Datacenter string
	HostID     string
	Addr       string
	Status     NodeStatus
	State      NodeState
}

func (s NodeStatusInfo) String() string {
	return fmt.Sprintf("host: %s, DC: %s, Status: %s%s", s.Addr, s.Datacenter, s.Status, s.State)
}

// IsUN returns true if host is Up and NORMAL meaning it's a fully functional
// live node.
func (s NodeStatusInfo) IsUN() bool {
	return s.Status == NodeStatusUp && s.State == NodeStateNormal
}

// NodeStatusInfoSlice adds functionality to Status response.
type NodeStatusInfoSlice []NodeStatusInfo

// Datacenter resturns sub slice containing only nodes from given datacenters.
func (s NodeStatusInfoSlice) Datacenter(dcs []string) NodeStatusInfoSlice {
	m := strset.New(dcs...)

	var filtered NodeStatusInfoSlice
	for _, h := range s {
		if m.Has(h.Datacenter) {
			filtered = append(filtered, h)
		}
	}
	return filtered
}

// Hosts returns slice of address of all nodes.
func (s NodeStatusInfoSlice) Hosts() []string {
	var hosts []string
	for _, h := range s {
		hosts = append(hosts, h.Addr)
	}
	return hosts
}

// LiveHosts returns slice of address of nodes in UN state.
func (s NodeStatusInfoSlice) LiveHosts() []string {
	var hosts []string
	for _, h := range s {
		if h.IsUN() {
			hosts = append(hosts, h.Addr)
		}
	}
	return hosts
}

// DownHosts returns slice of address of nodes that are down.
func (s NodeStatusInfoSlice) DownHosts() []string {
	var hosts []string
	for _, h := range s {
		if h.Status == NodeStatusDown {
			hosts = append(hosts, h.Addr)
		}
	}
	return hosts
}

// CommandStatus specifies a result of a command
type CommandStatus string

// Command statuses
const (
	CommandRunning    CommandStatus = "RUNNING"
	CommandSuccessful CommandStatus = "SUCCESSFUL"
	CommandFailed     CommandStatus = "FAILED"
)

// Partitioners
const (
	Murmur3Partitioner = "org.apache.cassandra.dht.Murmur3Partitioner"
)

// ReplicationStrategy specifies type of a keyspace replication strategy.
type ReplicationStrategy string

// Replication strategies
const (
	LocalStrategy           = "org.apache.cassandra.locator.LocalStrategy"
	SimpleStrategy          = "org.apache.cassandra.locator.SimpleStrategy"
	NetworkTopologyStrategy = "org.apache.cassandra.locator.NetworkTopologyStrategy"
)

// Ring describes token ring of a keyspace.
type Ring struct {
	Tokens      []TokenRange
	HostDC      map[string]string
	Replication ReplicationStrategy
}

// Datacenters returs a list of datacenters the keyspace is replicated in.
func (r Ring) Datacenters() []string {
	v := strset.NewWithSize(len(r.HostDC))
	for _, dc := range r.HostDC {
		v.Add(dc)
	}
	return v.List()
}

// TokenRange describes replicas of a token (range).
type TokenRange struct {
	StartToken int64
	EndToken   int64
	Replicas   []string
}

// Unit describes keyspace and some tables in that keyspace.
type Unit struct {
	Keyspace string
	Tables   []string
}

// ScyllaFeatures specifies features supported by the Scylla version.
type ScyllaFeatures struct {
	RowLevelRepair bool
}

var masterScyllaFeatures = ScyllaFeatures{
	RowLevelRepair: true,
}

const (
	scyllaMasterVersion           = "666.development"
	scyllaEnterpriseMasterVersion = "9999.enterprise_dev"
)

func makeScyllaFeatures(ver string) (ScyllaFeatures, error) {
	// Trim build version suffix as it breaks constraints
	ver = strings.Split(ver, "-")[0]

	// Detect master builds
	if ver == scyllaMasterVersion || ver == scyllaEnterpriseMasterVersion {
		return masterScyllaFeatures, nil
	}

	v, err := version.NewSemver(ver)
	if err != nil {
		return ScyllaFeatures{}, err
	}

	rowLevelRepair, err := version.NewConstraint(">= 3.1, < 2000")
	if err != nil {
		panic(err) // must
	}
	return ScyllaFeatures{
		RowLevelRepair: rowLevelRepair.Check(v),
	}, nil
}
