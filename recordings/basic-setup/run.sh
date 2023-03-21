#!/usr/bin/bash

set -euEo pipefail
shopt -s inherit_errexit

trap 'rm -rf -- "${tmpdir}" && job_ids="$( jobs -p )" && if [[ -n "${job_ids}" ]]; then kill ${job_ids}; fi && wait' EXIT

CUSTOM_PS1="${CUSTOM_PS1:-$ }"
CUSTOM_PS1_SLEEP_SEC="${CUSTOM_CHAR_SLEEP_SEC:-0.5}"
CUSTOM_CHAR_SLEEP_SEC="${CUSTOM_CHAR_SLEEP_SEC:-0.03}"
CUSTOM_ECHO_SLEEP_SEC="${CUSTOM_ECHO_SLEEP_SEC:-1}"
CUSTOM_ECHO_MIN_SLEEP_SEC="${CUSTOM_ECHO_MIN_SLEEP_SEC:-2}"
CUSTOM_COMMAND_SLEEP_SEC="${CUSTOM_COMMAND_SLEEP_SEC:-1}"
CUSTOM_WATCH_FINISH_SLEEP_SEC="${CUSTOM_WATCH_FINISH_SLEEP_SEC:-5}"

tmpdir="$( mktemp -d )"
cd "${tmpdir}"

function print-ps1() {
  echo -n "${CUSTOM_PS1@P}"
  sleep "${CUSTOM_PS1_SLEEP_SEC}"
}

function user-echo() {
  sleep "${CUSTOM_ECHO_MIN_SLEEP_SEC}" &
  min_sleep_pid="$!"

  print-ps1

  while IFS='' read -n1 character; do
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
  cmd=$( cat )
  user-echo watch -n1 -e "${cmd}"
  ( echo | watch -n1 -e "${cmd} && ( ( ${wait_cmd} ) || ( sleep ${CUSTOM_WATCH_FINISH_SLEEP_SEC} && false ) )" ) || [[ $? == 8 ]]
  run_last_pid=$!
  sleep "${CUSTOM_COMMAND_SLEEP_SEC}"
}

run <<'EOF'
git clone --depth=1 --branch=v1.9.0-alpha.2 https://github.com/scylladb/scylla-operator.git
EOF
run <<'EOF'
cd scylla-operator
EOF

run <<< "# First we'll install our dependencies. If you already have them installed in your cluster, you can skip this step."
sleep 1
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
prometheus_crds="$( find ./examples/third-party/prometheus-operator/ -name '*.crd.yaml' -printf '-f=%p\n' )"
EOF
run <<'EOF'
kubectl wait --for='condition=established' "${prometheus_crds}"
EOF
run <<'EOF'
kubectl -n=cert-manager rollout status --timeout=10m deployment.apps/cert-manager{,-cainjector,-webhook}
EOF
run <<'EOF'
kubectl -n=prometheus-operator rollout status --timeout=10m deployment.apps/prometheus-operator
EOF

run <<< "# Now let's deploy Scylla Operator"
run <<'EOF'
kubectl -n=scylla-operator apply --server-side -f=./deploy/operator/
EOF
run <<'EOF'
kubectl wait --for='condition=established' crd/nodeconfigs.scylla.scylladb.com
EOF
run <<'EOF'
kubectl wait --for='condition=established' crd/scyllaoperatorconfigs.scylla.scylladb.com
EOF
run <<'EOF'
kubectl wait --for='condition=established' crd/scylladbmonitorings.scylla.scylladb.com
EOF
run <<'EOF'
kubectl -n=scylla-operator rollout status --timeout=10m deployment.apps/{scylla-operator,webhook-server}
EOF

run <<< "# Optionally, deploy Scylla Manager"
run <<'EOF'
kubectl -n=scylla-manager apply --server-side -f=./deploy/manager/dev/
EOF
run <<'EOF'
kubectl -n=scylla-manager rollout status --timeout=10m deployment.apps/scylla-manager{,-controller}
EOF

run <<< "# Optionally, enable cluster tuning."
run <<< "# Make sure your nodes are labeled the same way or adjust the node selector on the NodeConfig."
sleep 3
run <<'EOF'
kubectl apply --server-side -f=./examples/common/nodeconfig-alpha.yaml
EOF
run <<'EOF'
kubectl get nodeconfigs.scylla.scylladb.com/cluster -o=yaml
EOF
run <<'EOF'
kubectl wait --for='condition=Reconciled' nodeconfigs.scylla.scylladb.com/cluster
EOF

run <<< "# Create demo namespace"
run <<'EOF'
kubectl create namespace demo --dry-run=client -o=yaml | kubectl apply --server-side -f=-
EOF

run <<< "# Optionally, create a config for ScyllaDB cluster"
# FIXME: remove the embedded fallback
if [[ ! -f ./examples/scylladb/example.scyllacluster.yaml ]]; then
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
run <<'EOF'
head -n 20 ./examples/scylladb/scylla-config.cm.yaml
EOF
sleep 5
run <<'EOF'
kubectl -n=demo apply --server-side --force-conflicts -f=./examples/scylladb/scylla-config.cm.yaml
EOF

run <<< "# Create a ScyllaDB cluster"
# FIXME: remove the embedded fallback
if [[ ! -f ./examples/scylladb/example.scyllacluster.yaml ]]; then
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
run <<'EOF'
head -n 25 ./examples/scylladb/example.scyllacluster.yaml
EOF
sleep 5
run <<'EOF'
kubectl -n=demo apply --server-side --force-conflicts -f=./examples/scylladb/example.scyllacluster.yaml
EOF
run <<'EOF'
kubectl -n=demo wait --for='condition=Progressing=false' --timeout=10m scyllacluster.scylla.scylladb.com/example
EOF
run <<'EOF'
kubectl -n=demo wait --for='condition=Degraded=false' --timeout=10m scyllacluster.scylla.scylladb.com/example
EOF
run <<'EOF'
kubectl -n=demo wait --for='condition=Available' --timeout=10m scyllacluster.scylla.scylladb.com/example
EOF
run <<'EOF'
kubectl -n=demo get scyllacluster.scylla.scylladb.com/example --template='{{ printf "ScyllaCluster has %d ready member(s)\n" ( index .status.racks "us-east-1a" ).readyMembers }}'
EOF
run <<'EOF'
kubectl -n=demo get scyllacluster.scylla.scylladb.com,statefulsets,pods,pvc
EOF
sleep 5
run <<'EOF'
kubectl -n=demo get configmaps,secrets
EOF
sleep 5


run <<< "# Create ScyllaDBMonitoring"
run <<'EOF'
monitoring_manifest=$( mktemp )
EOF
run <<'EOF'
yq e '. | .spec.endpointsSelector.matchLabels["scylla/cluster"] = "example"' ./examples/monitoring/v1alpha1/scylladbmonitoring.yaml | tee "${monitoring_manifest}"
EOF
run <<'EOF'
kubectl -n=demo apply --server-side -f="${monitoring_manifest}"
EOF
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
run <<< "# Verify we can login to Grafana"
run <<'EOF'
kubectl -n=demo port-forward svc/example-grafana 5000:3000 >/dev/null &
EOF
run <<'EOF'
grafana_serving_cert="$( kubectl -n=demo get secret/example-grafana-serving-ca --template='{{ index .data "tls.crt" }}' | base64 -d )"
EOF
run <<'EOF'
grafana_user="$( kubectl -n=demo get secret/example-grafana-admin-credentials --template='{{ index .data "username" }}' | base64 -d )"
EOF
run <<'EOF'
grafana_password="$( kubectl -n=demo get secret/example-grafana-admin-credentials --template='{{ index .data "password" }}' | base64 -d )"
EOF
sleep 3
run <<'EOF'
curl --fail --retry-all-errors 10 --max-time 3 --retry-max-time 5 --cacert <( echo "${grafana_serving_cert}" ) --resolve 'test-grafana.test.svc.cluster.local:5000:127.0.0.1' -IL 'https://test-grafana.test.svc.cluster.local:5000/' --user "${grafana_user}:${grafana_password}"
EOF
sleep 3

run <<< "# Scale the ScyllaDB cluster from 1 to 3 nodes"
run <<'EOF'
kubectl -n=demo patch scyllacluster.scylla.scylladb.com/example --type=json -p='[{"op": "replace", "path": "/spec/datacenter/racks/0/members", "value": 3}]'
EOF
run <<'EOF'
kubectl -n=demo wait --for='condition=Progressing=false' --timeout=10m scyllacluster.scylla.scylladb.com/example
EOF
run <<'EOF'
kubectl -n=demo wait --for='condition=Degraded=false' --timeout=10m scyllacluster.scylla.scylladb.com/example
EOF
run <<'EOF'
kubectl -n=demo wait --for='condition=Available' --timeout=10m scyllacluster.scylla.scylladb.com/example
EOF
run <<'EOF'
kubectl -n=demo get scyllacluster.scylla.scylladb.com/example --template='{{ printf "ScyllaCluster has %d ready member(s)\n" ( index .status.racks "us-east-1a" ).readyMembers }}'
EOF
sleep 5
run <<'EOF'
kubectl -n=demo get scyllacluster.scylla.scylladb.com,statefulsets,pods,pvc
EOF
sleep 5
run <<'EOF'
kubectl -n=demo get configmaps,secrets
EOF

run <<< "# Thanks for watching!"
