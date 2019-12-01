# dfm - Dotfiles Manager

dfm is a generic file syncing utility to keep two directories in sync, using symbolic links. It's targeted at keeping a home directory in sync with a dotfiles repository, but can be used in other circumstances as well.

**Features:**

- No dependency on git. dfm works fine with git or any other RCS.
- No configuration files. Add files to be tracked just by placing them in the directory.
- No runtime dependencies. dfm can be run directly from the single binary file on a brand new machine.
- Overlay multiple repositories on top of each other.
- Automatically clean up removed files.

[![asciicast](https://asciinema.org/a/284642.svg)](https://asciinema.org/a/284642)

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

dfm has two modes for tracking files: link and copy. The method used in this documentation is link, where files are tracked by using symlinks into the repos. With copy, files are copied directly to the target location from the repos. Copying files is useful if you want to dtop using dfm to migrate to another dotfiles solution.

```bash
export DFM_DIR=~/dotfiles
dfm copy --force
rm -rf ~/dotfiles
```

Once you run `dfm copy --force`, dfm replaces all of the symlinks with hard copies, so it's safe to simply delete the dfm directory afterwards.

**Note:** `dfm copy` and `dfm link` both have automatic cleanup behavior. If you use `dfm copy`, remove a file from a repo, and run `dfm copy` again, the automatic cleanup will remove the file from your home directory. It's recommended to only use `dfm copy` when you are uninstalling dfm.

### Managing other directories

Each dfm directory manages a single target directory. For dotfiles, the target directory is your home folder, but dfm can manage any directory you choose, by configuring that directory in `dfm init`.

```bash
dfm -d ~/vhosts init --repos files --target /etc/nginx/vhost.d
dfm -d ~/vhosts link
```

## Development

dfm is built with go, so make sure you have a go compiler set up on your system. The project is a go module, so the other dependencies will be installed automatically when you build the software.

To make and run locally:

```
make
./bin/dfm
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
