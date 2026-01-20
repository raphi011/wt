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

    local commands="create open tidy list exec cd mv note hook pr config completion"

    # Handle subcommand-specific completions
    case "${words[1]}" in
        create)
            case "$prev" in
                -d|--dir)
                    COMPREPLY=($(compgen -d -- "$cur"))
                    return
                    ;;
                --hook|--note)
                    return
                    ;;
            esac
            if [[ $cword -eq 2 ]]; then
                # Complete branch names
                local branches=$(git branch --all --format='%(refname:short)' 2>/dev/null | sed 's|origin/||' | sort -u)
                COMPREPLY=($(compgen -W "$branches" -- "$cur"))
            else
                COMPREPLY=($(compgen -W "-d --dir --note --hook --no-hook" -- "$cur"))
            fi
            ;;
        open)
            case "$prev" in
                -d|--dir)
                    COMPREPLY=($(compgen -d -- "$cur"))
                    return
                    ;;
                --hook|--note)
                    return
                    ;;
            esac
            if [[ $cword -eq 2 ]]; then
                # Complete local branch names only
                local branches=$(git branch --format='%(refname:short)' 2>/dev/null)
                COMPREPLY=($(compgen -W "$branches" -- "$cur"))
            else
                COMPREPLY=($(compgen -W "-d --dir --note --hook --no-hook" -- "$cur"))
            fi
            ;;
        tidy)
            case "$prev" in
                -d|--dir)
                    COMPREPLY=($(compgen -d -- "$cur"))
                    return
                    ;;
                --hook)
                    return
                    ;;
            esac
            if [[ $cword -eq 2 ]] && [[ "$cur" != -* ]]; then
                # Complete worktree IDs and branch names (optional target)
                local ids=$(wt list 2>/dev/null | awk '{print $1}')
                local branches=$(wt list 2>/dev/null | awk '{print $3}')
                COMPREPLY=($(compgen -W "$ids $branches" -- "$cur"))
            else
                COMPREPLY=($(compgen -W "-d --dir -n --dry-run -f --force -c --include-clean -r --refresh --reset-cache --hook --no-hook" -- "$cur"))
            fi
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
            COMPREPLY=($(compgen -W "-d --dir --json -a --all -s --sort -r --reverse" -- "$cur"))
            ;;
        exec)
            case "$prev" in
                -d|--dir)
                    COMPREPLY=($(compgen -d -- "$cur"))
                    return
                    ;;
            esac
            if [[ $cword -eq 2 ]]; then
                # Complete worktree IDs and branch names
                local ids=$(wt list 2>/dev/null | awk '{print $1}')
                local branches=$(wt list 2>/dev/null | awk '{print $3}')
                COMPREPLY=($(compgen -W "$ids $branches" -- "$cur"))
            else
                COMPREPLY=($(compgen -W "-d --dir" -- "$cur"))
            fi
            ;;
        cd)
            case "$prev" in
                -d|--dir)
                    COMPREPLY=($(compgen -d -- "$cur"))
                    return
                    ;;
            esac
            if [[ $cword -eq 2 ]]; then
                # Complete worktree IDs and branch names
                local ids=$(wt list 2>/dev/null | awk '{print $1}')
                local branches=$(wt list 2>/dev/null | awk '{print $3}')
                COMPREPLY=($(compgen -W "$ids $branches" -- "$cur"))
            else
                COMPREPLY=($(compgen -W "-d --dir -p --project" -- "$cur"))
            fi
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
                COMPREPLY=($(compgen -W "-d --dir --hook --no-hook" -- "$cur"))
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
                COMPREPLY=($(compgen -W "-d --dir --forge --note --hook --no-hook" -- "$cur"))
            elif [[ "${words[2]}" == "merge" ]]; then
                case "$prev" in
                    -d|--dir)
                        COMPREPLY=($(compgen -d -- "$cur"))
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
                if [[ $cword -eq 3 ]]; then
                    # Complete worktree IDs and branch names
                    local ids=$(wt list 2>/dev/null | awk '{print $1}')
                    local branches=$(wt list 2>/dev/null | awk '{print $3}')
                    COMPREPLY=($(compgen -W "$ids $branches" -- "$cur"))
                else
                    COMPREPLY=($(compgen -W "-d --dir -s --strategy -k --keep --hook --no-hook" -- "$cur"))
                fi
            fi
            ;;
        note)
            if [[ $cword -eq 2 ]]; then
                COMPREPLY=($(compgen -W "set get clear" -- "$cur"))
            elif [[ "${words[2]}" == "set" ]]; then
                case "$prev" in
                    -d|--dir)
                        COMPREPLY=($(compgen -d -- "$cur"))
                        return
                        ;;
                esac
                if [[ $cword -eq 4 ]]; then
                    # Complete worktree IDs and branch names (second positional after text)
                    local ids=$(wt list 2>/dev/null | awk '{print $1}')
                    local branches=$(wt list 2>/dev/null | awk '{print $3}')
                    COMPREPLY=($(compgen -W "$ids $branches" -- "$cur"))
                else
                    COMPREPLY=($(compgen -W "-d --dir" -- "$cur"))
                fi
            elif [[ "${words[2]}" == "get" ]] || [[ "${words[2]}" == "clear" ]]; then
                case "$prev" in
                    -d|--dir)
                        COMPREPLY=($(compgen -d -- "$cur"))
                        return
                        ;;
                esac
                if [[ $cword -eq 3 ]]; then
                    # Complete worktree IDs and branch names
                    local ids=$(wt list 2>/dev/null | awk '{print $1}')
                    local branches=$(wt list 2>/dev/null | awk '{print $3}')
                    COMPREPLY=($(compgen -W "$ids $branches" -- "$cur"))
                else
                    COMPREPLY=($(compgen -W "-d --dir" -- "$cur"))
                fi
            fi
            ;;
        hook)
            if [[ $cword -eq 2 ]]; then
                COMPREPLY=($(compgen -W "run" -- "$cur"))
            elif [[ "${words[2]}" == "run" ]]; then
                case "$prev" in
                    -d|--dir)
                        COMPREPLY=($(compgen -d -- "$cur"))
                        return
                        ;;
                esac
                if [[ $cword -eq 3 ]]; then
                    # Complete hook names (first positional)
                    local hooks=$(wt config hooks 2>/dev/null | awk '{print $1}')
                    COMPREPLY=($(compgen -W "$hooks" -- "$cur"))
                elif [[ $cword -eq 4 ]]; then
                    # Complete worktree IDs and branch names (second positional, optional)
                    local ids=$(wt list 2>/dev/null | awk '{print $1}')
                    local branches=$(wt list 2>/dev/null | awk '{print $3}')
                    COMPREPLY=($(compgen -W "$ids $branches" -- "$cur"))
                else
                    COMPREPLY=($(compgen -W "-d --dir" -- "$cur"))
                fi
            fi
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
                'create:Create new worktree for a branch'
                'open:Open worktree for existing local branch'
                'tidy:Tidy up merged worktrees'
                'list:List worktrees with stable IDs'
                'exec:Run command in worktree by ID'
                'cd:Print worktree path for shell scripting'
                'mv:Move worktrees to another directory'
                'note:Manage branch notes'
                'hook:Manage hooks'
                'pr:Work with GitHub PRs'
                'config:Manage configuration'
                'completion:Generate completion script'
            )
            _describe 'command' commands
            ;;
        args)
            case $words[1] in
                create)
                    _arguments \
                        '1:branch:__wt_all_branches' \
                        '-d[target directory]:directory:_files -/' \
                        '--dir[target directory]:directory:_files -/' \
                        '--note[set note on branch]:note:' \
                        '--hook[run named hook]:hook:' \
                        '--no-hook[skip post-create hook]'
                    ;;
                open)
                    _arguments \
                        '1:branch:__wt_local_branches' \
                        '-d[target directory]:directory:_files -/' \
                        '--dir[target directory]:directory:_files -/' \
                        '--note[set note on branch]:note:' \
                        '--hook[run named hook]:hook:' \
                        '--no-hook[skip post-create hook]'
                    ;;
                tidy)
                    _arguments \
                        '1:worktree ID or branch:__wt_worktree_targets' \
                        '-d[target directory]:directory:_files -/' \
                        '--dir[target directory]:directory:_files -/' \
                        '-n[preview without removing]' \
                        '--dry-run[preview without removing]' \
                        '-f[force remove even if not merged/dirty]' \
                        '--force[force remove even if not merged/dirty]' \
                        '-c[also remove clean worktrees]' \
                        '--include-clean[also remove clean worktrees]' \
                        '-r[fetch origin and refresh PR status]' \
                        '--refresh[fetch origin and refresh PR status]' \
                        '--reset-cache[clear cache and reset IDs from 1]' \
                        '--hook[run named hook]:hook:' \
                        '--no-hook[skip post-removal hooks]'
                    ;;
                list)
                    _arguments \
                        '-d[target directory]:directory:_files -/' \
                        '--dir[target directory]:directory:_files -/' \
                        '--json[output as JSON]' \
                        '-a[show all worktrees]' \
                        '--all[show all worktrees]' \
                        '-s[sort by]:field:(id repo branch)' \
                        '--sort[sort by]:field:(id repo branch)' \
                        '-r[reverse sort order]' \
                        '--reverse[reverse sort order]'
                    ;;
                exec)
                    _arguments \
                        '1:worktree ID or branch:__wt_worktree_targets' \
                        '-d[target directory]:directory:_files -/' \
                        '--dir[target directory]:directory:_files -/'
                    ;;
                cd)
                    _arguments \
                        '1:worktree ID or branch:__wt_worktree_targets' \
                        '-d[directory to scan]:directory:_files -/' \
                        '--dir[directory to scan]:directory:_files -/' \
                        '-p[print main repository path]' \
                        '--project[print main repository path]'
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
                                open)
                                    _arguments \
                                        '1:PR number:' \
                                        '2:repository:' \
                                        '-d[target directory]:directory:_files -/' \
                                        '--dir[target directory]:directory:_files -/' \
                                        '--hook[run named hook]:hook:' \
                                        '--no-hook[skip post-create hook]'
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
                                        '--no-hook[skip post-create hook]'
                                    ;;
                                merge)
                                    _arguments \
                                        '1:worktree ID or branch:__wt_worktree_targets' \
                                        '-d[worktree directory]:directory:_files -/' \
                                        '--dir[worktree directory]:directory:_files -/' \
                                        '-s[merge strategy]:strategy:(squash rebase merge)' \
                                        '--strategy[merge strategy]:strategy:(squash rebase merge)' \
                                        '-k[keep worktree and branch after merge]' \
                                        '--keep[keep worktree and branch after merge]' \
                                        '--hook[run named hook]:hook:' \
                                        '--no-hook[skip post-merge hook]'
                                    ;;
                            esac
                            ;;
                    esac
                    ;;
                note)
                    _arguments -C \
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
                                        '2:worktree ID or branch:__wt_worktree_targets' \
                                        '-d[worktree directory]:directory:_files -/' \
                                        '--dir[worktree directory]:directory:_files -/'
                                    ;;
                                get|clear)
                                    _arguments \
                                        '1:worktree ID or branch:__wt_worktree_targets' \
                                        '-d[worktree directory]:directory:_files -/' \
                                        '--dir[worktree directory]:directory:_files -/'
                                    ;;
                            esac
                            ;;
                    esac
                    ;;
                hook)
                    _arguments -C \
                        '1: :->subcmd' \
                        '*:: :->args'
                    case $state in
                        subcmd)
                            local subcommands=(
                                'run:Run a hook by name'
                            )
                            _describe 'subcommand' subcommands
                            ;;
                        args)
                            case $words[1] in
                                run)
                                    _arguments \
                                        '1:hook name:__wt_hook_names' \
                                        '2:worktree ID or branch:__wt_worktree_targets' \
                                        '-d[worktree directory]:directory:_files -/' \
                                        '--dir[worktree directory]:directory:_files -/'
                                    ;;
                            esac
                            ;;
                    esac
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

# Helper: complete worktree IDs and branch names
__wt_worktree_targets() {
    local targets
    targets=(${(f)"$(wt list 2>/dev/null | awk '{print $1; print $3}')"})
    _describe 'worktree ID or branch' targets
}

# Helper: complete hook names
__wt_hook_names() {
    local hooks
    hooks=(${(f)"$(wt config hooks 2>/dev/null | awk '{print $1}')"})
    _describe 'hook name' hooks
}

_wt "$@"
`

const fishCompletions = `# wt completions - supports fish autosuggestions and tab completion
complete -c wt -f

# Subcommands (shown in completions and autosuggestions)
complete -c wt -n "not __fish_seen_subcommand_from create open tidy list exec cd mv note hook pr config completion" -a "create" -d "Create new worktree"
complete -c wt -n "not __fish_seen_subcommand_from create open tidy list exec cd mv note hook pr config completion" -a "open" -d "Open worktree for existing branch"
complete -c wt -n "not __fish_seen_subcommand_from create open tidy list exec cd mv note hook pr config completion" -a "tidy" -d "Tidy up merged worktrees"
complete -c wt -n "not __fish_seen_subcommand_from create open tidy list exec cd mv note hook pr config completion" -a "list" -d "List worktrees with stable IDs"
complete -c wt -n "not __fish_seen_subcommand_from create open tidy list exec cd mv note hook pr config completion" -a "exec" -d "Run command in worktree by ID"
complete -c wt -n "not __fish_seen_subcommand_from create open tidy list exec cd mv note hook pr config completion" -a "cd" -d "Print worktree path"
complete -c wt -n "not __fish_seen_subcommand_from create open tidy list exec cd mv note hook pr config completion" -a "mv" -d "Move worktrees to another directory"
complete -c wt -n "not __fish_seen_subcommand_from create open tidy list exec cd mv note hook pr config completion" -a "note" -d "Manage branch notes"
complete -c wt -n "not __fish_seen_subcommand_from create open tidy list exec cd mv note hook pr config completion" -a "hook" -d "Manage hooks"
complete -c wt -n "not __fish_seen_subcommand_from create open tidy list exec cd mv note hook pr config completion" -a "pr" -d "Work with PRs"
complete -c wt -n "not __fish_seen_subcommand_from create open tidy list exec cd mv note hook pr config completion" -a "config" -d "Manage configuration"
complete -c wt -n "not __fish_seen_subcommand_from create open tidy list exec cd mv note hook pr config completion" -a "completion" -d "Generate completion script"

# create: branch name (positional), then flags
complete -c wt -n "__fish_seen_subcommand_from create; and not __fish_seen_argument" -a "(git branch --all --format='%(refname:short)' 2>/dev/null | string replace 'origin/' '' | sort -u)" -d "Branch name"
complete -c wt -n "__fish_seen_subcommand_from create" -s d -l dir -r -a "(__fish_complete_directories)" -d "Base directory"
complete -c wt -n "__fish_seen_subcommand_from create" -l note -r -d "Set note on branch"
complete -c wt -n "__fish_seen_subcommand_from create" -l hook -d "Run named hook instead of default"
complete -c wt -n "__fish_seen_subcommand_from create" -l no-hook -d "Skip post-create hook"

# open: local branch name (positional), then flags
complete -c wt -n "__fish_seen_subcommand_from open; and not __fish_seen_argument" -a "(git branch --format='%(refname:short)' 2>/dev/null)" -d "Local branch"
complete -c wt -n "__fish_seen_subcommand_from open" -s d -l dir -r -a "(__fish_complete_directories)" -d "Base directory"
complete -c wt -n "__fish_seen_subcommand_from open" -l note -r -d "Set note on branch"
complete -c wt -n "__fish_seen_subcommand_from open" -l hook -d "Run named hook instead of default"
complete -c wt -n "__fish_seen_subcommand_from open" -l no-hook -d "Skip post-create hook"

# tidy: optional target (ID or branch), then flags
complete -c wt -n "__fish_seen_subcommand_from tidy; and not __fish_seen_argument" -a "(__wt_worktree_targets)" -d "Worktree ID or branch"
complete -c wt -n "__fish_seen_subcommand_from tidy" -s d -l dir -r -a "(__fish_complete_directories)" -d "Directory to scan"
complete -c wt -n "__fish_seen_subcommand_from tidy" -s n -l dry-run -d "Preview without removing"
complete -c wt -n "__fish_seen_subcommand_from tidy" -s f -l force -d "Force remove even if not merged/dirty"
complete -c wt -n "__fish_seen_subcommand_from tidy" -s c -l include-clean -d "Also remove clean worktrees"
complete -c wt -n "__fish_seen_subcommand_from tidy" -s r -l refresh -d "Fetch origin and refresh PR status"
complete -c wt -n "__fish_seen_subcommand_from tidy" -l reset-cache -d "Clear cache and reset IDs from 1"
complete -c wt -n "__fish_seen_subcommand_from tidy" -l hook -d "Run named hook instead of default"
complete -c wt -n "__fish_seen_subcommand_from tidy" -l no-hook -d "Skip post-removal hooks"

# list: flags only (no positional args)
complete -c wt -n "__fish_seen_subcommand_from list" -s d -l dir -r -a "(__fish_complete_directories)" -d "Directory to scan"
complete -c wt -n "__fish_seen_subcommand_from list" -l json -d "Output as JSON"
complete -c wt -n "__fish_seen_subcommand_from list" -s a -l all -d "Show all worktrees (not just current repo)"
complete -c wt -n "__fish_seen_subcommand_from list" -s s -l sort -r -a "id repo branch" -d "Sort by field"
complete -c wt -n "__fish_seen_subcommand_from list" -s r -l reverse -d "Reverse sort order"

# exec: worktree ID or branch (positional), then -- command
complete -c wt -n "__fish_seen_subcommand_from exec; and not __fish_seen_argument" -a "(__wt_worktree_targets)" -d "Worktree ID or branch"
complete -c wt -n "__fish_seen_subcommand_from exec" -s d -l dir -r -a "(__fish_complete_directories)" -d "Directory to scan"

# cd: worktree ID or branch (positional), then flags
complete -c wt -n "__fish_seen_subcommand_from cd; and not __fish_seen_argument" -a "(__wt_worktree_targets)" -d "Worktree ID or branch"
complete -c wt -n "__fish_seen_subcommand_from cd" -s d -l dir -r -a "(__fish_complete_directories)" -d "Directory to scan"
complete -c wt -n "__fish_seen_subcommand_from cd" -s p -l project -d "Print main repository path"

# mv: flags only (no positional args)
complete -c wt -n "__fish_seen_subcommand_from mv" -s d -l dir -r -a "(__fish_complete_directories)" -d "Destination directory"
complete -c wt -n "__fish_seen_subcommand_from mv" -l format -d "Worktree naming format"
complete -c wt -n "__fish_seen_subcommand_from mv" -s n -l dry-run -d "Show what would be moved"
complete -c wt -n "__fish_seen_subcommand_from mv" -s f -l force -d "Force move dirty worktrees"

# note: subcommands
complete -c wt -n "__fish_seen_subcommand_from note; and not __fish_seen_subcommand_from set get clear" -a "set" -d "Set a note on a branch"
complete -c wt -n "__fish_seen_subcommand_from note; and not __fish_seen_subcommand_from set get clear" -a "get" -d "Get the note for a branch"
complete -c wt -n "__fish_seen_subcommand_from note; and not __fish_seen_subcommand_from set get clear" -a "clear" -d "Clear the note from a branch"
# note set/get/clear: worktree ID or branch (positional), then flags
complete -c wt -n "__fish_seen_subcommand_from note; and __fish_seen_subcommand_from set get clear" -a "(__wt_worktree_targets)" -d "Worktree ID or branch"
complete -c wt -n "__fish_seen_subcommand_from note; and __fish_seen_subcommand_from set get clear" -s d -l dir -r -a "(__fish_complete_directories)" -d "Worktree directory for ID lookup"

# hook: subcommands
complete -c wt -n "__fish_seen_subcommand_from hook; and not __fish_seen_subcommand_from run" -a "run" -d "Run a hook by name"
# hook run: hook name (required first), then worktree ID/branch (optional), then flags
complete -c wt -n "__fish_seen_subcommand_from hook; and __fish_seen_subcommand_from run" -a "(__wt_hook_names)" -d "Hook name"
complete -c wt -n "__fish_seen_subcommand_from hook; and __fish_seen_subcommand_from run" -a "(__wt_worktree_targets)" -d "Worktree ID or branch"
complete -c wt -n "__fish_seen_subcommand_from hook; and __fish_seen_subcommand_from run" -s d -l dir -r -a "(__fish_complete_directories)" -d "Worktree directory for target lookup"

# pr: subcommands
complete -c wt -n "__fish_seen_subcommand_from pr; and not __fish_seen_subcommand_from open clone merge" -a "open" -d "Checkout PR from existing local repo"
complete -c wt -n "__fish_seen_subcommand_from pr; and not __fish_seen_subcommand_from open clone merge" -a "clone" -d "Clone repo and checkout PR"
complete -c wt -n "__fish_seen_subcommand_from pr; and not __fish_seen_subcommand_from open clone merge" -a "merge" -d "Merge PR and clean up worktree"
# pr open: PR number (first positional), then repo (second positional), then flags
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from open" -a "(gh pr list --json number,title --jq '.[] | \"\\(.number)\t\\(.title)\"' 2>/dev/null)" -d "PR number"
# Repo names from default_path (second positional after PR number)
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from open" -a "(__wt_list_repos)" -d "Repository"
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from open" -s d -l dir -r -a "(__fish_complete_directories)" -d "Base directory"
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from open" -l hook -d "Run named hook instead of default"
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from open" -l no-hook -d "Skip post-create hook"
# pr clone: PR number (first positional), then org/repo (second positional), then flags
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from clone" -s d -l dir -r -a "(__fish_complete_directories)" -d "Base directory"
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from clone" -l forge -r -a "github gitlab" -d "Forge type"
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from clone" -l note -r -d "Set note on branch"
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from clone" -l hook -d "Run named hook instead of default"
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from clone" -l no-hook -d "Skip post-create hook"
# pr merge: worktree ID or branch (positional), then flags
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from merge; and not __fish_seen_argument" -a "(__wt_worktree_targets)" -d "Worktree ID or branch"
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from merge" -s d -l dir -r -a "(__fish_complete_directories)" -d "Worktree directory"
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from merge" -s s -l strategy -r -a "squash rebase merge" -d "Merge strategy"
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from merge" -s k -l keep -d "Keep worktree and branch after merge"
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from merge" -l hook -d "Run named hook instead of default"
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from merge" -l no-hook -d "Skip post-merge hook"

# Helper function to list repos in default_path
function __wt_list_repos
    set -l dir "$WT_DEFAULT_PATH"
    if test -z "$dir"
        set -l config_file ~/.config/wt/config.toml
        if test -f "$config_file"
            set dir (grep '^default_path' "$config_file" 2>/dev/null | sed 's/.*= *"\?\([^"]*\)"\?/\1/' | string replace '~' "$HOME")
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

# Helper function to list worktree IDs and branch names
function __wt_worktree_targets
    wt list 2>/dev/null | awk '{print $1; print $3}'
end

# Helper function to list hook names
function __wt_hook_names
    wt config hooks 2>/dev/null | awk '{print $1}'
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
