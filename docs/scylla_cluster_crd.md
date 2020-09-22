# Scylla Cluster CRD

Scylla database clusters can be created and configuring using the `clusters.scylla.scylladb.com` custom resource definition (CRD).

Please refer to the the [user guide walk-through](generic.md) for deployment instructions.
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
  repairs:
  - name: "weekly us-east-1 repair"
    intensity: 2
    interval: "7d"
    dc: ["us-east-1"]
  backups:
  - name: "daily users backup"
    rateLimit: 50
    location: ["s3:cluster-backups"]
    interval: "1d"
    keyspace: ["users"]
  - name: "weekly full cluster backup"
    rateLimit: 50
    location: ["s3:cluster-backups"]
    interval: "7d"
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

### Cluster Settings

* `version`: The version of Scylla to use. It is used as the image tag to pull.
* `repository`: Optional field. Specifies a custom image repo. If left unset, the official docker hub repo is used (scylladb/scylla).
* `developerMode`: Optional field. If it's true, then Scylla is started in [developer mode](https://www.scylladb.com/2016/09/13/test-dev-env/). This setting is for shared test/dev environments.
* `cpuset`: Optional field. If it's true, then the operator will start Scylla with cpu pinning for maximum performance. For this to work, you need to set the kubelet to use the [static cpu policy](https://kubernetes.io/blog/2018/07/24/feature-highlight-cpu-manager/) and only specify limits in resources.

In the Scylla model, each cluster contains datacenters and each datacenter contains racks. At the moment, the operator only supports single datacenter setups.

### Scylla Manager settings

Tasks are scheduled only when Scylla Manager is deployed in K8s cluster.

Repairs:
* `name` - mandatory - human readable name of the task. It must be unique across all tasks.
* `startDate` - specifies the task start date expressed in the RFC3339 format or `now[+duration]`, e.g. `now+3d2h10m`, 
valid units are d, h, m, s (default "now").
* `interval` - task schedule interval e.g. `3d2h10m`, valid units are d, h, m, s (default "0").
* `numRetries` - the number of times a scheduled task will retry to run before failing (default 3).
* `dc` - list of datacenter glob patterns, e.g. `["dc1", "!otherdc*"]` used to specify the DCs to include or exclude from backup.
* `failFast` - stop repair on first error.
* `intensity` - integer >= 1 or a decimal between (0,1), higher values may result in higher speed and cluster load. 
0 value means repair at maximum intensity.
* `parallel` - The maximum number of repair jobs to run in parallel, each node can participate in at most one repair 
at any given time. Default is means system will repair at maximum parallelism.
* `keyspace` - a list of keyspace/tables glob patterns, e.g. `["keyspace", "!keyspace.table_prefix_*"]`
used to include or exclude keyspaces from repair.
* `smallTableThreshold` - enable small table optimization for tables of size lower than given threshold.
Supported units `[B, MiB, GiB, TiB]` (default `"1GiB"`).

Backups:

* `name` - mandatory - human readable name of the task. It must be unique across all tasks.
* `startDate` - specifies the task start date expressed in the RFC3339 format or `now[+duration]`, e.g. `now+3d2h10m`, 
valid units are d, h, m, s (default "now").
* `interval` - task schedule interval e.g. `3d2h10m`, valid units are d, h, m, s (default "0").
* `numRetries` - the number of times a scheduled task will retry to run before failing (default 3).
* `dc` - a list of datacenter glob patterns, e.g. `["dc1","!otherdc*"]` used to specify the DCs to include or exclude from backup.
* `keyspace` - a list of keyspace/tables glob patterns, e.g. `["keyspace","!keyspace.table_prefix_*"]` used to include or exclude keyspaces from backup.
* `location` - a list of backup locations in the format `[<dc>:]<provider>:<name>` ex. `s3:my-bucket`. 
The `<dc>:` part is optional and is only needed when different datacenters are being used to upload data to different locations. 
`<name>` must be an alphanumeric string and may contain a dash and or a dot, but other characters are forbidden.
The only supported storage <provider> at the moment are `s3` and `gcs`.
* `rateLimit` - a list of megabytes (MiB) per second rate limits expressed in the format `[<dc>:]<limit>`.
The `<dc>:` part is optional and only needed when different datacenters need different upload limits.
Set to 0 for no limit (default 100).
* `retention` - The number of backups which are to be stored (default 3).
* `snapshotParallel` - a list of snapshot parallelism limits in the format `[<dc>:]<limit>`.
The `<dc>:` part is optional and allows for specifying different limits in selected datacenters.
If The `<dc>:` part is not set, the limit is global (e.g. `["dc1:2,5"]`) the runs are parallel in n nodes (2 in dc1)
and n nodes in all the other datacenters.
* `uploadParallel` - a list of upload parallelism limits in the format `[<dc>:]<limit>`.
The `<dc>:` part is optional and allows for specifying different limits in selected datacenters.
If The `<dc>:` part is not set the limit is global (e.g. `["dc1:2,5"]`) the runs are parallel in n nodes (2 in dc1)
and n nodes in all the other datacenters.


### Datacenter Settings

* `name`: Name of the datacenter. Usually, a datacenter corresponds to a region.
* `racks`: List of racks for the specific datacenter.

### Rack Settings

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
