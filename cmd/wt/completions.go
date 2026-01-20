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

    local commands="add prune list exec cd mv note hook pr config completion"

    # Handle subcommand-specific completions
    case "${words[1]}" in
        add)
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
                # Complete branch names (all branches for existing, -b for new)
                local branches=$(git branch --all --format='%(refname:short)' 2>/dev/null | sed 's|origin/||' | sort -u)
                COMPREPLY=($(compgen -W "$branches" -- "$cur"))
            else
                COMPREPLY=($(compgen -W "-b --new-branch -d --dir --note --hook --no-hook" -- "$cur"))
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
            COMPREPLY=($(compgen -W "-i --id -d --dir -n --dry-run -f --force -c --include-clean -a --all -r --refresh --reset-cache --hook --no-hook" -- "$cur"))
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
            COMPREPLY=($(compgen -W "-d --dir --json -a --all -s --sort -r --refresh" -- "$cur"))
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
            esac
            COMPREPLY=($(compgen -W "-i --id -d --dir" -- "$cur"))
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
            esac
            COMPREPLY=($(compgen -W "-i --id -d --dir -p --project" -- "$cur"))
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
                COMPREPLY=($(compgen -W "-i --id -d --dir -s --strategy -k --keep --hook --no-hook" -- "$cur"))
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
                    -i|--id)
                        # Complete worktree IDs only
                        local ids=$(wt list 2>/dev/null | awk '{print $1}')
                        COMPREPLY=($(compgen -W "$ids" -- "$cur"))
                        return
                        ;;
                esac
                COMPREPLY=($(compgen -W "-i --id -d --dir" -- "$cur"))
            elif [[ "${words[2]}" == "get" ]] || [[ "${words[2]}" == "clear" ]]; then
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
                COMPREPLY=($(compgen -W "-i --id -d --dir" -- "$cur"))
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
            esac
            if [[ $cword -eq 2 ]]; then
                # Complete hook names (first positional)
                local hooks=$(wt config hooks 2>/dev/null | awk '{print $1}')
                COMPREPLY=($(compgen -W "$hooks" -- "$cur"))
            else
                COMPREPLY=($(compgen -W "-i --id -d --dir" -- "$cur"))
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
                'add:Add worktree for branch'
                'prune:Prune merged worktrees'
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
                add)
                    _arguments \
                        '1:branch:__wt_all_branches' \
                        '-b[create new branch]' \
                        '--new-branch[create new branch]' \
                        '-d[target directory]:directory:_files -/' \
                        '--dir[target directory]:directory:_files -/' \
                        '--note[set note on branch]:note:' \
                        '--hook[run named hook]:hook:' \
                        '--no-hook[skip post-add hook]'
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
                        '-a[prune all worktrees, not just current repo]' \
                        '--all[prune all worktrees, not just current repo]' \
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
                        '-r[fetch origin and refresh PR status]' \
                        '--refresh[fetch origin and refresh PR status]'
                    ;;
                exec)
                    _arguments \
                        '-i[worktree ID]:id:__wt_worktree_ids' \
                        '--id[worktree ID]:id:__wt_worktree_ids' \
                        '-d[target directory]:directory:_files -/' \
                        '--dir[target directory]:directory:_files -/'
                    ;;
                cd)
                    _arguments \
                        '-i[worktree ID]:id:__wt_worktree_ids' \
                        '--id[worktree ID]:id:__wt_worktree_ids' \
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
                                add-existing)
                                    _arguments \
                                        '1:PR number:' \
                                        '2:repository:' \
                                        '-d[target directory]:directory:_files -/' \
                                        '--dir[target directory]:directory:_files -/' \
                                        '--hook[run named hook]:hook:' \
                                        '--no-hook[skip post-add hook]'
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
                                        '--no-hook[skip post-add hook]'
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
                hook)
                    _arguments \
                        '1:hook name:__wt_hook_names' \
                        '-i[worktree ID]:id:__wt_worktree_ids' \
                        '--id[worktree ID]:id:__wt_worktree_ids' \
                        '-d[worktree directory]:directory:_files -/' \
                        '--dir[worktree directory]:directory:_files -/'
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

_wt "$@"
`

const fishCompletions = `# wt completions - supports fish autosuggestions and tab completion
complete -c wt -f

# Subcommands (shown in completions and autosuggestions)
complete -c wt -n "not __fish_seen_subcommand_from add prune list exec cd mv note hook pr config completion" -a "add" -d "Add worktree for branch"
complete -c wt -n "not __fish_seen_subcommand_from add prune list exec cd mv note hook pr config completion" -a "prune" -d "Prune merged worktrees"
complete -c wt -n "not __fish_seen_subcommand_from add prune list exec cd mv note hook pr config completion" -a "list" -d "List worktrees with stable IDs"
complete -c wt -n "not __fish_seen_subcommand_from add prune list exec cd mv note hook pr config completion" -a "exec" -d "Run command in worktree by ID"
complete -c wt -n "not __fish_seen_subcommand_from add prune list exec cd mv note hook pr config completion" -a "cd" -d "Print worktree path"
complete -c wt -n "not __fish_seen_subcommand_from add prune list exec cd mv note hook pr config completion" -a "mv" -d "Move worktrees to another directory"
complete -c wt -n "not __fish_seen_subcommand_from add prune list exec cd mv note hook pr config completion" -a "note" -d "Manage branch notes"
complete -c wt -n "not __fish_seen_subcommand_from add prune list exec cd mv note hook pr config completion" -a "hook" -d "Manage hooks"
complete -c wt -n "not __fish_seen_subcommand_from add prune list exec cd mv note hook pr config completion" -a "pr" -d "Work with PRs"
complete -c wt -n "not __fish_seen_subcommand_from add prune list exec cd mv note hook pr config completion" -a "config" -d "Manage configuration"
complete -c wt -n "not __fish_seen_subcommand_from add prune list exec cd mv note hook pr config completion" -a "completion" -d "Generate completion script"

# add: branch name (positional), then flags
complete -c wt -n "__fish_seen_subcommand_from add; and not __fish_seen_argument" -a "(git branch --all --format='%(refname:short)' 2>/dev/null | string replace 'origin/' '' | sort -u)" -d "Branch name"
complete -c wt -n "__fish_seen_subcommand_from add" -s b -l new-branch -d "Create new branch"
complete -c wt -n "__fish_seen_subcommand_from add" -s d -l dir -r -a "(__fish_complete_directories)" -d "Base directory"
complete -c wt -n "__fish_seen_subcommand_from add" -l note -r -d "Set note on branch"
complete -c wt -n "__fish_seen_subcommand_from add" -l hook -d "Run named hook instead of default"
complete -c wt -n "__fish_seen_subcommand_from add" -l no-hook -d "Skip post-add hook"

# prune: --id flag, then other flags
complete -c wt -n "__fish_seen_subcommand_from prune" -s i -l id -r -a "(__wt_worktree_ids)" -d "Worktree ID to remove"
complete -c wt -n "__fish_seen_subcommand_from prune" -s d -l dir -r -a "(__fish_complete_directories)" -d "Directory to scan"
complete -c wt -n "__fish_seen_subcommand_from prune" -s n -l dry-run -d "Preview without removing"
complete -c wt -n "__fish_seen_subcommand_from prune" -s f -l force -d "Force remove even if not merged/dirty"
complete -c wt -n "__fish_seen_subcommand_from prune" -s c -l include-clean -d "Also remove clean worktrees"
complete -c wt -n "__fish_seen_subcommand_from prune" -s a -l all -d "Prune all worktrees (not just current repo)"
complete -c wt -n "__fish_seen_subcommand_from prune" -s r -l refresh -d "Fetch origin and refresh PR status"
complete -c wt -n "__fish_seen_subcommand_from prune" -l reset-cache -d "Clear cache and reset IDs from 1"
complete -c wt -n "__fish_seen_subcommand_from prune" -l hook -d "Run named hook instead of default"
complete -c wt -n "__fish_seen_subcommand_from prune" -l no-hook -d "Skip post-removal hooks"

# list: flags only (no positional args)
complete -c wt -n "__fish_seen_subcommand_from list" -s d -l dir -r -a "(__fish_complete_directories)" -d "Directory to scan"
complete -c wt -n "__fish_seen_subcommand_from list" -l json -d "Output as JSON"
complete -c wt -n "__fish_seen_subcommand_from list" -s a -l all -d "Show all worktrees (not just current repo)"
complete -c wt -n "__fish_seen_subcommand_from list" -s s -l sort -r -a "id repo branch" -d "Sort by field"
complete -c wt -n "__fish_seen_subcommand_from list" -s r -l refresh -d "Fetch origin and refresh PR status"

# exec: --id flag (required), then -- command
complete -c wt -n "__fish_seen_subcommand_from exec" -s i -l id -r -a "(__wt_worktree_ids)" -d "Worktree ID"
complete -c wt -n "__fish_seen_subcommand_from exec" -s d -l dir -r -a "(__fish_complete_directories)" -d "Directory to scan"

# cd: --id flag (required), then flags
complete -c wt -n "__fish_seen_subcommand_from cd" -s i -l id -r -a "(__wt_worktree_ids)" -d "Worktree ID"
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
# note set/get/clear: --id flag (optional), then flags
complete -c wt -n "__fish_seen_subcommand_from note; and __fish_seen_subcommand_from set get clear" -s i -l id -r -a "(__wt_worktree_ids)" -d "Worktree ID"
complete -c wt -n "__fish_seen_subcommand_from note; and __fish_seen_subcommand_from set get clear" -s d -l dir -r -a "(__fish_complete_directories)" -d "Worktree directory for ID lookup"

# hook: hook name (required first), then --id (optional), then flags
complete -c wt -n "__fish_seen_subcommand_from hook; and not __fish_seen_argument" -a "(__wt_hook_names)" -d "Hook name"
complete -c wt -n "__fish_seen_subcommand_from hook" -s i -l id -r -a "(__wt_worktree_ids)" -d "Worktree ID"
complete -c wt -n "__fish_seen_subcommand_from hook" -s d -l dir -r -a "(__fish_complete_directories)" -d "Worktree directory for target lookup"

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
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from open" -l no-hook -d "Skip post-add hook"
# pr clone: PR number (first positional), then org/repo (second positional), then flags
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from clone" -s d -l dir -r -a "(__fish_complete_directories)" -d "Base directory"
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from clone" -l forge -r -a "github gitlab" -d "Forge type"
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from clone" -l note -r -d "Set note on branch"
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from clone" -l hook -d "Run named hook instead of default"
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from clone" -l no-hook -d "Skip post-add hook"
# pr merge: --id flag (optional), then flags
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from merge" -s i -l id -r -a "(__wt_worktree_ids)" -d "Worktree ID"
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

# Helper function to list worktree IDs only
function __wt_worktree_ids
    wt list 2>/dev/null | awk '{print $1}'
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
