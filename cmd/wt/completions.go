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

    local commands="add prune list show repos exec cd mv note label hook pr config completion"

    # Handle subcommand-specific completions
    case "${words[1]}" in
        add)
            case "$prev" in
                -d|--dir)
                    COMPREPLY=($(compgen -d -- "$cur"))
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
                COMPREPLY=($(compgen -W "-b --new-branch -r --repository -l --label -d --dir --base -f --fetch --note --hook --no-hook -a --arg" -- "$cur"))
            fi
            ;;
        prune)
            case "$prev" in
                -d|--dir)
                    COMPREPLY=($(compgen -d -- "$cur"))
                    return
                    ;;
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
            COMPREPLY=($(compgen -W "-i --id -d --dir -n --dry-run -f --force -c --include-clean -g --global -R --refresh --reset-cache --hook --no-hook -a --arg" -- "$cur"))
            ;;
        list)
            case "$prev" in
                -d|--dir)
                    COMPREPLY=($(compgen -d -- "$cur"))
                    return
                    ;;
                -s|--sort)
                    COMPREPLY=($(compgen -W "id repo branch" -- "$cur"))
                    return
                    ;;
            esac
            COMPREPLY=($(compgen -W "-d --dir --json -g --global -s --sort -R --refresh" -- "$cur"))
            ;;
        repos)
            case "$prev" in
                -d|--dir)
                    COMPREPLY=($(compgen -d -- "$cur"))
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
            COMPREPLY=($(compgen -W "-d --dir -l --label --json" -- "$cur"))
            ;;
        show)
            case "$prev" in
                -d|--dir)
                    COMPREPLY=($(compgen -d -- "$cur"))
                    return
                    ;;
                -i|--id)
                    # Complete worktree IDs only
                    local ids=$(wt list 2>/dev/null | awk '{print $1}')
                    COMPREPLY=($(compgen -W "$ids" -- "$cur"))
                    return
                    ;;
            esac
            COMPREPLY=($(compgen -W "-i --id -d --dir -R --refresh --json" -- "$cur"))
            ;;
        exec)
            case "$prev" in
                -d|--dir)
                    COMPREPLY=($(compgen -d -- "$cur"))
                    return
                    ;;
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
            COMPREPLY=($(compgen -W "-i --id -r --repository -l --label -d --dir" -- "$cur"))
            ;;
        cd)
            case "$prev" in
                -d|--dir)
                    COMPREPLY=($(compgen -d -- "$cur"))
                    return
                    ;;
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
                --hook|-a|--arg)
                    return
                    ;;
            esac
            COMPREPLY=($(compgen -W "-i --id -r --repository -d --dir -p --project --hook --no-hook -a --arg" -- "$cur"))
            ;;
        mv)
            case "$prev" in
                -d|--dir)
                    COMPREPLY=($(compgen -d -- "$cur"))
                    return
                    ;;
                --format)
                    return
                    ;;
            esac
            COMPREPLY=($(compgen -W "-d --dir --format -n --dry-run -f --force" -- "$cur"))
            ;;
        pr)
            if [[ $cword -eq 2 ]]; then
                COMPREPLY=($(compgen -W "open clone merge" -- "$cur"))
            elif [[ "${words[2]}" == "open" ]]; then
                case "$prev" in
                    -d|--dir)
                        COMPREPLY=($(compgen -d -- "$cur"))
                        return
                        ;;
                    --hook)
                        return
                        ;;
                esac
                COMPREPLY=($(compgen -W "-d --dir --hook --no-hook -a --arg" -- "$cur"))
            elif [[ "${words[2]}" == "clone" ]]; then
                case "$prev" in
                    -d|--dir)
                        COMPREPLY=($(compgen -d -- "$cur"))
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
                COMPREPLY=($(compgen -W "-d --dir --forge --note --hook --no-hook -a --arg" -- "$cur"))
            elif [[ "${words[2]}" == "merge" ]]; then
                case "$prev" in
                    -d|--dir)
                        COMPREPLY=($(compgen -d -- "$cur"))
                        return
                        ;;
                    -i|--id)
                        # Complete worktree IDs only
                        local ids=$(wt list 2>/dev/null | awk '{print $1}')
                        COMPREPLY=($(compgen -W "$ids" -- "$cur"))
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
                COMPREPLY=($(compgen -W "-i --id -d --dir -s --strategy -k --keep --hook --no-hook -a --arg" -- "$cur"))
            fi
            ;;
        note)
            # note get is default, so flags work directly on note
            case "$prev" in
                -d|--dir)
                    COMPREPLY=($(compgen -d -- "$cur"))
                    return
                    ;;
                -i|--id)
                    # Complete worktree IDs only
                    local ids=$(wt list 2>/dev/null | awk '{print $1}')
                    COMPREPLY=($(compgen -W "$ids" -- "$cur"))
                    return
                    ;;
            esac
            if [[ $cword -eq 2 ]]; then
                COMPREPLY=($(compgen -W "set get clear -i --id -d --dir" -- "$cur"))
            elif [[ "${words[2]}" == "set" ]]; then
                COMPREPLY=($(compgen -W "-i --id -d --dir" -- "$cur"))
            elif [[ "${words[2]}" == "get" ]] || [[ "${words[2]}" == "clear" ]]; then
                COMPREPLY=($(compgen -W "-i --id -d --dir" -- "$cur"))
            else
                COMPREPLY=($(compgen -W "-i --id -d --dir" -- "$cur"))
            fi
            ;;
        label)
            # label list is default, so flags work directly on label
            case "$prev" in
                -d|--dir)
                    COMPREPLY=($(compgen -d -- "$cur"))
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
                COMPREPLY=($(compgen -W "add remove list clear -r --repository -d --dir -g --global" -- "$cur"))
            elif [[ "${words[2]}" == "add" ]] || [[ "${words[2]}" == "remove" ]]; then
                COMPREPLY=($(compgen -W "-r --repository -d --dir" -- "$cur"))
            elif [[ "${words[2]}" == "list" ]]; then
                COMPREPLY=($(compgen -W "-r --repository -d --dir -g --global" -- "$cur"))
            elif [[ "${words[2]}" == "clear" ]]; then
                COMPREPLY=($(compgen -W "-r --repository -d --dir" -- "$cur"))
            else
                COMPREPLY=($(compgen -W "-r --repository -d --dir -g --global" -- "$cur"))
            fi
            ;;
        hook)
            case "$prev" in
                -d|--dir)
                    COMPREPLY=($(compgen -d -- "$cur"))
                    return
                    ;;
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
            COMPREPLY=($(compgen -W "$hooks -i --id -r --repository -l --label -d --dir -a --arg" -- "$cur"))
            ;;
        config)
            if [[ $cword -eq 2 ]]; then
                COMPREPLY=($(compgen -W "init show hooks" -- "$cur"))
            elif [[ "${words[2]}" == "init" ]]; then
                COMPREPLY=($(compgen -W "-f --force" -- "$cur"))
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
                'prune:Prune merged worktrees'
                'list:List worktrees with stable IDs'
                'show:Show worktree details'
                'repos:List repositories'
                'exec:Run command in worktree by ID'
                'cd:Print worktree path for shell scripting'
                'mv:Move worktrees to another directory'
                'note:Manage branch notes'
                'label:Manage repository labels'
                'hook:Run configured hook'
                'pr:Work with GitHub PRs'
                'config:Manage configuration'
                'completion:Generate completion script'
            )
            _describe 'command' commands
            ;;
        args)
            case $words[1] in
                add)
                    _arguments \
                        '1:branch:__wt_all_branches' \
                        '-b[create new branch]' \
                        '--new-branch[create new branch]' \
                        '*-r[repository name]:repository:__wt_repo_names' \
                        '*--repository[repository name]:repository:__wt_repo_names' \
                        '*-l[target repos by label]:label:__wt_label_names' \
                        '*--label[target repos by label]:label:__wt_label_names' \
                        '-d[target directory]:directory:_files -/' \
                        '--dir[target directory]:directory:_files -/' \
                        '--base[base branch to create from]:branch:__wt_all_branches' \
                        '-f[fetch base branch before creating]' \
                        '--fetch[fetch base branch before creating]' \
                        '--note[set note on branch]:note:' \
                        '--hook[run named hook]:hook:' \
                        '--no-hook[skip post-add hook]' \
                        '*-a[set hook variable KEY=VALUE]:arg:' \
                        '*--arg[set hook variable KEY=VALUE]:arg:'
                    ;;
                prune)
                    _arguments \
                        '-i[worktree ID to remove]:id:__wt_worktree_ids' \
                        '--id[worktree ID to remove]:id:__wt_worktree_ids' \
                        '-d[target directory]:directory:_files -/' \
                        '--dir[target directory]:directory:_files -/' \
                        '-n[preview without removing]' \
                        '--dry-run[preview without removing]' \
                        '-f[force remove even if not merged/dirty]' \
                        '--force[force remove even if not merged/dirty]' \
                        '-c[also remove clean worktrees]' \
                        '--include-clean[also remove clean worktrees]' \
                        '-g[prune all worktrees, not just current repo]' \
                        '--global[prune all worktrees, not just current repo]' \
                        '-R[fetch origin and refresh PR status]' \
                        '--refresh[fetch origin and refresh PR status]' \
                        '--reset-cache[clear cache and reset IDs from 1]' \
                        '--hook[run named hook]:hook:' \
                        '--no-hook[skip post-removal hooks]' \
                        '*-a[set hook variable KEY=VALUE]:arg:' \
                        '*--arg[set hook variable KEY=VALUE]:arg:'
                    ;;
                list)
                    _arguments \
                        '-d[target directory]:directory:_files -/' \
                        '--dir[target directory]:directory:_files -/' \
                        '--json[output as JSON]' \
                        '-g[show all worktrees]' \
                        '--global[show all worktrees]' \
                        '-s[sort by]:field:(id repo branch)' \
                        '--sort[sort by]:field:(id repo branch)' \
                        '-R[fetch origin and refresh PR status]' \
                        '--refresh[fetch origin and refresh PR status]'
                    ;;
                repos)
                    _arguments \
                        '-d[directory to scan]:directory:_files -/' \
                        '--dir[directory to scan]:directory:_files -/' \
                        '-l[filter by label]:label:__wt_label_names' \
                        '--label[filter by label]:label:__wt_label_names' \
                        '--json[output as JSON]'
                    ;;
                show)
                    _arguments \
                        '-i[worktree ID]:id:__wt_worktree_ids' \
                        '--id[worktree ID]:id:__wt_worktree_ids' \
                        '-d[directory to scan]:directory:_files -/' \
                        '--dir[directory to scan]:directory:_files -/' \
                        '-R[refresh PR status from API]' \
                        '--refresh[refresh PR status from API]' \
                        '--json[output as JSON]'
                    ;;
                exec)
                    _arguments \
                        '*-i[worktree ID]:id:__wt_worktree_ids' \
                        '*--id[worktree ID]:id:__wt_worktree_ids' \
                        '*-r[repository name]:repository:__wt_repo_names' \
                        '*--repository[repository name]:repository:__wt_repo_names' \
                        '*-l[target repos by label]:label:__wt_label_names' \
                        '*--label[target repos by label]:label:__wt_label_names' \
                        '-d[target directory]:directory:_files -/' \
                        '--dir[target directory]:directory:_files -/'
                    ;;
                cd)
                    _arguments \
                        '-i[worktree ID]:id:__wt_worktree_ids' \
                        '--id[worktree ID]:id:__wt_worktree_ids' \
                        '-r[repository name]:repository:__wt_repo_names' \
                        '--repository[repository name]:repository:__wt_repo_names' \
                        '-d[directory to scan]:directory:_files -/' \
                        '--dir[directory to scan]:directory:_files -/' \
                        '-p[print main repository path]' \
                        '--project[print main repository path]' \
                        '--hook[run named hook instead of default]:hook:' \
                        '--no-hook[skip hooks]' \
                        '-a[set hook variable KEY=VALUE]:arg:' \
                        '--arg[set hook variable KEY=VALUE]:arg:'
                    ;;
                mv)
                    _arguments \
                        '-d[destination directory]:directory:_files -/' \
                        '--dir[destination directory]:directory:_files -/' \
                        '--format[worktree naming format]:format:' \
                        '-n[show what would be moved]' \
                        '--dry-run[show what would be moved]' \
                        '-f[force move dirty worktrees]' \
                        '--force[force move dirty worktrees]'
                    ;;
                pr)
                    _arguments -C \
                        '1: :->subcmd' \
                        '*:: :->args'
                    case $state in
                        subcmd)
                            local subcommands=(
                                'open:Checkout PR from existing local repo'
                                'clone:Clone repo and checkout PR'
                                'merge:Merge PR and clean up worktree'
                            )
                            _describe 'subcommand' subcommands
                            ;;
                        args)
                            case $words[1] in
                                add-existing)
                                    _arguments \
                                        '1:PR number:' \
                                        '2:repository:' \
                                        '-d[target directory]:directory:_files -/' \
                                        '--dir[target directory]:directory:_files -/' \
                                        '--hook[run named hook]:hook:' \
                                        '--no-hook[skip post-add hook]' \
                                        '*-a[set hook variable KEY=VALUE]:arg:' \
                                        '*--arg[set hook variable KEY=VALUE]:arg:'
                                    ;;
                                clone)
                                    _arguments \
                                        '1:PR number:' \
                                        '2:repository (org/repo):' \
                                        '-d[target directory]:directory:_files -/' \
                                        '--dir[target directory]:directory:_files -/' \
                                        '--forge[forge type]:forge:(github gitlab)' \
                                        '--note[set note on branch]:note:' \
                                        '--hook[run named hook]:hook:' \
                                        '--no-hook[skip post-add hook]' \
                                        '*-a[set hook variable KEY=VALUE]:arg:' \
                                        '*--arg[set hook variable KEY=VALUE]:arg:'
                                    ;;
                                merge)
                                    _arguments \
                                        '-i[worktree ID]:id:__wt_worktree_ids' \
                                        '--id[worktree ID]:id:__wt_worktree_ids' \
                                        '-d[worktree directory]:directory:_files -/' \
                                        '--dir[worktree directory]:directory:_files -/' \
                                        '-s[merge strategy]:strategy:(squash rebase merge)' \
                                        '--strategy[merge strategy]:strategy:(squash rebase merge)' \
                                        '-k[keep worktree and branch after merge]' \
                                        '--keep[keep worktree and branch after merge]' \
                                        '--hook[run named hook]:hook:' \
                                        '--no-hook[skip post-merge hook]' \
                                        '*-a[set hook variable KEY=VALUE]:arg:' \
                                        '*--arg[set hook variable KEY=VALUE]:arg:'
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
                        '-d[worktree directory]:directory:_files -/' \
                        '--dir[worktree directory]:directory:_files -/' \
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
                                        '-d[worktree directory]:directory:_files -/' \
                                        '--dir[worktree directory]:directory:_files -/'
                                    ;;
                                get|clear)
                                    _arguments \
                                        '-i[worktree ID]:id:__wt_worktree_ids' \
                                        '--id[worktree ID]:id:__wt_worktree_ids' \
                                        '-d[worktree directory]:directory:_files -/' \
                                        '--dir[worktree directory]:directory:_files -/'
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
                        '-d[directory to scan for repos]:directory:_files -/' \
                        '--dir[directory to scan for repos]:directory:_files -/' \
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
                                        '*--repository[repository name]:repository:__wt_repo_names' \
                                        '-d[directory to scan for repos]:directory:_files -/' \
                                        '--dir[directory to scan for repos]:directory:_files -/'
                                    ;;
                                list)
                                    _arguments \
                                        '*-r[repository name]:repository:__wt_repo_names' \
                                        '*--repository[repository name]:repository:__wt_repo_names' \
                                        '-d[directory to scan for repos]:directory:_files -/' \
                                        '--dir[directory to scan for repos]:directory:_files -/' \
                                        '-g[list all labels across repos]' \
                                        '--global[list all labels across repos]'
                                    ;;
                                clear)
                                    _arguments \
                                        '*-r[repository name]:repository:__wt_repo_names' \
                                        '*--repository[repository name]:repository:__wt_repo_names' \
                                        '-d[directory to scan for repos]:directory:_files -/' \
                                        '--dir[directory to scan for repos]:directory:_files -/'
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
                        '-d[directory]:directory:_files -/' \
                        '--dir[directory]:directory:_files -/' \
                        '*-a[set hook variable KEY=VALUE]:arg:' \
                        '*--arg[set hook variable KEY=VALUE]:arg:'
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
                                        '-f[overwrite existing config]' \
                                        '--force[overwrite existing config]'
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
complete -c wt -n "not __fish_seen_subcommand_from add prune list show repos exec cd mv note label hook pr config completion" -a "add" -d "Add worktree for branch"
complete -c wt -n "not __fish_seen_subcommand_from add prune list show repos exec cd mv note label hook pr config completion" -a "prune" -d "Prune merged worktrees"
complete -c wt -n "not __fish_seen_subcommand_from add prune list show repos exec cd mv note label hook pr config completion" -a "list" -d "List worktrees with stable IDs"
complete -c wt -n "not __fish_seen_subcommand_from add prune list show repos exec cd mv note label hook pr config completion" -a "show" -d "Show worktree details"
complete -c wt -n "not __fish_seen_subcommand_from add prune list show repos exec cd mv note label hook pr config completion" -a "repos" -d "List repositories"
complete -c wt -n "not __fish_seen_subcommand_from add prune list show repos exec cd mv note label hook pr config completion" -a "exec" -d "Run command in worktree by ID"
complete -c wt -n "not __fish_seen_subcommand_from add prune list show repos exec cd mv note label hook pr config completion" -a "cd" -d "Print worktree path"
complete -c wt -n "not __fish_seen_subcommand_from add prune list show repos exec cd mv note label hook pr config completion" -a "mv" -d "Move worktrees to another directory"
complete -c wt -n "not __fish_seen_subcommand_from add prune list show repos exec cd mv note label hook pr config completion" -a "note" -d "Manage branch notes"
complete -c wt -n "not __fish_seen_subcommand_from add prune list show repos exec cd mv note label hook pr config completion" -a "label" -d "Manage repository labels"
complete -c wt -n "not __fish_seen_subcommand_from add prune list show repos exec cd mv note label hook pr config completion" -a "hook" -d "Run configured hook"
complete -c wt -n "not __fish_seen_subcommand_from add prune list show repos exec cd mv note label hook pr config completion" -a "pr" -d "Work with PRs"
complete -c wt -n "not __fish_seen_subcommand_from add prune list show repos exec cd mv note label hook pr config completion" -a "config" -d "Manage configuration"
complete -c wt -n "not __fish_seen_subcommand_from add prune list show repos exec cd mv note label hook pr config completion" -a "completion" -d "Generate completion script"

# add: branch name (positional), then flags
complete -c wt -n "__fish_seen_subcommand_from add; and not __fish_seen_argument" -a "(git branch --all --format='%(refname:short)' 2>/dev/null | string replace 'origin/' '' | sort -u)" -d "Branch name"
complete -c wt -n "__fish_seen_subcommand_from add" -s b -l new-branch -d "Create new branch"
complete -c wt -n "__fish_seen_subcommand_from add" -s r -l repository -r -a "(__wt_list_repos)" -d "Repository name (repeatable)"
complete -c wt -n "__fish_seen_subcommand_from add" -s l -l label -r -a "(__wt_list_labels)" -d "Target repos by label (repeatable)"
complete -c wt -n "__fish_seen_subcommand_from add" -s d -l dir -r -a "(__fish_complete_directories)" -d "Base directory"
complete -c wt -n "__fish_seen_subcommand_from add" -l base -r -a "(git branch --all --format='%(refname:short)' 2>/dev/null | string replace 'origin/' '' | sort -u)" -d "Base branch to create from"
complete -c wt -n "__fish_seen_subcommand_from add" -s f -l fetch -d "Fetch base branch before creating"
complete -c wt -n "__fish_seen_subcommand_from add" -l note -r -d "Set note on branch"
complete -c wt -n "__fish_seen_subcommand_from add" -l hook -d "Run named hook instead of default"
complete -c wt -n "__fish_seen_subcommand_from add" -l no-hook -d "Skip post-add hook"
complete -c wt -n "__fish_seen_subcommand_from add" -s a -l arg -r -d "Set hook variable KEY=VALUE"

# prune: --id flag, then other flags
complete -c wt -n "__fish_seen_subcommand_from prune" -s i -l id -r -a "(__wt_worktree_ids)" -d "Worktree ID to remove"
complete -c wt -n "__fish_seen_subcommand_from prune" -s d -l dir -r -a "(__fish_complete_directories)" -d "Directory to scan"
complete -c wt -n "__fish_seen_subcommand_from prune" -s n -l dry-run -d "Preview without removing"
complete -c wt -n "__fish_seen_subcommand_from prune" -s f -l force -d "Force remove even if not merged/dirty"
complete -c wt -n "__fish_seen_subcommand_from prune" -s c -l include-clean -d "Also remove clean worktrees"
complete -c wt -n "__fish_seen_subcommand_from prune" -s g -l global -d "Prune all worktrees (not just current repo)"
complete -c wt -n "__fish_seen_subcommand_from prune" -s R -l refresh -d "Fetch origin and refresh PR status"
complete -c wt -n "__fish_seen_subcommand_from prune" -l reset-cache -d "Clear cache and reset IDs from 1"
complete -c wt -n "__fish_seen_subcommand_from prune" -l hook -d "Run named hook instead of default"
complete -c wt -n "__fish_seen_subcommand_from prune" -l no-hook -d "Skip post-removal hooks"
complete -c wt -n "__fish_seen_subcommand_from prune" -s a -l arg -r -d "Set hook variable KEY=VALUE"

# list: flags only (no positional args)
complete -c wt -n "__fish_seen_subcommand_from list" -s d -l dir -r -a "(__fish_complete_directories)" -d "Directory to scan"
complete -c wt -n "__fish_seen_subcommand_from list" -l json -d "Output as JSON"
complete -c wt -n "__fish_seen_subcommand_from list" -s g -l global -d "Show all worktrees (not just current repo)"
complete -c wt -n "__fish_seen_subcommand_from list" -s s -l sort -r -a "id repo branch" -d "Sort by field"
complete -c wt -n "__fish_seen_subcommand_from list" -s R -l refresh -d "Fetch origin and refresh PR status"

# repos: list repositories
complete -c wt -n "__fish_seen_subcommand_from repos" -s d -l dir -r -a "(__fish_complete_directories)" -d "Directory to scan"
complete -c wt -n "__fish_seen_subcommand_from repos" -s l -l label -r -a "(__wt_list_labels)" -d "Filter by label"
complete -c wt -n "__fish_seen_subcommand_from repos" -l json -d "Output as JSON"

# show: --id flag (optional), then other flags
complete -c wt -n "__fish_seen_subcommand_from show" -s i -l id -r -a "(__wt_worktree_ids)" -d "Worktree ID"
complete -c wt -n "__fish_seen_subcommand_from show" -s d -l dir -r -a "(__fish_complete_directories)" -d "Directory to scan"
complete -c wt -n "__fish_seen_subcommand_from show" -s R -l refresh -d "Refresh PR status from API"
complete -c wt -n "__fish_seen_subcommand_from show" -l json -d "Output as JSON"

# exec: --id, -r, -l flags, then -- command
complete -c wt -n "__fish_seen_subcommand_from exec" -s i -l id -r -a "(__wt_worktree_ids)" -d "Worktree ID"
complete -c wt -n "__fish_seen_subcommand_from exec" -s r -l repository -r -a "(__wt_list_repos)" -d "Repository name (repeatable)"
complete -c wt -n "__fish_seen_subcommand_from exec" -s l -l label -r -a "(__wt_list_labels)" -d "Target repos by label (repeatable)"
complete -c wt -n "__fish_seen_subcommand_from exec" -s d -l dir -r -a "(__fish_complete_directories)" -d "Directory to scan"

# cd: --id or -r flag, then flags
complete -c wt -n "__fish_seen_subcommand_from cd" -s i -l id -r -a "(__wt_worktree_ids)" -d "Worktree ID"
complete -c wt -n "__fish_seen_subcommand_from cd" -s r -l repository -r -a "(__wt_list_repos)" -d "Repository name"
complete -c wt -n "__fish_seen_subcommand_from cd" -s d -l dir -r -a "(__fish_complete_directories)" -d "Directory to scan"
complete -c wt -n "__fish_seen_subcommand_from cd" -s p -l project -d "Print main repository path"
complete -c wt -n "__fish_seen_subcommand_from cd" -l hook -d "Run named hook instead of default"
complete -c wt -n "__fish_seen_subcommand_from cd" -l no-hook -d "Skip hooks"
complete -c wt -n "__fish_seen_subcommand_from cd" -s a -l arg -r -d "Set hook variable KEY=VALUE"

# mv: flags only (no positional args)
complete -c wt -n "__fish_seen_subcommand_from mv" -s d -l dir -r -a "(__fish_complete_directories)" -d "Destination directory"
complete -c wt -n "__fish_seen_subcommand_from mv" -l format -d "Worktree naming format"
complete -c wt -n "__fish_seen_subcommand_from mv" -s n -l dry-run -d "Show what would be moved"
complete -c wt -n "__fish_seen_subcommand_from mv" -s f -l force -d "Force move dirty worktrees"

# note: subcommands (get is default, so flags work directly on note)
complete -c wt -n "__fish_seen_subcommand_from note; and not __fish_seen_subcommand_from set get clear" -a "set" -d "Set a note on a branch"
complete -c wt -n "__fish_seen_subcommand_from note; and not __fish_seen_subcommand_from set get clear" -a "get" -d "Get the note for a branch"
complete -c wt -n "__fish_seen_subcommand_from note; and not __fish_seen_subcommand_from set get clear" -a "clear" -d "Clear the note from a branch"
# note: --id and --dir flags work directly (get is default)
complete -c wt -n "__fish_seen_subcommand_from note; and not __fish_seen_subcommand_from set get clear" -s i -l id -r -a "(__wt_worktree_ids)" -d "Worktree ID"
complete -c wt -n "__fish_seen_subcommand_from note; and not __fish_seen_subcommand_from set get clear" -s d -l dir -r -a "(__fish_complete_directories)" -d "Worktree directory for ID lookup"
# note set/get/clear: --id flag (optional), then flags
complete -c wt -n "__fish_seen_subcommand_from note; and __fish_seen_subcommand_from set get clear" -s i -l id -r -a "(__wt_worktree_ids)" -d "Worktree ID"
complete -c wt -n "__fish_seen_subcommand_from note; and __fish_seen_subcommand_from set get clear" -s d -l dir -r -a "(__fish_complete_directories)" -d "Worktree directory for ID lookup"

# label: subcommands (list is default, so flags work directly on label)
complete -c wt -n "__fish_seen_subcommand_from label; and not __fish_seen_subcommand_from add remove list clear" -a "add" -d "Add a label to a repository"
complete -c wt -n "__fish_seen_subcommand_from label; and not __fish_seen_subcommand_from add remove list clear" -a "remove" -d "Remove a label from a repository"
complete -c wt -n "__fish_seen_subcommand_from label; and not __fish_seen_subcommand_from add remove list clear" -a "list" -d "List labels for a repository"
complete -c wt -n "__fish_seen_subcommand_from label; and not __fish_seen_subcommand_from add remove list clear" -a "clear" -d "Clear all labels from a repository"
# label: flags work directly (list is default)
complete -c wt -n "__fish_seen_subcommand_from label; and not __fish_seen_subcommand_from add remove list clear" -s r -l repository -r -a "(__wt_list_repos)" -d "Repository name (repeatable)"
complete -c wt -n "__fish_seen_subcommand_from label; and not __fish_seen_subcommand_from add remove list clear" -s d -l dir -r -a "(__fish_complete_directories)" -d "Directory to scan for repos"
complete -c wt -n "__fish_seen_subcommand_from label; and not __fish_seen_subcommand_from add remove list clear" -s g -l global -d "List all labels across repos"
# label add/remove/clear: -r and -d flags
complete -c wt -n "__fish_seen_subcommand_from label; and __fish_seen_subcommand_from add remove clear" -s r -l repository -r -a "(__wt_list_repos)" -d "Repository name (repeatable)"
complete -c wt -n "__fish_seen_subcommand_from label; and __fish_seen_subcommand_from add remove clear" -s d -l dir -r -a "(__fish_complete_directories)" -d "Directory to scan for repos"
# label list: -r, -d, -g flags
complete -c wt -n "__fish_seen_subcommand_from label; and __fish_seen_subcommand_from list" -s r -l repository -r -a "(__wt_list_repos)" -d "Repository name (repeatable)"
complete -c wt -n "__fish_seen_subcommand_from label; and __fish_seen_subcommand_from list" -s d -l dir -r -a "(__fish_complete_directories)" -d "Directory to scan for repos"
complete -c wt -n "__fish_seen_subcommand_from label; and __fish_seen_subcommand_from list" -s g -l global -d "List all labels across repos"

# hook: multiple hook names supported, then --id/-r/-l (optional), then flags
complete -c wt -n "__fish_seen_subcommand_from hook" -a "(__wt_hook_names)" -d "Hook name"
complete -c wt -n "__fish_seen_subcommand_from hook" -s i -l id -r -a "(__wt_worktree_ids)" -d "Worktree ID"
complete -c wt -n "__fish_seen_subcommand_from hook" -s r -l repository -r -a "(__wt_list_repos)" -d "Repository name (repeatable)"
complete -c wt -n "__fish_seen_subcommand_from hook" -s l -l label -r -a "(__wt_list_labels)" -d "Target repos by label (repeatable)"
complete -c wt -n "__fish_seen_subcommand_from hook" -s d -l dir -r -a "(__fish_complete_directories)" -d "Directory for target lookup"
complete -c wt -n "__fish_seen_subcommand_from hook" -s a -l arg -r -d "Set hook variable KEY=VALUE"

# pr: subcommands
complete -c wt -n "__fish_seen_subcommand_from pr; and not __fish_seen_subcommand_from open clone merge" -a "open" -d "Checkout PR from existing local repo"
complete -c wt -n "__fish_seen_subcommand_from pr; and not __fish_seen_subcommand_from open clone merge" -a "clone" -d "Clone repo and checkout PR"
complete -c wt -n "__fish_seen_subcommand_from pr; and not __fish_seen_subcommand_from open clone merge" -a "merge" -d "Merge PR and clean up worktree"
# pr open: PR number (first positional), then repo (second positional), then flags
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from open" -a "(gh pr list --json number,title --jq '.[] | \"\\(.number)\t\\(.title)\"' 2>/dev/null)" -d "PR number"
# Repo names from worktree_dir (second positional after PR number)
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from open" -a "(__wt_list_repos)" -d "Repository"
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from open" -s d -l dir -r -a "(__fish_complete_directories)" -d "Base directory"
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from open" -l hook -d "Run named hook instead of default"
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from open" -l no-hook -d "Skip post-add hook"
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from open" -s a -l arg -r -d "Set hook variable KEY=VALUE"
# pr clone: PR number (first positional), then org/repo (second positional), then flags
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from clone" -s d -l dir -r -a "(__fish_complete_directories)" -d "Base directory"
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from clone" -l forge -r -a "github gitlab" -d "Forge type"
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from clone" -l note -r -d "Set note on branch"
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from clone" -l hook -d "Run named hook instead of default"
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from clone" -l no-hook -d "Skip post-add hook"
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from clone" -s a -l arg -r -d "Set hook variable KEY=VALUE"
# pr merge: --id flag (optional), then flags
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from merge" -s i -l id -r -a "(__wt_worktree_ids)" -d "Worktree ID"
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from merge" -s d -l dir -r -a "(__fish_complete_directories)" -d "Worktree directory"
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from merge" -s s -l strategy -r -a "squash rebase merge" -d "Merge strategy"
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from merge" -s k -l keep -d "Keep worktree and branch after merge"
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from merge" -l hook -d "Run named hook instead of default"
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from merge" -l no-hook -d "Skip post-merge hook"
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from merge" -s a -l arg -r -d "Set hook variable KEY=VALUE"

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
complete -c wt -n "__fish_seen_subcommand_from config; and __fish_seen_subcommand_from init" -s f -l force -d "Overwrite existing config file"
complete -c wt -n "__fish_seen_subcommand_from config; and __fish_seen_subcommand_from show" -l json -d "Output as JSON"
complete -c wt -n "__fish_seen_subcommand_from config; and __fish_seen_subcommand_from hooks" -l json -d "Output as JSON"

# completion
complete -c wt -n "__fish_seen_subcommand_from completion" -a "fish" -d "Fish shell"
complete -c wt -n "__fish_seen_subcommand_from completion" -a "bash" -d "Bash shell"
complete -c wt -n "__fish_seen_subcommand_from completion" -a "zsh" -d "Zsh shell"

# Global help
complete -c wt -s h -l help -d "Show help message"
`
