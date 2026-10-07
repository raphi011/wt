package main

import (
	"github.com/spf13/cobra"

	"github.com/raphi011/wt/internal/output"
)

func newCompletionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:       "completion <shell>",
		Short:     "Generate completion script",
		GroupID:   GroupConfig,
		Long:      `Generate shell completion script for bash, zsh, fish, or powershell.`,
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		Args:      cobra.ExactArgs(1),
		Example: `  # Fish
  wt completion fish > ~/.config/fish/completions/wt.fish

  # Bash
  wt completion bash > ~/.local/share/bash-completion/completions/wt

  # Zsh
  wt completion zsh > ~/.zfunc/_wt
  # Then add ~/.zfunc to fpath in .zshrc`,
		RunE: func(cmd *cobra.Command, args []string) error {
			w := output.FromContext(cmd.Context()).Writer()
			switch args[0] {
			case "bash":
				return cmd.Root().GenBashCompletionV2(w, true)
			case "zsh":
				return cmd.Root().GenZshCompletion(w)
			case "fish":
				return cmd.Root().GenFishCompletion(w, true)
			case "powershell":
				return cmd.Root().GenPowerShellCompletionWithDesc(w)
			}
			return nil
		},
	}

	return cmd
}
