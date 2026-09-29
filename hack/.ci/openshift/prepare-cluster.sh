#!/usr/bin/env bash
#
# Copyright (C) 2026 ScyllaDB
#
# This script prepares a freshly provisioned OpenShift cluster for the e2e suites, applying what the KubernetesCluster
# can't set up itself: the ScyllaDB node label and the static CPU manager policy on the workers, and the default
# OperatorHub sources.

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

# The pool starts updating once the KubeletConfig is rendered into a MachineConfig, and updates the workers one at a
# time.
kubectl wait --for=condition=Updating=True machineconfigpool/worker --timeout=10m
kubectl wait --for=condition=Updated=True machineconfigpool/worker --timeout=30m

kubectl patch operatorhub/cluster --type=json --patch='[{"op": "replace", "path": "/spec/disableAllDefaultSources", "value": false}, {"op": "replace", "path": "/spec/sources", "value": []}]'
