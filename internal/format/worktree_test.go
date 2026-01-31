package format

import (
	"strings"
	"testing"
	"time"
)

func TestValidateFormat(t *testing.T) {
	tests := []struct {
		name    string
		format  string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "default format",
			format:  DefaultWorktreeFormat,
			wantErr: false,
		},
		{
			name:    "origin only",
			format:  "{origin}",
			wantErr: false,
		},
		{
			name:    "all placeholders",
			format:  "{repo}_{origin}_{branch}",
			wantErr: false,
		},
		{
			name:    "unknown placeholder",
			format:  "{unknown}-{branch}",
			wantErr: true,
			errMsg:  "unknown placeholder",
		},
		{
			name:    "no placeholders",
			format:  "static-name",
			wantErr: true,
			errMsg:  "must contain at least one placeholder",
		},
		{
			name:    "empty format",
			format:  "",
			wantErr: true,
			errMsg:  "must contain at least one placeholder",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFormat(tt.format)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateFormat(%q) expected error, got nil", tt.format)
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateFormat(%q) error = %q, want containing %q", tt.format, err.Error(), tt.errMsg)
				}
			} else if err != nil {
				t.Errorf("ValidateFormat(%q) unexpected error: %v", tt.format, err)
			}
		})
	}
}

func TestFormatWorktreeName(t *testing.T) {
	tests := []struct {
		name   string
		format string
		params FormatParams
		want   string
	}{
		{
			name:   "default format",
			format: DefaultWorktreeFormat,
			params: FormatParams{
				RepoName:   "wt",
				BranchName: "feature-branch",
				Origin:     "wt",
			},
			want: "wt-feature-branch",
		},
		{
			name:   "origin format",
			format: "{origin}_{branch}",
			params: FormatParams{
				RepoName:   "my-wt",
				BranchName: "feature-branch",
				Origin:     "wt-origin",
			},
			want: "wt-origin_feature-branch",
		},
		{
			name:   "branch with slashes",
			format: "{repo}-{branch}",
			params: FormatParams{
				RepoName:   "repo",
				BranchName: "feature/add-login",
				Origin:     "repo",
			},
			want: "repo-feature-add-login",
		},
		{
			name:   "all placeholders",
			format: "{repo}+{origin}+{branch}",
			params: FormatParams{
				RepoName:   "folder-name",
				BranchName: "my-branch",
				Origin:     "origin-name",
			},
			want: "folder-name+origin-name+my-branch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatWorktreeName(tt.format, tt.params)
			if got != tt.want {
				t.Errorf("FormatWorktreeName(%q, %+v) = %q, want %q", tt.format, tt.params, got, tt.want)
			}
		})
	}
}

func TestSanitizeForPath(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no special chars",
			input: "feature-branch",
			want:  "feature-branch",
		},
		{
			name:  "forward slash",
			input: "feature/add-login",
			want:  "feature-add-login",
		},
		{
			name:  "backslash",
			input: "feature\\branch",
			want:  "feature-branch",
		},
		{
			name:  "colon",
			input: "feature:fix",
			want:  "feature-fix",
		},
		{
			name:  "multiple special chars",
			input: "a/b\\c:d*e?f\"g<h>i|j",
			want:  "a-b-c-d-e-f-g-h-i-j",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeForPath(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeForPath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestUniqueWorktreePath(t *testing.T) {
	tests := []struct {
		name       string
		basePath   string
		existsFunc PathExistsFunc
		want       string
	}{
		{
			name:     "path does not exist",
			basePath: "/worktrees/repo-feature",
			existsFunc: func(path string) bool {
				return false
			},
			want: "/worktrees/repo-feature",
		},
		{
			name:     "path exists, first suffix available",
			basePath: "/worktrees/repo-feature",
			existsFunc: func(path string) bool {
				return path == "/worktrees/repo-feature"
			},
			want: "/worktrees/repo-feature-1",
		},
		{
			name:     "first two suffixes taken",
			basePath: "/worktrees/repo-feature",
			existsFunc: func(path string) bool {
				return path == "/worktrees/repo-feature" ||
					path == "/worktrees/repo-feature-1"
			},
			want: "/worktrees/repo-feature-2",
		},
		{
			name:     "many suffixes taken",
			basePath: "/worktrees/repo-main",
			existsFunc: func(path string) bool {
				existing := map[string]bool{
					"/worktrees/repo-main":   true,
					"/worktrees/repo-main-1": true,
					"/worktrees/repo-main-2": true,
					"/worktrees/repo-main-3": true,
					"/worktrees/repo-main-4": true,
				}
				return existing[path]
			},
			want: "/worktrees/repo-main-5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := UniqueWorktreePath(tt.basePath, tt.existsFunc)
			if got != tt.want {
				t.Errorf("UniqueWorktreePath(%q) = %q, want %q", tt.basePath, got, tt.want)
			}
		})
	}
}

func TestRelativeTimeFrom(t *testing.T) {
	now := time.Date(2026, 1, 31, 14, 30, 0, 0, time.UTC)

	tests := []struct {
		name string
		t    time.Time
		want string
	}{
		{
			name: "just now",
			t:    now.Add(-1 * time.Second),
			want: "just now",
		},
		{
			name: "seconds ago",
			t:    now.Add(-30 * time.Second),
			want: "30s ago",
		},
		{
			name: "minutes ago",
			t:    now.Add(-5 * time.Minute),
			want: "5m ago",
		},
		{
			name: "hours ago",
			t:    now.Add(-3 * time.Hour),
			want: "3h ago",
		},
		{
			name: "yesterday",
			t:    now.Add(-24 * time.Hour),
			want: "yesterday",
		},
		{
			name: "2 days ago",
			t:    now.Add(-48 * time.Hour),
			want: "2d ago",
		},
		{
			name: "6 days ago",
			t:    now.Add(-6 * 24 * time.Hour),
			want: "6d ago",
		},
		{
			name: "week or more shows date",
			t:    now.Add(-7 * 24 * time.Hour),
			want: "2026-01-24",
		},
		{
			name: "old date",
			t:    time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC),
			want: "2025-06-15",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RelativeTimeFrom(tt.t, now)
			if got != tt.want {
				t.Errorf("RelativeTimeFrom() = %q, want %q", got, tt.want)
			}
		})
	}
}
