#!/bin/bash

HOSTFS="/mnt/hostfs"

function sleep_forever() {
    while true; do sleep 100; done
}

function setup_kubectl() {
# Setup kubectl
    cd $HOSTFS/var/run/secrets/kubernetes.io/serviceaccount
    TOKEN=$(cat token)
    kubectl config set-cluster scylla --server=https://kubernetes.default --certificate-authority=ca.crt
    kubectl config set-credentials yanniszarkadas@gmail.com --token=$TOKEN
    kubectl config set-context scylla --cluster=scylla --user=yanniszarkadas@gmail.com
    kubectl config use-context scylla
}

setup_kubectl
echo "Changing kubelet configs and restarting the kubelet service"
if [[ `grep "cpuManagerPolicy" $HOSTFS/home/kubernetes/kubelet-config.yaml | wc -l` -eq 0 ]]
then
  kubectl drain $NODE --force --ignore-daemonsets --delete-local-data --grace-period=60
  echo cpuManagerPolicy: static | tee -a $HOSTFS/home/kubernetes/kubelet-config.yaml
elif [[ `grep "cpuManagerPolicy" $HOSTFS/home/kubernetes/kubelet-config.yaml | wc -l` -gt 1 ]]
then
  kubectl drain $NODE --force --ignore-daemonsets --delete-local-data --grace-period=60
  sed -i '/cpuManagerPolicy/d' $HOSTFS/home/kubernetes/kubelet-config.yaml
  echo cpuManagerPolicy: static | tee -a $HOSTFS/home/kubernetes/kubelet-config.yaml
else
  echo "Policy already there!"
  echo "Uncondoning the node"
  kubectl uncordon $NODE
  sleep_forever
fi
rm $HOSTFS/var/lib/kubelet/cpu_manager_state

# Couldn't get systemctl to work...
# Not a good solution, but it works
kill -9 $(pidof kubelet)

# systemctl restart kubelet
# echo "Here is the result"
# sudo journalctl -u kubelet | grep cpumanager
# echo "Uncondoning the node"
# kubectl uncordon $NODE
