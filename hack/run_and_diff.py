#!/usr/bin/env python3
import sys
import logging
import argparse
from subprocess import run, PIPE

log = logging.getLogger(__name__)

DESCRIPTION = """Run a command and check if it caused changes.

This is useful for running formatting commands in CI and checking if they
modified git status.
"""


def parse_args():
    parser = argparse.ArgumentParser(description=DESCRIPTION)
    parser.add_argument("command", metavar="CMD", type=str, nargs="+",
                        help="a command to run and report changes.")
    parser.add_argument("-v", "--verbose", action="store_const",
                        default=False, const=True, help="Verbosity")
    return parser.parse_args()


def main():
    global log
    logging.basicConfig(level=logging.INFO)

    args = parse_args()
    cmd = args.command

    log.info("Running command: %s", cmd)
    run(cmd)

    log.info("Checking for changes...")
    res = run(["git", "status", "-s"], stdout=PIPE)
    if res.stdout:
        log.error("Running command %s introduced changes: %s", cmd, res.stdout)
        if args.verbose:
            log.info("Analytical diff following...")
            run(["git", "status", "-vvv"])


if __name__ == "__main__":
    main(sys.exit(main()))
