#!/bin/bash

set -euExo pipefail
shopt -s inherit_errexit

raid_device="$( { grep -E "/mnt/stateful_partition/kube-ephemeral-ssd($| )" /proc/mounts || test $? = 1; } | sed -E -e 's/([^ ]+) .+/\1/' )"

if [[ -z "${raid_device}" ]]; then
  if [[ -f "${RAID_DEVICE_FILE}" ]]; then
    raid_device="$( head -n 1 "${RAID_DEVICE_FILE}" )"
  else
    echo "Raid device file does not exist"
  fi

  if [[ -z "${raid_device}" ]]; then
    echo "Raid device not mounted"
    exit 255
  fi
else
  echo "${raid_device}" > "${RAID_DEVICE_FILE}"
fi

raid_device_type="$( blkid -o value -s TYPE "${raid_device}" )"
if [[ "${raid_device_type}" == "xfs" ]]; then
  echo "Raid device's file system is already xfs."
  exit 1
fi
