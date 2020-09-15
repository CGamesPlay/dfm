#!/bin/bash
# Tests dfm correctly handling a file that has been unlinked behind dfm's back.
set -e
. "$(dirname "$0")/../helpers.sh"

export HOME="$(pwd)/home"
export DFM_DIR="$HOME/dfmdir"

mkdir -p ~/dfmdir/files
echo 'config' > ~/dfmdir/files/.bashrc

dfm init --repos files
dfm link

rm ~/.bashrc
rm ~/dfmdir/files/.bashrc
dfm link
dfm link # no output on second run
