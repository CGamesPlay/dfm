#!/bin/bash
# Runs a set of DFM commands in empty directories and make some simple
# assertions.
set -e

banner() {
  echo
  echo "# $1"
}

fail() {
  echo "$1" >&2
  exit 1
}

cd "$(dirname "$0")"
export DFM_DIR=dfmdir
dfm() {
  echo "\$ dfm" "$@"
  ../bin/dfm "$@"
}

rm -rf dfmdir test_home
mkdir -p dfmdir/files test_home
echo 'config file' > dfmdir/files/.bashrc

dfm --repos files --target test_home init

banner "Sync dry run"
dfm link -n
[ ! -e test_home/.bashrc ] || fail "dry-run modified files"

banner "Initial sync"
dfm link
[ -L test_home/.bashrc ] || fail ".bashrc is not a symlink"

banner "Everything is up to date"
dfm link -v

banner "Adding a new config file"
echo 'config file' > dfmdir/files/.zshrc
dfm link
[ -L test_home/.bashrc ] || fail '.bashrc was removed'
[ -L test_home/.zshrc ] || fail '.zshrc was not created'

banner "Removing a config file"
rm dfmdir/files/.zshrc
dfm link
[ ! -e test_home/.zshrc ] || fail '.zshrc was not removed'

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

rm -rf dfmdir test_home
