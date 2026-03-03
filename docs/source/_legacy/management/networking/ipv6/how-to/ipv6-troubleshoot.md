# Troubleshoot IPv6 networking issues

This guide helps you diagnose and resolve common IPv6 networking issues in ScyllaDB clusters.

## Quick diagnostics

Run these commands to quickly assess your IPv6 configuration:

```bash
# Check pod IPv6 addresses
kubectl get pods -n scylla -l scylla-operator.scylladb.com/pod-type=scylladb-node -o wide
```

**Expected output for dual-stack:**
```
NAME                            READY   STATUS    IP
scylla-dual-stack-us-east-1a-0  2/2     Running   10.244.2.42
scylla-dual-stack-us-east-1a-1  2/2     Running   10.244.2.43
```

Pods show IPv4 addresses. IPv6 addresses are assigned but not displayed in `kubectl get pods`. To verify IPv6, check services or use the address detection command from the migration guide.

```bash
# Verify service IP families
kubectl get svc -o custom-columns=NAME:.metadata.name,IP-FAMILIES:.spec.ipFamilies,POLICY:.spec.ipFamilyPolicy -n scylla
```

**Expected output for dual-stack:**
```
NAME                            IP-FAMILIES        POLICY
scylla-dual-stack-client        [IPv4 IPv6]        PreferDualStack
scylla-dual-stack-us-east-1a-0  [IPv4 IPv6]        PreferDualStack
```

**Expected output for IPv6-only:**
```
NAME                          IP-FAMILIES   POLICY
scylla-ipv6-client            [IPv6]        SingleStack
scylla-ipv6-us-east-1a-0      [IPv6]        SingleStack
```

Services should show the expected IP families based on your configuration.

## Related documentation

- [Configure IPv6 networking](ipv6-configure.md)
- [Migrate to IPv6](ipv6-migrate.md)
- [IPv6 networking concepts](../concepts/ipv6-networking.md)
- [IPv6 configuration reference](../reference/ipv6-configuration.md)
