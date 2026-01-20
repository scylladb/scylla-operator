#!/bin/bash
#
# Copyright (C) 2017 ScyllaDB
#

readonly SCYLLA_IMAGE=${SCYLLA_IMAGE}

set -eu -o pipefail

function scylla_up() {
  local -r exec="docker compose exec -T"

  echo "==> Running Scylla ${SCYLLA_IMAGE}"
  docker pull ${SCYLLA_IMAGE}
  docker compose up -d --wait || ( docker compose ps --format json | jq -M 'select(.Health == "unhealthy") | .Service' | xargs docker compose logs; exit 1 )
}

function scylla_down() {
  echo "==> Stopping Scylla"
  docker compose down
}

function scylla_restart() {
  scylla_down
  scylla_up
}

scylla_restart

sudo chmod 0777 /tmp/scylla_node_1/cql.m
sudo chmod 0777 /tmp/scylla_node_2/cql.m
sudo chmod 0777 /tmp/scylla_node_3/cql.m

readonly clusterSize=3
readonly scylla_liveset="192.168.100.11,192.168.100.12,192.168.100.13"
readonly cversion="3.11.4"
readonly proto=4
readonly args="-cluster-socket /tmp/scylla_node_1/cql.m -gocql.timeout=60s -proto=${proto} -rf=1 -clusterSize=${clusterSize} -autowait=2000ms -compressor=snappy -gocql.cversion=${cversion} -cluster=${scylla_liveset}"

TAGS=$*

if [ ! -z "$TAGS" ];
then
	echo "==> Running ${TAGS} tests with args: ${args}"
	go test -v -timeout=5m -race -tags="$TAGS" ${args} ./...
fi
