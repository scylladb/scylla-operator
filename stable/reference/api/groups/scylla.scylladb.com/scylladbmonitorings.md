# ScyllaDBMonitoring (scylla.scylladb.com/v1alpha1)

**APIVersion**: scylla.scylladb.com/v1alpha1
<br/>
**Kind**: ScyllaDBMonitoring
<br/>
**PluralName**: scylladbmonitorings
<br/>
**SingularName**: scylladbmonitoring
<br/>
**Scope**: Namespaced
<br/>
**ListKind**: ScyllaDBMonitoringList
<br/>
**Served**: true
<br/>
**Storage**: true
<br/>

## Description

ScyllaDBMonitoring defines a monitoring instance for ScyllaDB clusters.

## Specification

| Property                                                                   | Type   | Description                                                                                                                                                                                                                                                                                                                                                                                           |
|----------------------------------------------------------------------------|--------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| apiVersion                                                                 | string | APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: [https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources](https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources)    |
| kind                                                                       | string | Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: [https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds](https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds) |
| [metadata](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-metadata) | object |                                                                                                                                                                                                                                                                                                                                                                                                       |
| [spec](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec)         | object | spec defines the desired state of this ScyllaDBMonitoring.                                                                                                                                                                                                                                                                                                                                            |
| [status](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-status)     | object | status is the current status of this ScyllaDBMonitoring.                                                                                                                                                                                                                                                                                                                                              |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-metadata"></a>

### .metadata

#### Description

#### Type

object

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec"></a>

### .spec

#### Description

spec defines the desired state of this ScyllaDBMonitoring.

#### Type

object

| Property                                                                                          | Type   | Description                                                                                                                                                                                                                                                                                                  |
|---------------------------------------------------------------------------------------------------|--------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [components](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components)               | object | components hold additional config for the monitoring components in use.                                                                                                                                                                                                                                      |
| [endpointsSelector](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-endpointsselector) | object | endpointsSelector select which Endpoints should be scraped. For local ScyllaDB clusters or datacenters, this is the same selector as if you were trying to select member Services. For remote ScyllaDB clusters, this can select any endpoints that are created manually or for a Service without selectors. |
| type                                                                                              | string | type determines the platform type of the monitoring setup.                                                                                                                                                                                                                                                   |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components"></a>

### .spec.components

#### Description

components hold additional config for the monitoring components in use.

#### Type

object

| Property                                                                                       | Type   | Description                                                         |
|------------------------------------------------------------------------------------------------|--------|---------------------------------------------------------------------|
| [grafana](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana)       | object | grafana holds configuration for the grafana instance, if any.       |
| [prometheus](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus) | object | prometheus holds configuration for the prometheus instance, if any. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana"></a>

### .spec.components.grafana

#### Description

grafana holds configuration for the grafana instance, if any.

#### Type

object

| Property                                                                                                       | Type           | Description                                                                                                                                                                                                                                                                                        |
|----------------------------------------------------------------------------------------------------------------|----------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [authentication](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-authentication) | object         | authentication hold the authentication options for accessing Grafana.                                                                                                                                                                                                                              |
| [datasources](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-datasources)       | array (object) | datasources is a list of Grafana datasources to configure. It’s expected to be set when using Prometheus component in External mode. At most one datasource is allowed for now (only Prometheus is supported).                                                                                     |
| [exposeOptions](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-exposeoptions)   | object         | exposeOptions specifies options for exposing Grafana UI.  Deprecated: This field will be removed in the next version of the API. Support for it will be removed in the future versions of the operator. We recommend managing your own Ingress or HTTPRoute resources to expose Grafana if needed. |
| [placement](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement)           | object         | placement describes restrictions for the nodes Grafana is scheduled on.                                                                                                                                                                                                                            |
| [resources](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-resources)           | object         | resources the Grafana container will use.                                                                                                                                                                                                                                                          |
| servingCertSecretName                                                                                          | string         | servingCertSecretName is the name of the secret holding a serving cert-key pair. If not specified, the operator will create a self-signed CA that creates the default serving cert-key pair.                                                                                                       |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-authentication"></a>

### .spec.components.grafana.authentication

#### Description

authentication hold the authentication options for accessing Grafana.

#### Type

object

| Property                      | Type    | Description                                                                    |
|-------------------------------|---------|--------------------------------------------------------------------------------|
| insecureEnableAnonymousAccess | boolean | insecureEnableAnonymousAccess allows access to Grafana without authentication. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-datasources"></a>

### .spec.components.grafana.datasources[]

#### Description

#### Type

object

| Property                                                                                                                         | Type   | Description                                                                                                                                                                         |
|----------------------------------------------------------------------------------------------------------------------------------|--------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| name                                                                                                                             | string | name is the name of the datasource as it will appear in Grafana. Only “prometheus” is supported as that’s the datasource name expected by the ScyllaDB monitoring stack dashboards. |
| [prometheusOptions](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-datasources-prometheusoptions) | object | prometheusOptions defines Prometheus-specific options.                                                                                                                              |
| type                                                                                                                             | string | type is the type of the datasource. Only “prometheus” is supported.                                                                                                                 |
| url                                                                                                                              | string | url is the URL of the datasource.                                                                                                                                                   |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-datasources-prometheusoptions"></a>

### .spec.components.grafana.datasources[].prometheusOptions

#### Description

prometheusOptions defines Prometheus-specific options.

#### Type

object

| Property                                                                                                                 | Type   | Description                                                          |
|--------------------------------------------------------------------------------------------------------------------------|--------|----------------------------------------------------------------------|
| [auth](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-datasources-prometheusoptions-auth) | object | auth holds authentication options for connecting to Prometheus.      |
| [tls](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-datasources-prometheusoptions-tls)   | object | tls holds TLS configuration for connecting to Prometheus over HTTPS. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-datasources-prometheusoptions-auth"></a>

### .spec.components.grafana.datasources[].prometheusOptions.auth

#### Description

auth holds authentication options for connecting to Prometheus.

#### Type

object

| Property                                                                                                                                                  | Type   | Description                                                |
|-----------------------------------------------------------------------------------------------------------------------------------------------------------|--------|------------------------------------------------------------|
| [bearerTokenOptions](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-datasources-prometheusoptions-auth-bearertokenoptions) | object | bearerToken holds options for Bearer token authentication. |
| type                                                                                                                                                      | string | type is the type of authentication to use.                 |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-datasources-prometheusoptions-auth-bearertokenoptions"></a>

### .spec.components.grafana.datasources[].prometheusOptions.auth.bearerTokenOptions

#### Description

bearerToken holds options for Bearer token authentication.

#### Type

object

| Property                                                                                                                                                   | Type   | Description                                                                                                  |
|------------------------------------------------------------------------------------------------------------------------------------------------------------|--------|--------------------------------------------------------------------------------------------------------------|
| [secretRef](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-datasources-prometheusoptions-auth-bearertokenoptions-secretref) | object | secretRef is a reference to a key in a Secret holding a Bearer token to use to authenticate with Prometheus. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-datasources-prometheusoptions-auth-bearertokenoptions-secretref"></a>

### .spec.components.grafana.datasources[].prometheusOptions.auth.bearerTokenOptions.secretRef

#### Description

secretRef is a reference to a key in a Secret holding a Bearer token to use to authenticate with Prometheus.

#### Type

object

| Property   | Type   | Description                     |
|------------|--------|---------------------------------|
| key        | string | key within the selected object. |
| name       | string | name of the selected object.    |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-datasources-prometheusoptions-tls"></a>

### .spec.components.grafana.datasources[].prometheusOptions.tls

#### Description

tls holds TLS configuration for connecting to Prometheus over HTTPS.

#### Type

object

| Property                                                                                                                                                               | Type    | Description                                                                                                                                                                                              |
|------------------------------------------------------------------------------------------------------------------------------------------------------------------------|---------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [caCertConfigMapRef](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-datasources-prometheusoptions-tls-cacertconfigmapref)               | object  | caCert is a reference to a key within the CA bundle ConfigMap. The key should hold the CA cert in PEM format. When not specified, system CAs are used.                                                   |
| [clientTLSKeyPairSecretRef](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-datasources-prometheusoptions-tls-clienttlskeypairsecretref) | object  | clientTLSKeyPairSecretRef is a reference to a Secret holding client TLS certificate and key for mTLS authentication. It’s expected to be a standard Kubernetes TLS Secret with tls.crt and tls.key keys. |
| insecureSkipVerify                                                                                                                                                     | boolean | insecureSkipVerify controls whether to skip server certificate verification.                                                                                                                             |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-datasources-prometheusoptions-tls-cacertconfigmapref"></a>

### .spec.components.grafana.datasources[].prometheusOptions.tls.caCertConfigMapRef

#### Description

caCert is a reference to a key within the CA bundle ConfigMap. The key should hold the CA cert in PEM format. When not specified, system CAs are used.

#### Type

object

| Property   | Type   | Description                     |
|------------|--------|---------------------------------|
| key        | string | key within the selected object. |
| name       | string | name of the selected object.    |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-datasources-prometheusoptions-tls-clienttlskeypairsecretref"></a>

### .spec.components.grafana.datasources[].prometheusOptions.tls.clientTLSKeyPairSecretRef

#### Description

clientTLSKeyPairSecretRef is a reference to a Secret holding client TLS certificate and key for mTLS authentication. It’s expected to be a standard Kubernetes TLS Secret with tls.crt and tls.key keys.

#### Type

object

| Property   | Type   | Description           |
|------------|--------|-----------------------|
| name       | string | Name of the referent. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-exposeoptions"></a>

### .spec.components.grafana.exposeOptions

#### Description

exposeOptions specifies options for exposing Grafana UI.  Deprecated: This field will be removed in the next version of the API. Support for it will be removed in the future versions of the operator. We recommend managing your own Ingress or HTTPRoute resources to expose Grafana if needed.

#### Type

object

| Property                                                                                                                 | Type   | Description                                                       |
|--------------------------------------------------------------------------------------------------------------------------|--------|-------------------------------------------------------------------|
| [webInterface](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-exposeoptions-webinterface) | object | webInterface specifies expose options for the user web interface. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-exposeoptions-webinterface"></a>

### .spec.components.grafana.exposeOptions.webInterface

#### Description

webInterface specifies expose options for the user web interface.

#### Type

object

| Property                                                                                                                    | Type   | Description                                  |
|-----------------------------------------------------------------------------------------------------------------------------|--------|----------------------------------------------|
| [ingress](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-exposeoptions-webinterface-ingress) | object | ingress is an Ingress configuration options. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-exposeoptions-webinterface-ingress"></a>

### .spec.components.grafana.exposeOptions.webInterface.ingress

#### Description

ingress is an Ingress configuration options.

#### Type

object

| Property                                                                                                                                    | Type           | Description                                                                |
|---------------------------------------------------------------------------------------------------------------------------------------------|----------------|----------------------------------------------------------------------------|
| [annotations](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-exposeoptions-webinterface-ingress-annotations) | object         | annotations specifies custom annotations merged into every Ingress object. |
| disabled                                                                                                                                    | boolean        | disabled controls if Ingress object creation is disabled.                  |
| dnsDomains                                                                                                                                  | array (string) | dnsDomains is a list of DNS domains this ingress is reachable by.          |
| ingressClassName                                                                                                                            | string         | ingressClassName specifies Ingress class name.                             |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-exposeoptions-webinterface-ingress-annotations"></a>

### .spec.components.grafana.exposeOptions.webInterface.ingress.annotations

#### Description

annotations specifies custom annotations merged into every Ingress object.

#### Type

object

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement"></a>

### .spec.components.grafana.placement

#### Description

placement describes restrictions for the nodes Grafana is scheduled on.

#### Type

object

| Property                                                                                                                   | Type           | Description                                                                                                             |
|----------------------------------------------------------------------------------------------------------------------------|----------------|-------------------------------------------------------------------------------------------------------------------------|
| [nodeAffinity](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-nodeaffinity)       | object         | nodeAffinity describes node affinity scheduling rules for the pod.                                                      |
| [podAffinity](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podaffinity)         | object         | podAffinity describes pod affinity scheduling rules.                                                                    |
| [podAntiAffinity](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podantiaffinity) | object         | podAntiAffinity describes pod anti-affinity scheduling rules.                                                           |
| [tolerations](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-tolerations)         | array (object) | tolerations allow the pod to tolerate any taint that matches the triple <key,value,effect> using the matching operator. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-nodeaffinity"></a>

### .spec.components.grafana.placement.nodeAffinity

#### Description

nodeAffinity describes node affinity scheduling rules for the pod.

#### Type

object

| Property                                                                                                                                                                                                | Type           | Description                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    |
|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [preferredDuringSchedulingIgnoredDuringExecution](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-nodeaffinity-preferredduringschedulingignoredduringexecution) | array (object) | The scheduler will prefer to schedule pods to nodes that satisfy the affinity expressions specified by this field, but it may choose a node that violates one or more of the expressions. The node that is most preferred is the one with the greatest sum of weights, i.e. for each node that meets all of the scheduling requirements (resource request, requiredDuringScheduling affinity expressions, etc.), compute a sum by iterating through the elements of this field and adding “weight” to the sum if the node matches the corresponding matchExpressions; the node(s) with the highest sum are the most preferred. |
| [requiredDuringSchedulingIgnoredDuringExecution](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-nodeaffinity-requiredduringschedulingignoredduringexecution)   | object         | If the affinity requirements specified by this field are not met at scheduling time, the pod will not be scheduled onto the node. If the affinity requirements specified by this field cease to be met at some point during pod execution (e.g. due to an update), the system may or may not try to eventually evict the pod from its node.                                                                                                                                                                                                                                                                                    |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-nodeaffinity-preferredduringschedulingignoredduringexecution"></a>

### .spec.components.grafana.placement.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[]

#### Description

An empty preferred scheduling term matches all objects with implicit weight 0 (i.e. it’s a no-op). A null preferred scheduling term matches no objects (i.e. is also a no-op).

#### Type

object

| Property                                                                                                                                                                      | Type    | Description                                                                             |
|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|---------|-----------------------------------------------------------------------------------------|
| [preference](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-nodeaffinity-preferredduringschedulingignoredduringexecution-preference) | object  | A node selector term, associated with the corresponding weight.                         |
| weight                                                                                                                                                                        | integer | Weight associated with matching the corresponding nodeSelectorTerm, in the range 1-100. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-nodeaffinity-preferredduringschedulingignoredduringexecution-preference"></a>

### .spec.components.grafana.placement.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[].preference

#### Description

A node selector term, associated with the corresponding weight.

#### Type

object

| Property                                                                                                                                                                                             | Type           | Description                                            |
|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------|--------------------------------------------------------|
| [matchExpressions](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-nodeaffinity-preferredduringschedulingignoredduringexecution-preference-matchexpressions) | array (object) | A list of node selector requirements by node’s labels. |
| [matchFields](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-nodeaffinity-preferredduringschedulingignoredduringexecution-preference-matchfields)           | array (object) | A list of node selector requirements by node’s fields. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-nodeaffinity-preferredduringschedulingignoredduringexecution-preference-matchexpressions"></a>

### .spec.components.grafana.placement.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[].preference.matchExpressions[]

#### Description

A node selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

#### Type

object

| Property   | Type           | Description                                                                                                                                                                                                                                                                                                                                         |
|------------|----------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| key        | string         | The label key that the selector applies to.                                                                                                                                                                                                                                                                                                         |
| operator   | string         | Represents a key’s relationship to a set of values. Valid operators are In, NotIn, Exists, DoesNotExist. Gt, and Lt.                                                                                                                                                                                                                                |
| values     | array (string) | An array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. If the operator is Gt or Lt, the values array must have a single element, which will be interpreted as an integer. This array is replaced during a strategic merge patch. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-nodeaffinity-preferredduringschedulingignoredduringexecution-preference-matchfields"></a>

### .spec.components.grafana.placement.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[].preference.matchFields[]

#### Description

A node selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

#### Type

object

| Property   | Type           | Description                                                                                                                                                                                                                                                                                                                                         |
|------------|----------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| key        | string         | The label key that the selector applies to.                                                                                                                                                                                                                                                                                                         |
| operator   | string         | Represents a key’s relationship to a set of values. Valid operators are In, NotIn, Exists, DoesNotExist. Gt, and Lt.                                                                                                                                                                                                                                |
| values     | array (string) | An array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. If the operator is Gt or Lt, the values array must have a single element, which will be interpreted as an integer. This array is replaced during a strategic merge patch. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-nodeaffinity-requiredduringschedulingignoredduringexecution"></a>

### .spec.components.grafana.placement.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution

#### Description

If the affinity requirements specified by this field are not met at scheduling time, the pod will not be scheduled onto the node. If the affinity requirements specified by this field cease to be met at some point during pod execution (e.g. due to an update), the system may or may not try to eventually evict the pod from its node.

#### Type

object

| Property                                                                                                                                                                                   | Type           | Description                                                  |
|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------|--------------------------------------------------------------|
| [nodeSelectorTerms](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-nodeaffinity-requiredduringschedulingignoredduringexecution-nodeselectorterms) | array (object) | Required. A list of node selector terms. The terms are ORed. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-nodeaffinity-requiredduringschedulingignoredduringexecution-nodeselectorterms"></a>

### .spec.components.grafana.placement.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[]

#### Description

A null or empty node selector term matches no objects. The requirements of them are ANDed. The TopologySelectorTerm type implements a subset of the NodeSelectorTerm.

#### Type

object

| Property                                                                                                                                                                                                   | Type           | Description                                            |
|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------|--------------------------------------------------------|
| [matchExpressions](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-nodeaffinity-requiredduringschedulingignoredduringexecution-nodeselectorterms-matchexpressions) | array (object) | A list of node selector requirements by node’s labels. |
| [matchFields](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-nodeaffinity-requiredduringschedulingignoredduringexecution-nodeselectorterms-matchfields)           | array (object) | A list of node selector requirements by node’s fields. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-nodeaffinity-requiredduringschedulingignoredduringexecution-nodeselectorterms-matchexpressions"></a>

### .spec.components.grafana.placement.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[].matchExpressions[]

#### Description

A node selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

#### Type

object

| Property   | Type           | Description                                                                                                                                                                                                                                                                                                                                         |
|------------|----------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| key        | string         | The label key that the selector applies to.                                                                                                                                                                                                                                                                                                         |
| operator   | string         | Represents a key’s relationship to a set of values. Valid operators are In, NotIn, Exists, DoesNotExist. Gt, and Lt.                                                                                                                                                                                                                                |
| values     | array (string) | An array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. If the operator is Gt or Lt, the values array must have a single element, which will be interpreted as an integer. This array is replaced during a strategic merge patch. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-nodeaffinity-requiredduringschedulingignoredduringexecution-nodeselectorterms-matchfields"></a>

### .spec.components.grafana.placement.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[].matchFields[]

#### Description

A node selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

#### Type

object

| Property   | Type           | Description                                                                                                                                                                                                                                                                                                                                         |
|------------|----------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| key        | string         | The label key that the selector applies to.                                                                                                                                                                                                                                                                                                         |
| operator   | string         | Represents a key’s relationship to a set of values. Valid operators are In, NotIn, Exists, DoesNotExist. Gt, and Lt.                                                                                                                                                                                                                                |
| values     | array (string) | An array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. If the operator is Gt or Lt, the values array must have a single element, which will be interpreted as an integer. This array is replaced during a strategic merge patch. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podaffinity"></a>

### .spec.components.grafana.placement.podAffinity

#### Description

podAffinity describes pod affinity scheduling rules.

#### Type

object

| Property                                                                                                                                                                                               | Type           | Description                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  |
|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [preferredDuringSchedulingIgnoredDuringExecution](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podaffinity-preferredduringschedulingignoredduringexecution) | array (object) | The scheduler will prefer to schedule pods to nodes that satisfy the affinity expressions specified by this field, but it may choose a node that violates one or more of the expressions. The node that is most preferred is the one with the greatest sum of weights, i.e. for each node that meets all of the scheduling requirements (resource request, requiredDuringScheduling affinity expressions, etc.), compute a sum by iterating through the elements of this field and adding “weight” to the sum if the node has pods which matches the corresponding podAffinityTerm; the node(s) with the highest sum are the most preferred. |
| [requiredDuringSchedulingIgnoredDuringExecution](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podaffinity-requiredduringschedulingignoredduringexecution)   | array (object) | If the affinity requirements specified by this field are not met at scheduling time, the pod will not be scheduled onto the node. If the affinity requirements specified by this field cease to be met at some point during pod execution (e.g. due to a pod label update), the system may or may not try to eventually evict the pod from its node. When there are multiple elements, the lists of nodes corresponding to each podAffinityTerm are intersected, i.e. all terms must be satisfied.                                                                                                                                           |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podaffinity-preferredduringschedulingignoredduringexecution"></a>

### .spec.components.grafana.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[]

#### Description

The weights of all of the matched WeightedPodAffinityTerm fields are added per-node to find the most preferred node(s)

#### Type

object

| Property                                                                                                                                                                               | Type    | Description                                                                            |
|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|---------|----------------------------------------------------------------------------------------|
| [podAffinityTerm](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm) | object  | Required. A pod affinity term, associated with the corresponding weight.               |
| weight                                                                                                                                                                                 | integer | weight associated with matching the corresponding podAffinityTerm, in the range 1-100. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm"></a>

### .spec.components.grafana.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm

#### Description

Required. A pod affinity term, associated with the corresponding weight.

#### Type

object

| Property                                                                                                                                                                                                   | Type           | Description                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [labelSelector](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-labelselector)         | object         | A label query over a set of resources, in this case pods. If it’s null, this PodAffinityTerm matches with no Pods.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| matchLabelKeys                                                                                                                                                                                             | array (string) | MatchLabelKeys is a set of pod label keys to select which pods will be taken into consideration. The keys are used to lookup values from the incoming pod labels, those key-value labels are merged with labelSelector as key in (value) to select the group of existing pods which pods will be taken into consideration for the incoming pod’s pod (anti) affinity. Keys that don’t exist in the incoming pod labels will be ignored. The default value is empty. The same key is forbidden to exist in both matchLabelKeys and labelSelector. Also, matchLabelKeys cannot be set when labelSelector isn’t set.             |
| mismatchLabelKeys                                                                                                                                                                                          | array (string) | MismatchLabelKeys is a set of pod label keys to select which pods will be taken into consideration. The keys are used to lookup values from the incoming pod labels, those key-value labels are merged with labelSelector as key notin (value) to select the group of existing pods which pods will be taken into consideration for the incoming pod’s pod (anti) affinity. Keys that don’t exist in the incoming pod labels will be ignored. The default value is empty. The same key is forbidden to exist in both mismatchLabelKeys and labelSelector. Also, mismatchLabelKeys cannot be set when labelSelector isn’t set. |
| [namespaceSelector](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-namespaceselector) | object         | A label query over the set of namespaces that the term applies to. The term is applied to the union of the namespaces selected by this field and the ones listed in the namespaces field. null selector and null or empty namespaces list means “this pod’s namespace”. An empty selector ({}) matches all namespaces.                                                                                                                                                                                                                                                                                                        |
| namespaces                                                                                                                                                                                                 | array (string) | namespaces specifies a static list of namespace names that the term applies to. The term is applied to the union of the namespaces listed in this field and the ones selected by namespaceSelector. null or empty namespaces list and null namespaceSelector means “this pod’s namespace”.                                                                                                                                                                                                                                                                                                                                    |
| topologyKey                                                                                                                                                                                                | string         | This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching the labelSelector in the specified namespaces, where co-located is defined as running on a node whose value of the label with key topologyKey matches that of any node on which any of the selected pods is running. Empty topologyKey is not allowed.                                                                                                                                                                                                                                                                      |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-labelselector"></a>

### .spec.components.grafana.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector

#### Description

A label query over a set of resources, in this case pods. If it’s null, this PodAffinityTerm matches with no Pods.

#### Type

object

| Property                                                                                                                                                                                                               | Type           | Description                                                                                                                                                                                                                                                     |
|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [matchExpressions](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-labelselector-matchexpressions) | array (object) | matchExpressions is a list of label selector requirements. The requirements are ANDed.                                                                                                                                                                          |
| [matchLabels](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-labelselector-matchlabels)           | object         | matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is “key”, the operator is “In”, and the values array contains only “value”. The requirements are ANDed. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-labelselector-matchexpressions"></a>

### .spec.components.grafana.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchExpressions[]

#### Description

A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

#### Type

object

| Property   | Type           | Description                                                                                                                                                                                                                                |
|------------|----------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| key        | string         | key is the label key that the selector applies to.                                                                                                                                                                                         |
| operator   | string         | operator represents a key’s relationship to a set of values. Valid operators are In, NotIn, Exists and DoesNotExist.                                                                                                                       |
| values     | array (string) | values is an array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-labelselector-matchlabels"></a>

### .spec.components.grafana.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchLabels

#### Description

matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is “key”, the operator is “In”, and the values array contains only “value”. The requirements are ANDed.

#### Type

object

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-namespaceselector"></a>

### .spec.components.grafana.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector

#### Description

A label query over the set of namespaces that the term applies to. The term is applied to the union of the namespaces selected by this field and the ones listed in the namespaces field. null selector and null or empty namespaces list means “this pod’s namespace”. An empty selector ({}) matches all namespaces.

#### Type

object

| Property                                                                                                                                                                                                                   | Type           | Description                                                                                                                                                                                                                                                     |
|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [matchExpressions](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-namespaceselector-matchexpressions) | array (object) | matchExpressions is a list of label selector requirements. The requirements are ANDed.                                                                                                                                                                          |
| [matchLabels](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-namespaceselector-matchlabels)           | object         | matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is “key”, the operator is “In”, and the values array contains only “value”. The requirements are ANDed. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-namespaceselector-matchexpressions"></a>

### .spec.components.grafana.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchExpressions[]

#### Description

A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

#### Type

object

| Property   | Type           | Description                                                                                                                                                                                                                                |
|------------|----------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| key        | string         | key is the label key that the selector applies to.                                                                                                                                                                                         |
| operator   | string         | operator represents a key’s relationship to a set of values. Valid operators are In, NotIn, Exists and DoesNotExist.                                                                                                                       |
| values     | array (string) | values is an array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-namespaceselector-matchlabels"></a>

### .spec.components.grafana.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchLabels

#### Description

matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is “key”, the operator is “In”, and the values array contains only “value”. The requirements are ANDed.

#### Type

object

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podaffinity-requiredduringschedulingignoredduringexecution"></a>

### .spec.components.grafana.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[]

#### Description

Defines a set of pods (namely those matching the labelSelector relative to the given namespace(s)) that this pod should be co-located (affinity) or not co-located (anti-affinity) with, where co-located is defined as running on a node whose value of the label with key <topologyKey> matches that of any node on which a pod of the set of pods is running

#### Type

object

| Property                                                                                                                                                                                  | Type           | Description                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [labelSelector](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podaffinity-requiredduringschedulingignoredduringexecution-labelselector)         | object         | A label query over a set of resources, in this case pods. If it’s null, this PodAffinityTerm matches with no Pods.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| matchLabelKeys                                                                                                                                                                            | array (string) | MatchLabelKeys is a set of pod label keys to select which pods will be taken into consideration. The keys are used to lookup values from the incoming pod labels, those key-value labels are merged with labelSelector as key in (value) to select the group of existing pods which pods will be taken into consideration for the incoming pod’s pod (anti) affinity. Keys that don’t exist in the incoming pod labels will be ignored. The default value is empty. The same key is forbidden to exist in both matchLabelKeys and labelSelector. Also, matchLabelKeys cannot be set when labelSelector isn’t set.             |
| mismatchLabelKeys                                                                                                                                                                         | array (string) | MismatchLabelKeys is a set of pod label keys to select which pods will be taken into consideration. The keys are used to lookup values from the incoming pod labels, those key-value labels are merged with labelSelector as key notin (value) to select the group of existing pods which pods will be taken into consideration for the incoming pod’s pod (anti) affinity. Keys that don’t exist in the incoming pod labels will be ignored. The default value is empty. The same key is forbidden to exist in both mismatchLabelKeys and labelSelector. Also, mismatchLabelKeys cannot be set when labelSelector isn’t set. |
| [namespaceSelector](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podaffinity-requiredduringschedulingignoredduringexecution-namespaceselector) | object         | A label query over the set of namespaces that the term applies to. The term is applied to the union of the namespaces selected by this field and the ones listed in the namespaces field. null selector and null or empty namespaces list means “this pod’s namespace”. An empty selector ({}) matches all namespaces.                                                                                                                                                                                                                                                                                                        |
| namespaces                                                                                                                                                                                | array (string) | namespaces specifies a static list of namespace names that the term applies to. The term is applied to the union of the namespaces listed in this field and the ones selected by namespaceSelector. null or empty namespaces list and null namespaceSelector means “this pod’s namespace”.                                                                                                                                                                                                                                                                                                                                    |
| topologyKey                                                                                                                                                                               | string         | This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching the labelSelector in the specified namespaces, where co-located is defined as running on a node whose value of the label with key topologyKey matches that of any node on which any of the selected pods is running. Empty topologyKey is not allowed.                                                                                                                                                                                                                                                                      |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podaffinity-requiredduringschedulingignoredduringexecution-labelselector"></a>

### .spec.components.grafana.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector

#### Description

A label query over a set of resources, in this case pods. If it’s null, this PodAffinityTerm matches with no Pods.

#### Type

object

| Property                                                                                                                                                                                              | Type           | Description                                                                                                                                                                                                                                                     |
|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [matchExpressions](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podaffinity-requiredduringschedulingignoredduringexecution-labelselector-matchexpressions) | array (object) | matchExpressions is a list of label selector requirements. The requirements are ANDed.                                                                                                                                                                          |
| [matchLabels](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podaffinity-requiredduringschedulingignoredduringexecution-labelselector-matchlabels)           | object         | matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is “key”, the operator is “In”, and the values array contains only “value”. The requirements are ANDed. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podaffinity-requiredduringschedulingignoredduringexecution-labelselector-matchexpressions"></a>

### .spec.components.grafana.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchExpressions[]

#### Description

A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

#### Type

object

| Property   | Type           | Description                                                                                                                                                                                                                                |
|------------|----------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| key        | string         | key is the label key that the selector applies to.                                                                                                                                                                                         |
| operator   | string         | operator represents a key’s relationship to a set of values. Valid operators are In, NotIn, Exists and DoesNotExist.                                                                                                                       |
| values     | array (string) | values is an array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podaffinity-requiredduringschedulingignoredduringexecution-labelselector-matchlabels"></a>

### .spec.components.grafana.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchLabels

#### Description

matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is “key”, the operator is “In”, and the values array contains only “value”. The requirements are ANDed.

#### Type

object

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podaffinity-requiredduringschedulingignoredduringexecution-namespaceselector"></a>

### .spec.components.grafana.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector

#### Description

A label query over the set of namespaces that the term applies to. The term is applied to the union of the namespaces selected by this field and the ones listed in the namespaces field. null selector and null or empty namespaces list means “this pod’s namespace”. An empty selector ({}) matches all namespaces.

#### Type

object

| Property                                                                                                                                                                                                  | Type           | Description                                                                                                                                                                                                                                                     |
|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [matchExpressions](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podaffinity-requiredduringschedulingignoredduringexecution-namespaceselector-matchexpressions) | array (object) | matchExpressions is a list of label selector requirements. The requirements are ANDed.                                                                                                                                                                          |
| [matchLabels](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podaffinity-requiredduringschedulingignoredduringexecution-namespaceselector-matchlabels)           | object         | matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is “key”, the operator is “In”, and the values array contains only “value”. The requirements are ANDed. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podaffinity-requiredduringschedulingignoredduringexecution-namespaceselector-matchexpressions"></a>

### .spec.components.grafana.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchExpressions[]

#### Description

A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

#### Type

object

| Property   | Type           | Description                                                                                                                                                                                                                                |
|------------|----------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| key        | string         | key is the label key that the selector applies to.                                                                                                                                                                                         |
| operator   | string         | operator represents a key’s relationship to a set of values. Valid operators are In, NotIn, Exists and DoesNotExist.                                                                                                                       |
| values     | array (string) | values is an array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podaffinity-requiredduringschedulingignoredduringexecution-namespaceselector-matchlabels"></a>

### .spec.components.grafana.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchLabels

#### Description

matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is “key”, the operator is “In”, and the values array contains only “value”. The requirements are ANDed.

#### Type

object

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podantiaffinity"></a>

### .spec.components.grafana.placement.podAntiAffinity

#### Description

podAntiAffinity describes pod anti-affinity scheduling rules.

#### Type

object

| Property                                                                                                                                                                                                   | Type           | Description                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [preferredDuringSchedulingIgnoredDuringExecution](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podantiaffinity-preferredduringschedulingignoredduringexecution) | array (object) | The scheduler will prefer to schedule pods to nodes that satisfy the anti-affinity expressions specified by this field, but it may choose a node that violates one or more of the expressions. The node that is most preferred is the one with the greatest sum of weights, i.e. for each node that meets all of the scheduling requirements (resource request, requiredDuringScheduling anti-affinity expressions, etc.), compute a sum by iterating through the elements of this field and subtracting “weight” from the sum if the node has pods which matches the corresponding podAffinityTerm; the node(s) with the highest sum are the most preferred. |
| [requiredDuringSchedulingIgnoredDuringExecution](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podantiaffinity-requiredduringschedulingignoredduringexecution)   | array (object) | If the anti-affinity requirements specified by this field are not met at scheduling time, the pod will not be scheduled onto the node. If the anti-affinity requirements specified by this field cease to be met at some point during pod execution (e.g. due to a pod label update), the system may or may not try to eventually evict the pod from its node. When there are multiple elements, the lists of nodes corresponding to each podAffinityTerm are intersected, i.e. all terms must be satisfied.                                                                                                                                                  |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podantiaffinity-preferredduringschedulingignoredduringexecution"></a>

### .spec.components.grafana.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[]

#### Description

The weights of all of the matched WeightedPodAffinityTerm fields are added per-node to find the most preferred node(s)

#### Type

object

| Property                                                                                                                                                                                   | Type    | Description                                                                            |
|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|---------|----------------------------------------------------------------------------------------|
| [podAffinityTerm](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podantiaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm) | object  | Required. A pod affinity term, associated with the corresponding weight.               |
| weight                                                                                                                                                                                     | integer | weight associated with matching the corresponding podAffinityTerm, in the range 1-100. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podantiaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm"></a>

### .spec.components.grafana.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm

#### Description

Required. A pod affinity term, associated with the corresponding weight.

#### Type

object

| Property                                                                                                                                                                                                       | Type           | Description                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [labelSelector](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podantiaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-labelselector)         | object         | A label query over a set of resources, in this case pods. If it’s null, this PodAffinityTerm matches with no Pods.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| matchLabelKeys                                                                                                                                                                                                 | array (string) | MatchLabelKeys is a set of pod label keys to select which pods will be taken into consideration. The keys are used to lookup values from the incoming pod labels, those key-value labels are merged with labelSelector as key in (value) to select the group of existing pods which pods will be taken into consideration for the incoming pod’s pod (anti) affinity. Keys that don’t exist in the incoming pod labels will be ignored. The default value is empty. The same key is forbidden to exist in both matchLabelKeys and labelSelector. Also, matchLabelKeys cannot be set when labelSelector isn’t set.             |
| mismatchLabelKeys                                                                                                                                                                                              | array (string) | MismatchLabelKeys is a set of pod label keys to select which pods will be taken into consideration. The keys are used to lookup values from the incoming pod labels, those key-value labels are merged with labelSelector as key notin (value) to select the group of existing pods which pods will be taken into consideration for the incoming pod’s pod (anti) affinity. Keys that don’t exist in the incoming pod labels will be ignored. The default value is empty. The same key is forbidden to exist in both mismatchLabelKeys and labelSelector. Also, mismatchLabelKeys cannot be set when labelSelector isn’t set. |
| [namespaceSelector](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podantiaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-namespaceselector) | object         | A label query over the set of namespaces that the term applies to. The term is applied to the union of the namespaces selected by this field and the ones listed in the namespaces field. null selector and null or empty namespaces list means “this pod’s namespace”. An empty selector ({}) matches all namespaces.                                                                                                                                                                                                                                                                                                        |
| namespaces                                                                                                                                                                                                     | array (string) | namespaces specifies a static list of namespace names that the term applies to. The term is applied to the union of the namespaces listed in this field and the ones selected by namespaceSelector. null or empty namespaces list and null namespaceSelector means “this pod’s namespace”.                                                                                                                                                                                                                                                                                                                                    |
| topologyKey                                                                                                                                                                                                    | string         | This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching the labelSelector in the specified namespaces, where co-located is defined as running on a node whose value of the label with key topologyKey matches that of any node on which any of the selected pods is running. Empty topologyKey is not allowed.                                                                                                                                                                                                                                                                      |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podantiaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-labelselector"></a>

### .spec.components.grafana.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector

#### Description

A label query over a set of resources, in this case pods. If it’s null, this PodAffinityTerm matches with no Pods.

#### Type

object

| Property                                                                                                                                                                                                                   | Type           | Description                                                                                                                                                                                                                                                     |
|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [matchExpressions](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podantiaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-labelselector-matchexpressions) | array (object) | matchExpressions is a list of label selector requirements. The requirements are ANDed.                                                                                                                                                                          |
| [matchLabels](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podantiaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-labelselector-matchlabels)           | object         | matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is “key”, the operator is “In”, and the values array contains only “value”. The requirements are ANDed. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podantiaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-labelselector-matchexpressions"></a>

### .spec.components.grafana.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchExpressions[]

#### Description

A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

#### Type

object

| Property   | Type           | Description                                                                                                                                                                                                                                |
|------------|----------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| key        | string         | key is the label key that the selector applies to.                                                                                                                                                                                         |
| operator   | string         | operator represents a key’s relationship to a set of values. Valid operators are In, NotIn, Exists and DoesNotExist.                                                                                                                       |
| values     | array (string) | values is an array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podantiaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-labelselector-matchlabels"></a>

### .spec.components.grafana.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchLabels

#### Description

matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is “key”, the operator is “In”, and the values array contains only “value”. The requirements are ANDed.

#### Type

object

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podantiaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-namespaceselector"></a>

### .spec.components.grafana.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector

#### Description

A label query over the set of namespaces that the term applies to. The term is applied to the union of the namespaces selected by this field and the ones listed in the namespaces field. null selector and null or empty namespaces list means “this pod’s namespace”. An empty selector ({}) matches all namespaces.

#### Type

object

| Property                                                                                                                                                                                                                       | Type           | Description                                                                                                                                                                                                                                                     |
|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [matchExpressions](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podantiaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-namespaceselector-matchexpressions) | array (object) | matchExpressions is a list of label selector requirements. The requirements are ANDed.                                                                                                                                                                          |
| [matchLabels](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podantiaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-namespaceselector-matchlabels)           | object         | matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is “key”, the operator is “In”, and the values array contains only “value”. The requirements are ANDed. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podantiaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-namespaceselector-matchexpressions"></a>

### .spec.components.grafana.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchExpressions[]

#### Description

A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

#### Type

object

| Property   | Type           | Description                                                                                                                                                                                                                                |
|------------|----------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| key        | string         | key is the label key that the selector applies to.                                                                                                                                                                                         |
| operator   | string         | operator represents a key’s relationship to a set of values. Valid operators are In, NotIn, Exists and DoesNotExist.                                                                                                                       |
| values     | array (string) | values is an array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podantiaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-namespaceselector-matchlabels"></a>

### .spec.components.grafana.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchLabels

#### Description

matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is “key”, the operator is “In”, and the values array contains only “value”. The requirements are ANDed.

#### Type

object

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podantiaffinity-requiredduringschedulingignoredduringexecution"></a>

### .spec.components.grafana.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[]

#### Description

Defines a set of pods (namely those matching the labelSelector relative to the given namespace(s)) that this pod should be co-located (affinity) or not co-located (anti-affinity) with, where co-located is defined as running on a node whose value of the label with key <topologyKey> matches that of any node on which a pod of the set of pods is running

#### Type

object

| Property                                                                                                                                                                                      | Type           | Description                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [labelSelector](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podantiaffinity-requiredduringschedulingignoredduringexecution-labelselector)         | object         | A label query over a set of resources, in this case pods. If it’s null, this PodAffinityTerm matches with no Pods.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| matchLabelKeys                                                                                                                                                                                | array (string) | MatchLabelKeys is a set of pod label keys to select which pods will be taken into consideration. The keys are used to lookup values from the incoming pod labels, those key-value labels are merged with labelSelector as key in (value) to select the group of existing pods which pods will be taken into consideration for the incoming pod’s pod (anti) affinity. Keys that don’t exist in the incoming pod labels will be ignored. The default value is empty. The same key is forbidden to exist in both matchLabelKeys and labelSelector. Also, matchLabelKeys cannot be set when labelSelector isn’t set.             |
| mismatchLabelKeys                                                                                                                                                                             | array (string) | MismatchLabelKeys is a set of pod label keys to select which pods will be taken into consideration. The keys are used to lookup values from the incoming pod labels, those key-value labels are merged with labelSelector as key notin (value) to select the group of existing pods which pods will be taken into consideration for the incoming pod’s pod (anti) affinity. Keys that don’t exist in the incoming pod labels will be ignored. The default value is empty. The same key is forbidden to exist in both mismatchLabelKeys and labelSelector. Also, mismatchLabelKeys cannot be set when labelSelector isn’t set. |
| [namespaceSelector](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podantiaffinity-requiredduringschedulingignoredduringexecution-namespaceselector) | object         | A label query over the set of namespaces that the term applies to. The term is applied to the union of the namespaces selected by this field and the ones listed in the namespaces field. null selector and null or empty namespaces list means “this pod’s namespace”. An empty selector ({}) matches all namespaces.                                                                                                                                                                                                                                                                                                        |
| namespaces                                                                                                                                                                                    | array (string) | namespaces specifies a static list of namespace names that the term applies to. The term is applied to the union of the namespaces listed in this field and the ones selected by namespaceSelector. null or empty namespaces list and null namespaceSelector means “this pod’s namespace”.                                                                                                                                                                                                                                                                                                                                    |
| topologyKey                                                                                                                                                                                   | string         | This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching the labelSelector in the specified namespaces, where co-located is defined as running on a node whose value of the label with key topologyKey matches that of any node on which any of the selected pods is running. Empty topologyKey is not allowed.                                                                                                                                                                                                                                                                      |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podantiaffinity-requiredduringschedulingignoredduringexecution-labelselector"></a>

### .spec.components.grafana.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector

#### Description

A label query over a set of resources, in this case pods. If it’s null, this PodAffinityTerm matches with no Pods.

#### Type

object

| Property                                                                                                                                                                                                  | Type           | Description                                                                                                                                                                                                                                                     |
|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [matchExpressions](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podantiaffinity-requiredduringschedulingignoredduringexecution-labelselector-matchexpressions) | array (object) | matchExpressions is a list of label selector requirements. The requirements are ANDed.                                                                                                                                                                          |
| [matchLabels](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podantiaffinity-requiredduringschedulingignoredduringexecution-labelselector-matchlabels)           | object         | matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is “key”, the operator is “In”, and the values array contains only “value”. The requirements are ANDed. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podantiaffinity-requiredduringschedulingignoredduringexecution-labelselector-matchexpressions"></a>

### .spec.components.grafana.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchExpressions[]

#### Description

A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

#### Type

object

| Property   | Type           | Description                                                                                                                                                                                                                                |
|------------|----------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| key        | string         | key is the label key that the selector applies to.                                                                                                                                                                                         |
| operator   | string         | operator represents a key’s relationship to a set of values. Valid operators are In, NotIn, Exists and DoesNotExist.                                                                                                                       |
| values     | array (string) | values is an array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podantiaffinity-requiredduringschedulingignoredduringexecution-labelselector-matchlabels"></a>

### .spec.components.grafana.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchLabels

#### Description

matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is “key”, the operator is “In”, and the values array contains only “value”. The requirements are ANDed.

#### Type

object

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podantiaffinity-requiredduringschedulingignoredduringexecution-namespaceselector"></a>

### .spec.components.grafana.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector

#### Description

A label query over the set of namespaces that the term applies to. The term is applied to the union of the namespaces selected by this field and the ones listed in the namespaces field. null selector and null or empty namespaces list means “this pod’s namespace”. An empty selector ({}) matches all namespaces.

#### Type

object

| Property                                                                                                                                                                                                      | Type           | Description                                                                                                                                                                                                                                                     |
|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [matchExpressions](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podantiaffinity-requiredduringschedulingignoredduringexecution-namespaceselector-matchexpressions) | array (object) | matchExpressions is a list of label selector requirements. The requirements are ANDed.                                                                                                                                                                          |
| [matchLabels](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podantiaffinity-requiredduringschedulingignoredduringexecution-namespaceselector-matchlabels)           | object         | matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is “key”, the operator is “In”, and the values array contains only “value”. The requirements are ANDed. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podantiaffinity-requiredduringschedulingignoredduringexecution-namespaceselector-matchexpressions"></a>

### .spec.components.grafana.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchExpressions[]

#### Description

A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

#### Type

object

| Property   | Type           | Description                                                                                                                                                                                                                                |
|------------|----------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| key        | string         | key is the label key that the selector applies to.                                                                                                                                                                                         |
| operator   | string         | operator represents a key’s relationship to a set of values. Valid operators are In, NotIn, Exists and DoesNotExist.                                                                                                                       |
| values     | array (string) | values is an array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-podantiaffinity-requiredduringschedulingignoredduringexecution-namespaceselector-matchlabels"></a>

### .spec.components.grafana.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchLabels

#### Description

matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is “key”, the operator is “In”, and the values array contains only “value”. The requirements are ANDed.

#### Type

object

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-placement-tolerations"></a>

### .spec.components.grafana.placement.tolerations[]

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

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-resources"></a>

### .spec.components.grafana.resources

#### Description

resources the Grafana container will use.

#### Type

object

| Property                                                                                                     | Type           | Description                                                                                                                                                                                                                                                                                                                                                                                                                  |
|--------------------------------------------------------------------------------------------------------------|----------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [claims](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-resources-claims)     | array (object) | Claims lists the names of resources, defined in spec.resourceClaims, that are used by this container.  This field depends on the DynamicResourceAllocation feature gate.  This field is immutable. It can only be set for containers.                                                                                                                                                                                        |
| [limits](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-resources-limits)     | object         | Limits describes the maximum amount of compute resources allowed. More info: [https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/)                                                                                                                                                                                |
| [requests](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-resources-requests) | object         | Requests describes the minimum amount of compute resources required. If Requests is omitted for a container, it defaults to Limits if that is explicitly specified, otherwise to an implementation-defined value. Requests cannot exceed Limits. More info: [https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/) |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-resources-claims"></a>

### .spec.components.grafana.resources.claims[]

#### Description

ResourceClaim references one entry in PodSpec.ResourceClaims.

#### Type

object

| Property   | Type   | Description                                                                                                                                                         |
|------------|--------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| name       | string | Name must match the name of one entry in pod.spec.resourceClaims of the Pod where this field is used. It makes that resource available inside a container.          |
| request    | string | Request is the name chosen for a request in the referenced claim. If empty, everything from the claim is made available, otherwise only the result of this request. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-resources-limits"></a>

### .spec.components.grafana.resources.limits

#### Description

Limits describes the maximum amount of compute resources allowed. More info: [https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/)

#### Type

object

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-grafana-resources-requests"></a>

### .spec.components.grafana.resources.requests

#### Description

Requests describes the minimum amount of compute resources required. If Requests is omitted for a container, it defaults to Limits if that is explicitly specified, otherwise to an implementation-defined value. Requests cannot exceed Limits. More info: [https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/)

#### Type

object

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus"></a>

### .spec.components.prometheus

#### Description

prometheus holds configuration for the prometheus instance, if any.

#### Type

object

| Property                                                                                                        | Type   | Description                                                                                                                                                                                                                                                                                              |
|-----------------------------------------------------------------------------------------------------------------|--------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [exposeOptions](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-exposeoptions) | object | exposeOptions specifies options for exposing Prometheus UI.  Deprecated: This field will be removed in the next version of the API. Support for it will be removed in the future versions of the operator. We recommend managing your own Ingress or HTTPRoute resources to expose Prometheus if needed. |
| mode                                                                                                            | string | mode defines the mode of the Prometheus instance.                                                                                                                                                                                                                                                        |
| [placement](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement)         | object | placement describes restrictions for the nodes Prometheus is scheduled on.                                                                                                                                                                                                                               |
| [resources](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-resources)         | object | resources the Prometheus container will use.                                                                                                                                                                                                                                                             |
| [storage](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-storage)             | object | storage describes the underlying storage that Prometheus will consume.                                                                                                                                                                                                                                   |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-exposeoptions"></a>

### .spec.components.prometheus.exposeOptions

#### Description

exposeOptions specifies options for exposing Prometheus UI.  Deprecated: This field will be removed in the next version of the API. Support for it will be removed in the future versions of the operator. We recommend managing your own Ingress or HTTPRoute resources to expose Prometheus if needed.

#### Type

object

| Property                                                                                                                    | Type   | Description                                                       |
|-----------------------------------------------------------------------------------------------------------------------------|--------|-------------------------------------------------------------------|
| [webInterface](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-exposeoptions-webinterface) | object | webInterface specifies expose options for the user web interface. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-exposeoptions-webinterface"></a>

### .spec.components.prometheus.exposeOptions.webInterface

#### Description

webInterface specifies expose options for the user web interface.

#### Type

object

| Property                                                                                                                       | Type   | Description                                  |
|--------------------------------------------------------------------------------------------------------------------------------|--------|----------------------------------------------|
| [ingress](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-exposeoptions-webinterface-ingress) | object | ingress is an Ingress configuration options. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-exposeoptions-webinterface-ingress"></a>

### .spec.components.prometheus.exposeOptions.webInterface.ingress

#### Description

ingress is an Ingress configuration options.

#### Type

object

| Property                                                                                                                                       | Type           | Description                                                                |
|------------------------------------------------------------------------------------------------------------------------------------------------|----------------|----------------------------------------------------------------------------|
| [annotations](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-exposeoptions-webinterface-ingress-annotations) | object         | annotations specifies custom annotations merged into every Ingress object. |
| disabled                                                                                                                                       | boolean        | disabled controls if Ingress object creation is disabled.                  |
| dnsDomains                                                                                                                                     | array (string) | dnsDomains is a list of DNS domains this ingress is reachable by.          |
| ingressClassName                                                                                                                               | string         | ingressClassName specifies Ingress class name.                             |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-exposeoptions-webinterface-ingress-annotations"></a>

### .spec.components.prometheus.exposeOptions.webInterface.ingress.annotations

#### Description

annotations specifies custom annotations merged into every Ingress object.

#### Type

object

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement"></a>

### .spec.components.prometheus.placement

#### Description

placement describes restrictions for the nodes Prometheus is scheduled on.

#### Type

object

| Property                                                                                                                      | Type           | Description                                                                                                             |
|-------------------------------------------------------------------------------------------------------------------------------|----------------|-------------------------------------------------------------------------------------------------------------------------|
| [nodeAffinity](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-nodeaffinity)       | object         | nodeAffinity describes node affinity scheduling rules for the pod.                                                      |
| [podAffinity](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podaffinity)         | object         | podAffinity describes pod affinity scheduling rules.                                                                    |
| [podAntiAffinity](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podantiaffinity) | object         | podAntiAffinity describes pod anti-affinity scheduling rules.                                                           |
| [tolerations](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-tolerations)         | array (object) | tolerations allow the pod to tolerate any taint that matches the triple <key,value,effect> using the matching operator. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-nodeaffinity"></a>

### .spec.components.prometheus.placement.nodeAffinity

#### Description

nodeAffinity describes node affinity scheduling rules for the pod.

#### Type

object

| Property                                                                                                                                                                                                   | Type           | Description                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    |
|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [preferredDuringSchedulingIgnoredDuringExecution](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-nodeaffinity-preferredduringschedulingignoredduringexecution) | array (object) | The scheduler will prefer to schedule pods to nodes that satisfy the affinity expressions specified by this field, but it may choose a node that violates one or more of the expressions. The node that is most preferred is the one with the greatest sum of weights, i.e. for each node that meets all of the scheduling requirements (resource request, requiredDuringScheduling affinity expressions, etc.), compute a sum by iterating through the elements of this field and adding “weight” to the sum if the node matches the corresponding matchExpressions; the node(s) with the highest sum are the most preferred. |
| [requiredDuringSchedulingIgnoredDuringExecution](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-nodeaffinity-requiredduringschedulingignoredduringexecution)   | object         | If the affinity requirements specified by this field are not met at scheduling time, the pod will not be scheduled onto the node. If the affinity requirements specified by this field cease to be met at some point during pod execution (e.g. due to an update), the system may or may not try to eventually evict the pod from its node.                                                                                                                                                                                                                                                                                    |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-nodeaffinity-preferredduringschedulingignoredduringexecution"></a>

### .spec.components.prometheus.placement.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[]

#### Description

An empty preferred scheduling term matches all objects with implicit weight 0 (i.e. it’s a no-op). A null preferred scheduling term matches no objects (i.e. is also a no-op).

#### Type

object

| Property                                                                                                                                                                         | Type    | Description                                                                             |
|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|---------|-----------------------------------------------------------------------------------------|
| [preference](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-nodeaffinity-preferredduringschedulingignoredduringexecution-preference) | object  | A node selector term, associated with the corresponding weight.                         |
| weight                                                                                                                                                                           | integer | Weight associated with matching the corresponding nodeSelectorTerm, in the range 1-100. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-nodeaffinity-preferredduringschedulingignoredduringexecution-preference"></a>

### .spec.components.prometheus.placement.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[].preference

#### Description

A node selector term, associated with the corresponding weight.

#### Type

object

| Property                                                                                                                                                                                                | Type           | Description                                            |
|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------|--------------------------------------------------------|
| [matchExpressions](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-nodeaffinity-preferredduringschedulingignoredduringexecution-preference-matchexpressions) | array (object) | A list of node selector requirements by node’s labels. |
| [matchFields](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-nodeaffinity-preferredduringschedulingignoredduringexecution-preference-matchfields)           | array (object) | A list of node selector requirements by node’s fields. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-nodeaffinity-preferredduringschedulingignoredduringexecution-preference-matchexpressions"></a>

### .spec.components.prometheus.placement.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[].preference.matchExpressions[]

#### Description

A node selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

#### Type

object

| Property   | Type           | Description                                                                                                                                                                                                                                                                                                                                         |
|------------|----------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| key        | string         | The label key that the selector applies to.                                                                                                                                                                                                                                                                                                         |
| operator   | string         | Represents a key’s relationship to a set of values. Valid operators are In, NotIn, Exists, DoesNotExist. Gt, and Lt.                                                                                                                                                                                                                                |
| values     | array (string) | An array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. If the operator is Gt or Lt, the values array must have a single element, which will be interpreted as an integer. This array is replaced during a strategic merge patch. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-nodeaffinity-preferredduringschedulingignoredduringexecution-preference-matchfields"></a>

### .spec.components.prometheus.placement.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[].preference.matchFields[]

#### Description

A node selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

#### Type

object

| Property   | Type           | Description                                                                                                                                                                                                                                                                                                                                         |
|------------|----------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| key        | string         | The label key that the selector applies to.                                                                                                                                                                                                                                                                                                         |
| operator   | string         | Represents a key’s relationship to a set of values. Valid operators are In, NotIn, Exists, DoesNotExist. Gt, and Lt.                                                                                                                                                                                                                                |
| values     | array (string) | An array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. If the operator is Gt or Lt, the values array must have a single element, which will be interpreted as an integer. This array is replaced during a strategic merge patch. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-nodeaffinity-requiredduringschedulingignoredduringexecution"></a>

### .spec.components.prometheus.placement.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution

#### Description

If the affinity requirements specified by this field are not met at scheduling time, the pod will not be scheduled onto the node. If the affinity requirements specified by this field cease to be met at some point during pod execution (e.g. due to an update), the system may or may not try to eventually evict the pod from its node.

#### Type

object

| Property                                                                                                                                                                                      | Type           | Description                                                  |
|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------|--------------------------------------------------------------|
| [nodeSelectorTerms](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-nodeaffinity-requiredduringschedulingignoredduringexecution-nodeselectorterms) | array (object) | Required. A list of node selector terms. The terms are ORed. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-nodeaffinity-requiredduringschedulingignoredduringexecution-nodeselectorterms"></a>

### .spec.components.prometheus.placement.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[]

#### Description

A null or empty node selector term matches no objects. The requirements of them are ANDed. The TopologySelectorTerm type implements a subset of the NodeSelectorTerm.

#### Type

object

| Property                                                                                                                                                                                                      | Type           | Description                                            |
|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------|--------------------------------------------------------|
| [matchExpressions](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-nodeaffinity-requiredduringschedulingignoredduringexecution-nodeselectorterms-matchexpressions) | array (object) | A list of node selector requirements by node’s labels. |
| [matchFields](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-nodeaffinity-requiredduringschedulingignoredduringexecution-nodeselectorterms-matchfields)           | array (object) | A list of node selector requirements by node’s fields. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-nodeaffinity-requiredduringschedulingignoredduringexecution-nodeselectorterms-matchexpressions"></a>

### .spec.components.prometheus.placement.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[].matchExpressions[]

#### Description

A node selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

#### Type

object

| Property   | Type           | Description                                                                                                                                                                                                                                                                                                                                         |
|------------|----------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| key        | string         | The label key that the selector applies to.                                                                                                                                                                                                                                                                                                         |
| operator   | string         | Represents a key’s relationship to a set of values. Valid operators are In, NotIn, Exists, DoesNotExist. Gt, and Lt.                                                                                                                                                                                                                                |
| values     | array (string) | An array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. If the operator is Gt or Lt, the values array must have a single element, which will be interpreted as an integer. This array is replaced during a strategic merge patch. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-nodeaffinity-requiredduringschedulingignoredduringexecution-nodeselectorterms-matchfields"></a>

### .spec.components.prometheus.placement.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[].matchFields[]

#### Description

A node selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

#### Type

object

| Property   | Type           | Description                                                                                                                                                                                                                                                                                                                                         |
|------------|----------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| key        | string         | The label key that the selector applies to.                                                                                                                                                                                                                                                                                                         |
| operator   | string         | Represents a key’s relationship to a set of values. Valid operators are In, NotIn, Exists, DoesNotExist. Gt, and Lt.                                                                                                                                                                                                                                |
| values     | array (string) | An array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. If the operator is Gt or Lt, the values array must have a single element, which will be interpreted as an integer. This array is replaced during a strategic merge patch. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podaffinity"></a>

### .spec.components.prometheus.placement.podAffinity

#### Description

podAffinity describes pod affinity scheduling rules.

#### Type

object

| Property                                                                                                                                                                                                  | Type           | Description                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  |
|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [preferredDuringSchedulingIgnoredDuringExecution](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podaffinity-preferredduringschedulingignoredduringexecution) | array (object) | The scheduler will prefer to schedule pods to nodes that satisfy the affinity expressions specified by this field, but it may choose a node that violates one or more of the expressions. The node that is most preferred is the one with the greatest sum of weights, i.e. for each node that meets all of the scheduling requirements (resource request, requiredDuringScheduling affinity expressions, etc.), compute a sum by iterating through the elements of this field and adding “weight” to the sum if the node has pods which matches the corresponding podAffinityTerm; the node(s) with the highest sum are the most preferred. |
| [requiredDuringSchedulingIgnoredDuringExecution](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podaffinity-requiredduringschedulingignoredduringexecution)   | array (object) | If the affinity requirements specified by this field are not met at scheduling time, the pod will not be scheduled onto the node. If the affinity requirements specified by this field cease to be met at some point during pod execution (e.g. due to a pod label update), the system may or may not try to eventually evict the pod from its node. When there are multiple elements, the lists of nodes corresponding to each podAffinityTerm are intersected, i.e. all terms must be satisfied.                                                                                                                                           |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podaffinity-preferredduringschedulingignoredduringexecution"></a>

### .spec.components.prometheus.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[]

#### Description

The weights of all of the matched WeightedPodAffinityTerm fields are added per-node to find the most preferred node(s)

#### Type

object

| Property                                                                                                                                                                                  | Type    | Description                                                                            |
|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|---------|----------------------------------------------------------------------------------------|
| [podAffinityTerm](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm) | object  | Required. A pod affinity term, associated with the corresponding weight.               |
| weight                                                                                                                                                                                    | integer | weight associated with matching the corresponding podAffinityTerm, in the range 1-100. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm"></a>

### .spec.components.prometheus.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm

#### Description

Required. A pod affinity term, associated with the corresponding weight.

#### Type

object

| Property                                                                                                                                                                                                      | Type           | Description                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [labelSelector](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-labelselector)         | object         | A label query over a set of resources, in this case pods. If it’s null, this PodAffinityTerm matches with no Pods.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| matchLabelKeys                                                                                                                                                                                                | array (string) | MatchLabelKeys is a set of pod label keys to select which pods will be taken into consideration. The keys are used to lookup values from the incoming pod labels, those key-value labels are merged with labelSelector as key in (value) to select the group of existing pods which pods will be taken into consideration for the incoming pod’s pod (anti) affinity. Keys that don’t exist in the incoming pod labels will be ignored. The default value is empty. The same key is forbidden to exist in both matchLabelKeys and labelSelector. Also, matchLabelKeys cannot be set when labelSelector isn’t set.             |
| mismatchLabelKeys                                                                                                                                                                                             | array (string) | MismatchLabelKeys is a set of pod label keys to select which pods will be taken into consideration. The keys are used to lookup values from the incoming pod labels, those key-value labels are merged with labelSelector as key notin (value) to select the group of existing pods which pods will be taken into consideration for the incoming pod’s pod (anti) affinity. Keys that don’t exist in the incoming pod labels will be ignored. The default value is empty. The same key is forbidden to exist in both mismatchLabelKeys and labelSelector. Also, mismatchLabelKeys cannot be set when labelSelector isn’t set. |
| [namespaceSelector](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-namespaceselector) | object         | A label query over the set of namespaces that the term applies to. The term is applied to the union of the namespaces selected by this field and the ones listed in the namespaces field. null selector and null or empty namespaces list means “this pod’s namespace”. An empty selector ({}) matches all namespaces.                                                                                                                                                                                                                                                                                                        |
| namespaces                                                                                                                                                                                                    | array (string) | namespaces specifies a static list of namespace names that the term applies to. The term is applied to the union of the namespaces listed in this field and the ones selected by namespaceSelector. null or empty namespaces list and null namespaceSelector means “this pod’s namespace”.                                                                                                                                                                                                                                                                                                                                    |
| topologyKey                                                                                                                                                                                                   | string         | This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching the labelSelector in the specified namespaces, where co-located is defined as running on a node whose value of the label with key topologyKey matches that of any node on which any of the selected pods is running. Empty topologyKey is not allowed.                                                                                                                                                                                                                                                                      |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-labelselector"></a>

### .spec.components.prometheus.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector

#### Description

A label query over a set of resources, in this case pods. If it’s null, this PodAffinityTerm matches with no Pods.

#### Type

object

| Property                                                                                                                                                                                                                  | Type           | Description                                                                                                                                                                                                                                                     |
|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [matchExpressions](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-labelselector-matchexpressions) | array (object) | matchExpressions is a list of label selector requirements. The requirements are ANDed.                                                                                                                                                                          |
| [matchLabels](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-labelselector-matchlabels)           | object         | matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is “key”, the operator is “In”, and the values array contains only “value”. The requirements are ANDed. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-labelselector-matchexpressions"></a>

### .spec.components.prometheus.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchExpressions[]

#### Description

A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

#### Type

object

| Property   | Type           | Description                                                                                                                                                                                                                                |
|------------|----------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| key        | string         | key is the label key that the selector applies to.                                                                                                                                                                                         |
| operator   | string         | operator represents a key’s relationship to a set of values. Valid operators are In, NotIn, Exists and DoesNotExist.                                                                                                                       |
| values     | array (string) | values is an array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-labelselector-matchlabels"></a>

### .spec.components.prometheus.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchLabels

#### Description

matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is “key”, the operator is “In”, and the values array contains only “value”. The requirements are ANDed.

#### Type

object

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-namespaceselector"></a>

### .spec.components.prometheus.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector

#### Description

A label query over the set of namespaces that the term applies to. The term is applied to the union of the namespaces selected by this field and the ones listed in the namespaces field. null selector and null or empty namespaces list means “this pod’s namespace”. An empty selector ({}) matches all namespaces.

#### Type

object

| Property                                                                                                                                                                                                                      | Type           | Description                                                                                                                                                                                                                                                     |
|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [matchExpressions](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-namespaceselector-matchexpressions) | array (object) | matchExpressions is a list of label selector requirements. The requirements are ANDed.                                                                                                                                                                          |
| [matchLabels](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-namespaceselector-matchlabels)           | object         | matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is “key”, the operator is “In”, and the values array contains only “value”. The requirements are ANDed. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-namespaceselector-matchexpressions"></a>

### .spec.components.prometheus.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchExpressions[]

#### Description

A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

#### Type

object

| Property   | Type           | Description                                                                                                                                                                                                                                |
|------------|----------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| key        | string         | key is the label key that the selector applies to.                                                                                                                                                                                         |
| operator   | string         | operator represents a key’s relationship to a set of values. Valid operators are In, NotIn, Exists and DoesNotExist.                                                                                                                       |
| values     | array (string) | values is an array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-namespaceselector-matchlabels"></a>

### .spec.components.prometheus.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchLabels

#### Description

matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is “key”, the operator is “In”, and the values array contains only “value”. The requirements are ANDed.

#### Type

object

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podaffinity-requiredduringschedulingignoredduringexecution"></a>

### .spec.components.prometheus.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[]

#### Description

Defines a set of pods (namely those matching the labelSelector relative to the given namespace(s)) that this pod should be co-located (affinity) or not co-located (anti-affinity) with, where co-located is defined as running on a node whose value of the label with key <topologyKey> matches that of any node on which a pod of the set of pods is running

#### Type

object

| Property                                                                                                                                                                                     | Type           | Description                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [labelSelector](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podaffinity-requiredduringschedulingignoredduringexecution-labelselector)         | object         | A label query over a set of resources, in this case pods. If it’s null, this PodAffinityTerm matches with no Pods.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| matchLabelKeys                                                                                                                                                                               | array (string) | MatchLabelKeys is a set of pod label keys to select which pods will be taken into consideration. The keys are used to lookup values from the incoming pod labels, those key-value labels are merged with labelSelector as key in (value) to select the group of existing pods which pods will be taken into consideration for the incoming pod’s pod (anti) affinity. Keys that don’t exist in the incoming pod labels will be ignored. The default value is empty. The same key is forbidden to exist in both matchLabelKeys and labelSelector. Also, matchLabelKeys cannot be set when labelSelector isn’t set.             |
| mismatchLabelKeys                                                                                                                                                                            | array (string) | MismatchLabelKeys is a set of pod label keys to select which pods will be taken into consideration. The keys are used to lookup values from the incoming pod labels, those key-value labels are merged with labelSelector as key notin (value) to select the group of existing pods which pods will be taken into consideration for the incoming pod’s pod (anti) affinity. Keys that don’t exist in the incoming pod labels will be ignored. The default value is empty. The same key is forbidden to exist in both mismatchLabelKeys and labelSelector. Also, mismatchLabelKeys cannot be set when labelSelector isn’t set. |
| [namespaceSelector](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podaffinity-requiredduringschedulingignoredduringexecution-namespaceselector) | object         | A label query over the set of namespaces that the term applies to. The term is applied to the union of the namespaces selected by this field and the ones listed in the namespaces field. null selector and null or empty namespaces list means “this pod’s namespace”. An empty selector ({}) matches all namespaces.                                                                                                                                                                                                                                                                                                        |
| namespaces                                                                                                                                                                                   | array (string) | namespaces specifies a static list of namespace names that the term applies to. The term is applied to the union of the namespaces listed in this field and the ones selected by namespaceSelector. null or empty namespaces list and null namespaceSelector means “this pod’s namespace”.                                                                                                                                                                                                                                                                                                                                    |
| topologyKey                                                                                                                                                                                  | string         | This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching the labelSelector in the specified namespaces, where co-located is defined as running on a node whose value of the label with key topologyKey matches that of any node on which any of the selected pods is running. Empty topologyKey is not allowed.                                                                                                                                                                                                                                                                      |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podaffinity-requiredduringschedulingignoredduringexecution-labelselector"></a>

### .spec.components.prometheus.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector

#### Description

A label query over a set of resources, in this case pods. If it’s null, this PodAffinityTerm matches with no Pods.

#### Type

object

| Property                                                                                                                                                                                                 | Type           | Description                                                                                                                                                                                                                                                     |
|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [matchExpressions](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podaffinity-requiredduringschedulingignoredduringexecution-labelselector-matchexpressions) | array (object) | matchExpressions is a list of label selector requirements. The requirements are ANDed.                                                                                                                                                                          |
| [matchLabels](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podaffinity-requiredduringschedulingignoredduringexecution-labelselector-matchlabels)           | object         | matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is “key”, the operator is “In”, and the values array contains only “value”. The requirements are ANDed. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podaffinity-requiredduringschedulingignoredduringexecution-labelselector-matchexpressions"></a>

### .spec.components.prometheus.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchExpressions[]

#### Description

A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

#### Type

object

| Property   | Type           | Description                                                                                                                                                                                                                                |
|------------|----------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| key        | string         | key is the label key that the selector applies to.                                                                                                                                                                                         |
| operator   | string         | operator represents a key’s relationship to a set of values. Valid operators are In, NotIn, Exists and DoesNotExist.                                                                                                                       |
| values     | array (string) | values is an array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podaffinity-requiredduringschedulingignoredduringexecution-labelselector-matchlabels"></a>

### .spec.components.prometheus.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchLabels

#### Description

matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is “key”, the operator is “In”, and the values array contains only “value”. The requirements are ANDed.

#### Type

object

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podaffinity-requiredduringschedulingignoredduringexecution-namespaceselector"></a>

### .spec.components.prometheus.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector

#### Description

A label query over the set of namespaces that the term applies to. The term is applied to the union of the namespaces selected by this field and the ones listed in the namespaces field. null selector and null or empty namespaces list means “this pod’s namespace”. An empty selector ({}) matches all namespaces.

#### Type

object

| Property                                                                                                                                                                                                     | Type           | Description                                                                                                                                                                                                                                                     |
|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [matchExpressions](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podaffinity-requiredduringschedulingignoredduringexecution-namespaceselector-matchexpressions) | array (object) | matchExpressions is a list of label selector requirements. The requirements are ANDed.                                                                                                                                                                          |
| [matchLabels](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podaffinity-requiredduringschedulingignoredduringexecution-namespaceselector-matchlabels)           | object         | matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is “key”, the operator is “In”, and the values array contains only “value”. The requirements are ANDed. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podaffinity-requiredduringschedulingignoredduringexecution-namespaceselector-matchexpressions"></a>

### .spec.components.prometheus.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchExpressions[]

#### Description

A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

#### Type

object

| Property   | Type           | Description                                                                                                                                                                                                                                |
|------------|----------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| key        | string         | key is the label key that the selector applies to.                                                                                                                                                                                         |
| operator   | string         | operator represents a key’s relationship to a set of values. Valid operators are In, NotIn, Exists and DoesNotExist.                                                                                                                       |
| values     | array (string) | values is an array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podaffinity-requiredduringschedulingignoredduringexecution-namespaceselector-matchlabels"></a>

### .spec.components.prometheus.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchLabels

#### Description

matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is “key”, the operator is “In”, and the values array contains only “value”. The requirements are ANDed.

#### Type

object

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podantiaffinity"></a>

### .spec.components.prometheus.placement.podAntiAffinity

#### Description

podAntiAffinity describes pod anti-affinity scheduling rules.

#### Type

object

| Property                                                                                                                                                                                                      | Type           | Description                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [preferredDuringSchedulingIgnoredDuringExecution](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podantiaffinity-preferredduringschedulingignoredduringexecution) | array (object) | The scheduler will prefer to schedule pods to nodes that satisfy the anti-affinity expressions specified by this field, but it may choose a node that violates one or more of the expressions. The node that is most preferred is the one with the greatest sum of weights, i.e. for each node that meets all of the scheduling requirements (resource request, requiredDuringScheduling anti-affinity expressions, etc.), compute a sum by iterating through the elements of this field and subtracting “weight” from the sum if the node has pods which matches the corresponding podAffinityTerm; the node(s) with the highest sum are the most preferred. |
| [requiredDuringSchedulingIgnoredDuringExecution](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podantiaffinity-requiredduringschedulingignoredduringexecution)   | array (object) | If the anti-affinity requirements specified by this field are not met at scheduling time, the pod will not be scheduled onto the node. If the anti-affinity requirements specified by this field cease to be met at some point during pod execution (e.g. due to a pod label update), the system may or may not try to eventually evict the pod from its node. When there are multiple elements, the lists of nodes corresponding to each podAffinityTerm are intersected, i.e. all terms must be satisfied.                                                                                                                                                  |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podantiaffinity-preferredduringschedulingignoredduringexecution"></a>

### .spec.components.prometheus.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[]

#### Description

The weights of all of the matched WeightedPodAffinityTerm fields are added per-node to find the most preferred node(s)

#### Type

object

| Property                                                                                                                                                                                      | Type    | Description                                                                            |
|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|---------|----------------------------------------------------------------------------------------|
| [podAffinityTerm](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podantiaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm) | object  | Required. A pod affinity term, associated with the corresponding weight.               |
| weight                                                                                                                                                                                        | integer | weight associated with matching the corresponding podAffinityTerm, in the range 1-100. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podantiaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm"></a>

### .spec.components.prometheus.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm

#### Description

Required. A pod affinity term, associated with the corresponding weight.

#### Type

object

| Property                                                                                                                                                                                                          | Type           | Description                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [labelSelector](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podantiaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-labelselector)         | object         | A label query over a set of resources, in this case pods. If it’s null, this PodAffinityTerm matches with no Pods.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| matchLabelKeys                                                                                                                                                                                                    | array (string) | MatchLabelKeys is a set of pod label keys to select which pods will be taken into consideration. The keys are used to lookup values from the incoming pod labels, those key-value labels are merged with labelSelector as key in (value) to select the group of existing pods which pods will be taken into consideration for the incoming pod’s pod (anti) affinity. Keys that don’t exist in the incoming pod labels will be ignored. The default value is empty. The same key is forbidden to exist in both matchLabelKeys and labelSelector. Also, matchLabelKeys cannot be set when labelSelector isn’t set.             |
| mismatchLabelKeys                                                                                                                                                                                                 | array (string) | MismatchLabelKeys is a set of pod label keys to select which pods will be taken into consideration. The keys are used to lookup values from the incoming pod labels, those key-value labels are merged with labelSelector as key notin (value) to select the group of existing pods which pods will be taken into consideration for the incoming pod’s pod (anti) affinity. Keys that don’t exist in the incoming pod labels will be ignored. The default value is empty. The same key is forbidden to exist in both mismatchLabelKeys and labelSelector. Also, mismatchLabelKeys cannot be set when labelSelector isn’t set. |
| [namespaceSelector](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podantiaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-namespaceselector) | object         | A label query over the set of namespaces that the term applies to. The term is applied to the union of the namespaces selected by this field and the ones listed in the namespaces field. null selector and null or empty namespaces list means “this pod’s namespace”. An empty selector ({}) matches all namespaces.                                                                                                                                                                                                                                                                                                        |
| namespaces                                                                                                                                                                                                        | array (string) | namespaces specifies a static list of namespace names that the term applies to. The term is applied to the union of the namespaces listed in this field and the ones selected by namespaceSelector. null or empty namespaces list and null namespaceSelector means “this pod’s namespace”.                                                                                                                                                                                                                                                                                                                                    |
| topologyKey                                                                                                                                                                                                       | string         | This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching the labelSelector in the specified namespaces, where co-located is defined as running on a node whose value of the label with key topologyKey matches that of any node on which any of the selected pods is running. Empty topologyKey is not allowed.                                                                                                                                                                                                                                                                      |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podantiaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-labelselector"></a>

### .spec.components.prometheus.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector

#### Description

A label query over a set of resources, in this case pods. If it’s null, this PodAffinityTerm matches with no Pods.

#### Type

object

| Property                                                                                                                                                                                                                      | Type           | Description                                                                                                                                                                                                                                                     |
|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [matchExpressions](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podantiaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-labelselector-matchexpressions) | array (object) | matchExpressions is a list of label selector requirements. The requirements are ANDed.                                                                                                                                                                          |
| [matchLabels](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podantiaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-labelselector-matchlabels)           | object         | matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is “key”, the operator is “In”, and the values array contains only “value”. The requirements are ANDed. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podantiaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-labelselector-matchexpressions"></a>

### .spec.components.prometheus.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchExpressions[]

#### Description

A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

#### Type

object

| Property   | Type           | Description                                                                                                                                                                                                                                |
|------------|----------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| key        | string         | key is the label key that the selector applies to.                                                                                                                                                                                         |
| operator   | string         | operator represents a key’s relationship to a set of values. Valid operators are In, NotIn, Exists and DoesNotExist.                                                                                                                       |
| values     | array (string) | values is an array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podantiaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-labelselector-matchlabels"></a>

### .spec.components.prometheus.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.labelSelector.matchLabels

#### Description

matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is “key”, the operator is “In”, and the values array contains only “value”. The requirements are ANDed.

#### Type

object

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podantiaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-namespaceselector"></a>

### .spec.components.prometheus.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector

#### Description

A label query over the set of namespaces that the term applies to. The term is applied to the union of the namespaces selected by this field and the ones listed in the namespaces field. null selector and null or empty namespaces list means “this pod’s namespace”. An empty selector ({}) matches all namespaces.

#### Type

object

| Property                                                                                                                                                                                                                          | Type           | Description                                                                                                                                                                                                                                                     |
|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [matchExpressions](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podantiaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-namespaceselector-matchexpressions) | array (object) | matchExpressions is a list of label selector requirements. The requirements are ANDed.                                                                                                                                                                          |
| [matchLabels](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podantiaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-namespaceselector-matchlabels)           | object         | matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is “key”, the operator is “In”, and the values array contains only “value”. The requirements are ANDed. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podantiaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-namespaceselector-matchexpressions"></a>

### .spec.components.prometheus.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchExpressions[]

#### Description

A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

#### Type

object

| Property   | Type           | Description                                                                                                                                                                                                                                |
|------------|----------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| key        | string         | key is the label key that the selector applies to.                                                                                                                                                                                         |
| operator   | string         | operator represents a key’s relationship to a set of values. Valid operators are In, NotIn, Exists and DoesNotExist.                                                                                                                       |
| values     | array (string) | values is an array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podantiaffinity-preferredduringschedulingignoredduringexecution-podaffinityterm-namespaceselector-matchlabels"></a>

### .spec.components.prometheus.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[].podAffinityTerm.namespaceSelector.matchLabels

#### Description

matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is “key”, the operator is “In”, and the values array contains only “value”. The requirements are ANDed.

#### Type

object

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podantiaffinity-requiredduringschedulingignoredduringexecution"></a>

### .spec.components.prometheus.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[]

#### Description

Defines a set of pods (namely those matching the labelSelector relative to the given namespace(s)) that this pod should be co-located (affinity) or not co-located (anti-affinity) with, where co-located is defined as running on a node whose value of the label with key <topologyKey> matches that of any node on which a pod of the set of pods is running

#### Type

object

| Property                                                                                                                                                                                         | Type           | Description                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [labelSelector](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podantiaffinity-requiredduringschedulingignoredduringexecution-labelselector)         | object         | A label query over a set of resources, in this case pods. If it’s null, this PodAffinityTerm matches with no Pods.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| matchLabelKeys                                                                                                                                                                                   | array (string) | MatchLabelKeys is a set of pod label keys to select which pods will be taken into consideration. The keys are used to lookup values from the incoming pod labels, those key-value labels are merged with labelSelector as key in (value) to select the group of existing pods which pods will be taken into consideration for the incoming pod’s pod (anti) affinity. Keys that don’t exist in the incoming pod labels will be ignored. The default value is empty. The same key is forbidden to exist in both matchLabelKeys and labelSelector. Also, matchLabelKeys cannot be set when labelSelector isn’t set.             |
| mismatchLabelKeys                                                                                                                                                                                | array (string) | MismatchLabelKeys is a set of pod label keys to select which pods will be taken into consideration. The keys are used to lookup values from the incoming pod labels, those key-value labels are merged with labelSelector as key notin (value) to select the group of existing pods which pods will be taken into consideration for the incoming pod’s pod (anti) affinity. Keys that don’t exist in the incoming pod labels will be ignored. The default value is empty. The same key is forbidden to exist in both mismatchLabelKeys and labelSelector. Also, mismatchLabelKeys cannot be set when labelSelector isn’t set. |
| [namespaceSelector](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podantiaffinity-requiredduringschedulingignoredduringexecution-namespaceselector) | object         | A label query over the set of namespaces that the term applies to. The term is applied to the union of the namespaces selected by this field and the ones listed in the namespaces field. null selector and null or empty namespaces list means “this pod’s namespace”. An empty selector ({}) matches all namespaces.                                                                                                                                                                                                                                                                                                        |
| namespaces                                                                                                                                                                                       | array (string) | namespaces specifies a static list of namespace names that the term applies to. The term is applied to the union of the namespaces listed in this field and the ones selected by namespaceSelector. null or empty namespaces list and null namespaceSelector means “this pod’s namespace”.                                                                                                                                                                                                                                                                                                                                    |
| topologyKey                                                                                                                                                                                      | string         | This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching the labelSelector in the specified namespaces, where co-located is defined as running on a node whose value of the label with key topologyKey matches that of any node on which any of the selected pods is running. Empty topologyKey is not allowed.                                                                                                                                                                                                                                                                      |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podantiaffinity-requiredduringschedulingignoredduringexecution-labelselector"></a>

### .spec.components.prometheus.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector

#### Description

A label query over a set of resources, in this case pods. If it’s null, this PodAffinityTerm matches with no Pods.

#### Type

object

| Property                                                                                                                                                                                                     | Type           | Description                                                                                                                                                                                                                                                     |
|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [matchExpressions](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podantiaffinity-requiredduringschedulingignoredduringexecution-labelselector-matchexpressions) | array (object) | matchExpressions is a list of label selector requirements. The requirements are ANDed.                                                                                                                                                                          |
| [matchLabels](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podantiaffinity-requiredduringschedulingignoredduringexecution-labelselector-matchlabels)           | object         | matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is “key”, the operator is “In”, and the values array contains only “value”. The requirements are ANDed. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podantiaffinity-requiredduringschedulingignoredduringexecution-labelselector-matchexpressions"></a>

### .spec.components.prometheus.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchExpressions[]

#### Description

A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

#### Type

object

| Property   | Type           | Description                                                                                                                                                                                                                                |
|------------|----------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| key        | string         | key is the label key that the selector applies to.                                                                                                                                                                                         |
| operator   | string         | operator represents a key’s relationship to a set of values. Valid operators are In, NotIn, Exists and DoesNotExist.                                                                                                                       |
| values     | array (string) | values is an array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podantiaffinity-requiredduringschedulingignoredduringexecution-labelselector-matchlabels"></a>

### .spec.components.prometheus.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].labelSelector.matchLabels

#### Description

matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is “key”, the operator is “In”, and the values array contains only “value”. The requirements are ANDed.

#### Type

object

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podantiaffinity-requiredduringschedulingignoredduringexecution-namespaceselector"></a>

### .spec.components.prometheus.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector

#### Description

A label query over the set of namespaces that the term applies to. The term is applied to the union of the namespaces selected by this field and the ones listed in the namespaces field. null selector and null or empty namespaces list means “this pod’s namespace”. An empty selector ({}) matches all namespaces.

#### Type

object

| Property                                                                                                                                                                                                         | Type           | Description                                                                                                                                                                                                                                                     |
|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [matchExpressions](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podantiaffinity-requiredduringschedulingignoredduringexecution-namespaceselector-matchexpressions) | array (object) | matchExpressions is a list of label selector requirements. The requirements are ANDed.                                                                                                                                                                          |
| [matchLabels](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podantiaffinity-requiredduringschedulingignoredduringexecution-namespaceselector-matchlabels)           | object         | matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is “key”, the operator is “In”, and the values array contains only “value”. The requirements are ANDed. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podantiaffinity-requiredduringschedulingignoredduringexecution-namespaceselector-matchexpressions"></a>

### .spec.components.prometheus.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchExpressions[]

#### Description

A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

#### Type

object

| Property   | Type           | Description                                                                                                                                                                                                                                |
|------------|----------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| key        | string         | key is the label key that the selector applies to.                                                                                                                                                                                         |
| operator   | string         | operator represents a key’s relationship to a set of values. Valid operators are In, NotIn, Exists and DoesNotExist.                                                                                                                       |
| values     | array (string) | values is an array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-podantiaffinity-requiredduringschedulingignoredduringexecution-namespaceselector-matchlabels"></a>

### .spec.components.prometheus.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[].namespaceSelector.matchLabels

#### Description

matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is “key”, the operator is “In”, and the values array contains only “value”. The requirements are ANDed.

#### Type

object

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-placement-tolerations"></a>

### .spec.components.prometheus.placement.tolerations[]

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

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-resources"></a>

### .spec.components.prometheus.resources

#### Description

resources the Prometheus container will use.

#### Type

object

| Property                                                                                                        | Type           | Description                                                                                                                                                                                                                                                                                                                                                                                                                  |
|-----------------------------------------------------------------------------------------------------------------|----------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [claims](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-resources-claims)     | array (object) | Claims lists the names of resources, defined in spec.resourceClaims, that are used by this container.  This field depends on the DynamicResourceAllocation feature gate.  This field is immutable. It can only be set for containers.                                                                                                                                                                                        |
| [limits](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-resources-limits)     | object         | Limits describes the maximum amount of compute resources allowed. More info: [https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/)                                                                                                                                                                                |
| [requests](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-resources-requests) | object         | Requests describes the minimum amount of compute resources required. If Requests is omitted for a container, it defaults to Limits if that is explicitly specified, otherwise to an implementation-defined value. Requests cannot exceed Limits. More info: [https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/) |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-resources-claims"></a>

### .spec.components.prometheus.resources.claims[]

#### Description

ResourceClaim references one entry in PodSpec.ResourceClaims.

#### Type

object

| Property   | Type   | Description                                                                                                                                                         |
|------------|--------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| name       | string | Name must match the name of one entry in pod.spec.resourceClaims of the Pod where this field is used. It makes that resource available inside a container.          |
| request    | string | Request is the name chosen for a request in the referenced claim. If empty, everything from the claim is made available, otherwise only the result of this request. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-resources-limits"></a>

### .spec.components.prometheus.resources.limits

#### Description

Limits describes the maximum amount of compute resources allowed. More info: [https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/)

#### Type

object

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-resources-requests"></a>

### .spec.components.prometheus.resources.requests

#### Description

Requests describes the minimum amount of compute resources required. If Requests is omitted for a container, it defaults to Limits if that is explicitly specified, otherwise to an implementation-defined value. Requests cannot exceed Limits. More info: [https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/)

#### Type

object

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-storage"></a>

### .spec.components.prometheus.storage

#### Description

storage describes the underlying storage that Prometheus will consume.

#### Type

object

| Property                                                                                                                            | Type   | Description                                                                                                                                                                                                                                                                                                                            |
|-------------------------------------------------------------------------------------------------------------------------------------|--------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [annotations](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-storage-annotations)                 | object | Annotations is an unstructured key value map stored with a resource that may be set by external tools to store and retrieve arbitrary metadata. They are not queryable and should be preserved when modifying objects. More info: [http://kubernetes.io/docs/user-guide/annotations](http://kubernetes.io/docs/user-guide/annotations) |
| [labels](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-storage-labels)                           | object | Map of string keys and values that can be used to organize and categorize (scope and select) objects. May match selectors of replication controllers and services. More info: [http://kubernetes.io/docs/user-guide/labels](http://kubernetes.io/docs/user-guide/labels)                                                               |
| [volumeClaimTemplate](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-storage-volumeclaimtemplate) | object | volumeClaimTemplates is a PVC template defining storage to be used by Prometheus.                                                                                                                                                                                                                                                      |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-storage-annotations"></a>

### .spec.components.prometheus.storage.annotations

#### Description

Annotations is an unstructured key value map stored with a resource that may be set by external tools to store and retrieve arbitrary metadata. They are not queryable and should be preserved when modifying objects. More info: [http://kubernetes.io/docs/user-guide/annotations](http://kubernetes.io/docs/user-guide/annotations)

#### Type

object

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-storage-labels"></a>

### .spec.components.prometheus.storage.labels

#### Description

Map of string keys and values that can be used to organize and categorize (scope and select) objects. May match selectors of replication controllers and services. More info: [http://kubernetes.io/docs/user-guide/labels](http://kubernetes.io/docs/user-guide/labels)

#### Type

object

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-storage-volumeclaimtemplate"></a>

### .spec.components.prometheus.storage.volumeClaimTemplate

#### Description

volumeClaimTemplates is a PVC template defining storage to be used by Prometheus.

#### Type

object

| Property                                                                                                                          | Type   | Description                                                                                                                                                                                                   |
|-----------------------------------------------------------------------------------------------------------------------------------|--------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [metadata](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-storage-volumeclaimtemplate-metadata) | object | May contain labels and annotations that will be copied into the PVC when creating it. No other fields are allowed and will be rejected during validation.                                                     |
| [spec](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-storage-volumeclaimtemplate-spec)         | object | The specification for the PersistentVolumeClaim. The entire content is copied unchanged into the PVC that gets created from this template. The same fields as in a PersistentVolumeClaim are also valid here. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-storage-volumeclaimtemplate-metadata"></a>

### .spec.components.prometheus.storage.volumeClaimTemplate.metadata

#### Description

May contain labels and annotations that will be copied into the PVC when creating it. No other fields are allowed and will be rejected during validation.

#### Type

object

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-storage-volumeclaimtemplate-spec"></a>

### .spec.components.prometheus.storage.volumeClaimTemplate.spec

#### Description

The specification for the PersistentVolumeClaim. The entire content is copied unchanged into the PVC that gets created from this template. The same fields as in a PersistentVolumeClaim are also valid here.

#### Type

object

| Property                                                                                                                                         | Type           | Description                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
|--------------------------------------------------------------------------------------------------------------------------------------------------|----------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| accessModes                                                                                                                                      | array (string) | accessModes contains the desired access modes the volume should have. More info: [https://kubernetes.io/docs/concepts/storage/persistent-volumes#access-modes-1](https://kubernetes.io/docs/concepts/storage/persistent-volumes#access-modes-1)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
| [dataSource](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-storage-volumeclaimtemplate-spec-datasource)       | object         | dataSource field can be used to specify either: \* An existing VolumeSnapshot object (snapshot.storage.k8s.io/VolumeSnapshot) \* An existing PVC (PersistentVolumeClaim) If the provisioner or an external controller can support the specified data source, it will create a new volume based on the contents of the specified data source. When the AnyVolumeDataSource feature gate is enabled, dataSource contents will be copied to dataSourceRef, and dataSourceRef contents will be copied to dataSource when dataSourceRef.namespace is not specified. If the namespace is specified, then dataSourceRef will not be copied to dataSource.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          |
| [dataSourceRef](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-storage-volumeclaimtemplate-spec-datasourceref) | object         | dataSourceRef specifies the object from which to populate the volume with data, if a non-empty volume is desired. This may be any object from a non-empty API group (non core object) or a PersistentVolumeClaim object. When this field is specified, volume binding will only succeed if the type of the specified object matches some installed volume populator or dynamic provisioner. This field will replace the functionality of the dataSource field and as such if both fields are non-empty, they must have the same value. For backwards compatibility, when namespace isn’t specified in dataSourceRef, both fields (dataSource and dataSourceRef) will be set to the same value automatically if one of them is empty and the other is non-empty. When namespace is specified in dataSourceRef, dataSource isn’t set to the same value and must be empty. There are three important differences between dataSource and dataSourceRef: \* While dataSource only allows two specific types of objects, dataSourceRef   allows any non-core object, as well as PersistentVolumeClaim objects. \* While dataSource ignores disallowed values (dropping them), dataSourceRef   preserves all values, and generates an error if a disallowed value is   specified. \* While dataSource only allows local objects, dataSourceRef allows objects   in any namespaces. (Beta) Using this field requires the AnyVolumeDataSource feature gate to be enabled. (Alpha) Using the namespace field of dataSourceRef requires the CrossNamespaceVolumeDataSource feature gate to be enabled. |
| [resources](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-storage-volumeclaimtemplate-spec-resources)         | object         | resources represents the minimum resources the volume should have. Users are allowed to specify resource requirements that are lower than previous value but must still be higher than capacity recorded in the status field of the claim. More info: [https://kubernetes.io/docs/concepts/storage/persistent-volumes#resources](https://kubernetes.io/docs/concepts/storage/persistent-volumes#resources)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  |
| [selector](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-storage-volumeclaimtemplate-spec-selector)           | object         | selector is a label query over volumes to consider for binding.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
| storageClassName                                                                                                                                 | string         | storageClassName is the name of the StorageClass required by the claim. More info: [https://kubernetes.io/docs/concepts/storage/persistent-volumes#class-1](https://kubernetes.io/docs/concepts/storage/persistent-volumes#class-1)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         |
| volumeAttributesClassName                                                                                                                        | string         | volumeAttributesClassName may be used to set the VolumeAttributesClass used by this claim. If specified, the CSI driver will create or update the volume with the attributes defined in the corresponding VolumeAttributesClass. This has a different purpose than storageClassName, it can be changed after the claim is created. An empty string or nil value indicates that no VolumeAttributesClass will be applied to the claim. If the claim enters an Infeasible error state, this field can be reset to its previous value (including nil) to cancel the modification. If the resource referred to by volumeAttributesClass does not exist, this PersistentVolumeClaim will be set to a Pending state, as reflected by the modifyVolumeStatus field, until such as a resource exists. More info: [https://kubernetes.io/docs/concepts/storage/volume-attributes-classes/](https://kubernetes.io/docs/concepts/storage/volume-attributes-classes/)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| volumeMode                                                                                                                                       | string         | volumeMode defines what type of volume is required by the claim. Value of Filesystem is implied when not included in claim spec.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| volumeName                                                                                                                                       | string         | volumeName is the binding reference to the PersistentVolume backing this claim.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-storage-volumeclaimtemplate-spec-datasource"></a>

### .spec.components.prometheus.storage.volumeClaimTemplate.spec.dataSource

#### Description

dataSource field can be used to specify either: \* An existing VolumeSnapshot object (snapshot.storage.k8s.io/VolumeSnapshot) \* An existing PVC (PersistentVolumeClaim) If the provisioner or an external controller can support the specified data source, it will create a new volume based on the contents of the specified data source. When the AnyVolumeDataSource feature gate is enabled, dataSource contents will be copied to dataSourceRef, and dataSourceRef contents will be copied to dataSource when dataSourceRef.namespace is not specified. If the namespace is specified, then dataSourceRef will not be copied to dataSource.

#### Type

object

| Property   | Type   | Description                                                                                                                                                                                     |
|------------|--------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| apiGroup   | string | APIGroup is the group for the resource being referenced. If APIGroup is not specified, the specified Kind must be in the core API group. For any other third-party types, APIGroup is required. |
| kind       | string | Kind is the type of resource being referenced                                                                                                                                                   |
| name       | string | Name is the name of resource being referenced                                                                                                                                                   |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-storage-volumeclaimtemplate-spec-datasourceref"></a>

### .spec.components.prometheus.storage.volumeClaimTemplate.spec.dataSourceRef

#### Description

dataSourceRef specifies the object from which to populate the volume with data, if a non-empty volume is desired. This may be any object from a non-empty API group (non core object) or a PersistentVolumeClaim object. When this field is specified, volume binding will only succeed if the type of the specified object matches some installed volume populator or dynamic provisioner. This field will replace the functionality of the dataSource field and as such if both fields are non-empty, they must have the same value. For backwards compatibility, when namespace isn’t specified in dataSourceRef, both fields (dataSource and dataSourceRef) will be set to the same value automatically if one of them is empty and the other is non-empty. When namespace is specified in dataSourceRef, dataSource isn’t set to the same value and must be empty. There are three important differences between dataSource and dataSourceRef: \* While dataSource only allows two specific types of objects, dataSourceRef   allows any non-core object, as well as PersistentVolumeClaim objects. \* While dataSource ignores disallowed values (dropping them), dataSourceRef   preserves all values, and generates an error if a disallowed value is   specified. \* While dataSource only allows local objects, dataSourceRef allows objects   in any namespaces. (Beta) Using this field requires the AnyVolumeDataSource feature gate to be enabled. (Alpha) Using the namespace field of dataSourceRef requires the CrossNamespaceVolumeDataSource feature gate to be enabled.

#### Type

object

| Property   | Type   | Description                                                                                                                                                                                                                                                                                                                                                                                    |
|------------|--------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| apiGroup   | string | APIGroup is the group for the resource being referenced. If APIGroup is not specified, the specified Kind must be in the core API group. For any other third-party types, APIGroup is required.                                                                                                                                                                                                |
| kind       | string | Kind is the type of resource being referenced                                                                                                                                                                                                                                                                                                                                                  |
| name       | string | Name is the name of resource being referenced                                                                                                                                                                                                                                                                                                                                                  |
| namespace  | string | Namespace is the namespace of resource being referenced Note that when a namespace is specified, a gateway.networking.k8s.io/ReferenceGrant object is required in the referent namespace to allow that namespace’s owner to accept the reference. See the ReferenceGrant documentation for details. (Alpha) This field requires the CrossNamespaceVolumeDataSource feature gate to be enabled. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-storage-volumeclaimtemplate-spec-resources"></a>

### .spec.components.prometheus.storage.volumeClaimTemplate.spec.resources

#### Description

resources represents the minimum resources the volume should have. Users are allowed to specify resource requirements that are lower than previous value but must still be higher than capacity recorded in the status field of the claim. More info: [https://kubernetes.io/docs/concepts/storage/persistent-volumes#resources](https://kubernetes.io/docs/concepts/storage/persistent-volumes#resources)

#### Type

object

| Property                                                                                                                                         | Type   | Description                                                                                                                                                                                                                                                                                                                                                                                                                  |
|--------------------------------------------------------------------------------------------------------------------------------------------------|--------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [limits](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-storage-volumeclaimtemplate-spec-resources-limits)     | object | Limits describes the maximum amount of compute resources allowed. More info: [https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/)                                                                                                                                                                                |
| [requests](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-storage-volumeclaimtemplate-spec-resources-requests) | object | Requests describes the minimum amount of compute resources required. If Requests is omitted for a container, it defaults to Limits if that is explicitly specified, otherwise to an implementation-defined value. Requests cannot exceed Limits. More info: [https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/) |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-storage-volumeclaimtemplate-spec-resources-limits"></a>

### .spec.components.prometheus.storage.volumeClaimTemplate.spec.resources.limits

#### Description

Limits describes the maximum amount of compute resources allowed. More info: [https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/)

#### Type

object

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-storage-volumeclaimtemplate-spec-resources-requests"></a>

### .spec.components.prometheus.storage.volumeClaimTemplate.spec.resources.requests

#### Description

Requests describes the minimum amount of compute resources required. If Requests is omitted for a container, it defaults to Limits if that is explicitly specified, otherwise to an implementation-defined value. Requests cannot exceed Limits. More info: [https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/)

#### Type

object

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-storage-volumeclaimtemplate-spec-selector"></a>

### .spec.components.prometheus.storage.volumeClaimTemplate.spec.selector

#### Description

selector is a label query over volumes to consider for binding.

#### Type

object

| Property                                                                                                                                                        | Type           | Description                                                                                                                                                                                                                                                     |
|-----------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [matchExpressions](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-storage-volumeclaimtemplate-spec-selector-matchexpressions) | array (object) | matchExpressions is a list of label selector requirements. The requirements are ANDed.                                                                                                                                                                          |
| [matchLabels](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-storage-volumeclaimtemplate-spec-selector-matchlabels)           | object         | matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is “key”, the operator is “In”, and the values array contains only “value”. The requirements are ANDed. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-storage-volumeclaimtemplate-spec-selector-matchexpressions"></a>

### .spec.components.prometheus.storage.volumeClaimTemplate.spec.selector.matchExpressions[]

#### Description

A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

#### Type

object

| Property   | Type           | Description                                                                                                                                                                                                                                |
|------------|----------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| key        | string         | key is the label key that the selector applies to.                                                                                                                                                                                         |
| operator   | string         | operator represents a key’s relationship to a set of values. Valid operators are In, NotIn, Exists and DoesNotExist.                                                                                                                       |
| values     | array (string) | values is an array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-components-prometheus-storage-volumeclaimtemplate-spec-selector-matchlabels"></a>

### .spec.components.prometheus.storage.volumeClaimTemplate.spec.selector.matchLabels

#### Description

matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is “key”, the operator is “In”, and the values array contains only “value”. The requirements are ANDed.

#### Type

object

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-endpointsselector"></a>

### .spec.endpointsSelector

#### Description

endpointsSelector select which Endpoints should be scraped. For local ScyllaDB clusters or datacenters, this is the same selector as if you were trying to select member Services. For remote ScyllaDB clusters, this can select any endpoints that are created manually or for a Service without selectors.

#### Type

object

| Property                                                                                                          | Type           | Description                                                                                                                                                                                                                                                     |
|-------------------------------------------------------------------------------------------------------------------|----------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [matchExpressions](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-endpointsselector-matchexpressions) | array (object) | matchExpressions is a list of label selector requirements. The requirements are ANDed.                                                                                                                                                                          |
| [matchLabels](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-endpointsselector-matchlabels)           | object         | matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is “key”, the operator is “In”, and the values array contains only “value”. The requirements are ANDed. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-endpointsselector-matchexpressions"></a>

### .spec.endpointsSelector.matchExpressions[]

#### Description

A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

#### Type

object

| Property   | Type           | Description                                                                                                                                                                                                                                |
|------------|----------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| key        | string         | key is the label key that the selector applies to.                                                                                                                                                                                         |
| operator   | string         | operator represents a key’s relationship to a set of values. Valid operators are In, NotIn, Exists and DoesNotExist.                                                                                                                       |
| values     | array (string) | values is an array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-spec-endpointsselector-matchlabels"></a>

### .spec.endpointsSelector.matchLabels

#### Description

matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is “key”, the operator is “In”, and the values array contains only “value”. The requirements are ANDed.

#### Type

object

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-status"></a>

### .status

#### Description

status is the current status of this ScyllaDBMonitoring.

#### Type

object

| Property                                                                              | Type           | Description                                                                                                                                                                                   |
|---------------------------------------------------------------------------------------|----------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [conditions](#api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-status-conditions) | array (object) | conditions hold conditions describing ScyllaDBMonitoring state. To determine whether a cluster rollout is finished, look for Available=True,Progressing=False,Degraded=False.                 |
| observedGeneration                                                                    | integer        | observedGeneration is the most recent generation observed for this ScyllaDBMonitoring. It corresponds to the ScyllaDBMonitoring’s generation, which is updated on mutation by the API Server. |

<a id="api-scylla-scylladb-com-scylladbmonitorings-v1alpha1-status-conditions"></a>

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
