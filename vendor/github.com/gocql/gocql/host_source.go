/*
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
/*
 * Content before git sha 34fdeebefcbf183ed7f916f931aa0586fdaa1b40
 * Copyright (c) 2016, The Gocql authors,
 * provided under the BSD-3-Clause License.
 * See the NOTICE file distributed with this work for additional information.
 */

package gocql

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	frm "github.com/gocql/gocql/internal/frame"
)

var (
	ErrCannotFindHost    = errors.New("cannot find host")
	ErrHostAlreadyExists = errors.New("host already exists")
)

type nodeState int32

func (n nodeState) String() string {
	if n == NodeUp {
		return "UP"
	} else if n == NodeDown {
		return "DOWN"
	}
	return fmt.Sprintf("UNKNOWN_%d", n)
}

const (
	NodeUp nodeState = iota
	NodeDown
)

type cassVersion struct {
	Qualifier string
	Major     int
	Minor     int
	Patch     int
}

func (c *cassVersion) Set(v string) error {
	if v == "" {
		return nil
	}

	return c.UnmarshalCQL(nil, []byte(v))
}

func (c *cassVersion) UnmarshalCQL(info TypeInfo, data []byte) error {
	return c.unmarshal(data)
}

func (c *cassVersion) unmarshal(data []byte) error {
	v := strings.SplitN(strings.TrimPrefix(strings.TrimSuffix(string(data), "-SNAPSHOT"), "v"), ".", 3)

	if len(v) < 2 {
		return fmt.Errorf("invalid version string: %s", data)
	}

	var err error
	c.Major, err = strconv.Atoi(v[0])
	if err != nil {
		return fmt.Errorf("invalid major version %v: %v", v[0], err)
	}

	if len(v) == 2 {
		vMinor := strings.SplitN(v[1], "-", 2)
		c.Minor, err = strconv.Atoi(vMinor[0])
		if err != nil {
			return fmt.Errorf("invalid minor version %v: %v", vMinor[0], err)
		}
		if len(vMinor) == 2 {
			c.Qualifier = vMinor[1]
		}
		return nil
	}

	c.Minor, err = strconv.Atoi(v[1])
	if err != nil {
		return fmt.Errorf("invalid minor version %v: %v", v[1], err)
	}

	vPatch := strings.SplitN(v[2], "-", 2)
	c.Patch, err = strconv.Atoi(vPatch[0])
	if err != nil {
		return fmt.Errorf("invalid patch version %v: %v", vPatch[0], err)
	}
	if len(vPatch) == 2 {
		c.Qualifier = vPatch[1]
	}
	return nil
}

func (c cassVersion) Before(major, minor, patch int) bool {
	// We're comparing us (cassVersion) with the provided version (major, minor, patch)
	// We return true if our version is lower (comes before) than the provided one.
	if c.Major < major {
		return true
	} else if c.Major == major {
		if c.Minor < minor {
			return true
		} else if c.Minor == minor && c.Patch < patch {
			return true
		}
	}
	return false
}

func (c cassVersion) AtLeast(major, minor, patch int) bool {
	return !c.Before(major, minor, patch)
}

func (c cassVersion) String() string {
	if c.Qualifier != "" {
		return fmt.Sprintf("%d.%d.%d-%v", c.Major, c.Minor, c.Patch, c.Qualifier)
	}
	return fmt.Sprintf("v%d.%d.%d", c.Major, c.Minor, c.Patch)
}

func (c cassVersion) nodeUpDelay() time.Duration {
	if c.Major >= 2 && c.Minor >= 2 {
		// CASSANDRA-8236
		return 0
	}

	return 10 * time.Second
}

type AddressPort struct {
	Address net.IP
	Port    uint16
}

func (a AddressPort) Equal(o AddressPort) bool {
	return a.Address.Equal(o.Address) && a.Port == o.Port
}

func (a AddressPort) IsValid() bool {
	return len(a.Address) != 0 && !a.Address.IsUnspecified() && a.Port != 0
}

func (a AddressPort) String() string {
	return fmt.Sprintf("%s:%d", a.Address, a.Port)
}

func (a AddressPort) ToNetAddr() string {
	return net.JoinHostPort(a.Address.String(), strconv.Itoa(int(a.Port)))
}

type translatedAddresses struct {
	CQL           AddressPort
	ShardAware    AddressPort
	ShardAwareTLS AddressPort
}

func (h translatedAddresses) Equal(o *translatedAddresses) bool {
	return h.CQL.Equal(o.CQL) && h.ShardAware.Equal(o.ShardAware) && h.ShardAwareTLS.Equal(o.ShardAwareTLS)
}

type HostInfoBuilder struct {
	TranslatedAddresses *translatedAddresses
	Workload            string
	HostId              string
	SchemaVersion       string
	Hostname            string
	ClusterName         string
	Partitioner         string
	Rack                string
	DseVersion          string
	DataCenter          string
	ConnectAddress      net.IP
	BroadcastAddress    net.IP
	PreferredIP         net.IP
	RpcAddress          net.IP
	Peer                net.IP
	ListenAddress       net.IP
	Tokens              []string
	Version             cassVersion
	Port                int
}

func (b HostInfoBuilder) Build() HostInfo {
	return HostInfo{
		dseVersion:          b.DseVersion,
		hostId:              b.HostId,
		dataCenter:          b.DataCenter,
		schemaVersion:       b.SchemaVersion,
		hostname:            b.Hostname,
		clusterName:         b.ClusterName,
		partitioner:         b.Partitioner,
		rack:                b.Rack,
		workload:            b.Workload,
		tokens:              b.Tokens,
		preferredIP:         b.PreferredIP,
		broadcastAddress:    b.BroadcastAddress,
		rpcAddress:          b.RpcAddress,
		connectAddress:      b.ConnectAddress,
		listenAddress:       b.ListenAddress,
		translatedAddresses: b.TranslatedAddresses,
		version:             b.Version,
		port:                b.Port,
		peer:                b.Peer,
	}
}

type HostInfo struct {
	translatedAddresses *translatedAddresses
	dseVersion          string
	hostId              string
	dataCenter          string
	schemaVersion       string
	hostname            string
	clusterName         string
	partitioner         string
	rack                string
	workload            string
	rpcAddress          net.IP
	tokens              []string
	preferredIP         net.IP
	peer                net.IP
	listenAddress       net.IP
	connectAddress      net.IP
	broadcastAddress    net.IP
	version             cassVersion
	scyllaFeatures      ScyllaHostFeatures
	port                int
	// TODO(zariel): reduce locking maybe, not all values will change, but to ensure
	// that we are thread safe use a mutex to access all fields.
	mu    sync.RWMutex
	state nodeState
	graph bool
}

func (h *HostInfo) Equal(host *HostInfo) bool {
	if h == host {
		// prevent rlock reentry
		return true
	}

	return h.HostID() == host.HostID() && h.ConnectAddress().Equal(host.ConnectAddress()) && h.Port() == host.Port()
}

func (h *HostInfo) Peer() net.IP {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.peer
}

func (h *HostInfo) invalidConnectAddr() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	addr, _ := h.connectAddressLocked()
	return !validIpAddr(addr)
}

func validIpAddr(addr net.IP) bool {
	return addr != nil && !addr.IsUnspecified()
}

func (h *HostInfo) connectAddressLocked() (net.IP, string) {
	if h.translatedAddresses != nil && h.translatedAddresses.CQL.IsValid() {
		return h.translatedAddresses.CQL.Address, "connect_address"
	} else if validIpAddr(h.connectAddress) {
		return h.connectAddress, "connect_address"
	} else if validIpAddr(h.rpcAddress) {
		return h.rpcAddress, "rpc_adress"
	} else if validIpAddr(h.preferredIP) {
		// where does perferred_ip get set?
		return h.preferredIP, "preferred_ip"
	} else if validIpAddr(h.broadcastAddress) {
		return h.broadcastAddress, "broadcast_address"
	} else if validIpAddr(h.peer) {
		return h.peer, "peer"
	}
	return net.IPv4zero, "invalid"
}

func (h *HostInfo) getDriverFacingIpAddressLocked() net.IP {
	if validIpAddr(h.rpcAddress) {
		return h.rpcAddress
	} else if validIpAddr(h.preferredIP) {
		return h.preferredIP
	} else if validIpAddr(h.broadcastAddress) {
		return h.broadcastAddress
	} else if validIpAddr(h.peer) {
		return h.peer
	}
	return net.IPv4zero
}

// nodeToNodeAddress returns address broadcasted between node to nodes.
// It's either `broadcast_address` if host info is read from system.local or `peer` if read from system.peers.
// This IP address is also part of CQL Event emitted on topology/status changes,
// but does not uniquely identify the node in case multiple nodes use the same IP address.
func (h *HostInfo) nodeToNodeAddress() net.IP {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if validIpAddr(h.broadcastAddress) {
		return h.broadcastAddress
	} else if validIpAddr(h.peer) {
		return h.peer
	}
	return net.IPv4zero
}

// Returns the address that should be used to connect to the host.
// If you wish to override this, use an AddressTranslator
func (h *HostInfo) ConnectAddress() net.IP {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if addr, _ := h.connectAddressLocked(); validIpAddr(addr) {
		return addr
	}
	panic(fmt.Sprintf("no valid connect address for host: %v. Is your cluster configured correctly?", h))
}

func (h *HostInfo) UntranslatedConnectAddress() net.IP {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.connectAddress
}

func (h *HostInfo) BroadcastAddress() net.IP {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.broadcastAddress
}

func (h *HostInfo) ListenAddress() net.IP {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.listenAddress
}

func (h *HostInfo) RPCAddress() net.IP {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.rpcAddress
}

func (h *HostInfo) PreferredIP() net.IP {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.preferredIP
}

func (h *HostInfo) DataCenter() string {
	h.mu.RLock()
	dc := h.dataCenter
	h.mu.RUnlock()
	return dc
}

func (h *HostInfo) Rack() string {
	h.mu.RLock()
	rack := h.rack
	h.mu.RUnlock()
	return rack
}

func (h *HostInfo) HostID() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.hostId
}

func (h *HostInfo) WorkLoad() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.workload
}

func (h *HostInfo) Graph() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.graph
}

func (h *HostInfo) DSEVersion() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.dseVersion
}

func (h *HostInfo) Partitioner() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.partitioner != "" {
		return h.partitioner
	}
	return h.scyllaFeatures.partitioner
}

func (h *HostInfo) ClusterName() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.clusterName
}

func (h *HostInfo) Version() cassVersion {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.version
}

func (h *HostInfo) State() nodeState {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.state
}

func (h *HostInfo) setState(state nodeState) *HostInfo {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.state = state
	return h
}

func (h *HostInfo) Tokens() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.tokens
}

func (h *HostInfo) Port() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.port
}

func (h *HostInfo) update(from *HostInfo) {
	if h == from {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	from.mu.RLock()
	defer from.mu.RUnlock()

	// autogenerated do not update
	if h.peer == nil {
		h.peer = from.peer
	}
	if h.broadcastAddress == nil {
		h.broadcastAddress = from.broadcastAddress
	}
	if h.listenAddress == nil {
		h.listenAddress = from.listenAddress
	}
	if h.rpcAddress == nil {
		h.rpcAddress = from.rpcAddress
	}
	if h.preferredIP == nil {
		h.preferredIP = from.preferredIP
	}
	if h.connectAddress == nil {
		h.connectAddress = from.connectAddress
	}
	if h.port == 0 {
		h.port = from.port
	}
	if h.dataCenter == "" {
		h.dataCenter = from.dataCenter
	}
	if h.rack == "" {
		h.rack = from.rack
	}
	if h.hostId == "" {
		h.hostId = from.hostId
	}
	if h.workload == "" {
		h.workload = from.workload
	}
	if h.dseVersion == "" {
		h.dseVersion = from.dseVersion
	}
	if h.partitioner == "" {
		h.partitioner = from.partitioner
	}
	if h.clusterName == "" {
		h.clusterName = from.clusterName
	}
	if h.version == (cassVersion{}) {
		h.version = from.version
	}
	if h.tokens == nil {
		h.tokens = from.tokens
	}
}

func (h *HostInfo) IsUp() bool {
	return h != nil && h.State() == NodeUp
}

func (h *HostInfo) IsBusy(s *Session) bool {
	pool, ok := s.pool.getPool(h)
	return ok && h != nil && pool.InFlight() >= MAX_IN_FLIGHT_THRESHOLD
}

// ConnectAddressAndPort returns "{ConnectAddress}:{Port}"
// Deprecated: Use ConnectAddress and Port separately.
func (h *HostInfo) ConnectAddressAndPort() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	addr, _ := h.connectAddressLocked()
	return net.JoinHostPort(addr.String(), strconv.Itoa(h.port))
}

func (h *HostInfo) String() string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	connectAddr, source := h.connectAddressLocked()
	return fmt.Sprintf("[HostInfo hostname=%q connectAddress=%q peer=%q rpc_address=%q broadcast_address=%q "+
		"preferred_ip=%q connect_addr=%q connect_addr_source=%q "+
		"port=%d data_center=%q rack=%q host_id=%q version=%q state=%s num_tokens=%d]",
		h.hostname, h.connectAddress, h.peer, h.rpcAddress, h.broadcastAddress, h.preferredIP,
		connectAddr, source,
		h.port, h.dataCenter, h.rack, h.hostId, h.version, h.state, len(h.tokens))
}

func (h *HostInfo) setScyllaFeatures(s ScyllaHostFeatures) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.scyllaFeatures = s
}

func (h *HostInfo) ScyllaFeatures() ScyllaHostFeatures {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.scyllaFeatures
}

// ScyllaShardAwarePort returns the shard aware port of this host.
// Returns zero if the shard aware port is not known.
func (h *HostInfo) ScyllaShardAwarePort() uint16 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.scyllaFeatures.ShardAwarePort()
}

// ScyllaShardAwarePortTLS returns the TLS-enabled shard aware port of this host.
// Returns zero if the shard aware port is not known.
func (h *HostInfo) ScyllaShardAwarePortTLS() uint16 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.scyllaFeatures.ShardAwarePortTLS()
}

// ScyllaShardCount returns count of shards on the node.
func (h *HostInfo) ScyllaShardCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.scyllaFeatures.ShardsCount()
}

func (h *HostInfo) setTranslatedConnectionInfo(info translatedAddresses) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.translatedAddresses = &info
}

func (h *HostInfo) getTranslatedConnectionInfo() *translatedAddresses {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.translatedAddresses
}

// Returns true if we are using system_schema.keyspaces instead of system.schema_keyspaces
func checkSystemSchema(control controlConnection) (bool, error) {
	iter := control.querySystem("SELECT * FROM system_schema.keyspaces")
	if err := iter.err; err != nil {
		if errf, ok := err.(*frm.ErrorFrame); ok {
			if errf.Code == ErrCodeSyntax {
				return false, nil
			}
		}

		return false, err
	}

	return true, nil
}

// Given a map that represents a row from either system.local or system.peers
// return as much information as we can in *HostInfo
func hostInfoFromMap(row map[string]interface{}, defaultPort int) (*HostInfo, error) {
	const assertErrorMsg = "Assertion failed for %s"
	var ok bool

	host := HostInfo{}

	// Default to our connected port if the cluster doesn't have port information
	for key, value := range row {
		switch key {
		case "data_center":
			host.dataCenter, ok = value.(string)
			if !ok {
				return nil, fmt.Errorf(assertErrorMsg, "data_center")
			}
		case "rack":
			host.rack, ok = value.(string)
			if !ok {
				return nil, fmt.Errorf(assertErrorMsg, "rack")
			}
		case "host_id":
			hostId, ok := value.(UUID)
			if !ok {
				return nil, fmt.Errorf(assertErrorMsg, "host_id")
			}
			host.hostId = hostId.String()
		case "release_version":
			version, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf(assertErrorMsg, "release_version")
			}
			host.version.Set(version)
		case "peer":
			ip, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf(assertErrorMsg, "peer")
			}
			host.peer = net.ParseIP(ip)
		case "cluster_name":
			host.clusterName, ok = value.(string)
			if !ok {
				return nil, fmt.Errorf(assertErrorMsg, "cluster_name")
			}
		case "partitioner":
			host.partitioner, ok = value.(string)
			if !ok {
				return nil, fmt.Errorf(assertErrorMsg, "partitioner")
			}
		case "broadcast_address":
			ip, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf(assertErrorMsg, "broadcast_address")
			}
			host.broadcastAddress = net.ParseIP(ip)
		case "preferred_ip":
			ip, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf(assertErrorMsg, "preferred_ip")
			}
			host.preferredIP = net.ParseIP(ip)
		case "rpc_address":
			ip, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf(assertErrorMsg, "rpc_address")
			}
			host.rpcAddress = net.ParseIP(ip)
		case "native_address":
			ip, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf(assertErrorMsg, "native_address")
			}
			host.rpcAddress = net.ParseIP(ip)
		case "listen_address":
			ip, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf(assertErrorMsg, "listen_address")
			}
			host.listenAddress = net.ParseIP(ip)
		case "native_port":
			native_port, ok := value.(int)
			if !ok {
				return nil, fmt.Errorf(assertErrorMsg, "native_port")
			}
			host.port = native_port
		case "workload":
			host.workload, ok = value.(string)
			if !ok {
				return nil, fmt.Errorf(assertErrorMsg, "workload")
			}
		case "graph":
			host.graph, ok = value.(bool)
			if !ok {
				return nil, fmt.Errorf(assertErrorMsg, "graph")
			}
		case "tokens":
			host.tokens, ok = value.([]string)
			if !ok {
				return nil, fmt.Errorf(assertErrorMsg, "tokens")
			}
		case "dse_version":
			host.dseVersion, ok = value.(string)
			if !ok {
				return nil, fmt.Errorf(assertErrorMsg, "dse_version")
			}
		case "schema_version":
			schemaVersion, ok := value.(UUID)
			if !ok {
				return nil, fmt.Errorf(assertErrorMsg, "schema_version")
			}
			host.schemaVersion = schemaVersion.String()
		}
		// TODO(thrawn01): Add 'port'? once CASSANDRA-7544 is complete
		// Not sure what the port field will be called until the JIRA issue is complete
	}

	if host.port == 0 {
		host.port = defaultPort
	}

	host.connectAddress = host.getDriverFacingIpAddressLocked()
	return &host, nil
}

func hostInfoFromIter(iter *Iter, defaultPort int) (*HostInfo, error) {
	rows, err := iter.SliceMap()
	if err != nil {
		// TODO(zariel): make typed error
		return nil, err
	}

	if len(rows) == 0 {
		return nil, errors.New("query returned 0 rows")
	}

	host, err := hostInfoFromMap(rows[0], defaultPort)
	if err != nil {
		return nil, err
	}
	return host, nil
}

// debounceRingRefresh submits a ring refresh request to the ring refresh debouncer.
func (s *Session) debounceRingRefresh() {
	s.ringRefresher.Debounce()
}

// refreshRing executes a ring refresh immediately and cancels pending debounce ring refresh requests.
func (s *Session) refreshRingNow() error {
	err, ok := <-s.ringRefresher.RefreshNow()
	if !ok {
		return errors.New("could not refresh ring because stop was requested")
	}

	return err
}

func (s *Session) refreshRing() error {
	hosts, partitioner, err := s.hostSource.GetHostsFromSystem()
	if err != nil {
		return err
	}
	prevHosts := s.hostSource.getHostsMap()

	for _, h := range hosts {
		if s.cfg.filterHost(h) {
			continue
		}

		if host, ok := s.hostSource.addHostIfMissing(h); !ok {
			s.startPoolFill(h)
		} else {
			// host (by hostID) already exists; determine if IP has changed
			newHostID := h.HostID()
			existing, ok := prevHosts[newHostID]
			if !ok {
				return fmt.Errorf("get existing host=%s from prevHosts: %w", h, ErrCannotFindHost)
			}
			if h.UntranslatedConnectAddress().Equal(existing.UntranslatedConnectAddress()) && h.nodeToNodeAddress().Equal(existing.nodeToNodeAddress()) {
				// no host IP change
				host.update(h)
			} else {
				// host IP has changed
				// remove old HostInfo (w/old IP)
				s.removeHost(existing)
				if _, alreadyExists := s.hostSource.addHostIfMissing(h); alreadyExists {
					return fmt.Errorf("add new host=%s after removal: %w", h, ErrHostAlreadyExists)
				}
				// add new HostInfo (same hostID, new IP)
				s.startPoolFill(h)
			}
		}
		delete(prevHosts, h.HostID())
	}

	for _, host := range prevHosts {
		s.metadataDescriber.RemoveTabletsWithHost(host)
		s.removeHost(host)
	}
	s.policy.SetPartitioner(partitioner)

	return nil
}
