package main

import (
	"fmt"
	"os"
	"os/exec"
	"path"
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

// MakeDirAll will make sure all directories in dest/relative exist.
func MakeDirAll(relative, source, dest string) error {
	// XXX - when creating directories, use source to find the permissions of
	// each new directory.
	return os.MkdirAll(path.Join(dest, relative), 0777)
}

// MoveFile will move the file from source to dest, failing if the file already
// exists.
func MoveFile(source, dest string) error {
	stat, _ := os.Stat(dest)
	if stat != nil {
		return fmt.Errorf("%s: already exists", dest)
	}
	// This implementation shells out to mv to avoid cross-device failures that
	// might happen with os.Rename.
	cmd := exec.Command("mv", "-n", source, dest)
	if err := cmd.Run(); err != nil {
		if exitErr := err.(*exec.ExitError); exitErr != nil && len(exitErr.Stderr) > 0 {
			return fmt.Errorf(string(exitErr.Stderr))
		}
		return fmt.Errorf("failed to move file")
	}
	return nil
}

// CopyFile will copy the file from source to dest.
func CopyFile(source, dest string) error {
	// This implementation shells out to cp to avoid dealing with permissions,
	// timestamps, extended attributes, etc.
	cmd := exec.Command("cp", "-pn", source, dest)
	if err := cmd.Run(); err != nil {
		if exitErr := err.(*exec.ExitError); exitErr != nil && len(exitErr.Stderr) > 0 {
			return fmt.Errorf(string(exitErr.Stderr))
		}
		return fmt.Errorf("failed to copy file")
	}
	return nil
}

// LinkFile creates a link at dest that points to source.
func LinkFile(source, dest string) error {
	if !path.IsAbs(source) {
		return fmt.Errorf("must use an absolute path for link source")
	}
	return os.Symlink(source, dest)
}
