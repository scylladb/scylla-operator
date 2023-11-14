all: run

script=./run.sh
OUTPUT?=./run.cast

run-fast:
	$(strip CUSTOM_PS1_SLEEP_SEC=0 CUSTOM_CHAR_SLEEP_SEC=0 CUSTOM_ECHO_SLEEP_SEC=0 CUSTOM_ECHO_MIN_SLEEP_SEC=0 CUSTOM_COMMAND_SLEEP_SEC=0 SKIP_MANUAL_SLEEPS="true" \
    ./run.sh)
.PHONY: run-fast

run-fast-local: export SCYLLA_OPERATOR_REPO:=../../.
run-fast-local: run-fast
.PHONY: run-fast-local

run:
	./run.sh
.PHONY: run

run-local: export SCYLLA_OPERATOR_REPO:=../../.
run-local: run
.PHONY: run-local

record:
	asciinema rec --idle-time-limit=5 --rows=25 --cols=100  --overwrite --command "$(script)" "$(OUTPUT)"
.PHONY: record
