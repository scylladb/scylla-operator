package scylla

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/scylladb/scylla-go-driver/frame"
	"github.com/scylladb/scylla-go-driver/transport"
)

// TODO: Add retry policy.
// TODO: Add Query Paging.

type EventType = string

const (
	TopologyChange EventType = "TOPOLOGY_CHANGE"
	StatusChange   EventType = "STATUS_CHANGE"
	SchemaChange   EventType = "SCHEMA_CHANGE"
)

type Consistency = uint16

const (
	ANY         Consistency = 0x0000
	ONE         Consistency = 0x0001
	TWO         Consistency = 0x0002
	THREE       Consistency = 0x0003
	QUORUM      Consistency = 0x0004
	ALL         Consistency = 0x0005
	LOCALQUORUM Consistency = 0x0006
	EACHQUORUM  Consistency = 0x0007
	SERIAL      Consistency = 0x0008
	LOCALSERIAL Consistency = 0x0009
	LOCALONE    Consistency = 0x000A
)

var (
	ErrNoHosts   = fmt.Errorf("error in session config: no hosts given")
	ErrEventType = fmt.Errorf("error in session config: invalid event\npossible events:\n" +
		"TopologyChange EventType = \"TOPOLOGY_CHANGE\"\n" +
		"StatusChange   EventType = \"STATUS_CHANGE\"\n" +
		"SchemaChange   EventType = \"SCHEMA_CHANGE\"")
	ErrConsistency = fmt.Errorf("error in session config: invalid consistency\npossible consistencies are:\n" +
		"ANY         Consistency = 0x0000\n" +
		"ONE         Consistency = 0x0001\n" +
		"TWO         Consistency = 0x0002\n" +
		"THREE       Consistency = 0x0003\n" +
		"QUORUM      Consistency = 0x0004\n" +
		"ALL         Consistency = 0x0005\n" +
		"LOCALQUORUM Consistency = 0x0006\n" +
		"EACHQUORUM  Consistency = 0x0007\n" +
		"SERIAL      Consistency = 0x0008\n" +
		"LOCALSERIAL Consistency = 0x0009\n" +
		"LOCALONE    Consistency = 0x000A")
	errNoConnection = fmt.Errorf("no working connection")
)

type Compression = frame.Compression

var (
	Snappy Compression = frame.Snappy
	Lz4    Compression = frame.Lz4
)

type SessionConfig struct {
	Hosts               []string
	Events              []EventType
	HostSelectionPolicy transport.HostSelectionPolicy
	RetryPolicy         transport.RetryPolicy

	SchemaAgreementInterval time.Duration
	// Controls the timeout for the automatic wait for schema agreement after sending a schema-altering statement.
	// If less or equal to 0, the automatic schema agreement is disabled.
	AutoAwaitSchemaAgreementTimeout time.Duration

	transport.ConnConfig
}

func DefaultSessionConfig(keyspace string, hosts ...string) SessionConfig {
	return SessionConfig{
		Hosts:                           hosts,
		HostSelectionPolicy:             transport.NewTokenAwarePolicy(""),
		RetryPolicy:                     transport.NewDefaultRetryPolicy(),
		SchemaAgreementInterval:         200 * time.Millisecond,
		AutoAwaitSchemaAgreementTimeout: 60 * time.Second,
		ConnConfig:                      transport.DefaultConnConfig(keyspace),
	}
}

func (cfg SessionConfig) Clone() SessionConfig {
	v := cfg

	v.Hosts = make([]string, len(cfg.Hosts))
	copy(v.Hosts, cfg.Hosts)

	v.Events = make([]EventType, len(cfg.Events))
	copy(v.Events, cfg.Events)

	v.TLSConfig = v.TLSConfig.Clone()

	return v
}

func (cfg *SessionConfig) Validate() error {
	if len(cfg.Hosts) == 0 {
		return ErrNoHosts
	}
	for _, e := range cfg.Events {
		if e != TopologyChange && e != StatusChange && e != SchemaChange {
			return ErrEventType
		}
	}
	if cfg.DefaultConsistency > LOCALONE {
		return ErrConsistency
	}
	return nil
}

type Session struct {
	cfg     SessionConfig
	cluster *transport.Cluster
}

func NewSession(ctx context.Context, cfg SessionConfig) (*Session, error) {
	cfg = cfg.Clone()

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	cluster, err := transport.NewCluster(ctx, cfg.ConnConfig, cfg.HostSelectionPolicy, cfg.Events, cfg.Hosts...)
	if err != nil {
		return nil, err
	}

	s := &Session{
		cfg:     cfg,
		cluster: cluster,
	}

	return s, nil
}

func (s *Session) Query(content string) Query {
	return Query{session: s,
		stmt: transport.Statement{Content: content, Consistency: s.cfg.DefaultConsistency},
		exec: func(ctx context.Context, conn *transport.Conn, stmt transport.Statement, pagingState frame.Bytes) (transport.QueryResult, error) {
			return conn.Query(ctx, stmt, pagingState)
		},
		asyncExec: func(ctx context.Context, conn *transport.Conn, stmt transport.Statement, pagingState frame.Bytes, handler transport.ResponseHandler) {
			conn.AsyncQuery(ctx, stmt, pagingState, handler)
		},
	}
}

func (s *Session) Prepare(ctx context.Context, content string) (Query, error) {
	stmt := transport.Statement{Content: content, Consistency: frame.ALL}

	// Prepare on all nodes concurrently.
	nodes := s.cluster.Topology().Nodes
	resStmt := make([]transport.Statement, len(nodes))
	resErr := make([]error, len(nodes))
	var wg sync.WaitGroup
	for i := range nodes {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			resStmt[idx], resErr[idx] = nodes[idx].Prepare(ctx, stmt)
		}(i)
	}
	wg.Wait()

	// Find first result that succeeded.
	for i := range nodes {
		if resErr[i] == nil {
			return Query{
				session: s,
				stmt:    resStmt[i],
				exec: func(ctx context.Context, conn *transport.Conn, stmt transport.Statement, pagingState frame.Bytes) (transport.QueryResult, error) {
					return conn.Execute(ctx, stmt, pagingState)
				},
				asyncExec: func(ctx context.Context, conn *transport.Conn, stmt transport.Statement, pagingState frame.Bytes, handler transport.ResponseHandler) {
					conn.AsyncExecute(ctx, stmt, pagingState, handler)
				},
			}, nil
		}
	}

	return Query{}, fmt.Errorf("prepare failed on all nodes, details: %v", resErr)
}

func (s *Session) AwaitSchemaAgreement(ctx context.Context, timeout time.Duration) error {
	ticker := time.NewTicker(s.cfg.SchemaAgreementInterval)
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	defer ticker.Stop()
	for {
		agreement, err := s.CheckSchemaAgreement(ctx)
		if err != nil {
			return err
		}
		if agreement {
			return nil
		}

		select {
		case <-ticker.C:
			continue
		case <-timer.C:
			return fmt.Errorf("schema not in agreement after %v", timeout)
		case <-ctx.Done():
			return fmt.Errorf("schema not in agreement before: %w", ctx.Err())
		}
	}
}

func (s *Session) handleAutoAwaitSchemaAgreement(ctx context.Context, stmt string, result *transport.QueryResult) error {
	if s.cfg.AutoAwaitSchemaAgreementTimeout <= 0 {
		return nil
	}

	if result.SchemaChange != nil {
		if err := s.AwaitSchemaAgreement(ctx, s.cfg.AutoAwaitSchemaAgreementTimeout); err != nil {
			return fmt.Errorf("failed to reach schema agreement after a schema-altering statement: %v, %w", stmt, err)
		}
	}

	return nil
}

func (s *Session) CheckSchemaAgreement(ctx context.Context) (bool, error) {
	// Get schema version from all nodes concurrently.
	nodes := s.cluster.Topology().Nodes
	versions := make([]frame.UUID, len(nodes))
	errors := make([]error, len(nodes))
	var wg sync.WaitGroup
	for i := range nodes {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			versions[idx], errors[idx] = nodes[idx].FetchSchemaVersion(ctx)
		}(i)
	}
	wg.Wait()

	for _, err := range errors {
		if err != nil {
			return false, fmt.Errorf("schema version check failed: %v", errors)
		}
	}

	for _, version := range versions {
		if version != versions[0] {
			return false, nil
		}
	}

	return true, nil
}

func (s *Session) NewTokenAwarePolicy() transport.HostSelectionPolicy {
	return transport.NewTokenAwarePolicy("")
}

func (s *Session) NewTokenAwareDCAwarePolicy(localDC string) transport.HostSelectionPolicy {
	return transport.NewTokenAwarePolicy(localDC)
}

func (s *Session) Close() {
	log.Println("session: close")
	s.cluster.Close()
}
