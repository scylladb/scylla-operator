package transport

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/scylladb/scylla-go-driver/frame"
	. "github.com/scylladb/scylla-go-driver/frame/request"
	. "github.com/scylladb/scylla-go-driver/frame/response"

	"go.uber.org/atomic"
)

type response struct {
	frame.Header
	frame.Response
	Err error
}

type ResponseHandler chan response

type request struct {
	frame.Request
	StreamID        frame.StreamID
	Compress        bool
	Tracing         bool
	ResponseHandler ResponseHandler

	ctx context.Context // nolint:containedctx // cancelling sending request can't be done without it.
}

type skippedError struct {
	err error
}

func (e *skippedError) Error() string {
	return fmt.Sprintf("request skipped: %s", e.err)
}

func (e *skippedError) Unwrap() error {
	return e.err
}

var _connCloseRequest = request{ctx: context.Background()}

type stats struct {
	inFlight atomic.Uint32
	inQueue  atomic.Uint32
}

type connWriter struct {
	conn       *bufio.Writer
	buf        frame.Buffer
	compr      *compr
	requestCh  chan request
	stats      *stats
	connString func() string
	connClose  func()

	// For use only when skipping sending a request.
	freeStream func(frame.StreamID)
}

func (c *connWriter) submit(r request) {
	c.stats.inQueue.Inc()
	c.requestCh <- r
}

func (c *connWriter) loop(ctx context.Context) {
	for {
		size := len(c.requestCh)
		// If there are no requests backoff.
		// Through experimentation, we know that, sleeping more than 1ms makes no difference or is counterproductive.
		if size == 0 {
			time.Sleep(time.Millisecond)
			size = len(c.requestCh)
		}
		if size == 0 {
			size = 1
		} else if size > maxCoalescedRequests {
			size = maxCoalescedRequests
		}

		for i := 0; i < size; i++ {
			r := <-c.requestCh
			if r == _connCloseRequest {
				return
			}
			c.stats.inQueue.Dec()
			if err := c.send(ctx, r); err != nil {
				r.ResponseHandler <- response{Err: fmt.Errorf("%s send: %w", c.connString(), err)}
				if _, ok := err.(*skippedError); ok {
					c.freeStream(r.StreamID)
					continue
				}
				log.Printf("%s fatal send error, closing connection due to %s", c.connString(), err)
				c.connClose()
				return
			}
			c.stats.inFlight.Inc()
		}
		if err := c.conn.Flush(); err != nil {
			log.Printf("%s fatal flush error, closing connection due to %s", c.connString(), err)
			c.connClose()
			return
		}
	}
}

func (c *connWriter) send(ctx context.Context, r request) error {
	c.buf.Reset()

	// Dump request with header to buffer
	h := frame.Header{
		Version:  frame.CQLv4,
		StreamID: r.StreamID,
		OpCode:   r.OpCode(),
	}
	h.WriteTo(&c.buf)
	r.WriteTo(&c.buf)

	// Update length in header
	b := c.buf.Bytes()
	l := uint32(len(b) - frame.HeaderSize)
	binary.BigEndian.PutUint32(b[5:9], l)

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("send cancelled due to %w", err)
	}

	if r.ctx.Err() != nil {
		return &skippedError{err: r.ctx.Err()}
	}

	// Send
	var err error
	if r.Compress {
		if c.compr != nil {
			_, err = c.compr.compress(ctx, r.ctx, c.conn, c.buf.BytesBuffer())
		} else {
			return errComprUnspecified
		}
	} else {
		_, err = frame.CopyBuffer(&c.buf, c.conn)
	}
	return err
}

type connReader struct {
	conn        io.LimitedReader
	buf         frame.Buffer
	bufw        io.Writer
	stats       *stats
	compr       *compr
	handleEvent func(r response)
	connString  func() string
	connClose   func()

	h      map[frame.StreamID]ResponseHandler
	s      streamIDAllocator
	closed bool
	mu     sync.Mutex // mu guards h, s and closed
}

func (c *connReader) setHandler(h ResponseHandler) (frame.StreamID, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return invalidStreamID, fmt.Errorf("%s closed", c.connString())
	}

	streamID, err := c.s.Alloc()
	if err != nil {
		return streamID, fmt.Errorf("%s stream ID alloc: %w", c.connString(), err)
	}

	c.h[streamID] = h
	return streamID, err
}

// handler free given streamID and return corresponding handler.
func (c *connReader) handler(streamID frame.StreamID) ResponseHandler {
	c.mu.Lock()
	h := c.h[streamID]
	c.s.Free(streamID)
	delete(c.h, streamID)
	c.mu.Unlock()
	return h
}

func (c *connReader) freeStream(streamID frame.StreamID) {
	c.mu.Lock()
	c.s.Free(streamID)
	delete(c.h, streamID)
	c.mu.Unlock()
}

// loop terminates when its connection gets closed by the pool, especially when session context is done.
func (c *connReader) loop() {
	c.bufw = frame.BufferWriter(&c.buf)
	for {
		resp := c.recv()
		if resp.StreamID == eventStreamID {
			if c.handleEvent != nil {
				c.handleEvent(resp)
			}
			continue
		}

		if resp.Err != nil {
			log.Printf("%s fatal receive error, closing connection due to %s", c.connString(), resp.Err)
			c.connClose()
			c.drainHandlers()
			return
		}

		c.stats.inFlight.Dec()

		if h := c.handler(resp.StreamID); h != nil {
			h <- resp
		} else {
			log.Printf("%s received unknown stream ID %d, closing connection", c.connString(), resp.StreamID)
			c.connClose()
			c.drainHandlers()
			return
		}
	}
}

func (c *connReader) recv() response {
	c.buf.Reset()

	var r response

	// Read header
	c.conn.N = frame.HeaderSize
	if _, err := io.Copy(c.bufw, &c.conn); err != nil {
		r.Err = fmt.Errorf("read header: %w", err)
		return r
	}
	r.Header = frame.ParseHeader(&c.buf)
	if err := c.buf.Error(); err != nil {
		r.Err = fmt.Errorf("parse header: %w", err)
		return r
	}

	// Read body
	c.conn.N = int64(r.Header.Length)
	if r.Header.Flags&frame.Compress != 0 {
		if _, err := c.compr.decompress(c.bufw, &c.conn); err != nil {
			r.Err = fmt.Errorf("read body: %w", err)
			return r
		}
	} else {
		if _, err := io.Copy(c.bufw, &c.conn); err != nil {
			r.Err = fmt.Errorf("read body: %w", err)
			return r
		}
	}

	r.Response = c.parse(r.Header.OpCode)
	if r.Response == nil {
		r.Err = fmt.Errorf("response type not supported")
		return r
	}
	if err := c.buf.Error(); err != nil {
		r.Err = fmt.Errorf("parse body: %w", err)
		return r
	}

	return r
}

func (c *connReader) drainHandlers() {
	c.mu.Lock()
	c.closed = true
	for _, h := range c.h {
		h <- response{Err: fmt.Errorf("%s closed", c.connString())}
	}
	c.mu.Unlock()
}

func (c *connReader) parse(op frame.OpCode) frame.Response {
	// TODO add all responses
	switch op {
	case frame.OpError:
		return ParseError(&c.buf)
	case frame.OpReady:
		return ParseReady(&c.buf)
	case frame.OpResult:
		return ParseResult(&c.buf)
	case frame.OpSupported:
		return ParseSupported(&c.buf)
	case frame.OpEvent:
		return ParseEvent(&c.buf)
	case frame.OpAuthenticate:
		return ParseAuthenticate(&c.buf)
	case frame.OpAuthSuccess:
		return ParseAuthSuccess(&c.buf)
	case frame.OpAuthChallenge:
		return ParseAuthChallenge(&c.buf)
	default:
		log.Fatalf("not supported")
		return nil
	}
}

type Conn struct {
	cfg       ConnConfig
	conn      net.Conn
	event     ConnEvent
	w         connWriter
	r         connReader
	stats     *stats
	closeOnce sync.Once
	onClose   func(conn *Conn)
}

type ConnConfig struct {
	Username   string
	Password   string
	Keyspace   string
	TCPNoDelay bool
	Timeout    time.Duration

	// If not nil, all connections will use TLS according to TLSConfig,
	// please note that the default port (9042) may not support TLS.
	TLSConfig *tls.Config

	DefaultConsistency frame.Consistency
	DefaultPort        string

	Compression     frame.Compression
	ComprBufferSize int

	ConnObserver ConnObserver
}

func DefaultConnConfig(keyspace string) ConnConfig {
	return ConnConfig{
		Username:           "cassandra",
		Password:           "cassandra",
		Keyspace:           keyspace,
		TCPNoDelay:         true,
		Timeout:            500 * time.Millisecond,
		DefaultConsistency: frame.LOCALQUORUM,
		DefaultPort:        "9042",
		ConnObserver:       LoggingConnObserver{},
		ComprBufferSize:    comprBufferSize,
	}
}

const (
	requestChanSize      = maxStreamID / 2
	targetWaiting        = requestChanSize
	maxCoalescedRequests = 100
	ioBufferSize         = 8192
	comprBufferSize      = 64 * 1024 // 64 Kb
)

// OpenShardConn opens connection mapped to a specific shard on Scylla node.
func OpenShardConn(ctx context.Context, addr string, si ShardInfo, cfg ConnConfig) (*Conn, error) {
	it := ShardPortIterator(si)
	maxTries := (maxPort-minPort+1)/int(si.NrShards) + 1
	for i := 0; i < maxTries; i++ {
		conn, err := OpenLocalPortConn(ctx, addr, it(), cfg)
		if err != nil {
			log.Printf("%s dial error: %s (try %d/%d)", addr, err, i, maxTries)
			if conn != nil {
				conn.Close()
			}
			continue
		}
		return conn, nil
	}

	return nil, fmt.Errorf("failed to open connection on shard %d: all local ports are busy", si.Shard)
}

// OpenLocalPortConn opens connection on a given local port.
//
// If error and connection are returned the connection is not valid and must be closed by the caller.
func OpenLocalPortConn(ctx context.Context, addr string, localPort uint16, cfg ConnConfig) (*Conn, error) {
	localAddr, err := net.ResolveTCPAddr("tcp", ":"+strconv.Itoa(int(localPort)))
	if err != nil {
		return nil, fmt.Errorf("resolve local TCP address: %w", err)
	}

	return OpenConn(ctx, addr, localAddr, cfg)
}

// OpenConn opens connection with specific local address.
// In case lAddr is nil, random local address is used.
//
// If error and connection are returned the connection is not valid and must be closed by the caller.
func OpenConn(ctx context.Context, addr string, localAddr *net.TCPAddr, cfg ConnConfig) (*Conn, error) {
	d := net.Dialer{
		Timeout:   cfg.Timeout,
		LocalAddr: localAddr,
	}
	conn, err := d.DialContext(ctx, "tcp", withPort(addr, cfg.DefaultPort))
	if err != nil {
		return nil, fmt.Errorf("dial TCP address %s: %w", addr, err)
	}

	tcpConn := conn.(*net.TCPConn)
	if err := tcpConn.SetNoDelay(cfg.TCPNoDelay); err != nil {
		return nil, fmt.Errorf("set TCP no delay option: %w", err)
	}

	if cfg.TLSConfig != nil {
		tConn, err := WrapTLS(ctx, tcpConn, cfg.TLSConfig)
		if err != nil {
			return nil, err
		}

		return WrapConn(ctx, tConn, cfg)
	}

	return WrapConn(ctx, tcpConn, cfg)
}

func WrapTLS(ctx context.Context, conn *net.TCPConn, cfg *tls.Config) (net.Conn, error) {
	cfg = cfg.Clone()
	tconn := tls.Client(conn, cfg)
	if err := tconn.HandshakeContext(ctx); err != nil {
		if err := tconn.Close(); err != nil {
			log.Printf("%s failed to close: %s", tconn.RemoteAddr(), err)
		} else {
			log.Printf("%s closed", tconn.RemoteAddr())
		}

		return nil, err
	}

	return tconn, nil
}

// WrapConn transforms tcp connection to a working Scylla connection.
// If error and connection are returned the connection is not valid and must be closed by the caller.
func WrapConn(ctx context.Context, conn net.Conn, cfg ConnConfig) (*Conn, error) {
	s := new(stats)
	c := new(Conn)
	*c = Conn{
		cfg:  cfg,
		conn: conn,
		event: ConnEvent{
			Addr:  conn.RemoteAddr().String(),
			Shard: UnknownShard,
		},
		w: connWriter{
			conn:       bufio.NewWriterSize(conn, ioBufferSize),
			requestCh:  make(chan request, requestChanSize),
			stats:      s,
			connString: c.String,
			connClose:  c.Close,
		},
		r: connReader{
			conn: io.LimitedReader{
				R: bufio.NewReaderSize(conn, ioBufferSize),
			},
			stats:      s,
			h:          make(map[frame.StreamID]ResponseHandler),
			connString: c.String,
			connClose:  c.Close,
		},
		stats: s,
	}
	c.w.freeStream = c.r.freeStream

	if cfg.Compression != "" {
		if compr, err := newCompr(false, cfg.Compression, cfg.ComprBufferSize); err != nil {
			return c, err
		} else {
			c.r.compr = compr
		}
		if compr, err := newCompr(true, cfg.Compression, cfg.ComprBufferSize); err != nil {
			return c, err
		} else {
			c.w.compr = compr
		}
	}

	go c.w.loop(ctx)
	go c.r.loop()

	if err := c.init(ctx); err != nil {
		return c, err
	}

	return c, nil
}

func (cfg *ConnConfig) validate() error {
	if cfg.Keyspace != "" {
		if err := validateKeyspace(cfg.Keyspace); err != nil {
			return err
		}
	}
	if cfg.DefaultConsistency < frame.ANY || cfg.DefaultConsistency > frame.LOCALONE {
		return fmt.Errorf("unknown consistency: %v", cfg.DefaultConsistency)
	}
	return nil
}

func validateKeyspace(keyspace string) error {
	if keyspace == "" || len(keyspace) > 48 {
		return fmt.Errorf("keyspace: invalid length")
	}

	for _, c := range keyspace {
		if !(unicode.IsLetter(c) || unicode.IsDigit(c) || c == '_') {
			return fmt.Errorf("keyspace: illegal characters present")
		}
	}
	return nil
}

const cqlVersion = "3.0.0"

func (c *Conn) init(ctx context.Context) error {
	if s, err := c.Supported(ctx); err != nil {
		return fmt.Errorf("supported: %w", err)
	} else {
		c.event.Shard = s.ScyllaSupported().Shard
	}
	opts := frame.StartupOptions{"CQL_VERSION": cqlVersion}
	if c.cfg.Compression != "" {
		opts["COMPRESSION"] = string(c.cfg.Compression)
	}
	if err := c.Startup(ctx, opts); err != nil {
		return fmt.Errorf("startup: %w", err)
	}

	if c.cfg.Keyspace != "" {
		if err := c.UseKeyspace(ctx, c.cfg.Keyspace); err != nil {
			return err
		}
	}
	return nil
}

func (c *Conn) Supported(ctx context.Context) (*Supported, error) {
	res, err := c.sendRequest(ctx, &Options{}, false, false)
	if err != nil {
		return nil, err
	}
	if v, ok := res.(*Supported); ok {
		return v, nil
	}
	return nil, responseAsError(res)
}

func (c *Conn) Startup(ctx context.Context, options frame.StartupOptions) error {
	res, err := c.sendRequest(ctx, &Startup{Options: options}, false, false)
	if err != nil {
		return err
	}
	switch v := res.(type) {
	case *Ready:
		return nil
	case *Authenticate:
		return c.AuthResponse(ctx, v)
	default:
		return responseAsError(res)
	}
}

// 'AllowAllAuthenticator' and 'org.apache.cassandra.auth.AllowAllAuthenticator' do not require authentication.
var approvedAuthenticators = map[string]struct{}{
	"PasswordAuthenticator":                           {},
	"org.apache.cassandra.auth.PasswordAuthenticator": {},
	"com.scylladb.auth.TransitionalAuthenticator":     {},
}

func (c *Conn) AuthResponse(ctx context.Context, a *Authenticate) error {
	if _, ok := approvedAuthenticators[a.Name]; !ok {
		return fmt.Errorf("authenticator %q not supported", a.Name)
	}
	req := AuthResponse{
		Username: c.cfg.Username,
		Password: c.cfg.Password,
	}
	res, err := c.sendRequest(ctx, &req, false, false)
	if err != nil {
		return fmt.Errorf("can't send auth response: %w", err)
	}
	switch v := res.(type) {
	case *AuthSuccess:
		return nil
	case *AuthChallenge:
		return fmt.Errorf("authentication challenge is not yet supported: %#+v", v)
	default:
		return responseAsError(v)
	}
}

func (c *Conn) UseKeyspace(ctx context.Context, ks string) error {
	_, err := c.Query(ctx, makeStatement(fmt.Sprintf("USE %q", ks)), nil)
	return err
}

func (c *Conn) Query(ctx context.Context, s Statement, pagingState frame.Bytes) (QueryResult, error) {
	req := makeQuery(s, pagingState)
	res, err := c.sendRequest(ctx, &req, s.Compression, s.Tracing)
	if err != nil {
		return QueryResult{}, err
	}

	return MakeQueryResult(res, s.Metadata)
}

func (c *Conn) Prepare(ctx context.Context, s Statement) (Statement, error) {
	req := Prepare{Query: s.Content}
	res, err := c.sendRequest(ctx, &req, false, false)
	if err != nil {
		return Statement{}, err
	}

	if v, ok := res.(*PreparedResult); ok {
		s.ID = v.ID
		s.Values = make([]frame.Value, len(v.Metadata.Columns))
		s.PkIndexes = v.Metadata.PkIndexes
		s.PkCnt = v.Metadata.PkCnt
		s.Metadata = &v.ResultMetadata
		return s, nil
	}

	return Statement{}, responseAsError(res)
}

func (c *Conn) Execute(ctx context.Context, s Statement, pagingState frame.Bytes) (QueryResult, error) {
	req := makeExecute(s, pagingState)
	res, err := c.sendRequest(ctx, &req, s.Compression, s.Tracing)
	if err != nil {
		return QueryResult{}, err
	}

	return MakeQueryResult(res, s.Metadata)
}

func (c *Conn) RegisterEventHandler(ctx context.Context, h func(r response), e ...frame.EventType) error {
	c.r.handleEvent = h
	req := Register{EventTypes: e}
	res, err := c.sendRequest(ctx, &req, false, false)
	if err != nil {
		return err
	}
	if _, ok := res.(*Ready); ok {
		return nil
	}
	return responseAsError(res)
}

func MakeResponseHandler() ResponseHandler {
	// Each handler may encounter 2 responses, one from connWriter.loop() and one from drainHandlers().
	const responseHandlerSize = 2
	h := make(ResponseHandler, responseHandlerSize)
	return h
}

func MakeResponseHandlerWithError(err error) ResponseHandler {
	h := make(ResponseHandler, 1)
	h <- response{Err: err}
	return h
}

func (c *Conn) sendRequest(ctx context.Context, req frame.Request, compress, tracing bool) (frame.Response, error) {
	if err := c.sendController(ctx); err != nil {
		return nil, fmt.Errorf("request skipped, %w", err)
	}
	h := MakeResponseHandler()

	streamID, err := c.r.setHandler(h)
	if err != nil {
		return nil, fmt.Errorf("set handler: %w", err)
	}

	r := request{
		Request:         req,
		StreamID:        streamID,
		Compress:        compress,
		Tracing:         tracing,
		ResponseHandler: h,
		ctx:             ctx,
	}

	// requestCh might be full after terminating writeLoop so some goroutines could hang here forever.
	// this could be fixed by changing requestChanSize to be able to hold all possible streamIDs,
	// adding a grace period before terminating writeLoop or counting active streams.
	c.w.submit(r)

	select {
	case resp := <-h:
		return resp.Response, resp.Err
	case <-ctx.Done():
		return nil, fmt.Errorf("no response, %w", ctx.Err())
	}
}

func (c *Conn) asyncSendRequest(ctx context.Context, req frame.Request, compress, tracing bool, h ResponseHandler) {
control:
	if err := c.sendController(ctx); err != nil {
		h <- response{Err: fmt.Errorf("no response, %v", err)}
		return
	}

	streamID, err := c.r.setHandler(h)
	if err != nil {
		if errors.Is(err, errAllStreamsBusy) {
			goto control
		} else {
			h <- response{Err: fmt.Errorf("set handler %w", err)}
			return
		}
	}

	r := request{
		Request:         req,
		StreamID:        streamID,
		Compress:        compress,
		Tracing:         tracing,
		ResponseHandler: h,
		ctx:             ctx,
	}

	// requestCh might be full after terminating writeLoop so some goroutines could hang here forever.
	// this could be fixed by changing requestChanSize to be able to hold all possible streamIDs,
	// adding a grace period before terminating writeLoop or counting active streams.
	c.w.submit(r)
}

func (c *Conn) sendController(ctx context.Context) error {
	if size := c.Waiting(); size < targetWaiting {
		return ctx.Err()
	}

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if size := c.Waiting(); size < targetWaiting {
				return ctx.Err()
			}
			continue
		}
	}
}

func (c *Conn) AsyncQuery(ctx context.Context, s Statement, pagingState frame.Bytes, h ResponseHandler) {
	req := makeQuery(s, pagingState)
	c.asyncSendRequest(ctx, &req, s.Compression, s.Tracing, h)
}

func (c *Conn) AsyncExecute(ctx context.Context, s Statement, pagingState frame.Bytes, h ResponseHandler) {
	req := makeExecute(s, pagingState)
	c.asyncSendRequest(ctx, &req, s.Compression, s.Tracing, h)
}

func (c *Conn) Waiting() int {
	return int(c.stats.inQueue.Load() + c.stats.inFlight.Load())
}

func (c *Conn) setOnClose(f func(conn *Conn)) {
	c.onClose = f
}

func (c *Conn) Event() ConnEvent {
	return c.event
}

func (c *Conn) Shard() int {
	return int(c.event.Shard)
}

// Close closes connection and terminates reader and writer go routines.
func (c *Conn) Close() {
	c.closeOnce.Do(func() {
		if err := c.conn.Close(); err != nil {
			log.Printf("%s failed to close: %s", c, err)
		} else {
			log.Printf("%s closed", c)
		}
		c.w.requestCh <- _connCloseRequest
		if c.onClose != nil {
			c.onClose(c)
		}
	})
}

func (c *Conn) String() string {
	return fmt.Sprintf("[addr=%s shard=%d]", c.conn.RemoteAddr(), c.event.Shard)
}

func (c *Conn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

// withPort appends new port only if addr does not contain any.
func withPort(addr, newPort string) string {
	host, oldPort, err := net.SplitHostPort(addr)
	if err != nil {
		return net.JoinHostPort(trimIPv6Brackets(addr), newPort)
	}
	if oldPort != "" {
		return addr
	}
	return net.JoinHostPort(host, newPort)
}

func trimIPv6Brackets(host string) string {
	host = strings.TrimPrefix(host, "[")
	return strings.TrimSuffix(host, "]")
}
