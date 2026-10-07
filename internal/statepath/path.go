// Package statepath derives wt configuration and state file locations.
// Resolving a path does not create directories or access the state files.
package statepath

import (
	"os"
	"path/filepath"
)

const (
	directoryName = ".wt"
	registryName  = "repos.json"
	historyName   = "history.json"
	prCacheName   = "prs.json"
	configName    = "config.toml"
)

// Dir returns the registry override's directory, or ~/.wt when unset.
func Dir(registryOverride string) (string, error) {
	if registryOverride != "" {
		return filepath.Dir(registryOverride), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, directoryName), nil
}

// Registry returns the override verbatim, or ~/.wt/repos.json when unset.
func Registry(override string) (string, error) {
	if override != "" {
		return override, nil
	}
	return inDir("", registryName)
}

// History returns the override verbatim, or ~/.wt/history.json when unset.
// History is independent of the registry override.
func History(override string) (string, error) {
	if override != "" {
		return override, nil
	}
	return inDir("", historyName)
}

// PRCache returns prs.json in the effective registry directory.
func PRCache(registryOverride string) (string, error) {
	return inDir(registryOverride, prCacheName)
}

// Config returns config.toml in the effective registry directory.
// Global config loading and initialization pass an empty override.
func Config(registryOverride string) (string, error) {
	return inDir(registryOverride, configName)
}

func inDir(registryOverride, name string) (string, error) {
	dir, err := Dir(registryOverride)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name), nil
}
