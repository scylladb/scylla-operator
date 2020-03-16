// Copyright (C) 2017 ScyllaDB

package scyllaclient

import (
	"context"
	"net"

	"github.com/scylladb/mermaid/internal/httputil/middleware"
	"github.com/scylladb/mermaid/scyllaclient/internal/agent/client/operations"
	"github.com/scylladb/mermaid/scyllaclient/internal/agent/models"
)

// NodeInfo provides basic information about Scylla node.
type NodeInfo models.NodeInfo

// NodeInfo returns basic information about `host` node
func (c *Client) NodeInfo(ctx context.Context, host string) (*NodeInfo, error) {
	p := operations.NodeInfoParams{
		Context: middleware.ForceHost(ctx, host),
	}
	resp, err := c.agentOps.NodeInfo(&p)
	if err != nil {
		return nil, err
	}
	return (*NodeInfo)(resp.Payload), nil
}

// CQLAddr returns CQL address from NodeInfo.
// Scylla can have separate rpc_address (CQL), listen_address and respectfully
// broadcast_rpc_address and broadcast_address if some 3rd party routing
// is added.
// `fallback` argument is used in case any of above addresses is zero address.
func (ni *NodeInfo) CQLAddr(fallback string) string {
	const ipv4Zero, ipv6Zero = "0.0.0.0", "::0"

	if ni.BroadcastRPCAddress != "" {
		return net.JoinHostPort(ni.BroadcastRPCAddress, ni.NativeTransportPort)
	}
	if ni.RPCAddress != "" {
		if ni.RPCAddress == ipv4Zero || ni.RPCAddress == ipv6Zero {
			return net.JoinHostPort(fallback, ni.NativeTransportPort)
		}
		return net.JoinHostPort(ni.RPCAddress, ni.NativeTransportPort)
	}
	if ni.ListenAddress == ipv4Zero || ni.ListenAddress == ipv6Zero {
		return net.JoinHostPort(fallback, ni.NativeTransportPort)
	}

	return net.JoinHostPort(ni.ListenAddress, ni.NativeTransportPort)
}
