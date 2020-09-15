# dfm - Dotfiles Manager

dfm is a generic file syncing utility to keep two directories in sync, using symbolic links. It's targeted at keeping a home directory in sync with a dotfiles repository, but can be used in other circumstances as well.

**Features:**

- No dependency on git. dfm works fine with git, Dropbox, or any other file syncing tool.
- No configuration files. Add files to be tracked just by placing them in the directory.
- No runtime dependencies. dfm can be run directly from the single binary file on a brand new machine.
- Overlay multiple repositories on top of each other.
- Automatically clean up removed files.

[![asciicast](https://asciinema.org/a/284642.svg)](https://asciinema.org/a/284642)

## Project status

This project is "done". It's maintained, but there are no planned new features. It's actively used to handle [the author's dotfiles](https://github.com/CGamesPlay/dotfiles) and possibly others with few issues reported. If you identify a bug or dfm is not compatible with some OS, please don't hesitate to file a pull request or open an issue.

## Installation

dfm is distributed as a single binary and can be installed anywhere. To install it to your `/usr/local/bin` directory, use a script like this one:

```bash
DFM_VERSION=$(uname -s | tr A-Z a-z)_amd64 # "darwin_amd64" or "linux_amd64"
curl -sSL https://github.com/cgamesplay/dfm/releases/latest/download/$DFM_VERSION.tar.gz -o dfm.tar.gz
tar -xf dfm.tar.gz
sudo mv dfm /usr/local/bin/
dfm --version
```

## Quick Start

To get started with dfm from a blank slate (to see how it works), try these commands:

```bash
$ mkdir -p ~/dotfiles/files
$ export DFM_DIR=~/dotfiles
$ dfm init --repos files
$ dfm add ~/.bashrc ~/.vimrc
$ ls -la ~/.bashrc ~/.vimrc ~/dotfiles/files
```

Notice that `~/.bashrc` and `~/.vimrc` have been replaced with symlinks, and the real files live in `~/dotfiles/files`. You can store `~/dotfiles/files` in a source control system, or even the entire `~/dotfiles` directory, if you have extra scripts that you would like to add. Note that `.dfm.toml` is machine-specific and should not be added to source control.

When you are setting up a new machine, assuming you have already created `~/dotfiles/files` on that machine (e.g. by cloning your git repository, syncing, whatever):

```bash
export DFM_DIR=~/dotfiles
dfm init --repos files
dfm link --dry-run
dfm link
```

`dfm link` will scan `~/dotfiles/files` and create symlinks in your home folder for each file found. If you have removed files from the repository, running `dfm link` again will automatically delete those broken symlinks.

## Usage Guide

dfm includes a help command that explains all available options. Run one of these to learn more:

```bash
dfm help
dfm help init
dfm help add
dfm help link
```

### Recommended workflow

This is the recommended workflow to effectively use dfm with your dotfiles. Look at [CGamesPlay/dotfiles](https://github.com/CGamesPlay/dotfiles) for a working example of this workflow.

1. Write a bootstrap script that installs dfm. I recommend downloading the binary from github via curl, but `go get` might be an option if you prefer. The single binary can be placed anywhere; I prefer to place it in `~/dotfiles/bin`.
2. Configure the dfm directory. It's best to make a shell function: `function dfm() { ~/dotfiles/bin/dfm --dfm-dir=~/dotfiles "$@" }`  If you don't mind global configuration, you can also place dfm anywhere in your `$PATH` and set a global environment variable `DFM_DIR=~/dotfiles`. 
3. Run `dfm init --repos=files` on each machine to configure it to use `~/dotfiles/files` as the main file repository.
4. Use `dfm add` to add all of your existing configuration to `~/dotfiles/files`, or copy them from your existing dotfiles repository. dfm does not rename files, so the file structure in `~/dotfiles/files` should look exactly like you want it to appear in `~/`.
5. Run `dfm link` to synchronize all of the symlinks in your home directory.

### Multiple repositories

dfm supports multiple repositories of files. When multiple repositories are configured, `dfm link` will link to the file in the last listed repository which has the file in question. For example:

```bash
$ export DFM_DIR=~/dotfiles
# Assume this directory structure:
# ~/dotfiles
#    /shared
#       /.bashrc
#       /.my.cnf
#    /work
#       /.my.cnf
$ dfm init --repos=shared,work
$ dfm link
shared/.bashrc -> ~/.bashrc
work/.my.cnf -> ~/.my.cnf
```

Notice that `.my.cnf` was listed in both `shared` and `work`. Because `work` was listed second in the `dfm init` call, it is the repository that was used for `.my.cnf`.

**Tip:** repos are just paths relative to the dfm directory. You could use `machines/web` as a repo, or even an absolute path like `~/other-dotfiles`.

### Ejecting

If you want to stop using dfm for some files, you can use `dfm eject` to copy it to your home directory and prevent dfm from automatically cleaning it up later. For example:

```bash
export DFM_DIR=~/dotfiles
dfm eject ~/.bashrc
rm ~/dotfiles/files/.bashrc
```

dfm will always use a hard copy when using `eject`, so it's safe to simply delete the files from the dfm repo afterwards. Keep in mind that if your dfm directory is shared, any other machines using it will simply see that the files were deleted, and will automatically clean them up when you next run `dfm link`.

If you want to stop using dfm entirely, `dfm eject` with no arguments will eject all tracked files. You can remove your dfm repos afterwards.

### Managing other directories

Each dfm directory manages a single target directory. For dotfiles, the target directory is your home folder, but dfm can manage any directory you choose, by configuring that directory in `dfm init`.

```bash
dfm -d ~/vhosts init --repos files --target /etc/nginx/vhost.d
dfm -d ~/vhosts link
```

## Development

dfm is built with go, so make sure you have a go compiler set up on your system. The project is a go module, so the other dependencies will be installed automatically when you build the software.

To make and install locally:

```
make install
```

To test:

```bash
make test
```

## Prior art

There are lots of other dotfile managers out there, which dfm draws inspiration from:

- [bonclay](https://github.com/talal/bonclay) is similar in scope to dfm (built for dotfiles but applicable to general file syncing). It supports renaming config files, but requires managing a configuration file to do so. The automatic cleanup is limited to scanning the directory for broken symlinks.
- [homemaker](https://github.com/FooSoft/homemaker) has many of the same goals as dfm, but requires a configuration file. The tasks functionality is potentially useful, and you could replace homemaker's `links` functionality with a task to run `dfm`.
- There's plenty of other options on [Github does dotfiles](https://dotfiles.github.io), but either require too many software dependencies, are too tied to specific workflows, or require too much configuration/learning to adopt easily.
