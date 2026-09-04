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
