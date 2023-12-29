// Copyright (C) 2017 ScyllaDB

package scyllaclient

import (
	"context"
	"net"
	"net/url"
	"time"

	"github.com/scylladb/scylla-operator/pkg/util/timeutc"
	scyllaoperations "github.com/scylladb/scylladb-swagger-go-client/scylladb/gen/v1/client/operations"
)

// Ping checks if host is available using HTTP ping and returns RTT.
// Ping requests are not retried, use this function with caution.
func (c *Client) Ping(ctx context.Context, host string) (time.Duration, error) {
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
	_, err := c.scyllaClient.Operations.SystemUptimeMsGet(&scyllaoperations.SystemUptimeMsGetParams{
		Context: forceHost(ctx, host),
	})
	if err != nil {
		return err
	}
	return nil
}
