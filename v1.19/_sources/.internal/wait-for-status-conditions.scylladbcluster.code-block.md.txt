:::{code-block} bash
kubectl --context=${CONTROL_PLANE_CONTEXT} wait --for='condition=Progressing=False' scylladbcluster.scylla.scylladb.com/dev-cluster
kubectl --context=${CONTROL_PLANE_CONTEXT} wait --for='condition=Degraded=False' scylladbcluster.scylla.scylladb.com/dev-cluster
kubectl --context=${CONTROL_PLANE_CONTEXT} wait --for='condition=Available=True' scylladbcluster.scylla.scylladb.com/dev-cluster
:::