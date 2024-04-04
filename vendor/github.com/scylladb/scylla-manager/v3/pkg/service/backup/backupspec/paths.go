// Copyright (C) 2017 ScyllaDB

package backupspec

import (
	"os"
	"path"
	"strings"

	"github.com/scylladb/scylla-manager/v3/pkg/util/uuid"
)

const (
	// MetadataVersion is the suffix for version file.
	MetadataVersion = ".version"
	// Manifest is name of the manifest file.
	Manifest = "manifest.json.gz"
	// Schema is the name of the schema file.
	Schema = "schema.tar.gz"
	// TempFileExt is suffix for the temporary files.
	TempFileExt = ".tmp"

	sep = string(os.PathSeparator)
)

type dirKind string

// Enumeration of dirKinds.
const (
	SchemaDirKind = dirKind("schema")
	SSTDirKind    = dirKind("sst")
	MetaDirKind   = dirKind("meta")
)

// RemoteManifestLevel calculates maximal depth of recursive listing starting at
// baseDir to list all manifests.
func RemoteManifestLevel(baseDir string) int {
	a := len(strings.Split(remoteManifestDir(uuid.Nil, "a", "b"), sep))
	b := len(strings.Split(baseDir, sep))
	return a - b
}

// RemoteManifestFile returns path to the manifest file.
func RemoteManifestFile(clusterID, taskID uuid.UUID, snapshotTag, dc, nodeID string) string {
	manifestName := strings.Join([]string{
		"task",
		taskID.String(),
		"tag",
		snapshotTag,
		Manifest,
	}, "_")

	return path.Join(
		remoteManifestDir(clusterID, dc, nodeID),
		manifestName,
	)
}

func remoteManifestDir(clusterID uuid.UUID, dc, nodeID string) string {
	return path.Join(
		"backup",
		string(MetaDirKind),
		"cluster",
		clusterID.String(),
		"dc",
		dc,
		"node",
		nodeID,
	)
}

// RemoteMetaClusterDCDir returns path to DC dir for the provided cluster.
func RemoteMetaClusterDCDir(clusterID uuid.UUID) string {
	return path.Join(
		"backup",
		string(MetaDirKind),
		"cluster",
		clusterID.String(),
		"dc",
	)
}

// RemoteSchemaFile returns path to the schema file.
func RemoteSchemaFile(clusterID, taskID uuid.UUID, snapshotTag string) string {
	manifestName := strings.Join([]string{
		"task",
		taskID.String(),
		"tag",
		snapshotTag,
		Schema,
	}, "_")

	return path.Join(
		remoteSchemaDir(clusterID),
		manifestName,
	)
}

func remoteSchemaDir(clusterID uuid.UUID) string {
	return path.Join(
		"backup",
		string(SchemaDirKind),
		"cluster",
		clusterID.String(),
	)
}

// RemoteSSTableVersionDir returns path to the sstable version directory.
func RemoteSSTableVersionDir(clusterID uuid.UUID, dc, nodeID, keyspace, table, version string) string {
	return path.Join(
		RemoteSSTableDir(clusterID, dc, nodeID, keyspace, table),
		version,
	)
}

// RemoteSSTableBaseDir returns path to the sstable base directory.
func RemoteSSTableBaseDir(clusterID uuid.UUID, dc, nodeID string) string {
	return path.Join(
		"backup",
		string(SSTDirKind),
		"cluster",
		clusterID.String(),
		"dc",
		dc,
		"node",
		nodeID,
	)
}

// RemoteSSTableDir returns path to given table's sstable directory.
func RemoteSSTableDir(clusterID uuid.UUID, dc, nodeID, keyspace, table string) string {
	return path.Join(
		RemoteSSTableBaseDir(clusterID, dc, nodeID),
		"keyspace",
		keyspace,
		"table",
		table,
	)
}

// TempFile returns temporary path for the provided file.
func TempFile(f string) string {
	return f + TempFileExt
}

const (
	// DataDir is the data dir prefix.
	DataDir = "data:"

	// ScyllaManifest defines the name of backup manifest file.
	ScyllaManifest = "manifest.json"
	// ScyllaSchema defines the name of backup CQL schema file.
	ScyllaSchema = "schema.cql"
)

// KeyspaceDir return keyspace directory.
func KeyspaceDir(keyspace string) string {
	return DataDir + keyspace
}

// UploadTableDir returns table upload directory.
func UploadTableDir(keyspace, table, version string) string {
	return path.Join(
		KeyspaceDir(keyspace),
		table+"-"+version,
		"upload",
	)
}
