ScyllaCluster (scylla.scylladb.com/v1)
======================================

| **APIVersion**: scylla.scylladb.com/v1
| **Kind**: ScyllaCluster
| **PluralName**: scyllaclusters
| **SingularName**: scyllacluster
| **Scope**: Namespaced
| **ListKind**: ScyllaClusterList
| **Served**: true
| **Storage**: true

Description
-----------
ScyllaCluster defines a Scylla cluster.

Specification
-------------

.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - apiVersion
     - string
     - APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources
   * - kind
     - string
     - Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds
   * - :ref:`metadata<api-scylla.scylladb.com-scyllaclusters-v1-.metadata>`
     - object
     - 
   * - :ref:`spec<api-scylla.scylladb.com-scyllaclusters-v1-.spec>`
     - object
     - spec defines the desired state of this scylla cluster.
   * - :ref:`status<api-scylla.scylladb.com-scyllaclusters-v1-.status>`
     - object
     - status is the current status of this scylla cluster.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.metadata:

.metadata
^^^^^^^^^

Description
"""""""""""


Type
""""
object


.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec:

.spec
^^^^^

Description
"""""""""""
spec defines the desired state of this scylla cluster.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - agentRepository
     - string
     - agentRepository is the repository to pull the agent image from.
   * - agentVersion
     - string
     - agentVersion indicates the version of Scylla Manager Agent to use.
   * - :ref:`alternator<api-scylla.scylladb.com-scyllaclusters-v1-.spec.alternator>`
     - object
     - alternator designates this cluster an Alternator cluster.
   * - automaticOrphanedNodeCleanup
     - boolean
     - automaticOrphanedNodeCleanup controls if automatic orphan node cleanup should be performed.
   * - :ref:`backups<api-scylla.scylladb.com-scyllaclusters-v1-.spec.backups[]>`
     - array (object)
     - backups specifies backup tasks in Scylla Manager. When Scylla Manager is not installed, these will be ignored.
   * - cpuset
     - boolean
     - cpuset determines if the cluster will use cpu-pinning. Deprecated: `cpuset` is deprecated. It is now treated as if it is always set to true regardless of its value.
   * - :ref:`datacenter<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter>`
     - object
     - datacenter holds a specification of a datacenter.
   * - developerMode
     - boolean
     - developerMode determines if the cluster runs in developer-mode.
   * - dnsDomains
     - array (string)
     - dnsDomains is a list of DNS domains this cluster is reachable by. These domains are used when setting up the infrastructure, like certificates. EXPERIMENTAL. Do not rely on any particular behaviour controlled by this field.
   * - :ref:`exposeOptions<api-scylla.scylladb.com-scyllaclusters-v1-.spec.exposeOptions>`
     - object
     - exposeOptions specifies options for exposing ScyllaCluster services. This field is immutable. EXPERIMENTAL. Do not rely on any particular behaviour controlled by this field.
   * - externalSeeds
     - array (string)
     - externalSeeds specifies the external seeds to propagate to ScyllaDB binary on startup as "seeds" parameter of seed-provider.
   * - forceRedeploymentReason
     - string
     - forceRedeploymentReason can be used to force a rolling update of all racks by providing a unique string.
   * - :ref:`genericUpgrade<api-scylla.scylladb.com-scyllaclusters-v1-.spec.genericUpgrade>`
     - object
     - genericUpgrade allows to configure behavior of generic upgrade logic.
   * - :ref:`imagePullSecrets<api-scylla.scylladb.com-scyllaclusters-v1-.spec.imagePullSecrets[]>`
     - array (object)
     - imagePullSecrets is an optional list of references to secrets in the same namespace used for pulling Scylla and Agent images.
   * - ipFamily
     - string
     - IPFamily specifies the IP family for this cluster. All services, broadcast addresses, and pod IPs will use this IP family.
   * - minReadySeconds
     - integer
     - minReadySeconds is the minimum number of seconds for which a newly created ScyllaDB node should be ready for it to be considered available. When used to control load balanced traffic, this can give the load balancer in front of a node enough time to notice that the node is ready and start forwarding traffic in time. Because it all depends on timing, the order is not guaranteed and, if possible, you should use readinessGates instead. If not provided, Operator will determine this value.
   * - minTerminationGracePeriodSeconds
     - integer
     - minTerminationGracePeriodSeconds specifies minimum duration in seconds to wait before every drained node is terminated. This gives time to potential load balancer in front of a node to notice that node is not ready anymore and stop forwarding new requests. This applies only when node is terminated gracefully. If not provided, Operator will determine this value. EXPERIMENTAL. Do not rely on any particular behaviour controlled by this field.
   * - :ref:`network<api-scylla.scylladb.com-scyllaclusters-v1-.spec.network>`
     - object
     - network holds the networking config.
   * - :ref:`podMetadata<api-scylla.scylladb.com-scyllaclusters-v1-.spec.podMetadata>`
     - object
     - podMetadata controls shared metadata for all pods created based on this spec.
   * - :ref:`readinessGates<api-scylla.scylladb.com-scyllaclusters-v1-.spec.readinessGates[]>`
     - array (object)
     - readinessGates specifies custom readiness gates that will be evaluated for every ScyllaDB Pod readiness. It's projected into every ScyllaDB Pod as its readinessGate. Refer to upstream documentation to learn more about readiness gates.
   * - :ref:`repairs<api-scylla.scylladb.com-scyllaclusters-v1-.spec.repairs[]>`
     - array (object)
     - repairs specify repair tasks in Scylla Manager. When Scylla Manager is not installed, these will be ignored.
   * - repository
     - string
     - repository is the image repository to pull the Scylla image from.
   * - scyllaArgs
     - string
     - scyllaArgs will be appended to Scylla binary during startup. This is supported from 4.2.0 Scylla version.
   * - sysctls
     - array (string)
     - sysctls holds the sysctl properties to be applied during initialization given as a list of key=value pairs. Example: fs.aio-max-nr=232323 Deprecated: `sysctls` is deprecated. Use NodeConfig to configure sysctls instead. See NodeConfig resource reference for details.
   * - version
     - string
     - version is a version tag of Scylla to use.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.alternator:

.spec.alternator
^^^^^^^^^^^^^^^^

Description
"""""""""""
alternator designates this cluster an Alternator cluster.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - insecureDisableAuthorization
     - boolean
     - insecureDisableAuthorization disables Alternator authorization. If not specified, the authorization is enabled. For backwards compatibility the authorization is disabled when this field is not specified and a manual port is used.
   * - insecureEnableHTTP
     - boolean
     - insecureEnableHTTP enables serving Alternator traffic also on insecure HTTP port.
   * - port
     - integer
     - port is the port number used to bind the Alternator API. Deprecated: `port` is deprecated and may be ignored in the future. Please make sure to avoid using hostNetworking and work with standard Kubernetes concepts like Services.
   * - :ref:`servingCertificate<api-scylla.scylladb.com-scyllaclusters-v1-.spec.alternator.servingCertificate>`
     - object
     - servingCertificate references a TLS certificate for serving secure traffic.
   * - writeIsolation
     - string
     - writeIsolation indicates the isolation level.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.alternator.servingCertificate:

.spec.alternator.servingCertificate
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
servingCertificate references a TLS certificate for serving secure traffic.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`operatorManagedOptions<api-scylla.scylladb.com-scyllaclusters-v1-.spec.alternator.servingCertificate.operatorManagedOptions>`
     - object
     - operatorManagedOptions specifies options for certificates manged by the operator.
   * - type
     - string
     - type determines the source of this certificate.
   * - :ref:`userManagedOptions<api-scylla.scylladb.com-scyllaclusters-v1-.spec.alternator.servingCertificate.userManagedOptions>`
     - object
     - userManagedOptions specifies options for certificates manged by users.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.alternator.servingCertificate.operatorManagedOptions:

.spec.alternator.servingCertificate.operatorManagedOptions
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
operatorManagedOptions specifies options for certificates manged by the operator.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - additionalDNSNames
     - array (string)
     - additionalDNSNames represents external DNS names that the certificates should be signed for.
   * - additionalIPAddresses
     - array (string)
     - additionalIPAddresses represents external IP addresses that the certificates should be signed for.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.alternator.servingCertificate.userManagedOptions:

.spec.alternator.servingCertificate.userManagedOptions
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
userManagedOptions specifies options for certificates manged by users.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - secretName
     - string
     - secretName references a kubernetes.io/tls type secret containing the TLS cert and key.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.backups[]:

.spec.backups[]
^^^^^^^^^^^^^^^

Description
"""""""""""


Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - cron
     - string
     - cron specifies the task schedule as a cron expression. It supports an extended syntax including @monthly, @weekly, @daily, @midnight, @hourly, @every X[h|m|s].
   * - dc
     - array (string)
     - dc is a list of datacenter glob patterns, e.g. 'dc1,!otherdc*' used to specify the DCs to include or exclude from backup.
   * - interval
     - string
     - interval represents a task schedule interval e.g. 3d2h10m, valid units are d, h, m, s. Deprecated: please use cron instead.
   * - keyspace
     - array (string)
     - keyspace is a list of keyspace/tables glob patterns, e.g. 'keyspace,!keyspace.table_prefix_*' used to include or exclude keyspaces from repair.
   * - location
     - array (string)
     - location is a list of backup locations in the format [<dc>:]<provider>:<name> ex. s3:my-bucket. The <dc>: part is optional and is only needed when different datacenters are being used to upload data to different locations. <name> must be an alphanumeric string and may contain a dash and or a dot, but other characters are forbidden. The only supported storage <provider> at the moment are s3 and gcs.
   * - name
     - string
     - name specifies the name of a task.
   * - numRetries
     - integer
     - numRetries indicates how many times a scheduled task will be retried before failing.
   * - rateLimit
     - array (string)
     - rateLimit is a list of megabytes (MiB) per second rate limits expressed in the format [<dc>:]<limit>. The <dc>: part is optional and only needed when different datacenters need different upload limits. Set to 0 for no limit (default 100).
   * - retention
     - integer
     - retention is the number of backups which are to be stored.
   * - retryWait
     - string
     - retryWait specifies the initial exponential backoff duration for task retries. For instance, if set to 10 minutes, the first retry will be attempted after 10 minutes, the second after 20 minutes, the third after 40 minutes, and so on, up to the number of retries specified in `numRetries`. If not set, the default values is left to ScyllaDB Manager to decide.
   * - snapshotParallel
     - array (string)
     - snapshotParallel is a list of snapshot parallelism limits in the format [<dc>:]<limit>. The <dc>: part is optional and allows for specifying different limits in selected datacenters. If The <dc>: part is not set, the limit is global (e.g. 'dc1:2,5') the runs are parallel in n nodes (2 in dc1) and n nodes in all the other datacenters.
   * - startDate
     - string
     - startDate specifies the task start date expressed in the RFC3339 format or now[+duration], e.g. now+3d2h10m, valid units are d, h, m, s.
   * - timezone
     - string
     - timezone specifies the timezone of cron field.
   * - uploadParallel
     - array (string)
     - uploadParallel is a list of upload parallelism limits in the format [<dc>:]<limit>. The <dc>: part is optional and allows for specifying different limits in selected datacenters. If The <dc>: part is not set the limit is global (e.g. 'dc1:2,5') the runs are parallel in n nodes (2 in dc1) and n nodes in all the other datacenters.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter:

.spec.datacenter
^^^^^^^^^^^^^^^^

Description
"""""""""""
datacenter holds a specification of a datacenter.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - name
     - string
     - name is the name of the scylla datacenter. Used in the cassandra-rackdc.properties file.
   * - :ref:`racks<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[]>`
     - array (object)
     - racks specify the racks in the datacenter.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[]:

.spec.datacenter.racks[]
^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
RackSpec is the desired state for a Scylla Rack.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`agentResources<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].agentResources>`
     - object
     - agentResources specify the resources for the Agent container.
   * - :ref:`agentVolumeMounts<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].agentVolumeMounts[]>`
     - array (object)
     - AgentVolumeMounts to be added to Agent container.
   * - :ref:`exposeOptions<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].exposeOptions>`
     - object
     - exposeOptions specifies rack-specific parameters related to exposing ScyllaDBDatacenter backends.
   * - members
     - integer
     - members is the number of Scylla instances in this rack.
   * - name
     - string
     - name is the name of the Scylla Rack. Used in the cassandra-rackdc.properties file.
   * - :ref:`placement<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement>`
     - object
     - placement describes restrictions for the nodes Scylla is scheduled on.
   * - :ref:`resources<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].resources>`
     - object
     - resources the Scylla container will use.
   * - scyllaAgentConfig
     - string
     - Scylla config map name to customize scylla manager agent
   * - scyllaConfig
     - string
     - Scylla config map name to customize scylla.yaml
   * - :ref:`storage<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].storage>`
     - object
     - storage describes the underlying storage that Scylla will consume.
   * - :ref:`volumeMounts<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumeMounts[]>`
     - array (object)
     - VolumeMounts to be added to Scylla container.
   * - :ref:`volumes<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[]>`
     - array (object)
     - Volumes added to Scylla Pod.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].agentResources:

.spec.datacenter.racks[].agentResources
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
agentResources specify the resources for the Agent container.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`claims<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].agentResources.claims[]>`
     - array (object)
     - Claims lists the names of resources, defined in spec.resourceClaims, that are used by this container.  This field depends on the DynamicResourceAllocation feature gate.  This field is immutable. It can only be set for containers.
   * - :ref:`limits<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].agentResources.limits>`
     - object
     - Limits describes the maximum amount of compute resources allowed. More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
   * - :ref:`requests<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].agentResources.requests>`
     - object
     - Requests describes the minimum amount of compute resources required. If Requests is omitted for a container, it defaults to Limits if that is explicitly specified, otherwise to an implementation-defined value. Requests cannot exceed Limits. More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].agentResources.claims[]:

.spec.datacenter.racks[].agentResources.claims[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
ResourceClaim references one entry in PodSpec.ResourceClaims.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - name
     - string
     - Name must match the name of one entry in pod.spec.resourceClaims of the Pod where this field is used. It makes that resource available inside a container.
   * - request
     - string
     - Request is the name chosen for a request in the referenced claim. If empty, everything from the claim is made available, otherwise only the result of this request.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].agentResources.limits:

.spec.datacenter.racks[].agentResources.limits
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
Limits describes the maximum amount of compute resources allowed. More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/

Type
""""
object


.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].agentResources.requests:

.spec.datacenter.racks[].agentResources.requests
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
Requests describes the minimum amount of compute resources required. If Requests is omitted for a container, it defaults to Limits if that is explicitly specified, otherwise to an implementation-defined value. Requests cannot exceed Limits. More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/

Type
""""
object


.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].agentVolumeMounts[]:

.spec.datacenter.racks[].agentVolumeMounts[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
VolumeMount describes a mounting of a Volume within a container.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - mountPath
     - string
     - Path within the container at which the volume should be mounted.  Must not contain ':'.
   * - mountPropagation
     - string
     - mountPropagation determines how mounts are propagated from the host to container and the other way around. When not set, MountPropagationNone is used. This field is beta in 1.10. When RecursiveReadOnly is set to IfPossible or to Enabled, MountPropagation must be None or unspecified (which defaults to None).
   * - name
     - string
     - This must match the Name of a Volume.
   * - readOnly
     - boolean
     - Mounted read-only if true, read-write otherwise (false or unspecified). Defaults to false.
   * - recursiveReadOnly
     - string
     - RecursiveReadOnly specifies whether read-only mounts should be handled recursively.  If ReadOnly is false, this field has no meaning and must be unspecified.  If ReadOnly is true, and this field is set to Disabled, the mount is not made recursively read-only.  If this field is set to IfPossible, the mount is made recursively read-only, if it is supported by the container runtime.  If this field is set to Enabled, the mount is made recursively read-only if it is supported by the container runtime, otherwise the pod will not be started and an error will be generated to indicate the reason.  If this field is set to IfPossible or Enabled, MountPropagation must be set to None (or be unspecified, which defaults to None).  If this field is not specified, it is treated as an equivalent of Disabled.
   * - subPath
     - string
     - Path within the volume from which the container's volume should be mounted. Defaults to "" (volume's root).
   * - subPathExpr
     - string
     - Expanded path within the volume from which the container's volume should be mounted. Behaves similarly to SubPath but environment variable references $(VAR_NAME) are expanded using the container's environment. Defaults to "" (volume's root). SubPathExpr and SubPath are mutually exclusive.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].exposeOptions:

.spec.datacenter.racks[].exposeOptions
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
exposeOptions specifies rack-specific parameters related to exposing ScyllaDBDatacenter backends.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`nodeService<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].exposeOptions.nodeService>`
     - object
     - nodeService controls properties of Service dedicated for each ScyllaDBDatacenter node in given rack.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].exposeOptions.nodeService:

.spec.datacenter.racks[].exposeOptions.nodeService
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
nodeService controls properties of Service dedicated for each ScyllaDBDatacenter node in given rack.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`annotations<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].exposeOptions.nodeService.annotations>`
     - object
     - annotations is a custom key value map that gets merged with managed object annotations.
   * - :ref:`labels<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].exposeOptions.nodeService.labels>`
     - object
     - labels is a custom key value map that gets merged with managed object labels.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].exposeOptions.nodeService.annotations:

.spec.datacenter.racks[].exposeOptions.nodeService.annotations
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
annotations is a custom key value map that gets merged with managed object annotations.

Type
""""
object


.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].exposeOptions.nodeService.labels:

.spec.datacenter.racks[].exposeOptions.nodeService.labels
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
labels is a custom key value map that gets merged with managed object labels.

Type
""""
object


.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement:

.spec.datacenter.racks[].placement
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
placement describes restrictions for the nodes Scylla is scheduled on.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`nodeAffinity<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.nodeAffinity>`
     - object
     - nodeAffinity describes node affinity scheduling rules for the pod.
   * - :ref:`podAffinity<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAffinity>`
     - object
     - podAffinity describes pod affinity scheduling rules.
   * - :ref:`podAntiAffinity<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAntiAffinity>`
     - object
     - podAntiAffinity describes pod anti-affinity scheduling rules.
   * - :ref:`tolerations<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.tolerations[]>`
     - array (object)
     - tolerations allow the pod to tolerate any taint that matches the triple <key,value,effect> using the matching operator.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.nodeAffinity:

.spec.datacenter.racks[].placement.nodeAffinity
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
nodeAffinity describes node affinity scheduling rules for the pod.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`preferredDuringSchedulingIgnoredDuringExecution<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[]>`
     - array (object)
     - The scheduler will prefer to schedule pods to nodes that satisfy the affinity expressions specified by this field, but it may choose a node that violates one or more of the expressions. The node that is most preferred is the one with the greatest sum of weights, i.e. for each node that meets all of the scheduling requirements (resource request, requiredDuringScheduling affinity expressions, etc.), compute a sum by iterating through the elements of this field and adding "weight" to the sum if the node matches the corresponding matchExpressions; the node(s) with the highest sum are the most preferred.
   * - :ref:`requiredDuringSchedulingIgnoredDuringExecution<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution>`
     - object
     - If the affinity requirements specified by this field are not met at scheduling time, the pod will not be scheduled onto the node. If the affinity requirements specified by this field cease to be met at some point during pod execution (e.g. due to an update), the system may or may not try to eventually evict the pod from its node.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[]:

.spec.datacenter.racks[].placement.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
An empty preferred scheduling term matches all objects with implicit weight 0 (i.e. it's a no-op). A null preferred scheduling term matches no objects (i.e. is also a no-op).

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`preference<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[].preference>`
     - object
     - A node selector term, associated with the corresponding weight.
   * - weight
     - integer
     - Weight associated with matching the corresponding nodeSelectorTerm, in the range 1-100.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[].preference:

.spec.datacenter.racks[].placement.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[].preference
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
A node selector term, associated with the corresponding weight.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`matchExpressions<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[].preference.matchExpressions[]>`
     - array (object)
     - A list of node selector requirements by node's labels.
   * - :ref:`matchFields<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[].preference.matchFields[]>`
     - array (object)
     - A list of node selector requirements by node's fields.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[].preference.matchExpressions[]:

.spec.datacenter.racks[].placement.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[].preference.matchExpressions[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
A node selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - key
     - string
     - The label key that the selector applies to.
   * - operator
     - string
     - Represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists, DoesNotExist. Gt, and Lt.
   * - values
     - array (string)
     - An array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. If the operator is Gt or Lt, the values array must have a single element, which will be interpreted as an integer. This array is replaced during a strategic merge patch.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[].preference.matchFields[]:

.spec.datacenter.racks[].placement.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[].preference.matchFields[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
A node selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - key
     - string
     - The label key that the selector applies to.
   * - operator
     - string
     - Represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists, DoesNotExist. Gt, and Lt.
   * - values
     - array (string)
     - An array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. If the operator is Gt or Lt, the values array must have a single element, which will be interpreted as an integer. This array is replaced during a strategic merge patch.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution:

.spec.datacenter.racks[].placement.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
If the affinity requirements specified by this field are not met at scheduling time, the pod will not be scheduled onto the node. If the affinity requirements specified by this field cease to be met at some point during pod execution (e.g. due to an update), the system may or may not try to eventually evict the pod from its node.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`nodeSelectorTerms<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[]>`
     - array (object)
     - Required. A list of node selector terms. The terms are ORed.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[]:

.spec.datacenter.racks[].placement.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
A null or empty node selector term matches no objects. The requirements of them are ANDed. The TopologySelectorTerm type implements a subset of the NodeSelectorTerm.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`matchExpressions<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[].matchExpressions[]>`
     - array (object)
     - A list of node selector requirements by node's labels.
   * - :ref:`matchFields<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[].matchFields[]>`
     - array (object)
     - A list of node selector requirements by node's fields.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[].matchExpressions[]:

.spec.datacenter.racks[].placement.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[].matchExpressions[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
A node selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - key
     - string
     - The label key that the selector applies to.
   * - operator
     - string
     - Represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists, DoesNotExist. Gt, and Lt.
   * - values
     - array (string)
     - An array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. If the operator is Gt or Lt, the values array must have a single element, which will be interpreted as an integer. This array is replaced during a strategic merge patch.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[].matchFields[]:

.spec.datacenter.racks[].placement.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[].matchFields[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
A node selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - key
     - string
     - The label key that the selector applies to.
   * - operator
     - string
     - Represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists, DoesNotExist. Gt, and Lt.
   * - values
     - array (string)
     - An array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. If the operator is Gt or Lt, the values array must have a single element, which will be interpreted as an integer. This array is replaced during a strategic merge patch.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAffinity:

.spec.datacenter.racks[].placement.podAffinity
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
podAffinity describes pod affinity scheduling rules.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`preferredDuringSchedulingIgnoredDuringExecution<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[]>`
     - array (object)
     - The scheduler will prefer to schedule pods to nodes that satisfy the affinity expressions specified by this field, but it may choose a node that violates one or more of the expressions. The node that is most preferred is the one with the greatest sum of weights, i.e. for each node that meets all of the scheduling requirements (resource request, requiredDuringScheduling affinity expressions, etc.), compute a sum by iterating through the elements of this field and adding "weight" to the sum if the node has pods which matches the corresponding podAffinityTerm; the node(s) with the highest sum are the most preferred.
   * - :ref:`requiredDuringSchedulingIgnoredDuringExecution<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[]>`
     - array (object)
     - If the affinity requirements specified by this field are not met at scheduling time, the pod will not be scheduled onto the node. If the affinity requirements specified by this field cease to be met at some point during pod execution (e.g. due to a pod label update), the system may or may not try to eventually evict the pod from its node. When there are multiple elements, the lists of nodes corresponding to each podAffinityTerm are intersected, i.e. all terms must be satisfied.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[]:

.spec.datacenter.racks[].placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
The weights of all of the matched WeightedPodAffinityTerm fields are added per-node to find the most preferred node(s)

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`podAffinityTerm<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm>`
     - object
     - Required. A pod affinity term, associated with the corresponding weight.
   * - weight
     - integer
     - weight associated with matching the corresponding podAffinityTerm, in the range 1-100.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm:

.spec.datacenter.racks[].placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
Required. A pod affinity term, associated with the corresponding weight.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`labelSelector<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector>`
     - object
     - A label query over a set of resources, in this case pods. If it's null, this PodAffinityTerm matches with no Pods.
   * - matchLabelKeys
     - array (string)
     - MatchLabelKeys is a set of pod label keys to select which pods will be taken into consideration. The keys are used to lookup values from the incoming pod labels, those key-value labels are merged with `labelSelector` as `key in (value)` to select the group of existing pods which pods will be taken into consideration for the incoming pod's pod (anti) affinity. Keys that don't exist in the incoming pod labels will be ignored. The default value is empty. The same key is forbidden to exist in both matchLabelKeys and labelSelector. Also, matchLabelKeys cannot be set when labelSelector isn't set.
   * - mismatchLabelKeys
     - array (string)
     - MismatchLabelKeys is a set of pod label keys to select which pods will be taken into consideration. The keys are used to lookup values from the incoming pod labels, those key-value labels are merged with `labelSelector` as `key notin (value)` to select the group of existing pods which pods will be taken into consideration for the incoming pod's pod (anti) affinity. Keys that don't exist in the incoming pod labels will be ignored. The default value is empty. The same key is forbidden to exist in both mismatchLabelKeys and labelSelector. Also, mismatchLabelKeys cannot be set when labelSelector isn't set.
   * - :ref:`namespaceSelector<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector>`
     - object
     - A label query over the set of namespaces that the term applies to. The term is applied to the union of the namespaces selected by this field and the ones listed in the namespaces field. null selector and null or empty namespaces list means "this pod's namespace". An empty selector ({}) matches all namespaces.
   * - namespaces
     - array (string)
     - namespaces specifies a static list of namespace names that the term applies to. The term is applied to the union of the namespaces listed in this field and the ones selected by namespaceSelector. null or empty namespaces list and null namespaceSelector means "this pod's namespace".
   * - topologyKey
     - string
     - This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching the labelSelector in the specified namespaces, where co-located is defined as running on a node whose value of the label with key topologyKey matches that of any node on which any of the selected pods is running. Empty topologyKey is not allowed.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector:

.spec.datacenter.racks[].placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
A label query over a set of resources, in this case pods. If it's null, this PodAffinityTerm matches with no Pods.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`matchExpressions<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchExpressions[]>`
     - array (object)
     - matchExpressions is a list of label selector requirements. The requirements are ANDed.
   * - :ref:`matchLabels<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchLabels>`
     - object
     - matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchExpressions[]:

.spec.datacenter.racks[].placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchExpressions[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - key
     - string
     - key is the label key that the selector applies to.
   * - operator
     - string
     - operator represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists and DoesNotExist.
   * - values
     - array (string)
     - values is an array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchLabels:

.spec.datacenter.racks[].placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchLabels
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

Type
""""
object


.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector:

.spec.datacenter.racks[].placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
A label query over the set of namespaces that the term applies to. The term is applied to the union of the namespaces selected by this field and the ones listed in the namespaces field. null selector and null or empty namespaces list means "this pod's namespace". An empty selector ({}) matches all namespaces.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`matchExpressions<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchExpressions[]>`
     - array (object)
     - matchExpressions is a list of label selector requirements. The requirements are ANDed.
   * - :ref:`matchLabels<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchLabels>`
     - object
     - matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchExpressions[]:

.spec.datacenter.racks[].placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchExpressions[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - key
     - string
     - key is the label key that the selector applies to.
   * - operator
     - string
     - operator represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists and DoesNotExist.
   * - values
     - array (string)
     - values is an array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchLabels:

.spec.datacenter.racks[].placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchLabels
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

Type
""""
object


.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[]:

.spec.datacenter.racks[].placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
Defines a set of pods (namely those matching the labelSelector relative to the given namespace(s)) that this pod should be co-located (affinity) or not co-located (anti-affinity) with, where co-located is defined as running on a node whose value of the label with key <topologyKey> matches that of any node on which a pod of the set of pods is running

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`labelSelector<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector>`
     - object
     - A label query over a set of resources, in this case pods. If it's null, this PodAffinityTerm matches with no Pods.
   * - matchLabelKeys
     - array (string)
     - MatchLabelKeys is a set of pod label keys to select which pods will be taken into consideration. The keys are used to lookup values from the incoming pod labels, those key-value labels are merged with `labelSelector` as `key in (value)` to select the group of existing pods which pods will be taken into consideration for the incoming pod's pod (anti) affinity. Keys that don't exist in the incoming pod labels will be ignored. The default value is empty. The same key is forbidden to exist in both matchLabelKeys and labelSelector. Also, matchLabelKeys cannot be set when labelSelector isn't set.
   * - mismatchLabelKeys
     - array (string)
     - MismatchLabelKeys is a set of pod label keys to select which pods will be taken into consideration. The keys are used to lookup values from the incoming pod labels, those key-value labels are merged with `labelSelector` as `key notin (value)` to select the group of existing pods which pods will be taken into consideration for the incoming pod's pod (anti) affinity. Keys that don't exist in the incoming pod labels will be ignored. The default value is empty. The same key is forbidden to exist in both mismatchLabelKeys and labelSelector. Also, mismatchLabelKeys cannot be set when labelSelector isn't set.
   * - :ref:`namespaceSelector<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector>`
     - object
     - A label query over the set of namespaces that the term applies to. The term is applied to the union of the namespaces selected by this field and the ones listed in the namespaces field. null selector and null or empty namespaces list means "this pod's namespace". An empty selector ({}) matches all namespaces.
   * - namespaces
     - array (string)
     - namespaces specifies a static list of namespace names that the term applies to. The term is applied to the union of the namespaces listed in this field and the ones selected by namespaceSelector. null or empty namespaces list and null namespaceSelector means "this pod's namespace".
   * - topologyKey
     - string
     - This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching the labelSelector in the specified namespaces, where co-located is defined as running on a node whose value of the label with key topologyKey matches that of any node on which any of the selected pods is running. Empty topologyKey is not allowed.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector:

.spec.datacenter.racks[].placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
A label query over a set of resources, in this case pods. If it's null, this PodAffinityTerm matches with no Pods.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`matchExpressions<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchExpressions[]>`
     - array (object)
     - matchExpressions is a list of label selector requirements. The requirements are ANDed.
   * - :ref:`matchLabels<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchLabels>`
     - object
     - matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchExpressions[]:

.spec.datacenter.racks[].placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchExpressions[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - key
     - string
     - key is the label key that the selector applies to.
   * - operator
     - string
     - operator represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists and DoesNotExist.
   * - values
     - array (string)
     - values is an array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchLabels:

.spec.datacenter.racks[].placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchLabels
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

Type
""""
object


.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector:

.spec.datacenter.racks[].placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
A label query over the set of namespaces that the term applies to. The term is applied to the union of the namespaces selected by this field and the ones listed in the namespaces field. null selector and null or empty namespaces list means "this pod's namespace". An empty selector ({}) matches all namespaces.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`matchExpressions<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchExpressions[]>`
     - array (object)
     - matchExpressions is a list of label selector requirements. The requirements are ANDed.
   * - :ref:`matchLabels<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchLabels>`
     - object
     - matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchExpressions[]:

.spec.datacenter.racks[].placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchExpressions[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - key
     - string
     - key is the label key that the selector applies to.
   * - operator
     - string
     - operator represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists and DoesNotExist.
   * - values
     - array (string)
     - values is an array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchLabels:

.spec.datacenter.racks[].placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchLabels
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

Type
""""
object


.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAntiAffinity:

.spec.datacenter.racks[].placement.podAntiAffinity
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
podAntiAffinity describes pod anti-affinity scheduling rules.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`preferredDuringSchedulingIgnoredDuringExecution<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[]>`
     - array (object)
     - The scheduler will prefer to schedule pods to nodes that satisfy the anti-affinity expressions specified by this field, but it may choose a node that violates one or more of the expressions. The node that is most preferred is the one with the greatest sum of weights, i.e. for each node that meets all of the scheduling requirements (resource request, requiredDuringScheduling anti-affinity expressions, etc.), compute a sum by iterating through the elements of this field and subtracting "weight" from the sum if the node has pods which matches the corresponding podAffinityTerm; the node(s) with the highest sum are the most preferred.
   * - :ref:`requiredDuringSchedulingIgnoredDuringExecution<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[]>`
     - array (object)
     - If the anti-affinity requirements specified by this field are not met at scheduling time, the pod will not be scheduled onto the node. If the anti-affinity requirements specified by this field cease to be met at some point during pod execution (e.g. due to a pod label update), the system may or may not try to eventually evict the pod from its node. When there are multiple elements, the lists of nodes corresponding to each podAffinityTerm are intersected, i.e. all terms must be satisfied.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[]:

.spec.datacenter.racks[].placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
The weights of all of the matched WeightedPodAffinityTerm fields are added per-node to find the most preferred node(s)

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`podAffinityTerm<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm>`
     - object
     - Required. A pod affinity term, associated with the corresponding weight.
   * - weight
     - integer
     - weight associated with matching the corresponding podAffinityTerm, in the range 1-100.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm:

.spec.datacenter.racks[].placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
Required. A pod affinity term, associated with the corresponding weight.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`labelSelector<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector>`
     - object
     - A label query over a set of resources, in this case pods. If it's null, this PodAffinityTerm matches with no Pods.
   * - matchLabelKeys
     - array (string)
     - MatchLabelKeys is a set of pod label keys to select which pods will be taken into consideration. The keys are used to lookup values from the incoming pod labels, those key-value labels are merged with `labelSelector` as `key in (value)` to select the group of existing pods which pods will be taken into consideration for the incoming pod's pod (anti) affinity. Keys that don't exist in the incoming pod labels will be ignored. The default value is empty. The same key is forbidden to exist in both matchLabelKeys and labelSelector. Also, matchLabelKeys cannot be set when labelSelector isn't set.
   * - mismatchLabelKeys
     - array (string)
     - MismatchLabelKeys is a set of pod label keys to select which pods will be taken into consideration. The keys are used to lookup values from the incoming pod labels, those key-value labels are merged with `labelSelector` as `key notin (value)` to select the group of existing pods which pods will be taken into consideration for the incoming pod's pod (anti) affinity. Keys that don't exist in the incoming pod labels will be ignored. The default value is empty. The same key is forbidden to exist in both mismatchLabelKeys and labelSelector. Also, mismatchLabelKeys cannot be set when labelSelector isn't set.
   * - :ref:`namespaceSelector<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector>`
     - object
     - A label query over the set of namespaces that the term applies to. The term is applied to the union of the namespaces selected by this field and the ones listed in the namespaces field. null selector and null or empty namespaces list means "this pod's namespace". An empty selector ({}) matches all namespaces.
   * - namespaces
     - array (string)
     - namespaces specifies a static list of namespace names that the term applies to. The term is applied to the union of the namespaces listed in this field and the ones selected by namespaceSelector. null or empty namespaces list and null namespaceSelector means "this pod's namespace".
   * - topologyKey
     - string
     - This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching the labelSelector in the specified namespaces, where co-located is defined as running on a node whose value of the label with key topologyKey matches that of any node on which any of the selected pods is running. Empty topologyKey is not allowed.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector:

.spec.datacenter.racks[].placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
A label query over a set of resources, in this case pods. If it's null, this PodAffinityTerm matches with no Pods.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`matchExpressions<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchExpressions[]>`
     - array (object)
     - matchExpressions is a list of label selector requirements. The requirements are ANDed.
   * - :ref:`matchLabels<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchLabels>`
     - object
     - matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchExpressions[]:

.spec.datacenter.racks[].placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchExpressions[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - key
     - string
     - key is the label key that the selector applies to.
   * - operator
     - string
     - operator represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists and DoesNotExist.
   * - values
     - array (string)
     - values is an array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchLabels:

.spec.datacenter.racks[].placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchLabels
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

Type
""""
object


.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector:

.spec.datacenter.racks[].placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
A label query over the set of namespaces that the term applies to. The term is applied to the union of the namespaces selected by this field and the ones listed in the namespaces field. null selector and null or empty namespaces list means "this pod's namespace". An empty selector ({}) matches all namespaces.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`matchExpressions<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchExpressions[]>`
     - array (object)
     - matchExpressions is a list of label selector requirements. The requirements are ANDed.
   * - :ref:`matchLabels<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchLabels>`
     - object
     - matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchExpressions[]:

.spec.datacenter.racks[].placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchExpressions[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - key
     - string
     - key is the label key that the selector applies to.
   * - operator
     - string
     - operator represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists and DoesNotExist.
   * - values
     - array (string)
     - values is an array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchLabels:

.spec.datacenter.racks[].placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchLabels
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

Type
""""
object


.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[]:

.spec.datacenter.racks[].placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
Defines a set of pods (namely those matching the labelSelector relative to the given namespace(s)) that this pod should be co-located (affinity) or not co-located (anti-affinity) with, where co-located is defined as running on a node whose value of the label with key <topologyKey> matches that of any node on which a pod of the set of pods is running

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`labelSelector<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector>`
     - object
     - A label query over a set of resources, in this case pods. If it's null, this PodAffinityTerm matches with no Pods.
   * - matchLabelKeys
     - array (string)
     - MatchLabelKeys is a set of pod label keys to select which pods will be taken into consideration. The keys are used to lookup values from the incoming pod labels, those key-value labels are merged with `labelSelector` as `key in (value)` to select the group of existing pods which pods will be taken into consideration for the incoming pod's pod (anti) affinity. Keys that don't exist in the incoming pod labels will be ignored. The default value is empty. The same key is forbidden to exist in both matchLabelKeys and labelSelector. Also, matchLabelKeys cannot be set when labelSelector isn't set.
   * - mismatchLabelKeys
     - array (string)
     - MismatchLabelKeys is a set of pod label keys to select which pods will be taken into consideration. The keys are used to lookup values from the incoming pod labels, those key-value labels are merged with `labelSelector` as `key notin (value)` to select the group of existing pods which pods will be taken into consideration for the incoming pod's pod (anti) affinity. Keys that don't exist in the incoming pod labels will be ignored. The default value is empty. The same key is forbidden to exist in both mismatchLabelKeys and labelSelector. Also, mismatchLabelKeys cannot be set when labelSelector isn't set.
   * - :ref:`namespaceSelector<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector>`
     - object
     - A label query over the set of namespaces that the term applies to. The term is applied to the union of the namespaces selected by this field and the ones listed in the namespaces field. null selector and null or empty namespaces list means "this pod's namespace". An empty selector ({}) matches all namespaces.
   * - namespaces
     - array (string)
     - namespaces specifies a static list of namespace names that the term applies to. The term is applied to the union of the namespaces listed in this field and the ones selected by namespaceSelector. null or empty namespaces list and null namespaceSelector means "this pod's namespace".
   * - topologyKey
     - string
     - This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching the labelSelector in the specified namespaces, where co-located is defined as running on a node whose value of the label with key topologyKey matches that of any node on which any of the selected pods is running. Empty topologyKey is not allowed.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector:

.spec.datacenter.racks[].placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
A label query over a set of resources, in this case pods. If it's null, this PodAffinityTerm matches with no Pods.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`matchExpressions<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchExpressions[]>`
     - array (object)
     - matchExpressions is a list of label selector requirements. The requirements are ANDed.
   * - :ref:`matchLabels<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchLabels>`
     - object
     - matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchExpressions[]:

.spec.datacenter.racks[].placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchExpressions[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - key
     - string
     - key is the label key that the selector applies to.
   * - operator
     - string
     - operator represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists and DoesNotExist.
   * - values
     - array (string)
     - values is an array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchLabels:

.spec.datacenter.racks[].placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchLabels
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

Type
""""
object


.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector:

.spec.datacenter.racks[].placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
A label query over the set of namespaces that the term applies to. The term is applied to the union of the namespaces selected by this field and the ones listed in the namespaces field. null selector and null or empty namespaces list means "this pod's namespace". An empty selector ({}) matches all namespaces.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`matchExpressions<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchExpressions[]>`
     - array (object)
     - matchExpressions is a list of label selector requirements. The requirements are ANDed.
   * - :ref:`matchLabels<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchLabels>`
     - object
     - matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchExpressions[]:

.spec.datacenter.racks[].placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchExpressions[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - key
     - string
     - key is the label key that the selector applies to.
   * - operator
     - string
     - operator represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists and DoesNotExist.
   * - values
     - array (string)
     - values is an array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchLabels:

.spec.datacenter.racks[].placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchLabels
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

Type
""""
object


.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].placement.tolerations[]:

.spec.datacenter.racks[].placement.tolerations[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
The pod this Toleration is attached to tolerates any taint that matches the triple <key,value,effect> using the matching operator <operator>.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - effect
     - string
     - Effect indicates the taint effect to match. Empty means match all taint effects. When specified, allowed values are NoSchedule, PreferNoSchedule and NoExecute.
   * - key
     - string
     - Key is the taint key that the toleration applies to. Empty means match all taint keys. If the key is empty, operator must be Exists; this combination means to match all values and all keys.
   * - operator
     - string
     - Operator represents a key's relationship to the value. Valid operators are Exists and Equal. Defaults to Equal. Exists is equivalent to wildcard for value, so that a pod can tolerate all taints of a particular category.
   * - tolerationSeconds
     - integer
     - TolerationSeconds represents the period of time the toleration (which must be of effect NoExecute, otherwise this field is ignored) tolerates the taint. By default, it is not set, which means tolerate the taint forever (do not evict). Zero and negative values will be treated as 0 (evict immediately) by the system.
   * - value
     - string
     - Value is the taint value the toleration matches to. If the operator is Exists, the value should be empty, otherwise just a regular string.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].resources:

.spec.datacenter.racks[].resources
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
resources the Scylla container will use.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`claims<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].resources.claims[]>`
     - array (object)
     - Claims lists the names of resources, defined in spec.resourceClaims, that are used by this container.  This field depends on the DynamicResourceAllocation feature gate.  This field is immutable. It can only be set for containers.
   * - :ref:`limits<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].resources.limits>`
     - object
     - Limits describes the maximum amount of compute resources allowed. More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
   * - :ref:`requests<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].resources.requests>`
     - object
     - Requests describes the minimum amount of compute resources required. If Requests is omitted for a container, it defaults to Limits if that is explicitly specified, otherwise to an implementation-defined value. Requests cannot exceed Limits. More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].resources.claims[]:

.spec.datacenter.racks[].resources.claims[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
ResourceClaim references one entry in PodSpec.ResourceClaims.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - name
     - string
     - Name must match the name of one entry in pod.spec.resourceClaims of the Pod where this field is used. It makes that resource available inside a container.
   * - request
     - string
     - Request is the name chosen for a request in the referenced claim. If empty, everything from the claim is made available, otherwise only the result of this request.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].resources.limits:

.spec.datacenter.racks[].resources.limits
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
Limits describes the maximum amount of compute resources allowed. More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/

Type
""""
object


.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].resources.requests:

.spec.datacenter.racks[].resources.requests
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
Requests describes the minimum amount of compute resources required. If Requests is omitted for a container, it defaults to Limits if that is explicitly specified, otherwise to an implementation-defined value. Requests cannot exceed Limits. More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/

Type
""""
object


.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].storage:

.spec.datacenter.racks[].storage
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
storage describes the underlying storage that Scylla will consume.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - capacity
     - string
     - capacity describes the requested size of each persistent volume.
   * - :ref:`metadata<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].storage.metadata>`
     - object
     - metadata controls shared metadata for the volume claim for this rack. At this point, the values are applied only for the initial claim and are not reconciled during its lifetime. Note that this may get fixed in the future and this behaviour shouldn't be relied on in any way.
   * - storageClassName
     - string
     - storageClassName is the name of a storageClass to request.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].storage.metadata:

.spec.datacenter.racks[].storage.metadata
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
metadata controls shared metadata for the volume claim for this rack. At this point, the values are applied only for the initial claim and are not reconciled during its lifetime. Note that this may get fixed in the future and this behaviour shouldn't be relied on in any way.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`annotations<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].storage.metadata.annotations>`
     - object
     - annotations is a custom key value map that gets merged with managed object annotations.
   * - :ref:`labels<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].storage.metadata.labels>`
     - object
     - labels is a custom key value map that gets merged with managed object labels.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].storage.metadata.annotations:

.spec.datacenter.racks[].storage.metadata.annotations
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
annotations is a custom key value map that gets merged with managed object annotations.

Type
""""
object


.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].storage.metadata.labels:

.spec.datacenter.racks[].storage.metadata.labels
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
labels is a custom key value map that gets merged with managed object labels.

Type
""""
object


.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumeMounts[]:

.spec.datacenter.racks[].volumeMounts[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
VolumeMount describes a mounting of a Volume within a container.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - mountPath
     - string
     - Path within the container at which the volume should be mounted.  Must not contain ':'.
   * - mountPropagation
     - string
     - mountPropagation determines how mounts are propagated from the host to container and the other way around. When not set, MountPropagationNone is used. This field is beta in 1.10. When RecursiveReadOnly is set to IfPossible or to Enabled, MountPropagation must be None or unspecified (which defaults to None).
   * - name
     - string
     - This must match the Name of a Volume.
   * - readOnly
     - boolean
     - Mounted read-only if true, read-write otherwise (false or unspecified). Defaults to false.
   * - recursiveReadOnly
     - string
     - RecursiveReadOnly specifies whether read-only mounts should be handled recursively.  If ReadOnly is false, this field has no meaning and must be unspecified.  If ReadOnly is true, and this field is set to Disabled, the mount is not made recursively read-only.  If this field is set to IfPossible, the mount is made recursively read-only, if it is supported by the container runtime.  If this field is set to Enabled, the mount is made recursively read-only if it is supported by the container runtime, otherwise the pod will not be started and an error will be generated to indicate the reason.  If this field is set to IfPossible or Enabled, MountPropagation must be set to None (or be unspecified, which defaults to None).  If this field is not specified, it is treated as an equivalent of Disabled.
   * - subPath
     - string
     - Path within the volume from which the container's volume should be mounted. Defaults to "" (volume's root).
   * - subPathExpr
     - string
     - Expanded path within the volume from which the container's volume should be mounted. Behaves similarly to SubPath but environment variable references $(VAR_NAME) are expanded using the container's environment. Defaults to "" (volume's root). SubPathExpr and SubPath are mutually exclusive.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[]:

.spec.datacenter.racks[].volumes[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
Volume represents a named volume in a pod that may be accessed by any container in the pod.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`awsElasticBlockStore<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].awsElasticBlockStore>`
     - object
     - awsElasticBlockStore represents an AWS Disk resource that is attached to a kubelet's host machine and then exposed to the pod. Deprecated: AWSElasticBlockStore is deprecated. All operations for the in-tree awsElasticBlockStore type are redirected to the ebs.csi.aws.com CSI driver. More info: https://kubernetes.io/docs/concepts/storage/volumes#awselasticblockstore
   * - :ref:`azureDisk<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].azureDisk>`
     - object
     - azureDisk represents an Azure Data Disk mount on the host and bind mount to the pod. Deprecated: AzureDisk is deprecated. All operations for the in-tree azureDisk type are redirected to the disk.csi.azure.com CSI driver.
   * - :ref:`azureFile<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].azureFile>`
     - object
     - azureFile represents an Azure File Service mount on the host and bind mount to the pod. Deprecated: AzureFile is deprecated. All operations for the in-tree azureFile type are redirected to the file.csi.azure.com CSI driver.
   * - :ref:`cephfs<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].cephfs>`
     - object
     - cephFS represents a Ceph FS mount on the host that shares a pod's lifetime. Deprecated: CephFS is deprecated and the in-tree cephfs type is no longer supported.
   * - :ref:`cinder<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].cinder>`
     - object
     - cinder represents a cinder volume attached and mounted on kubelets host machine. Deprecated: Cinder is deprecated. All operations for the in-tree cinder type are redirected to the cinder.csi.openstack.org CSI driver. More info: https://examples.k8s.io/mysql-cinder-pd/README.md
   * - :ref:`configMap<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].configMap>`
     - object
     - configMap represents a configMap that should populate this volume
   * - :ref:`csi<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].csi>`
     - object
     - csi (Container Storage Interface) represents ephemeral storage that is handled by certain external CSI drivers.
   * - :ref:`downwardAPI<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].downwardAPI>`
     - object
     - downwardAPI represents downward API about the pod that should populate this volume
   * - :ref:`emptyDir<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].emptyDir>`
     - object
     - emptyDir represents a temporary directory that shares a pod's lifetime. More info: https://kubernetes.io/docs/concepts/storage/volumes#emptydir
   * - :ref:`ephemeral<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].ephemeral>`
     - object
     - ephemeral represents a volume that is handled by a cluster storage driver. The volume's lifecycle is tied to the pod that defines it - it will be created before the pod starts, and deleted when the pod is removed.  Use this if: a) the volume is only needed while the pod runs, b) features of normal volumes like restoring from snapshot or capacity    tracking are needed, c) the storage driver is specified through a storage class, and d) the storage driver supports dynamic volume provisioning through    a PersistentVolumeClaim (see EphemeralVolumeSource for more    information on the connection between this volume type    and PersistentVolumeClaim).  Use PersistentVolumeClaim or one of the vendor-specific APIs for volumes that persist for longer than the lifecycle of an individual pod.  Use CSI for light-weight local ephemeral volumes if the CSI driver is meant to be used that way - see the documentation of the driver for more information.  A pod can use both types of ephemeral volumes and persistent volumes at the same time.
   * - :ref:`fc<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].fc>`
     - object
     - fc represents a Fibre Channel resource that is attached to a kubelet's host machine and then exposed to the pod.
   * - :ref:`flexVolume<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].flexVolume>`
     - object
     - flexVolume represents a generic volume resource that is provisioned/attached using an exec based plugin. Deprecated: FlexVolume is deprecated. Consider using a CSIDriver instead.
   * - :ref:`flocker<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].flocker>`
     - object
     - flocker represents a Flocker volume attached to a kubelet's host machine. This depends on the Flocker control service being running. Deprecated: Flocker is deprecated and the in-tree flocker type is no longer supported.
   * - :ref:`gcePersistentDisk<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].gcePersistentDisk>`
     - object
     - gcePersistentDisk represents a GCE Disk resource that is attached to a kubelet's host machine and then exposed to the pod. Deprecated: GCEPersistentDisk is deprecated. All operations for the in-tree gcePersistentDisk type are redirected to the pd.csi.storage.gke.io CSI driver. More info: https://kubernetes.io/docs/concepts/storage/volumes#gcepersistentdisk
   * - :ref:`gitRepo<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].gitRepo>`
     - object
     - gitRepo represents a git repository at a particular revision. Deprecated: GitRepo is deprecated. To provision a container with a git repo, mount an EmptyDir into an InitContainer that clones the repo using git, then mount the EmptyDir into the Pod's container.
   * - :ref:`glusterfs<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].glusterfs>`
     - object
     - glusterfs represents a Glusterfs mount on the host that shares a pod's lifetime. Deprecated: Glusterfs is deprecated and the in-tree glusterfs type is no longer supported.
   * - :ref:`hostPath<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].hostPath>`
     - object
     - hostPath represents a pre-existing file or directory on the host machine that is directly exposed to the container. This is generally used for system agents or other privileged things that are allowed to see the host machine. Most containers will NOT need this. More info: https://kubernetes.io/docs/concepts/storage/volumes#hostpath
   * - :ref:`image<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].image>`
     - object
     - image represents an OCI object (a container image or artifact) pulled and mounted on the kubelet's host machine. The volume is resolved at pod startup depending on which PullPolicy value is provided:  - Always: the kubelet always attempts to pull the reference. Container creation will fail If the pull fails. - Never: the kubelet never pulls the reference and only uses a local image or artifact. Container creation will fail if the reference isn't present. - IfNotPresent: the kubelet pulls if the reference isn't already present on disk. Container creation will fail if the reference isn't present and the pull fails.  The volume gets re-resolved if the pod gets deleted and recreated, which means that new remote content will become available on pod recreation. A failure to resolve or pull the image during pod startup will block containers from starting and may add significant latency. Failures will be retried using normal volume backoff and will be reported on the pod reason and message. The types of objects that may be mounted by this volume are defined by the container runtime implementation on a host machine and at minimum must include all valid types supported by the container image field. The OCI object gets mounted in a single directory (spec.containers[*].volumeMounts.mountPath) by merging the manifest layers in the same way as for container images. The volume will be mounted read-only (ro) and non-executable files (noexec). Sub path mounts for containers are not supported (spec.containers[*].volumeMounts.subpath) before 1.33. The field spec.securityContext.fsGroupChangePolicy has no effect on this volume type.
   * - :ref:`iscsi<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].iscsi>`
     - object
     - iscsi represents an ISCSI Disk resource that is attached to a kubelet's host machine and then exposed to the pod. More info: https://kubernetes.io/docs/concepts/storage/volumes/#iscsi
   * - name
     - string
     - name of the volume. Must be a DNS_LABEL and unique within the pod. More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names
   * - :ref:`nfs<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].nfs>`
     - object
     - nfs represents an NFS mount on the host that shares a pod's lifetime More info: https://kubernetes.io/docs/concepts/storage/volumes#nfs
   * - :ref:`persistentVolumeClaim<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].persistentVolumeClaim>`
     - object
     - persistentVolumeClaimVolumeSource represents a reference to a PersistentVolumeClaim in the same namespace. More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#persistentvolumeclaims
   * - :ref:`photonPersistentDisk<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].photonPersistentDisk>`
     - object
     - photonPersistentDisk represents a PhotonController persistent disk attached and mounted on kubelets host machine. Deprecated: PhotonPersistentDisk is deprecated and the in-tree photonPersistentDisk type is no longer supported.
   * - :ref:`portworxVolume<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].portworxVolume>`
     - object
     - portworxVolume represents a portworx volume attached and mounted on kubelets host machine. Deprecated: PortworxVolume is deprecated. All operations for the in-tree portworxVolume type are redirected to the pxd.portworx.com CSI driver when the CSIMigrationPortworx feature-gate is on.
   * - :ref:`projected<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].projected>`
     - object
     - projected items for all in one resources secrets, configmaps, and downward API
   * - :ref:`quobyte<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].quobyte>`
     - object
     - quobyte represents a Quobyte mount on the host that shares a pod's lifetime. Deprecated: Quobyte is deprecated and the in-tree quobyte type is no longer supported.
   * - :ref:`rbd<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].rbd>`
     - object
     - rbd represents a Rados Block Device mount on the host that shares a pod's lifetime. Deprecated: RBD is deprecated and the in-tree rbd type is no longer supported.
   * - :ref:`scaleIO<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].scaleIO>`
     - object
     - scaleIO represents a ScaleIO persistent volume attached and mounted on Kubernetes nodes. Deprecated: ScaleIO is deprecated and the in-tree scaleIO type is no longer supported.
   * - :ref:`secret<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].secret>`
     - object
     - secret represents a secret that should populate this volume. More info: https://kubernetes.io/docs/concepts/storage/volumes#secret
   * - :ref:`storageos<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].storageos>`
     - object
     - storageOS represents a StorageOS volume attached and mounted on Kubernetes nodes. Deprecated: StorageOS is deprecated and the in-tree storageos type is no longer supported.
   * - :ref:`vsphereVolume<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].vsphereVolume>`
     - object
     - vsphereVolume represents a vSphere volume attached and mounted on kubelets host machine. Deprecated: VsphereVolume is deprecated. All operations for the in-tree vsphereVolume type are redirected to the csi.vsphere.vmware.com CSI driver.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].awsElasticBlockStore:

.spec.datacenter.racks[].volumes[].awsElasticBlockStore
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
awsElasticBlockStore represents an AWS Disk resource that is attached to a kubelet's host machine and then exposed to the pod. Deprecated: AWSElasticBlockStore is deprecated. All operations for the in-tree awsElasticBlockStore type are redirected to the ebs.csi.aws.com CSI driver. More info: https://kubernetes.io/docs/concepts/storage/volumes#awselasticblockstore

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - fsType
     - string
     - fsType is the filesystem type of the volume that you want to mount. Tip: Ensure that the filesystem type is supported by the host operating system. Examples: "ext4", "xfs", "ntfs". Implicitly inferred to be "ext4" if unspecified. More info: https://kubernetes.io/docs/concepts/storage/volumes#awselasticblockstore
   * - partition
     - integer
     - partition is the partition in the volume that you want to mount. If omitted, the default is to mount by volume name. Examples: For volume /dev/sda1, you specify the partition as "1". Similarly, the volume partition for /dev/sda is "0" (or you can leave the property empty).
   * - readOnly
     - boolean
     - readOnly value true will force the readOnly setting in VolumeMounts. More info: https://kubernetes.io/docs/concepts/storage/volumes#awselasticblockstore
   * - volumeID
     - string
     - volumeID is unique ID of the persistent disk resource in AWS (Amazon EBS volume). More info: https://kubernetes.io/docs/concepts/storage/volumes#awselasticblockstore

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].azureDisk:

.spec.datacenter.racks[].volumes[].azureDisk
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
azureDisk represents an Azure Data Disk mount on the host and bind mount to the pod. Deprecated: AzureDisk is deprecated. All operations for the in-tree azureDisk type are redirected to the disk.csi.azure.com CSI driver.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - cachingMode
     - string
     - cachingMode is the Host Caching mode: None, Read Only, Read Write.
   * - diskName
     - string
     - diskName is the Name of the data disk in the blob storage
   * - diskURI
     - string
     - diskURI is the URI of data disk in the blob storage
   * - fsType
     - string
     - fsType is Filesystem type to mount. Must be a filesystem type supported by the host operating system. Ex. "ext4", "xfs", "ntfs". Implicitly inferred to be "ext4" if unspecified.
   * - kind
     - string
     - kind expected values are Shared: multiple blob disks per storage account  Dedicated: single blob disk per storage account  Managed: azure managed data disk (only in managed availability set). defaults to shared
   * - readOnly
     - boolean
     - readOnly Defaults to false (read/write). ReadOnly here will force the ReadOnly setting in VolumeMounts.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].azureFile:

.spec.datacenter.racks[].volumes[].azureFile
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
azureFile represents an Azure File Service mount on the host and bind mount to the pod. Deprecated: AzureFile is deprecated. All operations for the in-tree azureFile type are redirected to the file.csi.azure.com CSI driver.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - readOnly
     - boolean
     - readOnly defaults to false (read/write). ReadOnly here will force the ReadOnly setting in VolumeMounts.
   * - secretName
     - string
     - secretName is the  name of secret that contains Azure Storage Account Name and Key
   * - shareName
     - string
     - shareName is the azure share Name

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].cephfs:

.spec.datacenter.racks[].volumes[].cephfs
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
cephFS represents a Ceph FS mount on the host that shares a pod's lifetime. Deprecated: CephFS is deprecated and the in-tree cephfs type is no longer supported.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - monitors
     - array (string)
     - monitors is Required: Monitors is a collection of Ceph monitors More info: https://examples.k8s.io/volumes/cephfs/README.md#how-to-use-it
   * - path
     - string
     - path is Optional: Used as the mounted root, rather than the full Ceph tree, default is /
   * - readOnly
     - boolean
     - readOnly is Optional: Defaults to false (read/write). ReadOnly here will force the ReadOnly setting in VolumeMounts. More info: https://examples.k8s.io/volumes/cephfs/README.md#how-to-use-it
   * - secretFile
     - string
     - secretFile is Optional: SecretFile is the path to key ring for User, default is /etc/ceph/user.secret More info: https://examples.k8s.io/volumes/cephfs/README.md#how-to-use-it
   * - :ref:`secretRef<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].cephfs.secretRef>`
     - object
     - secretRef is Optional: SecretRef is reference to the authentication secret for User, default is empty. More info: https://examples.k8s.io/volumes/cephfs/README.md#how-to-use-it
   * - user
     - string
     - user is optional: User is the rados user name, default is admin More info: https://examples.k8s.io/volumes/cephfs/README.md#how-to-use-it

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].cephfs.secretRef:

.spec.datacenter.racks[].volumes[].cephfs.secretRef
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
secretRef is Optional: SecretRef is reference to the authentication secret for User, default is empty. More info: https://examples.k8s.io/volumes/cephfs/README.md#how-to-use-it

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - name
     - string
     - Name of the referent. This field is effectively required, but due to backwards compatibility is allowed to be empty. Instances of this type with an empty value here are almost certainly wrong. More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].cinder:

.spec.datacenter.racks[].volumes[].cinder
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
cinder represents a cinder volume attached and mounted on kubelets host machine. Deprecated: Cinder is deprecated. All operations for the in-tree cinder type are redirected to the cinder.csi.openstack.org CSI driver. More info: https://examples.k8s.io/mysql-cinder-pd/README.md

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - fsType
     - string
     - fsType is the filesystem type to mount. Must be a filesystem type supported by the host operating system. Examples: "ext4", "xfs", "ntfs". Implicitly inferred to be "ext4" if unspecified. More info: https://examples.k8s.io/mysql-cinder-pd/README.md
   * - readOnly
     - boolean
     - readOnly defaults to false (read/write). ReadOnly here will force the ReadOnly setting in VolumeMounts. More info: https://examples.k8s.io/mysql-cinder-pd/README.md
   * - :ref:`secretRef<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].cinder.secretRef>`
     - object
     - secretRef is optional: points to a secret object containing parameters used to connect to OpenStack.
   * - volumeID
     - string
     - volumeID used to identify the volume in cinder. More info: https://examples.k8s.io/mysql-cinder-pd/README.md

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].cinder.secretRef:

.spec.datacenter.racks[].volumes[].cinder.secretRef
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
secretRef is optional: points to a secret object containing parameters used to connect to OpenStack.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - name
     - string
     - Name of the referent. This field is effectively required, but due to backwards compatibility is allowed to be empty. Instances of this type with an empty value here are almost certainly wrong. More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].configMap:

.spec.datacenter.racks[].volumes[].configMap
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
configMap represents a configMap that should populate this volume

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - defaultMode
     - integer
     - defaultMode is optional: mode bits used to set permissions on created files by default. Must be an octal value between 0000 and 0777 or a decimal value between 0 and 511. YAML accepts both octal and decimal values, JSON requires decimal values for mode bits. Defaults to 0644. Directories within the path are not affected by this setting. This might be in conflict with other options that affect the file mode, like fsGroup, and the result can be other mode bits set.
   * - :ref:`items<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].configMap.items[]>`
     - array (object)
     - items if unspecified, each key-value pair in the Data field of the referenced ConfigMap will be projected into the volume as a file whose name is the key and content is the value. If specified, the listed keys will be projected into the specified paths, and unlisted keys will not be present. If a key is specified which is not present in the ConfigMap, the volume setup will error unless it is marked optional. Paths must be relative and may not contain the '..' path or start with '..'.
   * - name
     - string
     - Name of the referent. This field is effectively required, but due to backwards compatibility is allowed to be empty. Instances of this type with an empty value here are almost certainly wrong. More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names
   * - optional
     - boolean
     - optional specify whether the ConfigMap or its keys must be defined

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].configMap.items[]:

.spec.datacenter.racks[].volumes[].configMap.items[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
Maps a string key to a path within a volume.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - key
     - string
     - key is the key to project.
   * - mode
     - integer
     - mode is Optional: mode bits used to set permissions on this file. Must be an octal value between 0000 and 0777 or a decimal value between 0 and 511. YAML accepts both octal and decimal values, JSON requires decimal values for mode bits. If not specified, the volume defaultMode will be used. This might be in conflict with other options that affect the file mode, like fsGroup, and the result can be other mode bits set.
   * - path
     - string
     - path is the relative path of the file to map the key to. May not be an absolute path. May not contain the path element '..'. May not start with the string '..'.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].csi:

.spec.datacenter.racks[].volumes[].csi
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
csi (Container Storage Interface) represents ephemeral storage that is handled by certain external CSI drivers.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - driver
     - string
     - driver is the name of the CSI driver that handles this volume. Consult with your admin for the correct name as registered in the cluster.
   * - fsType
     - string
     - fsType to mount. Ex. "ext4", "xfs", "ntfs". If not provided, the empty value is passed to the associated CSI driver which will determine the default filesystem to apply.
   * - :ref:`nodePublishSecretRef<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].csi.nodePublishSecretRef>`
     - object
     - nodePublishSecretRef is a reference to the secret object containing sensitive information to pass to the CSI driver to complete the CSI NodePublishVolume and NodeUnpublishVolume calls. This field is optional, and  may be empty if no secret is required. If the secret object contains more than one secret, all secret references are passed.
   * - readOnly
     - boolean
     - readOnly specifies a read-only configuration for the volume. Defaults to false (read/write).
   * - :ref:`volumeAttributes<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].csi.volumeAttributes>`
     - object
     - volumeAttributes stores driver-specific properties that are passed to the CSI driver. Consult your driver's documentation for supported values.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].csi.nodePublishSecretRef:

.spec.datacenter.racks[].volumes[].csi.nodePublishSecretRef
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
nodePublishSecretRef is a reference to the secret object containing sensitive information to pass to the CSI driver to complete the CSI NodePublishVolume and NodeUnpublishVolume calls. This field is optional, and  may be empty if no secret is required. If the secret object contains more than one secret, all secret references are passed.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - name
     - string
     - Name of the referent. This field is effectively required, but due to backwards compatibility is allowed to be empty. Instances of this type with an empty value here are almost certainly wrong. More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].csi.volumeAttributes:

.spec.datacenter.racks[].volumes[].csi.volumeAttributes
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
volumeAttributes stores driver-specific properties that are passed to the CSI driver. Consult your driver's documentation for supported values.

Type
""""
object


.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].downwardAPI:

.spec.datacenter.racks[].volumes[].downwardAPI
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
downwardAPI represents downward API about the pod that should populate this volume

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - defaultMode
     - integer
     - Optional: mode bits to use on created files by default. Must be a Optional: mode bits used to set permissions on created files by default. Must be an octal value between 0000 and 0777 or a decimal value between 0 and 511. YAML accepts both octal and decimal values, JSON requires decimal values for mode bits. Defaults to 0644. Directories within the path are not affected by this setting. This might be in conflict with other options that affect the file mode, like fsGroup, and the result can be other mode bits set.
   * - :ref:`items<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].downwardAPI.items[]>`
     - array (object)
     - Items is a list of downward API volume file

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].downwardAPI.items[]:

.spec.datacenter.racks[].volumes[].downwardAPI.items[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
DownwardAPIVolumeFile represents information to create the file containing the pod field

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`fieldRef<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].downwardAPI.items[].fieldRef>`
     - object
     - Required: Selects a field of the pod: only annotations, labels, name, namespace and uid are supported.
   * - mode
     - integer
     - Optional: mode bits used to set permissions on this file, must be an octal value between 0000 and 0777 or a decimal value between 0 and 511. YAML accepts both octal and decimal values, JSON requires decimal values for mode bits. If not specified, the volume defaultMode will be used. This might be in conflict with other options that affect the file mode, like fsGroup, and the result can be other mode bits set.
   * - path
     - string
     - Required: Path is  the relative path name of the file to be created. Must not be absolute or contain the '..' path. Must be utf-8 encoded. The first item of the relative path must not start with '..'
   * - :ref:`resourceFieldRef<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].downwardAPI.items[].resourceFieldRef>`
     - object
     - Selects a resource of the container: only resources limits and requests (limits.cpu, limits.memory, requests.cpu and requests.memory) are currently supported.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].downwardAPI.items[].fieldRef:

.spec.datacenter.racks[].volumes[].downwardAPI.items[].fieldRef
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
Required: Selects a field of the pod: only annotations, labels, name, namespace and uid are supported.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - apiVersion
     - string
     - Version of the schema the FieldPath is written in terms of, defaults to "v1".
   * - fieldPath
     - string
     - Path of the field to select in the specified API version.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].downwardAPI.items[].resourceFieldRef:

.spec.datacenter.racks[].volumes[].downwardAPI.items[].resourceFieldRef
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
Selects a resource of the container: only resources limits and requests (limits.cpu, limits.memory, requests.cpu and requests.memory) are currently supported.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - containerName
     - string
     - Container name: required for volumes, optional for env vars
   * - divisor
     - 
     - Specifies the output format of the exposed resources, defaults to "1"
   * - resource
     - string
     - Required: resource to select

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].emptyDir:

.spec.datacenter.racks[].volumes[].emptyDir
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
emptyDir represents a temporary directory that shares a pod's lifetime. More info: https://kubernetes.io/docs/concepts/storage/volumes#emptydir

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - medium
     - string
     - medium represents what type of storage medium should back this directory. The default is "" which means to use the node's default medium. Must be an empty string (default) or Memory. More info: https://kubernetes.io/docs/concepts/storage/volumes#emptydir
   * - sizeLimit
     - 
     - sizeLimit is the total amount of local storage required for this EmptyDir volume. The size limit is also applicable for memory medium. The maximum usage on memory medium EmptyDir would be the minimum value between the SizeLimit specified here and the sum of memory limits of all containers in a pod. The default is nil which means that the limit is undefined. More info: https://kubernetes.io/docs/concepts/storage/volumes#emptydir

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].ephemeral:

.spec.datacenter.racks[].volumes[].ephemeral
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
ephemeral represents a volume that is handled by a cluster storage driver. The volume's lifecycle is tied to the pod that defines it - it will be created before the pod starts, and deleted when the pod is removed.  Use this if: a) the volume is only needed while the pod runs, b) features of normal volumes like restoring from snapshot or capacity    tracking are needed, c) the storage driver is specified through a storage class, and d) the storage driver supports dynamic volume provisioning through    a PersistentVolumeClaim (see EphemeralVolumeSource for more    information on the connection between this volume type    and PersistentVolumeClaim).  Use PersistentVolumeClaim or one of the vendor-specific APIs for volumes that persist for longer than the lifecycle of an individual pod.  Use CSI for light-weight local ephemeral volumes if the CSI driver is meant to be used that way - see the documentation of the driver for more information.  A pod can use both types of ephemeral volumes and persistent volumes at the same time.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`volumeClaimTemplate<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].ephemeral.volumeClaimTemplate>`
     - object
     - Will be used to create a stand-alone PVC to provision the volume. The pod in which this EphemeralVolumeSource is embedded will be the owner of the PVC, i.e. the PVC will be deleted together with the pod.  The name of the PVC will be `<pod name>-<volume name>` where `<volume name>` is the name from the `PodSpec.Volumes` array entry. Pod validation will reject the pod if the concatenated name is not valid for a PVC (for example, too long).  An existing PVC with that name that is not owned by the pod will *not* be used for the pod to avoid using an unrelated volume by mistake. Starting the pod is then blocked until the unrelated PVC is removed. If such a pre-created PVC is meant to be used by the pod, the PVC has to updated with an owner reference to the pod once the pod exists. Normally this should not be necessary, but it may be useful when manually reconstructing a broken cluster.  This field is read-only and no changes will be made by Kubernetes to the PVC after it has been created.  Required, must not be nil.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].ephemeral.volumeClaimTemplate:

.spec.datacenter.racks[].volumes[].ephemeral.volumeClaimTemplate
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
Will be used to create a stand-alone PVC to provision the volume. The pod in which this EphemeralVolumeSource is embedded will be the owner of the PVC, i.e. the PVC will be deleted together with the pod.  The name of the PVC will be `<pod name>-<volume name>` where `<volume name>` is the name from the `PodSpec.Volumes` array entry. Pod validation will reject the pod if the concatenated name is not valid for a PVC (for example, too long).  An existing PVC with that name that is not owned by the pod will *not* be used for the pod to avoid using an unrelated volume by mistake. Starting the pod is then blocked until the unrelated PVC is removed. If such a pre-created PVC is meant to be used by the pod, the PVC has to updated with an owner reference to the pod once the pod exists. Normally this should not be necessary, but it may be useful when manually reconstructing a broken cluster.  This field is read-only and no changes will be made by Kubernetes to the PVC after it has been created.  Required, must not be nil.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`metadata<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].ephemeral.volumeClaimTemplate.metadata>`
     - object
     - May contain labels and annotations that will be copied into the PVC when creating it. No other fields are allowed and will be rejected during validation.
   * - :ref:`spec<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].ephemeral.volumeClaimTemplate.spec>`
     - object
     - The specification for the PersistentVolumeClaim. The entire content is copied unchanged into the PVC that gets created from this template. The same fields as in a PersistentVolumeClaim are also valid here.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].ephemeral.volumeClaimTemplate.metadata:

.spec.datacenter.racks[].volumes[].ephemeral.volumeClaimTemplate.metadata
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
May contain labels and annotations that will be copied into the PVC when creating it. No other fields are allowed and will be rejected during validation.

Type
""""
object


.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].ephemeral.volumeClaimTemplate.spec:

.spec.datacenter.racks[].volumes[].ephemeral.volumeClaimTemplate.spec
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
The specification for the PersistentVolumeClaim. The entire content is copied unchanged into the PVC that gets created from this template. The same fields as in a PersistentVolumeClaim are also valid here.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - accessModes
     - array (string)
     - accessModes contains the desired access modes the volume should have. More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#access-modes-1
   * - :ref:`dataSource<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].ephemeral.volumeClaimTemplate.spec.dataSource>`
     - object
     - dataSource field can be used to specify either: * An existing VolumeSnapshot object (snapshot.storage.k8s.io/VolumeSnapshot) * An existing PVC (PersistentVolumeClaim) If the provisioner or an external controller can support the specified data source, it will create a new volume based on the contents of the specified data source. When the AnyVolumeDataSource feature gate is enabled, dataSource contents will be copied to dataSourceRef, and dataSourceRef contents will be copied to dataSource when dataSourceRef.namespace is not specified. If the namespace is specified, then dataSourceRef will not be copied to dataSource.
   * - :ref:`dataSourceRef<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].ephemeral.volumeClaimTemplate.spec.dataSourceRef>`
     - object
     - dataSourceRef specifies the object from which to populate the volume with data, if a non-empty volume is desired. This may be any object from a non-empty API group (non core object) or a PersistentVolumeClaim object. When this field is specified, volume binding will only succeed if the type of the specified object matches some installed volume populator or dynamic provisioner. This field will replace the functionality of the dataSource field and as such if both fields are non-empty, they must have the same value. For backwards compatibility, when namespace isn't specified in dataSourceRef, both fields (dataSource and dataSourceRef) will be set to the same value automatically if one of them is empty and the other is non-empty. When namespace is specified in dataSourceRef, dataSource isn't set to the same value and must be empty. There are three important differences between dataSource and dataSourceRef: * While dataSource only allows two specific types of objects, dataSourceRef   allows any non-core object, as well as PersistentVolumeClaim objects. * While dataSource ignores disallowed values (dropping them), dataSourceRef   preserves all values, and generates an error if a disallowed value is   specified. * While dataSource only allows local objects, dataSourceRef allows objects   in any namespaces. (Beta) Using this field requires the AnyVolumeDataSource feature gate to be enabled. (Alpha) Using the namespace field of dataSourceRef requires the CrossNamespaceVolumeDataSource feature gate to be enabled.
   * - :ref:`resources<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].ephemeral.volumeClaimTemplate.spec.resources>`
     - object
     - resources represents the minimum resources the volume should have. If RecoverVolumeExpansionFailure feature is enabled users are allowed to specify resource requirements that are lower than previous value but must still be higher than capacity recorded in the status field of the claim. More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#resources
   * - :ref:`selector<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].ephemeral.volumeClaimTemplate.spec.selector>`
     - object
     - selector is a label query over volumes to consider for binding.
   * - storageClassName
     - string
     - storageClassName is the name of the StorageClass required by the claim. More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#class-1
   * - volumeAttributesClassName
     - string
     - volumeAttributesClassName may be used to set the VolumeAttributesClass used by this claim. If specified, the CSI driver will create or update the volume with the attributes defined in the corresponding VolumeAttributesClass. This has a different purpose than storageClassName, it can be changed after the claim is created. An empty string or nil value indicates that no VolumeAttributesClass will be applied to the claim. If the claim enters an Infeasible error state, this field can be reset to its previous value (including nil) to cancel the modification. If the resource referred to by volumeAttributesClass does not exist, this PersistentVolumeClaim will be set to a Pending state, as reflected by the modifyVolumeStatus field, until such as a resource exists. More info: https://kubernetes.io/docs/concepts/storage/volume-attributes-classes/
   * - volumeMode
     - string
     - volumeMode defines what type of volume is required by the claim. Value of Filesystem is implied when not included in claim spec.
   * - volumeName
     - string
     - volumeName is the binding reference to the PersistentVolume backing this claim.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].ephemeral.volumeClaimTemplate.spec.dataSource:

.spec.datacenter.racks[].volumes[].ephemeral.volumeClaimTemplate.spec.dataSource
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
dataSource field can be used to specify either: * An existing VolumeSnapshot object (snapshot.storage.k8s.io/VolumeSnapshot) * An existing PVC (PersistentVolumeClaim) If the provisioner or an external controller can support the specified data source, it will create a new volume based on the contents of the specified data source. When the AnyVolumeDataSource feature gate is enabled, dataSource contents will be copied to dataSourceRef, and dataSourceRef contents will be copied to dataSource when dataSourceRef.namespace is not specified. If the namespace is specified, then dataSourceRef will not be copied to dataSource.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - apiGroup
     - string
     - APIGroup is the group for the resource being referenced. If APIGroup is not specified, the specified Kind must be in the core API group. For any other third-party types, APIGroup is required.
   * - kind
     - string
     - Kind is the type of resource being referenced
   * - name
     - string
     - Name is the name of resource being referenced

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].ephemeral.volumeClaimTemplate.spec.dataSourceRef:

.spec.datacenter.racks[].volumes[].ephemeral.volumeClaimTemplate.spec.dataSourceRef
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
dataSourceRef specifies the object from which to populate the volume with data, if a non-empty volume is desired. This may be any object from a non-empty API group (non core object) or a PersistentVolumeClaim object. When this field is specified, volume binding will only succeed if the type of the specified object matches some installed volume populator or dynamic provisioner. This field will replace the functionality of the dataSource field and as such if both fields are non-empty, they must have the same value. For backwards compatibility, when namespace isn't specified in dataSourceRef, both fields (dataSource and dataSourceRef) will be set to the same value automatically if one of them is empty and the other is non-empty. When namespace is specified in dataSourceRef, dataSource isn't set to the same value and must be empty. There are three important differences between dataSource and dataSourceRef: * While dataSource only allows two specific types of objects, dataSourceRef   allows any non-core object, as well as PersistentVolumeClaim objects. * While dataSource ignores disallowed values (dropping them), dataSourceRef   preserves all values, and generates an error if a disallowed value is   specified. * While dataSource only allows local objects, dataSourceRef allows objects   in any namespaces. (Beta) Using this field requires the AnyVolumeDataSource feature gate to be enabled. (Alpha) Using the namespace field of dataSourceRef requires the CrossNamespaceVolumeDataSource feature gate to be enabled.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - apiGroup
     - string
     - APIGroup is the group for the resource being referenced. If APIGroup is not specified, the specified Kind must be in the core API group. For any other third-party types, APIGroup is required.
   * - kind
     - string
     - Kind is the type of resource being referenced
   * - name
     - string
     - Name is the name of resource being referenced
   * - namespace
     - string
     - Namespace is the namespace of resource being referenced Note that when a namespace is specified, a gateway.networking.k8s.io/ReferenceGrant object is required in the referent namespace to allow that namespace's owner to accept the reference. See the ReferenceGrant documentation for details. (Alpha) This field requires the CrossNamespaceVolumeDataSource feature gate to be enabled.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].ephemeral.volumeClaimTemplate.spec.resources:

.spec.datacenter.racks[].volumes[].ephemeral.volumeClaimTemplate.spec.resources
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
resources represents the minimum resources the volume should have. If RecoverVolumeExpansionFailure feature is enabled users are allowed to specify resource requirements that are lower than previous value but must still be higher than capacity recorded in the status field of the claim. More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#resources

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`limits<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].ephemeral.volumeClaimTemplate.spec.resources.limits>`
     - object
     - Limits describes the maximum amount of compute resources allowed. More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
   * - :ref:`requests<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].ephemeral.volumeClaimTemplate.spec.resources.requests>`
     - object
     - Requests describes the minimum amount of compute resources required. If Requests is omitted for a container, it defaults to Limits if that is explicitly specified, otherwise to an implementation-defined value. Requests cannot exceed Limits. More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].ephemeral.volumeClaimTemplate.spec.resources.limits:

.spec.datacenter.racks[].volumes[].ephemeral.volumeClaimTemplate.spec.resources.limits
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
Limits describes the maximum amount of compute resources allowed. More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/

Type
""""
object


.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].ephemeral.volumeClaimTemplate.spec.resources.requests:

.spec.datacenter.racks[].volumes[].ephemeral.volumeClaimTemplate.spec.resources.requests
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
Requests describes the minimum amount of compute resources required. If Requests is omitted for a container, it defaults to Limits if that is explicitly specified, otherwise to an implementation-defined value. Requests cannot exceed Limits. More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/

Type
""""
object


.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].ephemeral.volumeClaimTemplate.spec.selector:

.spec.datacenter.racks[].volumes[].ephemeral.volumeClaimTemplate.spec.selector
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
selector is a label query over volumes to consider for binding.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`matchExpressions<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].ephemeral.volumeClaimTemplate.spec.selector.matchExpressions[]>`
     - array (object)
     - matchExpressions is a list of label selector requirements. The requirements are ANDed.
   * - :ref:`matchLabels<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].ephemeral.volumeClaimTemplate.spec.selector.matchLabels>`
     - object
     - matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].ephemeral.volumeClaimTemplate.spec.selector.matchExpressions[]:

.spec.datacenter.racks[].volumes[].ephemeral.volumeClaimTemplate.spec.selector.matchExpressions[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - key
     - string
     - key is the label key that the selector applies to.
   * - operator
     - string
     - operator represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists and DoesNotExist.
   * - values
     - array (string)
     - values is an array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].ephemeral.volumeClaimTemplate.spec.selector.matchLabels:

.spec.datacenter.racks[].volumes[].ephemeral.volumeClaimTemplate.spec.selector.matchLabels
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

Type
""""
object


.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].fc:

.spec.datacenter.racks[].volumes[].fc
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
fc represents a Fibre Channel resource that is attached to a kubelet's host machine and then exposed to the pod.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - fsType
     - string
     - fsType is the filesystem type to mount. Must be a filesystem type supported by the host operating system. Ex. "ext4", "xfs", "ntfs". Implicitly inferred to be "ext4" if unspecified.
   * - lun
     - integer
     - lun is Optional: FC target lun number
   * - readOnly
     - boolean
     - readOnly is Optional: Defaults to false (read/write). ReadOnly here will force the ReadOnly setting in VolumeMounts.
   * - targetWWNs
     - array (string)
     - targetWWNs is Optional: FC target worldwide names (WWNs)
   * - wwids
     - array (string)
     - wwids Optional: FC volume world wide identifiers (wwids) Either wwids or combination of targetWWNs and lun must be set, but not both simultaneously.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].flexVolume:

.spec.datacenter.racks[].volumes[].flexVolume
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
flexVolume represents a generic volume resource that is provisioned/attached using an exec based plugin. Deprecated: FlexVolume is deprecated. Consider using a CSIDriver instead.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - driver
     - string
     - driver is the name of the driver to use for this volume.
   * - fsType
     - string
     - fsType is the filesystem type to mount. Must be a filesystem type supported by the host operating system. Ex. "ext4", "xfs", "ntfs". The default filesystem depends on FlexVolume script.
   * - :ref:`options<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].flexVolume.options>`
     - object
     - options is Optional: this field holds extra command options if any.
   * - readOnly
     - boolean
     - readOnly is Optional: defaults to false (read/write). ReadOnly here will force the ReadOnly setting in VolumeMounts.
   * - :ref:`secretRef<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].flexVolume.secretRef>`
     - object
     - secretRef is Optional: secretRef is reference to the secret object containing sensitive information to pass to the plugin scripts. This may be empty if no secret object is specified. If the secret object contains more than one secret, all secrets are passed to the plugin scripts.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].flexVolume.options:

.spec.datacenter.racks[].volumes[].flexVolume.options
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
options is Optional: this field holds extra command options if any.

Type
""""
object


.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].flexVolume.secretRef:

.spec.datacenter.racks[].volumes[].flexVolume.secretRef
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
secretRef is Optional: secretRef is reference to the secret object containing sensitive information to pass to the plugin scripts. This may be empty if no secret object is specified. If the secret object contains more than one secret, all secrets are passed to the plugin scripts.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - name
     - string
     - Name of the referent. This field is effectively required, but due to backwards compatibility is allowed to be empty. Instances of this type with an empty value here are almost certainly wrong. More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].flocker:

.spec.datacenter.racks[].volumes[].flocker
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
flocker represents a Flocker volume attached to a kubelet's host machine. This depends on the Flocker control service being running. Deprecated: Flocker is deprecated and the in-tree flocker type is no longer supported.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - datasetName
     - string
     - datasetName is Name of the dataset stored as metadata -> name on the dataset for Flocker should be considered as deprecated
   * - datasetUUID
     - string
     - datasetUUID is the UUID of the dataset. This is unique identifier of a Flocker dataset

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].gcePersistentDisk:

.spec.datacenter.racks[].volumes[].gcePersistentDisk
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
gcePersistentDisk represents a GCE Disk resource that is attached to a kubelet's host machine and then exposed to the pod. Deprecated: GCEPersistentDisk is deprecated. All operations for the in-tree gcePersistentDisk type are redirected to the pd.csi.storage.gke.io CSI driver. More info: https://kubernetes.io/docs/concepts/storage/volumes#gcepersistentdisk

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - fsType
     - string
     - fsType is filesystem type of the volume that you want to mount. Tip: Ensure that the filesystem type is supported by the host operating system. Examples: "ext4", "xfs", "ntfs". Implicitly inferred to be "ext4" if unspecified. More info: https://kubernetes.io/docs/concepts/storage/volumes#gcepersistentdisk
   * - partition
     - integer
     - partition is the partition in the volume that you want to mount. If omitted, the default is to mount by volume name. Examples: For volume /dev/sda1, you specify the partition as "1". Similarly, the volume partition for /dev/sda is "0" (or you can leave the property empty). More info: https://kubernetes.io/docs/concepts/storage/volumes#gcepersistentdisk
   * - pdName
     - string
     - pdName is unique name of the PD resource in GCE. Used to identify the disk in GCE. More info: https://kubernetes.io/docs/concepts/storage/volumes#gcepersistentdisk
   * - readOnly
     - boolean
     - readOnly here will force the ReadOnly setting in VolumeMounts. Defaults to false. More info: https://kubernetes.io/docs/concepts/storage/volumes#gcepersistentdisk

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].gitRepo:

.spec.datacenter.racks[].volumes[].gitRepo
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
gitRepo represents a git repository at a particular revision. Deprecated: GitRepo is deprecated. To provision a container with a git repo, mount an EmptyDir into an InitContainer that clones the repo using git, then mount the EmptyDir into the Pod's container.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - directory
     - string
     - directory is the target directory name. Must not contain or start with '..'.  If '.' is supplied, the volume directory will be the git repository.  Otherwise, if specified, the volume will contain the git repository in the subdirectory with the given name.
   * - repository
     - string
     - repository is the URL
   * - revision
     - string
     - revision is the commit hash for the specified revision.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].glusterfs:

.spec.datacenter.racks[].volumes[].glusterfs
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
glusterfs represents a Glusterfs mount on the host that shares a pod's lifetime. Deprecated: Glusterfs is deprecated and the in-tree glusterfs type is no longer supported.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - endpoints
     - string
     - endpoints is the endpoint name that details Glusterfs topology.
   * - path
     - string
     - path is the Glusterfs volume path. More info: https://examples.k8s.io/volumes/glusterfs/README.md#create-a-pod
   * - readOnly
     - boolean
     - readOnly here will force the Glusterfs volume to be mounted with read-only permissions. Defaults to false. More info: https://examples.k8s.io/volumes/glusterfs/README.md#create-a-pod

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].hostPath:

.spec.datacenter.racks[].volumes[].hostPath
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
hostPath represents a pre-existing file or directory on the host machine that is directly exposed to the container. This is generally used for system agents or other privileged things that are allowed to see the host machine. Most containers will NOT need this. More info: https://kubernetes.io/docs/concepts/storage/volumes#hostpath

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - path
     - string
     - path of the directory on the host. If the path is a symlink, it will follow the link to the real path. More info: https://kubernetes.io/docs/concepts/storage/volumes#hostpath
   * - type
     - string
     - type for HostPath Volume Defaults to "" More info: https://kubernetes.io/docs/concepts/storage/volumes#hostpath

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].image:

.spec.datacenter.racks[].volumes[].image
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
image represents an OCI object (a container image or artifact) pulled and mounted on the kubelet's host machine. The volume is resolved at pod startup depending on which PullPolicy value is provided:  - Always: the kubelet always attempts to pull the reference. Container creation will fail If the pull fails. - Never: the kubelet never pulls the reference and only uses a local image or artifact. Container creation will fail if the reference isn't present. - IfNotPresent: the kubelet pulls if the reference isn't already present on disk. Container creation will fail if the reference isn't present and the pull fails.  The volume gets re-resolved if the pod gets deleted and recreated, which means that new remote content will become available on pod recreation. A failure to resolve or pull the image during pod startup will block containers from starting and may add significant latency. Failures will be retried using normal volume backoff and will be reported on the pod reason and message. The types of objects that may be mounted by this volume are defined by the container runtime implementation on a host machine and at minimum must include all valid types supported by the container image field. The OCI object gets mounted in a single directory (spec.containers[*].volumeMounts.mountPath) by merging the manifest layers in the same way as for container images. The volume will be mounted read-only (ro) and non-executable files (noexec). Sub path mounts for containers are not supported (spec.containers[*].volumeMounts.subpath) before 1.33. The field spec.securityContext.fsGroupChangePolicy has no effect on this volume type.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - pullPolicy
     - string
     - Policy for pulling OCI objects. Possible values are: Always: the kubelet always attempts to pull the reference. Container creation will fail If the pull fails. Never: the kubelet never pulls the reference and only uses a local image or artifact. Container creation will fail if the reference isn't present. IfNotPresent: the kubelet pulls if the reference isn't already present on disk. Container creation will fail if the reference isn't present and the pull fails. Defaults to Always if :latest tag is specified, or IfNotPresent otherwise.
   * - reference
     - string
     - Required: Image or artifact reference to be used. Behaves in the same way as pod.spec.containers[*].image. Pull secrets will be assembled in the same way as for the container image by looking up node credentials, SA image pull secrets, and pod spec image pull secrets. More info: https://kubernetes.io/docs/concepts/containers/images This field is optional to allow higher level config management to default or override container images in workload controllers like Deployments and StatefulSets.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].iscsi:

.spec.datacenter.racks[].volumes[].iscsi
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
iscsi represents an ISCSI Disk resource that is attached to a kubelet's host machine and then exposed to the pod. More info: https://kubernetes.io/docs/concepts/storage/volumes/#iscsi

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - chapAuthDiscovery
     - boolean
     - chapAuthDiscovery defines whether support iSCSI Discovery CHAP authentication
   * - chapAuthSession
     - boolean
     - chapAuthSession defines whether support iSCSI Session CHAP authentication
   * - fsType
     - string
     - fsType is the filesystem type of the volume that you want to mount. Tip: Ensure that the filesystem type is supported by the host operating system. Examples: "ext4", "xfs", "ntfs". Implicitly inferred to be "ext4" if unspecified. More info: https://kubernetes.io/docs/concepts/storage/volumes#iscsi
   * - initiatorName
     - string
     - initiatorName is the custom iSCSI Initiator Name. If initiatorName is specified with iscsiInterface simultaneously, new iSCSI interface <target portal>:<volume name> will be created for the connection.
   * - iqn
     - string
     - iqn is the target iSCSI Qualified Name.
   * - iscsiInterface
     - string
     - iscsiInterface is the interface Name that uses an iSCSI transport. Defaults to 'default' (tcp).
   * - lun
     - integer
     - lun represents iSCSI Target Lun number.
   * - portals
     - array (string)
     - portals is the iSCSI Target Portal List. The portal is either an IP or ip_addr:port if the port is other than default (typically TCP ports 860 and 3260).
   * - readOnly
     - boolean
     - readOnly here will force the ReadOnly setting in VolumeMounts. Defaults to false.
   * - :ref:`secretRef<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].iscsi.secretRef>`
     - object
     - secretRef is the CHAP Secret for iSCSI target and initiator authentication
   * - targetPortal
     - string
     - targetPortal is iSCSI Target Portal. The Portal is either an IP or ip_addr:port if the port is other than default (typically TCP ports 860 and 3260).

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].iscsi.secretRef:

.spec.datacenter.racks[].volumes[].iscsi.secretRef
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
secretRef is the CHAP Secret for iSCSI target and initiator authentication

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - name
     - string
     - Name of the referent. This field is effectively required, but due to backwards compatibility is allowed to be empty. Instances of this type with an empty value here are almost certainly wrong. More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].nfs:

.spec.datacenter.racks[].volumes[].nfs
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
nfs represents an NFS mount on the host that shares a pod's lifetime More info: https://kubernetes.io/docs/concepts/storage/volumes#nfs

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - path
     - string
     - path that is exported by the NFS server. More info: https://kubernetes.io/docs/concepts/storage/volumes#nfs
   * - readOnly
     - boolean
     - readOnly here will force the NFS export to be mounted with read-only permissions. Defaults to false. More info: https://kubernetes.io/docs/concepts/storage/volumes#nfs
   * - server
     - string
     - server is the hostname or IP address of the NFS server. More info: https://kubernetes.io/docs/concepts/storage/volumes#nfs

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].persistentVolumeClaim:

.spec.datacenter.racks[].volumes[].persistentVolumeClaim
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
persistentVolumeClaimVolumeSource represents a reference to a PersistentVolumeClaim in the same namespace. More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#persistentvolumeclaims

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - claimName
     - string
     - claimName is the name of a PersistentVolumeClaim in the same namespace as the pod using this volume. More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#persistentvolumeclaims
   * - readOnly
     - boolean
     - readOnly Will force the ReadOnly setting in VolumeMounts. Default false.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].photonPersistentDisk:

.spec.datacenter.racks[].volumes[].photonPersistentDisk
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
photonPersistentDisk represents a PhotonController persistent disk attached and mounted on kubelets host machine. Deprecated: PhotonPersistentDisk is deprecated and the in-tree photonPersistentDisk type is no longer supported.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - fsType
     - string
     - fsType is the filesystem type to mount. Must be a filesystem type supported by the host operating system. Ex. "ext4", "xfs", "ntfs". Implicitly inferred to be "ext4" if unspecified.
   * - pdID
     - string
     - pdID is the ID that identifies Photon Controller persistent disk

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].portworxVolume:

.spec.datacenter.racks[].volumes[].portworxVolume
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
portworxVolume represents a portworx volume attached and mounted on kubelets host machine. Deprecated: PortworxVolume is deprecated. All operations for the in-tree portworxVolume type are redirected to the pxd.portworx.com CSI driver when the CSIMigrationPortworx feature-gate is on.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - fsType
     - string
     - fSType represents the filesystem type to mount Must be a filesystem type supported by the host operating system. Ex. "ext4", "xfs". Implicitly inferred to be "ext4" if unspecified.
   * - readOnly
     - boolean
     - readOnly defaults to false (read/write). ReadOnly here will force the ReadOnly setting in VolumeMounts.
   * - volumeID
     - string
     - volumeID uniquely identifies a Portworx volume

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].projected:

.spec.datacenter.racks[].volumes[].projected
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
projected items for all in one resources secrets, configmaps, and downward API

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - defaultMode
     - integer
     - defaultMode are the mode bits used to set permissions on created files by default. Must be an octal value between 0000 and 0777 or a decimal value between 0 and 511. YAML accepts both octal and decimal values, JSON requires decimal values for mode bits. Directories within the path are not affected by this setting. This might be in conflict with other options that affect the file mode, like fsGroup, and the result can be other mode bits set.
   * - :ref:`sources<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].projected.sources[]>`
     - array (object)
     - sources is the list of volume projections. Each entry in this list handles one source.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].projected.sources[]:

.spec.datacenter.racks[].volumes[].projected.sources[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
Projection that may be projected along with other supported volume types. Exactly one of these fields must be set.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`clusterTrustBundle<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].projected.sources[].clusterTrustBundle>`
     - object
     - ClusterTrustBundle allows a pod to access the `.spec.trustBundle` field of ClusterTrustBundle objects in an auto-updating file.  Alpha, gated by the ClusterTrustBundleProjection feature gate.  ClusterTrustBundle objects can either be selected by name, or by the combination of signer name and a label selector.  Kubelet performs aggressive normalization of the PEM contents written into the pod filesystem.  Esoteric PEM features such as inter-block comments and block headers are stripped.  Certificates are deduplicated. The ordering of certificates within the file is arbitrary, and Kubelet may change the order over time.
   * - :ref:`configMap<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].projected.sources[].configMap>`
     - object
     - configMap information about the configMap data to project
   * - :ref:`downwardAPI<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].projected.sources[].downwardAPI>`
     - object
     - downwardAPI information about the downwardAPI data to project
   * - :ref:`podCertificate<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].projected.sources[].podCertificate>`
     - object
     - Projects an auto-rotating credential bundle (private key and certificate chain) that the pod can use either as a TLS client or server.  Kubelet generates a private key and uses it to send a PodCertificateRequest to the named signer.  Once the signer approves the request and issues a certificate chain, Kubelet writes the key and certificate chain to the pod filesystem.  The pod does not start until certificates have been issued for each podCertificate projected volume source in its spec.  Kubelet will begin trying to rotate the certificate at the time indicated by the signer using the PodCertificateRequest.Status.BeginRefreshAt timestamp.  Kubelet can write a single file, indicated by the credentialBundlePath field, or separate files, indicated by the keyPath and certificateChainPath fields.  The credential bundle is a single file in PEM format.  The first PEM entry is the private key (in PKCS#8 format), and the remaining PEM entries are the certificate chain issued by the signer (typically, signers will return their certificate chain in leaf-to-root order).  Prefer using the credential bundle format, since your application code can read it atomically.  If you use keyPath and certificateChainPath, your application must make two separate file reads. If these coincide with a certificate rotation, it is possible that the private key and leaf certificate you read may not correspond to each other.  Your application will need to check for this condition, and re-read until they are consistent.  The named signer controls chooses the format of the certificate it issues; consult the signer implementation's documentation to learn how to use the certificates it issues.
   * - :ref:`secret<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].projected.sources[].secret>`
     - object
     - secret information about the secret data to project
   * - :ref:`serviceAccountToken<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].projected.sources[].serviceAccountToken>`
     - object
     - serviceAccountToken is information about the serviceAccountToken data to project

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].projected.sources[].clusterTrustBundle:

.spec.datacenter.racks[].volumes[].projected.sources[].clusterTrustBundle
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
ClusterTrustBundle allows a pod to access the `.spec.trustBundle` field of ClusterTrustBundle objects in an auto-updating file.  Alpha, gated by the ClusterTrustBundleProjection feature gate.  ClusterTrustBundle objects can either be selected by name, or by the combination of signer name and a label selector.  Kubelet performs aggressive normalization of the PEM contents written into the pod filesystem.  Esoteric PEM features such as inter-block comments and block headers are stripped.  Certificates are deduplicated. The ordering of certificates within the file is arbitrary, and Kubelet may change the order over time.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`labelSelector<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].projected.sources[].clusterTrustBundle.labelSelector>`
     - object
     - Select all ClusterTrustBundles that match this label selector.  Only has effect if signerName is set.  Mutually-exclusive with name.  If unset, interpreted as "match nothing".  If set but empty, interpreted as "match everything".
   * - name
     - string
     - Select a single ClusterTrustBundle by object name.  Mutually-exclusive with signerName and labelSelector.
   * - optional
     - boolean
     - If true, don't block pod startup if the referenced ClusterTrustBundle(s) aren't available.  If using name, then the named ClusterTrustBundle is allowed not to exist.  If using signerName, then the combination of signerName and labelSelector is allowed to match zero ClusterTrustBundles.
   * - path
     - string
     - Relative path from the volume root to write the bundle.
   * - signerName
     - string
     - Select all ClusterTrustBundles that match this signer name. Mutually-exclusive with name.  The contents of all selected ClusterTrustBundles will be unified and deduplicated.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].projected.sources[].clusterTrustBundle.labelSelector:

.spec.datacenter.racks[].volumes[].projected.sources[].clusterTrustBundle.labelSelector
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
Select all ClusterTrustBundles that match this label selector.  Only has effect if signerName is set.  Mutually-exclusive with name.  If unset, interpreted as "match nothing".  If set but empty, interpreted as "match everything".

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`matchExpressions<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].projected.sources[].clusterTrustBundle.labelSelector.matchExpressions[]>`
     - array (object)
     - matchExpressions is a list of label selector requirements. The requirements are ANDed.
   * - :ref:`matchLabels<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].projected.sources[].clusterTrustBundle.labelSelector.matchLabels>`
     - object
     - matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].projected.sources[].clusterTrustBundle.labelSelector.matchExpressions[]:

.spec.datacenter.racks[].volumes[].projected.sources[].clusterTrustBundle.labelSelector.matchExpressions[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - key
     - string
     - key is the label key that the selector applies to.
   * - operator
     - string
     - operator represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists and DoesNotExist.
   * - values
     - array (string)
     - values is an array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].projected.sources[].clusterTrustBundle.labelSelector.matchLabels:

.spec.datacenter.racks[].volumes[].projected.sources[].clusterTrustBundle.labelSelector.matchLabels
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

Type
""""
object


.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].projected.sources[].configMap:

.spec.datacenter.racks[].volumes[].projected.sources[].configMap
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
configMap information about the configMap data to project

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`items<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].projected.sources[].configMap.items[]>`
     - array (object)
     - items if unspecified, each key-value pair in the Data field of the referenced ConfigMap will be projected into the volume as a file whose name is the key and content is the value. If specified, the listed keys will be projected into the specified paths, and unlisted keys will not be present. If a key is specified which is not present in the ConfigMap, the volume setup will error unless it is marked optional. Paths must be relative and may not contain the '..' path or start with '..'.
   * - name
     - string
     - Name of the referent. This field is effectively required, but due to backwards compatibility is allowed to be empty. Instances of this type with an empty value here are almost certainly wrong. More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names
   * - optional
     - boolean
     - optional specify whether the ConfigMap or its keys must be defined

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].projected.sources[].configMap.items[]:

.spec.datacenter.racks[].volumes[].projected.sources[].configMap.items[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
Maps a string key to a path within a volume.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - key
     - string
     - key is the key to project.
   * - mode
     - integer
     - mode is Optional: mode bits used to set permissions on this file. Must be an octal value between 0000 and 0777 or a decimal value between 0 and 511. YAML accepts both octal and decimal values, JSON requires decimal values for mode bits. If not specified, the volume defaultMode will be used. This might be in conflict with other options that affect the file mode, like fsGroup, and the result can be other mode bits set.
   * - path
     - string
     - path is the relative path of the file to map the key to. May not be an absolute path. May not contain the path element '..'. May not start with the string '..'.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].projected.sources[].downwardAPI:

.spec.datacenter.racks[].volumes[].projected.sources[].downwardAPI
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
downwardAPI information about the downwardAPI data to project

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`items<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].projected.sources[].downwardAPI.items[]>`
     - array (object)
     - Items is a list of DownwardAPIVolume file

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].projected.sources[].downwardAPI.items[]:

.spec.datacenter.racks[].volumes[].projected.sources[].downwardAPI.items[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
DownwardAPIVolumeFile represents information to create the file containing the pod field

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`fieldRef<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].projected.sources[].downwardAPI.items[].fieldRef>`
     - object
     - Required: Selects a field of the pod: only annotations, labels, name, namespace and uid are supported.
   * - mode
     - integer
     - Optional: mode bits used to set permissions on this file, must be an octal value between 0000 and 0777 or a decimal value between 0 and 511. YAML accepts both octal and decimal values, JSON requires decimal values for mode bits. If not specified, the volume defaultMode will be used. This might be in conflict with other options that affect the file mode, like fsGroup, and the result can be other mode bits set.
   * - path
     - string
     - Required: Path is  the relative path name of the file to be created. Must not be absolute or contain the '..' path. Must be utf-8 encoded. The first item of the relative path must not start with '..'
   * - :ref:`resourceFieldRef<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].projected.sources[].downwardAPI.items[].resourceFieldRef>`
     - object
     - Selects a resource of the container: only resources limits and requests (limits.cpu, limits.memory, requests.cpu and requests.memory) are currently supported.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].projected.sources[].downwardAPI.items[].fieldRef:

.spec.datacenter.racks[].volumes[].projected.sources[].downwardAPI.items[].fieldRef
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
Required: Selects a field of the pod: only annotations, labels, name, namespace and uid are supported.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - apiVersion
     - string
     - Version of the schema the FieldPath is written in terms of, defaults to "v1".
   * - fieldPath
     - string
     - Path of the field to select in the specified API version.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].projected.sources[].downwardAPI.items[].resourceFieldRef:

.spec.datacenter.racks[].volumes[].projected.sources[].downwardAPI.items[].resourceFieldRef
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
Selects a resource of the container: only resources limits and requests (limits.cpu, limits.memory, requests.cpu and requests.memory) are currently supported.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - containerName
     - string
     - Container name: required for volumes, optional for env vars
   * - divisor
     - 
     - Specifies the output format of the exposed resources, defaults to "1"
   * - resource
     - string
     - Required: resource to select

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].projected.sources[].podCertificate:

.spec.datacenter.racks[].volumes[].projected.sources[].podCertificate
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
Projects an auto-rotating credential bundle (private key and certificate chain) that the pod can use either as a TLS client or server.  Kubelet generates a private key and uses it to send a PodCertificateRequest to the named signer.  Once the signer approves the request and issues a certificate chain, Kubelet writes the key and certificate chain to the pod filesystem.  The pod does not start until certificates have been issued for each podCertificate projected volume source in its spec.  Kubelet will begin trying to rotate the certificate at the time indicated by the signer using the PodCertificateRequest.Status.BeginRefreshAt timestamp.  Kubelet can write a single file, indicated by the credentialBundlePath field, or separate files, indicated by the keyPath and certificateChainPath fields.  The credential bundle is a single file in PEM format.  The first PEM entry is the private key (in PKCS#8 format), and the remaining PEM entries are the certificate chain issued by the signer (typically, signers will return their certificate chain in leaf-to-root order).  Prefer using the credential bundle format, since your application code can read it atomically.  If you use keyPath and certificateChainPath, your application must make two separate file reads. If these coincide with a certificate rotation, it is possible that the private key and leaf certificate you read may not correspond to each other.  Your application will need to check for this condition, and re-read until they are consistent.  The named signer controls chooses the format of the certificate it issues; consult the signer implementation's documentation to learn how to use the certificates it issues.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - certificateChainPath
     - string
     - Write the certificate chain at this path in the projected volume.  Most applications should use credentialBundlePath.  When using keyPath and certificateChainPath, your application needs to check that the key and leaf certificate are consistent, because it is possible to read the files mid-rotation.
   * - credentialBundlePath
     - string
     - Write the credential bundle at this path in the projected volume.  The credential bundle is a single file that contains multiple PEM blocks. The first PEM block is a PRIVATE KEY block, containing a PKCS#8 private key.  The remaining blocks are CERTIFICATE blocks, containing the issued certificate chain from the signer (leaf and any intermediates).  Using credentialBundlePath lets your Pod's application code make a single atomic read that retrieves a consistent key and certificate chain.  If you project them to separate files, your application code will need to additionally check that the leaf certificate was issued to the key.
   * - keyPath
     - string
     - Write the key at this path in the projected volume.  Most applications should use credentialBundlePath.  When using keyPath and certificateChainPath, your application needs to check that the key and leaf certificate are consistent, because it is possible to read the files mid-rotation.
   * - keyType
     - string
     - The type of keypair Kubelet will generate for the pod.  Valid values are "RSA3072", "RSA4096", "ECDSAP256", "ECDSAP384", "ECDSAP521", and "ED25519".
   * - maxExpirationSeconds
     - integer
     - maxExpirationSeconds is the maximum lifetime permitted for the certificate.  Kubelet copies this value verbatim into the PodCertificateRequests it generates for this projection.  If omitted, kube-apiserver will set it to 86400(24 hours). kube-apiserver will reject values shorter than 3600 (1 hour).  The maximum allowable value is 7862400 (91 days).  The signer implementation is then free to issue a certificate with any lifetime *shorter* than MaxExpirationSeconds, but no shorter than 3600 seconds (1 hour).  This constraint is enforced by kube-apiserver. `kubernetes.io` signers will never issue certificates with a lifetime longer than 24 hours.
   * - signerName
     - string
     - Kubelet's generated CSRs will be addressed to this signer.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].projected.sources[].secret:

.spec.datacenter.racks[].volumes[].projected.sources[].secret
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
secret information about the secret data to project

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`items<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].projected.sources[].secret.items[]>`
     - array (object)
     - items if unspecified, each key-value pair in the Data field of the referenced Secret will be projected into the volume as a file whose name is the key and content is the value. If specified, the listed keys will be projected into the specified paths, and unlisted keys will not be present. If a key is specified which is not present in the Secret, the volume setup will error unless it is marked optional. Paths must be relative and may not contain the '..' path or start with '..'.
   * - name
     - string
     - Name of the referent. This field is effectively required, but due to backwards compatibility is allowed to be empty. Instances of this type with an empty value here are almost certainly wrong. More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names
   * - optional
     - boolean
     - optional field specify whether the Secret or its key must be defined

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].projected.sources[].secret.items[]:

.spec.datacenter.racks[].volumes[].projected.sources[].secret.items[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
Maps a string key to a path within a volume.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - key
     - string
     - key is the key to project.
   * - mode
     - integer
     - mode is Optional: mode bits used to set permissions on this file. Must be an octal value between 0000 and 0777 or a decimal value between 0 and 511. YAML accepts both octal and decimal values, JSON requires decimal values for mode bits. If not specified, the volume defaultMode will be used. This might be in conflict with other options that affect the file mode, like fsGroup, and the result can be other mode bits set.
   * - path
     - string
     - path is the relative path of the file to map the key to. May not be an absolute path. May not contain the path element '..'. May not start with the string '..'.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].projected.sources[].serviceAccountToken:

.spec.datacenter.racks[].volumes[].projected.sources[].serviceAccountToken
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
serviceAccountToken is information about the serviceAccountToken data to project

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - audience
     - string
     - audience is the intended audience of the token. A recipient of a token must identify itself with an identifier specified in the audience of the token, and otherwise should reject the token. The audience defaults to the identifier of the apiserver.
   * - expirationSeconds
     - integer
     - expirationSeconds is the requested duration of validity of the service account token. As the token approaches expiration, the kubelet volume plugin will proactively rotate the service account token. The kubelet will start trying to rotate the token if the token is older than 80 percent of its time to live or if the token is older than 24 hours.Defaults to 1 hour and must be at least 10 minutes.
   * - path
     - string
     - path is the path relative to the mount point of the file to project the token into.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].quobyte:

.spec.datacenter.racks[].volumes[].quobyte
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
quobyte represents a Quobyte mount on the host that shares a pod's lifetime. Deprecated: Quobyte is deprecated and the in-tree quobyte type is no longer supported.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - group
     - string
     - group to map volume access to Default is no group
   * - readOnly
     - boolean
     - readOnly here will force the Quobyte volume to be mounted with read-only permissions. Defaults to false.
   * - registry
     - string
     - registry represents a single or multiple Quobyte Registry services specified as a string as host:port pair (multiple entries are separated with commas) which acts as the central registry for volumes
   * - tenant
     - string
     - tenant owning the given Quobyte volume in the Backend Used with dynamically provisioned Quobyte volumes, value is set by the plugin
   * - user
     - string
     - user to map volume access to Defaults to serivceaccount user
   * - volume
     - string
     - volume is a string that references an already created Quobyte volume by name.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].rbd:

.spec.datacenter.racks[].volumes[].rbd
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
rbd represents a Rados Block Device mount on the host that shares a pod's lifetime. Deprecated: RBD is deprecated and the in-tree rbd type is no longer supported.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - fsType
     - string
     - fsType is the filesystem type of the volume that you want to mount. Tip: Ensure that the filesystem type is supported by the host operating system. Examples: "ext4", "xfs", "ntfs". Implicitly inferred to be "ext4" if unspecified. More info: https://kubernetes.io/docs/concepts/storage/volumes#rbd
   * - image
     - string
     - image is the rados image name. More info: https://examples.k8s.io/volumes/rbd/README.md#how-to-use-it
   * - keyring
     - string
     - keyring is the path to key ring for RBDUser. Default is /etc/ceph/keyring. More info: https://examples.k8s.io/volumes/rbd/README.md#how-to-use-it
   * - monitors
     - array (string)
     - monitors is a collection of Ceph monitors. More info: https://examples.k8s.io/volumes/rbd/README.md#how-to-use-it
   * - pool
     - string
     - pool is the rados pool name. Default is rbd. More info: https://examples.k8s.io/volumes/rbd/README.md#how-to-use-it
   * - readOnly
     - boolean
     - readOnly here will force the ReadOnly setting in VolumeMounts. Defaults to false. More info: https://examples.k8s.io/volumes/rbd/README.md#how-to-use-it
   * - :ref:`secretRef<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].rbd.secretRef>`
     - object
     - secretRef is name of the authentication secret for RBDUser. If provided overrides keyring. Default is nil. More info: https://examples.k8s.io/volumes/rbd/README.md#how-to-use-it
   * - user
     - string
     - user is the rados user name. Default is admin. More info: https://examples.k8s.io/volumes/rbd/README.md#how-to-use-it

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].rbd.secretRef:

.spec.datacenter.racks[].volumes[].rbd.secretRef
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
secretRef is name of the authentication secret for RBDUser. If provided overrides keyring. Default is nil. More info: https://examples.k8s.io/volumes/rbd/README.md#how-to-use-it

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - name
     - string
     - Name of the referent. This field is effectively required, but due to backwards compatibility is allowed to be empty. Instances of this type with an empty value here are almost certainly wrong. More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].scaleIO:

.spec.datacenter.racks[].volumes[].scaleIO
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
scaleIO represents a ScaleIO persistent volume attached and mounted on Kubernetes nodes. Deprecated: ScaleIO is deprecated and the in-tree scaleIO type is no longer supported.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - fsType
     - string
     - fsType is the filesystem type to mount. Must be a filesystem type supported by the host operating system. Ex. "ext4", "xfs", "ntfs". Default is "xfs".
   * - gateway
     - string
     - gateway is the host address of the ScaleIO API Gateway.
   * - protectionDomain
     - string
     - protectionDomain is the name of the ScaleIO Protection Domain for the configured storage.
   * - readOnly
     - boolean
     - readOnly Defaults to false (read/write). ReadOnly here will force the ReadOnly setting in VolumeMounts.
   * - :ref:`secretRef<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].scaleIO.secretRef>`
     - object
     - secretRef references to the secret for ScaleIO user and other sensitive information. If this is not provided, Login operation will fail.
   * - sslEnabled
     - boolean
     - sslEnabled Flag enable/disable SSL communication with Gateway, default false
   * - storageMode
     - string
     - storageMode indicates whether the storage for a volume should be ThickProvisioned or ThinProvisioned. Default is ThinProvisioned.
   * - storagePool
     - string
     - storagePool is the ScaleIO Storage Pool associated with the protection domain.
   * - system
     - string
     - system is the name of the storage system as configured in ScaleIO.
   * - volumeName
     - string
     - volumeName is the name of a volume already created in the ScaleIO system that is associated with this volume source.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].scaleIO.secretRef:

.spec.datacenter.racks[].volumes[].scaleIO.secretRef
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
secretRef references to the secret for ScaleIO user and other sensitive information. If this is not provided, Login operation will fail.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - name
     - string
     - Name of the referent. This field is effectively required, but due to backwards compatibility is allowed to be empty. Instances of this type with an empty value here are almost certainly wrong. More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].secret:

.spec.datacenter.racks[].volumes[].secret
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
secret represents a secret that should populate this volume. More info: https://kubernetes.io/docs/concepts/storage/volumes#secret

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - defaultMode
     - integer
     - defaultMode is Optional: mode bits used to set permissions on created files by default. Must be an octal value between 0000 and 0777 or a decimal value between 0 and 511. YAML accepts both octal and decimal values, JSON requires decimal values for mode bits. Defaults to 0644. Directories within the path are not affected by this setting. This might be in conflict with other options that affect the file mode, like fsGroup, and the result can be other mode bits set.
   * - :ref:`items<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].secret.items[]>`
     - array (object)
     - items If unspecified, each key-value pair in the Data field of the referenced Secret will be projected into the volume as a file whose name is the key and content is the value. If specified, the listed keys will be projected into the specified paths, and unlisted keys will not be present. If a key is specified which is not present in the Secret, the volume setup will error unless it is marked optional. Paths must be relative and may not contain the '..' path or start with '..'.
   * - optional
     - boolean
     - optional field specify whether the Secret or its keys must be defined
   * - secretName
     - string
     - secretName is the name of the secret in the pod's namespace to use. More info: https://kubernetes.io/docs/concepts/storage/volumes#secret

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].secret.items[]:

.spec.datacenter.racks[].volumes[].secret.items[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
Maps a string key to a path within a volume.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - key
     - string
     - key is the key to project.
   * - mode
     - integer
     - mode is Optional: mode bits used to set permissions on this file. Must be an octal value between 0000 and 0777 or a decimal value between 0 and 511. YAML accepts both octal and decimal values, JSON requires decimal values for mode bits. If not specified, the volume defaultMode will be used. This might be in conflict with other options that affect the file mode, like fsGroup, and the result can be other mode bits set.
   * - path
     - string
     - path is the relative path of the file to map the key to. May not be an absolute path. May not contain the path element '..'. May not start with the string '..'.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].storageos:

.spec.datacenter.racks[].volumes[].storageos
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
storageOS represents a StorageOS volume attached and mounted on Kubernetes nodes. Deprecated: StorageOS is deprecated and the in-tree storageos type is no longer supported.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - fsType
     - string
     - fsType is the filesystem type to mount. Must be a filesystem type supported by the host operating system. Ex. "ext4", "xfs", "ntfs". Implicitly inferred to be "ext4" if unspecified.
   * - readOnly
     - boolean
     - readOnly defaults to false (read/write). ReadOnly here will force the ReadOnly setting in VolumeMounts.
   * - :ref:`secretRef<api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].storageos.secretRef>`
     - object
     - secretRef specifies the secret to use for obtaining the StorageOS API credentials.  If not specified, default values will be attempted.
   * - volumeName
     - string
     - volumeName is the human-readable name of the StorageOS volume.  Volume names are only unique within a namespace.
   * - volumeNamespace
     - string
     - volumeNamespace specifies the scope of the volume within StorageOS.  If no namespace is specified then the Pod's namespace will be used.  This allows the Kubernetes name scoping to be mirrored within StorageOS for tighter integration. Set VolumeName to any name to override the default behaviour. Set to "default" if you are not using namespaces within StorageOS. Namespaces that do not pre-exist within StorageOS will be created.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].storageos.secretRef:

.spec.datacenter.racks[].volumes[].storageos.secretRef
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
secretRef specifies the secret to use for obtaining the StorageOS API credentials.  If not specified, default values will be attempted.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - name
     - string
     - Name of the referent. This field is effectively required, but due to backwards compatibility is allowed to be empty. Instances of this type with an empty value here are almost certainly wrong. More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].volumes[].vsphereVolume:

.spec.datacenter.racks[].volumes[].vsphereVolume
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
vsphereVolume represents a vSphere volume attached and mounted on kubelets host machine. Deprecated: VsphereVolume is deprecated. All operations for the in-tree vsphereVolume type are redirected to the csi.vsphere.vmware.com CSI driver.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - fsType
     - string
     - fsType is filesystem type to mount. Must be a filesystem type supported by the host operating system. Ex. "ext4", "xfs", "ntfs". Implicitly inferred to be "ext4" if unspecified.
   * - storagePolicyID
     - string
     - storagePolicyID is the storage Policy Based Management (SPBM) profile ID associated with the StoragePolicyName.
   * - storagePolicyName
     - string
     - storagePolicyName is the storage Policy Based Management (SPBM) profile name.
   * - volumePath
     - string
     - volumePath is the path that identifies vSphere volume vmdk

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.exposeOptions:

.spec.exposeOptions
^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
exposeOptions specifies options for exposing ScyllaCluster services. This field is immutable. EXPERIMENTAL. Do not rely on any particular behaviour controlled by this field.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`broadcastOptions<api-scylla.scylladb.com-scyllaclusters-v1-.spec.exposeOptions.broadcastOptions>`
     - object
     - BroadcastOptions defines how ScyllaDB node publishes its IP address to other nodes and clients.
   * - :ref:`cql<api-scylla.scylladb.com-scyllaclusters-v1-.spec.exposeOptions.cql>`
     - object
     - cql specifies expose options for CQL SSL backend. EXPERIMENTAL. Do not rely on any particular behaviour controlled by this field.
   * - :ref:`nodeService<api-scylla.scylladb.com-scyllaclusters-v1-.spec.exposeOptions.nodeService>`
     - object
     - nodeService controls properties of Service dedicated for each ScyllaCluster node.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.exposeOptions.broadcastOptions:

.spec.exposeOptions.broadcastOptions
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
BroadcastOptions defines how ScyllaDB node publishes its IP address to other nodes and clients.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`clients<api-scylla.scylladb.com-scyllaclusters-v1-.spec.exposeOptions.broadcastOptions.clients>`
     - object
     - clients specifies options related to the address that is broadcasted for communication with clients. This field controls the `broadcast_rpc_address` value in ScyllaDB config.
   * - :ref:`nodes<api-scylla.scylladb.com-scyllaclusters-v1-.spec.exposeOptions.broadcastOptions.nodes>`
     - object
     - nodes specifies options related to the address that is broadcasted for communication with other nodes. This field controls the `broadcast_address` value in ScyllaDB config.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.exposeOptions.broadcastOptions.clients:

.spec.exposeOptions.broadcastOptions.clients
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
clients specifies options related to the address that is broadcasted for communication with clients. This field controls the `broadcast_rpc_address` value in ScyllaDB config.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`podIP<api-scylla.scylladb.com-scyllaclusters-v1-.spec.exposeOptions.broadcastOptions.clients.podIP>`
     - object
     - podIP holds options related to Pod IP address.
   * - type
     - string
     - type of the address that is broadcasted.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.exposeOptions.broadcastOptions.clients.podIP:

.spec.exposeOptions.broadcastOptions.clients.podIP
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
podIP holds options related to Pod IP address.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - source
     - string
     - sourceType specifies source of the Pod IP.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.exposeOptions.broadcastOptions.nodes:

.spec.exposeOptions.broadcastOptions.nodes
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
nodes specifies options related to the address that is broadcasted for communication with other nodes. This field controls the `broadcast_address` value in ScyllaDB config.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`podIP<api-scylla.scylladb.com-scyllaclusters-v1-.spec.exposeOptions.broadcastOptions.nodes.podIP>`
     - object
     - podIP holds options related to Pod IP address.
   * - type
     - string
     - type of the address that is broadcasted.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.exposeOptions.broadcastOptions.nodes.podIP:

.spec.exposeOptions.broadcastOptions.nodes.podIP
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
podIP holds options related to Pod IP address.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - source
     - string
     - sourceType specifies source of the Pod IP.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.exposeOptions.cql:

.spec.exposeOptions.cql
^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
cql specifies expose options for CQL SSL backend. EXPERIMENTAL. Do not rely on any particular behaviour controlled by this field.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`ingress<api-scylla.scylladb.com-scyllaclusters-v1-.spec.exposeOptions.cql.ingress>`
     - object
     - ingress is an Ingress configuration options. EXPERIMENTAL. Do not rely on any particular behaviour controlled by this field.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.exposeOptions.cql.ingress:

.spec.exposeOptions.cql.ingress
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
ingress is an Ingress configuration options. EXPERIMENTAL. Do not rely on any particular behaviour controlled by this field.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`annotations<api-scylla.scylladb.com-scyllaclusters-v1-.spec.exposeOptions.cql.ingress.annotations>`
     - object
     - annotations is a custom key value map that gets merged with managed object annotations.
   * - disabled
     - boolean
     - disabled controls if Ingress object creation is disabled. Unless disabled, there is an Ingress objects created for every Scylla node. EXPERIMENTAL. Do not rely on any particular behaviour controlled by this field.
   * - ingressClassName
     - string
     - ingressClassName specifies Ingress class name. EXPERIMENTAL. Do not rely on any particular behaviour controlled by this field.
   * - :ref:`labels<api-scylla.scylladb.com-scyllaclusters-v1-.spec.exposeOptions.cql.ingress.labels>`
     - object
     - labels is a custom key value map that gets merged with managed object labels.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.exposeOptions.cql.ingress.annotations:

.spec.exposeOptions.cql.ingress.annotations
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
annotations is a custom key value map that gets merged with managed object annotations.

Type
""""
object


.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.exposeOptions.cql.ingress.labels:

.spec.exposeOptions.cql.ingress.labels
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
labels is a custom key value map that gets merged with managed object labels.

Type
""""
object


.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.exposeOptions.nodeService:

.spec.exposeOptions.nodeService
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
nodeService controls properties of Service dedicated for each ScyllaCluster node.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - allocateLoadBalancerNodePorts
     - boolean
     - allocateLoadBalancerNodePorts controls value of service.spec.allocateLoadBalancerNodePorts of each node Service. Check Kubernetes corev1.Service documentation about semantic of this field.
   * - :ref:`annotations<api-scylla.scylladb.com-scyllaclusters-v1-.spec.exposeOptions.nodeService.annotations>`
     - object
     - annotations is a custom key value map that gets merged with managed object annotations.
   * - externalTrafficPolicy
     - string
     - externalTrafficPolicy controls value of service.spec.externalTrafficPolicy of each node Service. Check Kubernetes corev1.Service documentation about semantic of this field.
   * - internalTrafficPolicy
     - string
     - internalTrafficPolicy controls value of service.spec.internalTrafficPolicy of each node Service. Check Kubernetes corev1.Service documentation about semantic of this field.
   * - :ref:`labels<api-scylla.scylladb.com-scyllaclusters-v1-.spec.exposeOptions.nodeService.labels>`
     - object
     - labels is a custom key value map that gets merged with managed object labels.
   * - loadBalancerClass
     - string
     - loadBalancerClass controls value of service.spec.loadBalancerClass of each node Service. Check Kubernetes corev1.Service documentation about semantic of this field.
   * - type
     - string
     - type is the Kubernetes Service type.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.exposeOptions.nodeService.annotations:

.spec.exposeOptions.nodeService.annotations
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
annotations is a custom key value map that gets merged with managed object annotations.

Type
""""
object


.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.exposeOptions.nodeService.labels:

.spec.exposeOptions.nodeService.labels
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
labels is a custom key value map that gets merged with managed object labels.

Type
""""
object


.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.genericUpgrade:

.spec.genericUpgrade
^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
genericUpgrade allows to configure behavior of generic upgrade logic.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - failureStrategy
     - string
     - failureStrategy specifies which logic is executed when upgrade failure happens. Currently only Retry is supported.
   * - pollInterval
     - string
     - pollInterval specifies how often upgrade logic polls on state updates. Increasing this value should lower number of requests sent to apiserver, but it may affect overall time spent during upgrade. DEPRECATED.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.imagePullSecrets[]:

.spec.imagePullSecrets[]
^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
LocalObjectReference contains enough information to let you locate the referenced object inside the same namespace.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - name
     - string
     - Name of the referent. This field is effectively required, but due to backwards compatibility is allowed to be empty. Instances of this type with an empty value here are almost certainly wrong. More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.network:

.spec.network
^^^^^^^^^^^^^

Description
"""""""""""
network holds the networking config.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - dnsPolicy
     - string
     - dnsPolicy defines how a pod's DNS will be configured.
   * - hostNetworking
     - boolean
     - hostNetworking determines if scylla uses the host's network namespace. Setting this option avoids going through Kubernetes SDN and exposes scylla on node's IP. Deprecated: `hostNetworking` is deprecated and may be ignored in the future.
   * - ipFamilies
     - array (string)
     - ipFamilies specifies the IP families to use. Supports: IPv4, IPv6.
   * - ipFamilyPolicy
     - string
     - ipFamilyPolicy specifies the IP family policy for the cluster. Supports: SingleStack, PreferDualStack, RequireDualStack.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.podMetadata:

.spec.podMetadata
^^^^^^^^^^^^^^^^^

Description
"""""""""""
podMetadata controls shared metadata for all pods created based on this spec.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`annotations<api-scylla.scylladb.com-scyllaclusters-v1-.spec.podMetadata.annotations>`
     - object
     - annotations is a custom key value map that gets merged with managed object annotations.
   * - :ref:`labels<api-scylla.scylladb.com-scyllaclusters-v1-.spec.podMetadata.labels>`
     - object
     - labels is a custom key value map that gets merged with managed object labels.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.podMetadata.annotations:

.spec.podMetadata.annotations
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
annotations is a custom key value map that gets merged with managed object annotations.

Type
""""
object


.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.podMetadata.labels:

.spec.podMetadata.labels
^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
labels is a custom key value map that gets merged with managed object labels.

Type
""""
object


.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.readinessGates[]:

.spec.readinessGates[]
^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
PodReadinessGate contains the reference to a pod condition

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - conditionType
     - string
     - ConditionType refers to a condition in the pod's condition list with matching type.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.spec.repairs[]:

.spec.repairs[]
^^^^^^^^^^^^^^^

Description
"""""""""""


Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - cron
     - string
     - cron specifies the task schedule as a cron expression. It supports an extended syntax including @monthly, @weekly, @daily, @midnight, @hourly, @every X[h|m|s].
   * - dc
     - array (string)
     - dc is a list of datacenter glob patterns, e.g. 'dc1', '!otherdc*' used to specify the DCs to include or exclude from backup.
   * - failFast
     - boolean
     - failFast indicates if a repair should be stopped on first error.
   * - host
     - string
     - host specifies a host to repair. If empty, all hosts are repaired.
   * - ignoreDownHosts
     - boolean
     - ignoreDownHosts indicates that the nodes in down state should be ignored during repair.
   * - intensity
     - string
     - intensity indicates how many token ranges (per shard) to repair in a single Scylla repair job. By default this is 1. If you set it to 0 the number of token ranges is adjusted to the maximum supported by node (see max_repair_ranges_in_parallel in Scylla logs). Valid values are 0 and integers >= 1. Higher values will result in increased cluster load and slightly faster repairs. Changing the intensity impacts repair granularity if you need to resume it, the higher the value the more work on resume. For Scylla clusters that *do not support row-level repair*, intensity can be a decimal between (0,1). In that case it specifies percent of shards that can be repaired in parallel on a repair master node. For Scylla clusters that are row-level repair enabled, setting intensity below 1 has the same effect as setting intensity 1.
   * - interval
     - string
     - interval represents a task schedule interval e.g. 3d2h10m, valid units are d, h, m, s. Deprecated: please use cron instead.
   * - keyspace
     - array (string)
     - keyspace is a list of keyspace/tables glob patterns, e.g. 'keyspace,!keyspace.table_prefix_*' used to include or exclude keyspaces from repair.
   * - name
     - string
     - name specifies the name of a task.
   * - numRetries
     - integer
     - numRetries indicates how many times a scheduled task will be retried before failing.
   * - parallel
     - integer
     - parallel is the maximum number of Scylla repair jobs that can run at the same time (on different token ranges and replicas). Each node can take part in at most one repair at any given moment. By default the maximum possible parallelism is used. The effective parallelism depends on a keyspace replication factor (RF) and the number of nodes. The formula to calculate it is as follows: number of nodes / RF, ex. for 6 node cluster with RF=3 the maximum parallelism is 2.
   * - retryWait
     - string
     - retryWait specifies the initial exponential backoff duration for task retries. For instance, if set to 10 minutes, the first retry will be attempted after 10 minutes, the second after 20 minutes, the third after 40 minutes, and so on, up to the number of retries specified in `numRetries`. If not set, the default values is left to ScyllaDB Manager to decide.
   * - smallTableThreshold
     - string
     - smallTableThreshold enable small table optimization for tables of size lower than given threshold. Supported units [B, MiB, GiB, TiB].
   * - startDate
     - string
     - startDate specifies the task start date expressed in the RFC3339 format or now[+duration], e.g. now+3d2h10m, valid units are d, h, m, s.
   * - timezone
     - string
     - timezone specifies the timezone of cron field.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.status:

.status
^^^^^^^

Description
"""""""""""
status is the current status of this scylla cluster.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - availableMembers
     - integer
     - availableMembers is the number of ScyllaDB members in all racks that are available.
   * - :ref:`backups<api-scylla.scylladb.com-scyllaclusters-v1-.status.backups[]>`
     - array (object)
     - backups reflects status of backup tasks.
   * - :ref:`conditions<api-scylla.scylladb.com-scyllaclusters-v1-.status.conditions[]>`
     - array (object)
     - conditions hold conditions describing ScyllaCluster state. To determine whether a cluster rollout is finished, look for Available=True,Progressing=False,Degraded=False.
   * - managerId
     - string
     - managerId contains ID under which cluster was registered in Scylla Manager.
   * - members
     - integer
     - members is the number of ScyllaDB members in all racks.
   * - observedGeneration
     - integer
     - observedGeneration is the most recent generation observed for this ScyllaCluster. It corresponds to the ScyllaCluster's generation, which is updated on mutation by the API Server.
   * - rackCount
     - integer
     - rackCount is the number of ScyllaDB racks in this cluster.
   * - :ref:`racks<api-scylla.scylladb.com-scyllaclusters-v1-.status.racks>`
     - object
     - racks reflect status of cluster racks.
   * - readyMembers
     - integer
     - readyMembers is the number of ScyllaDB members in all racks that are ready.
   * - :ref:`repairs<api-scylla.scylladb.com-scyllaclusters-v1-.status.repairs[]>`
     - array (object)
     - repairs reflects status of repair tasks.
   * - :ref:`upgrade<api-scylla.scylladb.com-scyllaclusters-v1-.status.upgrade>`
     - object
     - upgrade reflects state of ongoing upgrade procedure.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.status.backups[]:

.status.backups[]
^^^^^^^^^^^^^^^^^

Description
"""""""""""


Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - cron
     - string
     - cron reflects the task schedule as a cron expression.
   * - dc
     - array (string)
     - dc reflects a list of datacenter glob patterns, e.g. 'dc1,!otherdc*' used to specify the DCs to include or exclude from backup.
   * - error
     - string
     - error holds the task error, if any.
   * - id
     - string
     - id reflects identification number of the repair task.
   * - interval
     - string
     - interval reflects a task schedule interval.
   * - keyspace
     - array (string)
     - keyspace reflects a list of keyspace/tables glob patterns, e.g. 'keyspace,!keyspace.table_prefix_*' used to include or exclude keyspaces from repair.
   * - :ref:`labels<api-scylla.scylladb.com-scyllaclusters-v1-.status.backups[].labels>`
     - object
     - labels reflects the labels of a task.
   * - location
     - array (string)
     - location reflects a list of backup locations in the format [<dc>:]<provider>:<name> ex. s3:my-bucket.
   * - name
     - string
     - name reflects the name of a task.
   * - numRetries
     - integer
     - numRetries reflects how many times a scheduled task will be retried before failing.
   * - rateLimit
     - array (string)
     - rateLimit reflects a list of megabytes (MiB) per second rate limits expressed in the format [<dc>:]<limit>.
   * - retention
     - integer
     - retention reflects the number of backups which are to be stored.
   * - retryWait
     - string
     - retryWait reflects the initial exponential backoff duration for task retries.
   * - snapshotParallel
     - array (string)
     - snapshotParallel reflects a list of snapshot parallelism limits in the format [<dc>:]<limit>.
   * - startDate
     - string
     - startDate reflects the task start date expressed in the RFC3339 format
   * - timezone
     - string
     - timezone reflects the timezone of cron field.
   * - uploadParallel
     - array (string)
     - uploadParallel reflects a list of upload parallelism limits in the format [<dc>:]<limit>.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.status.backups[].labels:

.status.backups[].labels
^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
labels reflects the labels of a task.

Type
""""
object


.. _api-scylla.scylladb.com-scyllaclusters-v1-.status.conditions[]:

.status.conditions[]
^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
Condition contains details for one aspect of the current state of this API Resource.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - lastTransitionTime
     - string
     - lastTransitionTime is the last time the condition transitioned from one status to another. This should be when the underlying condition changed.  If that is not known, then using the time when the API field changed is acceptable.
   * - message
     - string
     - message is a human readable message indicating details about the transition. This may be an empty string.
   * - observedGeneration
     - integer
     - observedGeneration represents the .metadata.generation that the condition was set based upon. For instance, if .metadata.generation is currently 12, but the .status.conditions[x].observedGeneration is 9, the condition is out of date with respect to the current state of the instance.
   * - reason
     - string
     - reason contains a programmatic identifier indicating the reason for the condition's last transition. Producers of specific condition types may define expected values and meanings for this field, and whether the values are considered a guaranteed API. The value should be a CamelCase string. This field may not be empty.
   * - status
     - string
     - status of the condition, one of True, False, Unknown.
   * - type
     - string
     - type of condition in CamelCase or in foo.example.com/CamelCase.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.status.racks:

.status.racks
^^^^^^^^^^^^^

Description
"""""""""""
racks reflect status of cluster racks.

Type
""""
object


.. _api-scylla.scylladb.com-scyllaclusters-v1-.status.repairs[]:

.status.repairs[]
^^^^^^^^^^^^^^^^^

Description
"""""""""""


Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - cron
     - string
     - cron reflects the task schedule as a cron expression.
   * - dc
     - array (string)
     - dc reflects a list of datacenter glob patterns, e.g. 'dc1', '!otherdc*' used to specify the DCs to include or exclude from repair.
   * - error
     - string
     - error holds the task error, if any.
   * - failFast
     - boolean
     - failFast indicates if a repair should be stopped on first error.
   * - host
     - string
     - host reflects a host to repair.
   * - id
     - string
     - id reflects identification number of the repair task.
   * - ignoreDownHosts
     - boolean
     - ignoreDownHosts reflects whether the nodes in down state are ignored during repair.
   * - intensity
     - string
     - intensity indicates how many token ranges (per shard) to repair in a single Scylla repair job. By default this is 1.
   * - interval
     - string
     - interval reflects a task schedule interval.
   * - keyspace
     - array (string)
     - keyspace reflects a list of keyspace/tables glob patterns, e.g. 'keyspace,!keyspace.table_prefix_*' used to include or exclude keyspaces from repair.
   * - :ref:`labels<api-scylla.scylladb.com-scyllaclusters-v1-.status.repairs[].labels>`
     - object
     - labels reflects the labels of a task.
   * - name
     - string
     - name reflects the name of a task.
   * - numRetries
     - integer
     - numRetries reflects how many times a scheduled task will be retried before failing.
   * - parallel
     - integer
     - parallel reflects the maximum number of Scylla repair jobs that can run at the same time (on different token ranges and replicas).
   * - retryWait
     - string
     - retryWait reflects the initial exponential backoff duration for task retries.
   * - smallTableThreshold
     - string
     - smallTableThreshold reflects whether small table optimization for tables, of size lower than given threshold, are enabled.
   * - startDate
     - string
     - startDate reflects the task start date expressed in the RFC3339 format
   * - timezone
     - string
     - timezone reflects the timezone of cron field.

.. _api-scylla.scylladb.com-scyllaclusters-v1-.status.repairs[].labels:

.status.repairs[].labels
^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
labels reflects the labels of a task.

Type
""""
object


.. _api-scylla.scylladb.com-scyllaclusters-v1-.status.upgrade:

.status.upgrade
^^^^^^^^^^^^^^^

Description
"""""""""""
upgrade reflects state of ongoing upgrade procedure.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - currentNode
     - string
     - currentNode node under upgrade. DEPRECATED.
   * - currentRack
     - string
     - currentRack rack under upgrade. DEPRECATED.
   * - dataSnapshotTag
     - string
     - dataSnapshotTag is the snapshot tag of data keyspaces.
   * - fromVersion
     - string
     - fromVersion reflects from which version ScyllaCluster is being upgraded.
   * - state
     - string
     - state reflects current upgrade state.
   * - systemSnapshotTag
     - string
     - systemSnapshotTag is the snapshot tag of system keyspaces.
   * - toVersion
     - string
     - toVersion reflects to which version ScyllaCluster is being upgraded.
