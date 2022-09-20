package transport

import (
	"context"
	"fmt"
	"log"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/scylladb/scylla-go-driver/frame"
	. "github.com/scylladb/scylla-go-driver/frame/response"

	"go.uber.org/atomic"
)

type (
	peerMap    = map[string]*Node
	dcRacksMap = map[string]int
	dcRFMap    = map[string]uint32
	ksMap      = map[string]keyspace

	requestChan chan struct{}
)

type Cluster struct {
	topology          atomic.Value // *topology
	control           *Conn
	cfg               ConnConfig
	handledEvents     []frame.EventType // This will probably be moved to config.
	knownHosts        map[string]struct{}
	refreshChan       requestChan
	reopenControlChan requestChan
	closeChan         requestChan

	queryInfoCounter atomic.Uint64
}

type topology struct {
	localDC    string
	peers      peerMap
	dcRacks    dcRacksMap
	Nodes      []*Node
	policyInfo policyInfo
	keyspaces  ksMap
}

type keyspace struct {
	strategy strategy
	// TODO: Add and use attributes below.
	// tables		      map[string]table
	// user_defined_types map[string](string, cqltype)
}

type strategyClass string

// JCN stands for Java Class Name.
const (
	networkTopologyStrategyJCN strategyClass = "org.apache.cassandra.locator.NetworkTopologyStrategy"
	simpleStrategyJCN          strategyClass = "org.apache.cassandra.locator.SimpleStrategy"
	localStrategyJCN           strategyClass = "org.apache.cassandra.locator.LocalStrategy"
	networkTopologyStrategy    strategyClass = "NetworkTopologyStrategy"
	simpleStrategy             strategyClass = "SimpleStrategy"
	localStrategy              strategyClass = "LocalStrategy"
)

type strategy struct {
	class strategyClass
	rf    uint32            // Used in simpleStrategy.
	dcRF  dcRFMap           // Used in networkTopologyStrategy.
	data  map[string]string // Used in other strategy.
}

// QueryInfo represents data required for host selection policy to create query plan.
// Token and strategy are only necessary for token aware policies.
type QueryInfo struct {
	tokenAware bool
	token      Token
	topology   *topology
	strategy   strategy
	offset     uint64 // For round robin strategies.
}

func (c *Cluster) NewQueryInfo() QueryInfo {
	return QueryInfo{
		tokenAware: false,
		topology:   c.Topology(),
		offset:     c.generateOffset(),
	}
}

func (c *Cluster) NewTokenAwareQueryInfo(t Token, ks string) (QueryInfo, error) {
	top := c.Topology()
	// When keyspace is not specified, we take default keyspace from ConnConfig.
	if ks == "" {
		if c.cfg.Keyspace == "" {
			// We don't know anything about the keyspace, fallback to non-token aware query.
			return c.NewQueryInfo(), nil
		}
		ks = c.cfg.Keyspace
	}
	if stg, ok := top.keyspaces[ks]; ok {
		return QueryInfo{
			tokenAware: true,
			token:      t,
			topology:   top,
			strategy:   stg.strategy,
			offset:     c.generateOffset(),
		}, nil
	} else {
		var allKs []string
		for k := range top.keyspaces {
			allKs = append(allKs, k)
		}
		sort.Strings(allKs)
		return QueryInfo{}, fmt.Errorf("couldn't find keyspace %q in current topology, known keyspaces are: %s", ks, strings.Join(allKs, ", "))
	}
}

// TODO overflow and negative modulo.
func (c *Cluster) generateOffset() uint64 {
	return c.queryInfoCounter.Inc() - 1
}

// NewCluster also creates control connection and starts handling events and refreshing topology.
func NewCluster(ctx context.Context, cfg ConnConfig, p HostSelectionPolicy, e []frame.EventType, hosts ...string) (*Cluster, error) {
	kh := make(map[string]struct{}, len(hosts))
	for _, h := range hosts {
		kh[h] = struct{}{}
	}

	c := &Cluster{
		cfg:               cfg,
		handledEvents:     e,
		knownHosts:        kh,
		refreshChan:       make(requestChan, 1),
		reopenControlChan: make(requestChan, 1),
		closeChan:         make(requestChan, 1),
	}

	localDC := ""
	if p, ok := p.(*TokenAwarePolicy); ok {
		localDC = p.localDC
	}
	c.setTopology(&topology{localDC: localDC})

	if control, err := c.NewControl(ctx); err != nil {
		return nil, fmt.Errorf("create control connection: %w", err)
	} else {
		c.control = control
	}
	if err := c.refreshTopology(ctx); err != nil {
		return nil, fmt.Errorf("refresh topology: %w", err)
	}

	go c.loop(ctx)
	return c, nil
}

func (c *Cluster) NewControl(ctx context.Context) (*Conn, error) {
	log.Printf("cluster: open control connection")
	var errs []string
	for addr := range c.knownHosts {
		conn, err := OpenConn(ctx, addr, nil, c.cfg)
		if err == nil {
			if err := conn.RegisterEventHandler(ctx, c.handleEvent, c.handledEvents...); err == nil {
				return conn, nil
			} else {
				errs = append(errs, fmt.Sprintf("%s failed to register for events: %s", conn, err))
			}
		} else {
			errs = append(errs, fmt.Sprintf("%s failed to connect: %s", addr, err))
		}
		if conn != nil {
			conn.Close()
		}
	}

	return nil, fmt.Errorf("couldn't open control connection to any known host:\n%s", strings.Join(errs, "\n"))
}

// refreshTopology creates new topology filled with the result of keyspaceQuery, localQuery and peerQuery.
// Old topology is replaced with the new one atomically to prevent dirty reads.
func (c *Cluster) refreshTopology(ctx context.Context) error {
	log.Printf("cluster: refresh topology")
	rows, err := c.getAllNodesInfo(ctx)
	if err != nil {
		return fmt.Errorf("query info about nodes in cluster: %w", err)
	}

	old := c.Topology().peers
	t := newTopology()
	t.localDC = c.Topology().localDC
	t.keyspaces, err = c.updateKeyspace(ctx)
	if err != nil {
		return fmt.Errorf("query keyspaces: %w", err)
	}

	type uniqueRack struct {
		dc   string
		rack string
	}
	u := make(map[uniqueRack]struct{})

	for _, r := range rows {
		n, err := c.parseNodeFromRow(r)
		if err != nil {
			return err
		}
		// If node is present in both maps we can reuse its connection pool.
		if node, ok := old[n.addr]; ok {
			n.pool = node.pool
			n.setStatus(node.Status())
		} else {
			if pool, err := NewConnPool(ctx, n.addr, c.cfg); err != nil {
				n.setStatus(statusDown)
			} else {
				n.setStatus(statusUP)
				n.pool = pool
			}
		}
		// Every encountered node becomes known host for future use.
		c.knownHosts[n.addr] = struct{}{}
		t.peers[n.addr] = n
		t.Nodes = append(t.Nodes, n)
		u[uniqueRack{dc: n.datacenter, rack: n.rack}] = struct{}{}
		if err := parseTokensFromRow(n, r, &t.policyInfo.ring); err != nil {
			return err
		}
	}
	// Counts unique racks in data centers.
	for k := range u {
		t.dcRacks[k.dc]++
	}
	// We want to close pools of nodes present in previous and absent in current topology.
	for k, v := range old {
		if _, ok := t.peers[k]; v.pool != nil && !ok {
			v.pool.Close()
		}
	}

	if ks, ok := t.keyspaces[c.cfg.Keyspace]; ok {
		t.policyInfo.Preprocess(t, ks)
	} else {
		t.policyInfo.Preprocess(t, keyspace{})
	}

	c.setTopology(t)
	drainChan(c.refreshChan)
	return nil
}

func newTopology() *topology {
	return &topology{
		peers:   make(peerMap),
		dcRacks: make(dcRacksMap),
		Nodes:   make([]*Node, 0),
		policyInfo: policyInfo{
			ring: make(Ring, 0),
		},
	}
}

var (
	peerQuery = Statement{
		Content:     "SELECT host_id, data_center, rack, tokens, rpc_address, preferred_ip, peer FROM system.peers",
		Consistency: frame.ONE,
	}

	localQuery = Statement{
		Content:     "SELECT host_id, data_center, rack, tokens, rpc_address, broadcast_address FROM system.local",
		Consistency: frame.ONE,
	}

	keyspaceQuery = Statement{
		Content:     "SELECT keyspace_name, replication FROM system_schema.keyspaces",
		Consistency: frame.ONE,
	}
)

const (
	hostIDIndex = 0
	dcIndex     = 1
	rackIndex   = 2
	tokensIndex = 3
	addrIndex   = 4

	ksNameIndex      = 0
	replicationIndex = 1
)

func (c *Cluster) getAllNodesInfo(ctx context.Context) ([]frame.Row, error) {
	peerRes, err := c.control.Query(ctx, peerQuery, nil)
	if err != nil {
		return nil, fmt.Errorf("discover peer topology: %w", err)
	}

	localRes, err := c.control.Query(ctx, localQuery, nil)
	if err != nil {
		return nil, fmt.Errorf("discover local topology: %w", err)
	}

	return append(peerRes.Rows, localRes.Rows[0]), nil
}

func (c *Cluster) parseNodeFromRow(r frame.Row) (*Node, error) {
	hostID, err := r[hostIDIndex].AsUUID()
	if err != nil {
		return nil, fmt.Errorf("host ID column: %w", err)
	}
	dc, err := r[dcIndex].AsText()
	if err != nil {
		return nil, fmt.Errorf("datacenter column: %w", err)
	}
	rack, err := r[rackIndex].AsText()
	if err != nil {
		return nil, fmt.Errorf("rack column: %w", err)
	}
	// Possible IP addresses starts from addrIndex in both system.local and system.peers queries.
	// They are grouped with decreasing priority.
	var addr net.IP
	for i := addrIndex; i < len(r); i++ {
		addr, err = r[i].AsIP()
		if err == nil && !addr.IsUnspecified() {
			break
		} else if err == nil && addr.IsUnspecified() {
			host, _, err := net.SplitHostPort(c.control.conn.RemoteAddr().String())
			if err == nil {
				addr = net.ParseIP(host)
				break
			}
		}
	}
	if addr == nil || addr.IsUnspecified() {
		return nil, fmt.Errorf("all addr columns conatin invalid IP")
	}
	return &Node{
		hostID:     hostID,
		addr:       addr.String(),
		datacenter: dc,
		rack:       rack,
	}, nil
}

func (c *Cluster) updateKeyspace(ctx context.Context) (ksMap, error) {
	rows, err := c.control.Query(ctx, keyspaceQuery, nil)
	if err != nil {
		return nil, err
	}
	res := make(ksMap, len(rows.Rows))
	for _, r := range rows.Rows {
		name, err := r[ksNameIndex].AsText()
		if err != nil {
			return nil, fmt.Errorf("keyspace name column: %w", err)
		}
		stg, err := parseStrategyFromRow(r)
		if err != nil {
			return nil, fmt.Errorf("keyspace replication column: %w", err)
		}
		res[name] = keyspace{strategy: stg}
	}
	return res, nil
}

func parseStrategyFromRow(r frame.Row) (strategy, error) {
	stg, err := r[replicationIndex].AsStringMap()
	if err != nil {
		return strategy{}, fmt.Errorf("strategy and rf column: %w", err)
	}
	className, ok := stg["class"]
	if !ok {
		return strategy{}, fmt.Errorf("strategy map should have a 'class' field")
	}
	delete(stg, "class")
	// We set strategy name to its shorter version.
	switch strategyClass(className) {
	case simpleStrategyJCN, simpleStrategy:
		return parseSimpleStrategy(simpleStrategy, stg)
	case networkTopologyStrategyJCN, networkTopologyStrategy:
		return parseNetworkStrategy(networkTopologyStrategy, stg)
	case localStrategyJCN, localStrategy:
		return strategy{
			class: localStrategy,
			rf:    1,
		}, nil
	default:
		return strategy{
			class: strategyClass(className),
			data:  stg,
		}, nil
	}
}

func parseSimpleStrategy(name strategyClass, stg map[string]string) (strategy, error) {
	rfStr, ok := stg["replication_factor"]
	if !ok {
		return strategy{}, fmt.Errorf("replication_factor field not found")
	}
	rf, err := strconv.ParseUint(rfStr, 10, 32)
	if err != nil {
		return strategy{}, fmt.Errorf("could not parse replication factor as unsigned int")
	}
	return strategy{
		class: name,
		rf:    uint32(rf),
	}, nil
}

func parseNetworkStrategy(name strategyClass, stg map[string]string) (strategy, error) {
	dcRF := make(dcRFMap, len(stg))
	for dc, v := range stg {
		rf, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return strategy{}, fmt.Errorf("could not parse replication factor as int")
		}
		dcRF[dc] = uint32(rf)
	}
	return strategy{
		class: name,
		dcRF:  dcRF,
	}, nil
}

// parseTokensFromRow also inserts tokens into ring.
func parseTokensFromRow(n *Node, r frame.Row, ring *Ring) error {
	if tokens, err := r[tokensIndex].AsStringSlice(); err != nil {
		return err
	} else {
		for _, t := range tokens {
			if v, err := strconv.ParseInt(t, 10, 64); err != nil {
				return fmt.Errorf("couldn't parse token string: %w", err)
			} else {
				*ring = append(*ring, RingEntry{
					node:  n,
					token: Token(v),
				})
			}
		}
	}
	return nil
}

func (c *Cluster) Topology() *topology {
	return c.topology.Load().(*topology)
}

func (c *Cluster) setTopology(t *topology) {
	c.topology.Store(t)
}

// handleEvent creates function which is passed to control connection
// via registerEvents in order to handle events right away instead
// of registering handlers for them.
func (c *Cluster) handleEvent(r response) {
	if r.Err != nil {
		log.Printf("cluster: received event with error: %v", r.Err)
		c.RequestReopenControl()
		return
	}
	switch v := r.Response.(type) {
	case *TopologyChange:
		c.handleTopologyChange(v)
	case *StatusChange:
		c.handleStatusChange(v)
	case *SchemaChange:
		// TODO: add schema change.
	default:
		log.Printf("cluster: unsupported event type: %v", r.Response)
	}
}

func (c *Cluster) handleTopologyChange(v *TopologyChange) {
	log.Printf("cluster: handle topology change: %+#v", v)
	c.RequestRefresh()
}

func (c *Cluster) handleStatusChange(v *StatusChange) {
	log.Printf("cluster: handle status change: %+#v", v)
	m := c.Topology().peers
	addr := v.Address.String()
	if n, ok := m[addr]; ok {
		switch v.Status {
		case frame.Up:
			n.setStatus(statusUP)
		case frame.Down:
			n.setStatus(statusDown)
		default:
			log.Printf("cluster: status change not supported: %+#v", v)
		}
	} else {
		log.Printf("cluster: unknown node %s received status change: %+#v in topology %v", addr, v, m)
		c.RequestRefresh()
	}
}

const refreshInterval = 60 * time.Second

// loop handles cluster requests.
func (c *Cluster) loop(ctx context.Context) {
	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-c.refreshChan:
			c.tryRefresh(ctx)
		case <-c.reopenControlChan:
			c.tryReopenControl(ctx)
		case <-ctx.Done():
			log.Printf("cluster closing due to: %v", ctx.Err())
			c.handleClose()
			return
		case <-c.closeChan:
			c.handleClose()
			return
		case <-ticker.C:
			c.tryRefresh(ctx)
		}
	}
}

const tryRefreshInterval = time.Second

// tryRefresh refreshes cluster topology.
// In case of error tries to reopen control connection and tries again.
func (c *Cluster) tryRefresh(ctx context.Context) {
	if err := c.refreshTopology(ctx); err != nil {
		c.RequestReopenControl()
		time.AfterFunc(tryRefreshInterval, c.RequestRefresh)
		log.Printf("cluster: refresh topology: %v", err)
	}
}

const tryReopenControlInterval = time.Second

func (c *Cluster) tryReopenControl(ctx context.Context) {
	log.Printf("cluster: reopen control connection")
	if control, err := c.NewControl(ctx); err != nil {
		time.AfterFunc(tryReopenControlInterval, c.RequestReopenControl)
		log.Printf("cluster: failed to reopen control connection: %v", err)
	} else {
		c.control.Close()
		c.control = control
	}
	drainChan(c.reopenControlChan)
}

func (c *Cluster) handleClose() {
	log.Printf("cluster: handle cluster close")
	c.control.Close()
	m := c.Topology().peers
	for _, v := range m {
		if v.pool != nil {
			v.pool.Close()
		}
	}
}

func (c *Cluster) RequestRefresh() {
	log.Printf("cluster: requested to refresh cluster topology")
	select {
	case c.refreshChan <- struct{}{}:
	default:
	}
}

func (c *Cluster) RequestReopenControl() {
	log.Printf("cluster: requested to reopen control connection")
	select {
	case c.reopenControlChan <- struct{}{}:
	default:
	}
}

func (c *Cluster) Close() {
	log.Printf("cluster: requested to close cluster")
	select {
	case c.closeChan <- struct{}{}:
	default:
	}
}

func drainChan(c requestChan) {
	select {
	case <-c:
	default:
	}
}
