## Version  migrations


### `v0.3.0` -> `v1.0.0` migration

`v0.3.0` used a very common name as a CRD kind (`Cluster`). In `v1.0.0` this issue was solved by using less common kind
which is easier to disambiguate (`ScyllaCluster`).
***This change is backward incompatible, which means manual migration is needed.***

This procedure involves having two CRDs registered at the same time. We will detach Scylla Pods
from Scylla Operator for a short period to ensure that nothing is garbage collected when Scylla Operator is upgraded.
Compared to the [upgrade guide](upgrade.md) where full deletion is requested, this procedure shouldn't cause downtimes.
Although detaching resources from their controller is considered hacky. This means that you shouldn't run procedure
out of the box on production. Make sure this procedure works well multiple times on your staging environment first.

***Read the whole procedure and make sure you understand what is going on before executing any of the commands!***

In case of any issues or questions regarding this procedure, you're welcomed on our [Scylla Users Slack](http://slack.scylladb.com/)
on #kubernetes channel.

### Procedure

1. Execute this whole procedure for each cluster sequentially. To get a list of existing clusters execute the following
    ```
    kubectl -n scylla get cluster.scylla.scylladb.com

    NAME             AGE
    simple-cluster   30m
    ```
    All below commands will use `scylla` namespace and `simple-cluster` as a cluster name.
1. Make sure you're using v1.0.0 tag:
    ```
    git checkout v1.0.0
    ```
1. Upgrade your `cert-manager` to `v1.0.0`. If you installed it from a static file from this repo, simply execute the following:
   ```
    kubectl apply -f examples/common/cert-manager.yaml
   ```
    If your `cert-manager` was installed in another way, follow official instructions on `cert-manager` website.
1. `examples/common/operator.yaml` file contains multiple resources. Extract **only** `CustomResourceDefinition` to separate file.
1. Install v1.0.0 CRD definition from file created in the previous step:
    ```
    kubectl apply -f examples/common/crd.yaml
    ```
1. Save your existing `simple-cluster` Cluster definition to a file:
    ```
    kubectl -n scylla get cluster.scylla.scylladb.com simple-cluster -o yaml > existing-cluster.yaml
    ```
1. Migrate `Kind` and `ApiVersion` to new values using:
    ```
    sed -i 's/scylla.scylladb.com\/v1alpha1/scylla.scylladb.com\/v1/g' existing-cluster.yaml
    sed -i 's/kind: Cluster/kind: ScyllaCluster/g' existing-cluster.yaml
    ```
1. Install migrated CRD instance
    ```
    kubectl apply -f existing-cluster.yaml
    ```
    At this point, we should have two CRDs describing your Scylla cluster, although the new one is not controlled by the Operator.
1. Get UUID of newly created ScyllaCluster resource:
    ```
    kubectl -n scylla get ScyllaCluster simple-cluster --template="{{ .metadata.uid }}"

    12a3678d-8511-4c9c-8a48-fa78d3992694
    ```
    Save output UUID somewhere, it will be referred as `<new-cluster-uid>` in commands below.

   ***Depending on your shell, you might get additional '%' sign at the end of UUID, make sure to remove it!***

1. Upgrade ClusterRole attached to each of the Scylla nodes to grant them permission to lookup Scylla clusters:
    ```
    kubectl patch ClusterRole simple-cluster-member --type "json" -p '[{"op":"add","path":"/rules/-","value":{"apiGroups":["scylla.scylladb.com"],"resources":["scyllaclusters"],"verbs":["get"]}}]'
    ```
    Amend role name according to your cluster name, it should look like `<scylla-cluster-name>-member`.
1. Get a list of all Services associated with your cluster. First get list of all services:
   ```
    kubectl -n scylla get svc -l "scylla/cluster=simple-cluster"

    NAME                                    TYPE        CLUSTER-IP     EXTERNAL-IP   PORT(S)                                                           AGE
    simple-cluster-client                   ClusterIP   None           <none>        9180/TCP                                                          109m
    simple-cluster-us-east-1-us-east-1a-0   ClusterIP   10.43.23.96    <none>        7000/TCP,7001/TCP,7199/TCP,10001/TCP,9042/TCP,9142/TCP,9160/TCP   109m
    simple-cluster-us-east-1-us-east-1a-1   ClusterIP   10.43.66.22    <none>        7000/TCP,7001/TCP,7199/TCP,10001/TCP,9042/TCP,9142/TCP,9160/TCP   108m
    simple-cluster-us-east-1-us-east-1a-2   ClusterIP   10.43.246.25   <none>        7000/TCP,7001/TCP,7199/TCP,10001/TCP,9042/TCP,9142/TCP,9160/TCP   106m

   ```
1. For each service, change its `ownerReference` to point to new CRD instance:
   ```
    kubectl -n scylla patch svc <cluster-svc-name> --type='json' -p='[{"op": "replace", "path": "/metadata/ownerReferences/0/apiVersion", "value":"scylla.scylladb.com/v1"}, {"op": "replace", "path": "/metadata/ownerReferences/0/kind", "value":"ScyllaCluster"}, {"op": "replace", "path": "/metadata/ownerReferences/0/uid", "value":"<new-cluster-uid>"}]'
   ```
    Replace `<cluster-svc-name>` with Service name, and `<new-cluster-uid>` with saved UUID from one of the previous steps.
1. Get a list of all Services again to see if none was deleted. Check also "Age" column, it shouldn't be lower than previous result.
   ```
    kubectl -n scylla get svc -l "scylla/cluster=simple-cluster"

    NAME                                    TYPE        CLUSTER-IP     EXTERNAL-IP   PORT(S)                                                           AGE
    simple-cluster-client                   ClusterIP   None           <none>        9180/TCP                                                          110m
    simple-cluster-us-east-1-us-east-1a-0   ClusterIP   10.43.23.96    <none>        7000/TCP,7001/TCP,7199/TCP,10001/TCP,9042/TCP,9142/TCP,9160/TCP   110m
    simple-cluster-us-east-1-us-east-1a-1   ClusterIP   10.43.66.22    <none>        7000/TCP,7001/TCP,7199/TCP,10001/TCP,9042/TCP,9142/TCP,9160/TCP   109m
    simple-cluster-us-east-1-us-east-1a-2   ClusterIP   10.43.246.25   <none>        7000/TCP,7001/TCP,7199/TCP,10001/TCP,9042/TCP,9142/TCP,9160/TCP   107m

   ```
1. Get a list of StatefulSets associated with your cluster:
    ```
    kubectl -n scylla get sts -l "scylla/cluster=simple-cluster"

    NAME                                  READY   AGE
    simple-cluster-us-east-1-us-east-1a   3/3     104m
    ```
1. For each StatefulSet from previous step, change its `ownerReference` to point to new CRD instance.

   ```
    kubectl -n scylla patch sts <cluster-sts-name> --type='json' -p='[{"op": "replace", "path": "/metadata/ownerReferences/0/apiVersion", "value":"scylla.scylladb.com/v1"}, {"op": "replace", "path": "/metadata/ownerReferences/0/kind", "value":"ScyllaCluster"}, {"op": "replace", "path": "/metadata/ownerReferences/0/uid", "value":"<new-cluster-uid>"}]'
   ```
    Replace `<cluster-sts-name>` with StatefulSet name, and `<new-cluster-uid>` with saved UUID from one of the previous steps.

1. Now when all k8s resources bound to Scylla are attached to new CRD, we can remove 0.3.0 Operator and old CRD definition.
    Checkout `v0.3.0` version, and remove Scylla Operator, and old CRD:
   ```
    git checkout v0.3.0
    kubectl delete -f examples/generic/operator.yaml
   ```
1. Checkout `v1.0.0`, and install upgraded Scylla Operator:
   ```
    git checkout v1.0.0
    kubectl apply -f examples/common/operator.yaml
   ```
1. Wait until Scylla Operator boots up:
   ```
    kubectl -n scylla-operator-system wait --for=condition=ready pod --all --timeout=600s
   ```
1. Get a list of StatefulSets associated with your cluster:
    ```
    kubectl -n scylla get sts -l "scylla/cluster=simple-cluster"

    NAME                                  READY   AGE
    simple-cluster-us-east-1-us-east-1a   3/3     104m
1. For each StatefulSet from previous step, change its sidecar container image to `v1.0.0`, and wait until change will be propagated. This step will initiate a rolling restart of pods one by one.
    ```
    kubectl -n scylla patch sts <cluster-sts> --type='json' -p='[{"op": "replace", "path": "/spec/template/spec/initContainers/0/image", "value":"scylladb/scylla-operator:v1.0.0"}]'
    kubectl -n scylla rollout status sts <cluster-sts>
    ```
    Replace `<cluster-sts-name>` with StatefulSet name.
1. If you're using Scylla Manager, bump Scylla Manager Controller image to `v1.0.0`
   ```
    kubectl -n scylla-manager-system patch sts scylla-manager-controller --type='json' -p='[{"op": "replace", "path": "/spec/template/spec/containers/0/image", "value":"scylladb/scylla-operator:v1.0.0"}]'
   ```
1. Your Scylla cluster is now migrated to `v1.0.0`.
