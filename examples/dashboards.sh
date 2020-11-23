#!/usr/bin/env bash

TARGET="common"

cp ${TARGET}/grafana/values.yaml.in ${TARGET}/grafana/values.yaml

cat ${TARGET}/grafana/dashboards/alternator.4.0.json | sed -e 's/^/          /' > ${TARGET}/grafana/dashboards/alternator.4.0.json.tmp
sed -i -e "/alternator-4.0:/r ${TARGET}/grafana/dashboards/alternator.4.0.json.tmp" ${TARGET}/grafana/values.yaml
sed -i -e "/alternator-4.0:/a\\      json: |" ${TARGET}/grafana/values.yaml
rm -f ${TARGET}/grafana/dashboards/alternator.4.0.json.tmp

cat ${TARGET}/grafana/dashboards/scylla-cpu.4.0.json | sed -e 's/^/          /' > ${TARGET}/grafana/dashboards/scylla-cpu.4.0.json.tmp
sed -i -e "/scylla-cpu-4.0:/r ${TARGET}/grafana/dashboards/scylla-cpu.4.0.json.tmp" ${TARGET}/grafana/values.yaml
sed -i -e "/scylla-cpu-4.0:/a\\      json: |" ${TARGET}/grafana/values.yaml
rm -f ${TARGET}/grafana/dashboards/scylla-cpu.4.0.json.tmp

cat ${TARGET}/grafana/dashboards/scylla-cql.4.0.json | sed -e 's/^/          /' > ${TARGET}/grafana/dashboards/scylla-cql.4.0.json.tmp
sed -i -e "/scylla-cql-4.0:/r ${TARGET}/grafana/dashboards/scylla-cql.4.0.json.tmp" ${TARGET}/grafana/values.yaml
sed -i -e "/scylla-cql-4.0:/a\\      json: |" ${TARGET}/grafana/values.yaml
rm -f ${TARGET}/grafana/dashboards/scylla-cql.4.0.json.tmp

cat ${TARGET}/grafana/dashboards/scylla-detailed.4.0.json | sed -e 's/^/          /' > ${TARGET}/grafana/dashboards/scylla-detailed.4.0.json.tmp
sed -i -e "/scylla-detailed-4.0:/r ${TARGET}/grafana/dashboards/scylla-detailed.4.0.json.tmp" ${TARGET}/grafana/values.yaml
sed -i -e "/scylla-detailed-4.0:/a\\      json: |" ${TARGET}/grafana/values.yaml
rm -f ${TARGET}/grafana/dashboards/scylla-detailed.4.0.json.tmp

cat ${TARGET}/grafana/dashboards/scylla-errors.4.0.json | sed -e 's/^/          /' > ${TARGET}/grafana/dashboards/scylla-errors.4.0.json.tmp
sed -i -e "/scylla-errors-4.0:/r ${TARGET}/grafana/dashboards/scylla-errors.4.0.json.tmp" ${TARGET}/grafana/values.yaml
sed -i -e "/scylla-errors-4.0:/a\\      json: |" ${TARGET}/grafana/values.yaml
rm -f ${TARGET}/grafana/dashboards/scylla-errors.4.0.json.tmp

cat ${TARGET}/grafana/dashboards/scylla-io.4.0.json | sed -e 's/^/          /' > ${TARGET}/grafana/dashboards/scylla-io.4.0.json.tmp
sed -i -e "/scylla-io-4.0:/r ${TARGET}/grafana/dashboards/scylla-io.4.0.json.tmp" ${TARGET}/grafana/values.yaml
sed -i -e "/scylla-io-4.0:/a\\      json: |" ${TARGET}/grafana/values.yaml
rm -f ${TARGET}/grafana/dashboards/scylla-io.4.0.json.tmp

cat ${TARGET}/grafana/dashboards/scylla-os.4.0.json | sed -e 's/^/          /' > ${TARGET}/grafana/dashboards/scylla-os.4.0.json.tmp
sed -i -e "/scylla-os-4.0:/r ${TARGET}/grafana/dashboards/scylla-os.4.0.json.tmp" ${TARGET}/grafana/values.yaml
sed -i -e "/scylla-os-4.0:/a\\      json: |" ${TARGET}/grafana/values.yaml
rm -f ${TARGET}/grafana/dashboards/scylla-os.4.0.json.tmp

cat ${TARGET}/grafana/dashboards/scylla-overview.4.0.json | sed -e 's/^/          /' > ${TARGET}/grafana/dashboards/scylla-overview.4.0.json.tmp
sed -i -e "/scylla-overview-4.0:/r ${TARGET}/grafana/dashboards/scylla-overview.4.0.json.tmp" ${TARGET}/grafana/values.yaml
sed -i -e "/scylla-overview-4.0:/a\\      json: |" ${TARGET}/grafana/values.yaml
rm -f ${TARGET}/grafana/dashboards/scylla-overview.4.0.json.tmp
