#!/bin/bash

set -euExo pipefail
shopt -s inherit_errexit

ephemeral_mountpoint='/mnt/stateful_partition/kube-ephemeral-ssd'

raid_device="$( head -n 1 "${RAID_DEVICE_FILE}" )"

systemctl stop kube-node-configuration.service
systemctl stop kubelet.service
crictl_pods="$( /home/kubernetes/bin/crictl pods -q )"
if [[ -n "${crictl_pods}" ]]; then
  /home/kubernetes/bin/crictl stopp ${crictl_pods}
fi
systemctl stop containerd.service

tmp_dir="$( mktemp -d )"
cp -rf "${ephemeral_mountpoint}"/. "${tmp_dir}"/

function safe_umount_multiple_mount_points {
  for line in ${1}; do
    if mountpoint -q "${line}"; then
      umount "${line}"
    fi
  done
}

mount_points="$( { grep -e '/home/kubernetes/containerized_mounter' -e '/var/lib/containerd' -e '/home/containerd' /proc/mounts || test $? = 1; } | sed -E -e 's/([^ ]+) ([^ ]+) .+/\2/' | sort -u -r )"
safe_umount_multiple_mount_points "${mount_points}"

# Separate line due to existing dependencies
mount_points="$( { grep -e "${raid_device}" /proc/mounts || test $? = 1; } | sed -E -e 's/([^ ]+) ([^ ]+) .+/\2/' | sort -r )"
safe_umount_multiple_mount_points "${mount_points}"

mkfs.xfs -f "${raid_device}"

cp -rf "${tmp_dir}"/. "${ephemeral_mountpoint}"/
rm -rf "${tmp_dir}"
