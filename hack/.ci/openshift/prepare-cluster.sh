#!/usr/bin/env bash
#
# Copyright (C) 2026 ScyllaDB
#
# This script prepares a freshly provisioned OpenShift cluster for the e2e suites, applying what the KubernetesCluster
# can't set up itself: the ScyllaDB node label and the static CPU manager policy on the workers.

set -euExo pipefail
shopt -s inherit_errexit

# The KubernetesCluster has no dedicated node pools, so ScyllaDB runs on the workers.
kubectl label node --overwrite --selector=node-role.kubernetes.io/worker="" scylla.scylladb.com/node-type=scylla

kubectl label machineconfigpool/worker --overwrite cpumanager-policy=static
kubectl apply --server-side -f=- <<EOM
apiVersion: machineconfiguration.openshift.io/v1
kind: KubeletConfig
metadata:
  name: cpumanager-policy-static
spec:
  machineConfigPoolSelector:
    matchLabels:
      cpumanager-policy: static
  kubeletConfig:
    cpuManagerPolicy: static
EOM

# A fresh cluster's worker pool can be mid-update for reasons of its own, so the pool's conditions alone could report
# a rollout that doesn't include the KubeletConfig yet. The pool's status only lists the MachineConfig rendered from it
# once every worker runs it, and it updates them one at a time.
kubectl wait --for=condition=Success=True kubeletconfig/cpumanager-policy-static --timeout=10m
KUBELET_MC="$( kubectl get machineconfigs -o=json | jq -er '.items[] | select(any(.metadata.ownerReferences[]?; .kind == "KubeletConfig" and .name == "cpumanager-policy-static")) | .metadata.name' )"
export KUBELET_MC
timeout --verbose 30m bash -c 'until kubectl get machineconfigpool/worker -o=json | jq -e "any(.status.configuration.source[]?; .name == env.KUBELET_MC)" >/dev/null; do sleep 15; done'

# Rolling out the KubeletConfig restarts the workers, which disrupts the cluster operators running there.
kubectl wait --timeout=10m --all clusteroperators \
  --for=condition=Available=True \
  --for=condition=Progressing=False \
  --for=condition=Degraded=False
