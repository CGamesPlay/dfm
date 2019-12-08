#!/bin/bash
# Tests for multiple repositories working correctly
set -e
. "$(dirname "$0")/../helpers.sh"

export HOME="$(pwd)/home"
export DFM_DIR="$HOME/dfmdir"

mkdir -p ~/dfmdir/one ~/dfmdir/two
echo 'one' > ~/dfmdir/one/.bashrc
echo 'two' > ~/dfmdir/two/.bashrc
echo 'one' > ~/dfmdir/one/.vimrc
echo 'two' > ~/dfmdir/two/.zshrc

dfm init --repos one,two
dfm link
[ "$(readlink ~/.bashrc)" == ~/dfmdir/two/.bashrc ] || fail 'wrong bashrc selected'
[ "$(readlink ~/.vimrc)" == ~/dfmdir/one/.vimrc ] || fail 'wrong vimrc selected'
[ "$(readlink ~/.zshrc)" == ~/dfmdir/two/.zshrc ] || fail 'wrong zshrc selected'
