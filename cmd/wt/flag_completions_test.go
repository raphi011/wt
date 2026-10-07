package main

import (
	"slices"
	"testing"

	"github.com/spf13/cobra"
)

func TestFlagValueCompletions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cmd  func() *cobra.Command
		flag string
		want []string
	}{
		{"list sort", newListCmd, "sort", []string{"date", "repo", "branch"}},
		{"repo list sort", newRepoListCmd, "sort", []string{"name", "label"}},
		{"repo clone clone-mode", newRepoCloneCmd, "clone-mode", []string{"bare", "regular"}},
		{"repo convert clone-mode", newRepoConvertCmd, "clone-mode", []string{"bare", "regular"}},
		{"pr checkout clone-mode", newPrCheckoutCmd, "clone-mode", []string{"bare", "regular"}},
		{"pr checkout forge", newPrCheckoutCmd, "forge", []string{"github", "gitlab"}},
		{"pr merge strategy", newPrMergeCmd, "strategy", []string{"squash", "rebase", "merge"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cmd := tt.cmd()
			complete, ok := cmd.GetFlagCompletionFunc(tt.flag)
			if !ok {
				t.Fatalf("no completion registered for --%s", tt.flag)
			}
			got, directive := complete(cmd, nil, "")
			if !slices.Equal(got, tt.want) {
				t.Errorf("--%s completions = %v, want %v", tt.flag, got, tt.want)
			}
			if directive != cobra.ShellCompDirectiveNoFileComp {
				t.Errorf("--%s directive = %v, want NoFileComp", tt.flag, directive)
			}
		})
	}
}
