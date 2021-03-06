#!/bin/bash
# Runs a set of DFM commands in empty directories and make some simple
# assertions.
set -e
. "$(dirname "$0")/../helpers.sh"

export DFM_DIR=dfmdir

mkdir -p dfmdir/files test_home
echo 'config file' > dfmdir/files/.bashrc

dfm init --repos files --target test_home

banner "Sync dry run"
dfm link -n
[ ! -e test_home/.bashrc ] || fail "dry-run modified files"

banner "Initial sync"
dfm link
[ -L test_home/.bashrc ] || fail ".bashrc is not a symlink"

banner "Everything is up to date"
dfm link -v

banner "Adding a new config file"
mkdir -p dfmdir/files/.ssh
echo 'config file' > dfmdir/files/.ssh/config
dfm link
[ -L test_home/.bashrc ] || fail '.bashrc was removed'
[ -L test_home/.ssh/config ] || fail '.ssh/config was not created'

banner "Removing a config file"
rm -rf dfmdir/files/.ssh
dfm link
[ ! -e test_home/.ssh/config ] || fail '.ssh/config was not removed'

banner "Importing with add"
mkdir -p test_home/.config/fish
echo 'config file' > test_home/.config/fish/config.fish
dfm add test_home/.config
[ -L test_home/.config/fish/config.fish ] || fail 'config.fish was not replaced by a link'
[ -f dfmdir/files/.config/fish/config.fish ] || fail 'config.fish is not a regular file'

banner 'Exporting files with copy'
dfm copy --force
[ -e test_home/.bashrc ] || fail 'bashrc missing'
[ ! -L test_home/.bashrc ] || fail 'bashrc is a link'

banner 'Cleaning up'
dfm remove
[ ! -e test_home/.config ] || fail 'empty directory not cleaned'
