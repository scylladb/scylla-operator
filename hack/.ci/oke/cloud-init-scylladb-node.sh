#!/bin/bash

set -euo pipefail
curl --fail -H "Authorization: Bearer Oracle" -L0 \
  http://169.254.169.254/opc/v2/instance/metadata/oke_init_script \
  | base64 --decode > /var/run/oke-init.sh
bash /var/run/oke-init.sh --kubelet-extra-args "--cpu-manager-policy=static"
