# Back up and restore

ScyllaDB Operator supports automated backup and restore through [ScyllaDB Manager](https://operator.docs.scylladb.com/v1.21/understand/manager.md) using the ScyllaDBManagerTask CRD.
The Manager Agent sidecar on each ScyllaDB pod uploads snapshots to a [supported backup destination](https://manager.docs.scylladb.com/stable/backup/index.html).

## Guides

- [Restore from backup](https://operator.docs.scylladb.com/v1.21/operate/restore-from-backup.md) — Restore a ScyllaDB cluster from a Manager backup snapshot.
