package main

import (
	"fmt"
	"os"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

const emptyConfig = `manifest = []
repos = ["files"]
target = "/home/test"
`

const fileContent = "# config file"

func newFs(config string, files []string) afero.Fs {
	fs := afero.NewMemMapFs()
	fs.MkdirAll("/home/test/dotfiles/files", 0777)
	fs.MkdirAll("/home/test/dotfiles/inactive", 0777)
	if config != "" {
		afero.WriteFile(fs, "/home/test/dotfiles/.dfm.toml", []byte(emptyConfig), 0666)
	}
	for _, filename := range files {
		afero.WriteFile(fs, filename, []byte(fileContent), 0666)
	}
	return fs
}

func newDfm(t *testing.T, fs afero.Fs) *Dfm {
	dfm, err := NewDfmFs(fs, "/home/test/dotfiles")
	require.NoError(t, err)
	return dfm
}

func initialSync(t *testing.T, dfm *Dfm) {
	err := dfm.LinkAll(noErrorHandler)
	require.NoError(t, err)
	*dfm = *newDfm(t, dfm.fs)
}

type logMessage struct {
	operation, relative, repo, reason string
}

type testLog struct {
	messages []logMessage
}

func (logger *testLog) log(operation, relative, repo string, reason error) {
	message := ""
	if reason != nil {
		message = reason.Error()
	}
	logger.messages = append(logger.messages, logMessage{operation, relative, repo, message})
}

func TestInit(t *testing.T) {
	fs := newFs("", []string{})
	dfm := newDfm(t, fs)
	dfm.Config.targetPath = "/home/test"
	dfm.Config.repos = []string{"files"}
	err := dfm.Init()
	require.NoError(t, err)
	cfgBytes, err := afero.ReadFile(fs, "/home/test/dotfiles/.dfm.toml")
	require.NoError(t, err)
	require.Equal(t, emptyConfig, string(cfgBytes))
}

func TestInitBadPath(t *testing.T) {
	fs := newFs("", []string{})
	_, err := NewDfmFs(fs, "/home/test/wrongdir")
	require.IsType(t, (*os.PathError)(nil), err)
	pathError := err.(*os.PathError)
	require.Equal(t, pathError.Path, "/home/test/wrongdir")
}

func TestAdd(t *testing.T) {
	fs := newFs(emptyConfig, []string{"/home/test/.bashrc"})
	dfm := newDfm(t, fs)
	err := dfm.AddFile("/home/test/.bashrc", "files", true)
	require.NoError(t, err)
	bytes, err := afero.ReadFile(fs, "/home/test/dotfiles/files/.bashrc")
	require.NoError(t, err)
	require.Equal(t, fileContent, string(bytes))
	bytes, err = afero.ReadFile(fs, "/home/test/.bashrc")
	require.NoError(t, err)
	require.Equal(t, "symlink to /home/test/dotfiles/files/.bashrc", string(bytes))
	require.Equal(t, map[string]bool{".bashrc": true}, dfm.Config.manifest)
}

func TestAddCopy(t *testing.T) {
	fs := newFs(emptyConfig, []string{"/home/test/.bashrc"})
	dfm := newDfm(t, fs)
	err := dfm.AddFile("/home/test/.bashrc", "files", false)
	require.NoError(t, err)
	bytes, err := afero.ReadFile(fs, "/home/test/dotfiles/files/.bashrc")
	require.NoError(t, err)
	require.Equal(t, fileContent, string(bytes))
	bytes, err = afero.ReadFile(fs, "/home/test/.bashrc")
	require.NoError(t, err)
	require.Equal(t, fileContent, string(bytes))
	require.Equal(t, map[string]bool{".bashrc": true}, dfm.Config.manifest)
}

func TestAddOutside(t *testing.T) {
	fs := newFs(emptyConfig, []string{"/mnt/external/.bashrc"})
	dfm := newDfm(t, fs)
	err := dfm.AddFile("/mnt/external/.bashrc", "files", true)
	require.IsType(t, (*FileError)(nil), err)
	fileError := err.(*FileError)
	require.Equal(t, fileError.Filename, "/mnt/external/.bashrc")
	require.Equal(t, fileError.Message, "not in target path (/home/test)")
}

func TestAddNested(t *testing.T) {
	fs := newFs(emptyConfig, []string{"/home/test/.config/fish/config.fish"})
	dfm := newDfm(t, fs)
	err := dfm.AddFile("/home/test/.config/fish/config.fish", "files", true)
	require.NoError(t, err)
	bytes, err := afero.ReadFile(fs, "/home/test/dotfiles/files/.config/fish/config.fish")
	require.NoError(t, err)
	require.Equal(t, fileContent, string(bytes))
}

func TestSync(t *testing.T) {
	fs := newFs(emptyConfig, []string{
		"/home/test/dotfiles/files/.config/fish/config.fish",
	})
	dfm := newDfm(t, fs)
	logger := &testLog{}
	dfm.Logger = logger.log
	handleFile := func(s, d string) error {
		return nil
	}
	err := dfm.runSync(noErrorHandler, OperationLink, handleFile)
	require.NoError(t, err)
	require.Equal(t, map[string]bool{".config/fish/config.fish": true}, dfm.Config.manifest)
	require.Equal(t, []logMessage{
		{OperationLink, ".config/fish/config.fish", "files", ""},
	}, logger.messages)
}

func TestSyncErrorPartial(t *testing.T) {
	fs := newFs(emptyConfig, []string{
		"/home/test/dotfiles/files/.fileA",
		"/home/test/dotfiles/files/.fileC",
	})
	dfm := newDfm(t, fs)
	initialSync(t, dfm)
	var logger testLog
	dfm.Logger = logger.log

	handleFile := func(s, d string) error {
		if d == "/home/test/.fileB" {
			return fmt.Errorf("fake error")
		} else if d == "/home/test/.fileC" {
			require.FailNow(t, "runSync should have aborted at fileB")
		}
		exists, err := afero.Exists(fs, d)
		if err != nil {
			return err
		} else if exists {
			return ErrNotNeeded
		}
		return LinkFile(dfm.fs, s, d)
	}
	afero.WriteFile(fs, "/home/test/dotfiles/files/.fileB", []byte(fileContent), 0666)
	err := dfm.runSync(noErrorHandler, OperationLink, handleFile)
	require.Error(t, err)
	require.Equal(t, ".fileB: fake error", err.Error())
	require.Equal(t, map[string]bool{".fileA": true, ".fileB": true, ".fileC": true}, dfm.Config.manifest)
	require.Equal(t, []logMessage{
		{OperationSkip, ".fileA", "files", ".fileA: already up to date"},
	}, logger.messages)
}

func TestSyncIgnoreError(t *testing.T) {
	fs := newFs(emptyConfig, []string{
		"/home/test/dotfiles/files/.fileA",
		"/home/test/dotfiles/files/.fileC",
	})
	dfm := newDfm(t, fs)
	initialSync(t, dfm)
	var logger testLog
	dfm.Logger = logger.log

	handleFile := func(s, d string) error {
		if d == "/home/test/.fileB" {
			return fmt.Errorf("fake error")
		}
		exists, err := afero.Exists(fs, d)
		if err != nil {
			return err
		} else if exists {
			return ErrNotNeeded
		}
		return LinkFile(dfm.fs, s, d)
	}
	errorHandler := func(err *FileError) error {
		return nil
	}
	afero.WriteFile(fs, "/home/test/dotfiles/files/.fileB", []byte(fileContent), 0666)
	err := dfm.runSync(errorHandler, OperationLink, handleFile)
	require.NoError(t, err)
	require.Equal(t, map[string]bool{".fileA": true, ".fileB": true, ".fileC": true}, dfm.Config.manifest)
	require.Equal(t, []logMessage{
		{OperationSkip, ".fileA", "files", ".fileA: already up to date"},
		{OperationSkip, ".fileB", "files", ".fileB: fake error"},
		{OperationSkip, ".fileC", "files", ".fileC: already up to date"},
	}, logger.messages)
}

func TestSyncRetry(t *testing.T) {
	fs := newFs(emptyConfig, []string{
		"/home/test/dotfiles/files/.fileA",
	})
	dfm := newDfm(t, fs)
	var logger testLog
	dfm.Logger = logger.log

	timesCalled := 0
	handleFile := func(s, d string) (err error) {
		timesCalled++
		if timesCalled == 1 {
			return fmt.Errorf("temporary error")
		}
		exists, err := afero.Exists(fs, d)
		if err != nil {
			return err
		} else if exists {
			return nil
		}
		return LinkFile(dfm.fs, s, d)
	}
	errorHandler := func(err *FileError) error {
		if err.Message == "temporary error" {
			return Retry
		}
		return err
	}
	err := dfm.runSync(errorHandler, OperationLink, handleFile)
	require.NoError(t, err)
	require.Equal(t, map[string]bool{".fileA": true}, dfm.Config.manifest)
	require.Equal(t, timesCalled, 2)
	require.Equal(t, []logMessage{
		{OperationLink, ".fileA", "files", ""},
	}, logger.messages)
}

func TestEjectFiles(t *testing.T) {
	fs := newFs(emptyConfig, []string{"/home/test/dotfiles/files/.bashrc"})
	dfm := newDfm(t, fs)
	err := dfm.EjectFiles([]string{".bashrc"}, noErrorHandler)
	require.NoError(t, err)
	bytes, err := afero.ReadFile(fs, "/home/test/.bashrc")
	require.NoError(t, err)
	require.Equal(t, fileContent, string(bytes))
	require.Equal(t, map[string]bool{}, dfm.Config.manifest)
}

func TestAutoclean(t *testing.T) {
	fs := newFs(emptyConfig, []string{
		"/home/test/dotfiles/files/.config/fileA",
	})
	dfm := newDfm(t, fs)
	initialSync(t, dfm)
	var logger testLog
	dfm.Logger = logger.log

	fs.Rename(
		"/home/test/dotfiles/files/.config/fileA",
		"/home/test/dotfiles/files/.fileB",
	)

	handleFile := func(s, d string) error {
		return nil
	}
	err := dfm.runSync(noErrorHandler, OperationLink, handleFile)
	require.NoError(t, err)
	require.Equal(t, map[string]bool{".fileB": true}, dfm.Config.manifest)
	require.Equal(t, []logMessage{
		{OperationLink, ".fileB", "files", ""},
		{OperationRemove, ".config/fileA", "", ""},
	}, logger.messages)
}

func TestIsActiveRepo(t *testing.T) {
	fs := newFs(emptyConfig, []string{})
	dfm := newDfm(t, fs)
	err := dfm.assertIsActiveRepo("inactive")
	require.Error(t, err)
	require.Contains(t, err.Error(), `repo "inactive" is not active`)
	err = dfm.assertIsActiveRepo("invalid")
	require.Error(t, err)
	require.Contains(t, err.Error(), `repo "invalid" does not exist`)
}

func TestChangeConfig(t *testing.T) {
	fs := newFs(emptyConfig, []string{})
	dfm := newDfm(t, fs)
	dfm.Config.manifest["some/existing/file"] = true
	dfm.Config.repos = []string{"files2"}
	err := dfm.Config.Save()
	require.NoError(t, err)
	cfgBytes, err := afero.ReadFile(fs, "/home/test/dotfiles/.dfm.toml")
	require.NoError(t, err)
	require.Equal(t,
		`manifest = ["some/existing/file"]
repos = ["files2"]
target = "/home/test"
`,
		string(cfgBytes),
	)
}

func TestDryRun(t *testing.T) {
	fs := newFs(emptyConfig, []string{
		"/home/test/dotfiles/files/.fileA",
	})
	dfm := newDfm(t, fs)
	initialSync(t, dfm)
	fs.Rename(
		"/home/test/dotfiles/files/.fileA",
		"/home/test/dotfiles/files/.fileB",
	)
	fs = afero.NewReadOnlyFs(fs)
	dfm, err := NewDfmFs(fs, "/home/test/dotfiles")
	require.NoError(t, err)
	var logger testLog
	dfm.Logger = logger.log
	dfm.DryRun = true

	handleFile := func(s, d string) error {
		return nil
	}
	err = dfm.runSync(noErrorHandler, OperationLink, handleFile)
	require.NoError(t, err)
	require.Equal(t, map[string]bool{".fileB": true}, dfm.Config.manifest)
	require.Equal(t, []logMessage{
		{OperationLink, ".fileB", "files", ""},
		{OperationRemove, ".fileA", "", ""},
	}, logger.messages)
}
