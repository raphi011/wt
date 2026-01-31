package registry

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRepoHasLabel(t *testing.T) {
	t.Parallel()

	repo := Repo{
		Name:   "test",
		Path:   "/test",
		Labels: []string{"foo", "bar"},
	}

	tests := []struct {
		label string
		want  bool
	}{
		{"foo", true},
		{"bar", true},
		{"baz", false},
		{"", false},
	}

	for _, tt := range tests {
		if got := repo.HasLabel(tt.label); got != tt.want {
			t.Errorf("HasLabel(%q) = %v, want %v", tt.label, got, tt.want)
		}
	}
}

func TestRepoMatchesLabels(t *testing.T) {
	t.Parallel()

	repo := Repo{
		Name:   "test",
		Path:   "/test",
		Labels: []string{"foo", "bar"},
	}

	tests := []struct {
		labels []string
		want   bool
	}{
		{[]string{"foo"}, true},
		{[]string{"bar"}, true},
		{[]string{"baz"}, false},
		{[]string{"foo", "baz"}, true},
		{[]string{}, false},
	}

	for _, tt := range tests {
		if got := repo.MatchesLabels(tt.labels); got != tt.want {
			t.Errorf("MatchesLabels(%v) = %v, want %v", tt.labels, got, tt.want)
		}
	}
}

func TestRegistryAddRemove(t *testing.T) {
	t.Parallel()

	reg := &Registry{Repos: []Repo{}}

	// Add repo
	repo := Repo{
		Name: "test",
		Path: "/tmp/test",
	}

	if err := reg.Add(repo); err != nil {
		t.Fatalf("Add() failed: %v", err)
	}

	if len(reg.Repos) != 1 {
		t.Errorf("expected 1 repo, got %d", len(reg.Repos))
	}

	// Try to add duplicate path
	if err := reg.Add(repo); err == nil {
		t.Error("expected error adding duplicate path")
	}

	// Try to add duplicate name
	repo2 := Repo{
		Name: "test",
		Path: "/tmp/test2",
	}
	if err := reg.Add(repo2); err == nil {
		t.Error("expected error adding duplicate name")
	}

	// Remove repo
	if err := reg.Remove("test"); err != nil {
		t.Fatalf("Remove() failed: %v", err)
	}

	if len(reg.Repos) != 0 {
		t.Errorf("expected 0 repos, got %d", len(reg.Repos))
	}

	// Try to remove non-existent
	if err := reg.Remove("nonexistent"); err == nil {
		t.Error("expected error removing non-existent repo")
	}
}

func TestRegistryFind(t *testing.T) {
	t.Parallel()

	reg := &Registry{
		Repos: []Repo{
			{Name: "foo", Path: "/tmp/foo"},
			{Name: "bar", Path: "/tmp/bar", Labels: []string{"backend"}},
		},
	}

	// Find by name
	repo, err := reg.FindByName("foo")
	if err != nil {
		t.Fatalf("FindByName() failed: %v", err)
	}
	if repo.Name != "foo" {
		t.Errorf("expected foo, got %s", repo.Name)
	}

	// Find non-existent
	_, err = reg.FindByName("baz")
	if err == nil {
		t.Error("expected error for non-existent repo")
	}

	// Find by label
	repos := reg.FindByLabel("backend")
	if len(repos) != 1 {
		t.Errorf("expected 1 repo with label, got %d", len(repos))
	}
	if repos[0].Name != "bar" {
		t.Errorf("expected bar, got %s", repos[0].Name)
	}
}

func TestRegistryLabels(t *testing.T) {
	t.Parallel()

	reg := &Registry{
		Repos: []Repo{
			{Name: "foo", Path: "/tmp/foo"},
		},
	}

	// Add label
	if err := reg.AddLabel("foo", "backend"); err != nil {
		t.Fatalf("AddLabel() failed: %v", err)
	}

	repo, _ := reg.FindByName("foo")
	if !repo.HasLabel("backend") {
		t.Error("expected repo to have backend label")
	}

	// Add same label again (should be idempotent)
	if err := reg.AddLabel("foo", "backend"); err != nil {
		t.Fatalf("AddLabel() failed on duplicate: %v", err)
	}

	// Remove label
	if err := reg.RemoveLabel("foo", "backend"); err != nil {
		t.Fatalf("RemoveLabel() failed: %v", err)
	}

	repo, _ = reg.FindByName("foo")
	if repo.HasLabel("backend") {
		t.Error("expected repo to not have backend label")
	}

	// Add multiple labels
	reg.AddLabel("foo", "api")
	reg.AddLabel("foo", "frontend")

	// Clear labels
	if err := reg.ClearLabels("foo"); err != nil {
		t.Fatalf("ClearLabels() failed: %v", err)
	}

	repo, _ = reg.FindByName("foo")
	if len(repo.Labels) != 0 {
		t.Errorf("expected 0 labels, got %d", len(repo.Labels))
	}
}

func TestRegistrySaveLoad(t *testing.T) {
	t.Parallel()

	// Create temp dir for test
	tmpDir, err := os.MkdirTemp("", "wt-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Override home directory for test
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Create registry
	reg := &Registry{
		Repos: []Repo{
			{Name: "foo", Path: "/tmp/foo", Labels: []string{"backend"}},
			{Name: "bar", Path: "/tmp/bar"},
		},
	}

	// Save
	if err := reg.Save(); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Verify file exists
	path := filepath.Join(tmpDir, ".wt", "repos.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("registry file was not created")
	}

	// Load
	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if len(loaded.Repos) != 2 {
		t.Errorf("expected 2 repos, got %d", len(loaded.Repos))
	}

	repo, _ := loaded.FindByName("foo")
	if !repo.HasLabel("backend") {
		t.Error("expected loaded repo to have backend label")
	}
}

func TestAllLabels(t *testing.T) {
	t.Parallel()

	reg := &Registry{
		Repos: []Repo{
			{Name: "foo", Path: "/tmp/foo", Labels: []string{"backend", "api"}},
			{Name: "bar", Path: "/tmp/bar", Labels: []string{"frontend", "api"}},
		},
	}

	labels := reg.AllLabels()
	if len(labels) != 3 {
		t.Errorf("expected 3 unique labels, got %d", len(labels))
	}

	// Labels should be sorted
	expected := []string{"api", "backend", "frontend"}
	for i, l := range expected {
		if labels[i] != l {
			t.Errorf("labels[%d] = %s, want %s", i, labels[i], l)
		}
	}
}

func TestAllRepoNames(t *testing.T) {
	t.Parallel()

	reg := &Registry{
		Repos: []Repo{
			{Name: "zoo", Path: "/tmp/zoo"},
			{Name: "alpha", Path: "/tmp/alpha"},
			{Name: "beta", Path: "/tmp/beta"},
		},
	}

	names := reg.AllRepoNames()
	if len(names) != 3 {
		t.Errorf("expected 3 names, got %d", len(names))
	}

	// Names should be sorted
	expected := []string{"alpha", "beta", "zoo"}
	for i, n := range expected {
		if names[i] != n {
			t.Errorf("names[%d] = %s, want %s", i, names[i], n)
		}
	}
}
