package forge

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/raphi011/wt/internal/config"
)

func TestNormalizeGitLabState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"opened", PRStateOpen},
		{"merged", PRStateMerged},
		{"closed", PRStateClosed},
		// case insensitivity
		{"OPENED", PRStateOpen},
		{"Merged", PRStateMerged},
		{"Closed", PRStateClosed},
		// unknown state gets uppercased
		{"unknown", "UNKNOWN"},
		{"custom", "CUSTOM"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got := normalizeGitLabState(tt.input)
			if got != tt.want {
				t.Errorf("normalizeGitLabState(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestGitHub_FormatState(t *testing.T) {
	t.Parallel()

	gh := &GitHub{}

	tests := []struct {
		state string
		want  string
	}{
		{PRStateMerged, "merged"},
		{PRStateOpen, "open"},
		{PRStateDraft, "draft"},
		{PRStateClosed, "closed"},
		{"UNKNOWN", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			t.Parallel()
			got := gh.FormatState(tt.state)
			if got != tt.want {
				t.Errorf("GitHub.FormatState(%q) = %q, want %q", tt.state, got, tt.want)
			}
		})
	}
}

func TestGitLab_FormatState(t *testing.T) {
	t.Parallel()

	gl := &GitLab{}

	tests := []struct {
		state string
		want  string
	}{
		{PRStateMerged, "merged"},
		{PRStateOpen, "open"},
		{PRStateDraft, "draft"},
		{PRStateClosed, "closed"},
		{"UNKNOWN", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			t.Parallel()
			got := gl.FormatState(tt.state)
			if got != tt.want {
				t.Errorf("GitLab.FormatState(%q) = %q, want %q", tt.state, got, tt.want)
			}
		})
	}
}

func TestGitLab_MergePR_RebaseStrategy(t *testing.T) {
	t.Parallel()

	gl := &GitLab{}

	err := gl.MergePR(context.Background(), "https://gitlab.com/group/repo", 1, "rebase")
	if err == nil {
		t.Fatal("MergePR() with rebase strategy should return error")
	}

	want := "rebase merge strategy is not supported on GitLab"
	if got := err.Error(); !strings.Contains(got, want) {
		t.Errorf("MergePR() error = %q, want it to contain %q", got, want)
	}
}

func TestGitHub_Name(t *testing.T) {
	t.Parallel()

	gh := &GitHub{}
	if got := gh.Name(); got != "github" {
		t.Errorf("GitHub.Name() = %q, want %q", got, "github")
	}
}

func TestGitLab_Name(t *testing.T) {
	t.Parallel()

	gl := &GitLab{}
	if got := gl.Name(); got != "gitlab" {
		t.Errorf("GitLab.Name() = %q, want %q", got, "gitlab")
	}
}

func TestConfigureBareRepo(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	ctx := context.Background()

	// Create a bare repo
	bareDir := tmpDir + "/bare.git"
	if err := exec.CommandContext(ctx, "git", "init", "--bare", bareDir).Run(); err != nil {
		t.Fatalf("failed to init bare repo: %v", err)
	}

	// Add a remote origin
	if err := exec.CommandContext(ctx, "git", "-C", bareDir, "remote", "add", "origin", "https://github.com/test/repo.git").Run(); err != nil {
		t.Fatalf("failed to add remote: %v", err)
	}

	// Run configureBareRepo
	if err := configureBareRepo(ctx, bareDir); err != nil {
		t.Fatalf("configureBareRepo failed: %v", err)
	}

	// Verify fetch refspec was set
	out, err := exec.CommandContext(ctx, "git", "-C", bareDir, "config", "remote.origin.fetch").Output()
	if err != nil {
		t.Fatalf("failed to read fetch config: %v", err)
	}
	got := strings.TrimSpace(string(out))
	want := "+refs/heads/*:refs/remotes/origin/*"
	if got != want {
		t.Errorf("fetch refspec = %q, want %q", got, want)
	}
}

func TestGitHub_CloneRepo_Validation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		repoSpec string
		wantErr  string
	}{
		{"no slash", "noslash", "invalid repo spec"},
		{"empty org", "/repo", "org and repo must not be empty"},
		{"empty repo", "org/", "org and repo must not be empty"},
		{"empty string", "", "invalid repo spec"},
		{"too many parts", "a/b/c", "invalid repo spec"},
	}

	gh := &GitHub{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := gh.CloneRepo(context.Background(), tt.repoSpec, t.TempDir())
			if err == nil {
				t.Fatalf("CloneRepo(%q) expected error, got nil", tt.repoSpec)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("CloneRepo(%q) error = %q, want it to contain %q", tt.repoSpec, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestGitHub_CloneBareRepo_Validation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		repoSpec string
		wantErr  string
	}{
		{"no slash", "noslash", "invalid repo spec"},
		{"empty org", "/repo", "org and repo must not be empty"},
		{"empty repo", "org/", "org and repo must not be empty"},
		{"empty string", "", "invalid repo spec"},
		{"too many parts", "a/b/c", "invalid repo spec"},
	}

	gh := &GitHub{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := gh.CloneBareRepo(context.Background(), tt.repoSpec, t.TempDir())
			if err == nil {
				t.Fatalf("CloneBareRepo(%q) expected error, got nil", tt.repoSpec)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("CloneBareRepo(%q) error = %q, want it to contain %q", tt.repoSpec, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestGitLab_CloneRepo_Validation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		repoSpec string
		wantErr  string
	}{
		{"no slash", "noslash", "invalid repo spec"},
		{"empty group", "/repo", "group must not be empty"},
		{"empty repo name", "group/", "repo name must not be empty"},
		{"empty string", "", "invalid repo spec"},
	}

	gl := &GitLab{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := gl.CloneRepo(context.Background(), tt.repoSpec, t.TempDir())
			if err == nil {
				t.Fatalf("CloneRepo(%q) expected error, got nil", tt.repoSpec)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("CloneRepo(%q) error = %q, want it to contain %q", tt.repoSpec, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestGitLab_CloneBareRepo_Validation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		repoSpec string
		wantErr  string
	}{
		{"no slash", "noslash", "invalid repo spec"},
		{"empty group", "/repo", "group must not be empty"},
		{"empty repo name", "group/", "repo name must not be empty"},
		{"empty string", "", "invalid repo spec"},
	}

	gl := &GitLab{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := gl.CloneBareRepo(context.Background(), tt.repoSpec, t.TempDir())
			if err == nil {
				t.Fatalf("CloneBareRepo(%q) expected error, got nil", tt.repoSpec)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("CloneBareRepo(%q) error = %q, want it to contain %q", tt.repoSpec, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestGitHub_getUserForRepo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		forgeConfig *config.ForgeConfig
		repoPath    string
		want        string
	}{
		{
			name:        "nil ForgeConfig",
			forgeConfig: nil,
			repoPath:    "org/repo",
			want:        "",
		},
		{
			name: "matching rule",
			forgeConfig: &config.ForgeConfig{
				Rules: []config.ForgeRule{
					{Pattern: "org/*", User: "my-user"},
				},
			},
			repoPath: "org/repo",
			want:     "my-user",
		},
		{
			name: "no matching rule",
			forgeConfig: &config.ForgeConfig{
				Rules: []config.ForgeRule{
					{Pattern: "other/*", User: "other-user"},
				},
			},
			repoPath: "org/repo",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gh := &GitHub{ForgeConfig: tt.forgeConfig}
			got := gh.getUserForRepo(tt.repoPath)
			if got != tt.want {
				t.Errorf("getUserForRepo(%q) = %q, want %q", tt.repoPath, got, tt.want)
			}
		})
	}
}
