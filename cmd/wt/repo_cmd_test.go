package main

import "testing"

func TestIsGitURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		// Full URLs - should return true
		{"https URL", "https://github.com/org/repo", true},
		{"https URL with .git", "https://github.com/org/repo.git", true},
		{"http URL", "http://github.com/org/repo", true},
		{"git protocol URL", "git://github.com/org/repo", true},
		{"ssh protocol URL", "ssh://git@github.com/org/repo", true},
		{"SSH format", "git@github.com:org/repo.git", true},
		{"SSH format no .git", "git@github.com:org/repo", true},
		{"SSH with alias", "git@github.com-personal:org/repo.git", true},
		{"file URL", "file:///path/to/repo", true},
		{"gitlab SSH", "git@gitlab.com:group/subgroup/repo.git", true},

		// Short-form - should return false
		{"org/repo", "org/repo", false},
		{"org/repo-with-dashes", "org/repo-with-dashes", false},
		{"Org123/Repo456", "Org123/Repo456", false},
		{"just repo name", "myrepo", false},
		{"repo-with-dashes", "my-awesome-repo", false},
		{"repo.with.dots", "my.repo.name", false},
		{"underscore_repo", "my_repo", false},
		{"complex org/repo", "my-org/my-awesome-repo", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := isGitURL(tt.input)
			if result != tt.expected {
				t.Errorf("isGitURL(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExtractRepoNameFromURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{"https URL", "https://github.com/org/repo", "repo"},
		{"https URL with .git", "https://github.com/org/repo.git", "repo"},
		{"SSH format", "git@github.com:org/repo.git", "repo"},
		{"SSH format no .git", "git@github.com:org/repo", "repo"},
		{"gitlab subgroup", "git@gitlab.com:group/subgroup/repo.git", "repo"},
		{"simple path", "repo", "repo"},
		{"with .git suffix", "repo.git", "repo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := extractRepoNameFromURL(tt.url)
			if result != tt.expected {
				t.Errorf("extractRepoNameFromURL(%q) = %q, want %q", tt.url, result, tt.expected)
			}
		})
	}
}
