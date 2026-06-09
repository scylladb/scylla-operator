# Configure dual-stack networking with IPv6

**What you'll achieve**: Deploy a ScyllaDB cluster that uses IPv6 for ScyllaDB communication and provides services accessible via both IPv6 and IPv4.

**Before you begin**: 
- You have a Kubernetes cluster with dual-stack networking enabled
- You have ScyllaDB Operator installed
- You have `kubectl` configured

**After completion**: Your ScyllaDB cluster will use IPv6 for inter-node communication while services are accessible via both IPv6 and IPv4.

:::{note}
This configuration is production-ready. For IPv4-first dual-stack, see [Configure dual-stack with IPv4](ipv6-configure.md).
:::

## Step 1: Create the configuration

Create a file `scylla-ipv6-first.yaml`:

```yaml
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
metadata:
  name: scylla-ipv6-first
  namespace: scylla
spec:
  version: 6.2.2
  agentVersion: 3.3.3
  datacenter:
    name: datacenter1
    racks:
      - name: rack1
        members: 3
        storage:
          capacity: 100Gi
          storageClassName: scylladb-local-xfs
        resources:
          limits:
            cpu: 4
            memory: 16Gi
          requests:
            cpu: 4
            memory: 16Gi
  
  network:
    ipFamilyPolicy: PreferDualStack
    ipFamilies:
      - IPv6  # ScyllaDB uses IPv6
      - IPv4  # Services also support IPv4
    dnsPolicy: ClusterFirst
  
  exposeOptions:
    nodeService:
      type: Headless
    broadcastOptions:
      nodes:
        type: PodIP
        podIP:
          source: Status
      clients:
        type: ServiceClusterIP
```

## Step 2: Apply the configuration

```bash
kubectl create namespace scylla
kubectl apply -f scylla-ipv6-first.yaml
```

## Step 3: Wait for the cluster to be ready

Monitor pod creation:

```bash
kubectl get pods -n scylla -l scylla-operator.scylladb.com/pod-type=scylladb-node -w
```

Wait until all pods show `Running` status.

## Step 4: Verify dual-stack configuration

Check that services have both IP families:

```bash
kubectl get svc -n scylla -o custom-columns=NAME:.metadata.name,IP-FAMILIES:.spec.ipFamilies,POLICY:.spec.ipFamilyPolicy
```

Expected output shows `[IPv6 IPv4]` for IP families:

```
NAME                              IP-FAMILIES        POLICY
scylla-ipv6-first-client          [IPv6 IPv4]        PreferDualStack
scylla-ipv6-first-datacenter1-... [IPv6 IPv4]        PreferDualStack
```

## Step 5: Verify cluster uses IPv6

Check that ScyllaDB is using IPv6 addresses:

```bash
NAMESPACE=scylla
CLUSTER_NAME=scylla-ipv6-first

pods=$(kubectl -n "${NAMESPACE}" get pods -l scylla/cluster="${CLUSTER_NAME}" -l scylla-operator.scylladb.com/pod-type=scylladb-node -o name)

for pod in ${pods}; do
  kubectl -n "${NAMESPACE}" exec "${pod}" -c scylla -- nodetool status
done
```

Expected output shows IPv6 addresses (with colons):

```
Datacenter: datacenter1
=======================
Status=Up/Down
|/ State=Normal/Leaving/Joining/Moving
--  Address              Load      Tokens  Owns  Host ID                              Rack
UN  fd00:10:244:1::7f    501.79 KB 256     ?     4583fff5-2aa6-4041-9be8-c74bcabaff8c rack1
UN  fd00:10:244:2::6d    494.49 KB 256     ?     b1f889b4-80e7-4685-a3c5-1b81797c2ce4 rack1
UN  fd00:10:244:3::6c    494.96 KB 256     ?     7a4bb6da-415e-4fc3-a6ca-0369c0e76bf0 rack1
```

## Next steps

- [Migrate existing clusters to IPv6](ipv6-migrate.md)
- [Troubleshoot IPv6 issues](ipv6-troubleshoot.md)
