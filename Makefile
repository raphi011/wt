VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

.PHONY: build install test test-integration clean snapshot testdoc

build:
	go build -ldflags "$(LDFLAGS)" -o wt ./cmd/wt

install:
	go install -ldflags "$(LDFLAGS)" ./cmd/wt

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
