# Maintenance mode

When maintenance mode is enabled, readiness probe of ScyllaDB Pod will always return failure and liveness probe will always succeed. This causes that Pod under maintenance
is being removed from K8s Load Balancer and DNS registry but Pod itself stays alive.

This allows the ScyllaDB Operator to interact with ScyllaDB and ScyllaDB dependencies inside the Pod.
For example user may turn off ScyllaDB process, do something with the filesystem and bring the process back again.

To enable maintenance mode add `scylla/node-maintenance` label to service in front of ScyllaDB Pod.

```bash
kubectl -n scylla label svc simple-cluster-us-east1-b-us-east1-2 scylla/node-maintenance=""
```

To disable, simply remove this label from service.

```bash
kubectl -n scylla label svc simple-cluster-us-east1-b-us-east1-2 scylla/node-maintenance-
```
