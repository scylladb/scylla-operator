# Configure IPv6-only single-stack

Deploy a ScyllaDB cluster that uses only IPv6 for all communication.

:::{warning}
IPv6-only single-stack is **experimental**.
It works but has not undergone the same level of testing and validation as dual-stack configurations.
For production deployments, use [dual-stack networking](configure-dual-stack.md) instead.

Track progress: [#3211](https://github.com/scylladb/scylla-operator/issues/3211).
:::

## Prerequisites

| Requirement | Details |
|---|---|
| Kubernetes cluster | IPv6 networking enabled; nodes must have IPv6 `InternalIP` addresses |
| ScyllaDB Operator | Already installed ([Installation](../../../install-operator/index.md)) |
| ScyllaDB version | 2024.1 or newer recommended |
| Applications | All clients must support IPv6 — there is no IPv4 fallback in this configuration |

## Apply the configuration

Apply the IPv6-only example:

```bash
kubectl create namespace scylla
kubectl apply --server-side -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/examples/ipv6/scylla-cluster-ipv6.yaml
```

The network section of the manifest:

```yaml
network:
  dnsPolicy: ClusterFirst
  ipFamilyPolicy: SingleStack
  ipFamilies:
    - IPv6

exposeOptions:
  broadcastOptions:
    nodes:
      type: PodIP
      podIP:
        source: Status
    clients:
      type: PodIP
      podIP:
        source: Status
```

Key differences from dual-stack:

| Setting | Dual-stack | IPv6-only |
|---|---|---|
| `ipFamilyPolicy` | `PreferDualStack` | `SingleStack` |
| `ipFamilies` | `[IPv6, IPv4]` or `[IPv4, IPv6]` | `[IPv6]` |
| Service addresses | Both IPv4 and IPv6 | IPv6 only |
| Client requirements | Either protocol | IPv6 required |

## Wait for the cluster to be ready

```bash
kubectl -n scylla get pods -l scylla-operator.scylladb.com/pod-type=scylladb-node -w
```

Wait until all pods show `Running` status.

## Verify IPv6-only Services

```bash
kubectl -n scylla get svc -o custom-columns=NAME:.metadata.name,IP-FAMILIES:.spec.ipFamilies,POLICY:.spec.ipFamilyPolicy
```

**Expected output:**

```
NAME                                                IP-FAMILIES   POLICY
scylla-ipv6-example-client                          [IPv6]        SingleStack
scylla-ipv6-example-ipv6-datacenter-ipv6-rack-a-0   [IPv6]        SingleStack
```

## Verify cluster health

```bash
kubectl -n scylla exec -it scylla-ipv6-example-ipv6-datacenter-ipv6-rack-a-0 \
  -c scylla -- nodetool status
```

**Expected output** with IPv6 addresses:

```
Datacenter: ipv6-datacenter
===========================
Status=Up/Down
|/ State=Normal/Leaving/Joining/Moving
--  Address              Load       Tokens  Owns  Host ID                               Rack
UN  fd00:10:244:1::7f    501 KB     256     ?     4583fff5-...                          ipv6-rack-a
UN  fd00:10:244:2::6d    494 KB     256     ?     b1f889b4-...                          ipv6-rack-a
UN  fd00:10:244:3::6c    494 KB     256     ?     7a4bb6da-...                          ipv6-rack-a
```

All nodes should show status `UN` (Up / Normal) with IPv6 addresses (colon-separated).

## Limitations

- IPv6-only has not been validated as extensively as dual-stack configurations.
- Clients that do not support IPv6 cannot connect to the cluster.
- Monitoring and management tools must also support IPv6.
- If you need to support IPv4 clients, use [dual-stack](configure-dual-stack.md) instead.

## Troubleshoot

### Pods fail to schedule with `Unschedulable` status

If pods remain in `Pending` state with reason `Unschedulable`:

```bash
kubectl describe pod -n scylla <pod-name> | grep -A5 "Events:"
```

Check that:
- Nodes are labelled with the correct zone topology labels.
- The `ipFamilies: [IPv6]` field is set at the ScyllaCluster level, not just on the service.

### CQL connections refused after enabling single-stack

If clients report connection refused errors after switching to IPv6:

1. Verify the headless service has an IPv6 `ClusterIP`:
   ```bash
   kubectl get svc -n scylla -o wide
   ```
2. Confirm the pod has an IPv6 address assigned:
   ```bash
   kubectl get pod -n scylla <pod-name> -o jsonpath='{.status.podIPs}'
   ```
3. Ensure your CQL driver is configured to use IPv6 addresses. Many drivers default to IPv4 contact points.

### Clients cannot resolve the IPv6 address via DNS

If DNS returns an A record (IPv4) instead of AAAA (IPv6):
- Check that `spec.ipFamilyPolicy` is set to `SingleStack`, not `PreferDualStack`.
- Verify the Kubernetes cluster itself uses IPv6 for pod networking by checking `kubectl get nodes -o wide`.

## Related pages

- [Get started with IPv6](get-started.md)
- [Configure dual-stack networking](configure-dual-stack.md)
- [Troubleshoot IPv6 issues](troubleshooting.md)
- [Networking architecture](../../../understand/networking.md)
