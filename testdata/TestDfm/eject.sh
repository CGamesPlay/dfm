#!/bin/bash
# Tests for multiple repositories working correctly
set -e
. "$(dirname "$0")/../helpers.sh"

export HOME="$(pwd)/home"
export DFM_DIR="$HOME/dfmdir"

mkdir -p ~/dfmdir/files
echo 'config' > ~/dfmdir/files/.bashrc
echo 'config' > ~/dfmdir/files/.zshrc

dfm init --repos files
dfm link

banner 'Ejecting one file'
dfm eject ~/.bashrc
rm ~/dfmdir/files/.bashrc
dfm link
[ ! -L ~/.bashrc ] || fail 'bashrc still linked'
[ -e ~/.bashrc ] || fail 'bashrc missing'

banner 'Ejecting everything'
dfm eject
rm ~/dfmdir/files/.zshrc
dfm link
[ ! -L ~/.zshrc ] || fail 'zshrc still linked'
[ -e ~/.zshrc ] || fail 'zshrc missing'
