# Automatic data cleanup

This document explains the automatic data cleanup feature in {{productName}}.

## Overview

{{productName}} automates the execution of [node cleanup](https://docs.scylladb.com/manual/stable/operating-scylla/nodetool-commands/cleanup.html)
procedures following cluster scaling operations. This feature ensures that your ScyllaDB cluster maintains storage
efficiency and data integrity by removing stale data and preventing data resurrection.

:::{note}
While ScyllaDB performs automatic data cleanup for tablet-enabled keyspaces, it does not cover system or standard vnode-based
keyspaces. To bridge this gap, {{productName}} implements automatic cleanup for these remaining keyspaces.
:::

## Reasons for running cleanup

When you scale a ScyllaDB cluster horizontally (adding or removing nodes), the ownership of data tokens changes across
the cluster (topology changes). Data ownership is determined by the token ranges assigned to each node in the cluster.
Cleanups address the issues that may arise from the redistribution of token ownership:

1. **Stale data**: When a node loses ownership of certain tokens, the data associated with those tokens remains on the node's disk.
2. **Data resurrection**: If you do not remove this stale data, it can lead to data resurrection later.

## Cleanup triggering mechanism

Cleanup is triggered after scaling operations (adding or removing nodes/racks) are complete. It runs on all nodes that
have changed token ownership as a result of the scaling operation. This also includes the cluster bootstrap as nodes are
added one by one and the token ring changes with each addition.

During scale-out, cleanup runs on all nodes except the last one added, as it does not lose any tokens.

### How cleanup is triggered

{{productName}} tracks changes in the token ring of the cluster. When a mismatch is detected between the current state
of the ring and the last cleaned-up state, it triggers a cleanup job for the affected nodes. Before triggering
cleanup jobs, {{productName}} ensures the cluster is stable (conditions: `StatefulSetControllerProgressing=False`,
`Available=True`, and `Degraded=False`).

:::{note}
Because {{productName}} relies strictly on token ring changes, there are some limitations, which are described in the
[Known limitations](#known-limitations) section.
:::

## Inspecting cleanup jobs

{{productName}} creates a Job for each node that requires cleanup. If any job is still running, the `ScyllaCluster`
status contains the `JobControllerProgressing` condition set to `True`. When a job completes successfully, {{productName}} removes it
from the cluster. The condition's message contains the name of the running job(s).

When no cleanup jobs are running, the `JobControllerProgressing` condition is set to `False`.

You can inspect the condition by running:

```shell
kubectl get scyllacluster <sc-name> -o jsonpath='{.status.conditions[?(@.type=="JobControllerProgressing")]}' | jq
```

Where `<sc-name>` is the name of your ScyllaCluster.

You should see output similar to this (when no jobs are running):

```json
{
  "lastTransitionTime": "2025-12-12T13:51:52Z",
  "message": "",
  "observedGeneration": 2,
  "reason": "AsExpected",
  "status": "False",
  "type": "JobControllerProgressing"
}
```

:::{caution}
You may not see the cleanup jobs in the cluster if they complete quickly (e.g., on small datasets or cluster bootstrap)
and are removed before you can inspect them.

To ensure they were created and completed, you can inspect Kubernetes events:

```shell
kubectl get events | grep job
```

The output should contain entries similar to:

```shell
30m  Normal  JobCreated        job/cleanup-scylla-us-east-1-us-east-1a-0  Job default/cleanup-scylla-us-east-1-us-east-1a-0 created
30m  Normal  SuccessfulCreate  job/cleanup-scylla-us-east-1-us-east-1a-0  Created pod: cleanup-scylla-us-east-1-us-east-1a-0-tpd7x
30m  Normal  Completed         job/cleanup-scylla-us-east-1-us-east-1a-0  Job completed
30m  Normal  JobDeleted        job/cleanup-scylla-us-east-1-us-east-1a-0  Job default/cleanup-scylla-us-east-1-us-east-1a-0 deleted
```

You can see that for each cleanup job, there are events for job creation, pod creation, job completion, and job deletion.
:::

## Known limitations

{{productName}} triggers cleanup based on token ring changes. While this approach is safe, it has some side effects where
cleanup may be triggered unnecessarily or not at all.

### Unnecessary cleanups on node decommission

When you decommission a node, tokens are redistributed to the remaining nodes. These nodes do not require cleanup since
they are not losing any tokens. However, {{productName}} still triggers cleanup for these nodes due to the token ring change.
This is a safe operation, but it may lead to an unnecessary spike in I/O load.

### Missed cleanup on RF changes

When you decrease the replication factor (RF) of a keyspace, the token ring remains unchanged. Thus, {{productName}} does
not detect the need for cleanup. You should trigger [cluster cleanup](https://docs.scylladb.com/manual/stable/operating-scylla/nodetool-commands/cluster/cleanup.html)
manually in such cases (run on any node):

```shell
kubectl exec -it service/<sc-name>-client -c scylla -- nodetool cluster cleanup
```

Where `<sc-name>` correspond to your ScyllaCluster name.
