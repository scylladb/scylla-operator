// Copyright (C) 2017 ScyllaDB

package scyllaclient

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	api "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
	"github.com/pkg/errors"
	scyllaV2Client "github.com/scylladb/scylla-manager/v3/swagger/gen/scylla/v2/client"
	"github.com/scylladb/scylla-manager/v3/swagger/gen/scylla/v2/client/config"
	"github.com/scylladb/scylla-manager/v3/swagger/gen/scylla/v2/models"
)

// ConfigClient provides means to interact with Scylla config API on a given
// host if it's directly accessible.
type ConfigClient struct {
	addr   string
	client *scyllaV2Client.ScyllaV2
}

func NewConfigClient(addr string) *ConfigClient {
	setOpenAPIGlobals()

	t := http.DefaultTransport
	t = fixContentType(t)
	c := &http.Client{
		Timeout:   30 * time.Second,
		Transport: t,
	}

	scyllaV2Runtime := api.NewWithClient(
		addr, scyllaV2Client.DefaultBasePath, scyllaV2Client.DefaultSchemes, c,
	)

	return &ConfigClient{
		addr:   addr,
		client: scyllaV2Client.New(scyllaV2Runtime, strfmt.Default),
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

// NativeTransportPortSSL returns node listen SSL port.
func (c *ConfigClient) NativeTransportPortSSL(ctx context.Context) (string, error) {
	resp, err := c.client.Config.FindConfigNativeTransportPortSsl(config.NewFindConfigNativeTransportPortSslParamsWithContext(ctx))
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

// DataDirectory returns node data directory.
func (c *ConfigClient) DataDirectory(ctx context.Context) (string, error) {
	resp, err := c.client.Config.FindConfigDataFileDirectories(config.NewFindConfigDataFileDirectoriesParamsWithContext(ctx))
	if err != nil {
		return "", err
	}
	if len(resp.Payload) == 0 {
		return "", nil
	}
	return resp.Payload[0], nil
}

// ClientEncryptionOptions represents Client encryption configuration options.
type ClientEncryptionOptions = models.ClientEncryptionOptions

// ClientEncryptionOptions returns if client encryption options.
func (c *ConfigClient) ClientEncryptionOptions(ctx context.Context) (*ClientEncryptionOptions, error) {
	resp, err := c.client.Config.FindConfigClientEncryptionOptions(config.NewFindConfigClientEncryptionOptionsParamsWithContext(ctx))
	if err != nil {
		return nil, err
	}
	return resp.Payload, err
}

const passwordAuthenticator = "PasswordAuthenticator"

// CQLPasswordProtectionEnabled returns if CQL Username/Password authentication is enabled.
func (c *ConfigClient) CQLPasswordProtectionEnabled(ctx context.Context) (bool, error) {
	resp, err := c.client.Config.FindConfigAuthenticator(config.NewFindConfigAuthenticatorParamsWithContext(ctx))
	if err != nil {
		return false, err
	}
	return strings.Replace(resp.Payload, "org.apache.cassandra.auth.", "", 1) == passwordAuthenticator, err
}

// AlternatorPort returns node alternator port.
func (c *ConfigClient) AlternatorPort(ctx context.Context) (string, error) {
	resp, err := c.client.Config.FindConfigAlternatorPort(config.NewFindConfigAlternatorPortParamsWithContext(ctx))
	if isStatusCode400(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return fmt.Sprint(resp.Payload), err
}

// AlternatorAddress returns node alternator address.
func (c *ConfigClient) AlternatorAddress(ctx context.Context) (string, error) {
	resp, err := c.client.Config.FindConfigAlternatorAddress(config.NewFindConfigAlternatorAddressParamsWithContext(ctx))
	if isStatusCode400(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return resp.Payload, err
}

// AlternatorHTTPSPort returns node alternator HTTPS port.
func (c *ConfigClient) AlternatorHTTPSPort(ctx context.Context) (string, error) {
	resp, err := c.client.Config.FindConfigAlternatorHTTPSPort(config.NewFindConfigAlternatorHTTPSPortParamsWithContext(ctx))
	if isStatusCode400(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return fmt.Sprint(resp.Payload), err
}

// UUIDSStableIdentifiers returns if node is using uuid-like sstable naming.
func (c *ConfigClient) UUIDSStableIdentifiers(ctx context.Context) (bool, error) {
	resp, err := c.client.Config.FindConfigUUIDSstableIdentifiersEnabled(config.NewFindConfigUUIDSstableIdentifiersEnabledParamsWithContext(ctx))
	if isStatusCode400(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return resp.Payload, err
}

// ConsistentClusterManagement returns true if node uses RAFT for cluster management and DDL.
func (c *ConfigClient) ConsistentClusterManagement(ctx context.Context) (bool, error) {
	resp, err := c.client.Config.FindConfigConsistentClusterManagement(config.NewFindConfigConsistentClusterManagementParamsWithContext(ctx))
	if isStatusCode400(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return resp.Payload, err
}

// AlternatorEnforceAuthorization returns whether alternator requires authorization.
func (c *ConfigClient) AlternatorEnforceAuthorization(ctx context.Context) (bool, error) {
	resp, err := c.client.Config.FindConfigAlternatorEnforceAuthorization(config.NewFindConfigAlternatorEnforceAuthorizationParamsWithContext(ctx))
	if isStatusCode400(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return resp.Payload, err
}

func isStatusCode400(err error) bool {
	// Scylla will return 400 when alternator is disabled, for example:
	// {"message": "No such config entry: alternator_port", "code": 400}
	return StatusCodeOf(err) == http.StatusBadRequest
}

// NodeInfo returns aggregated information about Scylla node.
func (c *ConfigClient) NodeInfo(ctx context.Context) (*NodeInfo, error) {
	apiAddress, apiPort, err := net.SplitHostPort(c.addr)
	if err != nil {
		return nil, errors.Wrapf(err, "split %s into host port chunks", c.addr)
	}

	ceo, err := c.ClientEncryptionOptions(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "fetch Scylla config client encryption enabled")
	}

	ni := &NodeInfo{
		APIAddress:                  apiAddress,
		APIPort:                     apiPort,
		ClientEncryptionEnabled:     strings.EqualFold(ceo.Enabled, "true"),
		ClientEncryptionRequireAuth: strings.EqualFold(ceo.RequireClientAuth, "true"),
	}

	ffs := []struct {
		Field   *string
		Fetcher func(context.Context) (string, error)
	}{
		{Field: &ni.BroadcastAddress, Fetcher: c.BroadcastAddress},
		{Field: &ni.BroadcastRPCAddress, Fetcher: c.BroadcastRPCAddress},
		{Field: &ni.ListenAddress, Fetcher: c.ListenAddress},
		{Field: &ni.NativeTransportPort, Fetcher: c.NativeTransportPort},
		{Field: &ni.NativeTransportPortSsl, Fetcher: c.NativeTransportPortSSL},
		{Field: &ni.PrometheusAddress, Fetcher: c.PrometheusAddress},
		{Field: &ni.PrometheusPort, Fetcher: c.PrometheusPort},
		{Field: &ni.RPCAddress, Fetcher: c.RPCAddress},
		{Field: &ni.RPCPort, Fetcher: c.RPCPort},
		{Field: &ni.AlternatorAddress, Fetcher: c.AlternatorAddress},
		{Field: &ni.AlternatorPort, Fetcher: c.AlternatorPort},
		{Field: &ni.AlternatorHTTPSPort, Fetcher: c.AlternatorHTTPSPort},
	}

	for i, ff := range ffs {
		*ff.Field, err = ff.Fetcher(ctx)
		if err != nil {
			return nil, errors.Wrapf(err, "agent: fetch Scylla config %d", i)
		}
	}

	ffb := []struct {
		Field   *bool
		Fetcher func(context.Context) (bool, error)
	}{
		{Field: &ni.CqlPasswordProtected, Fetcher: c.CQLPasswordProtectionEnabled},
		{Field: &ni.AlternatorEnforceAuthorization, Fetcher: c.AlternatorEnforceAuthorization},
		{Field: &ni.SstableUUIDFormat, Fetcher: c.UUIDSStableIdentifiers},
		{Field: &ni.ConsistentClusterManagement, Fetcher: c.ConsistentClusterManagement},
	}

	for i, ff := range ffb {
		*ff.Field, err = ff.Fetcher(ctx)
		if err != nil {
			return nil, errors.Wrapf(err, "agent: fetch Scylla config %d", i)
		}
	}

	return ni, nil
}
