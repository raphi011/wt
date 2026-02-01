package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:       "init <shell>",
		Short:     "Output shell wrapper function",
		GroupID:   GroupConfig,
		ValidArgs: []string{"bash", "zsh", "fish"},
		Args:      cobra.ExactArgs(1),
		Long: `Output shell wrapper function that makes 'wt cd' change directories.

Without this wrapper, 'wt cd' only prints the path (since subprocesses
cannot change the parent shell's directory). The wrapper intercepts
'wt cd' and performs the actual directory change.`,
		Example: `  eval "$(wt init bash)"           # add to ~/.bashrc
  eval "$(wt init zsh)"            # add to ~/.zshrc
  wt init fish | source            # add to ~/.config/fish/config.fish`,
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "fish":
				fmt.Print(fishInit)
			case "bash":
				fmt.Print(bashInit)
			case "zsh":
				fmt.Print(zshInit)
			default:
				return fmt.Errorf("unsupported shell: %s (supported: fish, bash, zsh)", args[0])
			}
			return nil
		},
	}

	return cmd
}

const bashInit = `# wt shell wrapper
# Install: eval "$(wt init bash)"

wt() {
    if [[ "$1" == "cd" ]]; then
        shift
        local dir
        dir="$(command wt cd "$@")" && cd "$dir"
    else
        command wt "$@"
    fi
}
`

const zshInit = `# wt shell wrapper
# Install: eval "$(wt init zsh)"

wt() {
    if [[ "$1" == "cd" ]]; then
        shift
        local dir
        dir="$(command wt cd "$@")" && cd "$dir"
    else
        command wt "$@"
    fi
}
`

const fishInit = `# wt shell wrapper
# Install: wt init fish | source
# Or add to config.fish: wt init fish | source

function wt --wraps=wt --description 'Git worktree manager'
    if test (count $argv) -gt 0; and test "$argv[1]" = "cd"
        set -l dir (command wt $argv)
        and cd $dir
    else
        command wt $argv
    end
end
`
