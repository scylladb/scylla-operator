package scyllaclient

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"runtime"
	"sort"
	"sync"
	"time"

	api "github.com/go-openapi/runtime/client"
	apiMiddleware "github.com/go-openapi/runtime/middleware"
	"github.com/go-openapi/strfmt"
	"github.com/hailocab/go-hostpool"
	"github.com/scylladb/go-log"
	"github.com/scylladb/go-set/strset"
	"github.com/scylladb/scylla-operator/pkg/auth"
	scyllaClient "github.com/scylladb/scylla-operator/pkg/scyllaclient/internal/scylla/client"
	scyllaOperations "github.com/scylladb/scylla-operator/pkg/scyllaclient/internal/scylla/client/operations"
	"github.com/scylladb/scylla-operator/pkg/util/httpx"
)

type Client struct {
	config Config
	logger log.Logger

	scyllaOps *scyllaOperations.Client
	transport http.RoundTripper

	mu      sync.RWMutex
	dcCache map[string]string
}

func NewClient(config Config, logger log.Logger) (*Client, error) {
	/*if err := config.Validate(); err != nil {
		return nil, errors.Wrap(err, "invalid config")
	}*/
	setOpenAPIGlobals()
	hosts := make([]string, len(config.Hosts))
	copy(hosts, config.Hosts)

	pool := hostpool.NewEpsilonGreedy(hosts, config.PoolDecayDuration, &hostpool.LinearEpsilonValueCalculator{})

	if config.Transport == nil {
		config.Transport = DefaultTransport()
	}
	transport := config.Transport
	transport = timeout(transport, config.Timeout)
	transport = requestLogger(transport, logger)
	transport = hostPool(transport, pool, config.Port)
	transport = auth.AddToken(transport, config.AuthToken)
	transport = fixContentType(transport)

	c := &http.Client{Transport: transport}

	scyllaRuntime := api.NewWithClient(
		scyllaClient.DefaultHost, scyllaClient.DefaultBasePath, []string{config.Scheme}, c,
	)
	// Debug can be turned on by SWAGGER_DEBUG or DEBUG env variable
	scyllaRuntime.Debug = false

	scyllaOps := scyllaOperations.New(retryable(scyllaRuntime, config, logger), strfmt.Default)

	return &Client{
		config:    config,
		logger:    logger,
		scyllaOps: scyllaOps,
		transport: transport,
		dcCache:   make(map[string]string),
	}, nil
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

	resp, err := c.scyllaOps.SnitchDatacenterGet(&scyllaOperations.SnitchDatacenterGetParams{
		Context: ctx,
		Host:    &host,
	})
	if err != nil {
		return "", err
	}
	dc = resp.Payload

	return
}

func (c *Client) Status(ctx context.Context, host string) (NodeStatusInfoSlice, error) {
	// Always query same host
	ctx = forceHost(ctx, host)

	// Get all hosts
	resp, err := c.scyllaOps.StorageServiceHostIDGet(&scyllaOperations.StorageServiceHostIDGetParams{Context: ctx})
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
	live, err := c.scyllaOps.GossiperEndpointLiveGet(&scyllaOperations.GossiperEndpointLiveGetParams{Context: ctx})
	if err != nil {
		return nil, err
	}
	setNodeStatus(all, NodeStatusUp, live.Payload)

	// Get joining nodes
	joining, err := c.scyllaOps.StorageServiceNodesJoiningGet(&scyllaOperations.StorageServiceNodesJoiningGetParams{Context: ctx})
	if err != nil {
		return nil, err
	}
	setNodeState(all, NodeStateJoining, joining.Payload)

	// Get leaving nodes
	leaving, err := c.scyllaOps.StorageServiceNodesLeavingGet(&scyllaOperations.StorageServiceNodesLeavingGetParams{Context: ctx})
	if err != nil {
		return nil, err
	}
	setNodeState(all, NodeStateLeaving, leaving.Payload)

	// Get moving nodes
	moving, err := c.scyllaOps.StorageServiceNodesMovingGet(&scyllaOperations.StorageServiceNodesMovingGetParams{Context: ctx})
	if err != nil {
		return nil, err
	}
	setNodeState(all, NodeStateMoving, moving.Payload)

	// Sort by Datacenter and Address
	sort.Slice(all, func(i, j int) bool {
		if all[i].Datacenter != all[j].Datacenter {
			return all[i].Datacenter < all[j].Datacenter
		}
		return all[i].Addr < all[j].Addr
	})

	return all, nil
}

func (c *Client) Decommission(ctx context.Context, host string) error {
	queryCtx := forceHost(ctx, host)
	// On decommission request api server waits till decommission is completed
	// Usually decommission takes significant amount of time therefore request is failing by timeout
	// As result of the scylla client will retry it and get 500 response that is saying that
	//   decommission is already in progress.
	// To avoid that we pass noRetry to the context
	queryCtx = noRetry(queryCtx)
	_, err := c.scyllaOps.StorageServiceDecommissionPost(&scyllaOperations.StorageServiceDecommissionPostParams{Context: queryCtx})
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) OperationMode(ctx context.Context, host string) (OperationalMode, error) {
	resp, err := c.scyllaOps.StorageServiceOperationModeGet(&scyllaOperations.StorageServiceOperationModeGetParams{Context: forceHost(ctx, host)})
	if err != nil {
		return "", err
	}
	return operationalModeFromString(resp.Payload), nil
}

func (c *Client) IsNativeTransportEnabled(ctx context.Context, host string) (bool, error) {
	resp, err := c.scyllaOps.StorageServiceNativeTransportGet(&scyllaOperations.StorageServiceNativeTransportGetParams{Context: forceHost(ctx, host)})
	if err != nil {
		return false, err
	}
	return resp.Payload, nil
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

var setOpenAPIGlobalsOnce sync.Once

func setOpenAPIGlobals() {
	setOpenAPIGlobalsOnce.Do(func() {
		// Timeout is defined in http client that we provide in api.NewWithClient.
		// If Context is provided to operation, which is always the case here,
		// this value has no meaning since OpenAPI runtime ignores it.
		api.DefaultTimeout = 0
		// Disable debug output to stderr, it could have been enabled by setting
		// SWAGGER_DEBUG or DEBUG env variables.
		apiMiddleware.Debug = false
	})
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
