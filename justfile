version := env_var_or_default("VERSION", "dev")
commit := `git rev-parse --short HEAD 2>/dev/null || echo "none"`
date := `date -u +"%Y-%m-%dT%H:%M:%SZ"`
ldflags := "-s -w -X main.version=" + version + " -X main.commit=" + commit + " -X main.date=" + date

build:
    go build -buildvcs=false -ldflags "{{ldflags}}" -o wt ./cmd/wt

install: && install-completions
    go install -buildvcs=false -ldflags "{{ldflags}}" ./cmd/wt

install-completions:
    #!/usr/bin/env bash
    set -euo pipefail
    shell_name="$(basename "${SHELL:-}")"
    case "$shell_name" in
        fish)
            dest="$HOME/.config/fish/completions/wt.fish"
            mkdir -p "$(dirname "$dest")"
            go run ./cmd/wt completion fish > "$dest"
            ;;
        bash)
            dest="$HOME/.local/share/bash-completion/completions/wt"
            mkdir -p "$(dirname "$dest")"
            go run ./cmd/wt completion bash > "$dest"
            ;;
        zsh)
            if command -v brew &>/dev/null; then
                dest="$(brew --prefix)/share/zsh/site-functions/_wt"
            else
                dest="$HOME/.zfunc/_wt"
            fi
            mkdir -p "$(dirname "$dest")"
            go run ./cmd/wt completion zsh > "$dest"
            ;;
        *)
            echo "Warning: unsupported shell '$shell_name', skipping completions"
            echo "Run manually: wt completion <shell>"
            exit 0
            ;;
    esac
    echo "Installed completions to $dest"

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
