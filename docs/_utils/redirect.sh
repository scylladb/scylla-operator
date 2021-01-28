#!/usr/bin/env bash
#
# Copyright (C) 2017 ScyllaDB
#

set -eu -o pipefail

cat <<- _EOF_
<!DOCTYPE html>
<html>
  <head>
    <title>Redirecting to Driver</title>
    <meta charset="utf-8">
    <meta http-equiv="refresh" content="0; URL=./${LATEST_VERSION}/index.html">
    <link rel="canonical" href="./${LATEST_VERSION}/index.html">
  </head>
</html>
_EOF_
