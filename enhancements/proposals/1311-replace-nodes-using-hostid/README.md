# Replace ScyllaCluster nodes using Host ID

## Summary

The proposed change aims to improve the replacement procedure of ScyllaCluster nodes by adding new replacement procedure
and prepare ground for getting out of old one using deprecated ScyllaDB parameters. Both procedures will coexist in
Scylla Operator, users will have time to shift to a new one until the old one is removed in one of the following Scylla
Operator versions.

## Motivation

The motivation behind the proposed change is to have a replacement procedure that uses stable node identifiers.
Currently, the Scylla Operator uses IP addresses of a node to identify which node should be replaced. The issue is that
depending on the ScyllaCluster configuration, IP addresses might be ephemeral so it’s error prone to use it as an
identifier as they may change during when the procedure is executed. Because there is a better stable alternative, IP
address approach became deprecated, hence we need to shift to it.

### Goals

Add replace node procedure using stable Host IDs.
Remain backward compatible.

### Non-Goals

Remove existing replace procedure.

## Proposal

I propose to add a new node replacement procedure which is using stable Host ID identifiers. Both old and new will
coexist in Scylla Operator making it safer to implement, allow changes to be rolled out iteratively and without
compatibility issues during Operator upgrades/downgrades. To make users aware that old procedure is being deprecated, it
will be visible in release notes as well as a special Warning log message will be printed out in Operator logs telling
that user is using deprecated procedure and they should shift to the new one.

### Risks and Mitigations

#### ScyllaDB version requirement

Node replacement procedure alignment requires the newest 5.2 ScyllaDB Open Source and 2023.1 Enterprise, meaning node
replacement new procedure won’t work when Scylla doesn’t meet minimum version criteria. Operator will spin old procedure
for older versions, and new one for versions fulfilling these criteria.

## Design Details

### API changes

Status field dedicated to storing IP addresses of nodes being replaced is going to be deprecated.

```go
type RackStatus struct {
// DEPRECATED
// replace_address_first_boot holds addresses which should be replaced by new nodes.
ReplaceAddressFirstBoot map[string]string `json:"replace_address_first_boot,omitempty"`
}
```

Field will be used until the old procedure is removed in at least the next version. After it’s gone, the map will always
be empty.

### Procedure flow

#### Existing procedure

Current replace procedure consists of putting a `scylla/replace` label on a node dedicated Service. Operator detects
that
the user wants to replace this node, stores the IP address of the node to be replaced in `ScyllaCluster.status.racks[]
.replace_address_first_boot`.
After that, Service, Pod and PVC are removed, and new Service is created with a label `scylla/replace` having the IP
address of the node which is being replaced. Sidecar picks this value and propagates it to the
`replace-address-first-boot` parameter to ScyllaDB. When Pod becomes Ready, the label is removed from the Service and
status is cleared.

#### New procedure

Operator will trigger this procedure only when it detects it’s supported by the deployed ScyllaDB version, otherwise it
will fall back to the old procedure.
Instead of keeping HostID of a node being replaced in status, the Operator will keep the Service intact and sidecar will
use HostID
stored as an annotation to add a replace-node-first-boot parameter to ScyllaDB when the replace label is present.
ScyllaDB supports replacing a node with the same IP address, this won’t disturb replacing when the Operator is being
upgraded and versions are skewed.

When a Scylla Operator sees that a replacement label `scylla/replace` is added by the user to the node dedicated Service
and its value is empty, node Pod and associated PVC will be removed, and Service will be updated with
`internal.scylla-operator.scylladb.com/replacing-node-hostid` label with node Host ID as a value.
When the new Pod comes up, sidecar will propagate the value of HostID annotation from Service into
`replace-node-first-boot` parameter into ScyllaDB.
When Service Host ID annotation finally mismatches value stored in the replacement label, and Pod becomes ready, the
label will be removed ending the procedure.

## Test Plan

Existing E2E validating the replace procedure will stay, but used ScyllaDB version will be fixed to 5.1.x, and will be
extended with validation of ScyllaDB parameter used for replacement.
New E2Es will check whether new procedure with new ScyllaDB replace parameter is used for ScyllaDB OS versions >=5.2,
and Scylla Enterprise >=2023.1.

## Upgrade / Downgrade Strategy

Regular Operator upgrade/downgrade strategy.

## Version Skew Strategy

In case when an updated Operator is a leader, it will trigger replacing nodes using host ID.
When an old Operator would be a leader, it would fill out a `scylla/replace` label with the ClusterIP address of a node
being replaced and use the old procedure. Sidecar would replace nodes using addresses.

In case when the ScyllaDB version is not high enough, the Operator would fall back to old replace procedure.
In case when the ScyllaDB version would be changed in between Pod is removed and a new one is started, updated sidecar
would add the node replace parameter anyway. It will cause Scylla to fail, node wouldn’t be replaced, but new one
wouldn’t join either.
