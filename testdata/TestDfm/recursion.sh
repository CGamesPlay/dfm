#!/bin/bash
# Tests that specific-path commands are working as intended.
set -e
. "$(dirname "$0")/../helpers.sh"

export HOME="$(pwd)/home"
export DFM_DIR="$HOME/dfmdir"

mkdir -p ~/dfmdir/files

dfm init --repos files
cd ~/dfmdir/files
echo 'vim config' > .vimrc
dfm add .vimrc && fail 'file recursively added' || true
dfm link .vimrc
