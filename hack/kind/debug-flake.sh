#!/bin/bash
#
# Copyright (C) 2026 ScyllaDB
#
# This script runs a suspect test case in a loop to verify if it's flaky and collect artifacts for debugging.
# Usage: ${0} -N <number_of_runs> --focus <test_case_pattern> --out <output_dir> [--recreate]

set -euo pipefail
shopt -s inherit_errexit

readonly parent_dir="$( dirname "${BASH_SOURCE[0]}" )"

source "${parent_dir}/lib.sh"

# Default values.
NUM_RUNS=10
FOCUS_PATTERN=""
OUTPUT_DIR="debug-flake-results"
RECREATE_CLUSTER=false
VERBOSE=false
STOP_AFTER_N_FAILURES=0
CLUSTER_NAME="${CLUSTER_NAME:-scylla-operator-debug-flake}"

function print_usage {
  cat <<EOF
Usage: ${0} -N <number_of_runs> --focus <test_case_pattern> --out <output_dir> [--recreate] [--verbose] [--stop-after-n-failures <number>]

Options:
  -N <number>                      Number of times to run the test (default: 10)
  --focus <pattern>                Test case pattern to focus on (required)
  --out <directory>                Output directory for test results (default: debug-flake-results)
  --recreate                       Recreate the KIND cluster before each test run (default: false)
  --verbose                        Show verbose output from cluster setup and test execution (default: false)
  --stop-after-n-failures <number> Stop execution after N failures (default: 0, meaning run all tests)
  -h, --help                       Show this help message

Environment variables:
  CLUSTER_NAME             Name of the KIND cluster to use (default: scylla-operator-debug-flake)

Example:
  ${0} -N 10 --focus "TestCassandraClusterScaling" --out debug-flake-results
  ${0} -N 100 --focus "TestFlaky" --out results --stop-after-n-failures 5
EOF
}

# Parse command-line arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    -N)
      NUM_RUNS="$2"
      shift 2
      ;;
    --focus)
      FOCUS_PATTERN="$2"
      shift 2
      ;;
    --out)
      OUTPUT_DIR="$2"
      shift 2
      ;;
    --recreate)
      RECREATE_CLUSTER=true
      shift
      ;;
    --verbose)
      VERBOSE=true
      shift
      ;;
    --stop-after-n-failures)
      STOP_AFTER_N_FAILURES="$2"
      shift 2
      ;;
    -h|--help)
      print_usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" > /dev/stderr
      print_usage > /dev/stderr
      exit 1
      ;;
  esac
done

# Validate required arguments.
if [ -z "${FOCUS_PATTERN}" ]; then
  echo "Error: --focus option is required" > /dev/stderr
  print_usage > /dev/stderr
  exit 1
fi

if ! [[ "${NUM_RUNS}" =~ ^[0-9]+$ ]] || [ "${NUM_RUNS}" -lt 1 ]; then
  echo "Error: -N must be a positive integer" > /dev/stderr
  exit 1
fi

if ! [[ "${STOP_AFTER_N_FAILURES}" =~ ^[0-9]+$ ]]; then
  echo "Error: --stop-after-n-failures must be a non-negative integer" > /dev/stderr
  exit 1
fi

# Create output directory.
mkdir -p "${OUTPUT_DIR}"
OUTPUT_DIR="$(realpath "${OUTPUT_DIR}")"

echo "=========================================="
echo "Debug Flake Test Runner"
echo "=========================================="
echo "Test pattern: ${FOCUS_PATTERN}"
echo "Number of runs: ${NUM_RUNS}"
echo "Output directory: ${OUTPUT_DIR}"
echo "Cluster name: ${CLUSTER_NAME}"
echo "Recreate cluster per run: ${RECREATE_CLUSTER}"
echo "Verbose mode: ${VERBOSE}"
if [ "${STOP_AFTER_N_FAILURES}" -gt 0 ]; then
  echo "Stop after N failures: ${STOP_AFTER_N_FAILURES}"
fi
echo "=========================================="
echo ""

export CLUSTER_NAME

# Set up the KIND cluster once before the loop (unless recreate is requested).
if [ "${RECREATE_CLUSTER}" = false ]; then
  echo "Setting up KIND cluster..."
  RECREATE=false "${parent_dir}/../cluster-setup.sh"
  echo ""
fi

# Build and push the operator image once (unless SO_IMAGE is already set).
echo "Preparing operator image..."
build-and-push-operator-image "${parent_dir}/../../.."
echo ""

# Initialize counters.
TOTAL_RUNS=0
FAILED_RUNS=0
PASSED_RUNS=0

# Run the test in a loop.
for ((i=1; i<=NUM_RUNS; i++)); do
  TOTAL_RUNS=$((TOTAL_RUNS + 1))
  RUN_DIR="${OUTPUT_DIR}/run-${i}"
  mkdir -p "${RUN_DIR}"

  echo "=========================================="
  echo "Run ${i}/${NUM_RUNS}"
  echo "=========================================="
  echo "Artifacts directory: ${RUN_DIR}"
  echo ""

  # Recreate cluster if requested.
  if [ "${RECREATE_CLUSTER}" = true ]; then
    echo "Recreating KIND cluster..."
    if [ "${VERBOSE}" = true ]; then
      RECREATE=true "${parent_dir}/../cluster-setup.sh"
    else
      RECREATE=true "${parent_dir}/../cluster-setup.sh" > /dev/null 2>&1
    fi
    echo ""
  fi

  # Run the test with the specified focus pattern
  export ARTIFACTS="${RUN_DIR}"
  export SO_FOCUS="${FOCUS_PATTERN}"
  # We want to wait for E2E test pods to be deleted before starting the next run, to avoid interference between runs and to ensure clean state.
  export SO_WAIT_FOR_E2E_POD_DELETE=true

  # Run the test and capture the exit code
  if [ "${VERBOSE}" = true ]; then
    if "${parent_dir}/../run-e2e-tests.sh"; then
      echo "✓ Run ${i}/${NUM_RUNS} PASSED"
      PASSED_RUNS=$((PASSED_RUNS + 1))
    else
      echo "✗ Run ${i}/${NUM_RUNS} FAILED"
      FAILED_RUNS=$((FAILED_RUNS + 1))
    fi
  else
    if "${parent_dir}/../run-e2e-tests.sh" > /dev/null 2>&1; then
      echo "✓ Run ${i}/${NUM_RUNS} PASSED"
      PASSED_RUNS=$((PASSED_RUNS + 1))
    else
      echo "✗ Run ${i}/${NUM_RUNS} FAILED"
      FAILED_RUNS=$((FAILED_RUNS + 1))
    fi
  fi

  # Check if we should stop early after reaching the failure threshold.
  if [ "${STOP_AFTER_N_FAILURES}" -gt 0 ] && [ "${FAILED_RUNS}" -ge "${STOP_AFTER_N_FAILURES}" ]; then
    echo ""
    echo "Stopping early: reached ${FAILED_RUNS} failure(s) (threshold: ${STOP_AFTER_N_FAILURES})"
    break
  fi

  echo ""
done

# Print summary
echo "=========================================="
echo "Summary"
echo "=========================================="
echo "Total runs: ${TOTAL_RUNS}"
echo "Passed: ${PASSED_RUNS}"
echo "Failed: ${FAILED_RUNS}"

if [ "${STOP_AFTER_N_FAILURES}" -gt 0 ] && [ "${FAILED_RUNS}" -ge "${STOP_AFTER_N_FAILURES}" ]; then
  echo "Stopped early after ${FAILED_RUNS} failure(s)"
fi

if [ "${TOTAL_RUNS}" -gt 0 ]; then
  SUCCESS_RATE=$(awk "BEGIN {printf \"%.2f\", (${PASSED_RUNS}/${TOTAL_RUNS})*100}")
  echo "Success rate: ${SUCCESS_RATE}%"
fi

echo ""
echo "Test results stored in: ${OUTPUT_DIR}"
echo ""

if [ "${FAILED_RUNS}" -gt 0 ]; then
  echo "⚠️  Test appears to be flaky (${FAILED_RUNS}/${TOTAL_RUNS} failures)"
  echo ""
  echo "Here's a prompt you can use to analyze the results with our OpenCode `flake-tests-debugger` agent:"
  echo ""
  echo "Analyze the flaky test results in '${OUTPUT_DIR}' directory. The test pattern is '${FOCUS_PATTERN}".
  exit 1
else
  echo "✓ All runs passed - test appears to be stable"
  exit 0
fi
