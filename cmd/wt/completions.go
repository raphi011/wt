package main

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

    local commands="create open clean list pr config completion"

    # Handle subcommand-specific completions
    case "${words[1]}" in
        create)
            case "$prev" in
                -d|--dir)
                    COMPREPLY=($(compgen -d -- "$cur"))
                    return
                    ;;
                --hook)
                    return
                    ;;
            esac
            if [[ $cword -eq 2 ]]; then
                # Complete branch names
                local branches=$(git branch --all --format='%(refname:short)' 2>/dev/null | sed 's|origin/||' | sort -u)
                COMPREPLY=($(compgen -W "$branches" -- "$cur"))
            else
                COMPREPLY=($(compgen -W "-d --dir --hook --no-hook" -- "$cur"))
            fi
            ;;
        open)
            case "$prev" in
                -d|--dir)
                    COMPREPLY=($(compgen -d -- "$cur"))
                    return
                    ;;
                --hook)
                    return
                    ;;
            esac
            if [[ $cword -eq 2 ]]; then
                # Complete local branch names only
                local branches=$(git branch --format='%(refname:short)' 2>/dev/null)
                COMPREPLY=($(compgen -W "$branches" -- "$cur"))
            else
                COMPREPLY=($(compgen -W "-d --dir --hook --no-hook" -- "$cur"))
            fi
            ;;
        clean)
            case "$prev" in
                -d|--dir)
                    COMPREPLY=($(compgen -d -- "$cur"))
                    return
                    ;;
            esac
            COMPREPLY=($(compgen -W "-d --dir -n --dry-run --refresh-pr -e --empty" -- "$cur"))
            ;;
        list)
            case "$prev" in
                -d|--dir)
                    COMPREPLY=($(compgen -d -- "$cur"))
                    return
                    ;;
            esac
            COMPREPLY=($(compgen -W "-d --dir --json" -- "$cur"))
            ;;
        pr)
            if [[ $cword -eq 2 ]]; then
                COMPREPLY=($(compgen -W "open" -- "$cur"))
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
            fi
            ;;
        config)
            if [[ $cword -eq 2 ]]; then
                COMPREPLY=($(compgen -W "init hooks" -- "$cur"))
            elif [[ "${words[2]}" == "init" ]]; then
                COMPREPLY=($(compgen -W "-f --force" -- "$cur"))
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
                'clean:Cleanup merged worktrees'
                'list:List worktrees'
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
                        '--hook[run named hook]:hook:' \
                        '--no-hook[skip post-create hook]'
                    ;;
                open)
                    _arguments \
                        '1:branch:__wt_local_branches' \
                        '-d[target directory]:directory:_files -/' \
                        '--dir[target directory]:directory:_files -/' \
                        '--hook[run named hook]:hook:' \
                        '--no-hook[skip post-create hook]'
                    ;;
                clean)
                    _arguments \
                        '-d[target directory]:directory:_files -/' \
                        '--dir[target directory]:directory:_files -/' \
                        '-n[preview without removing]' \
                        '--dry-run[preview without removing]' \
                        '--refresh-pr[force refresh PR cache]' \
                        '-e[also remove 0-commit worktrees]' \
                        '--empty[also remove 0-commit worktrees]'
                    ;;
                list)
                    _arguments \
                        '-d[target directory]:directory:_files -/' \
                        '--dir[target directory]:directory:_files -/' \
                        '--json[output as JSON]'
                    ;;
                pr)
                    _arguments -C \
                        '1: :->subcmd' \
                        '*:: :->args'
                    case $state in
                        subcmd)
                            local subcommands=(
                                'open:Checkout PR as new worktree'
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

_wt "$@"
`

const fishCompletions = `# wt completions - supports fish autosuggestions and tab completion
complete -c wt -f

# Subcommands (shown in completions and autosuggestions)
complete -c wt -n "not __fish_seen_subcommand_from create open clean list pr config completion" -a "create" -d "Create new worktree"
complete -c wt -n "not __fish_seen_subcommand_from create open clean list pr config completion" -a "open" -d "Open worktree for existing branch"
complete -c wt -n "not __fish_seen_subcommand_from create open clean list pr config completion" -a "clean" -d "Cleanup merged worktrees"
complete -c wt -n "not __fish_seen_subcommand_from create open clean list pr config completion" -a "list" -d "List worktrees"
complete -c wt -n "not __fish_seen_subcommand_from create open clean list pr config completion" -a "pr" -d "Work with PRs/MRs"
complete -c wt -n "not __fish_seen_subcommand_from create open clean list pr config completion" -a "config" -d "Manage configuration"
complete -c wt -n "not __fish_seen_subcommand_from create open clean list pr config completion" -a "completion" -d "Generate completion script"

# create: branch name (positional), then flags
complete -c wt -n "__fish_seen_subcommand_from create; and not __fish_seen_argument" -a "(git branch --all --format='%(refname:short)' 2>/dev/null | string replace 'origin/' '' | sort -u)" -d "Branch name"
complete -c wt -n "__fish_seen_subcommand_from create" -s d -l dir -r -a "(__fish_complete_directories)" -d "Base directory"
complete -c wt -n "__fish_seen_subcommand_from create" -l hook -d "Run named hook instead of default"
complete -c wt -n "__fish_seen_subcommand_from create" -l no-hook -d "Skip post-create hook"

# open: local branch name (positional), then flags
complete -c wt -n "__fish_seen_subcommand_from open; and not __fish_seen_argument" -a "(git branch --format='%(refname:short)' 2>/dev/null)" -d "Local branch"
complete -c wt -n "__fish_seen_subcommand_from open" -s d -l dir -r -a "(__fish_complete_directories)" -d "Base directory"
complete -c wt -n "__fish_seen_subcommand_from open" -l hook -d "Run named hook instead of default"
complete -c wt -n "__fish_seen_subcommand_from open" -l no-hook -d "Skip post-create hook"

# clean: flags only (no positional args)
complete -c wt -n "__fish_seen_subcommand_from clean" -s d -l dir -r -a "(__fish_complete_directories)" -d "Directory to scan"
complete -c wt -n "__fish_seen_subcommand_from clean" -s n -l dry-run -d "Preview without removing"
complete -c wt -n "__fish_seen_subcommand_from clean" -l refresh-pr -d "Force refresh PR cache"
complete -c wt -n "__fish_seen_subcommand_from clean" -s e -l empty -d "Also remove worktrees with 0 commits ahead"

# list: flags only (no positional args)
complete -c wt -n "__fish_seen_subcommand_from list" -s d -l dir -r -a "(__fish_complete_directories)" -d "Directory to scan"
complete -c wt -n "__fish_seen_subcommand_from list" -l json -d "Output as JSON"

# pr: subcommands
complete -c wt -n "__fish_seen_subcommand_from pr; and not __fish_seen_subcommand_from open" -a "open" -d "Checkout PR/MR as new worktree"
# pr open: PR number (first positional), then repo (second positional), then flags
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from open" -a "(gh pr list --json number,title --jq '.[] | \"\\(.number)\t\\(.title)\"' 2>/dev/null)" -d "PR number"
# Repo names from default_path (second positional after PR number)
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from open" -a "(__wt_list_repos)" -d "Repository"
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from open" -s d -l dir -r -a "(__fish_complete_directories)" -d "Base directory"
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from open" -l hook -d "Run named hook instead of default"
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from open" -l no-hook -d "Skip post-create hook"

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

# config: subcommands
complete -c wt -n "__fish_seen_subcommand_from config; and not __fish_seen_subcommand_from init hooks" -a "init" -d "Create default config file"
complete -c wt -n "__fish_seen_subcommand_from config; and not __fish_seen_subcommand_from init hooks" -a "hooks" -d "List available hooks"
complete -c wt -n "__fish_seen_subcommand_from config; and __fish_seen_subcommand_from init" -s f -l force -d "Overwrite existing config file"
complete -c wt -n "__fish_seen_subcommand_from config; and __fish_seen_subcommand_from hooks" -l json -d "Output as JSON"

# completion
complete -c wt -n "__fish_seen_subcommand_from completion" -a "fish" -d "Fish shell"
complete -c wt -n "__fish_seen_subcommand_from completion" -a "bash" -d "Bash shell"
complete -c wt -n "__fish_seen_subcommand_from completion" -a "zsh" -d "Zsh shell"

# Global help
complete -c wt -s h -l help -d "Show help message"
`
