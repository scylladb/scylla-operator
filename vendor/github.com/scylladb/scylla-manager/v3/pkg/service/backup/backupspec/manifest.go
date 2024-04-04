// Copyright (C) 2017 ScyllaDB

package backupspec

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path"
	"runtime"
	"strings"

	jsoniter "github.com/json-iterator/go"
	"github.com/pkg/errors"
	"github.com/scylladb/scylla-manager/v3/pkg/util/inexlist/ksfilter"
	"github.com/scylladb/scylla-manager/v3/pkg/util/pathparser"
	"github.com/scylladb/scylla-manager/v3/pkg/util/uuid"
	"go.uber.org/multierr"
)

// ManifestInfo represents manifest on remote location.
type ManifestInfo struct {
	Location    Location
	DC          string
	ClusterID   uuid.UUID
	NodeID      string
	TaskID      uuid.UUID
	SnapshotTag string
	Temporary   bool
}

// Path returns path to the file that manifest points to.
func (m *ManifestInfo) Path() string {
	f := RemoteManifestFile(m.ClusterID, m.TaskID, m.SnapshotTag, m.DC, m.NodeID)
	if m.Temporary {
		f = TempFile(f)
	}
	return f
}

// SchemaPath returns path to the schema file that manifest points to.
func (m *ManifestInfo) SchemaPath() string {
	return RemoteSchemaFile(m.ClusterID, m.TaskID, m.SnapshotTag)
}

// SSTableVersionDir returns path to the sstable version directory.
func (m *ManifestInfo) SSTableVersionDir(keyspace, table, version string) string {
	return RemoteSSTableVersionDir(m.ClusterID, m.DC, m.NodeID, keyspace, table, version)
}

// LocationSSTableVersionDir returns path to the sstable version directory with location remote path prefix.
func (m *ManifestInfo) LocationSSTableVersionDir(keyspace, table, version string) string {
	return m.Location.RemotePath(RemoteSSTableVersionDir(m.ClusterID, m.DC, m.NodeID, keyspace, table, version))
}

// ParsePath extracts properties from full remote path to manifest.
func (m *ManifestInfo) ParsePath(s string) error {
	// Clear values
	*m = ManifestInfo{}

	// Clean path for usage with strings.Split
	s = strings.TrimPrefix(path.Clean(s), sep)

	parsers := []pathparser.Parser{
		pathparser.Static("backup"),
		pathparser.Static(string(MetaDirKind)),
		pathparser.Static("cluster"),
		pathparser.ID(&m.ClusterID),
		pathparser.Static("dc"),
		pathparser.String(&m.DC),
		pathparser.Static("node"),
		pathparser.String(&m.NodeID),
		m.fileNameParser,
	}
	n, err := pathparser.New(s, sep).Parse(parsers...)
	if err != nil {
		return err
	}
	if n < len(parsers) {
		return errors.Errorf("no input at position %d", n)
	}

	m.Temporary = strings.HasSuffix(s, TempFileExt)

	return nil
}

func (m *ManifestInfo) fileNameParser(v string) error {
	parsers := []pathparser.Parser{
		pathparser.Static("task"),
		pathparser.ID(&m.TaskID),
		pathparser.Static("tag"),
		pathparser.Static("sm"),
		func(v string) error {
			tag := "sm_" + v
			if !IsSnapshotTag(tag) {
				return errors.Errorf("invalid snapshot tag %s", tag)
			}
			m.SnapshotTag = tag
			return nil
		},
		pathparser.Static(Manifest, TempFile(Manifest)),
	}

	n, err := pathparser.New(v, "_").Parse(parsers...)
	if err != nil {
		return err
	}
	if n < len(parsers) {
		return errors.Errorf("input too short")
	}
	return nil
}

// ManifestContent is structure containing information about the backup.
type ManifestContent struct {
	Version     string  `json:"version"`
	ClusterName string  `json:"cluster_name"`
	IP          string  `json:"ip"`
	Size        int64   `json:"size"`
	Tokens      []int64 `json:"tokens"`
	Schema      string  `json:"schema"`
}

// ManifestContentWithIndex is structure containing information about the backup
// and the index.
type ManifestContentWithIndex struct {
	ManifestContent
	Index []FilesMeta `json:"index"`

	indexFile string
}

// Read loads the ManifestContent from JSON and tees the Index to a file.
func (m *ManifestContentWithIndex) Read(r io.Reader) error {
	f, err := os.CreateTemp(os.TempDir(), "manifestIndex")
	if err != nil {
		return err
	}

	defer f.Close()

	m.indexFile = f.Name()

	runtime.SetFinalizer(m, func(m *ManifestContentWithIndex) {
		os.Remove(m.indexFile) // nolint: errcheck
	})

	gr, err := gzip.NewReader(io.TeeReader(r, f))
	if err != nil {
		return err
	}

	if err := json.NewDecoder(gr).Decode(&m.ManifestContent); err != nil {
		return err
	}
	return gr.Close()
}

// Write writes the ManifestContentWithIndex as compressed JSON.
func (m *ManifestContentWithIndex) Write(w io.Writer) error {
	gw := gzip.NewWriter(w)

	if err := json.NewEncoder(gw).Encode(m); err != nil {
		return err
	}

	return gw.Close()
}

// ReadIndex loads the index from the indexfile into the struct.
func (m *ManifestContentWithIndex) ReadIndex() ([]FilesMeta, error) {
	if m.indexFile == "" {
		return nil, errors.New("index file not set, did not perform a successful Read")
	}

	f, err := os.Open(m.indexFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}

	tempM := new(ManifestContentWithIndex)
	dec := json.NewDecoder(gr)
	err = dec.Decode(&tempM)

	return tempM.Index, err
}

// LoadIndex loads the entire index into memory so that it can be filtered or marshalled.
func (m *ManifestContentWithIndex) LoadIndex() (err error) {
	m.Index, err = m.ReadIndex()
	return
}

// IndexLength reads the indexes from the Indexfile and returns the length.
func (m *ManifestContentWithIndex) IndexLength() (n int, err error) {
	if m.Index != nil {
		n = len(m.Index)
		return
	}

	err = m.ForEachIndexIter(nil, func(fm FilesMeta) {
		n++
	})
	return
}

// ForEachIndexIterWithError streams the indexes from the Manifest JSON, filters them and performs a
// callback on each as they are read in. It stops iteration after callback returns an error.
func (m *ManifestContentWithIndex) ForEachIndexIterWithError(keyspace []string, cb func(fm FilesMeta) error) (err error) {
	f, err := os.Open(m.indexFile)
	if err != nil {
		return err
	}
	defer func() {
		err = multierr.Append(err, f.Close())
	}()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}

	filter, err := ksfilter.NewFilter(keyspace)
	if err != nil {
		return errors.Wrap(err, "create filter")
	}

	iter := jsoniter.Parse(jsoniter.ConfigDefault, gr, 1024)

	for k := iter.ReadObject(); iter.Error == nil; k = iter.ReadObject() {
		if k != "index" {
			iter.Skip()
			continue
		}

		iter.ReadArrayCB(func(it *jsoniter.Iterator) bool {
			var m FilesMeta
			it.ReadVal(&m)
			if filter.Check(m.Keyspace, m.Table) {
				err = cb(m)
			}
			return err == nil
		})
		break
	}

	return multierr.Append(iter.Error, err)
}

// ForEachIndexIter is a wrapper for ForEachIndexIterWithError
// that takes callback which doesn't return an error.
func (m *ManifestContentWithIndex) ForEachIndexIter(keyspace []string, cb func(fm FilesMeta)) error {
	return m.ForEachIndexIterWithError(keyspace, func(fm FilesMeta) error {
		cb(fm)
		return nil
	})
}

// ForEachIndexIterFiles performs an action for each filtered file in the index.
func (m *ManifestContentWithIndex) ForEachIndexIterFiles(keyspace []string, mi *ManifestInfo, cb func(dir string, files []string)) error {
	return m.ForEachIndexIter(keyspace, func(fm FilesMeta) {
		dir := RemoteSSTableVersionDir(mi.ClusterID, mi.DC, mi.NodeID, fm.Keyspace, fm.Table, fm.Version)
		cb(dir, fm.Files)
	})
}

// ManifestInfoWithContent is intended for passing manifest with its content.
type ManifestInfoWithContent struct {
	*ManifestInfo
	*ManifestContentWithIndex
}

func NewManifestInfoWithContent() ManifestInfoWithContent {
	return ManifestInfoWithContent{
		ManifestInfo:             new(ManifestInfo),
		ManifestContentWithIndex: new(ManifestContentWithIndex),
	}
}

// FilesInfo specifies paths to files backed up for a table (and node) within
// a location.
// Note that a backup for a table usually consists of multiple instances of
// FilesInfo since data is replicated across many nodes.
type FilesInfo struct {
	Location Location    `json:"location"`
	Schema   string      `json:"schema"`
	Files    []FilesMeta `json:"files"`
}

// FilesMeta contains information about SST files of particular keyspace/table.
type FilesMeta struct {
	Keyspace string   `json:"keyspace"`
	Table    string   `json:"table"`
	Version  string   `json:"version"`
	Files    []string `json:"files"`
	Size     int64    `json:"size"`

	Path string `json:"path,omitempty"`
}
