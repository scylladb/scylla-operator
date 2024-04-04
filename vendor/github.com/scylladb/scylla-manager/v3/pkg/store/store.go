// Copyright (C) 2017 ScyllaDB

package store

import (
	"encoding"

	"github.com/pkg/errors"
	"github.com/scylladb/gocqlx/v2"
	"github.com/scylladb/gocqlx/v2/qb"
	"github.com/scylladb/gocqlx/v2/table"
	"github.com/scylladb/scylla-manager/v3/pkg/util/uuid"
)

// ErrInvalidKey is returned when Entry have nil clusterID or empty key.
var ErrInvalidKey = errors.New("Missing clusterID or key")

// Entry is key value pair that can be persisted in Store.
// Key is obtained by Key method. Value is obtained by marshalling.
type Entry interface {
	Key() (clusterID uuid.UUID, key string)
	encoding.BinaryMarshaler
	encoding.BinaryUnmarshaler
}

// Store is specifies interface used for storing arbitrary entries.
// In order to store value, provided type must implement KeyValue interface.
type Store interface {
	Put(v Entry) error
	Get(v Entry) error
	Check(v Entry) (bool, error)
	Delete(v Entry) error
	DeleteAll(clusterID uuid.UUID) error
}

// TableStore stores entries in a table, table must.
type TableStore struct {
	session gocqlx.Session
	table   *table.Table
}

var _ Store = &TableStore{}

func NewTableStore(session gocqlx.Session, table *table.Table) *TableStore {
	return &TableStore{
		session: session,
		table:   table,
	}
}

// Put saves provided entry into table.
func (s *TableStore) Put(e Entry) error {
	clusterID, key := e.Key()
	if clusterID == uuid.Nil || key == "" {
		return ErrInvalidKey
	}
	value, err := e.MarshalBinary()
	if err != nil {
		return errors.Wrap(err, "marshal")
	}

	return s.table.InsertQuery(s.session).BindMap(qb.M{
		"cluster_id": clusterID,
		"key":        key,
		"value":      value,
	}).ExecRelease()
}

// Get marshals value into existing entry based on it's key.
func (s *TableStore) Get(e Entry) error {
	clusterID, key := e.Key()
	if clusterID == uuid.Nil || key == "" {
		return ErrInvalidKey
	}

	var v []byte
	err := s.table.GetQuery(s.session, "value").BindMap(qb.M{
		"cluster_id": clusterID,
		"key":        key,
	}).GetRelease(&v)
	if err != nil {
		return err
	}
	return e.UnmarshalBinary(v)
}

// Check if entry with given key exists.
func (s *TableStore) Check(e Entry) (bool, error) {
	clusterID, key := e.Key()
	if clusterID == uuid.Nil || key == "" {
		return false, ErrInvalidKey
	}

	var v int
	err := s.table.GetQuery(s.session, "COUNT(value)").BindMap(qb.M{
		"cluster_id": clusterID,
		"key":        key,
	}).GetRelease(&v)
	if err != nil {
		return false, err
	}
	return v > 0, nil
}

// Delete removes entry for a given cluster and key.
func (s *TableStore) Delete(e Entry) error {
	clusterID, key := e.Key()
	if clusterID == uuid.Nil || key == "" {
		return ErrInvalidKey
	}

	return s.table.DeleteQuery(s.session).BindMap(qb.M{
		"cluster_id": clusterID,
		"key":        key,
	}).ExecRelease()
}

// DeleteAll removes all entries for a cluster.
func (s *TableStore) DeleteAll(clusterID uuid.UUID) error {
	return qb.Delete(s.table.Name()).Where(qb.Eq("cluster_id")).Query(s.session).BindMap(qb.M{
		"cluster_id": clusterID,
	}).ExecRelease()
}
