# Install ScyllaDB Operator in Red Hat OpenShift

ScyllaDB Operator is a Red Hat OpenShift Certified Operator available for installation through the embedded software catalog,
a user interface for discovering Operators working in conjunction with Operator Lifecycle Manager (OLM), which installs and manages Operators in a cluster.
This guide describes how to install ScyllaDB Operator from the software catalog in a Red Hat OpenShift cluster.

## Prerequisites

This guide requires you to have access to an OpenShift Container Platform cluster using an account with `cluster-admin` permissions.

Ensure that your OpenShift cluster meets the [general prerequisites](https://operator.docs.scylladb.com/v1.20/installation/kubernetes-prerequisites.md) for ScyllaDB Operator installation.

Additionally, ensure that you have `kubectl` or OpenShift CLI (`oc`) installed.

## Install ScyllaDB Operator

ScyllaDB Operator can be installed in a Red Hat OpenShift cluster from the software catalog through one of the below methods: OpenShift Container Platform web console or CLI.

Web Console

This procedure describes how to install and subscribe to ScyllaDB Operator from software catalog by using the OpenShift Container Platform web console.

#### NOTE
This procedure aims to follow the generic Operator installation steps outlined in the upstream documentation: [Installing from the software catalog by using the web console](https://docs.redhat.com/en/documentation/openshift_container_platform/4.20/html/operators/administrator-tasks#olm-installing-from-software-catalog-using-web-console_olm-adding-operators-to-a-cluster).
You can refer to it for more detailed instructions, or follow the below steps for a more contained, ScyllaDB Operator specific guidance.

### Procedure

1. Navigate to `Ecosystem` -> `Software Catalog`.
2. Search for ScyllaDB Operator and select the `Certified` version by setting the `Source` filter to `Certified` or ensuring that the ScyllaDB Operator tile has the `Certified` tag next to it.
3. In the description dialog, read the information about the operator and click `Install`.
4. In the `Install Operator` dialog, configure your ScyllaDB Operator installation:
   * For clusters on cloud providers with token-based authentication:
     - AWS Security Token Service (STS Mode in the web console): enter the Amazon Resource Name (ARN) of the AWS IAM role of your service account in the role ARN field.
   * Select the `stable` Update Channel and version 1.20.3 from the list.
   * Select the `All namespaces on the cluster` installation mode.
   * Select the Operator recommended installed namespace: `scylla-operator`.

     #### WARNING
     ScyllaDB Operator must run in a reserved `scylla-operator` namespace. [It is not currently possible](https://github.com/scylladb/scylla-operator/issues/2563) to run it in a different namespace.
   * Select the `Manual` update approval strategy to manually approve ScyllaDB Operator upgrades when new versions are available.
5. Click `Install` to install and subscribe to ScyllaDB Operator.
6. In the `Install Plan` dialog, review the manual install plan and click `Approve` to approve it.
7. Log in to the OpenShift cluster in the terminal. Ensure that `kubectl` is configured to communicate with your OpenShift cluster.

CLI

This procedure describes how to install and subscribe to ScyllaDB Operator from software catalog by using the CLI.

#### NOTE
This procedure aims to follow the generic Operator installation steps outlined in the upstream documentation: [Installing from the software catalog by using the CLI](https://docs.redhat.com/en/documentation/openshift_container_platform/4.20/html/operators/administrator-tasks#olm-installing-operator-from-software-catalog-using-cli_olm-adding-operators-to-a-cluster).
You can refer to it for more detailed instructions, or follow the below steps for a more contained, ScyllaDB Operator specific guidance.

### Procedure

1. Log in to the OpenShift cluster in the terminal. Ensure that `kubectl` is configured to communicate with your OpenShift cluster.
2. Ensure that the ScyllaDB Operator package is available:
   ```bash
   kubectl get -n=openshift-marketplace packagemanifest scylladb-operator
   ```

   **Expected output**:
   ```console
   NAME                CATALOG               AGE
   scylladb-operator   Certified Operators   2d22h
   ```
3. Create the `scylla-operator` namespace:

   #### WARNING
   ScyllaDB Operator must run in a reserved `scylla-operator` namespace. [It is not currently possible](https://github.com/scylladb/scylla-operator/issues/2563) to run it in a different namespace.

   ```bash
   kubectl create namespace scylla-operator
   ```
4. Create an `OperatorGroup` object in the `scylla-operator` namespace:
   ```bash
   kubectl apply --server-side -n=scylla-operator -f=- <<EOF
   apiVersion: operators.coreos.com/v1
   kind: OperatorGroup
   metadata:
     name: scylladb-operator
     namespace: scylla-operator
   EOF
   ```
5. Create a `Subscription` object to install and subscribe to ScyllaDB Operator:

   Clusters on cloud providers with token-based authentication require you to include relevant cloud-provider specific fields in the `Subscription` object’s config section.
   Apply the appropriate manifest based on your environment. In the below table, manifests are provided for AWS STS mode and generic installations.

   AWS STS

   If the cluster is in AWS STS mode, include the role ARN details in the `Subscription` manifest.
   ```bash
   kubectl apply --server-side -n=scylla-operator -f=- <<EOF
   apiVersion: operators.coreos.com/v1alpha1
   kind: Subscription
   metadata:
     name: scylladb-operator
     namespace: scylla-operator
   spec:
     channel: stable
     installPlanApproval: Manual
     name: scylladb-operator
     source: certified-operators
     sourceNamespace: openshift-marketplace
     startingCSV: scylladb-operator.v1.20.3
     config:
       env:
       - name: ROLEARN
         value: "<role_arn>"
   EOF
   ```

   Generic
   ```bash
   kubectl apply --server-side -n=scylla-operator -f=- <<EOF
   apiVersion: operators.coreos.com/v1alpha1
   kind: Subscription
   metadata:
     name: scylladb-operator
     namespace: scylla-operator
   spec:
     channel: stable
     installPlanApproval: Manual
     name: scylladb-operator
     source: certified-operators
     sourceNamespace: openshift-marketplace
     startingCSV: scylladb-operator.v1.20.3
   EOF
   ```
6. Review and approve the generated `InstallPlan` to start the installation:
   - Locate the generated `InstallPlan` object and note its name:
     ```bash
     kubectl -n=scylla-operator get installplan -l=operators.coreos.com/scylladb-operator.scylla-operator=""
     ```

     **Example expected output**:
     ```console
     NAME            CSV                         APPROVAL   APPROVED
     install-tw6bv   scylladb-operator.v1.20.0   Manual     true
     ```
   - Describe the `InstallPlan` object and review the changes that will be applied:
     ```bash
     kubectl -n=scylla-operator describe installplan <install-plan-name>
     ```
   - Approve the reviewed `InstallPlan` to start the installation:
     ```bash
     kubectl -n=scylla-operator patch installplan --type=merge -p='{"spec":{"approved":true}}' <install-plan-name>
     ```

     **Example expected output**:
     ```console
     installplan.operators.coreos.com/install-tw6bv patched
     ```
7. Wait for the ScyllaDB Operator to be installed:

   Wait for `ClusterServiceVersion` to be created:
   ```bash
   kubectl -n=scylla-operator wait --for=create --timeout=10m csv/scylladb-operator.v1.20.3 
   ```

   **Expected output**:
   ```console
   clusterserviceversion.operators.coreos.com/scylladb-operator.v1.20.3 condition met
   ```

   Wait for `ClusterServiceVersion` to reach `Succeeded` phase:
   ```bash
   kubectl wait -n=scylla-operator --timeout=5m --for=jsonpath='{.status.phase}'=Succeeded clusterserviceversions.operators.coreos.com/scylladb-operator.v1.20.3
   ```

   **Expected output**:
   ```console
   clusterserviceversion.operators.coreos.com/scylladb-operator.v1.20.3 condition met
   ```

### Verify the ScyllaDB Operator installation

Wait for CRDs to propagate to all apiservers:

```shell
kubectl wait --for='condition=established' crd/scyllaclusters.scylla.scylladb.com crd/nodeconfigs.scylla.scylladb.com crd/scyllaoperatorconfigs.scylla.scylladb.com crd/scylladbmonitorings.scylla.scylladb.com
```

**Expected output**:

```console
customresourcedefinition.apiextensions.k8s.io/scyllaclusters.scylla.scylladb.com condition met
customresourcedefinition.apiextensions.k8s.io/nodeconfigs.scylla.scylladb.com condition met
customresourcedefinition.apiextensions.k8s.io/scyllaoperatorconfigs.scylla.scylladb.com condition met
customresourcedefinition.apiextensions.k8s.io/scylladbmonitorings.scylla.scylladb.com condition met
```

Wait for the components to roll out:

```shell
kubectl -n=scylla-operator rollout status --timeout=10m deployment.apps/{scylla-operator,webhook-server}
```

**Expected output**:

```console
deployment "scylla-operator" successfully rolled out
deployment "webhook-server" successfully rolled out
```

## Set up local storage on dedicated nodes and enable tuning

ScyllaDB Operator enables local storage configuration and performance tuning through the [`NodeConfig`](https://operator.docs.scylladb.com/v1.20/resources/nodeconfigs.md) resource.
The below table contains example `NodeConfig` manifests for a selected set of OpenShift deployment models and platforms:

- A production-ready configuration for Red Hat OpenShift Service on AWS (ROSA) clusters with local NVMe storage.
- A development-oriented configuration using loop devices intended for environments with no local disks available.

#### NOTE
Performance tuning is enabled for all nodes that are selected by `NodeConfig` by default, unless explicitly opted-out of.

ROSA (NVMe)

The following manifest creates a RAID0 array from the available NVMe devices, formats it with the XFS filesystem, and enables performance tuning recommended for production-grade ScyllaDB deployments on the selected nodes.

```yaml
apiVersion: scylla.scylladb.com/v1alpha1
kind: NodeConfig
metadata:
  name: scylladb-nodepool-1
spec:
  localDiskSetup:
    filesystems:
    - device: /dev/md/nvmes
      type: xfs
    mounts:
    - device: /dev/md/nvmes
      mountPoint: /var/lib/persistent-volumes
      unsupportedOptions:
      - prjquota
    raids:
    - name: nvmes
      type: RAID0
      RAID0:
        devices:
          modelRegex: Amazon EC2 NVMe Instance Storage
          nameRegex: ^/dev/nvme\d+n\d+$
  sysctls:
  - name: fs.aio-max-nr
    value: "30000000"
  - name: fs.file-max
    value: "9223372036854775807"
  - name: fs.nr_open
    value: "1073741816"
  - name: fs.inotify.max_user_instances
    value: "1200"
  - name: vm.swappiness
    value: "1"
  - name: vm.vfs_cache_pressure
    value: "2000"
  placement:
    nodeSelector:
      scylla.scylladb.com/node-type: scylla
    tolerations:
    - effect: NoSchedule
      key: scylla-operator.scylladb.com/dedicated
      operator: Equal
      value: scyllaclusters
```

You can apply it using the following command:

```shell
kubectl -n=scylla-operator apply --server-side -f=https://raw.githubusercontent.com/scylladb/scylla-operator/v1.20/examples/openshift/rosa/nodeconfig.yaml
```

Any platform (Loop devices)

The following manifest creates a loop device, formats it with the XFS filesystem, and enables performance tuning on the selected nodes.

```yaml
apiVersion: scylla.scylladb.com/v1alpha1
kind: NodeConfig
metadata:
  name: scylladb-nodepool-1
spec:
  localDiskSetup:
    loopDevices:
    - name: persistent-volumes
      imagePath: /var/lib/persistent-volumes.img
      size: 80Gi
    filesystems:
    - device: /dev/loops/persistent-volumes
      type: xfs
    mounts:
    - device: /dev/loops/persistent-volumes
      mountPoint: /var/lib/persistent-volumes
      unsupportedOptions:
      - prjquota
  sysctls:
  - name: fs.aio-max-nr
    value: "30000000"
  - name: fs.file-max
    value: "9223372036854775807"
  - name: fs.nr_open
    value: "1073741816"
  - name: fs.inotify.max_user_instances
    value: "1200"
  - name: vm.swappiness
    value: "1"
  - name: vm.vfs_cache_pressure
    value: "2000"
  placement:
    nodeSelector:
      scylla.scylladb.com/node-type: scylla
    tolerations:
    - effect: NoSchedule
      key: scylla-operator.scylladb.com/dedicated
      operator: Equal
      value: scyllaclusters
```

```shell
kubectl -n=scylla-operator apply --server-side -f=https://raw.githubusercontent.com/scylladb/scylla-operator/v1.20/examples/generic/nodeconfig-alpha.yaml
```

Having applied the `NodeConfig` manifest, wait for it to apply changes to the selected Kubernetes nodes:

```bash
kubectl wait --timeout=10m --for='condition=Progressing=False' nodeconfigs.scylla.scylladb.com/scylladb-nodepool-1
kubectl wait --timeout=10m --for='condition=Degraded=False' nodeconfigs.scylla.scylladb.com/scylladb-nodepool-1
kubectl wait --timeout=10m --for='condition=Available=True' nodeconfigs.scylla.scylladb.com/scylladb-nodepool-1
```

## Install Local CSI Driver

Local CSI Driver dynamically provisions PersistentVolumes on local storage configured on the nodes using the `NodeConfig` resource.
To install Local CSI Driver in a Red Hat OpenShift cluster, run the following command:

```shell
kubectl -n=local-csi-driver apply --server-side -f=https://raw.githubusercontent.com/scylladb/scylla-operator/v1.20/examples/common/local-volume-provisioner/local-csi-driver/{00_clusterrole_def,00_clusterrole_def_openshift,00_clusterrole,00_namespace,00_scylladb-local-xfs.storageclass,10_csidriver,10_serviceaccount,20_clusterrolebinding,50_daemonset}.yaml
```

Having applied the manifests, wait for the Local CSI Driver `DaemonSet` to be deployed:

```shell
kubectl -n=local-csi-driver rollout status --timeout=10m daemonset.apps/local-csi-driver
```

## Install ScyllaDB Manager

ScyllaDB Manager enables you to schedule second-day operations tasks, such as backups and repairs.

Run the following command to install a production-ready ScyllaDB Manager deployment:

#### WARNING
ScyllaDB Manager must run in a reserved `scylla-manager` namespace. It is [not currently possible](https://github.com/scylladb/scylla-operator/issues/2563) to use a different namespace for ScyllaDB Manager deployment.

```shell
kubectl -n=scylla-manager apply --server-side -f=https://raw.githubusercontent.com/scylladb/scylla-operator/v1.20/deploy/manager-prod.yaml
```

Having applied the manifest, wait for ScyllaDB Manager to deploy:

```shell
kubectl -n=scylla-manager rollout status --timeout=10m deployment.apps/scylla-manager
```

## Next steps

- Deploy a ScyllaDB cluster by following the [ScyllaCluster deployment guide](https://operator.docs.scylladb.com/v1.20/resources/scyllaclusters/basics.md).
- To set up ScyllaDB Monitoring, refer to [Setting up ScyllaDB Monitoring on OpenShift](https://operator.docs.scylladb.com/v1.20/management/monitoring/external-prometheus-on-openshift.md). Visit the [ScyllaDB Monitoring overview](https://operator.docs.scylladb.com/v1.20/management/monitoring/overview.md) for more information about the monitoring stack.
