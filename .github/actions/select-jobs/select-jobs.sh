#!/usr/bin/env bash
#
# Copyright (C) 2026 ScyllaDB
#
# This script prints, as a JSON list, the ids out of the given ones that match a JSON list of glob patterns. As in
# dorny/paths-filter with its default 'some' quantifier, an id is selected when it matches any pattern, and a pattern
# '!x' matches every id 'x' does not, so a list can't both include and exclude.

set -euExo pipefail
shopt -s inherit_errexit

if [[ "$#" -lt 1 ]]; then
  echo -e "Invalid arguments.\nUsage: ${0} <patterns_json> [<id>...]" >&2
  exit 1
fi

# The process substitution below doesn't propagate a jq failure, so the input is validated on its own first.
if ! jq -e 'type == "array" and all(.[]; type == "string")' <<< "${1}" > /dev/null; then
  echo "Patterns ${1} are not a JSON list of strings." >&2
  exit 1
fi
mapfile -t patterns < <( jq -r '.[]' <<< "${1}" )
ids=( "${@:2}" )

# A pattern that matches no id would otherwise go unnoticed, so a renamed job would silently drop out of, or back into,
# its callers.
for pattern in "${patterns[@]}"; do
  for id in "${ids[@]}"; do
    if [[ "${id}" == ${pattern#!} ]]; then
      continue 2
    fi
  done
  echo "Pattern ${pattern} matches no id." >&2
  exit 1
done

selected=()
for id in "${ids[@]}"; do
  for pattern in "${patterns[@]}"; do
    if [[ "${pattern}" == '!'* && "${id}" != ${pattern#!} ]] || [[ "${pattern}" != '!'* && "${id}" == ${pattern} ]]; then
      selected+=( "${id}" )
      continue 2
    fi
  done
done

# Selecting nothing would skip every job and pass whatever gates on them.
if [[ "${#selected[@]}" -eq 0 ]]; then
  echo "Patterns ${1} select no id." >&2
  exit 1
fi

jq -nc '$ARGS.positional' --args "${selected[@]}"
