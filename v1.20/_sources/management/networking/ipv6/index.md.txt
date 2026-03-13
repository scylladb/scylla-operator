# IPv6 networking

IPv6 networking support enables you to run ScyllaDB clusters on IPv6 networks, with options for IPv6-only, IPv4-only, or dual-stack (both protocols) configurations.

:::{note}
IPv6-only configurations are experimental. All other configurations are production-ready. See [Production readiness](reference/ipv6-configuration.md#production-readiness) for details.
:::

## Tutorials

Start here if you're new to IPv6 networking in ScyllaDB:

- [Getting started with IPv6 networking](tutorials/ipv6-getting-started.md) - Your first IPv6-enabled cluster with step-by-step guidance

## How-to guides

Practical guides for specific tasks:

- [Configure dual-stack with IPv4](how-to/ipv6-configure.md) - IPv4-first dual-stack (recommended)
- [Configure dual-stack with IPv6](how-to/ipv6-configure-ipv6-first.md) - IPv6-first dual-stack
- [Configure IPv6-only](how-to/ipv6-configure-ipv6-only.md) - IPv6 single-stack (experimental)
- [Migrate clusters to IPv6](how-to/ipv6-migrate.md) - Migrate existing clusters from IPv4 to IPv6
- [Troubleshoot IPv6 networking issues](how-to/ipv6-troubleshoot.md) - Diagnose and resolve common problems

## Reference

Technical specifications and API details:

- [IPv6 configuration reference](reference/ipv6-configuration.md) - Complete API reference for IPv6 settings

## Concepts

Deep explanations of how IPv6 networking works:

- [IPv6 networking concepts](concepts/ipv6-networking.md) - Understand how IPv6 support works in ScyllaDB

## Configuration examples

Quick examples for common scenarios:

**Dual-stack (recommended for production)**:
```yaml
network:
  ipFamilyPolicy: PreferDualStack
  ipFamilies:
    - IPv4  # ScyllaDB uses IPv4
    - IPv6  # Services also support IPv6
  dnsPolicy: ClusterFirst
```

**IPv6-only (experimental)**:
```yaml
network:
  ipFamilyPolicy: SingleStack
  ipFamilies:
    - IPv6
  dnsPolicy: ClusterFirst
```

**IPv4-only (default)**:
```yaml
network:
  ipFamilyPolicy: SingleStack
  ipFamilies:
    - IPv4
```

For complete examples, see the [how-to guides](how-to/ipv6-configure.md).

## Key concepts

### IP family selection

The **first** IP family in `network.ipFamilies` determines which protocol ScyllaDB uses internally:

- `[IPv6]` → ScyllaDB uses IPv6
- `[IPv4, IPv6]` → ScyllaDB uses IPv4, services support both
- `[IPv6, IPv4]` → ScyllaDB uses IPv6, services support both

Learn more in [IPv6 networking concepts](concepts/ipv6-networking.md).

### Dual-stack behavior

"Dual-stack" refers to Kubernetes services having both IPv4 and IPv6 addresses. ScyllaDB itself always runs on a single IP family (the first one configured).

This provides client flexibility while maintaining internal consistency.

### DNS configuration

For IPv6, `dnsPolicy: ClusterFirst` is essential to ensure proper DNS resolution of IPv6 addresses.

## Prerequisites

Before configuring IPv6:

- **Network**: Cluster must have IPv6 networking configured

## Feature status

| Configuration | Status | Production Ready |
|---------------|--------|------------------|
| Dual-stack (IPv4 + IPv6) | Stable | Yes |
| IPv6-only | Experimental | No |
| IPv4-only | Stable | Yes |

For IPv6-only production support progress, see [#3211](https://github.com/scylladb/scylla-operator/issues/3211).

## Getting help

If you need assistance:

1. **Check documentation**: Review the troubleshooting guide and concepts
2. **Search issues**: Look for similar problems in [GitHub issues](https://github.com/scylladb/scylla-operator/issues)
3. **Ask the community**: Join [ScyllaDB Slack](https://scylladb-users.slack.com/)
4. **Open an issue**: Report bugs or request features on [GitHub](https://github.com/scylladb/scylla-operator/issues/new)

## Related documentation

- [Networking overview](../index.md)
- [ScyllaCluster API Reference](../../../reference/api/groups/scylla.scylladb.com/scyllaclusters)
- [Multi-Datacenter Deployments](../../../resources/scyllaclusters/multidc/multidc)
- [Kubernetes IPv6 Documentation](https://kubernetes.io/docs/concepts/services-networking/dual-stack/)

```{toctree}
:hidden:
:maxdepth: 2

tutorials/ipv6-getting-started
how-to/ipv6-configure
how-to/ipv6-configure-ipv6-first
how-to/ipv6-configure-ipv6-only
how-to/ipv6-migrate
how-to/ipv6-troubleshoot
reference/ipv6-configuration
concepts/ipv6-networking
```
