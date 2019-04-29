# Scylla Cluster CRD

Scylla database clusters can be created and configuring using the `clusters.scylla.scylladb.com` custom resource definition (CRD).

Please refer to the the [user guide walk-through](quickstart.md) for deployment instructions.
This page will explain all the available configuration options on the Scylla CRD.

## Sample

```yaml
apiVersion: scylla.scylladb.com/v1alpha1
kind: Cluster
metadata:
  name: simple-cluster
  namespace: scylla
spec:
  version: 2.3.1
  repository: scylladb/scylla
  developerMode: true
  cpuset: false
  datacenter:
    name: us-east-1
    racks:
      - name: us-east-1a
        members: 3
        storage:
          capacity: 500G
          storageClassName: local-raid-disks
        resources:
          requests:
            cpu: 8
            memory: 32Gi
          limits:
            cpu: 8
            memory: 32Gi
        placement:
          nodeAffinity:
            requiredDuringSchedulingIgnoredDuringExecution:
              nodeSelectorTerms:
                - matchExpressions:
                  - key: failure-domain.beta.kubernetes.io/region
                    operator: In
                    values: 
                      - us-east-1
                  - key: failure-domain.beta.kubernetes.io/zone
                    operator: In
                    values: 
                      - us-east-1a
          tolerations:
            - key: role
              operator: Equal
              value: scylla-clusters
              effect: NoSchedule
```

## Settings Explanation

Cluster Settings:

* `version`: The version of Scylla to use. It is used as the image tag to pull.
* `repository`: Optional field. Specifies a custom image repo. If left unset, the official docker hub repo is used (scylladb/scylla).
* `developerMode`: Optional field. If it's true, then Scylla is started in [developer mode](https://www.scylladb.com/2016/09/13/test-dev-env/). This setting is for shared test/dev environments.
* `cpuset`: Optional field. If it's true, then the operator will start Scylla with cpu pinning for maximum performance. For this to work, you need to set the kubelet to use the [static cpu policy](https://kubernetes.io/blog/2018/07/24/feature-highlight-cpu-manager/) and only specify limits in resources.

In the Scylla model, each cluster contains datacenters and each datacenter contains racks. At the moment, the operator only supports single datacenter setups.

Datacenter Settings:

* `name`: Name of the datacenter. Usually, a datacenter corresponds to a region.
* `racks`: List of racks for the specific datacenter.

Rack Settings:

* `name`: Name of the rack. Usually, a rack corresponds to an availability zone.
* `members`: Number of Scylla members for the specific rack. (In Scylla documentation, they are called nodes. We don't call them nodes to avoid confusion as a Scylla Node corresponds to a Kubernetes Pod, not a Kubernetes Node).
* `storage`: Defines the specs of the underlying storage.
  * `capacity`: Capacity of the PersistentVolume to request.
  * `storageClassName`: Optional field. [StorageClass](https://kubernetes.io/docs/concepts/storage/storage-classes/) of PersistentVolume to request.
* `resources`: Defines the CPU and RAM resources for the Scylla Pods.
  * `requests`: The minimum amount of resources needed to run a Scylla Pod.
    * `cpu`: CPU requests.
    * `memory`: RAM requests.
  * `limits`: The maximum amount of resources that can be used by a Scylla Pod.
    * `cpu`: CPU limits.
    * `memory`: RAM limits.
* `placement`: Defines the placement of Scylla Pods. Has the following subfields:
    * [`nodeAffinity`](https://kubernetes.io/docs/concepts/configuration/assign-pod-node/#node-affinity-beta-feature)
    * [`podAffinity`](https://kubernetes.io/docs/concepts/configuration/assign-pod-node/#inter-pod-affinity-and-anti-affinity-beta-feature)
    * [`podAntiAffinity`](https://kubernetes.io/docs/concepts/configuration/assign-pod-node/#inter-pod-affinity-and-anti-affinity-beta-feature)
    * [`tolerations`](https://kubernetes.io/docs/concepts/configuration/taint-and-toleration)
