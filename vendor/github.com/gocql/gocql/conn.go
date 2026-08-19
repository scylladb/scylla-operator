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
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	frm "github.com/gocql/gocql/internal/frame"
	"github.com/gocql/gocql/tablets"

	"github.com/gocql/gocql/internal/lru"
	"github.com/gocql/gocql/internal/streams"
)

// approve the authenticator with the list of allowed authenticators. If the provided list is empty,
// the given authenticator is allowed.
func approve(authenticator string, approvedAuthenticators []string) bool {
	if len(approvedAuthenticators) == 0 {
		return true
	}
	for _, s := range approvedAuthenticators {
		if authenticator == s {
			return true
		}
	}
	return false
}

type Authenticator interface {
	Challenge(req []byte) (resp []byte, auth Authenticator, err error)
	Success(data []byte) error
}

type WarningHandlerBuilder func(session *Session) WarningHandler

type WarningHandler interface {
	HandleWarnings(qry ExecutableQuery, host *HostInfo, warnings []string)
}

// PasswordAuthenticator specifies credentials to be used when authenticating.
// It can be configured with an "allow list" of authenticator class names to avoid
// attempting to authenticate with Cassandra if it doesn't provide an expected authenticator.
type PasswordAuthenticator struct {
	Username string
	Password string
	// Setting this to nil or empty will allow authenticating with any authenticator
	// provided by the server.  This is the default behavior of most other driver
	// implementations.
	AllowedAuthenticators []string
}

func (p PasswordAuthenticator) Challenge(req []byte) ([]byte, Authenticator, error) {
	if !approve(string(req), p.AllowedAuthenticators) {
		return nil, nil, fmt.Errorf("unexpected authenticator %q", req)
	}
	resp := make([]byte, 2+len(p.Username)+len(p.Password))
	resp[0] = 0
	copy(resp[1:], p.Username)
	resp[len(p.Username)+1] = 0
	copy(resp[2+len(p.Username):], p.Password)
	return resp, nil, nil
}

func (p PasswordAuthenticator) Success(data []byte) error {
	return nil
}

// SslOptions configures TLS use.
//
// Warning: Due to historical reasons, the SslOptions is insecure by default, so you need to set EnableHostVerification
// to true if no Config is set. Most users should set SslOptions.Config to a *tls.Config.
// SslOptions and Config.InsecureSkipVerify interact as follows:
//
//	Config.InsecureSkipVerify | EnableHostVerification | Result
//	Config is nil             | false                  | do not verify host
//	Config is nil             | true                   | verify host
//	false                     | false                  | verify host
//	true                      | false                  | do not verify host
//	false                     | true                   | verify host
//	true                      | true                   | verify host
type SslOptions struct {
	*tls.Config

	// CertPath and KeyPath are optional depending on server
	// config, but both fields must be omitted to avoid using a
	// client certificate
	CertPath string
	KeyPath  string
	CaPath   string //optional depending on server config
	// If you want to verify the hostname and server cert (like a wildcard for cass cluster) then you should turn this
	// on.
	// This option is basically the inverse of tls.Config.InsecureSkipVerify.
	// See InsecureSkipVerify in http://golang.org/pkg/crypto/tls/ for more info.
	//
	// See SslOptions documentation to see how EnableHostVerification interacts with the provided tls.Config.
	EnableHostVerification bool
	// DisableStrictCertificateValidation disables strict chain validation.
	// Strict validation requires TLS verification (InsecureSkipVerify=false).
	// When false (default) with verification enabled, the driver validates the
	// entire chain up to a self-signed root. When true, Go's default applies.
	//
	// Deprecated: This option is provided for backward compatibility and will be removed
	// in a future version. You should ensure your certificate chains are properly configured
	// and avoid using this option.
	DisableStrictCertificateValidation bool
}

type ConnConfig struct {
	Dialer          Dialer
	Logger          StdLogger
	Authenticator   Authenticator
	Compressor      Compressor
	HostDialer      HostDialer
	AuthProvider    func(h *HostInfo) (Authenticator, error)
	tlsConfig       *tls.Config
	CQLVersion      string
	ConnectTimeout  time.Duration
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ProtoVersion    int
	Keepalive       time.Duration
	disableCoalesce bool
	// isControlConn marks the connection used by the control connection, which is
	// the only one reporting the driver configuration on startup.
	isControlConn bool
}

func (c *ConnConfig) logger() StdLogger {
	if c.Logger == nil {
		return &defaultLogger{}
	}
	return c.Logger
}

type ConnErrorHandler interface {
	HandleError(conn *Conn, err error, closed bool)
}

type connErrorHandlerFn func(conn *Conn, err error, closed bool)

func (fn connErrorHandlerFn) HandleError(conn *Conn, err error, closed bool) {
	fn(conn, err, closed)
}

type ConnInterface interface {
	Close()
	exec(ctx context.Context, req frameBuilder, tracer Tracer, requestTimeout time.Duration) (*framer, error)
	awaitSchemaAgreement(ctx context.Context) error
	executeQuery(ctx context.Context, qry *Query) *Iter
	executeQueryWithMetrics(ctx context.Context, qry *Query, metrics *queryMetrics) *Iter
	querySystem(ctx context.Context, query string, values ...any) *Iter
	getIsSchemaV2() bool
	setSchemaV2(s bool)
	isScyllaConn() bool
	getScyllaSupported() ScyllaConnectionFeatures
}

// Conn is a single connection to a Cassandra node. It can be used to execute
// queries, but users are usually advised to use a more reliable, higher
// level API.
type Conn struct {
	auth           Authenticator
	cfg            *ConnConfig
	frameObserver  FrameHeaderObserver
	streamObserver StreamObserver
	w              contextWriter
	logger         StdLogger
	ctx            context.Context
	errorHandler   ConnErrorHandler
	compressor     Compressor
	supported      map[string][]string
	streams        *streams.IDGenerator
	host           *HostInfo
	// calls stores a map from stream ID to callReq.
	// This map is protected by mu.
	// calls should not be used when closed is true, calls is set to nil when closed=true.
	calls map[int]*callReq
	// segScratch holds the reusable buffers inbound v5 segments are read into.
	// Only touched by the receive path, which runs on the serve() goroutine.
	segScratch segmentScratch
	// headerReader is the reader the current frame or segment header is read
	// through (see readFrameHeader, readFirstSegmentHeader). Reused rather than
	// allocated per header, and like segScratch only touched by whichever
	// goroutine is currently receiving. The two header reads never nest, and the
	// startup coordinator's reader never overlaps serve(): a frameTicker tick is
	// only sent while the previous response is still outstanding, and
	// processFrameSource touches neither field after handing that response to its
	// caller.
	//
	// Note that last clause is what makes it safe, not an ordering — setupConn
	// returns on the startupErr send, which the options goroutine performs before
	// close(frameTicker), so the startup reader can still be unwinding when
	// serve() starts. Work added to processFrameSource after the response is
	// delivered would need a real barrier here.
	headerReader headerReader
	r            connReadSource
	session      *Session
	framers      connFramers
	cancel       context.CancelFunc
	// currentKeyspace is the keyspace this connection was switched to by
	// Conn.UseKeyspace, and the default the prepared-statement cache is keyed by
	// (see executeQuery/executeBatch). It deliberately tracks only driver-issued
	// USE: a `USE` statement a caller executes as an ordinary query switches the
	// server side of whichever single pooled connection it landed on, and the
	// driver does not follow it — so cache keys keep using the configured keyspace.
	//
	// Atomic because UseKeyspace is exported: a caller invoking it on a live
	// connection would otherwise race the request goroutines reading it.
	currentKeyspace atomic.Pointer[string]
	addr            string
	// systemRequest carries the timeouts driver-issued system queries are sent
	// with. See systemRequestState for why the two travel together.
	//
	// Atomic because finalizeConnection switches the timeout from
	// cfg.ConnectTimeout to cfg.MetadataSchemaRequestTimeout at the very end of
	// Session.init, by which point the control connection is already registered
	// for server push events: a SCHEMA_CHANGE arriving inside that window makes
	// the event-debouncer goroutine run querySystem on this connection
	// concurrently with the write.
	systemRequest    atomic.Pointer[systemRequestState]
	cqlProtoExts     []cqlProtocolExtension
	scyllaSupported  ScyllaConnectionFeatures
	writeTimeout     atomic.Int64
	mu               sync.Mutex
	tabletsRoutingV1 int32
	headerBuf        [headSize]byte
	isShardAware     bool
	// true if connection close process for the connection started.
	// closed is protected by mu.
	closed     bool
	isSchemaV2 bool
	version    uint8
}

func (c *Conn) getIsSchemaV2() bool {
	return c.isSchemaV2
}

func (c *Conn) setSchemaV2(s bool) {
	c.isSchemaV2 = s
}

// systemRequestState is the immutable pair of timeouts a driver-issued system
// query is sent with: the client-side deadline, and the ScyllaDB-only
// " USING TIMEOUT ...ms" clause pre-rendered from it (empty when the clause does
// not apply). The two are published as one snapshot so a reader can never pair
// one with a stale version of the other - which would ask the server to abort a
// system query on a deadline the client is not waiting on, or vice versa.
//
// Field order keeps the string first so the GC scans 8 pointer bytes, not 16
// (govet's fieldalignment).
type systemRequestState struct {
	usingClause string
	timeout     time.Duration
}

// systemRequestStatement returns stmt with the USING TIMEOUT clause appended and
// the client-side timeout to send it with, both taken from one snapshot so the
// two can never disagree. It is the only way the query paths should reach them.
func (c *Conn) systemRequestStatement(stmt string) (string, time.Duration) {
	state := c.getSystemRequestState()
	return stmt + state.usingClause, state.timeout
}

// getSystemRequestState returns the current snapshot.
func (c *Conn) getSystemRequestState() systemRequestState {
	if state := c.systemRequest.Load(); state != nil {
		return *state
	}
	return systemRequestState{}
}

// setSystemRequestTimeout publishes t together with the clause derived from it.
// The clause is ScyllaDB-only and needs a positive timeout, so it is empty
// otherwise - a timeout the caller disabled must not leave an older clause in
// force.
func (c *Conn) setSystemRequestTimeout(t time.Duration) {
	next := systemRequestState{timeout: t}
	if t > time.Duration(0) && c.isScyllaConn() {
		next.usingClause = " USING TIMEOUT " + strconv.FormatInt(t.Milliseconds(), 10) + "ms"
	}
	c.systemRequest.Store(&next)
}

// recalculateSystemRequestTimeout re-renders the clause for the timeout already
// in effect. It is called once the connection knows whether it talks to
// ScyllaDB, which the clause depends on.
//
// It re-reads the timeout rather than naming the value its only caller knows is
// in force (cfg.ConnectTimeout, from dialWithoutObserver): republishing whatever
// is current cannot overwrite a timeout some later change publishes earlier in
// startup, which would put every system query back under an unrelated setting.
func (c *Conn) recalculateSystemRequestTimeout() {
	c.setSystemRequestTimeout(c.getSystemRequestState().timeout)
}

func (c *Conn) finalizeConnection() {
	// When connection just created all timeouts are set to `cfg.ConnectTimeout`
	// It is done to make sure that connection is easy to establish when users set very low `WriteTimeout` and/or `Timeout`
	// This method sets timeouts to `operational` values after connection successfully created
	c.writeTimeout.Store(int64(c.cfg.WriteTimeout))
	c.setSystemRequestTimeout(c.session.cfg.MetadataSchemaRequestTimeout)
	c.w.setWriteTimeout(c.cfg.WriteTimeout)
	c.r.SetTimeout(c.cfg.ReadTimeout)
}

func (c *Conn) getScyllaSupported() ScyllaConnectionFeatures {
	return c.scyllaSupported
}

// connectShard establishes a connection to a shard.
// If nrShards is zero, shard-aware dialing is disabled.
// note: every connection needs to get `conn.finalizeConnection` called ont it when initialization process is done
func (s *Session) connectShard(ctx context.Context, host *HostInfo, errorHandler ConnErrorHandler,
	shardID, nrShards int) (*Conn, error) {
	return s.dialShard(ctx, host, s.connCfg, errorHandler, shardID, nrShards)
}

// dial establishes a connection to a Cassandra node and notifies the session's connectObserver.
// note: every connection needs to get `conn.finalizeConnection` called on it when initialization process is done
func (s *Session) dial(ctx context.Context, host *HostInfo, connConfig *ConnConfig, errorHandler ConnErrorHandler) (*Conn, error) {
	return s.dialShard(ctx, host, connConfig, errorHandler, 0, 0)
}

func translateHostAddresses(addressTranslator AddressTranslator, host *HostInfo, logger StdLogger) (translatedAddresses, error) {
	addr, err := translateAddressPort(addressTranslator, host, AddressPort{
		Address: host.UntranslatedConnectAddress(),
		Port:    uint16(host.Port()),
	}, logger)
	if err != nil {
		return translatedAddresses{}, fmt.Errorf("unable to translate regular cql address: %w", err)
	}
	resultedInfo := translatedAddresses{
		CQL: addr,
	}

	scyllaFeatures := host.ScyllaFeatures()
	if port := scyllaFeatures.ShardAwarePort(); port != 0 {
		addr, err = translateAddressPort(addressTranslator, host,
			AddressPort{
				Address: host.UntranslatedConnectAddress(),
				Port:    port,
			}, logger)
		if err != nil {
			return translatedAddresses{}, fmt.Errorf("unable to translate shard aware address: %w", err)
		}
		resultedInfo.ShardAware = addr
	}
	if port := scyllaFeatures.ShardAwarePortTLS(); port != 0 {
		addr, err = translateAddressPort(addressTranslator, host,
			AddressPort{
				Address: host.UntranslatedConnectAddress(),
				Port:    port,
			}, logger)
		if err != nil {
			return translatedAddresses{}, fmt.Errorf("unable to translate shard aware tls address: %w", err)
		}
		resultedInfo.ShardAwareTLS = addr
	}
	return resultedInfo, nil
}

// dialShard establishes a connection to a host/shard and notifies the session's connectObserver.
// If nrShards is zero, shard-aware dialing is disabled.
// note: every connection needs to get `conn.finalizeConnection` called on it when initialization process is done
func (s *Session) dialShard(ctx context.Context, host *HostInfo, connConfig *ConnConfig, errorHandler ConnErrorHandler,
	shardID, nrShards int) (*Conn, error) {
	var obs ObservedConnect

	current := host.getTranslatedConnectionInfo()
	updated, err := translateHostAddresses(s.addressTranslator, host, s.logger)
	if err != nil {
		return nil, err
	}
	if current == nil || !updated.Equal(current) {
		host.setTranslatedConnectionInfo(updated)
	}

	if s.connectObserver != nil {
		obs.Host = host
		obs.Start = time.Now()
	}

	conn, err := s.dialWithoutObserver(ctx, host, connConfig, errorHandler, shardID, nrShards)

	if s.connectObserver != nil {
		obs.End = time.Now()
		obs.Err = err
		s.connectObserver.ObserveConnect(obs)
	}

	return conn, err
}

// dialWithoutObserver establishes connection to a Cassandra node.
//
// dialWithoutObserver does not notify the connection observer, so you most probably want to call dial() instead.
//
// If nrShards is zero, shard-aware dialing is disabled.
func (s *Session) dialWithoutObserver(ctx context.Context, host *HostInfo, cfg *ConnConfig, errorHandler ConnErrorHandler,
	shardID, nrShards int) (*Conn, error) {

	shardDialer, ok := cfg.HostDialer.(ShardDialer)
	var (
		dialedHost *DialedHost
		err        error
	)

	isShardAware := false
	if ok && nrShards > 0 {
		isShardAware = true
		dialedHost, err = shardDialer.DialShard(ctx, host, shardID, nrShards)
	} else {
		dialedHost, err = cfg.HostDialer.DialHost(ctx, host)
	}

	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(ctx)
	c := &Conn{
		r: &connReader{
			conn: dialedHost.Conn,
			r:    bufio.NewReader(dialedHost.Conn),
		},
		cfg:           cfg,
		calls:         make(map[int]*callReq),
		version:       uint8(cfg.ProtoVersion),
		isShardAware:  isShardAware,
		addr:          dialedHost.Conn.RemoteAddr().String(),
		errorHandler:  errorHandler,
		compressor:    cfg.Compressor,
		session:       s,
		streams:       s.streamIDGenerator(),
		host:          host,
		isSchemaV2:    true, // Try using "system.peers_v2" until proven otherwise
		frameObserver: s.frameObserver,
		w: &deadlineContextWriter{
			w:         dialedHost.Conn,
			semaphore: make(chan struct{}, 1),
			quit:      make(chan struct{}),
		},
		ctx:            ctx,
		cancel:         cancel,
		logger:         cfg.logger(),
		streamObserver: s.streamObserver,
	}
	c.setSystemRequestTimeout(cfg.ConnectTimeout)

	if err := c.init(ctx, dialedHost); err != nil {
		cancel()
		c.Close()
		return nil, err
	}

	return c, nil
}

func (s *Session) streamIDGenerator() *streams.IDGenerator {
	if s.cfg.MaxRequestsPerConn > 0 {
		return streams.NewLimited(s.cfg.MaxRequestsPerConn)
	}
	return streams.New()
}

func (c *Conn) init(ctx context.Context, dialedHost *DialedHost) error {
	c.r.SetTimeout(c.cfg.ConnectTimeout)
	c.writeTimeout.Store(int64(c.cfg.ConnectTimeout))
	c.w.setWriteTimeout(c.cfg.ConnectTimeout)

	if c.session.cfg.AuthProvider != nil {
		var err error
		c.auth, err = c.cfg.AuthProvider(c.host)
		if err != nil {
			return err
		}
	} else {
		c.auth = c.cfg.Authenticator
	}

	startup := &startupCoordinator{
		frameTicker: make(chan struct{}),
		conn:        c,
	}

	// The driver configuration is identical for every connection of a session,
	// so it is reported only on the control connection to keep the other STARTUP
	// frames small. Leaving the reporter nil elsewhere reuses the same path that
	// a session with reporting disabled takes.
	if c.cfg.isControlConn {
		startup.driverConfigReporter = c.session.driverConfigReporter
	}

	if err := startup.setupConn(ctx); err != nil {
		return err
	}

	// dont coalesce startup frames
	if c.session.cfg.WriteCoalesceWaitTime > 0 && !c.cfg.disableCoalesce && !dialedHost.DisableCoalesce {
		c.w = newWriteCoalescer(dialedHost.Conn, c.cfg.ConnectTimeout, c.session.cfg.WriteCoalesceWaitTime, ctx.Done())
	}

	if c.isScyllaConn() { // ScyllaDB does not support system.peers_v2
		c.setSchemaV2(false)
	}

	go c.serve(ctx)
	go c.heartBeat(ctx)

	return nil
}

func (c *Conn) Write(p []byte) (n int, err error) {
	return c.w.writeContext(context.Background(), p)
}

// Read reads data from the connection.
//
// The driver itself reads through the connection's connReader (which owns the
// read-deadline handling); Read is retained so that *Conn keeps satisfying
// io.Reader for external callers, as it does io.Writer via Write.
func (c *Conn) Read(p []byte) (n int, err error) {
	return c.r.Read(p)
}

type startupCoordinator struct {
	conn                 *Conn
	frameTicker          chan struct{}
	driverConfigReporter *driverConfigReporter
}

func (s *startupCoordinator) setupConn(ctx context.Context) error {
	var cancel context.CancelFunc
	if s.conn.cfg.ConnectTimeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, s.conn.cfg.ConnectTimeout)
	} else {
		ctx, cancel = context.WithCancel(ctx)
	}
	defer cancel()

	// Only for proto v5+.
	// Indicates if STARTUP has been completed.
	// github.com/apache/cassandra/blob/trunk/doc/native_protocol_v5.spec
	// 2.3.1 Initial Handshake
	// 	In order to support both v5 and earlier formats, the v5 framing format is not
	//  applied to message exchanges before an initial handshake is completed.
	startupCompleted := &atomic.Bool{}
	startupCompleted.Store(false)

	startupErr := make(chan error)
	go func() {
		for range s.frameTicker {
			err := s.conn.recv(ctx, startupCompleted.Load())
			if err != nil {
				select {
				case startupErr <- err:
				case <-ctx.Done():
				}

				return
			}
		}
	}()

	go func() {
		defer close(s.frameTicker)
		err := s.options(ctx, startupCompleted)
		select {
		case startupErr <- err:
		case <-ctx.Done():
		}
	}()

	select {
	case err := <-startupErr:
		if err != nil {
			return err
		}
	case <-ctx.Done():
		return errors.New("gocql: no response to connection startup within timeout")
	}

	return nil
}

// write sends the given frame on the connection during startup and returns
// the parsed response frame.
//
// NOTE: The returned frame must not retain any byte-slice references to the
// framer's read buffer, because the framer is released back to the pool
// immediately after parseFrame returns (via defer). Frame types that use
// readBytesCopy (e.g. SupportedFrame, AuthChallengeFrame, AuthSuccessFrame)
// are safe; frame types that use readBytes and expose []byte fields would not
// be safe and must not be returned from this function.
func (s *startupCoordinator) write(ctx context.Context, frame frameBuilder, startupCompleted *atomic.Bool) (frame, error) {
	select {
	case s.frameTicker <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	framer, err := s.conn.execInternal(ctx, frame, nil, s.conn.cfg.ConnectTimeout, startupCompleted.Load())
	if err != nil {
		return nil, err
	}
	defer framer.Release()

	return framer.parseFrame()
}

func (s *startupCoordinator) options(ctx context.Context, startupCompleted *atomic.Bool) error {
	frame, err := s.write(ctx, &writeOptionsFrame{}, startupCompleted)
	if err != nil {
		return err
	}

	v, ok := frame.(*frm.SupportedFrame)
	if !ok {
		return NewErrProtocol("Unknown type of response to startup frame: %T", frame)
	}
	// Keep raw supported multimap for debug purposes
	s.conn.supported = v.Supported
	s.conn.scyllaSupported = parseSupported(s.conn.supported, s.conn.logger)
	s.conn.recalculateSystemRequestTimeout()
	if current := s.conn.host.ScyllaFeatures(); current != s.conn.scyllaSupported.ScyllaHostFeatures {
		s.conn.host.setScyllaFeatures(s.conn.scyllaSupported.ScyllaHostFeatures)
	}
	s.conn.cqlProtoExts = parseCQLProtocolExtensions(s.conn.supported, s.conn.version, s.conn.logger)

	// initFramerCache must be called after startup(), because startup() may
	// nil out c.compressor if the server does not support the requested
	// compression algorithm. Calling it before would snapshot a stale
	// compressor and set FlagCompress, causing protocol errors.
	err = s.startup(ctx, startupCompleted)
	if err != nil {
		return err
	}
	s.conn.initFramerCache()
	return nil
}

// startupOptions builds the STARTUP options the driver always sends.
//
// The application's options go in first so the driver-owned keys below can
// overwrite them, never the other way round. They describe the driver, the
// protocol it is speaking and the session the connection belongs to, and are
// not the application's to change: a callback that set CQL_VERSION could make
// every connection in the cluster fail the handshake, one that set DRIVER_NAME
// or DRIVER_VERSION would misreport the driver to the server for the life of
// the connection, and one that set SESSION_ID would break correlating a
// session's connections in system.clients.
//
// The ordering is easy to lose, which is how it was lost: upstream has no
// ApplicationInfo hook and writes these three as a map literal, so merging the
// two put the callback last.
func startupOptions(cqlVersion, driverName, driverVersion string, info ApplicationInfo, driverConfig *driverConfigReporter, sessionID string, isScyllaConn bool) map[string]string {
	m := map[string]string{}

	if info != nil {
		info.UpdateStartupOptions(m)
	}

	if driverConfig != nil {
		driverConfig.updateStartupOptions(m, isScyllaConn)
	}

	m[sessionIDStartupKey] = sessionID
	m["CQL_VERSION"] = cqlVersion
	m["DRIVER_NAME"] = driverName
	m["DRIVER_VERSION"] = driverVersion

	return m
}

func (s *startupCoordinator) startup(ctx context.Context, startupCompleted *atomic.Bool) error {
	// COMPRESSION and the CQL protocol extensions below are driver-owned too, and
	// are already protected by being written after the callback has run.
	m := startupOptions(
		s.conn.cfg.CQLVersion,
		s.conn.session.cfg.DriverName,
		s.conn.session.cfg.DriverVersion,
		s.conn.session.cfg.ApplicationInfo,
		s.driverConfigReporter,
		s.conn.session.id,
		s.conn.isScyllaConn(),
	)

	if s.conn.compressor != nil {
		comp := s.conn.supported["COMPRESSION"]
		name := s.conn.compressor.Name()
		for _, compressor := range comp {
			if compressor == name {
				m["COMPRESSION"] = compressor
				break
			}
		}

		if _, ok := m["COMPRESSION"]; !ok {
			s.conn.compressor = nil
		}
	}

	for _, ext := range s.conn.cqlProtoExts {
		serialized := ext.serialize()
		for k, v := range serialized {
			m[k] = v
		}
	}

	frame, err := s.write(ctx, &writeStartupFrame{opts: m}, startupCompleted)
	if err != nil {
		return err
	}

	switch v := frame.(type) {
	case error:
		return v
	case *frm.ReadyFrame:
		// Startup is successfully completed, so we could use Native Protocol 5
		startupCompleted.Store(true)
		return nil
	case *frm.AuthenticateFrame:
		// Startup is successfully completed, so we could use Native Protocol 5
		startupCompleted.Store(true)
		return s.authenticateHandshake(ctx, v, startupCompleted)
	default:
		return NewErrProtocol("Unknown type of response to startup frame: %s", v)
	}
}

func (s *startupCoordinator) authenticateHandshake(ctx context.Context, authFrame *frm.AuthenticateFrame, startupCompleted *atomic.Bool) error {
	if s.conn.auth == nil {
		return fmt.Errorf("authentication required (using %q)", authFrame.Class)
	}

	resp, challenger, err := s.conn.auth.Challenge([]byte(authFrame.Class))
	if err != nil {
		return err
	}

	req := &writeAuthResponseFrame{data: resp}
	for {
		frame, err := s.write(ctx, req, startupCompleted)
		if err != nil {
			return err
		}

		switch v := frame.(type) {
		case error:
			return v
		case *frm.AuthSuccessFrame:
			if challenger != nil {
				return challenger.Success(v.Data)
			}
			return nil
		case *frm.AuthChallengeFrame:
			resp, challenger, err = challenger.Challenge(v.Data)
			if err != nil {
				return err
			}

			req = &writeAuthResponseFrame{
				data: resp,
			}
		default:
			return fmt.Errorf("unknown frame response during authentication: %v", v)
		}
	}
}

func (c *Conn) closeWithError(err error) {
	if c == nil {
		return
	}

	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	callsToClose := c.calls
	// It is safe to change c.calls to nil. Nobody should use it after c.closed is set to true.
	c.calls = nil
	c.mu.Unlock()

	var cerr error
	if err == nil {
		// Graceful closes do not inject an error into call.resp, so cancel the
		// connection first to unblock any exec() calls before waiting for them.
		c.cancel()
		cerr = c.close()
	}

	for _, req := range callsToClose {
		if err != nil {
			// We need to send the error to all waiting queries.
			select {
			case req.resp <- callResp{err: err}:
				// exec() received the error. Wait for it to finish touching the callReq
				// before recycling it.
			case <-req.timeout:
				// exec() already timed out and returned.
			}
		}
		req.waitExecDone("closeWithError")
		if req.streamObserverContext != nil {
			req.streamObserverEndOnce.Do(func() {
				req.streamObserverContext.StreamAbandoned(ObservedStream{
					Host: c.host,
				})
			})
		}
		putCallReq(req)
	}

	// Allow GC of pooled framers. Safe to do after the drain loop above has
	// resolved all in-flight exec() calls. Any event goroutines still running
	// will see pool==nil in releaseFramer and simply drop the framer.
	c.framers.close()

	if err != nil {
		c.cancel()
		cerr = c.close()
	}

	if err != nil {
		c.errorHandler.HandleError(c, err, true)
	} else if cerr != nil {
		// TODO(zariel): is it a good idea to do this?
		c.errorHandler.HandleError(c, cerr, true)
	}
}

func (c *Conn) isTabletSupported() bool {
	return atomic.LoadInt32(&c.tabletsRoutingV1) == 1
}

func (c *Conn) setTabletSupported(val bool) {
	intVal := int32(0)
	if val {
		intVal = 1
	}
	atomic.StoreInt32(&c.tabletsRoutingV1, intVal)
}

// usesMetadataID reports whether SCYLLA_USE_METADATA_ID was negotiated on this
// connection. This is the extension alone; see tracksResultMetadataID for the
// question the request path actually asks.
//
// It reads the framer config rather than a separate field on Conn, so the request
// path and the pooled framers that encode and decode the result metadata ID cannot
// disagree: a Conn that believed the extension was on while its framers did not
// would ask the server to skip metadata while writing no ID to compare against.
// initFramerCache populates the config during connection setup, before any query
// can run.
//
// One framer is not built from that config: framerPool.get falls back to newFramer
// when the pool is disabled, which yields scyllaUseMetadataID false regardless of
// what was negotiated. That is correct during the handshake, and unreachable from
// the request path afterwards — execInternal acquires its framer before addCall
// rejects a closed connection, so a framer taken after the pool closed belongs to a
// call that never writes a frame. It is a property of call ordering rather than of
// construction, so see #982, which also covers flagLWT and tabletsRoutingV1.
func (c *Conn) usesMetadataID() bool {
	return c.framers.defaults.scyllaUseMetadataID
}

// tracksResultMetadataID reports whether an EXECUTE on this connection carries a
// result metadata ID for the server to compare its own against — either because
// native protocol v5 makes the field mandatory, or because
// SCYLLA_USE_METADATA_ID backported it to v4. Either way the server answers a
// stale ID with METADATA_CHANGED, which is what makes skipping result metadata
// recoverable. See shouldSkipResultMetadata.
//
// Read from the same framer config as usesMetadataID, for the same reason, and
// because initCache has already masked the protocol version there.
func (c *Conn) tracksResultMetadataID() bool {
	return c.framers.defaults.proto > protoVersion4 || c.framers.defaults.scyllaUseMetadataID
}

func (c *Conn) close() error {
	return c.r.Close()
}

func (c *Conn) Close() {
	c.closeWithError(nil)
}

// Serve starts the stream multiplexer for this connection, which is required
// to execute any queries. This method runs as long as the connection is
// open and is therefore usually called in a separate goroutine.
func (c *Conn) serve(ctx context.Context) {
	var err error
	for {
		err = c.recv(ctx, true)
		if err == nil {
			continue
		}
		// A benign idle timeout: the peer simply had nothing to send. Log it and
		// read again rather than dropping a healthy connection.
		//
		// Expected to be unreachable in practice — the wait for a header's first byte
		// runs with the read deadline disarmed (readFrameHeader,
		// readFirstSegmentHeader), so an idle connection cannot time out at all. The
		// branch is kept as the safety net for a regression in that disarm: without
		// it, such a regression would close every idle connection once per ReadTimeout
		// instead of printing this line. It is deliberately narrow — the deadline is
		// re-armed once the peer starts sending, and a timeout that already consumed
		// part of a header is not normalised to ErrReadHeaderTimeout, because the
		// stream position would be unknown and continuing would mis-frame everything
		// after it.
		if errors.Is(err, ErrReadHeaderTimeout) {
			c.logger.Print("gocql: read header timeout") // TODO: Provide more details from wrapped error?
			err = nil
			continue
		}
		break
	}

	c.closeWithError(err)
}

func (c *Conn) discardFrame(r io.Reader, head frm.FrameHeader) error {
	// Read the body from the same reader that supplied the header: for proto v5
	// this may be a segment buffer rather than the socket (c.r).
	_, err := io.CopyN(io.Discard, r, int64(head.Length))
	if err != nil {
		return err
	}
	return nil
}

type protocolError struct {
	frame frame
}

func (p *protocolError) Error() string {
	if err, ok := p.frame.(error); ok {
		return err.Error()
	}
	return fmt.Sprintf("gocql: received unexpected frame on stream %d: %v", p.frame.Header().Stream, p.frame)
}

func (c *Conn) heartBeat(ctx context.Context) {
	sleepTime := 1 * time.Second
	timer := time.NewTimer(sleepTime)
	defer timer.Stop()

	var failures int

	for {
		if failures > 5 {
			c.closeWithError(fmt.Errorf("gocql: heartbeat failed"))
			return
		}

		timer.Reset(sleepTime)

		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}

		framer, err := c.exec(context.Background(), &writeOptionsFrame{}, nil, c.cfg.ConnectTimeout)
		if err != nil {
			failures++
			continue
		}

		resp, err := framer.parseFrame()
		framer.Release()
		if err != nil {
			// invalid frame
			failures++
			continue
		}

		switch resp.(type) {
		case *frm.SupportedFrame:
			// Everything ok
			sleepTime = 30 * time.Second
			failures = 0
		case error:
			// TODO: should we do something here?
		default:
			panic(fmt.Sprintf("gocql: unknown frame in response to options: %T", resp))
		}
	}
}

func (c *Conn) recv(ctx context.Context, startupCompleted bool) error {
	// If startup is completed and native proto 5+ is set up then we should
	// unwrap payload from compressed/uncompressed frame
	if startupCompleted && c.version > protoVersion4 {
		return c.recvSegment(ctx)
	}

	return c.processFrame(ctx, c.r)
}

// frameSource supplies one CQL frame to processFrame. The header always comes
// from r — the socket, or a buffer holding an already-received segment payload.
//
// Field order is dictated by govet's fieldalignment rather than by how the fields
// relate: body's length and capacity are the only trailing words here that hold no
// pointer, so it goes last to keep the pointer-scanned prefix short. It belongs
// with r.
type frameSource struct {
	r io.Reader
	// segment, when non-nil, is the self-contained v5 segment payload r reads
	// from. Such a segment carries only whole frames, so a header declaring a body
	// longer than what is left in the segment is a framing violation, and
	// processFrameSource rejects it before readFrame acts on the declared length.
	//
	// That check is what keeps the length honest here. Off the socket, a lying
	// header costs the peer the bytes it claimed or stalls into a net.Error that
	// takes the connection down; out of a segment the short read is immediate and
	// yields io.ErrUnexpectedEOF, which is not a net.Error, so processFrameSource
	// keeps it per-request and leaves the connection up. A ~20-byte segment could
	// otherwise buy a maxFrameSize allocation, repeatable for as long as the peer
	// cares to send them.
	//
	// Nil on the pre-v5 socket path, and nil for a reassembled frame, where
	// framer.adoptFrameBody already matches the declared length exactly.
	segment *bytes.Reader
	// netStart/netEnd are the window of the network read that delivered these
	// bytes, for FrameHeaderObserver. On v5 the CQL header is parsed out of a
	// segment that has already arrived, so processFrameSource cannot measure the
	// network wait itself — the reader that could is recvSegment, and it passes
	// the window down here.
	//
	// Zero means "not measured", which is the pre-v5 socket path: there
	// processFrameSource reads the header off the wire and times it directly.
	netStart, netEnd time.Time
	// body is set only when the whole frame body is already in memory and the
	// buffer holding it can be given away: the read framer then adopts it instead
	// of reading and copying the body a second time (see framer.adoptFrameBody).
	body []byte
}

// readBody fills f with the frame body described by head, either by reading it
// from s.r or by adopting the buffer s already holds.
func (s frameSource) readBody(f *framer, head *frm.FrameHeader) error {
	if s.body != nil {
		return f.adoptFrameBody(s.body, head)
	}
	return f.readFrame(s.r, head)
}

func (c *Conn) processFrame(ctx context.Context, r io.Reader) error {
	return c.processFrameSource(ctx, frameSource{r: r})
}

// readFrameHeader reads one CQL frame header from r with the read deadline
// disarmed for its first byte: the serve() loop waits indefinitely for the next
// inbound frame, so a short ReadTimeout must not fire on an idle connection. Once
// the peer has started sending, headerReader re-arms the deadline, so the rest of
// the header — and the body read that follows — are bounded by the operational
// timeout. The disarm is also cleared via defer, which covers the paths that
// deliver no byte at all, and means a panic cannot leave the connection
// deadline-free.
//
// Disarming through a dedicated flag (rather than zeroing and restoring the
// connReader timeout) keeps the operational timeout intact, so a concurrent
// finalizeConnection switching the reader from ConnectTimeout to ReadTimeout is
// never lost. The header read itself clears any stale deadline (connReader.Read).
//
// Unlike readFirstSegmentHeader, the disarm here is a dynamic check: on proto v5 r
// is a reader over an already-received segment payload, which has no deadline to
// disarm. Missing it in that case is correct, not a bug.
func (c *Conn) readFrameHeader(r io.Reader) (frm.FrameHeader, error) {
	d, _ := r.(deadlineDisarmer)
	if d != nil {
		d.setDisarm(true)
		defer d.setDisarm(false)
	}

	c.headerReader.reset(r, d)
	return readHeader(&c.headerReader, c.headerBuf[:])
}

func (c *Conn) processFrameSource(ctx context.Context, src frameSource) error {
	// not safe for concurrent reads
	r := src.r

	// The observer documents Start/End as when the header started and finished
	// coming off the network. When the caller already did that read and measured
	// it (v5: the header is parsed out of a segment that has arrived), its window
	// is the truthful one; timing the parse here would report memory speed.
	headStartTime, headEndTime := src.netStart, src.netEnd
	measureHere := headStartTime.IsZero() && c.frameObserver != nil

	if measureHere {
		headStartTime = time.Now()
	}
	// were just reading headers over and over and copy bodies
	head, err := c.readFrameHeader(r)

	if measureHere {
		headEndTime = time.Now()
	}
	if err != nil {
		return err
	}

	if c.frameObserver != nil {
		c.frameObserver.ObserveFrameHeader(context.Background(), ObservedFrameHeader{
			Version: head.Version,
			Flags:   head.Flags,
			Stream:  int16(head.Stream),
			Opcode:  head.Op,
			Length:  int32(head.Length),
			Start:   headStartTime,
			End:     headEndTime,
			Host:    c.host,
		})
	}

	// Fatal, like the stream bound below, and for the same reason: the segment
	// payload's CRC32 already verified, so this is not corruption in flight but a
	// peer whose framing does not describe the bytes it sent. Nothing later on this
	// connection can be trusted to start where we think it does. Making it
	// per-request instead is what would let the peer repeat it — see frameSource.
	if src.segment != nil && head.Length > src.segment.Len() {
		return fmt.Errorf("gocql: frame header declares a %d byte body but only %d bytes remain in the self-contained segment", head.Length, src.segment.Len())
	}

	if head.Stream > c.streams.NumStreams {
		return fmt.Errorf("gocql: frame header stream is beyond call expected bounds: %d", head.Stream)
	} else if head.Stream <= 0 {
		// reserved stream that we dont use, probably due to a protocol error
		// or a bug in Cassandra, this should be an error, parse it and return.
		framer, err := c.readFrameIntoFramer(src, head)
		if err != nil {
			return err
		}

		frame, err := framer.parseFrame()
		// NOTE: Safe to release the framer here because all error frame types
		// (from parseErrorFrame) contain only strings, scalars, and defensively-
		// copied []byte fields. None retain sub-slices of the framer's read buffer.
		c.releaseReadFramer(framer)
		if err != nil {
			if head.Stream == -1 {
				// Event frame parse errors should not close the connection.
				c.logger.Printf("gocql: unable to parse event frame: %v\n", err)
				return nil
			}
			return err
		}

		if head.Stream == -1 { // reserved stream for events
			if c.session != nil {
				go c.session.handleEvent(frame)
			}
			return nil
		}

		return &protocolError{
			frame: frame,
		}
	}

	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return ErrConnectionClosed
	}
	call, ok := c.calls[head.Stream]
	delete(c.calls, head.Stream)
	c.mu.Unlock()
	if call == nil || !ok {
		c.logger.Printf("gocql: received response for stream which has no handler: header=%v\n", head)
		return c.discardFrame(r, head)
	} else if head.Stream != call.streamID {
		panic(fmt.Sprintf("call has incorrect streamID: got %d expected %d", call.streamID, head.Stream))
	}

	framer := c.getReadFramer()

	err = src.readBody(framer, &head)

	desynced := bodyReadDesyncedConn(err)

	// Deliver the outcome before returning it, even when fatal. head.Stream was
	// already removed from c.calls above, so closeWithError's drain loop can no
	// longer find this call: returning early would leave the caller waiting out its
	// full request timeout for a response that can never arrive, and would leak the
	// callReq. Delivering first makes it fail immediately, and the framer/stream
	// accounting stays on the paths that already handle it.
	//
	// we either, return a response to the caller, the caller timedout, or the
	// connection has closed. Either way we should never block indefinatly here
	select {
	case call.resp <- callResp{framer: framer, err: err}:
		// Framer ownership transferred to caller
	case <-call.timeout:
		c.abandonRecvCall(call, framer)
	case <-ctx.Done():
		c.abandonRecvCall(call, framer)
	}

	if desynced {
		return err
	}

	return nil
}

// bodyReadDesyncedConn reports whether a frame-body read failure left the
// connection at an unknown stream offset, in which case it is fatal: the
// unconsumed remainder of the body is still on the wire and every subsequent read
// would be mis-framed. A decode failure, or an over-long frame whose body was
// successfully discarded, leaves the stream aligned and stays a per-request error.
//
// Two kinds of failure qualify.
//
// A network error means the body was read partially or not at all. errors.As
// rather than a type assertion: readFrame wraps the read error, so an assertion on
// the wrapper never matches (which is precisely how this used to leave desynced
// connections in the pool).
//
// A failed read deadline arm means connReader.Read never reached the socket, so
// the whole body is still queued on a connection that is otherwise healthy — the
// worst case, because nothing else will surface the problem and serve() reads the
// body as its next frame header. It needs naming rather than being left to the
// net.Error test: net.Conn.SetReadDeadline is only conventionally a *net.OpError,
// and a connection from a user-supplied Dialer or HostDialer may return a plain
// error, which would classify this as per-request.
func bodyReadDesyncedConn(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, errArmReadDeadline) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr)
}

func (c *Conn) readFrameIntoFramer(src frameSource, head frm.FrameHeader) (*framer, error) {
	framer := c.getReadFramer()
	// Take the body from the same source that supplied the header: for proto v5
	// that is a segment payload or a reassembled frame rather than the socket (c.r).
	if err := src.readBody(framer, &head); err != nil {
		c.releaseReadFramer(framer)
		return nil, err
	}
	return framer, nil
}

func (c *Conn) abandonRecvCall(call *callReq, framer *framer) {
	c.releaseReadFramer(framer)
	c.releaseStream(call)
	call.waitExecDone("abandonRecvCall")
	putCallReq(call)
}

func (c *Conn) releaseStream(call *callReq) {
	c.streams.Clear(call.streamID)

	if call.streamObserverContext != nil {
		call.streamObserverEndOnce.Do(func() {
			call.streamObserverContext.StreamFinished(ObservedStream{
				Host: c.host,
			})
		})
	}
}

func (c *Conn) recvSegment(ctx context.Context) error {
	// Read the first segment's header with the read deadline disarmed: on an
	// idle connection the serve() loop blocks here waiting for the peer to
	// start sending the next frame, and that wait must not be bounded by
	// ReadTimeout. Only the wait for the header's first byte is unbounded — the
	// rest of the header (headerReader), the payload of this segment and any
	// continuation segments are all read with the deadline re-armed, so a peer that
	// starts sending and then stalls mid-read is caught by the per-read ReadTimeout.
	//
	// Note the deadline is per-read (see connReader.Read), not per-frame: a
	// single logical CQL frame may span many segments (recvSplitFrame), so
	// ReadTimeout bounds how long any one read may stall, not the total time
	// to assemble a frame. And a read that stalls but keeps making progress is
	// resumed up to maxReadAttempts times, so one read can take that multiple of
	// ReadTimeout before failing — only a read that delivers nothing fails within a
	// single one. A peer that keeps trickling progress is therefore not bounded by
	// time at all; it is bounded by the frame length recvSplitFrame enforces against
	// the reassembled size.
	//
	// netStart/netEnd bracket this read for FrameHeaderObserver. The CQL headers
	// inside are parsed out of memory further down, so timing them there would
	// report the parse rather than the network wait the observer documents; the
	// window is measured here and carried to processFrameSource instead. Sampled
	// only when an observer is installed, so an unobserved connection pays nothing.
	netStart := c.observedNow()
	hdr, err := c.readFirstSegmentHeader()
	if err != nil {
		return err
	}

	payload, err := readSegmentPayload(c.r, hdr, c.compressor, &c.segScratch)
	if err != nil {
		return err
	}
	netEnd := c.observedNow()

	if hdr.isSelfContained {
		// The segment holds one or more complete CQL frames.
		return c.processAllFramesInSegment(ctx, bytes.NewReader(payload), netStart, netEnd)
	}

	// The segment is the first slice of a single large CQL frame split across
	// several non-self-contained segments; reassemble the whole frame before
	// processing it.
	return c.recvSplitFrame(ctx, payload, netStart, netEnd)
}

// observedNow returns the current time when a frame header observer is
// installed, and the zero time otherwise. Callers thread the result into
// frameSource, where a zero value means "not measured" and processFrameSource
// falls back to timing the header read itself.
func (c *Conn) observedNow() time.Time {
	if c.frameObserver == nil {
		return time.Time{}
	}
	return time.Now()
}

// headerReader reads one frame or segment header, bounding the read-deadline
// disarm to the wait for its first byte.
//
// The idle wait has to be unbounded: serve() blocks on the next header for as long
// as the peer has nothing to send, and a short ReadTimeout must not drop a healthy
// connection. But once the first byte has arrived the peer is mid-header, and the
// rest belongs under ReadTimeout like any other transfer — otherwise a peer that
// sends one byte and then stops holds the serve goroutine for as long as it keeps
// the socket open, and the connection is never dropped.
//
// The disarm is per-read (connReader.armDeadline runs once per read attempt), so
// bounding it means capping the first read to a single byte and clearing the disarm
// before the next one; the remainder is then read with the deadline armed. Capping
// costs one extra read of a bufio.Reader, which the first read has already filled
// from the socket.
//
// n counts the bytes delivered. That is the caller's benign-vs-fatal signal for a
// header-read timeout: nothing consumed is the idle peer, part of a header consumed
// leaves the stream at an unknown offset (readFirstSegmentHeader, readHeader).
type headerReader struct {
	r io.Reader
	// disarm is the reader whose deadline is disarmed for the first byte, and is
	// cleared once that byte arrives so the cap and the re-arm happen exactly once.
	// Nil when there is no deadline to bound: on proto v5 the CQL header is parsed
	// out of a segment payload that has already been received.
	disarm deadlineDisarmer
	n      int
}

func (h *headerReader) reset(r io.Reader, disarm deadlineDisarmer) {
	h.r = r
	h.disarm = disarm
	h.n = 0
}

func (h *headerReader) Read(p []byte) (int, error) {
	if h.disarm != nil && len(p) > 1 {
		// Only the wait for the first byte runs without a deadline.
		p = p[:1]
	}
	n, err := h.r.Read(p)
	h.n += n
	if n > 0 && h.disarm != nil {
		// The peer has started sending: bound everything from here on. The caller's
		// deferred re-arm still covers the paths that deliver no byte at all.
		h.disarm.setDisarm(false)
		h.disarm = nil
	}
	return n, err
}

// readFirstSegmentHeader reads the header of the next segment, with the read
// deadline disarmed for its first byte so an idle serve() loop can block
// indefinitely waiting for the next frame to begin. The deadline is disarmed via a
// dedicated flag (so connReader.Read does not re-arm it), which leaves the
// operational timeout value intact, so a concurrent finalizeConnection switching
// the reader from ConnectTimeout to ReadTimeout is never clobbered.
//
// Only the idle wait is unbounded. headerReader re-arms the deadline as soon as the
// peer delivers a byte, so the rest of the header is bounded by ReadTimeout and a
// peer that sends a header prefix and then stalls cannot hold the serve goroutine;
// the deferred clear covers the paths that deliver no byte at all, and a panic.
//
// A read timeout during the idle wait is normalised to ErrReadHeaderTimeout so
// serve() treats it as a benign idle timeout instead of closing the connection —
// but only if the read consumed nothing. A timeout partway through a header leaves
// the stream at an unknown offset, so it stays a plain error and takes the
// connection down rather than mis-framing everything that follows. That is the
// timeout the re-arm above makes reachable.
func (c *Conn) readFirstSegmentHeader() (segmentHeader, error) {
	// No type assertion: Conn.r is a connReadSource, so the disarm always applies.
	c.r.setDisarm(true)
	defer c.r.setDisarm(false)

	// Counted rather than plumbing a byte count out of the segment header readers:
	// the count only matters here, where the benign/fatal decision is made.
	c.headerReader.reset(c.r, c.r)

	hdr, err := readSegmentHeader(&c.headerReader, c.compressor)
	if err != nil {
		var netErr net.Error
		if c.headerReader.n == 0 && errors.As(err, &netErr) && netErr.Timeout() {
			return segmentHeader{}, fmt.Errorf("%w: %w", ErrReadHeaderTimeout, err)
		}
		return segmentHeader{}, err
	}
	return hdr, nil
}

// recvSplitFrame reassembles a single CQL frame that the peer split across
// multiple non-self-contained segments and processes it. first is the payload of
// the segment already consumed by recvSegment.
//
// Every read here runs with the deadline armed (the peer is mid-transfer), so a
// peer that stops sending fails within one ReadTimeout — see connReader.Read for
// how a read that stalls but keeps progressing is resumed instead. The
// reassembly buffer is allocated exactly once, sized to the frame length the peer
// declared in the CQL frame header, and appending the arriving payloads is bounded
// by that length. So neither a lying header nor incremental growth can inflate it:
// growing a buffer to a maxFrameSize frame would end up holding ~512 MiB for a
// valid 256 MiB response. Ownership of the buffer is then handed to the read
// framer rather than copied into it, so the frame is never resident twice.
//
// The declared length itself is the peer's to choose, up to maxFrameSize, so a
// small hostile prologue still buys this one allocation before any body byte has
// arrived. Accepted deliberately, because it is bounded: at most once per
// connection — the continuation reads run under ReadTimeout, so a peer that
// stalls after declaring takes the connection down with it. Growing the buffer
// as payloads arrive would blunt that, at the cost of roughly doubling peak
// memory for every valid large frame, which is the common case. Contrast the
// self-contained path, where the same lie is repeatable per-request and is
// therefore rejected before the allocation instead (see frameSource).
//
// netStart/netEnd are the network-read window of the first segment, for
// FrameHeaderObserver. netEnd is extended below if the CQL header itself needed
// more segments to arrive — but only that far: the observer's End is when the
// header finished arriving, not the rest of the frame.
func (c *Conn) recvSplitFrame(ctx context.Context, first []byte, netStart, netEnd time.Time) error {
	// The CQL frame header may itself be split across segments, in which case the
	// frame length cannot be learnt from the first segment alone. Accumulate into a
	// local buffer until the header is complete; this is bounded by one segment
	// payload plus headSize, because a continuation segment must make progress and
	// only headSize bytes are needed. Segment payloads alias c.segScratch, so each
	// has to be copied before the next segment is read.
	if len(first) < headSize {
		accumulated := append([]byte(nil), first...)
		for len(accumulated) < headSize {
			payload, err := c.readContinuationSegment()
			if err != nil {
				return err
			}
			accumulated = append(accumulated, payload...)
		}
		first = accumulated
		netEnd = c.observedNow()
	}

	// Peek the CQL frame header (without consuming it — processFrame re-reads it
	// from the reassembled frame) to learn the total frame length.
	head, err := readHeader(bytes.NewReader(first[:headSize]), c.headerBuf[:])
	if err != nil {
		return err
	}
	if head.Length < 0 || head.Length > maxFrameSize {
		return fmt.Errorf("gocql: invalid frame body length in segmented frame: %d", head.Length)
	}
	total := headSize + head.Length
	if len(first) > total {
		return fmt.Errorf("gocql: segmented frame exceeds its declared length %d", total)
	}

	frame := make([]byte, 0, total)
	frame = append(frame, first...)
	for len(frame) < total {
		payload, err := c.readContinuationSegment()
		if err != nil {
			return err
		}
		if len(frame)+len(payload) > total {
			return fmt.Errorf("gocql: segmented frame exceeds its declared length %d", total)
		}
		frame = append(frame, payload...)
	}

	// The body is handed over as an owned buffer: the read framer adopts it
	// instead of allocating and copying another frame-sized buffer. The header is
	// still read from the front of the same bytes, so processFrame observes and
	// validates it exactly as it does for an unsegmented frame.
	return c.processFrameSource(ctx, frameSource{
		r:        bytes.NewReader(frame),
		body:     frame[headSize:],
		netStart: netStart,
		netEnd:   netEnd,
	})
}

// readContinuationSegment reads the next segment of a frame split across several
// segments and returns its payload, which aliases c.segScratch and is therefore
// only valid until the next segment is read. The reads run with the deadline armed,
// so a peer that stops sending fails within one ReadTimeout (connReader.Read).
// A self-contained segment, or one that makes no forward progress (empty
// payload), is rejected so a hostile peer cannot drive an infinite reassembly
// loop.
func (c *Conn) readContinuationSegment() ([]byte, error) {
	hdr, err := readSegmentHeader(c.r, c.compressor)
	if err != nil {
		return nil, fmt.Errorf("gocql: failed to read continuation segment header: %w", err)
	}
	if hdr.isSelfContained {
		return nil, fmt.Errorf("gocql: received self-contained segment, but expected a continuation")
	}
	payload, err := readSegmentPayload(c.r, hdr, c.compressor, &c.segScratch)
	if err != nil {
		return nil, fmt.Errorf("gocql: failed to read continuation segment payload: %w", err)
	}
	if len(payload) == 0 {
		return nil, fmt.Errorf("gocql: continuation segment made no progress (empty payload)")
	}
	return payload, nil
}

func (c *Conn) processAllFramesInSegment(ctx context.Context, r *bytes.Reader, netStart, netEnd time.Time) error {
	// A self-contained segment carries one or more complete CQL frames, so we
	// drain them all. This is safe to iterate: the segment payload has already
	// been CRC32-verified, and processFrameSource consumes exactly one frame
	// (header + body) per call, keeping r aligned for the next iteration. It
	// returns a non-nil error only for connection-fatal conditions (which stops
	// the loop); a per-request decode error is delivered to that request's
	// waiting caller and it returns nil, so sibling frames in the same segment are
	// still processed.
	//
	// r is passed as the segment as well as the reader, which is what bounds each
	// frame's declared body length by the bytes actually left in the segment (see
	// frameSource).
	//
	// Every frame packed into this segment reports the same observer window: they
	// did all arrive in the one network read, and attributing a slice of it to
	// each would be an invention.
	var err error
	for r.Len() > 0 && err == nil {
		err = c.processFrameSource(ctx, frameSource{
			r:        r,
			segment:  r,
			netStart: netStart,
			netEnd:   netEnd,
		})
	}

	return err
}

// deadlineDisarmer is a reader whose read deadline can be disarmed for a read
// that is expected to wait indefinitely (the idle wait for the next frame or
// segment header). See connReader.armDeadline.
type deadlineDisarmer interface {
	setDisarm(bool)
}

// connReadSource is the reader Conn reads through: the part of net.Conn the
// receive path actually uses, plus read-timeout control and the deadline disarm.
//
// Unexported, deliberately. The disarm is not optional for the connection's own
// reader, so requiring it here turns "this reader silently loses the disarm"
// from a runtime surprise into a compile error, and an unexported method means
// only this package can implement it. That costs nothing: Conn.r is unexported
// and no exported API accepts or returns one.
type connReadSource interface {
	// Read reads data from the connection.
	Read(p []byte) (n int, err error)

	// Close closes the connection.
	Close() error

	// RemoteAddr returns the remote network address, if known.
	RemoteAddr() net.Addr

	// SetTimeout sets the timeout duration for reading data from the conn.
	SetTimeout(timeout time.Duration)

	// GetTimeout returns the timeout duration.
	GetTimeout() time.Duration

	deadlineDisarmer
}

// connReader implements connReadSource.
type connReader struct {
	conn    net.Conn
	r       *bufio.Reader
	timeout atomic.Int64
	disarm  atomic.Bool
}

var _ connReadSource = (*connReader)(nil)

// maxReadAttempts bounds how many times Read re-arms the deadline and resumes a
// read that timed out while still making progress. See Read.
const maxReadAttempts = 5

// errArmReadDeadline marks a read that failed before it reached the socket,
// because the read deadline could not be armed. Nothing was consumed, which makes
// it fatal to a frame body read: the body is still queued on a connection that is
// otherwise healthy (see bodyReadDesyncedConn). It is a named error rather than a
// bare wrap because that classification cannot be inferred from the underlying
// error, which need not be a net.Error.
var errArmReadDeadline = errors.New("gocql: unable to arm the read deadline")

// Read fills p, resuming across read-deadline expiries for as long as the peer
// keeps delivering bytes.
//
// The retry exists because ReadTimeout is a per-read budget, not a transfer budget:
// a large frame body arriving over a slow link can need more than one budget while
// being perfectly healthy. Each attempt arms a fresh deadline and resumes at p[n:],
// so no bytes are dropped.
//
// It is gated on forward progress, which is what keeps ReadTimeout meaningful as
// "identify faulty connections early and drop it" (see ClusterConfig.ReadTimeout):
// an attempt that delivered nothing means the peer has stopped, and that fails
// immediately in a single ReadTimeout rather than after maxReadAttempts of them.
// Only a timeout is resumable — for any other network error the stream's position
// is no longer trustworthy, so resuming could silently mis-frame.
//
// Note the caller can still observe a partial read: on failure n bytes of p were
// consumed off the connection and cannot be put back, so a frame reader that gets
// an error here has to treat the connection as desynced (see processFrameSource).
func (c *connReader) Read(p []byte) (n int, err error) {
	for attempt := 0; attempt < maxReadAttempts; attempt++ {
		if aerr := c.armDeadline(); aerr != nil {
			// Wrapped so a caller can tell this from a read that reached the socket:
			// nothing was consumed here, so a frame body read that hits it leaves the
			// whole body queued. Double %w keeps the underlying error inspectable.
			return n, fmt.Errorf("%w: %w", errArmReadDeadline, aerr)
		}

		nn, rerr := io.ReadFull(c.r, p[n:])
		n += nn
		if rerr == nil {
			return n, nil
		}
		err = rerr

		var netErr net.Error
		if !errors.As(err, &netErr) || !netErr.Timeout() {
			// Not resumable. Timeout() rather than the deprecated Temporary(), which
			// is also true for e.g. ECONNRESET — where the connection is gone and
			// retrying just burns attempts.
			break
		}
		if nn == 0 {
			// The peer stopped sending, not merely slowed down.
			break
		}
	}

	return n, err
}

// armDeadline arms (or clears) the underlying read deadline for the read attempt
// that is about to start — once per attempt, so a Read that resumes after a timeout
// gets a fresh deadline, and so a concurrent finalizeConnection switching the
// reader from ConnectTimeout to ReadTimeout is picked up by the next attempt.
//
// The setter error is returned rather than dropped: a net.Conn
// whose SetReadDeadline fails would otherwise be read with no deadline at all,
// or with a stale one from an earlier read. This mirrors the write path, where
// deadlineContextWriter.writeContext and writeCoalescer.flush both report
// SetWriteDeadline failures instead of writing anyway.
func (c *connReader) armDeadline() error {
	if c.conn == nil {
		return nil
	}
	if c.disarm.Load() {
		// The read deadline is disarmed around the frame/segment header read in
		// the serve() loop: on an idle connection that read blocks indefinitely
		// waiting for the next frame, so a short ReadTimeout must not fire here.
		// We disarm via this flag rather than by zeroing the operational timeout
		// so that a concurrent finalizeConnection (which switches the reader from
		// ConnectTimeout to ReadTimeout) is never clobbered by a restore.
		return c.conn.SetReadDeadline(time.Time{})
	}
	if timeout := c.GetTimeout(); timeout > 0 {
		return c.conn.SetReadDeadline(time.Now().Add(timeout))
	}
	// A read deadline is absolute and persists across reads: once the timeout is
	// disabled we must clear any deadline armed by a previous read (or during
	// connection setup, e.g. ConnectTimeout), otherwise idle connections keep
	// tripping the stale deadline.
	return c.conn.SetReadDeadline(time.Time{})
}

// setDisarm enables or disables the read-deadline disarm used around the
// frame/segment header read (see Read). It deliberately leaves the operational
// timeout value untouched so it can be toggled without racing a concurrent
// finalizeConnection.
func (c *connReader) setDisarm(v bool) {
	c.disarm.Store(v)
}

func (c *connReader) Close() error {
	return c.conn.Close()
}

func (c *connReader) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

func (c *connReader) SetTimeout(timeout time.Duration) {
	c.timeout.Store(int64(timeout))
}

func (c *connReader) GetTimeout() time.Duration {
	return time.Duration(c.timeout.Load())
}

type callReq struct {
	// streamObserverContext is notified about events regarding this stream
	streamObserverContext StreamObserverContext
	// resp will receive the frame that was sent as a response to this stream.
	resp     chan callResp
	timeout  chan struct{} // indicates to recv() that a call has timed out
	timer    *time.Timer
	streamID int // current stream in use
	// streamObserverEndOnce ensures that either StreamAbandoned or StreamFinished is called,
	// but not both.
	streamObserverEndOnce sync.Once
	done                  sync.WaitGroup
}

var callReqPool = sync.Pool{
	New: func() any {
		return &callReq{
			resp: make(chan callResp),
		}
	},
}

func getCallReq(streamID int) *callReq {
	call := callReqPool.Get().(*callReq)
	call.timeout = make(chan struct{})
	call.streamID = streamID
	call.streamObserverContext = nil
	call.streamObserverEndOnce = sync.Once{}
	call.done = sync.WaitGroup{}
	call.done.Add(1)
	return call
}

func putCallReq(call *callReq) {
	if call.timer != nil {
		if !call.timer.Stop() {
			select {
			case <-call.timer.C:
			default:
			}
		}
	}
	call.streamObserverContext = nil
	call.streamObserverEndOnce = sync.Once{}
	call.streamID = 0
	call.timeout = nil
	callReqPool.Put(call)
}

func (call *callReq) finishExec() {
	call.done.Done()
}

func (call *callReq) waitExecDone(where string) {
	waitCallReqDone(call, where)
}

// removeCallIfOpen removes a call from c.calls only if exec() still owns its
// cleanup. Once the connection has started closing, closeWithError() becomes
// responsible for draining and recycling detached callReqs.
func (c *Conn) removeCallIfOpen(streamID int) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed || c.calls == nil {
		return false
	}

	delete(c.calls, streamID)
	return true
}

type callResp struct {
	// framer is the response frame.
	// May be nil if err is not nil.
	framer *framer
	// err is error encountered, if any.
	err error
}

// contextWriter is like io.Writer, but takes context as well.
type contextWriter interface {
	// writeContext writes p to the connection.
	//
	// If ctx is canceled before we start writing p (e.g. during waiting while another write is currently in progress),
	// p is not written and ctx.Err() is returned. Context is ignored after we start writing p (i.e. we don't interrupt
	// blocked writes that are in progress) so that we always either write the full frame or not write it at all.
	//
	// It returns the number of bytes written from p (0 <= n <= len(p)) and any error that caused the write to stop
	// early. writeContext must return a non-nil error if it returns n < len(p). writeContext must not modify the
	// data in p, even temporarily.
	writeContext(ctx context.Context, p []byte) (n int, err error)

	setWriteTimeout(timeout time.Duration)
}

type deadlineWriter interface {
	SetWriteDeadline(time.Time) error
	io.Writer
}

type deadlineContextWriter struct {
	w deadlineWriter
	// semaphore protects critical section for SetWriteDeadline/Write.
	// It is a channel with capacity 1.
	semaphore chan struct{}
	// quit closed once the connection is closed.
	quit    chan struct{}
	timeout atomic.Int64
}

func (c *deadlineContextWriter) setWriteTimeout(timeout time.Duration) {
	c.timeout.Store(int64(timeout))
}

// writeContext implements contextWriter.
func (c *deadlineContextWriter) writeContext(ctx context.Context, p []byte) (int, error) {
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	case <-c.quit:
		return 0, ErrConnectionClosed
	case c.semaphore <- struct{}{}:
		// acquired
	}

	defer func() {
		// release
		<-c.semaphore
	}()

	timeout := c.timeout.Load()
	if timeout > 0 {
		err := c.w.SetWriteDeadline(time.Now().Add(time.Duration(timeout)))
		if err != nil {
			return 0, err
		}
	}
	return c.w.Write(p)
}

func newWriteCoalescer(conn deadlineWriter, writeTimeout, coalesceDuration time.Duration,
	quit <-chan struct{}) *writeCoalescer {
	wc := &writeCoalescer{
		writeCh: make(chan writeRequest),
		c:       conn,
		quit:    quit,
	}
	wc.setWriteTimeout(writeTimeout)
	go wc.writeFlusher(coalesceDuration)
	return wc
}

type writeCoalescer struct {
	c                deadlineWriter
	quit             <-chan struct{}
	writeCh          chan writeRequest
	testEnqueuedHook func()
	testFlushedHook  func()
	timeout          atomic.Int64
}

func (w *writeCoalescer) setWriteTimeout(timeout time.Duration) {
	w.timeout.Store(int64(timeout))
}

type writeRequest struct {
	// resultChan is a channel (with buffer size 1) where to send results of the write.
	resultChan chan<- writeResult
	// data to write.
	data []byte
}

type writeResult struct {
	err error
	n   int
}

// writeResultChanPool pools buffered channels used for write coalescer results.
// Each channel is used in a strict produce-once/consume-once pattern:
// the flusher goroutine sends exactly one writeResult, and writeContext
// reads exactly one. After reading, the channel is empty and safe to reuse.
var writeResultChanPool = sync.Pool{
	New: func() interface{} {
		return make(chan writeResult, 1)
	},
}

// writeContext implements contextWriter.
func (w *writeCoalescer) writeContext(ctx context.Context, p []byte) (int, error) {
	resultChan := writeResultChanPool.Get().(chan writeResult)
	wr := writeRequest{
		resultChan: resultChan,
		data:       p,
	}

	select {
	case <-ctx.Done():
		writeResultChanPool.Put(resultChan)
		return 0, ctx.Err()
	case <-w.quit:
		writeResultChanPool.Put(resultChan)
		return 0, io.EOF // TODO: better error here?
	case w.writeCh <- wr:
		// enqueued for writing
	}

	if w.testEnqueuedHook != nil {
		w.testEnqueuedHook()
	}

	result := <-resultChan
	writeResultChanPool.Put(resultChan)
	return result.n, result.err
}

func (w *writeCoalescer) writeFlusher(interval time.Duration) {
	timer := time.NewTimer(interval)
	defer timer.Stop()

	if !timer.Stop() {
		<-timer.C
	}

	w.writeFlusherImpl(timer.C, func() { timer.Reset(interval) })
}

func (w *writeCoalescer) writeFlusherImpl(timerC <-chan time.Time, resetTimer func()) {
	running := false

	var buffers net.Buffers
	var resultChans []chan<- writeResult

	for {
		select {
		case req := <-w.writeCh:
			buffers = append(buffers, req.data)
			resultChans = append(resultChans, req.resultChan)
			if !running {
				// Start timer on first write.
				resetTimer()
				running = true
			}
		case <-w.quit:
			result := writeResult{
				n:   0,
				err: io.EOF, // TODO: better error here?
			}
			// Unblock whoever was waiting.
			for _, resultChan := range resultChans {
				// resultChan has capacity 1, so it does not block.
				resultChan <- result
			}
			return
		case <-timerC:
			running = false
			w.flush(resultChans, buffers)
			buffers = nil
			resultChans = nil
			if w.testFlushedHook != nil {
				w.testFlushedHook()
			}
		}
	}
}

func (w *writeCoalescer) flush(resultChans []chan<- writeResult, buffers net.Buffers) {
	// Flush everything we have so far.
	timeout := w.timeout.Load()
	if timeout > 0 {
		err := w.c.SetWriteDeadline(time.Now().Add(time.Duration(timeout)))
		if err != nil {
			for i := range resultChans {
				resultChans[i] <- writeResult{
					n:   0,
					err: err,
				}
			}
			return
		}
	}
	// Copy buffers because WriteTo modifies buffers in-place.
	buffers2 := make(net.Buffers, len(buffers))
	copy(buffers2, buffers)
	n, err := buffers2.WriteTo(w.c)
	// Writes of bytes before n succeeded, writes of bytes starting from n failed with err.
	// Use n as remaining byte counter.
	for i := range buffers {
		if int64(len(buffers[i])) <= n {
			// this buffer was fully written.
			resultChans[i] <- writeResult{
				n:   len(buffers[i]),
				err: nil,
			}
			n -= int64(len(buffers[i]))
		} else {
			// this buffer was not (fully) written.
			resultChans[i] <- writeResult{
				n:   int(n),
				err: err,
			}
			n = 0
		}
	}
}

// addCall attempts to add a call to c.calls.
// It fails with error if the connection already started closing or if a call for the given stream
// already exists.
func (c *Conn) addCall(call *callReq) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return ErrConnectionClosed
	}
	existingCall := c.calls[call.streamID]
	if existingCall != nil {
		return fmt.Errorf("attempting to use stream already in use: %d -> %d", call.streamID,
			existingCall.streamID)
	}
	c.calls[call.streamID] = call
	return nil
}

// exec executes a frame on the connection and returns the response framer.
//
// IMPORTANT: The caller takes ownership of the returned framer and MUST call
// framer.Release() when done reading the response. Failure to release the framer
// will leak memory and prevent buffer reuse.
//
// The framer should be released as soon as the response data is no longer needed,
// typically via defer immediately after parsing or after transferring ownership
// to an Iter.
func (c *Conn) exec(ctx context.Context, req frameBuilder, tracer Tracer, requestTimeout time.Duration) (*framer, error) {
	return c.execInternal(ctx, req, tracer, requestTimeout, true)
}

func (c *Conn) execInternal(ctx context.Context, req frameBuilder, tracer Tracer, requestTimeout time.Duration, startupCompleted bool) (*framer, error) {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, &QueryError{err: ctxErr, potentiallyExecuted: false}
	}

	// TODO: move tracer onto conn
	stream, ok := c.streams.GetStream()
	if !ok {
		return nil, &QueryError{err: ErrNoStreams, potentiallyExecuted: false}
	}

	// resp is basically a waiting semaphore protecting the framer
	framer := c.getWriteFramer()

	call := getCallReq(stream)

	if c.streamObserver != nil {
		call.streamObserverContext = c.streamObserver.StreamContext(ctx)
	}

	if err := c.addCall(call); err != nil {
		call.finishExec()
		putCallReq(call)
		c.releaseWriteFramer(framer)
		return nil, &QueryError{err: err, potentiallyExecuted: false}
	}

	// After this point, we need to either read from call.resp or close(call.timeout)
	// since closeWithError can try to write a connection close error to call.resp.
	// If we don't close(call.timeout) or read from call.resp, closeWithError can deadlock.

	var (
		stopWaiting   bool
		releaseStream bool
		recycleCall   bool
		closeErr      error
	)

	defer func() {
		if closeErr != nil {
			c.closeWithError(closeErr)
		}
	}()

	defer func() {
		if stopWaiting {
			close(call.timeout)
		}
		call.finishExec()
		if releaseStream {
			c.releaseStream(call)
		}
		if recycleCall {
			putCallReq(call)
		}
	}()

	if tracer != nil {
		framer.trace()
	}

	if call.streamObserverContext != nil {
		call.streamObserverContext.StreamStarted(ObservedStream{
			Host: c.host,
		})
	}

	err := req.buildFrame(framer, stream)
	if err != nil {
		c.releaseWriteFramer(framer)
		// closeWithError waits for exec() to stop touching the callReq, so the
		// deferred epilogue below is responsible for signaling completion.
		stopWaiting = true
		if c.removeCallIfOpen(call.streamID) {
			// We failed to serialize the frame into a buffer. This should not affect
			// the connection as we didn't write anything, so exec() still owns the
			// stream/call cleanup.
			releaseStream = true
			recycleCall = true
		}
		return nil, &QueryError{err: err, potentiallyExecuted: false}
	}

	if c.version > protoVersion4 && startupCompleted {
		if err = framer.prepareModernLayout(); err != nil {
			c.releaseWriteFramer(framer)
			// prepareModernLayout failed before any bytes were written, so this
			// is equivalent to a buildFrame failure: the connection is untouched
			// and the request was never sent. Signal completion via the deferred
			// epilogue and let exec() own the stream/call cleanup.
			stopWaiting = true
			if c.removeCallIfOpen(call.streamID) {
				releaseStream = true
				recycleCall = true
			}
			return nil, &QueryError{err: err, potentiallyExecuted: false}
		}
	}

	n, err := c.w.writeContext(ctx, framer.buf)
	c.releaseWriteFramer(framer)
	if err != nil {
		// closeWithError waits for exec() to stop touching the callReq, so defer
		// the completion signal and only record the cleanup we need here.
		stopWaiting = true
		if (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) && n == 0 {
			// We have not started to write this frame.
			// Release the stream as no response can come from the server on the stream.
			if c.removeCallIfOpen(call.streamID) {
				// We need to release the stream after we remove the call from c.calls,
				// otherwise the existingCall != nil check above could fail.
				releaseStream = true
				recycleCall = true
			}
		} else {
			// I think this is the correct thing to do, im not entirely sure. It is not
			// ideal as readers might still get some data, but they probably wont.
			// Here we need to be careful as the stream is not available and if all
			// writes just timeout or fail then the pool might use this connection to
			// send a frame on, with all the streams used up and not returned.
			closeErr = err
		}
		return nil, &QueryError{err: err, potentiallyExecuted: true}
	}

	var timeoutCh <-chan time.Time
	if requestTimeout > 0 {
		if call.timer == nil {
			call.timer = time.NewTimer(requestTimeout)
		} else {
			if !call.timer.Stop() {
				select {
				case <-call.timer.C:
				default:
				}
			}
			call.timer.Reset(requestTimeout)
		}
		timeoutCh = call.timer.C
	}

	var ctxDone <-chan struct{}
	if ctx != nil {
		ctxDone = ctx.Done()
	}

	select {
	case resp := <-call.resp:
		stopWaiting = true
		if resp.err != nil {
			c.releaseReadFramer(resp.framer)
			if !c.Closed() {
				// if the connection is closed then we cant release the stream,
				// this is because the request is still outstanding and we have
				// been handed another error from another stream which caused the
				// connection to close.
				releaseStream = true
				recycleCall = true
			}
			return nil, &QueryError{err: resp.err, potentiallyExecuted: true}
		}
		// dont release the stream if detect a timeout as another request can reuse
		// that stream and get a response for the old request, which we have no
		// easy way of detecting.
		//
		// Ensure that the stream is not released if there are potentially outstanding
		// requests on the stream to prevent nil pointer dereferences in recv().
		releaseStream = true
		recycleCall = true

		if v := resp.framer.header.Version.Version(); v != c.version {
			c.releaseReadFramer(resp.framer)
			return nil, &QueryError{err: NewErrProtocol("unexpected protocol version in response: got %d expected %d", v, c.version), potentiallyExecuted: true}
		}

		// NOTE: The returned framer becomes the caller's responsibility to release.
		// It is not released here to allow zero-copy access to the response data.
		// The caller must call Release() on the returned read framer when done reading the response.
		return resp.framer, nil
	case <-timeoutCh:
		stopWaiting = true
		return nil, &QueryError{err: ErrTimeoutNoResponse, potentiallyExecuted: true, timeout: requestTimeout, inFlight: c.streams.InUse()}
	case <-ctxDone:
		stopWaiting = true
		return nil, &QueryError{err: ctx.Err(), potentiallyExecuted: true, timeout: requestTimeout, inFlight: c.streams.InUse()}
	case <-c.ctx.Done():
		stopWaiting = true
		return nil, &QueryError{err: ErrConnectionClosed, potentiallyExecuted: true}
	}
}

// ObservedStream observes a single request/response stream.
type ObservedStream struct {
	// Host of the connection used to send the stream.
	Host *HostInfo
}

// StreamObserver is notified about request/response pairs.
// Streams are created for executing queries/batches or
// internal requests to the database and might live longer than
// execution of the query - the stream is still tracked until
// response arrives so that stream IDs are not reused.
type StreamObserver interface {
	// StreamContext is called before creating a new stream.
	// ctx is context passed to Session.Query / Session.Batch,
	// but might also be an internal context (for example
	// for internal requests that use control connection).
	// StreamContext might return nil if it is not interested
	// in the details of this stream.
	// StreamContext is called before the stream is created
	// and the returned StreamObserverContext might be discarded
	// without any methods called on the StreamObserverContext if
	// creation of the stream fails.
	// Note that if you don't need to track per-stream data,
	// you can always return the same StreamObserverContext.
	StreamContext(ctx context.Context) StreamObserverContext
}

// StreamObserverContext is notified about state of a stream.
// A stream is started every time a request is written to the server
// and is finished when a response is received.
// It is abandoned when the underlying network connection is closed
// before receiving a response.
type StreamObserverContext interface {
	// StreamStarted is called when the stream is started.
	// This happens just before a request is written to the wire.
	StreamStarted(observedStream ObservedStream)

	// StreamAbandoned is called when we stop waiting for response.
	// This happens when the underlying network connection is closed.
	// StreamFinished won't be called if StreamAbandoned is.
	StreamAbandoned(observedStream ObservedStream)

	// StreamFinished is called when we receive a response for the stream.
	StreamFinished(observedStream ObservedStream)
}

type preparedStatment struct {
	response         resultMetadata
	id               []byte
	resultMetadataID []byte
	request          preparedMetadata
}

type inflightPrepare struct {
	done chan struct{}
	err  error

	preparedStatment *preparedStatment
}

func (c *Conn) prepareStatement(ctx context.Context, stmt string, tracer Tracer, keyspace string, requestTimeout time.Duration) (*preparedStatment, error) {
	cacheKey := c.session.stmtsLRU.keyFor(c.host.hostUUID(), keyspace, stmt)
	flight, ok := c.session.stmtsLRU.execIfMissing(cacheKey, func(cache *lru.Cache[stmtCacheKey]) *inflightPrepare {
		flight := &inflightPrepare{
			done: make(chan struct{}),
		}
		cache.Add(cacheKey, flight)
		return flight
	})

	if !ok {
		go func() {
			defer close(flight.done)

			prep := &writePrepareFrame{
				statement: stmt,
			}
			if c.version > protoVersion4 {
				prep.keyspace = keyspace
			}

			// we won the race to do the load, if our context is canceled we shouldnt
			// stop the load as other callers are waiting for it but this caller should get
			// their context cancelled error.
			framer, err := c.exec(c.ctx, prep, tracer, requestTimeout)
			if err != nil {
				flight.err = err
				c.session.stmtsLRU.remove(cacheKey)
				return
			}
			defer framer.Release()

			frame, err := framer.parseFrame()
			if err != nil {
				flight.err = err
				c.session.stmtsLRU.remove(cacheKey)
				return
			}

			// TODO(zariel): tidy this up, simplify handling of frame parsing so its not duplicated
			// everytime we need to parse a frame.
			if len(framer.traceID) > 0 && tracer != nil {
				tracer.Trace(framer.traceID)
			}

			switch x := frame.(type) {
			case *resultPreparedFrame:
				flight.preparedStatment = &preparedStatment{
					// preparedID is already defensively copied by readShortBytesCopy()
					// in the frame parser; resultMetadataID likewise.
					id:               x.preparedID,
					resultMetadataID: x.resultMetadataID,
					request:          x.reqMeta,
					response:         x.respMeta,
				}
			case error:
				flight.err = x
			default:
				flight.err = NewErrProtocol("Unknown type in response to prepare frame: %s", x)
			}

			if flight.err != nil {
				c.session.stmtsLRU.remove(cacheKey)
			}
		}()
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-flight.done:
		return flight.preparedStatment, flight.err
	}
}

func marshalQueryValue(typ TypeInfo, value any, dst *queryValues) error {
	if named, ok := value.(*namedValue); ok {
		dst.name = named.name
		value = named.value
	}

	if _, ok := value.(unsetColumn); !ok {
		val, err := Marshal(typ, value)
		if err != nil {
			return err
		}

		dst.value = val
	} else {
		dst.isUnset = true
	}

	return nil
}

// shouldSkipResultMetadata reports whether an EXECUTE request should ask the
// server to skip result metadata in its RESULT/Rows response.
//
// A per-query NoSkipMetadata() (queryDisableSkipMetadata) always forces metadata.
//
// hasColumns is not only an optimization. A statement whose RESULT/Prepared
// response carries no result metadata cannot have its metadata reused, and on
// current ScyllaDB asking to skip it is unrecoverable: such a statement is handed
// an ID hashed from empty metadata, the server compares the returned ID against
// that same empty-metadata ID, always matches, and so never sets
// METADATA_CHANGED — leaving the driver with a response it has no columns to
// decode. LIST ROLES OF is the motivating case. The server-side fixes for this
// (scylladb/scylladb#29233, scylladb/scylladb#29275) are both closed unmerged, so
// this gate is load-bearing, not cosmetic. The Scylla python-driver and
// java-driver check the same thing, java-driver first of all.
//
// idTracked means the response carries a result metadata ID mechanism the driver
// can rely on: the connection speaks a protocol that exchanges result metadata
// IDs *and* the prepared statement holds a non-empty one. Both halves matter — the
// ID exchange is what makes the server report staleness with METADATA_CHANGED, and
// an ID is what it has to compare against. With both in hand skipping is safe, so
// the session-level DisableSkipMetadata is deliberately overridden, including when
// it was set explicitly. That flag is itself a workaround for the invalidation bug
// this mechanism fixes (https://github.com/scylladb/scylladb/issues/20860), and
// upstream gocql skips by default on every protocol version.
//
// This follows scylladb/scylla-drivers#81: skipping is the safe default "if
// SCYLLA_USE_METADATA_ID was negotiated or CQL v5 is used". The Scylla java-driver
// reaches the same rule from the other direction — DefaultPreparedStatement's
// resolveSkipMetadata returns true for any non-empty result metadata ID, which
// native v5 always supplies. The Scylla python-driver implements the extension half
// only, deliberately leaving native v5 alone.
//
// A statement prepared before the ID exchange was available has no ID. The prepared
// cache is keyed by host and survives reconnects, so that is reachable during a
// rolling upgrade. Such a statement asks for metadata for one more round trip: the
// empty ID it sends is treated as a mismatch, the server answers METADATA_CHANGED
// with a fresh ID, and later executions skip. Gating here means there is never a
// window where the driver skips metadata it cannot recover.
func shouldSkipResultMetadata(sessionDisableSkipMetadata, queryDisableSkipMetadata, idTracked, hasColumns bool) bool {
	disableSkipMeta := queryDisableSkipMetadata || (!idTracked && sessionDisableSkipMetadata)
	return !disableSkipMeta && hasColumns
}

// metadataIDTracked reports whether an EXECUTE for this prepared statement can rely
// on the server to report result-metadata changes, which is what makes skipping
// metadata safe. Both conditions are required: the connection must exchange result
// metadata IDs at all (Conn.tracksResultMetadataID — native v5 or the
// SCYLLA_USE_METADATA_ID extension), and the statement must carry a non-empty
// result metadata ID for the server to compare its own against. See
// shouldSkipResultMetadata.
func metadataIDTracked(idExchangeActive bool, resultMetadataID []byte) bool {
	return idExchangeActive && len(resultMetadataID) > 0
}

func (c *Conn) executeQuery(ctx context.Context, qry *Query) (iter *Iter) {
	return c.executeQueryWithMetrics(ctx, qry, qry.metrics)
}

func (c *Conn) executeQueryWithMetrics(ctx context.Context, qry *Query, metrics *queryMetrics) (iter *Iter) {
	params := queryParams{
		consistency: qry.cons,
	}

	// frame checks that it is not 0
	params.serialConsistency = qry.serialCons
	params.defaultTimestamp = qry.defaultTimestamp
	params.defaultTimestampValue = qry.defaultTimestampValue

	if len(qry.pageState) > 0 {
		params.pagingState = qry.pageState
	}
	if qry.pageSize > 0 {
		params.pageSize = qry.pageSize
	}
	// Always forward these to the framer regardless of protocol version. On
	// protocol < v5 the frame writer rejects them with an explicit
	// "unsupported option" error instead of silently dropping the value.
	params.keyspace = qry.keyspace
	params.nowInSeconds = qry.nowInSecondsValue

	// If a keyspace for the qry is overriden,
	// then we should use it to create stmt cache key
	usedKeyspace := c.getCurrentKeyspace()
	if qry.keyspace != "" {
		usedKeyspace = qry.keyspace
	}

	var (
		frame frameBuilder
		info  *preparedStatment
	)

	// The keyspace and table this attempt routes by, used below to attribute a
	// tablet-routing hint. Held in locals rather than read back out of
	// qry.routingInfo: one *Query (and one *queryRoutingInfo) is shared by every
	// speculative execution goroutine and by the auto-paging copy, so a sibling
	// goroutine can overwrite the cached pair between the write below and the read
	// at the end of this function. Seeded from the cache for the non-prepared path,
	// where only GetRoutingKey has written it.
	routingKeyspace, routingTable := qry.routingInfo.keyspaceTable()

	if !qry.skipPrepare && qry.shouldPrepare() {
		// Prepare all DML queries. Other queries can not be prepared.
		var err error
		info, err = c.prepareStatement(ctx, qry.stmt, qry.trace, usedKeyspace, qry.GetRequestTimeout())
		if err != nil {
			return &Iter{err: err}
		}

		values := qry.values
		if qry.binding != nil {
			values, err = qry.binding(&QueryInfo{
				Id:          info.id,
				Args:        info.request.columns,
				Rval:        info.response.columns,
				PKeyColumns: info.request.pkeyColumns,
			})

			if err != nil {
				return &Iter{err: err}
			}
		}

		if len(values) != info.request.actualColCount {
			return &Iter{err: fmt.Errorf("gocql: expected %d values send got %d", info.request.actualColCount, len(values))}
		}

		params.values = make([]queryValues, len(values))
		for i := 0; i < len(values); i++ {
			v := &params.values[i]
			value := values[i]
			typ := info.request.columns[i].TypeInfo
			if err := marshalQueryValue(typ, value, v); err != nil {
				return &Iter{err: err}
			}
		}

		params.skipMeta = shouldSkipResultMetadata(
			c.session.cfg.DisableSkipMetadata,
			qry.disableSkipMetadata,
			metadataIDTracked(c.tracksResultMetadataID(), info.resultMetadataID),
			len(info.response.columns) != 0,
		)

		frame = &writeExecuteFrame{
			preparedID:       info.id,
			resultMetadataID: info.resultMetadataID,
			params:           params,
			customPayload:    qry.customPayload,
		}

		// Set "lwt", keyspace", "table" property in the query if it is present in preparedMetadata
		routingKeyspace = info.request.keyspace
		if routingKeyspace == "" {
			routingKeyspace = usedKeyspace
		}
		routingTable = info.request.table

		qry.routingInfo.mu.Lock()
		qry.routingInfo.lwt = info.request.lwt
		qry.routingInfo.keyspace = routingKeyspace
		qry.routingInfo.table = routingTable
		qry.routingInfo.mu.Unlock()
	} else {
		frame = &writeQueryFrame{
			statement:     qry.stmt,
			params:        params,
			customPayload: qry.customPayload,
		}
	}

	framer, err := c.exec(ctx, frame, qry.trace, qry.GetRequestTimeout())
	if err != nil {
		return &Iter{err: err}
	}
	warningHandler := WarningHandler(nil)
	if c.session != nil {
		warningHandler = c.session.warningHandler
	}

	resp, err := framer.parseFrame()
	if err != nil {
		return newErrorIterWithReleasedFramer(err, framer).
			bindWarningHandlerWithMetrics(qry, metrics, warningHandler)
	}

	if len(framer.customPayload) > 0 {
		if hint, ok := framer.customPayload["tablets-routing-v1"]; ok {
			tablet, err := unmarshalTabletHint(hint, c.version, routingKeyspace, routingTable)
			if err != nil {
				return newErrorIterWithReleasedFramer(err, framer).
					bindWarningHandlerWithMetrics(qry, metrics, warningHandler)
			}
			c.session.metadataDescriber.AddTablet(tablet)
		}
	}

	if len(framer.traceID) > 0 && qry.trace != nil {
		qry.trace.Trace(framer.traceID)
	}

	switch x := resp.(type) {
	case *resultVoidFrame:
		return (&Iter{framer: framer}).
			bindWarningHandlerWithMetrics(qry, metrics, warningHandler)
	case *resultRowsFrame:
		if x.meta.noMetaData() && info == nil {
			return newErrorIterWithReleasedFramer(
				errors.New("gocql: did not receive metadata but prepared info is nil"),
				framer,
			).bindWarningHandlerWithMetrics(qry, metrics, warningHandler)
		}

		if x.meta.newMetadataID != nil && x.meta.noMetaData() {
			// METADATA_CHANGED obliges the server to include the new metadata, so this
			// response is malformed, and there are two wrong ways to continue.
			//
			// Adopting the new ID while keeping the old columns is unrecoverable: the
			// server would match the ID from then on and stop sending metadata, leaving
			// the driver decoding rows against stale columns indefinitely. The
			// python-driver guards that the same way.
			//
			// Decoding *this* response against the cached columns is no better. The
			// server has just declared them stale, and the noMetaData() branch below
			// would reuse them anyway — which is precisely the misdecode this whole
			// mechanism exists to prevent (scylladb/scylladb#20860).
			//
			// So do neither. Fail the query with the old ID still cached: a retry
			// resends it, the server reports the mismatch again, and it has another
			// chance to answer with the metadata it owes.
			return newErrorIterWithReleasedFramer(
				fmt.Errorf("gocql: server reported changed result metadata for %q but sent no column metadata", qry.stmt),
				framer,
			).bindWarningHandlerWithMetrics(qry, metrics, warningHandler)
		}

		if x.meta.newMetadataID != nil {
			// If a RESULT/Rows message reports changed resultset metadata with the
			// Metadata_changed flag, the reported new resultset metadata must be used
			// in subsequent executions.
			cacheKey := c.session.stmtsLRU.keyFor(c.host.hostUUID(), usedKeyspace, qry.stmt)
			// Use the already-completed local `info` rather than dereferencing the
			// cached inflight entry's preparedStatment field. `info` comes from the
			// prepareStatement call above and is guaranteed complete.
			//
			// updateMetadataIfSame performs the presence/identity check and the
			// replacement atomically under the cache lock, so a concurrent eviction
			// or a newer/in-flight prepare installed for the same key between check
			// and replace cannot be resurrected or clobbered. It only replaces the
			// entry while it is still the exact prepared statement `info` points to
			// (pointer identity, not id bytes), so a same-id reprepare of a newer
			// generation is left untouched.
			//
			// `response` caches this whole resultMetadata, so it keeps this response's
			// flags (METADATA_CHANGED, possibly HasMorePages) and pagingState alongside
			// the columns. Only the columns are reused: the code below reads
			// morePages()/noMetaData() off the live x.meta, and overwrites
			// iter.meta.pagingState from it.
			if info != nil {
				newInflight := &inflightPrepare{
					done: make(chan struct{}),
					preparedStatment: &preparedStatment{
						id:               info.id,
						resultMetadataID: x.meta.newMetadataID,
						request:          info.request,
						response:         x.meta,
					},
				}
				// Close done to avoid deadlocks on subsequent requests waiting on this.
				close(newInflight.done)
				if c.session.stmtsLRU.updateMetadataIfSame(cacheKey, info, newInflight) {
					// Update info so the code below sees the updated prepared statement.
					info = newInflight.preparedStatment
				}
			}
		}

		iter := (&Iter{
			meta:    x.meta,
			framer:  framer,
			numRows: x.numRows,
		}).bindWarningHandlerWithMetrics(qry, metrics, warningHandler)

		if x.meta.noMetaData() {
			iter.meta = info.response
			// pagingState is already independently allocated by readBytesCopy()
			// during frame parsing, no additional copy needed.
			iter.meta.pagingState = x.meta.pagingState
		}

		if x.meta.morePages() && !qry.disableAutoPage {
			newQry := cloneQueryForNextPage(qry, metrics, x.meta.pagingState)

			iter.next = newNextIter(newQry, int((1-qry.prefetch)*float64(x.numRows)))

			if iter.next.pos < 1 {
				iter.next.pos = 1
			}
		}

		return iter
	case *resultKeyspaceFrame:
		return (&Iter{framer: framer}).
			bindWarningHandlerWithMetrics(qry, metrics, warningHandler)
	case *frm.SchemaChangeKeyspace, *frm.SchemaChangeTable, *frm.SchemaChangeFunction, *frm.SchemaChangeAggregate, *frm.SchemaChangeType:
		iter := (&Iter{framer: framer}).
			bindWarningHandlerWithMetrics(qry, metrics, warningHandler)
		if err := c.awaitSchemaAgreement(ctx); err != nil {
			// TODO: should have this behind a flag
			c.logger.Println(err)
		}
		// dont return an error from this, might be a good idea to give a warning
		// though. The impact of this returning an error would be that the cluster
		// is not consistent with regards to its schema.
		return iter
	case *RequestErrUnprepared:
		stmtCacheKey := c.session.stmtsLRU.keyFor(c.host.hostUUID(), usedKeyspace, qry.stmt)
		c.session.stmtsLRU.evictPreparedID(stmtCacheKey, x.StatementId)
		framer.Release()
		return c.executeQueryWithMetrics(ctx, qry, metrics)
	case error:
		return newErrorIterWithReleasedFramer(x, framer).
			bindWarningHandlerWithMetrics(qry, metrics, warningHandler)
	default:
		return newErrorIterWithReleasedFramer(
			NewErrProtocol("Unknown type in response to execute query (%T): %s", x, x),
			framer,
		).bindWarningHandlerWithMetrics(qry, metrics, warningHandler)
	}
}

func cloneQueryForNextPage(qry *Query, metrics *queryMetrics, pageState []byte) *Query {
	next := cloneQuery(qry, metrics)
	next.pageState = pageState
	// Automatic pages belong to one logical execution, and newNextIter pins
	// this exact run until the page is retired.
	return next
}

func (c *Conn) Pick(qry *Query) *Conn {
	if c.Closed() {
		return nil
	}
	return c
}

func (c *Conn) Closed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

func (c *Conn) Address() string {
	return c.addr
}

func (c *Conn) AvailableStreams() int {
	return c.streams.Available()
}

func useKeyspaceStmt(keyspace string) string {
	return `USE "` + strings.ReplaceAll(keyspace, `"`, `""`) + `"`
}

func (c *Conn) UseKeyspace(keyspace string) error {
	q := &writeQueryFrame{statement: useKeyspaceStmt(keyspace)}
	q.params.consistency = c.session.cons

	framer, err := c.exec(c.ctx, q, nil, c.cfg.ConnectTimeout)
	if err != nil {
		return err
	}
	defer framer.Release()

	resp, err := framer.parseFrame()
	if err != nil {
		return err
	}

	switch x := resp.(type) {
	case *resultKeyspaceFrame:
	case error:
		return x
	default:
		return NewErrProtocol("unknown frame in response to USE: %v", x)
	}

	c.setCurrentKeyspace(keyspace)

	return nil
}

// setCurrentKeyspace records the keyspace this connection was switched to. See the
// currentKeyspace field for why it is atomic.
func (c *Conn) setCurrentKeyspace(keyspace string) {
	c.currentKeyspace.Store(&keyspace)
}

// getCurrentKeyspace returns the keyspace this connection was switched to, or ""
// if it was never switched (Cluster.Keyspace empty, or a control connection).
func (c *Conn) getCurrentKeyspace() string {
	if ks := c.currentKeyspace.Load(); ks != nil {
		return *ks
	}
	return ""
}

func (c *Conn) executeBatch(ctx context.Context, batch *Batch) (iter *Iter) {
	n := len(batch.Entries)
	req := &writeBatchFrame{
		typ:                   batch.Type,
		statements:            make([]batchStatment, n),
		consistency:           batch.Cons,
		serialConsistency:     batch.serialCons,
		defaultTimestamp:      batch.defaultTimestamp,
		defaultTimestampValue: batch.defaultTimestampValue,
		customPayload:         batch.CustomPayload,
	}

	// Always forward these to the framer regardless of protocol version. On
	// protocol < v5 the frame writer rejects them with an explicit
	// "unsupported option" error instead of silently dropping the value.
	req.keyspace = batch.keyspace
	req.nowInSeconds = batch.nowInSeconds

	usedKeyspace := c.getCurrentKeyspace()
	if batch.keyspace != "" {
		usedKeyspace = batch.keyspace
	}

	stmts := make(map[string]string, len(batch.Entries))

	hasLwtEntries := false

	for i := 0; i < n; i++ {
		entry := &batch.Entries[i]
		b := &req.statements[i]

		if len(entry.Args) > 0 || entry.binding != nil {
			info, err := c.prepareStatement(batch.Context(), entry.Stmt, batch.trace, usedKeyspace, batch.GetRequestTimeout())
			if err != nil {
				return &Iter{err: err}
			}

			var values []any
			if entry.binding == nil {
				values = entry.Args
			} else {
				values, err = entry.binding(&QueryInfo{
					Id:          info.id,
					Args:        info.request.columns,
					Rval:        info.response.columns,
					PKeyColumns: info.request.pkeyColumns,
				})
				if err != nil {
					return &Iter{err: err}
				}
			}

			if len(values) != info.request.actualColCount {
				return &Iter{err: fmt.Errorf("gocql: batch statement %d expected %d values send got %d", i, info.request.actualColCount, len(values))}
			}

			b.preparedID = info.id
			stmts[string(info.id)] = entry.Stmt

			b.values = make([]queryValues, info.request.actualColCount)

			for j := 0; j < info.request.actualColCount; j++ {
				v := &b.values[j]
				value := values[j]
				typ := info.request.columns[j].TypeInfo
				if err := marshalQueryValue(typ, value, v); err != nil {
					return &Iter{err: err}
				}
			}

			if !hasLwtEntries && info.request.lwt {
				hasLwtEntries = true
			}
		} else {
			b.statement = entry.Stmt
		}
	}

	// The batch is considered to be conditional if even one of the
	// statements is conditional.
	batch.routingInfo.mu.Lock()
	batch.routingInfo.lwt = hasLwtEntries
	batch.routingInfo.mu.Unlock()

	// TODO: should batch support tracing?
	framer, err := c.exec(batch.Context(), req, batch.trace, batch.GetRequestTimeout())
	if err != nil {
		return &Iter{err: err}
	}
	warningHandler := WarningHandler(nil)
	if c.session != nil {
		warningHandler = c.session.warningHandler
	}

	resp, err := framer.parseFrame()
	if err != nil {
		return newErrorIterWithReleasedFramer(err, framer).bindWarningHandler(batch, warningHandler)
	}

	if len(framer.traceID) > 0 && batch.trace != nil {
		batch.trace.Trace(framer.traceID)
	}

	switch x := resp.(type) {
	case *resultVoidFrame:
		return (&Iter{framer: framer}).bindWarningHandler(batch, warningHandler)
	case *RequestErrUnprepared:
		stmt, found := stmts[string(x.StatementId)]
		if found {
			key := c.session.stmtsLRU.keyFor(c.host.hostUUID(), usedKeyspace, stmt)
			c.session.stmtsLRU.evictPreparedID(key, x.StatementId)
		}
		framer.Release()
		return c.executeBatch(ctx, batch)
	case *resultRowsFrame:
		iter := (&Iter{
			meta:    x.meta,
			framer:  framer,
			numRows: x.numRows,
		}).bindWarningHandler(batch, warningHandler)

		return iter
	case error:
		return newErrorIterWithReleasedFramer(x, framer).bindWarningHandler(batch, warningHandler)
	default:
		return newErrorIterWithReleasedFramer(NewErrProtocol("Unknown type in response to batch statement: %s", x), framer).bindWarningHandler(batch, warningHandler)
	}
}

func (c *Conn) querySystem(ctx context.Context, query string, values ...any) *Iter {
	stmt, timeout := c.systemRequestStatement(query)
	q := c.session.Query(stmt, values...).Consistency(One).Trace(nil)
	q.skipPrepare = true
	q.disableSkipMetadata = true
	// we want to keep the query on this connection
	q.conn = c
	q.SetRequestTimeout(timeout)
	return c.executeQuery(ctx, q)
}

const qrySystemPeers = "SELECT peer, data_center, host_id, rack, release_version, rpc_address, schema_version, tokens FROM system.peers"
const qrySystemPeersCassandra = "SELECT peer, data_center, host_id, preferred_ip, rack, release_version, rpc_address, schema_version, tokens FROM system.peers"
const qrySystemPeersV2 = "SELECT peer, data_center, host_id, native_address, native_port, preferred_ip, rack, release_version, schema_version, tokens FROM system.peers_v2"

const qrySystemLocal = "SELECT broadcast_address, cluster_name, data_center, host_id, listen_address, partitioner, rack, release_version, rpc_address, schema_version, tokens FROM system.local WHERE key='local'"

func getSchemaAgreement(localSchemaVersion string, querySystemPeersRows []schemaAgreementHost, logger StdLogger) error {
	versions := make(map[string]struct{})

	for _, row := range querySystemPeersRows {
		if !row.IsValid() {
			logger.Printf("invalid peer or peer with empty schema_version: peer=%q", row)
			continue
		}
		versions[row.SchemaVersion.String()] = struct{}{}
	}

	if localSchemaVersion != "" {
		versions[localSchemaVersion] = struct{}{}
	}

	if len(versions) > 1 {
		schemas := make([]string, 0, len(versions))
		for schema := range versions {
			schemas = append(schemas, schema)
		}

		return &ErrSchemaMismatch{schemas: schemas}
	}

	return nil
}

type schemaAgreementHost struct {
	DataCenter    string
	Rack          string
	RPCAddress    string
	HostID        UUID
	SchemaVersion UUID
}

func (h *schemaAgreementHost) IsValid() bool {
	return h.DataCenter != "" && h.Rack != "" && h.HostID.String() != "" && h.SchemaVersion.String() != ""
}

func (c *Conn) awaitSchemaAgreement(ctx context.Context) error {
	endDeadline := time.Now().Add(c.session.cfg.MaxWaitSchemaAgreement)
	deadlineCtx, cancelDeadline := context.WithDeadline(ctx, endDeadline)
	defer cancelDeadline()

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	var lastErr error
	for time.Now().Before(endDeadline) {
		queryCtx, cancel := context.WithCancel(deadlineCtx)

		var (
			hosts              []schemaAgreementHost
			localSchemaVersion string
			wg                 sync.WaitGroup
			errMu              sync.Mutex
			firstErr           error
		)

		recordErr := func(err error) {
			if err == nil {
				return
			}
			errMu.Lock()
			defer errMu.Unlock()
			if firstErr == nil {
				firstErr = err
				cancel()
			}
		}

		wg.Add(2)
		go func() {
			defer wg.Done()
			var query string
			if c.getIsSchemaV2() {
				query = "SELECT host_id, data_center, rack, schema_version, preferred_ip FROM system.peers_v2"
			} else {
				query = "SELECT host_id, data_center, rack, schema_version, rpc_address FROM system.peers"
			}
			iter := c.querySystem(queryCtx, query)
			var tmp schemaAgreementHost
			for iter.Scan(&tmp.HostID, &tmp.DataCenter, &tmp.Rack, &tmp.SchemaVersion, &tmp.RPCAddress) {
				hosts = append(hosts, tmp)
			}
			recordErr(iter.Close())
		}()
		go func() {
			defer wg.Done()
			iter := c.querySystem(queryCtx, "SELECT schema_version FROM system.local WHERE key='local'")
			for iter.Scan(&localSchemaVersion) {
			}
			recordErr(iter.Close())
		}()
		wg.Wait()
		cancel()

		if ctx.Err() != nil {
			return ctx.Err()
		}
		if firstErr != nil {
			if deadlineCtx.Err() != nil {
				// The internal per-round context hit endDeadline rather than a real
				// query failure; preserve it as lastErr and let the loop condition
				// above exit naturally instead of surfacing a raw deadline error.
				lastErr = firstErr
				break
			}
			return firstErr
		}

		if err := getSchemaAgreement(localSchemaVersion, hosts, c.logger); err == ErrConnectionClosed || err == nil {
			return err
		} else {
			lastErr = err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadlineCtx.Done():
		case <-ticker.C:
		}
	}

	return lastErr
}

var (
	ErrQueryArgLength    = errors.New("gocql: query argument length mismatch")
	ErrTimeoutNoResponse = errors.New("gocql: no response received from cassandra within timeout period")
	// Deprecated: ErrTooManyTimeouts is no longer produced by the library.
	// It will be removed in a future major release.
	ErrTooManyTimeouts     = errors.New("gocql: too many query timeouts on the connection")
	ErrConnectionClosed    = errors.New("gocql: connection closed waiting for response")
	ErrNoStreams           = errors.New("gocql: no streams available on connection")
	ErrHostDown            = errors.New("gocql: host is nil or down")
	ErrNoPool              = errors.New("gocql: host does not have a pool")
	ErrNoConnectionsInPool = errors.New("gocql: host pool does not have connections")
)

type ErrSchemaMismatch struct {
	schemas []string
}

func (e *ErrSchemaMismatch) Error() string {
	return fmt.Sprintf("gocql: cluster schema versions not consistent: %+v", e.schemas)
}

type QueryError struct {
	err                 error
	timeout             time.Duration
	inFlight            int
	potentiallyExecuted bool
	isIdempotent        bool
}

func (e *QueryError) IsIdempotent() bool {
	return e.isIdempotent
}

func (e *QueryError) PotentiallyExecuted() bool {
	return e.potentiallyExecuted
}

func (e *QueryError) Error() string {
	if e.timeout > 0 {
		return fmt.Sprintf("%s (timeout: %v, in-flight: %d) (potentially executed: %v)", e.err.Error(), e.timeout, e.inFlight, e.potentiallyExecuted)
	}
	return fmt.Sprintf("%s (potentially executed: %v)", e.err.Error(), e.potentiallyExecuted)
}

func (e *QueryError) Unwrap() error {
	return e.err
}

func unmarshalTabletHint(hint []byte, _ uint8, keyspace, table string) (tablets.TabletInfo, error) {
	return tablets.ParseHint(hint, keyspace, table)
}
