# Scylla Manager CRD

Scylla Manager can be created and configured using the `scyllamanagerss.scylla.scylladb.com` custom resource definition (CRD).

## Sample
```yaml
apiVersion: scylla.scylladb.com/v1alpha1
kind: ScyllaManager
metadata:
  name: scylla-manager
  namespace: example-manager
spec:
  replicas: 1
  image: docker.io/scylladb/scylla-manager:3.0.0
  taskStatusRefreshTime: 300000000000 # 300 seconds
  scyllaClusterSelector:
    matchLabels:
      example-manager: "true"
      managed: "true"
      managed-by: scylla-manager

  database:
    connection:
      server: scylla-manager-cluster-manager-dc-manager-rack-0.example-manager.svc
      username: cassandra
      password: cassandra

  repairs:
    - name: "weekly us-east-1 repair"
      intensity: "2"
      cron: "@weekly"
      dc: ["us-east-1"]
  backups:
    - name: "daily users backup"
      rateLimit: ["50"]
      location: ["s3:cluster-backups"]
      cron: "@daily"
      keyspace: ["users"]
    - name: "weekly full cluster backup"
      rateLimit: ["50"]
      location: ["s3:cluster-backups"]
      cron: "@weekly"
```