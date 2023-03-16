#!/usr/bin/bash

set -euEo pipefail
shopt -s inherit_errexit

trap 'rm -rf -- "${tmpdir}" && job_ids="$( jobs -p )" && if [[ -n "${job_ids}" ]]; then kill ${job_ids}; fi && wait' EXIT

CUSTOM_PS1="${CUSTOM_PS1:-$ }"
CUSTOM_CHAR_SLEEP_SEC="${CUSTOM_CHAR_SLEEP_SEC:-0.05}"
CUSTOM_ECHO_SLEEP_SEC="${CUSTOM_ECHO_SLEEP_SEC:-0.2}"
CUSTOM_COMMAND_SLEEP_SEC="${CUSTOM_COMMAND_SLEEP_SEC:-1}"
CUSTOM_WATCH_FINISH_SLEEP_SEC="${CUSTOM_WATCH_FINISH_SLEEP_SEC:-5}"

tmpdir="$( mktemp -d )"
cd "${tmpdir}"

function user-echo() {
  echo -n "${CUSTOM_PS1@P}"

  while IFS='' read -n1 character; do
    echo -n "$character"
    sleep "${CUSTOM_CHAR_SLEEP_SEC}"
  done < <( echo -n "$@" )

  sleep "${CUSTOM_ECHO_SLEEP_SEC}"
  echo
}

function run {
  user-echo "$@"
  "$@"
  run_last_pid=$!
  sleep "${CUSTOM_COMMAND_SLEEP_SEC}"
}

function run-bg {
  user-echo "$@" '&'
  "$@" &
  run_last_pid=$!
  sleep "${CUSTOM_COMMAND_SLEEP_SEC}"
}

function run-eval {
  user-echo "$@"
  eval "$@"
  run_last_pid=$!
  sleep "${CUSTOM_COMMAND_SLEEP_SEC}"
}

function run-watch {
  wait_pids=($1)
  wait_cmd=""
  for pid in ${wait_pids[*]}; do
    if [[ -n "${wait_cmd}" ]]; then
      wait_cmd+=" || "
    fi
    wait_cmd+="test -d /proc/${pid}/"
  done
  shift
  user-echo watch -n1 -e "$*"
  ( echo | watch -n1 -e "$* && ( ( ${wait_cmd} ) || ( sleep ${CUSTOM_WATCH_FINISH_SLEEP_SEC} && false ) )" ) || [[ $? == 8 ]]
  run_last_pid=$!
  sleep "${CUSTOM_COMMAND_SLEEP_SEC}"
}

run git clone --depth=1 --branch=v1.9.0-alpha.2 https://github.com/scylladb/scylla-operator.git
run cd scylla-operator

user-echo "# First we'll install our dependencies. If you already have them installed in your cluster, you can skip this step."
run kubectl apply --server-side -f ./examples/common/cert-manager.yaml
run kubectl -n prometheus-operator apply --server-side -f ./examples/third-party/prometheus-operator/
run kubectl wait --for condition=established crd/certificates.cert-manager.io crd/issuers.cert-manager.io
run kubectl wait --for condition=established $( find ./examples/third-party/prometheus-operator/ -name '*.crd.yaml' -printf '-f=%p\n' )
run kubectl -n cert-manager rollout status deployment.apps/cert-manager{,-cainjector,-webhook}
run kubectl -n prometheus-operator rollout status deployment.apps/prometheus-operator

user-echo "# Now let's deploy Scylla Operator"
run kubectl -n scylla-operator apply --server-side -f ./deploy/operator/
run kubectl wait --for condition=established crd/nodeconfigs.scylla.scylladb.com
run kubectl wait --for condition=established crd/scyllaoperatorconfigs.scylla.scylladb.com
run kubectl wait --for condition=established crd/scylladbmonitorings.scylla.scylladb.com
run kubectl -n scylla-operator rollout status deployment.apps/{scylla-operator,webhook-server}

user-echo "# Optionally deploy Scylla Manager"
run kubectl -n scylla-manager apply --server-side -f ./deploy/manager/dev/
run kubectl -n scylla-manager rollout status deployment.apps/scylla-manager{,-controller}

user-echo "# Enable cluster tuning. Make sure your nodes are labeled the same way or adjust the node selector on the NodeConfig."
run kubectl apply --server-side -f ./examples/common/nodeconfig-alpha.yaml
run kubectl wait --for condition=Reconciled nodeconfigs.scylla.scylladb.com/cluster
run kubectl get nodeconfigs.scylla.scylladb.com/cluster -o yaml

user-echo "# Create demo namespace"
run-eval kubectl create namespace demo --dry-run=client -o yaml '|' kubectl apply --server-side -f -

user-echo "# Optionally, create a config for ScyllaDB cluster"
# FIXME: remove the embedded fallback
if [[ ! -f ./examples/scylladb/example.scyllacluster.yaml ]]; then
  user-echo "# Falling back to embedded scyllacluster definition"
  mkdir -p ./examples/scylladb/
  cat > ./examples/scylladb/scylla-config.cm.yaml <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: scylla-config
data:
  scylla.yaml: |
    authenticator: PasswordAuthenticator
    authorizer: CassandraAuthorizer
EOF
fi
run head -n 20 ./examples/scylladb/scylla-config.cm.yaml
sleep 5
run kubectl -n demo apply --server-side --force-conflicts -f ./examples/scylladb/scylla-config.cm.yaml

user-echo "# Create a ScyllaDB cluster"
# FIXME: remove the embedded fallback
if [[ ! -f ./examples/scylladb/example.scyllacluster.yaml ]]; then
  user-echo "# Falling back to embedded scyllacluster definition"
  mkdir -p ./examples/scylladb/
  cat > ./examples/scylladb/example.scyllacluster.yaml <<EOF
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
metadata:
  name: example
spec:
  version: 5.0.5
  agentVersion: 3.0.1
  developerMode: true
  datacenter:
    name: us-east-1
    racks:
    - name: us-east-1a
      members: 1
      storage:
        capacity: 100Mi
      resources:
        requests:
          cpu: 10m
          memory: 100Mi
        limits:
          cpu: 1
          memory: 1Gi
EOF
fi
run head -n 20 ./examples/scylladb/example.scyllacluster.yaml
sleep 5
run kubectl -n demo apply --server-side --force-conflicts -f ./examples/scylladb/example.scyllacluster.yaml
out_file="$( mktemp -t wait_logs.XXXXX )"
run-eval "kubectl -n demo wait --for='condition=Progressing=false' --timeout=10m scyllacluster.scylla.scylladb.com/example >>'${out_file}' 2>&1 &"
wpids="${run_last_pid}"
run-eval "kubectl -n demo wait --for='condition=Degraded=false' --timeout=10m scyllacluster.scylla.scylladb.com/example >>'${out_file}' 2>&1 &"
wpids+=" ${run_last_pid}"
run-eval "kubectl -n demo wait --for='condition=Available' --timeout=10m scyllacluster.scylla.scylladb.com/example >>'${out_file}' 2>&1 &"
wpids+=" ${run_last_pid}"
run-watch "${wpids}" kubectl -n demo get scyllacluster.scylla.scylladb.com,statefulsets,pods,pvc,configmaps,secrets
sleep 1
run cat "${out_file}"
run kubectl -n demo get scyllacluster.scylla.scylladb.com/example --template='{{ printf "ScyllaCluster has %d ready member(s)\n" ( index .status.racks "us-east-1a" ).readyMembers }}'

user-echo "# Create ScyllaDBMonitoring"
run kubectl -n demo apply --server-side -f ./examples/monitoring/v1alpha1/scylladbmonitoring.yaml
run kubectl -n demo wait --for='condition=Progressing=false' --timeout=10m scylladbmonitoring.scylla.scylladb.com/example
run kubectl -n demo wait --for='condition=Degraded=false' --timeout=10m scylladbmonitoring.scylla.scylladb.com/example
run kubectl -n demo wait --for='condition=Available' --timeout=10m scylladbmonitoring.scylla.scylladb.com/example
run kubectl -n demo get svc/example-grafana
# Sleep a bit to avoid talking to grafana too soon
sleep 5
run-bg kubectl -n demo port-forward svc/example-grafana 5000:3000 >/dev/null
run-eval 'curl --cacert <( kubectl -n demo get secret/example-grafana-serving-ca --template='"'"'{{ index .data "tls.crt" }}'"'"' | base64 -d ) --resolve "test-grafana.test.svc.cluster.local:5000:127.0.0.1" -IL https://test-grafana.test.svc.cluster.local:5000/'

user-echo "# Scale the ScyllaDB cluster from 1 to 3 nodes"
run kubectl -n demo patch scyllacluster.scylla.scylladb.com/example --type='json' -p='[{"op": "replace", "path": "/spec/datacenter/racks/0/members", "value": 3}]' --template='{{ .metadata.generation }}'
out_file="$( mktemp -t wait_logs.XXXXX )"
run-eval "kubectl -n demo wait --for='condition=Progressing=false' --timeout=10m scyllacluster.scylla.scylladb.com/example >>'${out_file}' 2>&1 &"
wpids="${run_last_pid}"
run-eval "kubectl -n demo wait --for='condition=Degraded=false' --timeout=10m scyllacluster.scylla.scylladb.com/example >>'${out_file}' 2>&1 &"
wpids+=" ${run_last_pid}"
run-eval "kubectl -n demo wait --for='condition=Available' --timeout=10m scyllacluster.scylla.scylladb.com/example >>'${out_file}' 2>&1 &"
wpids+=" ${run_last_pid}"
run-watch "${wpids}" kubectl -n demo get scyllacluster.scylla.scylladb.com,statefulsets,pods,pvc,configmaps,secrets
# Sleep 1s so bash has time to write the redirected output.
sleep 1
run cat "${out_file}"
run kubectl -n demo get scyllacluster.scylla.scylladb.com/example --template='{{ printf "ScyllaCluster has %d ready member(s)\n" ( index .status.racks "us-east-1a" ).readyMembers }}'

