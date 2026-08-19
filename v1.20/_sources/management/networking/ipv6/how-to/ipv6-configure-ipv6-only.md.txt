# Configure IPv6-only networking

**What you'll achieve**: Deploy a ScyllaDB cluster that uses only IPv6 for all communication.

**Before you begin**: 
- You have a Kubernetes cluster with IPv6 support
- You have ScyllaDB Operator installed
- You have `kubectl` configured
- Your cluster and applications support IPv6-only operation

**After completion**: Your ScyllaDB cluster will use only IPv6 for all communication.

:::{warning}
**Experimental Feature**: IPv6-only configurations are experimental. See [Production readiness](../reference/ipv6-configuration.md#production-readiness) for details. For production deployments, use [dual-stack with IPv4](ipv6-configure.md) or [dual-stack with IPv6](ipv6-configure-ipv6-first.md) instead.
:::

## Step 1: Apply the configuration

Apply the IPv6-only configuration:

```bash
kubectl create namespace scylla
kubectl apply -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/examples/ipv6/scylla-cluster-ipv6.yaml
```

## Step 2: Wait for the cluster to be ready

Monitor pod creation:

```bash
kubectl get pods -n scylla -l scylla-operator.scylladb.com/pod-type=scylladb-node -w
```

Wait until all pods show `Running` status.

## Step 3: Verify IPv6-only configuration

Check that pods have IPv6 addresses:

```bash
kubectl get pods -n scylla -l scylla-operator.scylladb.com/pod-type=scylladb-node -o wide
```

Expected output shows IPv6 addresses in the IP column:

```
NAME                       READY   STATUS    RESTARTS   AGE   IP                NODE
scylla-ipv6-datacenter-... 2/2     Running   0          5m    fd00:10:244:1::7f scylla-worker-1
scylla-ipv6-datacenter-... 2/2     Running   0          4m    fd00:10:244:2::6d scylla-worker-2
scylla-ipv6-datacenter-... 2/2     Running   0          3m    fd00:10:244:3::6c scylla-worker-3
```

## Step 4: Verify cluster health

Check that all nodes are up:

```bash
NAMESPACE=scylla
CLUSTER_NAME=scylla-ipv6

pods=$(kubectl -n "${NAMESPACE}" get pods -l scylla/cluster="${CLUSTER_NAME}" -l scylla-operator.scylladb.com/pod-type=scylladb-node -o name)

for pod in ${pods}; do
  kubectl -n "${NAMESPACE}" exec "${pod}" -c scylla -- nodetool status
done
```

Expected output shows IPv6 addresses:

```
Datacenter: datacenter
======================
Status=Up/Down
|/ State=Normal/Leaving/Joining/Moving
--  Address              Load      Tokens  Owns  Host ID                              Rack
UN  fd00:10:244:1::7f    501.79 KB 256     ?     4583fff5-2aa6-4041-9be8-c74bcabaff8c rack
UN  fd00:10:244:2::6d    494.49 KB 256     ?     b1f889b4-80e7-4685-a3c5-1b81797c2ce4 rack
UN  fd00:10:244:3::6c    494.96 KB 256     ?     7a4bb6da-415e-4fc3-a6ca-0369c0e76bf0 rack
```

## Next steps

- [Troubleshoot IPv6 issues](ipv6-troubleshoot.md)
