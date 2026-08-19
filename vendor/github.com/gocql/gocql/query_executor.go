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
 * Copyright (c) 2016, The Gocql authors,
 * provided under the BSD-3-Clause License.
 * See the NOTICE file distributed with this work for additional information.
 */

package gocql

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type ExecutableQuery interface {
	borrowForExecution()    // Used to ensure that the query stays alive for lifetime of a particular execution goroutine.
	releaseAfterExecution() // Used when a goroutine finishes its execution attempts, either with ok result or an error.
	execute(ctx context.Context, conn *Conn, metrics *queryMetrics) *Iter
	finishAttempt(token attemptToken, keyspace string, end time.Time, iter *Iter, host *HostInfo)
	retryPolicy() RetryPolicy
	speculativeExecutionPolicy() SpeculativeExecutionPolicy
	GetRoutingKey() ([]byte, error)
	Keyspace() string
	Table() string
	IsIdempotent() bool
	IsLWT() bool
	GetCustomPartitioner() Partitioner
	GetHostID() string

	withContext(context.Context) ExecutableQuery

	RetryableQuery

	GetSession() *Session
}

type queryExecutor struct {
	pool   *policyConnPool
	policy HostSelectionPolicy
}

type queryExecutionResult struct {
	iter        *Iter
	consistency Consistency
}

// queryExecutionResults returns an unbuffered result handoff. Once the caller
// accepts a winner it stops receiving and cancels the execution context. A
// buffer could accept a late loser after that point and orphan its Iter.
func queryExecutionResults() chan queryExecutionResult {
	return make(chan queryExecutionResult)
}

func (q *queryExecutor) attemptQuery(ctx context.Context, qry ExecutableQuery, metrics *queryMetrics,
	executionAttempts *atomic.Int64, localAttempts *int64, conn *Conn) *Iter {
	token := metrics.beginAttempt()
	iter := qry.execute(ctx, conn, metrics)
	end := time.Now()

	// Retry accounting is page-scoped and must become visible before an
	// observer callback can block this runner while another speculative branch
	// asks the retry policy for the completed-attempt count.
	if executionAttempts != nil {
		executionAttempts.Add(1)
	} else {
		*localAttempts++
	}
	// Report the query's effective keyspace to observers rather than the
	// pool/session keyspace. Query.SetKeyspace()/Batch.SetKeyspace() (proto v5
	// keyspace override) make these diverge, and Keyspace() is the single
	// source of truth for a statement's keyspace (routing/prepared metadata,
	// then the SetKeyspace override, then the session default).
	qry.finishAttempt(token, qry.Keyspace(), end, iter, conn.host)

	return iter
}

func (q *queryExecutor) speculate(ctx context.Context, qry, releaseQry ExecutableQuery, sp SpeculativeExecutionPolicy,
	metrics *queryMetrics, executionAttempts *atomic.Int64, hostIter NextHost,
	results chan queryExecutionResult) queryExecutionResult {
	ticker := time.NewTicker(sp.Delay())
	defer ticker.Stop()

	for i := 0; i < sp.Attempts(); i++ {
		select {
		case <-ticker.C:
			releaseQry.borrowForExecution() // prevent Query.Release while this runner uses the captured execution view.
			metrics.retain()
			go q.run(ctx, qry, releaseQry, metrics, executionAttempts, hostIter, results)
		case <-ctx.Done():
			return queryExecutionResult{
				iter:        &Iter{err: ctx.Err()},
				consistency: qry.GetConsistency(),
			}
		case result := <-results:
			return result
		}
	}

	return queryExecutionResult{}
}

func (q *queryExecutor) executeQuery(qry ExecutableQuery, metrics *queryMetrics) (*Iter, error) {
	var hostIter NextHost

	// check if the hostID is specified for the query,
	// if true  - the query execute at the specified host.
	// if false - the query execute at the host picked by HostSelectionPolicy
	if hostID := qry.GetHostID(); hostID != "" {
		pool, ok := q.pool.getPoolByHostID(hostID)
		if !ok {
			// if the specified host ID have no connection pool we return error
			return nil, fmt.Errorf("query is targeting unknown host id %s: %w", hostID, ErrNoPool)
		} else if pool.Size() == 0 {
			// if the pool have no connection we return error
			return nil, fmt.Errorf("query is targeting host id %s that driver is not connected to: %w", hostID, ErrNoConnectionsInPool)
		}
		hostIter = newSingleHost(pool.host, 5, 200*time.Millisecond).selectHost
	} else {
		hostIter = q.policy.Pick(qry)
	}

	// check if the query is not marked as idempotent, if
	// it is, we force the policy to NonSpeculative
	sp := qry.speculativeExecutionPolicy()
	if qry.GetHostID() != "" || !qry.IsIdempotent() || sp.Attempts() == 0 {
		iter, consistency := q.do(qry.Context(), qry, metrics, nil, hostIter)
		if consistency != qry.GetConsistency() {
			qry.SetConsistency(consistency)
		}
		return iter, nil
	}
	executionQry := queryForSpeculativeExecution(qry, metrics)
	executionAttempts := new(atomic.Int64)
	// Runner borrows can be released immediately after publishing a result.
	// Keep the original alive through winner-state propagation below.
	qry.borrowForExecution()
	defer qry.releaseAfterExecution()

	// When speculative execution is enabled, we could be accessing the host iterator from multiple goroutines below.
	// To ensure we don't call it concurrently, we wrap the returned NextHost function here to synchronize access to it.
	var mu sync.Mutex
	origHostIter := hostIter
	hostIter = func() SelectedHost {
		mu.Lock()
		defer mu.Unlock()
		return origHostIter()
	}

	ctx, cancel := context.WithCancel(qry.Context())
	defer cancel()

	results := queryExecutionResults()

	// Launch the main execution
	qry.borrowForExecution() // prevent Query.Release while this runner uses the captured execution view.
	metrics.retain()
	go q.run(ctx, executionQry, qry, metrics, executionAttempts, hostIter, results)

	// The speculative executions are launched _in addition_ to the main
	// execution, on a timer. So Speculation{2} would make 3 executions running
	// in total.
	if result := q.speculate(ctx, executionQry, qry, sp, metrics, executionAttempts, hostIter, results); result.iter != nil {
		if result.consistency != qry.GetConsistency() {
			qry.SetConsistency(result.consistency)
		}
		return result.iter, nil
	}

	select {
	case result := <-results:
		if result.consistency != qry.GetConsistency() {
			qry.SetConsistency(result.consistency)
		}
		return result.iter, nil
	case <-ctx.Done():
		return &Iter{err: ctx.Err()}, nil
	}
}

func queryForSpeculativeExecution(qry ExecutableQuery, metrics *queryMetrics) ExecutableQuery {
	switch qry := qry.(type) {
	case *Query:
		return cloneQuery(qry, metrics)
	case *Batch:
		executionBatch := *qry
		executionBatch.metrics = metrics
		executionBatch.metricsOwner = queryMetricsOwner{}
		executionBatch.executionAttempts = nil
		executionBatch.Entries = append([]BatchEntry(nil), qry.Entries...)
		return &executionBatch
	default:
		return qry.withContext(qry.Context())
	}
}

type fallbackExecutionRetryableQuery struct {
	ExecutableQuery
	attempts *atomic.Int64
}

func (q fallbackExecutionRetryableQuery) Attempts() int {
	return int(q.attempts.Load())
}

func queryForExecution(qry ExecutableQuery, metrics *queryMetrics, attempts *atomic.Int64) ExecutableQuery {
	switch qry := qry.(type) {
	case *Query:
		executionQry := cloneQuery(qry, metrics)
		executionQry.executionAttempts = attempts
		return executionQry
	case *Batch:
		executionBatch := *qry
		executionBatch.metrics = metrics
		executionBatch.executionAttempts = attempts
		return &executionBatch
	default:
		return &fallbackExecutionRetryableQuery{
			ExecutableQuery: qry,
			attempts:        attempts,
		}
	}
}

func queryForRetryExecution(qry ExecutableQuery, metrics *queryMetrics) ExecutableQuery {
	switch qry := qry.(type) {
	case *Query:
		return cloneQuery(qry, metrics)
	case *Batch:
		executionBatch := *qry
		executionBatch.metrics = metrics
		executionBatch.metricsOwner = queryMetricsOwner{}
		executionBatch.executionAttempts = nil
		return &executionBatch
	default:
		return qry.withContext(qry.Context())
	}
}

func (q *queryExecutor) do(ctx context.Context, qry ExecutableQuery, metrics *queryMetrics,
	executionAttempts *atomic.Int64, hostIter NextHost) (*Iter, Consistency) {
	rt := qry.retryPolicy()
	if rt == nil {
		rt = defaultRetryPolicy
	}

	lwtRT, isRTSupportsLWT := rt.(LWTRetryPolicy)

	var getShouldRetry func(qry RetryableQuery) bool
	var getRetryType func(error) RetryType

	if isRTSupportsLWT && qry.IsLWT() {
		getShouldRetry = lwtRT.AttemptLWT
		getRetryType = lwtRT.GetRetryTypeLWT
	} else {
		getShouldRetry = rt.Attempt
		getRetryType = rt.GetRetryType
	}
	var potentiallyExecuted bool
	var retryableQry RetryableQuery
	var localAttempts int64

	execute := func(qry ExecutableQuery, selectedHost SelectedHost) (iter *Iter, retry RetryType) {
		host := selectedHost.Info()
		if host == nil || !host.IsUp() {
			return &Iter{
				err: &QueryError{
					err:                 ErrHostDown,
					potentiallyExecuted: potentiallyExecuted,
				},
			}, RetryNextHost
		}
		pool, ok := q.pool.getPool(host)
		if !ok {
			return &Iter{
				err: &QueryError{
					err:                 ErrNoPool,
					potentiallyExecuted: potentiallyExecuted,
				},
			}, RetryNextHost
		}
		conn := pool.Pick(selectedHost.Token(), qry)
		if conn == nil {
			return &Iter{
				err: &QueryError{
					err:                 ErrNoConnectionsInPool,
					potentiallyExecuted: potentiallyExecuted,
				},
			}, RetryNextHost
		}
		iter = q.attemptQuery(ctx, qry, metrics, executionAttempts, &localAttempts, conn)
		iter.host = selectedHost.Info()
		// Update host
		if iter.err == nil {
			return iter, RetryType(255)
		}

		switch {
		case errors.Is(iter.err, context.Canceled),
			errors.Is(iter.err, context.DeadlineExceeded):
			selectedHost.Mark(nil)
			potentiallyExecuted = true
			retry = Rethrow
		default:
			selectedHost.Mark(iter.err)
			retry = RetryType(255) // Don't enforce retry and get it from retry policy
		}

		var qErr *QueryError
		if errors.As(iter.err, &qErr) {
			potentiallyExecuted = potentiallyExecuted || qErr.PotentiallyExecuted()
			qErr.potentiallyExecuted = potentiallyExecuted
			qErr.isIdempotent = qry.IsIdempotent()
			iter.err = qErr
		} else {
			iter.err = &QueryError{
				err:                 iter.err,
				potentiallyExecuted: potentiallyExecuted,
				isIdempotent:        qry.IsIdempotent(),
			}
		}
		return iter, retry
	}

	var lastErr error
	selectedHost := hostIter()
	for selectedHost != nil {
		iter, retryType := execute(qry, selectedHost)
		if iter.err == nil {
			return iter, qry.GetConsistency()
		}
		lastErr = iter.err

		// Exit if retry policy decides to not retry anymore
		if retryType == RetryType(255) {
			if retryableQry == nil {
				// Retry policies may mutate consistency. Lazily copy the query
				// after the first failed attempt so successful executions keep
				// their allocation profile, concurrent speculative executions
				// do not race, and custom policies still see *Query or *Batch.
				if executionAttempts == nil {
					executionAttempts = new(atomic.Int64)
					executionAttempts.Store(localAttempts)
				}
				qry = queryForRetryExecution(qry, metrics)
				retryableQry = queryForExecution(qry, metrics, executionAttempts)
			}
			retryableQry.SetConsistency(qry.GetConsistency())
			shouldRetry := getShouldRetry(retryableQry)
			qry.SetConsistency(retryableQry.GetConsistency())
			if !shouldRetry {
				return iter, qry.GetConsistency()
			}
			retryType = getRetryType(iter.err)
		}

		// If query is unsuccessful, check the error with RetryPolicy to retry
		switch retryType {
		case Retry:
			iter.finalize(true)
			// retry on the same host
			continue
		case Rethrow, Ignore:
			return iter, qry.GetConsistency()
		case RetryNextHost:
			iter.finalize(true)
			// retry on the next host
			selectedHost = hostIter()
			continue
		default:
			// Undefined? Return nil and error, this will panic in the requester
			return &Iter{err: ErrUnknownRetryType}, qry.GetConsistency()
		}
	}
	if lastErr != nil {
		return &Iter{err: lastErr}, qry.GetConsistency()
	}
	return &Iter{err: ErrNoConnections}, qry.GetConsistency()
}

func (q *queryExecutor) run(ctx context.Context, qry, releaseQry ExecutableQuery, metrics *queryMetrics,
	executionAttempts *atomic.Int64, hostIter NextHost, results chan<- queryExecutionResult) {
	defer metrics.release()
	iter, consistency := q.do(ctx, qry, metrics, executionAttempts, hostIter)
	select {
	case results <- queryExecutionResult{iter: iter, consistency: consistency}:
	case <-ctx.Done():
		iter.discard()
	}
	releaseQry.releaseAfterExecution()
}
