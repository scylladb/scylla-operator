# IPv6 configuration reference

This reference documents the IPv6-specific API fields, automatic ScyllaDB settings, validation rules, and version requirements for IPv6 networking.

For complete field definitions and schemas, see the ScyllaCluster API reference.
For setup instructions, see [IPv6 networking](../deploy-scylladb/set-up-networking/ipv6/index.md).

## Network configuration fields

IPv6 networking is configured through the `spec.network` section of a ScyllaCluster.

### `spec.network.ipFamilyPolicy`

Controls how Kubernetes assigns IP families to the Services created for ScyllaDB nodes. Defaults to `SingleStack`. Allowed values: `SingleStack` (single IP family only), `PreferDualStack` (both families if supported, falls back to single-stack), `RequireDualStack` (both families required, rejected if unsupported).

### `spec.network.ipFamilies`

A list of IP families (`IPv4`, `IPv6`) that the ScyllaDB cluster uses. Defaults to `[IPv4]` when omitted. The **first** entry determines which protocol ScyllaDB uses for all internal communication (`listen_address`, `rpc_address`, broadcast addresses, seed resolution).
The second entry (if present) is used only at the Kubernetes Service level for dual-stack accessibility.

### `spec.network.dnsPolicy`

Sets the DNS resolution policy for ScyllaDB pods. Defaults to `ClusterFirstWithHostNet`. Accepts any Kubernetes [DNSPolicy](https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/#pod-s-dns-policy). For IPv6 configurations, set this to `ClusterFirst` to ensure proper AAAA record resolution.

## Automatic ScyllaDB configuration

When `spec.network.ipFamilies` includes IPv6 as the first entry, the Operator automatically applies the following ScyllaDB arguments. You do not need to set these manually.

| ScyllaDB argument | IPv4 value | IPv6 value | Purpose |
|---|---|---|---|
| `--listen-address` | `0.0.0.0` | `::` | Interface ScyllaDB listens on for inter-node communication |
| `--rpc-address` | not set (ScyllaDB defaults to `0.0.0.0`) | `::` | Interface ScyllaDB listens on for CQL client connections |
| `--enable-ipv6-dns-lookup` | not set | `1` | Enables AAAA DNS record resolution in ScyllaDB |

### Broadcast addresses

Broadcast addresses (`--broadcast-address` and `--broadcast-rpc-address`) are configured through `spec.exposeOptions.broadcastOptions`, not by setting ScyllaDB arguments directly. The Operator ensures broadcast addresses match the selected IP family.

See [Exposing clusters](../deploy-scylladb/set-up-networking/expose-clusters.md) for details on broadcast options.

## Validation rules

The Operator validates IPv6-related fields at admission time.

### Consistency requirements

- The first entry in `spec.network.ipFamilies` determines ScyllaDB's protocol. All nodes in the cluster use the same protocol.
- If `--listen-address` or `--rpc-address` are set manually via `additionalScyllaDBArguments`, they must be compatible with the selected IP family. Values containing `:` are treated as IPv6; `0.0.0.0`, `::`, and empty strings are treated as wildcards and are valid for either family.

### Unsupported configurations

- Different IP families across datacenters in a multi-datacenter deployment.
- Changing the IP family of an existing cluster (requires cluster recreation).

## Example configurations

### IPv4 single-stack (default)

No `network` section is needed. IPv4 single-stack is the default behavior.

### IPv4-first dual-stack

```yaml
spec:
  network:
    ipFamilyPolicy: PreferDualStack
    ipFamilies:
      - IPv4
      - IPv6
    dnsPolicy: ClusterFirst
```

ScyllaDB uses IPv4 for internal communication. Services are accessible over both IPv4 and IPv6.

### IPv6-first dual-stack

```yaml
spec:
  network:
    ipFamilyPolicy: PreferDualStack
    ipFamilies:
      - IPv6
      - IPv4
    dnsPolicy: ClusterFirst
```

ScyllaDB uses IPv6 for internal communication. Services are accessible over both IPv6 and IPv4.

### IPv6-only single-stack

:::{caution}
IPv6-only single-stack is experimental and not recommended for production use. See [GitHub issue #3211](https://github.com/scylladb/scylla-operator/issues/3211) for status.
:::

```yaml
spec:
  network:
    ipFamilyPolicy: SingleStack
    ipFamilies:
      - IPv6
    dnsPolicy: ClusterFirst
```

Complete example manifests are available in the repository:

- [`examples/ipv6/scylla-cluster-dual-stack.yaml`](https://github.com/scylladb/scylla-operator/blob/master/examples/ipv6/scylla-cluster-dual-stack.yaml) — production-ready dual-stack setup
- [`examples/ipv6/scylla-cluster-minimal-dual-stack.yaml`](https://github.com/scylladb/scylla-operator/blob/master/examples/ipv6/scylla-cluster-minimal-dual-stack.yaml) — minimal dual-stack example
- [`examples/ipv6/scylla-cluster-ipv6.yaml`](https://github.com/scylladb/scylla-operator/blob/master/examples/ipv6/scylla-cluster-ipv6.yaml) — IPv6 single-stack setup
