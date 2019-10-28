#!/bin/bash
set -e

banner() {
  echo
  echo "$1"
}

fail() {
  echo "$1" >&2
  exit 1
}

cd "$(dirname "$0")"
dfm() {
  ../dfm -d dfmdir "$@"
}

rm -rf dfmdir test_home
mkdir -p dfmdir/files test_home
echo 'config file' > dfmdir/files/.bashrc

dfm -R files --target test_home init

banner "Testing initial sync"
dfm link
[ -L test_home/.bashrc ] || fail ".bashrc is not a symlink"

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
dfm add test_home/.config/fish/config.fish
[ -L test_home/.config/fish/config.fish ] || fail 'config.fish was not replaced by a link'
[ -f dfmdir/files/.config/fish/config.fish ] || fail 'config.fish is not a regular file'

banner 'Cleaning up'
dfm remove
