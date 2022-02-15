#!/usr/bin/env bash
#
# Copyright (C) 2021 ScyllaDB
#

set -euExo pipefail
shopt -s inherit_errexit

POETRY_VERSION=${POETRY_VERSION:-1.1.12}
POETRY_CHECKSUM=${POETRY_CHECKSUM:-62eebaf303ce9fcb32b8c616b15e4c633921233f72a647f2c82c2d767f9ee36daea476be8712237206a8eaeca456d405118e1280abd394a0b26ccdb6d875bdc4}

poetry_installer="$( mktemp )"
curl --fail -sSL "https://raw.githubusercontent.com/python-poetry/poetry/${POETRY_VERSION}/get-poetry.py" | \
  tee "${poetry_installer}" | \
  sha512sum -c <( echo "${POETRY_CHECKSUM}" - )
python3 "${poetry_installer}" --version "${POETRY_VERSION}" -y
