package main

import (
	"os"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

const emptyConfig = `manifest = []
repos = ["files"]
target = "/home/test"
`

const preinstalledContent = "preinstalled config file"

func newFs(config string, preinstalled []string) afero.Fs {
	fs := afero.NewMemMapFs()
	fs.MkdirAll("/home/test/dotfiles/files", 0777)
	fs.MkdirAll("/home/test/dotfiles/inactive", 0777)
	if config != "" {
		afero.WriteFile(fs, "/home/test/dotfiles/.dfm.toml", []byte(emptyConfig), 0666)
	}
	for _, filename := range preinstalled {
		afero.WriteFile(fs, filename, []byte(preinstalledContent), 0666)
	}
	return fs
}

func newDfm(t *testing.T, fs afero.Fs) *Dfm {
	dfm, err := NewDfmFs(fs, "/home/test/dotfiles")
	require.NoError(t, err)
	return dfm
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
	require.Equal(t, preinstalledContent, string(bytes))
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
	require.Equal(t, preinstalledContent, string(bytes))
	bytes, err = afero.ReadFile(fs, "/home/test/.bashrc")
	require.NoError(t, err)
	require.Equal(t, preinstalledContent, string(bytes))
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
	require.Equal(t, preinstalledContent, string(bytes))
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
	t.Skip("Use a synced file instead of the nonsynced one")
	fs := newFs(emptyConfig, []string{})
	dfm := newDfm(t, fs)
	dfm.Config.repos = []string{"files2"}
	err := dfm.Config.Save()
	require.NoError(t, err)
	cfgBytes, err := afero.ReadFile(fs, "/home/test/dotfiles/.dfm.toml")
	require.NoError(t, err)
	require.Equal(t,
		`manifest = ["asdf"]
repos = ["files2"]
target = "/home/test"
`,
		string(cfgBytes),
	)
}
