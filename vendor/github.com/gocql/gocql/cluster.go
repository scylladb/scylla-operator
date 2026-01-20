/*
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
/*
 * Content before git sha 34fdeebefcbf183ed7f916f931aa0586fdaa1b40
 * Copyright (c) 2012, The Gocql authors,
 * provided under the BSD-3-Clause License.
 * See the NOTICE file distributed with this work for additional information.
 */

package gocql

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"sync/atomic"
	"time"

	"github.com/gocql/gocql/internal/eventbus"
)

const defaultDriverName = "ScyllaDB GoCQL Driver"

// PoolConfig configures the connection pool used by the driver, it defaults to
// using a round-robin host selection policy and a round-robin connection selection
// policy for each host.
type PoolConfig struct {
	// HostSelectionPolicy sets the policy for selecting which host to use for a
	// given query (default: RoundRobinHostPolicy())
	// It is not supported to use a single HostSelectionPolicy in multiple sessions
	// (even if you close the old session before using in a new session).
	HostSelectionPolicy HostSelectionPolicy
}

func (p PoolConfig) buildPool(session *Session) *policyConnPool {
	return newPolicyConnPool(session)
}

// ClusterConfig is a struct to configure the default cluster implementation
// of gocql. It has a variety of attributes that can be used to modify the
// behavior to fit the most common use cases. Applications that require a
// different setup must implement their own cluster.
type ClusterConfig struct {
	// BatchObserver will set the provided batch observer on all queries created from this session.
	// Use it to collect metrics / stats from batch queries by providing an implementation of BatchObserver.
	BatchObserver BatchObserver
	// Dialer will be used to establish all connections created for this Cluster.
	// If not provided, a default dialer configured with ConnectTimeout will be used.
	// Dialer is ignored if HostDialer is provided.
	Dialer Dialer
	// ApplicationInfo reports application information to the server by inserting it into options of the STARTUP frame
	ApplicationInfo ApplicationInfo
	// DNSResolver Resolves DNS names to IP addresses
	DNSResolver DNSResolver
	// Logger for this ClusterConfig.
	// If not specified, defaults to the gocql.defaultLogger.
	Logger StdLogger
	// HostDialer will be used to establish all connections for this Cluster.
	// Unlike Dialer, HostDialer is responsible for setting up the entire connection, including the TLS session.
	// To support shard-aware port, HostDialer should implement ShardDialer.
	// If not provided, Dialer will be used instead.
	HostDialer HostDialer
	// StreamObserver will be notified of stream state changes.
	// This can be used to track in-flight protocol requests and responses.
	StreamObserver StreamObserver
	// FrameHeaderObserver will set the provided frame header observer on all frames' headers created from this session.
	// Use it to collect metrics / stats from frames by providing an implementation of FrameHeaderObserver.
	FrameHeaderObserver FrameHeaderObserver
	// ConnectObserver will set the provided connect observer on all queries
	// created from this session.
	ConnectObserver ConnectObserver
	// QueryObserver will set the provided query observer on all queries created from this session.
	// Use it to collect metrics / stats from queries by providing an implementation of QueryObserver.
	QueryObserver QueryObserver
	// AddressTranslator will translate addresses found on peer discovery and/or
	// node change events.
	AddressTranslator AddressTranslator
	// HostFilter will filter all incoming events for host, any which don't pass
	// the filter will be ignored. If set will take precedence over any options set
	// via Discovery
	HostFilter HostFilter
	// Compression algorithm.
	// Default: nil
	Compressor Compressor
	// Default: nil
	Authenticator Authenticator
	actualSslOpts atomic.Value
	// PoolConfig configures the underlying connection pool, allowing the
	// configuration of host selection and connection selection policies.
	PoolConfig PoolConfig
	// Default retry policy to use for queries.
	// Default: SimpleRetryPolicy{NumRetries: 3}.
	RetryPolicy RetryPolicy
	// ConvictionPolicy decides whether to mark host as down based on the error and host info.
	// Default: SimpleConvictionPolicy
	ConvictionPolicy ConvictionPolicy
	// Default reconnection policy to use for reconnecting before trying to mark host as down.
	ReconnectionPolicy ReconnectionPolicy
	// A reconnection policy to use for reconnecting when connecting to the cluster first time.
	InitialReconnectionPolicy ReconnectionPolicy
	WarningsHandlerBuilder    WarningHandlerBuilder
	// SslOpts configures TLS use when HostDialer is not set.
	// SslOpts is ignored if HostDialer is set.
	SslOpts *SslOptions
	// An Authenticator factory. Can be used to create alternative authenticators.
	// Default: nil
	AuthProvider       func(h *HostInfo) (Authenticator, error)
	ClientRoutesConfig *ClientRoutesConfig
	// The version of the driver that is going to be reported to the server.
	// Defaulted to current library version
	DriverVersion string
	// The name of the driver that is going to be reported to the server.
	// Default: "ScyllaDB GoLang Driver"
	DriverName string
	// Initial keyspace. Optional.
	Keyspace string
	// CQL version (default: 3.0.0)
	CQLVersion string
	// addresses for the initial connections. It is recommended to use the value set in
	// the Cassandra config for broadcast_address or listen_address, an IP address not
	// a domain name. This is because events from Cassandra will use the configured IP
	// address, which is used to index connected hosts. If the domain name specified
	// resolves to more than 1 IP address then the driver may connect multiple times to
	// the same host, and will not mark the node being down or up from events.
	Hosts []string
	// The time to wait for frames before flushing the frames connection to Cassandra.
	// Can help reduce syscall overhead by making less calls to write. Set to 0 to
	// disable.
	//
	// (default: 200 microseconds)
	WriteCoalesceWaitTime time.Duration
	// WriteTimeout limits the time the driver waits to write a request to a network connection.
	// WriteTimeout should be lower than or equal to Timeout.
	// WriteTimeout defaults to the value of Timeout.
	WriteTimeout time.Duration
	// The keepalive period to use, enabled if > 0 (default: 15 seconds)
	// SocketKeepalive is used to set up the default dialer and is ignored if Dialer or HostDialer is provided.
	SocketKeepalive time.Duration
	// If not zero, gocql attempt to reconnect known DOWN nodes in every ReconnectInterval.
	ReconnectInterval time.Duration
	// The maximum amount of time to wait for schema agreement in a cluster after
	// receiving a schema change frame. (default: 60s)
	MaxWaitSchemaAgreement time.Duration
	// ProtoVersion sets the version of the native protocol to use, this will
	// enable features in the driver for specific protocol versions, generally this
	// should be set to a known version (2,3,4) for the cluster being connected to.
	//
	// If it is 0 or unset (the default) then the driver will attempt to discover the
	// highest supported protocol for the cluster. In clusters with nodes of different
	// versions the protocol selected is not defined (ie, it can be any of the supported in the cluster)
	ProtoVersion int
	// Maximum number of inflight requests allowed per connection.
	// Default: 32768 for CQL v3 and newer
	// Default: 128 for older CQL versions
	MaxRequestsPerConn int
	// Timeout defines the maximum time to wait for a single server response.
	// The default is 11 seconds, which is slightly higher than the default
	// server-side timeout for most query types.
	//
	// When a session creates a Query or Batch, it inherits this timeout as
	// the request timeout.
	//
	// Important notes:
	// 1. This value should be greater than the server timeout for all queries
	//    you execute. Otherwise, you risk creating retry storms: the server
	//    may still be processing the request while the client times out and retries.
	// 2. This timeout does not apply during initial connection setup.
	//    For that, see ConnectTimeout.
	Timeout time.Duration
	// The timeout for the requests to the schema tables. (default: 60s)
	MetadataSchemaRequestTimeout time.Duration
	// ConnectTimeout limits the time spent during connection setup.
	// During initial connection setup, internal queries, AUTH requests will return an error if the client
	// does not receive a response within the ConnectTimeout period.
	// ConnectTimeout is applied to the connection setup queries independently.
	// ConnectTimeout also limits the duration of dialing a new TCP connection
	// in case there is no Dialer nor HostDialer configured.
	// ConnectTimeout has a default value of 11 seconds.
	ConnectTimeout time.Duration
	// Port used when dialing.
	// Default: 9042
	Port int
	// The size of the connection pool for each host.
	// The pool filling runs in separate gourutine during the session initialization phase.
	// gocql will always try to get 1 connection on each host pool
	// during session initialization AND it will attempt
	// to fill each pool afterward asynchronously if NumConns > 1.
	// Notice: There is no guarantee that pool filling will be finished in the initialization phase.
	// Also, it describes a maximum number of connections at the same time.
	// Default: 2
	NumConns int
	// The gocql driver may hold excess shard connections to reuse them when existing connections are dropped.
	// This configuration variable defines the limit for such excess connections. Once the limit is reached,
	// gocql starts dropping any additional excess connections.
	// The limit is computed as `MaxExcessShardConnectionsRate` * <number_of_shards>.
	MaxExcessShardConnectionsRate float32
	// Maximum cache size for prepared statements globally for gocql.
	// Default: 1000
	MaxPreparedStmts int
	// Default page size to use for created sessions.
	// Default: 5000
	PageSize int
	// Maximum cache size for query info about statements for each session.
	// Default: 1000
	MaxRoutingKeyInfo int
	// ReadTimeout limits the time the driver waits for data from the connection.
	// It has only one purpose, identify faulty connection early and drop it.
	// Default: 11 Seconds
	ReadTimeout time.Duration
	// Consistency for the serial part of queries, values can be either SERIAL or LOCAL_SERIAL.
	// Default: unset
	SerialConsistency Consistency
	// Default consistency level.
	// Default: Quorum
	Consistency Consistency
	// Configure events the driver will register for
	Events struct {
		// disable registering for status events (node up/down)
		DisableNodeStatusEvents bool
		// disable registering for topology events (node added/removed/moved)
		DisableTopologyEvents bool
		// disable registering for schema events (keyspace/table/function removed/created/updated)
		DisableSchemaEvents bool
	}
	// Default idempotence for queries
	DefaultIdempotence bool
	// Sends a client side timestamp for all requests which overrides the timestamp at which it arrives at the server.
	// Default: true, only enabled for protocol 3 and above.
	DefaultTimestamp bool
	// DisableSkipMetadata will override the internal result metadata cache so that the driver does not
	// send skip_metadata for queries, this means that the result will always contain
	// the metadata to parse the rows and will not reuse the metadata from the prepared
	// statement.
	//
	// See https://issues.apache.org/jira/browse/CASSANDRA-10786
	// See https://github.com/scylladb/scylladb/issues/20860
	//
	// Default: true
	DisableSkipMetadata bool
	// DisableShardAwarePort will prevent the driver from connecting to Scylla's shard-aware port,
	// even if there are nodes in the cluster that support it.
	//
	// It is generally recommended to leave this option turned off because gocql can use
	// the shard-aware port to make the process of establishing more robust.
	// However, if you have a cluster with nodes which expose shard-aware port
	// but the port is unreachable due to network configuration issues, you can use
	// this option to work around the issue. Set it to true only if you neither can fix
	// your network nor disable shard-aware port on your nodes.
	DisableShardAwarePort bool
	// If DisableInitialHostLookup then the driver will not attempt to get host info
	// from the system.peers table, this will mean that the driver will connect to
	// hosts supplied and will not attempt to lookup the hosts information, this will
	// mean that data_center, rack and token information will not be available and as
	// such host filtering and token aware query routing will not be available.
	DisableInitialHostLookup bool
	// internal config for testing
	disableControlConn bool
	disableInit        bool
	// If IgnorePeerAddr is true and the address in system.peers does not match
	// the supplied host by either initial hosts or discovered via events then the
	// host will be replaced with the supplied address.
	//
	// For example if an event comes in with host=10.0.0.1 but when looking up that
	// address in system.local or system.peers returns 127.0.0.1, the peer will be
	// set to 10.0.0.1 which is what will be used to connect to.
	IgnorePeerAddr bool
	// An event bus configuration
	EventBusConfig eventbus.EventBusConfig
}

type DNSResolver interface {
	LookupIP(host string) ([]net.IP, error)
}

type ApplicationInfo interface {
	UpdateStartupOptions(map[string]string)
}

type StaticApplicationInfo struct {
	applicationName    string
	applicationVersion string
	clientID           string
}

func NewStaticApplicationInfo(name, version, clientID string) *StaticApplicationInfo {
	return &StaticApplicationInfo{
		applicationName:    name,
		applicationVersion: version,
		clientID:           clientID,
	}
}

func (i *StaticApplicationInfo) UpdateStartupOptions(opts map[string]string) {
	if i.applicationName != "" {
		opts["APPLICATION_NAME"] = i.applicationName
	}
	if i.applicationVersion != "" {
		opts["APPLICATION_VERSION"] = i.applicationVersion
	}
	if i.clientID != "" {
		opts["CLIENT_ID"] = i.clientID
	}
}

type SimpleDNSResolver struct {
	hostLookupPreferV4 bool
}

func NewSimpleDNSResolver(hostLookupPreferV4 bool) *SimpleDNSResolver {
	return &SimpleDNSResolver{
		hostLookupPreferV4,
	}
}

func (r SimpleDNSResolver) LookupIP(host string) ([]net.IP, error) {
	ips, err := net.LookupIP(host)
	if err != nil {
		return nil, err
	}
	// Filter to v4 addresses if any present
	if r.hostLookupPreferV4 {
		var preferredIPs []net.IP
		for _, v := range ips {
			if v4 := v.To4(); v4 != nil {
				preferredIPs = append(preferredIPs, v4)
			}
		}
		if len(preferredIPs) != 0 {
			ips = preferredIPs
		}
	}
	return ips, nil
}

var defaultDnsResolver = NewSimpleDNSResolver(os.Getenv("GOCQL_HOST_LOOKUP_PREFER_V4") == "true")

type Dialer interface {
	DialContext(ctx context.Context, network, addr string) (net.Conn, error)
}

// NewCluster generates a new config for the default cluster implementation.
//
// The supplied hosts are used to initially connect to the cluster then the rest of
// the ring will be automatically discovered. It is recommended to use the value set in
// the Cassandra config for broadcast_address or listen_address, an IP address not
// a domain name. This is because events from Cassandra will use the configured IP
// address, which is used to index connected hosts. If the domain name specified
// resolves to more than 1 IP address then the driver may connect multiple times to
// the same host, and will not mark the node being down or up from events.
func NewCluster(hosts ...string) *ClusterConfig {
	logger := &defaultLogger{}
	cfg := &ClusterConfig{
		Hosts:                         hosts,
		CQLVersion:                    "3.0.0",
		Timeout:                       11 * time.Second,
		ConnectTimeout:                60 * time.Second,
		ReadTimeout:                   11 * time.Second,
		WriteTimeout:                  11 * time.Second,
		Port:                          9042,
		MaxExcessShardConnectionsRate: 2,
		NumConns:                      2,
		Consistency:                   Quorum,
		MaxPreparedStmts:              defaultMaxPreparedStmts,
		MaxRoutingKeyInfo:             1000,
		PageSize:                      5000,
		DefaultTimestamp:              true,
		DriverName:                    defaultDriverName,
		DriverVersion:                 defaultDriverVersion,
		MaxWaitSchemaAgreement:        60 * time.Second,
		ReconnectInterval:             60 * time.Second,
		ConvictionPolicy:              &SimpleConvictionPolicy{},
		ReconnectionPolicy:            &ConstantReconnectionPolicy{MaxRetries: 3, Interval: 1 * time.Second},
		InitialReconnectionPolicy:     &NoReconnectionPolicy{},
		SocketKeepalive:               15 * time.Second,
		WriteCoalesceWaitTime:         200 * time.Microsecond,
		MetadataSchemaRequestTimeout:  60 * time.Second,
		DisableSkipMetadata:           true,
		WarningsHandlerBuilder:        DefaultWarningHandlerBuilder,
		Logger:                        logger,
		DNSResolver:                   defaultDnsResolver,
		EventBusConfig: eventbus.EventBusConfig{
			InputEventsQueueSize: 10240,
		},
	}

	return cfg
}

func (cfg *ClusterConfig) logger() StdLogger {
	if cfg.Logger == nil {
		return &defaultLogger{}
	}
	return cfg.Logger
}

// CreateSession initializes the cluster based on this config and returns a
// session object that can be used to interact with the database.
func (cfg *ClusterConfig) CreateSession() (*Session, error) {
	return NewSession(*cfg)
}

func (cfg *ClusterConfig) CreateSessionNonBlocking() (*Session, error) {
	return NewSessionNonBlocking(*cfg)
}

func (cfg *ClusterConfig) filterHost(host *HostInfo) bool {
	return !(cfg.HostFilter == nil || cfg.HostFilter.Accept(host))
}

func (cfg *ClusterConfig) ValidateAndInitSSL() error {
	if cfg.SslOpts == nil {
		return nil
	}
	actualTLSConfig, err := setupTLSConfig(cfg.SslOpts)
	if err != nil {
		return fmt.Errorf("failed to initialize ssl configuration: %s", err.Error())
	}

	cfg.actualSslOpts.Store(actualTLSConfig)
	return nil
}

func (cfg *ClusterConfig) getActualTLSConfig() *tls.Config {
	val, ok := cfg.actualSslOpts.Load().(*tls.Config)
	if !ok {
		return nil
	}
	return val.Clone()
}

type ClusterOption func(*ClusterConfig)

func (cfg *ClusterConfig) WithOptions(opts ...ClusterOption) *ClusterConfig {
	for _, opt := range opts {
		opt(cfg)
	}
	return cfg
}

type ClientRoutesOption func(*ClientRoutesConfig)

func WithMaxResolverConcurrency(val int) func(*ClientRoutesConfig) {
	return func(cfg *ClientRoutesConfig) {
		cfg.MaxResolverConcurrency = val
	}
}

func WithResolveHealthyEndpointPeriod(val time.Duration) func(*ClientRoutesConfig) {
	return func(cfg *ClientRoutesConfig) {
		cfg.ResolveHealthyEndpointPeriod = val
	}
}

func WithEndpoints(endpoints ...ClientRoutesEndpoint) func(*ClientRoutesConfig) {
	return func(cfg *ClientRoutesConfig) {
		cfg.Endpoints = endpoints
	}
}

func WithTable(tableName string) func(*ClientRoutesConfig) {
	return func(cfg *ClientRoutesConfig) {
		cfg.TableName = tableName
	}
}

func WithClientRoutes(opts ...ClientRoutesOption) func(*ClusterConfig) {
	pmCfg := ClientRoutesConfig{
		// Don't resolve healthy nodes by default
		ResolveHealthyEndpointPeriod: 0,
		MaxResolverConcurrency:       1,
		TableName:                    "system.client_routes",
		ResolverCacheDuration:        time.Millisecond * 500,
	}
	for _, opt := range opts {
		opt(&pmCfg)
	}
	return func(cfg *ClusterConfig) {
		cfg.ClientRoutesConfig = &pmCfg
		if len(cfg.Hosts) == 0 {
			for _, ep := range pmCfg.Endpoints {
				if ep.ConnectionAddr != "" {
					cfg.Hosts = append(cfg.Hosts, ep.ConnectionAddr)
				}
			}
		}
		// TODO: cfg.ControlConnectionOnlyToInitialNodes
	}
}

func (cfg *ClusterConfig) Validate() error {
	if len(cfg.Hosts) == 0 {
		return ErrNoHosts
	}

	if cfg.Authenticator != nil && cfg.AuthProvider != nil {
		return errors.New("Can't use both Authenticator and AuthProvider in cluster config.")
	}

	if cfg.InitialReconnectionPolicy == nil {
		return errors.New("InitialReconnectionPolicy is nil")
	}

	if cfg.InitialReconnectionPolicy.GetMaxRetries() <= 0 {
		return errors.New("InitialReconnectionPolicy.GetMaxRetries returns negative number")
	}

	if cfg.ReconnectionPolicy == nil {
		return errors.New("ReconnectionPolicy is nil")
	}

	if cfg.InitialReconnectionPolicy.GetMaxRetries() <= 0 {
		return errors.New("ReconnectionPolicy.GetMaxRetries returns negative number")
	}

	if cfg.PageSize < 0 {
		return errors.New("PageSize should be positive number or zero")
	}

	if cfg.MaxRoutingKeyInfo < 0 {
		return errors.New("MaxRoutingKeyInfo should be positive number or zero")
	}

	if cfg.MaxPreparedStmts < 0 {
		return errors.New("MaxPreparedStmts should be positive number or zero")
	}

	if cfg.SocketKeepalive < 0 {
		return errors.New("SocketKeepalive should be positive time.Duration or zero")
	}

	if cfg.MaxRequestsPerConn < 0 {
		return errors.New("MaxRequestsPerConn should be positive number or zero")
	}

	if cfg.NumConns < 0 {
		return errors.New("NumConns should be positive non-zero number or zero")
	}

	if cfg.Port <= 0 || cfg.Port > 65535 {
		return errors.New("Port should be a valid port number: a number between 1 and 65535")
	}

	if cfg.WriteTimeout < 0 {
		return errors.New("WriteTimeout should be positive time.Duration or zero")
	}

	if cfg.Timeout < 0 {
		return errors.New("Timeout should be positive time.Duration or zero")
	}

	if cfg.ConnectTimeout < 0 {
		return errors.New("ConnectTimeout should be positive time.Duration or zero")
	}

	if cfg.MetadataSchemaRequestTimeout < 0 {
		return errors.New("MetadataSchemaRequestTimeout should be positive time.Duration or zero")
	}

	if cfg.WriteCoalesceWaitTime < 0 {
		return errors.New("WriteCoalesceWaitTime should be positive time.Duration or zero")
	}

	if cfg.ReconnectInterval < 0 {
		return errors.New("ReconnectInterval should be positive time.Duration or zero")
	}

	if cfg.MaxWaitSchemaAgreement < 0 {
		return errors.New("MaxWaitSchemaAgreement should be positive time.Duration or zero")
	}

	if cfg.ProtoVersion < 0 {
		return errors.New("ProtoVersion should be positive number or zero")
	}

	if !cfg.DisableSkipMetadata {
		cfg.Logger.Println("warning: enabling skipping metadata can lead to unpredictable results when executing query and altering columns involved in the query.")
	}

	if cfg.SerialConsistency > 0 && !cfg.SerialConsistency.IsSerial() {
		return fmt.Errorf("the default SerialConsistency level is not allowed to be anything else but SERIAL or LOCAL_SERIAL. Recived value: %v", cfg.SerialConsistency)
	}

	if cfg.DNSResolver == nil {
		return fmt.Errorf("DNSResolver is empty")
	}

	if cfg.MaxExcessShardConnectionsRate < 0 {
		return fmt.Errorf("MaxExcessShardConnectionsRate should be positive number or zero")
	}

	if cfg.ClientRoutesConfig != nil {
		if cfg.AddressTranslator != nil {
			return fmt.Errorf("AddressTranslator and ClientRoutesConfig should not be set at the same time")
		}
		if err := cfg.ClientRoutesConfig.Validate(); err != nil {
			return fmt.Errorf("ClientRoutesConfig is invalid: %v", err)
		}
	}

	return cfg.ValidateAndInitSSL()
}

var (
	ErrNoHosts              = errors.New("no hosts provided")
	ErrNoConnectionsStarted = errors.New("no connections were made when creating the session")
	ErrHostQueryFailed      = errors.New("unable to populate Hosts")
)

func setupTLSConfig(sslOpts *SslOptions) (*tls.Config, error) {
	//  Config.InsecureSkipVerify | EnableHostVerification | Result
	//  Config is nil             | true                   | verify host
	//  Config is nil             | false                  | do not verify host
	//  false                     | false                  | verify host
	//  true                      | false                  | do not verify host
	//  false                     | true                   | verify host
	//  true                      | true                   | verify host
	var tlsConfig *tls.Config
	if sslOpts.Config == nil {
		tlsConfig = &tls.Config{
			InsecureSkipVerify: !sslOpts.EnableHostVerification,
			// Ticket max size is 16371 bytes, so it can grow up to 16mb max.
			ClientSessionCache: tls.NewLRUClientSessionCache(1024),
		}
	} else {
		// use clone to avoid race.
		tlsConfig = sslOpts.Config.Clone()
	}

	if tlsConfig.InsecureSkipVerify && sslOpts.EnableHostVerification {
		tlsConfig.InsecureSkipVerify = false
	}

	// ca cert is optional
	if sslOpts.CaPath != "" {
		if tlsConfig.RootCAs == nil {
			tlsConfig.RootCAs = x509.NewCertPool()
		}

		pem, err := ioutil.ReadFile(sslOpts.CaPath)
		if err != nil {
			return nil, fmt.Errorf("unable to open CA certs: %v", err)
		}

		if !tlsConfig.RootCAs.AppendCertsFromPEM(pem) {
			return nil, errors.New("failed parsing or CA certs")
		}
	}

	if sslOpts.CertPath != "" || sslOpts.KeyPath != "" {
		mycert, err := tls.LoadX509KeyPair(sslOpts.CertPath, sslOpts.KeyPath)
		if err != nil {
			return nil, fmt.Errorf("unable to load X509 key pair: %v", err)
		}
		tlsConfig.Certificates = append(tlsConfig.Certificates, mycert)
	}

	return tlsConfig, nil
}
