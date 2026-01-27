Wait for CRDs to propagate to all apiservers:
:::{code-block} shell
kubectl wait --for='condition=established' crd/scyllaclusters.scylla.scylladb.com crd/nodeconfigs.scylla.scylladb.com crd/scyllaoperatorconfigs.scylla.scylladb.com crd/scylladbmonitorings.scylla.scylladb.com
:::

**Expected output**:
:::{code-block} console
customresourcedefinition.apiextensions.k8s.io/scyllaclusters.scylla.scylladb.com condition met
customresourcedefinition.apiextensions.k8s.io/nodeconfigs.scylla.scylladb.com condition met
customresourcedefinition.apiextensions.k8s.io/scyllaoperatorconfigs.scylla.scylladb.com condition met
customresourcedefinition.apiextensions.k8s.io/scylladbmonitorings.scylla.scylladb.com condition met
:::

Wait for the components to roll out:
:::{code-block} shell
kubectl -n=scylla-operator rollout status --timeout=10m deployment.apps/{scylla-operator,webhook-server}
:::

**Expected output**:
:::{code-block} console
deployment "scylla-operator" successfully rolled out
deployment "webhook-server" successfully rolled out
:::

