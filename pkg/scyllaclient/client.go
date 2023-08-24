package scyllaclient

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"runtime"
	"sort"
	"strings"
	"time"

	api "github.com/go-openapi/runtime/client"
	apiMiddleware "github.com/go-openapi/runtime/middleware"
	"github.com/go-openapi/strfmt"
	"github.com/hailocab/go-hostpool"
	"github.com/scylladb/go-set/strset"
	"github.com/scylladb/scylla-operator/pkg/auth"
	"github.com/scylladb/scylla-operator/pkg/util/httpx"
	scyllaclient "github.com/scylladb/scylladb-swagger-go-client/scylladb/gen/v1/client"
	scyllaoperations "github.com/scylladb/scylladb-swagger-go-client/scylladb/gen/v1/client/operations"
)

func init() {
	// Timeout is defined in http client that we provide in api.NewWithClient.
	// If Context is provided to operation, which is always the case here,
	// this value has no meaning since OpenAPI runtime ignores it.
	api.DefaultTimeout = 0
	// Disable debug output to stderr, it could have been enabled by setting
	// SWAGGER_DEBUG or DEBUG env variables.
	apiMiddleware.Debug = false
}

type Client struct {
	config *Config

	scyllaClient *scyllaclient.ScylladbV1
	transport    http.RoundTripper
	pool         hostpool.HostPool
}

func NewClient(config *Config) (*Client, error) {
	err := config.Validate()
	if err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	hosts := make([]string, len(config.Hosts))
	copy(hosts, config.Hosts)

	pool := hostpool.NewEpsilonGreedy(hosts, config.PoolDecayDuration, &hostpool.LinearEpsilonValueCalculator{})

	if config.Transport == nil {
		config.Transport = DefaultTransport()
	}
	transport := config.Transport
	transport = timeout(transport, config.Timeout)
	transport = requestLogger(transport)
	transport = hostPool(transport, pool, config.Port)
	transport = auth.AddToken(transport, config.AuthToken)
	transport = fixContentType(transport)

	c := &http.Client{Transport: transport}

	scyllaRuntime := api.NewWithClient(
		scyllaclient.DefaultHost, scyllaclient.DefaultBasePath, []string{config.Scheme}, c,
	)
	// Debug can be turned on by SWAGGER_DEBUG or DEBUG env variable
	scyllaRuntime.Debug = false

	scyllaClient := scyllaclient.New(scyllaRuntime, strfmt.Default)

	return &Client{
		config:       config,
		scyllaClient: scyllaClient,
		transport:    transport,
		pool:         pool,
	}, nil
}

func (c *Client) Close() {
	if c.pool != nil {
		c.pool.Close()
	}
}

func (c *Client) Status(ctx context.Context, host string) (NodeStatusInfoSlice, error) {
	if len(host) > 0 {
		// Always query same host
		ctx = forceHost(ctx, host)
	}

	// Get all hosts
	resp, err := c.scyllaClient.Operations.StorageServiceHostIDGet(&scyllaoperations.StorageServiceHostIDGetParams{Context: ctx})
	if err != nil {
		return nil, err
	}

	all := make([]NodeStatusInfo, len(resp.Payload))
	for i, p := range resp.Payload {
		all[i].Addr = p.Key
		all[i].HostID = p.Value
	}

	// Get live nodes
	live, err := c.scyllaClient.Operations.GossiperEndpointLiveGet(&scyllaoperations.GossiperEndpointLiveGetParams{Context: ctx})
	if err != nil {
		return nil, err
	}
	setNodeStatus(all, NodeStatusUp, live.Payload)

	// Get joining nodes
	joining, err := c.scyllaClient.Operations.StorageServiceNodesJoiningGet(&scyllaoperations.StorageServiceNodesJoiningGetParams{Context: ctx})
	if err != nil {
		return nil, err
	}
	setNodeState(all, NodeStateJoining, joining.Payload)

	// Get leaving nodes
	leaving, err := c.scyllaClient.Operations.StorageServiceNodesLeavingGet(&scyllaoperations.StorageServiceNodesLeavingGetParams{Context: ctx})
	if err != nil {
		return nil, err
	}
	setNodeState(all, NodeStateLeaving, leaving.Payload)

	// Get moving nodes
	moving, err := c.scyllaClient.Operations.StorageServiceNodesMovingGet(&scyllaoperations.StorageServiceNodesMovingGetParams{Context: ctx})
	if err != nil {
		return nil, err
	}
	setNodeState(all, NodeStateMoving, moving.Payload)

	// Sort by Host ID
	sort.Slice(all, func(i, j int) bool {
		return all[i].HostID < all[j].HostID
	})

	return all, nil
}

func (c *Client) GetLocalHostId(ctx context.Context, host string, retry bool) (string, error) {
	if len(host) > 0 {
		ctx = forceHost(ctx, host)
	}

	if !retry {
		ctx = noRetry(ctx)
	}

	resp, err := c.scyllaClient.Operations.StorageServiceHostidLocalGet(&scyllaoperations.StorageServiceHostidLocalGetParams{Context: ctx})
	if err != nil {
		return "", err
	}

	return resp.GetPayload(), nil
}

func (c *Client) GetIPToHostIDMap(ctx context.Context, host string) (map[string]string, error) {
	if len(host) > 0 {
		ctx = forceHost(ctx, host)
	}

	resp, err := c.scyllaClient.Operations.StorageServiceHostIDGet(&scyllaoperations.StorageServiceHostIDGetParams{
		Context: ctx,
	})
	if err != nil {
		return nil, err
	}

	mapping := make(map[string]string, len(resp.Payload))

	for _, e := range resp.Payload {
		mapping[e.Key] = e.Value
	}

	return mapping, nil
}

func (c *Client) GetTokenRing(ctx context.Context, host string) ([]string, error) {
	if len(host) > 0 {
		ctx = forceHost(ctx, host)
	}

	resp, err := c.scyllaClient.Operations.StorageServiceTokensGet(&scyllaoperations.StorageServiceTokensGetParams{Context: ctx})
	if err != nil {
		return nil, err
	}

	return resp.GetPayload(), nil
}

func (c *Client) GetNodeTokens(ctx context.Context, host, endpoint string) ([]string, error) {
	ctx = forceHost(ctx, host)

	resp, err := c.scyllaClient.Operations.StorageServiceTokensByEndpointGet(&scyllaoperations.StorageServiceTokensByEndpointGetParams{
		Endpoint: endpoint,
		Context:  ctx,
	})
	if err != nil {
		return nil, err
	}

	return resp.GetPayload(), nil
}

func (c *Client) Cleanup(ctx context.Context, host string, keyspace string) error {
	const (
		// Cleanup is synchronous call and may take a long time to finish.
		// Default request timeout in client is not big enough to clean huge keyspace.
		cleanupTimeout = 24 * time.Hour
	)

	ctx = forceHost(ctx, host)
	ctx = customTimeout(ctx, cleanupTimeout)

	_, err := c.scyllaClient.Operations.StorageServiceKeyspaceCleanupByKeyspacePost(&scyllaoperations.StorageServiceKeyspaceCleanupByKeyspacePostParams{
		Context:  ctx,
		Keyspace: keyspace,
	})
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) StopCleanup(ctx context.Context, host string) error {
	ctx = forceHost(ctx, host)

	_, err := c.scyllaClient.Operations.CompactionManagerStopCompactionPost(&scyllaoperations.CompactionManagerStopCompactionPostParams{
		Context: ctx,
		Type:    string(CleanupCompactionType),
	})
	if err != nil {
		return err
	}

	return nil
}

const (
	snapshotTimeout = 5 * time.Minute
	drainTimeout    = 5 * time.Minute
)

// Keyspaces return a list of all the keyspaces.
func (c *Client) Keyspaces(ctx context.Context) ([]string, error) {
	resp, err := c.scyllaClient.Operations.StorageServiceKeyspacesGet(&scyllaoperations.StorageServiceKeyspacesGetParams{Context: ctx})
	if err != nil {
		return nil, err
	}
	return resp.Payload, nil
}

// Snapshots lists available snapshots.
func (c *Client) Snapshots(ctx context.Context, host string) ([]string, error) {
	ctx = customTimeout(ctx, snapshotTimeout)

	resp, err := c.scyllaClient.Operations.StorageServiceSnapshotsGet(&scyllaoperations.StorageServiceSnapshotsGetParams{
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

// TakeSnapshot flushes and takes a snapshot of a keyspace.
// Multiple keyspaces may have the same tag.
func (c *Client) TakeSnapshot(ctx context.Context, host, tag, keyspace string, tables ...string) error {
	ctx = customTimeout(ctx, snapshotTimeout)

	var cfPtr *string

	if len(tables) > 0 {
		v := strings.Join(tables, ",")
		cfPtr = &v
	}

	if _, err := c.scyllaClient.Operations.StorageServiceKeyspaceFlushByKeyspacePost(&scyllaoperations.StorageServiceKeyspaceFlushByKeyspacePostParams{ // nolint: errcheck
		Context:  forceHost(ctx, host),
		Keyspace: keyspace,
		Cf:       cfPtr,
	}); err != nil {
		return err
	}

	if _, err := c.scyllaClient.Operations.StorageServiceSnapshotsPost(&scyllaoperations.StorageServiceSnapshotsPostParams{ // nolint: errcheck
		Context: forceHost(ctx, host),
		Tag:     &tag,
		Kn:      &keyspace,
		Cf:      cfPtr,
	}); err != nil {
		return err
	}

	return nil
}

// DeleteSnapshot removes a snapshot with a given tag.
func (c *Client) DeleteSnapshot(ctx context.Context, host, tag string) error {
	ctx = customTimeout(ctx, snapshotTimeout)

	_, err := c.scyllaClient.Operations.StorageServiceSnapshotsDelete(&scyllaoperations.StorageServiceSnapshotsDeleteParams{ // nolint: errcheck
		Context: forceHost(ctx, host),
		Tag:     &tag,
	})
	return err
}

// Drain makes node unavailable for writes, flushes memtables and replays commitlog
func (c *Client) Drain(ctx context.Context, host string) error {
	ctx = customTimeout(ctx, drainTimeout)

	if _, err := c.scyllaClient.Operations.StorageServiceDrainPost(&scyllaoperations.StorageServiceDrainPostParams{ // nolint: errcheck
		Context: forceHost(ctx, host),
	}); err != nil {
		return err
	}

	return nil
}

func (c *Client) Decommission(ctx context.Context, host string) error {
	queryCtx := forceHost(ctx, host)
	// On decommission request api server waits till decommission is completed
	// Usually decommission takes significant amount of time therefore request is failing by timeout
	// As result of the scylla client will retry it and get 500 response that is saying that
	//   decommission is already in progress.
	// To avoid that we pass noRetry to the context
	queryCtx = noRetry(queryCtx)
	_, err := c.scyllaClient.Operations.StorageServiceDecommissionPost(&scyllaoperations.StorageServiceDecommissionPostParams{Context: queryCtx})
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) ScyllaVersion(ctx context.Context) (string, error) {
	resp, err := c.scyllaClient.Operations.StorageServiceScyllaReleaseVersionGet(&scyllaoperations.StorageServiceScyllaReleaseVersionGetParams{Context: ctx})
	if err != nil {
		return "", err
	}
	return resp.Payload, nil
}

func (c *Client) OperationMode(ctx context.Context, host string) (OperationalMode, error) {
	resp, err := c.scyllaClient.Operations.StorageServiceOperationModeGet(&scyllaoperations.StorageServiceOperationModeGetParams{Context: forceHost(ctx, host)})
	if err != nil {
		return "", err
	}
	return operationalModeFromString(resp.Payload), nil
}

func (c *Client) IsNativeTransportEnabled(ctx context.Context, host string) (bool, error) {
	resp, err := c.scyllaClient.Operations.StorageServiceNativeTransportGet(&scyllaoperations.StorageServiceNativeTransportGetParams{Context: forceHost(ctx, host)})
	if err != nil {
		return false, err
	}
	return resp.Payload, nil
}

func (c *Client) HasSchemaAgreement(ctx context.Context) (bool, error) {
	resp, err := c.scyllaClient.Operations.StorageProxySchemaVersionsGet(&scyllaoperations.StorageProxySchemaVersionsGetParams{Context: ctx})
	if err != nil {
		return false, err
	}
	versions := map[string]struct{}{}
	for _, kv := range resp.Payload {
		versions[kv.Key] = struct{}{}
	}

	return len(versions) == 1, nil
}

func DefaultTransport() *http.Transport {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		MaxIdleConnsPerHost:   runtime.GOMAXPROCS(0) + 1,

		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
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

// fixContentType adjusts Scylla REST API response so that it can be consumed
// by Open API.
func fixContentType(next http.RoundTripper) http.RoundTripper {
	return httpx.RoundTripperFunc(func(req *http.Request) (resp *http.Response, err error) {
		defer func() {
			if resp != nil {
				// Force JSON, Scylla returns "text/plain" that misleads the
				// unmarshaller and breaks processing.
				resp.Header.Set("Content-Type", "application/json")
			}
		}()
		return next.RoundTrip(req)
	})
}
