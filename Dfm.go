package main

import (
	"fmt"
	"os"
	"path"
	"sort"
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
	// suppressed the error.
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

func (dfm *Dfm) assertIsActiveRepo(repo string) error {
	if !dfm.IsValidRepo(repo) {
		return fmt.Errorf("repo %#v does not exist. To create it, run:\nmkdir %s", repo, dfm.RepoPath(repo, ""))
	} else if !dfm.HasRepo(repo) {
		return fmt.Errorf("repo %#v is not active, cannot add files to it", repo)
	}
	return nil
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
// error checking. Returns the relative path and an error value.
func (dfm *Dfm) addFile(relativePath string, repo string, link bool) (string, error) {
	fs := dfm.fs
	targetPath := dfm.TargetPath(relativePath)
	repoPath := dfm.RepoPath(repo, relativePath)
	isRegular, err := IsRegularFile(fs, targetPath)
	if err != nil {
		return "", WrapFileError(err, targetPath)
	} else if !isRegular {
		if linked, err := IsLinkedFile(fs, repoPath, targetPath); linked || err != nil {
			if err != nil {
				return "", err
			}
			return "", ErrNotNeeded
		}
		return "", NewFileError(targetPath, "only regular files are supported")
	}
	if dfm.DryRun {
		// do nothing
	} else {
		if err := MakeDirAll(fs, path.Dir(relativePath), dfm.Config.targetPath, dfm.RepoPath(repo, "")); err != nil {
			return "", WrapFileError(err, relativePath)
		}
		if link {
			if err := MoveFile(fs, targetPath, repoPath); err != nil {
				return "", WrapFileError(err, repoPath)
			}
			if err := LinkFile(fs, repoPath, targetPath); err != nil {
				return "", WrapFileError(err, targetPath)
			}
		} else {
			if err := CopyFile(fs, targetPath, repoPath); err != nil {
				return "", WrapFileError(err, repoPath)
			}
		}
	}
	return relativePath, nil
}

// AddFile will copy the provided file into dfm, optionally replacing the
// original with a symlink to the imported file.
func (dfm *Dfm) AddFile(filename string, repo string, link bool) error {
	return dfm.AddFiles([]string{filename}, repo, link, noErrorHandler)
}

// AddFiles will copy all of the provided files into dfm, optionally replacing
// the originals with symlinks to the imported ones.
func (dfm *Dfm) AddFiles(inputFilenames []string, repo string, link bool, errorHandler ErrorHandler) error {
	if err := dfm.assertIsActiveRepo(repo); err != nil {
		return err
	}

	fileList := ordered_map.NewOrderedMap()
	for _, inputFilename := range inputFilenames {
		joined := pathJoin(dfm.Config.targetPath, inputFilename)
		if !strings.HasPrefix(joined, dfm.Config.targetPath) {
			return NewFileErrorf(inputFilename, "not in target path (%s)", dfm.Config.targetPath)
		} else if strings.HasPrefix(joined, dfm.Config.path) {
			return NewFileError(inputFilename, "cannot add a file already inside the dfm directory")
		}
		err := populateFileList(dfm.fs, dfm.Config.targetPath, inputFilename, fileList, repo)
		if err != nil {
			return err
		}
	}

	iter := fileList.IterFunc()
	var overallErr error
	for kv, ok := iter(); ok; kv, ok = iter() {
		filename := kv.Key.(string)
		fileOperation := OperationAdd
		var relativePath string
		skip, abort, fileErr := processWithRetry(errorHandler, func() *FileError {
			var rawErr error
			relativePath, rawErr = dfm.addFile(filename, repo, link)
			if rawErr == nil {
				return nil
			}
			return WrapFileError(rawErr, filename)
		})
		if abort {
			overallErr = fileErr
			break
		} else if skip {
			fileOperation = OperationSkip
		} else {
			dfm.Config.manifest[relativePath] = true
		}
		dfm.log(fileOperation, filename, repo, fileErr)
	}

	if saveErr := dfm.saveConfig(); saveErr != nil {
		return saveErr
	}
	return overallErr
}

// buildFileList scans the given paths in each repo, and returns an OrderedMap
// of relative -> repo. Only the file existing in the last-referenced repo will
// be used.
func (dfm *Dfm) buildFileList(paths []string) (*ordered_map.OrderedMap, error) {
	fs := dfm.fs
	// Map relative -> repo. Later repos override earlier ones.
	fileList := ordered_map.NewOrderedMap()
	for _, path := range paths {
		found := false
		for _, repo := range dfm.Config.repos {
			err := populateFileList(fs, dfm.RepoPath(repo, ""), path, fileList, repo)
			if err == nil {
				found = true
			} else if !os.IsNotExist(err) {
				return nil, err
			}
		}
		if !found {
			return nil, NewFileError(path, "not found in any active repositories")
		}
	}
	return fileList, nil
}

// syncFiles will handle the given list of files and add files to the manifest
// appropriately.
func (dfm *Dfm) syncFiles(
	fileList *ordered_map.OrderedMap,
	nextManifest map[string]bool,
	errorHandler ErrorHandler,
	operation string,
	handleFile func(s, d string) error,
) error {
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
		skip, abort, fileErr := processWithRetry(errorHandler, func() *FileError {
			rawErr := handleFile(repoPath, targetPath)
			if rawErr == nil {
				return nil
			}
			return WrapFileError(rawErr, relative)
		})
		if abort {
			overallErr = fileErr
			break
		} else if skip {
			fileOperation = OperationSkip
		}
		dfm.log(fileOperation, relative, repo, fileErr)
	}
	return overallErr
}

// runPartialSync is used for syncing specific files. It accepts a list of
// relative filenames to sync, updates the manifest, but does not run the
// cleanup.
func (dfm *Dfm) runPartialSync(
	inputFilenames []string,
	errorHandler ErrorHandler,
	operation string,
	handleFile func(s, d string) error,
) error {
	fileList, err := dfm.buildFileList(inputFilenames)
	if err != nil {
		return err
	}
	err = dfm.syncFiles(fileList, dfm.Config.manifest, errorHandler, operation, handleFile)
	if saveErr := dfm.saveConfig(); saveErr != nil {
		return saveErr
	}
	return err
}

// runSync is the main sync function, responsible for listing all files to be
// synced, syncing them, then running the cleanup.
func (dfm *Dfm) runSync(
	errorHandler ErrorHandler,
	operation string,
	handleFile func(s, d string) error,
) error {
	fileList, err := dfm.buildFileList([]string{"."})
	if err != nil {
		return err
	}

	nextManifest := make(map[string]bool, fileList.Len())
	err = dfm.syncFiles(fileList, nextManifest, errorHandler, operation, handleFile)
	if err != nil {
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
	return err
}

// handleLink is the workhorse for linking files.
func (dfm *Dfm) handleLink(s, d string) error {
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
	if err := MakeDirAll(dfm.fs, path.Dir(relativePath), repoPath, dfm.Config.targetPath); err != nil {
		return err
	}
	return LinkFile(dfm.fs, s, d)
}

// handleCopy is the workhorse for copying files.
func (dfm *Dfm) handleCopy(s, d string) error {
	// XXX - check if file is identical
	if dfm.DryRun {
		return nil
	}
	isLinked, err := IsLinkedFile(dfm.fs, s, d)
	if err != nil {
		return err
	} else if isLinked {
		// We allow copy to replace a link to its source file. This should only
		// come up when ejecting.
		err = RemoveFile(dfm.fs, d)
		if err != nil {
			return err
		}
	}
	relativePath := d[len(dfm.Config.targetPath)+1:]
	repoPath := s[:len(s)-len(relativePath)-1]
	if err := MakeDirAll(dfm.fs, path.Dir(relativePath), repoPath, dfm.Config.targetPath); err != nil {
		return err
	}
	return CopyFile(dfm.fs, s, d)
}

// LinkFiles creates symlinks for the given files only. Does not run the
// autoclean, but does update the manifest.
func (dfm *Dfm) LinkFiles(inputFilenames []string, errorHandler ErrorHandler) error {
	return dfm.runPartialSync(inputFilenames, errorHandler, OperationLink, dfm.handleLink)
}

// LinkAll creates symlinks for files in all repos in the target directory and
// runs the autoclean.
func (dfm *Dfm) LinkAll(errorHandler ErrorHandler) error {
	return dfm.runSync(errorHandler, OperationLink, dfm.handleLink)
}

// CopyFiles copies the given files to the target directory. Does not run the
// autoclean, but does update the manifest.
func (dfm *Dfm) CopyFiles(inputFilenames []string, errorHandler ErrorHandler) error {
	return dfm.runPartialSync(inputFilenames, errorHandler, OperationCopy, dfm.handleCopy)
}

// CopyAll copies all files in all report to the target directory and
// runs the autoclean.
func (dfm *Dfm) CopyAll(errorHandler ErrorHandler) error {
	return dfm.runSync(errorHandler, OperationCopy, dfm.handleCopy)
}

// RemoveFiles removes the given files from the target directory and from the
// manifest.
func (dfm *Dfm) RemoveFiles(inputFilenames []string) error {
	nextManifest := make(map[string]bool, len(dfm.Config.manifest))
	for filename := range dfm.Config.manifest {
		nextManifest[filename] = true
	}
	for _, filename := range inputFilenames {
		if _, ok := nextManifest[filename]; !ok {
			dfm.log(OperationSkip, filename, "", NewFileError(filename, "not tracked by dfm"))
		} else {
			delete(nextManifest, filename)
		}
	}
	dfm.autoclean(nextManifest)
	if saveErr := dfm.saveConfig(); saveErr != nil {
		return saveErr
	}
	return nil
}

// RemoveAll removes all tracked files from the target directory.
func (dfm *Dfm) RemoveAll() error {
	nextManifest := map[string]bool{}
	dfm.autoclean(nextManifest)
	if saveErr := dfm.saveConfig(); saveErr != nil {
		return saveErr
	}
	return nil
}

// EjectFiles copies the given files to the target directory, but removes them
// from the manifest. This results in future operations failing due to an
// existing file, as well as the autoclean never removing the files.
func (dfm *Dfm) EjectFiles(inputFilenames []string, errorHandler ErrorHandler) error {
	fileList, err := dfm.buildFileList(inputFilenames)
	if err != nil {
		return err
	}
	err = dfm.syncFiles(fileList, dfm.Config.manifest, errorHandler, OperationCopy, dfm.handleCopy)
	iter := fileList.IterFunc()
	for kv, ok := iter(); ok; kv, ok = iter() {
		relative := kv.Key.(string)
		// Remove the file from the manifest
		delete(dfm.Config.manifest, relative)
	}
	if saveErr := dfm.saveConfig(); saveErr != nil {
		return saveErr
	}
	return err
}

// autoclean will remove all synced files from the target directory except those
// that are listed in nextManifest. The manifest will be updated but not saved.
func (dfm *Dfm) autoclean(nextManifest map[string]bool) {
	var toRemove []string
	for filename := range dfm.Config.manifest {
		_, found := nextManifest[filename]
		if !found {
			toRemove = append(toRemove, filename)
		}
	}
	sort.Strings(toRemove)
	for _, filename := range toRemove {
		var err error
		if !dfm.DryRun {
			err = RemoveFile(dfm.fs, dfm.TargetPath(filename))
			if err == nil {
				err = CleanDirectories(dfm.fs, path.Dir(dfm.TargetPath(filename)), dfm.Config.targetPath)
			}
		}
		dfm.log(OperationRemove, filename, "", err)
		if err == nil {
			delete(dfm.Config.manifest, filename)
		}
	}
	for filename := range nextManifest {
		dfm.Config.manifest[filename] = true
	}
}
