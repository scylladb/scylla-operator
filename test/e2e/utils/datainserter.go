// Copyright (c) 2021 ScyllaDB

package utils

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/gocql/gocql"
	"github.com/scylladb/gocqlx/v3"
	"github.com/scylladb/gocqlx/v3/table"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	apimachineryutilrand "k8s.io/apimachinery/pkg/util/rand"
)

const (
	nRows = 10

	// cqlSessionCloseTimeout is the maximum time to wait for gocql Session.Close() to complete. If Close() does not finish
	// within this window we abandon it and log a warning to avoid hanging the entire test suite.
	cqlSessionCloseTimeout = 30 * time.Second
)

type DataInserter struct {
	session     *gocqlx.Session
	keyspace    string
	table       *table.Table
	data        []*TestData
	replication Replication
}

// Replication describes a keyspace's replication strategy as a CQL 'replication' map value, e.g.
// "{'class': 'NetworkTopologyStrategy', 'us-east-1': 4}". Build one with NetworkTopologyStrategyReplication - the
// only class this repo's keyspaces use - which only accepts the options that class actually supports, rather than
// an arbitrary class/options pair that could describe an invalid combination.
// See https://docs.scylladb.com/manual/stable/cql/ddl.html.
type Replication struct {
	cql string
}

// NetworkTopologyStrategyReplication describes a NetworkTopologyStrategy keyspace, replicating to the given number
// of nodes in each named datacenter. The special key "replication_factor" sets the default for every datacenter not
// given its own entry. A nil or empty map is ScyllaDB's own default of one replica per rack of every datacenter, which is
// RF-rack-valid by construction.
// See https://docs.scylladb.com/manual/stable/cql/ddl.html#networktopologystrategy.
func NetworkTopologyStrategyReplication(datacenterReplicationFactors map[string]int) Replication {
	parts := []string{`'class': 'NetworkTopologyStrategy'`}
	for _, dc := range slices.Sorted(maps.Keys(datacenterReplicationFactors)) {
		parts = append(parts, fmt.Sprintf(`'%s': %d`, dc, datacenterReplicationFactors[dc]))
	}

	return Replication{cql: "{" + strings.Join(parts, ", ") + "}"}
}

type TestData struct {
	Id   int    `db:"id"`
	Data string `db:"data"`
}

type DataInserterOption func(*DataInserter)

func WithSession(session *gocqlx.Session) func(*DataInserter) {
	return func(di *DataInserter) {
		di.session = session
	}
}

// WithReplication overrides the keyspace's default replication strategy.
func WithReplication(replication Replication) DataInserterOption {
	return func(di *DataInserter) {
		di.replication = replication
	}
}

func NewDataInserter(hosts []string, options ...DataInserterOption) (*DataInserter, error) {
	keyspace := apimachineryutilrand.String(8)
	table := table.New(table.Metadata{
		Name:    fmt.Sprintf(`"%s"."test"`, keyspace),
		Columns: []string{"id", "data"},
		PartKey: []string{"id"},
	})
	data := make([]*TestData, 0, nRows)
	for i := range nRows {
		data = append(data, &TestData{Id: i, Data: apimachineryutilrand.String(32)})
	}

	di := &DataInserter{
		keyspace:    keyspace,
		table:       table,
		data:        data,
		replication: NetworkTopologyStrategyReplication(nil),
	}

	for _, option := range options {
		option(di)
	}

	if di.session == nil {
		err := di.SetClientEndpoints(hosts)
		if err != nil {
			return nil, fmt.Errorf("can't set client endpoints: %w", err)
		}
	}

	return di, nil
}

func (di *DataInserter) Close() {
	done := make(chan struct{})
	go func() {
		defer close(done)
		if di.session != nil {
			di.session.Close()
		}
	}()

	select {
	case <-done:
	case <-time.After(cqlSessionCloseTimeout):
		framework.Infof("WARNING: gocql Session.Close() did not return within %v; abandoning to avoid suite hang", cqlSessionCloseTimeout)
	}
}

// SetClientEndpoints creates a new session and closes a previous session if it existed.
// In case an error was returned, DataInserter can no Longer be used.
func (di *DataInserter) SetClientEndpoints(hosts []string) error {
	di.Close()

	if len(hosts) == 0 {
		return fmt.Errorf("at least one enpoint is required")
	}

	framework.Infof("Creating CQL session (hosts=%q)", strings.Join(hosts, ", "))
	err := di.createSession(hosts)
	if err != nil {
		return fmt.Errorf("can't create session: %w", err)
	}

	return nil
}

func (di *DataInserter) Insert(ctx context.Context) error {
	framework.Infof("Creating keyspace %q with replication %s", di.keyspace, di.replication.cql)
	err := di.session.ExecStmt(fmt.Sprintf(`CREATE KEYSPACE %q WITH replication = %s`, di.keyspace, di.replication.cql))
	if err != nil {
		return fmt.Errorf("can't create keyspace: %w", err)
	}

	framework.Infof("Creating table %s", di.table.Name())
	err = di.session.ExecStmt(fmt.Sprintf(
		`CREATE TABLE %s (id int primary key, data text)`,
		di.table.Name(),
	))
	if err != nil {
		return fmt.Errorf("can't create table: %w", err)
	}

	framework.Infof("Awaiting schema agreement")
	err = di.session.AwaitSchemaAgreement(ctx)
	if err != nil {
		return fmt.Errorf("can't await schema agreement: %w", err)
	}

	framework.Infof("Inserting data into table %s", di.table.Name())
	for _, t := range di.data {
		q := di.session.Query(di.table.Insert()).BindStruct(t)
		err = q.ExecRelease()
		if err != nil {
			return fmt.Errorf("can't insert data: %w", err)
		}
	}

	return nil
}

func (di *DataInserter) AwaitSchemaAgreement(ctx context.Context) error {
	framework.Infof("Awaiting schema agreement")
	err := di.session.AwaitSchemaAgreement(ctx)
	if err != nil {
		return fmt.Errorf("can't await schema agreement: %w", err)
	}

	framework.Infof("Schema agreement reached")
	return nil
}

func (di *DataInserter) Read() ([]*TestData, error) {
	framework.Infof("Reading data from table %s", di.table.Name())

	q := di.session.Query(di.table.SelectAll()).BindStruct(&TestData{})
	var res []*TestData
	err := q.SelectRelease(&res)
	if err != nil {
		return nil, fmt.Errorf("can't select data: %w", err)
	}

	slices.SortFunc(res, func(a, b *TestData) int {
		if a.Id < b.Id {
			return -1
		}
		if a.Id > b.Id {
			return 1
		}
		return 0
	})

	return res, nil
}

func (di *DataInserter) GetExpected() []*TestData {
	return di.data
}

func (di *DataInserter) createSession(hosts []string) error {
	clusterConfig := gocql.NewCluster(hosts...)
	// Set a small reconnect interval to avoid flakes, if not reconnected in time.
	clusterConfig.ReconnectInterval = 500 * time.Millisecond

	session, err := gocqlx.WrapSession(clusterConfig.CreateSession())
	if err != nil {
		return fmt.Errorf("can't create gocqlx session: %w", err)
	}

	session.SetConsistency(gocql.All)

	di.session = &session

	return nil
}
