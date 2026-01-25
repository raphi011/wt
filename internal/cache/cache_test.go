package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPRInfo_IsStale(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cachedAt time.Time
		want     bool
	}{
		{
			name:     "zero time is stale",
			cachedAt: time.Time{},
			want:     true,
		},
		{
			name:     "recent cache is not stale",
			cachedAt: time.Now().Add(-1 * time.Hour),
			want:     false,
		},
		{
			name:     "old cache is stale",
			cachedAt: time.Now().Add(-25 * time.Hour),
			want:     true,
		},
		{
			name:     "just under max age",
			cachedAt: time.Now().Add(-CacheMaxAge + time.Minute),
			want:     false,
		},
		{
			name:     "just past max age",
			cachedAt: time.Now().Add(-CacheMaxAge - time.Second),
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			pr := &PRInfo{CachedAt: tt.cachedAt}
			if got := pr.IsStale(); got != tt.want {
				t.Errorf("IsStale() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMakeWorktreeKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "simple path",
			path: "/home/user/worktrees/repo-feature",
			want: "repo-feature",
		},
		{
			name: "nested path",
			path: "/a/b/c/d/my-worktree",
			want: "my-worktree",
		},
		{
			name: "just folder name",
			path: "repo-branch",
			want: "repo-branch",
		},
		{
			name: "trailing slash",
			path: "/home/user/worktrees/repo-feature/",
			want: "repo-feature",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := MakeWorktreeKey(tt.path); got != tt.want {
				t.Errorf("MakeWorktreeKey(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestCachePath(t *testing.T) {
	t.Parallel()

	got := CachePath("/home/user/worktrees")
	want := "/home/user/worktrees/.wt-cache.json"
	if got != want {
		t.Errorf("CachePath() = %q, want %q", got, want)
	}
}

func TestLockPath(t *testing.T) {
	t.Parallel()

	got := LockPath("/home/user/worktrees")
	want := "/home/user/worktrees/.wt-cache.lock"
	if got != want {
		t.Errorf("LockPath() = %q, want %q", got, want)
	}
}

func TestLoad_NonExistent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cache, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// New cache initializes PRs map (will be set to nil only when loading existing cache)
	if cache.PRs == nil {
		t.Error("expected PRs to be initialized for new cache")
	}
	if cache.Worktrees == nil {
		t.Error("expected Worktrees to be initialized")
	}
	if cache.NextID != 1 {
		t.Errorf("expected NextID = 1, got %d", cache.NextID)
	}
}

func TestLoad_NewFormat(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cachePath := filepath.Join(dir, ".wt-cache.json")

	// Write cache in new format
	data := `{
		"worktrees": {
			"repo-feature": {
				"id": 5,
				"path": "/home/user/worktrees/repo-feature",
				"branch": "feature",
				"origin_url": "git@github.com:user/repo.git"
			}
		},
		"next_id": 6
	}`
	if err := os.WriteFile(cachePath, []byte(data), 0600); err != nil {
		t.Fatalf("failed to write cache file: %v", err)
	}

	cache, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cache.NextID != 6 {
		t.Errorf("expected NextID = 6, got %d", cache.NextID)
	}
	if len(cache.Worktrees) != 1 {
		t.Errorf("expected 1 worktree, got %d", len(cache.Worktrees))
	}
	entry := cache.Worktrees["repo-feature"]
	if entry == nil {
		t.Fatal("expected repo-feature entry")
	}
	if entry.ID != 5 {
		t.Errorf("expected ID = 5, got %d", entry.ID)
	}
	if entry.Branch != "feature" {
		t.Errorf("expected Branch = 'feature', got %q", entry.Branch)
	}
}

func TestLoad_OldFormat(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cachePath := filepath.Join(dir, ".wt-cache.json")

	// Write cache in old format (just PRCache)
	data := `{
		"git@github.com:user/repo.git": {
			"main": {
				"number": 1,
				"state": "OPEN",
				"url": "https://github.com/user/repo/pull/1"
			}
		}
	}`
	if err := os.WriteFile(cachePath, []byte(data), 0600); err != nil {
		t.Fatalf("failed to write cache file: %v", err)
	}

	cache, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Old format should be migrated but PRs cleared
	if cache.PRs != nil {
		t.Error("expected PRs to be nil after migration")
	}
	if cache.Worktrees == nil {
		t.Error("expected Worktrees to be initialized")
	}
	if cache.NextID != 1 {
		t.Errorf("expected NextID = 1, got %d", cache.NextID)
	}
}

func TestLoad_Corrupted(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cachePath := filepath.Join(dir, ".wt-cache.json")

	// Write corrupted data
	if err := os.WriteFile(cachePath, []byte("not valid json{{{"), 0600); err != nil {
		t.Fatalf("failed to write cache file: %v", err)
	}

	cache, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Corrupted cache should start fresh
	if cache.Worktrees == nil {
		t.Error("expected Worktrees to be initialized")
	}
	if cache.NextID != 1 {
		t.Errorf("expected NextID = 1, got %d", cache.NextID)
	}
}

func TestLoad_NilMaps(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cachePath := filepath.Join(dir, ".wt-cache.json")

	// Write cache with null maps
	data := `{"worktrees": null, "next_id": 0}`
	if err := os.WriteFile(cachePath, []byte(data), 0600); err != nil {
		t.Fatalf("failed to write cache file: %v", err)
	}

	cache, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cache.Worktrees == nil {
		t.Error("expected Worktrees to be initialized")
	}
	if cache.NextID != 1 {
		t.Errorf("expected NextID = 1 (corrected from 0), got %d", cache.NextID)
	}
}

func TestSave(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cache := &Cache{
		Worktrees: map[string]*WorktreeIDEntry{
			"repo-feature": {
				ID:        1,
				Path:      "/home/user/worktrees/repo-feature",
				Branch:    "feature",
				OriginURL: "git@github.com:user/repo.git",
			},
		},
		NextID: 2,
	}

	if err := Save(dir, cache); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify file exists and is valid JSON
	cachePath := filepath.Join(dir, ".wt-cache.json")
	data, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("failed to read cache file: %v", err)
	}

	var loaded Cache
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("failed to parse saved cache: %v", err)
	}

	if loaded.NextID != 2 {
		t.Errorf("expected NextID = 2, got %d", loaded.NextID)
	}
	if len(loaded.Worktrees) != 1 {
		t.Errorf("expected 1 worktree, got %d", len(loaded.Worktrees))
	}
}

func TestSave_Atomic(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cachePath := filepath.Join(dir, ".wt-cache.json")

	// Write initial cache
	initial := &Cache{
		Worktrees: map[string]*WorktreeIDEntry{
			"old-entry": {ID: 1, Path: "/old"},
		},
		NextID: 2,
	}
	if err := Save(dir, initial); err != nil {
		t.Fatalf("Save() initial error = %v", err)
	}

	// Overwrite with new cache
	updated := &Cache{
		Worktrees: map[string]*WorktreeIDEntry{
			"new-entry": {ID: 5, Path: "/new"},
		},
		NextID: 6,
	}
	if err := Save(dir, updated); err != nil {
		t.Fatalf("Save() updated error = %v", err)
	}

	// Verify temp file was cleaned up
	tmpPath := cachePath + ".tmp"
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Error("temp file should not exist after save")
	}

	// Verify updated content
	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.NextID != 6 {
		t.Errorf("expected NextID = 6, got %d", loaded.NextID)
	}
	if _, ok := loaded.Worktrees["new-entry"]; !ok {
		t.Error("expected new-entry in worktrees")
	}
}

func TestLoadWithLock(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Write initial cache
	initial := &Cache{
		Worktrees: map[string]*WorktreeIDEntry{
			"test-entry": {ID: 1, Path: "/test"},
		},
		NextID: 2,
	}
	if err := Save(dir, initial); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	cache, unlock, err := LoadWithLock(dir)
	if err != nil {
		t.Fatalf("LoadWithLock() error = %v", err)
	}
	defer unlock()

	if cache.NextID != 2 {
		t.Errorf("expected NextID = 2, got %d", cache.NextID)
	}

	// Lock file should exist
	lockPath := filepath.Join(dir, ".wt-cache.lock")
	if _, err := os.Stat(lockPath); os.IsNotExist(err) {
		t.Error("lock file should exist while locked")
	}
}

func TestCache_GetOrAssignID(t *testing.T) {
	t.Parallel()

	cache := &Cache{
		Worktrees: make(map[string]*WorktreeIDEntry),
		NextID:    1,
	}

	// First assignment
	info1 := WorktreeInfo{
		Path:      "/worktrees/repo-feature",
		RepoPath:  "/repos/repo",
		Branch:    "feature",
		OriginURL: "git@github.com:user/repo.git",
	}
	id1 := cache.GetOrAssignID(info1)
	if id1 != 1 {
		t.Errorf("expected first ID = 1, got %d", id1)
	}
	if cache.NextID != 2 {
		t.Errorf("expected NextID = 2, got %d", cache.NextID)
	}

	// Second assignment (different worktree)
	info2 := WorktreeInfo{
		Path:      "/worktrees/repo-bugfix",
		RepoPath:  "/repos/repo",
		Branch:    "bugfix",
		OriginURL: "git@github.com:user/repo.git",
	}
	id2 := cache.GetOrAssignID(info2)
	if id2 != 2 {
		t.Errorf("expected second ID = 2, got %d", id2)
	}

	// Same worktree should return same ID
	id1again := cache.GetOrAssignID(info1)
	if id1again != 1 {
		t.Errorf("expected same ID = 1, got %d", id1again)
	}

	// Verify metadata is updated
	entry := cache.Worktrees["repo-feature"]
	if entry.Branch != "feature" {
		t.Errorf("expected Branch = 'feature', got %q", entry.Branch)
	}
}

func TestCache_GetOrAssignID_UpdatesMetadata(t *testing.T) {
	t.Parallel()

	cache := &Cache{
		Worktrees: make(map[string]*WorktreeIDEntry),
		NextID:    1,
	}

	// Initial assignment
	info := WorktreeInfo{
		Path:      "/worktrees/repo-feature",
		Branch:    "feature",
		OriginURL: "git@github.com:user/repo.git",
	}
	cache.GetOrAssignID(info)

	// Update with different metadata
	info.Branch = "feature-v2"
	info.OriginURL = "git@github.com:user/repo-new.git"
	cache.GetOrAssignID(info)

	entry := cache.Worktrees["repo-feature"]
	if entry.Branch != "feature-v2" {
		t.Errorf("expected Branch = 'feature-v2', got %q", entry.Branch)
	}
	if entry.OriginURL != "git@github.com:user/repo-new.git" {
		t.Errorf("expected updated OriginURL, got %q", entry.OriginURL)
	}
}

func TestCache_GetOrAssignID_ClearsRemovedAt(t *testing.T) {
	t.Parallel()

	now := time.Now()
	cache := &Cache{
		Worktrees: map[string]*WorktreeIDEntry{
			"repo-feature": {
				ID:        1,
				Path:      "/worktrees/repo-feature",
				RemovedAt: &now,
			},
		},
		NextID: 2,
	}

	// Re-assign should clear RemovedAt
	info := WorktreeInfo{Path: "/worktrees/repo-feature"}
	cache.GetOrAssignID(info)

	entry := cache.Worktrees["repo-feature"]
	if entry.RemovedAt != nil {
		t.Error("expected RemovedAt to be cleared")
	}
}

func TestCache_GetByID(t *testing.T) {
	t.Parallel()

	now := time.Now()
	cache := &Cache{
		Worktrees: map[string]*WorktreeIDEntry{
			"repo-feature": {
				ID:   1,
				Path: "/worktrees/repo-feature",
			},
			"repo-removed": {
				ID:        2,
				Path:      "/worktrees/repo-removed",
				RemovedAt: &now,
			},
		},
	}

	// Existing entry
	path, found, removed := cache.GetByID(1)
	if !found {
		t.Error("expected to find ID 1")
	}
	if removed {
		t.Error("expected ID 1 not to be removed")
	}
	if path != "/worktrees/repo-feature" {
		t.Errorf("expected path = '/worktrees/repo-feature', got %q", path)
	}

	// Removed entry
	path, found, removed = cache.GetByID(2)
	if !found {
		t.Error("expected to find ID 2")
	}
	if !removed {
		t.Error("expected ID 2 to be removed")
	}

	// Non-existent entry
	_, found, _ = cache.GetByID(999)
	if found {
		t.Error("expected not to find ID 999")
	}
}

func TestCache_GetBranchByID(t *testing.T) {
	t.Parallel()

	now := time.Now()
	cache := &Cache{
		Worktrees: map[string]*WorktreeIDEntry{
			"repo-feature": {
				ID:     1,
				Path:   "/worktrees/repo-feature",
				Branch: "feature",
			},
			"repo-removed": {
				ID:        2,
				Path:      "/worktrees/repo-removed",
				Branch:    "removed-branch",
				RemovedAt: &now,
			},
		},
	}

	// Existing entry
	branch, path, found, removed := cache.GetBranchByID(1)
	if !found {
		t.Error("expected to find ID 1")
	}
	if removed {
		t.Error("expected ID 1 not to be removed")
	}
	if branch != "feature" {
		t.Errorf("expected branch = 'feature', got %q", branch)
	}
	if path != "/worktrees/repo-feature" {
		t.Errorf("expected path = '/worktrees/repo-feature', got %q", path)
	}

	// Removed entry
	branch, _, found, removed = cache.GetBranchByID(2)
	if !found {
		t.Error("expected to find ID 2")
	}
	if !removed {
		t.Error("expected ID 2 to be removed")
	}
	if branch != "removed-branch" {
		t.Errorf("expected branch = 'removed-branch', got %q", branch)
	}

	// Non-existent entry
	_, _, found, _ = cache.GetBranchByID(999)
	if found {
		t.Error("expected not to find ID 999")
	}
}

func TestCache_GetBranchByPRNumber(t *testing.T) {
	t.Parallel()

	cache := &Cache{
		Worktrees: map[string]*WorktreeIDEntry{
			"repo-feature": {
				ID:        1,
				Path:      "/worktrees/repo-feature",
				Branch:    "feature",
				OriginURL: "git@github.com:user/repo.git",
				PR: &PRInfo{
					Number:   42,
					CachedAt: time.Now(),
				},
			},
			"repo-stale": {
				ID:        2,
				Path:      "/worktrees/repo-stale",
				Branch:    "stale-branch",
				OriginURL: "git@github.com:user/repo.git",
				PR: &PRInfo{
					Number:   99,
					CachedAt: time.Now().Add(-48 * time.Hour), // stale
				},
			},
		},
	}

	// Valid PR lookup
	branch := cache.GetBranchByPRNumber("git@github.com:user/repo.git", 42)
	if branch != "feature" {
		t.Errorf("expected branch = 'feature', got %q", branch)
	}

	// Wrong origin URL
	branch = cache.GetBranchByPRNumber("git@github.com:other/repo.git", 42)
	if branch != "" {
		t.Errorf("expected empty branch for wrong origin, got %q", branch)
	}

	// Stale PR should not be returned
	branch = cache.GetBranchByPRNumber("git@github.com:user/repo.git", 99)
	if branch != "" {
		t.Errorf("expected empty branch for stale PR, got %q", branch)
	}

	// Non-existent PR
	branch = cache.GetBranchByPRNumber("git@github.com:user/repo.git", 123)
	if branch != "" {
		t.Errorf("expected empty branch for non-existent PR, got %q", branch)
	}
}

func TestCache_GetPRForBranch(t *testing.T) {
	t.Parallel()

	pr := &PRInfo{Number: 42, State: "OPEN"}
	cache := &Cache{
		Worktrees: map[string]*WorktreeIDEntry{
			"repo-feature": {
				ID: 1,
				PR: pr,
			},
			"repo-no-pr": {
				ID: 2,
			},
		},
	}

	// Has PR
	got := cache.GetPRForBranch("repo-feature")
	if got != pr {
		t.Errorf("expected PR, got %v", got)
	}

	// No PR
	got = cache.GetPRForBranch("repo-no-pr")
	if got != nil {
		t.Errorf("expected nil PR, got %v", got)
	}

	// Non-existent entry
	got = cache.GetPRForBranch("non-existent")
	if got != nil {
		t.Errorf("expected nil PR for non-existent entry, got %v", got)
	}
}

func TestCache_GetPRByOriginAndBranch(t *testing.T) {
	t.Parallel()

	pr := &PRInfo{Number: 42, State: "OPEN"}
	cache := &Cache{
		Worktrees: map[string]*WorktreeIDEntry{
			"repo-feature": {
				ID:        1,
				Branch:    "feature",
				OriginURL: "git@github.com:user/repo.git",
				PR:        pr,
			},
		},
	}

	// Found
	got := cache.GetPRByOriginAndBranch("git@github.com:user/repo.git", "feature")
	if got != pr {
		t.Errorf("expected PR, got %v", got)
	}

	// Wrong origin
	got = cache.GetPRByOriginAndBranch("git@github.com:other/repo.git", "feature")
	if got != nil {
		t.Errorf("expected nil PR for wrong origin, got %v", got)
	}

	// Wrong branch
	got = cache.GetPRByOriginAndBranch("git@github.com:user/repo.git", "other-branch")
	if got != nil {
		t.Errorf("expected nil PR for wrong branch, got %v", got)
	}
}

func TestCache_SetPRForBranch(t *testing.T) {
	t.Parallel()

	cache := &Cache{
		Worktrees: map[string]*WorktreeIDEntry{
			"repo-feature": {ID: 1},
		},
	}

	pr := &PRInfo{Number: 42, State: "OPEN"}
	cache.SetPRForBranch("repo-feature", pr)

	got := cache.Worktrees["repo-feature"].PR
	if got != pr {
		t.Errorf("expected PR to be set, got %v", got)
	}

	// Setting on non-existent entry should be a no-op
	cache.SetPRForBranch("non-existent", pr)
	// No panic = success
}

func TestCache_SetPRByOriginAndBranch(t *testing.T) {
	t.Parallel()

	cache := &Cache{
		Worktrees: map[string]*WorktreeIDEntry{
			"repo-feature": {
				ID:        1,
				Branch:    "feature",
				OriginURL: "git@github.com:user/repo.git",
			},
		},
	}

	pr := &PRInfo{Number: 42, State: "OPEN"}
	cache.SetPRByOriginAndBranch("git@github.com:user/repo.git", "feature", pr)

	got := cache.Worktrees["repo-feature"].PR
	if got != pr {
		t.Errorf("expected PR to be set, got %v", got)
	}

	// Setting on non-matching origin/branch should be a no-op
	otherPR := &PRInfo{Number: 99}
	cache.SetPRByOriginAndBranch("git@github.com:other/repo.git", "feature", otherPR)
	if cache.Worktrees["repo-feature"].PR == otherPR {
		t.Error("PR should not be changed for non-matching origin")
	}
}

func TestCache_MarkRemoved(t *testing.T) {
	t.Parallel()

	cache := &Cache{
		Worktrees: map[string]*WorktreeIDEntry{
			"repo-feature": {ID: 1},
		},
	}

	// Verify not removed initially
	if cache.Worktrees["repo-feature"].RemovedAt != nil {
		t.Error("expected RemovedAt to be nil initially")
	}

	cache.MarkRemoved("repo-feature")

	if cache.Worktrees["repo-feature"].RemovedAt == nil {
		t.Error("expected RemovedAt to be set")
	}

	// Marking non-existent entry should be a no-op
	cache.MarkRemoved("non-existent")
	// No panic = success
}

func TestMarkRemovedByKey(t *testing.T) {
	t.Parallel()

	cache := &Cache{
		Worktrees: map[string]*WorktreeIDEntry{
			"repo-feature": {ID: 1},
		},
	}

	MarkRemovedByKey(cache, "repo-feature")

	if cache.Worktrees["repo-feature"].RemovedAt == nil {
		t.Error("expected RemovedAt to be set")
	}

	// Marking non-existent entry should be a no-op
	MarkRemovedByKey(cache, "non-existent")
	// No panic = success
}

func TestCache_SyncWorktrees(t *testing.T) {
	t.Parallel()

	cache := &Cache{
		Worktrees: map[string]*WorktreeIDEntry{
			"repo-existing": {
				ID:   1,
				Path: "/worktrees/repo-existing",
			},
			"repo-will-be-removed": {
				ID:   2,
				Path: "/worktrees/repo-will-be-removed",
			},
		},
		NextID: 3,
	}

	worktrees := []WorktreeInfo{
		{Path: "/worktrees/repo-existing", Branch: "existing"},
		{Path: "/worktrees/repo-new", Branch: "new"},
	}

	pathToID := cache.SyncWorktrees(worktrees)

	// Check returned map
	if pathToID["/worktrees/repo-existing"] != 1 {
		t.Errorf("expected existing worktree to have ID 1, got %d", pathToID["/worktrees/repo-existing"])
	}
	if pathToID["/worktrees/repo-new"] != 3 {
		t.Errorf("expected new worktree to have ID 3, got %d", pathToID["/worktrees/repo-new"])
	}

	// Check existing entry updated
	if cache.Worktrees["repo-existing"].Branch != "existing" {
		t.Error("expected existing entry to be updated")
	}

	// Check removed entry is marked
	if cache.Worktrees["repo-will-be-removed"].RemovedAt == nil {
		t.Error("expected removed entry to have RemovedAt set")
	}

	// Check new entry created
	if cache.Worktrees["repo-new"] == nil {
		t.Error("expected new entry to be created")
	}
	if cache.Worktrees["repo-new"].ID != 3 {
		t.Errorf("expected new entry to have ID 3, got %d", cache.Worktrees["repo-new"].ID)
	}

	// Check NextID incremented
	if cache.NextID != 4 {
		t.Errorf("expected NextID = 4, got %d", cache.NextID)
	}
}

func TestCache_SyncWorktrees_AlreadyRemoved(t *testing.T) {
	t.Parallel()

	removedTime := time.Now().Add(-1 * time.Hour)
	cache := &Cache{
		Worktrees: map[string]*WorktreeIDEntry{
			"repo-already-removed": {
				ID:        1,
				Path:      "/worktrees/repo-already-removed",
				RemovedAt: &removedTime,
			},
		},
		NextID: 2,
	}

	// Sync with empty list
	cache.SyncWorktrees([]WorktreeInfo{})

	// Should not update RemovedAt if already set
	if cache.Worktrees["repo-already-removed"].RemovedAt != &removedTime {
		t.Error("expected RemovedAt to remain unchanged")
	}
}

func TestCache_Reset(t *testing.T) {
	t.Parallel()

	cache := &Cache{
		PRs: PRCache{
			"origin": {"branch": &PRInfo{Number: 1}},
		},
		Worktrees: map[string]*WorktreeIDEntry{
			"repo-feature": {ID: 5},
		},
		NextID: 10,
	}

	cache.Reset()

	if cache.PRs != nil {
		t.Error("expected PRs to be nil after reset")
	}
	if len(cache.Worktrees) != 0 {
		t.Errorf("expected Worktrees to be empty, got %d entries", len(cache.Worktrees))
	}
	if cache.NextID != 1 {
		t.Errorf("expected NextID = 1, got %d", cache.NextID)
	}
}
