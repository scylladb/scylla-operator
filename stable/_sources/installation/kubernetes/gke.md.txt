# GKE

## Kubelet

### Static CPU policy

GKE allows you to set [static CPU policy](./generic.md#static-cpu-policy) using a [node system configuration](https://cloud.google.com/kubernetes-engine/docs/how-to/node-system-config)
:::{code} yaml
:number-lines:
kubeletConfig:
  cpuManagerPolicy: static
:::
