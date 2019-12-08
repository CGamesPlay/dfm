#!/bin/bash
# Helper library for other bash-based tests.
set -e

banner() {
  echo
  echo "# $1"
}

fail() {
  echo "$1" >&2
  exit 1
}

dfm() {
  echo "\$ dfm" "$@"
  command dfm "$@"
}
