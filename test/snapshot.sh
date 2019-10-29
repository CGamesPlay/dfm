#!/bin/bash
# Runs the test script and compare the output against the saved snapshot. The
# test fails if the output changes.
set -e

cd "$(dirname "$0")"
TEST_DIR="$(pwd)"
./test.sh >compare.txt || { cat compare.txt; exit 1; }
sed -i '' s/${TEST_DIR//\//\\\/}/./g compare.txt
diff -u snapshot.txt compare.txt >&2
rm compare.txt
