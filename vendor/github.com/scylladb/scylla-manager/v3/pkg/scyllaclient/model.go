// Copyright (C) 2017 ScyllaDB

package scyllaclient

import (
	"reflect"

	"github.com/gocql/gocql"
	"github.com/scylladb/go-set/strset"
	"github.com/scylladb/gocqlx/v2"
	"github.com/scylladb/scylla-manager/v3/pkg/util/slice"
)

// NodeStatus represents nodetool Status=Up/Down.
type NodeStatus bool

// NodeStatus enumeration.
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

// NodeState represents nodetool State=Normal/Leaving/Joining/Moving.
type NodeState string

// NodeState enumeration.
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

// NodeStatusInfo represents a nodetool status line.
type NodeStatusInfo struct {
	Datacenter string
	HostID     string
	Addr       string
	Status     NodeStatus
	State      NodeState
}

// IsUN returns true if host is Up and NORMAL meaning it's a fully functional
// live node.
func (s NodeStatusInfo) IsUN() bool {
	return s.Status == NodeStatusUp && s.State == NodeStateNormal
}

// NodeStatusInfoSlice adds functionality to Status response.
type NodeStatusInfoSlice []NodeStatusInfo

// Datacenter returns sub slice containing only nodes from given datacenters.
func (s NodeStatusInfoSlice) Datacenter(dcs []string) NodeStatusInfoSlice {
	m := strset.New(dcs...)
	return s.filter(func(i int) bool {
		return m.Has(s[i].Datacenter)
	})
}

// DatacenterMap returns dc to nodes mapping.
func (s NodeStatusInfoSlice) DatacenterMap(dc []string) map[string][]string {
	dcMap := make(map[string][]string)
	for _, h := range s {
		if slice.ContainsString(dc, h.Datacenter) {
			dcMap[h.Datacenter] = append(dcMap[h.Datacenter], h.Addr)
		}
	}
	return dcMap
}

// HostDC returns node to dc mapping.
func (s NodeStatusInfoSlice) HostDC() map[string]string {
	hostDC := make(map[string]string)
	for _, h := range s {
		hostDC[h.Addr] = h.Datacenter
	}
	return hostDC
}

// Up returns sub slice containing only nodes with status up.
func (s NodeStatusInfoSlice) Up() NodeStatusInfoSlice {
	return s.filter(func(i int) bool {
		return s[i].Status == NodeStatusUp
	})
}

// Down returns sub slice containing only nodes with status down.
func (s NodeStatusInfoSlice) Down() NodeStatusInfoSlice {
	return s.filter(func(i int) bool {
		return s[i].Status == NodeStatusDown
	})
}

// State returns sub slice containing only nodes in a given state.
func (s NodeStatusInfoSlice) State(state NodeState) NodeStatusInfoSlice {
	return s.filter(func(i int) bool {
		return s[i].State == state
	})
}

// Live returns sub slice of nodes in UN state.
func (s NodeStatusInfoSlice) Live() NodeStatusInfoSlice {
	return s.filter(func(i int) bool {
		return s[i].IsUN()
	})
}

func (s NodeStatusInfoSlice) filter(f func(i int) bool) NodeStatusInfoSlice {
	var filtered NodeStatusInfoSlice
	for i, h := range s {
		if f(i) {
			filtered = append(filtered, h)
		}
	}
	return filtered
}

// HostIDs returns slice of IDs of all nodes.
func (s NodeStatusInfoSlice) HostIDs() []string {
	var ids []string
	for _, h := range s {
		ids = append(ids, h.HostID)
	}
	return ids
}

// Hosts returns slice of address of all nodes.
func (s NodeStatusInfoSlice) Hosts() []string {
	var hosts []string
	for _, h := range s {
		hosts = append(hosts, h.Addr)
	}
	return hosts
}

// CommandStatus specifies a result of a command.
type CommandStatus string

// Command statuses.
const (
	CommandRunning    CommandStatus = "RUNNING"
	CommandSuccessful CommandStatus = "SUCCESSFUL"
	CommandFailed     CommandStatus = "FAILED"
)

// ReplicationStrategy specifies type of keyspace replication strategy.
type ReplicationStrategy string

// Replication strategies.
const (
	LocalStrategy           = "org.apache.cassandra.locator.LocalStrategy"
	SimpleStrategy          = "org.apache.cassandra.locator.SimpleStrategy"
	NetworkTopologyStrategy = "org.apache.cassandra.locator.NetworkTopologyStrategy"
)

// Ring describes token ring of a keyspace.
type Ring struct {
	ReplicaTokens []ReplicaTokenRanges
	HostDC        map[string]string
	Replication   ReplicationStrategy
	RF            int
	DCrf          map[string]int // initialized only for NetworkTopologyStrategy
}

// Datacenters returns a list of datacenters the keyspace is replicated in.
func (r Ring) Datacenters() []string {
	v := strset.NewWithSize(len(r.HostDC))
	for _, dc := range r.HostDC {
		v.Add(dc)
	}
	return v.List()
}

// TokenRange describes the beginning and end of a token range.
type TokenRange struct {
	StartToken int64 `db:"start_token"`
	EndToken   int64 `db:"end_token"`
}

func (t TokenRange) MarshalUDT(name string, info gocql.TypeInfo) ([]byte, error) {
	f := gocqlx.DefaultMapper.FieldByName(reflect.ValueOf(t), name)
	return gocql.Marshal(info, f.Interface())
}

func (t *TokenRange) UnmarshalUDT(name string, info gocql.TypeInfo, data []byte) error {
	f := gocqlx.DefaultMapper.FieldByName(reflect.ValueOf(t), name)
	return gocql.Unmarshal(info, data, f.Addr().Interface())
}

// ReplicaTokenRanges describes all token ranges belonging to given replica set.
type ReplicaTokenRanges struct {
	ReplicaSet []string     // Sorted lexicographically
	Ranges     []TokenRange // Sorted by start token
}

// Unit describes keyspace and some tables in that keyspace.
type Unit struct {
	Keyspace string
	Tables   []string
}

// ViewBuildStatus defines build status of a view.
type ViewBuildStatus string

// ViewBuildStatus enumeration.
const (
	StatusUnknown ViewBuildStatus = "UNKNOWN"
	StatusStarted ViewBuildStatus = "STARTED"
	StatusSuccess ViewBuildStatus = "SUCCESS"
)

// ViewBuildStatusOrder lists all view build statuses in the order of their execution.
func ViewBuildStatusOrder() []ViewBuildStatus {
	return []ViewBuildStatus{
		StatusUnknown,
		StatusStarted,
		StatusSuccess,
	}
}

// Index returns status position in ViewBuildStatusOrder.
func (s ViewBuildStatus) Index() int {
	return slice.Index(ViewBuildStatusOrder(), s)
}
