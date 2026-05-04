// Copyright (c) 2015 The gocql Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gocql

import (
	"errors"
	"fmt"
	"maps"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/singleflight"

	frm "github.com/gocql/gocql/internal/frame"
	"github.com/gocql/gocql/tablets"
)

// schema metadata for a keyspace
type KeyspaceMetadata struct {
	StrategyOptions   map[string]any
	Tables            map[string]*TableMetadata
	Functions         map[string]*FunctionMetadata
	Aggregates        map[string]*AggregateMetadata
	Types             map[string]*TypeMetadata
	Indexes           map[string]*IndexMetadata
	Views             map[string]*ViewMetadata
	tablesInvalidated map[string]struct{}
	Name              string
	StrategyClass     string
	CreateStmts       string
	DurableWrites     bool
}

// Clone returns a shallow copy of the keyspace metadata with
// cloned Tables, Indexes, Views, and tablesInvalidated maps so that mutations
// do not race with concurrent readers of the original.
func (ks *KeyspaceMetadata) Clone() *KeyspaceMetadata {
	cloned := &KeyspaceMetadata{
		Name:            ks.Name,
		DurableWrites:   ks.DurableWrites,
		StrategyClass:   ks.StrategyClass,
		StrategyOptions: maps.Clone(ks.StrategyOptions),
		Tables:          maps.Clone(ks.Tables),
		Functions:       maps.Clone(ks.Functions),
		Aggregates:      maps.Clone(ks.Aggregates),
		Types:           maps.Clone(ks.Types),
		Indexes:         maps.Clone(ks.Indexes),
		Views:           maps.Clone(ks.Views),
		CreateStmts:     ks.CreateStmts,
	}
	if ks.tablesInvalidated != nil {
		cloned.tablesInvalidated = maps.Clone(ks.tablesInvalidated)
	}
	return cloned
}

func (ks *KeyspaceMetadata) removeTableData(tableName string) {
	if ks.Tables != nil {
		delete(ks.Tables, tableName)
	}
	for name, idx := range ks.Indexes {
		if idx != nil && idx.TableName == tableName {
			delete(ks.Indexes, name)
		}
	}
	for name, view := range ks.Views {
		if view != nil && view.BaseTableName == tableName {
			delete(ks.Views, name)
		}
	}
}

func (ks *KeyspaceMetadata) invalidateTable(tableName string) {
	ks.removeTableData(tableName)
	if ks.tablesInvalidated == nil {
		ks.tablesInvalidated = make(map[string]struct{})
	}
	ks.tablesInvalidated[tableName] = struct{}{}
}

func (ks *KeyspaceMetadata) removeTable(tableName string) {
	ks.removeTableData(tableName)
	if ks.tablesInvalidated != nil {
		delete(ks.tablesInvalidated, tableName)
	}
}

// schema metadata for a table (a.k.a. column family)
type TableMetadata struct {
	Columns           map[string]*ColumnMetadata
	Extensions        map[string]any
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
	Extensions              map[string]any
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

func compareInterfaceMaps(a, b map[string]any) bool {
	if len(a) != len(b) {
		return false
	}
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
	defer c.mu.Unlock()

	m := c.get()

	newM := map[string]*KeyspaceMetadata{}
	for name, metadata := range m {
		newM[name] = metadata
	}
	newM[keyspaceName] = keyspaceMetadata

	c.keyspaceMap.Store(newM)
	return true
}

func (c *cowKeyspaceMetadataMap) invalidateTable(keyspaceName, tableName string) {
	c.updateKeyspace(keyspaceName, func(ks *KeyspaceMetadata) {
		ks.invalidateTable(tableName)
	})
}

func (c *cowKeyspaceMetadataMap) removeKeyspace(keyspaceName string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	m := c.get()
	newM := maps.Clone(m)
	delete(newM, keyspaceName)

	c.keyspaceMap.Store(newM)
}

// updateKeyspace atomically clones a keyspace's mutable maps, applies fn to
// the clone, and publishes the result. This prevents data races between
// concurrent readers and writers of the same KeyspaceMetadata.
// Returns false if the keyspace was not found (no update applied).
func (c *cowKeyspaceMetadataMap) updateKeyspace(keyspaceName string, fn func(ks *KeyspaceMetadata)) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	m := c.get()
	ks, ok := m[keyspaceName]
	if !ok || ks == nil {
		return false
	}

	cloned := ks.Clone()
	fn(cloned)

	newM := maps.Clone(m)
	newM[keyspaceName] = cloned
	c.keyspaceMap.Store(newM)
	return true
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
	Options map[string]any
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
	tabletsMetadata  *tablets.CowTabletList
	keyspaceMetadata cowKeyspaceMetadataMap
}

// queries the cluster for schema information for a specific keyspace and for tablets
type metadataDescriber struct {
	keyspaceGroup singleflight.Group
	tableGroup    singleflight.Group
	session       *Session
	metadata      *Metadata

	// mu serialises refreshAllSchema calls so the snapshot-compare-refresh
	// cycle runs as an atomic batch.  Individual keyspace/table refreshes
	// are deduplicated by the singleflight groups above and do NOT need
	// this lock.
	//
	// Lock ordering: s.mu → cowKeyspaceMetadataMap.mu (never reversed).
	mu sync.Mutex
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

func (s *metadataDescriber) getKeyspaceInternal(keyspaceName string) (metadata *KeyspaceMetadata, wasReloaded bool, err error) {
	var found bool
	metadata, found = s.metadata.keyspaceMetadata.getKeyspace(keyspaceName)
	if !found {
		wasReloaded = true
		err = s.deduplicatedRefreshKeyspace(keyspaceName)
		if err != nil {
			return metadata, wasReloaded, err
		}

		metadata, found = s.metadata.keyspaceMetadata.getKeyspace(keyspaceName)
		if !found {
			return nil, true, fmt.Errorf("keyspace %s: %w", keyspaceName, ErrNotFound)
		}
	}

	return metadata, wasReloaded, nil
}

func (s *metadataDescriber) GetKeyspace(keyspaceName string) (*KeyspaceMetadata, error) {
	metadata, _, err := s.getKeyspaceInternal(keyspaceName)
	return metadata, err
}

func tableNotFoundError(keyspaceName, tableName string) error {
	return fmt.Errorf("table %s.%s: %w", keyspaceName, tableName, ErrNotFound)
}

// getTableFromSnapshot resolves a table lookup against a keyspace snapshot.
// It re-reads the latest published keyspace metadata before deciding that an
// invalidated table still needs a refresh, which avoids duplicate refreshes
// for callers holding a stale snapshot.
func (s *metadataDescriber) getTableFromSnapshot(
	keyspaceName, tableName string,
	keyspaceMetadata *KeyspaceMetadata,
	wasReloaded bool,
) (tableMetadata *TableMetadata, refreshNeeded bool, err error) {
	if tableMetadata, found := keyspaceMetadata.Tables[tableName]; found {
		return tableMetadata, false, nil
	}

	if latestMetadata, found := s.metadata.keyspaceMetadata.getKeyspace(keyspaceName); found && latestMetadata != keyspaceMetadata {
		keyspaceMetadata = latestMetadata
		if tableMetadata, found := keyspaceMetadata.Tables[tableName]; found {
			return tableMetadata, false, nil
		}
	}

	if wasReloaded {
		return nil, false, tableNotFoundError(keyspaceName, tableName)
	}

	if _, ok := keyspaceMetadata.tablesInvalidated[tableName]; !ok {
		return nil, false, tableNotFoundError(keyspaceName, tableName)
	}

	return nil, true, nil
}

func (s *metadataDescriber) GetTable(keyspaceName, tableName string) (*TableMetadata, error) {
	keyspaceMetadata, wasReloaded, err := s.getKeyspaceInternal(keyspaceName)
	if err != nil {
		return nil, err
	}

	tableMetadata, refreshNeeded, err := s.getTableFromSnapshot(keyspaceName, tableName, keyspaceMetadata, wasReloaded)
	if err != nil {
		return nil, err
	}
	if !refreshNeeded {
		return tableMetadata, nil
	}

	err = s.deduplicatedRefreshTable(keyspaceName, tableName)
	if err != nil {
		return nil, err
	}

	keyspaceMetadata, found := s.metadata.keyspaceMetadata.getKeyspace(keyspaceName)
	if !found {
		return nil, tableNotFoundError(keyspaceName, tableName)
	}

	tableMetadata, found = keyspaceMetadata.Tables[tableName]
	if !found {
		return nil, tableNotFoundError(keyspaceName, tableName)
	}

	return tableMetadata, nil
}

func (s *metadataDescriber) getTablets() tablets.TabletInfoList {
	return s.metadata.tabletsMetadata.Get()
}

func (s *metadataDescriber) getTableTablets(keyspace, table string) tablets.TabletEntryList {
	return s.metadata.tabletsMetadata.GetTableTablets(keyspace, table)
}

func (s *metadataDescriber) forEachTablet(fn func(keyspace, table string, entries tablets.TabletEntryList) bool) {
	s.metadata.tabletsMetadata.ForEach(fn)
}

func (s *metadataDescriber) AddTablet(tablet tablets.TabletInfo) {
	s.metadata.tabletsMetadata.AddTablet(tablet)
}

// RemoveTabletsWithHost removes tablets that contains given host.
// to be used outside the metadataDescriber
func (s *metadataDescriber) RemoveTabletsWithHost(host *HostInfo) {
	s.metadata.tabletsMetadata.RemoveTabletsWithHost(tablets.HostUUID(host.hostUUID()))
}

// RemoveTabletsWithKeyspace removes tablets for given keyspace.
// to be used outside the metadataDescriber
func (s *metadataDescriber) RemoveTabletsWithKeyspace(keyspace string) {
	s.metadata.tabletsMetadata.RemoveTabletsWithKeyspace(keyspace)
}

// RemoveTabletsWithTable removes tablets for given table.
// to be used outside the metadataDescriber
func (s *metadataDescriber) RemoveTabletsWithTable(keyspace string, table string) {
	s.metadata.tabletsMetadata.RemoveTabletsWithTable(keyspace, table)
}

// invalidateKeyspaceSchema clears the cached keyspace metadata
func (s *metadataDescriber) invalidateKeyspaceSchema(keyspaceName string) {
	s.metadata.keyspaceMetadata.removeKeyspace(keyspaceName)
}

func (s *metadataDescriber) invalidateTableSchema(keyspaceName, tableName string) {
	s.metadata.keyspaceMetadata.invalidateTable(keyspaceName, tableName)
}

// deduplicatedRefreshKeyspace collapses concurrent refreshKeyspaceSchema calls
// for the same keyspace into a single in-flight operation.
func (s *metadataDescriber) deduplicatedRefreshKeyspace(keyspaceName string) error {
	_, err, _ := s.keyspaceGroup.Do(keyspaceName, func() (any, error) {
		return nil, s.refreshKeyspaceSchema(keyspaceName)
	})
	return err
}

// deduplicatedRefreshTable collapses concurrent refreshTableSchema calls
// for the same keyspace/table into a single in-flight operation.
func (s *metadataDescriber) deduplicatedRefreshTable(keyspaceName, tableName string) error {
	key := keyspaceName + "\x00" + tableName
	_, err, _ := s.tableGroup.Do(key, func() (any, error) {
		return nil, s.refreshTableSchema(keyspaceName, tableName)
	})
	return err
}

func (s *metadataDescriber) refreshAllSchema() error {
	// mu serialises concurrent refreshAllSchema calls so each one sees a
	// consistent snapshot before deciding what changed.  Individual keyspace
	// refreshes inside the loop go through singleflight, so two overlapping
	// refreshAllSchema calls will not duplicate network queries — the second
	// caller blocks on mu while the first finishes.
	s.mu.Lock()
	defer s.mu.Unlock()

	copiedMap := make(map[string]*KeyspaceMetadata)
	for key, value := range s.metadata.keyspaceMetadata.get() {
		if value != nil {
			copiedMap[key] = value.Clone()
		} else {
			copiedMap[key] = nil
		}
	}

	for keyspaceName, metadata := range copiedMap {
		// Route through singleflight to dedup concurrent refreshes.
		err := s.deduplicatedRefreshKeyspace(keyspaceName)
		if errors.Is(err, ErrKeyspaceDoesNotExist) {
			s.invalidateKeyspaceSchema(keyspaceName)
			s.RemoveTabletsWithKeyspace(keyspaceName)
			continue
		} else if err != nil {
			return err
		}

		updatedMetadata, err := s.GetKeyspace(keyspaceName)
		if err != nil {
			return err
		}

		if !compareInterfaceMaps(metadata.StrategyOptions, updatedMetadata.StrategyOptions) {
			s.RemoveTabletsWithKeyspace(keyspaceName)
			continue
		}

		for tableName, tableMetadata := range metadata.Tables {
			if updatedTableMetadata, ok := updatedMetadata.Tables[tableName]; !ok || !tableMetadata.Equals(updatedTableMetadata) {
				s.RemoveTabletsWithTable(keyspaceName, tableName)
			}
		}
	}
	return nil
}

// forcibly updates the current KeyspaceMetadata held by the schema describer
// for a given named keyspace.
//
// All system schema queries are issued concurrently since none of them
// depend on each other's results. The results are only combined in
// compileMetadata after all queries complete.
func (s *metadataDescriber) refreshKeyspaceSchema(keyspaceName string) error {
	var (
		keyspace    *KeyspaceMetadata
		tables      []TableMetadata
		columns     []ColumnMetadata
		functions   []FunctionMetadata
		aggregates  []AggregateMetadata
		types       []TypeMetadata
		indexes     []IndexMetadata
		views       []ViewMetadata
		createStmts []byte
	)

	// Each goroutine writes to its own dedicated variable, so no
	// synchronisation is needed beyond errgroup itself.
	var g errgroup.Group

	g.Go(func() error {
		var err error
		keyspace, err = getKeyspaceMetadata(s.session, keyspaceName)
		return err
	})
	g.Go(func() error {
		var err error
		tables, err = getTableMetadata(s.session, keyspaceName)
		return err
	})
	g.Go(func() error {
		var err error
		columns, err = getColumnMetadata(s.session, keyspaceName)
		return err
	})
	g.Go(func() error {
		var err error
		functions, err = getFunctionsMetadata(s.session, keyspaceName)
		return err
	})
	g.Go(func() error {
		var err error
		aggregates, err = getAggregatesMetadata(s.session, keyspaceName)
		return err
	})
	g.Go(func() error {
		var err error
		types, err = getTypeMetadata(s.session, keyspaceName)
		return err
	})
	g.Go(func() error {
		var err error
		indexes, err = getIndexMetadata(s.session, keyspaceName)
		return err
	})
	g.Go(func() error {
		var err error
		views, err = getViewMetadata(s.session, keyspaceName)
		return err
	})
	g.Go(func() error {
		var err error
		createStmts, err = getCreateStatements(s.session, keyspaceName)
		return err
	})

	if err := g.Wait(); err != nil {
		return err
	}

	compileMetadata(keyspace, tables, columns, functions, aggregates, types, indexes, views, createStmts)

	s.metadata.keyspaceMetadata.set(keyspaceName, keyspace)

	return nil
}

func (s *metadataDescriber) refreshTableSchema(keyspaceName, tableName string) error {
	_, found := s.metadata.keyspaceMetadata.getKeyspace(keyspaceName)
	if !found {
		return s.deduplicatedRefreshKeyspace(keyspaceName)
	}

	// Perform network queries outside the lock.
	tables, err := getTableMetadataByName(s.session, keyspaceName, tableName)
	if err != nil {
		return err
	}

	columns, err := getColumnMetadataByTable(s.session, keyspaceName, tableName)
	if err != nil {
		return err
	}

	indexes, err := getIndexMetadataByTable(s.session, keyspaceName, tableName)
	if err != nil {
		return err
	}

	views, err := getViewMetadataByTable(s.session, keyspaceName, tableName)
	if err != nil {
		return err
	}

	// Atomically clone-and-swap the keyspace metadata to avoid data races
	// with concurrent readers.
	applied := s.metadata.keyspaceMetadata.updateKeyspace(keyspaceName, func(ks *KeyspaceMetadata) {
		if len(tables) == 0 {
			ks.removeTable(tableName)
		} else {
			compileTableMetadata(ks, tables, columns, indexes, views)
			if ks.tablesInvalidated != nil {
				delete(ks.tablesInvalidated, tableName)
			}
		}
	})
	if !applied {
		// Keyspace was removed between the initial check and the update.
		// Fall back to a full keyspace refresh to recover.
		return s.deduplicatedRefreshKeyspace(keyspaceName)
	}
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

func compileTableMetadata(
	keyspace *KeyspaceMetadata,
	tables []TableMetadata,
	columns []ColumnMetadata,
	indexes []IndexMetadata,
	views []ViewMetadata,
) {
	if keyspace.Tables == nil {
		keyspace.Tables = make(map[string]*TableMetadata)
	}
	for i := range tables {
		tables[i].Columns = make(map[string]*ColumnMetadata)
		keyspace.Tables[tables[i].Name] = &tables[i]
	}

	if keyspace.Indexes == nil {
		keyspace.Indexes = make(map[string]*IndexMetadata)
	}
	for name, ix := range keyspace.Indexes {
		for i := range tables {
			if ix.TableName == tables[i].Name {
				delete(keyspace.Indexes, name)
			}
		}
	}
	for i := range indexes {
		indexes[i].Columns = make(map[string]*ColumnMetadata)
		keyspace.Indexes[indexes[i].Name] = &indexes[i]
	}

	if keyspace.Views == nil {
		keyspace.Views = make(map[string]*ViewMetadata)
	}
	for name, v := range keyspace.Views {
		for i := range tables {
			if v.BaseTableName == tables[i].Name {
				delete(keyspace.Views, name)
			}
		}
	}
	for i := range views {
		v := &views[i]
		if _, ok := keyspace.Indexes[strings.TrimSuffix(v.ViewName, "_index")]; ok {
			continue
		}
		v.Columns = make(map[string]*ColumnMetadata)
		keyspace.Views[v.ViewName] = v
	}

	for i := range columns {
		col := &columns[i]
		col.Order = ASC
		if col.ClusteringOrder == "desc" {
			col.Order = DESC
		}

		table, ok := keyspace.Tables[col.Table]
		if !ok {
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

	const stmt = `SELECT durable_writes, replication FROM system_schema.keyspaces WHERE keyspace_name = ?`

	var replication map[string]string

	iter := session.control.querySystem(stmt, keyspaceName)
	if iter.NumRows() == 0 {
		iter.Close()
		return nil, ErrKeyspaceDoesNotExist
	}
	iter.Scan(&keyspace.DurableWrites, &replication)
	err := iter.Close()
	if err != nil {
		return nil, fmt.Errorf("error querying keyspace schema: %v", err)
	}

	keyspace.StrategyClass = replication["class"]
	delete(replication, "class")

	keyspace.StrategyOptions = make(map[string]any, len(replication))
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
	for iter.MapScan(map[string]any{
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

	conn := session.getConn()
	if conn == nil || !conn.isScyllaConn() {
		return tables, nil
	}

	// Fetch all ScyllaDB-specific table properties in a single query
	// instead of issuing one query per table (N+1 elimination).
	stmt = `SELECT * FROM system_schema.scylla_tables WHERE keyspace_name = ?`
	iter = session.control.querySystem(stmt, keyspaceName)

	scyllaOpts := make(map[string]TableMetadataOptions, len(tables))
	var opts TableMetadataOptions
	var tblName string
	for iter.MapScan(map[string]any{
		"table_name":  &tblName,
		"cdc":         &opts.CDC,
		"in_memory":   &opts.InMemory,
		"partitioner": &opts.Partitioner,
		"version":     &opts.Version,
	}) {
		scyllaOpts[tblName] = opts
		opts = TableMetadataOptions{}
		tblName = ""
	}
	if err := iter.Close(); err != nil && err != ErrNotFound {
		return nil, fmt.Errorf("error querying scylla table schema: %v", err)
	}

	for i, t := range tables {
		if sopts, ok := scyllaOpts[t.Name]; ok {
			tables[i].Options.CDC = sopts.CDC
			tables[i].Options.InMemory = sopts.InMemory
			tables[i].Options.Partitioner = sopts.Partitioner
			tables[i].Options.Version = sopts.Version
		}
	}

	return tables, nil
}

func getTableMetadataByName(session *Session, keyspaceName, tableName string) ([]TableMetadata, error) {
	if !session.useSystemSchema {
		return nil, nil
	}

	stmt := `SELECT * FROM system_schema.tables WHERE keyspace_name = ? AND table_name = ?`
	iter := session.control.querySystem(stmt, keyspaceName, tableName)

	var tables []TableMetadata
	table := TableMetadata{Keyspace: keyspaceName}
	for iter.MapScan(map[string]any{
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
		return nil, fmt.Errorf("error querying table schema: %w", err)
	}

	if conn := session.getConn(); conn == nil || !conn.isScyllaConn() {
		return tables, nil
	}

	stmt = `SELECT * FROM system_schema.scylla_tables WHERE keyspace_name = ? AND table_name = ?`
	for i, t := range tables {
		iter := session.control.querySystem(stmt, keyspaceName, t.Name)

		table := TableMetadata{}
		if iter.MapScan(map[string]any{
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
			return nil, fmt.Errorf("error querying scylla table schema: %w", err)
		}
	}

	return tables, nil
}

// columnMetadataColumns lists the columns consumed by getColumnMetadata and
// getColumnMetadataByTable. Selecting only these columns (instead of SELECT *)
// avoids deserializing unused fields such as keyspace_name (already known from
// the WHERE clause) and column_name_bytes (ScyllaDB-specific), which can add
// over 50 KB of wasted payload per keyspace with 80+ tables.
const columnMetadataColumns = `table_name, column_name, clustering_order, type, kind, position`

func getColumnMetadataByTable(session *Session, keyspaceName, tableName string) ([]ColumnMetadata, error) {
	const stmt = `SELECT ` + columnMetadataColumns + ` FROM system_schema.columns WHERE keyspace_name = ? AND table_name = ?`

	var columns []ColumnMetadata

	iter := session.control.querySystem(stmt, keyspaceName, tableName)
	column := ColumnMetadata{Keyspace: keyspaceName}

	for iter.MapScan(map[string]any{
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
		return nil, fmt.Errorf("error querying column schema: %w", err)
	}

	return columns, nil
}

func getIndexMetadataByTable(session *Session, keyspaceName, tableName string) ([]IndexMetadata, error) {
	if !session.useSystemSchema {
		return nil, nil
	}

	const stmt = `SELECT * FROM system_schema.indexes WHERE keyspace_name = ? AND table_name = ?`

	var indexes []IndexMetadata
	index := IndexMetadata{}

	iter := session.control.querySystem(stmt, keyspaceName, tableName)
	for iter.MapScan(map[string]any{
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

func getViewMetadataByTable(session *Session, keyspaceName, tableName string) ([]ViewMetadata, error) {
	if !session.useSystemSchema {
		return nil, nil
	}

	stmt := `SELECT * FROM system_schema.views WHERE keyspace_name = ? AND base_table_name = ? ALLOW FILTERING`

	iter := session.control.querySystem(stmt, keyspaceName, tableName)

	var views []ViewMetadata
	view := ViewMetadata{KeyspaceName: keyspaceName}

	for iter.MapScan(map[string]any{
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
		return nil, fmt.Errorf("error querying view schema: %w", err)
	}

	return views, nil
}

// query for column metadata in the system_schema.columns
func getColumnMetadata(session *Session, keyspaceName string) ([]ColumnMetadata, error) {
	const stmt = `SELECT ` + columnMetadataColumns + ` FROM system_schema.columns WHERE keyspace_name = ?`

	var columns []ColumnMetadata

	iter := session.control.querySystem(stmt, keyspaceName)
	column := ColumnMetadata{Keyspace: keyspaceName}

	for iter.MapScan(map[string]any{
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

	for iter.MapScan(map[string]any{
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
	for iter.MapScan(map[string]any{
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
	for iter.MapScan(map[string]any{
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
	for iter.MapScan(map[string]any{
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

	for iter.MapScan(map[string]any{
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
