# EKS

## Kubelet

### Static CPU policy

`eksctl` allows you to set [static CPU policy](./generic.md#static-cpu-policy) for each node pool like:
```{code} yaml
:number-lines:
apiVersion: eksctl.io/v1alpha5
kind: ClusterConfig
# ...
nodeGroups:
- name: scylla-pool
  kubeletExtraConfig:
    cpuManagerPolicy: static
```
