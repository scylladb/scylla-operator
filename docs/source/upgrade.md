# Upgrade of Scylla Operator

This pages describes Scylla Operator upgrade procedures.

## `v1.1.0` -> `v1.2.0`

1.2.0 release brought a lot of changes to the Scylla Operator deployment process.
To properly update Scylla Operator one must delete old objects and install updated ones.

Sidecar image is going to be upgraded automatically, so a rolling restart of your Scylla clusters is expected during the upgrade procedure.

1. Checkout source code of v1.2.0:
   ```
   git checkout v1.2.0
   ```
1. Remove old scylla operator namespace - in our case it's called `scylla-operator-system`:
    ```
    kubectl delete namespace scylla-operator-system --wait=true
    ```
1. Remove old webhooks:
    ```
    kubectl delete MutatingWebhookConfiguration scylla-operator-mutating-webhook-configuration
    kubectl delete ValidatingWebhookConfiguration scylla-operator-validating-webhook-configuration
    ```
1. Install Scylla Operator from deploy directory:
    ```
    kubectl -n scylla-operator apply -f deploy/operator
    ```
1. Wait until Scylla Operator is up and running:
    ```
    kubectl wait --for condition=established crd/scyllaclusters.scylla.scylladb.com
    kubectl -n scylla-operator rollout status deployment.apps/scylla-operator
    ```

## `v1.0.0` -> `v1.1.0`

During this update we will change probes and image for Scylla Operator.
A new version brings an automation for upgrade of sidecar image, so a rolling restart of managed Scylla clusters is expected.

1. Get name of StatefulSet managing Scylla Operator
   ```shell
   kubectl --namespace scylla-operator-system get sts --selector="control-plane=controller-manager"

   NAME                                 READY   AGE
   scylla-operator-controller-manager   1/1     95m
   ```

1. Change probes and used container image by applying following patch:
    ```yaml
    spec:
      template:
        spec:
          containers:
          - name: manager
            image: docker.io/scylladb/scylla-operator:1.1.0
            livenessProbe:
              httpGet:
                path: /healthz
                port: 8080
                scheme: HTTP
            readinessProbe:
              $retainKeys:
              - httpGet
              httpGet:
                path: /readyz
                port: 8080
                scheme: HTTP
    ```
    To apply above patch save it to file (`operator-patch.yaml` for example) and apply to Operator StatefulSet:
    ```shell
    kubectl -n scylla-operator-system patch sts scylla-operator-controller-manager --patch "$(cat operator-patch.yaml)"
    ```


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
