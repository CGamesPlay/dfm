// +build integration

package main

import (
	"flag"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

var update = flag.Bool("update", false, "update .snap files")
var testRoot string

// setupDirectory creates the scaffolding necessary for a given test.
func setupDirectory(t *testing.T) func() {
	// Create directory
	tmpdir, _ := os.LookupEnv("TMPDIR")
	if tmpdir == "" {
		tmpdir = "/tmp"
	}
	var err error
	tmpdir, err = filepath.EvalSymlinks(tmpdir)
	require.NoError(t, err, "creating temporary directory for test")
	testRoot, err = ioutil.TempDir(tmpdir, "dfm")
	require.NoError(t, err, "creating temporary directory for test")

	return func() {
		// Remove directory
		cmd := exec.Command("rm", "-rf", testRoot)
		err := cmd.Run()
		require.NoError(t, err, "cleaning temporary directory from test")
	}
}

func runScript(t *testing.T, script string) {
	_, err := exec.LookPath("dfm")
	require.NoError(t, err, "is dfm installed and in PATH?")
	absolute, err := filepath.Abs(script)
	require.NoError(t, err)

	cmd := exec.Command("bash", absolute)
	cmd.Dir = testRoot
	actualBytes, err := cmd.CombinedOutput()
	// Replace the testRoot with '/test/' in the output paths.
	actual := strings.ReplaceAll(string(actualBytes), testRoot, "/test")
	require.NoError(t, err, actual)

	snapshot := filepath.Join("testdata", t.Name()+".snap")
	directory := filepath.Dir(snapshot)
	err = os.MkdirAll(directory, 0755)
	if *update {
		ioutil.WriteFile(snapshot, []byte(actual), 0644)
	}
	expected, err := ioutil.ReadFile(snapshot)
	require.NoError(t, err)

	require.Equal(t, string(expected), actual, "to update snapshots run go test -update")
}

func TestDfm(t *testing.T) {
	scripts, err := filepath.Glob("testdata/TestDfm/*.sh")
	require.NoError(t, err)
	for _, script := range scripts {
		basename := filepath.Base(script)
		t.Run(basename, func(t *testing.T) {
			defer setupDirectory(t)()
			runScript(t, script)
		})
	}
}
