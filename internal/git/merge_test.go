package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseWtMerged(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		value    string
		strategy string
		target   string
		ts       time.Time
	}{
		{
			name:     "squash merge",
			value:    "squash:main@2026-03-28T14:30:00Z",
			strategy: "squash",
			target:   "main",
			ts:       time.Date(2026, 3, 28, 14, 30, 0, 0, time.UTC),
		},
		{
			name:     "regular merge",
			value:    "merge:develop@2026-01-15T09:00:00Z",
			strategy: "merge",
			target:   "develop",
			ts:       time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC),
		},
		{
			name:     "ff merge",
			value:    "ff:main@2026-06-01T00:00:00Z",
			strategy: "ff",
			target:   "main",
			ts:       time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "empty value",
			value:    "",
			strategy: "",
			target:   "",
			ts:       time.Time{},
		},
		{
			name:     "missing timestamp",
			value:    "squash:main",
			strategy: "squash",
			target:   "main",
			ts:       time.Time{},
		},
		{
			name:     "bad timestamp",
			value:    "squash:main@not-a-time",
			strategy: "squash",
			target:   "main",
			ts:       time.Time{},
		},
		{
			name:     "missing target",
			value:    "squash",
			strategy: "",
			target:   "",
			ts:       time.Time{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			strategy, target, ts := ParseWtMerged(tt.value)
			if strategy != tt.strategy {
				t.Errorf("strategy = %q, want %q", strategy, tt.strategy)
			}
			if target != tt.target {
				t.Errorf("target = %q, want %q", target, tt.target)
			}
			if !ts.Equal(tt.ts) {
				t.Errorf("ts = %v, want %v", ts, tt.ts)
			}
		})
	}
}

func TestFormatWtMerged(t *testing.T) {
	t.Parallel()

	ts := time.Date(2026, 3, 28, 14, 30, 0, 0, time.UTC)
	got := FormatWtMerged("squash", "main", ts)
	want := "squash:main@2026-03-28T14:30:00Z"
	if got != want {
		t.Errorf("FormatWtMerged() = %q, want %q", got, want)
	}
}

func TestFormatWtMergedRoundtrip(t *testing.T) {
	t.Parallel()

	ts := time.Date(2026, 3, 28, 14, 30, 0, 0, time.UTC)
	formatted := FormatWtMerged("squash", "main", ts)
	strategy, target, parsedTS := ParseWtMerged(formatted)

	if strategy != "squash" {
		t.Errorf("strategy = %q, want %q", strategy, "squash")
	}
	if target != "main" {
		t.Errorf("target = %q, want %q", target, "main")
	}
	if !parsedTS.Equal(ts) {
		t.Errorf("ts = %v, want %v", parsedTS, ts)
	}
}

func TestDetectMergeStrategy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want string
	}{
		{"no args", nil, "merge"},
		{"empty args", []string{}, "merge"},
		{"squash flag", []string{"--squash"}, "squash"},
		{"ff-only flag", []string{"--ff-only"}, "ff"},
		{"squash with other args", []string{"--no-commit", "--squash"}, "squash"},
		{"ff-only with other args", []string{"--ff-only", "--no-edit"}, "ff"},
		{"unrelated flags", []string{"--no-commit", "--no-edit"}, "merge"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := DetectMergeStrategy(tt.args)
			if got != tt.want {
				t.Errorf("DetectMergeStrategy(%v) = %q, want %q", tt.args, got, tt.want)
			}
		})
	}
}

func TestHasNoCommitFlag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want bool
	}{
		{"no args", nil, false},
		{"empty args", []string{}, false},
		{"has --no-commit", []string{"--no-commit"}, true},
		{"has --no-commit with others", []string{"--squash", "--no-commit"}, true},
		{"no --no-commit", []string{"--squash", "--no-edit"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := HasNoCommitFlag(tt.args)
			if got != tt.want {
				t.Errorf("HasNoCommitFlag(%v) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}

func TestGetSetClearWtMerged(t *testing.T) {
	t.Parallel()

	repoPath := setupTestRepo(t)
	ctx := context.Background()

	branch := "feature-test"
	if err := runGit(ctx, repoPath, "branch", branch); err != nil {
		t.Fatalf("failed to create branch: %v", err)
	}

	// Get on non-existent key returns empty
	val, err := GetWtMerged(ctx, repoPath, branch)
	if err != nil {
		t.Fatalf("GetWtMerged failed: %v", err)
	}
	if val != "" {
		t.Errorf("GetWtMerged = %q, want empty", val)
	}

	// Set
	mergeVal := "squash:main@2026-03-28T14:30:00Z"
	if err := SetWtMerged(ctx, repoPath, branch, mergeVal); err != nil {
		t.Fatalf("SetWtMerged failed: %v", err)
	}

	// Get after set
	val, err = GetWtMerged(ctx, repoPath, branch)
	if err != nil {
		t.Fatalf("GetWtMerged failed: %v", err)
	}
	if val != mergeVal {
		t.Errorf("GetWtMerged = %q, want %q", val, mergeVal)
	}

	// Clear
	if err := ClearWtMerged(ctx, repoPath, branch); err != nil {
		t.Fatalf("ClearWtMerged failed: %v", err)
	}

	// Get after clear
	val, err = GetWtMerged(ctx, repoPath, branch)
	if err != nil {
		t.Fatalf("GetWtMerged failed: %v", err)
	}
	if val != "" {
		t.Errorf("GetWtMerged after clear = %q, want empty", val)
	}

	// Clear on already cleared is not an error
	if err := ClearWtMerged(ctx, repoPath, branch); err != nil {
		t.Fatalf("ClearWtMerged on already cleared failed: %v", err)
	}
}

func TestIsWorktreeClean(t *testing.T) {
	t.Parallel()

	repoPath := setupTestRepo(t)
	ctx := context.Background()

	// Clean repo
	clean, err := IsWorktreeClean(ctx, repoPath)
	if err != nil {
		t.Fatalf("IsWorktreeClean failed: %v", err)
	}
	if !clean {
		t.Error("expected clean worktree")
	}

	// Create untracked file
	if err := os.WriteFile(filepath.Join(repoPath, "dirty.txt"), []byte("dirty"), 0o644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	clean, err = IsWorktreeClean(ctx, repoPath)
	if err != nil {
		t.Fatalf("IsWorktreeClean failed: %v", err)
	}
	if clean {
		t.Error("expected dirty worktree")
	}
}
