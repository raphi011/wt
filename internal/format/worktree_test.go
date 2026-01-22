package format

import (
	"strings"
	"testing"
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
			name:    "folder only",
			format:  "{folder}",
			wantErr: false,
		},
		{
			name:    "all placeholders",
			format:  "{repo}_{folder}_{branch}",
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
				GitOrigin:  "wt",
				BranchName: "feature-branch",
				FolderName: "wt",
			},
			want: "wt-feature-branch",
		},
		{
			name:   "folder format",
			format: "{folder}_{branch}",
			params: FormatParams{
				GitOrigin:  "wt",
				BranchName: "feature-branch",
				FolderName: "my-wt",
			},
			want: "my-wt_feature-branch",
		},
		{
			name:   "branch with slashes",
			format: "{repo}-{branch}",
			params: FormatParams{
				GitOrigin:  "repo",
				BranchName: "feature/add-login",
				FolderName: "repo",
			},
			want: "repo-feature-add-login",
		},
		{
			name:   "all placeholders",
			format: "{repo}+{folder}+{branch}",
			params: FormatParams{
				GitOrigin:  "origin-name",
				BranchName: "my-branch",
				FolderName: "folder",
			},
			want: "origin-name+folder+my-branch",
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
