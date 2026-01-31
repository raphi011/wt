package main

import (
	"os"

	"github.com/spf13/cobra"
)

func newCompletionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:       "completion <shell>",
		Short:     "Generate completion script",
		GroupID:   GroupConfig,
		Long:      `Generate shell completion script.`,
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
			switch args[0] {
			case "bash":
				return cmd.Root().GenBashCompletion(os.Stdout)
			case "zsh":
				return cmd.Root().GenZshCompletion(os.Stdout)
			case "fish":
				return cmd.Root().GenFishCompletion(os.Stdout, true)
			case "powershell":
				return cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
			}
			return nil
		},
	}

	return cmd
}
