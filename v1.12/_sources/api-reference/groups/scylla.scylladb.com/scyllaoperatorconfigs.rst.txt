ScyllaOperatorConfig (scylla.scylladb.com/v1alpha1)
===================================================

| **APIVersion**: scylla.scylladb.com/v1alpha1
| **Kind**: ScyllaOperatorConfig
| **PluralName**: scyllaoperatorconfigs
| **SingularName**: scyllaoperatorconfig
| **Scope**: Cluster
| **ListKind**: ScyllaOperatorConfigList
| **Served**: true
| **Storage**: true

Description
-----------
ScyllaOperatorConfig describes the Scylla Operator configuration.

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
   * - :ref:`metadata<api-scylla.scylladb.com-scyllaoperatorconfigs-v1alpha1-.metadata>`
     - object
     - 
   * - :ref:`spec<api-scylla.scylladb.com-scyllaoperatorconfigs-v1alpha1-.spec>`
     - object
     - spec defines the desired state of the operator.
   * - :ref:`status<api-scylla.scylladb.com-scyllaoperatorconfigs-v1alpha1-.status>`
     - object
     - status defines the observed state of the operator.

.. _api-scylla.scylladb.com-scyllaoperatorconfigs-v1alpha1-.metadata:

.metadata
^^^^^^^^^

Description
"""""""""""


Type
""""
object


.. _api-scylla.scylladb.com-scyllaoperatorconfigs-v1alpha1-.spec:

.spec
^^^^^

Description
"""""""""""
spec defines the desired state of the operator.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - scyllaUtilsImage
     - string
     - scyllaUtilsImage is a Scylla image used for running scylla utilities.

.. _api-scylla.scylladb.com-scyllaoperatorconfigs-v1alpha1-.status:

.status
^^^^^^^

Description
"""""""""""
status defines the observed state of the operator.

Type
""""
object

