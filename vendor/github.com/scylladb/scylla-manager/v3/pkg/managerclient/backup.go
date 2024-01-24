// Copyright (C) 2017 ScyllaDB

package managerclient

// Stage enumeration.
const (
	BackupStageInit         string = "INIT"
	BackupStageAwaitSchema  string = "AWAIT_SCHEMA"
	BackupStageSnapshot     string = "SNAPSHOT"
	BackupStageIndex        string = "INDEX"
	BackupStageManifest     string = "MANIFEST"
	BackupStageSchema       string = "SCHEMA"
	BackupStageUpload       string = "UPLOAD"
	BackupStageMoveManifest string = "MOVE_MANIFEST"
	BackupStageMigrate      string = "MIGRATE"
	BackupStagePurge        string = "PURGE"
	BackupStageDone         string = "DONE"
)

var backupStageName = map[string]string{
	BackupStageInit:         "initialising",
	BackupStageAwaitSchema:  "awaiting schema agreement",
	BackupStageSnapshot:     "taking snapshot",
	BackupStageIndex:        "indexing files",
	BackupStageManifest:     "uploading manifests",
	BackupStageSchema:       "uploading schema",
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
