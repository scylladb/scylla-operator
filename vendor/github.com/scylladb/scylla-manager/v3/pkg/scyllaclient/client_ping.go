// Copyright (C) 2017 ScyllaDB

package scyllaclient

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/scylladb/scylla-manager/v3/pkg/util/parallel"
	"github.com/scylladb/scylla-manager/v3/pkg/util/timeutc"
	"github.com/scylladb/scylla-manager/v3/swagger/gen/scylla/v1/client/operations"
	"go.uber.org/multierr"
)

// GetNodesWithLocationAccess returns subset of nodes which have access to remote location.
func (c *Client) GetNodesWithLocationAccess(ctx context.Context, nodes NodeStatusInfoSlice, remotePath string) (NodeStatusInfoSlice, error) {
	nodeErr := make([]error, len(nodes))
	err := parallel.Run(len(nodes), parallel.NoLimit, func(i int) error {
		n := nodes[i]

		err := c.RcloneCheckPermissions(ctx, n.Addr, remotePath)
		if err == nil {
			c.logger.Info(ctx, "Host location access check OK",
				"host", n.Addr,
				"location", remotePath,
			)
		} else {
			c.logger.Info(ctx, "Host location access check FAILED",
				"hosts", n.Addr,
				"location", remotePath,
				"err", err,
			)
		}
		nodeErr[i] = err

		return nil
	}, parallel.NopNotify)
	if err != nil {
		return nil, errors.Wrap(err, "check location access")
	}

	checked := make(NodeStatusInfoSlice, 0)
	for i, err := range nodeErr {
		if err == nil {
			checked = append(checked, nodes[i])
		}
	}

	if len(checked) == 0 {
		combinedErr := multierr.Combine(nodeErr...)
		return nil, errors.Wrapf(combinedErr, "no nodes with access to loaction: %s", remotePath)
	}

	return checked, nil
}

// GetLiveNodes returns subset of nodes that passed connectivity check.
func (c *Client) GetLiveNodes(ctx context.Context, nodes NodeStatusInfoSlice) (NodeStatusInfoSlice, error) {
	var (
		liveNodes NodeStatusInfoSlice
		nodeErr   = c.CheckHostsConnectivity(ctx, nodes.Hosts())
	)

	for i, err := range nodeErr {
		if err == nil {
			liveNodes = append(liveNodes, nodes[i])
		}
	}
	if len(liveNodes) == 0 {
		return nil, errors.Errorf("no live nodes")
	}

	return liveNodes, nil
}

// CheckHostsConnectivity returns a slice of errors, error at position i
// corresponds to host at position i.
func (c *Client) CheckHostsConnectivity(ctx context.Context, hosts []string) []error {
	c.logger.Info(ctx, "Checking hosts connectivity", "hosts", hosts)
	defer c.logger.Info(ctx, "Done checking hosts connectivity")

	size := len(hosts)

	var wg sync.WaitGroup
	wg.Add(size)

	errs := make([]error, size)
	for i := range hosts {
		go func(i int) {
			err := c.ping(ctx, hosts[i])
			if err == nil {
				c.logger.Info(ctx, "Host check OK", "host", hosts[i])
			} else {
				c.logger.Info(ctx, "Host check FAILED", "hosts", hosts[i], "err", err)
			}
			errs[i] = err
			wg.Done()
		}(i)
	}

	wg.Wait()

	return errs
}

// ClosestDC takes output of Datacenters, a map from DC to it's hosts and
// returns DCs sorted by speed the hosts respond. It's determined by
// the lowest latency over 3 Ping() invocations across random selection of
// hosts for each DC.
func (c *Client) ClosestDC(ctx context.Context, dcs map[string][]string) ([]string, error) {
	c.logger.Info(ctx, "Measuring datacenter latencies", "dcs", extractKeys(dcs))

	if len(dcs) == 0 {
		return nil, errors.Errorf("no dcs to choose from")
	}

	// Single DC no need to measure anything.
	if len(dcs) == 1 {
		for dc := range dcs {
			return []string{dc}, nil
		}
	}

	type dcRTT struct {
		dc  string
		rtt time.Duration
	}
	out := make(chan dcRTT, runtime.NumCPU()+1)
	size := 0

	// Test latency of 3 random hosts from each DC.
	for dc, hosts := range dcs {
		dc := dc
		hosts := pickNRandomHosts(3, hosts)
		size += len(hosts)

		for _, h := range hosts {
			go func(h string) {
				c.logger.Debug(ctx, "Measuring host RTT", "dc", dc, "host", h)
				rtt, err := c.Ping(ctx, h, 0)
				if err != nil {
					c.logger.Info(ctx, "Host RTT measurement failed",
						"dc", dc,
						"host", h,
						"err", err,
					)
					rtt = math.MaxInt64
				} else {
					c.logger.Debug(ctx, "Host RTT", "dc", dc, "host", h, "rtt", rtt)
				}
				out <- dcRTT{dc: dc, rtt: rtt}
			}(h)
		}
	}

	// Select the lowest latency for each DC.
	min := make(map[string]time.Duration, len(dcs))
	for i := 0; i < size; i++ {
		v := <-out
		if m, ok := min[v.dc]; !ok || m > v.rtt {
			min[v.dc] = v.rtt
		}
	}

	// Sort DCs by lowest latency.
	sorted := make([]string, 0, len(dcs))
	for dc := range dcs {
		sorted = append(sorted, dc)
	}
	sort.Slice(sorted, func(i, j int) bool {
		return min[sorted[i]] < min[sorted[j]]
	})

	// All hosts failed...
	if min[sorted[0]] == math.MaxInt64 {
		return nil, errors.New("could not connect to any node")
	}

	c.logger.Info(ctx, "Datacenters by latency (dec)", "dcs", sorted)

	return sorted, nil
}

func extractKeys(m map[string][]string) (keys []string) {
	for k := range m {
		keys = append(keys, k)
	}
	return
}

func pickNRandomHosts(n int, hosts []string) []string {
	if n >= len(hosts) {
		return hosts
	}

	rand := rand.New(rand.NewSource(timeutc.Now().UnixNano()))

	idxs := make(map[int]struct{})
	rh := make([]string, 0, n)
	for ; n > 0; n-- {
		idx := rand.Intn(len(hosts))
		if _, ok := idxs[idx]; !ok {
			idxs[idx] = struct{}{}
			rh = append(rh, hosts[idx])
		} else {
			n++
		}
	}
	return rh
}

// Ping checks if host is available using HTTP ping and returns RTT.
// Ping requests are not retried, use this function with caution.
func (c *Client) Ping(ctx context.Context, host string, timeout time.Duration) (time.Duration, error) {
	if timeout == 0 {
		timeout = c.config.Timeout
	}

	if ctxTimeout, hasCustomTimeout := hasCustomTimeout(ctx); hasCustomTimeout {
		timeout = min(ctxTimeout, timeout)
	}

	ctx = customTimeout(ctx, timeout)
	ctx = noRetry(ctx)

	t := timeutc.Now()
	err := c.ping(ctx, host)
	return timeutc.Since(t), err
}

func (c *Client) newURL(host, path string) url.URL {
	port := "80"
	if c.config.Scheme == "https" {
		port = "443"
	}

	return url.URL{
		Scheme: c.config.Scheme,
		Host:   net.JoinHostPort(host, port),
		Path:   path,
	}
}

func (c *Client) ping(ctx context.Context, host string) error {
	_, err := c.scyllaOps.StorageServiceScyllaReleaseVersionGet(&operations.StorageServiceScyllaReleaseVersionGetParams{
		Context: forceHost(ctx, host),
	})
	if err != nil {
		return err
	}
	return nil
}

func min(a, b time.Duration) time.Duration {
	if a > b {
		return b
	}
	return a
}

// PingAgent is a simple heartbeat ping to agent.
func (c *Client) PingAgent(ctx context.Context, host string, timeout time.Duration) (time.Duration, error) {
	if timeout == 0 {
		timeout = c.config.Timeout
	}
	if ctxTimeout, hasCustomTimeout := hasCustomTimeout(ctx); hasCustomTimeout {
		timeout = min(ctxTimeout, timeout)
	}
	ctx = customTimeout(ctx, timeout)
	ctx = noRetry(ctx)

	u := c.newURL(host, "/ping")
	req, err := http.NewRequestWithContext(forceHost(ctx, host), http.MethodGet, u.String(), bytes.NewReader(nil))
	if err != nil {
		return 0, err
	}

	t := timeutc.Now()
	resp, err := c.client.Do("PingAgent", req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return 0, fmt.Errorf("expected %d status code from ping response, got %d", http.StatusNoContent, resp.StatusCode)
	}
	return timeutc.Since(t), nil
}
