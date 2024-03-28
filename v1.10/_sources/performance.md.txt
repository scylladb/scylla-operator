# Performance tuning

Scylla Operator 1.6 introduces a new experimental feature allowing users to optimize Kubernetes nodes.

## Node tuning

Starting from Operator 1.6, a new CRD called NodeConfig is available, allowing users to target Nodes which should be tuned.
When a Node is supposed to be optimized, the Scylla Operator creates a DaemonSet covering these Nodes.
Nodes matching the provided placement conditions will be subject to tuning.

Below example NodeConfig tunes nodes having `scylla.scylladb.com/node-type=scylla` label:
```
apiVersion: scylla.scylladb.com/v1alpha1
kind: NodeConfig
metadata:
 name: cluster
spec:
 placement:
   nodeSelector:
     scylla.scylladb.com/node-type: scylla
```
For more details about new CRD use:
```
kubectl explain nodeconfigs.scylla.scylladb.com/v1alpha1
```

For all optimizations we use a Python script available in the Scylla image called perftune. 
Perftune executes the performance optmizations like tuning the kernel, network, disk devices, spreading IRQs across CPUs and more.

Tuning consists of two separate optimizations: common node tuning, and tuning based on Scylla Pods and their resource assignment.
Node tuning is executed immediately. Pod tuning is executed when Scylla Pod lands on the same Node.

Scylla works most efficently when it's pinned to CPU and not interrupted. 
One of the most common causes of context-switching are network interrupts. Packets coming to a node need to be processed, 
and this requires CPU shares.  

On K8s we always have at least a couple of processes running on the node: kubelet, kubernetes provider applications, daemons etc. 
These processes require CPU shares, so we cannot dedicate entire node processing power to Scylla, we need to leave space for others.  
We take advantage of it, and we pin IRQs to CPUs not used by any Scylla Pods exclusively.

Tuning resources are created in a special namespace called `scylla-operator-node-tuning`.

The tuning is applied only to pods with `Guaranteed` QoS class. Please double check your ScyllaCluster resource specification
to see if it meets all conditions.

## Kubernetes tuning

By default, the kubelet uses the CFS quota to enforce pod CPU limits.  
When the node runs many CPU-bound pods, the workload can move around different CPU cores depending on whether the pod 
is throttled and which CPU cores are available.
However, kubelet may be configured to assign CPUs exclusively, by setting the CPU manager policy to static.

Setting up kubelet configuration is provider specific. Please check the docs for your distribution or talk to your
provider.

Only pods within the [Guaranteed QoS class](https://kubernetes.io/docs/tasks/configure-pod-container/quality-service-pod/#create-a-pod-that-gets-assigned-a-qos-class-of-guaranteed)) can take advantage of this option. 
When such pod lands on a Node, kubelet will pin them to specific CPUs, and those won't be part of the shared pool.

In our case there are two requirements each ScyllaCluster must fulfill to receive a Guaranteed QoS class:
* resource request and limits must be equal or only limits have to be provided
* agentResources must be provided and their requests and limits must be equal, or only limits have to be provided

An example of such a ScyllaCluster that receives a Guaranteed QoS class is below:

```
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
metadata:
  name: guaranteed-cluster
  namespace: scylla
spec:
  version: 4.5.1
  agentVersion: 2.5.2
  datacenter:
    name: us-east-1
    racks:
    - name: us-east-1a
      members: 3
      storage:
        capacity: 500Gi
      agentResources:
        requests:
          cpu: 1
          memory: 1G
        limits:
          cpu: 1
          memory: 1G
      resources:
        requests:
          cpu: 4
          memory: 16G
        limits:
          cpu: 4
          memory: 16G
```