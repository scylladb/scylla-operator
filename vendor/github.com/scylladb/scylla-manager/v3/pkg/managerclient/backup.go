// Copyright (C) 2017 ScyllaDB

package managerclient

// Stage enumeration.
const (
	BackupStageInit         string = "INIT"
	BackupStageSnapshot     string = "SNAPSHOT"
	BackupStageAwaitSchema  string = "AWAIT_SCHEMA"
	BackupStageIndex        string = "INDEX"
	BackupStageManifest     string = "MANIFEST"
	BackupStageSchema       string = "SCHEMA"
	BackupStageDeduplicate  string = "DEDUPLICATE"
	BackupStageUpload       string = "UPLOAD"
	BackupStageMoveManifest string = "MOVE_MANIFEST"
	BackupStageMigrate      string = "MIGRATE"
	BackupStagePurge        string = "PURGE"
	BackupStageDone         string = "DONE"
)

var backupStageName = map[string]string{
	BackupStageInit:         "initialising",
	BackupStageSnapshot:     "taking snapshot",
	BackupStageAwaitSchema:  "awaiting schema agreement",
	BackupStageIndex:        "indexing files",
	BackupStageManifest:     "uploading manifests",
	BackupStageSchema:       "uploading schema",
	BackupStageDeduplicate:  "deduplicating the snapshot",
	BackupStageUpload:       "uploading data",
	BackupStageMoveManifest: "moving manifests",
	BackupStageMigrate:      "migrating legacy metadata",
	BackupStagePurge:        "retention",
	BackupStageDone:         "",
}

// BackupStageName returns verbose name for backup stage.
func BackupStageName(s string) string {
	return backupStageName[s]
}
