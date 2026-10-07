package statepath_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/raphi011/wt/internal/statepath"
)

func TestDefaultPaths(t *testing.T) {
	t.Parallel()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("get home directory: %v", err)
	}
	for _, tc := range []struct {
		name string
		path func(string) (string, error)
		want string
	}{
		{"directory", statepath.Dir, filepath.Join(home, ".wt")},
		{"registry", statepath.Registry, filepath.Join(home, ".wt", "repos.json")},
		{"history", statepath.History, filepath.Join(home, ".wt", "history.json")},
		{"PR cache", statepath.PRCache, filepath.Join(home, ".wt", "prs.json")},
		{"config", statepath.Config, filepath.Join(home, ".wt", "config.toml")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := tc.path("")
			if err != nil {
				t.Fatalf("derive default path: %v", err)
			}
			if got != tc.want {
				t.Errorf("path = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRegistryOverrides(t *testing.T) {
	t.Parallel()
	for _, override := range []string{
		filepath.Join(t.TempDir(), "custom", "registry.json"),
		"relative/registry.json",
		"./relative/../registry.json",
	} {
		t.Run(override, func(t *testing.T) {
			t.Parallel()
			dir := filepath.Dir(override)
			for _, tc := range []struct {
				name string
				path func(string) (string, error)
				want string
			}{
				{"directory", statepath.Dir, dir},
				{"registry", statepath.Registry, override},
				{"PR cache", statepath.PRCache, filepath.Join(dir, "prs.json")},
				{"config", statepath.Config, filepath.Join(dir, "config.toml")},
			} {
				got, err := tc.path(override)
				if err != nil {
					t.Fatalf("derive %s: %v", tc.name, err)
				}
				if got != tc.want {
					t.Errorf("%s = %q, want %q", tc.name, got, tc.want)
				}
			}
		})
	}
}

func TestDerivationDoesNotCreateDirectories(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		path func(string) (string, error)
	}{
		{"directory", statepath.Dir},
		{"registry", statepath.Registry},
		{"history", statepath.History},
		{"PR cache", statepath.PRCache},
		{"config", statepath.Config},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			missingDir := filepath.Join(t.TempDir(), "missing", "nested")
			override := filepath.Join(missingDir, "override.json")
			if _, err := tc.path(override); err != nil {
				t.Fatalf("derive path: %v", err)
			}
			if _, err := os.Stat(filepath.Dir(missingDir)); !os.IsNotExist(err) {
				t.Errorf("path derivation created a directory or failed to stat: %v", err)
			}
		})
	}
}
