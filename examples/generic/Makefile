clean:
	rm -f grafana/values.yaml

grafana/values.yaml:
	cp grafana/values.yaml.in grafana/values.yaml

	cat grafana/dashboards/scylla-cpu.4.0.json | sed -e 's/^/          /' > grafana/dashboards/scylla-cpu.4.0.json.tmp
	sed -i -e '/scylla-cpu-4.0:/r grafana/dashboards/scylla-cpu.4.0.json.tmp' grafana/values.yaml
	sed -i -e "/scylla-cpu-4.0:/a\\      json: |" grafana/values.yaml
	rm -f grafana/dashboards/scylla-cpu.4.0.json.tmp

	cat grafana/dashboards/scylla-cql.4.0.json | sed -e 's/^/          /' > grafana/dashboards/scylla-cql.4.0.json.tmp
	sed -i -e '/scylla-cql-4.0:/r grafana/dashboards/scylla-cql.4.0.json.tmp' grafana/values.yaml
	sed -i -e "/scylla-cql-4.0:/a\\      json: |" grafana/values.yaml
	rm -f grafana/dashboards/scylla-cql.4.0.json.tmp

	cat grafana/dashboards/scylla-detailed.4.0.json | sed -e 's/^/          /' > grafana/dashboards/scylla-detailed.4.0.json.tmp
	sed -i -e '/scylla-detailed-4.0:/r grafana/dashboards/scylla-detailed.4.0.json.tmp' grafana/values.yaml
	sed -i -e "/scylla-detailed-4.0:/a\\      json: |" grafana/values.yaml
	rm -f grafana/dashboards/scylla-detailed.4.0.json.tmp

	cat grafana/dashboards/scylla-errors.4.0.json | sed -e 's/^/          /' > grafana/dashboards/scylla-errors.4.0.json.tmp
	sed -i -e '/scylla-errors-4.0:/r grafana/dashboards/scylla-errors.4.0.json.tmp' grafana/values.yaml
	sed -i -e "/scylla-errors-4.0:/a\\      json: |" grafana/values.yaml
	rm -f grafana/dashboards/scylla-errors.4.0.json.tmp

	cat grafana/dashboards/scylla-io.4.0.json | sed -e 's/^/          /' > grafana/dashboards/scylla-io.4.0.json.tmp
	sed -i -e '/scylla-io-4.0:/r grafana/dashboards/scylla-io.4.0.json.tmp' grafana/values.yaml
	sed -i -e "/scylla-io-4.0:/a\\      json: |" grafana/values.yaml
	rm -f grafana/dashboards/scylla-io.4.0.json.tmp

	cat grafana/dashboards/scylla-os.4.0.json | sed -e 's/^/          /' > grafana/dashboards/scylla-os.4.0.json.tmp
	sed -i -e '/scylla-os-4.0:/r grafana/dashboards/scylla-os.4.0.json.tmp' grafana/values.yaml
	sed -i -e "/scylla-os-4.0:/a\\      json: |" grafana/values.yaml
	rm -f grafana/dashboards/scylla-os.4.0.json.tmp

	cat grafana/dashboards/scylla-overview.4.0.json | sed -e 's/^/          /' > grafana/dashboards/scylla-overview.4.0.json.tmp
	sed -i -e '/scylla-overview-4.0:/r grafana/dashboards/scylla-overview.4.0.json.tmp' grafana/values.yaml
	sed -i -e "/scylla-overview-4.0:/a\\      json: |" grafana/values.yaml
	rm -f grafana/dashboards/scylla-overview.4.0.json.tmp
