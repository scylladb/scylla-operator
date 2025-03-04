// Copyright (c) 2023 ScyllaDB.

package scyllaclient

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	api "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
	"github.com/scylladb/scylla-operator/pkg/auth"
	scyllaclient "github.com/scylladb/scylladb-swagger-go-client/scylladb/gen/v2/client"
	"github.com/scylladb/scylladb-swagger-go-client/scylladb/gen/v2/client/config"
)

const (
	agentPort      = "10001"
	defaultTimeout = 30 * time.Second
)

type ConfigClient struct {
	client *scyllaclient.ScylladbV2
}

func NewConfigClient(host, authToken string) *ConfigClient {
	var transport http.RoundTripper = DefaultTransport()
	transport = fixContentType(transport)
	transport = auth.AddToken(transport, authToken)

	client := &http.Client{
		Timeout:   defaultTimeout,
		Transport: transport,
	}

	host = net.JoinHostPort(host, agentPort)

	scyllaV2Runtime := api.NewWithClient(
		host, scyllaclient.DefaultBasePath, scyllaclient.DefaultSchemes, client,
	)

	return &ConfigClient{
		client: scyllaclient.New(scyllaV2Runtime, strfmt.Default),
	}
}

func (c *ConfigClient) BroadcastRPCAddress(ctx context.Context) (string, error) {
	resp, err := c.client.Config.FindConfigBroadcastRPCAddress(config.NewFindConfigBroadcastRPCAddressParamsWithContext(ctx))
	if err != nil {
		return "", fmt.Errorf("can't get broadcast_rpc_address: %w", err)
	}
	return resp.Payload, nil
}

func (c *ConfigClient) BroadcastAddress(ctx context.Context) (string, error) {
	resp, err := c.client.Config.FindConfigBroadcastAddress(config.NewFindConfigBroadcastAddressParamsWithContext(ctx))
	if err != nil {
		return "", fmt.Errorf("can't get broadcast_address: %w", err)
	}
	return resp.Payload, nil
}

// ReplaceAddressFirstBoot returns value of "replace_address_first_boot" config parameter.
func (c *ConfigClient) ReplaceAddressFirstBoot(ctx context.Context) (string, error) {
	resp, err := c.client.Config.FindConfigReplaceAddressFirstBoot(config.NewFindConfigReplaceAddressFirstBootParamsWithContext(ctx))
	if err != nil {
		return "", fmt.Errorf("can't get replace_address_first_boot: %w", err)
	}
	return resp.Payload, nil
}

// ReplaceNodeFirstBoot returns value of "replace_node_first_boot" config parameter.
func (c *ConfigClient) ReplaceNodeFirstBoot(ctx context.Context) (string, error) {
	resp, err := c.client.Config.FindConfigReplaceNodeFirstBoot(config.NewFindConfigReplaceNodeFirstBootParamsWithContext(ctx))
	if err != nil {
		return "", fmt.Errorf("can't get replace_node_first_boot: %w", err)
	}
	return resp.Payload, nil
}

// ReadRequestTimeoutInMs returns value of "read_request_timeout_in_ms" config parameter.
func (c *ConfigClient) ReadRequestTimeoutInMs(ctx context.Context) (int64, error) {
	resp, err := c.client.Config.FindConfigReadRequestTimeoutInMs(config.NewFindConfigReadRequestTimeoutInMsParamsWithContext(ctx))
	if err != nil {
		return 0, fmt.Errorf("can't get read_request_timeout_in_ms: %w", err)
	}
	return resp.Payload, nil
}
