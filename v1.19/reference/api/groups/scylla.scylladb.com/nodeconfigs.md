# NodeConfig (scylla.scylladb.com/v1alpha1)

**APIVersion**: scylla.scylladb.com/v1alpha1
<br/>
**Kind**: NodeConfig
<br/>
**PluralName**: nodeconfigs
<br/>
**SingularName**: nodeconfig
<br/>
**Scope**: Cluster
<br/>
**ListKind**: NodeConfigList
<br/>
**Served**: true
<br/>
**Storage**: true
<br/>

## Description

## Specification

| Property                                                           | Type   | Description                                                                                                                                                                                                                                                                                                                                                                                           |
|--------------------------------------------------------------------|--------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| apiVersion                                                         | string | APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: [https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources](https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources)    |
| kind                                                               | string | Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: [https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds](https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds) |
| [metadata](#api-scylla-scylladb-com-nodeconfigs-v1alpha1-metadata) | object |                                                                                                                                                                                                                                                                                                                                                                                                       |
| [spec](#api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec)         | object |                                                                                                                                                                                                                                                                                                                                                                                                       |
| [status](#api-scylla-scylladb-com-nodeconfigs-v1alpha1-status)     | object |                                                                                                                                                                                                                                                                                                                                                                                                       |

<a id="api-scylla-scylladb-com-nodeconfigs-v1alpha1-metadata"></a>

### .metadata

#### Description

#### Type

object

<a id="api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec"></a>

### .spec

#### Description

#### Type

object

| Property                                                                            | Type           | Description                                                                                                                                                                                                                                                                                                                                               |
|-------------------------------------------------------------------------------------|----------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| disableOptimizations                                                                | boolean        | disableOptimizations controls if nodes matching placement requirements are going to be optimized for performance. Turning off optimizations on already optimized Nodes does not revert changes. See [https://operator.docs.scylladb.com/stable/architecture/tuning.html](https://operator.docs.scylladb.com/stable/architecture/tuning.html) for details. |
| [localDiskSetup](#api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-localdisksetup) | object         | localDiskSetup contains options of automatic local disk setup.                                                                                                                                                                                                                                                                                            |
| [placement](#api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement)           | object         | placement contains scheduling rules for NodeConfig Pods.                                                                                                                                                                                                                                                                                                  |
| [sysctls](#api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-sysctls)               | array (object) | sysctls specifies a list of sysctls to configure on the node. Removing parameters from this list does not revert already applied configurations.                                                                                                                                                                                                          |

<a id="api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-localdisksetup"></a>

### .spec.localDiskSetup

#### Description

localDiskSetup contains options of automatic local disk setup.

#### Type

object

| Property                                                                                     | Type           | Description                                         |
|----------------------------------------------------------------------------------------------|----------------|-----------------------------------------------------|
| [filesystems](#api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-localdisksetup-filesystems) | array (object) | filesystems is a list of filesystem configurations. |
| [loopDevices](#api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-localdisksetup-loopdevices) | array (object) | loops is a list of loop device configurations.      |
| [mounts](#api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-localdisksetup-mounts)           | array (object) | mounts is a list of mount configuration.            |
| [raids](#api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-localdisksetup-raids)             | array (object) | raids is a list of raid configurations.             |

<a id="api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-localdisksetup-filesystems"></a>

### .spec.localDiskSetup.filesystems[]

#### Description

FilesystemConfiguration specifies filesystem configuration options.

#### Type

object

| Property   | Type   | Description                                                                    |
|------------|--------|--------------------------------------------------------------------------------|
| device     | string | device is a path to the device where the desired filesystem should be created. |
| type       | string | type is a desired filesystem type.                                             |

<a id="api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-localdisksetup-loopdevices"></a>

### .spec.localDiskSetup.loopDevices[]

#### Description

LoopDeviceConfiguration specifies loop device configuration options.

#### Type

object

| Property   | Type   | Description                                                                                              |
|------------|--------|----------------------------------------------------------------------------------------------------------|
| imagePath  | string | imagePath specifies path on host where backing image file for loop device should be located.             |
| name       | string | name specifies the name of the symlink that will point to actual loop device, created under /dev/loops/. |
| size       |        | size specifies the size of the loop device.                                                              |

<a id="api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-localdisksetup-mounts"></a>

### .spec.localDiskSetup.mounts[]

#### Description

MountConfiguration specifies mount configuration options.

#### Type

object

| Property           | Type           | Description                                                                                                                                                                                   |
|--------------------|----------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| device             | string         | device is path to a device that should be mounted.                                                                                                                                            |
| fsType             | string         | fsType specifies the filesystem on the device.                                                                                                                                                |
| mountPoint         | string         | mountPoint is a path where the device should be mounted at. If the mountPoint is a symlink, the mount will be set up for the target.                                                          |
| unsupportedOptions | array (string) | unsupportedOptions is a list of mount options used during device mounting. unsupported in this field name means that we won’t support all the available options passed down using this field. |

<a id="api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-localdisksetup-raids"></a>

### .spec.localDiskSetup.raids[]

#### Description

RAIDConfiguration is a configuration of a raid array.

#### Type

object

| Property                                                                               | Type   | Description                                                                 |
|----------------------------------------------------------------------------------------|--------|-----------------------------------------------------------------------------|
| [RAID0](#api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-localdisksetup-raids-raid0) | object | RAID0 specifies RAID0 options.                                              |
| name                                                                                   | string | name specifies the name of the raid device to be created under in /dev/md/. |
| type                                                                                   | string | type is a type of raid array.                                               |

<a id="api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-localdisksetup-raids-raid0"></a>

### .spec.localDiskSetup.raids[].RAID0

#### Description

RAID0 specifies RAID0 options.

#### Type

object

| Property                                                                                         | Type   | Description                                              |
|--------------------------------------------------------------------------------------------------|--------|----------------------------------------------------------|
| [devices](#api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-localdisksetup-raids-raid0-devices) | object | devices defines which devices constitute the raid array. |

<a id="api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-localdisksetup-raids-raid0-devices"></a>

### .spec.localDiskSetup.raids[].RAID0.devices

#### Description

devices defines which devices constitute the raid array.

#### Type

object

| Property   | Type   | Description                                                               |
|------------|--------|---------------------------------------------------------------------------|
| modelRegex | string | modelRegex is a regular expression filtering devices by their model name. |
| nameRegex  | string | nameRegex is a regular expression filtering devices by their name.        |

<a id="api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement"></a>

### .spec.placement

#### Description

placement contains scheduling rules for NodeConfig Pods.

#### Type

object

| Property                                                                                  | Type           | Description                                                                                                                                                                |
|-------------------------------------------------------------------------------------------|----------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [affinity](#api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity)         | object         | affinity is a group of affinity scheduling rules for NodeConfig Pods.                                                                                                      |
| [nodeSelector](#api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-nodeselector) | object         | nodeSelector is a selector which must be true for the NodeConfig Pod to fit on a node. Selector which must match a node’s labels for the pod to be scheduled on that node. |
| [tolerations](#api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-tolerations)   | array (object) | tolerations is a group of tolerations NodeConfig Pods are going to have.                                                                                                   |

<a id="api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity"></a>

### .spec.placement.affinity

#### Description

affinity is a group of affinity scheduling rules for NodeConfig Pods.

#### Type

object

| Property                                                                                                 | Type   | Description                                                                                                                   |
|----------------------------------------------------------------------------------------------------------|--------|-------------------------------------------------------------------------------------------------------------------------------|
| [nodeAffinity](#api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-nodeaffinity)       | object | Describes node affinity scheduling rules for the pod.                                                                         |
| [podAffinity](#api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podaffinity)         | object | Describes pod affinity scheduling rules (e.g. co-locate this pod in the same node, zone, etc. as some other pod(s)).          |
| [podAntiAffinity](#api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podantiaffinity) | object | Describes pod anti-affinity scheduling rules (e.g. avoid putting this pod in the same node, zone, etc. as some other pod(s)). |

<a id="api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-nodeaffinity"></a>

### .spec.placement.affinity.nodeAffinity

#### Description

Describes node affinity scheduling rules for the pod.

#### Type

object

| Property                                                                                                                                                                              | Type           | Description                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    |
|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [preferredDuringSchedulingIgnoredDuringExecution](#api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-nodeaffinity-preferredduringschedulingignoredduringexecution) | array (object) | The scheduler will prefer to schedule pods to nodes that satisfy the affinity expressions specified by this field, but it may choose a node that violates one or more of the expressions. The node that is most preferred is the one with the greatest sum of weights, i.e. for each node that meets all of the scheduling requirements (resource request, requiredDuringScheduling affinity expressions, etc.), compute a sum by iterating through the elements of this field and adding “weight” to the sum if the node matches the corresponding matchExpressions; the node(s) with the highest sum are the most preferred. |
| [requiredDuringSchedulingIgnoredDuringExecution](#api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-nodeaffinity-requiredduringschedulingignoredduringexecution)   | object         | If the affinity requirements specified by this field are not met at scheduling time, the pod will not be scheduled onto the node. If the affinity requirements specified by this field cease to be met at some point during pod execution (e.g. due to an update), the system may or may not try to eventually evict the pod from its node.                                                                                                                                                                                                                                                                                    |

<a id="api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-nodeaffinity-preferredduringschedulingignoredduringexecution"></a>

### .spec.placement.affinity.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[]

#### Description

An empty preferred scheduling term matches all objects with implicit weight 0 (i.e. it’s a no-op). A null preferred scheduling term matches no objects (i.e. is also a no-op).

#### Type

object

| Property                                                                                                                                                    | Type    | Description                                                                             |
|-------------------------------------------------------------------------------------------------------------------------------------------------------------|---------|-----------------------------------------------------------------------------------------|
| [preference](#api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-nodeaffinity-preferredduringschedulingignoredduringexecution-preference) | object  | A node selector term, associated with the corresponding weight.                         |
| weight                                                                                                                                                      | integer | Weight associated with matching the corresponding nodeSelectorTerm, in the range 1-100. |

<a id="api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-nodeaffinity-preferredduringschedulingignoredduringexecution-preference"></a>

### .spec.placement.affinity.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[].preference

#### Description

A node selector term, associated with the corresponding weight.

#### Type

object

| Property                                                                                                                                                                           | Type           | Description                                            |
|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------|--------------------------------------------------------|
| [matchExpressions](#api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-nodeaffinity-preferredduringschedulingignoredduringexecution-preference-matchexpressions) | array (object) | A list of node selector requirements by node’s labels. |
| [matchFields](#api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-nodeaffinity-preferredduringschedulingignoredduringexecution-preference-matchfields)           | array (object) | A list of node selector requirements by node’s fields. |

<a id="api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-nodeaffinity-preferredduringschedulingignoredduringexecution-preference-matchexpressions"></a>

### .spec.placement.affinity.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[].preference.matchExpressions[]

#### Description

A node selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

#### Type

object

| Property   | Type           | Description                                                                                                                                                                                                                                                                                                                                         |
|------------|----------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| key        | string         | The label key that the selector applies to.                                                                                                                                                                                                                                                                                                         |
| operator   | string         | Represents a key’s relationship to a set of values. Valid operators are In, NotIn, Exists, DoesNotExist. Gt, and Lt.                                                                                                                                                                                                                                |
| values     | array (string) | An array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. If the operator is Gt or Lt, the values array must have a single element, which will be interpreted as an integer. This array is replaced during a strategic merge patch. |

<a id="api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-nodeaffinity-preferredduringschedulingignoredduringexecution-preference-matchfields"></a>

### .spec.placement.affinity.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[].preference.matchFields[]

#### Description

A node selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

#### Type

object

| Property   | Type           | Description                                                                                                                                                                                                                                                                                                                                         |
|------------|----------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| key        | string         | The label key that the selector applies to.                                                                                                                                                                                                                                                                                                         |
| operator   | string         | Represents a key’s relationship to a set of values. Valid operators are In, NotIn, Exists, DoesNotExist. Gt, and Lt.                                                                                                                                                                                                                                |
| values     | array (string) | An array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. If the operator is Gt or Lt, the values array must have a single element, which will be interpreted as an integer. This array is replaced during a strategic merge patch. |

<a id="api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-nodeaffinity-requiredduringschedulingignoredduringexecution"></a>

### .spec.placement.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution

#### Description

If the affinity requirements specified by this field are not met at scheduling time, the pod will not be scheduled onto the node. If the affinity requirements specified by this field cease to be met at some point during pod execution (e.g. due to an update), the system may or may not try to eventually evict the pod from its node.

#### Type

object

| Property                                                                                                                                                                 | Type           | Description                                                  |
|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------|--------------------------------------------------------------|
| [nodeSelectorTerms](#api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-nodeaffinity-requiredduringschedulingignoredduringexecution-nodeselectorterms) | array (object) | Required. A list of node selector terms. The terms are ORed. |

<a id="api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-nodeaffinity-requiredduringschedulingignoredduringexecution-nodeselectorterms"></a>

### .spec.placement.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[]

#### Description

A null or empty node selector term matches no objects. The requirements of them are ANDed. The TopologySelectorTerm type implements a subset of the NodeSelectorTerm.

#### Type

object

| Property                                                                                                                                                                                 | Type           | Description                                            |
|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------|--------------------------------------------------------|
| [matchExpressions](#api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-nodeaffinity-requiredduringschedulingignoredduringexecution-nodeselectorterms-matchexpressions) | array (object) | A list of node selector requirements by node’s labels. |
| [matchFields](#api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-nodeaffinity-requiredduringschedulingignoredduringexecution-nodeselectorterms-matchfields)           | array (object) | A list of node selector requirements by node’s fields. |

<a id="api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-nodeaffinity-requiredduringschedulingignoredduringexecution-nodeselectorterms-matchexpressions"></a>

### .spec.placement.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[].matchExpressions[]

#### Description

A node selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

#### Type

object

| Property   | Type           | Description                                                                                                                                                                                                                                                                                                                                         |
|------------|----------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| key        | string         | The label key that the selector applies to.                                                                                                                                                                                                                                                                                                         |
| operator   | string         | Represents a key’s relationship to a set of values. Valid operators are In, NotIn, Exists, DoesNotExist. Gt, and Lt.                                                                                                                                                                                                                                |
| values     | array (string) | An array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. If the operator is Gt or Lt, the values array must have a single element, which will be interpreted as an integer. This array is replaced during a strategic merge patch. |

<a id="api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-nodeaffinity-requiredduringschedulingignoredduringexecution-nodeselectorterms-matchfields"></a>

### .spec.placement.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[].matchFields[]

#### Description

A node selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

#### Type

object

| Property   | Type           | Description                                                                                                                                                                                                                                                                                                                                         |
|------------|----------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| key        | string         | The label key that the selector applies to.                                                                                                                                                                                                                                                                                                         |
| operator   | string         | Represents a key’s relationship to a set of values. Valid operators are In, NotIn, Exists, DoesNotExist. Gt, and Lt.                                                                                                                                                                                                                                |
| values     | array (string) | An array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. If the operator is Gt or Lt, the values array must have a single element, which will be interpreted as an integer. This array is replaced during a strategic merge patch. |

<a id="api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podaffinity"></a>

### .spec.placement.affinity.podAffinity

#### Description

Describes pod affinity scheduling rules (e.g. co-locate this pod in the same node, zone, etc. as some other pod(s)).

#### Type

object

| Property                                                                                                                                                                             | Type           | Description                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  |
|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [preferredDuringSchedulingIgnoredDuringExecution](#api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podaffinity-preferredduringschedulingignoredduringexecution) | array (object) | The scheduler will prefer to schedule pods to nodes that satisfy the affinity expressions specified by this field, but it may choose a node that violates one or more of the expressions. The node that is most preferred is the one with the greatest sum of weights, i.e. for each node that meets all of the scheduling requirements (resource request, requiredDuringScheduling affinity expressions, etc.), compute a sum by iterating through the elements of this field and adding “weight” to the sum if the node has pods which matches the corresponding podAffinityTerm; the node(s) with the highest sum are the most preferred. |
| [requiredDuringSchedulingIgnoredDuringExecution](#api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podaffinity-requiredduringschedulingignoredduringexecution)   | array (object) | If the affinity requirements specified by this field are not met at scheduling time, the pod will not be scheduled onto the node. If the affinity requirements specified by this field cease to be met at some point during pod execution (e.g. due to a pod label update), the system may or may not try to eventually evict the pod from its node. When there are multiple elements, the lists of nodes corresponding to each podAffinityTerm are intersected, i.e. all terms must be satisfied.                                                                                                                                           |

<a id="api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podaffinity-preferredduringschedulingignoredduringexecution"></a>

### .spec.placement.affinity.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[]

#### Description

The weights of all of the matched WeightedPodAffinityTerm fields are added per-node to find the most preferred node(s)

#### Type

object

| Property                                                                                                                                                             | Type    | Description                                                                            |
|----------------------------------------------------------------------------------------------------------------------------------------------------------------------|---------|----------------------------------------------------------------------------------------|
| [podAffinityTerm](#api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm) | object  | Required. A pod affinity term, associated with the corresponding weight.               |
| weight                                                                                                                                                               | integer | weight associated with matching the corresponding podAffinityTerm, in the range 1-100. |

<a id="api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm"></a>

### .spec.placement.affinity.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm

#### Description

Required. A pod affinity term, associated with the corresponding weight.

#### Type

object

| Property                                                                                                                                                                                 | Type           | Description                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [labelSelector](#api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-labelselector)         | object         | A label query over a set of resources, in this case pods. If it’s null, this PodAffinityTerm matches with no Pods.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| matchLabelKeys                                                                                                                                                                           | array (string) | MatchLabelKeys is a set of pod label keys to select which pods will be taken into consideration. The keys are used to lookup values from the incoming pod labels, those key-value labels are merged with labelSelector as key in (value) to select the group of existing pods which pods will be taken into consideration for the incoming pod’s pod (anti) affinity. Keys that don’t exist in the incoming pod labels will be ignored. The default value is empty. The same key is forbidden to exist in both matchLabelKeys and labelSelector. Also, matchLabelKeys cannot be set when labelSelector isn’t set.             |
| mismatchLabelKeys                                                                                                                                                                        | array (string) | MismatchLabelKeys is a set of pod label keys to select which pods will be taken into consideration. The keys are used to lookup values from the incoming pod labels, those key-value labels are merged with labelSelector as key notin (value) to select the group of existing pods which pods will be taken into consideration for the incoming pod’s pod (anti) affinity. Keys that don’t exist in the incoming pod labels will be ignored. The default value is empty. The same key is forbidden to exist in both mismatchLabelKeys and labelSelector. Also, mismatchLabelKeys cannot be set when labelSelector isn’t set. |
| [namespaceSelector](#api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-namespaceselector) | object         | A label query over the set of namespaces that the term applies to. The term is applied to the union of the namespaces selected by this field and the ones listed in the namespaces field. null selector and null or empty namespaces list means “this pod’s namespace”. An empty selector ({}) matches all namespaces.                                                                                                                                                                                                                                                                                                        |
| namespaces                                                                                                                                                                               | array (string) | namespaces specifies a static list of namespace names that the term applies to. The term is applied to the union of the namespaces listed in this field and the ones selected by namespaceSelector. null or empty namespaces list and null namespaceSelector means “this pod’s namespace”.                                                                                                                                                                                                                                                                                                                                    |
| topologyKey                                                                                                                                                                              | string         | This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching the labelSelector in the specified namespaces, where co-located is defined as running on a node whose value of the label with key topologyKey matches that of any node on which any of the selected pods is running. Empty topologyKey is not allowed.                                                                                                                                                                                                                                                                      |

<a id="api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-labelselector"></a>

### .spec.placement.affinity.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector

#### Description

A label query over a set of resources, in this case pods. If it’s null, this PodAffinityTerm matches with no Pods.

#### Type

object

| Property                                                                                                                                                                                             | Type           | Description                                                                                                                                                                                                                                                     |
|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [matchExpressions](#api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-labelselector-matchexpressions) | array (object) | matchExpressions is a list of label selector requirements. The requirements are ANDed.                                                                                                                                                                          |
| [matchLabels](#api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-labelselector-matchlabels)           | object         | matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is “key”, the operator is “In”, and the values array contains only “value”. The requirements are ANDed. |

<a id="api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-labelselector-matchexpressions"></a>

### .spec.placement.affinity.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchExpressions[]

#### Description

A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

#### Type

object

| Property   | Type           | Description                                                                                                                                                                                                                                |
|------------|----------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| key        | string         | key is the label key that the selector applies to.                                                                                                                                                                                         |
| operator   | string         | operator represents a key’s relationship to a set of values. Valid operators are In, NotIn, Exists and DoesNotExist.                                                                                                                       |
| values     | array (string) | values is an array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch. |

<a id="api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-labelselector-matchlabels"></a>

### .spec.placement.affinity.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchLabels

#### Description

matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is “key”, the operator is “In”, and the values array contains only “value”. The requirements are ANDed.

#### Type

object

<a id="api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-namespaceselector"></a>

### .spec.placement.affinity.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector

#### Description

A label query over the set of namespaces that the term applies to. The term is applied to the union of the namespaces selected by this field and the ones listed in the namespaces field. null selector and null or empty namespaces list means “this pod’s namespace”. An empty selector ({}) matches all namespaces.

#### Type

object

| Property                                                                                                                                                                                                 | Type           | Description                                                                                                                                                                                                                                                     |
|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [matchExpressions](#api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-namespaceselector-matchexpressions) | array (object) | matchExpressions is a list of label selector requirements. The requirements are ANDed.                                                                                                                                                                          |
| [matchLabels](#api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-namespaceselector-matchlabels)           | object         | matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is “key”, the operator is “In”, and the values array contains only “value”. The requirements are ANDed. |

<a id="api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-namespaceselector-matchexpressions"></a>

### .spec.placement.affinity.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchExpressions[]

#### Description

A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

#### Type

object

| Property   | Type           | Description                                                                                                                                                                                                                                |
|------------|----------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| key        | string         | key is the label key that the selector applies to.                                                                                                                                                                                         |
| operator   | string         | operator represents a key’s relationship to a set of values. Valid operators are In, NotIn, Exists and DoesNotExist.                                                                                                                       |
| values     | array (string) | values is an array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch. |

<a id="api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-namespaceselector-matchlabels"></a>

### .spec.placement.affinity.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchLabels

#### Description

matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is “key”, the operator is “In”, and the values array contains only “value”. The requirements are ANDed.

#### Type

object

<a id="api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podaffinity-requiredduringschedulingignoredduringexecution"></a>

### .spec.placement.affinity.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[]

#### Description

Defines a set of pods (namely those matching the labelSelector relative to the given namespace(s)) that this pod should be co-located (affinity) or not co-located (anti-affinity) with, where co-located is defined as running on a node whose value of the label with key <topologyKey> matches that of any node on which a pod of the set of pods is running

#### Type

object

| Property                                                                                                                                                                | Type           | Description                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [labelSelector](#api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podaffinity-requiredduringschedulingignoredduringexecution-labelselector)         | object         | A label query over a set of resources, in this case pods. If it’s null, this PodAffinityTerm matches with no Pods.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| matchLabelKeys                                                                                                                                                          | array (string) | MatchLabelKeys is a set of pod label keys to select which pods will be taken into consideration. The keys are used to lookup values from the incoming pod labels, those key-value labels are merged with labelSelector as key in (value) to select the group of existing pods which pods will be taken into consideration for the incoming pod’s pod (anti) affinity. Keys that don’t exist in the incoming pod labels will be ignored. The default value is empty. The same key is forbidden to exist in both matchLabelKeys and labelSelector. Also, matchLabelKeys cannot be set when labelSelector isn’t set.             |
| mismatchLabelKeys                                                                                                                                                       | array (string) | MismatchLabelKeys is a set of pod label keys to select which pods will be taken into consideration. The keys are used to lookup values from the incoming pod labels, those key-value labels are merged with labelSelector as key notin (value) to select the group of existing pods which pods will be taken into consideration for the incoming pod’s pod (anti) affinity. Keys that don’t exist in the incoming pod labels will be ignored. The default value is empty. The same key is forbidden to exist in both mismatchLabelKeys and labelSelector. Also, mismatchLabelKeys cannot be set when labelSelector isn’t set. |
| [namespaceSelector](#api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podaffinity-requiredduringschedulingignoredduringexecution-namespaceselector) | object         | A label query over the set of namespaces that the term applies to. The term is applied to the union of the namespaces selected by this field and the ones listed in the namespaces field. null selector and null or empty namespaces list means “this pod’s namespace”. An empty selector ({}) matches all namespaces.                                                                                                                                                                                                                                                                                                        |
| namespaces                                                                                                                                                              | array (string) | namespaces specifies a static list of namespace names that the term applies to. The term is applied to the union of the namespaces listed in this field and the ones selected by namespaceSelector. null or empty namespaces list and null namespaceSelector means “this pod’s namespace”.                                                                                                                                                                                                                                                                                                                                    |
| topologyKey                                                                                                                                                             | string         | This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching the labelSelector in the specified namespaces, where co-located is defined as running on a node whose value of the label with key topologyKey matches that of any node on which any of the selected pods is running. Empty topologyKey is not allowed.                                                                                                                                                                                                                                                                      |

<a id="api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podaffinity-requiredduringschedulingignoredduringexecution-labelselector"></a>

### .spec.placement.affinity.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector

#### Description

A label query over a set of resources, in this case pods. If it’s null, this PodAffinityTerm matches with no Pods.

#### Type

object

| Property                                                                                                                                                                            | Type           | Description                                                                                                                                                                                                                                                     |
|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [matchExpressions](#api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podaffinity-requiredduringschedulingignoredduringexecution-labelselector-matchexpressions) | array (object) | matchExpressions is a list of label selector requirements. The requirements are ANDed.                                                                                                                                                                          |
| [matchLabels](#api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podaffinity-requiredduringschedulingignoredduringexecution-labelselector-matchlabels)           | object         | matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is “key”, the operator is “In”, and the values array contains only “value”. The requirements are ANDed. |

<a id="api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podaffinity-requiredduringschedulingignoredduringexecution-labelselector-matchexpressions"></a>

### .spec.placement.affinity.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchExpressions[]

#### Description

A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

#### Type

object

| Property   | Type           | Description                                                                                                                                                                                                                                |
|------------|----------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| key        | string         | key is the label key that the selector applies to.                                                                                                                                                                                         |
| operator   | string         | operator represents a key’s relationship to a set of values. Valid operators are In, NotIn, Exists and DoesNotExist.                                                                                                                       |
| values     | array (string) | values is an array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch. |

<a id="api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podaffinity-requiredduringschedulingignoredduringexecution-labelselector-matchlabels"></a>

### .spec.placement.affinity.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchLabels

#### Description

matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is “key”, the operator is “In”, and the values array contains only “value”. The requirements are ANDed.

#### Type

object

<a id="api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podaffinity-requiredduringschedulingignoredduringexecution-namespaceselector"></a>

### .spec.placement.affinity.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector

#### Description

A label query over the set of namespaces that the term applies to. The term is applied to the union of the namespaces selected by this field and the ones listed in the namespaces field. null selector and null or empty namespaces list means “this pod’s namespace”. An empty selector ({}) matches all namespaces.

#### Type

object

| Property                                                                                                                                                                                | Type           | Description                                                                                                                                                                                                                                                     |
|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [matchExpressions](#api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podaffinity-requiredduringschedulingignoredduringexecution-namespaceselector-matchexpressions) | array (object) | matchExpressions is a list of label selector requirements. The requirements are ANDed.                                                                                                                                                                          |
| [matchLabels](#api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podaffinity-requiredduringschedulingignoredduringexecution-namespaceselector-matchlabels)           | object         | matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is “key”, the operator is “In”, and the values array contains only “value”. The requirements are ANDed. |

<a id="api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podaffinity-requiredduringschedulingignoredduringexecution-namespaceselector-matchexpressions"></a>

### .spec.placement.affinity.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchExpressions[]

#### Description

A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

#### Type

object

| Property   | Type           | Description                                                                                                                                                                                                                                |
|------------|----------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| key        | string         | key is the label key that the selector applies to.                                                                                                                                                                                         |
| operator   | string         | operator represents a key’s relationship to a set of values. Valid operators are In, NotIn, Exists and DoesNotExist.                                                                                                                       |
| values     | array (string) | values is an array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch. |

<a id="api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podaffinity-requiredduringschedulingignoredduringexecution-namespaceselector-matchlabels"></a>

### .spec.placement.affinity.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchLabels

#### Description

matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is “key”, the operator is “In”, and the values array contains only “value”. The requirements are ANDed.

#### Type

object

<a id="api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podantiaffinity"></a>

### .spec.placement.affinity.podAntiAffinity

#### Description

Describes pod anti-affinity scheduling rules (e.g. avoid putting this pod in the same node, zone, etc. as some other pod(s)).

#### Type

object

| Property                                                                                                                                                                                 | Type           | Description                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [preferredDuringSchedulingIgnoredDuringExecution](#api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podantiaffinity-preferredduringschedulingignoredduringexecution) | array (object) | The scheduler will prefer to schedule pods to nodes that satisfy the anti-affinity expressions specified by this field, but it may choose a node that violates one or more of the expressions. The node that is most preferred is the one with the greatest sum of weights, i.e. for each node that meets all of the scheduling requirements (resource request, requiredDuringScheduling anti-affinity expressions, etc.), compute a sum by iterating through the elements of this field and subtracting “weight” from the sum if the node has pods which matches the corresponding podAffinityTerm; the node(s) with the highest sum are the most preferred. |
| [requiredDuringSchedulingIgnoredDuringExecution](#api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podantiaffinity-requiredduringschedulingignoredduringexecution)   | array (object) | If the anti-affinity requirements specified by this field are not met at scheduling time, the pod will not be scheduled onto the node. If the anti-affinity requirements specified by this field cease to be met at some point during pod execution (e.g. due to a pod label update), the system may or may not try to eventually evict the pod from its node. When there are multiple elements, the lists of nodes corresponding to each podAffinityTerm are intersected, i.e. all terms must be satisfied.                                                                                                                                                  |

<a id="api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podantiaffinity-preferredduringschedulingignoredduringexecution"></a>

### .spec.placement.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[]

#### Description

The weights of all of the matched WeightedPodAffinityTerm fields are added per-node to find the most preferred node(s)

#### Type

object

| Property                                                                                                                                                                 | Type    | Description                                                                            |
|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------|---------|----------------------------------------------------------------------------------------|
| [podAffinityTerm](#api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podantiaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm) | object  | Required. A pod affinity term, associated with the corresponding weight.               |
| weight                                                                                                                                                                   | integer | weight associated with matching the corresponding podAffinityTerm, in the range 1-100. |

<a id="api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podantiaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm"></a>

### .spec.placement.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm

#### Description

Required. A pod affinity term, associated with the corresponding weight.

#### Type

object

| Property                                                                                                                                                                                     | Type           | Description                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [labelSelector](#api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podantiaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-labelselector)         | object         | A label query over a set of resources, in this case pods. If it’s null, this PodAffinityTerm matches with no Pods.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| matchLabelKeys                                                                                                                                                                               | array (string) | MatchLabelKeys is a set of pod label keys to select which pods will be taken into consideration. The keys are used to lookup values from the incoming pod labels, those key-value labels are merged with labelSelector as key in (value) to select the group of existing pods which pods will be taken into consideration for the incoming pod’s pod (anti) affinity. Keys that don’t exist in the incoming pod labels will be ignored. The default value is empty. The same key is forbidden to exist in both matchLabelKeys and labelSelector. Also, matchLabelKeys cannot be set when labelSelector isn’t set.             |
| mismatchLabelKeys                                                                                                                                                                            | array (string) | MismatchLabelKeys is a set of pod label keys to select which pods will be taken into consideration. The keys are used to lookup values from the incoming pod labels, those key-value labels are merged with labelSelector as key notin (value) to select the group of existing pods which pods will be taken into consideration for the incoming pod’s pod (anti) affinity. Keys that don’t exist in the incoming pod labels will be ignored. The default value is empty. The same key is forbidden to exist in both mismatchLabelKeys and labelSelector. Also, mismatchLabelKeys cannot be set when labelSelector isn’t set. |
| [namespaceSelector](#api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podantiaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-namespaceselector) | object         | A label query over the set of namespaces that the term applies to. The term is applied to the union of the namespaces selected by this field and the ones listed in the namespaces field. null selector and null or empty namespaces list means “this pod’s namespace”. An empty selector ({}) matches all namespaces.                                                                                                                                                                                                                                                                                                        |
| namespaces                                                                                                                                                                                   | array (string) | namespaces specifies a static list of namespace names that the term applies to. The term is applied to the union of the namespaces listed in this field and the ones selected by namespaceSelector. null or empty namespaces list and null namespaceSelector means “this pod’s namespace”.                                                                                                                                                                                                                                                                                                                                    |
| topologyKey                                                                                                                                                                                  | string         | This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching the labelSelector in the specified namespaces, where co-located is defined as running on a node whose value of the label with key topologyKey matches that of any node on which any of the selected pods is running. Empty topologyKey is not allowed.                                                                                                                                                                                                                                                                      |

<a id="api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podantiaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-labelselector"></a>

### .spec.placement.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector

#### Description

A label query over a set of resources, in this case pods. If it’s null, this PodAffinityTerm matches with no Pods.

#### Type

object

| Property                                                                                                                                                                                                 | Type           | Description                                                                                                                                                                                                                                                     |
|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [matchExpressions](#api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podantiaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-labelselector-matchexpressions) | array (object) | matchExpressions is a list of label selector requirements. The requirements are ANDed.                                                                                                                                                                          |
| [matchLabels](#api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podantiaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-labelselector-matchlabels)           | object         | matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is “key”, the operator is “In”, and the values array contains only “value”. The requirements are ANDed. |

<a id="api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podantiaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-labelselector-matchexpressions"></a>

### .spec.placement.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchExpressions[]

#### Description

A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

#### Type

object

| Property   | Type           | Description                                                                                                                                                                                                                                |
|------------|----------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| key        | string         | key is the label key that the selector applies to.                                                                                                                                                                                         |
| operator   | string         | operator represents a key’s relationship to a set of values. Valid operators are In, NotIn, Exists and DoesNotExist.                                                                                                                       |
| values     | array (string) | values is an array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch. |

<a id="api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podantiaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-labelselector-matchlabels"></a>

### .spec.placement.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchLabels

#### Description

matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is “key”, the operator is “In”, and the values array contains only “value”. The requirements are ANDed.

#### Type

object

<a id="api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podantiaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-namespaceselector"></a>

### .spec.placement.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector

#### Description

A label query over the set of namespaces that the term applies to. The term is applied to the union of the namespaces selected by this field and the ones listed in the namespaces field. null selector and null or empty namespaces list means “this pod’s namespace”. An empty selector ({}) matches all namespaces.

#### Type

object

| Property                                                                                                                                                                                                     | Type           | Description                                                                                                                                                                                                                                                     |
|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [matchExpressions](#api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podantiaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-namespaceselector-matchexpressions) | array (object) | matchExpressions is a list of label selector requirements. The requirements are ANDed.                                                                                                                                                                          |
| [matchLabels](#api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podantiaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-namespaceselector-matchlabels)           | object         | matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is “key”, the operator is “In”, and the values array contains only “value”. The requirements are ANDed. |

<a id="api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podantiaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-namespaceselector-matchexpressions"></a>

### .spec.placement.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchExpressions[]

#### Description

A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

#### Type

object

| Property   | Type           | Description                                                                                                                                                                                                                                |
|------------|----------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| key        | string         | key is the label key that the selector applies to.                                                                                                                                                                                         |
| operator   | string         | operator represents a key’s relationship to a set of values. Valid operators are In, NotIn, Exists and DoesNotExist.                                                                                                                       |
| values     | array (string) | values is an array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch. |

<a id="api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podantiaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-namespaceselector-matchlabels"></a>

### .spec.placement.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchLabels

#### Description

matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is “key”, the operator is “In”, and the values array contains only “value”. The requirements are ANDed.

#### Type

object

<a id="api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podantiaffinity-requiredduringschedulingignoredduringexecution"></a>

### .spec.placement.affinity.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[]

#### Description

Defines a set of pods (namely those matching the labelSelector relative to the given namespace(s)) that this pod should be co-located (affinity) or not co-located (anti-affinity) with, where co-located is defined as running on a node whose value of the label with key <topologyKey> matches that of any node on which a pod of the set of pods is running

#### Type

object

| Property                                                                                                                                                                    | Type           | Description                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [labelSelector](#api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podantiaffinity-requiredduringschedulingignoredduringexecution-labelselector)         | object         | A label query over a set of resources, in this case pods. If it’s null, this PodAffinityTerm matches with no Pods.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| matchLabelKeys                                                                                                                                                              | array (string) | MatchLabelKeys is a set of pod label keys to select which pods will be taken into consideration. The keys are used to lookup values from the incoming pod labels, those key-value labels are merged with labelSelector as key in (value) to select the group of existing pods which pods will be taken into consideration for the incoming pod’s pod (anti) affinity. Keys that don’t exist in the incoming pod labels will be ignored. The default value is empty. The same key is forbidden to exist in both matchLabelKeys and labelSelector. Also, matchLabelKeys cannot be set when labelSelector isn’t set.             |
| mismatchLabelKeys                                                                                                                                                           | array (string) | MismatchLabelKeys is a set of pod label keys to select which pods will be taken into consideration. The keys are used to lookup values from the incoming pod labels, those key-value labels are merged with labelSelector as key notin (value) to select the group of existing pods which pods will be taken into consideration for the incoming pod’s pod (anti) affinity. Keys that don’t exist in the incoming pod labels will be ignored. The default value is empty. The same key is forbidden to exist in both mismatchLabelKeys and labelSelector. Also, mismatchLabelKeys cannot be set when labelSelector isn’t set. |
| [namespaceSelector](#api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podantiaffinity-requiredduringschedulingignoredduringexecution-namespaceselector) | object         | A label query over the set of namespaces that the term applies to. The term is applied to the union of the namespaces selected by this field and the ones listed in the namespaces field. null selector and null or empty namespaces list means “this pod’s namespace”. An empty selector ({}) matches all namespaces.                                                                                                                                                                                                                                                                                                        |
| namespaces                                                                                                                                                                  | array (string) | namespaces specifies a static list of namespace names that the term applies to. The term is applied to the union of the namespaces listed in this field and the ones selected by namespaceSelector. null or empty namespaces list and null namespaceSelector means “this pod’s namespace”.                                                                                                                                                                                                                                                                                                                                    |
| topologyKey                                                                                                                                                                 | string         | This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching the labelSelector in the specified namespaces, where co-located is defined as running on a node whose value of the label with key topologyKey matches that of any node on which any of the selected pods is running. Empty topologyKey is not allowed.                                                                                                                                                                                                                                                                      |

<a id="api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podantiaffinity-requiredduringschedulingignoredduringexecution-labelselector"></a>

### .spec.placement.affinity.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector

#### Description

A label query over a set of resources, in this case pods. If it’s null, this PodAffinityTerm matches with no Pods.

#### Type

object

| Property                                                                                                                                                                                | Type           | Description                                                                                                                                                                                                                                                     |
|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [matchExpressions](#api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podantiaffinity-requiredduringschedulingignoredduringexecution-labelselector-matchexpressions) | array (object) | matchExpressions is a list of label selector requirements. The requirements are ANDed.                                                                                                                                                                          |
| [matchLabels](#api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podantiaffinity-requiredduringschedulingignoredduringexecution-labelselector-matchlabels)           | object         | matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is “key”, the operator is “In”, and the values array contains only “value”. The requirements are ANDed. |

<a id="api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podantiaffinity-requiredduringschedulingignoredduringexecution-labelselector-matchexpressions"></a>

### .spec.placement.affinity.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchExpressions[]

#### Description

A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

#### Type

object

| Property   | Type           | Description                                                                                                                                                                                                                                |
|------------|----------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| key        | string         | key is the label key that the selector applies to.                                                                                                                                                                                         |
| operator   | string         | operator represents a key’s relationship to a set of values. Valid operators are In, NotIn, Exists and DoesNotExist.                                                                                                                       |
| values     | array (string) | values is an array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch. |

<a id="api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podantiaffinity-requiredduringschedulingignoredduringexecution-labelselector-matchlabels"></a>

### .spec.placement.affinity.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchLabels

#### Description

matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is “key”, the operator is “In”, and the values array contains only “value”. The requirements are ANDed.

#### Type

object

<a id="api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podantiaffinity-requiredduringschedulingignoredduringexecution-namespaceselector"></a>

### .spec.placement.affinity.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector

#### Description

A label query over the set of namespaces that the term applies to. The term is applied to the union of the namespaces selected by this field and the ones listed in the namespaces field. null selector and null or empty namespaces list means “this pod’s namespace”. An empty selector ({}) matches all namespaces.

#### Type

object

| Property                                                                                                                                                                                    | Type           | Description                                                                                                                                                                                                                                                     |
|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [matchExpressions](#api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podantiaffinity-requiredduringschedulingignoredduringexecution-namespaceselector-matchexpressions) | array (object) | matchExpressions is a list of label selector requirements. The requirements are ANDed.                                                                                                                                                                          |
| [matchLabels](#api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podantiaffinity-requiredduringschedulingignoredduringexecution-namespaceselector-matchlabels)           | object         | matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is “key”, the operator is “In”, and the values array contains only “value”. The requirements are ANDed. |

<a id="api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podantiaffinity-requiredduringschedulingignoredduringexecution-namespaceselector-matchexpressions"></a>

### .spec.placement.affinity.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchExpressions[]

#### Description

A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

#### Type

object

| Property   | Type           | Description                                                                                                                                                                                                                                |
|------------|----------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| key        | string         | key is the label key that the selector applies to.                                                                                                                                                                                         |
| operator   | string         | operator represents a key’s relationship to a set of values. Valid operators are In, NotIn, Exists and DoesNotExist.                                                                                                                       |
| values     | array (string) | values is an array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch. |

<a id="api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-affinity-podantiaffinity-requiredduringschedulingignoredduringexecution-namespaceselector-matchlabels"></a>

### .spec.placement.affinity.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchLabels

#### Description

matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is “key”, the operator is “In”, and the values array contains only “value”. The requirements are ANDed.

#### Type

object

<a id="api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-nodeselector"></a>

### .spec.placement.nodeSelector

#### Description

nodeSelector is a selector which must be true for the NodeConfig Pod to fit on a node. Selector which must match a node’s labels for the pod to be scheduled on that node.

#### Type

object

<a id="api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-placement-tolerations"></a>

### .spec.placement.tolerations[]

#### Description

The pod this Toleration is attached to tolerates any taint that matches the triple <key,value,effect> using the matching operator <operator>.

#### Type

object

| Property          | Type    | Description                                                                                                                                                                                                                                                                                                                            |
|-------------------|---------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| effect            | string  | Effect indicates the taint effect to match. Empty means match all taint effects. When specified, allowed values are NoSchedule, PreferNoSchedule and NoExecute.                                                                                                                                                                        |
| key               | string  | Key is the taint key that the toleration applies to. Empty means match all taint keys. If the key is empty, operator must be Exists; this combination means to match all values and all keys.                                                                                                                                          |
| operator          | string  | Operator represents a key’s relationship to the value. Valid operators are Exists, Equal, Lt, and Gt. Defaults to Equal. Exists is equivalent to wildcard for value, so that a pod can tolerate all taints of a particular category. Lt and Gt perform numeric comparisons (requires feature gate TaintTolerationComparisonOperators). |
| tolerationSeconds | integer | TolerationSeconds represents the period of time the toleration (which must be of effect NoExecute, otherwise this field is ignored) tolerates the taint. By default, it is not set, which means tolerate the taint forever (do not evict). Zero and negative values will be treated as 0 (evict immediately) by the system.            |
| value             | string  | Value is the taint value the toleration matches to. If the operator is Exists, the value should be empty, otherwise just a regular string.                                                                                                                                                                                             |

<a id="api-scylla-scylladb-com-nodeconfigs-v1alpha1-spec-sysctls"></a>

### .spec.sysctls[]

#### Description

Sysctl defines a kernel parameter to be set

#### Type

object

| Property   | Type   | Description                |
|------------|--------|----------------------------|
| name       | string | Name of a property to set  |
| value      | string | Value of a property to set |

<a id="api-scylla-scylladb-com-nodeconfigs-v1alpha1-status"></a>

### .status

#### Description

#### Type

object

| Property                                                                          | Type           | Description                                                                         |
|-----------------------------------------------------------------------------------|----------------|-------------------------------------------------------------------------------------|
| [conditions](#api-scylla-scylladb-com-nodeconfigs-v1alpha1-status-conditions)     | array (object) | conditions represents the latest available observations of current state.           |
| [nodeStatuses](#api-scylla-scylladb-com-nodeconfigs-v1alpha1-status-nodestatuses) | array (object) | nodeStatuses hold the status for each tuned node.                                   |
| observedGeneration                                                                | integer        | observedGeneration indicates the most recent generation observed by the controller. |

<a id="api-scylla-scylladb-com-nodeconfigs-v1alpha1-status-conditions"></a>

### .status.conditions[]

#### Description

#### Type

object

| Property           | Type    | Description                                                                                                                                                                                                                                                                                 |
|--------------------|---------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| lastTransitionTime | string  | lastTransitionTime is last time the condition transitioned from one status to another.                                                                                                                                                                                                      |
| message            | string  | message is a human-readable message indicating details about the transition.                                                                                                                                                                                                                |
| observedGeneration | integer | observedGeneration represents the .metadata.generation that the condition was set based upon. For instance, if .metadata.generation is currently 12, but the .status.conditions[x].observedGeneration is 9, the condition is out of date with respect to the current state of the instance. |
| reason             | string  | reason is the reason for condition’s last transition.                                                                                                                                                                                                                                       |
| status             | string  | status represents the state of the condition, one of True, False, or Unknown.                                                                                                                                                                                                               |
| type               | string  | type is the type of the NodeConfig condition.                                                                                                                                                                                                                                               |

<a id="api-scylla-scylladb-com-nodeconfigs-v1alpha1-status-nodestatuses"></a>

### .status.nodeStatuses[]

#### Description

#### Type

object

| Property        | Type           | Description   |
|-----------------|----------------|---------------|
| name            | string         |               |
| tunedContainers | array (string) |               |
| tunedNode       | boolean        |               |
