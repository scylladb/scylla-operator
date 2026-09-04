To verify the cluster state, execute `nodetool status` in each of the ScyllaDB cluster pods:
:::{code-block} bash
NAMESPACE=<namespace>
CLUSTER_NAME=<cluster-name>

# List all ScyllaDB pods in the cluster.
pods=$(kubectl -n "${NAMESPACE}" get pods \
    -l scylla/cluster="${CLUSTER_NAME}" \
    -l scylla-operator.scylladb.com/pod-type=scylladb-node \
    -o name)

# Execute nodetool status in each pod.
for pod in $pods; do
    kubectl -n "${NAMESPACE}" exec "${pod}" -c scylla -- nodetool status
done
:::

All nodes should report all other nodes as `UN` (Up and Normal) in the output, e.g.:

:::{code-block} console
Datacenter: us-east-1
===========================
Status=Up/Down
|/ State=Normal/Leaving/Joining/Moving
-- Address        Load    Tokens Owns Host ID                              Rack      
UN 10.221.135.48  3.30 KB 256    ?    5dd7f301-62d7-4ab7-986a-e7ea9d21be4d us-east-1a
UN 10.221.140.203 3.48 KB 256    ?    2f725f88-33fa-4ca7-b366-fa35e63e7c72 us-east-1b
UN 10.221.150.121 3.67 KB 256    ?    7063a262-fa3f-4f69-8a60-720f464b1483 us-east-1c
:::
