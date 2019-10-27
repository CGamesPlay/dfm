package main

import (
	"io/ioutil"
	"os"
	"path"

	"github.com/pelletier/go-toml"
)

// TomlFilename is the filename where the dfm configuration can be found.
const TomlFilename = ".dfm.toml"

type configFile struct {
	Repos []string `toml:"repos"`
}

// DfmConfig is the main object that holds the configuration for dfm.
type DfmConfig struct {
	path  string
	repos []string
}

// NewDfmConfig creates an empty DfmConfig.
func NewDfmConfig() DfmConfig {
	return DfmConfig{}
}

// SetDirectory takes a directory with a dfm.toml file in it and loads that
// configuration.
func (config *DfmConfig) SetDirectory(dir string) error {
	if _, err := os.Stat(dir); err != nil {
		return err
	}
	config.path = dir
	bytes, err := ioutil.ReadFile(path.Join(dir, TomlFilename))
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
	return nil
}

// applyFile looks at all settings that are set in the config file and applies
// them.
func (config *DfmConfig) applyFile(file configFile) {
	if file.Repos != nil {
		config.repos = file.Repos
	}
}

// SaveConfig writes a dfm.toml file to the config's path.
func (config *DfmConfig) SaveConfig() error {
	var file configFile
	if config.repos != nil {
		file.Repos = config.repos
	}

	bytes, err := toml.Marshal(file)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(path.Join(config.path, TomlFilename), bytes, 0644)
}
