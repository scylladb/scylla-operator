#!/bin/bash
#
# Copyright (c) 2021 ScyllaDB
#

set -euExo pipefail
shopt -s inherit_errexit

message_prefix='Testing tune2fs'

echo "${message_prefix}..." >&2

dir=$( dirname "${0}" )
bin=$( readlink -f "${dir}/../../../gke/xfs-formatter/bin/tune2fs" )
if [[ ! -f "${bin}" ]]; then
  echo "tune2fs binary does not exist at ${bin}" >&2
  exit 1
fi

function err_handler {
  echo "${message_prefix}...FAILED! Error on line ${1}. Use \`bash -x ${0}\` to debug it further." >&2
  exit 1
}
trap 'err_handler "${LINENO}"' ERR

tmp_file=$( mktemp )

function setup_fs {
  dd if=/dev/zero of="${tmp_file}" bs=1M count=256
  # Enable small filesystems with these 3 extra vars - https://lore.kernel.org/all/Yv4i0gWiHTkfWB5m@yuki/T/
  TEST_DIR=1 TEST_DEV=1 QA_CHECK_FS=1 mkfs -t "${1}" "${tmp_file}"
}

echo "${message_prefix}..testing invalid parameters." >&2
setup_fs 'xfs'
"${bin}" && false
"${bin}" -l && false
"${bin}" "${tmp_file}" && false
"${bin}" -l -l && false
"${bin}" "${tmp_file}" "${tmp_file}" && false
"${bin}" -l "${tmp_file}" "${tmp_file}" && false

echo "${message_prefix}..testing invalid tune2fs filesystems." >&2
for t in "minix" "msdos"; do
  setup_fs "${t}"
  "${bin}" -l "${tmp_file}" && false
done

echo "${message_prefix}..testing valid tune2fs filesystems." >&2
for t in "ext2" "ext3" "ext4" "xfs"; do
  setup_fs "${t}"
  "${bin}" -l "${tmp_file}" 
done

echo "${message_prefix}..SUCCESS." >&2
