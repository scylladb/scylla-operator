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
     - scyllaUtilsImage is a ScyllaDB image used for running ScyllaDB utilities.
   * - unsupportedBashToolsImageOverride
     - string
     - unsupportedBashToolsImageOverride allows to adjust a generic Bash image with extra tools used by the operator for auxiliary purposes. Setting this field renders your cluster unsupported. Use at your own risk.
   * - unsupportedGrafanaImageOverride
     - string
     - unsupportedGrafanaImageOverride allows to adjust Grafana image used by the operator for testing, dev or emergencies. Setting this field renders your cluster unsupported. Use at your own risk.
   * - unsupportedPrometheusVersionOverride
     - string
     - unsupportedPrometheusVersionOverride allows to adjust Prometheus version used by the operator for testing, dev or emergencies. Setting this field renders your cluster unsupported. Use at your own risk.

.. _api-scylla.scylladb.com-scyllaoperatorconfigs-v1alpha1-.status:

.status
^^^^^^^

Description
"""""""""""
status defines the observed state of the operator.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - bashToolsImage
     - string
     - bashToolsImage is a generic Bash image with extra tools used by the operator for auxiliary purposes.
   * - grafanaImage
     - string
     - grafanaImage is the image used by the operator to create a Grafana instance.
   * - observedGeneration
     - integer
     - observedGeneration is the most recent generation observed for this ScyllaOperatorConfig. It corresponds to the ScyllaOperatorConfig's generation, which is updated on mutation by the API Server.
   * - prometheusVersion
     - string
     - prometheusVersion is the Prometheus version used by the operator to create a Prometheus instance.
   * - scyllaDBUtilsImage
     - string
     - scyllaDBUtilsImage is the ScyllaDB image used for running ScyllaDB utilities.
