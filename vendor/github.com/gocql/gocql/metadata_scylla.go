// Copyright (c) 2015 The gocql Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gocql

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"

	frm "github.com/gocql/gocql/internal/frame"
	"github.com/gocql/gocql/tablets"
)

// schema metadata for a keyspace
type KeyspaceMetadata struct {
	StrategyOptions map[string]interface{}
	Tables          map[string]*TableMetadata
	Functions       map[string]*FunctionMetadata
	Aggregates      map[string]*AggregateMetadata
	Types           map[string]*TypeMetadata
	Indexes         map[string]*IndexMetadata
	Views           map[string]*ViewMetadata
	Name            string
	StrategyClass   string
	CreateStmts     string
	DurableWrites   bool
}

// schema metadata for a table (a.k.a. column family)
type TableMetadata struct {
	Columns           map[string]*ColumnMetadata
	Extensions        map[string]interface{}
	Keyspace          string
	Name              string
	PartitionKey      []*ColumnMetadata
	ClusteringColumns []*ColumnMetadata
	OrderedColumns    []string
	Flags             []string
	Options           TableMetadataOptions
}

type TableMetadataOptions struct {
	Caching                 map[string]string
	Compaction              map[string]string
	Compression             map[string]string
	CDC                     map[string]string
	SpeculativeRetry        string
	Comment                 string
	Version                 string
	Partitioner             string
	GcGraceSeconds          int
	MaxIndexInterval        int
	MemtableFlushPeriodInMs int
	MinIndexInterval        int
	ReadRepairChance        float64
	BloomFilterFpChance     float64
	DefaultTimeToLive       int
	DcLocalReadRepairChance float64
	CrcCheckChance          float64
	InMemory                bool
}

func (t *TableMetadataOptions) Equals(other *TableMetadataOptions) bool {
	if t == nil || other == nil {
		return t == other // Both must be nil to be equal
	}

	if t.BloomFilterFpChance != other.BloomFilterFpChance ||
		t.Comment != other.Comment ||
		t.CrcCheckChance != other.CrcCheckChance ||
		t.DcLocalReadRepairChance != other.DcLocalReadRepairChance ||
		t.DefaultTimeToLive != other.DefaultTimeToLive ||
		t.GcGraceSeconds != other.GcGraceSeconds ||
		t.MaxIndexInterval != other.MaxIndexInterval ||
		t.MemtableFlushPeriodInMs != other.MemtableFlushPeriodInMs ||
		t.MinIndexInterval != other.MinIndexInterval ||
		t.ReadRepairChance != other.ReadRepairChance ||
		t.SpeculativeRetry != other.SpeculativeRetry ||
		t.InMemory != other.InMemory ||
		t.Partitioner != other.Partitioner ||
		t.Version != other.Version {
		return false
	}

	if !compareStringMaps(t.Caching, other.Caching) ||
		!compareStringMaps(t.Compaction, other.Compaction) ||
		!compareStringMaps(t.Compression, other.Compression) ||
		!compareStringMaps(t.CDC, other.CDC) {
		return false
	}

	return true
}

type ViewMetadata struct {
	Columns                 map[string]*ColumnMetadata
	Extensions              map[string]interface{}
	WhereClause             string
	BaseTableName           string
	ID                      string
	KeyspaceName            string
	BaseTableID             string
	ViewName                string
	OrderedColumns          []string
	PartitionKey            []*ColumnMetadata
	ClusteringColumns       []*ColumnMetadata
	Options                 TableMetadataOptions
	DcLocalReadRepairChance float64 // After Scylla 4.2 by default read_repair turned off
	ReadRepairChance        float64 // After Scylla 4.2 by default read_repair turned off
	IncludeAllColumns       bool
}

type ColumnMetadata struct {
	Index           ColumnIndexMetadata
	Keyspace        string
	Table           string
	Name            string
	Type            string
	ClusteringOrder string
	ComponentIndex  int
	Kind            ColumnKind
	Order           ColumnOrder
}

func (c *ColumnMetadata) Equals(other *ColumnMetadata) bool {
	if c == nil || other == nil {
		return c == other
	}

	return c.Keyspace == other.Keyspace &&
		c.Table == other.Table &&
		c.Name == other.Name &&
		c.ComponentIndex == other.ComponentIndex &&
		c.Kind == other.Kind &&
		c.Type == other.Type &&
		c.ClusteringOrder == other.ClusteringOrder &&
		c.Order == other.Order &&
		c.Index.Equals(&other.Index)
}

// FunctionMetadata holds metadata for function constructs
type FunctionMetadata struct {
	Keyspace          string
	Name              string
	Body              string
	Language          string
	ReturnType        string
	ArgumentTypes     []string
	ArgumentNames     []string
	CalledOnNullInput bool
}

// AggregateMetadata holds metadata for aggregate constructs
type AggregateMetadata struct {
	Keyspace      string
	Name          string
	InitCond      string
	ReturnType    string
	StateType     string
	stateFunc     string
	finalFunc     string
	ArgumentTypes []string
	FinalFunc     FunctionMetadata
	StateFunc     FunctionMetadata
}

// TypeMetadata holds the metadata for views.
type TypeMetadata struct {
	Keyspace   string
	Name       string
	FieldNames []string
	FieldTypes []string
}

type IndexMetadata struct {
	Name              string
	KeyspaceName      string
	TableName         string // Name of corresponding view.
	Kind              string
	Options           map[string]string
	Columns           map[string]*ColumnMetadata
	OrderedColumns    []string
	PartitionKey      []*ColumnMetadata
	ClusteringColumns []*ColumnMetadata
}

func (t *TableMetadata) Equals(other *TableMetadata) bool {
	if t == nil || other == nil {
		return t == other
	}

	if t.Keyspace != other.Keyspace || t.Name != other.Name {
		return false
	}

	if len(t.PartitionKey) != len(other.PartitionKey) || !compareColumnSlices(t.PartitionKey, other.PartitionKey) {
		return false
	}

	if len(t.ClusteringColumns) != len(other.ClusteringColumns) || !compareColumnSlices(t.ClusteringColumns, other.ClusteringColumns) {
		return false
	}

	if len(t.Columns) != len(other.Columns) || !compareColumnsMap(t.Columns, other.Columns) {
		return false
	}

	if len(t.OrderedColumns) != len(other.OrderedColumns) || !compareStringSlices(t.OrderedColumns, other.OrderedColumns) {
		return false
	}

	if !t.Options.Equals(&other.Options) {
		return false
	}

	if len(t.Flags) != len(other.Flags) || !compareStringSlices(t.Flags, other.Flags) {
		return false
	}

	if len(t.Extensions) != len(other.Extensions) || !compareInterfaceMaps(t.Extensions, other.Extensions) {
		return false
	}

	return true
}

func compareColumnSlices(a, b []*ColumnMetadata) bool {
	for i := range a {
		if !a[i].Equals(b[i]) {
			return false
		}
	}
	return true
}

func compareColumnsMap(a, b map[string]*ColumnMetadata) bool {
	for k, v := range a {
		otherValue, exists := b[k]
		if !exists || !v.Equals(otherValue) {
			return false
		}
	}
	return true
}

func compareStringSlices(a, b []string) bool {
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func compareStringMaps(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if otherValue, exists := b[k]; !exists || v != otherValue {
			return false
		}
	}
	return true
}

func compareInterfaceMaps(a, b map[string]interface{}) bool {
	for k, v := range a {
		otherValue, exists := b[k]
		if !exists || !reflect.DeepEqual(v, otherValue) {
			return false
		}
	}
	return true
}

// cowTabletList implements a copy on write keyspace metadata map, its equivalent type is map[string]*KeyspaceMetadata
type cowKeyspaceMetadataMap struct {
	keyspaceMap atomic.Value
	mu          sync.Mutex
}

func (c *cowKeyspaceMetadataMap) get() map[string]*KeyspaceMetadata {
	l, ok := c.keyspaceMap.Load().(map[string]*KeyspaceMetadata)
	if !ok {
		return nil
	}
	return l
}

func (c *cowKeyspaceMetadataMap) getKeyspace(keyspaceName string) (*KeyspaceMetadata, bool) {
	m, ok := c.keyspaceMap.Load().(map[string]*KeyspaceMetadata)
	if !ok {
		return nil, ok
	}
	val, ok := m[keyspaceName]
	return val, ok
}

func (c *cowKeyspaceMetadataMap) set(keyspaceName string, keyspaceMetadata *KeyspaceMetadata) bool {
	c.mu.Lock()
	m := c.get()

	newM := map[string]*KeyspaceMetadata{}
	for name, metadata := range m {
		newM[name] = metadata
	}
	newM[keyspaceName] = keyspaceMetadata

	c.keyspaceMap.Store(newM)
	c.mu.Unlock()
	return true
}

func (c *cowKeyspaceMetadataMap) remove(keyspaceName string) {
	c.mu.Lock()
	m := c.get()

	newM := map[string]*KeyspaceMetadata{}
	for name, meta := range m {
		if name != keyspaceName {
			newM[name] = meta
		}
	}

	c.keyspaceMap.Store(newM)
	c.mu.Unlock()
}

const (
	IndexKindCustom = "CUSTOM"
)

// the ordering of the column with regard to its comparator
type ColumnOrder bool

const (
	ASC  ColumnOrder = false
	DESC             = true
)

type ColumnIndexMetadata struct {
	Options map[string]interface{}
	Name    string
	Type    string
}

func (c *ColumnIndexMetadata) Equals(other *ColumnIndexMetadata) bool {
	if c == nil || other == nil {
		return c == other
	}

	if c.Name != other.Name || c.Type != other.Type {
		return false
	}

	// Compare the Options map
	if len(c.Options) != len(other.Options) {
		return false
	}
	for k, v := range c.Options {
		otherValue, exists := other.Options[k]
		if !exists || !reflect.DeepEqual(v, otherValue) {
			return false
		}
	}

	return true
}

type ColumnKind int

const (
	ColumnUnkownKind ColumnKind = iota
	ColumnPartitionKey
	ColumnClusteringKey
	ColumnRegular
	ColumnCompact
	ColumnStatic
)

func (c ColumnKind) String() string {
	switch c {
	case ColumnPartitionKey:
		return "partition_key"
	case ColumnClusteringKey:
		return "clustering_key"
	case ColumnRegular:
		return "regular"
	case ColumnCompact:
		return "compact"
	case ColumnStatic:
		return "static"
	default:
		return fmt.Sprintf("unknown_column_%d", c)
	}
}

func (c *ColumnKind) UnmarshalCQL(typ TypeInfo, p []byte) error {
	if typ.Type() != TypeVarchar {
		return unmarshalErrorf("unable to marshall %s into ColumnKind, expected Varchar", typ)
	}

	kind, err := columnKindFromSchema(string(p))
	if err != nil {
		return err
	}
	*c = kind

	return nil
}

func columnKindFromSchema(kind string) (ColumnKind, error) {
	switch kind {
	case "partition_key":
		return ColumnPartitionKey, nil
	case "clustering_key", "clustering":
		return ColumnClusteringKey, nil
	case "regular":
		return ColumnRegular, nil
	case "compact_value":
		return ColumnCompact, nil
	case "static":
		return ColumnStatic, nil
	default:
		return -1, fmt.Errorf("unknown column kind: %q", kind)
	}
}

type Metadata struct {
	tabletsMetadata  tablets.CowTabletList
	keyspaceMetadata cowKeyspaceMetadataMap
}

// queries the cluster for schema information for a specific keyspace and for tablets
type metadataDescriber struct {
	session  *Session
	metadata *Metadata
	mu       sync.Mutex
}

// creates a session bound schema describer which will query and cache
// keyspace metadata and tablets metadata
func newMetadataDescriber(session *Session) *metadataDescriber {
	return &metadataDescriber{
		session: session,
		metadata: &Metadata{
			tabletsMetadata: tablets.NewCowTabletList(),
		},
	}
}

// getSchema returns the KeyspaceMetadata for the keyspace, if it is not present, loads it from `system_schema`
// does not require holding a lock
func (s *metadataDescriber) getSchema(keyspaceName string) (*KeyspaceMetadata, error) {
	metadata, found := s.metadata.keyspaceMetadata.getKeyspace(keyspaceName)
	if !found {
		// refresh the cache for this keyspace
		err := s.refreshSchema(keyspaceName)
		if err != nil {
			return nil, err
		}

		metadata, found = s.metadata.keyspaceMetadata.getKeyspace(keyspaceName)
		if !found {
			return nil, fmt.Errorf("Metadata not found for keyspace: %s", keyspaceName)
		}
	}

	return metadata, nil
}

func (s *metadataDescriber) getTablets() tablets.TabletInfoList {
	return s.metadata.tabletsMetadata.Get()
}

func (s *metadataDescriber) AddTablet(tablet *tablets.TabletInfo) {
	s.metadata.tabletsMetadata.AddTablet(tablet)
}

// RemoveTabletsWithHost removes tablets that contains given host.
// to be used outside the metadataDescriber
func (s *metadataDescriber) RemoveTabletsWithHost(host *HostInfo) {
	s.metadata.tabletsMetadata.RemoveTabletsWithHost(host.HostID())
}

// RemoveTabletsWithKeyspace removes tablets for given keyspace.
// to be used outside the metadataDescriber
func (s *metadataDescriber) RemoveTabletsWithKeyspace(keyspace string) {
	s.metadata.tabletsMetadata.RemoveTabletsWithKeyspace(keyspace)
}

// RemoveTabletsWithTable removes tablets for given table.
// to be used outside the metadataDescriber
func (s *metadataDescriber) RemoveTabletsWithTable(keyspace string, table string) {
	s.metadata.tabletsMetadata.RemoveTabletsWithTableFromTabletsList(keyspace, table)
}

// clearSchema clears the cached keyspace metadata
func (s *metadataDescriber) clearSchema(keyspaceName string) {
	s.metadata.keyspaceMetadata.remove(keyspaceName)
}

func (s *metadataDescriber) refreshAllSchema() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	copiedMap := make(map[string]*KeyspaceMetadata)

	for key, value := range s.metadata.keyspaceMetadata.get() {
		if value != nil {
			copiedMap[key] = &KeyspaceMetadata{
				Name:            value.Name,
				DurableWrites:   value.DurableWrites,
				StrategyClass:   value.StrategyClass,
				StrategyOptions: value.StrategyOptions,
				Tables:          value.Tables,
				Functions:       value.Functions,
				Aggregates:      value.Aggregates,
				Types:           value.Types,
				Indexes:         value.Indexes,
				Views:           value.Views,
				CreateStmts:     value.CreateStmts,
			}
		} else {
			copiedMap[key] = nil
		}
	}

	for keyspaceName, metadata := range copiedMap {
		// refresh the cache for this keyspace
		err := s.refreshSchema(keyspaceName)
		if errors.Is(err, ErrKeyspaceDoesNotExist) {
			s.clearSchema(keyspaceName)
			s.RemoveTabletsWithKeyspace(keyspaceName)
			continue
		} else if err != nil {
			return err
		}

		updatedMetadata, err := s.getSchema(keyspaceName)
		if err != nil {
			return err
		}

		if !compareInterfaceMaps(metadata.StrategyOptions, updatedMetadata.StrategyOptions) {
			s.RemoveTabletsWithKeyspace(keyspaceName)
			continue
		}

		for tableName, tableMetadata := range metadata.Tables {
			if updatedTableMetadata, ok := updatedMetadata.Tables[tableName]; !ok || tableMetadata.Equals(updatedTableMetadata) {
				s.RemoveTabletsWithTable(keyspaceName, tableName)
			}
		}
	}
	return nil
}

// forcibly updates the current KeyspaceMetadata held by the schema describer
// for a given named keyspace.
func (s *metadataDescriber) refreshSchema(keyspaceName string) error {
	var err error

	// query the system keyspace for schema data
	// TODO retrieve concurrently
	keyspace, err := getKeyspaceMetadata(s.session, keyspaceName)
	if err != nil {
		return err
	}
	tables, err := getTableMetadata(s.session, keyspaceName)
	if err != nil {
		return err
	}
	columns, err := getColumnMetadata(s.session, keyspaceName)
	if err != nil {
		return err
	}
	functions, err := getFunctionsMetadata(s.session, keyspaceName)
	if err != nil {
		return err
	}
	aggregates, err := getAggregatesMetadata(s.session, keyspaceName)
	if err != nil {
		return err
	}
	types, err := getTypeMetadata(s.session, keyspaceName)
	if err != nil {
		return err
	}
	indexes, err := getIndexMetadata(s.session, keyspaceName)
	if err != nil {
		return err
	}
	views, err := getViewMetadata(s.session, keyspaceName)
	if err != nil {
		return err
	}

	createStmts, err := getCreateStatements(s.session, keyspaceName)
	if err != nil {
		return err
	}

	// organize the schema data
	compileMetadata(keyspace, tables, columns, functions, aggregates, types, indexes, views, createStmts)

	// update the cache
	s.metadata.keyspaceMetadata.set(keyspaceName, keyspace)

	return nil
}

// "compiles" derived information about keyspace, table, and column metadata
// for a keyspace from the basic queried metadata objects returned by
// getKeyspaceMetadata, getTableMetadata, and getColumnMetadata respectively;
// Links the metadata objects together and derives the column composition of
// the partition key and clustering key for a table.
func compileMetadata(
	keyspace *KeyspaceMetadata,
	tables []TableMetadata,
	columns []ColumnMetadata,
	functions []FunctionMetadata,
	aggregates []AggregateMetadata,
	types []TypeMetadata,
	indexes []IndexMetadata,
	views []ViewMetadata,
	createStmts []byte,
) {
	keyspace.Tables = make(map[string]*TableMetadata)
	for i := range tables {
		tables[i].Columns = make(map[string]*ColumnMetadata)
		keyspace.Tables[tables[i].Name] = &tables[i]
	}
	keyspace.Functions = make(map[string]*FunctionMetadata, len(functions))
	for i := range functions {
		keyspace.Functions[functions[i].Name] = &functions[i]
	}
	keyspace.Aggregates = make(map[string]*AggregateMetadata, len(aggregates))
	for _, aggregate := range aggregates {
		aggregate.FinalFunc = *keyspace.Functions[aggregate.finalFunc]
		aggregate.StateFunc = *keyspace.Functions[aggregate.stateFunc]
		keyspace.Aggregates[aggregate.Name] = &aggregate
	}
	keyspace.Types = make(map[string]*TypeMetadata, len(types))
	for i := range types {
		keyspace.Types[types[i].Name] = &types[i]
	}
	keyspace.Indexes = make(map[string]*IndexMetadata, len(indexes))
	for i := range indexes {
		indexes[i].Columns = make(map[string]*ColumnMetadata)
		keyspace.Indexes[indexes[i].Name] = &indexes[i]

	}
	keyspace.Views = make(map[string]*ViewMetadata, len(views))
	for i := range views {
		v := &views[i]
		if _, ok := keyspace.Indexes[strings.TrimSuffix(v.ViewName, "_index")]; ok {
			continue
		}

		v.Columns = make(map[string]*ColumnMetadata)
		keyspace.Views[v.ViewName] = v
	}

	// add columns from the schema data
	for i := range columns {
		col := &columns[i]
		col.Order = ASC
		if col.ClusteringOrder == "desc" {
			col.Order = DESC
		}

		table, ok := keyspace.Tables[col.Table]
		if !ok {
			// If column owned by a table that the table name ends with `_index`
			// suffix then the table is a view corresponding to some index.
			if indexName, found := strings.CutSuffix(col.Table, "_index"); found {
				ix, ok := keyspace.Indexes[indexName]
				if ok {
					ix.Columns[col.Name] = col
					ix.OrderedColumns = append(ix.OrderedColumns, col.Name)
					continue
				}
			}

			view, ok := keyspace.Views[col.Table]
			if !ok {
				// if the schema is being updated we will race between seeing
				// the metadata be complete. Potentially we should check for
				// schema versions before and after reading the metadata and
				// if they dont match try again.
				continue
			}

			view.Columns[col.Name] = col
			view.OrderedColumns = append(view.OrderedColumns, col.Name)
			continue
		}

		table.Columns[col.Name] = col
		table.OrderedColumns = append(table.OrderedColumns, col.Name)
	}

	for i := range tables {
		t := &tables[i]
		t.PartitionKey, t.ClusteringColumns, t.OrderedColumns = compileColumns(t.Columns, t.OrderedColumns)
	}
	for i := range views {
		v := &views[i]
		v.PartitionKey, v.ClusteringColumns, v.OrderedColumns = compileColumns(v.Columns, v.OrderedColumns)
	}
	for i := range indexes {
		ix := &indexes[i]
		ix.PartitionKey, ix.ClusteringColumns, ix.OrderedColumns = compileColumns(ix.Columns, ix.OrderedColumns)
	}

	keyspace.CreateStmts = string(createStmts)
}

func compileColumns(columns map[string]*ColumnMetadata, orderedColumns []string) (
	partitionKey, clusteringColumns []*ColumnMetadata, sortedColumns []string) {
	clusteringColumnCount := componentColumnCountOfType(columns, ColumnClusteringKey)
	clusteringColumns = make([]*ColumnMetadata, clusteringColumnCount)

	partitionKeyCount := componentColumnCountOfType(columns, ColumnPartitionKey)
	partitionKey = make([]*ColumnMetadata, partitionKeyCount)

	var otherColumns []string
	for _, columnName := range orderedColumns {
		column := columns[columnName]
		if column.Kind == ColumnPartitionKey {
			partitionKey[column.ComponentIndex] = column
		} else if column.Kind == ColumnClusteringKey {
			clusteringColumns[column.ComponentIndex] = column
		} else {
			otherColumns = append(otherColumns, columnName)
		}
	}

	sortedColumns = orderedColumns[:0]
	for _, pk := range partitionKey {
		sortedColumns = append(sortedColumns, pk.Name)
	}
	for _, ck := range clusteringColumns {
		sortedColumns = append(sortedColumns, ck.Name)
	}
	for _, oc := range otherColumns {
		sortedColumns = append(sortedColumns, oc)
	}

	return
}

// returns the count of coluns with the given "kind" value.
func componentColumnCountOfType(columns map[string]*ColumnMetadata, kind ColumnKind) int {
	maxComponentIndex := -1
	for _, column := range columns {
		if column.Kind == kind && column.ComponentIndex > maxComponentIndex {
			maxComponentIndex = column.ComponentIndex
		}
	}
	return maxComponentIndex + 1
}

// query for keyspace metadata in the system_schema.keyspaces
func getKeyspaceMetadata(session *Session, keyspaceName string) (*KeyspaceMetadata, error) {
	if !session.useSystemSchema {
		return nil, ErrKeyspaceDoesNotExist
	}
	keyspace := &KeyspaceMetadata{Name: keyspaceName}

	const stmt = `
		SELECT durable_writes, replication
		FROM system_schema.keyspaces
		WHERE keyspace_name = ?`

	var replication map[string]string

	iter := session.control.querySystem(stmt, keyspaceName)
	if iter.NumRows() == 0 {
		return nil, ErrKeyspaceDoesNotExist
	}
	iter.Scan(&keyspace.DurableWrites, &replication)
	err := iter.Close()
	if err != nil {
		return nil, fmt.Errorf("error querying keyspace schema: %v", err)
	}

	keyspace.StrategyClass = replication["class"]
	delete(replication, "class")

	keyspace.StrategyOptions = make(map[string]interface{}, len(replication))
	for k, v := range replication {
		keyspace.StrategyOptions[k] = v
	}

	return keyspace, nil
}

// query for table metadata in the system_schema.tables, and system_schema.scylla_tables
// if connected to ScyllaDB
func getTableMetadata(session *Session, keyspaceName string) ([]TableMetadata, error) {
	if !session.useSystemSchema {
		return nil, nil
	}

	stmt := `SELECT * FROM system_schema.tables WHERE keyspace_name = ?`
	iter := session.control.querySystem(stmt, keyspaceName)

	var tables []TableMetadata
	table := TableMetadata{Keyspace: keyspaceName}
	for iter.MapScan(map[string]interface{}{
		"table_name":                  &table.Name,
		"bloom_filter_fp_chance":      &table.Options.BloomFilterFpChance,
		"caching":                     &table.Options.Caching,
		"comment":                     &table.Options.Comment,
		"compaction":                  &table.Options.Compaction,
		"compression":                 &table.Options.Compression,
		"crc_check_chance":            &table.Options.CrcCheckChance,
		"default_time_to_live":        &table.Options.DefaultTimeToLive,
		"gc_grace_seconds":            &table.Options.GcGraceSeconds,
		"max_index_interval":          &table.Options.MaxIndexInterval,
		"memtable_flush_period_in_ms": &table.Options.MemtableFlushPeriodInMs,
		"min_index_interval":          &table.Options.MinIndexInterval,
		"speculative_retry":           &table.Options.SpeculativeRetry,
		"flags":                       &table.Flags,
		"extensions":                  &table.Extensions,
	}) {
		tables = append(tables, table)
		table = TableMetadata{Keyspace: keyspaceName}
	}

	err := iter.Close()
	if err != nil && err != ErrNotFound {
		return nil, fmt.Errorf("error querying table schema: %v", err)
	}

	if session.getConn() == nil || !session.getConn().isScyllaConn() {
		return tables, nil
	}

	stmt = `SELECT * FROM system_schema.scylla_tables WHERE keyspace_name = ? AND table_name = ?`
	for i, t := range tables {
		iter := session.control.querySystem(stmt, keyspaceName, t.Name)

		table := TableMetadata{}
		if iter.MapScan(map[string]interface{}{
			"cdc":         &table.Options.CDC,
			"in_memory":   &table.Options.InMemory,
			"partitioner": &table.Options.Partitioner,
			"version":     &table.Options.Version,
		}) {
			tables[i].Options.CDC = table.Options.CDC
			tables[i].Options.Version = table.Options.Version
			tables[i].Options.Partitioner = table.Options.Partitioner
			tables[i].Options.InMemory = table.Options.InMemory
		}
		if err := iter.Close(); err != nil && err != ErrNotFound {
			return nil, fmt.Errorf("error querying scylla table schema: %v", err)
		}
	}

	return tables, nil
}

// query for column metadata in the system_schema.columns
func getColumnMetadata(session *Session, keyspaceName string) ([]ColumnMetadata, error) {
	const stmt = `SELECT * FROM system_schema.columns WHERE keyspace_name = ?`

	var columns []ColumnMetadata

	iter := session.control.querySystem(stmt, keyspaceName)
	column := ColumnMetadata{Keyspace: keyspaceName}

	for iter.MapScan(map[string]interface{}{
		"table_name":       &column.Table,
		"column_name":      &column.Name,
		"clustering_order": &column.ClusteringOrder,
		"type":             &column.Type,
		"kind":             &column.Kind,
		"position":         &column.ComponentIndex,
	}) {
		columns = append(columns, column)
		column = ColumnMetadata{Keyspace: keyspaceName}
	}

	if err := iter.Close(); err != nil && err != ErrNotFound {
		return nil, fmt.Errorf("error querying column schema: %v", err)
	}

	return columns, nil
}

// query for type metadata in the system_schema.types
func getTypeMetadata(session *Session, keyspaceName string) ([]TypeMetadata, error) {
	if !session.useSystemSchema {
		return nil, nil
	}

	stmt := `SELECT * FROM system_schema.types WHERE keyspace_name = ?`
	iter := session.control.querySystem(stmt, keyspaceName)

	var types []TypeMetadata
	tm := TypeMetadata{Keyspace: keyspaceName}

	for iter.MapScan(map[string]interface{}{
		"type_name":   &tm.Name,
		"field_names": &tm.FieldNames,
		"field_types": &tm.FieldTypes,
	}) {
		types = append(types, tm)
		tm = TypeMetadata{Keyspace: keyspaceName}
	}

	if err := iter.Close(); err != nil {
		return nil, err
	}

	return types, nil
}

// query for function metadata in the system_schema.functions
func getFunctionsMetadata(session *Session, keyspaceName string) ([]FunctionMetadata, error) {
	if !session.hasAggregatesAndFunctions || !session.useSystemSchema {
		return nil, nil
	}
	stmt := `SELECT * FROM system_schema.functions WHERE keyspace_name = ?`

	var functions []FunctionMetadata
	function := FunctionMetadata{Keyspace: keyspaceName}

	iter := session.control.querySystem(stmt, keyspaceName)
	for iter.MapScan(map[string]interface{}{
		"function_name":        &function.Name,
		"argument_types":       &function.ArgumentTypes,
		"argument_names":       &function.ArgumentNames,
		"body":                 &function.Body,
		"called_on_null_input": &function.CalledOnNullInput,
		"language":             &function.Language,
		"return_type":          &function.ReturnType,
	}) {
		functions = append(functions, function)
		function = FunctionMetadata{Keyspace: keyspaceName}
	}

	if err := iter.Close(); err != nil {
		return nil, err
	}

	return functions, nil
}

// query for aggregate metadata in the system_schema.aggregates
func getAggregatesMetadata(session *Session, keyspaceName string) ([]AggregateMetadata, error) {
	if !session.hasAggregatesAndFunctions || !session.useSystemSchema {
		return nil, nil
	}

	const stmt = `SELECT * FROM system_schema.aggregates WHERE keyspace_name = ?`

	var aggregates []AggregateMetadata
	aggregate := AggregateMetadata{Keyspace: keyspaceName}

	iter := session.control.querySystem(stmt, keyspaceName)
	for iter.MapScan(map[string]interface{}{
		"aggregate_name": &aggregate.Name,
		"argument_types": &aggregate.ArgumentTypes,
		"final_func":     &aggregate.finalFunc,
		"initcond":       &aggregate.InitCond,
		"return_type":    &aggregate.ReturnType,
		"state_func":     &aggregate.stateFunc,
		"state_type":     &aggregate.StateType,
	}) {
		aggregates = append(aggregates, aggregate)
		aggregate = AggregateMetadata{Keyspace: keyspaceName}
	}

	if err := iter.Close(); err != nil {
		return nil, err
	}

	return aggregates, nil
}

// query for index metadata in the system_schema.indexes
func getIndexMetadata(session *Session, keyspaceName string) ([]IndexMetadata, error) {
	if !session.useSystemSchema {
		return nil, nil
	}

	const stmt = `SELECT * FROM system_schema.indexes WHERE keyspace_name = ?`

	var indexes []IndexMetadata
	index := IndexMetadata{}

	iter := session.control.querySystem(stmt, keyspaceName)
	for iter.MapScan(map[string]interface{}{
		"index_name":    &index.Name,
		"keyspace_name": &index.KeyspaceName,
		"table_name":    &index.TableName,
		"kind":          &index.Kind,
		"options":       &index.Options,
	}) {
		indexes = append(indexes, index)
		index = IndexMetadata{}
	}

	if err := iter.Close(); err != nil {
		return nil, err
	}

	return indexes, nil
}

// get create statements for the keyspace
func getCreateStatements(session *Session, keyspaceName string) ([]byte, error) {
	if !session.useSystemSchema {
		return nil, nil
	}
	iter := session.control.query(fmt.Sprintf(`DESCRIBE KEYSPACE %s WITH INTERNALS`, keyspaceName))

	var createStatements []string

	var stmt string
	for iter.Scan(nil, nil, nil, &stmt) {
		if stmt == "" {
			continue
		}
		createStatements = append(createStatements, stmt)
	}

	if err := iter.Close(); err != nil {
		if errFrame, ok := err.(frm.ErrorFrame); ok && errFrame.Code == ErrCodeSyntax {
			// DESCRIBE KEYSPACE is not supported on older versions of Cassandra and Scylla
			// For such case schema statement is going to be recreated on the client side
			return nil, nil
		}
		return nil, fmt.Errorf("error querying keyspace schema: %v", err)
	}

	return []byte(strings.Join(createStatements, "\n")), nil
}

// query for view metadata in the system_schema.views
func getViewMetadata(session *Session, keyspaceName string) ([]ViewMetadata, error) {
	if !session.useSystemSchema {
		return nil, nil
	}

	stmt := `SELECT * FROM system_schema.views WHERE keyspace_name = ?`

	iter := session.control.querySystem(stmt, keyspaceName)

	var views []ViewMetadata
	view := ViewMetadata{KeyspaceName: keyspaceName}

	for iter.MapScan(map[string]interface{}{
		"id":                          &view.ID,
		"view_name":                   &view.ViewName,
		"base_table_id":               &view.BaseTableID,
		"base_table_name":             &view.BaseTableName,
		"include_all_columns":         &view.IncludeAllColumns,
		"where_clause":                &view.WhereClause,
		"bloom_filter_fp_chance":      &view.Options.BloomFilterFpChance,
		"caching":                     &view.Options.Caching,
		"comment":                     &view.Options.Comment,
		"compaction":                  &view.Options.Compaction,
		"compression":                 &view.Options.Compression,
		"crc_check_chance":            &view.Options.CrcCheckChance,
		"default_time_to_live":        &view.Options.DefaultTimeToLive,
		"gc_grace_seconds":            &view.Options.GcGraceSeconds,
		"max_index_interval":          &view.Options.MaxIndexInterval,
		"memtable_flush_period_in_ms": &view.Options.MemtableFlushPeriodInMs,
		"min_index_interval":          &view.Options.MinIndexInterval,
		"speculative_retry":           &view.Options.SpeculativeRetry,
		"extensions":                  &view.Extensions,
		"dclocal_read_repair_chance":  &view.DcLocalReadRepairChance,
		"read_repair_chance":          &view.ReadRepairChance,
	}) {
		views = append(views, view)
		view = ViewMetadata{KeyspaceName: keyspaceName}
	}

	err := iter.Close()
	if err != nil && err != ErrNotFound {
		return nil, fmt.Errorf("error querying view schema: %v", err)
	}

	return views, nil
}
