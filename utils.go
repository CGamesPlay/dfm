package main

import (
	"fmt"
	"os"
	"os/exec"
	"path"

	"github.com/spf13/afero"
)

func pathJoin(components ...string) string {
	if len(components) == 0 {
		return ""
	}
	result := components[len(components)-1]
	for i := len(components) - 2; i >= 0; i-- {
		if path.IsAbs(result) {
			return result
		}
		result = path.Join(components[i], result)
	}
	return result
}

// IsRegularFile will return true if the given file is a regular file (symlinks
// not allowed)
func IsRegularFile(fs afero.Fs, path string) (bool, error) {
	var (
		stat os.FileInfo
		err  error
	)
	if lstater, ok := fs.(afero.Lstater); ok {
		stat, _, err = lstater.LstatIfPossible(path)
		if err != nil {
			return false, err
		}
	} else {
		stat, err = fs.Stat(path)
		if err != nil {
			return false, err
		}
	}
	if !stat.Mode().IsRegular() {
		return false, nil
	}
	return true, nil
}

// MakeDirAll will make sure all directories in dest/relative exist.
func MakeDirAll(fs afero.Fs, relative, source, dest string) error {
	// XXX - when creating directories, use source to find the permissions of
	// each new directory.
	return fs.MkdirAll(path.Join(dest, relative), 0777)
}

// MoveFile will move the file from source to dest, failing if the file already
// exists.
func MoveFile(fs afero.Fs, source, dest string) error {
	stat, _ := fs.Stat(dest)
	if stat != nil {
		return fmt.Errorf("%s: already exists", dest)
	}

	switch fs.(type) {
	case *afero.OsFs:
		// This implementation shells out to mv to avoid cross-device failures
		// that might happen with os.Rename.
		cmd := exec.Command("mv", "-n", source, dest)
		if err := cmd.Run(); err != nil {
			if exitErr := err.(*exec.ExitError); exitErr != nil && len(exitErr.Stderr) > 0 {
				return fmt.Errorf(string(exitErr.Stderr))
			}
			return fmt.Errorf("failed to move file")
		}
		return nil
	case *afero.MemMapFs:
		return fs.Rename(source, dest)
	default:
		return &os.LinkError{
			Op:  "move",
			Old: source,
			New: dest,
			Err: fmt.Errorf("unsupported afero fs"),
		}
	}
}

// CopyFile will copy the file from source to dest.
func CopyFile(fs afero.Fs, source, dest string) error {
	switch fs.(type) {
	case *afero.OsFs:
		// This implementation shells out to cp to avoid dealing with
		// permissions, timestamps, extended attributes, etc.
		cmd := exec.Command("cp", "-pn", source, dest)
		if err := cmd.Run(); err != nil {
			if exitErr := err.(*exec.ExitError); exitErr != nil && len(exitErr.Stderr) > 0 {
				return fmt.Errorf(string(exitErr.Stderr))
			}
			return fmt.Errorf("failed to copy file")
		}
		return nil
	case *afero.MemMapFs:
		stat, _ := fs.Stat(dest)
		if stat != nil {
			return fmt.Errorf("%s: already exists", dest)
		}
		data, err := afero.ReadFile(fs, source)
		if err != nil {
			return err
		}
		err = afero.WriteFile(fs, dest, data, 0777)
		return err
	default:
		return &os.LinkError{
			Op:  "copy",
			Old: source,
			New: dest,
			Err: fmt.Errorf("unsupported afero fs"),
		}
	}
}

// IsLinkedFile decides if dest is already a link to source
func IsLinkedFile(fs afero.Fs, source, dest string) (bool, error) {
	switch fs.(type) {
	case *afero.OsFs:
		stat, err := os.Lstat(dest)
		if os.IsNotExist(err) {
			return false, nil
		} else if err != nil {
			return false, err
		} else if stat.Mode()&os.ModeSymlink == 0 {
			return false, nil
		}
		target, err := os.Readlink(dest)
		if err != nil || target != source {
			return false, err
		}
		return true, nil
	default:
		return false, fmt.Errorf("unsupported afero fs")
	}
}

// LinkFile creates a link at dest that points to source.
func LinkFile(fs afero.Fs, source, dest string) error {
	if !path.IsAbs(source) {
		return fmt.Errorf("must use an absolute path for link source")
	}
	switch fs.(type) {
	case *afero.OsFs:
		return os.Symlink(source, dest)
	case *afero.MemMapFs:
		stat, _ := fs.Stat(dest)
		if stat != nil {
			return fmt.Errorf("%s: already exists", dest)
		}
		content := "symlink to " + source
		return afero.WriteFile(fs, dest, []byte(content), 0666)
	default:
		return &os.LinkError{
			Op:  "link",
			Old: source,
			New: dest,
			Err: fmt.Errorf("unsupported afero fs"),
		}
	}
}

// RemoveFile removes the listed file.
func RemoveFile(fs afero.Fs, path string) error {
	return fs.Remove(path)
}
