package forge

import (
	"testing"
)

func TestExtractHost(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "SSH format github.com",
			url:  "git@github.com:user/repo.git",
			want: "github.com",
		},
		{
			name: "SSH format gitlab.com",
			url:  "git@gitlab.com:user/repo.git",
			want: "gitlab.com",
		},
		{
			name: "SSH format custom host",
			url:  "git@github.mycompany.com:org/repo.git",
			want: "github.mycompany.com",
		},
		{
			name: "HTTPS format github.com",
			url:  "https://github.com/user/repo.git",
			want: "github.com",
		},
		{
			name: "HTTPS format gitlab.com",
			url:  "https://gitlab.com/user/repo.git",
			want: "gitlab.com",
		},
		{
			name: "HTTPS format custom host",
			url:  "https://gitlab.internal.corp/org/repo.git",
			want: "gitlab.internal.corp",
		},
		{
			name: "HTTPS with port",
			url:  "https://code.company.com:8443/org/repo.git",
			want: "code.company.com",
		},
		{
			name: "HTTP format",
			url:  "http://github.mycompany.com/org/repo.git",
			want: "github.mycompany.com",
		},
		{
			name: "SSH protocol URL",
			url:  "ssh://git@github.com/user/repo.git",
			want: "github.com",
		},
		{
			name: "SSH protocol URL with port",
			url:  "ssh://git@gitlab.internal.corp:2222/org/repo.git",
			want: "gitlab.internal.corp",
		},
		{
			name: "empty string",
			url:  "",
			want: "",
		},
		{
			name: "invalid format",
			url:  "not-a-url",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractHost(tt.url)
			if got != tt.want {
				t.Errorf("extractHost(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestDetect(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		hostMap  map[string]string
		wantType string
	}{
		{
			name:     "github.com without hostMap",
			url:      "git@github.com:user/repo.git",
			hostMap:  nil,
			wantType: "*forge.GitHub",
		},
		{
			name:     "gitlab.com without hostMap",
			url:      "git@gitlab.com:user/repo.git",
			hostMap:  nil,
			wantType: "*forge.GitLab",
		},
		{
			name:     "custom host matched to github",
			url:      "git@github.mycompany.com:org/repo.git",
			hostMap:  map[string]string{"github.mycompany.com": "github"},
			wantType: "*forge.GitHub",
		},
		{
			name:     "custom host matched to gitlab",
			url:      "git@code.internal.corp:org/repo.git",
			hostMap:  map[string]string{"code.internal.corp": "gitlab"},
			wantType: "*forge.GitLab",
		},
		{
			name:     "hostMap priority over pattern matching",
			url:      "git@gitlab.mycompany.com:org/repo.git",
			hostMap:  map[string]string{"gitlab.mycompany.com": "github"},
			wantType: "*forge.GitHub",
		},
		{
			name:     "pattern fallback gitlab. prefix",
			url:      "git@gitlab.example.com:org/repo.git",
			hostMap:  nil,
			wantType: "*forge.GitLab",
		},
		{
			name:     "pattern fallback /gitlab/ in path",
			url:      "https://company.com/gitlab/org/repo.git",
			hostMap:  nil,
			wantType: "*forge.GitLab",
		},
		{
			name:     "default to github for unknown host",
			url:      "git@unknown.example.com:org/repo.git",
			hostMap:  nil,
			wantType: "*forge.GitHub",
		},
		{
			name:     "empty hostMap uses pattern matching",
			url:      "git@gitlab.com:user/repo.git",
			hostMap:  map[string]string{},
			wantType: "*forge.GitLab",
		},
		{
			name:     "HTTPS URL with custom host",
			url:      "https://github.enterprise.corp/org/repo.git",
			hostMap:  map[string]string{"github.enterprise.corp": "github"},
			wantType: "*forge.GitHub",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Detect(tt.url, tt.hostMap, nil)
			gotType := getForgeType(got)
			if gotType != tt.wantType {
				t.Errorf("Detect(%q, %v) = %s, want %s", tt.url, tt.hostMap, gotType, tt.wantType)
			}
		})
	}
}

func getForgeType(f Forge) string {
	switch f.(type) {
	case *GitHub:
		return "*forge.GitHub"
	case *GitLab:
		return "*forge.GitLab"
	default:
		return "unknown"
	}
}

func TestExtractRepoPath(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "SSH standard",
			url:  "git@github.com:user/repo.git",
			want: "user/repo",
		},
		{
			name: "SSH with alias",
			url:  "git@github.com-personal:user/repo.git",
			want: "user/repo",
		},
		{
			name: "SSH with alias and org",
			url:  "git@github.com-work:myorg/myrepo.git",
			want: "myorg/myrepo",
		},
		{
			name: "HTTPS github",
			url:  "https://github.com/user/repo.git",
			want: "user/repo",
		},
		{
			name: "HTTPS gitlab",
			url:  "https://gitlab.com/user/repo.git",
			want: "user/repo",
		},
		{
			name: "GitLab subgroups SSH",
			url:  "git@gitlab.com:group/subgroup/repo.git",
			want: "group/subgroup/repo",
		},
		{
			name: "GitLab subgroups HTTPS",
			url:  "https://gitlab.com/group/subgroup/repo.git",
			want: "group/subgroup/repo",
		},
		{
			name: "SSH protocol URL",
			url:  "ssh://git@github.com/user/repo.git",
			want: "user/repo",
		},
		{
			name: "SSH protocol URL with port",
			url:  "ssh://git@gitlab.internal.corp:2222/org/repo.git",
			want: "org/repo",
		},
		{
			name: "without .git suffix",
			url:  "git@github.com:user/repo",
			want: "user/repo",
		},
		{
			name: "HTTPS without .git suffix",
			url:  "https://github.com/user/repo",
			want: "user/repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractRepoPath(tt.url)
			if got != tt.want {
				t.Errorf("extractRepoPath(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}
