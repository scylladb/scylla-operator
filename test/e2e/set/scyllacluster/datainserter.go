// Copyright (c) 2021 ScyllaDB

package scyllacluster

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gocql/gocql"
	"github.com/scylladb/gocqlx/v2"
	"github.com/scylladb/gocqlx/v2/table"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/uuid"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
)

const nRows = 10

type DataInserter struct {
	client            corev1client.CoreV1Interface
	session           *gocqlx.Session
	keyspace          string
	table             *table.Table
	data              []*TestData
	replicationFactor int32
}

type TestData struct {
	Id   int    `db:"id"`
	Data string `db:"data"`
}

func NewDataInserter(ctx context.Context, client corev1client.CoreV1Interface, sc *scyllav1.ScyllaCluster, replicationFactor int32) (*DataInserter, error) {
	if replicationFactor < 1 {
		return nil, fmt.Errorf("replication factor can't be set to less than 1")
	}

	keyspace := strings.Replace(string(uuid.NewUUID()), "-", "", -1)
	table := table.New(table.Metadata{
		Name:    fmt.Sprintf(`"%s"."test"`, keyspace),
		Columns: []string{"id", "data"},
		PartKey: []string{"id"},
	})
	data := make([]*TestData, 0, nRows)
	for i := 0; i < nRows; i++ {
		data = append(data, &TestData{Id: i, Data: rand.String(32)})
	}

	di := &DataInserter{
		client:            client,
		keyspace:          keyspace,
		table:             table,
		data:              data,
		replicationFactor: replicationFactor,
	}

	var err error
	di.session, err = di.createSession(ctx, sc)
	if err != nil {
		return nil, fmt.Errorf("can't create session: %w", err)
	}

	return di, nil
}

func (di *DataInserter) Close() {
	di.session.Close()
}

// UpdateClientEndpoints closes an existing session and opens a new one.
// In case an error was returned, DataInserter can no Longer be used.
func (di *DataInserter) UpdateClientEndpoints(ctx context.Context, sc *scyllav1.ScyllaCluster) error {
	di.session.Close()

	var err error
	di.session, err = di.createSession(ctx, sc)
	if err != nil {
		return fmt.Errorf("can't create session: %w", err)
	}

	return nil
}

func (di *DataInserter) Insert() error {
	framework.By("Inserting data with RF=%d", di.replicationFactor)

	err := di.session.ExecStmt(fmt.Sprintf(`CREATE KEYSPACE "%s" WITH replication = {'class' : 'NetworkTopologyStrategy','replication_factor' : %d}`, di.keyspace, di.replicationFactor))
	if err != nil {
		return fmt.Errorf("can't create keyspace: %w", err)
	}

	err = di.session.ExecStmt(fmt.Sprintf("CREATE TABLE %s (id int primary key, data text)", di.table.Name()))
	if err != nil {
		return fmt.Errorf("can't create table: %w", err)
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

func (di *DataInserter) Read() ([]*TestData, error) {
	framework.By("Reading data with RF=%d", di.replicationFactor)

	di.session.SetConsistency(gocql.All)

	var res []*TestData
	q := di.session.Query(di.table.SelectAll()).BindStruct(&TestData{})
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

func (di *DataInserter) createSession(ctx context.Context, sc *scyllav1.ScyllaCluster) (*gocqlx.Session, error) {
	hosts, err := getHosts(ctx, di.client, sc)
	if err != nil {
		return nil, fmt.Errorf("can't get hosts: %w", err)
	}

	clusterConfig := gocql.NewCluster(hosts...)
	clusterConfig.Timeout = 3 * time.Second
	clusterConfig.ConnectTimeout = 3 * time.Second

	session, err := gocqlx.WrapSession(clusterConfig.CreateSession())
	if err != nil {
		return nil, fmt.Errorf("can't create gocqlx session: %w", err)
	}

	return &session, nil
}
