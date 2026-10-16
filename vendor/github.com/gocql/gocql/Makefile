SHELL := bash
.ONESHELL:
.SHELLFLAGS := -eo pipefail -c

MAKEFILE_PATH := $(abspath $(dir $(abspath $(lastword $(MAKEFILE_LIST)))))
KEY_PATH = ${MAKEFILE_PATH}/testdata/pki
BIN_DIR := "${MAKEFILE_PATH}/bin"

CASSANDRA_VERSION ?= LATEST
SCYLLA_VERSION ?= LATEST

HOST_ARCH := $(shell uname -m)

GOLANGCI_VERSION = 2.5.0
GET_VERSION_VERSION = 0.4.5
GET_VERSION_BIN = $(MAKEFILE_PATH)/bin/get-version
# get-version publishes one release archive per architecture, and every
# resolve-*-version target below depends on the binary being runnable: the
# amd64v3 build is an "Exec format error" on aarch64.
GET_VERSION_ARCH = $(if $(filter aarch64,$(HOST_ARCH)),arm64,amd64v3)

# scylla-ccm chooses which relocatable package to download from SCYLLA_ARCH and
# falls back to x86_64 (ccmlib/scylla_repository.py), so on an aarch64 host it
# would fetch a package whose scylla binary cannot run. uname -m already prints
# the two names ccm knows, x86_64 and aarch64.
SCYLLA_ARCH ?= $(HOST_ARCH)

TEST_CQL_PROTOCOL ?= 4
TEST_COMPRESSOR ?= snappy
TEST_OPTS ?=
# go test's own -timeout for both integration targets. It cannot be set through
# TEST_OPTS: TEST_OPTS is interpolated ahead of -timeout on the command line, and
# the later flag wins, so a -timeout there is silently discarded.
# Measured locally the suite lands anywhere between 280s and 470s depending on
# what else the machine is doing -- the protocol version barely moves it -- so
# the 10m default is only about 25% clear of the worst case. Overrunning it
# panics with a goroutine dump instead of naming the failing test, which is a
# bad way for a lane to fail, hence a knob to give a run more room.
# It covers the ScyllaDB target too, which used to hardcode 5m and so ignored the
# knob entirely; that raises the ScyllaDB default from 5m to 10m, which only
# changes how long a hung run burns before it dumps.
TEST_INTEGRATION_TIMEOUT ?= 10m
TEST_INTEGRATION_TAGS ?= integration gocql_debug
JVM_EXTRA_OPTS ?= -Dcassandra.test.fail_writes_ks=test -Dcassandra.custom_query_handler_class=org.apache.cassandra.cql3.CustomPayloadMirroringQueryHandler

# COVER_ARGS is appended to every `go test ... ./...` invocation below. It is
# empty by default, so plain `make test-unit` behaves exactly as before; the
# *-coverage targets further down set it via a recursive `make` call to
# instrument the same test run instead of duplicating its recipe.
COVER_ARGS ?=

# test-integration-scylla/cassandra can't use the same COVER_ARGS trick: they
# already pass custom, test-binary-defined flags (-distribution, -cluster,
# ...) that `go test` itself doesn't recognize. Once `go test` hits the first
# such flag, it stops parsing its own flags -- including the package pattern
# (./...), which then silently defaults to "." instead of erroring -- so
# anything placed after it (like COVER_ARGS previously was) is unreliable.
# COVER_BUILD_ARGS (flags `go test` itself must recognize, e.g. -cover)
# goes before the package pattern; COVER_RUNTIME_ARGS (everything meant for
# the test binary, after -args) goes after it, alongside the custom flags.
COVER_BUILD_ARGS ?=
COVER_RUNTIME_ARGS ?=
COVERAGE_DIR ?= $(MAKEFILE_PATH)/.coverage/data
# Normalize to an absolute path even when overridden on the command line
# (e.g. COVERAGE_DIR=.cov): go test's subpackages (and lz4, run via -C) each
# execute with their own directory as the working directory, so a relative
# -test.gocoverdir resolves differently -- and usually inaccessibly -- per
# package instead of to one shared directory at the repo root. `override` is
# required here: a variable set on the `make` command line takes precedence
# over a plain assignment in the Makefile, so without it this would silently
# do nothing whenever someone actually overrides COVERAGE_DIR.
override COVERAGE_DIR := $(abspath $(COVERAGE_DIR))

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
export SCYLLA_ARCH
export JAVA11_HOME=${JAVA_HOME_11_X64}
export JAVA17_HOME=${JAVA_HOME_17_X64}
export JAVA_HOME=${JAVA_HOME_11_X64}
export PATH := $(MAKEFILE_PATH)/bin:~/.sdkman/bin:$(PATH)

print-config:
	echo ${CASSANDRA_CONFIG}

.prepare-bin:
	@[[ -d "$(MAKEFILE_PATH)/bin" ]] || mkdir "$(MAKEFILE_PATH)/bin"

.prepare-get-version: .prepare-bin
	@if [[ ! -x "${GET_VERSION_BIN}" ]] || [[ "$$("${GET_VERSION_BIN}" -version 2>/dev/null)" != "${GET_VERSION_VERSION}" ]]; then
		echo "Installing get-version ${GET_VERSION_VERSION}"
		curl -sSLo /tmp/get-version.zip https://github.com/scylladb-actions/get-version/releases/download/v${GET_VERSION_VERSION}/get-version_${GET_VERSION_VERSION}_linux_${GET_VERSION_ARCH}.zip
		unzip -o /tmp/get-version.zip get-version -d "$(MAKEFILE_PATH)/bin" >/dev/null
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

# Unstable references contain '/' and ':', so they cannot be used raw as a
# cache filename. Encode underscores first so the three replacements cannot
# collide (for example, unstable/a_b:c and unstable/a:b_c stay distinct).
SCYLLA_VERSION_CACHE_KEY=$(subst :,_colon_,$(subst /,_slash_,$(subst _,_underscore_,$(SCYLLA_VERSION))))
SCYLLA_VERSION_FILE=/tmp/scylla-version-${SCYLLA_VERSION_CACHE_KEY}.resolved
SCYLLA_VERSION_RESOLVER=$(MAKEFILE_PATH)/ci/resolve-scylla-ccm-version.sh
resolve-scylla-version: .prepare-get-version
	@find "${SCYLLA_VERSION_FILE}" -mtime +0 -delete 2>/dev/null 1>&1 || true
	SCYLLA_VERSION_CACHED=
	if [[ -f "${SCYLLA_VERSION_FILE}" ]]; then
		SCYLLA_VERSION_CACHED=$$(cat "${SCYLLA_VERSION_FILE}")
		# Cache files outlive this Makefile. Discard a partial release written by
		# the old resolver rather than letting it reach CCM through the fast path.
		if ! bash "${SCYLLA_VERSION_RESOLVER}" --check "$${SCYLLA_VERSION_CACHED}"; then
			rm -f "${SCYLLA_VERSION_FILE}"
			SCYLLA_VERSION_CACHED=
		fi
	fi

	if [[ -n "$${SCYLLA_VERSION_CACHED}" ]]; then
		SCYLLA_VERSION_RESOLVED=$${SCYLLA_VERSION_CACHED}
	else
		SCYLLA_VERSION_RESOLVED=$$(GET_VERSION_BIN="${GET_VERSION_BIN}" bash "${SCYLLA_VERSION_RESOLVER}" "${SCYLLA_VERSION}")
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
	# Do not refresh the cache mtime on a hit. Aliases and partial release lines
	# must be looked up again after the one-day expiry so they can advance.
	if [[ -z "$${SCYLLA_VERSION_CACHED}" ]]; then
		echo "$${SCYLLA_VERSION_RESOLVED}" >"${SCYLLA_VERSION_FILE}"
	fi

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
	for conf_dir in ${CCM_CONFIG_DIR}/${CCM_CASSANDRA_CLUSTER_NAME}/node*/conf; do \
		sed -i 's/^#MAX_HEAP_SIZE=.*/MAX_HEAP_SIZE="256M"/' "$$conf_dir/cassandra-env.sh"; \
	done
	ccm start --wait-for-binary-proto --wait-other-notice --verbose
	ccm status
	ccm node1 nodetool status

scylla-start: .prepare-pki .prepare-scylla-ccm .prepare-environment-update-aio-max-nr resolve-scylla-version
	@if [ -d ${CCM_CONFIG_DIR}/${CCM_SCYLLA_CLUSTER_NAME} ] && ccm switch ${CCM_SCYLLA_CLUSTER_NAME} 2>/dev/null 1>&2 && ccm status | grep UP 2>/dev/null 1>&2; then
		echo "Scylla cluster is already started";
		exit 0;
	fi
	SCYLLA_VERSION_RESOLVED=$$(cat '${SCYLLA_VERSION_FILE}')
	if [[ -z "$${SCYLLA_VERSION_RESOLVED}" ]]; then
		echo "ScyllaDB version ${SCYLLA_VERSION} was not resolved"
		exit 1
	fi
	if ! bash "${SCYLLA_VERSION_RESOLVER}" --check "$${SCYLLA_VERSION_RESOLVED}"; then
		echo "ScyllaDB CCM version '$${SCYLLA_VERSION_RESOLVED}' does not name one release build or an unstable/hotfix build"
		exit 1
	fi
	echo "Start scylla $(SCYLLA_VERSION)($${SCYLLA_VERSION_RESOLVED}) cluster"
	ccm stop ${CCM_SCYLLA_CLUSTER_NAME} 2>/dev/null 1>&2 || true
	ccm remove ${CCM_SCYLLA_CLUSTER_NAME} 2>/dev/null 1>&2 || true
	ccm create ${CCM_SCYLLA_CLUSTER_NAME} -i ${CCM_SCYLLA_IP_PREFIX} --scylla -v "$${SCYLLA_VERSION_RESOLVED}" -n 3 -d --jvm_arg="--smp 2 --memory 1G --experimental-features udf --enable-user-defined-functions true"
	ccm updateconf ${SCYLLA_CONFIG}
	ccm start --wait-for-binary-proto --wait-other-notice --verbose
	ccm status
	ccm node1 nodetool status
	sudo chmod 0777 ${CCM_CONFIG_DIR}/${CCM_SCYLLA_CLUSTER_NAME}/*/cql.m || true

download-cassandra: .prepare-cassandra-ccm resolve-cassandra-version
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
	@SCYLLA_VERSION_RESOLVED=$$(cat '${SCYLLA_VERSION_FILE}')
	if [[ -z "$${SCYLLA_VERSION_RESOLVED}" ]]; then
		echo "ScyllaDB version ${SCYLLA_VERSION} was not resolved"
		exit 1
	fi
	if ! bash "${SCYLLA_VERSION_RESOLVER}" --check "$${SCYLLA_VERSION_RESOLVED}"; then
		echo "ScyllaDB CCM version '$${SCYLLA_VERSION_RESOLVED}' does not name one release build or an unstable/hotfix build"
		exit 1
	fi
	rm -rf /tmp/download.ccm || true
	mkdir /tmp/download.ccm || true
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

# The package pattern below is "." (this package only), not "./...". All
# integration-tagged test files live in this package; the handful of
# unconditional (untagged) test files in other packages -- dialer,
# hostpolicy, several serialization/* packages, etc. -- would also get
# built and would fail to parse -distribution/-cluster/etc, which only this
# package's test files register. Cross-package coverage attribution still
# works without testing those packages directly: -coverpkg=./... controls
# what gets *instrumented*, separately from what gets *run*, so code in
# other packages exercised transitively through this package's tests is
# still measured under test-integration-*-coverage.
test-integration-cassandra: cassandra-start
	@echo "Run integration tests for proto ${TEST_CQL_PROTOCOL} on cassandra ${CASSANDRA_VERSION}"
	if [[ -z "$${CASSANDRA_VERSION_RESOLVED}" ]]; then
		CASSANDRA_VERSION_RESOLVED=$$(cat '${CASSANDRA_VERSION_FILE}')
	fi
	if [[ -z "$${CASSANDRA_VERSION_RESOLVED}" ]]; then
		echo "Cassandra version ${CASSANDRA_VERSION} was not resolved"
		exit 1
	fi
	echo "go test -v ${TEST_OPTS} -tags \"${TEST_INTEGRATION_TAGS}\" ${COVER_BUILD_ARGS} -timeout=${TEST_INTEGRATION_TIMEOUT} . -args -distribution cassandra -runauth -gocql.timeout=60s -runssl -proto=${TEST_CQL_PROTOCOL} -rf=3 -clusterSize=3 -autowait=2000ms -compressor=${TEST_COMPRESSOR} -gocql.cversion=$${CASSANDRA_VERSION_RESOLVED} -cluster=$$(ccm liveset) ${COVER_RUNTIME_ARGS}"
	go test -v ${TEST_OPTS} -tags "${TEST_INTEGRATION_TAGS}" ${COVER_BUILD_ARGS} -timeout=${TEST_INTEGRATION_TIMEOUT} . -args -distribution cassandra -runauth -gocql.timeout=60s -runssl -proto=${TEST_CQL_PROTOCOL} -rf=3 -clusterSize=3 -autowait=2000ms -compressor=${TEST_COMPRESSOR} -gocql.cversion=$$(ccm node1 versionfrombuild) -cluster=$$(ccm liveset) ${COVER_RUNTIME_ARGS}

test-integration-scylla: scylla-start
	@echo "Run integration tests for proto ${TEST_CQL_PROTOCOL} on scylla ${SCYLLA_VERSION}"
	if [ -S "${CCM_CONFIG_DIR}/${CCM_SCYLLA_CLUSTER_NAME}/node1/cql.m" ]; then
		CLUSTER_SOCKET="-cluster-socket ${CCM_CONFIG_DIR}/${CCM_SCYLLA_CLUSTER_NAME}/node1/cql.m"
	else
		echo "Cluster socket is not found"
	fi
	SCYLLA_VERSION_RESOLVED=$$(cat '${SCYLLA_VERSION_FILE}')
	if [[ -z "$${SCYLLA_VERSION_RESOLVED}" ]]; then
		echo "ScyllaDB version ${SCYLLA_VERSION} was not resolved"
		exit 1
	fi
	# CCM references describe where a build came from, not the version string the
	# test binary parses. Read the installed build so unstable references work too.
	SCYLLA_SERVER_VERSION=$$(ccm node1 versionfrombuild | sed -E 's/~([[:alpha:]])/-\1/; s/^([0-9]+\.[0-9]+)\.([[:alpha:]])/\1.0-\2/')
	echo "go test -v ${TEST_OPTS} -tags \"${TEST_INTEGRATION_TAGS}\" ${COVER_BUILD_ARGS} -timeout=${TEST_INTEGRATION_TIMEOUT} . -args -distribution scylla $${CLUSTER_SOCKET} -gocql.timeout=60s -proto=${TEST_CQL_PROTOCOL} -rf=3 -clusterSize=3 -autowait=2000ms -compressor=${TEST_COMPRESSOR} -gocql.cversion=$${SCYLLA_SERVER_VERSION} -cluster=$$(ccm liveset) ${COVER_RUNTIME_ARGS}"
	go test -v ${TEST_OPTS} -tags "${TEST_INTEGRATION_TAGS}" ${COVER_BUILD_ARGS} -timeout=${TEST_INTEGRATION_TIMEOUT} . -args -distribution scylla $${CLUSTER_SOCKET} -gocql.timeout=60s -proto=${TEST_CQL_PROTOCOL} -rf=3 -clusterSize=3 -autowait=2000ms -compressor=${TEST_COMPRESSOR} -gocql.cversion=$${SCYLLA_SERVER_VERSION} -cluster=$$(ccm liveset) ${COVER_RUNTIME_ARGS}

# The lz4 compressor lives in a nested module (lz4/go.mod), so the root "./..."
# pattern does not reach it — it has to be invoked explicitly with `go test -C`,
# the same way check-go-mod-drift lists every module. Without that, the lz4
# tests and benchmarks are green or red independently of CI.
test-unit: .prepare-pki
	@echo "Run unit tests"
	bash ci/resolve-scylla-ccm-version_test.sh
	go clean -testcache
	go clean -C lz4 -testcache
ifeq ($(shell if [[ -n "$${GITHUB_STEP_SUMMARY}" ]]; then echo "running-in-workflow"; else echo "running-in-shell"; fi), running-in-workflow)
	echo "### Unit Test Results" >>$${GITHUB_STEP_SUMMARY}
	echo '```' >>$${GITHUB_STEP_SUMMARY}
	echo go test -tags unit -timeout=5m -race ./... ${COVER_ARGS}
	TEST_STATUS=0
	go test -tags unit -timeout=5m -race ./... ${COVER_ARGS} | tee -a "$${GITHUB_STEP_SUMMARY}" || TEST_STATUS=$${PIPESTATUS[0]}
	echo go test -C lz4 -tags unit -timeout=5m -race ./... ${COVER_ARGS}
	LZ4_TEST_STATUS=0
	go test -C lz4 -tags unit -timeout=5m -race ./... ${COVER_ARGS} | tee -a "$${GITHUB_STEP_SUMMARY}" || LZ4_TEST_STATUS=$${PIPESTATUS[0]}
	echo '```' >>"$${GITHUB_STEP_SUMMARY}"
	if (( TEST_STATUS != 0 )); then exit "$${TEST_STATUS}"; fi
	exit "$${LZ4_TEST_STATUS}"
else
	go test -v -tags unit -timeout=5m -race ./... ${COVER_ARGS}
	go test -C lz4 -v -tags unit -timeout=5m -race ./... ${COVER_ARGS}
endif

.prepare-coverage-dir:
	@mkdir -p "${COVERAGE_DIR}"

# Coverage-instrumented variants of the targets above. Each recurses into the
# original target with COVER_ARGS/COVER_BUILD_ARGS/COVER_RUNTIME_ARGS set,
# rather than duplicating its recipe, so CCM setup, version resolution and
# GITHUB_STEP_SUMMARY handling stay in one place. Every *-coverage target
# writes into the same COVERAGE_DIR, so unit and integration runs (and
# multiple integration runs against different clusters/tags) all accumulate
# into one combined report -- see coverage-report.
#
# COVERAGE_DIR is quoted (with an escaped \" so the quote survives being
# re-expanded, unquoted, inside the target recipe's own `go test` line) so a
# path containing spaces isn't word-split by the shell into several
# arguments.
#
# -covermode=atomic is explicit (rather than relying on -race, which forces
# it) because `go tool covdata` refuses to merge data recorded under
# different counter modes ("counter mode clash"): every *-coverage target
# and ccm-test-coverage need to agree, not just test-unit-coverage, which is
# the only one that would otherwise get atomic mode for free from -race.
test-unit-coverage: .prepare-coverage-dir
	@$(MAKE) test-unit COVER_ARGS="-cover -covermode=atomic -coverpkg=./... -args -test.gocoverdir=\"${COVERAGE_DIR}\""

test-integration-scylla-coverage: .prepare-coverage-dir
	@$(MAKE) test-integration-scylla COVER_BUILD_ARGS="-cover -covermode=atomic -coverpkg=./..." COVER_RUNTIME_ARGS="-test.gocoverdir=\"${COVERAGE_DIR}\""

test-integration-cassandra-coverage: .prepare-coverage-dir
	@$(MAKE) test-integration-cassandra COVER_BUILD_ARGS="-cover -covermode=atomic -coverpkg=./..." COVER_RUNTIME_ARGS="-test.gocoverdir=\"${COVERAGE_DIR}\""

# internal/ccm's tests (the only tests the "ccm" build tag selects -- nothing
# else in the module has a file gated by it) don't take any of the custom
# flags test-integration-scylla passes (-distribution, -cluster, ...); its
# test binary doesn't define them and would fail to parse them. It also needs
# to be targeted directly rather than through ./...: `go test` stops
# resolving its own flags (including the package pattern) at the first flag
# it doesn't recognize, so a ./... alongside those custom flags silently
# collapses to just ".", which is why reusing test-integration-scylla for
# this never actually ran internal/ccm's tests.
ccm-test-coverage: .prepare-coverage-dir
	@go test -tags "ccm gocql_debug" -timeout=5m -v -cover -covermode=atomic -coverpkg=./... ./internal/ccm/... -args -test.gocoverdir="${COVERAGE_DIR}"

# lz4 is a separate Go module (see its go.mod), so a single `go tool covdata`
# invocation can't render both it and the root module together -- `go tool
# cover` resolves source files against the module rooted at the current
# directory, and a merged profile spanning two modules fails with "no
# required module provides package ...". Filter per module with `-pkg` and
# run `go tool cover` from within each module's own directory instead.
coverage-report:
	@echo "Generate coverage report"
	go tool covdata textfmt -i="${COVERAGE_DIR}" -pkg="github.com/gocql/gocql/..." -o="${MAKEFILE_PATH}/coverage-root.out"
	go tool covdata textfmt -i="${COVERAGE_DIR}" -pkg="github.com/scylladb/gocql/lz4/..." -o="${MAKEFILE_PATH}/coverage-lz4.out"
ifeq ($(shell if [[ -n "$${GITHUB_STEP_SUMMARY}" ]]; then echo "running-in-workflow"; else echo "running-in-shell"; fi), running-in-workflow)
	echo "### Coverage Report" >>$${GITHUB_STEP_SUMMARY}
	echo '```' >>$${GITHUB_STEP_SUMMARY}
	go tool covdata percent -i="${COVERAGE_DIR}" -pkg="github.com/gocql/gocql/..." | tee -a "$${GITHUB_STEP_SUMMARY}"
	go tool cover -func="${MAKEFILE_PATH}/coverage-root.out" | tail -1 | sed 's/^total:/root module total:/' | tee -a "$${GITHUB_STEP_SUMMARY}"
	go tool covdata percent -i="${COVERAGE_DIR}" -pkg="github.com/scylladb/gocql/lz4/..." | tee -a "$${GITHUB_STEP_SUMMARY}"
	(cd lz4 && go tool cover -func="${MAKEFILE_PATH}/coverage-lz4.out") | tail -1 | sed 's/^total:/lz4 module total:/' | tee -a "$${GITHUB_STEP_SUMMARY}"
	echo '```' >>"$${GITHUB_STEP_SUMMARY}"
else
	go tool covdata percent -i="${COVERAGE_DIR}" -pkg="github.com/gocql/gocql/..."
	go tool cover -func="${MAKEFILE_PATH}/coverage-root.out" | tail -1 | sed 's/^total:/root module total:/'
	go tool covdata percent -i="${COVERAGE_DIR}" -pkg="github.com/scylladb/gocql/lz4/..."
	(cd lz4 && go tool cover -func="${MAKEFILE_PATH}/coverage-lz4.out") | tail -1 | sed 's/^total:/lz4 module total:/'
endif
	go tool cover -html="${MAKEFILE_PATH}/coverage-root.out" -o="${MAKEFILE_PATH}/coverage-root.html"
	(cd lz4 && go tool cover -html="${MAKEFILE_PATH}/coverage-lz4.out" -o="${MAKEFILE_PATH}/coverage-lz4.html")

clean-coverage:
	rm -rf "${COVERAGE_DIR}" "${MAKEFILE_PATH}"/coverage-root.* "${MAKEFILE_PATH}"/coverage-lz4.*

# The benchmarks are behind the `bench` build tag (and, in the root module, some
# behind `unit`), so the tags have to be passed or `go test -bench` compiles and
# runs almost nothing. `-run '^$$'` keeps this a benchmark-only run; the tests
# themselves belong to test-unit.
BENCH_OPTS = -tags "bench unit" -run '^$$' -bench=. -benchmem

# Each module's status is captured rather than left to abort the recipe, on both branches.
# .SHELLFLAGS is -eo pipefail, so a bare failing command takes the whole target down where
# it stands -- skipping the modules after it, and in the workflow branch the closing fence
# with them. The tests/bench run is last and is the one that matters most: it is the only
# end-to-end check that the frame hashing still matches a recording, and a root-module
# regression must not be what stops it from running. The bench workflow is label-gated, so
# on most changes the shell branch below is the only one that ever runs this at all.
# test-unit's workflow branch does the same dance for the same reason; its shell branch
# does not, so a root-module failure there still skips lz4.
test-bench:
	@echo "Run benchmark tests"
ifeq ($(shell if [[ -n "$${GITHUB_STEP_SUMMARY}" ]]; then echo "running-in-workflow"; else echo "running-in-shell"; fi), running-in-workflow)
	echo "### Benchmark Results" >>$${GITHUB_STEP_SUMMARY}
	echo '```' >>"$${GITHUB_STEP_SUMMARY}"
	echo go test ${BENCH_OPTS} ./...
	BENCH_STATUS=0
	go test ${BENCH_OPTS} ./... | tee -a "$${GITHUB_STEP_SUMMARY}" || BENCH_STATUS=$${PIPESTATUS[0]}
	echo go test -C lz4 ${BENCH_OPTS} ./...
	LZ4_BENCH_STATUS=0
	go test -C lz4 ${BENCH_OPTS} ./... | tee -a "$${GITHUB_STEP_SUMMARY}" || LZ4_BENCH_STATUS=$${PIPESTATUS[0]}
	echo go test -C tests/bench ${BENCH_OPTS} ./...
	REPLAY_BENCH_STATUS=0
	go test -C tests/bench ${BENCH_OPTS} ./... | tee -a "$${GITHUB_STEP_SUMMARY}" || REPLAY_BENCH_STATUS=$${PIPESTATUS[0]}
	echo '```' >>"$${GITHUB_STEP_SUMMARY}"
	if (( BENCH_STATUS != 0 )); then exit "$${BENCH_STATUS}"; fi
	if (( LZ4_BENCH_STATUS != 0 )); then exit "$${LZ4_BENCH_STATUS}"; fi
	exit "$${REPLAY_BENCH_STATUS}"
else
	echo go test ${BENCH_OPTS} ./...
	BENCH_STATUS=0
	go test ${BENCH_OPTS} ./... || BENCH_STATUS=$$?
	echo go test -C lz4 ${BENCH_OPTS} ./...
	LZ4_BENCH_STATUS=0
	go test -C lz4 ${BENCH_OPTS} ./... || LZ4_BENCH_STATUS=$$?
	echo go test -C tests/bench ${BENCH_OPTS} ./...
	REPLAY_BENCH_STATUS=0
	go test -C tests/bench ${BENCH_OPTS} ./... || REPLAY_BENCH_STATUS=$$?
	if (( BENCH_STATUS != 0 )); then exit "$${BENCH_STATUS}"; fi
	if (( LZ4_BENCH_STATUS != 0 )); then exit "$${LZ4_BENCH_STATUS}"; fi
	exit "$${REPLAY_BENCH_STATUS}"
endif

check-go-mod-drift:
	@echo "Check Go module drift"
	go mod tidy -diff
	go mod tidy -C lz4 -diff
	go mod tidy -C tests/bench -diff

# The one architecture-dependent part of check, split out so the arm64 lane can run
# it without the linters, which are not. It is not redundant with test-unit either:
# `go test` compiles no non-test file the `all` tag guards -- integration_only.go,
# internal/ccm, internal/debug/debug_on.go.
build:
	@echo "Build"
	go build -tags all .

# go.mod's require on github.com/scylladb/gocql/lz4 is inert here -- the `replace`
# beside it wins for every local build -- and Renovate skips it, so nothing notices
# when it drifts from the module's release tags. Consumers see the require and not
# the replace, so it still has to resolve for them. See go.mod and README.md 5.4.
check-lz4-pin:
	@echo "Check the lz4 pin against the newest lz4/v* tag"
	PINNED=$$(go list -m -f '{{.Version}}' github.com/scylladb/gocql/lz4) || {
		echo "::error::go.mod has no require for github.com/scylladb/gocql/lz4."
		exit 1
	}
	# Checked first: drop a replace and `go mod tidy` resolves the published release and
	# rewrites go.sum to match, so every check below stays green against code that is no
	# longer under test. tests/bench needs its own; its go.mod records why.
	for module in ".:./lz4" "tests/bench:../../lz4"; do
		DIR="$${module%%:*}"
		TARGET="$${module##*:}"
		REPLACED=$$(go -C "$${DIR}" list -m -f '{{with .Replace}}{{.Path}}{{end}}' github.com/scylladb/gocql/lz4 || true)
		if [[ "$${REPLACED}" != "$${TARGET}" ]]; then
			echo "::error::$${DIR}/go.mod no longer replaces github.com/scylladb/gocql/lz4 with $${TARGET}."
			echo "Without it the module resolves from the proxy and this repository tests the last release."
			exit 1
		fi
	done
	ALLTAGS=$$(git tag -l 'lz4/v*' 2>/dev/null || true)
	if [[ -z "$${ALLTAGS}" ]]; then
		if [[ -n "$${GITHUB_ACTIONS}" ]]; then
			echo "::error::no lz4/v* tags found at all, so this check verified nothing."
			echo "The workflow's checkout has stopped fetching tags."
			exit 1
		fi
		echo "No lz4/v* tags present (clone without tags); skipping"
		exit 0
	fi
	# An existence question, not an ordering one: an untagged pin can sit between two
	# tags, and so compare as older than the newest, while resolving for nobody.
	if ! git rev-parse -q --verify "refs/tags/lz4/$${PINNED}" >/dev/null; then
		echo "::error::go.mod requires lz4 $${PINNED} but no lz4/$${PINNED} tag exists."
		echo "Consumers cannot resolve the module. Tag it, or lower the require."
		exit 1
	fi
	# Pre-releases are dropped because `sort -V` orders v1.20.0-rc1 after v1.20.0.
	LATEST=$$(printf '%s\n' "$${ALLTAGS}" | sed 's|^lz4/||' | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$$' | sort -V | tail -1 || true)
	if [[ -z "$${LATEST}" ]]; then
		echo "Only pre-release lz4 tags present; the pin resolves, nothing to compare it against"
	elif [[ "$${PINNED}" != "$${LATEST}" ]]; then
		# A warning, not an error: a require is a lower bound so the pin still resolves,
		# and the tag and the bump commit cannot land atomically.
		echo "::warning::lz4 pin $${PINNED} lags tag lz4/$${LATEST}; bump go.mod and the versions README.md section 5.4 documents."
		echo "- lz4 pin $${PINNED} lags tag lz4/$${LATEST}" >> "$${GITHUB_STEP_SUMMARY:-/dev/null}"
	fi
	# Every lz4 version README.md documents must be the pinned one, compared exactly. A
	# substring test lets v1.19.0 be satisfied by v1.19.0-rc1, and asking only whether the
	# pin appears somewhere stays green while another mention is stale. `sort -u` collapses
	# to one line only when they all agree, so a single comparison covers both.
	DOCUMENTED=$$(grep -oE '(github\.com/scylladb/gocql/lz4[ @]|(^|[^A-Za-z0-9./])lz4/)v[0-9]+\.[0-9]+\.[0-9]+[A-Za-z0-9.-]*' README.md | grep -oE 'v[0-9]+\.[0-9]+\.[0-9]+[A-Za-z0-9.-]*' | sort -u || true)
	if [[ -z "$${DOCUMENTED}" ]]; then
		echo "README.md documents no lz4 version; section 5.4 should name $${PINNED}."
		exit 1
	fi
	if [[ "$${DOCUMENTED}" != "$${PINNED}" ]]; then
		echo "README.md documents lz4 $$(echo $${DOCUMENTED}); go.mod pins $${PINNED}."
		echo "Section 5.4 documents the lz4 version too; bump it alongside go.mod."
		exit 1
	fi

check: build .prepare-golangci check-go-mod-drift check-lz4-pin
	echo "Check linting"
	${BIN_DIR}/golangci-lint run

fix-go-mod-drift:
	@echo "Fix Go module drift"
	go mod tidy
	go mod tidy -C lz4
	go mod tidy -C tests/bench

fix: .prepare-golangci fix-go-mod-drift
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
	sdk install java 11.0.30-zulu;
	sdk install java 17.0.18-zulu;
	sdk default java 11.0.30-zulu;
	sdk use java 11.0.30-zulu;

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
