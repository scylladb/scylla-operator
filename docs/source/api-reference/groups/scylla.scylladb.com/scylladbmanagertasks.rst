ScyllaDBManagerTask (scylla.scylladb.com/v1alpha1)
==================================================

| **APIVersion**: scylla.scylladb.com/v1alpha1
| **Kind**: ScyllaDBManagerTask
| **PluralName**: scylladbmanagertasks
| **SingularName**: scylladbmanagertask
| **Scope**: Namespaced
| **ListKind**: ScyllaDBManagerTaskList
| **Served**: true
| **Storage**: true

Description
-----------


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
   * - :ref:`metadata<api-scylla.scylladb.com-scylladbmanagertasks-v1alpha1-.metadata>`
     - object
     - 
   * - :ref:`spec<api-scylla.scylladb.com-scylladbmanagertasks-v1alpha1-.spec>`
     - object
     - spec defines the desired state of ScyllaDBManagerTask.
   * - :ref:`status<api-scylla.scylladb.com-scylladbmanagertasks-v1alpha1-.status>`
     - object
     - status reflects the observed state of ScyllaDBManagerTask.

.. _api-scylla.scylladb.com-scylladbmanagertasks-v1alpha1-.metadata:

.metadata
^^^^^^^^^

Description
"""""""""""


Type
""""
object


.. _api-scylla.scylladb.com-scylladbmanagertasks-v1alpha1-.spec:

.spec
^^^^^

Description
"""""""""""
spec defines the desired state of ScyllaDBManagerTask.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`backup<api-scylla.scylladb.com-scylladbmanagertasks-v1alpha1-.spec.backup>`
     - object
     - backup specifies the options for a backup task.
   * - :ref:`repair<api-scylla.scylladb.com-scylladbmanagertasks-v1alpha1-.spec.repair>`
     - object
     - repair specifies the options for a repair task.
   * - :ref:`scyllaDBClusterRef<api-scylla.scylladb.com-scylladbmanagertasks-v1alpha1-.spec.scyllaDBClusterRef>`
     - object
     - scyllaDBClusterRef is a typed reference to the target cluster in the same namespace. Supported kind is ScyllaDBDatacenter in scylla.scylladb.com group.
   * - type
     - string
     - type specifies the type of the task.

.. _api-scylla.scylladb.com-scylladbmanagertasks-v1alpha1-.spec.backup:

.spec.backup
^^^^^^^^^^^^

Description
"""""""""""
backup specifies the options for a backup task.

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
     - cron specifies the task schedule as a cron expression. It supports the "standard" cron syntax `MIN HOUR DOM MON DOW`, as used by the Linux utility, as well as a set of non-standard macros: "@yearly", "@annually", "@monthly", "@weekly", "@daily", "@midnight", "@hourly", "@every [+-]?<duration>".
   * - dc
     - array (string)
     - dc specifies a list of datacenter `glob` patterns, e.g. `dc1`, `!otherdc*`, determining the datacenters to include or exclude from backup.
   * - keyspace
     - array (string)
     - keyspace specifies a list of `glob` patterns used to include or exclude tables from backup. The patterns match keyspaces and tables. Keyspace names are separated from table names with a dot e.g. `!keyspace.table_prefix_*`.
   * - location
     - array (string)
     - location specifies a list of backup locations in the following format: `[<dc>:]<provider>:<name>`. `<dc>:` is optional and allows to specify the location for a datacenter in a multi-datacenter cluster. `<provider>` specifies the storage provider. `<name>` specifies a bucket name and must be an alphanumeric string which may contain a dash and or a dot, but other characters are forbidden.
   * - numRetries
     - integer
     - numRetries specifies how many times a scheduled task should be retried before failing.
   * - rateLimit
     - array (string)
     - rateLimit specifies the limit for the upload rate, expressed in mebibytes (MiB) per second, at which the snapshot files can be uploaded from a ScyllaDB node to its backup destination, in the following format: `[<dc>:]<limit>`. `<dc>:` is optional and allows for specifying different upload limits in selected datacenters.
   * - retention
     - integer
     - retention specifies the number of backups to store.
   * - snapshotParallel
     - array (string)
     - snapshotParallel specifies a list of snapshot parallelism limits in the following format:  `[<dc>:]<limit>`. `<dc>:` is optional and allows for specifying different limits in selected datacenters. If `<dc>:` is not set, the limit is global. For instance, `[]string{"dc1:2", "5"}` corresponds to two parallel nodes in `dc1` datacenter and five parallel nodes in the other datacenters.
   * - startDate
     - string
     - startDate specifies the start date of the task. It is represented in RFC3339 form and is in UTC. If not set, the task is started immediately.
   * - uploadParallel
     - array (string)
     - uploadParallel specifies a list of upload parallelism limits in the following format: `[<dc>:]<limit>`. `<dc>:` is optional and allows for specifying different limits in selected datacenters. If `<dc>:` is not set, the limit is global. For instance, `[]string{"dc1:2", "5"}` corresponds to two parallel nodes in `dc1` datacenter and five parallel nodes in the other datacenters.

.. _api-scylla.scylladb.com-scylladbmanagertasks-v1alpha1-.spec.repair:

.spec.repair
^^^^^^^^^^^^

Description
"""""""""""
repair specifies the options for a repair task.

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
     - cron specifies the task schedule as a cron expression. It supports the "standard" cron syntax `MIN HOUR DOM MON DOW`, as used by the Linux utility, as well as a set of non-standard macros: "@yearly", "@annually", "@monthly", "@weekly", "@daily", "@midnight", "@hourly", "@every [+-]?<duration>".
   * - dc
     - array (string)
     - dc specifies a list of datacenter `glob` patterns, e.g. `dc1`, `!otherdc*`, determining the datacenters to include or exclude from repair.
   * - failFast
     - boolean
     - failFast indicates that a repair should be stopped on first encountered error.
   * - host
     - string
     - host specifies the IPv4 or IPv6 address of a node to repair. Specifying this field limits repair to token ranges replicated by a given node. When used in conjunction with `dc`, the node must belong to the specified datacenters. If not set, all hosts are repaired.
   * - ignoreDownHosts
     - boolean
     - ignoreDownHosts indicates that the nodes in down state should be ignored during repair.
   * - intensity
     - integer
     - intensity specifies the number of token ranges to repair in a single ScyllaDB node at the same time. Changing the intensity impacts the repair granularity in case it is resumed. The higher the value, the more work on resumption. When set to zero, the number of token ranges is adjusted to the maximum supported number. When set to a value greater than the maximum supported by the node, intensity is capped at the maximum supported value. Refer to repair documentation for details.
   * - keyspace
     - array (string)
     - keyspace specifies a list of `glob` patterns used to include or exclude tables from repair. The patterns match keyspaces and tables. Keyspace names are separated from table names with a dot e.g. `!keyspace.table_prefix_*`.
   * - numRetries
     - integer
     - numRetries specifies how many times a scheduled task should be retried before failing.
   * - parallel
     - integer
     - parallel specifies the maximum number of ScyllaDB repair jobs that can run at the same time (on different token ranges and replicas). Each node can take part in at most one repair at any given moment. By default, or when set to zero, the maximum possible parallelism is used. The maximal effective parallelism depends on keyspace replication strategy and cluster topology. When set to a value greater than the maximum supported by the node, parallel is capped at the maximum supported value. Refer to repair documentation for details.
   * - smallTableThreshold
     - 
     - smallTableThreshold enables small table optimization for tables of size lower than the given threshold.
   * - startDate
     - string
     - startDate specifies the start date of the task. It is represented in RFC3339 form and is in UTC. If not set, the task is started immediately.

.. _api-scylla.scylladb.com-scylladbmanagertasks-v1alpha1-.spec.scyllaDBClusterRef:

.spec.scyllaDBClusterRef
^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
scyllaDBClusterRef is a typed reference to the target cluster in the same namespace. Supported kind is ScyllaDBDatacenter in scylla.scylladb.com group.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - kind
     - string
     - kind specifies the type of the resource.
   * - name
     - string
     - name specifies the name of the resource in the same namespace.

.. _api-scylla.scylladb.com-scylladbmanagertasks-v1alpha1-.status:

.status
^^^^^^^

Description
"""""""""""
status reflects the observed state of ScyllaDBManagerTask.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`conditions<api-scylla.scylladb.com-scylladbmanagertasks-v1alpha1-.status.conditions[]>`
     - array (object)
     - conditions hold conditions describing ScyllaDBManagerTask state.
   * - observedGeneration
     - integer
     - observedGeneration is the most recent generation observed for this ScyllaDBManagerTask. It corresponds to the ScyllaDBManagerTask's generation, which is updated on mutation by the API Server.
   * - taskID
     - string
     - taskID reflects the internal identification number of the task in ScyllaDB Manager state. It can be used to identify the task when interacting directly with ScyllaDB Manager.

.. _api-scylla.scylladb.com-scylladbmanagertasks-v1alpha1-.status.conditions[]:

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
