# RemoteKubernetesCluster (scylla.scylladb.com/v1alpha1)

**APIVersion**: scylla.scylladb.com/v1alpha1
<br/>
**Kind**: RemoteKubernetesCluster
<br/>
**PluralName**: remotekubernetesclusters
<br/>
**SingularName**: remotekubernetescluster
<br/>
**Scope**: Cluster
<br/>
**ListKind**: RemoteKubernetesClusterList
<br/>
**Served**: true
<br/>
**Storage**: true
<br/>

## Description

## Specification

| Property                                                                        | Type   | Description                                                                                                                                                                                                                                                                                                                                                                                           |
|---------------------------------------------------------------------------------|--------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| apiVersion                                                                      | string | APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: [https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources](https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources)    |
| kind                                                                            | string | Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: [https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds](https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds) |
| [metadata](#api-scylla-scylladb-com-remotekubernetesclusters-v1alpha1-metadata) | object |                                                                                                                                                                                                                                                                                                                                                                                                       |
| [spec](#api-scylla-scylladb-com-remotekubernetesclusters-v1alpha1-spec)         | object | spec defines the desired state of the RemoteKubernetesCluster.                                                                                                                                                                                                                                                                                                                                        |
| [status](#api-scylla-scylladb-com-remotekubernetesclusters-v1alpha1-status)     | object | status defines the observed state of the RemoteKubernetesCluster.                                                                                                                                                                                                                                                                                                                                     |

<a id="api-scylla-scylladb-com-remotekubernetesclusters-v1alpha1-metadata"></a>

### .metadata

#### Description

#### Type

object

<a id="api-scylla-scylladb-com-remotekubernetesclusters-v1alpha1-spec"></a>

### .spec

#### Description

spec defines the desired state of the RemoteKubernetesCluster.

#### Type

object

| Property                                                                                                           | Type   | Description                                                                                                         |
|--------------------------------------------------------------------------------------------------------------------|--------|---------------------------------------------------------------------------------------------------------------------|
| [clientHealthcheckProbes](#api-scylla-scylladb-com-remotekubernetesclusters-v1alpha1-spec-clienthealthcheckprobes) | object | healthcheckProbes hold client healthcheck probes settings.                                                          |
| [kubeconfigSecretRef](#api-scylla-scylladb-com-remotekubernetesclusters-v1alpha1-spec-kubeconfigsecretref)         | object | kubeconfigSecretRef is a reference to a secret keeping kubeconfig allowing to connect to remote Kubernetes cluster. |

<a id="api-scylla-scylladb-com-remotekubernetesclusters-v1alpha1-spec-clienthealthcheckprobes"></a>

### .spec.clientHealthcheckProbes

#### Description

healthcheckProbes hold client healthcheck probes settings.

#### Type

object

| Property      | Type    | Description                                                      |
|---------------|---------|------------------------------------------------------------------|
| periodSeconds | integer | periodSeconds specifies the period of client healthcheck probes. |

<a id="api-scylla-scylladb-com-remotekubernetesclusters-v1alpha1-spec-kubeconfigsecretref"></a>

### .spec.kubeconfigSecretRef

#### Description

kubeconfigSecretRef is a reference to a secret keeping kubeconfig allowing to connect to remote Kubernetes cluster.

#### Type

object

| Property   | Type   | Description                                                              |
|------------|--------|--------------------------------------------------------------------------|
| name       | string | name is unique within a namespace to reference a secret resource.        |
| namespace  | string | namespace defines the space within which the secret name must be unique. |

<a id="api-scylla-scylladb-com-remotekubernetesclusters-v1alpha1-status"></a>

### .status

#### Description

status defines the observed state of the RemoteKubernetesCluster.

#### Type

object

| Property                                                                                   | Type           | Description                                                                                                                                                                                             |
|--------------------------------------------------------------------------------------------|----------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [conditions](#api-scylla-scylladb-com-remotekubernetesclusters-v1alpha1-status-conditions) | array (object) | conditions hold conditions describing RemoteKubernetesCluster state.                                                                                                                                    |
| observedGeneration                                                                         | integer        | observedGeneration is the most recent generation observed for this RemoteKubernetesCluster. It corresponds to the RemoteKubernetesCluster’s generation, which is updated on mutation by the API Server. |

<a id="api-scylla-scylladb-com-remotekubernetesclusters-v1alpha1-status-conditions"></a>

### .status.conditions[]

#### Description

Condition contains details for one aspect of the current state of this API Resource.

#### Type

object

| Property           | Type    | Description                                                                                                                                                                                                                                                                                                                     |
|--------------------|---------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| lastTransitionTime | string  | lastTransitionTime is the last time the condition transitioned from one status to another. This should be when the underlying condition changed.  If that is not known, then using the time when the API field changed is acceptable.                                                                                           |
| message            | string  | message is a human readable message indicating details about the transition. This may be an empty string.                                                                                                                                                                                                                       |
| observedGeneration | integer | observedGeneration represents the .metadata.generation that the condition was set based upon. For instance, if .metadata.generation is currently 12, but the .status.conditions[x].observedGeneration is 9, the condition is out of date with respect to the current state of the instance.                                     |
| reason             | string  | reason contains a programmatic identifier indicating the reason for the condition’s last transition. Producers of specific condition types may define expected values and meanings for this field, and whether the values are considered a guaranteed API. The value should be a CamelCase string. This field may not be empty. |
| status             | string  | status of the condition, one of True, False, Unknown.                                                                                                                                                                                                                                                                           |
| type               | string  | type of condition in CamelCase or in foo.example.com/CamelCase.                                                                                                                                                                                                                                                                 |
