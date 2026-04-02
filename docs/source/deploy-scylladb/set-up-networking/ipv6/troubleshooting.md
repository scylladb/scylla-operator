# Troubleshoot IPv6 issues

Diagnose and resolve common IPv6 networking issues in ScyllaDB clusters.

## Quick diagnostics

Run these commands to assess the IPv6 configuration at a glance:

```bash
# 1. Check that nodes have IPv6 addresses
kubectl get nodes -o jsonpath='{.items[*].status.addresses[?(@.type=="InternalIP")].address}' | tr ' ' '\n' | grep ':'

# 2. Check Service IP families
kubectl -n scylla get svc -o custom-columns=NAME:.metadata.name,IP-FAMILIES:.spec.ipFamilies,POLICY:.spec.ipFamilyPolicy

# 3. Check ScyllaDB cluster status
kubectl -n scylla exec -it <pod-name> -c scylla -- nodetool status
```

## Services show only one IP family

**Symptom:** After configuring `ipFamilyPolicy: PreferDualStack`, Services still show a single IP family.

**Possible causes:**

| Cause | How to verify | Solution |
|---|---|---|
| Kubernetes cluster does not support dual-stack | `kubectl get nodes -o jsonpath='{.items[*].status.addresses}'` — check for IPv6 `InternalIP` entries | Enable dual-stack at the cluster level or use a cluster with dual-stack support |
| Kubernetes version too old | `kubectl version` | Dual-stack requires Kubernetes 1.23 or newer (stable) |

`PreferDualStack` degrades gracefully to single-stack without errors.
If you need to detect the failure explicitly, use `RequireDualStack` instead — it fails if dual-stack is not available.

## Nodes cannot discover each other

**Symptom:** Pods are `Running` but `nodetool status` shows nodes as `DN` (Down / Normal) or only one node is visible.

**Possible causes:**

1. **DNS resolution failure.** Verify that `dnsPolicy: ClusterFirst` is set in the `network` section.
   Without it, AAAA (IPv6) records may not be resolved correctly.

   ```bash
   kubectl -n scylla exec -it <pod-name> -c scylla -- \
     nslookup <service-name>.scylla.svc.cluster.local
   ```

2. **IP family mismatch.** Confirm that the first entry in `ipFamilies` is the same across all datacenters in a multi-datacenter deployment.

3. **Network policy blocking IPv6.** Check that NetworkPolicies allow IPv6 traffic between ScyllaDB pods.

   ```bash
   kubectl -n scylla get networkpolicy -o yaml
   ```

## Clients cannot connect over IPv6

**Symptom:** Applications fail to connect to the cluster using an IPv6 address.

**Diagnostic steps:**

1. Verify the client Service has an IPv6 ClusterIP:

   ```bash
   kubectl -n scylla get svc <cluster-name>-client -o jsonpath='{.spec.clusterIPs}'
   ```

2. Confirm the client application supports IPv6.
   Some CQL drivers require explicit IPv6 configuration.

3. Test connectivity from within the cluster:

   ```bash
   kubectl run -it --rm cqlsh --image=scylladb/scylla-cqlsh:latest --restart=Never -- \
     <cluster-name>-client.scylla.svc.cluster.local 9042 \
     -e "SELECT cluster_name FROM system.local;"
   ```

## ScyllaDB uses the wrong IP family

**Symptom:** `nodetool status` shows IPv4 addresses when you expected IPv6, or vice versa.

**Cause:** ScyllaDB always uses the **first** entry in `network.ipFamilies`.

**Solution:** Verify the order of `ipFamilies` in your ScyllaCluster manifest:

```yaml
# ScyllaDB uses IPv6
network:
  ipFamilies:
    - IPv6   # ← ScyllaDB uses this
    - IPv4

# ScyllaDB uses IPv4
network:
  ipFamilies:
    - IPv4   # ← ScyllaDB uses this
    - IPv6
```

## Pods fail to start after enabling IPv6

**Symptom:** Pods go into `CrashLoopBackOff` after adding `network.ipFamilies` with IPv6.

**Diagnostic steps:**

1. Check previous container logs:

   ```bash
   kubectl -n scylla logs <pod-name> -c scylla --previous
   ```

2. Verify the ScyllaDB version supports IPv6 (2024.1 or newer recommended):

   ```bash
   kubectl -n scylla get scyllacluster -o jsonpath='{.items[*].spec.version}'
   ```

3. Check pod events:

   ```bash
   kubectl -n scylla describe pod <pod-name>
   ```

## Migration issues

**Symptom:** Problems during IPv4 → IPv6 migration.

**Diagnostic steps:**

1. Verify the rolling restart is progressing:

   ```bash
   kubectl -n scylla get pods -l scylla-operator.scylladb.com/pod-type=scylladb-node -w
   ```

2. If a pod is stuck, check its events and logs:

   ```bash
   kubectl -n scylla describe pod <pod-name>
   kubectl -n scylla logs <pod-name> -c scylla
   ```

3. To roll back, revert the `network` section to the previous configuration and apply:

   ```bash
   kubectl apply --server-side -f scylla-cluster.yaml
   ```

   See [Rolling back](migration.md#rolling-back) for details.

## Related pages

- [Get started with IPv6](get-started.md)
- [Configure dual-stack networking](configure-dual-stack.md)
- [Configure IPv6-only single-stack](configure-single-stack.md)
- [Migrate clusters to IPv6](migration.md)
- [Networking architecture](../../../understand/networking.md)
