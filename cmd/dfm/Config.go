package main

import (
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"github.com/pelletier/go-toml"
)

// TomlFilename is the filename where the dfm configuration can be found.
const TomlFilename = ".dfm.toml"

type configFile struct {
	Repos    []string `toml:"repos"`
	Target   string   `toml:"target"`
	Manifest []string `toml:"manifest"`
}

func manifestToConfig(manifest map[string]bool) []string {
	keys := make([]string, 0, len(manifest))
	for k := range manifest {
		keys = append(keys, k)
	}
	return keys
}

func configToManifest(config []string) map[string]bool {
	m := make(map[string]bool, len(config))
	for _, key := range config {
		m[key] = true
	}
	return m
}

// Config is the main object that holds the configuration for dfm.
type Config struct {
	// Main dfm directory
	path string
	// Target directory, normally ~/
	targetPath string
	// Active repos
	repos []string
	// Tracked files
	manifest map[string]bool
}

// NewDfmConfig creates an empty Config.
func NewDfmConfig() Config {
	home, _ := os.LookupEnv("HOME")
	return Config{
		targetPath: path.Clean(home),
		manifest:   map[string]bool{},
	}
}

// SetDirectory takes a directory with a dfm.toml file in it and loads that
// configuration.
func (config *Config) SetDirectory(dir string) error {
	absPath, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	config.path = absPath
	if _, err := os.Stat(dir); err != nil {
		return err
	}
	bytes, err := ioutil.ReadFile(path.Join(dir, TomlFilename))
	// Not having a config file is the same as having an empty config file, so
	// don't fail if the file doesn't exist.
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if bytes != nil {
		var file configFile
		if err := toml.Unmarshal(bytes, &file); err != nil {
			return err
		}
		config.applyFile(file)
	}
	targetPath, err := filepath.Abs(config.targetPath)
	if err != nil {
		return err
	}
	config.targetPath = targetPath
	return nil
}

// applyFile looks at all settings that are set in the config file and applies
// them.
func (config *Config) applyFile(file configFile) {
	if file.Repos != nil {
		config.repos = file.Repos
	}
	if file.Target != "" {
		config.targetPath = file.Target
	}
	if file.Manifest != nil {
		config.manifest = configToManifest(file.Manifest)
	}
}

// Save writes a dfm.toml file to the config's path.
func (config *Config) Save() error {
	var file configFile
	if config.repos != nil {
		file.Repos = config.repos
	}
	if config.targetPath != "" {
		file.Target = config.targetPath
	}
	if len(config.manifest) > 0 {
		file.Manifest = manifestToConfig(config.manifest)
	}

	bytes, err := toml.Marshal(file)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(path.Join(config.path, TomlFilename), bytes, 0644)
}
