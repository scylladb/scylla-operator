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

// The modes ScyllaDB's storage service reports.
// NONE is reported as STARTING.
const (
	OperationalModeStarting       OperationalMode = "STARTING"
	OperationalModeJoining        OperationalMode = "JOINING"
	OperationalModeBootstrap      OperationalMode = "BOOTSTRAP"
	OperationalModeNormal         OperationalMode = "NORMAL"
	OperationalModeLeaving        OperationalMode = "LEAVING"
	OperationalModeDecommissioned OperationalMode = "DECOMMISSIONED"
	OperationalModeMoving         OperationalMode = "MOVING"
	OperationalModeDraining       OperationalMode = "DRAINING"
	OperationalModeDrained        OperationalMode = "DRAINED"
	OperationalModeMaintenance    OperationalMode = "MAINTENANCE"
	OperationalModeUnknown        OperationalMode = "UNKNOWN"
)

var (
	operationalModeMap = map[string]OperationalMode{
		"STARTING":       OperationalModeStarting,
		"JOINING":        OperationalModeJoining,
		"BOOTSTRAP":      OperationalModeBootstrap,
		"NORMAL":         OperationalModeNormal,
		"LEAVING":        OperationalModeLeaving,
		"DECOMMISSIONED": OperationalModeDecommissioned,
		"MOVING":         OperationalModeMoving,
		"DRAINING":       OperationalModeDraining,
		"DRAINED":        OperationalModeDrained,
		"MAINTENANCE":    OperationalModeMaintenance,
	}
)

func (o OperationalMode) String() string {
	if _, ok := operationalModeMap[string(o)]; ok {
		return string(o)
	}
	return "UNKNOWN"
}

func operationalModeFromString(str string) OperationalMode {
	if om, ok := operationalModeMap[strings.ToUpper(str)]; ok {
		return om
	}
	return OperationalModeUnknown
}

type CompactionType string

const (
	CleanupCompactionType CompactionType = "CLEANUP"
)

// NodeStatusAndStateInfo represents a node's status and state (like in nodetool status).
type NodeStatusAndStateInfo struct {
	NodeStatusInfo
	State NodeState
}

func (s NodeStatusAndStateInfo) String() string {
	return fmt.Sprintf("host: %s, Status: %s%s", s.Addr, s.Status, s.State)
}

// IsUN returns true if host is Up and NORMAL meaning it's a fully functional
// live node.
func (s NodeStatusAndStateInfo) IsUN() bool {
	return s.Status == NodeStatusUp && s.State == NodeStateNormal
}

// NodeStatusAndStateInfoSlice adds functionality to Status response.
type NodeStatusAndStateInfoSlice []NodeStatusAndStateInfo

// Hosts returns slice of address of all nodes.
func (s NodeStatusAndStateInfoSlice) Hosts() []string {
	var hosts []string
	for _, h := range s {
		hosts = append(hosts, h.Addr)
	}
	return hosts
}

// HostIDs returns slice of HostID of all nodes.
func (s NodeStatusAndStateInfoSlice) HostIDs() []string {
	var hostIDs []string
	for _, h := range s {
		hostIDs = append(hostIDs, h.HostID)
	}
	return hostIDs
}

// LiveHosts returns slice of address of nodes in UN state.
func (s NodeStatusAndStateInfoSlice) LiveHosts() []string {
	var hosts []string
	for _, h := range s {
		if h.IsUN() {
			hosts = append(hosts, h.Addr)
		}
	}
	return hosts
}

// DownHosts returns slice of address of nodes that are down.
func (s NodeStatusAndStateInfoSlice) DownHosts() []string {
	var hosts []string
	for _, h := range s {
		if h.Status == NodeStatusDown {
			hosts = append(hosts, h.Addr)
		}
	}
	return hosts
}

// DownHostIDs returns slice of HostID of nodes that are down.
func (s NodeStatusAndStateInfoSlice) DownHostIDs() []string {
	var hostIDs []string
	for _, h := range s {
		if h.Status == NodeStatusDown {
			hostIDs = append(hostIDs, h.HostID)
		}
	}
	return hostIDs
}

// NodeStatusInfo represents the status (Up/Down) of a node.
type NodeStatusInfo struct {
	HostID string
	Addr   string
	Status NodeStatus
}

type NodeStatusInfoSlice []NodeStatusInfo

// UpHostIDs returns slice of HostID of nodes that are up.
func (s NodeStatusInfoSlice) UpHostIDs() []string {
	var hosts []string
	for _, h := range s {
		if h.Status == NodeStatusUp {
			hosts = append(hosts, h.Addr)
		}
	}
	return hosts
}

// HostIDs returns slice of HostID of all nodes.
func (s NodeStatusInfoSlice) HostIDs() []string {
	var hostIDs []string
	for _, h := range s {
		hostIDs = append(hostIDs, h.HostID)
	}
	return hostIDs
}

// DownHostIDs returns slice of HostID of nodes that are down.
func (s NodeStatusInfoSlice) DownHostIDs() []string {
	var hosts []string
	for _, h := range s {
		if h.Status == NodeStatusDown {
			hosts = append(hosts, h.HostID)
		}
	}
	return hosts
}
