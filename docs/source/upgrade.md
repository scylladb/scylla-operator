# Upgrade of Scylla Operator

This page describes Scylla Operator upgrade procedures.  
There are two generic update procedures - via Helm and via kubectl. Before upgrading, please check this page to find out
if your target version requires additional upgrade steps.

## Upgrade via Helm

Helm doesn't support managing CustomResourceDefinition resources ([#5871](https://github.com/helm/helm/issues/5871), [#7735](https://github.com/helm/helm/issues/7735))   
These are only created on first install and never updated. In order to update them, users have to do it manually.

Replace `<release_name>` with the name of your Helm release for Scylla Operator and replace `<version>` with the version number you want to install:
1. Make sure Helm chart repository is up-to-date:
   ```
   helm repo add scylla-operator https://storage.googleapis.com/scylla-operator-charts/stable
   helm repo update
   ```
2. Update CRD resources. We recommend using `--server-side` flag for `kubectl apply`, if your version supports it.
   ```
   tmpdir=$( mktemp -d ) \
     && helm pull scylla-operator/scylla-operator --version <version> --untar --untardir "${tmpdir}" \
     && find "${tmpdir}"/scylla-operator/crds/ -name '*.yaml' -printf '-f=%p ' \
     | xargs kubectl apply
   ``` 
3. Update Scylla Operator
   ```
   helm upgrade --version <version> <release_name> scylla-operator/scylla-operator
   ```

## Upgrade via kubectl

Replace `<version>` with the version number you want to install:

1. Checkout source code of version you want to use:
   ```
   git checkout <version>
   ```
2. Manifests use rolling minor version tag, you may want to pin it to specific version:
   ```
   find deploy/operator -name "*.yaml" | xargs sed --follow-symlinks -i -E "s^docker.io/scylladb/scylla-operator:[0-9]+\.[0-9]+^docker.io/scylladb/scylla-operator:<version>^g"
   ```
3. Update Scylla Operator. We recommend using `--server-side` flag for `kubectl apply`, if your version supports it.
   ```
   kubectl apply -f deploy/operator
   ```

---

## `v1.2.0` -> `v1.3.0`

Sidecar image is going to be upgraded automatically, so a rolling restart of your Scylla clusters is expected during the upgrade procedure.

1. Checkout source code of v1.3.0:
   ```
   git checkout v1.3.0
   ```
1. Update Scylla Operator from deploy directory:
    ```
    kubectl -n scylla-operator apply -f deploy/operator
    ```
1. Wait until Scylla Operator is up and running:
    ```
    kubectl wait --for condition=established crd/scyllaclusters.scylla.scylladb.com
    kubectl -n scylla-operator rollout status deployment.apps/scylla-operator
    ```

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

## `v1.7.0` -> `v1.8.0`

`v1.8.0` introduces new Manager Controller, which unfortunately is **not** backward compatible.
The Manager has to be manually migrated.  

1. Make sure you're on `v1.7.0` branch
   ```
   git checkout v1.7.0
   ```
2. Delete existing manager and its controller
   
   Note: You can keep scylla-manager-cluster to be reused by the new manager.
   ``` 
   kubectl delete -f examples/common/manager.yaml
   ```
   
3. Delete existing CRD and operator 
   ```
   kubectl delete -f examples/common/operator.yaml
   ```
4. Checkout `v1.8.0` version
   ```
   git checkout v1.8.0
   ```
5. Install new CRD and operator
   ```
   kubectl apply -f examples/common/operator.yaml 
   ```
6. Create ScyllaManager CRD. 
   - Fill in name and namespace. 
    
     Remember that Scylla Manager has to be in the same namespace as the clusters that are supposed to be managed. 
   ```yaml
   apiVersion: scylla.scylladb.com/v1alpha1
   kind: ScyllaManager
   metadata:
     name: scylla-manager
     namespace: example-manager
   spec: [...]
   ```
   - Fill in manager image, desired replicas, refresh time of tasks statuses and scyllaClusterSelector.
      
     Manager will register only the clusters which labels match the given selector.
   ```yaml
   apiVersion: scylla.scylladb.com/v1alpha1
   kind: ScyllaManager
   metadata:
    name: scylla-manager
    namespace: example-manager
   spec:
    replicas: 1
    image: docker.io/scylladb/scylla-manager:3.0.0
    taskStatusRefreshTime: 300000000000 # 300 seconds
    scyllaClusterSelector:
      matchLabels:
        example-manager: "true"
        managed: "true"
        managed-by: scylla-manager
   ```
   - Fill in manager database spec
   ```yaml
   apiVersion: scylla.scylladb.com/v1alpha1
   kind: ScyllaManager
   metadata:
     name: scylla-manager
     namespace: example-manager
   spec:
     replicas: 1
     image: docker.io/scylladb/scylla-manager:3.0.0
     taskStatusRefreshTime: 300000000000 # 300 seconds
     scyllaClusterSelector:
       matchLabels:
         example-manager: "true"
         managed: "true"
         managed-by: scylla-manager
   
     database:
       connection:
         server: scylla-manager-cluster-manager-dc-manager-rack-0.example-manager.svc
         username: cassandra
         password: cassandra
   
   ```
   - Move tasks scheduled by [Cluster CRD](scylla_cluster_crd.md) to Manager CRD.
   
     Notice that tasks scheduling changed `interval` -> `cron`.
   ```yaml
   apiVersion: scylla.scylladb.com/v1alpha1
   kind: ScyllaManager
   metadata:
     name: scylla-manager
     namespace: example-manager
   spec:
     replicas: 1
     image: docker.io/scylladb/scylla-manager:3.0.0
     taskStatusRefreshTime: 300000000000 # 300 seconds
     scyllaClusterSelector:
       matchLabels:
         example-manager: "true"
         managed: "true"
         managed-by: scylla-manager
   
     database:
       connection:
         server: scylla-manager-cluster-manager-dc-manager-rack-0.example-manager.svc
         username: cassandra
         password: cassandra
   
     repairs:
       - name: "weekly us-east-1 repair"
         intensity: "2"
         cron: "@weekly"
         dc: ["us-east-1"]
     backups:
       - name: "daily users backup"
         rateLimit: ["50"]
         location: ["s3:cluster-backups"]
         cron: "@daily"
         keyspace: ["users"]
       - name: "weekly full cluster backup"
         rateLimit: ["50"]
         location: ["s3:cluster-backups"]
         cron: "@weekly"
   ```
7. Create new Manager 
   ```
   kubectl apply -f manager.yaml
   ```
   The Operator shall register matching clusters in manager and schedule tasks.
   
   Manager Status can be tracked:
   ```
   kubectl get -n namespace ScyllaManager -o yaml -w 
   ```
### Upgrade Manager with helm

1. Make sure you're on `v1.7.0` branch
   ```
   git checkout v1.7.0
   ```
2. Delete existing manager, its controller and operator 

   Note: You can keep scylla-manager-cluster to be reused by the new manager.
   ``` 
   helm uninstall scylla-manager -n scylla-manager
   helm uninstall scylla-operator -n scylla-operator
   ```
3. Checkout `v1.8.0` version
   ```
   git checkout v1.8.0
   ```
4. Install new operator and CRDs
   ```
   helm install scylla-operator scylla/scylla-operator --create-namespace --namespace scylla-operator
   helm install scylla-manager scylla/scylla-manager --create-namespace --namespace scylla-manager
   ```