version := env_var_or_default("VERSION", "dev")
commit := `git rev-parse --short HEAD 2>/dev/null || echo "none"`
date := `date -u +"%Y-%m-%dT%H:%M:%SZ"`
ldflags := "-s -w -X main.version=" + version + " -X main.commit=" + commit + " -X main.date=" + date

build:
    go build -buildvcs=false -ldflags "{{ldflags}}" -o wt ./cmd/wt

install: && install-completions install-hooks
    go install -buildvcs=false -ldflags "{{ldflags}}" ./cmd/wt

install-completions:
    #!/usr/bin/env bash
    set -euo pipefail
    if [[ -z "${HOME:-}" ]]; then
        echo "Warning: \$HOME is not set, skipping completions" >&2
        exit 0
    fi
    gobin="$(go env GOBIN)"
    wt_bin="${gobin:-$(go env GOPATH)/bin}/wt"
    shell_name="$(basename "${SHELL:-}")"
    case "$shell_name" in
        fish)
            dest="$HOME/.config/fish/completions/wt.fish"
            ;;
        bash)
            dest="$HOME/.local/share/bash-completion/completions/wt"
            ;;
        zsh)
            brew_prefix="$(HOMEBREW_NO_AUTO_UPDATE=1 brew --prefix 2>/dev/null)" || brew_prefix=""
            if [[ -n "$brew_prefix" ]]; then
                dest="$brew_prefix/share/zsh/site-functions/_wt"
            else
                dest="$HOME/.zfunc/_wt"
            fi
            ;;
        *)
            if [[ -z "$shell_name" ]]; then
                echo "Warning: \$SHELL is not set, skipping completions" >&2
            else
                echo "Warning: unsupported shell '$shell_name', skipping completions" >&2
            fi
            echo "Run manually: wt completion <shell>" >&2
            exit 0
            ;;
    esac
    mkdir -p "$(dirname "$dest")"
    tmpfile="$(mktemp)"
    trap 'rm -f "$tmpfile"' EXIT
    "$wt_bin" completion "$shell_name" > "$tmpfile"
    if [[ ! -s "$tmpfile" ]]; then
        echo "Error: completion generation produced empty output" >&2
        exit 1
    fi
    mv "$tmpfile" "$dest"
    echo "Installed completions to $dest"

install-hooks:
    git config core.hooksPath .githooks
    @echo "Installed git hooks from .githooks/"

test:
    go test ./...

test-integration:
    WT_TEST_GITHUB_REPO=raphi011/wt-test go test -tags=integration -parallel=8 ./...

clean:
    rm -f wt

snapshot:
    goreleaser release --snapshot --clean

testdoc:
    go run ./tools/testdoc -root . -out docs/TESTS.md -integration
