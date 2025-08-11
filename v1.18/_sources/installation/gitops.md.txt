# GitOps (kubectl)

## Disclaimer

The following commands reference manifests that come from the same repository as the source code is being built from.
This means we can't have a pinned reference to the latest release as that is a [chicken-egg problem](https://en.wikipedia.org/wiki/Chicken_or_the_egg). Therefore, we use a rolling tag (e.g., `{major}.{minor}` or `latest`) for the particular branch in our manifests.
:::{caution}
For production deployment, you should always replace the {{productName}} image in all the manifests that contain it with a stable (full version) reference.
We'd encourage you to use a sha reference, although using full-version tags is also fine.
:::


## Installation

### Prerequisites

{{productName}} has a few dependencies that you need to install to your cluster first.

In case you already have a supported version of each of these dependencies installed in your cluster, you can skip this part.

#### Cert Manager

% The form of this code block is a workaround to allow resolution of smv_current_version - https://github.com/scylladb/scylla-operator/issues/2752
{{"""
BEGIN_CODE_BLOCK
kubectl apply --server-side -f=https://raw.githubusercontent.com/REPO/BRANCH/examples/third-party/cert-manager.yaml
END_CODE_BLOCK
""".replace("REPO", repository).replace("BRANCH", env.config.smv_current_version).replace("BEGIN_CODE_BLOCK", ":::{code-block} shell").replace("END_CODE_BLOCK", ":::")}}

:::{code-block} shell

# Wait for CRDs to propagate to all apiservers.
kubectl wait --for condition=established --timeout=60s crd/certificates.cert-manager.io crd/issuers.cert-manager.io

# Wait for components that other steps depend on.
for deploy in cert-manager{,-cainjector,-webhook}; do
    kubectl -n=cert-manager rollout status --timeout=10m deployment.apps/"${deploy}"
done

# Wait for webhook CA secret to be created.
for i in {1..30}; do
    { kubectl -n=cert-manager get secret/cert-manager-webhook-ca && break; } || sleep 1
done
:::

#### Prometheus Operator

% The form of this code block is a workaround to allow resolution of smv_current_version - https://github.com/scylladb/scylla-operator/issues/2752
{{"""
BEGIN_CODE_BLOCK
kubectl -n=prometheus-operator apply --server-side -f=https://raw.githubusercontent.com/REPO/BRANCH/examples/third-party/prometheus-operator.yaml
END_CODE_BLOCK
""".replace("REPO", repository).replace("BRANCH", env.config.smv_current_version).replace("BEGIN_CODE_BLOCK", ":::{code-block} shell").replace("END_CODE_BLOCK", ":::")}}

:::{code-block} shell
# Wait for CRDs to propagate to all apiservers.
kubectl wait --for='condition=established' crd/prometheuses.monitoring.coreos.com crd/prometheusrules.monitoring.coreos.com crd/servicemonitors.monitoring.coreos.com

# Wait for prometheus operator deployment.
kubectl -n=prometheus-operator rollout status --timeout=10m deployment.apps/prometheus-operator
:::

### {{productName}}

Once you have the dependencies installed and available in your cluster, it is the time to install {{productName}}.

% The form of this code block is a workaround to allow resolution of smv_current_version - https://github.com/scylladb/scylla-operator/issues/2752
{{"""
BEGIN_CODE_BLOCK
kubectl -n=scylla-operator apply --server-side -f=https://raw.githubusercontent.com/REPO/BRANCH/deploy/operator.yaml
END_CODE_BLOCK
""".replace("REPO", repository).replace("BRANCH", env.config.smv_current_version).replace("BEGIN_CODE_BLOCK", ":::{code-block} shell").replace("END_CODE_BLOCK", ":::")}}

::::{caution}
{{productName}} deployment references its own image that it later runs alongside each ScyllaDB instance. Therefore, you have to also replace the image in the environment variable called `SCYLLA_OPERATOR_IMAGE`:
:::{code-block} yaml
:linenos:
:emphasize-lines: 16,19,20
apiVersion: apps/v1
kind: Deployment
metadata:
  name: scylla-operator
  namespace: scylla-operator
# ...
spec:
  # ...
  template:
    # ...
    spec:
      # ...
      containers:
      - name: scylla-operator
        # ...
        image: docker.io/scylladb/scylla-operator:1.14.0@sha256:8c75c5780e2283f0a8f9734925352716f37e0e7f41007e50ce9b1d9924046fa1
        env:
          # ...
        - name: SCYLLA_OPERATOR_IMAGE
          value: docker.io/scylladb/scylla-operator:1.14.0@sha256:8c75c5780e2283f0a8f9734925352716f37e0e7f41007e50ce9b1d9924046fa1
:::
The {{productName}} image value and the `SCYLLA_OPERATOR_IMAGE` shall always match.
Be careful not to use a rolling tag for any of them to avoid an accidental skew!
::::

:::{code-block} shell
# Wait for CRDs to propagate to all apiservers.
kubectl wait --for='condition=established' crd/scyllaclusters.scylla.scylladb.com crd/nodeconfigs.scylla.scylladb.com crd/scyllaoperatorconfigs.scylla.scylladb.com crd/scylladbmonitorings.scylla.scylladb.com

# Wait for the components to deploy.
kubectl -n=scylla-operator rollout status --timeout=10m deployment.apps/{scylla-operator,webhook-server}
:::

### Setting up local storage on nodes and enabling tuning

:::{caution}
The following step heavily depends on the platform that you use, the machine type, or the options chosen when creating a node pool.

Please review the [NodeConfig](../resources/nodeconfigs.md) and adjust it for your platform!
:::

:::::{tab-set}

::::{tab-item} GKE (NVMe)
% The form of this code block is a workaround to allow resolution of smv_current_version - https://github.com/scylladb/scylla-operator/issues/2752
{{"""
BEGIN_CODE_BLOCK
kubectl -n=scylla-operator apply --server-side -f=https://raw.githubusercontent.com/REPO/BRANCH/examples/gke/nodeconfig-alpha.yaml
END_CODE_BLOCK
""".replace("REPO", repository).replace("BRANCH", env.config.smv_current_version).replace("BEGIN_CODE_BLOCK", ":::{code-block} shell").replace("END_CODE_BLOCK", ":::")}}
::::

::::{tab-item} EKS (NVMe)
% The form of this code block is a workaround to allow resolution of smv_current_version - https://github.com/scylladb/scylla-operator/issues/2752
{{"""
BEGIN_CODE_BLOCK
kubectl -n=scylla-operator apply --server-side -f=https://raw.githubusercontent.com/REPO/BRANCH/examples/eks/nodeconfig-alpha.yaml
END_CODE_BLOCK
""".replace("REPO", repository).replace("BRANCH", env.config.smv_current_version).replace("BEGIN_CODE_BLOCK", ":::{code-block} shell").replace("END_CODE_BLOCK", ":::")}}
::::

::::{tab-item} Any platform (Loop devices)
:::{caution}
This NodeConfig sets up loop devices instead of NVMe disks and is only intended for development purposes when you don't have the NVMe disks available.
Do not expect meaningful performance with this setup.
:::

% The form of this code block is a workaround to allow resolution of smv_current_version - https://github.com/scylladb/scylla-operator/issues/2752
{{"""
BEGIN_CODE_BLOCK
kubectl -n=scylla-operator apply --server-side -f=https://raw.githubusercontent.com/REPO/BRANCH/examples/generic/nodeconfig-alpha.yaml
END_CODE_BLOCK
""".replace("REPO", repository).replace("BRANCH", env.config.smv_current_version).replace("BEGIN_CODE_BLOCK", ":::{code-block} shell").replace("END_CODE_BLOCK", ":::")}}
::::

:::::

:::{note}
Performance tuning is enabled for all nodes that are selected by [NodeConfig](../resources/nodeconfigs.md) by default, unless opted-out.
:::

:::{code-block} shell
# Wait for NodeConfig to apply changes to the Kubernetes nodes.
kubectl wait --for='condition=Reconciled' --timeout=10m nodeconfigs.scylla.scylladb.com/scylladb-nodepool-1
:::

### Local CSI driver

% The form of this code block is a workaround to allow resolution of smv_current_version - https://github.com/scylladb/scylla-operator/issues/2752
{{"""
BEGIN_CODE_BLOCK
kubectl -n=local-csi-driver apply --server-side -f=https://raw.githubusercontent.com/REPO/BRANCH/examples/common/local-volume-provisioner/local-csi-driver/{00_clusterrole_def,00_clusterrole_def_openshift,00_clusterrole,00_namespace,00_scylladb-local-xfs.storageclass,10_csidriver,10_serviceaccount,20_clusterrolebinding,50_daemonset}.yaml
END_CODE_BLOCK
""".replace("REPO", repository).replace("BRANCH", env.config.smv_current_version).replace("BEGIN_CODE_BLOCK", ":::{code-block} shell").replace("END_CODE_BLOCK", ":::")}}

:::{code-block} shell
# Wait for it to deploy.
kubectl -n=local-csi-driver rollout status --timeout=10m daemonset.apps/local-csi-driver
:::

### ScyllaDB Manager

:::{include} ../.internal/manager-license-note.md
:::

:::::{tab-set}

::::{tab-item} Production (sized)

% The form of this code block is a workaround to allow resolution of smv_current_version - https://github.com/scylladb/scylla-operator/issues/2752
{{"""
BEGIN_CODE_BLOCK
kubectl -n=scylla-manager apply --server-side -f=https://raw.githubusercontent.com/REPO/BRANCH/deploy/manager-prod.yaml
END_CODE_BLOCK
""".replace("REPO", repository).replace("BRANCH", env.config.smv_current_version).replace("BEGIN_CODE_BLOCK", ":::{code-block} shell").replace("END_CODE_BLOCK", ":::")}}

::::

::::{tab-item} Development (sized)

% The form of this code block is a workaround to allow resolution of smv_current_version - https://github.com/scylladb/scylla-operator/issues/2752
{{"""
BEGIN_CODE_BLOCK
kubectl -n=scylla-manager apply --server-side -f=https://raw.githubusercontent.com/REPO/BRANCH/deploy/manager-dev.yaml
END_CODE_BLOCK
""".replace("REPO", repository).replace("BRANCH", env.config.smv_current_version).replace("BEGIN_CODE_BLOCK", ":::{code-block} shell").replace("END_CODE_BLOCK", ":::")}}

::::

:::::

:::{code-block} shell
# Wait for it to deploy.
kubectl -n=scylla-manager rollout status --timeout=10m deployment.apps/scylla-manager
:::

## Next steps

Now that you've successfully installed {{productName}}, it's time to look at [how to run ScyllaDB](../resources/scyllaclusters/basics.md).
