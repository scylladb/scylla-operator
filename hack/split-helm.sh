#!/usr/bin/env bash

# Splits helm template output into multiple manifest files.

if [ -z "$1" ]; then
  echo "Please provide an output directory"
  exit 1
fi

awk -vout=$1 -F": " '
  function basename(file) {
    sub(".*/", "", file)
    return file
  }

  function prefix(file) {
    if (match(file, ".*/charts/") != 0) {
      sub(".*/charts/", "", file)
      sub("/.*", "", file)
      return "10_" file "_"
    }

    return "10_"
  }

  # Parse source file name
  $0~/^# Source: (.*)$/ {
    file=out"/"prefix($2)basename($2);
    system ("mkdir -p $(dirname "file"); echo -n "" > "file);
  }

  # Copy content
  $0!~/(^#|^---$)/ {
    if (file) {

      print $0 >> file;
    }
  }
'
