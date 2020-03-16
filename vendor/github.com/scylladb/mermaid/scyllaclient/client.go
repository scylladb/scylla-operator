// Copyright (C) 2017 ScyllaDB

package scyllaclient

import (
	"crypto/tls"
	"net"
	"net/http"
	"runtime"
	"sync"
	"time"

	api "github.com/go-openapi/runtime/client"
	apiMiddleware "github.com/go-openapi/runtime/middleware"
	"github.com/go-openapi/strfmt"
	"github.com/hailocab/go-hostpool"
	"github.com/pkg/errors"
	"github.com/scylladb/go-log"
	"github.com/scylladb/mermaid/internal/httputil/middleware"
	agentClient "github.com/scylladb/mermaid/scyllaclient/internal/agent/client"
	agentOperations "github.com/scylladb/mermaid/scyllaclient/internal/agent/client/operations"
	scyllaClient "github.com/scylladb/mermaid/scyllaclient/internal/scylla/client"
	scyllaOperations "github.com/scylladb/mermaid/scyllaclient/internal/scylla/client/operations"
)

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

//go:generate ./internalgen.sh

// DefaultTransport returns a new http.Transport with similar default values to
// http.DefaultTransport. Do not use this for transient transports as it can
// leak file descriptors over time. Only use this for transports that will be
// re-used for the same host(s).
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

// Client provides means to interact with Scylla nodes.
type Client struct {
	config Config
	logger log.Logger

	scyllaOps *scyllaOperations.Client
	agentOps  *agentOperations.Client

	transport http.RoundTripper
}

// NewClient creates new scylla HTTP client.
func NewClient(config Config, logger log.Logger) (*Client, error) {
	if err := config.Validate(); err != nil {
		return nil, errors.Wrap(err, "invalid config")
	}
	setOpenAPIGlobals()

	// Copy hosts
	hosts := make([]string, len(config.Hosts))
	copy(hosts, config.Hosts)

	pool := hostpool.NewEpsilonGreedy(hosts, config.PoolDecayDuration, &hostpool.LinearEpsilonValueCalculator{})

	if config.Transport == nil {
		config.Transport = DefaultTransport()
	}
	transport := config.Transport
	transport = middleware.Timeout(transport, config.RequestTimeout, logger)
	transport = middleware.Logger(transport, logger)
	transport = middleware.HostPool(transport, pool, config.AgentPort)
	transport = middleware.Retry(transport, len(config.Hosts), logger)
	transport = middleware.AuthToken(transport, config.AuthToken)
	transport = middleware.FixContentType(transport)

	c := &http.Client{
		Timeout:   config.Timeout,
		Transport: transport,
	}

	scyllaRuntime := api.NewWithClient(
		scyllaClient.DefaultHost, scyllaClient.DefaultBasePath, []string{config.Scheme}, c,
	)
	agentRuntime := api.NewWithClient(
		agentClient.DefaultHost, agentClient.DefaultBasePath, []string{config.Scheme}, c,
	)

	// Debug can be turned on by SWAGGER_DEBUG or DEBUG env variable
	scyllaRuntime.Debug = false
	agentRuntime.Debug = false

	return &Client{
		config:    config,
		logger:    logger,
		scyllaOps: scyllaOperations.New(scyllaRuntime, strfmt.Default),
		agentOps:  agentOperations.New(agentRuntime, strfmt.Default),
		transport: transport,
	}, nil
}

// Timeout returns a timeout for a request.
func (c *Client) Timeout() time.Duration {
	return c.config.Timeout
}

// Close closes all the idle connections.
func (c *Client) Close() error {
	if t, ok := c.config.Transport.(*http.Transport); ok {
		t.CloseIdleConnections()
	}
	return nil
}
