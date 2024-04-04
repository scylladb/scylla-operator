// Copyright (C) 2017 ScyllaDB

package scyllaclient

import (
	"bytes"
	"context"
	stdErrors "errors"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/pkg/errors"
	"github.com/scylladb/go-set/strset"
	"github.com/scylladb/scylla-manager/v3/pkg/dht"
	"github.com/scylladb/scylla-manager/v3/pkg/util/slice"
	"go.uber.org/multierr"

	"github.com/scylladb/scylla-manager/v3/pkg/util/parallel"
	"github.com/scylladb/scylla-manager/v3/pkg/util/pointer"
	"github.com/scylladb/scylla-manager/v3/pkg/util/prom"
	"github.com/scylladb/scylla-manager/v3/swagger/gen/scylla/v1/client/operations"
	"github.com/scylladb/scylla-manager/v3/swagger/gen/scylla/v1/models"
)

// ErrHostInvalidResponse is to indicate that one of the root-causes is the invalid response from scylla-server.
var ErrHostInvalidResponse = fmt.Errorf("invalid response from host")

// ClusterName returns cluster name.
func (c *Client) ClusterName(ctx context.Context) (string, error) {
	resp, err := c.scyllaOps.StorageServiceClusterNameGet(&operations.StorageServiceClusterNameGetParams{Context: ctx})
	if err != nil {
		return "", err
	}

	return resp.Payload, nil
}

// Status returns nodetool status alike information, items are sorted by
// Datacenter and Address.
func (c *Client) Status(ctx context.Context) (NodeStatusInfoSlice, error) {
	// Get all hosts
	resp, err := c.scyllaOps.StorageServiceHostIDGet(&operations.StorageServiceHostIDGetParams{Context: ctx})
	if err != nil {
		return nil, err
	}

	all := make([]NodeStatusInfo, len(resp.Payload))
	for i, p := range resp.Payload {
		all[i].Addr = p.Key
		all[i].HostID = p.Value
	}

	// Get host datacenter (hopefully cached)
	for i := range all {
		all[i].Datacenter, err = c.HostDatacenter(ctx, all[i].Addr)
		if err != nil {
			return nil, err
		}
	}

	// Get live nodes
	live, err := c.scyllaOps.GossiperEndpointLiveGet(&operations.GossiperEndpointLiveGetParams{Context: ctx})
	if err != nil {
		return nil, err
	}
	setNodeStatus(all, NodeStatusUp, live.Payload)

	// Get joining nodes
	joining, err := c.scyllaOps.StorageServiceNodesJoiningGet(&operations.StorageServiceNodesJoiningGetParams{Context: ctx})
	if err != nil {
		return nil, err
	}
	setNodeState(all, NodeStateJoining, joining.Payload)

	// Get leaving nodes
	leaving, err := c.scyllaOps.StorageServiceNodesLeavingGet(&operations.StorageServiceNodesLeavingGetParams{Context: ctx})
	if err != nil {
		return nil, err
	}
	setNodeState(all, NodeStateLeaving, leaving.Payload)

	// Sort by Datacenter and Address
	sort.Slice(all, func(i, j int) bool {
		if all[i].Datacenter != all[j].Datacenter {
			return all[i].Datacenter < all[j].Datacenter
		}
		return all[i].Addr < all[j].Addr
	})

	return all, nil
}

// VerifyNodesAvailability checks if all nodes passed connectivity check and are in the UN state.
func (c *Client) VerifyNodesAvailability(ctx context.Context) error {
	status, err := c.Status(ctx)
	if err != nil {
		return errors.Wrap(err, "get status")
	}

	available, err := c.GetLiveNodes(ctx, status)
	if err != nil {
		return errors.Wrap(err, "get live nodes")
	}

	availableUN := available.Live()
	if len(status) == len(availableUN) {
		return nil
	}

	checked := strset.New()
	for _, n := range availableUN {
		checked.Add(n.HostID)
	}

	var unavailable []string
	for _, n := range status {
		if !checked.Has(n.HostID) {
			unavailable = append(unavailable, n.Addr)
		}
	}

	return fmt.Errorf("unavailable nodes: %v", unavailable)
}

func setNodeStatus(all []NodeStatusInfo, status NodeStatus, addrs []string) {
	if len(addrs) == 0 {
		return
	}
	m := strset.New(addrs...)

	for i := range all {
		if m.Has(all[i].Addr) {
			all[i].Status = status
		}
	}
}

func setNodeState(all []NodeStatusInfo, state NodeState, addrs []string) {
	if len(addrs) == 0 {
		return
	}
	m := strset.New(addrs...)

	for i := range all {
		if m.Has(all[i].Addr) {
			all[i].State = state
		}
	}
}

// Datacenters returns the available datacenters in this cluster.
func (c *Client) Datacenters(ctx context.Context) (map[string][]string, error) {
	resp, err := c.scyllaOps.StorageServiceHostIDGet(&operations.StorageServiceHostIDGetParams{Context: ctx})
	if err != nil {
		return nil, err
	}

	res := make(map[string][]string)
	var errs error

	for _, p := range resp.Payload {
		dc, err := c.HostDatacenter(ctx, p.Key)
		if err != nil {
			errs = multierr.Append(errs, err)
			continue
		}
		res[dc] = append(res[dc], p.Key)
	}

	return res, errs
}

// HostDatacenter looks up the datacenter that the given host belongs to.
func (c *Client) HostDatacenter(ctx context.Context, host string) (dc string, err error) {
	// Try reading from cache
	c.mu.RLock()
	dc = c.dcCache[host]
	c.mu.RUnlock()
	if dc != "" {
		return
	}

	resp, err := c.scyllaOps.SnitchDatacenterGet(&operations.SnitchDatacenterGetParams{
		Context: ctx,
		Host:    &host,
	})
	if err != nil {
		return "", err
	}
	dc = resp.Payload

	// Update cache
	c.mu.Lock()
	c.dcCache[host] = dc
	c.mu.Unlock()

	return
}

// HostIDs returns a mapping from host IP to UUID.
func (c *Client) HostIDs(ctx context.Context) (map[string]string, error) {
	resp, err := c.scyllaOps.StorageServiceHostIDGet(&operations.StorageServiceHostIDGetParams{Context: ctx})
	if err != nil {
		return nil, err
	}

	v := make(map[string]string, len(resp.Payload))
	for i := 0; i < len(resp.Payload); i++ {
		v[resp.Payload[i].Key] = resp.Payload[i].Value
	}
	return v, nil
}

// CheckHostsChanged returns true iff a host was added or removed from cluster.
// In such a case the client should be discarded.
func (c *Client) CheckHostsChanged(ctx context.Context) (bool, error) {
	cur, err := c.hosts(ctx)
	if err != nil {
		return false, err
	}
	if len(cur) != len(c.config.Hosts) {
		return true, err
	}
	return !strset.New(c.config.Hosts...).Has(cur...), nil
}

// hosts returns a list of all hosts in a cluster.
func (c *Client) hosts(ctx context.Context) ([]string, error) {
	resp, err := c.scyllaOps.StorageServiceHostIDGet(&operations.StorageServiceHostIDGetParams{Context: ctx})
	if err != nil {
		return nil, err
	}

	v := make([]string, len(resp.Payload))
	for i := 0; i < len(resp.Payload); i++ {
		v[i] = resp.Payload[i].Key
	}
	return v, nil
}

// Keyspaces return a list of all the keyspaces.
func (c *Client) Keyspaces(ctx context.Context) ([]string, error) {
	resp, err := c.scyllaOps.StorageServiceKeyspacesGet(&operations.StorageServiceKeyspacesGetParams{Context: ctx})
	if err != nil {
		return nil, err
	}
	return resp.Payload, nil
}

// Tables returns a slice of table names in a given keyspace.
func (c *Client) Tables(ctx context.Context, keyspace string) ([]string, error) {
	resp, err := c.scyllaOps.ColumnFamilyNameGet(&operations.ColumnFamilyNameGetParams{Context: ctx})
	if err != nil {
		return nil, err
	}

	var (
		prefix = keyspace + ":"
		tables []string
	)
	for _, v := range resp.Payload {
		if strings.HasPrefix(v, prefix) {
			tables = append(tables, v[len(prefix):])
		}
	}

	return tables, nil
}

// Tokens returns list of tokens for a node.
func (c *Client) Tokens(ctx context.Context, host string) ([]int64, error) {
	resp, err := c.scyllaOps.StorageServiceTokensByEndpointGet(&operations.StorageServiceTokensByEndpointGetParams{
		Endpoint: host,
		Context:  ctx,
	})
	if err != nil {
		return nil, err
	}

	tokens := make([]int64, len(resp.Payload))
	for i, s := range resp.Payload {
		v, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return tokens, errors.Wrapf(err, "parsing error at pos %d", i)
		}
		tokens[i] = v
	}
	return tokens, nil
}

// ShardCount returns number of shards in a node.
// If host is empty it will pick one from the pool.
func (c *Client) ShardCount(ctx context.Context, host string) (uint, error) {
	const (
		queryMetricName = "database_total_writes"
		metricName      = "scylla_" + queryMetricName
	)

	metrics, err := c.metrics(ctx, host, queryMetricName)
	if err != nil {
		return 0, err
	}

	if _, ok := metrics[metricName]; !ok {
		return 0, errors.Errorf("scylla doest not expose %s metric", metricName)
	}

	shards := len(metrics[metricName].Metric)
	if shards == 0 {
		return 0, errors.New("missing shard count")
	}

	return uint(shards), nil
}

// HostsShardCount runs ShardCount for many hosts.
func (c *Client) HostsShardCount(ctx context.Context, hosts []string) (map[string]uint, error) {
	shards := make([]uint, len(hosts))

	f := func(i int) error {
		sh, err := c.ShardCount(ctx, hosts[i])
		if err != nil {
			return parallel.Abort(errors.Wrapf(err, "%s: get shard count", hosts[i]))
		}
		shards[i] = sh
		return nil
	}
	if err := parallel.Run(len(hosts), parallel.NoLimit, f, parallel.NopNotify); err != nil {
		return nil, err
	}

	out := make(map[string]uint)
	for i, h := range hosts {
		out[h] = shards[i]
	}
	return out, nil
}

// metrics returns Scylla Prometheus metrics, `name` pattern be used to filter
// out only subset of metrics.
// If host is empty it will pick one from the pool.
func (c *Client) metrics(ctx context.Context, host, name string) (map[string]*prom.MetricFamily, error) {
	u := c.newURL(host, "/metrics")

	// In case host is not set select a host from a pool.
	if host != "" {
		ctx = forceHost(ctx, host)
	}
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), http.NoBody)
	if err != nil {
		return nil, err
	}

	if name != "" {
		q := r.URL.Query()
		q.Add("name", name)
		r.URL.RawQuery = q.Encode()
	}

	resp, err := c.client.Do("Metrics", r)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return prom.ParseText(resp.Body)
}

// DescribeRing returns a description of token range of a given keyspace.
func (c *Client) DescribeRing(ctx context.Context, keyspace string) (Ring, error) {
	resp, err := c.scyllaOps.StorageServiceDescribeRingByKeyspaceGet(&operations.StorageServiceDescribeRingByKeyspaceGetParams{
		Context:  ctx,
		Keyspace: keyspace,
	})
	if err != nil {
		return Ring{}, err
	}

	ring := Ring{
		ReplicaTokens: make([]ReplicaTokenRanges, 0),
		HostDC:        map[string]string{},
	}
	dcTokens := make(map[string]int)

	replicaTokens := make(map[uint64][]TokenRange)
	replicaHash := make(map[uint64][]string)

	for _, p := range resp.Payload {
		// Parse tokens
		startToken, err := strconv.ParseInt(p.StartToken, 10, 64)
		if err != nil {
			return Ring{}, errors.Wrap(err, "parse StartToken")
		}
		endToken, err := strconv.ParseInt(p.EndToken, 10, 64)
		if err != nil {
			return Ring{}, errors.Wrap(err, "parse EndToken")
		}

		// Ensure deterministic order or nodes in replica set
		sort.Strings(p.Endpoints)

		// Aggregate replica set token ranges
		hash := ReplicaHash(p.Endpoints)
		replicaHash[hash] = p.Endpoints
		replicaTokens[hash] = append(replicaTokens[hash], TokenRange{
			StartToken: startToken,
			EndToken:   endToken,
		})

		// Update host to DC mapping
		for _, e := range p.EndpointDetails {
			ring.HostDC[e.Host] = e.Datacenter
		}

		// Update DC token metrics
		dcs := strset.New()
		for _, e := range p.EndpointDetails {
			if !dcs.Has(e.Datacenter) {
				dcTokens[e.Datacenter]++
				dcs.Add(e.Datacenter)
			}
		}
	}

	for hash, tokens := range replicaTokens {
		// Ensure deterministic order of tokens
		sort.Slice(tokens, func(i, j int) bool {
			return tokens[i].StartToken < tokens[j].StartToken
		})

		ring.ReplicaTokens = append(ring.ReplicaTokens, ReplicaTokenRanges{
			ReplicaSet: replicaHash[hash],
			Ranges:     tokens,
		})
	}

	// Detect replication strategy
	if len(ring.HostDC) == 1 {
		ring.Replication = LocalStrategy
	} else {
		ring.Replication = NetworkTopologyStrategy
		for _, tokens := range dcTokens {
			if tokens != len(resp.Payload) {
				ring.Replication = SimpleStrategy
				break
			}
		}
	}

	return ring, nil
}

// ReplicaHash hashes replicas so that it can be used as a map key.
func ReplicaHash(replicaSet []string) uint64 {
	hash := xxhash.New()
	for _, r := range replicaSet {
		_, _ = hash.WriteString(r)   // nolint: errcheck
		_, _ = hash.WriteString(",") // nolint: errcheck
	}
	return hash.Sum64()
}

// Repair invokes async repair and returns the repair command ID.
func (c *Client) Repair(ctx context.Context, keyspace, table, master string, replicaSet []string, ranges []TokenRange) (int32, error) {
	dr := dumpRanges(ranges)
	p := operations.StorageServiceRepairAsyncByKeyspacePostParams{
		Context:        forceHost(ctx, master),
		Keyspace:       keyspace,
		ColumnFamilies: &table,
		Ranges:         &dr,
	}
	// Single node cluster repair fails with hosts param
	if len(replicaSet) > 1 {
		hosts := strings.Join(replicaSet, ",")
		p.Hosts = &hosts
	}

	resp, err := c.scyllaOps.StorageServiceRepairAsyncByKeyspacePost(&p)
	if err != nil {
		return 0, err
	}
	return resp.Payload, nil
}

func dumpRanges(ranges []TokenRange) string {
	var buf bytes.Buffer
	for i, ttr := range ranges {
		if i > 0 {
			_ = buf.WriteByte(',')
		}
		if ttr.StartToken > ttr.EndToken {
			_, _ = fmt.Fprintf(&buf, "%d:%d,%d:%d", dht.Murmur3MinToken, ttr.EndToken, ttr.StartToken, dht.Murmur3MaxToken)
		} else {
			_, _ = fmt.Fprintf(&buf, "%d:%d", ttr.StartToken, ttr.EndToken)
		}
	}
	return buf.String()
}

func repairStatusShouldRetryHandler(err error) *bool {
	s, m := StatusCodeAndMessageOf(err)
	if s == http.StatusInternalServerError && strings.Contains(m, "unknown repair id") {
		return pointer.BoolPtr(false)
	}
	return nil
}

const repairStatusTimeout = 30 * time.Minute

// RepairStatus waits for repair job to finish and returns its status.
func (c *Client) RepairStatus(ctx context.Context, host string, id int32) (CommandStatus, error) {
	ctx = forceHost(ctx, host)
	ctx = customTimeout(ctx, repairStatusTimeout)
	ctx = withShouldRetryHandler(ctx, repairStatusShouldRetryHandler)
	var (
		resp interface {
			GetPayload() models.RepairAsyncStatusResponse
		}
		err error
	)

	resp, err = c.scyllaOps.StorageServiceRepairStatus(&operations.StorageServiceRepairStatusParams{
		Context: ctx,
		ID:      id,
	})
	if err != nil {
		return "", err
	}
	return CommandStatus(resp.GetPayload()), nil
}

// When using long polling, wait duration starts only when node receives the
// request.
// longPollingTimeout is calculating timeout duration needed for request to
// reach node so context is not canceled before response is received.
func (c *Client) longPollingTimeout(waitSeconds int) time.Duration {
	return time.Second*time.Duration(waitSeconds) + c.config.Timeout
}

// ActiveRepairs returns a subset of hosts that are coordinators of a repair.
func (c *Client) ActiveRepairs(ctx context.Context, hosts []string) ([]string, error) {
	type hostError struct {
		host   string
		active bool
		err    error
	}
	out := make(chan hostError, runtime.NumCPU()+1)

	for _, h := range hosts {
		h := h
		go func() {
			a, err := c.hasActiveRepair(ctx, h)
			out <- hostError{
				host:   h,
				active: a,
				err:    errors.Wrapf(err, "host %s", h),
			}
		}()
	}

	var (
		active []string
		errs   error
	)
	for range hosts {
		v := <-out
		if v.err != nil {
			errs = multierr.Append(errs, v.err)
		}
		if v.active {
			active = append(active, v.host)
		}
	}
	return active, errs
}

func (c *Client) hasActiveRepair(ctx context.Context, host string) (bool, error) {
	const wait = 50 * time.Millisecond
	for i := 0; i < 10; i++ {
		resp, err := c.scyllaOps.StorageServiceActiveRepairGet(&operations.StorageServiceActiveRepairGetParams{
			Context: forceHost(ctx, host),
		})
		if err != nil {
			return false, err
		}
		if len(resp.Payload) > 0 {
			return true, nil
		}
		// wait before trying again
		t := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			t.Stop()
			return false, ctx.Err()
		case <-t.C:
		}
	}
	return false, nil
}

// KillAllRepairs forces a termination of all repairs running on a host, the
// operation is not retried to avoid side effects of a deferred kill.
func (c *Client) KillAllRepairs(ctx context.Context, hosts ...string) error {
	ctx = noRetry(ctx)

	f := func(i int) error {
		host := hosts[i]
		_, err := c.scyllaOps.StorageServiceForceTerminateRepairPost(&operations.StorageServiceForceTerminateRepairPostParams{
			Context: forceHost(ctx, host),
		})
		return err
	}

	notify := func(i int, err error) {
		host := hosts[i]
		c.logger.Error(ctx, "Failed to terminate repair",
			"host", host,
			"error", err,
		)
	}

	return parallel.Run(len(hosts), parallel.NoLimit, f, notify)
}

const snapshotTimeout = 30 * time.Minute

// Snapshots lists available snapshots.
func (c *Client) Snapshots(ctx context.Context, host string) ([]string, error) {
	ctx = customTimeout(ctx, snapshotTimeout)

	resp, err := c.scyllaOps.StorageServiceSnapshotsGet(&operations.StorageServiceSnapshotsGetParams{
		Context: forceHost(ctx, host),
	})
	if err != nil {
		return nil, err
	}

	var tags []string
	for _, p := range resp.Payload {
		tags = append(tags, p.Key)
	}

	return tags, nil
}

// SnapshotDetails returns an index of keyspaces and tables present in the given
// snapshot.
func (c *Client) SnapshotDetails(ctx context.Context, host, tag string) ([]Unit, error) {
	ctx = customTimeout(ctx, snapshotTimeout)

	resp, err := c.scyllaOps.StorageServiceSnapshotsGet(&operations.StorageServiceSnapshotsGetParams{
		Context: forceHost(ctx, host),
	})
	if err != nil {
		return nil, err
	}

	m := make(map[string]Unit)
	for _, p := range resp.Payload {
		if p.Key != tag {
			continue
		}
		for _, v := range p.Value {
			k, ok := m[v.Ks]
			if !ok {
				k = Unit{
					Keyspace: v.Ks,
				}
			}
			k.Tables = append(k.Tables, v.Cf)
			m[v.Ks] = k
		}
	}

	var s []Unit
	for _, v := range m {
		s = append(s, v)
	}
	sort.Slice(s, func(i, j int) bool {
		return s[i].Keyspace < s[j].Keyspace
	})

	return s, nil
}

// TakeSnapshot flushes and takes a snapshot of a keyspace.
// Multiple keyspaces may have the same tag.
// Flush is taken care of by Scylla, see table::snapshot for details.
// If snapshot already exists no error is returned.
func (c *Client) TakeSnapshot(ctx context.Context, host, tag, keyspace string, tables ...string) error {
	ctx = customTimeout(ctx, snapshotTimeout)
	ctx = withShouldRetryHandler(ctx, takeSnapshotShouldRetryHandler)

	var cf *string
	if len(tables) > 0 {
		cf = pointer.StringPtr(strings.Join(tables, ","))
	}

	p := operations.StorageServiceSnapshotsPostParams{
		Context: forceHost(ctx, host),
		Tag:     &tag,
		Kn:      &keyspace,
		Cf:      cf,
	}
	_, err := c.scyllaOps.StorageServiceSnapshotsPost(&p)

	// Ignore SnapshotAlreadyExists error
	if err != nil && isSnapshotAlreadyExists(err) {
		err = nil
	}

	return err
}

var snapshotAlreadyExistsRegex = regexp.MustCompile(`snapshot \w+ already exists`)

func isSnapshotAlreadyExists(err error) bool {
	_, msg := StatusCodeAndMessageOf(err)
	return snapshotAlreadyExistsRegex.MatchString(msg)
}

func takeSnapshotShouldRetryHandler(err error) *bool {
	if isSnapshotAlreadyExists(err) {
		return pointer.BoolPtr(false)
	}
	return nil
}

// DeleteSnapshot removes a snapshot with a given tag.
func (c *Client) DeleteSnapshot(ctx context.Context, host, tag string) error {
	ctx = customTimeout(ctx, snapshotTimeout)

	_, err := c.scyllaOps.StorageServiceSnapshotsDelete(&operations.StorageServiceSnapshotsDeleteParams{ // nolint: errcheck
		Context: forceHost(ctx, host),
		Tag:     &tag,
	})
	return err
}

// DeleteTableSnapshot removes a snapshot with a given tag.
// Removed data is restricted to the provided keyspace and table.
func (c *Client) DeleteTableSnapshot(ctx context.Context, host, tag, keyspace, table string) error {
	ctx = customTimeout(ctx, snapshotTimeout)

	_, err := c.scyllaOps.StorageServiceSnapshotsDelete(&operations.StorageServiceSnapshotsDeleteParams{ // nolint: errcheck
		Context: forceHost(ctx, host),
		Tag:     &tag,
		Kn:      pointer.StringPtr(keyspace),
		Cf:      pointer.StringPtr(table),
	})
	return err
}

// TableDiskSize returns total on disk size of the table in bytes.
func (c *Client) TableDiskSize(ctx context.Context, host, keyspace, table string) (int64, error) {
	resp, err := c.scyllaOps.ColumnFamilyMetricsTotalDiskSpaceUsedByNameGet(&operations.ColumnFamilyMetricsTotalDiskSpaceUsedByNameGetParams{
		Context: forceHost(ctx, host),
		Name:    keyspace + ":" + table,
	})
	if err != nil {
		return 0, err
	}
	return resp.Payload, nil
}

// TableExists returns true iff table exists.
func (c *Client) TableExists(ctx context.Context, host, keyspace, table string) (bool, error) {
	if host != "" {
		ctx = forceHost(ctx, host)
	}
	resp, err := c.scyllaOps.ColumnFamilyNameGet(&operations.ColumnFamilyNameGetParams{Context: ctx})
	if err != nil {
		return false, err
	}
	return slice.ContainsString(resp.Payload, keyspace+":"+table), nil
}

// TotalMemory returns Scylla total memory from particular host.
func (c *Client) TotalMemory(ctx context.Context, host string) (int64, error) {
	const (
		queryMetricName = "memory_total_memory"
		metricName      = "scylla_" + queryMetricName
	)

	metrics, err := c.metrics(ctx, host, queryMetricName)
	if err != nil {
		return 0, err
	}

	if _, ok := metrics[metricName]; !ok {
		return 0, errors.New("scylla doest not expose total memory metric")
	}

	var totalMemory int64
	for _, m := range metrics[metricName].Metric {
		switch {
		case m.Counter != nil && m.Counter.Value != nil:
			totalMemory += int64(*m.Counter.Value)
		case m.Gauge != nil && m.Gauge.Value != nil:
			totalMemory += int64(*m.Gauge.Value)
		}
	}

	return totalMemory, nil
}

// HostsTotalMemory runs TotalMemory for many hosts.
func (c *Client) HostsTotalMemory(ctx context.Context, hosts []string) (map[string]int64, error) {
	memory := make([]int64, len(hosts))

	f := func(i int) error {
		mem, err := c.TotalMemory(ctx, hosts[i])
		if err != nil {
			return parallel.Abort(errors.Wrapf(err, "%s: get total memory", hosts[i]))
		}
		memory[i] = mem
		return nil
	}
	if err := parallel.Run(len(hosts), parallel.NoLimit, f, parallel.NopNotify); err != nil {
		return nil, err
	}

	out := make(map[string]int64)
	for i, h := range hosts {
		out[h] = memory[i]
	}
	return out, nil
}

// HostKeyspaceTable is a tuple of Host and Keyspace and Table names.
type HostKeyspaceTable struct {
	Host     string
	Keyspace string
	Table    string
}

// HostKeyspaceTables is a slice of HostKeyspaceTable.
type HostKeyspaceTables []HostKeyspaceTable

// Hosts returns slice of unique hosts.
func (t HostKeyspaceTables) Hosts() []string {
	s := strset.New()
	for _, v := range t {
		s.Add(v.Host)
	}
	return s.List()
}

// TableDiskSizeReport returns total on disk size of tables in bytes.
func (c *Client) TableDiskSizeReport(ctx context.Context, hostKeyspaceTables HostKeyspaceTables) ([]int64, error) {
	// Get shard count of a first node to estimate parallelism limit
	shards, err := c.ShardCount(ctx, "")
	if err != nil {
		return nil, errors.Wrapf(err, "shard count")
	}

	var (
		limit  = len(hostKeyspaceTables.Hosts()) * int(shards)
		report = make([]int64, len(hostKeyspaceTables))
	)

	f := func(i int) error {
		v := hostKeyspaceTables[i]

		size, err := c.TableDiskSize(ctx, v.Host, v.Keyspace, v.Table)
		if err != nil {
			return parallel.Abort(errors.Wrapf(stdErrors.Join(err, ErrHostInvalidResponse), v.Host))
		}
		c.logger.Debug(ctx, "Table disk size",
			"host", v.Host,
			"keyspace", v.Keyspace,
			"table", v.Table,
			"size", size,
		)

		report[i] = size
		return nil
	}

	notify := func(i int, err error) {
		v := hostKeyspaceTables[i]
		c.logger.Error(ctx, "Failed to get table disk size",
			"host", v.Host,
			"keyspace", v.Keyspace,
			"table", v.Table,
			"error", err,
		)
	}

	err = parallel.Run(len(hostKeyspaceTables), limit, f, notify)
	return report, err
}

const loadSSTablesTimeout = time.Hour

// LoadSSTables that are already downloaded to host's table upload directory.
// Used API endpoint has the following properties:
// - It is synchronous - response is received only after the loading has finished
// - It immediately returns an error if called while loading is still happening
// - It returns nil when called on an empty upload dir
// Except for the error, LoadSSTables also checks if loading of SSTables is still happening.
func (c *Client) LoadSSTables(ctx context.Context, host, keyspace, table string, loadAndStream, primaryReplicaOnly bool) (bool, error) {
	const WIPError = "Already loading SSTables"

	_, err := c.scyllaOps.StorageServiceSstablesByKeyspacePost(&operations.StorageServiceSstablesByKeyspacePostParams{
		Context:            customTimeout(forceHost(ctx, host), loadSSTablesTimeout),
		Keyspace:           keyspace,
		Cf:                 table,
		LoadAndStream:      &loadAndStream,
		PrimaryReplicaOnly: &primaryReplicaOnly,
	})

	if err != nil && strings.Contains(err.Error(), WIPError) {
		return true, err
	}
	return false, err
}

// IsAutoCompactionEnabled checks if auto compaction of given table is enabled on the host.
func (c *Client) IsAutoCompactionEnabled(ctx context.Context, host, keyspace, table string) (bool, error) {
	resp, err := c.scyllaOps.ColumnFamilyAutocompactionByNameGet(&operations.ColumnFamilyAutocompactionByNameGetParams{
		Context: forceHost(ctx, host),
		Name:    keyspace + ":" + table,
	})
	if err != nil {
		return false, err
	}
	return resp.Payload, nil
}

// EnableAutoCompaction enables auto compaction on the host.
func (c *Client) EnableAutoCompaction(ctx context.Context, host, keyspace, table string) error {
	_, err := c.scyllaOps.ColumnFamilyAutocompactionByNamePost(&operations.ColumnFamilyAutocompactionByNamePostParams{
		Context: forceHost(ctx, host),
		Name:    keyspace + ":" + table,
	})
	return err
}

// DisableAutoCompaction disables auto compaction on the host.
func (c *Client) DisableAutoCompaction(ctx context.Context, host, keyspace, table string) error {
	_, err := c.scyllaOps.ColumnFamilyAutocompactionByNameDelete(&operations.ColumnFamilyAutocompactionByNameDeleteParams{
		Context: forceHost(ctx, host),
		Name:    keyspace + ":" + table,
	})
	return err
}

// FlushTable flushes writes stored in MemTable into SSTables stored on disk.
func (c *Client) FlushTable(ctx context.Context, host, keyspace, table string) error {
	_, err := c.scyllaOps.StorageServiceKeyspaceFlushByKeyspacePost(&operations.StorageServiceKeyspaceFlushByKeyspacePostParams{
		Cf:       &table,
		Keyspace: keyspace,
		Context:  forceHost(ctx, host),
	})
	return err
}

// ViewBuildStatus returns the earliest (among all nodes) build status for given view.
func (c *Client) ViewBuildStatus(ctx context.Context, keyspace, view string) (ViewBuildStatus, error) {
	resp, err := c.scyllaOps.StorageServiceViewBuildStatusesByKeyspaceAndViewGet(&operations.StorageServiceViewBuildStatusesByKeyspaceAndViewGetParams{
		Context:  ctx,
		Keyspace: keyspace,
		View:     view,
	})
	if err != nil {
		return "", err
	}

	if len(resp.Payload) == 0 {
		return StatusUnknown, nil
	}

	minStatus := StatusSuccess
	for _, v := range resp.Payload {
		status := ViewBuildStatus(v.Value)
		if status.Index() < minStatus.Index() {
			minStatus = status
		}
	}
	return minStatus, nil
}

// ToCanonicalIP replaces ":0:0" in IPv6 addresses with "::"
// ToCanonicalIP("192.168.0.1") -> "192.168.0.1"
// ToCanonicalIP("100:200:0:0:0:0:0:1") -> "100:200::1".
func ToCanonicalIP(host string) string {
	val := net.ParseIP(host)
	if val == nil {
		return host
	}
	return val.String()
}
