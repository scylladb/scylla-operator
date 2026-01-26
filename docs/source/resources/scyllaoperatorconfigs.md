# ScyllaOperatorConfigs

[ScyllaOperatorConfig](../reference/api/groups/scylla.scylladb.com/scyllaoperatorconfigs.rst) holds the global configuration for {{productName}}.
It is automatically created by {{productName}}, if it's missing, and also reports some of the configuration (like auxiliary images) that are in use.

This is a singleton resource where only a single instance named `cluster` is valid.

:::{code-block} yaml
:linenos:

apiVersion: scylla.scylladb.com/v1alpha1
kind: ScyllaOperatorConfig
metadata:
  name: cluster
  # ...
spec:
  scyllaUtilsImage: ""
status:
  bashToolsImage: registry.access.redhat.com/ubi9/ubi:9.3-1361.1699548029@sha256:6b95efc134c2af3d45472c0a2f88e6085433df058cc210abb2bb061ac4d74359
  grafanaImage: docker.io/grafana/grafana:9.5.12@sha256:7d2f2a8b7aebe30bf3f9ae0f190e508e571b43f65753ba3b1b1adf0800bc9256
  observedGeneration: 1
  prometheusVersion: v2.44.0
  scyllaDBUtilsImage: docker.io/scylladb/scylla:5.4.0@sha256:b9070afdb2be0d5c59b1c196e1bb66660351403cb30d5c6ba446ef8c3b0754f1
:::

## Tuning with ScyllaDB Enterprise

To enable tuning using the scripts from ScyllaDB Enterprise you have to adjust the `scyllaUtilsImage`
:::{code-block} yaml
:substitutions:

 spec:
   scyllaUtilsImage: "{{enterpriseImageRepository}}:{{scyllaDBImageTag}}"
:::
