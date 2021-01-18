# Upgrade of Scylla Operator

This pages describes Scylla Operator upgrade procedures.

## `v0.3.0` -> `v1.0.0`

***Note:*** There's an experimental migration procedure available [here](migration.md).

`v0.3.0` used a very common name as a CRD kind (`Cluster`). In `v1.0.0` this issue was solved by using less common
kind which is easier to disambiguate. (`ScyllaCluster`).
This change is backward incompatible, so Scylla cluster must be turned off and recreated from scratch.
In case you need to preserve your data, refer to backup and restore guide.

1. Get list of existing Scylla clusters
    ```
    kubectl -n scylla get cluster.scylla.scylladb.com

    NAME             AGE
    simple-cluster   30m
    ```
1. Delete each one of them

    ```
    kubectl -n scylla delete cluster.scylla.scylladb.com simple-cluster
    ```
1. Make sure you're on `v0.3.0` branch
    ```
    git checkout v0.3.0
    ```
1. Delete existing CRD and Operator
    ```
    kubectl delete -f examples/generic/operator.yaml
    ```
1. Checkout `v1.0.0` version
    ```
    git checkout v1.0.0
    ```
1. Install new CRD and Scylla Operator
    ```
    kubectl apply -f examples/common/operator.yaml
    ```
1. Migrate your existing Scylla Cluster definition. Change `apiVersion` and `kind` from:
    ```
    apiVersion: scylla.scylladb.com/v1alpha1
    kind: Cluster
    ```
   to:
    ```
    apiVersion: scylla.scylladb.com/v1
    kind: ScyllaCluster
    ```
1. Once your cluster definition is ready, use `kubectl apply` to install fresh Scylla cluster.

