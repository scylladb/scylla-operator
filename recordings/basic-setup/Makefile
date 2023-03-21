all: run

script=./run.sh
OUTPUT?=./run.cast

run-fast:
	$(strip CUSTOM_PS1_SLEEP_SEC=0 CUSTOM_CHAR_SLEEP_SEC=0 CUSTOM_ECHO_SLEEP_SEC=0 CUSTOM_ECHO_MIN_SLEEP_SEC=0 CUSTOM_COMMAND_SLEEP_SEC=0 \
    ./run.sh)

run:
	./run.sh

record:
	asciinema rec --idle-time-limit=5 --overwrite --command "$(script)" "$(OUTPUT)"
