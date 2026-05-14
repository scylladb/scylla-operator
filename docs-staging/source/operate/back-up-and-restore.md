# Back up and restore

ScyllaDB Operator supports automated backup and restore through [ScyllaDB Manager](../understand/manager.md).
The Manager Agent sidecar on each ScyllaDB pod uploads snapshots to object storage (Amazon S3 or Google Cloud Storage).

## Guides

- [Schedule backups](schedule-backups.md) — Configure credentials, schedule automated backup tasks, and list snapshots.
- [Restore from backup](restore-from-backup.md) — Restore a ScyllaDB cluster from a Manager backup snapshot.
