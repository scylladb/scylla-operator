package transport

import (
	"context"
	"fmt"

	"github.com/scylladb/scylla-go-driver/frame"
	"go.uber.org/atomic"
)

type nodeStatus = atomic.Bool

const (
	statusDown = false
	statusUP   = true
)

type Node struct {
	hostID     frame.UUID
	addr       string
	datacenter string
	rack       string
	pool       *ConnPool
	status     nodeStatus
}

func (n *Node) Status() bool {
	return n.status.Load()
}

func (n *Node) setStatus(v bool) {
	n.status.Store(v)
}

func (n *Node) LeastBusyConn() *Conn {
	return n.pool.LeastBusyConn()
}
func (n *Node) Conn(qi QueryInfo) *Conn {
	if qi.tokenAware {
		return n.pool.Conn(qi.token)
	}

	return n.LeastBusyConn()
}

func (n *Node) Prepare(ctx context.Context, s Statement) (Statement, error) {
	return n.LeastBusyConn().Prepare(ctx, s)
}

var versionQuery = Statement{
	Content:     "SELECT schema_version FROM system.local WHERE key='local'",
	Consistency: frame.ONE,
}

func (n *Node) FetchSchemaVersion(ctx context.Context) (frame.UUID, error) {
	conn := n.LeastBusyConn()
	res, err := conn.Query(ctx, versionQuery, nil)
	if err != nil {
		return frame.UUID{}, err
	}

	if len(res.Rows) < 1 {
		return frame.UUID{}, fmt.Errorf("schema_version query returned no rows")
	}

	if len(res.Rows[0]) < 1 {
		return frame.UUID{}, fmt.Errorf("schema_version query returned an empty row")
	}

	version, err := res.Rows[0][0].AsUUID()
	if err != nil {
		return version, fmt.Errorf("parsing schema_version: %w", err)
	}

	return version, nil
}

type RingEntry struct {
	node           *Node
	token          Token
	localReplicas  []*Node
	remoteReplicas []*Node
}

func (r RingEntry) Less(i RingEntry) bool {
	return r.token < i.token
}

type Ring []RingEntry

func (r Ring) Less(i, j int) bool { return r[i].token < r[j].token }
func (r Ring) Len() int           { return len(r) }
func (r Ring) Swap(i, j int)      { r[i], r[j] = r[j], r[i] }

// Iterator over all nodes starting from offset.
type replicaIter struct {
	ring    Ring
	offset  int
	fetched int
}

func (r *replicaIter) Next() *Node {
	if r.fetched >= len(r.ring) {
		return nil
	}

	ret := r.ring[r.offset].node
	r.offset++
	r.fetched++
	if r.offset >= len(r.ring) {
		r.offset = 0
	}

	return ret
}

// tokenLowerBound returns the position of first node with token larger than given, 0 if there wasn't one.
func (r Ring) tokenLowerBound(token Token) int {
	start, end := 0, len(r)
	for start < end {
		mid := int(uint(start+end) >> 1)
		if r[mid].token < token {
			start = mid + 1
		} else {
			end = mid
		}
	}

	if end >= len(r) {
		end = 0
	}

	return end
}
