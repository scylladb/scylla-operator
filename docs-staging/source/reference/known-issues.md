# Known issues

This page lists known issues, platform-specific caveats, and feature limitations in ScyllaDB Operator. For issues specific to Kubernetes environments, see [Supported Kubernetes environments](releases.md).

## Platform-specific issues

### TRUNCATE queries fail when hairpinning is disabled

`TRUNCATE` queries require [hairpinning](https://en.wikipedia.org/wiki/Hairpinning) to be enabled on the container network bridge. On some environments this is disabled by default.

**Learn more**: [#163](https://github.com/scylladb/scylla-operator/issues/163)

**Workaround**:

```shell
ip link set <bridge-interface> promisc on
```

### ScyllaDB Manager fails to boot when hairpinning is disabled

If ScyllaDB Manager fails to apply the 8th migration (`008_*`), the cause is the same hairpinning issue. Apply the workaround above.

### EKS: webhook connectivity with custom CNI

On EKS clusters using a custom CNI plugin, webhook connectivity can break because the API server may be unable to reach webhook pods. This is an upstream issue.

**Reference**: [aws/containers-roadmap#1215](https://github.com/aws/containers-roadmap/issues/1215)

See [Troubleshoot installation issues](../troubleshoot/troubleshoot-installation.md) for diagnosis steps.

### GKE: private clusters require a firewall rule

GKE private clusters restrict communication from the API server to node pods. A firewall rule must be added to allow the API server to reach the webhook pod on the serving port.

**Reference**: [GKE private clusters — add firewall rules](https://cloud.google.com/kubernetes-engine/docs/how-to/latest/network-isolation#add_firewall_rules)

See [Troubleshoot installation issues](../troubleshoot/troubleshoot-installation.md) for the firewall rule configuration.
