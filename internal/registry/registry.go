// Package registry manages the repo registry at ~/.wt/repos.json
package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Repo represents a registered git repository
type Repo struct {
	Path           string   `json:"path"`                      // Absolute path to repo
	Name           string   `json:"name"`                      // Display name
	WorktreeFormat string   `json:"worktree_format,omitempty"` // Optional per-repo override
	Labels         []string `json:"labels,omitempty"`          // Labels for grouping
}

// Registry holds all registered repos
type Registry struct {
	Repos []Repo `json:"repos"`
}

// wtDir returns the path to ~/.wt/, creating it if needed
func wtDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}
	dir := filepath.Join(home, ".wt")

	// Auto-create directory if it doesn't exist
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create ~/.wt directory: %w", err)
	}

	return dir, nil
}

// registryPath returns the path to ~/.wt/repos.json
func registryPath() (string, error) {
	dir, err := wtDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "repos.json"), nil
}

// Load reads the registry from ~/.wt/repos.json
// Returns empty registry if file doesn't exist (auto-creates ~/.wt/)
func Load() (*Registry, error) {
	path, err := registryPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Registry{Repos: []Repo{}}, nil
		}
		return nil, fmt.Errorf("read registry: %w", err)
	}

	var reg Registry
	if err := json.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("parse registry: %w", err)
	}

	return &reg, nil
}

// Save writes the registry to ~/.wt/repos.json atomically
func (r *Registry) Save() error {
	path, err := registryPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal registry: %w", err)
	}

	// Write to temp file first for atomic operation
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("write registry: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp) // Clean up temp file on failure
		return fmt.Errorf("save registry: %w", err)
	}

	return nil
}

// Add registers a new repo. Returns error if path already registered.
func (r *Registry) Add(repo Repo) error {
	// Normalize path
	absPath, err := filepath.Abs(repo.Path)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}
	repo.Path = absPath

	// Check for duplicate path
	for _, existing := range r.Repos {
		if existing.Path == repo.Path {
			return fmt.Errorf("repo already registered: %s", repo.Path)
		}
	}

	// Check for duplicate name
	for _, existing := range r.Repos {
		if existing.Name == repo.Name {
			return fmt.Errorf("repo name already exists: %s (use different name or labels to disambiguate)", repo.Name)
		}
	}

	r.Repos = append(r.Repos, repo)
	return nil
}

// Remove unregisters a repo by name or path
func (r *Registry) Remove(nameOrPath string) error {
	for i, repo := range r.Repos {
		if repo.Name == nameOrPath || repo.Path == nameOrPath {
			r.Repos = slices.Delete(r.Repos, i, i+1)
			return nil
		}
	}
	return fmt.Errorf("repo not found: %s", nameOrPath)
}

// Find looks up a repo by name or path
func (r *Registry) Find(ref string) (*Repo, error) {
	var matches []*Repo

	for i := range r.Repos {
		repo := &r.Repos[i]
		if repo.Name == ref || repo.Path == ref {
			return repo, nil
		}
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("repo not found: %s", ref)
	}

	return matches[0], nil
}

// FindByName looks up a repo by name only
func (r *Registry) FindByName(name string) (*Repo, error) {
	for i := range r.Repos {
		if r.Repos[i].Name == name {
			return &r.Repos[i], nil
		}
	}
	return nil, fmt.Errorf("repo not found: %s", name)
}

// FindByPath looks up a repo by path
func (r *Registry) FindByPath(path string) (*Repo, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	for i := range r.Repos {
		if r.Repos[i].Path == absPath {
			return &r.Repos[i], nil
		}
	}
	return nil, fmt.Errorf("repo not registered: %s", path)
}

// FindByLabel returns all repos with the given label
func (r *Registry) FindByLabel(label string) []*Repo {
	var matches []*Repo
	for i := range r.Repos {
		for _, l := range r.Repos[i].Labels {
			if l == label {
				matches = append(matches, &r.Repos[i])
				break
			}
		}
	}
	return matches
}

// FindByLabels returns repos matching any of the given labels
func (r *Registry) FindByLabels(labels []string) []*Repo {
	labelSet := make(map[string]bool)
	for _, l := range labels {
		labelSet[l] = true
	}

	var matches []*Repo
	for i := range r.Repos {
		for _, l := range r.Repos[i].Labels {
			if labelSet[l] {
				matches = append(matches, &r.Repos[i])
				break
			}
		}
	}
	return matches
}

// AllLabels returns all unique labels across all repos
func (r *Registry) AllLabels() []string {
	labelSet := make(map[string]bool)
	for _, repo := range r.Repos {
		for _, l := range repo.Labels {
			labelSet[l] = true
		}
	}

	var labels []string
	for l := range labelSet {
		labels = append(labels, l)
	}
	slices.Sort(labels)
	return labels
}

// AllRepoNames returns all repo names
func (r *Registry) AllRepoNames() []string {
	names := make([]string, len(r.Repos))
	for i, repo := range r.Repos {
		names[i] = repo.Name
	}
	slices.Sort(names)
	return names
}

// AddLabel adds a label to a repo
func (r *Registry) AddLabel(repoName, label string) error {
	repo, err := r.FindByName(repoName)
	if err != nil {
		return err
	}

	// Check if label already exists
	for _, l := range repo.Labels {
		if l == label {
			return nil // Already has label
		}
	}

	repo.Labels = append(repo.Labels, label)
	slices.Sort(repo.Labels)
	return nil
}

// RemoveLabel removes a label from a repo
func (r *Registry) RemoveLabel(repoName, label string) error {
	repo, err := r.FindByName(repoName)
	if err != nil {
		return err
	}

	for i, l := range repo.Labels {
		if l == label {
			repo.Labels = slices.Delete(repo.Labels, i, i+1)
			return nil
		}
	}
	return nil // Label wasn't present, that's fine
}

// ClearLabels removes all labels from a repo
func (r *Registry) ClearLabels(repoName string) error {
	repo, err := r.FindByName(repoName)
	if err != nil {
		return err
	}
	repo.Labels = nil
	return nil
}

// Update updates a repo's configuration
func (r *Registry) Update(name string, fn func(*Repo)) error {
	repo, err := r.FindByName(name)
	if err != nil {
		return err
	}
	fn(repo)
	return nil
}

// HasLabel checks if a repo has a specific label
func (repo *Repo) HasLabel(label string) bool {
	for _, l := range repo.Labels {
		if l == label {
			return true
		}
	}
	return false
}

// MatchesLabels checks if repo has any of the given labels
func (repo *Repo) MatchesLabels(labels []string) bool {
	for _, label := range labels {
		if repo.HasLabel(label) {
			return true
		}
	}
	return false
}

// GetEffectiveWorktreeFormat returns the worktree format to use for this repo
func (repo *Repo) GetEffectiveWorktreeFormat(defaultFormat string) string {
	if repo.WorktreeFormat != "" {
		return repo.WorktreeFormat
	}
	return defaultFormat
}

// String returns a display string for the repo
func (repo *Repo) String() string {
	if len(repo.Labels) > 0 {
		return fmt.Sprintf("%s (%s)", repo.Name, strings.Join(repo.Labels, ", "))
	}
	return repo.Name
}
