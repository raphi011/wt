package forge

import (
	"strings"

	"github.com/raphi011/wt/internal/git"
)

// Detect returns the appropriate Forge implementation based on the remote URL.
// Falls back to GitHub if the platform cannot be determined.
func Detect(remoteURL string) Forge {
	if isGitLab(remoteURL) {
		return &GitLab{}
	}
	// Default to GitHub (most common, backwards compatible)
	return &GitHub{}
}

// DetectFromRepo detects the forge for a repository by reading its origin URL.
// Returns GitHub as default if detection fails.
func DetectFromRepo(repoPath string) Forge {
	url, err := git.GetOriginURL(repoPath)
	if err != nil {
		return &GitHub{}
	}
	return Detect(url)
}

// ByName returns a Forge implementation by name.
// Supported names: "github", "gitlab"
// Returns GitHub as default for unknown names.
func ByName(name string) Forge {
	switch strings.ToLower(name) {
	case "gitlab":
		return &GitLab{}
	case "github":
		return &GitHub{}
	default:
		return &GitHub{}
	}
}

// isGitLab checks if a URL points to a GitLab instance
func isGitLab(url string) bool {
	url = strings.ToLower(url)

	// gitlab.com (SaaS)
	if strings.Contains(url, "gitlab.com") {
		return true
	}

	// Common self-hosted patterns
	if strings.Contains(url, "gitlab.") {
		return true
	}

	// Check for "/gitlab/" in path (some orgs host at company.com/gitlab/)
	if strings.Contains(url, "/gitlab/") {
		return true
	}

	return false
}

// isGitHub checks if a URL points to GitHub
func isGitHub(url string) bool {
	url = strings.ToLower(url)
	return strings.Contains(url, "github.com")
}
