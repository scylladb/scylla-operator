# IPv6 networking concepts

This document explains how IPv6 networking works in ScyllaDB clusters, including the design decisions and behaviors.

## Overview

IPv6 networking support in ScyllaDB Operator enables you to run ScyllaDB clusters on modern IPv6 networks. Understanding how IPv6 works in ScyllaDB helps you make informed decisions about network configuration.

## How IPv6 support works

### Operator orchestration

When you configure IPv6 networking via the `network.ipFamilies` field in the ScyllaCluster CRD, ScyllaDB Operator automatically orchestrates several components:

1. **ScyllaDB configuration**: Enables IPv6 DNS lookup, sets listen and broadcast addresses
2. **Service configuration**: Creates Kubernetes services with appropriate IP families
3. **Manager Agent**: Configures scylla-manager-agent for IPv6 connectivity
4. **Health probes**: Sets up readiness and liveness probes on IPv6 addresses
5. **Network policies**: Ensures proper IPv6 traffic flow

This automated configuration ensures all components work together correctly without manual intervention.

### Automatic vs manual configuration

The operator handles all IPv6 configuration automatically. You don't need to:
- Set ScyllaDB command-line arguments directly
- Configure listen or broadcast addresses manually
- Modify manager agent configuration
- Adjust health probe settings

**You only configure**:
- `network.ipFamilies` - Which IP protocol(s) to use
- `network.ipFamilyPolicy` - How to handle dual-stack
- `network.dnsPolicy` - DNS resolution strategy

The operator translates these high-level settings into appropriate low-level configurations.

## IP family selection behavior

### Primary IP family determines ScyllaDB protocol

The **first** IP family in `network.ipFamilies` is critical - it determines which protocol ScyllaDB uses for **all** internal operations:

```yaml
# Example 1: ScyllaDB uses IPv6
network:
  ipFamilies:
    - IPv6  # ← ScyllaDB uses this
    - IPv4  # Services also support this
```

```yaml
# Example 2: ScyllaDB uses IPv4
network:
  ipFamilies:
    - IPv4  # ← ScyllaDB uses this
    - IPv6  # Services also support this
```

### Why first IP family matters

ScyllaDB requires consistent addressing for:

**Gossip protocol**: Nodes exchange cluster membership information using a single protocol. Mixed protocols would break gossip.

**Data replication**: When nodes replicate data, they must use the same IP protocol to establish connections.

**Broadcast addresses**: All nodes must be able to reach each other's broadcast addresses using the same protocol.

**Token ring**: The consistent hash ring requires uniform addressing across all nodes.

### Service IP families

While ScyllaDB uses a single IP family, Kubernetes services can support multiple:

```yaml
network:
  ipFamilies:
    - IPv4  # ScyllaDB protocol
    - IPv6  # Additional service accessibility
```

**Result**:
- Services get both IPv4 and IPv6 addresses
- Clients can discover services via either protocol
- ScyllaDB nodes communicate using IPv4 (the first family)
- Actual database connections still use IPv4

This provides client flexibility while maintaining internal consistency.

## Dual-stack behavior explained

Understanding dual-stack is key to successful IPv6 adoption.

### What dual-stack means

"Dual-stack" refers to **Kubernetes services** having both IPv4 and IPv6 addresses, not ScyllaDB itself.

**Dual-stack components**:
- Kubernetes Services: Have both IPv4 and IPv6 ClusterIPs
- DNS records: Have both A (IPv4) and AAAA (IPv6) records
- Client access: Clients can connect via either protocol

**Single-stack components**:
- ScyllaDB nodes: Use one IP protocol (the first in `ipFamilies`)
- Inter-node communication: Uses one protocol
- Data replication: Uses one protocol

### How dual-stack works

![Dual-stack architecture diagram showing Kubernetes Service with both IPv4 and IPv6 ClusterIPs, IPv4 and IPv6 clients connecting to it, and ScyllaDB cluster using IPv4 for internal communication](../ipv6-dual-stack-architecture.svg)

**Key insight**: Services provide protocol translation, allowing diverse clients to reach a cluster running on a single protocol.

## Client connectivity patterns

### Single-stack IPv4

```yaml
network:
  ipFamilies:
    - IPv4
```

**Behavior**:
- Services: IPv4 only
- ScyllaDB: IPv4 protocol
- Clients: Must support IPv4

**Use when**: Standard deployments, maximum compatibility

### Single-stack IPv6

```yaml
network:
  ipFamilies:
    - IPv6
```

**Behavior**:
- Services: IPv6 only
- ScyllaDB: IPv6 protocol
- Clients: Must support IPv6

**Use when**: IPv6-only networks (experimental)

### Dual-stack IPv4-primary

```yaml
network:
  ipFamilies:
    - IPv4
    - IPv6
```

**Behavior**:
- Services: Both IPv4 and IPv6
- ScyllaDB: IPv4 protocol
- Clients: Can use either IPv4 or IPv6

**Use when**: Supporting diverse clients while keeping ScyllaDB on IPv4

### Dual-stack IPv6-primary

```yaml
network:
  ipFamilies:
    - IPv6
    - IPv4
```

**Behavior**:
- Services: Both IPv6 and IPv4
- ScyllaDB: IPv6 protocol
- Clients: Can use either IPv6 or IPv4

**Use when**: Migrating to IPv6 while supporting legacy IPv4 clients

## DNS configuration and IPv6

DNS configuration is critical for IPv6 networking.

### Why ClusterFirst matters

```yaml
network:
  dnsPolicy: ClusterFirst  # Essential for IPv6
```

**ClusterFirst ensures**:
- Kubernetes DNS (CoreDNS) handles name resolution
- AAAA records (IPv6) are correctly resolved
- Service discovery works for IPv6 addresses
- Pod-to-pod DNS works across IP families

### Without ClusterFirst

If `dnsPolicy` is not `ClusterFirst`:
- DNS may use node's resolver instead of cluster DNS
- AAAA records might not be resolved correctly
- ScyllaDB nodes may fail to discover each other
- Service names may resolve to wrong IP family

## Default behavior and fallbacks

Understanding defaults helps when troubleshooting.

### When network configuration is omitted

```yaml
# No network configuration
spec:
  datacenter:
    # ...
```

**Defaults**:
- `ipFamilies`: `[IPv4]`
- `ipFamilyPolicy`: Cluster default (usually `SingleStack`)
- `dnsPolicy`: `ClusterFirst`

**Result**: IPv4-only cluster (backward compatible)

### When ipFamilyPolicy is PreferDualStack

```yaml
network:
  ipFamilyPolicy: PreferDualStack
  ipFamilies:
    - IPv4
    - IPv6
```

**If cluster supports dual-stack**: Services get both IP families

**If cluster doesn't support dual-stack**: Services get first IP family only (IPv4)

**Advantage**: Graceful degradation without configuration changes

## Multi-datacenter considerations

Multi-datacenter deployments have additional requirements.

### IP family consistency requirement

All datacenters **must** use the same IP family:

```yaml
# Datacenter 1
network:
  ipFamilies:
    - IPv6

# Datacenter 2 (must match)
network:
  ipFamilies:
    - IPv6  # ✓ Same as DC1
```

**Why this is required**:

**Cross-datacenter replication**: Nodes in different datacenters must replicate data. They need compatible addressing to establish connections.

**Gossip synchronization**: Gossip information must propagate across datacenters. Mixed protocols break gossip.

**Broadcast address reachability**: Each node's broadcast address must be reachable from all datacenters.

**Token ring consistency**: The global token ring spans all datacenters and requires uniform addressing.

### Multi-datacenter services

Each datacenter's services can be dual-stack even if ScyllaDB uses single-stack:

```yaml
# Datacenter 1
network:
  ipFamilies:
    - IPv6  # ScyllaDB protocol
    - IPv4  # Service accessibility

# Datacenter 2
network:
  ipFamilies:
    - IPv6  # Same ScyllaDB protocol
    - IPv4  # Service accessibility
```

This provides local client flexibility while maintaining cluster consistency.

## Production readiness

For authoritative information about production readiness and experimental status, see [Production readiness](../reference/ipv6-configuration.md#production-readiness) in the configuration reference.

### Recommendation

Use **dual-stack** for production:
- Well-tested in CI/CD
- Provides fallback options
- Supports diverse clients
- Easier troubleshooting

For production IPv6-only support progress, see [#3211](https://github.com/scylladb/scylla-operator/issues/3211).

## Common misconceptions

### Misconception: Dual-stack means ScyllaDB uses both protocols

**Reality**: ScyllaDB uses the first IP family only. Services support both protocols.

### Misconception: Can mix IP families across datacenters

**Reality**: All datacenters must use the same IP family for ScyllaDB.

### Misconception: Can't use IPv6 without IPv6-only networking

**Reality**: Dual-stack (IPv4 + IPv6) is recommended and well-supported.

## Related documentation

- [IPv6 configuration reference](../reference/ipv6-configuration.md)
- [How to configure IPv6](../how-to/ipv6-configure.md)
- [Getting started with IPv6](../tutorials/ipv6-getting-started.md)
- [Troubleshoot IPv6 issues](../how-to/ipv6-troubleshoot.md)
