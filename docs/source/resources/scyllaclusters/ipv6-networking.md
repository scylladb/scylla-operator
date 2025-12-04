# IPv6 and Dual-Stack Networking for ScyllaClusters

## Overview

ScyllaDB supports IPv6 networking, which means you can run your database clusters on IPv6-only networks or in dual-stack environments that support both IPv4 and IPv6. 
This guide covers how to configure IPv6 networking for ScyllaDB clusters and helps you troubleshoot common issues you might encounter.

## Prerequisites

Before deploying ScyllaDB with IPv6, verify that your environment supports it:

### What your Kubernetes cluster needs

- **Kubernetes version**: 1.23 or newer (for proper dual-stack support)
- **Dual-stack setup**: Your cluster needs both IPv4 and IPv6 network ranges configured

### What ScyllaDB needs

- **ScyllaDB version**: 2024.1 or newer recommended for IPv6 support
- **Network setup**: ScyllaDB nodes must be able to talk to each other over IPv6

### Verifying IPv6 Support

Run these commands to check if your cluster is ready for IPv6:

```bash
kubectl get nodes -o jsonpath='{.items[*].status.addresses[?(@.type=="InternalIP")].address}' | tr ' ' '\n' | grep ':'

kubectl cluster-info dump | grep -i ipv6
```

### DNS Configuration for IPv6

When using IPv6, it's important to configure DNS properly for ScyllaDB to resolve IPv6 addresses:

```yaml
spec:
  network:
    dnsPolicy: ClusterFirst
```

**Why ClusterFirst matters for IPv6:**
- Make sure IPv6 addresses are resolved correctly within the cluster
- Required for ScyllaDB to discover other nodes using IPv6 addresses
- Enables proper service discovery in IPv6-only environments

## How ScyllaDB IPv6 Support Works

### Automatic Configuration

When you set `ipFamily: IPv6`, ScyllaDB Operator automatically configures:

1. **ScyllaDB IPv6 DNS Lookup**: Enables `--enable-ipv6-dns-lookup=1` flag for proper IPv6 address resolution
2. **IPv6 Broadcast Addresses**: Configures ScyllaDB to broadcast IPv6 addresses to nodes and clients  
3. **IPv6 Listen Addresses**: Sets ScyllaDB to listen on IPv6 wildcard address (`::`) instead of IPv4 (`0.0.0.0`)
4. **Manager Agent IPv6**: Configures scylla-manager-agent to connect to ScyllaDB API over IPv6 localhost (`::1`)
5. **Probe Servers IPv6**: Sets up health check and readiness probes to bind to IPv6 addresses
6. **Service IPv6**: Creates Kubernetes services with appropriate IPv6 address types

### ScyllaDB Configuration Changes

Here's what happens internally when `ipFamily: IPv6` is set:

```yaml
# ScyllaDB configuration changes (automatic)
scylla_args: 
  - --enable-ipv6-dns-lookup=1
  - --listen-address=::
  - --rpc-address=::
  - --broadcast-address=<pod-ipv6>
  - --broadcast-rpc-address=<pod-ipv6>

# Manager agent configuration (automatic)
scylla_manager_agent:
  api_address: "::1"
  api_port: 10000
```

**Important**: These configurations are applied automatically by the operator - you don't need to set them manually when using `ipFamily: IPv6`.

**Note**: The broadcast addresses (`--broadcast-address` and `--broadcast-rpc-address`) are determined by your `exposeOptions.broadcastOptions` configuration, not by directly setting ScyllaDB arguments. The operator automatically ensures they use the correct IP family based on your `ipFamily` setting.

## Configuration Options

### Simple IPv6 Configuration (Recommended)

The easiest way to configure IPv6 is using the unified `ipFamily` field:

```yaml
spec:
  ipFamily: IPv6
  
  network:
    ipFamilyPolicy: SingleStack | PreferDualStack | RequireDualStack
    ipFamilies: 
      - IPv6
      - IPv4
    
    dnsPolicy: ClusterFirst
```

### Advanced Broadcast Configuration

If you need to customize how ScyllaDB advertises its addresses, you can override the broadcast settings:

```yaml
spec:
  ipFamily: IPv6
  
  exposeOptions:
    broadcastOptions:
      nodes:
        type: PodIP
        podIP:
          source: Status
      
      clients:
        type: ServiceClusterIP
```

The `ipFamily` setting automatically configures both node and client broadcast addresses to use the same IP family, ensuring consistency across your ScyllaDB cluster.

## IPv6-only setup

:::{warning}
**Experimental Feature**: IPv6-only configurations are currently experimental and not recommended for production use. For production deployments, we recommend using dual-stack configurations or IPv4.
:::

To run ScyllaDB exclusively on IPv6, you just need to set the `ipFamily` field. The operator handles all the rest:

```yaml
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
metadata:
  name: scylla-ipv6-only
  namespace: scylla
spec:
  version: {{imageTag}}
  
  ipFamily: IPv6
  
  network:
    ipFamilyPolicy: SingleStack
    ipFamilies: ["IPv6"]
    - IPv6
    dnsPolicy: ClusterFirst
  
  datacenter:
    name: ipv6-dc
    racks:
    - name: ipv6-rack
      members: 3
      storage:
        capacity: 100Gi
        storageClassName: fast-ssd
      resources:
        requests:
          cpu: 2
          memory: 8Gi
        limits:
          cpu: 2
          memory: 8Gi
  
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

## Dual-stack Kubernetes Services

**Important**: ScyllaDB itself always runs on a single IP family (IPv4 OR IPv6), determined by the `ipFamily` setting. "Dual-stack" refers to Kubernetes services that can have both IPv4 and IPv6 addresses, allowing clients to reach the services via either protocol. However, the actual ScyllaDB database connections will use whichever IP family you specified in `ipFamily`.

There are two ways to configure dual-stack Kubernetes services:

### Simple Dual-Stack Services (Recommended)

The easiest approach is to let Kubernetes auto-detect dual-stack capabilities while you specify which IP family ScyllaDB should use internally:

```yaml
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
metadata:
  name: simple-dual-stack
  namespace: scylla
spec:
  version: {{imageTag}}
  
  ipFamily: IPv4
  
  datacenter:
    name: dual-stack-dc
    racks:
    - name: dual-stack-rack
      members: 3
      storage:
        capacity: 100Gi
      resources:
        requests:
          cpu: 2
          memory: 8Gi
  
  exposeOptions:
    broadcastOptions:
      nodes:
        type: PodIP
      clients:
        type: PodIP
```

### Explicit Dual-Stack Services (Advanced)

If you need explicit control over service IP families:

```yaml
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
metadata:
  name: explicit-dual-stack
  namespace: scylla
spec:
  version: {{imageTag}}
  
  ipFamily: IPv4
  
  network:
    dnsPolicy: ClusterFirst
    ipFamilyPolicy: PreferDualStack
    ipFamilies: ["IPv4", "IPv6"]  # Services accessible via both protocols
  
  datacenter:
    name: dual-stack-dc
    racks:
    - name: dual-stack-rack
      members: 3
      storage:
        capacity: 100Gi
      resources:
        requests:
          cpu: 2
          memory: 8Gi
  
  exposeOptions:
    broadcastOptions:
      nodes:
        type: PodIP
      clients:
        type: PodIP
```

::::{note}
**When do you need explicit service configuration?**

- **Most dual-stack clusters**: Use the simple approach - Kubernetes auto-detects dual-stack capabilities
- **Explicit control needed**: Use advanced approach when you need to force specific service networking behavior
- **Single IP family clusters**: Never needed - the `ipFamily` field is sufficient

**Client Connectivity in Dual-Stack**

Even with dual-stack services, ScyllaDB itself only communicates on one IP family:
- If `ipFamily: IPv4` → ScyllaDB nodes and clients communicate via IPv4
- If `ipFamily: IPv6` → ScyllaDB nodes and clients communicate via IPv6
- Kubernetes services can be reachable via both protocols, but the actual database connection uses the `ipFamily` protocol

**Why use IPv4 for ScyllaDB in dual-stack environments?**

IPv4 is recommended for ScyllaDB itself in dual-stack environments for maximum compatibility, while Kubernetes services can still support both IP families for client discovery and load balancing.
::::

## Client Connectivity

Understanding how clients actually connect to ScyllaDB in different configurations:

### IPv4-only ScyllaDB
```yaml
spec:
  ipFamily: IPv4  # ScyllaDB runs on IPv4
```
- Clients connect to ScyllaDB using IPv4 addresses
- All node-to-node communication uses IPv4

### IPv6-only ScyllaDB  
```yaml
spec:
  ipFamily: IPv6  # ScyllaDB runs on IPv6
```
- Clients connect to ScyllaDB using IPv6 addresses
- All node-to-node communication uses IPv6

### Dual-Stack Services with IPv4 ScyllaDB
```yaml
spec:
  ipFamily: IPv4                    # ScyllaDB runs on IPv4
  network:
    ipFamilies: ["IPv4", "IPv6"]    # Services available via both
```
- Clients can discover services via IPv4 or IPv6
- Actual ScyllaDB connections still use IPv4 (because `ipFamily: IPv4`)
- Useful when clients are on different network stacks but ScyllaDB needs consistent addressing

## Default Behavior

When IP family preferences are not explicitly specified, ScyllaCluster follows this priority:

### Automatic IP Selection

1. **Explicit IP Family**: If `ipFamily` is specified, use that for all ScyllaDB communication
2. **Network IP Families**: Uses the first IP family from `network.ipFamilies` 
3. **Fallback**: Defaults to IPv4 if no configuration is found (backward compatibility)

### Examples

```yaml
spec:
  ipFamily: IPv6
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

```yaml
spec:
  network:
    ipFamilies: ["IPv6", "IPv4"]  # ScyllaDB will use IPv6 (first in list)
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

**Result based on configuration:**
- `ipFamily: IPv6` → Uses IPv6 for ScyllaDB
- `ipFamily: IPv4` → Uses IPv4 for ScyllaDB  
- `network.ipFamilies: ["IPv6"]` → Uses IPv6 for ScyllaDB
- `network.ipFamilies: ["IPv4", "IPv6"]` → Uses IPv4 for ScyllaDB (first in list)
- No configuration → Uses IPv4 (backward compatibility)

## Migration Scenarios

### IPv4 to IPv6

Migrate an existing IPv4 cluster to IPv6:

1. **Update cluster configuration**:
   ```yaml
   spec:
     ipFamily: IPv6  # Simple change to IPv6
     network:
       ipFamilyPolicy: SingleStack
       ipFamilies: ["IPv6"]
   ```

2. **Rolling update** will reconfigure ScyllaDB for IPv6
3. **Verify connectivity** on IPv6
4. **Update clients** to use IPv6 addresses

### IPv4 to Dual-Stack Services

Add dual-stack Kubernetes services to an existing IPv4 ScyllaDB cluster:

1. **Update cluster network configuration**:
   ```yaml
   spec:
     ipFamily: IPv4  # Keep ScyllaDB on IPv4 for consistency
     network:
       ipFamilyPolicy: PreferDualStack
       ipFamilies: ["IPv4", "IPv6"]  # Services accessible via both protocols
   ```

2. **Rolling update** will configure services for dual-stack access
3. **Verify connectivity** - clients can now reach services via both IPv4 and IPv6
4. **Note**: ScyllaDB still uses IPv4 internally; only service access is dual-stack

### Example: Dual-Stack Services with IPv4 ScyllaDB

Here's a complete example showing how ScyllaDB uses IPv4 consistently while Kubernetes services can be reached via both IPv4 and IPv6:

```yaml
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
metadata:
  name: dual-stack-example
  namespace: scylla
spec:
  version: {{imageTag}}
  ipFamily: IPv4  # ScyllaDB database uses IPv4 consistently
  
  network:
    ipFamilies: ["IPv4", "IPv6"]  # Services accessible via both protocols
    ipFamilyPolicy: PreferDualStack
    dnsPolicy: ClusterFirst
  
  datacenter:
    name: dual-dc
    racks:
    - name: rack1
      members: 3
      storage:
        capacity: 100Gi
      resources:
        requests:
          cpu: 2
          memory: 8Gi
```

**Result**: ScyllaDB configuration (automatically applied by operator):
```yaml
scylla_args:
  - --listen-address=0.0.0.0
  - --rpc-address=0.0.0.0
  - --broadcast-address=10.244.1.5
  - --broadcast-rpc-address=10.244.1.5

# Manager agent configuration - IPv4 localhost
scylla_manager_agent:
  api_address: "127.0.0.1"             # IPv4 localhost
```

**Kubernetes Services created**:
```yaml
apiVersion: v1
kind: Service
metadata:
  name: dual-stack-example
spec:
  ipFamilies: ["IPv4", "IPv6"]
  ipFamilyPolicy: PreferDualStack
  clusterIPs: 
    - "10.96.1.100"      # IPv4 service IP
    - "fd00:10:96::100"  # IPv6 service IP
  ports:
  - name: cql
    port: 9042
```

**Client Access Scenarios**:
- **Service Discovery**: Clients can discover the service via either `10.96.1.100:9042` (IPv4) or `[fd00:10:96::100]:9042` (IPv6)
- **Actual Connection**: When clients connect through either service IP, they ultimately reach ScyllaDB pods listening on IPv4 (`10.244.1.5:9042`)
- **DNS Resolution**: Service DNS name resolves to both A (IPv4) and AAAA (IPv6) records
- **Network Path**: IPv6 clients → IPv6 service IP → Kubernetes routing → IPv4 ScyllaDB pod

This provides the flexibility similar to your mixed configuration example, but in a supported way: clients on IPv6-only networks can discover and reach the service, while ScyllaDB maintains consistent IPv4 networking internally.

## Multi-Datacenter IPv6

For multi-datacenter deployments with IPv6:

### Consistent IP Family Configuration

Make sure all datacenters use the same IP family configuration:

```yaml
# Datacenter 1
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
metadata:
  name: scylla-dc1
spec:
  ipFamily: IPv6  # Consistent across all DCs
  network:
    ipFamilyPolicy: SingleStack
    ipFamilies: ["IPv6"]
  datacenter:
    name: dc1
    # ...

---
# Datacenter 2  
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
metadata:
  name: scylla-dc2
spec:
  ipFamily: IPv6  # Must match DC1
  network:
    ipFamilyPolicy: SingleStack
    ipFamilies: ["IPv6"]
  datacenter:
    name: dc2
    # ...
```

For more details on multi-datacenter networking, see the [Multi-Datacenter Deployments guide](./multidc/multidc.md).

## Troubleshooting

### Common Issues

#### Connection Failures

**Symptoms**: ScyllaDB nodes cannot communicate or clients cannot connect.

**Causes & Solutions**:
- **Firewall blocking IPv6**: Configure firewall rules for IPv6
- **Wrong IP family**: Verify broadcast address configuration
- **DNS issues**: Check if DNS resolves IPv6 addresses

```bash
# Test IPv6 connectivity between nodes
kubectl exec -it scylla-cluster-dc1-rack1-0 -- ping6 <ipv6-address>

# Check ScyllaDB logs for IPv6 issues
kubectl logs scylla-cluster-dc1-rack1-0 -c scylla | grep -i ipv6
```

#### DNS Resolution Issues

**Symptoms**: ScyllaDB cannot resolve IPv6 addresses or connect to other nodes.

**Causes & Solutions**:
- **Wrong DNS policy**: Set `dnsPolicy: ClusterFirst` in network configuration
- **IPv6 DNS disabled**: Ensure cluster DNS (CoreDNS) supports IPv6 
- **DNS lookup failures**: ScyllaDB may need IPv6 DNS lookup enabled

```bash
# Check DNS policy in pod spec
kubectl get pods -l app.kubernetes.io/name=scylla -o yaml | grep -A 2 -B 2 dnsPolicy

# Test IPv6 DNS resolution from ScyllaDB pod
kubectl exec -it scylla-cluster-dc1-rack1-0 -- nslookup scylla-cluster-dc1-rack1-1.scylla-cluster.default.svc.cluster.local

# Check if CoreDNS supports IPv6
kubectl get configmap coredns -n kube-system -o yaml | grep -A 10 -B 10 -i ipv6
```

**Solution**: Add proper DNS configuration:
```yaml
spec:
  network:
    dnsPolicy: ClusterFirst  # Essential for IPv6 DNS resolution
```

#### Service Discovery Issues

**Symptoms**: ScyllaDB cannot discover other nodes in IPv6 clusters.

**Causes & Solutions**:
- **Incorrect broadcast address**: Verify `exposeOptions.broadcastOptions`
- **Service misconfiguration**: Check service IP families
- **Mixed IP families**: Make sure configuration is consistent across the cluster

```bash
# Check service configuration
kubectl get svc -o yaml | grep -A 5 -B 5 -i family

# Verify EndpointSlice address types
kubectl get endpointslices -o yaml | grep addressType
```

### Diagnostic Commands

```bash
# Check cluster IPv6 configuration
kubectl cluster-info dump | grep -E 'service-cluster-ip-range|cluster-cidr' | grep -i ipv6

# Verify pod IPv6 addresses
kubectl get pods -o wide | grep scylla

# Check service IP families
kubectl get svc -o custom-columns=NAME:.metadata.name,IP-FAMILIES:.spec.ipFamilies,POLICY:.spec.ipFamilyPolicy

# Inspect EndpointSlice configuration
kubectl get endpointslices -l app.kubernetes.io/name=scylla -o yaml

# Test ScyllaDB connectivity
kubectl exec -it <scylla-pod> -- nodetool status
```

### Performance Considerations

- **IPv6 overhead**: Slight protocol overhead compared to IPv4
- **Dual-stack complexity**: Additional network configuration may impact performance
- **DNS resolution**: IPv6 DNS queries may have different latency characteristics

## Best Practices

### Configuration

1. **Consistent IP families**: Use the same IP family configuration across all racks/datacenters
2. **Explicit preferences**: Specify IP family preferences for predictable behavior
3. **Testing**: Thoroughly test IPv6 connectivity before production deployment

### Security

1. **Firewall rules**: Configure IPv6 firewall rules alongside IPv4
2. **Network policies**: Apply Kubernetes NetworkPolicies for IPv6 traffic
3. **Monitoring**: Set up monitoring for both IPv4 and IPv6 connectivity

### Monitoring

Monitor IPv6-specific metrics:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: scylla-metrics-ipv6
spec:
  ipFamilyPolicy: SingleStack
  ipFamilies: ["IPv6"]
  ports:
  - name: prometheus
    port: 9180
    protocol: TCP
  selector:
    app.kubernetes.io/name: scylla
```

## Examples Repository

For complete working examples, see:
- [IPv6-only ScyllaCluster example](../../../../examples/ipv6/scylla-cluster-ipv6.yaml)
- [Dual-stack ScyllaCluster example](../../../../examples/ipv6/scylla-cluster-dual-stack.yaml)

## Related Documentation

- [ScyllaCluster API Reference](../../api-reference/groups/scylla.scylladb.com/scyllaclusters.rst)
- [Multi-Datacenter Deployments](./multidc/multidc.md)
- [Client Connectivity](./clients/index.md)
- [Kubernetes IPv6 Documentation](https://kubernetes.io/docs/concepts/services-networking/dual-stack/)
