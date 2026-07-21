# ScyllaOperatorConfig (scylla.scylladb.com/v1alpha1)

**APIVersion**: scylla.scylladb.com/v1alpha1
<br/>
**Kind**: ScyllaOperatorConfig
<br/>
**PluralName**: scyllaoperatorconfigs
<br/>
**SingularName**: scyllaoperatorconfig
<br/>
**Scope**: Cluster
<br/>
**ListKind**: ScyllaOperatorConfigList
<br/>
**Served**: true
<br/>
**Storage**: true
<br/>

## Description

ScyllaOperatorConfig describes the Scylla Operator configuration.

## Specification

| Property                                                                     | Type   | Description                                                                                                                                                                                                                                                                                                                                                                                           |
|------------------------------------------------------------------------------|--------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| apiVersion                                                                   | string | APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: [https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources](https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources)    |
| kind                                                                         | string | Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: [https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds](https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds) |
| [metadata](#api-scylla-scylladb-com-scyllaoperatorconfigs-v1alpha1-metadata) | object |                                                                                                                                                                                                                                                                                                                                                                                                       |
| [spec](#api-scylla-scylladb-com-scyllaoperatorconfigs-v1alpha1-spec)         | object | spec defines the desired state of the operator.                                                                                                                                                                                                                                                                                                                                                       |
| [status](#api-scylla-scylladb-com-scyllaoperatorconfigs-v1alpha1-status)     | object | status defines the observed state of the operator.                                                                                                                                                                                                                                                                                                                                                    |

<a id="api-scylla-scylladb-com-scyllaoperatorconfigs-v1alpha1-metadata"></a>

### .metadata

#### Description

#### Type

object

<a id="api-scylla-scylladb-com-scyllaoperatorconfigs-v1alpha1-spec"></a>

### .spec

#### Description

spec defines the desired state of the operator.

#### Type

object

| Property                             | Type   | Description                                                                                                                                                                                                      |
|--------------------------------------|--------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| configuredClusterDomain              | string | configuredClusterDomain allows users to set the configured Kubernetes cluster domain explicitly, instead of letting Scylla Operator automatically discover it.                                                   |
| scyllaUtilsImage                     | string | scyllaUtilsImage is a ScyllaDB image used for running ScyllaDB utilities.                                                                                                                                        |
| unsupportedBashToolsImageOverride    | string | unsupportedBashToolsImageOverride allows to adjust a generic Bash image with extra tools used by the operator for auxiliary purposes. Setting this field renders your cluster unsupported. Use at your own risk. |
| unsupportedGrafanaImageOverride      | string | unsupportedGrafanaImageOverride allows to adjust Grafana image used by the operator for testing, dev or emergencies. Setting this field renders your cluster unsupported. Use at your own risk.                  |
| unsupportedPrometheusVersionOverride | string | unsupportedPrometheusVersionOverride allows to adjust Prometheus version used by the operator for testing, dev or emergencies. Setting this field renders your cluster unsupported. Use at your own risk.        |

<a id="api-scylla-scylladb-com-scyllaoperatorconfigs-v1alpha1-status"></a>

### .status

#### Description

status defines the observed state of the operator.

#### Type

object

| Property                                                                                | Type           | Description                                                                                                                                                                                       |
|-----------------------------------------------------------------------------------------|----------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| bashToolsImage                                                                          | string         | bashToolsImage is a generic Bash image with extra tools used by the operator for auxiliary purposes.                                                                                              |
| clusterDomain                                                                           | string         | clusterDomain is the Kubernetes cluster domain used by the Scylla Operator.                                                                                                                       |
| [conditions](#api-scylla-scylladb-com-scyllaoperatorconfigs-v1alpha1-status-conditions) | array (object) | conditions hold conditions describing ScyllaOperatorConfig state.                                                                                                                                 |
| grafanaImage                                                                            | string         | grafanaImage is the image used by the operator to create a Grafana instance.                                                                                                                      |
| observedGeneration                                                                      | integer        | observedGeneration is the most recent generation observed for this ScyllaOperatorConfig. It corresponds to the ScyllaOperatorConfig’s generation, which is updated on mutation by the API Server. |
| prometheusVersion                                                                       | string         | prometheusVersion is the Prometheus version used by the operator to create a Prometheus instance.                                                                                                 |
| scyllaDBUtilsImage                                                                      | string         | scyllaDBUtilsImage is the ScyllaDB image used for running ScyllaDB utilities.                                                                                                                     |

<a id="api-scylla-scylladb-com-scyllaoperatorconfigs-v1alpha1-status-conditions"></a>

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
