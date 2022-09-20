export OS    := $(shell uname)
export GOBIN := $(PWD)/bin
export PATH  := $(GOBIN):$(PATH)

.PHONY: install-dependencies
install-dependencies:
	@rm -Rf bin
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

.PHONY: check
check:
	@$(GOBIN)/golangci-lint run ./...

.PHONY: build
build:
	go build ./...

.PHONY: test
test:
	go test -run ^Test ./...

.PHONY: test-no-cache
test-no-cache:
	go test -count=1 ./...

COMPOSE := docker-compose

.PHONY: integration-test
integration-test:
	@$(MAKE) pkg-integration-test PKG=./transport
	@$(MAKE) pkg-integration-test PKG=./

.PHONY: integration-bench
integration-bench: RUN=Integration
integration-bench:
	@$(MAKE) pkg-integration-test RUN=XXX PKG=./ ARGS='-test.bench=$(RUN) -test.benchmem -test.benchtime=5s $(ARGS)'

# Prevent invoking make with a package specific test without a constraining a package.
ifneq "$(filter pkg-%,$(MAKECMDGOALS))" ""
ifeq "$(PKG)" ""
$(error Please specify package name with PKG e.g. PKG=./transport)
endif
endif

.PHONY: pkg-integration-test
pkg-integration-test: RUN=Integration
pkg-integration-test:
ifeq ($(OS),Linux)
	@go test -v -tags integration -run $(RUN) $(PKG) $(ARGS)
else ifeq ($(OS),Darwin)
	@CGO_ENABLED=0 GOOS=linux go test -v -tags integration -c -o ./integration-test.dev $(PKG)
	@docker run --name "integration-test" \
		--network scylla_go_driver_public \
		-v "$(PWD)/testdata:/testdata" \
		-v "$(PWD)/integration-test.dev:/usr/bin/integration-test:ro" \
		-it --read-only --rm ubuntu integration-test -test.v -test.run $(RUN) $(ARGS)
else
	$(error Unsupported OS $(OS))
endif

.PHONY: docker-integration-test
docker-integration-test: RUN=Integration
docker-integration-test:
	@CGO_ENABLED=0 GOOS=linux go test -v -tags integration -c -o ./integration-test.dev $(PKG)
	@docker run --name "integration-test" \
		--network scylla_go_driver_public \
		-v "$(PWD)/testdata:/testdata" \
		-v "$(PWD)/integration-test.dev:/usr/bin/integration-test:ro" \
		-i --read-only --rm ubuntu integration-test -test.v -test.run $(RUN) $(ARGS)

.PHONY: run-benchtab
run-benchtab: CPUSET=4-5
run-benchtab:
ifeq ($(OS),Linux)
	@taskset -c $(CPUSET) go run ./experiments/cmd/benchtab -nodes "192.168.100.100:9042"
else ifeq ($(OS),Darwin)
	@CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-extldflags '-static'" -o ./benchtab.dev ./experiments/cmd/benchtab
	@docker run --name "benchtab" \
		--network scylla_go_driver_public \
		-v "$(PWD)/benchtab.dev:/usr/bin/benchtab:ro" \
		-v "$(PWD)/pprof:/pprof" \
		-it --read-only --rm --cpuset-cpus $(CPUSET) ubuntu benchtab -nodes "192.168.100.100:9042"
else
	$(error Unsupported OS $(OS))
endif

.PHONY: benchtab
benchtab:
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-extldflags '-static'" -o ./benchtab ./experiments/cmd/benchtab

.PHONY: scylla-up
scylla-up:
	@$(COMPOSE) up -d

.PHONY: scylla-down
scylla-down:
	@$(COMPOSE) down --volumes --remove-orphans

.PHONY: scylla-logs
scylla-logs:
	@$(COMPOSE) exec node tail -f /var/log/syslog

.PHONY: scylla-bash
scylla-bash:
	@$(COMPOSE) exec node bash

.PHONY: scylla-cqlsh
scylla-cqlsh:
	@$(COMPOSE) exec node cqlsh -u cassandra -p cassandra

