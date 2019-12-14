#!/bin/bash
# Tests that specific-path commands are working as intended.
set -e
. "$(dirname "$0")/../helpers.sh"

export HOME="$(pwd)/home"
export DFM_DIR="$HOME/dfmdir"

mkdir -p ~/dfmdir/files ~/.config/bash
echo 'config file' > ~/.bashrc
echo 'config file' > ~/.config/bash/00-test.sh
echo 'autoclean' > ~/dfmdir/files/AUTOCLEAN

dfm add ~/.bashrc && fail 'dfm add without init allowed'

dfm init --repos files
dfm link
rm ~/dfmdir/files/AUTOCLEAN
[ -L ~/AUTOCLEAN ] || fail "AUTOCLEAN is missing"

banner 'Importing bash config'
cd ~/.config/bash
dfm add ~/.bashrc .
[ -L ~/.bashrc ] || fail "bashrc is missing"
[ -L 00-test.sh ] || fail "00-test is missing"

banner 'Importing a new config file'
echo 'new config' > 10-test.sh
mv 10-test.sh "$DFM_DIR/files/.config/bash/"
dfm link 10-test.sh
[ -e 10-test.sh ] || fail "10-test is missing"
[ -L ~/AUTOCLEAN ] || fail "AUTOCLEAN is missing"

banner 'Reversing the import'
dfm remove 10-test.sh
mv "$DFM_DIR/files/.config/bash/10-test.sh" .
[ -e 10-test.sh ] || fail "10-test is missing"
[ -L ~/AUTOCLEAN ] || fail "AUTOCLEAN is missing"

banner 'Running autoclean'
echo 'new remote config' > "$DFM_DIR/files/.config/bash/20-test.sh"
dfm link
[ -e 10-test.sh ] || fail "10-test is missing"
[ -e 20-test.sh ] || fail "20-test is missing"
[ ! -L ~/AUTOCLEAN ] || fail "AUTOCLEAN is present"
