// Copyright (C) 2021 ScyllaDB

package load

import (
	"fmt"
	"reflect"
	"sort"

	"k8s.io/apimachinery/pkg/util/rand"

	"github.com/gocql/gocql"
	"github.com/scylladb/gocqlx/v2"
	"github.com/scylladb/gocqlx/v2/table"
)

type Load struct {
	session  gocqlx.Session
	table    *table.Table
	expected []*test
}

type test struct {
	Id   int
	Data string
}

func NewLoad(hosts ...string) (*Load, error) {
	session, err := newSession(hosts...)
	if err != nil {
		return nil, err
	}

	l := &Load{
		session: session,
		table: table.New(table.Metadata{
			Name:    "test.test",
			Columns: []string{"id", "data"},
			PartKey: []string{"id"},
		}),
		expected: nil,
	}

	return l, nil
}

func (l *Load) Close() {
	l.session.Close()
}

// Reset closes an existing session and opens a new one.
// In case an error was returned, Load can no Longer be used.
func (l *Load) Reset(hosts ...string) error {
	l.session.Close()

	s, err := newSession(hosts...)
	if err != nil {
		return err
	}

	l.session = s
	return nil
}

func (l *Load) Write(n, replicationFactor int) error {
	if err := l.session.ExecStmt(`DROP KEYSPACE IF EXISTS test`); err != nil {
		return err
	}

	if err := l.session.
		ExecStmt(fmt.Sprintf("CREATE KEYSPACE test WITH replication = {'class' : 'NetworkTopologyStrategy','replication_factor' : %d}", replicationFactor)); err != nil {
		return err
	}

	if err := l.session.ExecStmt(`CREATE TABLE test.test (id int primary key, data text)`); err != nil {
		return err
	}

	var lst []*test
	for i := 0; i < n; i++ {
		t := &test{Id: i, Data: rand.String(32)}
		q := l.session.Query(l.table.Insert()).BindStruct(t)
		if err := q.ExecRelease(); err != nil {
			return err
		}
		lst = append(lst, t)
	}
	l.expected = lst

	return nil
}

func (l *Load) Update() error {
	for _, t := range l.expected {
		t.Data = rand.String(32)
		q := l.session.Query(l.table.Update("data")).BindStruct(t)
		if err := q.ExecRelease(); err != nil {
			return err
		}
	}

	return nil
}

func (l *Load) SetConsistency(c gocql.Consistency) {
	l.session.SetConsistency(c)
}

func (l *Load) VerifyRead() error {
	var res []*test
	q := l.session.Query(l.table.SelectAll()).BindStruct(&test{})
	if err := q.SelectRelease(&res); err != nil {
		return err
	}

	sort.Slice(res, func(i, j int) bool {
		return res[i].Id < res[j].Id
	})
	if !reflect.DeepEqual(res, l.expected) {
		return fmt.Errorf("read data differs from expected data")
	}

	return nil
}

func newSession(hosts ...string) (gocqlx.Session, error) {
	return gocqlx.WrapSession(gocql.NewCluster(hosts...).CreateSession())
}
