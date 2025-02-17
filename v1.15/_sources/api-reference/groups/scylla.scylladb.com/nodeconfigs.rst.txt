NodeConfig (scylla.scylladb.com/v1alpha1)
=========================================

| **APIVersion**: scylla.scylladb.com/v1alpha1
| **Kind**: NodeConfig
| **PluralName**: nodeconfigs
| **SingularName**: nodeconfig
| **Scope**: Cluster
| **ListKind**: NodeConfigList
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
   * - :ref:`metadata<api-scylla.scylladb.com-nodeconfigs-v1alpha1-.metadata>`
     - object
     - 
   * - :ref:`spec<api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec>`
     - object
     - 
   * - :ref:`status<api-scylla.scylladb.com-nodeconfigs-v1alpha1-.status>`
     - object
     - 

.. _api-scylla.scylladb.com-nodeconfigs-v1alpha1-.metadata:

.metadata
^^^^^^^^^

Description
"""""""""""


Type
""""
object


.. _api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec:

.spec
^^^^^

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
   * - disableOptimizations
     - boolean
     - disableOptimizations controls if nodes matching placement requirements are going to be optimized. Turning off optimizations on already optimized Nodes does not revert changes.
   * - :ref:`localDiskSetup<api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.localDiskSetup>`
     - object
     - localDiskSetup contains options of automatic local disk setup.
   * - :ref:`placement<api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement>`
     - object
     - placement contains scheduling rules for NodeConfig Pods.

.. _api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.localDiskSetup:

.spec.localDiskSetup
^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
localDiskSetup contains options of automatic local disk setup.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`filesystems<api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.localDiskSetup.filesystems[]>`
     - array (object)
     - filesystems is a list of filesystem configurations.
   * - :ref:`loopDevices<api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.localDiskSetup.loopDevices[]>`
     - array (object)
     - loops is a list of loop device configurations.
   * - :ref:`mounts<api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.localDiskSetup.mounts[]>`
     - array (object)
     - mounts is a list of mount configuration.
   * - :ref:`raids<api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.localDiskSetup.raids[]>`
     - array (object)
     - raids is a list of raid configurations.

.. _api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.localDiskSetup.filesystems[]:

.spec.localDiskSetup.filesystems[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
FilesystemConfiguration specifies filesystem configuration options.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - device
     - string
     - device is a path to the device where the desired filesystem should be created.
   * - type
     - string
     - type is a desired filesystem type.

.. _api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.localDiskSetup.loopDevices[]:

.spec.localDiskSetup.loopDevices[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
LoopDeviceConfiguration specifies loop device configuration options.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - imagePath
     - string
     - imagePath specifies path on host where backing image file for loop device should be located.
   * - name
     - string
     - name specifies the name of the symlink that will point to actual loop device, created under `/dev/loops/`.
   * - size
     - 
     - size specifies the size of the loop device.

.. _api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.localDiskSetup.mounts[]:

.spec.localDiskSetup.mounts[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
MountConfiguration specifies mount configuration options.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - device
     - string
     - device is path to a device that should be mounted.
   * - fsType
     - string
     - fsType specifies the filesystem on the device.
   * - mountPoint
     - string
     - mountPoint is a path where the device should be mounted at. If the mountPoint is a symlink, the mount will be set up for the target.
   * - unsupportedOptions
     - array (string)
     - unsupportedOptions is a list of mount options used during device mounting. unsupported in this field name means that we won't support all the available options passed down using this field.

.. _api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.localDiskSetup.raids[]:

.spec.localDiskSetup.raids[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
RAIDConfiguration is a configuration of a raid array.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`RAID0<api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.localDiskSetup.raids[].RAID0>`
     - object
     - RAID0 specifies RAID0 options.
   * - name
     - string
     - name specifies the name of the raid device to be created under in `/dev/md/`.
   * - type
     - string
     - type is a type of raid array.

.. _api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.localDiskSetup.raids[].RAID0:

.spec.localDiskSetup.raids[].RAID0
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
RAID0 specifies RAID0 options.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`devices<api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.localDiskSetup.raids[].RAID0.devices>`
     - object
     - devices defines which devices constitute the raid array.

.. _api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.localDiskSetup.raids[].RAID0.devices:

.spec.localDiskSetup.raids[].RAID0.devices
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
devices defines which devices constitute the raid array.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - modelRegex
     - string
     - modelRegex is a regular expression filtering devices by their model name.
   * - nameRegex
     - string
     - nameRegex is a regular expression filtering devices by their name.

.. _api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement:

.spec.placement
^^^^^^^^^^^^^^^

Description
"""""""""""
placement contains scheduling rules for NodeConfig Pods.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`affinity<api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity>`
     - object
     - affinity is a group of affinity scheduling rules for NodeConfig Pods.
   * - :ref:`nodeSelector<api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.nodeSelector>`
     - object
     - nodeSelector is a selector which must be true for the NodeConfig Pod to fit on a node. Selector which must match a node's labels for the pod to be scheduled on that node.
   * - :ref:`tolerations<api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.tolerations[]>`
     - array (object)
     - tolerations is a group of tolerations NodeConfig Pods are going to have.

.. _api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity:

.spec.placement.affinity
^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
affinity is a group of affinity scheduling rules for NodeConfig Pods.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`nodeAffinity<api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.nodeAffinity>`
     - object
     - Describes node affinity scheduling rules for the pod.
   * - :ref:`podAffinity<api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAffinity>`
     - object
     - Describes pod affinity scheduling rules (e.g. co-locate this pod in the same node, zone, etc. as some other pod(s)).
   * - :ref:`podAntiAffinity<api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAntiAffinity>`
     - object
     - Describes pod anti-affinity scheduling rules (e.g. avoid putting this pod in the same node, zone, etc. as some other pod(s)).

.. _api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.nodeAffinity:

.spec.placement.affinity.nodeAffinity
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
Describes node affinity scheduling rules for the pod.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`preferredDuringSchedulingIgnoredDuringExecution<api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[]>`
     - array (object)
     - The scheduler will prefer to schedule pods to nodes that satisfy the affinity expressions specified by this field, but it may choose a node that violates one or more of the expressions. The node that is most preferred is the one with the greatest sum of weights, i.e. for each node that meets all of the scheduling requirements (resource request, requiredDuringScheduling affinity expressions, etc.), compute a sum by iterating through the elements of this field and adding "weight" to the sum if the node matches the corresponding matchExpressions; the node(s) with the highest sum are the most preferred.
   * - :ref:`requiredDuringSchedulingIgnoredDuringExecution<api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution>`
     - object
     - If the affinity requirements specified by this field are not met at scheduling time, the pod will not be scheduled onto the node. If the affinity requirements specified by this field cease to be met at some point during pod execution (e.g. due to an update), the system may or may not try to eventually evict the pod from its node.

.. _api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[]:

.spec.placement.affinity.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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
   * - :ref:`preference<api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[].preference>`
     - object
     - A node selector term, associated with the corresponding weight.
   * - weight
     - integer
     - Weight associated with matching the corresponding nodeSelectorTerm, in the range 1-100.

.. _api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[].preference:

.spec.placement.affinity.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[].preference
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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
   * - :ref:`matchExpressions<api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[].preference.matchExpressions[]>`
     - array (object)
     - A list of node selector requirements by node's labels.
   * - :ref:`matchFields<api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[].preference.matchFields[]>`
     - array (object)
     - A list of node selector requirements by node's fields.

.. _api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[].preference.matchExpressions[]:

.spec.placement.affinity.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[].preference.matchExpressions[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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

.. _api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[].preference.matchFields[]:

.spec.placement.affinity.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[].preference.matchFields[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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

.. _api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution:

.spec.placement.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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
   * - :ref:`nodeSelectorTerms<api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[]>`
     - array (object)
     - Required. A list of node selector terms. The terms are ORed.

.. _api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[]:

.spec.placement.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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
   * - :ref:`matchExpressions<api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[].matchExpressions[]>`
     - array (object)
     - A list of node selector requirements by node's labels.
   * - :ref:`matchFields<api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[].matchFields[]>`
     - array (object)
     - A list of node selector requirements by node's fields.

.. _api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[].matchExpressions[]:

.spec.placement.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[].matchExpressions[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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

.. _api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[].matchFields[]:

.spec.placement.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[].matchFields[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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

.. _api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAffinity:

.spec.placement.affinity.podAffinity
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
Describes pod affinity scheduling rules (e.g. co-locate this pod in the same node, zone, etc. as some other pod(s)).

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`preferredDuringSchedulingIgnoredDuringExecution<api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[]>`
     - array (object)
     - The scheduler will prefer to schedule pods to nodes that satisfy the affinity expressions specified by this field, but it may choose a node that violates one or more of the expressions. The node that is most preferred is the one with the greatest sum of weights, i.e. for each node that meets all of the scheduling requirements (resource request, requiredDuringScheduling affinity expressions, etc.), compute a sum by iterating through the elements of this field and adding "weight" to the sum if the node has pods which matches the corresponding podAffinityTerm; the node(s) with the highest sum are the most preferred.
   * - :ref:`requiredDuringSchedulingIgnoredDuringExecution<api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[]>`
     - array (object)
     - If the affinity requirements specified by this field are not met at scheduling time, the pod will not be scheduled onto the node. If the affinity requirements specified by this field cease to be met at some point during pod execution (e.g. due to a pod label update), the system may or may not try to eventually evict the pod from its node. When there are multiple elements, the lists of nodes corresponding to each podAffinityTerm are intersected, i.e. all terms must be satisfied.

.. _api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[]:

.spec.placement.affinity.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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
   * - :ref:`podAffinityTerm<api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm>`
     - object
     - Required. A pod affinity term, associated with the corresponding weight.
   * - weight
     - integer
     - weight associated with matching the corresponding podAffinityTerm, in the range 1-100.

.. _api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm:

.spec.placement.affinity.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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
   * - :ref:`labelSelector<api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector>`
     - object
     - A label query over a set of resources, in this case pods. If it's null, this PodAffinityTerm matches with no Pods.
   * - matchLabelKeys
     - array (string)
     - MatchLabelKeys is a set of pod label keys to select which pods will be taken into consideration. The keys are used to lookup values from the incoming pod labels, those key-value labels are merged with `LabelSelector` as `key in (value)` to select the group of existing pods which pods will be taken into consideration for the incoming pod's pod (anti) affinity. Keys that don't exist in the incoming pod labels will be ignored. The default value is empty. The same key is forbidden to exist in both MatchLabelKeys and LabelSelector. Also, MatchLabelKeys cannot be set when LabelSelector isn't set. This is an alpha field and requires enabling MatchLabelKeysInPodAffinity feature gate.
   * - mismatchLabelKeys
     - array (string)
     - MismatchLabelKeys is a set of pod label keys to select which pods will be taken into consideration. The keys are used to lookup values from the incoming pod labels, those key-value labels are merged with `LabelSelector` as `key notin (value)` to select the group of existing pods which pods will be taken into consideration for the incoming pod's pod (anti) affinity. Keys that don't exist in the incoming pod labels will be ignored. The default value is empty. The same key is forbidden to exist in both MismatchLabelKeys and LabelSelector. Also, MismatchLabelKeys cannot be set when LabelSelector isn't set. This is an alpha field and requires enabling MatchLabelKeysInPodAffinity feature gate.
   * - :ref:`namespaceSelector<api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector>`
     - object
     - A label query over the set of namespaces that the term applies to. The term is applied to the union of the namespaces selected by this field and the ones listed in the namespaces field. null selector and null or empty namespaces list means "this pod's namespace". An empty selector ({}) matches all namespaces.
   * - namespaces
     - array (string)
     - namespaces specifies a static list of namespace names that the term applies to. The term is applied to the union of the namespaces listed in this field and the ones selected by namespaceSelector. null or empty namespaces list and null namespaceSelector means "this pod's namespace".
   * - topologyKey
     - string
     - This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching the labelSelector in the specified namespaces, where co-located is defined as running on a node whose value of the label with key topologyKey matches that of any node on which any of the selected pods is running. Empty topologyKey is not allowed.

.. _api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector:

.spec.placement.affinity.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector
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
   * - :ref:`matchExpressions<api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchExpressions[]>`
     - array (object)
     - matchExpressions is a list of label selector requirements. The requirements are ANDed.
   * - :ref:`matchLabels<api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchLabels>`
     - object
     - matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

.. _api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchExpressions[]:

.spec.placement.affinity.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchExpressions[]
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

.. _api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchLabels:

.spec.placement.affinity.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchLabels
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

Type
""""
object


.. _api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector:

.spec.placement.affinity.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector
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
   * - :ref:`matchExpressions<api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchExpressions[]>`
     - array (object)
     - matchExpressions is a list of label selector requirements. The requirements are ANDed.
   * - :ref:`matchLabels<api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchLabels>`
     - object
     - matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

.. _api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchExpressions[]:

.spec.placement.affinity.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchExpressions[]
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

.. _api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchLabels:

.spec.placement.affinity.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchLabels
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

Type
""""
object


.. _api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[]:

.spec.placement.affinity.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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
   * - :ref:`labelSelector<api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector>`
     - object
     - A label query over a set of resources, in this case pods. If it's null, this PodAffinityTerm matches with no Pods.
   * - matchLabelKeys
     - array (string)
     - MatchLabelKeys is a set of pod label keys to select which pods will be taken into consideration. The keys are used to lookup values from the incoming pod labels, those key-value labels are merged with `LabelSelector` as `key in (value)` to select the group of existing pods which pods will be taken into consideration for the incoming pod's pod (anti) affinity. Keys that don't exist in the incoming pod labels will be ignored. The default value is empty. The same key is forbidden to exist in both MatchLabelKeys and LabelSelector. Also, MatchLabelKeys cannot be set when LabelSelector isn't set. This is an alpha field and requires enabling MatchLabelKeysInPodAffinity feature gate.
   * - mismatchLabelKeys
     - array (string)
     - MismatchLabelKeys is a set of pod label keys to select which pods will be taken into consideration. The keys are used to lookup values from the incoming pod labels, those key-value labels are merged with `LabelSelector` as `key notin (value)` to select the group of existing pods which pods will be taken into consideration for the incoming pod's pod (anti) affinity. Keys that don't exist in the incoming pod labels will be ignored. The default value is empty. The same key is forbidden to exist in both MismatchLabelKeys and LabelSelector. Also, MismatchLabelKeys cannot be set when LabelSelector isn't set. This is an alpha field and requires enabling MatchLabelKeysInPodAffinity feature gate.
   * - :ref:`namespaceSelector<api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector>`
     - object
     - A label query over the set of namespaces that the term applies to. The term is applied to the union of the namespaces selected by this field and the ones listed in the namespaces field. null selector and null or empty namespaces list means "this pod's namespace". An empty selector ({}) matches all namespaces.
   * - namespaces
     - array (string)
     - namespaces specifies a static list of namespace names that the term applies to. The term is applied to the union of the namespaces listed in this field and the ones selected by namespaceSelector. null or empty namespaces list and null namespaceSelector means "this pod's namespace".
   * - topologyKey
     - string
     - This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching the labelSelector in the specified namespaces, where co-located is defined as running on a node whose value of the label with key topologyKey matches that of any node on which any of the selected pods is running. Empty topologyKey is not allowed.

.. _api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector:

.spec.placement.affinity.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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
   * - :ref:`matchExpressions<api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchExpressions[]>`
     - array (object)
     - matchExpressions is a list of label selector requirements. The requirements are ANDed.
   * - :ref:`matchLabels<api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchLabels>`
     - object
     - matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

.. _api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchExpressions[]:

.spec.placement.affinity.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchExpressions[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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

.. _api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchLabels:

.spec.placement.affinity.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchLabels
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

Type
""""
object


.. _api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector:

.spec.placement.affinity.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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
   * - :ref:`matchExpressions<api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchExpressions[]>`
     - array (object)
     - matchExpressions is a list of label selector requirements. The requirements are ANDed.
   * - :ref:`matchLabels<api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchLabels>`
     - object
     - matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

.. _api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchExpressions[]:

.spec.placement.affinity.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchExpressions[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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

.. _api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchLabels:

.spec.placement.affinity.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchLabels
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

Type
""""
object


.. _api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAntiAffinity:

.spec.placement.affinity.podAntiAffinity
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
Describes pod anti-affinity scheduling rules (e.g. avoid putting this pod in the same node, zone, etc. as some other pod(s)).

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`preferredDuringSchedulingIgnoredDuringExecution<api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[]>`
     - array (object)
     - The scheduler will prefer to schedule pods to nodes that satisfy the anti-affinity expressions specified by this field, but it may choose a node that violates one or more of the expressions. The node that is most preferred is the one with the greatest sum of weights, i.e. for each node that meets all of the scheduling requirements (resource request, requiredDuringScheduling anti-affinity expressions, etc.), compute a sum by iterating through the elements of this field and adding "weight" to the sum if the node has pods which matches the corresponding podAffinityTerm; the node(s) with the highest sum are the most preferred.
   * - :ref:`requiredDuringSchedulingIgnoredDuringExecution<api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[]>`
     - array (object)
     - If the anti-affinity requirements specified by this field are not met at scheduling time, the pod will not be scheduled onto the node. If the anti-affinity requirements specified by this field cease to be met at some point during pod execution (e.g. due to a pod label update), the system may or may not try to eventually evict the pod from its node. When there are multiple elements, the lists of nodes corresponding to each podAffinityTerm are intersected, i.e. all terms must be satisfied.

.. _api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[]:

.spec.placement.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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
   * - :ref:`podAffinityTerm<api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm>`
     - object
     - Required. A pod affinity term, associated with the corresponding weight.
   * - weight
     - integer
     - weight associated with matching the corresponding podAffinityTerm, in the range 1-100.

.. _api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm:

.spec.placement.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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
   * - :ref:`labelSelector<api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector>`
     - object
     - A label query over a set of resources, in this case pods. If it's null, this PodAffinityTerm matches with no Pods.
   * - matchLabelKeys
     - array (string)
     - MatchLabelKeys is a set of pod label keys to select which pods will be taken into consideration. The keys are used to lookup values from the incoming pod labels, those key-value labels are merged with `LabelSelector` as `key in (value)` to select the group of existing pods which pods will be taken into consideration for the incoming pod's pod (anti) affinity. Keys that don't exist in the incoming pod labels will be ignored. The default value is empty. The same key is forbidden to exist in both MatchLabelKeys and LabelSelector. Also, MatchLabelKeys cannot be set when LabelSelector isn't set. This is an alpha field and requires enabling MatchLabelKeysInPodAffinity feature gate.
   * - mismatchLabelKeys
     - array (string)
     - MismatchLabelKeys is a set of pod label keys to select which pods will be taken into consideration. The keys are used to lookup values from the incoming pod labels, those key-value labels are merged with `LabelSelector` as `key notin (value)` to select the group of existing pods which pods will be taken into consideration for the incoming pod's pod (anti) affinity. Keys that don't exist in the incoming pod labels will be ignored. The default value is empty. The same key is forbidden to exist in both MismatchLabelKeys and LabelSelector. Also, MismatchLabelKeys cannot be set when LabelSelector isn't set. This is an alpha field and requires enabling MatchLabelKeysInPodAffinity feature gate.
   * - :ref:`namespaceSelector<api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector>`
     - object
     - A label query over the set of namespaces that the term applies to. The term is applied to the union of the namespaces selected by this field and the ones listed in the namespaces field. null selector and null or empty namespaces list means "this pod's namespace". An empty selector ({}) matches all namespaces.
   * - namespaces
     - array (string)
     - namespaces specifies a static list of namespace names that the term applies to. The term is applied to the union of the namespaces listed in this field and the ones selected by namespaceSelector. null or empty namespaces list and null namespaceSelector means "this pod's namespace".
   * - topologyKey
     - string
     - This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching the labelSelector in the specified namespaces, where co-located is defined as running on a node whose value of the label with key topologyKey matches that of any node on which any of the selected pods is running. Empty topologyKey is not allowed.

.. _api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector:

.spec.placement.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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
   * - :ref:`matchExpressions<api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchExpressions[]>`
     - array (object)
     - matchExpressions is a list of label selector requirements. The requirements are ANDed.
   * - :ref:`matchLabels<api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchLabels>`
     - object
     - matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

.. _api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchExpressions[]:

.spec.placement.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchExpressions[]
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

.. _api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchLabels:

.spec.placement.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchLabels
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

Type
""""
object


.. _api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector:

.spec.placement.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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
   * - :ref:`matchExpressions<api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchExpressions[]>`
     - array (object)
     - matchExpressions is a list of label selector requirements. The requirements are ANDed.
   * - :ref:`matchLabels<api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchLabels>`
     - object
     - matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

.. _api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchExpressions[]:

.spec.placement.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchExpressions[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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

.. _api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchLabels:

.spec.placement.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchLabels
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

Type
""""
object


.. _api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[]:

.spec.placement.affinity.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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
   * - :ref:`labelSelector<api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector>`
     - object
     - A label query over a set of resources, in this case pods. If it's null, this PodAffinityTerm matches with no Pods.
   * - matchLabelKeys
     - array (string)
     - MatchLabelKeys is a set of pod label keys to select which pods will be taken into consideration. The keys are used to lookup values from the incoming pod labels, those key-value labels are merged with `LabelSelector` as `key in (value)` to select the group of existing pods which pods will be taken into consideration for the incoming pod's pod (anti) affinity. Keys that don't exist in the incoming pod labels will be ignored. The default value is empty. The same key is forbidden to exist in both MatchLabelKeys and LabelSelector. Also, MatchLabelKeys cannot be set when LabelSelector isn't set. This is an alpha field and requires enabling MatchLabelKeysInPodAffinity feature gate.
   * - mismatchLabelKeys
     - array (string)
     - MismatchLabelKeys is a set of pod label keys to select which pods will be taken into consideration. The keys are used to lookup values from the incoming pod labels, those key-value labels are merged with `LabelSelector` as `key notin (value)` to select the group of existing pods which pods will be taken into consideration for the incoming pod's pod (anti) affinity. Keys that don't exist in the incoming pod labels will be ignored. The default value is empty. The same key is forbidden to exist in both MismatchLabelKeys and LabelSelector. Also, MismatchLabelKeys cannot be set when LabelSelector isn't set. This is an alpha field and requires enabling MatchLabelKeysInPodAffinity feature gate.
   * - :ref:`namespaceSelector<api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector>`
     - object
     - A label query over the set of namespaces that the term applies to. The term is applied to the union of the namespaces selected by this field and the ones listed in the namespaces field. null selector and null or empty namespaces list means "this pod's namespace". An empty selector ({}) matches all namespaces.
   * - namespaces
     - array (string)
     - namespaces specifies a static list of namespace names that the term applies to. The term is applied to the union of the namespaces listed in this field and the ones selected by namespaceSelector. null or empty namespaces list and null namespaceSelector means "this pod's namespace".
   * - topologyKey
     - string
     - This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching the labelSelector in the specified namespaces, where co-located is defined as running on a node whose value of the label with key topologyKey matches that of any node on which any of the selected pods is running. Empty topologyKey is not allowed.

.. _api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector:

.spec.placement.affinity.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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
   * - :ref:`matchExpressions<api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchExpressions[]>`
     - array (object)
     - matchExpressions is a list of label selector requirements. The requirements are ANDed.
   * - :ref:`matchLabels<api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchLabels>`
     - object
     - matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

.. _api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchExpressions[]:

.spec.placement.affinity.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchExpressions[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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

.. _api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchLabels:

.spec.placement.affinity.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchLabels
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

Type
""""
object


.. _api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector:

.spec.placement.affinity.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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
   * - :ref:`matchExpressions<api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchExpressions[]>`
     - array (object)
     - matchExpressions is a list of label selector requirements. The requirements are ANDed.
   * - :ref:`matchLabels<api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchLabels>`
     - object
     - matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

.. _api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchExpressions[]:

.spec.placement.affinity.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchExpressions[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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

.. _api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.affinity.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchLabels:

.spec.placement.affinity.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchLabels
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.

Type
""""
object


.. _api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.nodeSelector:

.spec.placement.nodeSelector
^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
nodeSelector is a selector which must be true for the NodeConfig Pod to fit on a node. Selector which must match a node's labels for the pod to be scheduled on that node.

Type
""""
object


.. _api-scylla.scylladb.com-nodeconfigs-v1alpha1-.spec.placement.tolerations[]:

.spec.placement.tolerations[]
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

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

.. _api-scylla.scylladb.com-nodeconfigs-v1alpha1-.status:

.status
^^^^^^^

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
   * - :ref:`conditions<api-scylla.scylladb.com-nodeconfigs-v1alpha1-.status.conditions[]>`
     - array (object)
     - conditions represents the latest available observations of current state.
   * - :ref:`nodeStatuses<api-scylla.scylladb.com-nodeconfigs-v1alpha1-.status.nodeStatuses[]>`
     - array (object)
     - nodeStatuses hold the status for each tuned node.
   * - observedGeneration
     - integer
     - observedGeneration indicates the most recent generation observed by the controller.

.. _api-scylla.scylladb.com-nodeconfigs-v1alpha1-.status.conditions[]:

.status.conditions[]
^^^^^^^^^^^^^^^^^^^^

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
   * - lastTransitionTime
     - string
     - lastTransitionTime is last time the condition transitioned from one status to another.
   * - message
     - string
     - message is a human-readable message indicating details about the transition.
   * - observedGeneration
     - integer
     - observedGeneration represents the .metadata.generation that the condition was set based upon. For instance, if .metadata.generation is currently 12, but the .status.conditions[x].observedGeneration is 9, the condition is out of date with respect to the current state of the instance.
   * - reason
     - string
     - reason is the reason for condition's last transition.
   * - status
     - string
     - status represents the state of the condition, one of True, False, or Unknown.
   * - type
     - string
     - type is the type of the NodeConfig condition.

.. _api-scylla.scylladb.com-nodeconfigs-v1alpha1-.status.nodeStatuses[]:

.status.nodeStatuses[]
^^^^^^^^^^^^^^^^^^^^^^

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
     - 
   * - tunedContainers
     - array (string)
     - 
   * - tunedNode
     - boolean
     - 
