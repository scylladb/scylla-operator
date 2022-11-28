// Copyright (c) 2021 ScyllaDB

package utils

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gocql/gocql"
	"github.com/scylladb/gocqlx/v2"
	"github.com/scylladb/gocqlx/v2/table"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
)

const nRows = 10

type DataInserter struct {
	session               *gocqlx.Session
	keyspace              string
	table                 *table.Table
	data                  []*TestData
	replicationFactor     int
	readConsistency       gocql.Consistency
	clusterConfigModifier func(*gocql.ClusterConfig) *gocql.ClusterConfig
	verbose               bool
}

type TestData struct {
	Id   int    `db:"id"`
	Data string `db:"data"`
}

type DataInserterOption func(*DataInserter)

func WithClusterConfigModifier(clusterConfigModifier func(*gocql.ClusterConfig) *gocql.ClusterConfig) func(*DataInserter) {
	return func(di *DataInserter) {
		di.clusterConfigModifier = clusterConfigModifier
	}
}

func WithReadConsistency(readConsistency gocql.Consistency) func(inserter *DataInserter) {
	return func(di *DataInserter) {
		di.readConsistency = readConsistency
	}
}

func NewDataInserter(hosts []string, verbose bool, options ...DataInserterOption) (*DataInserter, error) {
	keyspace := utilrand.String(8)
	table := table.New(table.Metadata{
		Name:    fmt.Sprintf(`"%s"."test"`, keyspace),
		Columns: []string{"id", "data"},
		PartKey: []string{"id"},
	})
	data := make([]*TestData, 0, nRows)
	for i := 0; i < nRows; i++ {
		data = append(data, &TestData{Id: i, Data: utilrand.String(32)})
	}

	di := &DataInserter{
		keyspace:          keyspace,
		table:             table,
		data:              data,
		replicationFactor: len(hosts),
		readConsistency:   gocql.All,
		verbose:           verbose,
	}

	for _, option := range options {
		option(di)
	}

	err := di.SetClientEndpoints(hosts)
	if err != nil {
		return nil, fmt.Errorf("can't set client endpoints: %w", err)
	}

	return di, nil
}

func (di *DataInserter) Close() {
	if di.session != nil {
		di.session.Close()
	}
}

// SetClientEndpoints creates a new session and closes a previous session if it existed.
// In case an error was returned, DataInserter can no Longer be used.
func (di *DataInserter) SetClientEndpoints(hosts []string) error {
	di.Close()

	if len(hosts) == 0 {
		return fmt.Errorf("at least one enpoint is required")
	}

	if di.verbose {
		framework.Infof("Creating CQL session (hosts=%q)", strings.Join(hosts, ", "))
	}
	err := di.createSession(hosts)
	if err != nil {
		return fmt.Errorf("can't create session: %w", err)
	}

	return nil
}

func (di *DataInserter) Insert() error {
	if di.verbose {
		framework.Infof("Creating keyspace %q with RF %d", di.keyspace, di.replicationFactor)
	}
	err := di.session.ExecStmt(fmt.Sprintf(
		`CREATE KEYSPACE %q WITH replication = {'class': 'NetworkTopologyStrategy', 'replication_factor': %d}`,
		di.keyspace,
		di.replicationFactor,
	))
	if err != nil {
		return fmt.Errorf("can't create keyspace: %w", err)
	}

	if di.verbose {
		framework.Infof("Creating table %s", di.table.Name())
	}
	err = di.session.ExecStmt(fmt.Sprintf(
		`CREATE TABLE %s (id int primary key, data text)`,
		di.table.Name(),
	))
	if err != nil {
		return fmt.Errorf("can't create table: %w", err)
	}

	if di.verbose {
		framework.Infof("Inserting data into table %s", di.table.Name())
	}
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

func (di *DataInserter) Read(ctx context.Context) ([]*TestData, error) {
	if di.verbose {
		framework.Infof("Reading data from table %s", di.table.Name())
	}

	q := di.session.Query(di.table.SelectAll()).
		WithContext(ctx).
		Consistency(di.readConsistency).
		BindStruct(&TestData{})
	var res []*TestData
	err := q.SelectRelease(&res)
	if err != nil {
		return nil, fmt.Errorf("can't select data: %w", err)
	}

	sort.Slice(res, func(i, j int) bool {
		return res[i].Id < res[j].Id
	})

	return res, nil
}

func (di *DataInserter) GetExpected() []*TestData {
	return di.data
}

func defaultClusterConfig(hosts []string) *gocql.ClusterConfig {
	clusterConfig := gocql.NewCluster(hosts...)
	clusterConfig.Timeout = 3 * time.Second
	clusterConfig.ConnectTimeout = 3 * time.Second
	// Set a small reconnect interval to avoid flakes, if not reconnected in time.
	clusterConfig.ReconnectInterval = 500 * time.Millisecond

	return clusterConfig
}

func (di *DataInserter) createSession(hosts []string) error {
	clusterConfig := defaultClusterConfig(hosts)
	if di.clusterConfigModifier != nil {
		clusterConfig = di.clusterConfigModifier(clusterConfig)
	}

	session, err := gocqlx.WrapSession(clusterConfig.CreateSession())
	if err != nil {
		return fmt.Errorf("can't create gocqlx session: %w", err)
	}

	session.SetConsistency(gocql.All)

	di.session = &session

	return nil
}
