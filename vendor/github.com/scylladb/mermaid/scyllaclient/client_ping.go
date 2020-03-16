// Copyright (C) 2017 ScyllaDB

package scyllaclient

import (
	"context"
	"io"
	"io/ioutil"
	"math"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"runtime"
	"sort"
	"sync"
	"time"

	apiRuntime "github.com/go-openapi/runtime"
	"github.com/pkg/errors"
	"github.com/scylladb/mermaid/internal/httputil/middleware"
	"github.com/scylladb/mermaid/internal/timeutc"
)

// CheckHostsConnectivity returns a slice of errors, error at position i
// corresponds to host at position i.
func (c *Client) CheckHostsConnectivity(ctx context.Context, hosts []string) []error {
	c.logger.Info(ctx, "Checking hosts connectivity", "hosts", hosts)

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
				rtt, err := c.PingN(ctx, h, 3, 0)
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
		return nil, errors.New("failed to connect to any node")
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

// PingN does "n" amount of pings towards the host and returns average RTT
// across all results.
// Pings are tried sequentially and if any of the pings fail function will
// return an error.
func (c *Client) PingN(ctx context.Context, host string, n int, timeout time.Duration) (time.Duration, error) {
	// Open connection to server.
	_, err := c.Ping(ctx, host)
	if err != nil {
		return 0, err
	}

	// Limit the running time of many loops to timeout
	if timeout == 0 {
		timeout = c.config.RequestTimeout
	}
	ctxWithTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Measure avg host RTT.
	var sum time.Duration
	for i := 0; i < n; i++ {
		d, err := c.Ping(ctxWithTimeout, host)
		if err != nil {
			return 0, err
		}
		sum += d
	}
	rtt := sum / time.Duration(n)

	return rtt, nil
}

// Ping checks if host is available using HTTP ping and returns RTT.
// Ping requests are not retried, use this function with caution.
func (c *Client) Ping(ctx context.Context, host string) (time.Duration, error) {
	ctx = middleware.DontRetry(ctx)

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
	u := c.newURL(host, "/")
	r, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return errors.Wrap(err, "create http request")
	}
	r = r.WithContext(middleware.ForceHost(ctx, host))

	resp, err := c.transport.RoundTrip(r)
	if resp != nil {
		io.Copy(ioutil.Discard, io.LimitReader(resp.Body, 1024)) // nolint: errcheck
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
			err = apiRuntime.NewAPIError("unknown error", nil, resp.StatusCode)
		}
	}
	return err
}
