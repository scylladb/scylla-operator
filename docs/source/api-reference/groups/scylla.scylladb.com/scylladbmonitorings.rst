ScyllaDBMonitoring (scylla.scylladb.com/v1alpha1)
=================================================

| **APIVersion**: scylla.scylladb.com/v1alpha1
| **Kind**: ScyllaDBMonitoring
| **PluralName**: scylladbmonitorings
| **SingularName**: scylladbmonitoring
| **Scope**: Namespaced
| **ListKind**: ScyllaDBMonitoringList
| **Served**: true
| **Storage**: true

Description
-----------
ScyllaDBMonitoring defines a monitoring instance for ScyllaDB clusters.

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
   * - :ref:`metadata<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.metadata>`
     - object
     - 
   * - :ref:`spec<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec>`
     - object
     - spec defines the desired state of this ScyllaDBMonitoring.
   * - :ref:`status<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.status>`
     - object
     - status is the current status of this ScyllaDBMonitoring.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.metadata:

.metadata
^^^^^^^^^

Description
"""""""""""


Type
""""
object


.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec:

.spec
^^^^^

Description
"""""""""""
spec defines the desired state of this ScyllaDBMonitoring.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`components<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components>`
     - object
     - components hold additional config for the monitoring components in use.
   * - :ref:`endpointsSelector<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.endpointsSelector>`
     - object
     - endpointsSelector select which Endpoints should be scraped. For local ScyllaDB clusters or datacenters, this is the same selector as if you were trying to select member Services. For remote ScyllaDB clusters, this can select any endpoints that are created manually or for a Service without selectors.
   * - type
     - string
     - type determines the platform type of the monitoring setup.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components:

.spec.components
^^^^^^^^^^^^^^^^

Description
"""""""""""
components hold additional config for the monitoring components in use.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`grafana<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana>`
     - object
     - grafana holds configuration for the grafana instance, if any.
   * - :ref:`prometheus<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus>`
     - object
     - prometheus holds configuration for the prometheus instance, if any.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana:

.spec.components.grafana
^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
grafana holds configuration for the grafana instance, if any.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`authentication<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.authentication>`
     - object
     - authentication hold the authentication options for accessing Grafana.
   * - :ref:`exposeOptions<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.exposeOptions>`
     - object
     - exposeOptions specifies options for exposing Grafana UI.
   * - :ref:`placement<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement>`
     - object
     - placement describes restrictions for the nodes Grafana is scheduled on.
   * - :ref:`resources<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.resources>`
     - object
     - resources the Grafana container will use.
   * - servingCertSecretName
     - string
     - servingCertSecretName is the name of the secret holding a serving cert-key pair. If not specified, the operator will create a self-signed CA that creates the default serving cert-key pair.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.authentication:

.spec.components.grafana.authentication
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
authentication hold the authentication options for accessing Grafana.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - insecureEnableAnonymousAccess
     - boolean
     - insecureEnableAnonymousAccess allows access to Grafana without authentication.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.exposeOptions:

.spec.components.grafana.exposeOptions
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
exposeOptions specifies options for exposing Grafana UI.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`webInterface<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.exposeOptions.webInterface>`
     - object
     - webInterface specifies expose options for the user web interface.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.exposeOptions.webInterface:

.spec.components.grafana.exposeOptions.webInterface
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
webInterface specifies expose options for the user web interface.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`ingress<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.exposeOptions.webInterface.ingress>`
     - object
     - ingress is an Ingress configuration options.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.exposeOptions.webInterface.ingress:

.spec.components.grafana.exposeOptions.webInterface.ingress
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
ingress is an Ingress configuration options.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`annotations<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.exposeOptions.webInterface.ingress.annotations>`
     - object
     - annotations specifies custom annotations merged into every Ingress object.
   * - disabled
     - boolean
     - disabled controls if Ingress object creation is disabled.
   * - dnsDomains
     - array (string)
     - dnsDomains is a list of DNS domains this ingress is reachable by.
   * - ingressClassName
     - string
     - ingressClassName specifies Ingress class name.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.exposeOptions.webInterface.ingress.annotations:

.spec.components.grafana.exposeOptions.webInterface.ingress.annotations
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
annotations specifies custom annotations merged into every Ingress object.

Type
""""
object


.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement:

.spec.components.grafana.placement
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
placement describes restrictions for the nodes Grafana is scheduled on.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`nodeAffinity<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.nodeAffinity>`
     - object
     - nodeAffinity describes node affinity scheduling rules for the pod.
   * - :ref:`podAffinity<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAffinity>`
     - object
     - podAffinity describes pod affinity scheduling rules.
   * - :ref:`podAntiAffinity<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAntiAffinity>`
     - object
     - podAntiAffinity describes pod anti-affinity scheduling rules.
   * - :ref:`tolerations<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.tolerations[]>`
     - array (object)
     - tolerations allow the pod to tolerate any taint that matches the triple <key,value,effect> using the matching operator.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.nodeAffinity:

.spec.components.grafana.placement.nodeAffinity
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
   * - :ref:`preferredDuringSchedulingIgnoredDuringExecution<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[]>`
     - array (object)
     - The scheduler will prefer to schedule pods to nodes that satisfy the affinity expressions specified by this field, but it may choose a node that violates one or more of the expressions. The node that is most preferred is the one with the greatest sum of weights, i.e. for each node that meets all of the scheduling requirements (resource request, requiredDuringScheduling affinity expressions, etc.), compute a sum by iterating through the elements of this field and adding "weight" to the sum if the node matches the corresponding matchExpressions; the node(s) with the highest sum are the most preferred.
   * - :ref:`requiredDuringSchedulingIgnoredDuringExecution<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution>`
     - object
     - If the affinity requirements specified by this field are not met at scheduling time, the pod will not be scheduled onto the node. If the affinity requirements specified by this field cease to be met at some point during pod execution (e.g. due to an update), the system may or may not try to eventually evict the pod from its node.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[]:

.spec.components.grafana.placement.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[]
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
   * - :ref:`preference<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[].preference>`
     - object
     - A node selector term, associated with the corresponding weight.
   * - weight
     - integer
     - Weight associated with matching the corresponding nodeSelectorTerm, in the range 1-100.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[].preference:

.spec.components.grafana.placement.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[].preference
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
   * - :ref:`matchExpressions<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[].preference.matchExpressions[]>`
     - array (object)
     - A list of node selector requirements by node's labels.
   * - :ref:`matchFields<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[].preference.matchFields[]>`
     - array (object)
     - A list of node selector requirements by node's fields.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[].preference.matchExpressions[]:

.spec.components.grafana.placement.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[].preference.matchExpressions[]
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

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[].preference.matchFields[]:

.spec.components.grafana.placement.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[].preference.matchFields[]
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

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution:

.spec.components.grafana.placement.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution
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
   * - :ref:`nodeSelectorTerms<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[]>`
     - array (object)
     - Required. A list of node selector terms. The terms are ORed.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[]:

.spec.components.grafana.placement.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[]
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
   * - :ref:`matchExpressions<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[].matchExpressions[]>`
     - array (object)
     - A list of node selector requirements by node's labels.
   * - :ref:`matchFields<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[].matchFields[]>`
     - array (object)
     - A list of node selector requirements by node's fields.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[].matchExpressions[]:

.spec.components.grafana.placement.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[].matchExpressions[]
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

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[].matchFields[]:

.spec.components.grafana.placement.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[].matchFields[]
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

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAffinity:

.spec.components.grafana.placement.podAffinity
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
   * - :ref:`preferredDuringSchedulingIgnoredDuringExecution<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[]>`
     - array (object)
     - The scheduler will prefer to schedule pods to nodes that satisfy the affinity expressions specified by this field, but it may choose a node that violates one or more of the expressions. The node that is most preferred is the one with the greatest sum of weights, i.e. for each node that meets all of the scheduling requirements (resource request, requiredDuringScheduling affinity expressions, etc.), compute a sum by iterating through the elements of this field and adding "weight" to the sum if the node has pods which matches the corresponding podAffinityTerm; the node(s) with the highest sum are the most preferred.
   * - :ref:`requiredDuringSchedulingIgnoredDuringExecution<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[]>`
     - array (object)
     - If the affinity requirements specified by this field are not met at scheduling time, the pod will not be scheduled onto the node. If the affinity requirements specified by this field cease to be met at some point during pod execution (e.g. due to a pod label update), the system may or may not try to eventually evict the pod from its node. When there are multiple elements, the lists of nodes corresponding to each podAffinityTerm are intersected, i.e. all terms must be satisfied.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[]:

.spec.components.grafana.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[]
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
   * - :ref:`podAffinityTerm<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm>`
     - object
     - Required. A pod affinity term, associated with the corresponding weight.
   * - weight
     - integer
     - weight associated with matching the corresponding podAffinityTerm, in the range 1-100.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm:

.spec.components.grafana.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm
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
   * - :ref:`labelSelector<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector>`
     - object
     - A label query over a set of resources, in this case pods. If it's null, this PodAffinityTerm matches with no Pods.
   * - matchLabelKeys
     - array (string)
     - MatchLabelKeys is a set of pod label keys to select which pods will be taken into consideration. The keys are used to lookup values from the incoming pod labels, those key-value labels are merged with `LabelSelector` as `key in (value)` to select the group of existing pods which pods will be taken into consideration for the incoming pod's pod (anti) affinity. Keys that don't exist in the incoming pod labels will be ignored. The default value is empty. The same key is forbidden to exist in both MatchLabelKeys and LabelSelector. Also, MatchLabelKeys cannot be set when LabelSelector isn't set. This is an alpha field and requires enabling MatchLabelKeysInPodAffinity feature gate.
   * - mismatchLabelKeys
     - array (string)
     - MismatchLabelKeys is a set of pod label keys to select which pods will be taken into consideration. The keys are used to lookup values from the incoming pod labels, those key-value labels are merged with `LabelSelector` as `key notin (value)` to select the group of existing pods which pods will be taken into consideration for the incoming pod's pod (anti) affinity. Keys that don't exist in the incoming pod labels will be ignored. The default value is empty. The same key is forbidden to exist in both MismatchLabelKeys and LabelSelector. Also, MismatchLabelKeys cannot be set when LabelSelector isn't set. This is an alpha field and requires enabling MatchLabelKeysInPodAffinity feature gate.
   * - :ref:`namespaceSelector<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector>`
     - object
     - A label query over the set of namespaces that the term applies to. The term is applied to the union of the namespaces selected by this field and the ones listed in the namespaces field. null selector and null or empty namespaces list means "this pod's namespace". An empty selector ({}) matches all namespaces.
   * - namespaces
     - array (string)
     - namespaces specifies a static list of namespace names that the term applies to. The term is applied to the union of the namespaces listed in this field and the ones selected by namespaceSelector. null or empty namespaces list and null namespaceSelector means "this pod's namespace".
   * - topologyKey
     - string
     - This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching the labelSelector in the specified namespaces, where co-located is defined as running on a node whose value of the label with key topologyKey matches that of any node on which any of the selected pods is running. Empty topologyKey is not allowed.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector:

.spec.components.grafana.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector
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
   * - :ref:`matchExpressions<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchExpressions[]>`
     - array (object)
     - matchExpressions is a list of label selector requirements. The requirements are ANDed.
   * - :ref:`matchLabels<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchLabels>`
     - object
     - matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchExpressions[]:

.spec.components.grafana.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchExpressions[]
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

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchLabels:

.spec.components.grafana.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchLabels
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

Type
""""
object


.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector:

.spec.components.grafana.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector
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
   * - :ref:`matchExpressions<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchExpressions[]>`
     - array (object)
     - matchExpressions is a list of label selector requirements. The requirements are ANDed.
   * - :ref:`matchLabels<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchLabels>`
     - object
     - matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchExpressions[]:

.spec.components.grafana.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchExpressions[]
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

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchLabels:

.spec.components.grafana.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchLabels
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

Type
""""
object


.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[]:

.spec.components.grafana.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[]
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
   * - :ref:`labelSelector<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector>`
     - object
     - A label query over a set of resources, in this case pods. If it's null, this PodAffinityTerm matches with no Pods.
   * - matchLabelKeys
     - array (string)
     - MatchLabelKeys is a set of pod label keys to select which pods will be taken into consideration. The keys are used to lookup values from the incoming pod labels, those key-value labels are merged with `LabelSelector` as `key in (value)` to select the group of existing pods which pods will be taken into consideration for the incoming pod's pod (anti) affinity. Keys that don't exist in the incoming pod labels will be ignored. The default value is empty. The same key is forbidden to exist in both MatchLabelKeys and LabelSelector. Also, MatchLabelKeys cannot be set when LabelSelector isn't set. This is an alpha field and requires enabling MatchLabelKeysInPodAffinity feature gate.
   * - mismatchLabelKeys
     - array (string)
     - MismatchLabelKeys is a set of pod label keys to select which pods will be taken into consideration. The keys are used to lookup values from the incoming pod labels, those key-value labels are merged with `LabelSelector` as `key notin (value)` to select the group of existing pods which pods will be taken into consideration for the incoming pod's pod (anti) affinity. Keys that don't exist in the incoming pod labels will be ignored. The default value is empty. The same key is forbidden to exist in both MismatchLabelKeys and LabelSelector. Also, MismatchLabelKeys cannot be set when LabelSelector isn't set. This is an alpha field and requires enabling MatchLabelKeysInPodAffinity feature gate.
   * - :ref:`namespaceSelector<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector>`
     - object
     - A label query over the set of namespaces that the term applies to. The term is applied to the union of the namespaces selected by this field and the ones listed in the namespaces field. null selector and null or empty namespaces list means "this pod's namespace". An empty selector ({}) matches all namespaces.
   * - namespaces
     - array (string)
     - namespaces specifies a static list of namespace names that the term applies to. The term is applied to the union of the namespaces listed in this field and the ones selected by namespaceSelector. null or empty namespaces list and null namespaceSelector means "this pod's namespace".
   * - topologyKey
     - string
     - This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching the labelSelector in the specified namespaces, where co-located is defined as running on a node whose value of the label with key topologyKey matches that of any node on which any of the selected pods is running. Empty topologyKey is not allowed.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector:

.spec.components.grafana.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector
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
   * - :ref:`matchExpressions<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchExpressions[]>`
     - array (object)
     - matchExpressions is a list of label selector requirements. The requirements are ANDed.
   * - :ref:`matchLabels<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchLabels>`
     - object
     - matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchExpressions[]:

.spec.components.grafana.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchExpressions[]
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

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchLabels:

.spec.components.grafana.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchLabels
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

Type
""""
object


.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector:

.spec.components.grafana.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector
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
   * - :ref:`matchExpressions<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchExpressions[]>`
     - array (object)
     - matchExpressions is a list of label selector requirements. The requirements are ANDed.
   * - :ref:`matchLabels<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchLabels>`
     - object
     - matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchExpressions[]:

.spec.components.grafana.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchExpressions[]
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

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchLabels:

.spec.components.grafana.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchLabels
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

Type
""""
object


.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAntiAffinity:

.spec.components.grafana.placement.podAntiAffinity
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
   * - :ref:`preferredDuringSchedulingIgnoredDuringExecution<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[]>`
     - array (object)
     - The scheduler will prefer to schedule pods to nodes that satisfy the anti-affinity expressions specified by this field, but it may choose a node that violates one or more of the expressions. The node that is most preferred is the one with the greatest sum of weights, i.e. for each node that meets all of the scheduling requirements (resource request, requiredDuringScheduling anti-affinity expressions, etc.), compute a sum by iterating through the elements of this field and adding "weight" to the sum if the node has pods which matches the corresponding podAffinityTerm; the node(s) with the highest sum are the most preferred.
   * - :ref:`requiredDuringSchedulingIgnoredDuringExecution<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[]>`
     - array (object)
     - If the anti-affinity requirements specified by this field are not met at scheduling time, the pod will not be scheduled onto the node. If the anti-affinity requirements specified by this field cease to be met at some point during pod execution (e.g. due to a pod label update), the system may or may not try to eventually evict the pod from its node. When there are multiple elements, the lists of nodes corresponding to each podAffinityTerm are intersected, i.e. all terms must be satisfied.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[]:

.spec.components.grafana.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[]
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
   * - :ref:`podAffinityTerm<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm>`
     - object
     - Required. A pod affinity term, associated with the corresponding weight.
   * - weight
     - integer
     - weight associated with matching the corresponding podAffinityTerm, in the range 1-100.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm:

.spec.components.grafana.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm
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
   * - :ref:`labelSelector<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector>`
     - object
     - A label query over a set of resources, in this case pods. If it's null, this PodAffinityTerm matches with no Pods.
   * - matchLabelKeys
     - array (string)
     - MatchLabelKeys is a set of pod label keys to select which pods will be taken into consideration. The keys are used to lookup values from the incoming pod labels, those key-value labels are merged with `LabelSelector` as `key in (value)` to select the group of existing pods which pods will be taken into consideration for the incoming pod's pod (anti) affinity. Keys that don't exist in the incoming pod labels will be ignored. The default value is empty. The same key is forbidden to exist in both MatchLabelKeys and LabelSelector. Also, MatchLabelKeys cannot be set when LabelSelector isn't set. This is an alpha field and requires enabling MatchLabelKeysInPodAffinity feature gate.
   * - mismatchLabelKeys
     - array (string)
     - MismatchLabelKeys is a set of pod label keys to select which pods will be taken into consideration. The keys are used to lookup values from the incoming pod labels, those key-value labels are merged with `LabelSelector` as `key notin (value)` to select the group of existing pods which pods will be taken into consideration for the incoming pod's pod (anti) affinity. Keys that don't exist in the incoming pod labels will be ignored. The default value is empty. The same key is forbidden to exist in both MismatchLabelKeys and LabelSelector. Also, MismatchLabelKeys cannot be set when LabelSelector isn't set. This is an alpha field and requires enabling MatchLabelKeysInPodAffinity feature gate.
   * - :ref:`namespaceSelector<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector>`
     - object
     - A label query over the set of namespaces that the term applies to. The term is applied to the union of the namespaces selected by this field and the ones listed in the namespaces field. null selector and null or empty namespaces list means "this pod's namespace". An empty selector ({}) matches all namespaces.
   * - namespaces
     - array (string)
     - namespaces specifies a static list of namespace names that the term applies to. The term is applied to the union of the namespaces listed in this field and the ones selected by namespaceSelector. null or empty namespaces list and null namespaceSelector means "this pod's namespace".
   * - topologyKey
     - string
     - This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching the labelSelector in the specified namespaces, where co-located is defined as running on a node whose value of the label with key topologyKey matches that of any node on which any of the selected pods is running. Empty topologyKey is not allowed.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector:

.spec.components.grafana.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector
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
   * - :ref:`matchExpressions<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchExpressions[]>`
     - array (object)
     - matchExpressions is a list of label selector requirements. The requirements are ANDed.
   * - :ref:`matchLabels<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchLabels>`
     - object
     - matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchExpressions[]:

.spec.components.grafana.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchExpressions[]
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

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchLabels:

.spec.components.grafana.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchLabels
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

Type
""""
object


.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector:

.spec.components.grafana.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector
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
   * - :ref:`matchExpressions<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchExpressions[]>`
     - array (object)
     - matchExpressions is a list of label selector requirements. The requirements are ANDed.
   * - :ref:`matchLabels<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchLabels>`
     - object
     - matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchExpressions[]:

.spec.components.grafana.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchExpressions[]
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

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchLabels:

.spec.components.grafana.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchLabels
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

Type
""""
object


.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[]:

.spec.components.grafana.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[]
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
   * - :ref:`labelSelector<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector>`
     - object
     - A label query over a set of resources, in this case pods. If it's null, this PodAffinityTerm matches with no Pods.
   * - matchLabelKeys
     - array (string)
     - MatchLabelKeys is a set of pod label keys to select which pods will be taken into consideration. The keys are used to lookup values from the incoming pod labels, those key-value labels are merged with `LabelSelector` as `key in (value)` to select the group of existing pods which pods will be taken into consideration for the incoming pod's pod (anti) affinity. Keys that don't exist in the incoming pod labels will be ignored. The default value is empty. The same key is forbidden to exist in both MatchLabelKeys and LabelSelector. Also, MatchLabelKeys cannot be set when LabelSelector isn't set. This is an alpha field and requires enabling MatchLabelKeysInPodAffinity feature gate.
   * - mismatchLabelKeys
     - array (string)
     - MismatchLabelKeys is a set of pod label keys to select which pods will be taken into consideration. The keys are used to lookup values from the incoming pod labels, those key-value labels are merged with `LabelSelector` as `key notin (value)` to select the group of existing pods which pods will be taken into consideration for the incoming pod's pod (anti) affinity. Keys that don't exist in the incoming pod labels will be ignored. The default value is empty. The same key is forbidden to exist in both MismatchLabelKeys and LabelSelector. Also, MismatchLabelKeys cannot be set when LabelSelector isn't set. This is an alpha field and requires enabling MatchLabelKeysInPodAffinity feature gate.
   * - :ref:`namespaceSelector<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector>`
     - object
     - A label query over the set of namespaces that the term applies to. The term is applied to the union of the namespaces selected by this field and the ones listed in the namespaces field. null selector and null or empty namespaces list means "this pod's namespace". An empty selector ({}) matches all namespaces.
   * - namespaces
     - array (string)
     - namespaces specifies a static list of namespace names that the term applies to. The term is applied to the union of the namespaces listed in this field and the ones selected by namespaceSelector. null or empty namespaces list and null namespaceSelector means "this pod's namespace".
   * - topologyKey
     - string
     - This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching the labelSelector in the specified namespaces, where co-located is defined as running on a node whose value of the label with key topologyKey matches that of any node on which any of the selected pods is running. Empty topologyKey is not allowed.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector:

.spec.components.grafana.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector
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
   * - :ref:`matchExpressions<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchExpressions[]>`
     - array (object)
     - matchExpressions is a list of label selector requirements. The requirements are ANDed.
   * - :ref:`matchLabels<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchLabels>`
     - object
     - matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchExpressions[]:

.spec.components.grafana.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchExpressions[]
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

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchLabels:

.spec.components.grafana.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchLabels
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

Type
""""
object


.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector:

.spec.components.grafana.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector
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
   * - :ref:`matchExpressions<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchExpressions[]>`
     - array (object)
     - matchExpressions is a list of label selector requirements. The requirements are ANDed.
   * - :ref:`matchLabels<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchLabels>`
     - object
     - matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchExpressions[]:

.spec.components.grafana.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchExpressions[]
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

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchLabels:

.spec.components.grafana.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchLabels
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

Type
""""
object


.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.placement.tolerations[]:

.spec.components.grafana.placement.tolerations[]
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

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.resources:

.spec.components.grafana.resources
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
resources the Grafana container will use.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`claims<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.resources.claims[]>`
     - array (object)
     - Claims lists the names of resources, defined in spec.resourceClaims, that are used by this container. 
        This is an alpha field and requires enabling the DynamicResourceAllocation feature gate. 
        This field is immutable. It can only be set for containers.
   * - :ref:`limits<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.resources.limits>`
     - object
     - Limits describes the maximum amount of compute resources allowed. More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
   * - :ref:`requests<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.resources.requests>`
     - object
     - Requests describes the minimum amount of compute resources required. If Requests is omitted for a container, it defaults to Limits if that is explicitly specified, otherwise to an implementation-defined value. Requests cannot exceed Limits. More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.resources.claims[]:

.spec.components.grafana.resources.claims[]
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

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.resources.limits:

.spec.components.grafana.resources.limits
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
Limits describes the maximum amount of compute resources allowed. More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/

Type
""""
object


.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.grafana.resources.requests:

.spec.components.grafana.resources.requests
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
Requests describes the minimum amount of compute resources required. If Requests is omitted for a container, it defaults to Limits if that is explicitly specified, otherwise to an implementation-defined value. Requests cannot exceed Limits. More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/

Type
""""
object


.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus:

.spec.components.prometheus
^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
prometheus holds configuration for the prometheus instance, if any.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`exposeOptions<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.exposeOptions>`
     - object
     - exposeOptions specifies options for exposing Prometheus UI.
   * - :ref:`placement<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement>`
     - object
     - placement describes restrictions for the nodes Prometheus is scheduled on.
   * - :ref:`resources<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.resources>`
     - object
     - resources the Prometheus container will use.
   * - :ref:`storage<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.storage>`
     - object
     - storage describes the underlying storage that Prometheus will consume.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.exposeOptions:

.spec.components.prometheus.exposeOptions
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
exposeOptions specifies options for exposing Prometheus UI.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`webInterface<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.exposeOptions.webInterface>`
     - object
     - webInterface specifies expose options for the user web interface.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.exposeOptions.webInterface:

.spec.components.prometheus.exposeOptions.webInterface
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
webInterface specifies expose options for the user web interface.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`ingress<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.exposeOptions.webInterface.ingress>`
     - object
     - ingress is an Ingress configuration options.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.exposeOptions.webInterface.ingress:

.spec.components.prometheus.exposeOptions.webInterface.ingress
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
ingress is an Ingress configuration options.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`annotations<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.exposeOptions.webInterface.ingress.annotations>`
     - object
     - annotations specifies custom annotations merged into every Ingress object.
   * - disabled
     - boolean
     - disabled controls if Ingress object creation is disabled.
   * - dnsDomains
     - array (string)
     - dnsDomains is a list of DNS domains this ingress is reachable by.
   * - ingressClassName
     - string
     - ingressClassName specifies Ingress class name.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.exposeOptions.webInterface.ingress.annotations:

.spec.components.prometheus.exposeOptions.webInterface.ingress.annotations
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
annotations specifies custom annotations merged into every Ingress object.

Type
""""
object


.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement:

.spec.components.prometheus.placement
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
placement describes restrictions for the nodes Prometheus is scheduled on.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`nodeAffinity<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.nodeAffinity>`
     - object
     - nodeAffinity describes node affinity scheduling rules for the pod.
   * - :ref:`podAffinity<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAffinity>`
     - object
     - podAffinity describes pod affinity scheduling rules.
   * - :ref:`podAntiAffinity<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAntiAffinity>`
     - object
     - podAntiAffinity describes pod anti-affinity scheduling rules.
   * - :ref:`tolerations<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.tolerations[]>`
     - array (object)
     - tolerations allow the pod to tolerate any taint that matches the triple <key,value,effect> using the matching operator.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.nodeAffinity:

.spec.components.prometheus.placement.nodeAffinity
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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
   * - :ref:`preferredDuringSchedulingIgnoredDuringExecution<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[]>`
     - array (object)
     - The scheduler will prefer to schedule pods to nodes that satisfy the affinity expressions specified by this field, but it may choose a node that violates one or more of the expressions. The node that is most preferred is the one with the greatest sum of weights, i.e. for each node that meets all of the scheduling requirements (resource request, requiredDuringScheduling affinity expressions, etc.), compute a sum by iterating through the elements of this field and adding "weight" to the sum if the node matches the corresponding matchExpressions; the node(s) with the highest sum are the most preferred.
   * - :ref:`requiredDuringSchedulingIgnoredDuringExecution<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution>`
     - object
     - If the affinity requirements specified by this field are not met at scheduling time, the pod will not be scheduled onto the node. If the affinity requirements specified by this field cease to be met at some point during pod execution (e.g. due to an update), the system may or may not try to eventually evict the pod from its node.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[]:

.spec.components.prometheus.placement.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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
   * - :ref:`preference<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[].preference>`
     - object
     - A node selector term, associated with the corresponding weight.
   * - weight
     - integer
     - Weight associated with matching the corresponding nodeSelectorTerm, in the range 1-100.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[].preference:

.spec.components.prometheus.placement.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[].preference
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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
   * - :ref:`matchExpressions<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[].preference.matchExpressions[]>`
     - array (object)
     - A list of node selector requirements by node's labels.
   * - :ref:`matchFields<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[].preference.matchFields[]>`
     - array (object)
     - A list of node selector requirements by node's fields.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[].preference.matchExpressions[]:

.spec.components.prometheus.placement.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[].preference.matchExpressions[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[].preference.matchFields[]:

.spec.components.prometheus.placement.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[].preference.matchFields[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution:

.spec.components.prometheus.placement.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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
   * - :ref:`nodeSelectorTerms<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[]>`
     - array (object)
     - Required. A list of node selector terms. The terms are ORed.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[]:

.spec.components.prometheus.placement.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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
   * - :ref:`matchExpressions<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[].matchExpressions[]>`
     - array (object)
     - A list of node selector requirements by node's labels.
   * - :ref:`matchFields<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[].matchFields[]>`
     - array (object)
     - A list of node selector requirements by node's fields.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[].matchExpressions[]:

.spec.components.prometheus.placement.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[].matchExpressions[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[].matchFields[]:

.spec.components.prometheus.placement.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[].matchFields[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAffinity:

.spec.components.prometheus.placement.podAffinity
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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
   * - :ref:`preferredDuringSchedulingIgnoredDuringExecution<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[]>`
     - array (object)
     - The scheduler will prefer to schedule pods to nodes that satisfy the affinity expressions specified by this field, but it may choose a node that violates one or more of the expressions. The node that is most preferred is the one with the greatest sum of weights, i.e. for each node that meets all of the scheduling requirements (resource request, requiredDuringScheduling affinity expressions, etc.), compute a sum by iterating through the elements of this field and adding "weight" to the sum if the node has pods which matches the corresponding podAffinityTerm; the node(s) with the highest sum are the most preferred.
   * - :ref:`requiredDuringSchedulingIgnoredDuringExecution<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[]>`
     - array (object)
     - If the affinity requirements specified by this field are not met at scheduling time, the pod will not be scheduled onto the node. If the affinity requirements specified by this field cease to be met at some point during pod execution (e.g. due to a pod label update), the system may or may not try to eventually evict the pod from its node. When there are multiple elements, the lists of nodes corresponding to each podAffinityTerm are intersected, i.e. all terms must be satisfied.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[]:

.spec.components.prometheus.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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
   * - :ref:`podAffinityTerm<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm>`
     - object
     - Required. A pod affinity term, associated with the corresponding weight.
   * - weight
     - integer
     - weight associated with matching the corresponding podAffinityTerm, in the range 1-100.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm:

.spec.components.prometheus.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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
   * - :ref:`labelSelector<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector>`
     - object
     - A label query over a set of resources, in this case pods. If it's null, this PodAffinityTerm matches with no Pods.
   * - matchLabelKeys
     - array (string)
     - MatchLabelKeys is a set of pod label keys to select which pods will be taken into consideration. The keys are used to lookup values from the incoming pod labels, those key-value labels are merged with `LabelSelector` as `key in (value)` to select the group of existing pods which pods will be taken into consideration for the incoming pod's pod (anti) affinity. Keys that don't exist in the incoming pod labels will be ignored. The default value is empty. The same key is forbidden to exist in both MatchLabelKeys and LabelSelector. Also, MatchLabelKeys cannot be set when LabelSelector isn't set. This is an alpha field and requires enabling MatchLabelKeysInPodAffinity feature gate.
   * - mismatchLabelKeys
     - array (string)
     - MismatchLabelKeys is a set of pod label keys to select which pods will be taken into consideration. The keys are used to lookup values from the incoming pod labels, those key-value labels are merged with `LabelSelector` as `key notin (value)` to select the group of existing pods which pods will be taken into consideration for the incoming pod's pod (anti) affinity. Keys that don't exist in the incoming pod labels will be ignored. The default value is empty. The same key is forbidden to exist in both MismatchLabelKeys and LabelSelector. Also, MismatchLabelKeys cannot be set when LabelSelector isn't set. This is an alpha field and requires enabling MatchLabelKeysInPodAffinity feature gate.
   * - :ref:`namespaceSelector<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector>`
     - object
     - A label query over the set of namespaces that the term applies to. The term is applied to the union of the namespaces selected by this field and the ones listed in the namespaces field. null selector and null or empty namespaces list means "this pod's namespace". An empty selector ({}) matches all namespaces.
   * - namespaces
     - array (string)
     - namespaces specifies a static list of namespace names that the term applies to. The term is applied to the union of the namespaces listed in this field and the ones selected by namespaceSelector. null or empty namespaces list and null namespaceSelector means "this pod's namespace".
   * - topologyKey
     - string
     - This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching the labelSelector in the specified namespaces, where co-located is defined as running on a node whose value of the label with key topologyKey matches that of any node on which any of the selected pods is running. Empty topologyKey is not allowed.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector:

.spec.components.prometheus.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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
   * - :ref:`matchExpressions<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchExpressions[]>`
     - array (object)
     - matchExpressions is a list of label selector requirements. The requirements are ANDed.
   * - :ref:`matchLabels<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchLabels>`
     - object
     - matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchExpressions[]:

.spec.components.prometheus.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchExpressions[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchLabels:

.spec.components.prometheus.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchLabels
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

Type
""""
object


.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector:

.spec.components.prometheus.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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
   * - :ref:`matchExpressions<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchExpressions[]>`
     - array (object)
     - matchExpressions is a list of label selector requirements. The requirements are ANDed.
   * - :ref:`matchLabels<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchLabels>`
     - object
     - matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchExpressions[]:

.spec.components.prometheus.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchExpressions[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchLabels:

.spec.components.prometheus.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchLabels
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

Type
""""
object


.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[]:

.spec.components.prometheus.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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
   * - :ref:`labelSelector<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector>`
     - object
     - A label query over a set of resources, in this case pods. If it's null, this PodAffinityTerm matches with no Pods.
   * - matchLabelKeys
     - array (string)
     - MatchLabelKeys is a set of pod label keys to select which pods will be taken into consideration. The keys are used to lookup values from the incoming pod labels, those key-value labels are merged with `LabelSelector` as `key in (value)` to select the group of existing pods which pods will be taken into consideration for the incoming pod's pod (anti) affinity. Keys that don't exist in the incoming pod labels will be ignored. The default value is empty. The same key is forbidden to exist in both MatchLabelKeys and LabelSelector. Also, MatchLabelKeys cannot be set when LabelSelector isn't set. This is an alpha field and requires enabling MatchLabelKeysInPodAffinity feature gate.
   * - mismatchLabelKeys
     - array (string)
     - MismatchLabelKeys is a set of pod label keys to select which pods will be taken into consideration. The keys are used to lookup values from the incoming pod labels, those key-value labels are merged with `LabelSelector` as `key notin (value)` to select the group of existing pods which pods will be taken into consideration for the incoming pod's pod (anti) affinity. Keys that don't exist in the incoming pod labels will be ignored. The default value is empty. The same key is forbidden to exist in both MismatchLabelKeys and LabelSelector. Also, MismatchLabelKeys cannot be set when LabelSelector isn't set. This is an alpha field and requires enabling MatchLabelKeysInPodAffinity feature gate.
   * - :ref:`namespaceSelector<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector>`
     - object
     - A label query over the set of namespaces that the term applies to. The term is applied to the union of the namespaces selected by this field and the ones listed in the namespaces field. null selector and null or empty namespaces list means "this pod's namespace". An empty selector ({}) matches all namespaces.
   * - namespaces
     - array (string)
     - namespaces specifies a static list of namespace names that the term applies to. The term is applied to the union of the namespaces listed in this field and the ones selected by namespaceSelector. null or empty namespaces list and null namespaceSelector means "this pod's namespace".
   * - topologyKey
     - string
     - This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching the labelSelector in the specified namespaces, where co-located is defined as running on a node whose value of the label with key topologyKey matches that of any node on which any of the selected pods is running. Empty topologyKey is not allowed.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector:

.spec.components.prometheus.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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
   * - :ref:`matchExpressions<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchExpressions[]>`
     - array (object)
     - matchExpressions is a list of label selector requirements. The requirements are ANDed.
   * - :ref:`matchLabels<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchLabels>`
     - object
     - matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchExpressions[]:

.spec.components.prometheus.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchExpressions[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchLabels:

.spec.components.prometheus.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchLabels
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

Type
""""
object


.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector:

.spec.components.prometheus.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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
   * - :ref:`matchExpressions<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchExpressions[]>`
     - array (object)
     - matchExpressions is a list of label selector requirements. The requirements are ANDed.
   * - :ref:`matchLabels<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchLabels>`
     - object
     - matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchExpressions[]:

.spec.components.prometheus.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchExpressions[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchLabels:

.spec.components.prometheus.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchLabels
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

Type
""""
object


.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAntiAffinity:

.spec.components.prometheus.placement.podAntiAffinity
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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
   * - :ref:`preferredDuringSchedulingIgnoredDuringExecution<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[]>`
     - array (object)
     - The scheduler will prefer to schedule pods to nodes that satisfy the anti-affinity expressions specified by this field, but it may choose a node that violates one or more of the expressions. The node that is most preferred is the one with the greatest sum of weights, i.e. for each node that meets all of the scheduling requirements (resource request, requiredDuringScheduling anti-affinity expressions, etc.), compute a sum by iterating through the elements of this field and adding "weight" to the sum if the node has pods which matches the corresponding podAffinityTerm; the node(s) with the highest sum are the most preferred.
   * - :ref:`requiredDuringSchedulingIgnoredDuringExecution<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[]>`
     - array (object)
     - If the anti-affinity requirements specified by this field are not met at scheduling time, the pod will not be scheduled onto the node. If the anti-affinity requirements specified by this field cease to be met at some point during pod execution (e.g. due to a pod label update), the system may or may not try to eventually evict the pod from its node. When there are multiple elements, the lists of nodes corresponding to each podAffinityTerm are intersected, i.e. all terms must be satisfied.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[]:

.spec.components.prometheus.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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
   * - :ref:`podAffinityTerm<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm>`
     - object
     - Required. A pod affinity term, associated with the corresponding weight.
   * - weight
     - integer
     - weight associated with matching the corresponding podAffinityTerm, in the range 1-100.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm:

.spec.components.prometheus.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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
   * - :ref:`labelSelector<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector>`
     - object
     - A label query over a set of resources, in this case pods. If it's null, this PodAffinityTerm matches with no Pods.
   * - matchLabelKeys
     - array (string)
     - MatchLabelKeys is a set of pod label keys to select which pods will be taken into consideration. The keys are used to lookup values from the incoming pod labels, those key-value labels are merged with `LabelSelector` as `key in (value)` to select the group of existing pods which pods will be taken into consideration for the incoming pod's pod (anti) affinity. Keys that don't exist in the incoming pod labels will be ignored. The default value is empty. The same key is forbidden to exist in both MatchLabelKeys and LabelSelector. Also, MatchLabelKeys cannot be set when LabelSelector isn't set. This is an alpha field and requires enabling MatchLabelKeysInPodAffinity feature gate.
   * - mismatchLabelKeys
     - array (string)
     - MismatchLabelKeys is a set of pod label keys to select which pods will be taken into consideration. The keys are used to lookup values from the incoming pod labels, those key-value labels are merged with `LabelSelector` as `key notin (value)` to select the group of existing pods which pods will be taken into consideration for the incoming pod's pod (anti) affinity. Keys that don't exist in the incoming pod labels will be ignored. The default value is empty. The same key is forbidden to exist in both MismatchLabelKeys and LabelSelector. Also, MismatchLabelKeys cannot be set when LabelSelector isn't set. This is an alpha field and requires enabling MatchLabelKeysInPodAffinity feature gate.
   * - :ref:`namespaceSelector<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector>`
     - object
     - A label query over the set of namespaces that the term applies to. The term is applied to the union of the namespaces selected by this field and the ones listed in the namespaces field. null selector and null or empty namespaces list means "this pod's namespace". An empty selector ({}) matches all namespaces.
   * - namespaces
     - array (string)
     - namespaces specifies a static list of namespace names that the term applies to. The term is applied to the union of the namespaces listed in this field and the ones selected by namespaceSelector. null or empty namespaces list and null namespaceSelector means "this pod's namespace".
   * - topologyKey
     - string
     - This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching the labelSelector in the specified namespaces, where co-located is defined as running on a node whose value of the label with key topologyKey matches that of any node on which any of the selected pods is running. Empty topologyKey is not allowed.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector:

.spec.components.prometheus.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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
   * - :ref:`matchExpressions<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchExpressions[]>`
     - array (object)
     - matchExpressions is a list of label selector requirements. The requirements are ANDed.
   * - :ref:`matchLabels<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchLabels>`
     - object
     - matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchExpressions[]:

.spec.components.prometheus.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchExpressions[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchLabels:

.spec.components.prometheus.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchLabels
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

Type
""""
object


.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector:

.spec.components.prometheus.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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
   * - :ref:`matchExpressions<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchExpressions[]>`
     - array (object)
     - matchExpressions is a list of label selector requirements. The requirements are ANDed.
   * - :ref:`matchLabels<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchLabels>`
     - object
     - matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchExpressions[]:

.spec.components.prometheus.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchExpressions[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchLabels:

.spec.components.prometheus.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchLabels
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

Type
""""
object


.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[]:

.spec.components.prometheus.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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
   * - :ref:`labelSelector<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector>`
     - object
     - A label query over a set of resources, in this case pods. If it's null, this PodAffinityTerm matches with no Pods.
   * - matchLabelKeys
     - array (string)
     - MatchLabelKeys is a set of pod label keys to select which pods will be taken into consideration. The keys are used to lookup values from the incoming pod labels, those key-value labels are merged with `LabelSelector` as `key in (value)` to select the group of existing pods which pods will be taken into consideration for the incoming pod's pod (anti) affinity. Keys that don't exist in the incoming pod labels will be ignored. The default value is empty. The same key is forbidden to exist in both MatchLabelKeys and LabelSelector. Also, MatchLabelKeys cannot be set when LabelSelector isn't set. This is an alpha field and requires enabling MatchLabelKeysInPodAffinity feature gate.
   * - mismatchLabelKeys
     - array (string)
     - MismatchLabelKeys is a set of pod label keys to select which pods will be taken into consideration. The keys are used to lookup values from the incoming pod labels, those key-value labels are merged with `LabelSelector` as `key notin (value)` to select the group of existing pods which pods will be taken into consideration for the incoming pod's pod (anti) affinity. Keys that don't exist in the incoming pod labels will be ignored. The default value is empty. The same key is forbidden to exist in both MismatchLabelKeys and LabelSelector. Also, MismatchLabelKeys cannot be set when LabelSelector isn't set. This is an alpha field and requires enabling MatchLabelKeysInPodAffinity feature gate.
   * - :ref:`namespaceSelector<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector>`
     - object
     - A label query over the set of namespaces that the term applies to. The term is applied to the union of the namespaces selected by this field and the ones listed in the namespaces field. null selector and null or empty namespaces list means "this pod's namespace". An empty selector ({}) matches all namespaces.
   * - namespaces
     - array (string)
     - namespaces specifies a static list of namespace names that the term applies to. The term is applied to the union of the namespaces listed in this field and the ones selected by namespaceSelector. null or empty namespaces list and null namespaceSelector means "this pod's namespace".
   * - topologyKey
     - string
     - This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching the labelSelector in the specified namespaces, where co-located is defined as running on a node whose value of the label with key topologyKey matches that of any node on which any of the selected pods is running. Empty topologyKey is not allowed.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector:

.spec.components.prometheus.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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
   * - :ref:`matchExpressions<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchExpressions[]>`
     - array (object)
     - matchExpressions is a list of label selector requirements. The requirements are ANDed.
   * - :ref:`matchLabels<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchLabels>`
     - object
     - matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchExpressions[]:

.spec.components.prometheus.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchExpressions[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchLabels:

.spec.components.prometheus.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchLabels
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

Type
""""
object


.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector:

.spec.components.prometheus.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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
   * - :ref:`matchExpressions<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchExpressions[]>`
     - array (object)
     - matchExpressions is a list of label selector requirements. The requirements are ANDed.
   * - :ref:`matchLabels<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchLabels>`
     - object
     - matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchExpressions[]:

.spec.components.prometheus.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchExpressions[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchLabels:

.spec.components.prometheus.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchLabels
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

Type
""""
object


.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.placement.tolerations[]:

.spec.components.prometheus.placement.tolerations[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.resources:

.spec.components.prometheus.resources
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
resources the Prometheus container will use.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`claims<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.resources.claims[]>`
     - array (object)
     - Claims lists the names of resources, defined in spec.resourceClaims, that are used by this container. 
        This is an alpha field and requires enabling the DynamicResourceAllocation feature gate. 
        This field is immutable. It can only be set for containers.
   * - :ref:`limits<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.resources.limits>`
     - object
     - Limits describes the maximum amount of compute resources allowed. More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
   * - :ref:`requests<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.resources.requests>`
     - object
     - Requests describes the minimum amount of compute resources required. If Requests is omitted for a container, it defaults to Limits if that is explicitly specified, otherwise to an implementation-defined value. Requests cannot exceed Limits. More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.resources.claims[]:

.spec.components.prometheus.resources.claims[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.resources.limits:

.spec.components.prometheus.resources.limits
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
Limits describes the maximum amount of compute resources allowed. More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/

Type
""""
object


.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.resources.requests:

.spec.components.prometheus.resources.requests
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
Requests describes the minimum amount of compute resources required. If Requests is omitted for a container, it defaults to Limits if that is explicitly specified, otherwise to an implementation-defined value. Requests cannot exceed Limits. More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/

Type
""""
object


.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.storage:

.spec.components.prometheus.storage
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
storage describes the underlying storage that Prometheus will consume.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`annotations<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.storage.annotations>`
     - object
     - Annotations is an unstructured key value map stored with a resource that may be set by external tools to store and retrieve arbitrary metadata. They are not queryable and should be preserved when modifying objects. More info: http://kubernetes.io/docs/user-guide/annotations
   * - :ref:`labels<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.storage.labels>`
     - object
     - Map of string keys and values that can be used to organize and categorize (scope and select) objects. May match selectors of replication controllers and services. More info: http://kubernetes.io/docs/user-guide/labels
   * - :ref:`volumeClaimTemplate<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.storage.volumeClaimTemplate>`
     - object
     - volumeClaimTemplates is a PVC template defining storage to be used by Prometheus.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.storage.annotations:

.spec.components.prometheus.storage.annotations
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
Annotations is an unstructured key value map stored with a resource that may be set by external tools to store and retrieve arbitrary metadata. They are not queryable and should be preserved when modifying objects. More info: http://kubernetes.io/docs/user-guide/annotations

Type
""""
object


.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.storage.labels:

.spec.components.prometheus.storage.labels
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
Map of string keys and values that can be used to organize and categorize (scope and select) objects. May match selectors of replication controllers and services. More info: http://kubernetes.io/docs/user-guide/labels

Type
""""
object


.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.storage.volumeClaimTemplate:

.spec.components.prometheus.storage.volumeClaimTemplate
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
volumeClaimTemplates is a PVC template defining storage to be used by Prometheus.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`metadata<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.storage.volumeClaimTemplate.metadata>`
     - object
     - May contain labels and annotations that will be copied into the PVC when creating it. No other fields are allowed and will be rejected during validation.
   * - :ref:`spec<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.storage.volumeClaimTemplate.spec>`
     - object
     - The specification for the PersistentVolumeClaim. The entire content is copied unchanged into the PVC that gets created from this template. The same fields as in a PersistentVolumeClaim are also valid here.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.storage.volumeClaimTemplate.metadata:

.spec.components.prometheus.storage.volumeClaimTemplate.metadata
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
May contain labels and annotations that will be copied into the PVC when creating it. No other fields are allowed and will be rejected during validation.

Type
""""
object


.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.storage.volumeClaimTemplate.spec:

.spec.components.prometheus.storage.volumeClaimTemplate.spec
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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
   * - :ref:`dataSource<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.storage.volumeClaimTemplate.spec.dataSource>`
     - object
     - dataSource field can be used to specify either: * An existing VolumeSnapshot object (snapshot.storage.k8s.io/VolumeSnapshot) * An existing PVC (PersistentVolumeClaim) If the provisioner or an external controller can support the specified data source, it will create a new volume based on the contents of the specified data source. When the AnyVolumeDataSource feature gate is enabled, dataSource contents will be copied to dataSourceRef, and dataSourceRef contents will be copied to dataSource when dataSourceRef.namespace is not specified. If the namespace is specified, then dataSourceRef will not be copied to dataSource.
   * - :ref:`dataSourceRef<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.storage.volumeClaimTemplate.spec.dataSourceRef>`
     - object
     - dataSourceRef specifies the object from which to populate the volume with data, if a non-empty volume is desired. This may be any object from a non-empty API group (non core object) or a PersistentVolumeClaim object. When this field is specified, volume binding will only succeed if the type of the specified object matches some installed volume populator or dynamic provisioner. This field will replace the functionality of the dataSource field and as such if both fields are non-empty, they must have the same value. For backwards compatibility, when namespace isn't specified in dataSourceRef, both fields (dataSource and dataSourceRef) will be set to the same value automatically if one of them is empty and the other is non-empty. When namespace is specified in dataSourceRef, dataSource isn't set to the same value and must be empty. There are three important differences between dataSource and dataSourceRef: * While dataSource only allows two specific types of objects, dataSourceRef allows any non-core object, as well as PersistentVolumeClaim objects. * While dataSource ignores disallowed values (dropping them), dataSourceRef preserves all values, and generates an error if a disallowed value is specified. * While dataSource only allows local objects, dataSourceRef allows objects in any namespaces. (Beta) Using this field requires the AnyVolumeDataSource feature gate to be enabled. (Alpha) Using the namespace field of dataSourceRef requires the CrossNamespaceVolumeDataSource feature gate to be enabled.
   * - :ref:`resources<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.storage.volumeClaimTemplate.spec.resources>`
     - object
     - resources represents the minimum resources the volume should have. If RecoverVolumeExpansionFailure feature is enabled users are allowed to specify resource requirements that are lower than previous value but must still be higher than capacity recorded in the status field of the claim. More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#resources
   * - :ref:`selector<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.storage.volumeClaimTemplate.spec.selector>`
     - object
     - selector is a label query over volumes to consider for binding.
   * - storageClassName
     - string
     - storageClassName is the name of the StorageClass required by the claim. More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#class-1
   * - volumeAttributesClassName
     - string
     - volumeAttributesClassName may be used to set the VolumeAttributesClass used by this claim. If specified, the CSI driver will create or update the volume with the attributes defined in the corresponding VolumeAttributesClass. This has a different purpose than storageClassName, it can be changed after the claim is created. An empty string value means that no VolumeAttributesClass will be applied to the claim but it's not allowed to reset this field to empty string once it is set. If unspecified and the PersistentVolumeClaim is unbound, the default VolumeAttributesClass will be set by the persistentvolume controller if it exists. If the resource referred to by volumeAttributesClass does not exist, this PersistentVolumeClaim will be set to a Pending state, as reflected by the modifyVolumeStatus field, until such as a resource exists. More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#volumeattributesclass (Alpha) Using this field requires the VolumeAttributesClass feature gate to be enabled.
   * - volumeMode
     - string
     - volumeMode defines what type of volume is required by the claim. Value of Filesystem is implied when not included in claim spec.
   * - volumeName
     - string
     - volumeName is the binding reference to the PersistentVolume backing this claim.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.storage.volumeClaimTemplate.spec.dataSource:

.spec.components.prometheus.storage.volumeClaimTemplate.spec.dataSource
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.storage.volumeClaimTemplate.spec.dataSourceRef:

.spec.components.prometheus.storage.volumeClaimTemplate.spec.dataSourceRef
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
dataSourceRef specifies the object from which to populate the volume with data, if a non-empty volume is desired. This may be any object from a non-empty API group (non core object) or a PersistentVolumeClaim object. When this field is specified, volume binding will only succeed if the type of the specified object matches some installed volume populator or dynamic provisioner. This field will replace the functionality of the dataSource field and as such if both fields are non-empty, they must have the same value. For backwards compatibility, when namespace isn't specified in dataSourceRef, both fields (dataSource and dataSourceRef) will be set to the same value automatically if one of them is empty and the other is non-empty. When namespace is specified in dataSourceRef, dataSource isn't set to the same value and must be empty. There are three important differences between dataSource and dataSourceRef: * While dataSource only allows two specific types of objects, dataSourceRef allows any non-core object, as well as PersistentVolumeClaim objects. * While dataSource ignores disallowed values (dropping them), dataSourceRef preserves all values, and generates an error if a disallowed value is specified. * While dataSource only allows local objects, dataSourceRef allows objects in any namespaces. (Beta) Using this field requires the AnyVolumeDataSource feature gate to be enabled. (Alpha) Using the namespace field of dataSourceRef requires the CrossNamespaceVolumeDataSource feature gate to be enabled.

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

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.storage.volumeClaimTemplate.spec.resources:

.spec.components.prometheus.storage.volumeClaimTemplate.spec.resources
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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
   * - :ref:`limits<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.storage.volumeClaimTemplate.spec.resources.limits>`
     - object
     - Limits describes the maximum amount of compute resources allowed. More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
   * - :ref:`requests<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.storage.volumeClaimTemplate.spec.resources.requests>`
     - object
     - Requests describes the minimum amount of compute resources required. If Requests is omitted for a container, it defaults to Limits if that is explicitly specified, otherwise to an implementation-defined value. Requests cannot exceed Limits. More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.storage.volumeClaimTemplate.spec.resources.limits:

.spec.components.prometheus.storage.volumeClaimTemplate.spec.resources.limits
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
Limits describes the maximum amount of compute resources allowed. More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/

Type
""""
object


.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.storage.volumeClaimTemplate.spec.resources.requests:

.spec.components.prometheus.storage.volumeClaimTemplate.spec.resources.requests
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
Requests describes the minimum amount of compute resources required. If Requests is omitted for a container, it defaults to Limits if that is explicitly specified, otherwise to an implementation-defined value. Requests cannot exceed Limits. More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/

Type
""""
object


.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.storage.volumeClaimTemplate.spec.selector:

.spec.components.prometheus.storage.volumeClaimTemplate.spec.selector
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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
   * - :ref:`matchExpressions<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.storage.volumeClaimTemplate.spec.selector.matchExpressions[]>`
     - array (object)
     - matchExpressions is a list of label selector requirements. The requirements are ANDed.
   * - :ref:`matchLabels<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.storage.volumeClaimTemplate.spec.selector.matchLabels>`
     - object
     - matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.storage.volumeClaimTemplate.spec.selector.matchExpressions[]:

.spec.components.prometheus.storage.volumeClaimTemplate.spec.selector.matchExpressions[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.components.prometheus.storage.volumeClaimTemplate.spec.selector.matchLabels:

.spec.components.prometheus.storage.volumeClaimTemplate.spec.selector.matchLabels
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

Type
""""
object


.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.endpointsSelector:

.spec.endpointsSelector
^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
endpointsSelector select which Endpoints should be scraped. For local ScyllaDB clusters or datacenters, this is the same selector as if you were trying to select member Services. For remote ScyllaDB clusters, this can select any endpoints that are created manually or for a Service without selectors.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`matchExpressions<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.endpointsSelector.matchExpressions[]>`
     - array (object)
     - matchExpressions is a list of label selector requirements. The requirements are ANDed.
   * - :ref:`matchLabels<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.endpointsSelector.matchLabels>`
     - object
     - matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.endpointsSelector.matchExpressions[]:

.spec.endpointsSelector.matchExpressions[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.spec.endpointsSelector.matchLabels:

.spec.endpointsSelector.matchLabels
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

Type
""""
object


.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.status:

.status
^^^^^^^

Description
"""""""""""
status is the current status of this ScyllaDBMonitoring.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`conditions<api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.status.conditions[]>`
     - array (object)
     - conditions hold conditions describing ScyllaDBMonitoring state. To determine whether a cluster rollout is finished, look for Available=True,Progressing=False,Degraded=False.
   * - observedGeneration
     - integer
     - observedGeneration is the most recent generation observed for this ScyllaDBMonitoring. It corresponds to the ScyllaDBMonitoring's generation, which is updated on mutation by the API Server.

.. _api-scylla.scylladb.com-scylladbmonitorings-v1alpha1-.status.conditions[]:

.status.conditions[]
^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
Condition contains details for one aspect of the current state of this API Resource. --- This struct is intended for direct use as an array at the field path .status.conditions.  For example, 
 type FooStatus struct{ // Represents the observations of a foo's current state. // Known .status.conditions.type are: "Available", "Progressing", and "Degraded" // +patchMergeKey=type // +patchStrategy=merge // +listType=map // +listMapKey=type Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"` 
 // other fields }

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
     - type of condition in CamelCase or in foo.example.com/CamelCase. --- Many .condition.type values are consistent across resources like Available, but because arbitrary conditions can be useful (see .node.status.conditions), the ability to deconflict is important. The regex it matches is (dns1123SubdomainFmt/)?(qualifiedNameFmt)
