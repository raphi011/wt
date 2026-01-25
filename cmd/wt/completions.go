package main

import "fmt"

func runCompletion(cmd *CompletionCmd) error {
	switch cmd.Shell {
	case "fish":
		fmt.Print(fishCompletions)
		return nil
	case "bash":
		fmt.Print(bashCompletions)
		return nil
	case "zsh":
		fmt.Print(zshCompletions)
		return nil
	default:
		return fmt.Errorf("unsupported shell: %s (supported: fish, bash, zsh)", cmd.Shell)
	}
}

const bashCompletions = `# wt bash completions
# Install: wt completion bash > ~/.local/share/bash-completion/completions/wt
# Or: wt completion bash >> ~/.bashrc

_wt_completions() {
    local cur prev words cword
    if type _init_completion &>/dev/null; then
        _init_completion || return
    else
        cur="${COMP_WORDS[COMP_CWORD]}"
        prev="${COMP_WORDS[COMP_CWORD-1]}"
        words=("${COMP_WORDS[@]}")
        cword=$COMP_CWORD
    fi

    local commands="add a prune p list ls show s repos r exec x cd mv note label hook pr config completion doctor"

    # Handle subcommand-specific completions
    case "${words[1]}" in
        add|a)
            case "$prev" in
                -r|--repository)
                    # Complete repo names from worktree_dir
                    local dir="$WT_WORKTREE_DIR"
                    if [[ -z "$dir" ]]; then
                        local config_file=~/.config/wt/config.toml
                        if [[ -f "$config_file" ]]; then
                            dir=$(grep '^worktree_dir' "$config_file" 2>/dev/null | sed 's/.*= *"\?\([^"]*\)"\?/\1/' | sed "s|~|$HOME|")
                        fi
                    fi
                    if [[ -d "$dir" ]]; then
                        local repos=""
                        for d in "$dir"/*/; do
                            if [[ -d "$d/.git" ]] || [[ -f "$d/.git" ]]; then
                                repos="$repos $(basename "$d")"
                            fi
                        done
                        COMPREPLY=($(compgen -W "$repos" -- "$cur"))
                    fi
                    return
                    ;;
                -l|--label)
                    # Complete labels from repos in worktree_dir
                    local dir="$WT_WORKTREE_DIR"
                    if [[ -z "$dir" ]]; then
                        local config_file=~/.config/wt/config.toml
                        if [[ -f "$config_file" ]]; then
                            dir=$(grep '^worktree_dir' "$config_file" 2>/dev/null | sed 's/.*= *"\?\([^"]*\)"\?/\1/' | sed "s|~|$HOME|")
                        fi
                    fi
                    if [[ -d "$dir" ]]; then
                        local labels=""
                        for d in "$dir"/*/; do
                            if [[ -d "$d/.git" ]] || [[ -f "$d/.git" ]]; then
                                local repo_labels=$(git -C "$d" config --local wt.labels 2>/dev/null)
                                if [[ -n "$repo_labels" ]]; then
                                    labels="$labels $(echo "$repo_labels" | tr ',' ' ')"
                                fi
                            fi
                        done
                        COMPREPLY=($(compgen -W "$(echo "$labels" | tr ' ' '\n' | sort -u)" -- "$cur"))
                    fi
                    return
                    ;;
                --hook|--note)
                    return
                    ;;
                --base)
                    # Complete branch names for --base
                    local branches=$(git branch --all --format='%(refname:short)' 2>/dev/null | sed 's|origin/||' | sort -u)
                    COMPREPLY=($(compgen -W "$branches" -- "$cur"))
                    return
                    ;;
            esac
            if [[ $cword -eq 2 ]]; then
                # Complete branch names (all branches for existing, -b for new)
                local branches=$(git branch --all --format='%(refname:short)' 2>/dev/null | sed 's|origin/||' | sort -u)
                COMPREPLY=($(compgen -W "$branches" -- "$cur"))
            else
                COMPREPLY=($(compgen -W "-b --new-branch -r --repository -l --label --base -f --fetch --note --hook --no-hook -a --arg" -- "$cur"))
            fi
            ;;
        prune|p)
            case "$prev" in
                -i|--id)
                    # Complete worktree IDs only
                    local ids=$(wt list 2>/dev/null | awk '{print $1}')
                    COMPREPLY=($(compgen -W "$ids" -- "$cur"))
                    return
                    ;;
                --hook)
                    return
                    ;;
            esac
            COMPREPLY=($(compgen -W "-i --id -n --dry-run -f --force -c --include-clean -g --global -v --verbose -R --refresh --reset-cache --hook --no-hook -a --arg" -- "$cur"))
            ;;
        list|ls)
            case "$prev" in
                -s|--sort)
                    COMPREPLY=($(compgen -W "id repo branch commit" -- "$cur"))
                    return
                    ;;
                -r|--repository)
                    # Complete repo names from worktree_dir
                    local dir="$WT_WORKTREE_DIR"
                    if [[ -z "$dir" ]]; then
                        local config_file=~/.config/wt/config.toml
                        if [[ -f "$config_file" ]]; then
                            dir=$(grep '^worktree_dir' "$config_file" 2>/dev/null | sed 's/.*= *"\?\([^"]*\)"\?/\1/' | sed "s|~|$HOME|")
                        fi
                    fi
                    if [[ -d "$dir" ]]; then
                        local repos=""
                        for d in "$dir"/*/; do
                            if [[ -d "$d/.git" ]] || [[ -f "$d/.git" ]]; then
                                repos="$repos $(basename "$d")"
                            fi
                        done
                        COMPREPLY=($(compgen -W "$repos" -- "$cur"))
                    fi
                    return
                    ;;
                -l|--label)
                    # Complete labels from repos in worktree_dir
                    local dir="$WT_WORKTREE_DIR"
                    if [[ -z "$dir" ]]; then
                        local config_file=~/.config/wt/config.toml
                        if [[ -f "$config_file" ]]; then
                            dir=$(grep '^worktree_dir' "$config_file" 2>/dev/null | sed 's/.*= *"\?\([^"]*\)"\?/\1/' | sed "s|~|$HOME|")
                        fi
                    fi
                    if [[ -d "$dir" ]]; then
                        local labels=""
                        for d in "$dir"/*/; do
                            if [[ -d "$d/.git" ]] || [[ -f "$d/.git" ]]; then
                                local repo_labels=$(git -C "$d" config --local wt.labels 2>/dev/null)
                                if [[ -n "$repo_labels" ]]; then
                                    labels="$labels $(echo "$repo_labels" | tr ',' ' ')"
                                fi
                            fi
                        done
                        COMPREPLY=($(compgen -W "$(echo "$labels" | tr ' ' '\n' | sort -u)" -- "$cur"))
                    fi
                    return
                    ;;
            esac
            COMPREPLY=($(compgen -W "--json -g --global -s --sort -R --refresh -r --repository -l --label" -- "$cur"))
            ;;
        repos|r)
            case "$prev" in
                -l|--label)
                    # Complete labels from repos in worktree_dir
                    local dir="$WT_WORKTREE_DIR"
                    if [[ -z "$dir" ]]; then
                        local config_file=~/.config/wt/config.toml
                        if [[ -f "$config_file" ]]; then
                            dir=$(grep '^worktree_dir' "$config_file" 2>/dev/null | sed 's/.*= *"\?\([^"]*\)"\?/\1/' | sed "s|~|$HOME|")
                        fi
                    fi
                    if [[ -d "$dir" ]]; then
                        local labels=""
                        for d in "$dir"/*/; do
                            if [[ -d "$d/.git" ]] || [[ -f "$d/.git" ]]; then
                                local repo_labels=$(git -C "$d" config --local wt.labels 2>/dev/null)
                                if [[ -n "$repo_labels" ]]; then
                                    labels="$labels $(echo "$repo_labels" | tr ',' ' ')"
                                fi
                            fi
                        done
                        COMPREPLY=($(compgen -W "$(echo "$labels" | tr ' ' '\n' | sort -u)" -- "$cur"))
                    fi
                    return
                    ;;
                -s|--sort)
                    COMPREPLY=($(compgen -W "name branch worktrees label" -- "$cur"))
                    return
                    ;;
            esac
            COMPREPLY=($(compgen -W "-l --label -s --sort --json" -- "$cur"))
            ;;
        show|s)
            case "$prev" in
                -i|--id)
                    # Complete worktree IDs only
                    local ids=$(wt list 2>/dev/null | awk '{print $1}')
                    COMPREPLY=($(compgen -W "$ids" -- "$cur"))
                    return
                    ;;
                -r|--repository)
                    # Complete repo names from worktree_dir
                    local dir="$WT_WORKTREE_DIR"
                    if [[ -z "$dir" ]]; then
                        local config_file=~/.config/wt/config.toml
                        if [[ -f "$config_file" ]]; then
                            dir=$(grep '^worktree_dir' "$config_file" 2>/dev/null | sed 's/.*= *"\?\([^"]*\)"\?/\1/' | sed "s|~|$HOME|")
                        fi
                    fi
                    if [[ -d "$dir" ]]; then
                        local repos=""
                        for d in "$dir"/*/; do
                            if [[ -d "$d/.git" ]] || [[ -f "$d/.git" ]]; then
                                repos="$repos $(basename "$d")"
                            fi
                        done
                        COMPREPLY=($(compgen -W "$repos" -- "$cur"))
                    fi
                    return
                    ;;
            esac
            COMPREPLY=($(compgen -W "-i --id -r --repository -R --refresh --json" -- "$cur"))
            ;;
        exec|x)
            case "$prev" in
                -i|--id)
                    # Complete worktree IDs only
                    local ids=$(wt list 2>/dev/null | awk '{print $1}')
                    COMPREPLY=($(compgen -W "$ids" -- "$cur"))
                    return
                    ;;
                -r|--repository)
                    # Complete repo names from worktree_dir
                    local dir="$WT_WORKTREE_DIR"
                    if [[ -z "$dir" ]]; then
                        local config_file=~/.config/wt/config.toml
                        if [[ -f "$config_file" ]]; then
                            dir=$(grep '^worktree_dir' "$config_file" 2>/dev/null | sed 's/.*= *"\?\([^"]*\)"\?/\1/' | sed "s|~|$HOME|")
                        fi
                    fi
                    if [[ -d "$dir" ]]; then
                        local repos=""
                        for d in "$dir"/*/; do
                            if [[ -d "$d/.git" ]] || [[ -f "$d/.git" ]]; then
                                repos="$repos $(basename "$d")"
                            fi
                        done
                        COMPREPLY=($(compgen -W "$repos" -- "$cur"))
                    fi
                    return
                    ;;
                -l|--label)
                    # Complete labels from repos in worktree_dir
                    local dir="$WT_WORKTREE_DIR"
                    if [[ -z "$dir" ]]; then
                        local config_file=~/.config/wt/config.toml
                        if [[ -f "$config_file" ]]; then
                            dir=$(grep '^worktree_dir' "$config_file" 2>/dev/null | sed 's/.*= *"\?\([^"]*\)"\?/\1/' | sed "s|~|$HOME|")
                        fi
                    fi
                    if [[ -d "$dir" ]]; then
                        local labels=""
                        for d in "$dir"/*/; do
                            if [[ -d "$d/.git" ]] || [[ -f "$d/.git" ]]; then
                                local repo_labels=$(git -C "$d" config --local wt.labels 2>/dev/null)
                                if [[ -n "$repo_labels" ]]; then
                                    labels="$labels $(echo "$repo_labels" | tr ',' ' ')"
                                fi
                            fi
                        done
                        COMPREPLY=($(compgen -W "$(echo "$labels" | tr ' ' '\n' | sort -u)" -- "$cur"))
                    fi
                    return
                    ;;
            esac
            COMPREPLY=($(compgen -W "-i --id -r --repository -l --label" -- "$cur"))
            ;;
        cd)
            case "$prev" in
                -i|--id)
                    # Complete worktree IDs only
                    local ids=$(wt list 2>/dev/null | awk '{print $1}')
                    COMPREPLY=($(compgen -W "$ids" -- "$cur"))
                    return
                    ;;
                -r|--repository)
                    # Complete repo names from worktree_dir
                    local dir="$WT_WORKTREE_DIR"
                    if [[ -z "$dir" ]]; then
                        local config_file=~/.config/wt/config.toml
                        if [[ -f "$config_file" ]]; then
                            dir=$(grep '^worktree_dir' "$config_file" 2>/dev/null | sed 's/.*= *"\?\([^"]*\)"\?/\1/' | sed "s|~|$HOME|")
                        fi
                    fi
                    if [[ -d "$dir" ]]; then
                        local repos=""
                        for d in "$dir"/*/; do
                            if [[ -d "$d/.git" ]] || [[ -f "$d/.git" ]]; then
                                repos="$repos $(basename "$d")"
                            fi
                        done
                        COMPREPLY=($(compgen -W "$repos" -- "$cur"))
                    fi
                    return
                    ;;
                -l|--label)
                    # Complete labels from repos in worktree_dir
                    local dir="$WT_WORKTREE_DIR"
                    if [[ -z "$dir" ]]; then
                        local config_file=~/.config/wt/config.toml
                        if [[ -f "$config_file" ]]; then
                            dir=$(grep '^worktree_dir' "$config_file" 2>/dev/null | sed 's/.*= *"\?\([^"]*\)"\?/\1/' | sed "s|~|$HOME|")
                        fi
                    fi
                    if [[ -d "$dir" ]]; then
                        local labels=""
                        for d in "$dir"/*/; do
                            if [[ -d "$d/.git" ]] || [[ -f "$d/.git" ]]; then
                                local repo_labels=$(git -C "$d" config --local wt.labels 2>/dev/null)
                                if [[ -n "$repo_labels" ]]; then
                                    labels="$labels $(echo "$repo_labels" | tr ',' ' ')"
                                fi
                            fi
                        done
                        COMPREPLY=($(compgen -W "$(echo "$labels" | tr ' ' '\n' | sort -u)" -- "$cur"))
                    fi
                    return
                    ;;
                --hook|-a|--arg)
                    return
                    ;;
            esac
            COMPREPLY=($(compgen -W "-i --id -r --repository -l --label -p --project --hook --no-hook -a --arg" -- "$cur"))
            ;;
        mv)
            case "$prev" in
                -r|--repository)
                    # Complete repo names from worktree_dir
                    local dir="$WT_WORKTREE_DIR"
                    if [[ -z "$dir" ]]; then
                        local config_file=~/.config/wt/config.toml
                        if [[ -f "$config_file" ]]; then
                            dir=$(grep '^worktree_dir' "$config_file" 2>/dev/null | sed 's/.*= *"\?\([^"]*\)"\?/\1/' | sed "s|~|$HOME|")
                        fi
                    fi
                    if [[ -d "$dir" ]]; then
                        local repos=""
                        for d in "$dir"/*/; do
                            if [[ -d "$d/.git" ]] || [[ -f "$d/.git" ]]; then
                                repos="$repos $(basename "$d")"
                            fi
                        done
                        COMPREPLY=($(compgen -W "$repos" -- "$cur"))
                    fi
                    return
                    ;;
                --format)
                    return
                    ;;
            esac
            COMPREPLY=($(compgen -W "-r --repository --format -n --dry-run -f --force" -- "$cur"))
            ;;
        pr)
            if [[ $cword -eq 2 ]]; then
                COMPREPLY=($(compgen -W "checkout create merge view" -- "$cur"))
            elif [[ "${words[2]}" == "checkout" ]]; then
                case "$prev" in
                    -r|--repository)
                        # Complete repo names from worktree_dir
                        local dir="$WT_WORKTREE_DIR"
                        if [[ -z "$dir" ]]; then
                            local config_file=~/.config/wt/config.toml
                            if [[ -f "$config_file" ]]; then
                                dir=$(grep '^worktree_dir' "$config_file" 2>/dev/null | sed 's/.*= *"\?\([^"]*\)"\?/\1/' | sed "s|~|$HOME|")
                            fi
                        fi
                        if [[ -d "$dir" ]]; then
                            local repos=""
                            for d in "$dir"/*/; do
                                if [[ -d "$d/.git" ]] || [[ -f "$d/.git" ]]; then
                                    repos="$repos $(basename "$d")"
                                fi
                            done
                            COMPREPLY=($(compgen -W "$repos" -- "$cur"))
                        fi
                        return
                        ;;
                    --forge)
                        COMPREPLY=($(compgen -W "github gitlab" -- "$cur"))
                        return
                        ;;
                    --hook|--note)
                        return
                        ;;
                esac
                COMPREPLY=($(compgen -W "-r --repository --forge --note --hook --no-hook -a --arg" -- "$cur"))
            elif [[ "${words[2]}" == "create" ]]; then
                case "$prev" in
                    -i|--id)
                        # Complete worktree IDs only
                        local ids=$(wt list 2>/dev/null | awk '{print $1}')
                        COMPREPLY=($(compgen -W "$ids" -- "$cur"))
                        return
                        ;;
                    -r|--repository)
                        # Complete repo names from worktree_dir
                        local dir="$WT_WORKTREE_DIR"
                        if [[ -z "$dir" ]]; then
                            local config_file=~/.config/wt/config.toml
                            if [[ -f "$config_file" ]]; then
                                dir=$(grep '^worktree_dir' "$config_file" 2>/dev/null | sed 's/.*= *"\?\([^"]*\)"\?/\1/' | sed "s|~|$HOME|")
                            fi
                        fi
                        if [[ -d "$dir" ]]; then
                            local repos=""
                            for d in "$dir"/*/; do
                                if [[ -d "$d/.git" ]] || [[ -f "$d/.git" ]]; then
                                    repos="$repos $(basename "$d")"
                                fi
                            done
                            COMPREPLY=($(compgen -W "$repos" -- "$cur"))
                        fi
                        return
                        ;;
                    -t|--title|-b|--body|--body-file|--base)
                        return
                        ;;
                esac
                COMPREPLY=($(compgen -W "-i --id -r --repository -t --title -b --body --body-file --base --draft -w --web" -- "$cur"))
            elif [[ "${words[2]}" == "merge" ]]; then
                case "$prev" in
                    -i|--id)
                        # Complete worktree IDs only
                        local ids=$(wt list 2>/dev/null | awk '{print $1}')
                        COMPREPLY=($(compgen -W "$ids" -- "$cur"))
                        return
                        ;;
                    -r|--repository)
                        # Complete repo names from worktree_dir
                        local dir="$WT_WORKTREE_DIR"
                        if [[ -z "$dir" ]]; then
                            local config_file=~/.config/wt/config.toml
                            if [[ -f "$config_file" ]]; then
                                dir=$(grep '^worktree_dir' "$config_file" 2>/dev/null | sed 's/.*= *"\?\([^"]*\)"\?/\1/' | sed "s|~|$HOME|")
                            fi
                        fi
                        if [[ -d "$dir" ]]; then
                            local repos=""
                            for d in "$dir"/*/; do
                                if [[ -d "$d/.git" ]] || [[ -f "$d/.git" ]]; then
                                    repos="$repos $(basename "$d")"
                                fi
                            done
                            COMPREPLY=($(compgen -W "$repos" -- "$cur"))
                        fi
                        return
                        ;;
                    -s|--strategy)
                        COMPREPLY=($(compgen -W "squash rebase merge" -- "$cur"))
                        return
                        ;;
                    --hook)
                        return
                        ;;
                esac
                COMPREPLY=($(compgen -W "-i --id -r --repository -s --strategy -k --keep --hook --no-hook -a --arg" -- "$cur"))
            elif [[ "${words[2]}" == "view" ]]; then
                case "$prev" in
                    -i|--id)
                        # Complete worktree IDs only
                        local ids=$(wt list 2>/dev/null | awk '{print $1}')
                        COMPREPLY=($(compgen -W "$ids" -- "$cur"))
                        return
                        ;;
                    -r|--repository)
                        # Complete repo names from worktree_dir
                        local dir="$WT_WORKTREE_DIR"
                        if [[ -z "$dir" ]]; then
                            local config_file=~/.config/wt/config.toml
                            if [[ -f "$config_file" ]]; then
                                dir=$(grep '^worktree_dir' "$config_file" 2>/dev/null | sed 's/.*= *"\?\([^"]*\)"\?/\1/' | sed "s|~|$HOME|")
                            fi
                        fi
                        if [[ -d "$dir" ]]; then
                            local repos=""
                            for d in "$dir"/*/; do
                                if [[ -d "$d/.git" ]] || [[ -f "$d/.git" ]]; then
                                    repos="$repos $(basename "$d")"
                                fi
                            done
                            COMPREPLY=($(compgen -W "$repos" -- "$cur"))
                        fi
                        return
                        ;;
                esac
                COMPREPLY=($(compgen -W "-i --id -r --repository -w --web" -- "$cur"))
            fi
            ;;
        note)
            # note get is default, so flags work directly on note
            case "$prev" in
                -i|--id)
                    # Complete worktree IDs only
                    local ids=$(wt list 2>/dev/null | awk '{print $1}')
                    COMPREPLY=($(compgen -W "$ids" -- "$cur"))
                    return
                    ;;
                -r|--repository)
                    # Complete repo names from worktree_dir
                    local dir="$WT_WORKTREE_DIR"
                    if [[ -z "$dir" ]]; then
                        local config_file=~/.config/wt/config.toml
                        if [[ -f "$config_file" ]]; then
                            dir=$(grep '^worktree_dir' "$config_file" 2>/dev/null | sed 's/.*= *"\?\([^"]*\)"\?/\1/' | sed "s|~|$HOME|")
                        fi
                    fi
                    if [[ -d "$dir" ]]; then
                        local repos=""
                        for d in "$dir"/*/; do
                            if [[ -d "$d/.git" ]] || [[ -f "$d/.git" ]]; then
                                repos="$repos $(basename "$d")"
                            fi
                        done
                        COMPREPLY=($(compgen -W "$repos" -- "$cur"))
                    fi
                    return
                    ;;
            esac
            if [[ $cword -eq 2 ]]; then
                COMPREPLY=($(compgen -W "set get clear -i --id -r --repository" -- "$cur"))
            elif [[ "${words[2]}" == "set" ]]; then
                COMPREPLY=($(compgen -W "-i --id -r --repository" -- "$cur"))
            elif [[ "${words[2]}" == "get" ]] || [[ "${words[2]}" == "clear" ]]; then
                COMPREPLY=($(compgen -W "-i --id -r --repository" -- "$cur"))
            else
                COMPREPLY=($(compgen -W "-i --id -r --repository" -- "$cur"))
            fi
            ;;
        label)
            # label list is default, so flags work directly on label
            case "$prev" in
                -r|--repository)
                    # Complete repo names from worktree_dir
                    local dir="$WT_WORKTREE_DIR"
                    if [[ -z "$dir" ]]; then
                        local config_file=~/.config/wt/config.toml
                        if [[ -f "$config_file" ]]; then
                            dir=$(grep '^worktree_dir' "$config_file" 2>/dev/null | sed 's/.*= *"\?\([^"]*\)"\?/\1/' | sed "s|~|$HOME|")
                        fi
                    fi
                    if [[ -d "$dir" ]]; then
                        local repos=""
                        for d in "$dir"/*/; do
                            if [[ -d "$d/.git" ]] || [[ -f "$d/.git" ]]; then
                                repos="$repos $(basename "$d")"
                            fi
                        done
                        COMPREPLY=($(compgen -W "$repos" -- "$cur"))
                    fi
                    return
                    ;;
            esac
            if [[ $cword -eq 2 ]]; then
                COMPREPLY=($(compgen -W "add remove list clear -r --repository -g --global" -- "$cur"))
            elif [[ "${words[2]}" == "add" ]] || [[ "${words[2]}" == "remove" ]]; then
                COMPREPLY=($(compgen -W "-r --repository" -- "$cur"))
            elif [[ "${words[2]}" == "list" ]]; then
                COMPREPLY=($(compgen -W "-r --repository -g --global" -- "$cur"))
            elif [[ "${words[2]}" == "clear" ]]; then
                COMPREPLY=($(compgen -W "-r --repository" -- "$cur"))
            else
                COMPREPLY=($(compgen -W "-r --repository -g --global" -- "$cur"))
            fi
            ;;
        hook)
            case "$prev" in
                -i|--id)
                    # Complete worktree IDs only
                    local ids=$(wt list 2>/dev/null | awk '{print $1}')
                    COMPREPLY=($(compgen -W "$ids" -- "$cur"))
                    return
                    ;;
                -r|--repository)
                    # Complete repo names from worktree_dir
                    local dir="$WT_WORKTREE_DIR"
                    if [[ -z "$dir" ]]; then
                        local config_file=~/.config/wt/config.toml
                        if [[ -f "$config_file" ]]; then
                            dir=$(grep '^worktree_dir' "$config_file" 2>/dev/null | sed 's/.*= *"\?\([^"]*\)"\?/\1/' | sed "s|~|$HOME|")
                        fi
                    fi
                    if [[ -d "$dir" ]]; then
                        local repos=""
                        for d in "$dir"/*/; do
                            if [[ -d "$d/.git" ]] || [[ -f "$d/.git" ]]; then
                                repos="$repos $(basename "$d")"
                            fi
                        done
                        COMPREPLY=($(compgen -W "$repos" -- "$cur"))
                    fi
                    return
                    ;;
                -l|--label)
                    # Complete labels from repos in worktree_dir
                    local dir="$WT_WORKTREE_DIR"
                    if [[ -z "$dir" ]]; then
                        local config_file=~/.config/wt/config.toml
                        if [[ -f "$config_file" ]]; then
                            dir=$(grep '^worktree_dir' "$config_file" 2>/dev/null | sed 's/.*= *"\?\([^"]*\)"\?/\1/' | sed "s|~|$HOME|")
                        fi
                    fi
                    if [[ -d "$dir" ]]; then
                        local labels=""
                        for d in "$dir"/*/; do
                            if [[ -d "$d/.git" ]] || [[ -f "$d/.git" ]]; then
                                local repo_labels=$(git -C "$d" config --local wt.labels 2>/dev/null)
                                if [[ -n "$repo_labels" ]]; then
                                    labels="$labels $(echo "$repo_labels" | tr ',' ' ')"
                                fi
                            fi
                        done
                        COMPREPLY=($(compgen -W "$(echo "$labels" | tr ' ' '\n' | sort -u)" -- "$cur"))
                    fi
                    return
                    ;;
            esac
            # Complete hook names for all positional args (supports multiple hooks)
            local hooks=$(wt config hooks 2>/dev/null | awk '{print $1}')
            COMPREPLY=($(compgen -W "$hooks -i --id -r --repository -l --label -a --arg -n --dry-run" -- "$cur"))
            ;;
        config)
            if [[ $cword -eq 2 ]]; then
                COMPREPLY=($(compgen -W "init show hooks" -- "$cur"))
            elif [[ "${words[2]}" == "init" ]]; then
                if [[ $cword -eq 3 ]] && [[ "$cur" != -* ]]; then
                    # Directory completion for worktree-dir positional arg
                    COMPREPLY=($(compgen -d -- "$cur"))
                else
                    COMPREPLY=($(compgen -W "-f --force -s --stdout" -- "$cur"))
                fi
            elif [[ "${words[2]}" == "show" ]]; then
                COMPREPLY=($(compgen -W "--json" -- "$cur"))
            elif [[ "${words[2]}" == "hooks" ]]; then
                COMPREPLY=($(compgen -W "--json" -- "$cur"))
            fi
            ;;
        completion)
            if [[ $cword -eq 2 ]]; then
                COMPREPLY=($(compgen -W "fish bash zsh" -- "$cur"))
            fi
            ;;
        doctor)
            COMPREPLY=($(compgen -W "--fix --reset" -- "$cur"))
            ;;
        *)
            if [[ $cword -eq 1 ]]; then
                COMPREPLY=($(compgen -W "$commands" -- "$cur"))
            fi
            ;;
    esac
}

complete -F _wt_completions wt
`

const zshCompletions = `#compdef wt
# wt zsh completions
# Install: wt completion zsh > ~/.zfunc/_wt
# Then add to ~/.zshrc: fpath=(~/.zfunc $fpath) && autoload -Uz compinit && compinit

_wt() {
    local curcontext="$curcontext" state line
    typeset -A opt_args

    _arguments -C \
        '1: :->command' \
        '*:: :->args'

    case $state in
        command)
            local commands=(
                'add:Add worktree for branch'
                'a:Add worktree for branch (alias)'
                'prune:Prune merged worktrees'
                'p:Prune merged worktrees (alias)'
                'list:List worktrees with stable IDs'
                'ls:List worktrees (alias)'
                'show:Show worktree details'
                's:Show worktree details (alias)'
                'repos:List repositories'
                'r:List repositories (alias)'
                'exec:Run command in worktree by ID'
                'x:Run command in worktree (alias)'
                'cd:Print worktree path for shell scripting'
                'mv:Move worktrees to another directory'
                'note:Manage branch notes'
                'label:Manage repository labels'
                'hook:Run configured hook'
                'pr:Work with GitHub PRs'
                'config:Manage configuration'
                'completion:Generate completion script'
                'doctor:Diagnose and repair cache'
            )
            _describe 'command' commands
            ;;
        args)
            case $words[1] in
                add|a)
                    _arguments \
                        '1:branch:__wt_all_branches' \
                        '-b[create new branch]' \
                        '--new-branch[create new branch]' \
                        '*-r[repository name]:repository:__wt_repo_names' \
                        '*--repository[repository name]:repository:__wt_repo_names' \
                        '*-l[target repos by label]:label:__wt_label_names' \
                        '*--label[target repos by label]:label:__wt_label_names' \
                        '--base[base branch to create from]:branch:__wt_all_branches' \
                        '-f[fetch base branch before creating]' \
                        '--fetch[fetch base branch before creating]' \
                        '--note[set note on branch]:note:' \
                        '--hook[run named hook]:hook:' \
                        '--no-hook[skip post-add hook]' \
                        '*-a[set hook variable KEY=VALUE]:arg:' \
                        '*--arg[set hook variable KEY=VALUE]:arg:'
                    ;;
                prune|p)
                    _arguments \
                        '-i[worktree ID to remove]:id:__wt_worktree_ids' \
                        '--id[worktree ID to remove]:id:__wt_worktree_ids' \
                        '-n[preview without removing]' \
                        '--dry-run[preview without removing]' \
                        '-f[force remove even if not merged/dirty]' \
                        '--force[force remove even if not merged/dirty]' \
                        '-c[also remove clean worktrees]' \
                        '--include-clean[also remove clean worktrees]' \
                        '-g[prune all worktrees, not just current repo]' \
                        '--global[prune all worktrees, not just current repo]' \
                        '-v[show skipped worktrees with reasons]' \
                        '--verbose[show skipped worktrees with reasons]' \
                        '-R[fetch origin and refresh PR status]' \
                        '--refresh[fetch origin and refresh PR status]' \
                        '--reset-cache[clear cache and reset IDs from 1]' \
                        '--hook[run named hook]:hook:' \
                        '--no-hook[skip post-removal hooks]' \
                        '*-a[set hook variable KEY=VALUE]:arg:' \
                        '*--arg[set hook variable KEY=VALUE]:arg:'
                    ;;
                list|ls)
                    _arguments \
                        '--json[output as JSON]' \
                        '-g[show all worktrees]' \
                        '--global[show all worktrees]' \
                        '-s[sort by]:field:(id repo branch commit)' \
                        '--sort[sort by]:field:(id repo branch commit)' \
                        '-R[fetch origin and refresh PR status]' \
                        '--refresh[fetch origin and refresh PR status]' \
                        '*-r[filter by repository name]:repository:__wt_repo_names' \
                        '*--repository[filter by repository name]:repository:__wt_repo_names' \
                        '*-l[filter by label]:label:__wt_label_names' \
                        '*--label[filter by label]:label:__wt_label_names'
                    ;;
                repos|r)
                    _arguments \
                        '-l[filter by label]:label:__wt_label_names' \
                        '--label[filter by label]:label:__wt_label_names' \
                        '-s[sort by]:field:(name branch worktrees label)' \
                        '--sort[sort by]:field:(name branch worktrees label)' \
                        '--json[output as JSON]'
                    ;;
                show|s)
                    _arguments \
                        '-i[worktree ID]:id:__wt_worktree_ids' \
                        '--id[worktree ID]:id:__wt_worktree_ids' \
                        '-r[repository name]:repository:__wt_repo_names' \
                        '--repository[repository name]:repository:__wt_repo_names' \
                        '-R[refresh PR status from API]' \
                        '--refresh[refresh PR status from API]' \
                        '--json[output as JSON]'
                    ;;
                exec|x)
                    _arguments \
                        '*-i[worktree ID]:id:__wt_worktree_ids' \
                        '*--id[worktree ID]:id:__wt_worktree_ids' \
                        '*-r[repository name]:repository:__wt_repo_names' \
                        '*--repository[repository name]:repository:__wt_repo_names' \
                        '*-l[target repos by label]:label:__wt_label_names' \
                        '*--label[target repos by label]:label:__wt_label_names'
                    ;;
                cd)
                    _arguments \
                        '-i[worktree ID]:id:__wt_worktree_ids' \
                        '--id[worktree ID]:id:__wt_worktree_ids' \
                        '-r[repository name]:repository:__wt_repo_names' \
                        '--repository[repository name]:repository:__wt_repo_names' \
                        '-l[repository label (must match one repo)]:label:__wt_label_names' \
                        '--label[repository label (must match one repo)]:label:__wt_label_names' \
                        '-p[print main repository path]' \
                        '--project[print main repository path]' \
                        '--hook[run named hook instead of default]:hook:' \
                        '--no-hook[skip hooks]' \
                        '-a[set hook variable KEY=VALUE]:arg:' \
                        '--arg[set hook variable KEY=VALUE]:arg:'
                    ;;
                mv)
                    _arguments \
                        '*-r[filter by repository name]:repository:__wt_repo_names' \
                        '*--repository[filter by repository name]:repository:__wt_repo_names' \
                        '--format[worktree naming format]:format:' \
                        '-n[show what would be moved]' \
                        '--dry-run[show what would be moved]' \
                        '-f[force move locked worktrees]' \
                        '--force[force move locked worktrees]'
                    ;;
                pr)
                    _arguments -C \
                        '1: :->subcmd' \
                        '*:: :->args'
                    case $state in
                        subcmd)
                            local subcommands=(
                                'checkout:Checkout PR (clones if needed)'
                                'create:Create PR for current branch'
                                'merge:Merge PR and clean up worktree'
                                'view:View PR details or open in browser'
                            )
                            _describe 'subcommand' subcommands
                            ;;
                        args)
                            case $words[1] in
                                checkout)
                                    _arguments \
                                        '1:PR number:' \
                                        '2:org/repo (clone mode):' \
                                        '-r[local repo name]:repository:__wt_repo_names' \
                                        '--repository[local repo name]:repository:__wt_repo_names' \
                                        '--forge[forge type]:forge:(github gitlab)' \
                                        '--note[set note on branch]:note:' \
                                        '--hook[run named hook]:hook:' \
                                        '--no-hook[skip post-add hook]' \
                                        '*-a[set hook variable KEY=VALUE]:arg:' \
                                        '*--arg[set hook variable KEY=VALUE]:arg:'
                                    ;;
                                create)
                                    _arguments \
                                        '-i[worktree ID]:id:__wt_worktree_ids' \
                                        '--id[worktree ID]:id:__wt_worktree_ids' \
                                        '-r[repository name]:repository:__wt_repo_names' \
                                        '--repository[repository name]:repository:__wt_repo_names' \
                                        '-t[PR title]:title:' \
                                        '--title[PR title]:title:' \
                                        '-b[PR body]:body:' \
                                        '--body[PR body]:body:' \
                                        '--body-file[read body from file]:file:_files' \
                                        '--base[base branch]:branch:__wt_all_branches' \
                                        '--draft[create as draft PR]' \
                                        '-w[open in browser after creation]' \
                                        '--web[open in browser after creation]'
                                    ;;
                                merge)
                                    _arguments \
                                        '-i[worktree ID]:id:__wt_worktree_ids' \
                                        '--id[worktree ID]:id:__wt_worktree_ids' \
                                        '-r[repository name]:repository:__wt_repo_names' \
                                        '--repository[repository name]:repository:__wt_repo_names' \
                                        '-s[merge strategy]:strategy:(squash rebase merge)' \
                                        '--strategy[merge strategy]:strategy:(squash rebase merge)' \
                                        '-k[keep worktree and branch after merge]' \
                                        '--keep[keep worktree and branch after merge]' \
                                        '--hook[run named hook]:hook:' \
                                        '--no-hook[skip post-merge hook]' \
                                        '*-a[set hook variable KEY=VALUE]:arg:' \
                                        '*--arg[set hook variable KEY=VALUE]:arg:'
                                    ;;
                                view)
                                    _arguments \
                                        '-i[worktree ID]:id:__wt_worktree_ids' \
                                        '--id[worktree ID]:id:__wt_worktree_ids' \
                                        '-r[repository name]:repository:__wt_repo_names' \
                                        '--repository[repository name]:repository:__wt_repo_names' \
                                        '-w[open PR in browser]' \
                                        '--web[open PR in browser]'
                                    ;;
                            esac
                            ;;
                    esac
                    ;;
                note)
                    # note get is default, so flags work directly
                    _arguments -C \
                        '-i[worktree ID]:id:__wt_worktree_ids' \
                        '--id[worktree ID]:id:__wt_worktree_ids' \
                        '-r[repository name]:repository:__wt_repo_names' \
                        '--repository[repository name]:repository:__wt_repo_names' \
                        '1: :->subcmd' \
                        '*:: :->args'
                    case $state in
                        subcmd)
                            local subcommands=(
                                'set:Set a note on a branch'
                                'get:Get the note for a branch'
                                'clear:Clear the note from a branch'
                            )
                            _describe 'subcommand' subcommands
                            ;;
                        args)
                            case $words[1] in
                                set)
                                    _arguments \
                                        '1:note text:' \
                                        '-i[worktree ID]:id:__wt_worktree_ids' \
                                        '--id[worktree ID]:id:__wt_worktree_ids' \
                                        '-r[repository name]:repository:__wt_repo_names' \
                                        '--repository[repository name]:repository:__wt_repo_names'
                                    ;;
                                get|clear)
                                    _arguments \
                                        '-i[worktree ID]:id:__wt_worktree_ids' \
                                        '--id[worktree ID]:id:__wt_worktree_ids' \
                                        '-r[repository name]:repository:__wt_repo_names' \
                                        '--repository[repository name]:repository:__wt_repo_names'
                                    ;;
                            esac
                            ;;
                    esac
                    ;;
                label)
                    # label list is default, so flags work directly
                    _arguments -C \
                        '*-r[repository name]:repository:__wt_repo_names' \
                        '*--repository[repository name]:repository:__wt_repo_names' \
                        '-g[list all labels across repos]' \
                        '--global[list all labels across repos]' \
                        '1: :->subcmd' \
                        '*:: :->args'
                    case $state in
                        subcmd)
                            local subcommands=(
                                'add:Add a label to a repository'
                                'remove:Remove a label from a repository'
                                'list:List labels for a repository'
                                'clear:Clear all labels from a repository'
                            )
                            _describe 'subcommand' subcommands
                            ;;
                        args)
                            case $words[1] in
                                add|remove)
                                    _arguments \
                                        '1:label:' \
                                        '*-r[repository name]:repository:__wt_repo_names' \
                                        '*--repository[repository name]:repository:__wt_repo_names'
                                    ;;
                                list)
                                    _arguments \
                                        '*-r[repository name]:repository:__wt_repo_names' \
                                        '*--repository[repository name]:repository:__wt_repo_names' \
                                        '-g[list all labels across repos]' \
                                        '--global[list all labels across repos]'
                                    ;;
                                clear)
                                    _arguments \
                                        '*-r[repository name]:repository:__wt_repo_names' \
                                        '*--repository[repository name]:repository:__wt_repo_names'
                                    ;;
                            esac
                            ;;
                    esac
                    ;;
                hook)
                    _arguments \
                        '1:hook name:__wt_hook_names' \
                        '*-i[worktree ID]:id:__wt_worktree_ids' \
                        '*--id[worktree ID]:id:__wt_worktree_ids' \
                        '*-r[repository name]:repository:__wt_repo_names' \
                        '*--repository[repository name]:repository:__wt_repo_names' \
                        '*-l[target repos by label]:label:__wt_label_names' \
                        '*--label[target repos by label]:label:__wt_label_names' \
                        '*-a[set hook variable KEY=VALUE]:arg:' \
                        '*--arg[set hook variable KEY=VALUE]:arg:' \
                        '-n[print command without executing]' \
                        '--dry-run[print command without executing]'
                    ;;
                config)
                    _arguments -C \
                        '1: :->subcmd' \
                        '*:: :->args'
                    case $state in
                        subcmd)
                            local subcommands=(
                                'init:Create default config file'
                                'show:Show effective configuration'
                                'hooks:List available hooks'
                            )
                            _describe 'subcommand' subcommands
                            ;;
                        args)
                            case $words[1] in
                                init)
                                    _arguments \
                                        '1:worktree directory:_files -/' \
                                        '-f[overwrite existing config]' \
                                        '--force[overwrite existing config]' \
                                        '-s[print config to stdout]' \
                                        '--stdout[print config to stdout]'
                                    ;;
                                show)
                                    _arguments \
                                        '--json[output as JSON]'
                                    ;;
                                hooks)
                                    _arguments \
                                        '--json[output as JSON]'
                                    ;;
                            esac
                            ;;
                    esac
                    ;;
                completion)
                    _arguments \
                        '1:shell:(fish bash zsh)'
                    ;;
                doctor)
                    _arguments \
                        '--fix[auto-fix recoverable issues]' \
                        '--reset[rebuild cache from scratch]'
                    ;;
            esac
            ;;
    esac
}

# Helper: complete all branches (local + remote)
__wt_all_branches() {
    local branches
    branches=(${(f)"$(git branch --all --format='%(refname:short)' 2>/dev/null | sed 's|origin/||' | sort -u)"})
    _describe 'branch' branches
}

# Helper: complete local branches only
__wt_local_branches() {
    local branches
    branches=(${(f)"$(git branch --format='%(refname:short)' 2>/dev/null)"})
    _describe 'branch' branches
}

# Helper: complete worktree IDs only
__wt_worktree_ids() {
    local ids
    ids=(${(f)"$(wt list 2>/dev/null | awk '{print $1}')"})
    _describe 'worktree ID' ids
}

# Helper: complete hook names
__wt_hook_names() {
    local hooks
    hooks=(${(f)"$(wt config hooks 2>/dev/null | awk '{print $1}')"})
    _describe 'hook name' hooks
}

# Helper: complete repo names in worktree_dir
__wt_repo_names() {
    local dir repos
    dir="$WT_WORKTREE_DIR"
    if [[ -z "$dir" ]]; then
        local config_file=~/.config/wt/config.toml
        if [[ -f "$config_file" ]]; then
            dir=$(grep '^worktree_dir' "$config_file" 2>/dev/null | sed 's/.*= *"\?\([^"]*\)"\?/\1/' | sed "s|~|$HOME|")
        fi
    fi
    if [[ -d "$dir" ]]; then
        repos=()
        for d in "$dir"/*/; do
            if [[ -d "$d/.git" ]] || [[ -f "$d/.git" ]]; then
                repos+=(${d:h:t})
            fi
        done
        _describe 'repository' repos
    fi
}

# Helper: complete label names from repos in worktree_dir
__wt_label_names() {
    local dir labels
    dir="$WT_WORKTREE_DIR"
    if [[ -z "$dir" ]]; then
        local config_file=~/.config/wt/config.toml
        if [[ -f "$config_file" ]]; then
            dir=$(grep '^worktree_dir' "$config_file" 2>/dev/null | sed 's/.*= *"\?\([^"]*\)"\?/\1/' | sed "s|~|$HOME|")
        fi
    fi
    if [[ -d "$dir" ]]; then
        labels=()
        for d in "$dir"/*/; do
            if [[ -d "$d/.git" ]] || [[ -f "$d/.git" ]]; then
                local repo_labels=$(git -C "$d" config --local wt.labels 2>/dev/null)
                if [[ -n "$repo_labels" ]]; then
                    labels+=(${(s:,:)repo_labels})
                fi
            fi
        done
        labels=(${(u)labels})  # unique
        _describe 'label' labels
    fi
}

_wt "$@"
`

const fishCompletions = `# wt completions - supports fish autosuggestions and tab completion
complete -c wt -f

# Subcommands (shown in completions and autosuggestions)
complete -c wt -n "not __fish_seen_subcommand_from add a prune p list ls show s repos r exec x cd mv note label hook pr config completion doctor" -a "add" -d "Add worktree for branch"
complete -c wt -n "not __fish_seen_subcommand_from add a prune p list ls show s repos r exec x cd mv note label hook pr config completion doctor" -a "a" -d "Add worktree (alias)"
complete -c wt -n "not __fish_seen_subcommand_from add a prune p list ls show s repos r exec x cd mv note label hook pr config completion doctor" -a "prune" -d "Prune merged worktrees"
complete -c wt -n "not __fish_seen_subcommand_from add a prune p list ls show s repos r exec x cd mv note label hook pr config completion doctor" -a "p" -d "Prune (alias)"
complete -c wt -n "not __fish_seen_subcommand_from add a prune p list ls show s repos r exec x cd mv note label hook pr config completion doctor" -a "list" -d "List worktrees with stable IDs"
complete -c wt -n "not __fish_seen_subcommand_from add a prune p list ls show s repos r exec x cd mv note label hook pr config completion doctor" -a "ls" -d "List worktrees (alias)"
complete -c wt -n "not __fish_seen_subcommand_from add a prune p list ls show s repos r exec x cd mv note label hook pr config completion doctor" -a "show" -d "Show worktree details"
complete -c wt -n "not __fish_seen_subcommand_from add a prune p list ls show s repos r exec x cd mv note label hook pr config completion doctor" -a "s" -d "Show (alias)"
complete -c wt -n "not __fish_seen_subcommand_from add a prune p list ls show s repos r exec x cd mv note label hook pr config completion doctor" -a "repos" -d "List repositories"
complete -c wt -n "not __fish_seen_subcommand_from add a prune p list ls show s repos r exec x cd mv note label hook pr config completion doctor" -a "r" -d "Repos (alias)"
complete -c wt -n "not __fish_seen_subcommand_from add a prune p list ls show s repos r exec x cd mv note label hook pr config completion doctor" -a "exec" -d "Run command in worktree by ID"
complete -c wt -n "not __fish_seen_subcommand_from add a prune p list ls show s repos r exec x cd mv note label hook pr config completion doctor" -a "x" -d "Exec (alias)"
complete -c wt -n "not __fish_seen_subcommand_from add a prune p list ls show s repos r exec x cd mv note label hook pr config completion doctor" -a "cd" -d "Print worktree path"
complete -c wt -n "not __fish_seen_subcommand_from add a prune p list ls show s repos r exec x cd mv note label hook pr config completion doctor" -a "mv" -d "Move worktrees to another directory"
complete -c wt -n "not __fish_seen_subcommand_from add a prune p list ls show s repos r exec x cd mv note label hook pr config completion doctor" -a "note" -d "Manage branch notes"
complete -c wt -n "not __fish_seen_subcommand_from add a prune p list ls show s repos r exec x cd mv note label hook pr config completion doctor" -a "label" -d "Manage repository labels"
complete -c wt -n "not __fish_seen_subcommand_from add a prune p list ls show s repos r exec x cd mv note label hook pr config completion doctor" -a "hook" -d "Run configured hook"
complete -c wt -n "not __fish_seen_subcommand_from add a prune p list ls show s repos r exec x cd mv note label hook pr config completion doctor" -a "pr" -d "Work with PRs"
complete -c wt -n "not __fish_seen_subcommand_from add a prune p list ls show s repos r exec x cd mv note label hook pr config completion doctor" -a "config" -d "Manage configuration"
complete -c wt -n "not __fish_seen_subcommand_from add a prune p list ls show s repos r exec x cd mv note label hook pr config completion doctor" -a "completion" -d "Generate completion script"
complete -c wt -n "not __fish_seen_subcommand_from add a prune p list ls show s repos r exec x cd mv note label hook pr config completion doctor" -a "doctor" -d "Diagnose and repair cache"

# add: branch name (positional), then flags
complete -c wt -n "__fish_seen_subcommand_from add a; and not __fish_seen_argument" -a "(git branch --all --format='%(refname:short)' 2>/dev/null | string replace 'origin/' '' | sort -u)" -d "Branch name"
complete -c wt -n "__fish_seen_subcommand_from add a" -s b -l new-branch -d "Create new branch"
complete -c wt -n "__fish_seen_subcommand_from add a" -s r -l repository -r -a "(__wt_list_repos)" -d "Repository name (repeatable)"
complete -c wt -n "__fish_seen_subcommand_from add a" -s l -l label -r -a "(__wt_list_labels)" -d "Target repos by label (repeatable)"
complete -c wt -n "__fish_seen_subcommand_from add a" -l base -r -a "(git branch --all --format='%(refname:short)' 2>/dev/null | string replace 'origin/' '' | sort -u)" -d "Base branch to create from"
complete -c wt -n "__fish_seen_subcommand_from add a" -s f -l fetch -d "Fetch base branch before creating"
complete -c wt -n "__fish_seen_subcommand_from add a" -l note -r -d "Set note on branch"
complete -c wt -n "__fish_seen_subcommand_from add a" -l hook -d "Run named hook instead of default"
complete -c wt -n "__fish_seen_subcommand_from add a" -l no-hook -d "Skip post-add hook"
complete -c wt -n "__fish_seen_subcommand_from add a" -s a -l arg -r -d "Set hook variable KEY=VALUE"

# prune: --id flag, then other flags
complete -c wt -n "__fish_seen_subcommand_from prune p" -s i -l id -r -a "(__wt_worktree_ids)" -d "Worktree ID to remove"
complete -c wt -n "__fish_seen_subcommand_from prune p" -s n -l dry-run -d "Preview without removing"
complete -c wt -n "__fish_seen_subcommand_from prune p" -s f -l force -d "Force remove even if not merged/dirty"
complete -c wt -n "__fish_seen_subcommand_from prune p" -s c -l include-clean -d "Also remove clean worktrees"
complete -c wt -n "__fish_seen_subcommand_from prune p" -s g -l global -d "Prune all worktrees (not just current repo)"
complete -c wt -n "__fish_seen_subcommand_from prune p" -s v -l verbose -d "Show skipped worktrees with reasons"
complete -c wt -n "__fish_seen_subcommand_from prune p" -s R -l refresh -d "Fetch origin and refresh PR status"
complete -c wt -n "__fish_seen_subcommand_from prune p" -l reset-cache -d "Clear cache and reset IDs from 1"
complete -c wt -n "__fish_seen_subcommand_from prune p" -l hook -d "Run named hook instead of default"
complete -c wt -n "__fish_seen_subcommand_from prune p" -l no-hook -d "Skip post-removal hooks"
complete -c wt -n "__fish_seen_subcommand_from prune p" -s a -l arg -r -d "Set hook variable KEY=VALUE"

# list: flags only (no positional args)
complete -c wt -n "__fish_seen_subcommand_from list ls" -l json -d "Output as JSON"
complete -c wt -n "__fish_seen_subcommand_from list ls" -s g -l global -d "Show all worktrees (not just current repo)"
complete -c wt -n "__fish_seen_subcommand_from list ls" -s s -l sort -r -a "id repo branch commit" -d "Sort by field"
complete -c wt -n "__fish_seen_subcommand_from list ls" -s R -l refresh -d "Fetch origin and refresh PR status"
complete -c wt -n "__fish_seen_subcommand_from list ls" -s r -l repository -r -a "(__wt_list_repos)" -d "Filter by repository name (repeatable)"
complete -c wt -n "__fish_seen_subcommand_from list ls" -s l -l label -r -a "(__wt_list_labels)" -d "Filter by label (repeatable)"

# repos: list repositories
complete -c wt -n "__fish_seen_subcommand_from repos r" -s l -l label -r -a "(__wt_list_labels)" -d "Filter by label"
complete -c wt -n "__fish_seen_subcommand_from repos r" -s s -l sort -r -a "name branch worktrees label" -d "Sort by field"
complete -c wt -n "__fish_seen_subcommand_from repos r" -l json -d "Output as JSON"

# show: --id or --repository flag (optional), then other flags
complete -c wt -n "__fish_seen_subcommand_from show s" -s i -l id -r -a "(__wt_worktree_ids)" -d "Worktree ID"
complete -c wt -n "__fish_seen_subcommand_from show s" -s r -l repository -r -a "(__wt_list_repos)" -d "Repository name"
complete -c wt -n "__fish_seen_subcommand_from show s" -s R -l refresh -d "Refresh PR status from API"
complete -c wt -n "__fish_seen_subcommand_from show s" -l json -d "Output as JSON"

# exec: --id, -r, -l flags, then -- command
complete -c wt -n "__fish_seen_subcommand_from exec x" -s i -l id -r -a "(__wt_worktree_ids)" -d "Worktree ID"
complete -c wt -n "__fish_seen_subcommand_from exec x" -s r -l repository -r -a "(__wt_list_repos)" -d "Repository name (repeatable)"
complete -c wt -n "__fish_seen_subcommand_from exec x" -s l -l label -r -a "(__wt_list_labels)" -d "Target repos by label (repeatable)"

# cd: --id, -r, or -l flag, then flags
complete -c wt -n "__fish_seen_subcommand_from cd" -s i -l id -r -a "(__wt_worktree_ids)" -d "Worktree ID"
complete -c wt -n "__fish_seen_subcommand_from cd" -s r -l repository -r -a "(__wt_list_repos)" -d "Repository name"
complete -c wt -n "__fish_seen_subcommand_from cd" -s l -l label -r -a "(__wt_list_labels)" -d "Repository label (must match one repo)"
complete -c wt -n "__fish_seen_subcommand_from cd" -s p -l project -d "Print main repository path"
complete -c wt -n "__fish_seen_subcommand_from cd" -l hook -d "Run named hook instead of default"
complete -c wt -n "__fish_seen_subcommand_from cd" -l no-hook -d "Skip hooks"
complete -c wt -n "__fish_seen_subcommand_from cd" -s a -l arg -r -d "Set hook variable KEY=VALUE"

# mv: flags (destination from config/env)
complete -c wt -n "__fish_seen_subcommand_from mv" -s r -l repository -r -a "(__wt_list_repos)" -d "Filter by repository name (repeatable)"
complete -c wt -n "__fish_seen_subcommand_from mv" -l format -d "Worktree naming format"
complete -c wt -n "__fish_seen_subcommand_from mv" -s n -l dry-run -d "Show what would be moved"
complete -c wt -n "__fish_seen_subcommand_from mv" -s f -l force -d "Force move locked worktrees"

# note: subcommands (get is default, so flags work directly on note)
complete -c wt -n "__fish_seen_subcommand_from note; and not __fish_seen_subcommand_from set get clear" -a "set" -d "Set a note on a branch"
complete -c wt -n "__fish_seen_subcommand_from note; and not __fish_seen_subcommand_from set get clear" -a "get" -d "Get the note for a branch"
complete -c wt -n "__fish_seen_subcommand_from note; and not __fish_seen_subcommand_from set get clear" -a "clear" -d "Clear the note from a branch"
# note: --id and --repository flags work directly (get is default)
complete -c wt -n "__fish_seen_subcommand_from note; and not __fish_seen_subcommand_from set get clear" -s i -l id -r -a "(__wt_worktree_ids)" -d "Worktree ID"
complete -c wt -n "__fish_seen_subcommand_from note; and not __fish_seen_subcommand_from set get clear" -s r -l repository -r -a "(__wt_list_repos)" -d "Repository name"
# note set/get/clear: --id and --repository flags (optional)
complete -c wt -n "__fish_seen_subcommand_from note; and __fish_seen_subcommand_from set get clear" -s i -l id -r -a "(__wt_worktree_ids)" -d "Worktree ID"
complete -c wt -n "__fish_seen_subcommand_from note; and __fish_seen_subcommand_from set get clear" -s r -l repository -r -a "(__wt_list_repos)" -d "Repository name"

# label: subcommands (list is default, so flags work directly on label)
complete -c wt -n "__fish_seen_subcommand_from label; and not __fish_seen_subcommand_from add a remove list clear" -a "add" -d "Add a label to a repository"
complete -c wt -n "__fish_seen_subcommand_from label; and not __fish_seen_subcommand_from add a remove list clear" -a "remove" -d "Remove a label from a repository"
complete -c wt -n "__fish_seen_subcommand_from label; and not __fish_seen_subcommand_from add a remove list clear" -a "list" -d "List labels for a repository"
complete -c wt -n "__fish_seen_subcommand_from label; and not __fish_seen_subcommand_from add a remove list clear" -a "clear" -d "Clear all labels from a repository"
# label: flags work directly (list is default)
complete -c wt -n "__fish_seen_subcommand_from label; and not __fish_seen_subcommand_from add a remove list clear" -s r -l repository -r -a "(__wt_list_repos)" -d "Repository name (repeatable)"
complete -c wt -n "__fish_seen_subcommand_from label; and not __fish_seen_subcommand_from add a remove list clear" -s g -l global -d "List all labels across repos"
# label add/remove/clear: -r flag
complete -c wt -n "__fish_seen_subcommand_from label; and __fish_seen_subcommand_from add a remove clear" -s r -l repository -r -a "(__wt_list_repos)" -d "Repository name (repeatable)"
# label list: -r, -g flags
complete -c wt -n "__fish_seen_subcommand_from label; and __fish_seen_subcommand_from list ls" -s r -l repository -r -a "(__wt_list_repos)" -d "Repository name (repeatable)"
complete -c wt -n "__fish_seen_subcommand_from label; and __fish_seen_subcommand_from list ls" -s g -l global -d "List all labels across repos"

# hook: multiple hook names supported, then --id/-r/-l (optional), then flags
complete -c wt -n "__fish_seen_subcommand_from hook" -a "(__wt_hook_names)" -d "Hook name"
complete -c wt -n "__fish_seen_subcommand_from hook" -s i -l id -r -a "(__wt_worktree_ids)" -d "Worktree ID"
complete -c wt -n "__fish_seen_subcommand_from hook" -s r -l repository -r -a "(__wt_list_repos)" -d "Repository name (repeatable)"
complete -c wt -n "__fish_seen_subcommand_from hook" -s l -l label -r -a "(__wt_list_labels)" -d "Target repos by label (repeatable)"
complete -c wt -n "__fish_seen_subcommand_from hook" -s a -l arg -r -d "Set hook variable KEY=VALUE"
complete -c wt -n "__fish_seen_subcommand_from hook" -s n -l dry-run -d "Print command without executing"

# pr: subcommands
complete -c wt -n "__fish_seen_subcommand_from pr; and not __fish_seen_subcommand_from checkout create merge view" -a "checkout" -d "Checkout PR (clones if needed)"
complete -c wt -n "__fish_seen_subcommand_from pr; and not __fish_seen_subcommand_from checkout create merge view" -a "create" -d "Create PR for current branch"
complete -c wt -n "__fish_seen_subcommand_from pr; and not __fish_seen_subcommand_from checkout create merge view" -a "merge" -d "Merge PR and clean up worktree"
complete -c wt -n "__fish_seen_subcommand_from pr; and not __fish_seen_subcommand_from checkout create merge view" -a "view" -d "View PR details or open in browser"
# pr checkout: PR number (first positional), then org/repo (second positional for clone mode), then flags
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from checkout" -a "(gh pr list --json number,title --jq '.[] | \"\\(.number)\t\\(.title)\"' 2>/dev/null)" -d "PR number"
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from checkout" -s r -l repository -r -a "(__wt_list_repos)" -d "Local repo name"
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from checkout" -l forge -r -a "github gitlab" -d "Forge type (for cloning)"
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from checkout" -l note -r -d "Set note on branch"
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from checkout" -l hook -d "Run named hook instead of default"
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from checkout" -l no-hook -d "Skip post-add hook"
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from checkout" -s a -l arg -r -d "Set hook variable KEY=VALUE"
# pr create: --id or --repository flag (optional), then flags
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from create" -s i -l id -r -a "(__wt_worktree_ids)" -d "Worktree ID"
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from create" -s r -l repository -r -a "(__wt_list_repos)" -d "Repository name"
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from create" -s t -l title -r -d "PR title"
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from create" -s b -l body -r -d "PR body (use - to read from stdin)"
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from create" -l body-file -r -a "(__fish_complete_path)" -d "Read body from file"
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from create" -l base -r -a "(git branch --all --format='%(refname:short)' 2>/dev/null | string replace 'origin/' '' | sort -u)" -d "Base branch"
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from create" -l draft -d "Create as draft PR"
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from create" -s w -l web -d "Open in browser after creation"
# pr merge: --id or --repository flag (optional), then flags
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from merge" -s i -l id -r -a "(__wt_worktree_ids)" -d "Worktree ID"
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from merge" -s r -l repository -r -a "(__wt_list_repos)" -d "Repository name"
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from merge" -s s -l strategy -r -a "squash rebase merge" -d "Merge strategy"
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from merge" -s k -l keep -d "Keep worktree and branch after merge"
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from merge" -l hook -d "Run named hook instead of default"
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from merge" -l no-hook -d "Skip post-merge hook"
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from merge" -s a -l arg -r -d "Set hook variable KEY=VALUE"
# pr view: --id or --repository flag (optional), --web flag
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from view" -s i -l id -r -a "(__wt_worktree_ids)" -d "Worktree ID"
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from view" -s r -l repository -r -a "(__wt_list_repos)" -d "Repository name"
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from view" -s w -l web -d "Open PR in browser"

# Helper function to list repos in worktree_dir
function __wt_list_repos
    set -l dir "$WT_WORKTREE_DIR"
    if test -z "$dir"
        set -l config_file ~/.config/wt/config.toml
        if test -f "$config_file"
            set dir (grep '^worktree_dir' "$config_file" 2>/dev/null | sed 's/.*= *"\?\([^"]*\)"\?/\1/' | string replace '~' "$HOME")
        end
    end
    if test -d "$dir"
        for d in $dir/*/
            if test -d "$d/.git" -o -f "$d/.git"
                basename $d
            end
        end
    end
end

# Helper function to list worktree IDs only
function __wt_worktree_ids
    wt list 2>/dev/null | awk '{print $1}'
end

# Helper function to list hook names
function __wt_hook_names
    wt config hooks 2>/dev/null | awk '{print $1}'
end

# Helper function to list labels from repos in worktree_dir
function __wt_list_labels
    set -l dir "$WT_WORKTREE_DIR"
    if test -z "$dir"
        set -l config_file ~/.config/wt/config.toml
        if test -f "$config_file"
            set dir (grep '^worktree_dir' "$config_file" 2>/dev/null | sed 's/.*= *"\?\([^"]*\)"\?/\1/' | string replace '~' "$HOME")
        end
    end
    if test -d "$dir"
        for d in $dir/*/
            if test -d "$d/.git" -o -f "$d/.git"
                set -l repo_labels (git -C "$d" config --local wt.labels 2>/dev/null)
                if test -n "$repo_labels"
                    string split ',' $repo_labels
                end
            end
        end | sort -u
    end
end

# config: subcommands
complete -c wt -n "__fish_seen_subcommand_from config; and not __fish_seen_subcommand_from init show hooks" -a "init" -d "Create default config file"
complete -c wt -n "__fish_seen_subcommand_from config; and not __fish_seen_subcommand_from init show hooks" -a "show" -d "Show effective configuration"
complete -c wt -n "__fish_seen_subcommand_from config; and not __fish_seen_subcommand_from init show hooks" -a "hooks" -d "List available hooks"
complete -c wt -n "__fish_seen_subcommand_from config; and __fish_seen_subcommand_from init" -xa "(__fish_complete_directories)" -d "Worktree directory"
complete -c wt -n "__fish_seen_subcommand_from config; and __fish_seen_subcommand_from init" -s f -l force -d "Overwrite existing config file"
complete -c wt -n "__fish_seen_subcommand_from config; and __fish_seen_subcommand_from init" -s s -l stdout -d "Print config to stdout"
complete -c wt -n "__fish_seen_subcommand_from config; and __fish_seen_subcommand_from show s" -l json -d "Output as JSON"
complete -c wt -n "__fish_seen_subcommand_from config; and __fish_seen_subcommand_from hooks" -l json -d "Output as JSON"

# completion
complete -c wt -n "__fish_seen_subcommand_from completion" -a "fish" -d "Fish shell"
complete -c wt -n "__fish_seen_subcommand_from completion" -a "bash" -d "Bash shell"
complete -c wt -n "__fish_seen_subcommand_from completion" -a "zsh" -d "Zsh shell"

# doctor: diagnose and repair cache
complete -c wt -n "__fish_seen_subcommand_from doctor" -l fix -d "Auto-fix recoverable issues"
complete -c wt -n "__fish_seen_subcommand_from doctor" -l reset -d "Rebuild cache from scratch"

# Global help
complete -c wt -s h -l help -d "Show help message"
`
