package scyllaclient

import (
	"fmt"
	"strings"
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

type CompactionType string

const (
	CompactionCompactionType CompactionType = "COMPACTION"
	CleanupCompactionType    CompactionType = "CLEANUP"
	ScrubCompactionType      CompactionType = "SCRUB"
	UpgradeCompactionType    CompactionType = "UPGRADE"
	ReshapeCompactionType    CompactionType = "RESHAPE"
)

// NodeStatusInfo represents a nodetool status line.
type NodeStatusInfo struct {
	HostID string
	Addr   string
	Status NodeStatus
	State  NodeState
}

func (s NodeStatusInfo) String() string {
	return fmt.Sprintf("host: %s, Status: %s%s", s.Addr, s.Status, s.State)
}

// IsUN returns true if host is Up and NORMAL meaning it's a fully functional
// live node.
func (s NodeStatusInfo) IsUN() bool {
	return s.Status == NodeStatusUp && s.State == NodeStateNormal
}

// NodeStatusInfoSlice adds functionality to Status response.
type NodeStatusInfoSlice []NodeStatusInfo

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
