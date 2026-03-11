# Known issues

This page lists known issues, platform-specific caveats, and feature limitations in ScyllaDB Operator. For issues specific to broken Kubernetes environments, see [Supported Kubernetes environments](releases.md#supported-kubernetes-environments).

## ScyllaDBCluster (multi-DC) limitations

ScyllaDBCluster (`v1alpha1`) is a **technical preview**. The following features are not yet available or documented for multi-datacenter deployments:

| Feature | Status | Tracking issue |
|---|---|---|
| Helm-based installation | Not documented | [#2856](https://github.com/scylladb/scylla-operator/issues/2856) |
| Alternator (DynamoDB-compatible API) | Not documented | [#2856](https://github.com/scylladb/scylla-operator/issues/2856) |
| Monitoring | Not documented | [#2856](https://github.com/scylladb/scylla-operator/issues/2856) |
| mTLS | Not documented | [#2856](https://github.com/scylladb/scylla-operator/issues/2856) |

## TLS

### Node-to-node mTLS not supported

The `AutomaticTLSCertificates` feature gate provisions client-to-node TLS certificates. Node-to-node mTLS (inter-node encryption) is not yet supported.

**Tracking issue**: [#2434](https://github.com/scylladb/scylla-operator/issues/2434)

## IPv6

### IPv6-only single-stack is experimental

IPv6-only single-stack networking (`ipFamilies: [IPv6]` with `ipFamilyPolicy: SingleStack`) is experimental and not recommended for production use. Use dual-stack instead.

**Tracking issue**: [#3211](https://github.com/scylladb/scylla-operator/issues/3211)

See [IPv6 configuration reference](ipv6-configuration.md#production-readiness) for the full production readiness matrix.

### Unsupported IPv6 configurations

- Different IP families across datacenters in a multi-datacenter deployment.
- Changing the IP family of an existing cluster requires cluster recreation.

## Automatic data cleanup

### Replication factor changes are not detected

The Operator tracks token ring hash changes to trigger automatic cleanup after scaling operations. However, decreasing a keyspace's replication factor does not change the token ring. In this case, you must run `nodetool cleanup` manually on each node.

See [Automatic data cleanup](../understand/automatic-data-cleanup.md) for details.

### Unnecessary cleanup after decommission

When a node is decommissioned, cleanup is triggered on the remaining nodes even though they don't strictly need it. This is safe but adds temporary I/O load.

## nodetool operations

### `nodetool move` is not supported

Using `nodetool move` changes token ownership without the Operator's knowledge and is not supported. Use scaling (add/remove nodes) to rebalance the cluster.

### `nodetool rebuild` has no Operator alternative

If `nodetool rebuild` is needed (for example, when adding a new datacenter), it must be coordinated manually. Ensure no Operator-managed operations are in progress before running it.

See [nodetool alternatives](nodetool-alternatives.md) for the complete list of safe and unsafe `nodetool` commands.

## Platform-specific issues

### Minikube: TRUNCATE queries fail

`TRUNCATE` queries require [hairpinning](https://en.wikipedia.org/wiki/Hairpinning) to be enabled. On Minikube this is disabled by default.

**Workaround**:

```shell
minikube ssh sudo ip link set docker0 promisc on
```

### Minikube: ScyllaDB Manager fails to boot

If ScyllaDB Manager fails to apply the 8th migration (`008_*`), the cause is the same hairpinning issue. Apply the `TRUNCATE` workaround above.

### EKS: webhook connectivity with custom CNI

On EKS clusters using a custom CNI plugin, webhook connectivity can break because the API server may be unable to reach webhook pods. This is an upstream issue.

**Reference**: [aws/containers-roadmap#1215](https://github.com/aws/containers-roadmap/issues/1215)

See [Troubleshoot installation issues](../troubleshoot/troubleshoot-installation.md) for diagnosis steps.

### GKE: private clusters require a firewall rule

GKE private clusters restrict communication from the API server to node pods. A firewall rule must be added to allow the API server to reach the webhook pod on the serving port.

See [Troubleshoot installation issues](../troubleshoot/troubleshoot-installation.md) for the firewall rule configuration.
