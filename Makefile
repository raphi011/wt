VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

.PHONY: build install test test-integration clean snapshot

build:
	go build -ldflags "$(LDFLAGS)" -o wt ./cmd/wt

install:
	go install -ldflags "$(LDFLAGS)" ./cmd/wt

test:
	go test ./...

test-integration:
	go test -tags=integration ./...

clean:
	rm -f wt

snapshot:
	goreleaser release --snapshot --clean
