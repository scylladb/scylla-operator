package gocql

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
)

// Polls system.peers at a specific interval to find new hosts
type ringDescriber struct {
	control         controlConnection
	cfg             *ClusterConfig
	logger          StdLogger
	mu              sync.RWMutex
	prevHosts       []*HostInfo
	prevPartitioner string

	// hosts are the set of all hosts in the cassandra ring that we know of.
	// key of map is host_id.
	hosts map[string]*HostInfo
	// hostIPToUUID maps host native address to host_id.
	hostIPToUUID map[string]string
}

func (r *ringDescriber) setControlConn(c controlConnection) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.control = c
}

// Ask the control node for the local host info
func (r *ringDescriber) getLocalHostInfo(conn ConnInterface) (*HostInfo, error) {
	iter := conn.querySystem(context.TODO(), qrySystemLocal)

	if iter == nil {
		return nil, errNoControl
	}

	host, err := hostInfoFromIter(iter, nil, r.cfg.Port, r.cfg.translateAddressPort)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve local host info: %w", err)
	}
	return host, nil
}

// Ask the control node for host info on all it's known peers
func (r *ringDescriber) getClusterPeerInfo(localHost *HostInfo, c ConnInterface) ([]*HostInfo, error) {
	var iter *Iter
	if c.getIsSchemaV2() {
		iter = c.querySystem(context.TODO(), qrySystemPeersV2)
	} else {
		iter = c.querySystem(context.TODO(), qrySystemPeers)
	}

	if iter == nil {
		return nil, errNoControl
	}

	rows, err := iter.SliceMap()
	if err != nil {
		// TODO(zariel): make typed error
		return nil, fmt.Errorf("unable to fetch peer host info: %s", err)
	}

	return getPeersFromQuerySystemPeers(rows, r.cfg.Port, r.cfg.translateAddressPort, r.logger)
}

func getPeersFromQuerySystemPeers(querySystemPeerRows []map[string]interface{}, port int, translateAddressPort func(addr net.IP, port int) (net.IP, int), logger StdLogger) ([]*HostInfo, error) {
	var peers []*HostInfo

	for _, row := range querySystemPeerRows {
		// extract all available info about the peer
		host, err := hostInfoFromMap(row, &HostInfo{port: port}, translateAddressPort)
		if err != nil {
			return nil, err
		} else if !isValidPeer(host) {
			// If it's not a valid peer
			logger.Printf("Found invalid peer '%s' "+
				"Likely due to a gossip or snitch issue, this host will be ignored", host)
			continue
		} else if isZeroToken(host) {
			continue
		}

		peers = append(peers, host)
	}

	return peers, nil
}

// Return true if the host is a valid peer
func isValidPeer(host *HostInfo) bool {
	return !(len(host.RPCAddress()) == 0 ||
		host.hostId == "" ||
		host.dataCenter == "" ||
		host.rack == "")
}

func isZeroToken(host *HostInfo) bool {
	return len(host.tokens) == 0
}

// GetHostsFromSystem returns a list of hosts found via queries to system.local and system.peers
func (r *ringDescriber) GetHostsFromSystem() ([]*HostInfo, string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.control == nil {
		return r.prevHosts, r.prevPartitioner, errNoControl
	}

	ch := r.control.getConn()
	localHost, err := r.getLocalHostInfo(ch.conn)
	if err != nil {
		return r.prevHosts, r.prevPartitioner, err
	}

	peerHosts, err := r.getClusterPeerInfo(localHost, ch.conn)
	if err != nil {
		return r.prevHosts, r.prevPartitioner, err
	}

	var hosts []*HostInfo
	if !isZeroToken(localHost) {
		hosts = []*HostInfo{localHost}
	}
	hosts = append(hosts, peerHosts...)

	var partitioner string
	if len(hosts) > 0 {
		partitioner = hosts[0].Partitioner()
	}

	r.prevHosts = hosts
	r.prevPartitioner = partitioner

	return hosts, partitioner, nil
}

// Given an ip/port return HostInfo for the specified ip/port
func (r *ringDescriber) getHostInfo(hostID UUID) (*HostInfo, error) {
	var host *HostInfo
	for _, table := range []string{"system.peers", "system.local"} {
		ch := r.control.getConn()
		var iter *Iter
		if ch.host.HostID() == hostID.String() {
			host = ch.host
			iter = nil
		}

		if table == "system.peers" {
			if ch.conn.getIsSchemaV2() {
				iter = ch.conn.querySystem(context.TODO(), qrySystemPeersV2)
			} else {
				iter = ch.conn.querySystem(context.TODO(), qrySystemPeers)
			}
		} else {
			iter = ch.conn.query(context.TODO(), fmt.Sprintf("SELECT * FROM %s", table))
		}

		if iter != nil {
			rows, err := iter.SliceMap()
			if err != nil {
				return nil, err
			}

			for _, row := range rows {
				h, err := hostInfoFromMap(row, &HostInfo{port: r.cfg.Port}, r.cfg.translateAddressPort)
				if err != nil {
					return nil, err
				}

				if h.HostID() == hostID.String() {
					host = h
					break
				}
			}
		}
	}

	if host == nil {
		return nil, errors.New("unable to fetch host info: invalid control connection")
	} else if host.invalidConnectAddr() {
		return nil, fmt.Errorf("host ConnectAddress invalid ip=%v: %v", host.connectAddress, host)
	}

	return host, nil
}

func (r *ringDescriber) getHostByIP(ip string) (*HostInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	hi, ok := r.hostIPToUUID[ip]
	return r.hosts[hi], ok
}

func (r *ringDescriber) getHost(hostID string) *HostInfo {
	r.mu.RLock()
	host := r.hosts[hostID]
	r.mu.RUnlock()
	return host
}

func (r *ringDescriber) getHostsList() []*HostInfo {
	r.mu.RLock()
	hosts := make([]*HostInfo, 0, len(r.hosts))
	for _, host := range r.hosts {
		hosts = append(hosts, host)
	}
	r.mu.RUnlock()
	return hosts
}

func (r *ringDescriber) getHostsMap() map[string]*HostInfo {
	r.mu.RLock()
	hosts := make(map[string]*HostInfo, len(r.hosts))
	for k, v := range r.hosts {
		hosts[k] = v
	}
	r.mu.RUnlock()
	return hosts
}

func (r *ringDescriber) addOrUpdate(host *HostInfo) *HostInfo {
	if existingHost, ok := r.addHostIfMissing(host); ok {
		existingHost.update(host)
		host = existingHost
	}
	return host
}

func (r *ringDescriber) addHostIfMissing(host *HostInfo) (*HostInfo, bool) {
	if host.invalidConnectAddr() {
		panic(fmt.Sprintf("invalid host: %v", host))
	}
	hostID := host.HostID()

	r.mu.Lock()
	if r.hosts == nil {
		r.hosts = make(map[string]*HostInfo)
	}
	if r.hostIPToUUID == nil {
		r.hostIPToUUID = make(map[string]string)
	}

	existing, ok := r.hosts[hostID]
	if !ok {
		r.hosts[hostID] = host
		r.hostIPToUUID[host.nodeToNodeAddress().String()] = hostID
		existing = host
	}
	r.mu.Unlock()
	return existing, ok
}

func (r *ringDescriber) removeHost(hostID string) bool {
	r.mu.Lock()
	if r.hosts == nil {
		r.hosts = make(map[string]*HostInfo)
	}
	if r.hostIPToUUID == nil {
		r.hostIPToUUID = make(map[string]string)
	}

	h, ok := r.hosts[hostID]
	if ok {
		delete(r.hostIPToUUID, h.nodeToNodeAddress().String())
	}
	delete(r.hosts, hostID)
	r.mu.Unlock()
	return ok
}
