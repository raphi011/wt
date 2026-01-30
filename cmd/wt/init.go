package main

import "fmt"

func (c *InitCmd) runInit() error {
	switch c.Shell {
	case "fish":
		fmt.Print(fishInit)
		return nil
	case "bash":
		fmt.Print(bashInit)
		return nil
	case "zsh":
		fmt.Print(zshInit)
		return nil
	default:
		return fmt.Errorf("unsupported shell: %s (supported: fish, bash, zsh)", c.Shell)
	}
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
    if test (count $argv) -gt 0 -a "$argv[1]" = "cd"
        set -l dir (command wt $argv)
        and cd $dir
    else
        command wt $argv
    end
end
`
