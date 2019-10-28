package main

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/cevaris/ordered_map"

	"github.com/spf13/afero"
)

const (
	// OperationAdd means a file was added to a repo.
	OperationAdd = "added"
	// OperationLink means a file was linked from a repo to the target.
	OperationLink = "linked"
	// OperationCopy means a file was copied from a repo to the target.
	OperationCopy = "copied"
	// OperationRemove means a file was removed from the target. If there was an
	// error removing the file, reason will describe it.
	OperationRemove = "removed"
	// OperationSkip means a file was not copied/linked to the target. The
	// reason will be the original error, even though the ErrorHandler
	// suppressed the error. If the error is nil, it's because the file is
	// already synced.
	OperationSkip = "skipped"
)

// Logger is the type of function that dfm calls whenever it performs a file
// operation.
type Logger func(operation, relative, repo string, reason error)

func noErrorHandler(err *FileError) error {
	return err
}

// Dfm is the main controller class for API access to dfm
type Dfm struct {
	// The configuration used by this dfm instance
	Config Config
	// The log function used by this dfm instance
	Logger Logger
	// When set, don't actually do file operations, only log
	DryRun bool
	fs     afero.Fs
}

// NewDfm creates a new dfm instance with the provided dfm dir.
func NewDfm(dfmDir string) (*Dfm, error) {
	return NewDfmFs(afero.NewOsFs(), dfmDir)
}

// NewDfmFs creates a new dfm instance using the provided filesystem driver and
// df mdir.
func NewDfmFs(fs afero.Fs, dfmDir string) (*Dfm, error) {
	config := Config{fs: fs}
	if err := config.SetDirectory(dfmDir); err != nil {
		return nil, err
	}
	return &Dfm{fs: fs, Config: config}, nil

}

func (dfm *Dfm) log(operation, relative, repo string, reason error) {
	if dfm.Logger != nil {
		dfm.Logger(operation, relative, repo, reason)
	}
}

func (dfm *Dfm) saveConfig() error {
	if dfm.DryRun {
		return nil
	}
	if saveErr := dfm.Config.Save(); saveErr != nil {
		return saveErr
	}
	return nil
}

// Init will prepare the configured directory for use with dfm, creating it if
// necessary.
func (dfm *Dfm) Init() error {
	return dfm.saveConfig()
}

// IsValidRepo returns true if the given name is a directory in the dfm dir.
func (dfm *Dfm) IsValidRepo(repo string) bool {
	fs := dfm.fs
	stat, err := fs.Stat(pathJoin(dfm.Config.path, repo))
	if err != nil {
		return false
	}
	return stat.IsDir()
}

// HasRepo returns true if the given name is a repository that is currently
// configured to be used.
func (dfm *Dfm) HasRepo(repo string) bool {
	for _, test := range dfm.Config.repos {
		if test == repo {
			return true
		}
	}
	return false
}

// RepoPath returns the path to the given file inside of the given repo.
func (dfm *Dfm) RepoPath(repo string, relative string) string {
	return pathJoin(dfm.Config.path, repo, relative)
}

// TargetPath returns the path to the given file inside of the target.
func (dfm *Dfm) TargetPath(relative string) string {
	return pathJoin(dfm.Config.targetPath, relative)
}

// addFile is the internal implementation of AddFile and AddFiles. Does less
// error checking.
func (dfm *Dfm) addFile(filename string, repo string, link bool) error {
	fs := dfm.fs
	targetPath, err := filepath.Abs(filename)
	if err != nil {
		return WrapFileError(err, filename)
	}
	// Verify file is under targetPath
	if !strings.HasPrefix(targetPath, dfm.Config.targetPath+"/") {
		return NewFileErrorf(targetPath, "not in target path (%s)", dfm.Config.targetPath)
	}
	relativePath := targetPath[len(dfm.Config.targetPath)+1:]
	isRegular, err := IsRegularFile(fs, targetPath)
	if err != nil {
		return WrapFileError(err, filename)
	} else if !isRegular {
		stat, _ := fs.Stat(targetPath)
		if stat.IsDir() {
			return NewFileError(filename, "directories are not supported")
		}
		return NewFileError(filename, "only regular files are supported")
	}
	repoPath := dfm.RepoPath(repo, relativePath)
	if dfm.DryRun {
		// do nothing
	} else {
		if err := MakeDirAll(fs, path.Dir(relativePath), dfm.Config.targetPath, dfm.RepoPath(repo, "")); err != nil {
			return WrapFileError(err, relativePath)
		}
		if link {
			if err := MoveFile(fs, targetPath, repoPath); err != nil {
				return WrapFileError(err, repoPath)
			}
			if err := LinkFile(fs, repoPath, targetPath); err != nil {
				return WrapFileError(err, targetPath)
			}
		} else {
			if err := CopyFile(fs, targetPath, repoPath); err != nil {
				return WrapFileError(err, repoPath)
			}
		}
	}
	dfm.log(OperationAdd, relativePath, repo, nil)
	dfm.Config.manifest[relativePath] = true
	return nil
}

func (dfm *Dfm) assertIsActiveRepo(repo string) error {
	if !dfm.IsValidRepo(repo) {
		return fmt.Errorf("repo %#v does not exist. To create it, run:\nmkdir %s", repo, dfm.RepoPath(repo, ""))
	} else if !dfm.HasRepo(repo) {
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
	if err := dfm.addFile(filename, repo, link); err != nil {
		return err
	}
	if err := dfm.saveConfig(); err != nil {
		return err
	}
	return nil
}

// AddFiles will copy all of the provided files into dfm, optionally replacing
// the originals with symlinks to the imported ones.
func (dfm *Dfm) AddFiles(filenames []string, repo string, link bool, errorHandler ErrorHandler) (err error) {
	if err = dfm.assertIsActiveRepo(repo); err != nil {
		return err
	}

	// XXX - this doesn't work, and directories
	for _, filename := range filenames {
		for {
			err = dfm.addFile(filename, repo, link)
			if err != nil {
				fileErr, ok := err.(*FileError)
				if !ok {
					fileErr = WrapFileError(err, filename)
				}
				err = errorHandler(fileErr)
			}
			if err != Retry {
				break
			}
		}
		if err != nil {
			break
		}
	}

	if saveErr := dfm.saveConfig(); saveErr != nil {
		return saveErr
	}
	return err
}

// runSync is the internal workhorse for both CopyAll and LinkAll.
func (dfm *Dfm) runSync(
	errorHandler ErrorHandler,
	operation string,
	handleFile func(s, d string) error,
) error {
	fs := dfm.fs
	// Map relative -> repo. Later repos override earlier ones.
	fileList := ordered_map.NewOrderedMap()
	for _, repo := range dfm.Config.repos {
		repoPath := dfm.RepoPath(repo, "")
		err := afero.Walk(fs, repoPath, func(path string, fi os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if fi.IsDir() {
				return nil
			}
			relativePath := path[len(repoPath)+1:]
			fileList.Set(relativePath, repo)
			return nil
		})
		if err != nil {
			return err
		}
	}

	nextManifest := make(map[string]bool, fileList.Len())
	iter := fileList.IterFunc()
	var overallErr error
	for kv, ok := iter(); ok; kv, ok = iter() {
		relative := kv.Key.(string)
		// Add this file to the manifest now. Even if there is an error, we
		// don't want autoclean to remove this file.
		nextManifest[relative] = true
		repo := kv.Value.(string)
		repoPath := dfm.RepoPath(repo, relative)
		targetPath := dfm.TargetPath(relative)
		fileOperation := operation
		var skipReason error
		for {
			// XXX - change this to (relative, repo)
			rawErr := handleFile(repoPath, targetPath)
			if rawErr == nil || rawErr == ErrNotNeeded {
				if rawErr == ErrNotNeeded {
					fileOperation = OperationSkip
					skipReason = nil
				}
			} else {
				wrappedErr, ok := rawErr.(*FileError)
				if !ok {
					wrappedErr = WrapFileError(rawErr, relative)
				}
				newErr := errorHandler(wrappedErr)
				if newErr == nil {
					fileOperation = OperationSkip
					skipReason = wrappedErr
				} else if newErr == Retry {
					continue
				}
				overallErr = newErr
			}
			break
		}
		if overallErr != nil {
			break
		}
		dfm.log(fileOperation, relative, repo, skipReason)
	}

	if overallErr != nil {
		// Since there was an error, we will bypass the autoclean. This
		// means all existing files plus all new files are presently synced.
		// Merge the old and new manifests.
		for filename := range dfm.Config.manifest {
			nextManifest[filename] = true
		}
		dfm.Config.manifest = nextManifest
	} else {
		dfm.autoclean(nextManifest)
	}

	if saveErr := dfm.saveConfig(); saveErr != nil {
		return saveErr
	}
	return overallErr
}

// LinkAll creates symlinks for files in all repos in the target directory.
func (dfm *Dfm) LinkAll(errorHandler ErrorHandler) error {
	return dfm.runSync(errorHandler, OperationLink, func(s, d string) error {
		done, err := IsLinkedFile(dfm.fs, s, d)
		if err != nil {
			return err
		} else if done {
			return ErrNotNeeded
		} else if dfm.DryRun {
			return nil
		}
		relativePath := d[len(dfm.Config.targetPath)+1:]
		repoPath := s[:len(s)-len(relativePath)-1]
		if err := MakeDirAll(dfm.fs, path.Dir(relativePath), dfm.Config.targetPath, repoPath); err != nil {
			return err
		}
		return LinkFile(dfm.fs, s, d)
	})
}

// CopyAll copies files in all repos into the target directory.
func (dfm *Dfm) CopyAll(errorHandler ErrorHandler) error {
	return dfm.runSync(errorHandler, OperationCopy, func(s, d string) error {
		// XXX - check if file is identical
		if dfm.DryRun {
			return nil
		}
		relativePath := d[len(dfm.Config.targetPath)+1:]
		repoPath := s[:len(s)-len(relativePath)-1]
		if err := MakeDirAll(dfm.fs, path.Dir(relativePath), dfm.Config.targetPath, repoPath); err != nil {
			return err
		}
		return CopyFile(dfm.fs, s, d)
	})
}

// RemoveAll removes all synced files from the target directory.
func (dfm *Dfm) RemoveAll() error {
	nextManifest := map[string]bool{}
	dfm.autoclean(nextManifest)
	if saveErr := dfm.saveConfig(); saveErr != nil {
		return saveErr
	}
	return nil
}

// autoclean will remove all synced files from the target directory except those
// that are listed in nextManifest. The manifest will be updated but not saved.
func (dfm *Dfm) autoclean(nextManifest map[string]bool) {
	for filename := range dfm.Config.manifest {
		_, found := nextManifest[filename]
		if !found {
			var err error
			if !dfm.DryRun {
				err = RemoveFile(dfm.fs, dfm.TargetPath(filename))
				// XXX - remove empty directories
			}
			dfm.log(OperationRemove, filename, "", err)
			if err == nil {
				delete(dfm.Config.manifest, filename)
			}
		}
	}
	for filename := range nextManifest {
		dfm.Config.manifest[filename] = true
	}
}
