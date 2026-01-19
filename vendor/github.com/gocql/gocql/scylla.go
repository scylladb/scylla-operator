package gocql

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"math"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gocql/gocql/internal/debug"
)

// ScyllaFeatures represents Scylla connection options as sent in SUPPORTED
// frame.
// FIXME: Should also follow `cqlProtocolExtension` interface.
type ScyllaConnectionFeatures struct {
	ScyllaHostFeatures
	// Comes from SCYLLA_SHARD
	shard int
}

func (f ScyllaConnectionFeatures) Shard() int {
	return f.shard
}

type ScyllaHostFeatures struct {
	// Comes from SCYLLA_PARTITIONER
	partitioner string
	// Comes from SCYLLA_SHARDING_ALGORITHM
	shardingAlgorithm string
	// Comes from SCYLLA_NR_SHARDS
	nrShards int
	// Comes from SCYLLA_SHARDING_IGNORE_MSB
	msbIgnore uint64
	// Comes from SCYLLA_LWT_ADD_METADATA_MARK.LWT_OPTIMIZATION_META_BIT_MASK
	lwtFlagMask int
	// Comes from SCYLLA_RATE_LIMIT_ERROR.ERROR_CODE
	rateLimitErrorCode int
	// Comes from SCYLLA_SHARD_AWARE_PORT
	shardAwarePort uint16
	// Comes from SCYLLA_SHARD_AWARE_PORT_SSL
	shardAwarePortTLS uint16
	// Comes from SCYLLA_USE_METADATA_ID
	// Signals that host supports proper prepared statement metadata invalidation read more at:
	// https://github.com/scylladb/scylladb/issues/20860
	// https://github.com/scylladb/scylladb/pull/23292
	isMetadataIDSupported bool
}

func (f ScyllaHostFeatures) IsPresent() bool {
	return f.nrShards != 0
}

func (f ScyllaHostFeatures) Partitioner() string {
	return f.partitioner
}

func (f ScyllaHostFeatures) ShardingAlgorithm() string {
	return f.shardingAlgorithm
}

func (f ScyllaHostFeatures) ShardsCount() int {
	return f.nrShards
}

func (f ScyllaHostFeatures) MSBIgnore() uint64 {
	return f.msbIgnore
}

func (f ScyllaHostFeatures) LWTFlagMask() int {
	return f.lwtFlagMask
}

func (f ScyllaHostFeatures) ShardAwarePort() uint16 {
	return f.shardAwarePort
}

func (f ScyllaHostFeatures) ShardAwarePortTLS() uint16 {
	return f.shardAwarePortTLS
}

func (f ScyllaHostFeatures) RateLimitErrorCode() int {
	return f.rateLimitErrorCode
}

func (f ScyllaHostFeatures) IsMetadataIDSupported() bool {
	return f.isMetadataIDSupported
}

// CQL Protocol extension interface for Scylla.
// Each extension is identified by a name and defines a way to serialize itself
// in STARTUP message payload.
type cqlProtocolExtension interface {
	name() string
	serialize() map[string]string
}

func findCQLProtoExtByName(exts []cqlProtocolExtension, name string) cqlProtocolExtension {
	for i := range exts {
		if exts[i].name() == name {
			return exts[i]
		}
	}
	return nil
}

// Top-level keys used for serialization/deserialization of CQL protocol
// extensions in SUPPORTED/STARTUP messages.
// Each key identifies a single extension.
const (
	lwtAddMetadataMarkKey = "SCYLLA_LWT_ADD_METADATA_MARK"
	rateLimitError        = "SCYLLA_RATE_LIMIT_ERROR"
	tabletsRoutingV1      = "TABLETS_ROUTING_V1"
)

// "tabletsRoutingV1" CQL Protocol Extension.
// This extension, if enabled (properly negotiated), allows Scylla server
// to send a tablet information in `custom_payload`.
//
// Implements cqlProtocolExtension interface.
type tabletsRoutingV1Ext struct {
}

var _ cqlProtocolExtension = &tabletsRoutingV1Ext{}

// Factory function to deserialize and create an `tabletsRoutingV1Ext` instance
// from SUPPORTED message payload.
func newTabletsRoutingV1Ext(supported map[string][]string) *tabletsRoutingV1Ext {
	if _, found := supported[tabletsRoutingV1]; found {
		return &tabletsRoutingV1Ext{}
	}
	return nil
}

func (ext *tabletsRoutingV1Ext) serialize() map[string]string {
	return map[string]string{
		tabletsRoutingV1: "",
	}
}

func (ext *tabletsRoutingV1Ext) name() string {
	return tabletsRoutingV1
}

// "Rate limit" CQL Protocol Extension.
// This extension, if enabled (properly negotiated), allows Scylla server
// to send a special kind of error.
//
// Implements cqlProtocolExtension interface.
type rateLimitExt struct {
	rateLimitErrorCode int
}

var _ cqlProtocolExtension = &rateLimitExt{}

// Factory function to deserialize and create an `rateLimitExt` instance
// from SUPPORTED message payload.
func newRateLimitExt(supported map[string][]string, logger StdLogger) *rateLimitExt {
	const rateLimitErrorCode = "ERROR_CODE"

	if v, found := supported[rateLimitError]; found {
		for i := range v {
			splitVal := strings.Split(v[i], "=")
			if splitVal[0] == rateLimitErrorCode {
				var (
					err       error
					errorCode int
				)
				if errorCode, err = strconv.Atoi(splitVal[1]); err != nil {
					if debug.Enabled {
						logger.Printf("scylla: failed to parse %s value %v: %s", rateLimitErrorCode, splitVal[1], err)
						return nil
					}
				}
				return &rateLimitExt{
					rateLimitErrorCode: errorCode,
				}
			}
		}
	}
	return nil
}

func (ext *rateLimitExt) serialize() map[string]string {
	return map[string]string{
		rateLimitError: "",
	}
}

func (ext *rateLimitExt) name() string {
	return rateLimitError
}

// "LWT prepared statements metadata mark" CQL Protocol Extension.
// This extension, if enabled (properly negotiated), allows Scylla server
// to set a special bit in prepared statements metadata, which would indicate
// whether the statement at hand is LWT statement or not.
//
// This is further used to consistently choose primary replicas in a predefined
// order for these queries, which can reduce contention over hot keys and thus
// increase LWT performance.
//
// Implements cqlProtocolExtension interface.
type lwtAddMetadataMarkExt struct {
	lwtOptMetaBitMask int
}

var _ cqlProtocolExtension = &lwtAddMetadataMarkExt{}

// Factory function to deserialize and create an `lwtAddMetadataMarkExt` instance
// from SUPPORTED message payload.
func newLwtAddMetaMarkExt(supported map[string][]string, logger StdLogger) *lwtAddMetadataMarkExt {
	const lwtOptMetaBitMaskKey = "LWT_OPTIMIZATION_META_BIT_MASK"

	if v, found := supported[lwtAddMetadataMarkKey]; found {
		for i := range v {
			splitVal := strings.Split(v[i], "=")
			if splitVal[0] == lwtOptMetaBitMaskKey {
				var (
					err     error
					bitMask int
				)
				if bitMask, err = strconv.Atoi(splitVal[1]); err != nil {
					if debug.Enabled {
						logger.Printf("scylla: failed to parse %s value %v: %s", lwtOptMetaBitMaskKey, splitVal[1], err)
						return nil
					}
				}
				return &lwtAddMetadataMarkExt{
					lwtOptMetaBitMask: bitMask,
				}
			}
		}
	}
	return nil
}

func (ext *lwtAddMetadataMarkExt) serialize() map[string]string {
	return map[string]string{
		lwtAddMetadataMarkKey: fmt.Sprintf("LWT_OPTIMIZATION_META_BIT_MASK=%d", ext.lwtOptMetaBitMask),
	}
}

func (ext *lwtAddMetadataMarkExt) name() string {
	return lwtAddMetadataMarkKey
}

func parseSupported(supported map[string][]string, logger StdLogger) ScyllaConnectionFeatures {
	const (
		scyllaShard             = "SCYLLA_SHARD"
		scyllaNrShards          = "SCYLLA_NR_SHARDS"
		scyllaPartitioner       = "SCYLLA_PARTITIONER"
		scyllaShardingAlgorithm = "SCYLLA_SHARDING_ALGORITHM"
		scyllaShardingIgnoreMSB = "SCYLLA_SHARDING_IGNORE_MSB"
		scyllaShardAwarePort    = "SCYLLA_SHARD_AWARE_PORT"
		scyllaShardAwarePortSSL = "SCYLLA_SHARD_AWARE_PORT_SSL"
		scyllaUseMetadataID     = "SCYLLA_USE_METADATA_ID"
	)

	var (
		si  ScyllaConnectionFeatures
		err error
	)

	if s, ok := supported[scyllaShard]; ok {
		if si.shard, err = strconv.Atoi(s[0]); err != nil {
			if debug.Enabled {
				logger.Printf("scylla: failed to parse %s value %v: %s", scyllaShard, s, err)
			}
		}
	}
	if s, ok := supported[scyllaNrShards]; ok {
		if si.nrShards, err = strconv.Atoi(s[0]); err != nil {
			if debug.Enabled {
				logger.Printf("scylla: failed to parse %s value %v: %s", scyllaNrShards, s, err)
			}
		}
	}
	if s, ok := supported[scyllaShardingIgnoreMSB]; ok {
		if si.msbIgnore, err = strconv.ParseUint(s[0], 10, 64); err != nil {
			if debug.Enabled {
				logger.Printf("scylla: failed to parse %s value %v: %s", scyllaShardingIgnoreMSB, s, err)
			}
		}
	}

	if s, ok := supported[scyllaPartitioner]; ok {
		si.partitioner = s[0]
	}
	if s, ok := supported[scyllaShardingAlgorithm]; ok {
		si.shardingAlgorithm = s[0]
	}
	if s, ok := supported[scyllaShardAwarePort]; ok {
		if shardAwarePort, err := strconv.ParseUint(s[0], 10, 16); err != nil {
			if debug.Enabled {
				logger.Printf("scylla: failed to parse %s value %v: %s", scyllaShardAwarePort, s, err)
			}
		} else {
			si.shardAwarePort = uint16(shardAwarePort)
		}
	}
	if s, ok := supported[scyllaShardAwarePortSSL]; ok {
		if shardAwarePortTLS, err := strconv.ParseUint(s[0], 10, 16); err != nil {
			if debug.Enabled {
				logger.Printf("scylla: failed to parse %s value %v: %s", scyllaShardAwarePortSSL, s, err)
			}
		} else {
			si.shardAwarePortTLS = uint16(shardAwarePortTLS)
		}
	}

	if lwtInfo := newLwtAddMetaMarkExt(supported, logger); lwtInfo != nil {
		si.lwtFlagMask = lwtInfo.lwtOptMetaBitMask
	}

	if rateLimitInfo := newRateLimitExt(supported, logger); rateLimitInfo != nil {
		si.rateLimitErrorCode = rateLimitInfo.rateLimitErrorCode
	}

	if _, ok := supported[scyllaUseMetadataID]; ok {
		si.isMetadataIDSupported = true
	}

	if si.partitioner != "org.apache.cassandra.dht.Murmur3Partitioner" || si.shardingAlgorithm != "biased-token-round-robin" || si.nrShards == 0 || si.msbIgnore == 0 {
		if debug.Enabled {
			logger.Printf("scylla: unsupported sharding configuration, partitioner=%s, algorithm=%s, no_shards=%d, msb_ignore=%d",
				si.partitioner, si.shardingAlgorithm, si.nrShards, si.msbIgnore)
		}
		return ScyllaConnectionFeatures{}
	}

	return si
}

func parseCQLProtocolExtensions(supported map[string][]string, logger StdLogger) []cqlProtocolExtension {
	exts := []cqlProtocolExtension{}

	lwtExt := newLwtAddMetaMarkExt(supported, logger)
	if lwtExt != nil {
		exts = append(exts, lwtExt)
	}

	rateLimitExt := newRateLimitExt(supported, logger)
	if rateLimitExt != nil {
		exts = append(exts, rateLimitExt)
	}

	tabletsExt := newTabletsRoutingV1Ext(supported)
	if tabletsExt != nil {
		exts = append(exts, tabletsExt)
	}

	return exts
}

// isScyllaConn checks if conn is suitable for scyllaConnPicker.
func (c *Conn) isScyllaConn() bool {
	return c.getScyllaSupported().nrShards != 0
}

// scyllaConnPicker is a specialised ConnPicker that selects connections based
// on token trying to get connection to a shard containing the given token.
// A list of excess connections is maintained to allow for lazy closing of
// connections to already opened shards. Keeping excess connections open helps
// reaching equilibrium faster since the likelihood of hitting the same shard
// decreases with the number of connections to the shard.
//
// scyllaConnPicker keeps track of the details about the shard-aware port.
// When used as a Dialer, it connects to the shard-aware port instead of the
// regular port (if the node supports it). For each subsequent connection
// it tries to make, the shard that it aims to connect to is chosen
// in a round-robin fashion.
type scyllaConnPicker struct {
	logger StdLogger
	// disableShardAwarePortUntil is used to temporarily disable new connections to the shard-aware port temporarily
	disableShardAwarePortUntil *atomic.Value
	hostId                     string
	address                    string
	conns                      []*Conn
	excessConns                []*Conn
	nrShards                   int
	pos                        uint64
	lastAttemptedShard         int
	msbIgnore                  uint64
	nrConns                    int
	shardAwarePortDisabled     bool
	excessConnsLimitRate       float32
}

func newScyllaConnPicker(conn *Conn, logger StdLogger) *scyllaConnPicker {
	addr := conn.Address()

	if conn.scyllaSupported.nrShards == 0 {
		panic(fmt.Sprintf("scylla: %s not a sharded connection", addr))
	}

	if debug.Enabled {
		logger.Printf("scylla: %s new conn picker sharding options %+v", addr, conn.scyllaSupported)
	}

	return &scyllaConnPicker{
		address:                addr,
		hostId:                 conn.host.hostId,
		nrShards:               conn.scyllaSupported.nrShards,
		msbIgnore:              conn.scyllaSupported.msbIgnore,
		lastAttemptedShard:     0,
		shardAwarePortDisabled: conn.session.cfg.DisableShardAwarePort,
		logger:                 logger,
		excessConnsLimitRate:   conn.session.cfg.MaxExcessShardConnectionsRate,

		disableShardAwarePortUntil: new(atomic.Value),
	}
}

func (p *scyllaConnPicker) Pick(t Token, qry ExecutableQuery) *Conn {
	if len(p.conns) == 0 {
		return nil
	}

	if t == nil {
		return p.leastBusyConn()
	}

	mmt, ok := t.(int64Token)
	// double check if that's murmur3 token
	if !ok {
		return nil
	}

	idx := -1

outer:
	for _, conn := range p.conns {
		if conn == nil {
			continue
		}

		if qry != nil && conn.isTabletSupported() {
			for _, replica := range conn.session.findTabletReplicasForToken(qry.Keyspace(), qry.Table(), int64(mmt)) {
				if replica.HostID() == p.hostId {
					idx = replica.ShardID()
					break outer
				}
			}
		}

		break
	}

	if idx == -1 {
		idx = p.shardOf(mmt)
	}

	if c := p.conns[idx]; c != nil {
		// We have this shard's connection
		// so let's give it to the caller.
		// But only if it's not loaded too much and load is well distributed.
		if qry != nil && qry.IsLWT() {
			return c
		}
		return p.maybeReplaceWithLessBusyConnection(c)
	}
	return p.leastBusyConn()
}

func (p *scyllaConnPicker) maybeReplaceWithLessBusyConnection(c *Conn) *Conn {
	if !isHeavyLoaded(c) {
		return c
	}
	alternative := p.leastBusyConn()
	if alternative == nil || alternative.AvailableStreams()*120 > c.AvailableStreams()*100 {
		return c
	} else {
		return alternative
	}
}

func isHeavyLoaded(c *Conn) bool {
	return c.streams.NumStreams/2 > c.AvailableStreams()
}

func (p *scyllaConnPicker) leastBusyConn() *Conn {
	var (
		leastBusyConn    *Conn
		streamsAvailable int
	)
	idx := int(atomic.AddUint64(&p.pos, 1))
	// find the conn which has the most available streams, this is racy
	for i := range p.conns {
		if conn := p.conns[(idx+i)%len(p.conns)]; conn != nil {
			if streams := conn.AvailableStreams(); streams > streamsAvailable {
				leastBusyConn = conn
				streamsAvailable = streams
			}
		}
	}
	return leastBusyConn
}

func (p *scyllaConnPicker) shardOf(token int64Token) int {
	shards := uint64(p.nrShards)
	z := uint64(token+math.MinInt64) << p.msbIgnore
	lo := z & 0xffffffff
	hi := (z >> 32) & 0xffffffff
	mul1 := lo * shards
	mul2 := hi * shards
	sum := (mul1 >> 32) + mul2
	return int(sum >> 32)
}

func (p *scyllaConnPicker) Put(conn *Conn) error {
	var (
		nrShards = conn.scyllaSupported.nrShards
		shard    = conn.scyllaSupported.shard
	)

	if nrShards == 0 {
		return errors.New("server reported that it has no shards")
	}

	if nrShards != p.nrShards {
		if debug.Enabled {
			p.logger.Printf("scylla: %s shard count changed from %d to %d, rebuilding connection pool",
				p.address, p.nrShards, nrShards)
		}
		p.handleShardCountChange(conn, nrShards)
	} else if nrShards != len(p.conns) {
		conns := p.conns
		p.conns = make([]*Conn, nrShards)
		copy(p.conns, conns)
	}

	if c := p.conns[shard]; c != nil {
		if conn.isShardAware {
			// A connection made to the shard-aware port resulted in duplicate
			// connection to the same shard being made. Because this is never
			// intentional, it suggests that a NAT or AddressTranslator
			// changes the source port along the way, therefore we can't trust
			// the shard-aware port to return connection to the shard
			// that we requested. Fall back to non-shard-aware port for some time.
			p.logger.Printf(
				"scylla: connection to shard-aware address %s resulted in wrong shard being assigned; please check that you are not behind a NAT or AddressTranslater which changes source ports; falling back to non-shard-aware port for %v",
				p.address,
				scyllaShardAwarePortFallbackDuration,
			)
			until := time.Now().Add(scyllaShardAwarePortFallbackDuration)
			p.disableShardAwarePortUntil.Store(until)

			return fmt.Errorf("connection landed on %d shard that already has connection", shard)
		} else {
			p.excessConns = append(p.excessConns, conn)
			if debug.Enabled {
				p.logger.Printf("scylla: %s put shard %d excess connection total: %d missing: %d excess: %d", p.address, shard, p.nrConns, p.nrShards-p.nrConns, len(p.excessConns))
			}
		}
	} else {
		p.conns[shard] = conn
		p.nrConns++
		if debug.Enabled {
			p.logger.Printf("scylla: %s put shard %d connection total: %d missing: %d", p.address, shard, p.nrConns, p.nrShards-p.nrConns)
		}
	}

	if p.shouldCloseExcessConns() {
		p.closeExcessConns()
	}

	return nil
}

func (p *scyllaConnPicker) handleShardCountChange(newConn *Conn, newShardCount int) {
	oldShardCount := p.nrShards
	oldConns := make([]*Conn, len(p.conns))
	copy(oldConns, p.conns)

	if debug.Enabled {
		p.logger.Printf("scylla: %s handling shard topology change from %d to %d", p.address, oldShardCount, newShardCount)
	}

	newConns := make([]*Conn, newShardCount)
	var toClose []*Conn
	migratedCount := 0

	for i, conn := range oldConns {
		if conn == nil {
			continue
		}
		if i < newShardCount {
			newConns[i] = conn
			migratedCount++
		} else {
			toClose = append(toClose, conn)
		}
	}

	p.nrShards = newShardCount
	p.msbIgnore = newConn.scyllaSupported.msbIgnore
	p.conns = newConns
	p.nrConns = migratedCount
	p.lastAttemptedShard = 0

	if len(toClose) > 0 {
		go closeConns(toClose...)
	}

	if debug.Enabled {
		p.logger.Printf("scylla: %s migrated %d/%d connections to new shard topology, closing %d excess connections", p.address, migratedCount, len(oldConns), len(toClose))
	}
}

func (p *scyllaConnPicker) shouldCloseExcessConns() bool {
	if p.nrConns >= p.nrShards {
		return true
	}
	return len(p.excessConns) > int(p.excessConnsLimitRate*float32(p.nrShards))
}

func (p *scyllaConnPicker) GetConnectionCount() int {
	return p.nrConns
}

func (p *scyllaConnPicker) GetExcessConnectionCount() int {
	return len(p.excessConns)
}

func (p *scyllaConnPicker) GetShardCount() int {
	return p.nrShards
}

func (p *scyllaConnPicker) Remove(conn *Conn) {
	shard := conn.scyllaSupported.shard

	if conn.scyllaSupported.nrShards == 0 {
		// It is possible for Remove to be called before the connection is added to the pool.
		// Ignoring these connections here is safe.
		if debug.Enabled {
			p.logger.Printf("scylla: %s has unknown sharding state, ignoring it", p.address)
		}
		return
	}
	if debug.Enabled {
		p.logger.Printf("scylla: %s remove shard %d connection", p.address, shard)
	}

	if p.conns[shard] != nil {
		p.conns[shard] = nil
		p.nrConns--
	}
}

func (p *scyllaConnPicker) InFlight() int {
	result := 0
	for _, conn := range p.conns {
		if conn != nil {
			result = result + (conn.streams.InUse())
		}
	}
	return result
}

func (p *scyllaConnPicker) Size() (int, int) {
	return p.nrConns, p.nrShards - p.nrConns
}

func (p *scyllaConnPicker) Close() {
	p.closeConns()
	p.closeExcessConns()
}

func (p *scyllaConnPicker) closeConns() {
	if len(p.conns) == 0 {
		if debug.Enabled {
			p.logger.Printf("scylla: %s no connections to close", p.address)
		}
		return
	}

	conns := p.conns
	p.conns = nil
	p.nrConns = 0

	if debug.Enabled {
		p.logger.Printf("scylla: %s closing %d connections", p.address, len(conns))
	}
	go closeConns(conns...)
}

func (p *scyllaConnPicker) closeExcessConns() {
	if len(p.excessConns) == 0 {
		if debug.Enabled {
			p.logger.Printf("scylla: %s no excess connections to close", p.address)
		}
		return
	}

	conns := p.excessConns
	p.excessConns = nil

	if debug.Enabled {
		p.logger.Printf("scylla: %s closing %d excess connections", p.address, len(conns))
	}
	go closeConns(conns...)
}

// Closing must be done outside of hostConnPool lock. If holding a lock
// a deadlock can occur when closing one of the connections returns error on close.
// See scylladb/gocql#53.
func closeConns(conns ...*Conn) {
	for _, conn := range conns {
		if conn != nil {
			conn.Close()
		}
	}
}

// NextShard returns the shardID to connect to.
// nrShard specifies how many shards the host has.
// If nrShards is zero, the caller shouldn't use shard-aware port.
func (p *scyllaConnPicker) NextShard() (shardID, nrShards int) {
	if p.shardAwarePortDisabled {
		return 0, 0
	}

	disableUntil, _ := p.disableShardAwarePortUntil.Load().(time.Time)
	if time.Now().Before(disableUntil) {
		// There is suspicion that the shard-aware-port is not reachable
		// or misconfigured, fall back to the non-shard-aware port
		return 0, 0
	}

	// Find the shard without a connection
	// It's important to start counting from 1 here because we want
	// to consider the next shard after the previously attempted one
	for i := 1; i <= p.nrShards; i++ {
		shardID := (p.lastAttemptedShard + i) % p.nrShards
		if p.conns == nil || p.conns[shardID] == nil {
			p.lastAttemptedShard = shardID
			return shardID, p.nrShards
		}
	}

	// We did not find an unallocated shard
	// We will dial the non-shard-aware port
	return 0, 0
}

// ShardDialer is like HostDialer but is shard-aware.
// If the driver wants to connect to a specific shard, it will call DialShard,
// otherwise it will call DialHost.
type ShardDialer interface {
	HostDialer

	// DialShard establishes a connection to the specified shard ID out of nrShards.
	// The returned connection must be directly usable for CQL protocol,
	// specifically DialShard is responsible also for setting up the TLS session if needed.
	DialShard(ctx context.Context, host *HostInfo, shardID, nrShards int) (*DialedHost, error)
}

// A dialer which dials a particular shard
type scyllaDialer struct {
	dialer    Dialer
	logger    StdLogger
	tlsConfig *tls.Config
	cfg       *ClusterConfig
}

const scyllaShardAwarePortFallbackDuration time.Duration = 5 * time.Minute

func (sd *scyllaDialer) DialHost(ctx context.Context, host *HostInfo) (*DialedHost, error) {
	ip := host.ConnectAddress()
	port := host.Port()

	if !validIpAddr(ip) {
		return nil, fmt.Errorf("host missing connect ip address: %v", ip)
	} else if port == 0 {
		return nil, fmt.Errorf("host missing port: %v", port)
	}

	addr := net.JoinHostPort(ip.String(), strconv.Itoa(port))
	translatedInfo := host.getTranslatedConnectionInfo()
	if translatedInfo != nil {
		addr = translatedInfo.CQL.ToNetAddr()
	}

	conn, err := sd.dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	return WrapTLS(ctx, conn, addr, sd.tlsConfig)
}

func (sd *scyllaDialer) DialShard(ctx context.Context, host *HostInfo, shardID, nrShards int) (*DialedHost, error) {
	ip := host.ConnectAddress()
	port := host.Port()

	if !validIpAddr(ip) {
		return nil, fmt.Errorf("host missing connect ip address: %v", ip)
	} else if port == 0 {
		return nil, fmt.Errorf("host missing port: %v", port)
	}

	iter := newScyllaPortIterator(shardID, nrShards)
	addr := net.JoinHostPort(ip.String(), strconv.Itoa(port))
	shardAwareAddr := ""
	translatedInfo := host.getTranslatedConnectionInfo()
	if translatedInfo != nil {
		addr = translatedInfo.CQL.ToNetAddr()
		if sd.tlsConfig != nil {
			if translatedInfo.ShardAwareTLS.IsValid() {
				shardAwareAddr = translatedInfo.ShardAwareTLS.ToNetAddr()
			}
		} else if translatedInfo.ShardAware.IsValid() {
			shardAwareAddr = translatedInfo.ShardAware.ToNetAddr()
		}
	}

	if debug.Enabled {
		sd.logger.Printf("scylla: connecting to shard %d", shardID)
	}

	conn, err := sd.dialShardAware(ctx, addr, shardAwareAddr, iter)
	if err != nil {
		return nil, err
	}

	return WrapTLS(ctx, conn, addr, sd.tlsConfig)
}

func (sd *scyllaDialer) dialShardAware(ctx context.Context, addr, shardAwareAddr string, iter *scyllaPortIterator) (net.Conn, error) {
	for {
		port, ok := iter.Next()
		if !ok {
			// We exhausted ports to connect from. Try the non-shard-aware port.
			return sd.dialer.DialContext(ctx, "tcp", addr)
		}

		ctxWithPort := context.WithValue(ctx, scyllaSourcePortCtx{}, port)
		conn, err := sd.dialer.DialContext(ctxWithPort, "tcp", shardAwareAddr)

		if isLocalAddrInUseErr(err) {
			// This indicates that the source port is already in use
			// We can immediately retry with another source port for this shard
			continue
		} else if err != nil {
			conn, err := sd.dialer.DialContext(ctx, "tcp", addr)
			if err == nil {
				// We failed to connect to the shard-aware port, but succeeded
				// in connecting to the non-shard-aware port. This might
				// indicate that the shard-aware port is just not reachable,
				// but we may also be unlucky and the node became reachable
				// just after we tried the first connection.
				// We can't avoid false positives here, so I'm putting it
				// behind a debug flag.
				if debug.Enabled {
					sd.logger.Printf(
						"scylla: %s couldn't connect to shard-aware address while the non-shard-aware address %s is available; this might be an issue with ",
						addr,
						shardAwareAddr,
					)
				}
			}
			return conn, err
		}
		return conn, err
	}
}

// ErrScyllaSourcePortAlreadyInUse An error value which can returned from
// a custom dialer implementation to indicate that the requested source port
// to dial from is already in use
var ErrScyllaSourcePortAlreadyInUse = errors.New("scylla: source port is already in use")

func isLocalAddrInUseErr(err error) bool {
	return errors.Is(err, syscall.EADDRINUSE) || errors.Is(err, ErrScyllaSourcePortAlreadyInUse)
}

// ScyllaShardAwareDialer wraps a net.Dialer, but uses a source port specified by gocql when connecting.
//
// Unlike in the case standard native transport ports, gocql can choose which shard will handle
// a new connection by connecting from a specific source port. If you are using your own net.Dialer
// in ClusterConfig, you can use ScyllaShardAwareDialer to "upgrade" it so that it connects
// from the source port chosen by gocql.
//
// Please note that ScyllaShardAwareDialer overwrites the LocalAddr field in order to choose
// the right source port for connection.
type ScyllaShardAwareDialer struct {
	net.Dialer
}

func (d *ScyllaShardAwareDialer) DialContext(ctx context.Context, network, addr string) (conn net.Conn, err error) {
	sourcePort := ScyllaGetSourcePort(ctx)
	if sourcePort == 0 {
		return d.Dialer.DialContext(ctx, network, addr)
	}
	dialerWithLocalAddr := d.Dialer
	dialerWithLocalAddr.LocalAddr, err = net.ResolveTCPAddr(network, fmt.Sprintf(":%d", sourcePort))
	if err != nil {
		return nil, err
	}

	return dialerWithLocalAddr.DialContext(ctx, network, addr)
}

type scyllaPortIterator struct {
	currentPort int
	shardCount  int
}

const (
	scyllaPortBasedBalancingMin = 0x8000
	scyllaPortBasedBalancingMax = 0xFFFF
)

func newScyllaPortIterator(shardID, shardCount int) *scyllaPortIterator {
	if shardCount == 0 {
		panic("shardCount cannot be 0")
	}

	// Find the smallest port p such that p >= min and p % shardCount == shardID
	port := scyllaPortBasedBalancingMin - scyllaShardForSourcePort(scyllaPortBasedBalancingMin, shardCount) + shardID
	if port < scyllaPortBasedBalancingMin {
		port += shardCount
	}

	return &scyllaPortIterator{
		currentPort: port,
		shardCount:  shardCount,
	}
}

func (spi *scyllaPortIterator) Next() (uint16, bool) {
	if spi == nil {
		return 0, false
	}

	p := spi.currentPort

	if p > scyllaPortBasedBalancingMax {
		return 0, false
	}

	spi.currentPort += spi.shardCount
	return uint16(p), true
}

func scyllaShardForSourcePort(sourcePort uint16, shardCount int) int {
	return int(sourcePort) % shardCount
}

type scyllaSourcePortCtx struct{}

// ScyllaGetSourcePort returns the source port that should be used when connecting to a node.
//
// Unlike in the case standard native transport ports, gocql can choose which shard will handle
// a new connection at the shard-aware port by connecting from a specific source port. Therefore,
// if you are using a custom Dialer and your nodes expose shard-aware ports, your dialer should
// use the source port specified by gocql.
//
// If this function returns 0, then your dialer can use any source port.
//
// If you aren't using a custom dialer, gocql will use a default one which uses appropriate source port.
// If you are using net.Dialer, consider wrapping it in a gocql.ScyllaShardAwareDialer.
func ScyllaGetSourcePort(ctx context.Context) uint16 {
	sourcePort, _ := ctx.Value(scyllaSourcePortCtx{}).(uint16)
	return sourcePort
}

// Returns a partitioner specific to the table, or "nil"
// if the cluster-global partitioner should be used
func scyllaGetTablePartitioner(session *Session, keyspaceName, tableName string) (Partitioner, error) {
	isCdc, err := scyllaIsCdcTable(session, keyspaceName, tableName)
	if err != nil {
		return nil, err
	}
	if isCdc {
		return scyllaCDCPartitioner{logger: &defaultLogger{}}, nil
	}

	return nil, nil
}
