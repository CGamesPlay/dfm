package main

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Dfm messages:
// - added file (relative, repo)
// - linked/copied file (relative, repo)
// - removed file (relative)

const (
	// OperationAdd means a file was added to a repo.
	OperationAdd = "added"
	// OperationLink means a file was linked from a repo to the target.
	OperationLink = "linked"
	// OperationCopy means a file was copied from a repo to the target.
	OperationCopy = "copied"
	// OperationRemove means a file was removed from the target.
	OperationRemove = "removed"
	// OperationSkip means a file was not copied/linked to the target. The
	// reason will be the original error, even though the ErrorHandler
	// suppressed the error. If the error is nil, it's because the file is
	// already synced.
	OperationSkip = "skipped"
)

// Logger is the type of function that dfm calls whenever it performs a file
// operation.
type Logger func(operation string, relative string, repo string, reason error)

// Dfm is the main controller class for API access to dfm
type Dfm struct {
	// The configuration used by this dfm instance
	Config DfmConfig
	// The log function used by this dfm instance
	Logger Logger
}

// NewDfm creates a new, empty dfm instance.
// XXX - drop this function
func NewDfm() Dfm {
	return Dfm{
		Config: NewDfmConfig(),
	}
}

func (dfm *Dfm) log(operation, relative, repo string, reason error) {
	if dfm.Logger != nil {
		dfm.Logger(operation, relative, repo, reason)
	}
}

// Init will prepare the configured directory for use with dfm, creating it if
// necessary.
func (dfm *Dfm) Init() error {
	return dfm.Config.Save()
}

// addFile is the internal implementation of AddFile and AddFiles. Does less
// error checking.
func (dfm *Dfm) addFile(filename string, repo string, link bool) FileError {
	targetPath, err := filepath.Abs(filename)
	if err != nil {
		return WrapFileError(err, filename)
	}
	// Verify file is under targetPath
	if !strings.HasPrefix(targetPath, dfm.Config.targetPath+"/") {
		return NewFileErrorf(targetPath, "not in target path (%s)", dfm.Config.targetPath)
	}
	relativePath := targetPath[len(dfm.Config.targetPath)+1:]
	stat, err := os.Lstat(targetPath)
	if err != nil {
		return WrapFileError(err, filename)
	}
	if stat.IsDir() {
		return NewFileError(filename, "directories are not supported")
	}
	if !stat.Mode().IsRegular() {
		return NewFileError(filename, "only regular files are supported")
	}
	repoPath := dfm.Config.RepoPath(repo, relativePath)
	if err := MakeDirAll(path.Dir(relativePath), dfm.Config.targetPath, dfm.Config.RepoPath(repo, "")); err != nil {
		return WrapFileError(err, relativePath)
	}
	if link {
		if err := MoveFile(targetPath, repoPath); err != nil {
			return WrapFileError(err, repoPath)
		}
		if err := LinkFile(repoPath, targetPath); err != nil {
			return WrapFileError(err, targetPath)
		}
	} else {
		if err := CopyFile(targetPath, repoPath); err != nil {
			return WrapFileError(err, repoPath)
		}
	}
	dfm.log(OperationAdd, relativePath, repo, nil)
	dfm.Config.AddToManifest(repo, relativePath)
	return nil
}

func (dfm *Dfm) assertIsActiveRepo(repo string) error {
	if !dfm.Config.IsValidRepo(repo) {
		return fmt.Errorf("repo %#v does not exist. To create it, run:\nmkdir %s", repo, dfm.Config.RepoPath(repo, ""))
	} else if !dfm.Config.HasRepo(repo) {
		return fmt.Errorf("repo %#v is not active, cannot add files to it", repo)
	}
	return nil
}

// AddFile will copy the provided file into dfm, optionally replacing the
// original with a symlink to the imported file.
func (dfm *Dfm) AddFile(filename string, repo string, link bool) error {
	if err := dfm.assertIsActiveRepo(repo); err != nil {
		return err
	}
	return dfm.addFile(filename, repo, link)
}

// AddFiles will copy all of the provided files into dfm, optionally replacing
// the originals with symlinks to the imported ones.
func (dfm *Dfm) AddFiles(filenames []string, repo string, link bool, errorHandler ErrorHandler) (err error) {
	if err = dfm.assertIsActiveRepo(repo); err != nil {
		return err
	}
	// If we have to abort the add, we still need to update the manifest with
	// all of the files that were successfully added.
	defer func() {
	}()

	for _, filename := range filenames {
		for {
			err = dfm.addFile(filename, repo, link)
			if err != nil {
				err = errorHandler(err.(FileError))
			}
			if err != Retry {
				break
			}
		}
		if err != nil {
			break
		}
	}

	if saveErr := dfm.Config.Save(); saveErr != nil {
		return saveErr
	}
	return err
}

// runSync is the internal workhorse for both CopyAll and LinkAll.
func (dfm *Dfm) runSync(
	errorHandler ErrorHandler,
	operation string,
	handleFile func(s, d string) error,
) (err error) {
	// Map relative -> repo. Later repos override earlier ones.
	fileList := map[string]string{}
	for _, repo := range dfm.Config.repos {
		repoPath := dfm.Config.RepoPath(repo, "")
		filepath.Walk(repoPath, func(path string, fi os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if fi.IsDir() {
				return nil
			}
			relativePath := path[len(repoPath)+1:]
			fileList[relativePath] = repo
			return nil
		})
	}

	nextManifest := make(map[string]bool, len(fileList))
	for relative, repo := range fileList {
		repoPath := dfm.Config.RepoPath(repo, relative)
		targetPath := dfm.Config.TargetPath(relative)
		fileOperation := operation
		var originalError error
		for {
			originalError = handleFile(repoPath, targetPath)
			if originalError != nil {
				err = errorHandler(WrapFileError(originalError, relative))
				if err == nil {
					fileOperation = OperationSkip
				}
			}
			if err != Retry {
				break
			}
		}
		dfm.log(fileOperation, relative, repo, err)
		if err != nil {
			break
		}
		nextManifest[relative] = true
	}

	if err != nil {
		// Since there was an error, we will bypass the autoclean. This means
		// all existing files plus all new files are presently synced. Merge the
		// old and new manifests.
		for filename := range dfm.Config.manifest {
			nextManifest[filename] = true
		}
	} else {
		// Autoclean
	}

	dfm.Config.manifest = nextManifest
	if saveErr := dfm.Config.Save(); saveErr != nil {
		return saveErr
	}
	return err
}

// LinkAll creates symlinks for files in all repos in the target directory.
func (dfm *Dfm) LinkAll(errorHandler ErrorHandler) error {
	return dfm.runSync(errorHandler, OperationLink, func(s, d string) error {
		// XXX - check if link is already correct
		return LinkFile(s, d)
	})
}

// CopyAll copies files in all repos into the target directory.
func (dfm *Dfm) CopyAll(errorHandler ErrorHandler) error {
	return dfm.runSync(errorHandler, OperationCopy, func(s, d string) error {
		// XXX - check if link is already correct
		return CopyFile(s, d)
	})
}
