// Copyright (C) 2017 ScyllaDB

package scyllaclient

import (
	"context"
	"fmt"
	"net"

	"github.com/go-openapi/strfmt"
	"github.com/pkg/errors"
	api "github.com/scylladb/mermaid/scyllaclient/internal/scylla_v2/client"
	"github.com/scylladb/mermaid/scyllaclient/internal/scylla_v2/client/config"
)

// ConfigClient provides means to interact with Scylla config API on a given
// host if it's directly accessible.
type ConfigClient struct {
	addr   string
	client *api.Scylla2
}

func NewConfigClient(addr string) *ConfigClient {
	setOpenAPIGlobals()

	t := api.DefaultTransportConfig().WithHost(addr)
	return &ConfigClient{
		addr:   addr,
		client: api.NewHTTPClientWithConfig(strfmt.Default, t),
	}
}

// ListenAddress returns node listen address.
func (c *ConfigClient) ListenAddress(ctx context.Context) (string, error) {
	resp, err := c.client.Config.FindConfigListenAddress(config.NewFindConfigListenAddressParamsWithContext(ctx))
	if err != nil {
		return "", err
	}
	return resp.Payload, err
}

// NativeTransportPort returns node listen port.
func (c *ConfigClient) NativeTransportPort(ctx context.Context) (string, error) {
	resp, err := c.client.Config.FindConfigNativeTransportPort(config.NewFindConfigNativeTransportPortParamsWithContext(ctx))
	if err != nil {
		return "", err
	}
	return fmt.Sprint(resp.Payload), err
}

// RPCAddress returns node rpc address.
func (c *ConfigClient) RPCAddress(ctx context.Context) (string, error) {
	resp, err := c.client.Config.FindConfigRPCAddress(config.NewFindConfigRPCAddressParamsWithContext(ctx))
	if err != nil {
		return "", err
	}
	return resp.Payload, err
}

// RPCPort returns node rpc port.
func (c *ConfigClient) RPCPort(ctx context.Context) (string, error) {
	resp, err := c.client.Config.FindConfigRPCPort(config.NewFindConfigRPCPortParamsWithContext(ctx))
	if err != nil {
		return "", err
	}
	return fmt.Sprint(resp.Payload), err
}

// BroadcastAddress returns node broadcast address.
func (c *ConfigClient) BroadcastAddress(ctx context.Context) (string, error) {
	resp, err := c.client.Config.FindConfigBroadcastAddress(config.NewFindConfigBroadcastAddressParamsWithContext(ctx))
	if err != nil {
		return "", err
	}
	return resp.Payload, err
}

// BroadcastRPCAddress returns node broadcast rpc address.
func (c *ConfigClient) BroadcastRPCAddress(ctx context.Context) (string, error) {
	resp, err := c.client.Config.FindConfigBroadcastRPCAddress(config.NewFindConfigBroadcastRPCAddressParamsWithContext(ctx))
	if err != nil {
		return "", err
	}
	return resp.Payload, err
}

// PrometheusAddress returns node prometheus address.
func (c *ConfigClient) PrometheusAddress(ctx context.Context) (string, error) {
	resp, err := c.client.Config.FindConfigPrometheusAddress(config.NewFindConfigPrometheusAddressParamsWithContext(ctx))
	if err != nil {
		return "", err
	}
	return resp.Payload, err
}

// PrometheusPort returns node prometheus port.
func (c *ConfigClient) PrometheusPort(ctx context.Context) (string, error) {
	resp, err := c.client.Config.FindConfigPrometheusPort(config.NewFindConfigPrometheusPortParamsWithContext(ctx))
	if err != nil {
		return "", err
	}
	return fmt.Sprint(resp.Payload), err
}

// NodeInfo returns aggregated information about Scylla node.
func (c *ConfigClient) NodeInfo(ctx context.Context) (*NodeInfo, error) {
	apiAddress, apiPort, err := net.SplitHostPort(c.addr)
	if err != nil {
		return nil, errors.Wrapf(err, "split %s into host port chunks", c.addr)
	}

	ni := &NodeInfo{
		APIAddress: apiAddress,
		APIPort:    apiPort,
	}

	ffs := []struct {
		Field   *string
		Fetcher func(context.Context) (string, error)
	}{
		{Field: &ni.BroadcastAddress, Fetcher: c.BroadcastAddress},
		{Field: &ni.BroadcastRPCAddress, Fetcher: c.BroadcastRPCAddress},
		{Field: &ni.ListenAddress, Fetcher: c.ListenAddress},
		{Field: &ni.NativeTransportPort, Fetcher: c.NativeTransportPort},
		{Field: &ni.PrometheusAddress, Fetcher: c.PrometheusAddress},
		{Field: &ni.PrometheusPort, Fetcher: c.PrometheusPort},
		{Field: &ni.RPCAddress, Fetcher: c.RPCAddress},
		{Field: &ni.RPCPort, Fetcher: c.RPCPort},
	}

	for i, ff := range ffs {
		*ff.Field, err = ff.Fetcher(ctx)
		if err != nil {
			return nil, errors.Wrapf(err, "fetch Scylla config %d", i)
		}
	}

	return ni, nil
}
