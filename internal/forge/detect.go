package forge

import (
	"context"
	"net/url"
	"strings"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/git"
)

// Detect returns the appropriate Forge implementation based on the remote URL.
// If hostMap is provided, checks for exact domain matches first.
// Falls back to pattern matching, then defaults to GitHub.
// If forgeConfig is provided, it's passed to the forge for user lookup.
func Detect(remoteURL string, hostMap map[string]string, forgeConfig *config.ForgeConfig) Forge {
	// Check hostMap first for exact domain match
	if len(hostMap) > 0 {
		host := extractHost(remoteURL)
		if forgeType, ok := hostMap[host]; ok {
			return ByNameWithConfig(forgeType, forgeConfig)
		}
	}

	// Fall back to pattern matching
	if isGitLab(remoteURL) {
		return &GitLab{ForgeConfig: forgeConfig}
	}
	// Default to GitHub (most common, backwards compatible)
	return &GitHub{ForgeConfig: forgeConfig}
}

// DetectFromRepo detects the forge for a repository by reading its origin URL.
// Returns GitHub as default if detection fails.
func DetectFromRepo(ctx context.Context, repoPath string, hostMap map[string]string, forgeConfig *config.ForgeConfig) Forge {
	url, err := git.GetOriginURL(ctx, repoPath)
	if err != nil {
		return &GitHub{ForgeConfig: forgeConfig}
	}
	return Detect(url, hostMap, forgeConfig)
}

// extractHost parses the hostname from a git remote URL.
// Handles SSH format (git@host:path) and HTTPS format (https://host/path).
func extractHost(remoteURL string) string {
	// SSH format: git@github.com:user/repo.git
	if strings.HasPrefix(remoteURL, "git@") {
		// Extract host between "git@" and ":"
		withoutPrefix := strings.TrimPrefix(remoteURL, "git@")
		if idx := strings.Index(withoutPrefix, ":"); idx > 0 {
			return withoutPrefix[:idx]
		}
	}

	// HTTPS format: https://github.com/user/repo.git
	if strings.HasPrefix(remoteURL, "http://") || strings.HasPrefix(remoteURL, "https://") {
		if parsed, err := url.Parse(remoteURL); err == nil {
			return parsed.Hostname()
		}
	}

	// SSH format with explicit ssh:// protocol: ssh://git@github.com/user/repo.git
	if strings.HasPrefix(remoteURL, "ssh://") {
		if parsed, err := url.Parse(remoteURL); err == nil {
			return parsed.Hostname()
		}
	}

	return ""
}

// ByName returns a Forge implementation by name.
// Supported names: "github", "gitlab"
// Returns GitHub as default for unknown names.
func ByName(name string) Forge {
	return ByNameWithConfig(name, nil)
}

// ByNameWithConfig returns a Forge implementation by name with config.
// Supported names: "github", "gitlab"
// Returns GitHub as default for unknown names.
func ByNameWithConfig(name string, forgeConfig *config.ForgeConfig) Forge {
	switch strings.ToLower(name) {
	case "gitlab":
		return &GitLab{ForgeConfig: forgeConfig}
	case "github":
		return &GitHub{ForgeConfig: forgeConfig}
	default:
		return &GitHub{ForgeConfig: forgeConfig}
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

// extractRepoPath extracts owner/repo from a git URL.
// Handles SSH aliases: git@github.com-personal:user/repo.git -> user/repo
// Handles SSH: git@github.com:user/repo.git -> user/repo
// Handles HTTPS: https://github.com/user/repo.git -> user/repo
// Handles SSH protocol: ssh://git@github.com/user/repo.git -> user/repo
// Handles GitLab subgroups: git@gitlab.com:group/sub/repo.git -> group/sub/repo
func extractRepoPath(url string) string {
	url = strings.TrimSuffix(url, ".git")

	// SSH format: git@host:path or git@host-alias:path
	if strings.HasPrefix(url, "git@") {
		parts := strings.SplitN(url, ":", 2)
		if len(parts) == 2 {
			return parts[1]
		}
	}

	// URL format: https://host/path or ssh://git@host/path
	if strings.Contains(url, "://") {
		parts := strings.SplitN(url, "://", 2)
		if len(parts) == 2 {
			// Remove host, keep path
			pathParts := strings.SplitN(parts[1], "/", 2)
			if len(pathParts) == 2 {
				return pathParts[1]
			}
		}
	}

	return url
}
