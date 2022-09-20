package scylla

import (
	"context"
	"fmt"

	"github.com/scylladb/scylla-go-driver/frame"
	"github.com/scylladb/scylla-go-driver/transport"
)

type Query struct {
	session   *Session
	stmt      transport.Statement
	buf       frame.Buffer
	exec      func(context.Context, *transport.Conn, transport.Statement, frame.Bytes) (transport.QueryResult, error)
	asyncExec func(context.Context, *transport.Conn, transport.Statement, frame.Bytes, transport.ResponseHandler)
	res       []transport.ResponseHandler
}

func (q *Query) Exec(ctx context.Context) (Result, error) {
	info, err := q.info()
	if err != nil {
		return Result{}, err
	}

	// Most queries don't need retries, rd will be allocated on first failure.
	var rd transport.RetryDecider
	var lastErr error
	n := q.session.cfg.HostSelectionPolicy.Node(info, 0)
	i := 0
	for n != nil {
	sameNodeRetries:
		for {
			conn := n.Conn(info)
			res, err := q.exec(ctx, conn, q.stmt, nil)
			if err != nil {
				ri := transport.RetryInfo{
					Error:       err,
					Idempotent:  q.stmt.Idempotent,
					Consistency: q.stmt.Consistency,
				}

				if rd == nil {
					rd = q.session.cfg.RetryPolicy.NewRetryDecider()
				}
				switch rd.Decide(ri) {
				case transport.RetrySameNode:
					continue sameNodeRetries
				case transport.RetryNextNode:
					lastErr = err
					break sameNodeRetries
				case transport.DontRetry:
					return Result{}, err
				}
			}

			return Result(res), q.session.handleAutoAwaitSchemaAgreement(ctx, q.stmt.Content, &res)
		}

		i++
		n = q.session.cfg.HostSelectionPolicy.Node(info, i)
	}
	return Result{}, lastErr
}

func (q *Query) pickConn(qi transport.QueryInfo) (*transport.Conn, error) {
	n := q.session.cfg.HostSelectionPolicy.Node(qi, 0)

	conn := n.Conn(qi)
	if conn == nil {
		return nil, errNoConnection
	}

	return conn, nil
}

func (q *Query) AsyncExec(ctx context.Context) {
	stmt := q.stmt.Clone()
	info, err := q.info()
	if err != nil {
		q.res = append(q.res, transport.MakeResponseHandlerWithError(err))
	}

	conn, err := q.pickConn(info)
	if err != nil {
		q.res = append(q.res, transport.MakeResponseHandlerWithError(err))
		return
	}

	h := transport.MakeResponseHandler()
	q.res = append(q.res, h)
	q.asyncExec(ctx, conn, stmt, nil, h)
}

var ErrNoQueryResults = fmt.Errorf("no query results to be fetched")

// Fetch returns results in the same order they were queried.
func (q *Query) Fetch() (Result, error) {
	if len(q.res) == 0 {
		return Result{}, ErrNoQueryResults
	}

	h := q.res[0]
	q.res = q.res[1:]

	resp := <-h
	if resp.Err != nil {
		return Result{}, resp.Err
	}

	res, err := transport.MakeQueryResult(resp.Response, q.stmt.Metadata)
	return Result(res), err
}

// https://github.com/scylladb/scylla/blob/40adf38915b6d8f5314c621a94d694d172360833/compound_compat.hh#L33-L47
func (q *Query) token() (transport.Token, bool) {
	if q.stmt.PkCnt == 0 {
		return 0, false
	}

	q.buf.Reset()
	if q.stmt.PkCnt == 1 {
		return transport.MurmurToken(q.stmt.Values[q.stmt.PkIndexes[0]].Bytes), true
	}
	for _, idx := range q.stmt.PkIndexes {
		size := q.stmt.Values[idx].N
		q.buf.WriteShort(frame.Short(size))
		q.buf.Write(q.stmt.Values[idx].Bytes)
		q.buf.WriteByte(0)
	}

	return transport.MurmurToken(q.buf.Bytes()), true
}

func (q *Query) info() (transport.QueryInfo, error) {
	token, tokenAware := q.token()
	if tokenAware {
		// TODO: Will the driver support using different keyspaces than default?
		info, err := q.session.cluster.NewTokenAwareQueryInfo(token, "")
		return info, err
	}

	return q.session.cluster.NewQueryInfo(), nil
}

func (q *Query) BindInt64(pos int, v int64) *Query {
	p := &q.stmt.Values[pos]
	if p.N == 0 {
		p.N = 8
		p.Bytes = make([]byte, 8)
	}

	p.Bytes[0] = byte(v >> 56)
	p.Bytes[1] = byte(v >> 48)
	p.Bytes[2] = byte(v >> 40)
	p.Bytes[3] = byte(v >> 32)
	p.Bytes[4] = byte(v >> 24)
	p.Bytes[5] = byte(v >> 16)
	p.Bytes[6] = byte(v >> 8)
	p.Bytes[7] = byte(v)

	return q
}

func (q *Query) SetPageSize(v int32) {
	q.stmt.PageSize = v
}

func (q *Query) PageSize() int32 {
	return q.stmt.PageSize
}

func (q *Query) SetCompression(v bool) {
	q.stmt.Compression = v
}

func (q *Query) Compression() bool {
	return q.stmt.Compression
}

func (q *Query) SetIdempotent(v bool) {
	q.stmt.Idempotent = v
}

func (q *Query) Idempotent() bool {
	return q.stmt.Idempotent
}

type Result transport.QueryResult

func (q *Query) Iter(ctx context.Context) Iter {
	it := Iter{
		requestCh: make(chan struct{}, 1),
		nextCh:    make(chan transport.QueryResult),
		errCh:     make(chan error, 1),
	}

	info, err := q.info()
	if err != nil {
		it.errCh <- err
		return it
	}

	worker := iterWorker{
		stmt: q.stmt.Clone(),

		rd:        q.session.cfg.RetryPolicy.NewRetryDecider(),
		queryInfo: info,
		pickNode:  q.session.cfg.HostSelectionPolicy.Node,
		queryExec: q.exec,

		requestCh: it.requestCh,
		nextCh:    it.nextCh,
		errCh:     it.errCh,
	}

	it.requestCh <- struct{}{}
	go worker.loop(ctx)
	return it
}

type Iter struct {
	result transport.QueryResult
	pos    int
	rowCnt int

	requestCh chan struct{}
	nextCh    chan transport.QueryResult
	errCh     chan error
	closed    bool
}

var (
	ErrClosedIter = fmt.Errorf("iter is closed")
	ErrNoMoreRows = fmt.Errorf("no more rows left")
)

func (it *Iter) Next() (frame.Row, error) {
	if it.closed {
		return nil, ErrClosedIter
	}

	if it.pos >= it.rowCnt {
		select {
		case r := <-it.nextCh:
			it.result = r
		case err := <-it.errCh:
			it.Close()
			return nil, err
		}

		it.pos = 0
		it.rowCnt = len(it.result.Rows)
		it.requestCh <- struct{}{}
	}

	// We probably got a zero-sized last page, retry to be sure
	if it.rowCnt == 0 {
		return it.Next()
	}

	res := it.result.Rows[it.pos]
	it.pos++
	return res, nil
}

func (it *Iter) Close() {
	if it.closed {
		return
	}
	it.closed = true
	close(it.requestCh)
}

type iterWorker struct {
	stmt        transport.Statement
	pagingState []byte
	queryExec   func(context.Context, *transport.Conn, transport.Statement, frame.Bytes) (transport.QueryResult, error)

	queryInfo transport.QueryInfo
	pickNode  func(transport.QueryInfo, int) *transport.Node
	nodeIdx   int
	conn      *transport.Conn

	rd transport.RetryDecider

	requestCh chan struct{}
	nextCh    chan transport.QueryResult
	errCh     chan error
}

func (w *iterWorker) loop(ctx context.Context) {
	n := w.pickNode(w.queryInfo, 0)
	if n == nil {
		w.errCh <- fmt.Errorf("can't pick a node to execute request")
		return
	}
	w.conn = n.Conn(w.queryInfo)

	for {
		_, ok := <-w.requestCh
		if !ok {
			return
		}

		res, err := w.exec(ctx)
		if err != nil {
			w.errCh <- err
			return
		}

		w.pagingState = res.PagingState
		w.nextCh <- res
		if !res.HasMorePages {
			w.errCh <- ErrNoMoreRows
			return
		}
	}
}

func (w *iterWorker) exec(ctx context.Context) (transport.QueryResult, error) {
	w.rd.Reset()
	var lastErr error
	for {
	sameNodeRetries:
		for {
			res, err := w.queryExec(ctx, w.conn, w.stmt, w.pagingState)
			if err != nil {
				ri := transport.RetryInfo{
					Error:       err,
					Idempotent:  w.stmt.Idempotent,
					Consistency: w.stmt.Consistency,
				}

				switch w.rd.Decide(ri) {
				case transport.RetrySameNode:
					continue sameNodeRetries
				case transport.RetryNextNode:
					lastErr = err
					break sameNodeRetries
				case transport.DontRetry:
					return transport.QueryResult{}, err
				}
			}

			return res, nil
		}

		w.nodeIdx++
		n := w.pickNode(w.queryInfo, w.nodeIdx)
		if n == nil {
			return transport.QueryResult{}, lastErr
		}

		w.conn = n.Conn(w.queryInfo)
	}
}
