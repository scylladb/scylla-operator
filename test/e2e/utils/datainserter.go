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
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
)

const nRows = 10

type DataInserter struct {
	session           *gocqlx.Session
	keyspace          string
	table             *table.Table
	data              []*TestData
	replicationFactor map[string]int
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

func NewDataInserter(hosts []string, options ...DataInserterOption) (*DataInserter, error) {
	// Instead of specifying hosts for the provided datacenter, use 'replication_factor' as a single key to specify a default RF.
	return NewMultiDCDataInserter(map[string][]string{"replication_factor": hosts})
}

func NewMultiDCDataInserter(dcHosts map[string][]string, options ...DataInserterOption) (*DataInserter, error) {
	keyspace := utilrand.String(8)
	table := table.New(table.Metadata{
		Name:    fmt.Sprintf(`"%s"."test"`, keyspace),
		Columns: []string{"id", "data"},
		PartKey: []string{"id"},
	})
	data := make([]*TestData, 0, nRows)
	for i := range nRows {
		data = append(data, &TestData{Id: i, Data: utilrand.String(32)})
	}

	replicationFactor := make(map[string]int, len(dcHosts))
	for dc, hosts := range dcHosts {
		replicationFactor[dc] = len(hosts)
	}

	di := &DataInserter{
		keyspace:          keyspace,
		table:             table,
		data:              data,
		replicationFactor: replicationFactor,
	}

	for _, option := range options {
		option(di)
	}

	if di.session == nil {
		err := di.SetClientEndpoints(slices.Flatten(helpers.GetMapValues(dcHosts)))
		if err != nil {
			return nil, fmt.Errorf("can't set client endpoints: %w", err)
		}
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

	framework.Infof("Creating CQL session (hosts=%q)", strings.Join(hosts, ", "))
	err := di.createSession(hosts)
	if err != nil {
		return fmt.Errorf("can't create session: %w", err)
	}

	return nil
}

func (di *DataInserter) Insert() error {
	ss := make([]string, 0, len(di.replicationFactor))
	for dc, rf := range di.replicationFactor {
		ss = append(ss, fmt.Sprintf("'%s': %d", dc, rf))
	}
	replicationFactor := strings.Join(ss, ",")

	framework.Infof("Creating keyspace %q with RF %q", di.keyspace, replicationFactor)
	err := di.session.ExecStmt(fmt.Sprintf(
		`CREATE KEYSPACE %q WITH replication = {'class': 'NetworkTopologyStrategy', %s}`,
		di.keyspace,
		replicationFactor,
	))
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

	sort.Slice(res, func(i, j int) bool {
		return res[i].Id < res[j].Id
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
