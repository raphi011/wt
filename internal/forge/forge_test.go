package forge

import (
	"context"
	"strings"
	"testing"
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
