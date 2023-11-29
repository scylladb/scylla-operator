#!/usr/bin/bash

set -euEo pipefail
shopt -s inherit_errexit

tmpdir="$( mktemp -d )"

trap 'rm -rf -- "${tmpdir}" && job_ids="$( jobs -p )" && if [[ -n "${job_ids}" ]]; then kill ${job_ids}; fi && wait' EXIT

default_ps1="\n\e[0;34m\$ \e[0m"
CUSTOM_PS1="${CUSTOM_PS1:-${default_ps1}}"
CUSTOM_PS1_SLEEP_SEC="${CUSTOM_CHAR_SLEEP_SEC:-0.5}"
CUSTOM_CHAR_SLEEP_SEC="${CUSTOM_CHAR_SLEEP_SEC:-0.03}"
CUSTOM_ECHO_SLEEP_SEC="${CUSTOM_ECHO_SLEEP_SEC:-1}"
CUSTOM_ECHO_MIN_SLEEP_SEC="${CUSTOM_ECHO_MIN_SLEEP_SEC:-2}"
CUSTOM_COMMAND_SLEEP_SEC="${CUSTOM_COMMAND_SLEEP_SEC:-1}"
CUSTOM_WATCH_FINISH_SLEEP_SEC="${CUSTOM_WATCH_FINISH_SLEEP_SEC:-5}"
SKIP_MANUAL_SLEEPS="${SKIP_MANUAL_SLEEPS:-"false"}"

function print-ps1() {
  echo -n "${CUSTOM_PS1@P}"
  sleep "${CUSTOM_PS1_SLEEP_SEC}"
}

function user-echo() {
  sleep "${CUSTOM_ECHO_MIN_SLEEP_SEC}" &
  min_sleep_pid="$!"

  print-ps1

  while read -r -N1 character; do
    echo -n "$character"
    sleep "${CUSTOM_CHAR_SLEEP_SEC}"
  done < <( echo -n "${*}" )

  sleep "${CUSTOM_ECHO_SLEEP_SEC}"
  wait "${min_sleep_pid}"
  echo
}

function run {
  cmd=$( cat )
  user-echo "${cmd}"
  eval "${cmd}"
  sleep "${CUSTOM_COMMAND_SLEEP_SEC}"
}

function sleep-extra {
  if [[ "${SKIP_MANUAL_SLEEPS}" != "true" ]]; then
    sleep "$@"
  fi
}

if [[ -z "${SCYLLA_OPERATOR_REPO+x}" ]]; then
  cd "${tmpdir}"

run <<< "# We'll start by cloning the Scylla Operator repository."
run <<< "# Note: For production deployments, you should always use a stable (fixed) tag and only update the image along with the new manifests. Rolling tags, like latest, may lead to your manifests and operator image getting out of sync and break the deployment."
run <<'EOF'
git clone --depth=1 --branch=master https://github.com/scylladb/scylla-operator.git
EOF
  run <<'EOF'
cd scylla-operator
EOF
else
  cd "${SCYLLA_OPERATOR_REPO}"
fi

run <<< "# Before we start the deployment, make sure your Kubernetes nodes for running ScyllaDB are labeled the same way to match the manifests."
run <<'EOF'
scylla_nodes=$( kubectl get nodes -l scylla.scylladb.com/node-type=scylla -o name )
if [[ "${scylla_nodes}" == "" ]]; then
  echo "Labeling all nodes to be used for ScyllaDB."
  kubectl label nodes scylla.scylladb.com/node-type=scylla --all
else
  printf "Using nodes %q for ScyllaDB." "${scylla_nodes}"
fi
EOF
sleep-extra 3

run <<< "# First, we'll install our dependencies. You can skip this step if you already have them installed in your cluster."
sleep-extra 1
run <<'EOF'
kubectl apply --server-side -f=./examples/common/cert-manager.yaml
EOF
run <<'EOF'
kubectl -n=prometheus-operator apply --server-side -f=./examples/third-party/prometheus-operator/
EOF
run <<'EOF'
kubectl wait --for='condition=established' crd/certificates.cert-manager.io crd/issuers.cert-manager.io
EOF
run <<'EOF'
kubectl wait --for='condition=established' \
crd/prometheuses.monitoring.coreos.com \
crd/prometheusrules.monitoring.coreos.com \
crd/servicemonitors.monitoring.coreos.com
EOF
run <<'EOF'
kubectl -n=cert-manager rollout status --timeout=10m deployment.apps/cert-manager{,-cainjector,-webhook}
EOF
run <<'EOF'
kubectl -n=prometheus-operator rollout status --timeout=10m deployment.apps/prometheus-operator
EOF

run <<< "# Now let's deploy Scylla Operator."
run <<'EOF'
kubectl -n=scylla-operator apply --server-side -f=./deploy/operator/
EOF
run <<'EOF'
kubectl wait --for='condition=established' \
crd/scyllaclusters.scylla.scylladb.com \
crd/nodeconfigs.scylla.scylladb.com \
crd/scyllaoperatorconfigs.scylla.scylladb.com \
crd/scylladbmonitorings.scylla.scylladb.com
EOF
run <<'EOF'
kubectl -n=scylla-operator rollout status --timeout=10m deployment.apps/{scylla-operator,webhook-server}
EOF

run <<< "# To run ScyllaClusters on local disks (NVMes in production), we need to set up our storage."

run <<< "# We need to set up our local disks into an XFS directory, which will be used by ScyllaDB local CSI driver to provision PVCs."
run <<< "# We will also enable node tuning."

run <<< "# Create a NodeConfig to set up a RAID array, a filesystem on top of it, and to tune it."
run <<< "# Note that this part depends on your infrastructure, disk names, and other properties."
run <<< "# Be sure to double check the NodeConfig and adjust it, if needs be."
run <<'EOF'
yq -e eval --colors '.' ./examples/gke/nodeconfig-alpha.yaml | head -n 25
EOF
sleep-extra 10
run <<'EOF'
kubectl apply --server-side -f=./examples/gke/nodeconfig-alpha.yaml
EOF

run <<< "# Wait for the NodeConfig to apply changes to the Kubernetes nodes."
run <<'EOF'
kubectl wait --for='condition=Reconciled' --timeout=10m nodeconfigs.scylla.scylladb.com/cluster
EOF
# TODO: Extend waiting for the NodeConfig when we fix https://github.com/scylladb/scylla-operator/issues/1557

run <<< "# Install the ScyllaDB local CSI driver that will provision storage for PersistentVolumeClaims."
run <<< "# The ScyllaDB local CSI driver is configured to use '/mnt/persistent-volumes' directory created by the NodeConfig."
run <<'EOF'
kubectl -n local-csi-driver apply --server-side -f=https://raw.githubusercontent.com/scylladb/k8s-local-volume-provisioner/v0.2.0/{deploy/kubernetes/local-csi-driver/{00_namespace,10_csidriver,10_driver_serviceaccount,10_provisioner_clusterrole,20_provisioner_clusterrolebinding},example/storageclass_xfs}.yaml
kubectl create -f=https://raw.githubusercontent.com/scylladb/k8s-local-volume-provisioner/v0.2.0/deploy/kubernetes/local-csi-driver/50_daemonset.yaml --dry-run=client -o yaml | yq -e '.spec.template.spec.nodeSelector += {"scylla.scylladb.com/node-type": "scylla"}' | kubectl -n=local-csi-driver apply --server-side -f -
kubectl -n=scylla-operator rollout status --timeout=10m deployment.apps/{scylla-operator,webhook-server}
EOF
run <<'EOF'
kubectl -n=local-csi-driver rollout status --timeout=10m daemonset.apps/local-csi-driver
EOF

run <<< "# Because all of the following manifests use a default storage class, we'll set 'scylladb-local-xfs' as the default for simplicity."
run <<< "# As there can be only one default storage class, make sure to unset the previous default one, if it exists in your setup."
run <<< "# This can be also controlled per ScyllaCluster."
run <<'EOF'
kubectl patch storageclass scylladb-local-xfs -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}'
EOF
sleep-extra 5

run <<< "# Optionally, deploy Scylla Manager."
run <<< "# Scylla Manager is global and shared by every ScyllaCluster."
run <<'EOF'
kubectl -n=scylla-manager apply --server-side -f=./deploy/manager/dev/
EOF
run <<'EOF'
kubectl -n=scylla-manager rollout status --timeout=10m deployment.apps/scylla-manager{,-controller}
EOF

run <<< "# We have now successfully deployed the infrastructure and can start creating ScyllaDB clusters."

run <<< "# Create demo namespace."
run <<'EOF'
kubectl create namespace demo --dry-run=client -o=yaml | kubectl apply --server-side -f=-
EOF

run <<< "# Optionally, create a config for our ScyllaDB cluster."
run <<'EOF'
yq -e eval --colors '.' ./examples/scylladb/scylla-config.cm.yaml | head -n 25
EOF
sleep-extra 5
run <<'EOF'
kubectl -n=demo apply --server-side -f=./examples/scylladb/scylla-config.cm.yaml
EOF

run <<< "# Create a ScyllaDB cluster (ScyllaCluster)."
run <<'EOF'
yq -e eval --colors '.' ./examples/scylladb/scylla.scyllacluster.yaml | head -n 25
EOF
sleep-extra 5
run <<'EOF'
kubectl -n=demo apply --server-side -f=./examples/scylladb/scylla.scyllacluster.yaml
EOF
run <<< "# Wait for the ScyllaCluster to roll out."
run <<'EOF'
kubectl -n=demo wait --for='condition=Progressing=false' --timeout=10m scyllacluster.scylla.scylladb.com/scylla
EOF
run <<'EOF'
kubectl -n=demo wait --for='condition=Degraded=false' --timeout=10m scyllacluster.scylla.scylladb.com/scylla
EOF
run <<'EOF'
kubectl -n=demo wait --for='condition=Available' --timeout=10m scyllacluster.scylla.scylladb.com/scylla
EOF
run <<'EOF'
kubectl -n=demo get scyllacluster.scylla.scylladb.com/scylla --template='{{ printf "ScyllaCluster has %d ready member(s)\n" ( index .status.racks "us-east-1a" ).readyMembers }}'
EOF
run <<'EOF'
kubectl -n=demo get scyllacluster.scylla.scylladb.com,statefulsets,pods,pvc
EOF
sleep-extra 5
run <<'EOF'
kubectl -n=demo get configmaps,secrets
EOF
sleep-extra 5

# TODO: Add client example

run <<< "# Create ScyllaDBMonitoring."
run <<< "# (ScyllaDBMonitoring is a namespaced resource, used to monitor 1-N ScyllaClusters in the same namespace.)"
run <<'EOF'
monitoring_manifest="$( mktemp )"
EOF
run <<'EOF'
yq -e eval '. | .spec.endpointsSelector.matchLabels["scylla/cluster"] = "scylla" | .spec.components.grafana.exposeOptions.webInterface.ingress.dnsDomains = ["example-grafana.demo.svc.cluster.local"]' ./examples/monitoring/v1alpha1/scylladbmonitoring.yaml | tee "${monitoring_manifest}" | yq -e eval --colors '.' | head -n 25
EOF
run <<'EOF'
kubectl -n=demo apply --server-side -f="${monitoring_manifest}"
EOF
run <<< "# Wait for the ScyllaDBMonitoring to roll out."
run <<'EOF'
kubectl -n=demo wait --for='condition=Progressing=false' --timeout=10m scylladbmonitoring.scylla.scylladb.com/example
EOF
run <<'EOF'
kubectl -n=demo wait --for='condition=Degraded=false' --timeout=10m scylladbmonitoring.scylla.scylladb.com/example
EOF
run <<'EOF'
kubectl -n=demo wait --for='condition=Available' --timeout=10m scylladbmonitoring.scylla.scylladb.com/example
EOF
run <<'EOF'
kubectl -n=demo rollout status --timeout=10m deployment.apps/example-grafana
EOF
run <<'EOF'
kubectl -n=demo get svc/example-grafana
EOF
run <<< "# Verify we can log in to Grafana."
run <<'EOF'
kubectl -n=demo port-forward svc/example-grafana 5000:3000 >/dev/null &
EOF
run <<'EOF'
grafana_serving_ca_cert="$( kubectl -n=demo get secret/example-grafana-serving-ca --template='{{ index .data "tls.crt" }}' | base64 -d )"
EOF
run <<'EOF'
grafana_user="$( kubectl -n=demo get secret/example-grafana-admin-credentials --template='{{ index .data "username" }}' | base64 -d )"
EOF
run <<'EOF'
grafana_password="$( kubectl -n=demo get secret/example-grafana-admin-credentials --template='{{ index .data "password" }}' | base64 -d )"
EOF
sleep-extra 3
run <<'EOF'
curl --fail --retry-all-errors 10 --max-time 3 --retry-max-time 5 --cacert <( echo "${grafana_serving_ca_cert}" ) --resolve 'example-grafana.demo.svc.cluster.local:5000:127.0.0.1' -IL 'https://example-grafana.demo.svc.cluster.local:5000/' --user "${grafana_user}:${grafana_password}"
EOF
sleep-extra 3

run <<< "# Scale the ScyllaDB cluster from 1 to 3 nodes."
run <<'EOF'
yq -e eval '.spec.datacenter.racks[0].members = 3' ./examples/scylladb/scylla.scyllacluster.yaml | kubectl -n=demo apply --server-side -f -
EOF

run <<< "# Wait for the ScyllaCluster to roll out."
run <<'EOF'
kubectl -n=demo wait --for='condition=Progressing=false' --timeout=10m scyllacluster.scylla.scylladb.com/scylla
EOF
run <<'EOF'
kubectl -n=demo wait --for='condition=Degraded=false' --timeout=10m scyllacluster.scylla.scylladb.com/scylla
EOF
run <<'EOF'
kubectl -n=demo wait --for='condition=Available' --timeout=10m scyllacluster.scylla.scylladb.com/scylla
EOF
run <<'EOF'
kubectl -n=demo get scyllacluster.scylla.scylladb.com/scylla --template='{{ printf "ScyllaCluster has %d ready member(s)\n" ( index .status.racks "us-east-1a" ).readyMembers }}'
EOF

sleep-extra 5
run <<'EOF'
kubectl -n=demo get scyllacluster.scylla.scylladb.com,statefulsets,pods,pvc
EOF
sleep-extra 5
run <<'EOF'
kubectl -n=demo get configmaps,secrets
EOF

run <<< "# Thanks for watching!"
