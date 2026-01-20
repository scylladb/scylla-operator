SHELL := bash
.ONESHELL:
.SHELLFLAGS := -eo pipefail -c

MAKEFILE_PATH := $(abspath $(dir $(abspath $(lastword $(MAKEFILE_LIST)))))
KEY_PATH = ${MAKEFILE_PATH}/testdata/pki
BIN_DIR := "${MAKEFILE_PATH}/bin"

CASSANDRA_VERSION ?= LATEST
SCYLLA_VERSION ?= LATEST

GOLANGCI_VERSION = 2.5.0

TEST_CQL_PROTOCOL ?= 4
TEST_COMPRESSOR ?= snappy
TEST_OPTS ?=
TEST_INTEGRATION_TAGS ?= integration gocql_debug
JVM_EXTRA_OPTS ?= -Dcassandra.test.fail_writes_ks=test -Dcassandra.custom_query_handler_class=org.apache.cassandra.cql3.CustomPayloadMirroringQueryHandler

CCM_CASSANDRA_CLUSTER_NAME = gocql_cassandra_integration_test
CCM_CASSANDRA_IP_PREFIX = 127.0.1.
CCM_CASSANDRA_REPO ?= github.com/apache/cassandra-ccm
CCM_CASSANDRA_VERSION ?= trunk

CCM_SCYLLA_CLUSTER_NAME = gocql_scylla_integration_test
CCM_SCYLLA_IP_PREFIX = 127.0.2.
CCM_SCYLLA_REPO ?= github.com/scylladb/scylla-ccm
CCM_SCYLLA_VERSION ?= master

ifeq (${CCM_CONFIG_DIR},)
	CCM_CONFIG_DIR = ~/.ccm
endif
CCM_CONFIG_DIR := $(shell readlink --canonicalize ${CCM_CONFIG_DIR})

CASSANDRA_CONFIG ?= "client_encryption_options.enabled: true" \
"client_encryption_options.keystore: ${KEY_PATH}/.keystore" \
"client_encryption_options.keystore_password: cassandra" \
"client_encryption_options.require_client_auth: true" \
"client_encryption_options.truststore: ${KEY_PATH}/.truststore" \
"client_encryption_options.truststore_password: cassandra" \
"concurrent_reads: 2" \
"concurrent_writes: 2" \
"write_request_timeout_in_ms: 5000" \
"read_request_timeout_in_ms: 5000"

ifeq (${CASSANDRA_VERSION},3-LATEST)
	CASSANDRA_CONFIG += "rpc_server_type: sync" \
"rpc_min_threads: 2" \
"rpc_max_threads: 2" \
"enable_user_defined_functions: true" \
"enable_materialized_views: true" \

else ifeq (${CASSANDRA_VERSION},4-LATEST)
	CASSANDRA_CONFIG +=	"enable_user_defined_functions: true" \
"enable_materialized_views: true"
else
	CASSANDRA_CONFIG += "user_defined_functions_enabled: true" \
"materialized_views_enabled: true"
endif

SCYLLA_CONFIG = "native_transport_port_ssl: 9142" \
"native_transport_port: 9042" \
"native_shard_aware_transport_port: 19042" \
"native_shard_aware_transport_port_ssl: 19142" \
"client_encryption_options.enabled: true" \
"client_encryption_options.certificate: ${KEY_PATH}/cassandra.crt" \
"client_encryption_options.keyfile: ${KEY_PATH}/cassandra.key" \
"client_encryption_options.truststore: ${KEY_PATH}/ca.crt" \
"client_encryption_options.require_client_auth: true" \
"maintenance_socket: workdir" \
"enable_tablets: true" \
"enable_user_defined_functions: true" \
"experimental_features: [udf]"

export JVM_EXTRA_OPTS
export JAVA11_HOME=${JAVA_HOME_11_X64}
export JAVA17_HOME=${JAVA_HOME_17_X64}
export JAVA_HOME=${JAVA_HOME_11_X64}
export PATH := $(MAKEFILE_PATH)/bin:~/.sdkman/bin:$(PATH)

print-config:
	echo ${CASSANDRA_CONFIG}

.prepare-bin:
	@[[ -d "$(MAKEFILE_PATH)/bin" ]] || mkdir "$(MAKEFILE_PATH)/bin"

.prepare-get-version: .prepare-bin
	@if [[ ! -f "$(MAKEFILE_PATH)/bin/get-version" ]]; then
		echo "bin/get-version is not found, installing it"
		curl -sSLo /tmp/get-version.zip https://github.com/scylladb-actions/get-version/releases/download/v0.3.0/get-version_0.3.0_linux_amd64v3.zip
		unzip /tmp/get-version.zip get-version -d "$(MAKEFILE_PATH)/bin" >/dev/null
	fi

.prepare-environment-update-aio-max-nr:
	@if (( $$(< /proc/sys/fs/aio-max-nr) < 2097152 )); then
		echo 2097152 | sudo tee /proc/sys/fs/aio-max-nr >/dev/null
	fi

clean-old-temporary-docker-images:
	@echo "Running Docker Hub image cleanup script..."
	python ci/clean-old-temporary-docker-images.py

CASSANDRA_VERSION_FILE=/tmp/cassandra-version-${CASSANDRA_VERSION}.resolved
resolve-cassandra-version: .prepare-get-version
	@find "${CASSANDRA_VERSION_FILE}" -mtime +0 -delete 2>/dev/null 1>&1 || true
	if [[ -f "${CASSANDRA_VERSION_FILE}" ]]; then
		echo "Resolved Cassandra ${CASSANDRA_VERSION} to $$(cat ${CASSANDRA_VERSION_FILE})"
		exit 0
	fi

	if [[ "${CASSANDRA_VERSION}" == "LATEST" ]]; then
		CASSANDRA_VERSION_RESOLVED=`get-version -source github-tag -repo apache/cassandra -prefix "cassandra-" -out-no-prefix -filters "^[0-9]+$$.^[0-9]+$$.^[0-9]+$$ and LAST.LAST.LAST" | tr -d '\"'`
	elif [[ "${CASSANDRA_VERSION}" == "5-LATEST" ]]; then
		CASSANDRA_VERSION_RESOLVED=`get-version -source github-tag -repo apache/cassandra -prefix "cassandra-" -out-no-prefix -filters "^[0-9]+$$.^[0-9]+$$.^[0-9]+$$ and 5.LAST.LAST" | tr -d '\"'`
	elif [[ "${CASSANDRA_VERSION}" == "4-LATEST" ]]; then
		CASSANDRA_VERSION_RESOLVED=`get-version -source github-tag -repo apache/cassandra -prefix "cassandra-" -out-no-prefix -filters "^[0-9]+$$.^[0-9]+$$.^[0-9]+$$ and 4.LAST.LAST" | tr -d '\"'`
	elif [[ "${CASSANDRA_VERSION}" == "3-LATEST" ]]; then
		CASSANDRA_VERSION_RESOLVED=`get-version -source github-tag -repo apache/cassandra -prefix "cassandra-" -out-no-prefix -filters "^[0-9]+$$.^[0-9]+$$.^[0-9]+$$ and 3.LAST.LAST" | tr -d '\"'`
	elif echo "${CASSANDRA_VERSION}" | grep -P '^[0-9\.]+'; then
		CASSANDRA_VERSION_RESOLVED=${CASSANDRA_VERSION}
	else
		echo "Unknown Cassandra version name '${CASSANDRA_VERSION}'"
		exit 1
	fi

	if [[ -z "$${CASSANDRA_VERSION_RESOLVED}" ]]; then
		echo "There is no ${CASSANDRA_VERSION} Cassandra version"
		if [[ -n "$${GITHUB_ENV}" ]]; then
			echo "value=NOT-FOUND" >>$${GITHUB_OUTPUT}
			echo "CASSANDRA_VERSION_RESOLVED=NOT-FOUND" >>$${GITHUB_ENV}
			exit 0
		fi
		exit 2
	fi

	echo "Resolved Cassandra ${CASSANDRA_VERSION} to $${CASSANDRA_VERSION_RESOLVED}"
	if [[ -n "$${GITHUB_OUTPUT}" ]]; then
		echo "value=$${CASSANDRA_VERSION_RESOLVED}" >>$${GITHUB_OUTPUT}
	fi
	if [[ -n "$${GITHUB_ENV}" ]]; then
		echo "CASSANDRA_VERSION_RESOLVED=$${CASSANDRA_VERSION_RESOLVED}" >>$${GITHUB_ENV}
	fi
	echo "$${CASSANDRA_VERSION_RESOLVED}" >${CASSANDRA_VERSION_FILE}

SCYLLA_VERSION_FILE=/tmp/scylla-version-${SCYLLA_VERSION}.resolved
resolve-scylla-version: .prepare-get-version
	@find "${SCYLLA_VERSION_FILE}" -mtime +0 -delete 2>/dev/null 1>&1 || true
	if [[ -f "${SCYLLA_VERSION_FILE}" ]]; then
		echo "Resolved ScyllaDB ${SCYLLA_VERSION} to $$(cat ${SCYLLA_VERSION_FILE})"
		exit 0
	fi

	if [[ "${SCYLLA_VERSION}" == "LTS-LATEST" ]]; then
		SCYLLA_VERSION_RESOLVED=`get-version --source dockerhub-imagetag --repo scylladb/scylla -filters "^[0-9]{4}$$.^[0-9]+$$.^[0-9]+$$ and LAST.1.LAST" | tr -d '\"'`
	elif [[ "${SCYLLA_VERSION}" == "LTS-PRIOR" ]]; then
		SCYLLA_VERSION_RESOLVED=`get-version --source dockerhub-imagetag --repo scylladb/scylla -filters "^[0-9]{4}$$.^[0-9]+$$.^[0-9]+$$ and LAST-1.1.LAST" | tr -d '\"'`
		if [[ -z "$${SCYLLA_VERSION_RESOLVED}" ]]; then
			SCYLLA_VERSION_RESOLVED=`get-version --source dockerhub-imagetag --repo scylladb/scylla-enterprise -filters "^[0-9]{4}$$.^[0-9]+$$.^[0-9]+$$ and LAST-1.1.LAST" | tr -d '\"'`
		fi
	elif [[ "${SCYLLA_VERSION}" == "LATEST" ]]; then
		SCYLLA_VERSION_RESOLVED=`get-version --source dockerhub-imagetag --repo scylladb/scylla -filters "^[0-9]{4}$$.^[0-9]+$$.^[0-9]+$$ and LAST.LAST.LAST" | tr -d '\"'`
	elif [[ "${SCYLLA_VERSION}" == "PRIOR" ]]; then
		SCYLLA_VERSION_RESOLVED=`get-version --source dockerhub-imagetag --repo scylladb/scylla -filters "^[0-9]{4}$$.^[0-9]+$$.^[0-9]+$$ and LAST.LAST.LAST-1" | tr -d '\"'`
	elif echo "${SCYLLA_VERSION}" | grep -P '^[0-9\.]+'; then
		SCYLLA_VERSION_RESOLVED=${SCYLLA_VERSION}
	else
		echo "Unknown ScyllaDB version name '${SCYLLA_VERSION}'"
		exit 1
	fi

	if [[ -z "$${SCYLLA_VERSION_RESOLVED}" ]]; then
		echo "There is no ${SCYLLA_VERSION} ScyllaDB version"
		if [[ -n "$${GITHUB_ENV}" ]]; then
			echo "value=NOT-FOUND" >>$${GITHUB_OUTPUT}
			echo "SCYLLA_VERSION_RESOLVED=NOT-FOUND" >>$${GITHUB_ENV}
			exit 0
		fi
		exit 2
	fi

	echo "Resolved ScyllaDB ${SCYLLA_VERSION} to $${SCYLLA_VERSION_RESOLVED}"
	if [[ -n "$${GITHUB_OUTPUT}" ]]; then
		echo "value=$${SCYLLA_VERSION_RESOLVED}" >>$${GITHUB_OUTPUT}
	fi
	if [[ -n "$${GITHUB_ENV}" ]]; then
		echo "SCYLLA_VERSION_RESOLVED=$${SCYLLA_VERSION_RESOLVED}" >>$${GITHUB_ENV}
	fi
	echo "$${SCYLLA_VERSION_RESOLVED}" >${SCYLLA_VERSION_FILE}

cassandra-start: .prepare-pki .prepare-cassandra-ccm .prepare-java resolve-cassandra-version
	@if [ -d ${CCM_CONFIG_DIR}/${CCM_CASSANDRA_CLUSTER_NAME} ] && ccm switch ${CCM_CASSANDRA_CLUSTER_NAME} 2>/dev/null 1>&2 && ccm status | grep UP 2>/dev/null 1>&2; then
		echo "Cassandra cluster is already started"
		exit 0
	fi
	if [[ -z "$${CASSANDRA_VERSION_RESOLVED}" ]]; then
		CASSANDRA_VERSION_RESOLVED=$$(cat '${CASSANDRA_VERSION_FILE}')
	fi
	if [[ -z "$${CASSANDRA_VERSION_RESOLVED}" ]]; then
		echo "Cassandra version ${CASSANDRA_VERSION} was not resolved"
		exit 1
	fi
	source ~/.sdkman/bin/sdkman-init.sh;
	echo "Start Cassandra ${CASSANDRA_VERSION}($${CASSANDRA_VERSION_RESOLVED}) cluster"
	ccm stop ${CCM_CASSANDRA_CLUSTER_NAME} 2>/dev/null 1>&2 || true
	ccm remove ${CCM_CASSANDRA_CLUSTER_NAME} 2>/dev/null 1>&2 || true
	ccm create ${CCM_CASSANDRA_CLUSTER_NAME} -i ${CCM_CASSANDRA_IP_PREFIX} -v "$${CASSANDRA_VERSION_RESOLVED}" -n3 -d --vnodes --jvm_arg="-Xmx256m -XX:NewSize=100m"
	ccm updateconf ${CASSANDRA_CONFIG}
	ccm start --wait-for-binary-proto --wait-other-notice --verbose
	ccm status
	ccm node1 nodetool status

scylla-start: .prepare-pki .prepare-scylla-ccm .prepare-environment-update-aio-max-nr resolve-scylla-version
	@if [ -d ${CCM_CONFIG_DIR}/${CCM_SCYLLA_CLUSTER_NAME} ] && ccm switch ${CCM_SCYLLA_CLUSTER_NAME} 2>/dev/null 1>&2 && ccm status | grep UP 2>/dev/null 1>&2; then
		echo "Scylla cluster is already started";
		exit 0;
	fi
	if [[ -z "$${SCYLLA_VERSION_RESOLVED}" ]]; then
		SCYLLA_VERSION_RESOLVED=$$(cat '${SCYLLA_VERSION_FILE}')
	fi
	if [[ -z "$${SCYLLA_VERSION_RESOLVED}" ]]; then
		echo "ScyllaDB version ${SCYLLA_VERSION} was not resolved"
		exit 1
	fi
	echo "Start scylla $(SCYLLA_VERSION)($${SCYLLA_VERSION_RESOLVED}) cluster"
	ccm stop ${CCM_SCYLLA_CLUSTER_NAME} 2>/dev/null 1>&2 || true
	ccm remove ${CCM_SCYLLA_CLUSTER_NAME} 2>/dev/null 1>&2 || true
	if [[ "$${SCYLLA_VERSION_RESOLVED}" != *:* ]]; then
		SCYLLA_VERSION_RESOLVED="release:$${SCYLLA_VERSION_RESOLVED}"
	fi
	ccm create ${CCM_SCYLLA_CLUSTER_NAME} -i ${CCM_SCYLLA_IP_PREFIX} --scylla -v $${SCYLLA_VERSION_RESOLVED} -n 3 -d --jvm_arg="--smp 2 --memory 1G --experimental-features udf --enable-user-defined-functions true"
	ccm updateconf ${SCYLLA_CONFIG}
	ccm start --wait-for-binary-proto --wait-other-notice --verbose
	ccm status
	ccm node1 nodetool status
	sudo chmod 0777 ${CCM_CONFIG_DIR}/${CCM_SCYLLA_CLUSTER_NAME}/*/cql.m || true

download-cassandra: .prepare-scylla-ccm resolve-cassandra-version
	@if [[ -z "$${CASSANDRA_VERSION_RESOLVED}" ]]; then
		CASSANDRA_VERSION_RESOLVED=$$(cat '${CASSANDRA_VERSION_FILE}')
	fi
	if [[ -z "$${CASSANDRA_VERSION_RESOLVED}" ]]; then
		echo "Cassandra version ${CASSANDRA_VERSION} was not resolved"
		exit 1
	fi
	rm -rf /tmp/download.ccm || true
	mkdir /tmp/download.ccm || true
	ccm create ccm_1 -i 127.0.254. -n 1:0 -v "$${CASSANDRA_VERSION_RESOLVED}" --config-dir=/tmp/download.ccm
	rm -rf /tmp/download.ccm

download-scylla: .prepare-scylla-ccm resolve-scylla-version
	@if [[ -z "$${SCYLLA_VERSION_RESOLVED}" ]]; then
		SCYLLA_VERSION_RESOLVED=$$(cat '${SCYLLA_VERSION_FILE}')
	fi
	if [[ -z "$${SCYLLA_VERSION_RESOLVED}" ]]; then
		echo "ScyllaDB version ${SCYLLA_VERSION} was not resolved"
		exit 1
	fi
	rm -rf /tmp/download.ccm || true
	mkdir /tmp/download.ccm || true
	if [[ "$${SCYLLA_VERSION_RESOLVED}" != *:* ]]; then
		SCYLLA_VERSION_RESOLVED="release:$${SCYLLA_VERSION_RESOLVED}"
	fi
	ccm create ccm_1 -i 127.0.254. -n 1:0 -v "$${SCYLLA_VERSION_RESOLVED}" --scylla --config-dir=/tmp/download.ccm
	rm -rf /tmp/download.ccm

cassandra-stop: .prepare-cassandra-ccm
	@echo "Stop cassandra cluster"
	@ccm stop --not-gently ${CCM_CASSANDRA_CLUSTER_NAME} 2>/dev/null 1>&2 || true
	@ccm remove ${CCM_CASSANDRA_CLUSTER_NAME} 2>/dev/null 1>&2 || true

scylla-stop: .prepare-scylla-ccm
	@echo "Stop scylla cluster"
	@ccm stop --not-gently ${CCM_SCYLLA_CLUSTER_NAME} 2>/dev/null 1>&2 || true
	@ccm remove ${CCM_SCYLLA_CLUSTER_NAME} 2>/dev/null 1>&2 || true

test-integration-cassandra: cassandra-start
	@echo "Run integration tests for proto ${TEST_CQL_PROTOCOL} on cassandra ${CASSANDRA_VERSION}"
	if [[ -z "$${CASSANDRA_VERSION_RESOLVED}" ]]; then
		CASSANDRA_VERSION_RESOLVED=$$(cat '${CASSANDRA_VERSION_FILE}')
	fi
	if [[ -z "$${CASSANDRA_VERSION_RESOLVED}" ]]; then
		echo "Cassandra version ${CASSANDRA_VERSION} was not resolved"
		exit 1
	fi
	echo "go test -v ${TEST_OPTS} -tags \"${TEST_INTEGRATION_TAGS}\" -distribution cassandra -timeout=5m -runauth -gocql.timeout=60s -runssl -proto=${TEST_CQL_PROTOCOL} -rf=3 -clusterSize=3 -autowait=2000ms -compressor=${TEST_COMPRESSOR} -gocql.cversion=$${CASSANDRA_VERSION_RESOLVED} -cluster=$$(ccm liveset) ./..."
	go test -v ${TEST_OPTS} -tags "${TEST_INTEGRATION_TAGS}" -distribution cassandra -timeout=5m -runauth -gocql.timeout=60s -runssl -proto=${TEST_CQL_PROTOCOL} -rf=3 -clusterSize=3 -autowait=2000ms -compressor=${TEST_COMPRESSOR} -gocql.cversion=$$(ccm node1 versionfrombuild) -cluster=$$(ccm liveset) ./...

test-integration-scylla: scylla-start
	@echo "Run integration tests for proto ${TEST_CQL_PROTOCOL} on scylla ${SCYLLA_VERSION}"
	if [ -S "${CCM_CONFIG_DIR}/${CCM_SCYLLA_CLUSTER_NAME}/node1/cql.m" ]; then
		CLUSTER_SOCKET="-cluster-socket ${CCM_CONFIG_DIR}/${CCM_SCYLLA_CLUSTER_NAME}/node1/cql.m"
	else
		echo "Cluster socket is not found"
	fi
	if [[ -z "$${SCYLLA_VERSION_RESOLVED}" ]]; then
		SCYLLA_VERSION_RESOLVED=$$(cat '${SCYLLA_VERSION_FILE}')
	fi
	if [[ -z "$${SCYLLA_VERSION_RESOLVED}" ]]; then
		echo "ScyllaDB version ${SCYLLA_VERSION} was not resolved"
		exit 1
	fi
	echo "go test -v ${TEST_OPTS} -tags \"${TEST_INTEGRATION_TAGS}\" -distribution scylla $${CLUSTER_SOCKET} -timeout=5m -gocql.timeout=60s -proto=${TEST_CQL_PROTOCOL} -rf=3 -clusterSize=3 -autowait=2000ms -compressor=${TEST_COMPRESSOR} -gocql.cversion=$${SCYLLA_VERSION_RESOLVED} -cluster=$$(ccm liveset) ./..."
	go test -v ${TEST_OPTS} -tags "${TEST_INTEGRATION_TAGS}" -distribution scylla $${CLUSTER_SOCKET} -timeout=5m -gocql.timeout=60s -proto=${TEST_CQL_PROTOCOL} -rf=3 -clusterSize=3 -autowait=2000ms -compressor=${TEST_COMPRESSOR} -gocql.cversion=$${SCYLLA_VERSION_RESOLVED} -cluster=$$(ccm liveset) ./...

test-unit: .prepare-pki
	@echo "Run unit tests"
	go clean -testcache
ifeq ($(shell if [[ -n "$${GITHUB_STEP_SUMMARY}" ]]; then echo "running-in-workflow"; else echo "running-in-shell"; fi), running-in-workflow)
	echo "### Unit Test Results" >>$${GITHUB_STEP_SUMMARY}
	echo '```' >>$${GITHUB_STEP_SUMMARY}
	echo go test -tags unit -timeout=5m -race ./...
	go test -tags unit -timeout=5m -race ./... | tee -a "$${GITHUB_STEP_SUMMARY}"; TEST_STATUS=$${PIPESTATUS[0]}; echo '```' >>"$${GITHUB_STEP_SUMMARY}"; exit "$${TEST_STATUS}"
else
	go test -v -tags unit -timeout=5m -race ./...
endif

test-bench:
	@echo "Run benchmark tests"
ifeq ($(shell if [[ -n "$${GITHUB_STEP_SUMMARY}" ]]; then echo "running-in-workflow"; else echo "running-in-shell"; fi), running-in-workflow)
	echo "### Benchmark Results" >>$${GITHUB_STEP_SUMMARY}
	echo '```' >>"$${GITHUB_STEP_SUMMARY}"
	echo go test -bench=. -benchmem ./...
	go test -bench=. -benchmem ./... | tee -a >>"$${GITHUB_STEP_SUMMARY}"
	echo '```' >>"$${GITHUB_STEP_SUMMARY}"
else
	go test -bench=. -benchmem ./...
endif

check: .prepare-golangci
	@echo "Build"
	go build -tags all .
	echo "Check linting"
	${BIN_DIR}/golangci-lint run

fix: .prepare-golangci
	@echo "Fix linting"
	${BIN_DIR}/golangci-lint run --fix

.prepare-java:
ifeq ($(shell if [ -f ~/.sdkman/bin/sdkman-init.sh ]; then echo "installed"; else echo "not-installed"; fi), not-installed)
	@$(MAKE) install-java
endif

install-java:
	@echo "Installing SDKMAN..."
	curl -s "https://get.sdkman.io" | bash
	echo "sdkman_auto_answer=true" >> ~/.sdkman/etc/config
	source ~/.sdkman/bin/sdkman-init.sh;
	echo "Installing Java versions...";
	sdk install java 11.0.24-zulu;
	sdk install java 17.0.12-zulu;
	sdk default java 11.0.24-zulu;
	sdk use java 11.0.24-zulu;

.prepare-cassandra-ccm:
	@if command -v ccm >/dev/null 2>&1 && grep CASSANDRA ${CCM_CONFIG_DIR}/ccm-type 2>/dev/null 1>&2 && grep ${CCM_CASSANDRA_VERSION} ${CCM_CONFIG_DIR}/ccm-version 2>/dev//null  1>&2; then
		echo "Cassandra CCM ${CCM_CASSANDRA_VERSION} is already installed";
		exit 0
	fi
	$(MAKE) install-cassandra-ccm

install-cassandra-ccm:
	@echo "Install CCM ${CCM_CASSANDRA_VERSION}"
	pip install "git+https://${CCM_CASSANDRA_REPO}.git@${CCM_CASSANDRA_VERSION}"
	mkdir ${CCM_CONFIG_DIR} 2>/dev/null || true
	echo CASSANDRA > ${CCM_CONFIG_DIR}/ccm-type
	echo ${CCM_CASSANDRA_VERSION} > ${CCM_CONFIG_DIR}/ccm-version

.prepare-scylla-ccm:
	@if command -v ccm >/dev/null 2>&1 && grep SCYLLA ${CCM_CONFIG_DIR}/ccm-type 2>/dev/null 1>&2 && grep ${CCM_SCYLLA_VERSION} ${CCM_CONFIG_DIR}/ccm-version 2>/dev//null  1>&2; then
		echo "Scylla CCM ${CCM_SCYLLA_VERSION} is already installed";
		exit 0
	fi
	$(MAKE) install-scylla-ccm

install-scylla-ccm:
	@echo "Installing Scylla CCM ${CCM_SCYLLA_VERSION}"
	pip install "git+https://${CCM_SCYLLA_REPO}.git@${CCM_SCYLLA_VERSION}"
	mkdir ${CCM_CONFIG_DIR} 2>/dev/null || true
	echo SCYLLA > ${CCM_CONFIG_DIR}/ccm-type
	echo ${CCM_SCYLLA_VERSION} > ${CCM_CONFIG_DIR}/ccm-version

.prepare-pki:
	@[ -f "testdata/pki/cassandra.key" ] || (echo "Generating new PKI" && cd testdata/pki/ && bash ./generate_certs.sh)

generate-pki:
	@echo "Generating new PKI"
	rm -f testdata/pki/.keystore testdata/pki/.truststore testdata/pki/*.p12 testdata/pki/*.key testdata/pki/*.crt || true
	cd testdata/pki/ && bash ./generate_certs.sh

.prepare-golangci:
	@if ! "${BIN_DIR}/golangci-lint" --version | grep '${GOLANGCI_VERSION}' >/dev/null 2>&1 ; then
		mkdir -p "${BIN_DIR}"
		echo "Installing golangci-lint to '${BIN_DIR}'"
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b bin/ v$(GOLANGCI_VERSION)
	fi
