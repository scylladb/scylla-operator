# IPv6 configuration reference

This reference documents IPv6-specific configuration guidance for ScyllaDB clusters.

For complete field definitions and schemas, see the [ScyllaCluster API Reference](../../../../reference/api/groups/scylla.scylladb.com/scyllaclusters).

## Overview

IPv6 networking in ScyllaDB is configured through the `spec.network` section of the ScyllaCluster resource. The key concepts:

1. **IP family selection**: The first IP family in `spec.network.ipFamilies` determines which protocol ScyllaDB uses internally
2. **Service dual-stack**: Kubernetes services can support both IPv4 and IPv6 simultaneously
3. **Automatic configuration**: The operator configures ScyllaDB settings automatically based on your network configuration

## Network configuration

Configure IPv6 through the `spec.network` section of your ScyllaCluster resource.

### Field reference

See the [ScyllaCluster API Reference](../../../../reference/api/groups/scylla.scylladb.com/scyllaclusters) for complete field definitions:
- `spec.network.ipFamilyPolicy` - Service IP family policy
- `spec.network.ipFamilies` - List of IP families to use
- `spec.network.dnsPolicy` - DNS policy for pods

### IP family order significance

The **first** IP family in `spec.network.ipFamilies` determines which protocol ScyllaDB uses for all internal communication:

- `[IPv6]` → ScyllaDB uses IPv6
- `[IPv4, IPv6]` → ScyllaDB uses IPv4, services support both
- `[IPv6, IPv4]` → ScyllaDB uses IPv6, services support both

**Example configurations**:

```yaml
# IPv6-only ScyllaDB
network:
  ipFamilyPolicy: SingleStack
  ipFamilies:
    - IPv6
  dnsPolicy: ClusterFirst
```

```yaml
# IPv4 ScyllaDB with dual-stack services (recommended)
network:
  ipFamilyPolicy: PreferDualStack
  ipFamilies:
    - IPv4  # ScyllaDB uses this
    - IPv6  # Services also accessible via IPv6
  dnsPolicy: ClusterFirst
```

```yaml
# IPv6 ScyllaDB with dual-stack services
network:
  ipFamilyPolicy: PreferDualStack
  ipFamilies:
    - IPv6  # ScyllaDB uses this
    - IPv4  # Services also accessible via IPv4
  dnsPolicy: ClusterFirst
```

### DNS policy requirement

For IPv6 configurations, `spec.network.dnsPolicy` must be set to `ClusterFirst` to ensure proper AAAA record resolution.

## Automatic operator configurations

When you configure `spec.network.ipFamilies`, the operator automatically applies corresponding ScyllaDB configurations. You don't need to set these manually.

### ScyllaDB configuration

The operator automatically configures these ScyllaDB arguments based on `network.ipFamilies`:

#### IPv6 DNS lookup

**ScyllaDB argument**: `--enable-ipv6-dns-lookup=1`

**When applied**: When the first IP family is `IPv6`

**Purpose**: Enables ScyllaDB to resolve IPv6 addresses

#### Listen addresses

**ScyllaDB arguments**:
- `--listen-address=::`
- `--rpc-address=::`

**When applied**: When the first IP family is `IPv6`

**Default values**:
- IPv6: `::`
- IPv4: `0.0.0.0`

**Purpose**: Sets which network interface ScyllaDB listens on

### Broadcast addresses

**ScyllaDB arguments**:
- `--broadcast-address=<pod-ip>`
- `--broadcast-rpc-address=<pod-ip>`

**When applied**: Always (IP family determined by first entry in `spec.network.ipFamilies`)

**Configured via**: `spec.exposeOptions.broadcastOptions` (see [ScyllaCluster API Reference](../../../../reference/api/groups/scylla.scylladb.com/scyllaclusters))

**Purpose**: Addresses that ScyllaDB advertises to other nodes and clients

:::{note}
Broadcast addresses are configured through `spec.exposeOptions.broadcastOptions`, not by directly setting ScyllaDB arguments. The operator ensures they use the correct IP family based on your network configuration.
:::

## Configuration validation

The operator validates IPv6 configurations in ScyllaCluster resources:

### Requirements for IPv6

When using IPv6 (first IP family is `IPv6` in `spec.network.ipFamilies`):
- `spec.network.dnsPolicy` should be `ClusterFirst`
- `spec.network.ipFamilies` must include `IPv6`

### Consistency requirements

- All datacenters in a multi-datacenter deployment must use the same IP family
- The first IP family determines ScyllaDB's internal protocol
- Service IP families must be compatible with cluster networking

### Unsupported configurations

These configurations are **not supported**:
- Different IP families across datacenters
- Mixing IPv4 and IPv6 within a single datacenter
- Changing IP family of an existing cluster (requires recreation)

## Configuration examples

See complete ScyllaCluster configuration examples in the repository:

- [Dual-stack configuration](../../../../../../examples/ipv6/scylla-cluster-dual-stack.yaml) - Full production-ready dual-stack setup
- [Minimal dual-stack configuration](../../../../../../examples/ipv6/scylla-cluster-minimal-dual-stack.yaml) - Simple dual-stack example
- [IPv6-only configuration](../../../../../../examples/ipv6/scylla-cluster-ipv6.yaml) - IPv6 single-stack setup

## Version requirements

### Minimum versions

- **ScyllaDB**: 2024.1 or newer
- **ScyllaDB Operator**: 1.20 or newer

## Production readiness

### Production-ready configurations

The following configurations are **production-ready**:
- **IPv4-only single-stack**: Fully supported (default Kubernetes behavior)
- **IPv4-first dual-stack**: Fully supported and recommended for IPv6 adoption
- **IPv6-first dual-stack**: Fully supported

### Experimental configurations

The following configurations are **experimental** and not recommended for production use:
- **IPv6-only single-stack**: Currently under development

:::{note}
**Experimental status**: IPv6-only configurations work but have not undergone the same level of testing and validation as dual-stack configurations. For production IPv6 deployments, use dual-stack configurations instead.

Track progress on productionizing IPv6-only: [#3211](https://github.com/scylladb/scylla-operator/issues/3211)
:::

## Related documentation

- [IPv6 networking concepts](../concepts/ipv6-networking.md)
- [How to configure IPv6](../how-to/ipv6-configure.md)
- [Getting started with IPv6](../tutorials/ipv6-getting-started.md)
- [ScyllaCluster API Reference](../../../../reference/api/groups/scylla.scylladb.com/scyllaclusters)
