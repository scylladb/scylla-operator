ScyllaDBDatacenterNodesStatusReport (scylla.scylladb.com/v1alpha1)
==================================================================

| **APIVersion**: scylla.scylladb.com/v1alpha1
| **Kind**: ScyllaDBDatacenterNodesStatusReport
| **PluralName**: scylladbdatacenternodesstatusreports
| **SingularName**: scylladbdatacenternodesstatusreport
| **Scope**: Namespaced
| **ListKind**: ScyllaDBDatacenterNodesStatusReportList
| **Served**: true
| **Storage**: true

Description
-----------
ScyllaDBDatacenterNodesStatusReport serves as an internal interface for propagating and aggregating node statuses observed by nodes in a ScyllaDB datacenter.
It is used by ScyllaDB Operator for coordinating operations such as topology changes and is not intended for direct user interaction.

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
   * - datacenterName
     - string
     - DatacenterName is the name of the reported ScyllaDB datacenter.
   * - kind
     - string
     - Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds
   * - :ref:`metadata<api-scylla.scylladb.com-scylladbdatacenternodesstatusreports-v1alpha1-.metadata>`
     - object
     - 
   * - :ref:`racks<api-scylla.scylladb.com-scylladbdatacenternodesstatusreports-v1alpha1-.racks[]>`
     - array (object)
     - Racks holds the list of rack status reports.

.. _api-scylla.scylladb.com-scylladbdatacenternodesstatusreports-v1alpha1-.metadata:

.metadata
^^^^^^^^^

Description
"""""""""""


Type
""""
object


.. _api-scylla.scylladb.com-scylladbdatacenternodesstatusreports-v1alpha1-.racks[]:

.racks[]
^^^^^^^^

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
   * - name
     - string
     - Name is the name of the rack.
   * - :ref:`nodes<api-scylla.scylladb.com-scylladbdatacenternodesstatusreports-v1alpha1-.racks[].nodes[]>`
     - array (object)
     - Nodes holds the list of node status reports collected from nodes from this rack.

.. _api-scylla.scylladb.com-scylladbdatacenternodesstatusreports-v1alpha1-.racks[].nodes[]:

.racks[].nodes[]
^^^^^^^^^^^^^^^^

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
   * - hostID
     - string
     - HostID is the ScyllaDB reporter node's host ID.
   * - :ref:`observedNodes<api-scylla.scylladb.com-scylladbdatacenternodesstatusreports-v1alpha1-.racks[].nodes[].observedNodes[]>`
     - array (object)
     - ObservedNodes holds the list of node statuses as observed by the reporter node.
   * - ordinal
     - integer
     - Ordinal is the ordinal of the reporter node within its rack.

.. _api-scylla.scylladb.com-scylladbdatacenternodesstatusreports-v1alpha1-.racks[].nodes[].observedNodes[]:

.racks[].nodes[].observedNodes[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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
   * - hostID
     - string
     - HostID is the ScyllaDB node's host ID.
   * - status
     - string
     - Status is the status of the node.
