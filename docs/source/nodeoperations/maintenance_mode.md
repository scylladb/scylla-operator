### Maintenance mode

When maintenance mode is enabled, readiness probe of Scylla Pod will always return failure and liveness probe will always succeed. This causes that Pod under maintenance
is being removed from K8s Load Balancer and DNS registry but Pod itself stays alive.

This allows the Scylla Operator to interact with Scylla and Scylla dependencies inside the Pod.
For example user may turn off Scylla process, do something with the filesystem and bring the process back again.

To enable maintenance mode add `scylla/node-maintenance` label to service in front of Scylla Pod.

```bash
kubectl -n scylla label svc simple-cluster-us-east1-b-us-east1-2 scylla/node-maintenance=""
```

To disable, simply remove this label from service.

```bash
kubectl -n scylla label svc simple-cluster-us-east1-b-us-east1-2 scylla/node-maintenance-
```
